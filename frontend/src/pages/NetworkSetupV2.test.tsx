import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, expect, it, vi } from 'vitest';
import { PublicNetworkSetup } from './NetworkSetupV2';
import { ThemeProvider } from '../hooks/useTheme';
afterEach(()=>{vi.restoreAllMocks();vi.unstubAllGlobals();});
it('recovers a failed approved-configuration read in the public frame even when theme persistence is denied',async()=>{
  const setup={protocol:'ops-v2',source:'approved-config',chainId:'0x539',chainIdDecimal:'1337',genesisHash:'0x'+'a'.repeat(64),privacyProxyPublicUrl:'https://ops.example.test'};
  const fetcher=vi.fn().mockResolvedValueOnce(new Response('',{status:503})).mockResolvedValue(new Response(JSON.stringify(setup),{status:200}));vi.stubGlobal('fetch',fetcher);
  vi.spyOn(Storage.prototype,'getItem').mockImplementation(()=>{throw new DOMException('denied','SecurityError');});vi.spyOn(Storage.prototype,'setItem').mockImplementation(()=>{throw new DOMException('denied','SecurityError');});
  render(<ThemeProvider><QueryClientProvider client={new QueryClient({defaultOptions:{queries:{retry:false}}})}><MemoryRouter><PublicNetworkSetup/></MemoryRouter></QueryClientProvider></ThemeProvider>);
  fireEvent.click(await screen.findByRole('button',{name:'Try again'}));expect(await screen.findByText('1337 (0x539)')).toBeInTheDocument();
  expect(fetcher).toHaveBeenCalledTimes(2);expect(fetcher.mock.calls.every(([path])=>path==='/api/chain-info')).toBe(true);
  const writeText=vi.fn().mockResolvedValue(undefined);vi.spyOn(navigator,'clipboard','get').mockReturnValue({writeText} as unknown as Clipboard);
  fireEvent.click(screen.getAllByTitle('Copy to clipboard')[2]);await waitFor(()=>expect(writeText).toHaveBeenCalledWith(setup.privacyProxyPublicUrl));
});
