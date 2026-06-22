import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { type ReactNode } from 'react';

// RD-1083: the /api-docs "Base URL" must fall back to the page's own origin,
// never a baked-in http://localhost:8082. BASE_URL is computed from getConfig at
// module load, so configure the mock before importing the module.
const mockGetConfig = vi.fn();
vi.mock('../lib/runtimeConfig', () => ({
  getConfig: (key: string, fallback?: string) => mockGetConfig(key, fallback),
}));

// PageHeader pulls in react-router (useNavigate); render its children inline so
// the test needs no router.
vi.mock('../components/PageHeader', () => ({
  PageHeader: ({ children }: { children?: ReactNode }) => <div>{children}</div>,
}));

async function renderApiDocs() {
  vi.resetModules();
  const ApiDocs = (await import('./ApiDocs')).default;
  return render(<ApiDocs />);
}

describe('ApiDocs Base URL (RD-1083)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Behave like getConfig with no value configured: return the caller's
    // fallback. Pre-fix that fallback was 'http://localhost:8082'; post-fix it
    // is window.location.origin.
    mockGetConfig.mockImplementation((_key: string, fallback?: string) => fallback ?? '');
  });

  it('falls back to window.location.origin, not http://localhost:8082', async () => {
    await renderApiDocs();
    expect(screen.getAllByText(window.location.origin).length).toBeGreaterThan(0);
    expect(screen.queryByText('http://localhost:8082')).toBeNull();
  });

  it('uses VITE_PUBLIC_API_URL when it is set', async () => {
    mockGetConfig.mockImplementation((key: string, fallback?: string) =>
      key === 'VITE_PUBLIC_API_URL' ? 'https://api.example.com' : (fallback ?? ''),
    );
    await renderApiDocs();
    expect(screen.getAllByText('https://api.example.com').length).toBeGreaterThan(0);
  });
});
