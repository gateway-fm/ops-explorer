# Third-Party Notices

This is the source-only third-party notice starter for the initial open-source
release. It lists notable dependencies and provenance decisions; it is NOT a
complete SBOM. Before any official binary or container image release, generate
scanner-backed SBOMs and update this file (see "Required Follow-Up").

## Provenance of In-Repo Code

- `backend/proto/chain_indexer/v1` and `backend/gen/go/chain_indexer/v1` —
  protocol definitions and protoc-generated Go code for the chain-indexer gRPC
  API. The source of truth is the gateway-fm/chain-indexer repository, expected
  to be published under Apache-2.0 as part of the same coordinated release.
- `backend/pkg/eth` — original, minimal, API-compatible Ethereum utility
  packages written for this project (address/hash types, EIP-55 checksum,
  hex utilities, a reflect-based RLP encoder, a JSON-RPC client). These
  implement public specifications; they are not derived from
  github.com/ethereum/go-ethereum, which is neither vendored nor present
  anywhere in this module's dependency graph (verified with `go list -m all`).
- `contracts/src` and `backend/internal/verifier/testdata` — project-owned
  example/test Solidity contracts, SPDX-tagged MIT.

## Notable Go Dependencies (direct)

All direct Go dependencies are Apache-2.0-compatible: permissive licenses
throughout, except hashicorp/golang-lru which is MPL-2.0 (a weak file-level
copyleft, satisfied here by using it as an unmodified library):

- github.com/go-chi/chi/v5 — MIT
- github.com/golang-jwt/jwt/v5 — MIT
- github.com/gorilla/websocket — BSD-2-Clause
- github.com/hashicorp/golang-lru/v2 — MPL-2.0 (file-level copyleft; used as an
  unmodified library dependency)
- github.com/jackc/pgx/v5, github.com/jackc/tern/v2 — MIT
- github.com/prometheus/client_golang — Apache-2.0
- github.com/rs/cors — MIT
- github.com/spf13/viper — MIT
- github.com/stretchr/testify — MIT
- golang.org/x/crypto, golang.org/x/sync, golang.org/x/time — BSD-3-Clause
- google.golang.org/grpc — Apache-2.0
- google.golang.org/protobuf — BSD-3-Clause

No GPL, LGPL, or AGPL dependencies are present in the Go module graph.

## Notable npm Dependencies

Frontend runtime dependencies are under permissive licenses (MIT/ISC/BSD/
Apache-2.0), including react, react-dom, ethers, viem, @tanstack/react-query,
@radix-ui/react-tooltip, recharts, lucide-react, and tailwind tooling. Dev
dependencies (vite, vitest, typescript, eslint, msw, testing-library,
playwright in e2e/) are likewise permissive. A scanner-backed license sweep is
part of the pre-binary-release follow-up.

## Build-Time and Image-Only Components (binary/image release review)

These are NOT distributed with the source repository, but are downloaded or
installed when building the container images. They matter for official image
releases, not for source publication:

- solc (Solidity compiler) static binaries — GPL-3.0. Downloaded (checksum-
  pinned) into the verification-enabled backend image (backend/Dockerfile)
  and invoked as a separate process (not linked). That image is
  local-build-only: the images published to Docker Hub (api, api-privacy,
  public-api, frontend) are built from Dockerfile.api / Dockerfile.public-api
  / frontend/Dockerfile and contain no solc or sol2uml (verified against
  gatewayfm/block-explorer-api:0.8.5). Distributing an image that bundles
  solc would require GPL-3.0 distribution compliance (license text + source
  availability for solc itself; mere aggregation — it does not affect this
  repository's Apache-2.0 licensing).
- sol2uml (npm) — MIT. Installed into the backend image for the contract UML
  diagram feature.
- Base images (golang:alpine builders, distroless/static-debian13, nginx for
  the frontend) — carry their own licenses and notices; include them in image
  SBOMs.

## Gateway Repositories In The Coordinated Release

- gateway-fm/block-explorer — this repository.
- gateway-fm/chain-indexer — source of the vendored proto definitions;
  required by standalone mode; expected Apache-2.0.
- gateway-fm/privacy-proxy — optional privacy-mode upstream; expected
  Apache-2.0.

## Required Follow-Up (before first official binary/image release)

- Generate complete SBOMs for source, binaries, and container images.
- Run Go/npm/container license scanners and replace the starter lists above
  with scanner-backed data.
- Resolve solc GPL-3.0 image-distribution obligations (or ship a
  no-verification image variant without solc).
- Publish source-to-image mapping and signed checksums/provenance for each
  image tag.
