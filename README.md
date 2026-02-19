<p align="center">
  <img src="frontend/public/mascot.png" alt="Block Explorer" width="180" />
</p>

<h1 align="center">Block Explorer</h1>

<p align="center">
  A lightweight, self-hosted block explorer for EVM-compatible chains.<br/>
  Point it at any RPC endpoint and get a fully indexed, searchable explorer with real-time updates.
</p>

<p align="center">
  <a href="https://github.com/gateway-fm/privacy-block-explorer/actions/workflows/ci.yml"><img src="https://github.com/gateway-fm/privacy-block-explorer/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
</p>

---

## Quickstart (Anvil)

Spins up a local Anvil testnet, the indexer, API, and frontend in one command:

```bash
make anvil
```

Open [http://localhost:3001](http://localhost:3001).

### Stop / Destroy

```bash
make anvil-stop     # stop containers
make anvil-destroy  # stop and remove volumes
```

### Logs

```bash
make anvil-logs
```

## External RPC

```bash
RPC_URL=https://rpc.example.com make run
```

```bash
make stop   # stop
make logs   # tail logs
```

## Endpoints

| Service | URL |
|---------|-----|
| Explorer | [localhost:3001](http://localhost:3001) |
| API | [localhost:8081](http://localhost:8081) |
| Anvil RPC | [localhost:8546](http://localhost:8546) |
