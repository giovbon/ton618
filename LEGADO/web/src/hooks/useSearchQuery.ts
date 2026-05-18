import { useInfiniteQuery } from '@tanstack/react-query';
import type { SearchResult } from '../types';
import { buildCompactPayload, processCompactResults } from '../utils/search/compactSearch';
import { buildGlobalPayload, processGlobalResults } from '../utils/search/globalSearch';

const PAGE_SIZE = 50;

interface SearchFetcherArgs {
  query: string;
  isCompact: boolean;
  authHeader: string | null;
  pageParam?: number;
  isSemantic?: boolean;
  onUnauthorized?: () => void;
}

interface SearchPageResponse {
  hits: SearchResult[];
  total: number;
  nextOffset: number | null;
}

/**
 * searchFetcher - Função isolada para facilitar testes e reuso.
 */
export const searchFetcher = async ({
  query,
  isCompact,
  authHeader,
  pageParam = 0,
  isSemantic = false,
  onUnauthorized,
}: SearchFetcherArgs): Promise<SearchPageResponse> => {
  if (!authHeader) return { hits: [], total: 0, nextOffset: null };

  const payload = isCompact
    ? buildCompactPayload(query, pageParam, isSemantic)
    : buildGlobalPayload(query, pageParam, isSemantic);

  const response = await fetch(`/api/search?t=${Date.now()}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: authHeader,
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    if (response.status === 401 && onUnauthorized) {
      onUnauthorized();
      throw new Error('Unauthorized');
    }
    throw new Error(`Erro na busca: ${response.status}`);
  }

  const data = await response.json();

  const processedResults = isCompact
    ? processCompactResults(data.hits.hits)
    : processGlobalResults(data.hits.hits);

  return {
    hits: processedResults as SearchResult[],
    total: isCompact ? 0 : data.hits.total.value,
    nextOffset: data.hits.hits.length >= PAGE_SIZE ? pageParam + PAGE_SIZE : null,
  };
};

/**
 * useSearchQuery - Refatoração do useSearch usando TanStack Query.
 */
export function useSearchQuery(
  query: string,
  isCompact: boolean,
  authHeader: string | null,
  onUnauthorized: () => void,
  isSemantic: boolean = false,
) {
  return useInfiniteQuery({
    queryKey: ['search', query, isCompact, isSemantic],
    queryFn: ({ pageParam }) =>
      searchFetcher({ query, isCompact, authHeader, pageParam, isSemantic, onUnauthorized }),
    getNextPageParam: (lastPage: SearchPageResponse) => lastPage.nextOffset,
    initialPageParam: 0,
    enabled: !!authHeader && (isCompact || query.trim() !== ''),
    staleTime: 1000 * 60 * 5,
  });
}
