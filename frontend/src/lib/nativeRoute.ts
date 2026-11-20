import { useEffect } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
export function normalizeRouteIdentifier(raw: string, kind: 'address' | 'hash' | 'block'): string | undefined {
  const value = raw.trim();
  if (kind === 'block') return /^(0|[1-9][0-9]{0,18})$/.test(value) && BigInt(value) <= 9223372036854775807n ? value : undefined;
  if (!(kind === 'address' ? /^0x[0-9a-f]{40}$/i : /^0x[0-9a-f]{64}$/i).test(value)) return undefined;
  const canonical = value.toLowerCase();
  return kind === 'address' && canonical === '0x' + '0'.repeat(40) ? undefined : canonical;
}
export function useCanonicalIdentifier(raw: string, kind: 'address' | 'hash' | 'block'): string | undefined {
  const location = useLocation(), navigate = useNavigate(), value = normalizeRouteIdentifier(raw, kind);
  useEffect(() => {
    if (value === undefined || raw === value) return;
    const parts = location.pathname.split('/'), index = parts.findIndex(part => { try { return decodeURIComponent(part) === raw; } catch { return false; } });
    if (index < 0) return;
    parts[index] = value;
    void navigate({pathname: parts.join('/'), search: location.search, hash: location.hash}, {replace: true, state: location.state});
  }, [raw, value, location, navigate]);
  return value;
}
