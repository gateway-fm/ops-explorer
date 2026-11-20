import type { AppIdentity } from './v2Session';
import { getConfig } from './runtimeConfig';

// Coordinates only. No rows, credentials or response bodies enter this record.
export interface NativePosition { v: 1; id: string; owner: string; created: number; expires: number; cursors: string[]; index: number; offset: number }
const storageKey = 'native-explorer-navigation-v1';
const maxRecords = 12, maxBytes = 64 * 1024, maxCursors = 24;
export function navigationOwner(identity: AppIdentity | null, path: string | null, limit: number, filters: string): string | null {
  if (!identity || !path || !Number.isSafeInteger(identity.expires) || identity.expires * 1000 <= Date.now()) return null;
  const query = new URLSearchParams(filters); query.sort();
  return JSON.stringify([identity.actor, identity.profile, identity.activation, window.location.origin, getConfig('VITE_API_URL', '/api'), getConfig('VITE_BASE_PATH'), path, limit, query.toString()]);
}
function valid(value: unknown, now: number): value is NativePosition {
  const v = value as NativePosition;
  return !!v && typeof v === 'object' && Object.keys(v).sort().join() === 'created,cursors,expires,id,index,offset,owner,v' && v.v === 1 && typeof v.id === 'string' && /^[a-f0-9-]{36}$/.test(v.id) && typeof v.owner === 'string' && v.owner.length <= 4096 && Number.isSafeInteger(v.created) && Number.isSafeInteger(v.expires) && v.created <= now && v.expires > now && v.expires <= v.created + 600000 && Array.isArray(v.cursors) && v.cursors.length > 0 && v.cursors.length <= maxCursors && v.cursors.every(c => typeof c === 'string' && c.length <= 8192 && !/[^\x21-\x7e]/.test(c)) && Number.isSafeInteger(v.index) && v.index >= 0 && v.index < v.cursors.length && Number.isSafeInteger(v.offset) && v.offset >= 0 && v.offset < 1000000;
}
let memory: NativePosition[] = [];
function records(): NativePosition[] {
  let raw: string | null;
  try { raw = sessionStorage.getItem(storageKey); }
  catch { return memory.filter(v => valid(v, Date.now())); }
  if (!raw) return memory.filter(v => valid(v, Date.now()));
  try {
    if (raw.length > maxBytes) throw Error();
    const values: unknown = JSON.parse(raw);
    if (!Array.isArray(values) || values.length > maxRecords || values.some(v => !valid(v, Date.now()) && (v as NativePosition)?.expires > Date.now())) throw Error();
    return values.filter(v => valid(v, Date.now()));
  } catch { memory = []; try { sessionStorage.removeItem(storageKey); } catch { /* unavailable storage */ } return []; }
}
export function restorePosition(owner: string, pointer?: unknown): NativePosition | undefined {
  const list = records();
  if (pointer === undefined) return [...list].reverse().find(value => value.owner === owner);
  if (typeof pointer === 'string') return list.find(value => value.owner === owner && value.id === pointer);
  // History holds its own coordinates so an earlier entry is not changed by
  // later visits to this resource. A matching original tab record is required.
  if (!valid(pointer, Date.now()) || pointer.owner !== owner) return;
  const original = list.find(value => value.owner === owner && value.id === pointer.id);
  return original && original.created === pointer.created && original.expires === pointer.expires ? pointer : undefined;
}
export function firstPosition(owner: string, expires: number): NativePosition {
  const created = Date.now();
  return { v: 1, id: crypto.randomUUID(), owner, created, expires: Math.min(expires * 1000, created + 600000), cursors: [''], index: 0, offset: 0 };
}
export function savePosition(position: NativePosition): void {
  const values = [...records().filter(v => v.id !== position.id), position].slice(-maxRecords);
  while (JSON.stringify(values).length > maxBytes && values.length > 1) values.shift();
  memory = values;
  try { if (JSON.stringify(values).length <= maxBytes) sessionStorage.setItem(storageKey, JSON.stringify(values)); } catch { /* mounted state still works */ }
}
export function advancePosition(position: NativePosition, cursor: string): NativePosition {
  if (!cursor || cursor.length > 8192 || /[^\x21-\x7e]/.test(cursor)) return position;
  let cursors = [...position.cursors.slice(0, position.index + 1), cursor], offset = position.offset;
  while (cursors.length > maxCursors || JSON.stringify(cursors).length > maxBytes / 2) { cursors = cursors.slice(1); offset++; }
  return { ...position, cursors, offset, index: cursors.length - 1 };
}
