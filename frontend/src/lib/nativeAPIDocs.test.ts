import { describe, expect, it } from 'vitest';
import { nativeAPIReadGroups } from './nativeAPIDocs';
import { decodeBlock, decodeTransaction, decodeToken, decodeAddressStats, decodeBalance, decodeCode, decodeContract, decodeSyncStatus, decodeIndexerProgress, decodeCatchup, decodeLatestBlock, decodeObservedStats } from './nativeReadModels';
import { readPage, readObservedHistory } from './v2Reads';

const endpoints=nativeAPIReadGroups.flatMap(group=>group.endpoints);
describe('finite native read API documentation',()=>{
  it('covers each finite operation from the native operation table without legacy APIs',()=>{
    // OPS internal/activitymethoddata/method.go finite GET operation inventory.
    const expected=['/chain-id','/stats','/stats/tx-history','/blocks','/blocks/latest/number','/blocks/hash/{hash}','/blocks/{number}','/blocks/{number}/transactions','/blocks/{number}/internal','/transactions/paginated','/transactions','/transactions/{hash}','/transactions/{hash}/internal','/transactions/{hash}/transfers','/transactions/{hash}/logs','/addresses/{address}/stats','/addresses/{address}/transactions','/addresses/{address}/balance','/addresses/{address}/code','/addresses/{address}/balances','/addresses/{address}/transfers','/addresses/{address}/internal','/addresses/{address}/logs','/addresses/{address}/contract','/addresses/{address}/is-contract','/logs','/tokens','/tokens/{address}','/tokens/{address}/holders','/tokens/{address}/transfers','/transfers','/accounts','/search/suggestions','/sync/status','/sync/indexer-progress','/sync/catchup'];
    expect(endpoints.map(endpoint=>endpoint.path).sort()).toEqual(expected.map(path=>'/v2/explorer'+path).sort());
    expect(new Set(endpoints.map(endpoint=>endpoint.path)).size).toBe(endpoints.length);
  });
  it('uses valid native response examples with observed scope and exact string quantities',()=>{
    const decoders:Record<string,(value:unknown)=>unknown>={
      '/blocks/{number}':decodeBlock,'/blocks/hash/{hash}':decodeBlock,'/blocks/latest/number':decodeLatestBlock,'/transactions/{hash}':decodeTransaction,'/tokens/{address}':decodeToken,
      '/addresses/{address}/stats':decodeAddressStats,'/addresses/{address}/balance':decodeBalance,'/addresses/{address}/code':decodeCode,'/addresses/{address}/contract':decodeContract,
      '/sync/status':decodeSyncStatus,'/sync/indexer-progress':decodeIndexerProgress,'/sync/catchup':decodeCatchup,'/stats':decodeObservedStats,'/stats/tx-history':readObservedHistory,
    };
    for(const endpoint of endpoints){
      const path=endpoint.path.replace('/v2/explorer',''),decode=decoders[path];
      if(decode) expect(()=>decode(endpoint.exampleResponse),path).not.toThrow();
      else if(!['/chain-id','/addresses/{address}/is-contract'].includes(path))expect(()=>readPage(endpoint.exampleResponse),path).not.toThrow();
    }
    expect(endpoints.find(endpoint=>endpoint.path.endsWith('/stats'))?.exampleResponse).not.toHaveProperty('totalTransactions');
  });
  it('distinguishes exact retained and CURRENT counts from page samples and unavailable data',()=>{
    const response=endpoints.find(endpoint=>endpoint.path==='/v2/explorer/stats')!.exampleResponse;
    const stats=decodeObservedStats(response);expect(stats.counts?.transactions?.range?.fromBlock).toBe('100');
    expect(stats.counts?.transactions?.coverage).toBe('history_pruned');expect(stats.counts?.accounts?.consistency).toBe('CURRENT');
    expect(stats.counts?.tokens?.status).toBe('unavailable');expect(stats.counts?.tokens?.value).toBeUndefined();
  });
  it('documents source-supported history and log bounds while keeping unsupported feed filters absent',()=>{
    const names=(path:string)=>endpoints.find(endpoint=>endpoint.path==='/v2/explorer'+path)?.params?.map(param=>param.name)??[];
    expect(names('/stats/tx-history')).toEqual(['limit','cursor','interval']);
    expect(names('/addresses/{address}/logs')).toEqual(['address','limit','cursor','topic0','from','to']);
    expect(names('/transactions/{hash}/logs')).toEqual(['hash','limit','cursor','address','topic0','from','to']);
    expect(names('/transfers')).toEqual(['limit','cursor','type']);expect(names('/tokens')).toContain('type');
  });
});
