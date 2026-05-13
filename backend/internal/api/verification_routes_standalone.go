//go:build !privacy

package api

import "github.com/go-chi/chi/v5"

// registerVerificationAPI wires up the contract-verification surfaces that the
// standalone block-explorer exposes:
//
//   - The Etherscan-compatible RPC entry-point at the API root, used by
//     `hardhat verify` and `forge verify-contract`.
//   - The native `/verify/*` endpoints (POST submit, POST standard-JSON,
//     GET compiler list).
//
// In privacy mode the matching `verification_routes_privacy.go` defines this
// as a no-op so none of these surfaces are exposed — see RD-XXX (block-
// explorer side of the rc1 verification disable). The privacy proxy does not
// implement the verification endpoint either, so a user clicking "Verify &
// Publish" in a privacy-mode dashboard would 404 today; the cleaner answer is
// to compile the surface out entirely.
func (s *Server) registerVerificationAPI(api chi.Router) {
	api.HandleFunc("/", s.handleEtherscanRPC)

	api.Route("/verify", func(r chi.Router) {
		r.Post("/", s.handleVerifyContract)
		r.Post("/standard-json", s.handleVerifyStandardJSON)
		r.Get("/compilers", s.handleListCompilers)
	})
}

// registerSourcifyAddressRoutes adds the Sourcify lookup endpoints inside the
// `/addresses/{address}` group. Sourcify pulls verified source from the
// public Sourcify repository and stores it locally — same threat shape as
// in-tree verification (publishing source on someone else's contract), and
// disabled in privacy mode for the same reason.
func (s *Server) registerSourcifyAddressRoutes(addrGroup chi.Router) {
	addrGroup.Get("/sourcify", s.handleFetchSourcify)
	addrGroup.Get("/sourcify/check", s.handleCheckSourcify)
}
