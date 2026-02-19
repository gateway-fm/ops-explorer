-- OP Stack deposit transaction L1 metadata
CREATE TABLE IF NOT EXISTS op_deposits (
    l2_tx_hash TEXT PRIMARY KEY REFERENCES transactions(hash) ON DELETE CASCADE,
    l1_block_number BIGINT NOT NULL,
    l1_block_timestamp BIGINT,
    l1_tx_hash TEXT NOT NULL,
    l1_tx_origin TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_op_deposits_l1_block ON op_deposits(l1_block_number DESC);
CREATE INDEX IF NOT EXISTS idx_op_deposits_l1_tx ON op_deposits(l1_tx_hash);
