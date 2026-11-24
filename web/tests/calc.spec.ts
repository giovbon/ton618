import { test, expect, Page } from '@playwright/test';

/**
 * E2E do bloco de cálculo (/calculator) — DECISIONS §6.31.
 *
 * Cobre o ciclo completo: inserir via slash menu, resultados ao vivo à direita
 * da mesma linha, erro em pt-BR, cascata de recálculo ao editar uma variável e
 * round-trip markdown (salva ```calc sem resultados e recalcula ao reabrir).
 */
test.describe('Calculadora /calculator', () => {
  test.beforeEach(async ({ page }) => {
    // Bloqueia rede externa (mesmo padrão de editor.spec.ts)
    await page.route(
      (url) => !url.href.includes('localhost') && !url.href.includes('127.0.0.1'),
      async (route) => {
        await route.abort('failed');
      },
    );

    await page.goto('/login');
    await page.fill('#pass', 'ton618');
    await page.click('#login-btn');
    await page.waitForURL('/', { waitUntil: 'commit' });
  });

  /** Abre um editor novo (nota nova com CUID2) e insere o bloco via /calculator. */
  async function openEditorWithCalcBlock(page: Page) {
    await page.goto('/editor');
    const editor = page.locator('.ProseMirror');
    try {
      await expect(editor).toBeVisible({ timeout: 10000 });
    } catch (e) {
      await page.screenshot({ path: 'calc-failure-editor.png' });
      throw e;
    }
    // Espera a inicialização do hash de save (500ms), como editor.spec.ts
    await page.waitForTimeout(1000);
    await editor.click();

    await page.keyboard.type('/');
    await expect(page.locator('#slash-menu')).toBeVisible({ timeout: 5000 });
    await page.keyboard.type('calc');
    await expect(page.locator('#slash-items .slash-item').first()).toContainText(
      'Calculadora',
    );
    await page.keyboard.press('Enter');

    await expect(page.locator('.calc-block')).toBeVisible({ timeout: 5000 });
  }

  test('insere via / e mostra resultados à direita, na mesma linha', async ({
    page,
  }) => {
    await openEditorWithCalcBlock(page);

    // Cada linha avalia ao vivo e o resultado aparece alinhado à direita
    await page.keyboard.type('var1 = 10');
    await expect(page.locator('.calc-result').first()).toHaveText('= 10');

    await page.keyboard.press('Enter');
    await page.keyboard.type('var2 = 20');
    await expect(page.locator('.calc-result').nth(1)).toHaveText('= 20');

    await page.keyboard.press('Enter');
    await page.keyboard.type('var1 + var2');
    await expect(page.locator('.calc-result').nth(2)).toHaveText('= 30');

    // Erro em pt-BR inline, na própria linha (nunca quebra o editor)
    await page.keyboard.press('Enter');
    await page.keyboard.type('1 / 0');
    await expect(page.locator('.calc-error').first()).toContainText(
      'divisão por zero',
    );

    // Tudo dentro do bloco, uma linha por <p>
    await expect(page.locator('.calc-block')).toHaveCount(1);
    await expect(page.locator('.calc-block p')).toHaveCount(4);

    // Regressão dos dois bugs de layout: o widget fica DENTRO do <p> (senão o
    // resultado cai numa linha abaixo) e alinhado à direita da MESMA linha.
    const geometry = await page.evaluate(() => {
      return [...document.querySelectorAll('.calc-block p')].map((p) => {
        const span = p.querySelector('.calc-result, .calc-error');
        if (!span) return { hasSpan: false };
        const row = p.getBoundingClientRect();
        const res = span.getBoundingClientRect();
        return {
          hasSpan: true,
          spanInsideP: span.parentElement === p,
          sameLine: res.top >= row.top - 2 && res.bottom <= row.bottom + 2,
          atRight: Math.abs(res.right - row.right) < 4,
          flushLeft: p.firstElementChild === span || row.left < res.left,
        };
      });
    });
    for (const g of geometry) {
      expect(g.hasSpan).toBe(true);
      expect(g.spanInsideP).toBe(true);
      expect(g.sameLine).toBe(true);
      expect(g.atRight).toBe(true);
    }

    // Regressão do caret: linha NOVA (sem widget) não pode herdar o flex do
    // <p> — o Chromium não pinta cursor em contêiner flex vazio.
    await page.keyboard.press('Enter');
    const lastRowDisplay = await page.evaluate(() => {
      const rows = document.querySelectorAll('.calc-block p');
      return getComputedStyle(rows[rows.length - 1]).display;
    });
    expect(lastRowDisplay).toBe('block');
  });

  test('editar uma variável recalcula as linhas dependentes na hora', async ({
    page,
  }) => {
    await openEditorWithCalcBlock(page);

    await page.keyboard.type('var1 = 10');
    await page.keyboard.press('Enter');
    await page.keyboard.type('var2 = 20');
    await page.keyboard.press('Enter');
    await page.keyboard.type('var1 + var2');
    await expect(page.locator('.calc-result').nth(2)).toHaveText('= 30');

    // volta para a primeira linha e muda 10 → 50
    await page.keyboard.press('Control+Home');
    await page.keyboard.press('End');
    await page.keyboard.press('Backspace');
    await page.keyboard.press('Backspace');
    await page.keyboard.type('50');

    await expect(page.locator('.calc-result').first()).toHaveText('= 50');
    await expect(page.locator('.calc-result').nth(2)).toHaveText('= 70');
  });

  test('med() aceita quantos valores quiser na mesma linha', async ({ page }) => {
    await openEditorWithCalcBlock(page);

    await page.keyboard.type('nota1 = 8');
    await page.keyboard.press('Enter');
    await page.keyboard.type('nota2 = 6');
    await page.keyboard.press('Enter');
    await page.keyboard.type('nota3 = 9');
    await page.keyboard.press('Enter');
    await page.keyboard.type('med(nota1, nota2, nota3)');
    await expect(page.locator('.calc-result').nth(3)).toHaveText('= 7,666666666667');

    // de 1 a N valores — aqui 5 literais
    await page.keyboard.press('Enter');
    await page.keyboard.type('med(10, 20, 30, 40, 50)');
    await expect(page.locator('.calc-result').nth(4)).toHaveText('= 30');

    // sem valores → erro em pt-BR na própria linha
    await page.keyboard.press('Enter');
    await page.keyboard.type('med()');
    await expect(page.locator('.calc-error').last()).toContainText(
      'espera ao menos 1 valor',
    );
  });

  test('salva só as expressões (```calc) e recalcula ao reabrir', async ({
    page,
  }) => {
    await openEditorWithCalcBlock(page);

    await page.keyboard.type('var1 = 10');
    await page.keyboard.press('Enter');
    await page.keyboard.type('var2 = 20');
    await page.keyboard.press('Enter');
    await page.keyboard.type('var1 + var2');

    // autosave (debounce de 2s)
    await expect(page.locator('#editor-status')).toHaveText('✓', {
      timeout: 15000,
    });

    // /editor sem parâmetro usa notes/<cuid>.md no dataset do input de título
    const filename = await page.locator('#file-name').getAttribute('data-filename');
    expect(filename).toBeTruthy();

    await page.goto(
      '/editor?file=' + encodeURIComponent(filename as string),
    );
    const editor = page.locator('.ProseMirror');
    await expect(editor).toBeVisible({ timeout: 10000 });
    await page.waitForTimeout(1000);

    // Round-trip: o bloco volta e os resultados são recalculados
    await expect(page.locator('.calc-block')).toHaveCount(1);
    await expect(page.locator('.calc-block p')).toHaveCount(3);
    await expect(page.locator('.calc-result').nth(2)).toHaveText('= 30');

    // O markdown gravado tem as expressões, nunca os resultados (§6.31)
    const saved = await page.locator('#content-output').inputValue();
    expect(saved).toContain('```calc');
    expect(saved).toContain('var1 = 10');
    expect(saved).toContain('var1 + var2');
    expect(saved).not.toContain('= 30');
  });
});
