import { test, expect } from '@playwright/test';

// E2E da página /database (Tabulator): #7 — confiança em refactor. Cobre o
// fluxo de inicialização da tabela e o alternar de visibilidade de coluna
// (seletor "Colunas" → checkboxes → localStorage), sem depender do conteúdo das
// notas (as colunas são definidas no servidor, independente de haver linhas).

test.beforeEach(async ({ page }) => {
  // Block all external network requests to prevent hangs offline.
  await page.route(url => !url.href.includes('localhost') && !url.href.includes('127.0.0.1'), async route => {
    await route.abort('failed');
  });

  // Login (Basic auth, senha padrão ton618).
  await page.goto('/login');
  await page.fill('#pass', 'ton618');
  await page.click('#login-btn');
  await page.waitForURL('/', { waitUntil: 'commit' });
});

test('Tabulator renderiza a tabela e o seletor de colunas funciona', async ({ page }) => {
  await page.goto('/database');

  // Estado determinístico: limpa a visibilidade salva no localStorage e recarrega
  // (o toggle persiste entre visitas; sem limpar o teste dependeria de estado prévio).
  await page.evaluate(() => localStorage.removeItem('db_column_visibility'));
  await page.reload();

  // O Tabulator inicializou: o cabeçalho existe.
  await expect(page.locator('#notes-table .tabulator-header')).toBeVisible();

  // Abre o seletor de colunas.
  await page.click('#column-selector-btn');
  await expect(page.locator('#column-selector-menu')).toBeVisible();

  // O dropdown foi populado com checkboxes (initTabulator rodeou).
  await expect(page.locator('#column-checkboxes label').first()).toBeVisible();

  // A coluna "Tipo" existe e começa visível (checked por padrão).
  const tipoLabel = page.locator('#column-checkboxes label').filter({ hasText: 'Tipo' });
  await expect(tipoLabel).toBeVisible();
  const tipoCheckbox = tipoLabel.locator('input[type="checkbox"]');
  await expect(tipoCheckbox).toBeChecked();

  // Desmarca → o change handler persiste `type: false` no localStorage (ocultar).
  await tipoCheckbox.uncheck();
  await expect(tipoCheckbox).not.toBeChecked();
  await expect
    .poll(() => page.evaluate(() => JSON.parse(localStorage.getItem('db_column_visibility') || '{}').type))
    .toBe(false);

  // Remarca → `type: true` (mostrar) de novo.
  await tipoCheckbox.check();
  await expect(tipoCheckbox).toBeChecked();
  await expect
    .poll(() => page.evaluate(() => JSON.parse(localStorage.getItem('db_column_visibility') || '{}').type))
    .toBe(true);
});

test('coluna "Modificação" começa oculta e pode ser mostrada', async ({ page }) => {
  await page.goto('/database');
  await page.evaluate(() => localStorage.removeItem('db_column_visibility'));
  await page.reload();

  await expect(page.locator('#notes-table .tabulator-header')).toBeVisible();
  await page.click('#column-selector-btn');
  await expect(page.locator('#column-selector-menu')).toBeVisible();

  // "Modificação" (campo mtime) nasce com visible:false → checkbox desmarcada.
  const mtimeLabel = page.locator('#column-checkboxes label').filter({ hasText: 'Modificação' });
  await expect(mtimeLabel).toBeVisible();
  const mtimeCheckbox = mtimeLabel.locator('input[type="checkbox"]');
  await expect(mtimeCheckbox).not.toBeChecked();

  // Mostra.
  await mtimeCheckbox.check();
  await expect(mtimeCheckbox).toBeChecked();
  await expect
    .poll(() => page.evaluate(() => JSON.parse(localStorage.getItem('db_column_visibility') || '{}').mtime))
    .toBe(true);
});

// A /database usa o MESMO tema visual da nota Tabulator (arquivo compartilhado
// static/tabulator-theme.css, classe .ton-tabulator) — as cores abaixo são as
// mesmas verificadas em tabulator.spec.ts — e a coluna "Tipo" exibe APENAS o
// valor cru do campo `tipo:` do frontmatter (ex: "tabulator"), sem ícone nem
// rótulo legível (formatter antigo removido — DECISIONS §17).
test('usa o tema compartilhado e a coluna Tipo exibe o valor cru sem ícone', async ({ page }) => {
  // Cria uma nota Tabulator (mesmo fluxo da nota real) para haver uma linha.
  await page.goto('/tabulator');
  await expect(page.locator('#tabulator-table .tabulator-header')).toBeVisible({ timeout: 20000 });
  await page.click('#tab-add-row');
  await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });

  await page.goto('/database');
  await expect(page.locator('#notes-table .tabulator-header')).toBeVisible({ timeout: 20000 });
  await page.mouse.move(4, 4); // sem hover em nenhuma linha

  const themeLoaded = await page.evaluate(() =>
    Array.from(document.styleSheets).some(s => (s.href || '').includes('tabulator-theme.css')),
  );
  expect(themeLoaded).toBe(true);

  const header = page.locator('#notes-table .tabulator-header');
  await expect(header).toHaveCSS('background-color', 'rgb(27, 27, 32)'); // #1b1b20
  await expect(header).toHaveCSS('border-bottom-width', '2px');
  await expect(header).toHaveCSS('border-bottom-color', 'rgb(63, 63, 70)'); // #3f3f46
  await expect(page.locator('#notes-table .tabulator-header .tabulator-col').first()).toHaveCSS(
    'background-color',
    'rgb(27, 27, 32)',
  );
  await expect(page.locator('#notes-table .tabulator-row').first()).toHaveCSS(
    'background-color',
    'rgb(18, 18, 21)', // #121215
  );
  await expect(page.locator('#notes-table .tabulator-row .tabulator-cell').first()).toHaveCSS(
    'border-right-color',
    'rgb(38, 38, 44)', // #26262c
  );

  // Coluna "Tipo": apenas o valor cru do campo `tipo:` (sem SVG — o formatter
  // de ícone + rótulo foi removido, ver DECISIONS §17).
  const typeCell = page
    .locator('#notes-table .tabulator-cell[tabulator-field="type"]')
    .filter({ hasText: /^tabulator$/ })
    .first();
  await expect(typeCell).toHaveText('tabulator');
  await expect(typeCell.locator('svg')).toHaveCount(0);
});
