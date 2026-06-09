import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the runtime config source so we can flip privacy mode per-test.
const mockGetConfig = vi.fn();
vi.mock('./runtimeConfig', () => ({
  getConfig: (key: string, fallback?: string) => mockGetConfig(key, fallback),
}));

import { features } from './features';

describe('features()', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('disables charts + gasTracker in privacy mode', () => {
    mockGetConfig.mockImplementation((key: string) =>
      key === 'VITE_PRIVACY_MODE' ? 'true' : '',
    );
    const f = features();
    expect(f.charts).toBe(false);
    expect(f.gasTracker).toBe(false);
    // contractVerification is also off in privacy mode (pre-existing flag).
    expect(f.contractVerification).toBe(false);
  });

  it('enables charts + gasTracker in standalone mode', () => {
    mockGetConfig.mockImplementation(() => '');
    const f = features();
    expect(f.charts).toBe(true);
    expect(f.gasTracker).toBe(true);
    expect(f.contractVerification).toBe(true);
  });
});
