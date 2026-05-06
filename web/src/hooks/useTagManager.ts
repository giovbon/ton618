import { useCallback, useMemo, useState } from 'preact/hooks';
import type { TagAutocompleteState } from '../types';

export function useTagManager(
  fetchWithAuth: (url: string) => Promise<Response | null>,
  query: string,
  setQuery: (q: string) => void,
) {
  const [availableTags, setAvailableTags] = useState<string[]>([]);
  const [tagAutocomplete, setTagAutocomplete] = useState<TagAutocompleteState>({
    active: false,
    matches: [],
    selectedIndex: 0,
    queryText: '',
  });

  const refreshAvailableTags = useCallback(() => {
    fetchWithAuth('/api/tags')
      .then((res) => (res?.ok ? res.json() : null))
      .then((data) => data && setAvailableTags(data.tags || []))
      .catch(console.error);
  }, [fetchWithAuth]);

  const handleInput = useCallback(
    (val: string) => {
      const lastWordMatch = val.match(/(?:^|\s)(#[\w\-À-ÿ]*)$/);
      if (lastWordMatch) {
        const partialTag = lastWordMatch[1].substring(1).toLowerCase();
        const matches = availableTags.filter((t) => t.toLowerCase().startsWith(partialTag));
        setTagAutocomplete({ active: true, matches, selectedIndex: 0, queryText: partialTag });
      } else {
        setTagAutocomplete({ active: false, matches: [], selectedIndex: 0, queryText: '' });
      }
    },
    [availableTags],
  );

  const applyTag = useCallback(
    (tag: string) => {
      const lastWordRegex = /(?:^|\s)(#[\w\-À-ÿ]*)$/;
      const match = query.match(lastWordRegex);
      if (match) {
        const matchStart = (match.index || 0) + (match[0].startsWith(' ') ? 1 : 0);
        const prefix = query.substring(0, matchStart);
        const newQuery = `${prefix}#${tag} `;
        setQuery(newQuery);
        setTagAutocomplete({ active: false, matches: [], selectedIndex: 0, queryText: '' });
      }
    },
    [query, setQuery],
  );

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (tagAutocomplete.active && tagAutocomplete.matches.length > 0) {
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          setTagAutocomplete((prev) => ({
            ...prev,
            selectedIndex: (prev.selectedIndex + 1) % prev.matches.length,
          }));
          return true;
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          setTagAutocomplete((prev) => ({
            ...prev,
            selectedIndex: (prev.selectedIndex - 1 + prev.matches.length) % prev.matches.length,
          }));
          return true;
        } else if (e.key === 'Enter' || e.key === 'Tab') {
          e.preventDefault();
          applyTag(tagAutocomplete.matches[tagAutocomplete.selectedIndex]);
          return true;
        } else if (e.key === 'Escape') {
          setTagAutocomplete((prev) => ({ ...prev, active: false }));
          return true;
        }
      }
      return false;
    },
    [tagAutocomplete, applyTag],
  );

  const state = useMemo(
    () => ({
      availableTags,
      tagAutocomplete,
    }),
    [availableTags, tagAutocomplete],
  );

  const actions = useMemo(
    () => ({
      refreshAvailableTags,
      handleInput,
      applyTag,
      handleKeyDown,
      setTagAutocomplete,
    }),
    [refreshAvailableTags, handleInput, applyTag, handleKeyDown],
  );

  return { state, actions };
}
