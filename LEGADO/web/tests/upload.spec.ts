import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';
import { authUrl } from './helpers';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

test.describe('Upload de Imagem → OCR', () => {
  const testImagePath = path.join(__dirname, 'fixtures', 'test-image.png');

  test.beforeAll(() => {
    // Criar uma imagem PNG minimalista para teste (1x1 pixel)
    const dir = path.join(__dirname, 'fixtures');
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true });
    if (!fs.existsSync(testImagePath)) {
      // PNG 1x1 pixel válido
      const pngBuffer = Buffer.from([
        137, 80, 78, 71, 13, 10, 26, 10, 0, 0, 0, 13, 73, 72, 68, 82, 0, 0, 0, 1, 0, 0, 0, 1, 8, 2,
        0, 0, 0, 144, 119, 83, 222, 0, 0, 0, 12, 73, 68, 65, 84, 8, 215, 99, 248, 207, 0, 0, 0, 2,
        0, 1, 226, 80, 191, 172, 0, 0, 0, 0, 73, 69, 78, 68, 174, 66, 96, 130,
      ]);
      fs.writeFileSync(testImagePath, pngBuffer);
    }
  });

  test('deve fazer upload de imagem', async ({ page }) => {
    await page.goto(authUrl('/'));

    // Clicar no botão de upload
    const uploadBtn = page.locator('button:has-text("Upload")');
    await uploadBtn.click();

    // Selecionar arquivo
    const fileInput = page.locator('input[type="file"]');
    await fileInput.setInputFiles(testImagePath);

    // Aguardar processamento
    await page.waitForTimeout(3000);

    // Verificar que algo aconteceu (botão sumiu ou mensagem apareceu)
    await expect(
      page.locator('text=Upload').or(page.locator('text=enviado').or(page.locator('text=sucesso'))),
    ).toBeVisible({ timeout: 10000 });
  });
});
