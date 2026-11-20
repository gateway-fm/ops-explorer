import { describe,it,expect,beforeEach } from 'vitest';
import { boundedHistoryPositions,firstPosition,savePosition } from './nativeNavigation';
describe('bounded native history maps shared by pagination and transfer filters',()=>{
 beforeEach(()=>sessionStorage.clear());
 it('releases invalid and evicted history pointers across many owners and standards',()=>{
  let pointers:Record<string,unknown>={invalid:{response:'must not survive'}};
  for(let i=0;i<40;i++){
   const owner=JSON.stringify(['actor','activation','/transfers',i%3===0?'ERC20':i%3===1?'ERC721':'ERC1155',i]);
   const position=firstPosition(owner,Math.floor(Date.now()/1000)+300);savePosition(position);
   pointers=boundedHistoryPositions(pointers,position);
   expect(Object.keys(pointers).length).toBeLessThanOrEqual(12);expect(JSON.stringify(pointers).length).toBeLessThanOrEqual(64*1024);expect(pointers[owner]).toEqual(position);
  }
  expect(pointers.invalid).toBeUndefined();expect(Object.keys(pointers)).toHaveLength(12);
 });
 it('bounds large historical snapshots even when matching tab records remain small',()=>{
  const pointers:Record<string,unknown>={};
  for(let i=0;i<6;i++){const p=firstPosition('transfer-kind-'+i,Math.floor(Date.now()/1000)+300);savePosition(p);pointers[p.owner]={...p,cursors:Array(6).fill('x'.repeat(8192))};}
  const next=firstPosition('ERC1155',Math.floor(Date.now()/1000)+300);savePosition(next);
  const result=boundedHistoryPositions(pointers,next);expect(JSON.stringify(result).length).toBeLessThanOrEqual(64*1024);expect(Object.keys(result).length).toBeLessThan(7);expect(result.ERC1155).toEqual(next);
 });
});
