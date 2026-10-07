import { test, expect, Page } from '@playwright/test';

/**
 * query-panel.spec.ts — E2E do painel de Consultas (DECISIONS §6.29).
 *
 * O que só um teste de navegador pode provar aqui:
 *   1. o fragmento é carregado pelo HTMX no `hx-trigger="load"`;
 *   2. o botão "atualizar" faz uma NOVA requisição (troca de conteúdo real);
 *   3. salvar a nota dispara `query-blocks-updated` e o painel se recarrega
 *      SOZINHO — e o container sobrevive ao swap (senão o listener morreria);
 *   4. erro de consulta vira texto no cartão, com HTTP 200.
 *
 * ⚠️ Roda contra o servidor de verdade (vault real). Por isso as notas usam uma
 * etiqueta única (gerada no run) e são apagadas no `finally` — nenhuma asserção
 * depende do conteúdo do acervo.
 */

const CARIMBO = Date.now();
const ETIQUETA = `zz-e2e-consulta-${CARIMBO}`;
const NOTA_ALFA = `notes/zz-e2e-alfa-${CARIMBO}.md`;
const NOTA_PAINEL = `notes/zz-e2e-painel-${CARIMBO}.md`;
const NOTA_ERRO = `notes/zz-e2e-erro-${CARIMBO}.md`;
const NOTA_SEM_BLOCO = `notes/zz-e2e-simples-${CARIMBO}.md`;

/** Bloco ```consulta com o YAML informado. */
function bloco(yaml: string): string {
  return '```consulta\n' + yaml + '\n```';
}

async function login(page: Page) {
  // Bloqueia rede externa para não travar offline (padrão dos outros specs).
  await page.route(url => !url.href.includes('localhost') && !url.href.includes('127.0.0.1'), async route => {
    await route.abort('failed');
  });

  // O app é de usuário único: o formulário pede SÓ a senha (o "admin" é implícito).
  await page.goto('/login');
  await page.fill('#pass', 'ton618');
  await page.click('#login-btn');
  await page.waitForURL('/', { waitUntil: 'commit' });
}

async function salvarNota(page: Page, filename: string, content: string, tags = '') {
  const resp = await page.request.post('/api/note/save', {
    form: { filename, content, tags, silent: 'true' },
  });
  expect(resp.status(), `salvar ${filename}`).toBeLessThan(300);
}

async function apagarNotas(page: Page, files: string[]) {
  for (const f of files) {
    await page.request.post('/api/notes/delete', { form: { filename: f } }).catch(() => {});
  }
}

/** Abre o editor da nota e espera o conteúdo aparecer. */
async function abrirEditor(page: Page, filename: string, trecho: string) {
  await page.goto(`/editor?file=${encodeURIComponent(filename)}`);
  await expect(page.locator('.ProseMirror')).toContainText(trecho, { timeout: 20000 });
}

test.describe('Painel de Consultas', () => {
  test('mostra o resultado, atualiza pelo botão e recarrega após salvar', async ({ page }) => {
    await login(page);

    try {
      // Duas notas com a etiqueta única: o resultado da consulta é previsível
      // mesmo com qualquer acervo real por trás.
      await salvarNota(page, NOTA_ALFA, '# Alfa\n\nnota alfa de teste\n', ETIQUETA);
      await salvarNota(
        page,
        NOTA_PAINEL,
        `# Painel\n\n${bloco(`tipo: notas\ntags: [${ETIQUETA}]\nmostrar: contagem`)}\n`
      );

      await abrirEditor(page, NOTA_PAINEL, 'tipo: notas');

      const painel = page.locator('#query-panel');
      await expect(painel.getByText('Consultas')).toBeVisible({ timeout: 20000 });
      await expect(painel).toContainText('1', { timeout: 20000 }); // 1 nota com a etiqueta
      await expect(painel).toContainText('notas');

      // 1) O texto do bloco continua sendo o que eu escrevi: o resultado NUNCA é
      // gravado na nota (senão o watcher reindexaria e o processo viraria loop).
      await expect(page.locator('.ProseMirror')).toContainText('tags:');

      // 2) Botão "atualizar" → nova requisição de verdade ao endpoint.
      const pedidoManual = page.waitForRequest(r => r.url().includes('/api/query'), { timeout: 20000 });
      await painel.getByRole('button', { name: 'atualizar' }).click();
      await pedidoManual;
      await expect(painel).toContainText('1');

      // 3) Salvar a nota dispara `query-blocks-updated` → o painel se recarrega
      // sozinho, sem reload da página.
      const pedidoAutomatico = page.waitForRequest(r => r.url().includes('/api/query'), { timeout: 25000 });
      await page.locator('.ProseMirror h1').click();
      await page.keyboard.press('End');
      await page.keyboard.type('!');
      await expect(page.locator('#editor-status')).toHaveText('✓', { timeout: 20000 });
      await pedidoAutomatico;

      // ...e o container sobreviveu à troca (swap innerHTML): o botão continua lá.
      await expect(painel.getByRole('button', { name: 'atualizar' })).toBeVisible();
      await expect(painel).toContainText('1');
    } finally {
      await apagarNotas(page, [NOTA_PAINEL, NOTA_ALFA]);
    }
  });

  test('bloco inválido vira mensagem no cartão, nunca erro HTTP', async ({ page }) => {
    await login(page);

    const status: number[] = [];
    page.on('response', r => {
      if (r.url().includes('/api/query')) status.push(r.status());
    });

    try {
      await salvarNota(page, NOTA_ERRO, `# Erro\n\n${bloco('tipo: notas\nonde: n.velocidade > 1')}\n`);
      await abrirEditor(page, NOTA_ERRO, 'n.velocidade > 1');

      const painel = page.locator('#query-panel');
      await expect(painel).toContainText('campo desconhecido', { timeout: 20000 });
      await expect(painel).toContainText('campos de n:');

      expect(status.length, 'o painel deveria ter buscado o fragmento').toBeGreaterThan(0);
      expect(status.every(s => s === 200), `status: ${status.join(',')}`).toBe(true);

      // A nota segue intacta no editor.
      await expect(page.locator('.ProseMirror')).toContainText('n.velocidade > 1');
    } finally {
      await apagarNotas(page, [NOTA_ERRO]);
    }
  });

  test('nota sem bloco não mostra painel', async ({ page }) => {
    await login(page);

    try {
      await salvarNota(page, NOTA_SEM_BLOCO, '# Simples\n\nsem consulta aqui\n');
      await abrirEditor(page, NOTA_SEM_BLOCO, 'sem consulta aqui');

      // O container existe (é o ponto de escuta do evento), mas vem vazio.
      await expect(page.locator('#query-panel')).toBeEmpty({ timeout: 20000 });
      await expect(page.getByText('Consultas')).toHaveCount(0);
    } finally {
      await apagarNotas(page, [NOTA_SEM_BLOCO]);
    }
  });
});
