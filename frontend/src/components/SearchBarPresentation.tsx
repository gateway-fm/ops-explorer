import type { FormEventHandler, KeyboardEventHandler, RefObject } from 'react';
import { Search } from 'lucide-react';
import { branding } from '../lib/branding';

export interface SearchSuggestionPresentation { type: string; value: string; label: string }
interface SearchBarPresentationProps<T extends SearchSuggestionPresentation> {
  variant?: 'hero' | 'default'; autoFocus?: boolean;
  search: string; setSearch: (value: string) => void;
  suggestions: T[]; showSuggestions: boolean; setShowSuggestions: (show: boolean) => void;
  selectedIndex: number; containerRef: RefObject<HTMLDivElement | null>; inputRef: RefObject<HTMLInputElement | null>;
  handleSearch: FormEventHandler<HTMLFormElement>; handleKeyDown: KeyboardEventHandler<HTMLInputElement>;
  navigateToSuggestion: (suggestion: T) => void;
}

// Established search markup, shared without its legacy API dependency.
export function SearchBarPresentation<T extends SearchSuggestionPresentation>({ variant = 'default', autoFocus,
  search, setSearch, suggestions, showSuggestions, setShowSuggestions, selectedIndex,
  containerRef, inputRef, handleSearch, handleKeyDown, navigateToSuggestion,
}: SearchBarPresentationProps<T>) {
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
            data-testid="search-input"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            onFocus={() => suggestions.length > 0 && setShowSuggestions(true)}
            onKeyDown={handleKeyDown}
            autoFocus={autoFocus}
            placeholder={`Search ${branding.name} by address, tx hash, or block number...`}
            className={
              isHero
                ? 'w-full h-[50px] pl-12 pr-14 bg-neutral-50 border border-neutral-200 rounded-xl text-base text-neutral-900 placeholder:text-neutral-400 focus:outline-none focus:border-primary focus:ring-2 focus:ring-primary/20 transition-all duration-200 shadow-card'
                : 'input pl-11 pr-12'
            }
          />
          {!search && (
            <kbd className={`absolute right-3 top-1/2 -translate-y-1/2 px-1.5 py-0.5 text-xs font-mono rounded ${isHero ? 'text-neutral-400 bg-neutral-100 border border-neutral-200' : 'text-neutral-400 bg-neutral-100 border border-neutral-200'}`}>
              /
            </kbd>
          )}
        </div>
      </form>

      {/* Suggestions dropdown */}
      {showSuggestions && suggestions.length > 0 && (
        <div className="absolute top-full left-0 right-0 mt-2 bg-neutral-50 rounded-xl border border-neutral-200 overflow-hidden z-50 shadow-elevated max-h-80 overflow-y-auto">
          {suggestions.map((suggestion, index) => (
            <button
              key={`${suggestion.type}-${suggestion.value}`}
              data-testid="search-suggestion"
              data-suggestion-type={suggestion.type}
              aria-selected={index === selectedIndex}
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
