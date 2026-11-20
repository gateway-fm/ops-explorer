import { describe, it, expect, beforeEach } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { renderWithProviders } from '../test/render';
import { server, http, HttpResponse, API } from '../test/msw';
import TokenTransfers from './TokenTransfers';
describe('retained V1 all-transfer type control',()=>{
 beforeEach(()=>server.resetHandlers());
 it('requests the selected source standard and restarts the page',async()=>{
  const seen:string[]=[];
  server.use(http.get(`${API}/token-transfers`,({request})=>{seen.push(new URL(request.url).search);return HttpResponse.json({data:[],total:0,page:3,pageSize:25,totalPages:3});}));
  renderWithProviders(<TokenTransfers/>,{path:'/token-transfers',initialEntries:['/token-transfers?page=3&type=ERC20']});
  await waitFor(()=>expect(seen.length).toBe(1));expect(seen[0]).toContain('type=ERC20');expect(seen[0]).toContain('page=3');
  fireEvent.click(screen.getByRole('tab',{name:'ERC-721'}));await waitFor(()=>expect(seen.at(-1)).toContain('type=ERC721'));expect(seen.at(-1)).toContain('page=1');
  expect(screen.getByRole('tab',{name:'ERC-721'})).toHaveClass('text-primary');
  fireEvent.click(screen.getByRole('tab',{name:'All'}));await waitFor(()=>expect(seen.at(-1)).not.toContain('type='));expect(seen.at(-1)).toContain('page=1');
 });
});
