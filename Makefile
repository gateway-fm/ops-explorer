include version.mk

DOCKER_REGISTRY ?= gatewayfm
IMAGE_PREFIX ?= block-explorer

.PHONY: dev dev-stop dev-destroy dev-logs dev-rebuild-backend
.PHONY: run run-privacy stop destroy logs rebuild-backend
.PHONY: version docker-build docker-build-api docker-build-indexer docker-build-frontend docker-build-dry-run
.PHONY: lint test build clean clean-build

# Default RPC URL (use host.docker.internal to reach host from Docker)
RPC_URL ?= http://privacy-proxy-anvil-1:8545
START_BLOCK ?= 0

# Port configuration
API_PORT ?= 8081
FRONTEND_PORT ?= 3001
POSTGRES_PORT ?= 5433
ANVIL_PORT ?= 8546

export API_PORT FRONTEND_PORT POSTGRES_PORT ANVIL_PORT

# =============================================================================
# Dev Environment (Anvil local testnet)
# =============================================================================

dev:
	@echo "Starting Block Explorer (dev mode with Anvil)..."
	@echo ""
	docker compose -f docker-compose.dev.yml build
	docker compose -f docker-compose.dev.yml up -d
	@echo ""
	@echo "Waiting for services..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://127.0.0.1:$(API_PORT)/api/stats >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 2; \
	done
	@echo ""
	@echo "Block Explorer (dev) is ready!"
	@echo ""
	@echo "  Explorer:  http://localhost:$(FRONTEND_PORT)"
	@echo "  API:       http://localhost:$(API_PORT)"
	@echo "  Anvil RPC: http://localhost:$(ANVIL_PORT)"
	@echo ""
	@echo "Test accounts (each with 10000 ETH):"
	@echo "  0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	@echo "  Private Key: 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	@echo ""

dev-stop:
	@echo "Stopping dev stack..."
	docker compose -f docker-compose.dev.yml down --remove-orphans
	@echo "Done"

dev-destroy:
	@echo "Destroying dev stack (containers, volumes, and images)..."
	docker compose -f docker-compose.dev.yml down -v --remove-orphans --rmi local
	@echo "Done"

dev-logs:
	docker compose -f docker-compose.dev.yml logs -f

dev-rebuild-backend:
	docker compose -f docker-compose.dev.yml build --no-cache api indexer && docker compose -f docker-compose.dev.yml up -d api indexer

# =============================================================================
# Production (external RPC)
# =============================================================================

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
	@echo "  Explorer:  http://localhost:$(FRONTEND_PORT)"
	@echo "  API:       http://localhost:$(API_PORT)"
	@echo ""

run-privacy:
	@echo "Starting Block Explorer with Privacy Proxy enabled..."
	@echo "  API  RPC: privacy-proxy-backend:8080 (proxy)"
	@echo "  Indexer RPC: privacy-proxy-anvil:8545 (direct, for indexing)"
	@echo ""
	docker compose build
	START_BLOCK=$(START_BLOCK) docker compose up -d
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
	@echo "  Explorer:  http://localhost:$(FRONTEND_PORT)"
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

rebuild-backend:
	docker compose build --no-cache api indexer && docker compose up -d api indexer

# Clean Docker environment (stop services, remove volumes)
clean:
	docker compose down -v --remove-orphans
	docker compose -f docker-compose.dev.yml down -v --remove-orphans
	docker system prune -f

# Clean build artifacts
clean-build:
	cd backend && go clean
	rm -rf frontend/.next frontend/node_modules

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
