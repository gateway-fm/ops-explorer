import { describe, it, expect } from 'vitest';
import type { FlowStep, FlowParticipant } from './api';
import { arrowLabel, clampIndex, lanesFor, stepTone, visibleSteps } from './txFlow';

const A = '0x' + '1'.repeat(40);
const B = '0x' + '2'.repeat(40);
const C = '0x' + '3'.repeat(40);

function step(over: Partial<FlowStep>): FlowStep {
  return { index: 0, kind: 'call', depth: 0, from: A, to: B, text: '', ...over };
}

describe('txFlow helpers', () => {
  it('classifies tones, failures first', () => {
    expect(stepTone(step({ readOnly: true }))).toBe('read');
    expect(stepTone(step({ kind: 'token_transfer' }))).toBe('money');
    expect(stepTone(step({ kind: 'event', error: 'x' }))).toBe('failed');
    expect(stepTone(step({ rolledBack: true }))).toBe('undone');
  });

  it('hides read-only and delegated-code steps unless they failed', () => {
    const steps = [
      step({ index: 0 }),
      step({ index: 1, readOnly: true }),
      step({ index: 2, readOnly: true, error: 'boom' }),
      step({ index: 3, callType: 'delegatecall' }),
    ];
    expect(visibleSteps(steps, false).map((s) => s.index)).toEqual([0, 2]);
    expect(visibleSteps(steps, true)).toHaveLength(4);
    expect(stepTone(steps[3])).toBe('delegate');
  });

  it('keeps only lanes the shown steps use, in backend order', () => {
    const participants: FlowParticipant[] = [
      { address: A, label: 'Sender', kind: 'account' },
      { address: C, label: 'Oracle', kind: 'contract' },
      { address: B, label: 'Vault', kind: 'contract' },
    ];
    expect(lanesFor([step({})], participants).map((l) => l.label)).toEqual(['Sender', 'Vault']);
  });

  it('labels arrows with amounts, token ids and repeats', () => {
    expect(arrowLabel(step({ kind: 'value_transfer', amount: '1.5', value: '1500000000000000000' }), 'tTEST')).toBe('1.5 tTEST');
    expect(arrowLabel(step({ kind: 'token_transfer', token: { address: C, symbol: 'USDC', raw: '5', amount: '0.000005' } }), 'ETH')).toBe('0.000005 USDC');
    expect(arrowLabel(step({ kind: 'token_transfer', token: { address: C, symbol: 'NFT', standard: 'ERC721', raw: '1', tokenId: '7' } }), 'ETH')).toBe('NFT #7');
    expect(arrowLabel(step({ method: 'latestAnswer', decoded: 'common', repeat: 30 }), 'ETH')).toBe('latestAnswer ×30');
    expect(arrowLabel(step({ kind: 'event', method: '0x' + 'd'.repeat(64) }), 'ETH')).toBe('0xdddddddd…');
  });

  it('clamps the walkthrough index', () => {
    expect(clampIndex(-1, 3)).toBe(0);
    expect(clampIndex(5, 3)).toBe(2);
    expect(clampIndex(0, 0)).toBe(0);
  });
});
