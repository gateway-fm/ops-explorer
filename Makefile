.PHONY: run stop logs anvil anvil-stop anvil-logs

# Default RPC URL (use host.docker.internal to reach host from Docker)
RPC_URL ?= http://host.docker.internal:8123
START_BLOCK ?= 0

run:
	@echo "Starting Block Explorer..."
	@echo "RPC URL: $(RPC_URL)"
	@echo ""
	docker compose build
	RPC_URL=$(RPC_URL) START_BLOCK=$(START_BLOCK) docker compose up -d
	@echo ""
	@echo "Waiting for services..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s http://127.0.0.1:8080/api/stats >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 2; \
	done
	@echo ""
	@echo "Block Explorer is ready!"
	@echo ""
	@echo "  Explorer:  http://localhost:3000"
	@echo "  API:       http://localhost:8080"
	@echo ""

stop:
	@echo "Stopping Block Explorer..."
	docker compose down -v --remove-orphans
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
		if curl -s http://127.0.0.1:8080/api/stats >/dev/null 2>&1; then \
			break; \
		fi; \
		sleep 2; \
	done
	@echo ""
	@echo "Block Explorer with Anvil is ready!"
	@echo ""
	@echo "  Explorer:  http://localhost:3000"
	@echo "  API:       http://localhost:8080"
	@echo "  Anvil RPC: http://localhost:8545"
	@echo ""
	@echo "Test accounts (each with 10000 ETH):"
	@echo "  0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	@echo "  Private Key: 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	@echo ""

anvil-stop:
	@echo "Stopping Anvil stack..."
	docker compose -f docker-compose.anvil.yml down -v --remove-orphans
	@echo "Done"

anvil-logs:
	docker compose -f docker-compose.anvil.yml logs -f
