-- Missing block ranges table for tracking gaps in indexed blocks
-- This follows Blockscout's pattern for identifying and backfilling missing blocks

CREATE TABLE IF NOT EXISTS missing_block_ranges (
    id SERIAL PRIMARY KEY,
    from_number BIGINT NOT NULL,
    to_number BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT valid_range CHECK (from_number <= to_number)
);

-- Index for efficient range queries
CREATE INDEX IF NOT EXISTS idx_missing_ranges_from ON missing_block_ranges(from_number);
CREATE INDEX IF NOT EXISTS idx_missing_ranges_to ON missing_block_ranges(to_number);

-- Track the boundaries of our scanning progress
CREATE TABLE IF NOT EXISTS indexer_progress (
    id SERIAL PRIMARY KEY,
    -- Minimum block number we've scanned (scanning backward toward genesis)
    min_fetched_block BIGINT DEFAULT 0,
    -- Maximum block number we've scanned (scanning forward toward head)
    max_fetched_block BIGINT DEFAULT 0,
    -- Whether initial backward scan is complete
    backfill_complete BOOLEAN DEFAULT false,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Insert initial progress record
INSERT INTO indexer_progress (min_fetched_block, max_fetched_block, backfill_complete)
VALUES (0, 0, false) ON CONFLICT DO NOTHING;
