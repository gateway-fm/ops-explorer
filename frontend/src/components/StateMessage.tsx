import type { ReactNode } from 'react';

/**
 * Shared loading / error / empty / restricted state message.
 *
 * The app historically rendered these states as bare ad-hoc strings with no
 * stable selector, so tests had to hang off Tailwind classes or copy. This
 * component gives every such state a deterministic `data-testid` so e2e and
 * RTL tests can assert "page loaded OK" (key element present AND no
 * `app-error`/`restricted-state`/stuck `app-loading`) without coupling to copy.
 *
 * Variants:
 *   - loading     -> data-testid="app-loading"
 *   - error       -> data-testid="app-error"
 *   - empty       -> data-testid="app-empty"
 *   - restricted  -> data-testid="restricted-state" (privacy gate; distinct
 *                    from a generic error so S14 can assert it specifically)
 */
export type StateVariant = 'loading' | 'error' | 'empty' | 'restricted';

const DEFAULT_TESTID: Record<StateVariant, string> = {
  loading: 'app-loading',
  error: 'app-error',
  empty: 'app-empty',
  restricted: 'restricted-state',
};

interface StateMessageProps {
  variant: StateVariant;
  /** Primary line. */
  title?: ReactNode;
  /** Secondary explanatory line. */
  detail?: ReactNode;
  /** Optional leading icon / illustration. */
  icon?: ReactNode;
  /** Optional action (e.g. a sign-in button). */
  action?: ReactNode;
  /** Override the default testid for the variant. */
  testid?: string;
  className?: string;
}

export function StateMessage({
  variant,
  title,
  detail,
  icon,
  action,
  testid,
  className = '',
}: StateMessageProps) {
  const resolvedTestId = testid ?? DEFAULT_TESTID[variant];

  if (variant === 'loading') {
    return (
      <div
        data-testid={resolvedTestId}
        role="status"
        aria-live="polite"
        className={`flex items-center justify-center gap-2 px-4 py-8 text-center text-sm text-neutral-400 ${className}`}
      >
        {icon}
        <span>{title ?? 'Loading...'}</span>
      </div>
    );
  }

  const tone =
    variant === 'error'
      ? 'text-error-600'
      : variant === 'restricted'
      ? 'text-neutral-900'
      : 'text-neutral-500';

  return (
    <div
      data-testid={resolvedTestId}
      className={`flex flex-col items-center justify-center gap-3 px-6 py-12 text-center ${className}`}
    >
      {icon}
      {title && <h2 className={`text-lg font-semibold ${tone}`}>{title}</h2>}
      {detail && <p className="mx-auto max-w-md text-sm text-neutral-500">{detail}</p>}
      {action}
    </div>
  );
}
