import type { SearchSuggestion } from './api';

// directSuggestion recognises a query that is already a complete identifier.
// Block numbers stay strings: converting through Number() rounds values above
// 2^53 and turns 22+ digits into exponent form ("1e+21").
export function directSuggestion(q: string): SearchSuggestion | null {
  if (/^0x[0-9a-fA-F]{40}$/.test(q)) return { type: 'address', value: q, label: q };
  if (/^0x[0-9a-fA-F]{64}$/.test(q)) return { type: 'transaction', value: q, label: q };
  if (/^\d+$/.test(q)) {
    const n = q.replace(/^0+(?=\d)/, '');
    return { type: 'block', value: n, label: `Block #${n}` };
  }
  return null;
}

// suggestionPath is the route a suggestion opens. Used for both a click on a
// suggestion and Enter on a complete identifier, so the two never disagree.
export function suggestionPath(s: SearchSuggestion): string {
  switch (s.type) {
    case 'block':
      return `/block/${s.value}`;
    case 'transaction':
      return `/tx/${s.value}`;
    case 'token':
      return `/token/${s.value}`;
    case 'address':
    default:
      return `/address/${s.value}`;
  }
}

export function suggestionBadge(type: SearchSuggestion['type']): string {
  switch (type) {
    case 'block': return '#';
    case 'transaction': return 'Tx';
    case 'address': return '@';
    case 'token': return 'Token';
    default: return '?';
  }
}

export function suggestionBadgeClass(type: SearchSuggestion['type']): string {
  switch (type) {
    case 'block':
      return 'bg-blue-50 text-blue-600 border border-blue-200';
    case 'transaction':
      return 'bg-green-50 text-green-600 border border-green-200';
    case 'address':
      return 'bg-primary-50 text-primary-600 border border-primary-200';
    case 'token':
      return 'bg-amber-50 text-amber-700 border border-amber-200';
    default:
      return 'bg-neutral-100 text-neutral-600';
  }
}
