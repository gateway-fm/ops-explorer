import { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, Info } from 'lucide-react';
import type { FlowParticipant, FlowStep, TransactionFlow as Flow } from '../lib/api';
import { AddressLink } from './AddressLink';
import { arrowLabel, clampIndex, isTechnical, lanesFor, stepTone, TONE_LABEL, visibleSteps, type StepTone } from '../lib/txFlow';

// Flow tab: the transaction drawn as a sequence diagram (one lane per address, one
// row per step) with a Back/Next walkthrough that explains the current step in plain
// words. The steps and their text come from the backend (GET /transactions/{hash}/flow).

const LANE_W = 150;
const ROW_H = 40;
const PAD_Y = 12;
// Room right of the last lane for an event label drawn beside its dot.
const LABEL_ROOM = 150;

const TONE_STROKE: Record<StepTone, string> = {
  call: 'stroke-primary',
  read: 'stroke-neutral-400',
  delegate: 'stroke-neutral-400',
  money: 'stroke-success-600 dark:stroke-success-500',
  create: 'stroke-primary',
  event: 'stroke-warning-600 dark:stroke-warning-500',
  failed: 'stroke-error-600 dark:stroke-error-500',
  undone: 'stroke-neutral-300',
};

// Status colours are fixed hex values, so each needs its own dark-mode shade to stay
// readable on the dark card; neutrals flip by themselves.
const TONE_TEXT: Record<StepTone, string> = {
  call: 'fill-primary',
  read: 'fill-neutral-500',
  delegate: 'fill-neutral-500',
  money: 'fill-success-600 dark:fill-success-500',
  create: 'fill-primary',
  event: 'fill-warning-700 dark:fill-warning-500',
  failed: 'fill-error-600 dark:fill-error-500',
  undone: 'fill-neutral-400',
};

// Same light/dark pairs as the shared .badge-* classes in index.css. The neutral
// scale flips by itself in dark mode, so neutral badges need no dark: overrides.
const TONE_BADGE: Record<StepTone, string> = {
  call: 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400',
  read: 'bg-neutral-100 text-neutral-600',
  delegate: 'bg-neutral-100 text-neutral-600',
  money: 'bg-success-50 dark:bg-success-500/10 text-success-700 dark:text-success-500',
  create: 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400',
  event: 'bg-warning-50 dark:bg-warning-500/10 text-warning-700 dark:text-warning-500',
  failed: 'bg-error-50 dark:bg-error-500/10 text-error-700 dark:text-error-500',
  undone: 'bg-neutral-100 text-neutral-500',
};

type View = 'diagram' | 'list';

function initialView(): View {
  try {
    return window.matchMedia?.('(max-width: 639px)').matches ? 'list' : 'diagram';
  } catch {
    return 'diagram';
  }
}

