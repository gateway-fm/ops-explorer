-- Block-explorer's own schema. Only one table: contract verification metadata.
-- Chain data (blocks, transactions, logs, etc.) lives in chain-indexer's
-- postgres and is read over gRPC. See RD-855 Phase 6.

CREATE TABLE IF NOT EXISTS contract_verifications (
    address           TEXT PRIMARY KEY,
    contract_name     TEXT,
    compiler_version  TEXT,
    optimization_used BOOLEAN NOT NULL DEFAULT FALSE,
    optimization_runs INTEGER,
    evm_version       TEXT,
    license_type      TEXT,
    source_code       TEXT,
    abi               JSONB,
    constructor_args  TEXT,
    verified_at       TIMESTAMP NOT NULL DEFAULT NOW()
);

---- create above / drop below ----

DROP TABLE IF EXISTS contract_verifications;
