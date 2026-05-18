import { expect, test } from '@playwright/test';
import { authUrl, createNoteViaApi, deleteNoteViaApi } from './helpers';

const credentials = btoa('admin:ton618_secret');

test.describe('Knowledge Map', () => {
  const notesToClean: string[] = [];

  test.beforeAll(async () => {
    // Pular se semântica não estiver habilitada
    try {
      const resp = await fetch('http://localhost:6180/api/health', {
        headers: { Authorization: `Basic ${credentials}` },
      });
      const data = await resp.json();
      if (data?.checks?.ollama !== 'up') {
        console.log('⏩ Ollama não disponível, pulando teste do Knowledge Map');
        return;
      }
    } catch {
      console.log('⏩ Backend não disponível, pulando teste do Knowledge Map');
      return;
    }

    // Criar notas com #embed para povoar o mapa
    const note1 = await createNoteViaApi(
      'Map Note 1',
      '---\ntags: [embed]\n---\n\nConteúdo para cluster A.',
    );
    const note2 = await createNoteViaApi(
      'Map Note 2',
      '---\ntags: [embed]\n---\n\nConteúdo para cluster B.',
    );
    notesToClean.push(note1, note2);
    await new Promise((r) => setTimeout(r, 3000));
  });

  test.afterAll(async () => {
    for (const f of notesToClean) {
      try {
        await deleteNoteViaApi(f);
      } catch {
        /* ignore */
      }
    }
  });

  test('deve exibir o knowledge map com clusters', async ({ page }) => {
    // Pular se semântica não disponível
    const healthResp = await fetch('http://localhost:6180/api/health', {
      headers: { Authorization: `Basic ${credentials}` },
    });
    const healthData = await healthResp.json();
    if (healthData?.checks?.ollama !== 'up') {
      test.skip();
      return;
    }

    await page.goto(authUrl('/'));

    // Navegar para o Knowledge Map
    const mapLink = page.locator(
      'a:has-text("Mapa"), a:has-text("Knowledge"), button:has-text("Mapa")',
    );
    await mapLink.click();
    await page.waitForTimeout(3000);

    // Verificar que clusters SVG/canvas aparecem
    const svg = page.locator('svg, canvas, .knowledge-map');
    await expect(svg).toBeVisible({ timeout: 10000 });

    // Verificar que há labels nos clusters
    const labels = page.locator('text=Cluster').or(page.locator('text=Map Note'));
    await expect(labels).toBeVisible({ timeout: 5000 });
  });
});
