import type { ReactNode } from 'react';
import { ChevronDown, ChevronRight, AlertTriangle } from 'lucide-react';
const KNOWN_SELECTORS: Record<string, string> = {
  '0xa9059cbb': 'transfer(address,uint256)',
  '0x23b872dd': 'transferFrom(address,address,uint256)',
  '0x095ea7b3': 'approve(address,uint256)',
  '0x40c10f19': 'mint(address,uint256)',
  '0x42842e0e': 'safeTransferFrom(address,address,uint256)',
  '0x2e1a7d4d': 'withdraw(uint256)',
  '0xd0e30db0': 'deposit()',
};

const CALL_TYPE_STYLES: Record<string, string> = {
  call: 'bg-blue-100 text-blue-700',
  staticcall: 'bg-neutral-100 text-neutral-500',
  delegatecall: 'bg-amber-100 text-amber-700',
  create: 'bg-green-100 text-green-700',
  create2: 'bg-green-100 text-green-700',
};

export function TraceRowPresentation({depth, hasChildren = false, open = true, onToggle, callType, isRoot = false, from, to, selector, value, gasUsed, error}: {depth: number; hasChildren?: boolean; open?: boolean; onToggle?: () => void; callType: string; isRoot?: boolean; from: ReactNode; to?: ReactNode; selector?: string | null; value?: ReactNode; gasUsed?: string; error?: string}) {
 return (
      <div
        data-testid="trace-row"
        className={`flex items-start gap-2 py-2 pr-3 text-sm border-l-2 ${
          error ? 'border-error-400 bg-error-50/40' : 'border-transparent hover:bg-primary-50/40'
        } transition-colors`}
        style={{ paddingLeft: `${depth * 1.5 + 0.5}rem` }}
      >
        {/* expand / collapse toggle (only when there are nested calls) */}
        <button
          onClick={onToggle}
          className={`mt-0.5 shrink-0 text-neutral-400 ${hasChildren ? 'hover:text-neutral-700' : 'invisible'}`}
          aria-label={open ? 'Collapse' : 'Expand'}
        >
          {open ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
        </button>

        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 min-w-0">
          <span className={`text-xs px-1.5 py-0.5 rounded font-medium uppercase ${CALL_TYPE_STYLES[callType] || 'bg-neutral-100 text-neutral-500'}`}>
            {isRoot ? `${callType} (top)` : callType}
          </span>

          {from}
          {to && <><span className="text-neutral-400">→</span>{to}</>}

          {selector && (
            <span
              className="text-xs font-mono text-neutral-500 bg-neutral-100 px-1.5 py-0.5 rounded"
              title={KNOWN_SELECTORS[selector] ? `${KNOWN_SELECTORS[selector]}  (${selector})` : selector}
            >
              {KNOWN_SELECTORS[selector] ? KNOWN_SELECTORS[selector].split('(')[0] : selector}
            </span>
          )}

          {value !== undefined && <span className="text-xs font-mono text-success-600 font-medium">{value}</span>}
          {gasUsed !== undefined && <span className="text-xs text-neutral-400">gas {gasUsed}</span>}

          {error && (
            <span className="inline-flex items-center gap-1 text-xs text-error-600 font-medium">
              <AlertTriangle className="w-3 h-3" />
              {error}
            </span>
          )}
        </div>
      </div>
 );
}
