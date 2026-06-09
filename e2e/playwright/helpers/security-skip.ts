import { test, expect } from '@playwright/test';

/**
 * Skip-hardening helpers for @security e2e specs.
 *
 * Two distinct concerns:
 *
 *  1. ENVIRONMENT preconditions (proxy not up, ALLOW_MOCK_LOGIN off, no seed
 *     data on this chain). These legitimately can't run everywhere, so by
 *     default we `test.skip`. But a skip-heavy security suite that always goes
 *     GREEN is worthless as a gate — so when CI_REQUIRE_E2E=1 the SAME
 *     precondition becomes a HARD FAILURE, forcing the environment to be wired
 *     correctly before a "passing" run is trusted. Use `securityGate`.
 *
 *  2. PRIVACY INVARIANTS (an outsider can see an address that must be hidden).
 *     This is NEVER an acceptable skip — it is exactly the regression the test
 *     exists to catch. It must ALWAYS hard-fail, in every environment. Use
 *     `assertPrivacyNotLeaked`. (This replaces the previous
 *     `if (addrExposed) test.skip(...)` self-green anti-pattern at
 *     15-address-labels.spec.ts:151 / 16-participant-visibility.spec.ts:151.)
 */

/** True when the suite must treat environment skips as failures. */
export function requireE2E(): boolean {
  const v = process.env.CI_REQUIRE_E2E;
  return v === '1' || v === 'true';
}

/**
 * Gate a test on an ENVIRONMENT precondition.
 *
 * @param skipCondition  truthy when the environment can't satisfy the test
 *                       (e.g. mock login unavailable, no grant id seeded).
 * @param reason         human explanation, shown on skip or failure.
 *
 * Default: `test.skip(true, reason)` when skipCondition is truthy.
 * Under CI_REQUIRE_E2E=1: throws, so the run goes RED instead of self-greening.
 */
export function securityGate(skipCondition: boolean, reason: string): void {
  if (!skipCondition) return;
  if (requireE2E()) {
    throw new Error(
      `[CI_REQUIRE_E2E] refusing to skip a @security test: ${reason}. ` +
        `The environment must be wired (privacy-proxy up, ALLOW_MOCK_LOGIN=true, seed data present) ` +
        `for the security suite to be a trustworthy gate.`,
    );
  }
  test.skip(true, reason);
}

/**
 * Assert a privacy invariant: an address that must be hidden from this viewer
 * is NOT exposed. ALWAYS hard-fails on exposure (no skip path) — exposure is
 * the precise regression these tests guard against.
 *
 * @param addrExposed  true when the protected address fragment was found in the
 *                     rendered page for a viewer who should not see it.
 * @param reason       context for the failure message.
 */
export function assertPrivacyNotLeaked(addrExposed: boolean | undefined, reason: string): void {
  // A privacy leak is a hard failure everywhere — never skipped, never softened.
  expect(addrExposed ?? false, `PRIVACY LEAK: ${reason}`).toBe(false);
}
