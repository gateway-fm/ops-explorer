import { describe,it,expect } from 'vitest';
import fixture from '../test/fixtures/native-block-counts-go.json';
import { decodeBlock,decodeBlocks } from './nativeReadModels';
// Actual Go explorerv2data.Block serialization; synthetic wire values only.
const raw=(name='exact-zero'):any=>structuredClone(fixture.Cases.find(entry=>entry.Name===name)!.Block);
const page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS'};
describe('strict block-scoped transaction counts',()=>{
 it('decodes every actual Go wire case, preserving exact zero, >2^53, partial and unavailable',()=>{
  expect(fixture.Provenance).toContain('no authorization or runtime claim');
  for(const entry of fixture.Cases)expect(decodeBlock(entry.Block)?.counts?.transactions?.status).toBe(entry.Block.counts.transactions.status);
  expect(decodeBlock(raw('exact-large'))?.transactionCount).toBe('9007199254740993');
  expect(decodeBlocks({items:fixture.Cases.map(entry=>entry.Block),page}).items).toHaveLength(4);
 });
 it('retains omitted source/count compatibility without treating an unqualified scalar as safe',()=>{
  const old=raw();delete old.counts;delete old.transactionCount;expect(decodeBlock(old)?.counts).toBeUndefined();
  const noSource=raw();delete noSource.source;expect(decodeBlock(noSource)?.counts?.transactions?.value).toBe('0');
  const scalar=raw();delete scalar.counts;expect(()=>decodeBlock(scalar)).toThrow();
  const exactWithoutScalar=raw();delete exactWithoutScalar.transactionCount;expect(decodeBlock(exactWithoutScalar)?.counts?.transactions?.value).toBe('0');
 });
 it.each(['fromBlock','throughBlock','headHash'])('rejects a count whose %s is not the parent canonical block',field=>{
  const value=raw();value.counts.transactions.range[field]=field==='headHash'?'0x'+'2'.repeat(64):'9007199254740994';expect(()=>decodeBlock(value)).toThrow();
 });
 it('rejects explicit NODE numeric counts but accepts its known unavailable count',()=>{
  for(const name of ['exact-zero','exact-large','partial']){const value=raw(name);value.source='NODE';expect(()=>decodeBlock(value)).toThrow();}
  const unavailable=raw('pruned-unknown');unavailable.source='NODE';expect(decodeBlock(unavailable)?.counts?.transactions?.status).toBe('unavailable');
 });
 it('never accepts a scalar from a lower bound, unknown count, or mismatching exact value',()=>{
  const mismatch=raw();mismatch.transactionCount='1';expect(()=>decodeBlock(mismatch)).toThrow();
  for(const name of ['partial','pruned-unknown']){const value=raw(name);value.transactionCount='82';expect(()=>decodeBlock(value)).toThrow();}
 });
 it('rejects foreign metrics, CURRENT semantics, missing numeric range and invented unavailable time',()=>{
  const changes=[(v:any)=>{v.counts.holders=v.counts.transactions;},(v:any)=>{v.counts.transactions.consistency='CURRENT';v.counts.transactions.coverage='current';delete v.counts.transactions.range;},(v:any)=>{delete v.counts.transactions.range;},(v:any)=>{v.counts.transactions.value='01';}];
  for(const change of changes){const value=raw();change(value);expect(()=>decodeBlock(value)).toThrow();}
  const unavailable=raw('pruned-unknown');unavailable.counts.transactions.observedAt='2026-11-20T00:00:00Z';expect(()=>decodeBlock(unavailable)).toThrow();
 });
});
