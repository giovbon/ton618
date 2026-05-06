import { expect, test } from '@playwright/test';
import { authUrl, createNoteViaApi } from './helpers';

test.describe('Deleção de Nota', () => {
  let _noteFilename: string;

  test.beforeAll(async () => {
    _noteFilename = await createNoteViaApi('Delete Test', 'Nota para ser deletada no teste.');
    await new Promise((r) => setTimeout(r, 2000));
  });

  test('deve deletar uma nota e ela desaparecer da busca', async ({ page }) => {
    await page.goto(authUrl('/'));

    // Buscar pela nota
    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill('Delete Test');
    await searchInput.press('Enter');
    await page.waitForTimeout(1000);

    // Clicar no resultado
    const noteLink = page.locator('text=Delete Test').first();
    await expect(noteLink).toBeVisible({ timeout: 3000 });
    await noteLink.click();
    await page.waitForTimeout(1000);

    // Clicar em deletar/excluir
    const deleteBtn = page.locator('button:has-text("Excluir"), button:has-text("Delete")');
    await expect(deleteBtn).toBeVisible();
    await deleteBtn.click();

    // Confirmar diálogo
    const confirmBtn = page.locator(
      'button:has-text("Confirmar"), button:has-text("Sim"), button:has-text("Yes")',
    );
    if (await confirmBtn.isVisible()) {
      await confirmBtn.click();
    }

    // Verificar mensagem de sucesso
    const successMsg = page
      .locator('text=excluído', { ignoreCase: true })
      .or(page.locator('text=deletado', { ignoreCase: true }));
    await expect(successMsg).toBeVisible({ timeout: 5000 });

    // Verificar que não aparece mais na busca
    await searchInput.fill('Delete Test');
    await searchInput.press('Enter');
    await page.waitForTimeout(2000);
    await expect(page.locator('text=Delete Test')).not.toBeVisible();
  });
});
