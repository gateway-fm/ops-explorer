import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
export function ChainInfoRow({ label, value, mono, link }: { label: string; value: ReactNode; mono?: boolean; link?: string }) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-start gap-1 sm:gap-4 py-3 border-b border-neutral-100 last:border-b-0">
      <dt className="text-sm text-neutral-500 sm:w-48 shrink-0">{label}</dt>
      <dd className={`text-sm text-neutral-900 break-all ${mono ? 'font-mono' : ''}`}>
        {link ? (
          <Link to={link} className="text-primary hover:text-primary-600 transition-colors">
            {value}
          </Link>
        ) : (
          value
        )}
      </dd>
    </div>
  );
}
