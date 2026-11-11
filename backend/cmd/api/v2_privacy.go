//go:build privacy

package main

import (
	"context"
	"explorer/internal/api"
	"explorer/internal/authv2"
	"explorer/pkg/log"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

func startV2IfSelected() (bool, error) {
	protocol := os.Getenv("EXPLORER_AUTH_PROTOCOL")
	if protocol == "" {
		return false, nil
	}
	if protocol != "ops-v2" {
		return true, authv2.ErrInvalid
	}
	// Read this mode before legacy config.Load: no default owner DSN, RPC_URL,
	// V1 refresh/JWKS credentials, .env autoloading or raw network poller exists.
	if os.Getenv("EXPLORER_SSO_OWNER_DSN") != "" || os.Getenv("DATABASE_URL") != "" || os.Getenv("INDEXER_URL") != "" {
		return true, authv2.ErrInvalid
	}
	sha, e := authv2.ExpectedSHA(os.Getenv("EXPLORER_SSO_SOURCE_SHA256"))
	if e != nil {
		return true, e
	}
	file, e := os.Open(os.Getenv("EXPLORER_SSO_SOURCE"))
	if e != nil {
		return true, authv2.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	_ = file.Close()
	if e != nil || len(raw) > 64<<10 {
		return true, authv2.ErrInvalid
	}
	source, e := authv2.ParseSource(raw, sha)
	if e != nil {
		return true, e
	}
	bindings, e := authv2.EnvironmentBindings(source)
	if e != nil {
		return true, e
	}
	port := 8080
	if text := os.Getenv("API_PORT"); text != "" {
		port, e = strconv.Atoi(text)
		if e != nil {
			return true, authv2.ErrInvalid
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	session, e := authv2.New(ctx, source, bindings, os.Getenv("EXPLORER_SSO_APP_DSN"), nil)
	if e != nil {
		return true, e
	}
	defer session.Close()
	provider := api.NewV2ReadClient(session)
	server, e := api.NewV2(session, provider, port)
	if e != nil {
		return true, e
	}
	log.RedactHTTPPaths = true
	log.Info("starting v2 API", "port", port)
	e = server.Start(ctx)
	if e == http.ErrServerClosed {
		e = nil
	}
	return true, e
}
