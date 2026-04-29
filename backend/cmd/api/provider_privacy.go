//go:build privacy

package main

// Privacy-mode build variant. Supports ONLY PRIVACY_PROXY_URL — reads
// routed through privacy-proxy's REST explorer API, which applies
// RBAC-based redaction before returning.
//
// The indexerclient package (chain-indexer gRPC client) is deliberately
// not imported in this file. Built with `go build -tags privacy`, the
// resulting binary does not contain indexerclient code, so no runtime
// misconfiguration or env-var injection can cause block-explorer-api
// to read chain data direct from chain-indexer and bypass the
// privacy-proxy redaction layer. This is the compile-time complement
// to the mutual-exclusion env check in provider_standalone.go — part
// of the three-layer defense (compile-time + runtime + network) the
// RD-855 trust model assumes.

import (
	"explorer/internal/api"
	"explorer/internal/auth"
	"explorer/internal/config"
	"explorer/internal/db"
	"explorer/internal/log"
	"explorer/internal/privacy"
	"explorer/internal/rpc"
)

func chooseProvider(cfg *config.Config, _ *db.DB, _ *rpc.Client) (*privacy.Client, *auth.SSOClient, api.DataProvider) {
	if cfg.IndexerURL != "" {
		log.Fatal("INDEXER_URL is set but this is the privacy-tagged build of block-explorer-api — only PRIVACY_PROXY_URL is supported. Unset INDEXER_URL, or run the default (non-privacy) build if you need chain-indexer direct access.")
	}
	if cfg.PrivacyProxyURL == "" {
		log.Fatal("PRIVACY_PROXY_URL is not set — privacy-tagged block-explorer-api requires reads to be routed through privacy-proxy's REST explorer API.")
	}

	privacyClient := privacy.NewClient(cfg.PrivacyProxyURL)
	ssoClient := auth.NewSSOClient(cfg.PrivacyProxyURL, cfg.PrivacyProxyPublicURL, cfg.SSOClientID, cfg.SSORedirectURI)
	dataProvider := api.NewProxyDataProvider(cfg.PrivacyProxyURL)
	log.Info("privacy build: reads routed through privacy-proxy", "proxy_url", cfg.PrivacyProxyURL)
	return privacyClient, ssoClient, dataProvider
}
