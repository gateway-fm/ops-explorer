import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it, vi } from 'vitest';
import { WalletsV2 } from './WalletsV2';
const mocks=vi.hoisted(()=>({address:'0x'+'a'.repeat(40),load:vi.fn(),cancel:vi.fn(),prepareUnlink:vi.fn()}));
vi.mock('../lib/v2Session',()=>{const state={identity:{activation:'one'}};return {appSession:{snapshot:()=>state,subscribe:()=>()=>{}}};});
vi.mock('../lib/v2Wallet',()=>({WalletController:class {
  state={links:[{address:mocks.address,verifiedAt:'2026-11-20T12:00:00Z',proofVersion:2}],stage:'idle',unlink:null,busy:false};
  snapshot=()=>this.state;subscribe=()=>()=>{};load=mocks.load;cancel=mocks.cancel;prepareUnlink=mocks.prepareUnlink;
}}));
afterEach(()=>vi.restoreAllMocks());
it('links and copies the exact verified own wallet without new metadata or proof requests',async()=>{
  const writeText=vi.fn().mockResolvedValue(undefined);vi.spyOn(navigator,'clipboard','get').mockReturnValue({writeText} as unknown as Clipboard);
  render(<MemoryRouter><WalletsV2/></MemoryRouter>);await waitFor(()=>expect(mocks.load).toHaveBeenCalledTimes(1));
  expect(screen.getByTitle(mocks.address)).toHaveAttribute('href','/address/'+mocks.address);
  fireEvent.click(screen.getByTitle('Copy to clipboard'));await waitFor(()=>expect(writeText).toHaveBeenCalledWith(mocks.address));
  expect(screen.getByText('Verified')).toBeInTheDocument();fireEvent.click(screen.getByRole('button',{name:'Unlink'}));expect(mocks.prepareUnlink).toHaveBeenCalledWith(mocks.address);expect(mocks.load).toHaveBeenCalledTimes(1);
});
