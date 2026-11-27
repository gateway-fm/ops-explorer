import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import type { SearchSuggestion } from '../lib/api';
import { directSuggestion } from '../lib/searchDirect';

// useSearchSuggestions drives the suggestion dropdown of every search box (the
// Home SearchBar and the Layout search).
//
// - A complete address, tx hash or block number is offered straight away,
//   without waiting for the indexer lookup, and stays offered if it fails.
// - The lookup is debounced, and a lookup that settles after the query changed
//   is ignored. Without that, a slow response for an earlier query overwrites
//   the newer suggestions (or restores the earlier query's address).
export function useSearchSuggestions(search: string) {
  const [suggestions, setSuggestions] = useState<SearchSuggestion[]>([]);
  const [open, setOpen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);

  useEffect(() => {
    const q = search.trim();
    if (!q) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- clearing suggestions when search is empty
      setSuggestions([]);
      setOpen(false);
      setSelectedIndex(-1);
      return;
    }

    const direct = directSuggestion(q);
    setSuggestions(direct ? [direct] : []);
    setOpen(!!direct);
    setSelectedIndex(-1);

    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const response = await api.searchSuggestions(q);
        if (cancelled) return;
        const found = response.suggestions || [];
        const merged =
          direct && !found.some((s) => s.type === direct.type && s.value.toLowerCase() === direct.value.toLowerCase())
            ? [direct, ...found]
            : found;
        setSuggestions(merged);
        setOpen(merged.length > 0);
        setSelectedIndex(-1);
      } catch (err) {
        if (cancelled) return;
        // The direct suggestion (if any) set above is still the right answer.
        console.error('Search suggestions error:', err);
      }
    }, 150);

    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [search]);

  const clear = () => {
    setSuggestions([]);
    setOpen(false);
    setSelectedIndex(-1);
  };

  return { suggestions, open, setOpen, selectedIndex, setSelectedIndex, clear };
}
