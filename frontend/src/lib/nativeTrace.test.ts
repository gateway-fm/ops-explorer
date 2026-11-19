import { describe, expect, it } from 'vitest';
import { nativeTraceForest } from './nativeTrace';
import type { InternalTransaction } from './v2ReadTypes';
const hash = '0x' + 'a'.repeat(64);
function trace(id: string, path: string, txHash = hash): InternalTransaction {return {id, traceAddress: path, txHash, blockNumber: '1', from: '0x' + 'b'.repeat(40), to: null, value: null, callType: 'call'};}
describe('native visible trace relationships', () => {
  it('orders numeric paths and attaches only present immediate parents', () => {
    const forest = nativeTraceForest([trace('b', '0,10'), trace('a', '0,2'), trace('p', '0'), trace('orphan', '1,0')]);
    expect(forest.map(n => n.trace.id)).toEqual(['p', 'orphan']);
    expect(forest[0].children.map(n => n.trace.id)).toEqual(['a', 'b']);
    expect(forest[0].parentUnavailable).toBe(true); expect(forest[1].parentUnavailable).toBe(true);
    expect(forest[0].trace.value).toBeNull();
  });
  it('never attaches a frame to a different transaction or invents a transaction root', () => {
    const forest = nativeTraceForest([trace('p', '0'), trace('other', '0,0', '0x' + 'c'.repeat(64))]);
    expect(forest).toHaveLength(2); expect(forest.every(n => !n.children.length)).toBe(true);
    expect(forest.every(n => n.trace.traceAddress !== '')).toBe(true);
  });
  it('keeps ambiguous duplicate paths and invalid paths as unjoined visible rows', () => {
    const forest = nativeTraceForest([trace('p1', '0'), trace('p2', '0'), trace('c', '0,0'), trace('bad', '0,x')]);
    expect(forest).toHaveLength(4); expect(forest.every(n => !n.children.length)).toBe(true);
  });
});
