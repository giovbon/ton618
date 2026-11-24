import { test, expect, Page } from '@playwright/test';

// E2E da nota Tabulator (/tabulator): cobre o fluxo de inserção de dados —
// inicialização da planilha, tipo de coluna, adicionar registro, editar célula,
// autosave, persistência após recarregar, ordenação (visual), filtro rápido,
// filtro por coluna, coluna de ARQUIVO (upload por clique na célula, remoção do
// arquivo da célula/registro apagando o arquivo do disco) e exportação CSV.
//
// Observação: /tabulator NÃO é a página /database (administração de notas); é o
// editor de uma nota-planilha de inserção de dados.

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

// Responde os `prompt()`/`confirm()` na ordem em que aparecem.
function answerDialogs(page: Page, prompts: string[]) {
  let i = 0;
  page.on('dialog', async dialog => {
    if (dialog.type() === 'prompt') {
      await dialog.accept(prompts[i++] ?? '');
    } else {
      await dialog.accept();
    }
  });
}

async function waitForTable(page: Page) {
  await expect(page.locator('#tabulator-table .tabulator-header')).toBeVisible({ timeout: 20000 });
}

// Edita a célula de uma coluna numa linha (índice 0-based) e confirma.
async function editCell(page: Page, rowIndex: number, field: string, value: string) {
  const cell = page
    .locator('#tabulator-table .tabulator-row')
    .nth(rowIndex)
    .locator(`.tabulator-cell[tabulator-field="${field}"]`);
  await cell.click();
  const input = cell.locator('input');
  await expect(input).toBeVisible({ timeout: 5000 });
  await input.fill(value);
  await input.press('Enter');
}

async function columnValues(page: Page, field: string) {
  return page
    .locator('#tabulator-table .tabulator-row')
    .locator(`.tabulator-cell[tabulator-field="${field}"]`)
    .allTextContents();
}

// Abre o menu ⋮ de uma coluna (pelo título) e escolhe um item.
async function columnMenu(page: Page, columnTitle: string, itemLabel: string) {
  const col = page.locator('#tabulator-table .tabulator-col').filter({ hasText: columnTitle }).first();
  await col.locator('.tabulator-header-popup-button').click();
  await page.locator('.tabulator-menu').getByText(itemLabel, { exact: true }).click();
}

// Abre o menu ⋮ → "Tipo da coluna" e escolhe o tipo no SUBMENU ao lado.
async function pickColumnType(page: Page, columnTitle: string, typeName: string) {
  await columnMenu(page, columnTitle, 'Tipo da coluna');
  const item = page.locator('.tabulator-menu-item').filter({ hasText: typeName }).first();
  await expect(item).toBeVisible({ timeout: 5000 });
  await item.click();
}