export function TransactionFlow({ flow }: { flow: Flow }) {
  const technicalCount = useMemo(() => flow.steps.filter(isTechnical).length, [flow.steps]);
  const [showTechnical, setShowTechnical] = useState(false);
  const [view, setView] = useState<View>(initialView);
  const steps = useMemo(() => visibleSteps(flow.steps, showTechnical), [flow.steps, showTechnical]);
  const lanes = useMemo(() => lanesFor(steps, flow.participants), [steps, flow.participants]);
  const [current, setCurrent] = useState(0);
  const idx = clampIndex(current, steps.length);
  const step = steps[idx];

  // Keep the same step selected when technical steps are shown or hidden.
  const toggleTechnical = () => {
    const keep = step?.index;
    const next = visibleSteps(flow.steps, !showTechnical);
    const pos = next.findIndex((s) => s.index >= (keep ?? 0));
    setShowTechnical(!showTechnical);
    setCurrent(pos < 0 ? next.length - 1 : pos);
  };

  const go = (d: number) => setCurrent(clampIndex(idx + d, steps.length));
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault();
      go(1);
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault();
      go(-1);
    }
  };

  const byAddress = useMemo(() => {
    const m = new Map<string, FlowParticipant>();
    for (const p of flow.participants) m.set(p.address.toLowerCase(), p);
    return m;
  }, [flow.participants]);

  return (
    <div className="space-y-4" data-testid="tx-flow" onKeyDown={onKeyDown}>
      <div className="card p-4 space-y-2">
        <p className="text-sm text-neutral-800" data-testid="flow-summary">{flow.summary}</p>
        {(flow.notes ?? []).map((n) => (
          <p key={n} className="flex items-start gap-1.5 text-xs text-neutral-500">
            <Info className="w-3.5 h-3.5 mt-0.5 shrink-0" />
            <span>{n}</span>
          </p>
        ))}
      </div>

      {step && (
        <Walkthrough
          step={step}
          position={idx}
          total={steps.length}
          currency={flow.currency}
          byAddress={byAddress}
          onBack={() => go(-1)}
          onNext={() => go(1)}
        />
      )}

      <div className="flex flex-wrap items-center gap-3 text-sm">
        <div className="inline-flex rounded-md border border-neutral-200 overflow-hidden" role="group" aria-label="Flow view">
          {(['diagram', 'list'] as const).map((v) => (
            <button
              key={v}
              type="button"
              onClick={() => setView(v)}
              aria-pressed={view === v}
              data-testid={`flow-view-${v}`}
              className={`px-3 py-1 capitalize ${view === v ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400 font-medium' : 'text-neutral-600 hover:bg-neutral-50'}`}
            >
              {v}
            </button>
          ))}
        </div>
        {technicalCount > 0 && (
          <label
            className="inline-flex items-center gap-2 text-neutral-600 cursor-pointer select-none"
            title="Read-only calls change nothing. Code delegation is a contract running another contract's code for itself, for example behind a proxy."
          >
            <input type="checkbox" checked={showTechnical} onChange={toggleTechnical} data-testid="flow-show-technical" />
            Show technical steps ({technicalCount})
          </label>
        )}
      </div>

      {view === 'diagram' ? (
        <SequenceDiagram steps={steps} lanes={lanes} current={idx} currency={flow.currency} onSelect={setCurrent} />
      ) : (
        <StepList steps={steps} current={idx} onSelect={setCurrent} />
      )}
    </div>
  );
}

