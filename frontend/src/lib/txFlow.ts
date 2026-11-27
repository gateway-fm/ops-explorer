import type { FlowParticipant, FlowStep } from './api';

// Helpers for the Flow tab. The backend owns the step model and its plain-language
// text; these only decide what to draw and how.

export type StepTone = 'call' | 'read' | 'delegate' | 'money' | 'create' | 'event' | 'failed' | 'undone';

export function stepTone(step: FlowStep): StepTone {
  if (step.error) return 'failed';
  if (step.rolledBack) return 'undone';
  switch (step.kind) {
    case 'value_transfer':
    case 'token_transfer':
      return 'money';
    case 'create':
      return 'create';
    case 'event':
      return 'event';
  }
  if (step.callType === 'delegatecall' || step.callType === 'callcode') return 'delegate';
  return step.readOnly ? 'read' : 'call';
}

export const TONE_LABEL: Record<StepTone, string> = {
  call: 'Call',
  read: 'Read-only call',
  delegate: 'Runs other code',
  money: 'Transfer',
  create: 'New contract',
  event: 'Event',
  failed: 'Failed',
  undone: 'Undone',
};

// Technical steps are read-only calls (they change nothing) and code delegation (a
// contract running another contract's code for itself, e.g. behind a proxy). Hiding
// them by default keeps the story to what changed hands. A failed step always shows.
export function isTechnical(step: FlowStep): boolean {
  if (step.error) return false;
  return !!step.readOnly || step.callType === 'delegatecall' || step.callType === 'callcode';
}

export function visibleSteps(steps: FlowStep[], showTechnical: boolean): FlowStep[] {
  return showTechnical ? steps : steps.filter((s) => !isTechnical(s));
}

// Lanes are the participants that appear in the shown steps, in the backend's order
// (first appearance), so hiding read-only calls also drops lanes only they used.
export function lanesFor(steps: FlowStep[], participants: FlowParticipant[]): FlowParticipant[] {
  const used = new Set<string>();
  for (const s of steps) {
    used.add(s.from.toLowerCase());
    used.add(s.to.toLowerCase());
  }
  const out = participants.filter((p) => used.has(p.address.toLowerCase()));
  // An address the backend did not list (should not happen) still gets a lane.
  const listed = new Set(out.map((p) => p.address.toLowerCase()));
  for (const s of steps) {
    for (const a of [s.from, s.to]) {
      if (!listed.has(a.toLowerCase())) {
        listed.add(a.toLowerCase());
        out.push({ address: a, label: a ? shortAddr(a) : 'new contract', kind: 'account' });
      }
    }
  }
  return out;
}

export function shortAddr(a: string): string {
  return /^0x[0-9a-fA-F]{40}$/.test(a) ? `${a.slice(0, 6)}…${a.slice(-4)}` : a;
}

// The short label drawn on a step's arrow.
export function arrowLabel(step: FlowStep, currency: string): string {
  const times = step.repeat && step.repeat > 1 ? ` ×${step.repeat}` : '';
  switch (step.kind) {
    case 'value_transfer':
      return `${step.amount ?? step.value} ${currency}${times}`;
    case 'token_transfer': {
      const t = step.token;
      if (!t) return 'Transfer';
      const sym = t.symbol ?? 'tokens';
      if (t.standard === 'ERC721' && t.tokenId) return `${sym} #${t.tokenId}`;
      return `${t.amount ?? `${t.raw} units`} ${sym}`;
    }
    case 'create':
      return 'create';
  }
  const name = step.method ? methodName(step) : '(no function)';
  const value = step.value ? ` + ${step.amount ?? step.value} ${currency}` : '';
  return `${name}${value}${times}`;
}

function methodName(step: FlowStep): string {
  if (step.decoded || !step.method) return step.method ?? '';
  // Unknown event topics are long; selectors are already short.
  return step.method.length > 12 ? `${step.method.slice(0, 10)}…` : step.method;
}

export function clampIndex(i: number, length: number): number {
  if (length <= 0) return 0;
  return Math.min(Math.max(i, 0), length - 1);
}
