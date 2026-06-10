import { render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import type { ReactElement, ReactNode } from 'react';
import { TooltipProvider } from '../components/ui/tooltip';

/**
 * Render a component inside the providers it needs (react-query + router),
 * with retries disabled so error/restricted states surface immediately.
 *
 * `path`/`initialEntries` let route-param components (Address, BlockDetail, …)
 * receive their useParams values.
 */
export function renderWithProviders(
  ui: ReactElement,
  opts: { path?: string; initialEntries?: string[] } = {},
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity }, mutations: { retry: false } },
  });
  const { path, initialEntries = ['/'] } = opts;

  const tree: ReactNode = path ? (
    <Routes>
      <Route path={path} element={ui} />
    </Routes>
  ) : (
    ui
  );

  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter initialEntries={initialEntries}>{tree}</MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>,
  );
}
