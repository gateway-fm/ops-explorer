import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { TooltipProvider } from './ui/tooltip';
import type { ReactElement } from 'react';
import { PrivateAddress, PrivateAddressInline } from './PrivateAddress';
import type { AddressVisibility } from '../lib/api';

function r(ui: ReactElement) {
  return render(
    <TooltipProvider>
      <MemoryRouter>{ui}</MemoryRouter>
    </TooltipProvider>,
  );
}

const addr = '0x407d73d8a49eeb85d32cf465507dd71d507100c1';

describe('PrivateAddress', () => {
  it('renders the [PRIVATE] compact placeholder when not visible', () => {
    r(<PrivateAddress address={addr} visibility={null} />);
    const el = screen.getByTestId('private-address');
    expect(el).toHaveTextContent('[PRIVATE]');
    expect(el).toHaveAttribute('data-redaction', 'private');
  });

  it('renders a clickable truncated link when visible', () => {
    const vis: AddressVisibility = { address: addr, visible: true, level: 'full', reason: 'public_address' };
    r(<PrivateAddress address={addr} visibility={vis} />);
    const link = screen.getByRole('link');
    expect(link).toHaveAttribute('href', `/address/${addr}`);
    expect(screen.queryByTestId('private-address')).not.toBeInTheDocument();
  });

  it('PrivateAddressInline always renders the [PRIVATE] marker', () => {
    r(<PrivateAddressInline />);
    expect(screen.getByTestId('private-address')).toHaveTextContent('[PRIVATE]');
  });
});
