/**
 * COMPACT SEARCH ENGINE - TON25
 */

import type { SearchResult } from '../../types';
import { QUOTE_REGEX, STOPWORDS, SYS_FILTER_REGEX, TAG_REGEX } from './common';
import type { SearchPayload } from './globalSearch';

/**
 * Processa um termo individual para a busca compacta.
 */
function processCompactTerm(word: string): string {
  const clean = word.toLowerCase();

  // No modo compacto, hífens são tratados como espaços para facilitar busca em slugs
  return clean.replace(/-/g, ' ');
}

/**
 * Formata a query string para o ZincSearch no modo Compacto.
 */
function formatCompactQuery(query: string = ''): string {
  if (!query || query.trim() === '' || query === '*') {
    return '*';
  }

  const resultTerms: string[] = [];
  let remaining = query;

  // 1. Hashtags
  let match: RegExpExecArray | null;
  while ((match = TAG_REGEX.exec(query)) !== null) {
    const tag = match[1].toLowerCase();
    resultTerms.push(`tags:${tag}`);
    remaining = remaining.replace(match[0], ' ');
  }

  // 2. Filtros de Sistema
  const filters = remaining.match(SYS_FILTER_REGEX);
  if (filters) {
    filters.forEach((f) => {
      if (f.startsWith('tags:') || f.startsWith('+tags:')) return;
      const cleanFilter = f.startsWith('+') ? f.substring(1) : f;
      resultTerms.push(cleanFilter);
      remaining = remaining.replace(f, ' ');
    });
  }

  // 3. Frases exatas
  while ((match = QUOTE_REGEX.exec(remaining)) !== null) {
    const phrase = match[1].toLowerCase().trim();
    resultTerms.push(`+"${phrase}"`);
    remaining = remaining.replace(match[0], ' ');
  }

  // 4. Palavras normais
  const words = remaining
    .trim()
    .split(/\s+/)
    .filter((w) => w.length > 0);
  words.forEach((word) => {
    const cleanWord = word.toLowerCase();
    if (STOPWORDS.includes(cleanWord) && (words.length > 1 || resultTerms.length > 0)) {
      return;
    }
    resultTerms.push(processCompactTerm(word));
  });

  return resultTerms.length > 0 ? resultTerms.join(' ') : '*';
}

/**
 * Constrói o payload de busca para o modo Compacto.
 */
export function buildCompactPayload(
  query: string,
  offset: number = 0,
  semantic: boolean = false,
): SearchPayload {
  const finalTerm = formatCompactQuery(query);

  return {
    search_type: 'querystring',
    query: {
      term: finalTerm,
      // Pesquisa restrita EXCLUSIVAMENTE pelo nome do arquivo no modo compacto
      fields: ['arquivo^1'],
      default_operator: 'AND',
    },
    compact: true,
    semantic: semantic,
    from: offset,
    size: 50,
    max_results: 50,
    highlight: {
      fields: {
        secao: { pre_tags: ['<mark class="zinc-hl">'], post_tags: ['</mark>'] },
      },
    },
    sort: ['-@timestamp'],
  };
}

/**
 * Processa e deduplica os resultados do modo compacto por arquivo.
 */
export function processCompactResults(hits: any[]): SearchResult[] {
  if (!hits) return [];

  const results: SearchResult[] = hits.map((hit) => ({
    ...hit._source,
    highlight: hit.highlight,
    final_score: hit.final_score,
    score_details: hit.score_details,
    id: hit.id || hit._id,
  }));

  const seen = new Set<string>();
  return results.filter((r) => {
    const file = r.arquivo;
    if (!file || seen.has(file)) return false;
    seen.add(file);
    return true;
  });
}
