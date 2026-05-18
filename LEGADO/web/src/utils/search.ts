import type { SearchResult } from '../types';
import { isStopword } from './search/common';
import { processCompactResults } from './search/compactSearch';
import { processGlobalResults } from './search/globalSearch';

/**
 * Utilitário legado para compatibilidade.
 * Delega o processamento de resultados para o motor correto (Global ou Compacto).
 */
export const processSearchResults = (hits: any[], isCompact: boolean): SearchResult[] => {
  return isCompact ? processCompactResults(hits) : processGlobalResults(hits);
};

export { isStopword };

/**
 * Formata o nome do arquivo para exibição no título.
 */
export const formatDisplayTitle = (filename?: string): string => {
  if (!filename) return '';
  // Remove diretório e extensão
  return filename.replace(/^(notes|links|attachments|pdfs)\//i, '').replace(/\.(md|pdf)$/i, '');
};

interface FragmentResult {
  fragment: string;
  hasMoreBefore: boolean;
  hasMoreAfter: boolean;
  isTruncated: boolean; // para compatibilidade legada
}

/**
 * Extrai um fragmento de texto ao ao redor do termo pesquisado.
 */
export const extractFragment = (
  text: string | null | undefined,
  query: string,
  terms: string[],
): FragmentResult => {
  const baseText = text || '';
  const fullQuery = (query || '').trim().replace(/"/g, '');

  // Aumentado para ~820 caracteres para garantir ~5 linhas de contexto
  if (baseText.length <= 820) {
    return {
      fragment: baseText,
      hasMoreBefore: false,
      hasMoreAfter: false,
      isTruncated: false,
    };
  }

  if (fullQuery.length < 2) {
    return {
      fragment: baseText.substring(0, 820),
      hasMoreBefore: false,
      hasMoreAfter: true,
      isTruncated: true,
    };
  }

  let idx = baseText.toLowerCase().indexOf(fullQuery.toLowerCase());

  if (idx === -1) {
    const firstStrong = terms.find((t) => !isStopword(t));
    if (firstStrong) {
      idx = baseText.toLowerCase().indexOf(firstStrong.toLowerCase());
    }
  }

  if (idx !== -1) {
    // 220 caracteres antes, 600 depois para dar ~5-6 linhas
    let start = Math.max(0, idx - 220);
    if (start > 0) {
      // Tenta alinhar com o início de uma linha para não quebrar Markdown
      const lastNewline = baseText.lastIndexOf('\n', start);
      if (lastNewline !== -1 && start - lastNewline < 120) {
        start = lastNewline + 1;
      }
    }

    let end = Math.min(baseText.length, idx + 600);
    if (end < baseText.length) {
      // Tenta alinhar com o fim de uma linha
      const nextNewline = baseText.indexOf('\n', end);
      if (nextNewline !== -1 && nextNewline - end < 120) {
        end = nextNewline;
      }
    }

    return {
      fragment: baseText.substring(start, end),
      hasMoreBefore: start > 0,
      hasMoreAfter: end < baseText.length,
      isTruncated: start > 0 || end < baseText.length,
    };
  }

  return {
    fragment: baseText.substring(0, 820),
    hasMoreBefore: false,
    hasMoreAfter: true,
    isTruncated: true,
  };
};

/**
 * Aplica realce de busca (highlight) em uma string de HTML.
 */
export const applyHtmlHighlight = (html: string, query: string, terms: string[]): string => {
  const fullQuery = (query || '').trim().replace(/"/g, '');
  if (fullQuery.length < 2) return html;

  interface Candidate {
    text: string;
    type: 'phrase' | 'term';
  }

  const candidates: Candidate[] = [];
  if (fullQuery.length >= 2) {
    candidates.push({ text: fullQuery, type: fullQuery.includes(' ') ? 'phrase' : 'term' });
  }

  const strongTerms = terms.filter((t) => !isStopword(t));
  strongTerms.forEach((t) => {
    if (t.toLowerCase() !== fullQuery.toLowerCase()) {
      candidates.push({ text: t, type: 'term' });
    }
  });

  if (candidates.length === 0) return html;

  // ORDENAÇÃO: Termos mais longos primeiro para garantir precedência no match
  candidates.sort((a, b) => b.text.length - a.text.length);

  const safeCands = candidates.map((c) => c.text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  const bStart = '(?<=[^a-zA-Z0-9À-ÿ]|^)';
  // Regex que busca os termos fora de tags HTML, respeitando limites de início mas permitindo sufixos
  const regex = new RegExp(`${bStart}(${safeCands.join('|')})(?![^<]*>)`, 'gi');

  return html.replace(regex, (matchedTerm) => {
    const matchedLower = matchedTerm.toLowerCase();
    const cand =
      candidates.find((c) => c.text.toLowerCase() === matchedLower) ||
      candidates.find((c) => matchedLower.startsWith(c.text.toLowerCase()));

    const className =
      cand?.type === 'phrase'
        ? 'bg-sky-500/30 text-sky-100 font-bold px-0.5 rounded shadow-sm'
        : 'bg-amber-500/40 text-amber-100 font-medium px-0.5 rounded shadow-sm';

    return `<mark class="${className}">${matchedTerm}</mark>`;
  });
};
