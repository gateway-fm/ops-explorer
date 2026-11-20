// Derived only from one ordinary authorized block-page response. No source,
// query, polling state or running subtotal is kept by this calculation.
export interface BlockTimeHeader { number:string; hash:string; timestamp:string }
export interface BlockTimeSample {
  seconds?:string;
  pairCount:number;
  range?:{fromBlock:string;throughBlock:string};
  reason?:'missing'|'invalid'|'duplicate'|'no_intervals';
}
const uint64 = (value:string) => /^(0|[1-9][0-9]*)$/.test(value) && value.length<=20 && BigInt(value)<(1n<<64n);
export function sampleBlockTime(headers:readonly BlockTimeHeader[]|undefined):BlockTimeSample {
  if(!headers?.length)return {pairCount:0,reason:'missing'};
  const heights=new Set<string>(),hashes=new Set<string>();
  const rows:{height:bigint;stamp:bigint}[]=[];
  for(const header of headers) {
    if(!uint64(header.number)||!uint64(header.timestamp)||!/^0x[0-9a-f]{64}$/.test(header.hash))return {pairCount:0,reason:'invalid'};
    if(heights.has(header.number)||hashes.has(header.hash))return {pairCount:0,reason:'duplicate'};
    heights.add(header.number);hashes.add(header.hash);
    if(header.number!=='0')rows.push({height:BigInt(header.number),stamp:BigInt(header.timestamp)});
  }
  rows.sort((a,b)=>a.height<b.height?-1:a.height>b.height?1:0);
  const range=rows.length?{fromBlock:String(rows[0].height),throughBlock:String(rows[rows.length-1].height)}:undefined;
  let elapsed=0n,pairCount=0;
  for(let i=1;i<rows.length;i++) {
    const older=rows[i-1],newer=rows[i];
    if(newer.height===older.height+1n&&newer.stamp>older.stamp){elapsed+=newer.stamp-older.stamp;pairCount++;}
  }
  if(!pairCount)return {pairCount,range,reason:'no_intervals'};
  // Exact rational mean, rounded half-up to the nearest hundredth of a second.
  const pairs=BigInt(pairCount),hundredths=(elapsed*200n+pairs)/(pairs*2n);
  return {pairCount,range,seconds:(hundredths/100n).toLocaleString('en-US')+'.'+(hundredths%100n).toString().padStart(2,'0')};
}
export function blockTimeSampleNote(sample:BlockTimeSample):string {
  const prefix=sample.reason==='duplicate'?'Average unavailable: repeated or conflicting block heights or hashes.'
    :sample.reason==='invalid'?'Average unavailable: the block page contains an invalid header.'
    :sample.seconds===undefined?'Average unavailable: no positive consecutive-height interval is available.'
    :'Mean of positive timestamp intervals, rounded half-up to two decimals.';
  const range=sample.range?` Page blocks ${sample.range.fromBlock}–${sample.range.throughBlock}; ${sample.pairCount} usable positive interval${sample.pairCount===1?'':'s'}.`:' No authorized block-page range is available.';
  return prefix+range+' Genesis, gaps and non-positive intervals are excluded. Only this response is used; polls and pages are not combined. This authorized-page sample is not a whole-chain or last-100-block average.';
}
