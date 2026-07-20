import { test, expect } from '@playwright/test';

test.describe('TipTap Editor', () => {
  test.beforeEach(async ({ page }) => {
    // Block all external network requests to prevent hangs offline
    await page.route(url => !url.href.includes('localhost') && !url.href.includes('127.0.0.1'), async route => {
      await route.abort('failed');
    });

    await page.goto('/login');
    await page.fill('#user', 'admin');
    await page.fill('#pass', 'ton618');
    await page.click('#login-btn');
    await page.waitForURL('/', { waitUntil: 'commit' });
  });

  test('creates a new note and auto-saves', async ({ page }) => {
    // 1. Navigate to the editor for a new note
    await page.goto('/editor');

    // 2. Wait for the TipTap editor to be ready
    const editor = page.locator('.ProseMirror');
    try {
      await expect(editor).toBeVisible({ timeout: 10000 });
    } catch (e) {
      await page.screenshot({ path: 'editor-failure.png' });
      throw e;
    }

    // Wait 1 second to ensure lastSavedHash initialization timeout (500ms) has fired on the empty editor
    await page.waitForTimeout(1000);

    // 3. Type text into the editor
    await editor.click();
    await page.keyboard.type('# Playwright Test Note');
    await page.keyboard.press('Enter');
    await page.keyboard.type('This note was created by automated E2E tests.');

    // 4. Verify that an auto-save request is completed in the UI
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 15000 });

    // Verify the text is actually in the editor DOM
    await expect(editor).toContainText('Playwright Test Note');
    await expect(editor).toContainText('This note was created by automated E2E tests.');
  });
});
