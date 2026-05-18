import { expect, test } from '@playwright/test';
import { authUrl, createNoteViaApi, deleteNoteViaApi } from './helpers';

test.describe('Edição Direta via URL', () => {
  let noteFilename: string;

  test.beforeAll(async () => {
    noteFilename = await createNoteViaApi(
      'Direct Edit Test',
      'Conteúdo original para edição direta.',
    );
    await new Promise((r) => setTimeout(r, 2000));
  });

  test.afterAll(async () => {
    if (noteFilename) await deleteNoteViaApi(noteFilename);
  });

  test('deve abrir nota por URL direta e salvar edição', async ({ page }) => {
    // Abrir nota diretamente pela URL com o parâmetro file
    const url = authUrl(`/?file=${noteFilename}`);
    await page.goto(url);

    // Verificar que o conteúdo carregou
    await expect(page.locator('text=Conteúdo original para edição direta')).toBeVisible({
      timeout: 5000,
    });

    // Verificar que o editor está visível
    const editorArea = page.locator('.cm-content, [contenteditable="true"]').first();
    await expect(editorArea).toBeVisible({ timeout: 3000 });

    // Modificar conteúdo
    await editorArea.click();
    await editorArea.fill('');
    await editorArea.type('Conteúdo modificado via URL direta!');

    // Salvar
    const saveBtn = page.locator('button:has-text("Salvar"), button:has-text("Save")');
    await expect(saveBtn).toBeVisible();
    await saveBtn.click();

    // Verificar feedback de salvamento
    const feedback = page
      .locator('text=salvo', { ignoreCase: true })
      .or(page.locator('text=saved', { ignoreCase: true }));
    await expect(feedback).toBeVisible({ timeout: 5000 });

    // Recarregar a página e verificar que o conteúdo persiste
    await page.reload();
    await page.waitForTimeout(1000);
    await expect(page.locator('text=Conteúdo modificado via URL direta!')).toBeVisible({
      timeout: 5000,
    });
  });
});
