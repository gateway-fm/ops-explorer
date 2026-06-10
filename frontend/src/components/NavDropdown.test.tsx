import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

// NavDropdown builds `blockchainItems` (Gas Tracker) at MODULE LOAD via
// features(), so the privacy value must be set before the module is imported.
// We therefore use vi.resetModules() + dynamic import per test.
const mockGetConfig = vi.fn();
vi.mock('../lib/runtimeConfig', () => ({
  getConfig: (key: string, fallback?: string) => mockGetConfig(key, fallback),
}));

// Stub the surrounding nav machinery so we can render NavDropdown in isolation.
vi.mock('../lib/auth', () => ({
  useAuth: () => ({ isAuthenticated: false, auth: {}, logout: vi.fn() }),
}));
vi.mock('../lib/login', () => ({ redirectToLogin: vi.fn() }));
vi.mock('../hooks/usePrivacyEnabled', () => ({ usePrivacyEnabled: () => false }));
vi.mock('../hooks/useTheme', () => ({ useTheme: () => ({ theme: 'light', setTheme: vi.fn() }) }));
vi.mock('./MetaMask', () => ({ MetaMaskFox: () => null }));
vi.mock('../lib/metamask', () => ({ addNetworkToMetaMask: vi.fn() }));
vi.mock('./NetworkMenu', () => ({ NetworkMenu: () => null }));
// AddNetworkButton pulls in react-query (useNetworkButton) + metamask helpers;
// it's irrelevant to the Charts/Gas-Tracker privacy gate under test, so stub it.
vi.mock('./AddNetworkButton', () => ({ AddNetworkButton: () => null }));

function setPrivacy(on: boolean) {
  mockGetConfig.mockImplementation((key: string, fallback?: string) => {
    if (key === 'VITE_PRIVACY_MODE') return on ? 'true' : '';
    return fallback ?? '';
  });
}

async function renderNav() {
  vi.resetModules();
  const { NavDropdown } = await import('./NavDropdown');
  return render(
    <MemoryRouter>
      <NavDropdown />
    </MemoryRouter>,
  );
}

describe('NavDropdown (privacy gate)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (window as any).ethereum;
  });

  it('standalone: shows Charts link and Gas Tracker entry', async () => {
    setPrivacy(false);
    await renderNav();
    // Charts is a top-level link, always rendered when enabled.
    expect(screen.getByRole('link', { name: 'Charts' })).toBeInTheDocument();
    // Gas Tracker lives inside the Blockchain dropdown — open it.
    fireEvent.click(screen.getByRole('button', { name: /Blockchain/i }));
    expect(screen.getByRole('link', { name: /Gas Tracker/i })).toBeInTheDocument();
  });

  it('privacy: hides Charts link and Gas Tracker entry', async () => {
    setPrivacy(true);
    await renderNav();
    expect(screen.queryByRole('link', { name: 'Charts' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Blockchain/i }));
    expect(screen.queryByRole('link', { name: /Gas Tracker/i })).not.toBeInTheDocument();
  });
});
