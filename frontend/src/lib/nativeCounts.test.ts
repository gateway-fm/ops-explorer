import goWire from '../test/fixtures/native-counts-go.json';
import { describe, expect, it } from 'vitest';
import { readScopedCount, readScopedCounts, scopedCountNote, scopedCountText } from './nativeCounts';
import { decodeAddressStats, decodeObservedStats, decodeToken } from './nativeReadModels';

const address='0x'+'a'.repeat(40),headHash='0x'+'b'.repeat(64);
const observedAt='2026-11-20T11:12:13.123456789Z';
const current={status:'exact',value:'0',population:'ORDINARY_VISIBLE_ROWS',consistency:'CURRENT',coverage:'current',observedAt} as const;
const history={...current,value:'9007199254740993',consistency:'SNAPSHOT',coverage:'complete_range',range:{fromBlock:'100',throughBlock:'123',headHash}} as const;
const token={address,symbol:'EX',decimals:18,tokenType:'ERC20',blockNumber:'123',createdAt:observedAt};
const page={hasMore:false,scanLimited:false,population:'LIVE_RETAINED_INDEXED_ROWS_CURRENT_RIGHTS',coverage:'OBSERVED_VISIBLE_PAGE'};
describe('scoped count contract at the native read boundary',()=>{
  // Serialized by the real explorerv2data Go structs. This qualifies wire
  // compatibility; actual historical authority is tested by the OPS engine.
  it.each(goWire)('accepts exact retained $coverage Go responses without hiding retention',row=>{
    const t=decodeToken(row.token),a=decodeAddressStats(row.address),stats=decodeObservedStats(row.stats);
    for(const c of [t?.counts?.tokenTransfers,a?.counts?.transactions,stats.counts?.transactions]){
      expect(c?.status).toBe('exact');expect(c?.coverage).toBe(row.coverage);expect(scopedCountText(c)).toBe('9,007,199,254,740,993');
      if(row.coverage==='history_pruned')expect(scopedCountNote(c)).toContain('pruned');
      if(row.coverage==='index_floor')expect(scopedCountNote(c)).toContain('not been indexed');
    }
  });
  it('preserves exact uint256 and RFC3339Nano without Number rounding',()=>{
    const max=((1n<<256n)-1n).toString(),count=readScopedCount({...history,value:max});
    expect(count.value).toBe(max);expect(count.observedAt).toBe(observedAt);
    expect(scopedCountText(count)).toBe(BigInt(max).toLocaleString('en-US'));
    expect(scopedCountText(readScopedCount(current))).toBe('0');
  });
  it.each(['scan_limited','history_pruned','index_floor'] as const)('labels %s as a lower bound with its actual retained range',coverage=>{
    const {value:_,...base}=history,count=readScopedCount({...base,status:'at_least',lowerBound:'9007199254740993',coverage});
    expect(scopedCountText(count)).toBe('≥9,007,199,254,740,993');
    expect(scopedCountNote(count)).toContain('Retained blocks 100–123');
    expect(scopedCountNote(count).includes('safety limit')).toBe(coverage==='scan_limited');
    expect(scopedCountNote(count)).toContain(observedAt);
  });
  it('keeps absence/unavailability distinct from zero and does not fabricate a sample time',()=>{
    const count=readScopedCount({status:'unavailable',population:'ORDINARY_VISIBLE_ROWS',consistency:'CURRENT',coverage:'current'});
    expect(scopedCountText(count)).toBe('—');expect(scopedCountText(undefined)).toBe('—');expect(count.observedAt).toBeUndefined();
  });
  it.each([
    {...history,value:3},{...history,value:'01'},{...history,value:(1n<<256n).toString()},
    {...history,lowerBound:'1'},{...history,coverage:'scan_limited'},{...history,range:undefined},
    {...history,observedAt:undefined},{...history,observedAt:'2026-02-30T11:12:13Z'},
    {...history,observedAt:'2026-11-20T11:12:60Z'},{...history,range:{...history.range,fromBlock:'124'}},
    {...current,range:history.range},{...current,population:'ALL_ROWS'},{...current,unexpected:'secret'},
    {...history,status:'unavailable'},{...current,status:'at_least',value:undefined,lowerBound:'1'},
  ])('rejects malformed, ambiguous or unproved count %#',raw=>expect(()=>readScopedCount(raw)).toThrow());
  it('rejects unknown metrics and mismatched flat exact summaries',()=>{
    expect(()=>readScopedCounts({globalUsers:current},['accounts'])).toThrow();
    expect(()=>decodeAddressStats({address,txCount:'1'})).toThrow();
    expect(()=>decodeAddressStats({address,txCount:'1',counts:{transactions:{...history,value:'2'}}})).toThrow();
    expect(decodeAddressStats({address,txCount:history.value,counts:{transactions:history}})?.counts?.transactions.value).toBe(history.value);
    const {value:_,...base}=history;
    expect(()=>decodeToken({...token,holderCount:'1',counts:{holders:{...base,status:'at_least',lowerBound:'1',coverage:'scan_limited'}}})).toThrow();
    expect(()=>decodeToken({...token,counts:{accounts:current}})).toThrow();
    expect(decodeToken({...token,holderCount:'0',counts:{holders:current}})?.holderCount).toBe('0');
  });
  it('does not promote a page sample into an account/transaction total',()=>{
    const sampled={privacyEnabled:true,observedTransactions:'3',observedParticipantAddresses:'7',observedBlocks:'2',page};
    expect(decodeObservedStats(sampled).counts).toBeUndefined();
    expect(()=>decodeObservedStats({...sampled,totalAddresses:'7'})).toThrow();
    expect(decodeObservedStats({...sampled,counts:{accounts:current},totalAddresses:'0'}).counts?.accounts.value).toBe('0');
    expect(()=>decodeObservedStats({...sampled,counts:{accounts:current},page:{...page,total:'0'}})).toThrow();
  });
  it('accepts indexed zero/max supply and exact prices, rejecting malformed/oversized prices before render',()=>{
    for(const totalSupply of ['0',((1n<<256n)-1n).toString()])expect(decodeToken({...token,totalSupply})?.totalSupply).toBe(totalSupply);
    for(const usdPrice of ['0','0.000000000000000000000000000001','1'.repeat(128)])expect(decodeToken({...token,usdPrice})?.usdPrice).toBe(usdPrice);
    for(const usdPrice of ['1'.repeat(129),'1e3','-1','00.1',0])expect(()=>decodeToken({...token,usdPrice})).toThrow();
    expect(()=>decodeToken({...token,totalSupply:(1n<<256n).toString()})).toThrow();
  });
});
