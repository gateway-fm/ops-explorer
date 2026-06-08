import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { AddressLabel, LABEL_CONFIG } from './AddressLabel';
import type { AddressVisibility, VisibilityReason } from '../lib/api';

// Contract under test (REQ-5.5 visibility labels): AddressLabel renders the
// configured short label for each known visibility reason, renders nothing for
// a reason it has no config for (e.g. no_access), and renders nothing when
// visibility data is present but the address is not visible.
//
// Expected text values come from LABEL_CONFIG itself (the source of truth),
// iterated via Object.keys — so the test cannot drift from the config and
// automatically covers any reason added later.
//
// Note (audit §5.2): LABEL_CONFIG contains keys that are NOT in the
// VisibilityReason union (visible_to_grant, participant_override) — a known,
// pre-existing union drift. We cast `key as VisibilityReason` rather than
// reconcile the union here (out of scope).

describe('AddressLabel', () => {
  it('renders the configured label for every reason in LABEL_CONFIG', () => {
    for (const key of Object.keys(LABEL_CONFIG)) {
      const { unmount } = render(
        <AddressLabel reason={key as VisibilityReason} />,
      );
      const expectedText = LABEL_CONFIG[key].text;
      expect(
        screen.getByText(expectedText),
        `reason ${key} should render label "${expectedText}"`,
      ).toBeInTheDocument();
      unmount();
    }
  });

  it('renders nothing for no_access (no LABEL_CONFIG entry)', () => {
    // no_access IS a member of the VisibilityReason union but has no
    // LABEL_CONFIG entry, so the component returns null (the address is
    // [PRIVATE], shown elsewhere).
    expect(LABEL_CONFIG['no_access']).toBeUndefined();
    const { container } = render(
      <AddressLabel reason={'no_access' as VisibilityReason} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when no reason and no visibility are provided', () => {
    const { container } = render(<AddressLabel />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing when visibility.visible is false (the !visible null-render)', () => {
    const visibility: AddressVisibility = {
      address: '0xabc',
      visible: false,
      level: 'redacted',
      reason: 'own_address', // even with a labelable reason, !visible wins
    };
    const { container } = render(<AddressLabel visibility={visibility} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('derives the reason from visibility when visible and reason prop is absent', () => {
    const visibility: AddressVisibility = {
      address: '0xabc',
      visible: true,
      level: 'full',
      reason: 'own_address',
    };
    render(<AddressLabel visibility={visibility} />);
    expect(screen.getByText(LABEL_CONFIG['own_address'].text)).toBeInTheDocument();
  });

  it('prefers the explicit reason prop over visibility.reason', () => {
    const visibility: AddressVisibility = {
      address: '0xabc',
      visible: true,
      level: 'full',
      reason: 'own_address',
    };
    render(
      <AddressLabel reason={'public_address' as VisibilityReason} visibility={visibility} />,
    );
    // reason prop ("Public") wins over visibility.reason ("Mine").
    expect(screen.getByText(LABEL_CONFIG['public_address'].text)).toBeInTheDocument();
    expect(screen.queryByText(LABEL_CONFIG['own_address'].text)).not.toBeInTheDocument();
  });
});
