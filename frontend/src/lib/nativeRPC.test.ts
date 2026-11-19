import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppSession, type NativeRPCRequest } from './v2Session';
const status={protocol:'ops-v2',authenticated:true,did:'ops:p1:'+'A'.repeat(43),source_profile:'privado',session_id:'11111111-1111-1111-1111-111111111111',expires_at:1300};
const org='22222222-2222-2222-2222-222222222222';
const self={actorId:status.did,principalKind:'human',sourceProfile:'privado',sourceProfileDigest:'sha256:'+'a'.repeat(64),expiresAt:1300,organizations:[{id:org,slug:'test',name:'Test',isOrgAdmin:false,isOrgReadonlyAdmin:false}]};
const response=(value:unknown)=>new Response(JSON.stringify(value),{headers:{'Content-Type':'application/json'}});
const request:NativeRPCRequest={jsonrpc:'2.0',id:'owned-request',method:'eth_call',params:[{to:'0x'+'b'.repeat(40),data:'0x12345678'},'latest']};
beforeEach(()=>{localStorage.clear();sessionStorage.clear();});
describe('native own-self and read-only RPC operation',()=>{
  it('uses only fixed protected endpoints, cookies and the original app activation',async()=>{
    const transport=vi.fn().mockResolvedValueOnce(response(status)).mockResolvedValueOnce(response(self)).mockResolvedValueOnce(response({jsonrpc:'2.0',id:request.id,result:'0x'}));
    const session=new AppSession('/api',()=>1000,transport as typeof fetch);await session.check();const operation=session.captureOperation();
    expect((await operation.self()).organizations[0]?.id).toBe(org);expect((await operation.rpc(org,request)).result).toBe('0x');
    expect(transport.mock.calls.map(([url])=>url)).toEqual(['/api/auth/status','/api/v2/self','/api/v2/rpc/'+org]);
    for(const [,options] of transport.mock.calls.slice(1)){expect(options.headers.get('Authorization')).toBeNull();expect(options.headers.get('X-OPS-App-Session')).toBe(status.session_id);expect(options.credentials).toBe('include');expect(options.redirect).toBe('error');}
    operation.dispose();await expect(operation.rpc(org,request)).rejects.toThrow('App context changed');
  });
  it.each(['eth_sendTransaction','eth_sendRawTransaction','wallet_switchEthereumChain'])('rejects %s before dispatch',async method=>{
    const transport=vi.fn(async()=>response(status));const session=new AppSession('/api',()=>1000,transport as typeof fetch);await session.check();
    await expect(session.captureOperation().rpc(org,{...request,method})).rejects.toThrow('Invalid native RPC');expect(transport).toHaveBeenCalledTimes(1);
  });
  it('rejects forged organizations, oversized payloads and mismatched RPC IDs',async()=>{
    const transport=vi.fn().mockResolvedValueOnce(response(status)).mockResolvedValueOnce(response({jsonrpc:'2.0',id:'another',result:'0x'}));const session=new AppSession('/api',()=>1000,transport as typeof fetch);await session.check();const operation=session.captureOperation();
    await expect(operation.rpc('../rpc',request)).rejects.toThrow();
    await expect(operation.rpc(org,{...request,params:['x'.repeat(256<<10)]})).rejects.toThrow('too large');
    await expect(operation.rpc(org,request)).rejects.toThrow('Invalid native RPC response');expect(transport).toHaveBeenCalledTimes(2);
  });
  it('rejects an own-self for another actor or unknown fields',async()=>{
    for(const bad of [{...self,actorId:'ops:p1:'+'B'.repeat(43)},{...self,bearer:'secret'}]){
      const transport=vi.fn().mockResolvedValueOnce(response(status)).mockResolvedValueOnce(response(bad));const session=new AppSession('/api',()=>1000,transport as typeof fetch);await session.check();await expect(session.captureOperation().self()).rejects.toThrow('Invalid own identity');
    }
  });
  it('does not publish a late RPC response after the original activation is discarded or expires',async()=>{
    for(const expire of [false,true]){
      let now=1000,finish!:(value:Response)=>void;const held=new Promise<Response>(resolve=>{finish=resolve;});
      const transport=vi.fn().mockResolvedValueOnce(response(status)).mockReturnValueOnce(held);const session=new AppSession('/api',()=>now,transport as typeof fetch);await session.check();const operation=session.captureOperation();const pending=operation.rpc(org,request);
      if(expire)now=1300;else session.discard();finish(response({jsonrpc:'2.0',id:request.id,result:'private'}));await expect(pending).rejects.toThrow('App context changed');
    }
  });
});
