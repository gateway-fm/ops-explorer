import { useNavigate } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  children?: React.ReactNode;
}

export function PageHeader({ title, subtitle, children }: PageHeaderProps) {
  const navigate = useNavigate();

  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 sm:gap-4 mb-4 sm:mb-6">
      <div className="flex items-center gap-3 sm:gap-4 min-w-0">
        <button
          onClick={() => navigate('/')}
          className="p-1.5 sm:p-2 rounded-lg bg-neutral-100 hover:bg-neutral-200 border border-neutral-200 transition-colors shrink-0"
          title="Go back"
        >
          <ArrowLeft className="w-4 h-4 text-neutral-500" />
        </button>
        <div className="min-w-0">
          <h1 className="text-xl sm:text-2xl font-bold text-neutral-900 truncate">{title}</h1>
          {subtitle && <p className="text-xs sm:text-sm text-neutral-500 truncate">{subtitle}</p>}
        </div>
      </div>
      {children && (
        <div className="flex items-center gap-2 flex-wrap pl-9 sm:pl-0">
          {children}
        </div>
      )}
    </div>
  );
}
