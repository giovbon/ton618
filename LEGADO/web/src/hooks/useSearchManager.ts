import { useCallback, useEffect, useMemo, useState } from 'preact/hooks';
import type { SearchResult } from '../types';
import { sortSearchResults } from '../utils/sorting';
import { useSearchQuery } from './useSearchQuery';

interface SearchManagerOptions {
  auth: string;
  handleLogout: () => void;
  lastEditedFileName: string | null;
  deletedFilenames: Set<string>;
}

export function useSearchManager({
  auth,
  handleLogout,
  lastEditedFileName,
  deletedFilenames,
}: SearchManagerOptions) {
  // Query state
  const [query, setQuery] = useState<string>(() => sessionStorage.getItem('ton_last_query') || '');
  const [debouncedQuery, setDebouncedQuery] = useState<string>(query);
  const [isCompactMode, setIsCompactMode] = useState<boolean>(
    () => localStorage.getItem('ton_compact_mode') === 'true',
  );
  const [isSemanticEnabled, setIsSemanticEnabled] = useState<boolean>(false);

  // Fetch results
  const { data, fetchNextPage, hasNextPage, isFetchingNextPage, isLoading, error } = useSearchQuery(
    debouncedQuery,
    isCompactMode,
    auth,
    handleLogout,
    isSemanticEnabled,
  );

  // Helper for normalization
  const normalize = useCallback(
    (f: string | null | undefined) => (f || '').replace(/\/+/g, '/').trim(),
    [],
  );

  // Process and Filter Results
  const results = useMemo(() => {
    const hits: SearchResult[] = data?.pages.flatMap((page) => page.hits) || [];
    const lastEditedNormalized = normalize(lastEditedFileName);

    const filteredHits =
      deletedFilenames.size === 0
        ? hits
        : hits.filter((h) => !deletedFilenames.has(normalize(h.arquivo || h._source?.arquivo)));

    return sortSearchResults(filteredHits, isSemanticEnabled, lastEditedNormalized, isCompactMode);
  }, [data, deletedFilenames, isCompactMode, lastEditedFileName, isSemanticEnabled, normalize]);

  const totalHits = data?.pages[0]?.total || 0;
  const searchTerms = useMemo(
    () => (debouncedQuery || '').trim().replace(/"/g, '').split(/\s+/),
    [debouncedQuery],
  );

  const isDataviewQuery = useMemo(() => {
    const q = debouncedQuery.trim().toUpperCase();
    return q.startsWith('TABLE ') || q.startsWith('LIST ') || q.startsWith('FROM ');
  }, [debouncedQuery]);

  // Actions
  const handleExecuteSearch = useCallback(() => {
    setDebouncedQuery(query);
    sessionStorage.setItem('ton_last_query', query);
  }, [query]);

  // Sync debounced query (standard behavior)
  useEffect(() => {
    if (isSemanticEnabled) return;
    const timer = setTimeout(() => {
      setDebouncedQuery(query);
      sessionStorage.setItem('ton_last_query', query);
    }, 300);
    return () => clearTimeout(timer);
  }, [query, isSemanticEnabled]);

  // Persist compact mode
  useEffect(() => {
    localStorage.setItem('ton_compact_mode', String(isCompactMode));
  }, [isCompactMode]);

  return useMemo(
    () => ({
      state: {
        query,
        debouncedQuery,
        isCompactMode,
        isSemanticEnabled,
        results,
        totalHits,
        searchTerms,
        isDataviewQuery,
        isLoading,
        hasNextPage,
        isFetchingNextPage,
        error,
      },
      actions: {
        setQuery,
        setIsCompactMode,
        setIsSemanticEnabled,
        handleExecuteSearch,
        fetchNextPage,
      },
    }),
    [
      query,
      debouncedQuery,
      isCompactMode,
      isSemanticEnabled,
      results,
      totalHits,
      searchTerms,
      isDataviewQuery,
      isLoading,
      hasNextPage,
      isFetchingNextPage,
      error,
      handleExecuteSearch,
      fetchNextPage,
    ],
  );
}
