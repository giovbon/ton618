import { expect, test } from '@playwright/test';
import { authUrl, createNoteViaApi, deleteNoteViaApi } from './helpers';

test.describe('Busca → Abrir → Editar → Salvar', () => {
  let noteFilename: string;

  test.beforeAll(async () => {
    noteFilename = await createNoteViaApi('Test Note E2E', 'Conteúdo inicial para teste E2E.');
    // Aguardar indexação
    await new Promise((r) => setTimeout(r, 2000));
  });

  test.afterAll(async () => {
    if (noteFilename) await deleteNoteViaApi(noteFilename);
  });

  test('deve buscar, abrir nota, editar conteúdo e salvar com sucesso', async ({ page }) => {
    // 1. Acessar página principal
    await page.goto(authUrl('/'));

    // 2. Buscar pela nota
    const searchInput = page.locator('input[type="text"]').first();
    await searchInput.fill('Test Note E2E');
    await searchInput.press('Enter');

    // 3. Aguardar resultados e clicar na nota
    await page.waitForTimeout(2000);
    const noteLink = page.locator('text=Test Note E2E').first();
    await expect(noteLink).toBeVisible({ timeout: 5000 });
    await noteLink.click();

    // 4. Verificar que o editor abriu com o conteúdo
    await page.waitForTimeout(1000);
    await expect(page.locator('text=Conteúdo inicial para teste E2E')).toBeVisible({
      timeout: 3000,
    });

    // 5. Clicar em editar (se houver modo toggle) ou editar diretamente
    const editButton = page.locator('button:has-text("Editar"), button:has-text("Edit")');
    if (await editButton.isVisible()) {
      await editButton.click();
    }

    // 6. Adicionar conteúdo editado
    const editorArea = page.locator('.cm-content, [contenteditable="true"], textarea').first();
    await editorArea.click();
    await editorArea.fill('');
    await editorArea.type('Conteúdo editado via E2E!');

    // 7. Salvar
    const saveButton = page.locator('button:has-text("Salvar"), button:has-text("Save")');
    await expect(saveButton).toBeVisible();
    await saveButton.click();

    // 8. Verificar mensagem de sucesso
    await expect(
      page
        .locator('text=salvo', { ignoreCase: true })
        .or(page.locator('text=saved', { ignoreCase: true })),
    ).toBeVisible({ timeout: 5000 });
  });
});
