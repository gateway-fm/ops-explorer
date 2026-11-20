export type CountMetric = 'transactions'|'internalTransactions'|'tokenTransfers'|'holders'|'accounts'|'tokens'|'blocks';
export interface ScopedCount {
  status: 'exact'|'at_least'|'unavailable';
  value?: string; lowerBound?: string;
  population: 'ORDINARY_VISIBLE_ROWS';
  consistency: 'SNAPSHOT'|'CURRENT';
  observedAt?: string;
  range?: {fromBlock:string; throughBlock:string; headHash:string};
  coverage: 'complete_range'|'scan_limited'|'current'|'history_pruned'|'index_floor';
}
export type ScopedCounts = Partial<Record<CountMetric,ScopedCount>>;
const quantity = (v:unknown):v is string => typeof v==='string' && /^(0|[1-9][0-9]{0,77})$/.test(v) && BigInt(v)<(1n<<256n);
function object(v:unknown):Record<string,unknown> {
  if(!v || typeof v!=='object' || Array.isArray(v))throw Error('Invalid scoped count');
  return v as Record<string,unknown>;
}
function fields(v:Record<string,unknown>,allowed:string[],required:string[]) {
  if(Object.keys(v).some(key=>!allowed.includes(key)) || required.some(key=>!Object.hasOwn(v,key)))throw Error('Invalid scoped count fields');
}
// Go emits RFC3339Nano. Validate the calendar without rounding its original
// fractional precision into a different observation timestamp.
function timestamp(v:unknown):v is string {
  if(typeof v!=='string')return false;
  const parts=/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?Z$/.exec(v);
  if(!parts)return false;
  const date=new Date(v);
  return Number.isFinite(date.getTime()) && Number(parts[1])>=1 && date.getUTCFullYear()===Number(parts[1]) && date.getUTCMonth()+1===Number(parts[2]) && date.getUTCDate()===Number(parts[3]) && date.getUTCHours()===Number(parts[4]) && date.getUTCMinutes()===Number(parts[5]) && date.getUTCSeconds()===Number(parts[6]);
}
export function readScopedCount(raw:unknown):ScopedCount {
  const value=object(raw);
  fields(value,['status','value','lowerBound','population','consistency','observedAt','range','coverage'],['status','population','consistency','coverage']);
  if(value.population!=='ORDINARY_VISIBLE_ROWS' || !['SNAPSHOT','CURRENT'].includes(String(value.consistency)) || !['complete_range','scan_limited','current','history_pruned','index_floor'].includes(String(value.coverage)))throw Error('Invalid count population');
  if(value.status==='exact') {
    if(!quantity(value.value) || 'lowerBound' in value || value.coverage==='scan_limited')throw Error('Invalid exact count');
  } else if(value.status==='at_least') {
    if(!quantity(value.lowerBound) || 'value' in value || value.consistency!=='SNAPSHOT' || !['scan_limited','history_pruned','index_floor'].includes(String(value.coverage)))throw Error('Invalid incomplete count');
  } else if(value.status==='unavailable') {
    if('value' in value || 'lowerBound' in value)throw Error('Unavailable count contains a number');
  } else throw Error('Invalid count status');
  if(value.observedAt!==undefined && !timestamp(value.observedAt) || value.status!=='unavailable' && !timestamp(value.observedAt))throw Error('Invalid count observation');
  if(value.consistency==='CURRENT' && (value.coverage!=='current' || value.range!==undefined) || value.consistency==='SNAPSHOT' && value.coverage==='current')throw Error('Invalid count consistency');
  if(value.consistency==='SNAPSHOT' && value.status!=='unavailable' && value.range===undefined)throw Error('Historical count has no retained range');
  if(value.range!==undefined) {
    const range=object(value.range);fields(range,['fromBlock','throughBlock','headHash'],['fromBlock','throughBlock','headHash']);
    if(!quantity(range.fromBlock) || !quantity(range.throughBlock) || BigInt(range.fromBlock)>BigInt(range.throughBlock) || typeof range.headHash!=='string' || !/^0x[0-9a-f]{64}$/.test(range.headHash))throw Error('Invalid count range');
  }
  return value as unknown as ScopedCount;
}
export function readScopedCounts(raw:unknown,allowed:readonly CountMetric[]):ScopedCounts {
  const input=object(raw),out:ScopedCounts={};
  if(Object.keys(input).length>allowed.length)throw Error('Invalid count metrics');
  for(const [key,value] of Object.entries(input)) {
    if(!allowed.includes(key as CountMetric))throw Error('Invalid count metric');
    out[key as CountMetric]=readScopedCount(value);
  }
  return out;
}
export function verifyFlatCount(value:unknown,count:ScopedCount|undefined):void {
  if(value===undefined)return;
  if(!count || count.status!=='exact' || !quantity(value) || value!==count.value)throw Error('Count summary disagrees with its exact result');
}
export function scopedCountText(count:ScopedCount|undefined):string {
  if(!count || count.status==='unavailable')return '—';
  const n=count.status==='exact'?count.value!:count.lowerBound!;
  return (count.status==='at_least'?'≥':'')+BigInt(n).toLocaleString('en-US');
}
export function scopedCountNote(count:ScopedCount|undefined):string {
  if(!count || count.status==='unavailable')return 'Count unavailable for this read.';
  const parts=[count.status==='at_least' && count.coverage==='scan_limited'?'Count stopped at the safety limit.':count.status==='at_least'?'At least this many permitted rows are available.':'Count covers your permitted rows.'];
  if(count.consistency==='CURRENT')parts.push('Current indexed values.');
  if(count.range)parts.push(`Retained blocks ${count.range.fromBlock}–${count.range.throughBlock}.`);
  if(count.coverage==='history_pruned')parts.push('Older history has been pruned.');
  if(count.coverage==='index_floor')parts.push('Older history has not been indexed.');
  if(count.observedAt)parts.push('Observed '+count.observedAt+'.');
  return parts.join(' ');
}
