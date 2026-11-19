import { Link } from 'react-router-dom';
import { TokenAvatar } from './TokenAvatar';
import { CopyButton } from './CopyButton';
import { formatTokenType } from '../lib/formatToken';
export interface TokenHeroModel { address:string;name?:string;symbol:string;tokenType:string;usdPrice?:string }
export function TokenHero({ token }: { token: TokenHeroModel }) {
  const name = token.name || 'Unknown Token';
  const symbol = token.symbol || '?';
  return (
    <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between lg:gap-6">
      <div className="flex min-w-0 items-center gap-4">
        <TokenAvatar symbol={symbol} address={token.address} size="lg" />
        <div className="min-w-0">
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <h1 className="text-2xl font-bold text-neutral-900 sm:text-3xl">{name}</h1>
            <span className="text-lg text-neutral-500">{symbol}</span>
            <TokenTypeBadge type={token.tokenType} />
            {token.usdPrice && (
              <span className="font-mono text-sm tabular-nums text-neutral-600">
                ${token.usdPrice}
              </span>
            )}
          </div>
        </div>
      </div>

      <div className="inline-flex min-w-0 max-w-full items-center gap-2 self-start rounded-full border border-neutral-200 bg-neutral-50 px-3 py-1.5 lg:self-auto">
        <Link
          to={`/address/${token.address}`}
          className="truncate font-mono text-sm text-neutral-600 transition-colors hover:text-primary"
          title={token.address}
        >
          {token.address}
        </Link>
        <CopyButton text={token.address} />
      </div>
    </div>
  );
}

export function TokenTypeBadge({ type }: { type: string }) {
  const tone =
    type === 'ERC721'
      ? 'bg-primary/10 text-primary'
      : type === 'ERC1155'
      ? 'bg-warning-500/10 text-warning-700 dark:text-warning-500'
      : 'bg-neutral-200/60 text-neutral-700';
  return (
    <span
      className={`inline-flex items-center rounded-md px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wider ${tone}`}
    >
      {formatTokenType(type)}
    </span>
  );
}

// ---------- Stat bar ----------
