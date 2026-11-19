import { ParamType } from 'ethers';

// Text integers remain strings all the way into ethers. JSON numbers in
// compound input are rejected rather than accepting rounded uint256 values.
export function parseNativeABIInput(raw:string, param:ParamType):unknown {
  if(param.baseType==='array'||param.baseType==='tuple') {
    const value:unknown=JSON.parse(raw);
    const visit=(v:unknown,p:ParamType):unknown=>{
      if(p.baseType==='array') {if(!Array.isArray(v)||v.length>1000||p.arrayLength!==-1&&v.length!==p.arrayLength)throw Error('Check the array length.');return v.map(x=>visit(x,p.arrayChildren!));}
      if(p.baseType==='tuple'){if(!Array.isArray(v)||v.length!==p.components!.length)throw Error('Use a JSON array for each tuple.');return v.map((x,i)=>visit(x,p.components![i]!));}
      if(typeof v==='number')throw Error('Use quoted decimal strings for integer inputs.');
      if(p.type==='bool'){if(typeof v!=='boolean')throw Error('Use true or false.');return v;}
      if(typeof v!=='string')throw Error('Use quoted strings for tuple and array values.');return v;
    };
    return visit(value,param);
  }
  if(param.type==='bool'){if(raw!=='true'&&raw!=='false')throw Error('Use true or false.');return raw==='true';}
  return raw;
}
