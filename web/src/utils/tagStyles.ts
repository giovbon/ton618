/**
 * Lógica para determinação de estilos de tags.
 * Garante consistência entre frontend e backend.
 */

interface TagStyle {
  mainCol: string;
  bgCol: string;
  borderCol: string;
  hue: number;
  pattern: number;
}

export const getTagStyle = (tag: string): TagStyle => {
  // Hash FNV-1a simples para determinismo
  let hash = 2166136261;
  for (let i = 0; i < tag.length; i++) {
    hash ^= tag.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  const sum = Math.abs(hash);

  const hue = sum % 360;
  // Refined Dark Style: Fundo neutro e texto vibrante
  const mainCol = `hsl(${hue}, 85%, 75%)`; // Cor do texto e ícone (indicador funcional)
  const bgCol = `#18181b`; // Cinza grafite neutro (Zinc-900)
  const borderCol = `rgba(63, 63, 70, 0.4)`; // Borda sutil (Zinc-700)

  return { mainCol, bgCol, borderCol, hue, pattern: sum % 64 };
};
