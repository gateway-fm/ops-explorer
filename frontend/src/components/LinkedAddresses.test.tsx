import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { type ReactNode } from 'react';
import { LinkedAddresses } from './LinkedAddresses';

// Mock hooks and API
const mockLinkedAddresses = vi.fn();
const mockCreateLinkChallenge = vi.fn();
const mockVerifyLink = vi.fn();
const mockUnlinkAddress = vi.fn();

vi.mock('../hooks/useLinkedAddresses', () => ({
  useLinkedAddresses: () => mockLinkedAddresses(),
  LINKED_ADDRESSES_QUERY_KEY: ['linkedAddresses'],
}));

vi.mock('../lib/api', () => ({
  api: {
    eth: {
      createLinkChallenge: () => mockCreateLinkChallenge(),
      verifyLink: (...args: unknown[]) => mockVerifyLink(...args),
      unlinkAddress: (addr: string) => mockUnlinkAddress(addr),
    },
  },
}));

// Tooltip provider mock (Radix requires it)
vi.mock('./ui/tooltip', () => ({
  Tooltip: ({ children }: { children: ReactNode }) => children,
  TooltipTrigger: ({ children }: { children: ReactNode; asChild?: boolean }) => children,
  TooltipContent: ({ children }: { children: ReactNode }) => (
    <span data-testid="tooltip-content">{children}</span>
  ),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{children}</MemoryRouter>
    </QueryClientProvider>
  );
}

describe('LinkedAddresses', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset window.ethereum
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (window as any).ethereum;
  });

  it('renders nothing when not enabled', () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [],
      userAddresses: [],
      systemAddresses: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: false,
    });

    const { container } = render(<LinkedAddresses />, { wrapper: createWrapper() });
    expect(container.innerHTML).toBe('');
  });

  it('shows loading state', () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [],
      userAddresses: [],
      systemAddresses: [],
      isLoading: true,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    render(<LinkedAddresses />, { wrapper: createWrapper() });
    expect(screen.getByText('Loading linked addresses...')).toBeInTheDocument();
  });

  it('shows error state', () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [],
      userAddresses: [],
      systemAddresses: [],
      isLoading: false,
      error: new Error('Connection failed'),
      refetch: vi.fn(),
      enabled: true,
    });

    render(<LinkedAddresses />, { wrapper: createWrapper() });
    expect(screen.getByText('Connection failed')).toBeInTheDocument();
  });

  it('shows empty state', () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [],
      userAddresses: [],
      systemAddresses: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    render(<LinkedAddresses />, { wrapper: createWrapper() });
    expect(screen.getByText('No linked addresses.')).toBeInTheDocument();
    expect(
      screen.getByText('Link your wallet to see your transactions filtered to your activity.')
    ).toBeInTheDocument();
  });

  it('renders user-linked addresses with verified label', () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [
        { address: '0x1234567890abcdef1234567890abcdef12345678', verified_at: '2024-01-01T00:00:00Z', link_type: 'user' },
      ],
      userAddresses: [
        { address: '0x1234567890abcdef1234567890abcdef12345678', verified_at: '2024-01-01T00:00:00Z', link_type: 'user' },
      ],
      systemAddresses: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    render(<LinkedAddresses />, { wrapper: createWrapper() });
    expect(screen.getByText('Verified addresses')).toBeInTheDocument();
    expect(screen.getByText('Verified')).toBeInTheDocument();
    expect(screen.getByText('Unlink')).toBeInTheDocument();
  });

  it('renders system-linked addresses with correct label', () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [
        { address: '0xabcdef1234567890abcdef1234567890abcdef12', verified_at: '2024-01-02T00:00:00Z', link_type: 'system' },
      ],
      userAddresses: [],
      systemAddresses: [
        { address: '0xabcdef1234567890abcdef1234567890abcdef12', verified_at: '2024-01-02T00:00:00Z', link_type: 'system' },
      ],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    render(<LinkedAddresses />, { wrapper: createWrapper() });
    expect(screen.getByText('Inferred from transactions')).toBeInTheDocument();
    expect(screen.getByText('From transactions')).toBeInTheDocument();
    // System addresses should not have an unlink button
    expect(screen.queryByText('Unlink')).not.toBeInTheDocument();
  });

  it('shows link button and handles MetaMask not installed', async () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [],
      userAddresses: [],
      systemAddresses: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    render(<LinkedAddresses />, { wrapper: createWrapper() });

    const linkButton = screen.getByText('Link wallet');
    expect(linkButton).toBeInTheDocument();

    // Click without MetaMask installed
    fireEvent.click(linkButton);

    await waitFor(() => {
      expect(
        screen.getByText('MetaMask is not installed. Please install MetaMask to link your wallet.')
      ).toBeInTheDocument();
    });
  });

  it('handles MetaMask user rejection gracefully', async () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [],
      userAddresses: [],
      systemAddresses: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    // Mock MetaMask that rejects
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (window as any).ethereum = {
      isMetaMask: true,
      request: vi.fn().mockRejectedValue(new Error('User rejected')),
    };

    render(<LinkedAddresses />, { wrapper: createWrapper() });

    fireEvent.click(screen.getByText('Link wallet'));

    await waitFor(() => {
      expect(
        screen.getByText('Wallet connection was rejected. Please try again.')
      ).toBeInTheDocument();
    });
  });

  it('completes link flow with MetaMask', async () => {
    mockLinkedAddresses.mockReturnValue({
      addresses: [],
      userAddresses: [],
      systemAddresses: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    const mockEthereum = {
      isMetaMask: true,
      request: vi.fn()
        .mockResolvedValueOnce(['0xabc123']) // eth_requestAccounts
        .mockResolvedValueOnce('0xsignature123'), // personal_sign
    };
    (window as Record<string, unknown>).ethereum = mockEthereum;

    mockCreateLinkChallenge.mockResolvedValue({
      nonce: 'test-nonce',
      message: 'Sign this message',
    });
    mockVerifyLink.mockResolvedValue({
      message: 'Address linked',
      address: '0xabc123',
    });

    render(<LinkedAddresses />, { wrapper: createWrapper() });

    fireEvent.click(screen.getByText('Link wallet'));

    await waitFor(() => {
      expect(mockCreateLinkChallenge).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(mockVerifyLink).toHaveBeenCalledWith('test-nonce', '0xabc123', '0xsignature123');
    });
  });

  it('handles unlink with confirmation', async () => {
    const addr = '0x1234567890abcdef1234567890abcdef12345678';
    mockLinkedAddresses.mockReturnValue({
      addresses: [{ address: addr, verified_at: '2024-01-01T00:00:00Z', link_type: 'user' }],
      userAddresses: [{ address: addr, verified_at: '2024-01-01T00:00:00Z', link_type: 'user' }],
      systemAddresses: [],
      isLoading: false,
      error: null,
      refetch: vi.fn(),
      enabled: true,
    });

    mockUnlinkAddress.mockResolvedValue({ success: true });

    render(<LinkedAddresses />, { wrapper: createWrapper() });

    // First click shows confirmation
    fireEvent.click(screen.getByText('Unlink'));

    await waitFor(() => {
      expect(screen.getByText(/Are you sure you want to unlink/)).toBeInTheDocument();
    });

    // Confirm unlink
    fireEvent.click(screen.getByText('Confirm'));

    await waitFor(() => {
      expect(mockUnlinkAddress).toHaveBeenCalledWith(addr);
    });
  });
});
