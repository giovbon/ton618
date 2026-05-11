/**
 * Quebra um texto em multiplas linhas para caber dentro de uma largura maxima,
 * usando um contexto de canvas 2D para medir o texto com precisao.
 * Preserva delimitadores (espacos, hifens, underscores) junto com a palavra anterior.
 */
export function wrapText(
  ctx: CanvasRenderingContext2D,
  text: string,
  maxWidth: number,
): string[] {
  const words = text.split(/([ \-_])/);
  const lines: string[] = [];
  let currentLine = "";

  for (const part of words) {
    const testLine = currentLine + part;
    if (ctx.measureText(testLine).width > maxWidth && currentLine) {
      lines.push(currentLine);
      currentLine = part;
    } else {
      currentLine = testLine;
    }
  }
  lines.push(currentLine);
  return lines;
}
