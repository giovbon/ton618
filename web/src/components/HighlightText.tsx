import { applyHtmlHighlight } from '../utils/search';

interface HighlightTextProps {
  text?: string;
  query?: string;
  searchTerms?: string[];
}

/**
 * Componente para Highlight de texto simples.
 * Escapa o texto para evitar XSS antes de aplicar o realce.
 */
export const HighlightText = ({ text, query, searchTerms }: HighlightTextProps) => {
  if (!text) return null;
  if (!query && !searchTerms) return <>{text}</>;

  // Usamos termos já processados se disponíveis, senão processamos aqui (fallback)
  const terms = searchTerms || (query || '').trim().replace(/"/g, '').split(/\s+/);

  const escapedText = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');

  const highlighted = applyHtmlHighlight(escapedText, query || '', terms);
  return <span dangerouslySetInnerHTML={{ __html: highlighted }} />;
};
