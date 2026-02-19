import { useState, useEffect, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Search } from 'lucide-react';
import { api } from '../lib/api';
import type { SearchSuggestion } from '../lib/api';

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

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'block': return '#';
      case 'transaction': return 'Tx';
      case 'address': return '@';
      default: return '?';
    }
  };

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'block':
        return 'bg-blue-50 text-blue-600 border border-blue-200';
      case 'transaction':
        return 'bg-green-50 text-green-600 border border-green-200';
      case 'address':
        return 'bg-primary-50 text-primary-600 border border-primary-200';
      default:
        return 'bg-neutral-100 text-neutral-600';
    }
  };

  const isHero = variant === 'hero';

  return (
    <div ref={containerRef} className="relative w-full">
      <form onSubmit={handleSearch}>
        <div className="relative">
          <Search className={`absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 ${isHero ? 'text-neutral-400' : 'text-neutral-400'}`} />
          <input
            ref={inputRef}
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onFocus={() => suggestions.length > 0 && setShowSuggestions(true)}
            onKeyDown={handleKeyDown}
            autoFocus={autoFocus}
            placeholder="Search by address, tx hash, or block number..."
            className={
              isHero
                ? 'w-full h-[50px] pl-12 pr-14 bg-white border border-neutral-200 rounded-xl text-base text-neutral-900 placeholder:text-neutral-400 focus:outline-none focus:border-primary focus:ring-2 focus:ring-primary/20 transition-all duration-200 shadow-card'
                : 'input pl-11 pr-12'
            }
          />
          <kbd className={`absolute right-3 top-1/2 -translate-y-1/2 px-1.5 py-0.5 text-xs font-mono rounded ${isHero ? 'text-neutral-400 bg-neutral-100 border border-neutral-200' : 'text-neutral-400 bg-neutral-100 border border-neutral-200'}`}>
            /
          </kbd>
        </div>
      </form>

      {/* Suggestions dropdown */}
      {showSuggestions && suggestions.length > 0 && (
        <div className="absolute top-full left-0 right-0 mt-2 bg-white rounded-xl border border-neutral-200 overflow-hidden z-50 shadow-elevated max-h-80 overflow-y-auto">
          {suggestions.map((suggestion, index) => (
            <button
              key={`${suggestion.type}-${suggestion.value}`}
              onClick={() => navigateToSuggestion(suggestion)}
              className={`w-full px-4 py-3 flex items-center gap-3 text-left transition-colors ${
                index === selectedIndex ? 'bg-primary-50' : 'hover:bg-neutral-50'
              }`}
            >
              <span className={`text-xs font-medium px-2 py-0.5 rounded ${getTypeColor(suggestion.type)}`}>
                {getTypeIcon(suggestion.type)}
              </span>
              <span className="text-sm text-neutral-700 font-mono truncate">
                {suggestion.label}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
