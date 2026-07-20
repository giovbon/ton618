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
  try {
    await page.waitForURL('/', { waitUntil: 'commit', timeout: 10000 });
  } catch (e) {
    console.log('Current URL on login timeout:', page.url());
    const errorText = await page.locator('#login-error').textContent().catch(() => 'no error text');
    console.log('Login error element text:', errorText);
    await page.screenshot({ path: 'login-failure.png' });
    throw e;
  }
});

test.describe('Agenda', () => {
  test('loads agenda page and timeline', async ({ page }) => {
    await page.goto('/agenda');

    // Verify title and basic layout
    await expect(page).toHaveTitle(/Agenda/);
    
    // Timeline container should be visible
    const timeline = page.locator('#agenda-timeline');
    await expect(timeline).toBeVisible();

    // Input area should be visible
    const input = page.locator('#agenda-input');
    await expect(input).toBeVisible();
    await expect(input).toHaveAttribute('placeholder', /Novo compromisso/);
  });

  test('creates a new appointment', async ({ page }) => {
    await page.goto('/agenda');

    const input = page.locator('#agenda-input');
    const saveBtn = page.locator('#agenda-save');
    
    // Type an appointment string
    const testAppointment = 'Reunião Playwright amanhã as 15h';
    await input.fill(testAppointment);
    
    // Wait for the preview to be generated via JS (chrono)
    const preview = page.locator('#agenda-preview');
    await expect(preview).toContainText('Data reconhecida:', { timeout: 10000 });

    // The backend uses a POST to /api/appointments/create
    const responsePromise = page.waitForResponse(response => 
      response.url().includes('/api/appointments/create') && (response.status() === 200 || response.status() === 201)
    );
    
    await saveBtn.click();
    
    // Wait for creation
    await responsePromise;

    // After creation, the input should be cleared
    await expect(input).toHaveValue('');
  });
});
