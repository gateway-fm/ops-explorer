import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NewItemsNotice } from './NewItemsNotice';

describe('NewItemsNotice', () => {
  it('renders nothing when count <= 0', () => {
    const { container } = render(<NewItemsNotice count={0} type="block" onClick={() => {}} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('pluralises and locale-formats the count', () => {
    render(<NewItemsNotice count={1} type="block" onClick={() => {}} />);
    expect(screen.getByTestId('new-items-notice')).toHaveTextContent('1 new block — click to load');
  });

  it('pluralises for >1 and formats thousands', () => {
    render(<NewItemsNotice count={1234} type="transaction" onClick={() => {}} />);
    expect(screen.getByTestId('new-items-notice')).toHaveTextContent('1,234 new transactions — click to load');
  });

  it('renders "N+" when approximate', () => {
    render(<NewItemsNotice count={25} type="transaction" onClick={() => {}} approximate />);
    expect(screen.getByTestId('new-items-notice')).toHaveTextContent('25+ new transactions');
  });

  it('fires onClick', async () => {
    const onClick = vi.fn();
    render(<NewItemsNotice count={3} type="block" onClick={onClick} />);
    await userEvent.click(screen.getByTestId('new-items-notice'));
    expect(onClick).toHaveBeenCalledOnce();
  });
});
