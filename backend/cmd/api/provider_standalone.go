//go:build !privacy

package main

// Default (standalone) build variant. Supports both INDEXER_URL
// (chain-indexer gRPC) and PRIVACY_PROXY_URL (privacy-proxy REST) modes.
// See provider_privacy.go for the locked-down variant that only
// supports PRIVACY_PROXY_URL.

import (
	"errors"

	"explorer/internal/api"
	"explorer/internal/api/cache"
	"explorer/internal/api/indexerclient"
	"explorer/internal/auth"
	"explorer/internal/config"
	"explorer/internal/db"
	"explorer/internal/privacy"
	"explorer/internal/rpc"
	"explorer/pkg/log"
)

// Chain-data modes selected by selectMode. Exactly one of INDEXER_URL /
// PRIVACY_PROXY_URL picks one of these; see selectMode for the contract.
const (
	modePrivacy    = "privacy"
	modeStandalone = "standalone"
)

// Sentinel errors returned by selectMode (kept as values so the pure function
// is unit-testable; chooseProvider turns them back into the original
// startup-fatal messages).
var (
	errBothModesSet    = errors.New("both INDEXER_URL and PRIVACY_PROXY_URL are set — these select mutually exclusive chain-data modes. Set INDEXER_URL for standalone mode (chain-indexer gRPC, raw data) OR PRIVACY_PROXY_URL for privacy mode (reads routed through privacy-proxy, redaction applied). Unset one of them.")
	errNoModeSet       = errors.New("neither INDEXER_URL nor PRIVACY_PROXY_URL is set — block-explorer needs a chain-data source. Set INDEXER_URL for standalone mode (chain-indexer gRPC) or PRIVACY_PROXY_URL for privacy mode (reads through privacy-proxy, redaction applied). Setting both is rejected.")
)

// selectMode resolves the chain-data mode from config WITHOUT side effects.
//
// Block-explorer no longer runs its own indexer (RD-855 Phase 6). A chain-data
// source is required — pick exactly one:
//
//	INDEXER_URL        standalone mode: reads go to chain-indexer over gRPC;
//	                   raw chain data, no redaction.
//	PRIVACY_PROXY_URL  privacy/proxy mode: reads go to privacy-proxy's REST
//	                   explorer API, which applies RBAC-based redaction.
//
// The two modes are mutually exclusive. Setting both would silently route chain
// data around privacy-proxy's redaction layer while still wiring SSO through it
// — a privacy footgun — so it is rejected (errBothModesSet). Setting neither is
// also rejected (errNoModeSet). Returning errors (rather than log.Fatal) keeps
// this unit-testable; chooseProvider is the sole caller and log.Fatals on error
// so startup behavior is unchanged.
func selectMode(cfg *config.Config) (string, error) {
	switch {
	case cfg.PrivacyProxyURL != "" && cfg.IndexerURL != "":
		return "", errBothModesSet
	case cfg.PrivacyProxyURL != "":
		return modePrivacy, nil
	case cfg.IndexerURL != "":
		return modeStandalone, nil
	default:
		return "", errNoModeSet
	}
}

func chooseProvider(cfg *config.Config, database *db.DB, rpcClient *rpc.Client) (*privacy.Client, *auth.SSOClient, api.DataProvider) {
	mode, err := selectMode(cfg)
	if err != nil {
		log.Fatal(err.Error())
		return nil, nil, nil // unreachable
	}

	switch mode {
	case modePrivacy:
		// Do NOT wrap with cache here — privacy-proxy responses are
		// auth-scoped per caller and a shared cache would leak across users.
		privacyClient := privacy.NewClient(cfg.PrivacyProxyURL)
		ssoClient := auth.NewSSOClient(cfg.PrivacyProxyURL, cfg.PrivacyProxyPublicURL, cfg.SSOClientID, cfg.SSOClientSecret, cfg.SSORedirectURI)
		dataProvider := api.NewProxyDataProvider(cfg.PrivacyProxyURL)
		log.Info("privacy/proxy mode enabled", "proxy_url", cfg.PrivacyProxyURL)
		return privacyClient, ssoClient, dataProvider

	default: // modeStandalone
		fallback := api.NewDirectDBProvider(database, rpcClient, nil)
		ip, err := indexerclient.New(indexerclient.Config{IndexerURL: cfg.IndexerURL}, fallback)
		if err != nil {
			log.Fatal("failed to construct indexerclient provider", "error", err)
		}
		var dataProvider api.DataProvider = ip
		dataProvider = wrapWithCache(dataProvider, cfg)
		log.Info("indexer-backed standalone mode", "indexer_url", cfg.IndexerURL)
		return nil, nil, dataProvider
	}
}

func wrapWithCache(inner api.DataProvider, cfg *config.Config) api.DataProvider {
	if !cfg.EnableProviderCache {
		log.Info("provider cache disabled (ENABLE_PROVIDER_CACHE=false)")
		return inner
	}
	log.Info("provider cache enabled")
	return cache.NewProvider(inner)
}
