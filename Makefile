include version.mk

DOCKER_REGISTRY ?= gatewayfm
IMAGE_PREFIX ?= block-explorer

.PHONY: run run-privacy stop destroy logs anvil anvil-privacy anvil-stop anvil-destroy anvil-logs
.PHONY: version docker-build docker-build-api docker-build-indexer docker-build-frontend
.PHONY: lint test build

# Default RPC URL (use host.docker.internal to reach host from Docker)
RPC_URL ?= http://privacy-proxy-anvil-1:8545
START_BLOCK ?= 0

# Port configuration
API_PORT ?= 8081
FRONTEND_PORT ?= 3001
POSTGRES_PORT ?= 5433
ANVIL_PORT ?= 8546

export API_PORT FRONTEND_PORT POSTGRES_PORT ANVIL_PORT

run:
	@echo "Starting Block Explorer..."
	@echo "RPC URL: $(RPC_URL)"
	@echo ""
	docker compose build
	RPC_URL=$(RPC_URL) START_BLOCK=$(START_BLOCK) docker compose up -d
	@echo ""
	@echo "Waiting for services..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://127.0.0.1:$(API_PORT)/api/stats >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 2; \
	done
	@echo ""
	@echo "Block Explorer is ready!"
	@echo ""
	@echo "  Explorer:  http://localhost:$(FRONTEND_PORT) (vite dev)"
	@echo "  API:       http://localhost:$(API_PORT)"
	@echo ""

run-privacy:
	@echo "Starting Block Explorer with Privacy enabled..."
	@echo "RPC URL: $(RPC_URL)"
	@echo ""
	docker compose build
	PRIVACY_ENABLED=true RPC_URL=$(RPC_URL) START_BLOCK=$(START_BLOCK) docker compose up -d
	@echo ""
	@echo "Waiting for services..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://127.0.0.1:$(API_PORT)/api/stats >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 2; \
	done
	@echo ""
	@echo "Block Explorer is ready! (Privacy ENABLED)"
	@echo ""
	@echo "  Explorer:  http://localhost:$(FRONTEND_PORT) (vite dev)"
	@echo "  API:       http://localhost:$(API_PORT)"
	@echo ""

stop:
	@echo "Stopping Block Explorer..."
	docker compose down -v --remove-orphans
	@echo "Done"

destroy:
	@echo "Destroying Block Explorer (containers, volumes, and images)..."
	docker compose down -v --remove-orphans --rmi local
	@echo "Done"

logs:
	docker compose logs -f

# Anvil (local testnet) commands
anvil:
	@echo "Starting Block Explorer with Anvil (local testnet)..."
	@echo ""
	docker compose -f docker-compose.anvil.yml build
	docker compose -f docker-compose.anvil.yml up -d
	@echo ""
	@echo "Waiting for services..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://127.0.0.1:$(API_PORT)/api/stats >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 2; \
	done
	@echo ""
	@echo "Block Explorer with Anvil is ready!"
	@echo ""
	@echo "  Explorer:  http://localhost:$(FRONTEND_PORT) (vite dev)"
	@echo "  API:       http://localhost:$(API_PORT)"
	@echo "  Anvil RPC: http://localhost:$(ANVIL_PORT)"
	@echo ""
	@echo "Test accounts (each with 10000 ETH):"
	@echo "  0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	@echo "  Private Key: 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	@echo ""

anvil-privacy:
	@echo "Starting Block Explorer with Anvil + Privacy enabled..."
	@echo ""
	docker compose -f docker-compose.anvil.yml build
	PRIVACY_ENABLED=true docker compose -f docker-compose.anvil.yml up -d
	@echo ""
	@echo "Waiting for services..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://127.0.0.1:$(API_PORT)/api/stats >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 2; \
	done
	@echo ""
	@echo "Block Explorer with Anvil is ready! (Privacy ENABLED)"
	@echo ""
	@echo "  Explorer:  http://localhost:$(FRONTEND_PORT) (vite dev)"
	@echo "  API:       http://localhost:$(API_PORT)"
	@echo "  Anvil RPC: http://localhost:$(ANVIL_PORT)"
	@echo ""
	@echo "Test accounts (each with 10000 ETH):"
	@echo "  0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	@echo "  Private Key: 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	@echo ""

anvil-stop:
	@echo "Stopping Anvil stack..."
	docker compose -f docker-compose.anvil.yml down -v --remove-orphans
	@echo "Done"

anvil-destroy:
	@echo "Destroying Anvil stack (containers, volumes, and images)..."
	docker compose -f docker-compose.anvil.yml down -v --remove-orphans --rmi local
	@echo "Done"

anvil-logs:
	docker compose -f docker-compose.anvil.yml logs -f

rebuild-backend:
	docker compose build --no-cache api indexer && docker compose up -d api indexer

anvil-rebuild-backend:
	docker compose -f docker-compose.anvil.yml build --no-cache api indexer && docker compose -f docker-compose.anvil.yml up -d api indexer

# =============================================================================
# Version
# =============================================================================

version:
	@echo "Version:    $(VERSION)"
	@echo "Git Rev:    $(GITREV)"
	@echo "Git Branch: $(GITBRANCH)"
	@echo "Build Date: $(DATE)"

# =============================================================================
# CI / Quality
# =============================================================================

lint:
	@echo "--- Backend ---"
	cd backend && go vet ./...
	@echo "--- Frontend ---"
	cd frontend && npx eslint .
	cd frontend && npx tsc -b

test:
	@echo "--- Backend ---"
	cd backend && go test -race -count=1 ./...

build:
	@echo "--- Backend ---"
	cd backend && CGO_ENABLED=0 go build -o /dev/null ./cmd/api
	cd backend && CGO_ENABLED=0 go build -o /dev/null ./cmd/indexer
	@echo "--- Frontend ---"
	cd frontend && npm run build

# =============================================================================
# Docker Builds
# =============================================================================

docker-build: docker-build-api docker-build-indexer docker-build-frontend

docker-build-api:
	@echo "Building $(DOCKER_REGISTRY)/$(IMAGE_PREFIX)-api:$(VERSION)"
	docker build -f backend/Dockerfile.api -t $(DOCKER_REGISTRY)/$(IMAGE_PREFIX)-api:$(VERSION) backend/

docker-build-indexer:
	@echo "Building $(DOCKER_REGISTRY)/$(IMAGE_PREFIX)-indexer:$(VERSION)"
	docker build -f backend/Dockerfile.indexer -t $(DOCKER_REGISTRY)/$(IMAGE_PREFIX)-indexer:$(VERSION) backend/

docker-build-frontend:
	@echo "Building $(DOCKER_REGISTRY)/$(IMAGE_PREFIX)-frontend:$(VERSION)"
	docker build -f frontend/Dockerfile -t $(DOCKER_REGISTRY)/$(IMAGE_PREFIX)-frontend:$(VERSION) frontend/

docker-build-dry-run:
	@echo "Docker build dry run (no push)..."
	@echo ""
	@$(MAKE) docker-build-api
	@echo ""
	@$(MAKE) docker-build-indexer
	@echo ""
	@$(MAKE) docker-build-frontend
	@echo ""
	@echo "All images built successfully."
