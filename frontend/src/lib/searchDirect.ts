import type { SearchSuggestion } from './api';

// directSuggestion recognises a query that is already a complete identifier.
export function directSuggestion(q: string): SearchSuggestion | null {
  if (/^0x[0-9a-fA-F]{40}$/.test(q)) return { type: 'address', value: q, label: q };
  if (/^0x[0-9a-fA-F]{64}$/.test(q)) return { type: 'transaction', value: q, label: q };
  if (/^\d+$/.test(q)) return { type: 'block', value: String(Number(q)), label: `Block #${Number(q)}` };
  return null;
}
