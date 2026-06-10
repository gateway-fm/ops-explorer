package chaininfo

import (
	"context"
	"math/big"
	"sync"
	"time"

	"explorer/internal/rpc"
	"explorer/pkg/log"
)

// Info holds cached chain metadata fetched from the node.
type Info struct {
	ChainID        string `json:"chainId"`
	ChainIDDecimal uint64 `json:"chainIdDecimal"`
	// PrivacyProxyPublicURL is the privacy-proxy PUBLIC BASE url (WITHOUT any
	// "/rpc" suffix). It is surfaced ONLY so the privacy-mode MetaMask setup
	// dialog can pre-fill the jwt-injector --upstream hint. It is NOT a wallet
	// RPC target: a browser wallet cannot attach the bearer + org path the
	// proxy requires, so we never hand the proxy endpoint to a wallet. The
	// public proxy URL is already public (it is the OAuth endpoint), so
	// surfacing it is safe; internal hosts (RPC_URL, internal PRIVACY_PROXY_URL,
	// INDEXER_URL, DATABASE_URL) are never exposed here. Empty (and omitted)
	// when PRIVACY_PROXY_PUBLIC_URL is not configured.
	PrivacyProxyPublicURL string `json:"privacyProxyPublicUrl,omitempty"`
	NetworkID             string `json:"networkId"`
	ClientVersion         string `json:"clientVersion"`
	ProtocolVersion       string `json:"protocolVersion"`
	LatestBlock           uint64 `json:"latestBlock"`
	GasPrice              string `json:"gasPrice"`
	PeerCount             int    `json:"peerCount"`
	IsSyncing             bool   `json:"isSyncing"`
	GenesisHash           string `json:"genesisHash"`
	UpdatedAt             string `json:"updatedAt"`
}

// Service caches chain info in memory and refreshes periodically.
type Service struct {
	rpc      *rpc.Client
	mu       sync.RWMutex
	cached   *Info
	interval time.Duration
	// privacyProxyPublicURL is the static privacy-proxy public base URL
	// surfaced via Info.PrivacyProxyPublicURL (a hint for the MetaMask setup
	// dialog, NOT a wallet RPC target). Configured once at startup (from
	// PRIVACY_PROXY_PUBLIC_URL) and never changes; the periodic node refresh
	// does not touch it.
	privacyProxyPublicURL string
}

// NewService creates a chain info service that refreshes at the given interval.
func NewService(rpcClient *rpc.Client, interval time.Duration) *Service {
	return &Service{
		rpc:      rpcClient,
		interval: interval,
	}
}

// SetPrivacyProxyPublicURL sets the privacy-proxy public base URL surfaced via
// Info.PrivacyProxyPublicURL. Callers should pass the proxy PUBLIC base URL
// WITHOUT any "/rpc" suffix — it is only a hint for the MetaMask setup dialog's
// jwt-injector --upstream field, never a wallet RPC target. An empty value
// leaves Info.PrivacyProxyPublicURL unset (omitted from the JSON response).
func (s *Service) SetPrivacyProxyPublicURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.privacyProxyPublicURL = url
}

// Start fetches chain info immediately, then refreshes on a timer until ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	s.refresh(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		}
	}
}

// Get returns the most recently cached chain info. The returned value is a
// copy with the statically-configured privacy-proxy public URL injected, so
// callers never mutate the internal cache and the periodically-refreshed node
// fields stay separate from the configured proxy URL.
func (s *Service) Get() *Info {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := Info{}
	if s.cached != nil {
		out = *s.cached
	}
	out.PrivacyProxyPublicURL = s.privacyProxyPublicURL
	return &out
}

func (s *Service) refresh(ctx context.Context) {
	info := &Info{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	raw := s.rpc.Raw()

	// Chain ID
	if chainID, err := s.rpc.ChainID(ctx); err == nil {
		info.ChainID = "0x" + chainID.Text(16)
		info.ChainIDDecimal = chainID.Uint64()
	}

	// Client version (web3_clientVersion)
	var clientVersion string
	if err := raw.CallContext(ctx, &clientVersion, "web3_clientVersion"); err == nil {
		info.ClientVersion = clientVersion
	}

	// Network ID (net_version)
	var netVersion string
	if err := raw.CallContext(ctx, &netVersion, "net_version"); err == nil {
		info.NetworkID = netVersion
	}

	// Protocol version (eth_protocolVersion)
	var protocolVersion string
	if err := raw.CallContext(ctx, &protocolVersion, "eth_protocolVersion"); err == nil {
		info.ProtocolVersion = protocolVersion
	}

	// Latest block
	if blockNum, err := s.rpc.BlockNumber(ctx); err == nil {
		info.LatestBlock = blockNum
	}

	// Gas price (eth_gasPrice)
	var gasPriceHex string
	if err := raw.CallContext(ctx, &gasPriceHex, "eth_gasPrice"); err == nil {
		if gp, ok := new(big.Int).SetString(gasPriceHex, 0); ok {
			info.GasPrice = gp.String()
		}
	}

	// Peer count (net_peerCount)
	var peerCountHex string
	if err := raw.CallContext(ctx, &peerCountHex, "net_peerCount"); err == nil {
		if pc, ok := new(big.Int).SetString(peerCountHex, 0); ok {
			info.PeerCount = int(pc.Int64())
		}
	}

	// Syncing (eth_syncing) — returns false or an object
	var syncing any
	if err := raw.CallContext(ctx, &syncing, "eth_syncing"); err == nil {
		if b, ok := syncing.(bool); ok {
			info.IsSyncing = b
		} else {
			info.IsSyncing = true // non-false means syncing
		}
	}

	// Genesis block hash
	var genesisBlock struct {
		Hash string `json:"hash"`
	}
	if err := raw.CallContext(ctx, &genesisBlock, "eth_getBlockByNumber", "0x0", false); err == nil {
		info.GenesisHash = genesisBlock.Hash
	}

	s.mu.Lock()
	s.cached = info
	s.mu.Unlock()

	log.Info("chain info refreshed",
		"chainId", info.ChainIDDecimal,
		"client", info.ClientVersion,
		"block", info.LatestBlock,
	)
}
