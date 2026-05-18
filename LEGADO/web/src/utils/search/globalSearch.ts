/**
 * GLOBAL SEARCH ENGINE (DEEP SEARCH) - TON25
 */

import type { SearchResult } from '../../types';
import { QUOTE_REGEX, STOPWORDS, SYS_FILTER_REGEX, TAG_REGEX } from './common';

/**
 * Processa um termo individual para a busca global.
 */
function processGlobalTerm(word: string): string {
  return word.toLowerCase();
}

/**
 * Formata a query string para o ZincSearch no modo Global.
 */
function formatGlobalQuery(query: string = ''): string {
  if (!query || query.trim() === '' || query === '*') {
    return '*';
  }

  const resultTerms: string[] = [];
  let remaining = query;

  // 1. Hashtags (#tag -> tags:tag)
  let match: RegExpExecArray | null;
  while ((match = TAG_REGEX.exec(query)) !== null) {
    const tag = match[1].toLowerCase();
    resultTerms.push(`tags:${tag}`);
    remaining = remaining.replace(match[0], ' ');
  }

  // 2. Filtros de Sistema (campo:valor)
  const filters = remaining.match(SYS_FILTER_REGEX);
  if (filters) {
    filters.forEach((f) => {
      if (f.startsWith('tags:') || f.startsWith('+tags:')) return;
      const cleanFilter = f.startsWith('+') ? f.substring(1) : f;
      resultTerms.push(cleanFilter);
      remaining = remaining.replace(f, ' ');
    });
  }

  // 3. Frases exatas ("termo termo")
  while ((match = QUOTE_REGEX.exec(remaining)) !== null) {
    const phrase = match[1].toLowerCase().trim();
    resultTerms.push(`+"${phrase}"`);
    remaining = remaining.replace(match[0], ' ');
  }

  // 4. Palavras normais com remoção de stopwords
  const words = remaining
    .trim()
    .split(/\s+/)
    .filter((w) => w.length > 0);
  words.forEach((word) => {
    const cleanWord = word.toLowerCase();
    // Pula stopwords se houver outros termos na busca
    if (STOPWORDS.includes(cleanWord) && (words.length > 1 || resultTerms.length > 0)) {
      return;
    }
    resultTerms.push(processGlobalTerm(word));
  });

  return resultTerms.length > 0 ? resultTerms.join(' ') : '*';
}

export interface SearchPayload {
  search_type: string;
  query: {
    term: string;
    fields: string[];
    default_operator: string;
  };
  compact: boolean;
  semantic: boolean;
  from: number;
  size: number;
  max_results: number;
  highlight?: {
    fields: Record<string, { pre_tags: string[]; post_tags: string[] }>;
  };
  sort?: string[];
}

/**
 * Constrói o payload de busca para o modo Global.
 */
export function buildGlobalPayload(
  query: string,
  offset: number = 0,
  semantic: boolean = false,
): SearchPayload {
  const finalTerm = formatGlobalQuery(query);

  const payload: SearchPayload = {
    search_type: 'querystring',
    query: {
      term: finalTerm,
      // Inclui 'texto' no escopo de busca
      fields: ['tags^50', 'arquivo^20', 'secao^10', 'texto^1'],
      default_operator: 'AND',
    },
    compact: false,
    semantic: semantic,
    from: offset,
    size: 50,
    max_results: 50,
  };

  if (query && query.trim() !== '' && query !== '*') {
    payload.highlight = {
      fields: {
        secao: { pre_tags: ['<mark class="zinc-hl">'], post_tags: ['</mark>'] },
        texto: { pre_tags: ['<mark class="zinc-hl">'], post_tags: ['</mark>'] },
      },
    };
  }

  // Se não houver query, ordena por data. Se houver, usa o score de relevância.
  if (!query || query === '*' || query.trim() === '') {
    payload.sort = ['-@timestamp'];
  }

  return payload;
}

/**
 * Processa os resultados após a busca Global.
 */
export function processGlobalResults(hits: any[]): SearchResult[] {
  if (!hits) return [];
  return hits.map((hit) => ({
    ...hit._source,
    highlight: hit.highlight,
    final_score: hit.final_score,
    score_details: hit.score_details,
    id: hit.id || hit._id, // Garantindo ID
  }));
}
