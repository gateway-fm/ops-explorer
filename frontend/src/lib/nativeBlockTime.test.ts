import { describe,it,expect } from 'vitest';
import { sampleBlockTime,blockTimeSampleNote } from './nativeBlockTime';
const header=(number:bigint|number,timestamp:bigint|number)=>({number:String(number),timestamp:String(timestamp),hash:'0x'+BigInt(number).toString(16).padStart(64,'0')});
describe('one authorized-page average block time',()=>{
 it.each([undefined,[],[header(1,10)],[header(0,0),header(1,10)]])('needs at least one usable non-genesis interval: %j',rows=>{expect(sampleBlockTime(rows).seconds).toBeUndefined();});
 it('sorts this page without mutation and only uses consecutive positive intervals',()=>{
  const rows=[header(8,100),header(2,20),header(1,10),header(7,97),header(0,0)],original=JSON.stringify(rows);
  expect(sampleBlockTime(rows)).toEqual({seconds:'6.50',pairCount:2,range:{fromBlock:'1',throughBlock:'8'}});
  expect(JSON.stringify(rows)).toBe(original);expect(blockTimeSampleNote(sampleBlockTime(rows))).toContain('gaps and non-positive intervals are excluded');
 });
 it('never interpolates a gap or turns zero/reversed timestamps into a value',()=>{
  for(const rows of [[header(1,0),header(3,100)],[header(1,10),header(2,10)],[header(1,20),header(2,10)]])expect(sampleBlockTime(rows)).toMatchObject({pairCount:0,reason:'no_intervals'});
  expect(sampleBlockTime([header(1,10),header(2,10),header(3,8),header(4,13)])).toMatchObject({seconds:'5.00',pairCount:1});
 });
 it('rejects duplicate/conflicting heights and a repeated hash at another height',()=>{
  for(const rows of [[header(1,10),header(1,10)],[header(1,10),{...header(1,20),hash:header(2,0).hash}],[header(1,10),{...header(2,20),hash:header(1,0).hash}]])expect(sampleBlockTime(rows)).toEqual({pairCount:0,reason:'duplicate'});
 });
 it('preserves uint64 heights and large timestamp deltas without Number conversion',()=>{
  const max=(1n<<64n)-1n;
  expect(sampleBlockTime([header(max-1n,1n),header(max,9007199254740994n)])).toEqual({seconds:'9,007,199,254,740,993.00',pairCount:1,range:{fromBlock:String(max-1n),throughBlock:String(max)}});
 });
 it('rounds the exact rational mean half-up to two decimals',()=>{
  const rows=Array.from({length:9},(_,i)=>header(i+1,1000+i+(i===8?1:0)));
  expect(sampleBlockTime(rows)).toMatchObject({seconds:'1.13',pairCount:8});
  expect(sampleBlockTime([header(1,1000),header(2,1001),header(3,1002),header(4,1004)])).toMatchObject({seconds:'1.33',pairCount:3});
 });
 it('rejects noncanonical or out-of-range headers rather than manufacturing a sample',()=>{
  for(const bad of [{...header(1,0),number:'01'},{...header(1,0),number:String(1n<<64n)},{...header(1,0),timestamp:String(1n<<64n)},{...header(1,0),hash:'0xBAD'}])expect(sampleBlockTime([bad,header(2,10)])).toEqual({pairCount:0,reason:'invalid'});
 });
});
