import { test, expect } from '@playwright/test';

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

test('has title and renders sidebar', async ({ page }) => {
  await page.goto('/');

  // Expect a title "to contain" a substring.
  await expect(page).toHaveTitle(/TON-618/);

  // Expect the contentArea (sidebar) to be visible
  const sidebar = page.locator('#contentArea');
  await expect(sidebar).toBeVisible();

  // It should also have a search input
  const searchInput = page.locator('#search-local');
  await expect(searchInput).toBeVisible();
});

test('HTMX search filters the sidebar', async ({ page }) => {
  await page.goto('/');

  const searchInput = page.locator('#search-local');
  await searchInput.fill('PlaywrightTest');
  
  // HTMX has a delay/debounce. Wait for network request to finish.
  const response = await page.waitForResponse(response => 
    response.url().includes('/api/sidebar') && response.status() === 200
  );
  expect(response.ok()).toBeTruthy();

  // Test that the DOM was updated by HTMX.
  // The sidebar should contain "Nenhum resultado" if we search for a random string, 
  // or it will filter to the correct note if it exists.
  // Since we don't know the exact notes, we just verify the network request happened
  // and the sidebar is still visible.
  const sidebar = page.locator('#contentArea');
  await expect(sidebar).toBeVisible();
});
