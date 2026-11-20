import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { CopyButton } from './CopyButton';
import { APICopyButton } from './APIReadPresentation';
import { TransactionInput } from './TransactionDetailPresentation';

afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers(); });
describe('shared clipboard feedback', () => {
  it('reports success only after clipboard resolves, then resets', async () => {
    vi.useFakeTimers(); let finish!: () => void;
    const writeText = vi.fn(() => new Promise<void>(resolve => { finish = resolve; }));
    vi.spyOn(navigator, 'clipboard', 'get').mockReturnValue({writeText} as unknown as Clipboard);
    render(<CopyButton text="exact authorized value"/>);
    fireEvent.click(screen.getByTitle('Copy to clipboard'));
    expect(writeText).toHaveBeenCalledWith('exact authorized value'); expect(screen.queryByLabelText('Copied')).toBeNull();
    await act(async () => finish()); expect(screen.getByLabelText('Copied')).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(1500)); expect(screen.queryByLabelText('Copied')).toBeNull();
  });
  it.each(['reject', 'unavailable'])('gives local failure feedback when clipboard is %s', async reason => {
    vi.spyOn(navigator, 'clipboard', 'get').mockReturnValue(reason === 'reject' ? {writeText: vi.fn().mockRejectedValue(Error('denied'))} as unknown as Clipboard : undefined as unknown as Clipboard);
    render(<APICopyButton text="/api"/>); await act(async () => fireEvent.click(screen.getByTitle('Copy')));
    expect(screen.getByRole('alert')).toHaveTextContent('Copy failed'); expect(screen.queryByLabelText('Copied')).toBeNull();
  });
  it('retires a pending completion when the displayed account changes', async () => {
    let finish!: () => void; const writeText = vi.fn(() => new Promise<void>(resolve => { finish = resolve; }));
    vi.spyOn(navigator, 'clipboard', 'get').mockReturnValue({writeText} as unknown as Clipboard);
    const view = render(<CopyButton text="old actor"/>); fireEvent.click(screen.getByTitle('Copy to clipboard'));
    view.rerender(<CopyButton text="new actor"/>); await act(async () => finish());
    expect(screen.queryByLabelText('Copied')).toBeNull(); expect(screen.getByTitle('Copy to clipboard')).toBeEnabled();
  });
  it('copies exact transaction input and reports a rejected write', async () => {
    const writeText = vi.fn().mockRejectedValue(Error('denied')); vi.spyOn(navigator, 'clipboard', 'get').mockReturnValue({writeText} as unknown as Clipboard);
    render(<TransactionInput data="deadbeef"/>); await act(async () => fireEvent.click(screen.getByText('Copy')));
    expect(writeText).toHaveBeenCalledWith('0xdeadbeef'); expect(screen.getByRole('alert')).toHaveTextContent('Copy failed');
  });
});
