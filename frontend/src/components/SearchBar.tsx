import { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import type { SearchSuggestion } from '../lib/api';
import { SearchBarPresentation } from './SearchBarPresentation';

interface SearchBarProps {
  variant?: 'hero' | 'default';
  autoFocus?: boolean;
}

export function SearchBar({ variant = 'default', autoFocus }: SearchBarProps) {
  const [search, setSearch] = useState('');
  const [suggestions, setSuggestions] = useState<SearchSuggestion[]>([]);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const navigate = useNavigate();
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Global keyboard shortcut to focus search
  useEffect(() => {
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }
      if (e.key === '/') {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };

    document.addEventListener('keydown', handleGlobalKeyDown);
    return () => document.removeEventListener('keydown', handleGlobalKeyDown);
  }, []);

  // Debounced search suggestions
  useEffect(() => {
    if (!search.trim() || search.length < 1) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- clearing suggestions when search is empty
      setSuggestions([]);
      return;
    }

    const timer = setTimeout(async () => {
      try {
        const response = await api.searchSuggestions(search.trim());
        setSuggestions(response.suggestions || []);
        setShowSuggestions(true);
        setSelectedIndex(-1);
      } catch (err) {
        console.error('Search suggestions error:', err);
        setSuggestions([]);
      }
    }, 150);

    return () => clearTimeout(timer);
  }, [search]);

  // Close suggestions when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setShowSuggestions(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const navigateToSuggestion = (suggestion: SearchSuggestion) => {
    switch (suggestion.type) {
      case 'block':
        navigate(`/block/${suggestion.value}`);
        break;
      case 'transaction':
        navigate(`/tx/${suggestion.value}`);
        break;
      case 'address':
        navigate(`/address/${suggestion.value}`);
        break;
    }
    setSearch('');
    setSuggestions([]);
    setShowSuggestions(false);
  };

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!search.trim()) return;

    if (selectedIndex >= 0 && suggestions[selectedIndex]) {
      navigateToSuggestion(suggestions[selectedIndex]);
      return;
    }

    const q = search.trim();
    if (/^\d+$/.test(q)) {
      navigate(`/block/${q}`);
    } else if (q.startsWith('0x') && q.length === 66) {
      navigate(`/tx/${q}`);
    } else if (q.startsWith('0x') && q.length === 42) {
      navigate(`/address/${q}`);
    }
    setSearch('');
    setSuggestions([]);
    setShowSuggestions(false);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!showSuggestions || suggestions.length === 0) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex((prev) =>
          prev < suggestions.length - 1 ? prev + 1 : prev
        );
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex((prev) => (prev > 0 ? prev - 1 : -1));
        break;
      case 'Escape':
        setShowSuggestions(false);
        setSelectedIndex(-1);
        break;
    }
  };

  return <SearchBarPresentation variant={variant} autoFocus={autoFocus} search={search} setSearch={setSearch}
    suggestions={suggestions} showSuggestions={showSuggestions} setShowSuggestions={setShowSuggestions}
    selectedIndex={selectedIndex} containerRef={containerRef} inputRef={inputRef} handleSearch={handleSearch}
    handleKeyDown={handleKeyDown} navigateToSuggestion={navigateToSuggestion} />;
}