test.describe('Nota Tabulator', () => {
  test('insere registros, edita células, salva, adiciona coluna e exporta CSV', async ({ page }) => {
    answerDialogs(page, ['Nova']);
    page.on('pageerror', err => console.error('BROWSER ERROR:', err.message));

    await page.goto('/tabulator');
    await waitForTable(page);

    // Colunas default presentes.
    await expect(
      page.locator('#tabulator-table .tabulator-col-title').filter({ hasText: 'Coluna 1' }),
    ).toBeVisible();

    // "Novo registro" adiciona uma linha.
    await page.click('#tab-add-row');
    await expect(page.locator('#tabulator-table .tabulator-row')).toHaveCount(2, { timeout: 10000 });

    // Edita a 1ª célula de dados da 1ª linha.
    await editCell(page, 0, 'c1', 'Ana');

    // Autosave (debounce ~1.2s) → indicador "salvo".
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });

    // A URL passa a apontar para a nota salva no banco.
    await expect.poll(() => decodeURIComponent(page.url())).toContain('file=notes/');
    const filename = decodeURIComponent(page.url().split('file=')[1]);
    expect(filename).toMatch(/^notes\/.+\.md$/);

    // Recarrega e confirma a persistência da célula.
    await page.reload();
    await waitForTable(page);
    await expect(page.locator('#tabulator-table .tabulator-row').first()).toContainText('Ana');

    // "+ Nova coluna" (sem select de tipo) → aparece no cabeçalho, com filtro.
    await page.click('#tab-add-col');
    const newCol = page.locator('#tabulator-table .tabulator-col').filter({ hasText: 'Nova' });
    await expect(newCol).toBeVisible({ timeout: 10000 });
    await expect(newCol.locator('.tabulator-header-filter input')).toBeVisible();
    await expect(page.locator('#tab-col-type')).toHaveCount(0);

    // Menu do cabeçalho (⋮): renomear / tipo da coluna / excluir coluna.
    const col1 = page.locator('#tabulator-table .tabulator-col[tabulator-field="c1"]');
    await col1.locator('.tabulator-header-popup-button').click();
    const headerMenu = page.locator('.tabulator-menu');
    await expect(headerMenu).toBeVisible({ timeout: 5000 });
    await expect(headerMenu).toContainText('Renomear coluna');
    await expect(headerMenu).toContainText('Tipo da coluna');
    await expect(headerMenu).toContainText('Excluir coluna');

    // "Tipo da coluna" abre um SUBMENU ao lado com os tipos — clique, sem digitar.
    await headerMenu.getByText('Tipo da coluna', { exact: true }).click();
    for (const label of ['Texto', 'Número', 'Data', 'Sim/Não', 'Arquivo']) {
      await expect(page.locator('.tab-type-name').filter({ hasText: label })).toBeVisible({ timeout: 5000 });
    }
    // O tipo atual (Texto) já vem marcado e não existe campo de texto.
    await expect(page.locator('.tab-type-item.is-current .tab-type-name')).toHaveText('Texto');
    await expect(page.locator('.tab-type-item input')).toHaveCount(0);
    // Escolher o tipo atual não muda nada e fecha o menu.
    await page.locator('.tabulator-menu-item').filter({ hasText: 'Texto' }).first().click();
    await expect(page.locator('.tab-type-name')).toHaveCount(0);

    // Exporta o CSV (download anexado pelo servidor).
    const [download] = await Promise.all([
      page.waitForEvent('download', { timeout: 20000 }),
      page.click('#tabulator-export-btn'),
    ]);
    expect(download.suggestedFilename()).toMatch(/\.csv$/);
  });

  test('ordena e filtra apenas a visão, sem reordenar os dados gravados', async ({ page }) => {
    answerDialogs(page, ['Coluna']);

    await page.goto('/tabulator');
    await waitForTable(page);

    // Duas linhas: 1ª "Bruno", 2ª "Ana" (ordem canônica = ordem de inserção).
    await page.click('#tab-add-row');
    await expect(page.locator('#tabulator-table .tabulator-row')).toHaveCount(2, { timeout: 10000 });
    await editCell(page, 0, 'c1', 'Bruno');
    await editCell(page, 1, 'c1', 'Ana');

    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
    await expect.poll(async () => (await columnValues(page, 'c1')).join(',')).toBe('Bruno,Ana');

    // Ordena ascendente pela coluna 1 (clique no título).
    await page.click('#tabulator-table .tabulator-col[tabulator-field="c1"] .tabulator-col-title');
    await expect.poll(async () => (await columnValues(page, 'c1')).join(',')).toBe('Ana,Bruno');

    // Filtro rápido da tabela (visual) → contador "1 de 2".
    await page.fill('#tab-filter', 'Bruno');
    await expect(page.locator('#tab-count')).toContainText('1 de 2 registro(s)', { timeout: 10000 });
    await expect.poll(async () => (await columnValues(page, 'c1')).join(',')).toBe('Bruno');

    // Limpa o filtro rápido.
    await page.fill('#tab-filter', '');
    await expect(page.locator('#tab-count')).toContainText('2 registro(s)', { timeout: 10000 });

    // Filtro por coluna (input no cabeçalho) → contador "1 de 2".
    const headerFilter = page.locator(
      '#tabulator-table .tabulator-col[tabulator-field="c1"] .tabulator-header-filter input',
    );
    await headerFilter.fill('Ana');
    await expect(page.locator('#tab-count')).toContainText('1 de 2 registro(s)', { timeout: 10000 });
    await headerFilter.fill('');
    await expect(page.locator('#tab-count')).toContainText('2 registro(s)', { timeout: 10000 });

    // Garante que o save já terminou e recarrega: a ordem gravada deve continuar
    // sendo a de inserção (Bruno, Ana) — ordenar/filtrar é só visual.
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
    await page.reload();
    await waitForTable(page);
    await expect.poll(async () => (await columnValues(page, 'c1')).join(',')).toBe('Bruno,Ana');
  });

  test('coluna de arquivo: clique na célula envia o arquivo e o link persiste', async ({ page }) => {
    // Prompt só do nome da nova coluna; o TIPO é escolhido no submodal ⋮.
    answerDialogs(page, ['Anexo']);

    await page.goto('/tabulator');
    await waitForTable(page);

    // Cria a coluna e muda o tipo para "arquivo" pelo menu ⋮.
    await page.click('#tab-add-col');
    await expect(
      page.locator('#tabulator-table .tabulator-col').filter({ hasText: 'Anexo' }),
    ).toBeVisible({ timeout: 10000 });
    await pickColumnType(page, 'Anexo', 'Arquivo');
    // O tipo é persistido desta forma no JSON da nota.
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
    const field = await page
      .locator('#tabulator-table .tabulator-col')
      .filter({ hasText: 'Anexo' })
      .first()
      .getAttribute('tabulator-field');
    expect(field).toBeTruthy();

    // Clique na célula abre o seletor de arquivos → envia "relatorio.pdf".
    const cell = page
      .locator('#tabulator-table .tabulator-row')
      .first()
      .locator(`.tabulator-cell[tabulator-field="${field}"]`);
    await expect(cell.locator('.tab-file-empty')).toBeVisible({ timeout: 5000 });

    const [chooser] = await Promise.all([page.waitForEvent('filechooser'), cell.click()]);
    await chooser.setFiles({
      name: 'relatorio.pdf',
      mimeType: 'application/pdf',
      buffer: Buffer.from('%PDF-1.4 conteudo de teste'),
    });

    // A célula passa a mostrar o link do arquivo (guardado em datasets/).
    const link = cell.locator('.tab-file-link');
    await expect(link).toHaveText('relatorio.pdf', { timeout: 15000 });
    expect(await link.getAttribute('href')).toContain('/file/download?name=datasets/relatorio');

    // O arquivo é baixável (prefixo datasets/ é permitido em /file/download).
    const resp = await page.request.get('/file/download?name=' + encodeURIComponent('datasets/relatorio.pdf').replace(/%2F/gi, '/'));
    expect(resp.status()).toBe(200);
    expect(await resp.text()).toContain('%PDF-1.4');

    // Persiste após recarregar.
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
    await page.reload();
    await waitForTable(page);
    await expect(
      page
        .locator('#tabulator-table .tabulator-row')
        .first()
        .locator(`.tabulator-cell[tabulator-field="${field}"] .tab-file-link`),
    ).toHaveText('relatorio.pdf');

    // O "×" remove o arquivo da célula...
    await page
      .locator('#tabulator-table .tabulator-row')
      .first()
      .locator(`.tabulator-cell[tabulator-field="${field}"] .tab-file-del`)
      .click();
    await expect(
      page
        .locator('#tabulator-table .tabulator-row')
        .first()
        .locator(`.tabulator-cell[tabulator-field="${field}"] .tab-file-empty`),
    ).toBeVisible();

    // ...e o arquivo também é apagado do disco (não fica órfão em datasets/),
    // depois do autosave.
    const fileURL = (name: string) =>
      '/file/download?name=' + encodeURIComponent(name).replace(/%2F/gi, '/');
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
    await expect
      .poll(async () => (await page.request.get(fileURL('datasets/relatorio.pdf'))).status(), {
        timeout: 20000,
      })
      .toBe(404);

    // Envia outro arquivo e exclui o REGISTRO: o arquivo vai junto.
    const [chooser2] = await Promise.all([page.waitForEvent('filechooser'), cell.click()]);
    await chooser2.setFiles({
      name: 'planilha.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from('a,b\n1,2\n'),
    });
    await expect(cell.locator('.tab-file-link')).toHaveText('planilha.csv', { timeout: 15000 });
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
    expect((await page.request.get(fileURL('datasets/planilha.csv'))).status()).toBe(200);

    await page.locator('#tabulator-table .tabulator-row').first().locator('.tab-del-btn').click();
    await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
    await expect
      .poll(async () => (await page.request.get(fileURL('datasets/planilha.csv'))).status(), {
        timeout: 20000,
      })
      .toBe(404);
  });

  // O desenho/cores da tabela vêm do arquivo ÚNICO compartilhado com a
  // /database (static/tabulator-theme.css, classe .ton-tabulator). As MESMAS
  // cores são verificadas em database.spec.ts — se uma tela mudar sozinha, um
  // dos dois testes quebra.
  test('usa o tema visual compartilhado com a página /database', async ({ page }) => {
    await page.goto('/tabulator');
    await waitForTable(page);
    await page.click('#tab-add-row');
    await expect(page.locator('#tabulator-table .tabulator-row')).toHaveCount(2, { timeout: 10000 });
    await editCell(page, 0, 'c1', 'Ana');

    // Sem hover: o clique acima deixaria a linha com a cor de hover.
    await page.mouse.move(4, 4);

    // O tema é o arquivo compartilhado (não um bloco de estilo local).
    const themeLoaded = await page.evaluate(() =>
      Array.from(document.styleSheets).some(s => (s.href || '').includes('tabulator-theme.css')),
    );
    expect(themeLoaded).toBe(true);

    const header = page.locator('#tabulator-table .tabulator-header');
    await expect(header).toHaveCSS('background-color', 'rgb(27, 27, 32)'); // #1b1b20
    await expect(header).toHaveCSS('border-bottom-width', '2px');
    await expect(header).toHaveCSS('border-bottom-color', 'rgb(63, 63, 70)'); // #3f3f46
    await expect(page.locator('#tabulator-table .tabulator-header .tabulator-col').first()).toHaveCSS(
      'background-color',
      'rgb(27, 27, 32)',
    );
    await expect(page.locator('#tabulator-table .tabulator-row').first()).toHaveCSS(
      'background-color',
      'rgb(18, 18, 21)', // #121215
    );
    await expect(
      page.locator('#tabulator-table .tabulator-row .tabulator-cell').first(),
    ).toHaveCSS('border-right-color', 'rgb(38, 38, 44)'); // #26262c
  });
});
