import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from './ui/tooltip';
import type { ReactElement } from 'react';
import { AddressLink } from './AddressLink';
import type { AddressVisibility } from '../lib/api';

// Redaction rendering for AddressLink (PLAN §6). useContractName calls
// useQuery unconditionally (hooks rule), so a QueryClientProvider is required
// even though these redaction paths pass a null arg and never fetch.

function r(ui: ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter>{ui}</MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>,
  );
}

const realAddr = '0x407d73d8a49eeb85d32cf465507dd71d507100c1';

describe('AddressLink redaction', () => {
  it('renders a lock-icon "Private" for the [PRIVATE] placeholder', () => {
    r(<AddressLink address="[PRIVATE]" />);
    const el = screen.getByTestId('private-address');
    expect(el).toHaveAttribute('data-redaction', 'private');
    expect(el).toHaveTextContent('Private');
  });

  it('renders [REDACTED] when visibility.level is redacted', () => {
    const vis: AddressVisibility = { address: realAddr, visible: false, level: 'redacted', reason: 'no_access' };
    r(<AddressLink address={realAddr} visibility={vis} />);
    const el = screen.getByTestId('private-address');
    expect(el).toHaveAttribute('data-redaction', 'redacted');
    expect(el).toHaveTextContent('[REDACTED]');
  });

  it('renders the pseudonym (not the real address) for a pseudonymous disclosure', () => {
    // A pseudonymous disclosure IS visible (visible:true) but shown under a
    // stable pseudonym; the `!visible` redaction branch precedes this one.
    const vis: AddressVisibility = { address: realAddr, visible: true, level: 'pseudonymous', reason: 'disclosure_grant', pseudonym: 'Addr-Alpha' };
    r(<AddressLink address={realAddr} visibility={vis} />);
    const el = screen.getByTestId('private-address');
    expect(el).toHaveAttribute('data-redaction', 'pseudonymous');
    expect(el).toHaveTextContent('Addr-Alpha');
    // The real address must NOT leak into the rendered output.
    expect(screen.queryByText(realAddr)).not.toBeInTheDocument();
  });

  it('renders a normal address link when fully visible', () => {
    r(<AddressLink address={realAddr} full showLabel={false} />);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', `/address/${realAddr}`);
    expect(screen.queryByTestId('private-address')).not.toBeInTheDocument();
  });
});
