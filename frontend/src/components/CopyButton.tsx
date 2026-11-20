import { Copy, Check } from 'lucide-react';
import { useClipboard } from '../hooks/useClipboard';

export function CopyButton({ text, className = '', label = 'Copy to clipboard', children }: { text: string; className?: string; label?: string; children?: React.ReactNode }) {
  const { status, copy } = useClipboard(text);

  return (
    <span className="inline-flex items-center gap-1">
    <button type="button"
      onClick={() => void copy()}
      className={`inline-flex items-center justify-center p-1 rounded text-neutral-400 hover:text-neutral-600 hover:bg-neutral-100 transition-colors ${className}`}
      title={label}
      aria-label={status === 'copied' ? 'Copied' : label}
      disabled={status === 'pending'}
    >
      {children ? status === 'copied' ? 'Copied' : children : status === 'copied' ? (
        <Check className="w-3.5 h-3.5 text-success-600" />
      ) : (
        <Copy className="w-3.5 h-3.5" />
      )}
    </button>
    {status === 'failed' && <span role="alert" className="text-xs text-error-600">Copy failed. Check browser clipboard permission.</span>}
    </span>
  );
}
