//go:build privacy

package api

import "github.com/go-chi/chi/v5"

// registerVerificationAPI is a no-op in privacy builds. The standalone
// implementation in verification_routes_standalone.go registers Etherscan-
// compatible + native /verify routes; in privacy mode neither surface is
// exposed because:
//
//  1. The corresponding /api/v1/explorer/contracts/verify endpoint isn't
//     implemented on privacy-proxy, so the BFF would have nowhere to forward
//     to anyway.
//  2. The "who can verify" RBAC story isn't designed yet — letting the call
//     reach the network surface invites a future caller from the wrong
//     authority bracket. Compile it out instead of relying on a runtime
//     guard.
func (s *Server) registerVerificationAPI(_ chi.Router) {}

// registerSourcifyAddressRoutes is also a no-op in privacy builds —
// Sourcify integration is a verification path under a different name and
// faces the same disabling rationale. See the standalone implementation
// for the live registrations.
func (s *Server) registerSourcifyAddressRoutes(_ chi.Router) {}
