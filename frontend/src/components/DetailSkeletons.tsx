export function DetailHeroSkeleton({wide = false}: {wide?: boolean}) {
  return (
    <div role="status" aria-label="Loading details" className="space-y-4">
      <div className="flex items-center gap-4">
        <div className="h-14 w-14 animate-pulse rounded-full bg-neutral-200" />
        <div className={`h-7 ${wide ? 'w-64' : 'w-48'} animate-pulse rounded bg-neutral-200`} />
      </div>
      <div className="h-9 w-96 max-w-full animate-pulse rounded-full bg-neutral-200/60" />
    </div>
  );
}

export function DetailRowsSkeleton({ rows = 4 }: { rows?: number }) {
  return (
    <div role="status" aria-label="Loading records" className="overflow-hidden rounded-2xl border border-neutral-200 bg-neutral-50">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 border-b border-neutral-200/60 px-4 py-5 last:border-b-0">
          <div className="h-3 w-32 animate-pulse rounded bg-neutral-200" />
          <div className="hidden h-3 w-16 animate-pulse rounded bg-neutral-200/70 md:block" />
          <div className="flex-1 h-3 animate-pulse rounded bg-neutral-200/70" />
          <div className="h-3 w-16 animate-pulse rounded bg-neutral-200" />
        </div>
      ))}
    </div>
  );
}
