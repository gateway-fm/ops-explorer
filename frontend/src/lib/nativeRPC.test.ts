import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppSession, type NativeRPCRequest } from './v2Session';
const status={protocol:'ops-v2',authenticated:true,did:'ops:p1:'+'A'.repeat(43),source_profile:'privado',session_id:'11111111-1111-1111-1111-111111111111',expires_at:1300};
const org='22222222-2222-2222-2222-222222222222';
const self={actorId:status.did,principalKind:'human',sourceProfile:'privado',sourceProfileDigest:'sha256:'+'a'.repeat(64),expiresAt:1300,organizations:[{id:org,slug:'test',name:'Test',isOrgAdmin:false,isOrgReadonlyAdmin:false}]};
const response=(value:unknown)=>new Response(JSON.stringify(value),{headers:{'Content-Type':'application/json'}});
const request:NativeRPCRequest={jsonrpc:'2.0',id:'owned-request',method:'eth_call',params:[{to:'0x'+'b'.repeat(40),data:'0x12345678'},'latest']};
beforeEach(()=>{localStorage.clear();sessionStorage.clear();});
describe('native own-self and read-only RPC operation',()=>{
  it('accepts current bounded own groups with exact totals, retaining legacy fixtures and explicit roles',async()=>{
    const groups=Array.from({length:256},(_,i)=>({id:(i+1).toString(16).padStart(8,'0')+'-3333-3333-3333-333333333333',name:'Group '+i,slug:'group-'+i}));groups[0].name='Банк🏦<>&"\\';groups[1].name='<'.repeat(256);groups[2].name='\ufeffReader\ufeff';
    const projected={...self,organizations:[{...self.organizations[0],groups,groupCount:4096,groupsTruncated:true}]};
    const transport=vi.fn().mockResolvedValueOnce(response(status)).mockResolvedValueOnce(response(projected));const session=new AppSession('/api',()=>1000,transport as typeof fetch);await session.check();
    const current=await session.captureOperation().self();expect(current.organizations).toEqual(projected.organizations);expect(current.organizations[0].isOrgAdmin).toBe(false);
  });
  it('rejects malformed, duplicate, oversized and inconsistent group projections before returning own facts',async()=>{
    const group={id:'33333333-3333-3333-3333-333333333333',name:'Reader',slug:'reader'},orgBase={...self.organizations[0],groups:[group],groupCount:1,groupsTruncated:false};
    const malformed=[{groups:null},{groupCount:0},{groupCount:1.5},{groupCount:-1},{groupCount:4294967296},{groupsTruncated:true},{groups:[group,group],groupCount:2},
      {groups:[{...group,id:'00000000-0000-0000-0000-000000000000'}]},{groups:[{...group,name:'a'.repeat(257)}]},{groups:[{...group,name:' Bank'}]},{groups:[{...group,name:'\u0085Reader'}]},{groups:[{...group,name:'Reader\u0085'}]},{groups:[{...group,name:'Bank\n'}]},
      {groups:[{...group,slug:'a'.repeat(101)}]},{groups:[{...group,slug:'../bad'}]},{groups:[{...group,allowedMethods:['eth_call']}]},{groups:[{...group,organizationId:org}]},
      {groups:[group,{...group,id:'44444444-4444-4444-4444-444444444444'}],groupCount:2},
      {groups:Array.from({length:257},(_,i)=>({...group,id:(i+1).toString(16).padStart(8,'0')+'-3333-3333-3333-333333333333',name:'Group '+i,slug:'g'+i})),groupCount:257}];
    malformed.push({groups:[group,{...group,id:'44444444-4444-4444-4444-444444444444',name:'Other name'}],groupCount:2});
    for(const change of malformed){const transport=vi.fn().mockResolvedValueOnce(response(status)).mockResolvedValueOnce(response({...self,organizations:[{...orgBase,...change}]}));const session=new AppSession('/api',()=>1000,transport as typeof fetch);await session.check();await expect(session.captureOperation().self()).rejects.toThrow();}
    const many=Array.from({length:129},(_,i)=>({...group,id:(i+1).toString(16).padStart(8,'0')+'-3333-3333-3333-333333333333',name:'Group '+i,slug:'g'+i}));
    const incomplete={...orgBase,groupCount:undefined};const transport=vi.fn().mockResolvedValueOnce(response(status)).mockResolvedValueOnce(response({...self,organizations:[incomplete]}));const session=new AppSession('/api',()=>1000,transport as typeof fetch);await session.check();await expect(session.captureOperation().self()).rejects.toThrow();
    const global=vi.fn().mockResolvedValueOnce(response(status)).mockResolvedValueOnce(response({...self,organizations:[{...orgBase,groups:many,groupCount:129},{...orgBase,id:'55555555-5555-5555-5555-555555555555',groups:many,groupCount:129}]}));const bounded=new AppSession('/api',()=>1000,global as typeof fetch);await bounded.check();await expect(bounded.captureOperation().self()).rejects.toThrow();
  });
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
