import { test, expect } from '@playwright/test';
import fs from 'node:fs';

test.beforeEach(async ({ page }) => {
  // Block all external network requests to prevent hangs offline
  await page.route(url => !url.href.includes('localhost') && !url.href.includes('127.0.0.1'), async route => {
    await route.abort('failed');
  });

  await page.goto('/login');
  await page.fill('#pass', 'ton618');
  await page.click('#login-btn');
  await page.waitForURL('/', { waitUntil: 'commit' });
});

test.describe('Note Lifecycle', () => {
  test('duplicates and deletes a note', async ({ page }) => {
    // 1. Create a new note to play with
    await page.goto('/editor');
    const editor = page.locator('#editor-content .ProseMirror');
    await expect(editor).toBeVisible({ timeout: 10000 });
    
    // Listen to console and page errors
    page.on('console', msg => console.log('BROWSER LOG:', msg.text()));
    page.on('pageerror', err => console.error('BROWSER ERROR:', err.message));

    // Wait 1 second to ensure lastSavedHash initialization timeout (500ms) has fired on the empty editor
    await page.waitForTimeout(1000);

    // Type some content to trigger auto-save
    await editor.click();
    await page.keyboard.type('Test note content for E2E lifecycle');

    // Wait for auto-save UI status indicator to show saved (✓)
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 15000 });

    const originalUrl = page.url();

    // 2. Duplicate the note
    const duplicateBtn = page.locator('#duplicate-editor-btn');
    await expect(duplicateBtn).toBeVisible();

    // Handle the browser confirm dialog automatically
    page.on('dialog', dialog => dialog.accept());

    // Click duplicate and wait for navigation
    await Promise.all([
      page.waitForNavigation(), // It redirects to the new file
      duplicateBtn.click()
    ]);

    // Ensure we are on a new file
    expect(page.url()).not.toBe(originalUrl);
    expect(decodeURIComponent(page.url())).toContain('/editor?file=notes/');

    // 3. Delete the duplicated note
    const deleteBtn = page.locator('#delete-editor-btn');
    await expect(deleteBtn).toBeVisible();

    // Click delete and wait for navigation to home /
    await Promise.all([
      page.waitForNavigation(),
      deleteBtn.click()
    ]);

    // Should redirect to root
    expect(new URL(page.url()).pathname).toBe('/');
  });

  test('downloads the exact markdown of the note', async ({ page }) => {
    // Aceita o confirm() do excluir no final do teste
    page.on('dialog', dialog => dialog.accept());

    await page.goto('/editor');
    const editor = page.locator('#editor-content .ProseMirror');
    await expect(editor).toBeVisible({ timeout: 10000 });

    // Frontmatter preenchido no painel: precisa sair no .md baixado
    await page.click('#toggle-fm-btn');
    await page.fill('#frontmatter-area', 'tags: [teste-download]');

    await editor.click();
    await page.keyboard.type('Conteudo exato para download');
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 15000 });

    const downloadBtn = page.locator('#download-editor-btn');
    await expect(downloadBtn).toBeVisible();

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      downloadBtn.click(),
    ]);

    // Só o basename, com extensão .md
    expect(download.suggestedFilename()).toMatch(/\.md$/);
    expect(download.suggestedFilename()).not.toContain('/');

    const content = fs.readFileSync((await download.path()) as string, 'utf8');

    // Frontmatter do painel + corpo do editor, na mesma ordem do save
    expect(content).toMatch(/^---\ntags: \[teste-download\]\n---\n/);
    expect(content).toContain('Conteudo exato para download');

    // Limpa a nota criada pelo teste
    await Promise.all([
      page.waitForNavigation(),
      page.click('#delete-editor-btn'),
    ]);
    expect(new URL(page.url()).pathname).toBe('/');
  });
});