function Walkthrough({
  step, position, total, currency, byAddress, onBack, onNext,
}: {
  step: FlowStep;
  position: number;
  total: number;
  currency: string;
  byAddress: Map<string, FlowParticipant>;
  onBack: () => void;
  onNext: () => void;
}) {
  const tone = stepTone(step);
  // A name worth showing beside the address: a contract name or token symbol, not
  // the short address the link already shows.
  const who = (a: string) => {
    const p = byAddress.get(a.toLowerCase());
    return p && !p.label.startsWith('0x') ? p : undefined;
  };
  return (
    <div className="card p-4 space-y-3" data-testid="flow-walkthrough">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs text-neutral-500" data-testid="flow-position">
          Step {position + 1} of {total}
        </span>
        <div className="flex gap-2">
          <button type="button" className="btn btn-secondary px-3 py-1" onClick={onBack} disabled={position === 0} data-testid="flow-back">
            <ChevronLeft className="w-4 h-4" /> Back
          </button>
          <button type="button" className="btn btn-primary px-3 py-1" onClick={onNext} disabled={position >= total - 1} data-testid="flow-next">
            Next <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      </div>

      <div aria-live="polite">
        <span className={`inline-block text-xs px-2 py-0.5 rounded font-medium mb-2 ${TONE_BADGE[tone]}`}>{TONE_LABEL[tone]}</span>
        <p className="text-base text-neutral-900 break-words" data-testid="flow-step-text">{step.text}</p>
      </div>

      <dl className="grid grid-cols-[auto,1fr] gap-x-3 gap-y-1 text-sm">
        {step.from && (
          <>
            <dt className="text-neutral-500">{step.kind === 'event' ? 'Emitted by' : 'From'}</dt>
            <dd className="min-w-0"><AddressLink address={step.from} label={who(step.from)?.label} chars={6} /></dd>
          </>
        )}
        {step.to && step.kind !== 'event' && (
          <>
            <dt className="text-neutral-500">To</dt>
            <dd className="min-w-0"><AddressLink address={step.to} label={who(step.to)?.label} chars={6} /></dd>
          </>
        )}
        {step.signature && (
          <>
            <dt className="text-neutral-500">{step.kind === 'event' ? 'Event' : 'Function'}</dt>
            <dd className="font-mono text-xs break-all text-neutral-700 self-center">{step.signature}</dd>
          </>
        )}
        {!step.signature && step.method && step.kind !== 'token_transfer' && (
          <>
            <dt className="text-neutral-500">{step.kind === 'event' ? 'Event topic' : 'Selector'}</dt>
            <dd className="font-mono text-xs break-all text-neutral-700 self-center">{step.method}</dd>
          </>
        )}
        {step.value && (
          <>
            <dt className="text-neutral-500">Value</dt>
            <dd className="font-mono text-xs self-center">{step.amount} {currency}</dd>
          </>
        )}
        {step.token && (
          <>
            <dt className="text-neutral-500">Token</dt>
            <dd className="min-w-0"><AddressLink address={step.token.address} label={step.token.symbol} chars={6} /></dd>
          </>
        )}
      </dl>

      {step.args && step.args.length > 0 && (
        <div className="space-y-1">
          <div className="text-xs text-neutral-500">Arguments</div>
          <ul className="code-block p-2 text-xs space-y-0.5">
            {step.args.map((a, i) => (
              <li key={`${a.name}-${i}`} className="break-all">
                <span className="text-neutral-500">{a.name || `arg${i}`}</span>
                <span className="text-neutral-400"> ({a.type})</span>: {a.value || <span className="italic text-neutral-400">not decoded</span>}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function SequenceDiagram({
  steps, lanes, current, currency, onSelect,
}: {
  steps: FlowStep[];
  lanes: FlowParticipant[];
  current: number;
  currency: string;
  onSelect: (i: number) => void;
}) {
  const laneIndex = useMemo(() => {
    const m = new Map<string, number>();
    lanes.forEach((l, i) => m.set(l.address.toLowerCase(), i));
    return m;
  }, [lanes]);
  const lanesW = lanes.length * LANE_W;
  const width = lanesW + LABEL_ROOM;
  const height = steps.length * ROW_H + PAD_Y * 2;
  const x = (a: string) => (laneIndex.get(a.toLowerCase()) ?? 0) * LANE_W + LANE_W / 2;
  const scrollRef = useRef<HTMLDivElement>(null);

  // Bring the current step into view as the walkthrough moves.
  useEffect(() => {
    const box = scrollRef.current;
    const s = steps[current];
    if (!box || !s) return;
    const rowTop = PAD_Y + current * ROW_H;
    const headerH = 48;
    if (rowTop < box.scrollTop || rowTop + ROW_H > box.scrollTop + box.clientHeight - headerH) {
      box.scrollTop = Math.max(0, rowTop - box.clientHeight / 3);
    }
    const left = Math.min(x(s.from), x(s.to)) - LANE_W / 2;
    const right = Math.max(x(s.from), x(s.to)) + LANE_W / 2;
    if (left < box.scrollLeft || right > box.scrollLeft + box.clientWidth) {
      box.scrollLeft = Math.max(0, left - 16);
    }
    // x depends only on lanes, already a dependency through steps/lanes changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current, steps]);

  return (
    <div
      ref={scrollRef}
      className="card overflow-auto max-h-[70vh] relative"
      data-testid="flow-diagram"
      tabIndex={0}
      aria-label="Sequence diagram of the transaction. Use the arrow keys to move between steps."
    >
      <div className="sticky top-0 z-10 flex bg-neutral-50 border-b border-neutral-200" style={{ width }}>
        {lanes.map((l) => (
          <div key={l.address} className="shrink-0 px-2 py-2 text-center" style={{ width: LANE_W }} title={l.address}>
            <div className="text-xs font-medium text-neutral-800 truncate">{l.label}</div>
            <div className="text-[10px] uppercase tracking-wide text-neutral-400">{l.kind}{l.verified ? ' · verified' : ''}</div>
          </div>
        ))}
      </div>
      <svg width={width} height={height} className="block" role="img" aria-label={`${steps.length} steps across ${lanes.length} addresses`}>
        <defs>
          {(Object.keys(TONE_STROKE) as StepTone[]).map((t) => (
            <marker key={t} id={`flow-arrow-${t}`} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
              <path d="M0,0 L10,5 L0,10 z" className={TONE_TEXT[t]} />
            </marker>
          ))}
        </defs>
        {lanes.map((l, i) => (
          <line
            key={l.address}
            x1={i * LANE_W + LANE_W / 2}
            x2={i * LANE_W + LANE_W / 2}
            y1={0}
            y2={height}
            className="stroke-neutral-200"
            strokeDasharray="4 4"
          />
        ))}
        {steps.map((s, i) => {
          const y = PAD_Y + i * ROW_H + ROW_H / 2;
          const tone = stepTone(s);
          const active = i === current;
          const x1 = x(s.from);
          const x2 = x(s.to);
          const label = arrowLabel(s, currency);
          const self = s.kind === 'event' || x1 === x2;
          const maxChars = self ? 22 : Math.max(8, Math.floor(Math.abs(x2 - x1) / 6.5));
          const shown = label.length > maxChars ? `${label.slice(0, maxChars - 1)}…` : label;
          return (
            <g
              key={`${s.index}-${i}`}
              data-testid="flow-row"
              data-active={active || undefined}
              onClick={() => onSelect(i)}
              className="cursor-pointer"
              opacity={tone === 'undone' && !active ? 0.6 : 1}
            >
              <rect
                x={0}
                y={y - ROW_H / 2}
                width={width}
                height={ROW_H}
                className={active ? 'fill-primary' : 'fill-neutral-100'}
                fillOpacity={active ? 0.12 : 0}
              />
              <text x={4} y={y + 4} className="fill-neutral-400 text-[10px]">{i + 1}</text>
              {self ? (
                <>
                  <circle cx={x1} cy={y} r={active ? 6 : 4.5} className={`${TONE_TEXT[tone]}`} />
                  <text x={x1 + 10} y={y + 4} className={`${TONE_TEXT[tone]} text-[11px] ${active ? 'font-semibold' : ''}`}>{shown}</text>
                </>
              ) : (
                <>
                  <line
                    x1={x1}
                    x2={x2 + (x2 > x1 ? -3 : 3)}
                    y1={y}
                    y2={y}
                    className={TONE_STROKE[tone]}
                    strokeWidth={active ? 2.5 : 1.5}
                    strokeDasharray={tone === 'read' || tone === 'delegate' ? '5 3' : undefined}
                    markerEnd={`url(#flow-arrow-${tone})`}
                  />
                  <text x={(x1 + x2) / 2} y={y - 6} textAnchor="middle" className={`${TONE_TEXT[tone]} text-[11px] ${active ? 'font-semibold' : ''}`}>
                    {shown}
                  </text>
                </>
              )}
              <title>{s.text}</title>
            </g>
          );
        })}
      </svg>
    </div>
  );
}

function StepList({ steps, current, onSelect }: { steps: FlowStep[]; current: number; onSelect: (i: number) => void }) {
  const activeRef = useRef<HTMLLIElement>(null);
  useEffect(() => {
    activeRef.current?.scrollIntoView?.({ block: 'nearest' });
  }, [current]);
  return (
    <ol className="card divide-y divide-neutral-100 max-h-[70vh] overflow-y-auto" data-testid="flow-list">
      {steps.map((s, i) => {
        const tone = stepTone(s);
        const active = i === current;
        return (
          <li key={`${s.index}-${i}`} ref={active ? activeRef : undefined}>
            <button
              type="button"
              onClick={() => onSelect(i)}
              aria-current={active ? 'step' : undefined}
              className={`w-full text-left flex items-start gap-2 py-2 pr-3 text-sm ${active ? 'bg-primary-50 dark:bg-primary-900/20' : 'hover:bg-neutral-50'}`}
              style={{ paddingLeft: `${Math.min(s.depth, 6) * 0.75 + 0.75}rem` }}
            >
              <span className="text-xs text-neutral-400 w-6 shrink-0 text-right">{i + 1}</span>
              <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium shrink-0 ${TONE_BADGE[tone]}`}>{TONE_LABEL[tone]}</span>
              <span className={`min-w-0 break-words ${tone === 'undone' ? 'text-neutral-400' : 'text-neutral-800'}`}>{s.text}</span>
            </button>
          </li>
        );
      })}
    </ol>
  );
}
