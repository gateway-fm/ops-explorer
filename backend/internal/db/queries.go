package db

import (
	"context"
	"encoding/json"
	"fmt"

	"explorer/internal/types"

	"github.com/jackc/pgx/v5"
)

// Block operations

func (d *DB) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	var num *uint64
	err := d.pool.QueryRow(ctx, "SELECT MAX(number) FROM blocks").Scan(&num)
	if err != nil || num == nil {
		return 0, err
	}
	return *num, nil
}

func (d *DB) InsertBlock(ctx context.Context, b *types.Block) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO blocks (number, hash, parent_hash, timestamp, gas_used, gas_limit, base_fee_per_gas, transaction_count,
			size, difficulty, total_difficulty, nonce, miner, extra_data, state_root, transactions_root, receipts_root)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (number) DO NOTHING`,
		b.Number, b.Hash, b.ParentHash, b.Timestamp, b.GasUsed, b.GasLimit, b.BaseFeePerGas, b.TransactionCount,
		b.Size, b.Difficulty, b.TotalDifficulty, b.Nonce, b.Miner, b.ExtraData, b.StateRoot, b.TransactionsRoot, b.ReceiptsRoot)
	return err
}

func (d *DB) GetBlock(ctx context.Context, number uint64) (*types.Block, error) {
	var b types.Block
	err := d.pool.QueryRow(ctx, `
		SELECT number, hash, parent_hash, timestamp, gas_used, gas_limit, base_fee_per_gas, transaction_count,
			size, difficulty, total_difficulty, nonce, miner, extra_data, state_root, transactions_root, receipts_root, created_at
		FROM blocks WHERE number = $1`, number).Scan(
		&b.Number, &b.Hash, &b.ParentHash, &b.Timestamp, &b.GasUsed, &b.GasLimit, &b.BaseFeePerGas, &b.TransactionCount,
		&b.Size, &b.Difficulty, &b.TotalDifficulty, &b.Nonce, &b.Miner, &b.ExtraData, &b.StateRoot, &b.TransactionsRoot, &b.ReceiptsRoot, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (d *DB) GetBlockByHash(ctx context.Context, hash string) (*types.Block, error) {
	var b types.Block
	err := d.pool.QueryRow(ctx, `
		SELECT number, hash, parent_hash, timestamp, gas_used, gas_limit, base_fee_per_gas, transaction_count,
			size, difficulty, total_difficulty, nonce, miner, extra_data, state_root, transactions_root, receipts_root, created_at
		FROM blocks WHERE hash = $1`, hash).Scan(
		&b.Number, &b.Hash, &b.ParentHash, &b.Timestamp, &b.GasUsed, &b.GasLimit, &b.BaseFeePerGas, &b.TransactionCount,
		&b.Size, &b.Difficulty, &b.TotalDifficulty, &b.Nonce, &b.Miner, &b.ExtraData, &b.StateRoot, &b.TransactionsRoot, &b.ReceiptsRoot, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &b, err
}

func (d *DB) GetBlocks(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Block, error) {
	var rows pgx.Rows
	var err error

	if beforeBlock != nil {
		rows, err = d.pool.Query(ctx, `
			SELECT number, hash, parent_hash, timestamp, gas_used, gas_limit, base_fee_per_gas, transaction_count,
				size, difficulty, total_difficulty, nonce, miner, extra_data, state_root, transactions_root, receipts_root, created_at
			FROM blocks WHERE number < $1 ORDER BY number DESC LIMIT $2`, *beforeBlock, limit)
	} else {
		rows, err = d.pool.Query(ctx, `
			SELECT number, hash, parent_hash, timestamp, gas_used, gas_limit, base_fee_per_gas, transaction_count,
				size, difficulty, total_difficulty, nonce, miner, extra_data, state_root, transactions_root, receipts_root, created_at
			FROM blocks ORDER BY number DESC LIMIT $1`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []types.Block
	for rows.Next() {
		var b types.Block
		if err := rows.Scan(&b.Number, &b.Hash, &b.ParentHash, &b.Timestamp, &b.GasUsed, &b.GasLimit, &b.BaseFeePerGas, &b.TransactionCount,
			&b.Size, &b.Difficulty, &b.TotalDifficulty, &b.Nonce, &b.Miner, &b.ExtraData, &b.StateRoot, &b.TransactionsRoot, &b.ReceiptsRoot, &b.CreatedAt); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	return blocks, rows.Err()
}

func (d *DB) DeleteBlock(ctx context.Context, number uint64) error {
	_, err := d.pool.Exec(ctx, "DELETE FROM blocks WHERE number = $1", number)
	return err
}

// Transaction operations

func (d *DB) InsertTransaction(ctx context.Context, tx *types.Transaction) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO transactions (hash, block_number, tx_index, from_address, to_address, value, gas_used, gas_price,
			gas_limit, max_fee_per_gas, max_priority_fee_per_gas, nonce, tx_type, input_data, status, error, revert_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (hash) DO NOTHING`,
		tx.Hash, tx.BlockNumber, tx.TxIndex, tx.From, tx.To, tx.Value, tx.GasUsed, tx.GasPrice,
		tx.GasLimit, tx.MaxFeePerGas, tx.MaxPriorityFeePerGas, tx.Nonce, tx.TxType, tx.InputData, tx.Status, tx.Error, tx.RevertReason)
	return err
}

func (d *DB) GetTransaction(ctx context.Context, hash string) (*types.Transaction, error) {
	var tx types.Transaction
	var valueStr string
	err := d.pool.QueryRow(ctx, `
		SELECT t.hash, t.block_number, t.tx_index, t.from_address, t.to_address, c.address, t.value::text,
			t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
			t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at
		FROM transactions t
		LEFT JOIN contracts c ON c.creation_tx = t.hash
		WHERE t.hash = $1`, hash).Scan(
		&tx.Hash, &tx.BlockNumber, &tx.TxIndex, &tx.From, &tx.To, &tx.ContractAddress, &valueStr,
		&tx.GasUsed, &tx.GasPrice, &tx.GasLimit, &tx.MaxFeePerGas, &tx.MaxPriorityFeePerGas, &tx.Nonce,
		&tx.TxType, &tx.InputData, &tx.Status, &tx.Error, &tx.RevertReason, &tx.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tx.Value = types.JSONString(valueStr)
	return &tx, nil
}

func (d *DB) GetTransactions(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	var rows pgx.Rows
	var err error

	if beforeBlock != nil {
		rows, err = d.pool.Query(ctx, `
			SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
				t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
				t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at
			FROM transactions t
			JOIN blocks b ON t.block_number = b.number
			WHERE t.block_number < $1 ORDER BY t.block_number DESC, t.tx_index DESC LIMIT $2`, *beforeBlock, limit)
	} else {
		rows, err = d.pool.Query(ctx, `
			SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
				t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
				t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at
			FROM transactions t
			JOIN blocks b ON t.block_number = b.number
			ORDER BY t.block_number DESC, t.tx_index DESC LIMIT $1`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTransactionsWithTimestamp(rows)
}

func (d *DB) GetTransactionsPaginated(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	var total int64
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := d.pool.Query(ctx, `
		SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
			t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
			t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at
		FROM transactions t
		JOIN blocks b ON t.block_number = b.number
		ORDER BY t.block_number DESC, t.tx_index DESC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	txs, err := scanTransactionsWithTimestamp(rows)
	return txs, total, err
}

func (d *DB) GetTransactionsByBlock(ctx context.Context, blockNumber uint64) ([]types.Transaction, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
			t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
			t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at
		FROM transactions t
		JOIN blocks b ON t.block_number = b.number
		WHERE t.block_number = $1 ORDER BY t.tx_index`, blockNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTransactionsWithTimestamp(rows)
}

func (d *DB) GetTransactionsByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	var rows pgx.Rows
	var err error

	if beforeBlock != nil {
		rows, err = d.pool.Query(ctx, `
			SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
				t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
				t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at
			FROM transactions t
			JOIN blocks b ON t.block_number = b.number
			WHERE (t.from_address = $1 OR t.to_address = $1) AND t.block_number < $2
			ORDER BY t.block_number DESC, t.tx_index DESC LIMIT $3`, address, *beforeBlock, limit)
	} else {
		rows, err = d.pool.Query(ctx, `
			SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
				t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
				t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at
			FROM transactions t
			JOIN blocks b ON t.block_number = b.number
			WHERE t.from_address = $1 OR t.to_address = $1
			ORDER BY t.block_number DESC, t.tx_index DESC LIMIT $2`, address, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTransactionsWithTimestamp(rows)
}

func scanTransactionsWithTimestamp(rows pgx.Rows) ([]types.Transaction, error) {
	var txs []types.Transaction
	for rows.Next() {
		var tx types.Transaction
		var valueStr string
		if err := rows.Scan(&tx.Hash, &tx.BlockNumber, &tx.BlockTimestamp, &tx.TxIndex, &tx.From, &tx.To, &valueStr,
			&tx.GasUsed, &tx.GasPrice, &tx.GasLimit, &tx.MaxFeePerGas, &tx.MaxPriorityFeePerGas, &tx.Nonce,
			&tx.TxType, &tx.InputData, &tx.Status, &tx.Error, &tx.RevertReason, &tx.CreatedAt); err != nil {
			return nil, err
		}
		tx.Value = types.JSONString(valueStr)
		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

// GetTransactionsWithCategories returns transactions with computed categories
func (d *DB) GetTransactionsWithCategories(ctx context.Context, limit int, beforeBlock *uint64) ([]types.Transaction, error) {
	var rows pgx.Rows
	var err error

	query := `
		SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
			t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
			t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at,
			-- Computed categories
			t.value > 0 as is_coin_transfer,
			(t.to_address IS NOT NULL AND LENGTH(t.input_data) > 0 AND EXISTS(SELECT 1 FROM contracts c WHERE LOWER(c.address) = LOWER(t.to_address))) as is_contract_call,
			(t.to_address IS NULL AND EXISTS(SELECT 1 FROM contracts c WHERE c.creation_tx = t.hash)) as is_contract_creation,
			(SELECT COUNT(*) FROM token_transfers tt WHERE tt.tx_hash = t.hash) as token_transfer_count
		FROM transactions t
		JOIN blocks b ON t.block_number = b.number`

	if beforeBlock != nil {
		query += ` WHERE t.block_number < $1 ORDER BY t.block_number DESC, t.tx_index DESC LIMIT $2`
		rows, err = d.pool.Query(ctx, query, *beforeBlock, limit)
	} else {
		query += ` ORDER BY t.block_number DESC, t.tx_index DESC LIMIT $1`
		rows, err = d.pool.Query(ctx, query, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTransactionsWithCategories(rows)
}

// GetTransactionsPaginatedWithCategories returns paginated transactions with computed categories
func (d *DB) GetTransactionsPaginatedWithCategories(ctx context.Context, page, pageSize int) ([]types.Transaction, int64, error) {
	var total int64
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM transactions`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := d.pool.Query(ctx, `
		SELECT t.hash, t.block_number, b.timestamp, t.tx_index, t.from_address, t.to_address, t.value::text,
			t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
			t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at,
			-- Computed categories
			t.value > 0 as is_coin_transfer,
			(t.to_address IS NOT NULL AND LENGTH(t.input_data) > 0 AND EXISTS(SELECT 1 FROM contracts c WHERE LOWER(c.address) = LOWER(t.to_address))) as is_contract_call,
			(t.to_address IS NULL AND EXISTS(SELECT 1 FROM contracts c WHERE c.creation_tx = t.hash)) as is_contract_creation,
			(SELECT COUNT(*) FROM token_transfers tt WHERE tt.tx_hash = t.hash) as token_transfer_count
		FROM transactions t
		JOIN blocks b ON t.block_number = b.number
		ORDER BY t.block_number DESC, t.tx_index DESC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	txs, err := scanTransactionsWithCategories(rows)
	return txs, total, err
}

// GetTransactionWithCategories returns a single transaction with computed categories
func (d *DB) GetTransactionWithCategories(ctx context.Context, hash string) (*types.Transaction, error) {
	var tx types.Transaction
	var valueStr string
	var isCoinTransfer, isContractCall, isContractCreation bool
	var tokenTransferCount int

	err := d.pool.QueryRow(ctx, `
		SELECT t.hash, t.block_number, t.tx_index, t.from_address, t.to_address, c.address, t.value::text,
			t.gas_used, t.gas_price, t.gas_limit, t.max_fee_per_gas, t.max_priority_fee_per_gas, t.nonce,
			t.tx_type, t.input_data, t.status, t.error, t.revert_reason, t.created_at,
			-- Computed categories
			t.value > 0 as is_coin_transfer,
			(t.to_address IS NOT NULL AND LENGTH(t.input_data) > 0 AND EXISTS(SELECT 1 FROM contracts c2 WHERE LOWER(c2.address) = LOWER(t.to_address))) as is_contract_call,
			(t.to_address IS NULL AND c.address IS NOT NULL) as is_contract_creation,
			(SELECT COUNT(*) FROM token_transfers tt WHERE tt.tx_hash = t.hash) as token_transfer_count
		FROM transactions t
		LEFT JOIN contracts c ON c.creation_tx = t.hash
		WHERE t.hash = $1`, hash).Scan(
		&tx.Hash, &tx.BlockNumber, &tx.TxIndex, &tx.From, &tx.To, &tx.ContractAddress, &valueStr,
		&tx.GasUsed, &tx.GasPrice, &tx.GasLimit, &tx.MaxFeePerGas, &tx.MaxPriorityFeePerGas, &tx.Nonce,
		&tx.TxType, &tx.InputData, &tx.Status, &tx.Error, &tx.RevertReason, &tx.CreatedAt,
		&isCoinTransfer, &isContractCall, &isContractCreation, &tokenTransferCount)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tx.Value = types.JSONString(valueStr)

	// Build categories
	tx.TxCategories = buildCategories(isCoinTransfer, isContractCall, isContractCreation, tokenTransferCount)
	tx.TokenTransferCount = tokenTransferCount

	return &tx, nil
}

func scanTransactionsWithCategories(rows pgx.Rows) ([]types.Transaction, error) {
	var txs []types.Transaction
	for rows.Next() {
		var tx types.Transaction
		var valueStr string
		var isCoinTransfer, isContractCall, isContractCreation bool
		var tokenTransferCount int

		if err := rows.Scan(&tx.Hash, &tx.BlockNumber, &tx.BlockTimestamp, &tx.TxIndex, &tx.From, &tx.To, &valueStr,
			&tx.GasUsed, &tx.GasPrice, &tx.GasLimit, &tx.MaxFeePerGas, &tx.MaxPriorityFeePerGas, &tx.Nonce,
			&tx.TxType, &tx.InputData, &tx.Status, &tx.Error, &tx.RevertReason, &tx.CreatedAt,
			&isCoinTransfer, &isContractCall, &isContractCreation, &tokenTransferCount); err != nil {
			return nil, err
		}
		tx.Value = types.JSONString(valueStr)

		// Build categories
		tx.TxCategories = buildCategories(isCoinTransfer, isContractCall, isContractCreation, tokenTransferCount)
		tx.TokenTransferCount = tokenTransferCount

		txs = append(txs, tx)
	}
	return txs, rows.Err()
}

func buildCategories(isCoinTransfer, isContractCall, isContractCreation bool, tokenTransferCount int) []string {
	var categories []string
	if isContractCreation {
		categories = append(categories, types.TxCategoryContractCreation)
	}
	if isContractCall {
		categories = append(categories, types.TxCategoryContractCall)
	}
	if isCoinTransfer {
		categories = append(categories, types.TxCategoryCoinTransfer)
	}
	if tokenTransferCount > 0 {
		categories = append(categories, types.TxCategoryTokenTransfer)
	}
	return categories
}

// Token operations

func (d *DB) InsertToken(ctx context.Context, t *types.Token) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO tokens (address, symbol, name, decimals, token_type, total_supply, block_number, creation_tx, l1_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (address) DO UPDATE SET
			symbol = COALESCE(EXCLUDED.symbol, tokens.symbol),
			name = COALESCE(EXCLUDED.name, tokens.name),
			decimals = COALESCE(EXCLUDED.decimals, tokens.decimals),
			total_supply = COALESCE(EXCLUDED.total_supply, tokens.total_supply)`,
		t.Address, t.Symbol, t.Name, t.Decimals, t.TokenType, t.TotalSupply, t.BlockNumber, t.CreationTx, t.L1Address)
	return err
}

func (d *DB) GetToken(ctx context.Context, address string) (*types.Token, error) {
	var t types.Token
	err := d.pool.QueryRow(ctx, `
		SELECT address, symbol, name, decimals, token_type, total_supply, holder_count, transfer_count,
			usd_price, icon_url, l1_address, block_number, creation_tx, off_chain_updated_at, created_at
		FROM tokens WHERE LOWER(address) = LOWER($1)`, address).Scan(
		&t.Address, &t.Symbol, &t.Name, &t.Decimals, &t.TokenType, &t.TotalSupply, &t.HolderCount, &t.TransferCount,
		&t.USDPrice, &t.IconURL, &t.L1Address, &t.BlockNumber, &t.CreationTx, &t.OffChainUpdatedAt, &t.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &t, err
}

func (d *DB) GetTokens(ctx context.Context, limit int, offset int, tokenType string) ([]types.Token, int64, error) {
	var total int64
	query := "SELECT COUNT(*) FROM tokens"
	if tokenType != "" {
		query += " WHERE token_type = $1"
		d.pool.QueryRow(ctx, query, tokenType).Scan(&total)
	} else {
		d.pool.QueryRow(ctx, query).Scan(&total)
	}

	var rows pgx.Rows
	var err error
	if tokenType != "" {
		rows, err = d.pool.Query(ctx, `
			SELECT address, symbol, name, decimals, token_type, total_supply, holder_count, transfer_count,
				usd_price, icon_url, l1_address, block_number, creation_tx, off_chain_updated_at, created_at
			FROM tokens WHERE token_type = $1
			ORDER BY holder_count DESC, address LIMIT $2 OFFSET $3`, tokenType, limit, offset)
	} else {
		rows, err = d.pool.Query(ctx, `
			SELECT address, symbol, name, decimals, token_type, total_supply, holder_count, transfer_count,
				usd_price, icon_url, l1_address, block_number, creation_tx, off_chain_updated_at, created_at
			FROM tokens ORDER BY holder_count DESC, address LIMIT $1 OFFSET $2`, limit, offset)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tokens []types.Token
	for rows.Next() {
		var t types.Token
		if err := rows.Scan(&t.Address, &t.Symbol, &t.Name, &t.Decimals, &t.TokenType, &t.TotalSupply, &t.HolderCount, &t.TransferCount,
			&t.USDPrice, &t.IconURL, &t.L1Address, &t.BlockNumber, &t.CreationTx, &t.OffChainUpdatedAt, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		tokens = append(tokens, t)
	}
	return tokens, total, rows.Err()
}

func (d *DB) UpdateTokenStats(ctx context.Context, address string, holderCount int, transferCount int) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE tokens SET holder_count = $2, transfer_count = $3 WHERE LOWER(address) = LOWER($1)`,
		address, holderCount, transferCount)
	return err
}

func (d *DB) UpdateTokenPrice(ctx context.Context, address string, price float64, iconURL *string) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE tokens SET usd_price = $2, icon_url = $3, off_chain_updated_at = NOW()
		WHERE LOWER(address) = LOWER($1)`,
		address, price, iconURL)
	return err
}

// Token transfer operations

func (d *DB) InsertTokenTransfer(ctx context.Context, t *types.TokenTransfer) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO token_transfers (tx_hash, log_index, token_address, from_address, to_address, value,
			block_number, timestamp, transfer_type, token_type, token_id, is_internal)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (tx_hash, log_index) DO NOTHING`,
		t.TxHash, t.LogIndex, t.TokenAddress, t.From, t.To, t.Value,
		t.BlockNumber, t.Timestamp, t.TransferType, t.TokenType, t.TokenID, t.IsInternal)
	return err
}

func (d *DB) GetTransfersByAddress(ctx context.Context, address string, limit int, beforeBlock *uint64) ([]types.TokenTransfer, error) {
	var rows pgx.Rows
	var err error

	if beforeBlock != nil {
		rows, err = d.pool.Query(ctx, `
			SELECT id, tx_hash, log_index, token_address, from_address, to_address, value::text, block_number,
				timestamp, transfer_type, token_type, token_id, is_internal
			FROM token_transfers WHERE (from_address = $1 OR to_address = $1) AND block_number < $2
			ORDER BY block_number DESC, log_index DESC LIMIT $3`, address, *beforeBlock, limit)
	} else {
		rows, err = d.pool.Query(ctx, `
			SELECT id, tx_hash, log_index, token_address, from_address, to_address, value::text, block_number,
				timestamp, transfer_type, token_type, token_id, is_internal
			FROM token_transfers WHERE from_address = $1 OR to_address = $1
			ORDER BY block_number DESC, log_index DESC LIMIT $2`, address, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTokenTransfers(rows)
}

func (d *DB) GetTransfersByTransaction(ctx context.Context, txHash string) ([]types.TokenTransfer, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, log_index, token_address, from_address, to_address, value::text, block_number,
			timestamp, transfer_type, token_type, token_id, is_internal
		FROM token_transfers WHERE tx_hash = $1
		ORDER BY log_index`, txHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTokenTransfers(rows)
}

func (d *DB) GetTransfersByToken(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	var total int64
	d.pool.QueryRow(ctx, "SELECT COUNT(*) FROM token_transfers WHERE LOWER(token_address) = LOWER($1)", tokenAddress).Scan(&total)

	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, log_index, token_address, from_address, to_address, value::text, block_number,
			timestamp, transfer_type, token_type, token_id, is_internal
		FROM token_transfers WHERE LOWER(token_address) = LOWER($1)
		ORDER BY block_number DESC, log_index DESC LIMIT $2 OFFSET $3`, tokenAddress, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	transfers, err := scanTokenTransfers(rows)
	return transfers, total, err
}

func (d *DB) GetAllTransfers(ctx context.Context, limit int, offset int) ([]types.TokenTransfer, int64, error) {
	var total int64
	d.pool.QueryRow(ctx, "SELECT COUNT(*) FROM token_transfers").Scan(&total)

	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, log_index, token_address, from_address, to_address, value::text, block_number,
			timestamp, transfer_type, token_type, token_id, is_internal
		FROM token_transfers
		ORDER BY block_number DESC, log_index DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	transfers, err := scanTokenTransfers(rows)
	return transfers, total, err
}

func scanTokenTransfers(rows pgx.Rows) ([]types.TokenTransfer, error) {
	var transfers []types.TokenTransfer
	for rows.Next() {
		var t types.TokenTransfer
		var valueStr string
		if err := rows.Scan(&t.ID, &t.TxHash, &t.LogIndex, &t.TokenAddress, &t.From, &t.To, &valueStr, &t.BlockNumber,
			&t.Timestamp, &t.TransferType, &t.TokenType, &t.TokenID, &t.IsInternal); err != nil {
			return nil, err
		}
		t.Value = types.JSONString(valueStr)
		transfers = append(transfers, t)
	}
	return transfers, rows.Err()
}

// Token holder operations

func (d *DB) GetTokenHolders(ctx context.Context, tokenAddress string, limit int, offset int) ([]types.TokenHolder, int64, error) {
	var total int64
	d.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT address) FROM balances
		WHERE LOWER(token_address) = LOWER($1) AND balance > 0`, tokenAddress).Scan(&total)

	rows, err := d.pool.Query(ctx, `
		WITH latest_balances AS (
			SELECT address, balance,
				ROW_NUMBER() OVER (PARTITION BY address ORDER BY block_number DESC) as rn
			FROM balances
			WHERE LOWER(token_address) = LOWER($1) AND balance > 0
		),
		total_supply AS (
			SELECT COALESCE(SUM(balance), 1) as supply FROM latest_balances WHERE rn = 1
		)
		SELECT lb.address, lb.balance::text,
			(lb.balance::numeric / NULLIF(ts.supply::numeric, 0) * 100)::numeric(10,4) as percentage,
			COALESCE((SELECT true FROM contracts WHERE LOWER(address) = LOWER(lb.address)), false) as is_contract
		FROM latest_balances lb, total_supply ts
		WHERE lb.rn = 1
		ORDER BY lb.balance DESC
		LIMIT $2 OFFSET $3`, tokenAddress, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var holders []types.TokenHolder
	for rows.Next() {
		var h types.TokenHolder
		var balanceStr string
		if err := rows.Scan(&h.Address, &balanceStr, &h.Percentage, &h.IsContract); err != nil {
			return nil, 0, err
		}
		h.Balance = types.JSONString(balanceStr)
		holders = append(holders, h)
	}
	return holders, total, rows.Err()
}

// Balance operations

func (d *DB) InsertBalance(ctx context.Context, b *types.Balance) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO balances (address, token_address, block_number, balance)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (address, token_address, block_number) DO UPDATE SET balance = EXCLUDED.balance`,
		b.Address, b.TokenAddress, b.BlockNumber, b.Balance)
	return err
}

func (d *DB) GetLatestBalance(ctx context.Context, address string, tokenAddress string) (*types.Balance, error) {
	var b types.Balance
	var balanceStr string
	err := d.pool.QueryRow(ctx, `
		SELECT address, token_address, block_number, balance::text
		FROM balances
		WHERE LOWER(address) = LOWER($1) AND LOWER(token_address) = LOWER($2)
		ORDER BY block_number DESC LIMIT 1`, address, tokenAddress).Scan(
		&b.Address, &b.TokenAddress, &b.BlockNumber, &balanceStr)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	b.Balance = types.JSONString(balanceStr)
	return &b, nil
}

func (d *DB) GetBalanceHistory(ctx context.Context, address string, tokenAddress string, limit int) ([]types.Balance, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT address, token_address, block_number, balance::text
		FROM balances
		WHERE LOWER(address) = LOWER($1) AND LOWER(token_address) = LOWER($2)
		ORDER BY block_number DESC LIMIT $3`, address, tokenAddress, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []types.Balance
	for rows.Next() {
		var b types.Balance
		var balanceStr string
		if err := rows.Scan(&b.Address, &b.TokenAddress, &b.BlockNumber, &balanceStr); err != nil {
			return nil, err
		}
		b.Balance = types.JSONString(balanceStr)
		balances = append(balances, b)
	}
	return balances, rows.Err()
}

func (d *DB) GetTokenBalances(ctx context.Context, address string) ([]types.Balance, error) {
	rows, err := d.pool.Query(ctx, `
		WITH latest_balances AS (
			SELECT address, token_address, balance, block_number,
				ROW_NUMBER() OVER (PARTITION BY token_address ORDER BY block_number DESC) as rn
			FROM balances
			WHERE LOWER(address) = LOWER($1)
		)
		SELECT address, token_address, block_number, balance::text
		FROM latest_balances WHERE rn = 1 AND balance > 0
		ORDER BY balance DESC`, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var balances []types.Balance
	for rows.Next() {
		var b types.Balance
		var balanceStr string
		if err := rows.Scan(&b.Address, &b.TokenAddress, &b.BlockNumber, &balanceStr); err != nil {
			return nil, err
		}
		b.Balance = types.JSONString(balanceStr)
		balances = append(balances, b)
	}
	return balances, rows.Err()
}

// Counter operations

func (d *DB) IncrementCounter(ctx context.Context, address string, counterType string, delta int64) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO counters (address, counter_type, count, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (address, counter_type) DO UPDATE SET
			count = counters.count + $3,
			updated_at = NOW()`,
		address, counterType, delta)
	return err
}

func (d *DB) GetCounter(ctx context.Context, address string, counterType string) (int64, error) {
	var count int64
	err := d.pool.QueryRow(ctx, `
		SELECT count FROM counters WHERE LOWER(address) = LOWER($1) AND counter_type = $2`,
		address, counterType).Scan(&count)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return count, err
}

func (d *DB) GetCounters(ctx context.Context, address string) ([]types.Counter, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT address, counter_type, count, updated_at
		FROM counters WHERE LOWER(address) = LOWER($1)`, address)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counters []types.Counter
	for rows.Next() {
		var c types.Counter
		if err := rows.Scan(&c.Address, &c.CounterType, &c.Count, &c.UpdatedAt); err != nil {
			return nil, err
		}
		counters = append(counters, c)
	}
	return counters, rows.Err()
}

// Address stats operations

func (d *DB) UpsertAddressStats(ctx context.Context, address string, blockNumber uint64, isContract bool) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO address_stats (address, tx_count, first_seen, last_seen, is_contract, updated_at)
		VALUES ($1, 1, $2, $2, $3, NOW())
		ON CONFLICT (address) DO UPDATE SET
			tx_count = address_stats.tx_count + 1,
			last_seen = $2,
			is_contract = COALESCE(EXCLUDED.is_contract, address_stats.is_contract),
			updated_at = NOW()`,
		address, blockNumber, isContract)
	return err
}

func (d *DB) UpdateAddressStatsCounters(ctx context.Context, address string, internalTxCount int, tokenTransferCount int) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE address_stats SET
			internal_tx_count = internal_tx_count + $2,
			token_transfer_count = token_transfer_count + $3,
			updated_at = NOW()
		WHERE LOWER(address) = LOWER($1)`,
		address, internalTxCount, tokenTransferCount)
	return err
}

func (d *DB) GetAddressStats(ctx context.Context, address string) (*types.AddressStats, error) {
	var s types.AddressStats
	err := d.pool.QueryRow(ctx, `
		SELECT address, tx_count, internal_tx_count, token_transfer_count, first_seen, last_seen, is_contract, updated_at
		FROM address_stats WHERE LOWER(address) = LOWER($1)`, address).Scan(
		&s.Address, &s.TxCount, &s.InternalTxCount, &s.TokenTransferCount, &s.FirstSeen, &s.LastSeen, &s.IsContract, &s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return &types.AddressStats{Address: address}, nil
	}
	return &s, err
}

func (d *DB) GetAccountsPaginated(ctx context.Context, page, pageSize int) ([]types.AddressStats, int64, error) {
	var total int64
	err := d.pool.QueryRow(ctx, `SELECT COUNT(*) FROM address_stats`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	rows, err := d.pool.Query(ctx, `
		SELECT address, tx_count, internal_tx_count, token_transfer_count, first_seen, last_seen, is_contract, updated_at
		FROM address_stats
		ORDER BY tx_count DESC, address ASC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var accounts []types.AddressStats
	for rows.Next() {
		var s types.AddressStats
		if err := rows.Scan(&s.Address, &s.TxCount, &s.InternalTxCount, &s.TokenTransferCount, &s.FirstSeen, &s.LastSeen, &s.IsContract, &s.UpdatedAt); err != nil {
			return nil, 0, err
		}
		accounts = append(accounts, s)
	}

	return accounts, total, rows.Err()
}

// Contract operations

func (d *DB) InsertContract(ctx context.Context, c *types.Contract) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO contracts (address, bytecode, bytecode_hash, creator, creation_tx, block_number, is_verified,
			contract_name, compiler_version, optimization_used, source_code, abi)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (address) DO NOTHING`,
		c.Address, c.Bytecode, c.BytecodeHash, c.Creator, c.CreationTx, c.BlockNumber, c.IsVerified,
		c.ContractName, c.CompilerVersion, c.OptimizationUsed, c.SourceCode, c.ABI)
	return err
}

func (d *DB) GetContract(ctx context.Context, address string) (*types.Contract, error) {
	var c types.Contract
	err := d.pool.QueryRow(ctx, `
		SELECT address, bytecode, bytecode_hash, creator, creation_tx, block_number, is_verified,
			contract_name, compiler_version, optimization_used, source_code, abi, created_at
		FROM contracts WHERE LOWER(address) = LOWER($1)`, address).Scan(
		&c.Address, &c.Bytecode, &c.BytecodeHash, &c.Creator, &c.CreationTx, &c.BlockNumber, &c.IsVerified,
		&c.ContractName, &c.CompilerVersion, &c.OptimizationUsed, &c.SourceCode, &c.ABI, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &c, err
}

func (d *DB) IsContract(ctx context.Context, address string) (bool, error) {
	var exists bool
	err := d.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contracts WHERE LOWER(address) = LOWER($1))`, address).Scan(&exists)
	return exists, err
}

func (d *DB) VerifyContract(ctx context.Context, address string, name string, compilerVersion string, optimizationUsed bool, sourceCode string, abi json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE contracts SET
			is_verified = true,
			contract_name = $2,
			compiler_version = $3,
			optimization_used = $4,
			source_code = $5,
			abi = $6
		WHERE LOWER(address) = LOWER($1)`,
		address, name, compilerVersion, optimizationUsed, sourceCode, abi)
	return err
}

// UpdateContractABI updates the ABI for a contract (manual ABI upload without full verification)
func (d *DB) UpdateContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE contracts SET abi = $2
		WHERE LOWER(address) = LOWER($1)`,
		address, abi)
	return err
}

// SetContractABI sets the ABI for any address, creating a minimal contract record if needed
func (d *DB) SetContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	// Try to update existing contract first
	result, err := d.pool.Exec(ctx, `
		UPDATE contracts SET abi = $2
		WHERE LOWER(address) = LOWER($1)`,
		address, abi)
	if err != nil {
		return err
	}

	// If no rows updated, the contract doesn't exist - this is expected for
	// addresses that aren't contracts but user wants to interact with ABI anyway
	if result.RowsAffected() == 0 {
		// Insert a minimal record for non-indexed contracts (e.g., external contracts)
		_, err = d.pool.Exec(ctx, `
			INSERT INTO contracts (address, bytecode, creator, creation_tx, block_number, is_verified, abi)
			VALUES ($1, '', '', '', 0, false, $2)
			ON CONFLICT (address) DO UPDATE SET abi = $2`,
			address, abi)
	}
	return err
}

func (d *DB) GetVerifiedContracts(ctx context.Context, limit int, offset int) ([]types.Contract, int64, error) {
	var total int64
	d.pool.QueryRow(ctx, "SELECT COUNT(*) FROM contracts WHERE is_verified = true").Scan(&total)

	rows, err := d.pool.Query(ctx, `
		SELECT address, bytecode, bytecode_hash, creator, creation_tx, block_number, is_verified,
			contract_name, compiler_version, optimization_used, source_code, abi, created_at
		FROM contracts WHERE is_verified = true
		ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var contracts []types.Contract
	for rows.Next() {
		var c types.Contract
		if err := rows.Scan(&c.Address, &c.Bytecode, &c.BytecodeHash, &c.Creator, &c.CreationTx, &c.BlockNumber, &c.IsVerified,
			&c.ContractName, &c.CompilerVersion, &c.OptimizationUsed, &c.SourceCode, &c.ABI, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		contracts = append(contracts, c)
	}
	return contracts, total, rows.Err()
}

// Log operations

func (d *DB) InsertLog(ctx context.Context, l *types.Log) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO logs (tx_hash, log_index, address, topic0, topic1, topic2, topic3, data, block_number, timestamp, removed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (tx_hash, log_index) DO NOTHING`,
		l.TxHash, l.LogIndex, l.Address, l.Topic0, l.Topic1, l.Topic2, l.Topic3, l.Data, l.BlockNumber, l.Timestamp, l.Removed)
	return err
}

func (d *DB) GetLogsByTransaction(ctx context.Context, txHash string) ([]types.Log, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, log_index, address, topic0, topic1, topic2, topic3, data, block_number, timestamp, removed
		FROM logs WHERE tx_hash = $1
		ORDER BY log_index`, txHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLogs(rows)
}

func (d *DB) GetLogsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.Log, int64, error) {
	var total int64
	d.pool.QueryRow(ctx, "SELECT COUNT(*) FROM logs WHERE LOWER(address) = LOWER($1)", address).Scan(&total)

	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, log_index, address, topic0, topic1, topic2, topic3, data, block_number, timestamp, removed
		FROM logs WHERE LOWER(address) = LOWER($1)
		ORDER BY block_number DESC, log_index DESC LIMIT $2 OFFSET $3`, address, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs, err := scanLogs(rows)
	return logs, total, err
}

func (d *DB) GetLogsByTopic(ctx context.Context, topic0 string, limit int, offset int) ([]types.Log, int64, error) {
	var total int64
	d.pool.QueryRow(ctx, "SELECT COUNT(*) FROM logs WHERE topic0 = $1", topic0).Scan(&total)

	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, log_index, address, topic0, topic1, topic2, topic3, data, block_number, timestamp, removed
		FROM logs WHERE topic0 = $1
		ORDER BY block_number DESC, log_index DESC LIMIT $2 OFFSET $3`, topic0, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs, err := scanLogs(rows)
	return logs, total, err
}

func (d *DB) GetLogs(ctx context.Context, address *string, topic0 *string, fromBlock *uint64, toBlock *uint64, limit int) ([]types.Log, error) {
	query := `SELECT id, tx_hash, log_index, address, topic0, topic1, topic2, topic3, data, block_number, timestamp, removed FROM logs WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if address != nil {
		query += fmt.Sprintf(" AND LOWER(address) = LOWER($%d)", argIdx)
		args = append(args, *address)
		argIdx++
	}
	if topic0 != nil {
		query += fmt.Sprintf(" AND topic0 = $%d", argIdx)
		args = append(args, *topic0)
		argIdx++
	}
	if fromBlock != nil {
		query += fmt.Sprintf(" AND block_number >= $%d", argIdx)
		args = append(args, *fromBlock)
		argIdx++
	}
	if toBlock != nil {
		query += fmt.Sprintf(" AND block_number <= $%d", argIdx)
		args = append(args, *toBlock)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY block_number DESC, log_index DESC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := d.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanLogs(rows)
}

func scanLogs(rows pgx.Rows) ([]types.Log, error) {
	var logs []types.Log
	for rows.Next() {
		var l types.Log
		if err := rows.Scan(&l.ID, &l.TxHash, &l.LogIndex, &l.Address, &l.Topic0, &l.Topic1, &l.Topic2, &l.Topic3, &l.Data, &l.BlockNumber, &l.Timestamp, &l.Removed); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// Internal transaction operations

func (d *DB) InsertInternalTransaction(ctx context.Context, it *types.InternalTransaction) error {
	_, err := d.pool.Exec(ctx, `
		INSERT INTO internal_transactions (tx_hash, block_number, trace_address, from_address, to_address, value,
			gas, gas_used, input, output, call_type, error, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (tx_hash, trace_address) DO NOTHING`,
		it.TxHash, it.BlockNumber, it.TraceAddress, it.From, it.To, it.Value,
		it.Gas, it.GasUsed, it.Input, it.Output, it.CallType, it.Error, it.Timestamp)
	return err
}

func (d *DB) GetInternalTransactionsByTx(ctx context.Context, txHash string) ([]types.InternalTransaction, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, block_number, trace_address, from_address, to_address, value::text,
			gas, gas_used, input, output, call_type, error, timestamp
		FROM internal_transactions WHERE tx_hash = $1
		ORDER BY trace_address`, txHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInternalTransactions(rows)
}

func (d *DB) GetInternalTransactionsByAddress(ctx context.Context, address string, limit int, offset int) ([]types.InternalTransaction, int64, error) {
	var total int64
	d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM internal_transactions
		WHERE LOWER(from_address) = LOWER($1) OR LOWER(to_address) = LOWER($1)`, address).Scan(&total)

	rows, err := d.pool.Query(ctx, `
		SELECT id, tx_hash, block_number, trace_address, from_address, to_address, value::text,
			gas, gas_used, input, output, call_type, error, timestamp
		FROM internal_transactions
		WHERE LOWER(from_address) = LOWER($1) OR LOWER(to_address) = LOWER($1)
		ORDER BY block_number DESC, id DESC LIMIT $2 OFFSET $3`, address, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	its, err := scanInternalTransactions(rows)
	return its, total, err
}

func scanInternalTransactions(rows pgx.Rows) ([]types.InternalTransaction, error) {
	var its []types.InternalTransaction
	for rows.Next() {
		var it types.InternalTransaction
		var valueStr string
		if err := rows.Scan(&it.ID, &it.TxHash, &it.BlockNumber, &it.TraceAddress, &it.From, &it.To, &valueStr,
			&it.Gas, &it.GasUsed, &it.Input, &it.Output, &it.CallType, &it.Error, &it.Timestamp); err != nil {
			return nil, err
		}
		it.Value = types.JSONString(valueStr)
		its = append(its, it)
	}
	return its, rows.Err()
}

// Sync status operations

func (d *DB) GetSyncStatus(ctx context.Context) (*types.SyncStatus, error) {
	var s types.SyncStatus
	err := d.pool.QueryRow(ctx, `
		SELECT id, last_indexed_block, last_verified_block, last_finalized_block, is_syncing, updated_at
		FROM sync_status ORDER BY id LIMIT 1`).Scan(
		&s.ID, &s.LastIndexedBlock, &s.LastVerifiedBlock, &s.LastFinalizedBlock, &s.IsSyncing, &s.UpdatedAt)
	if err == pgx.ErrNoRows {
		return &types.SyncStatus{}, nil
	}
	return &s, err
}

func (d *DB) UpdateSyncStatus(ctx context.Context, lastIndexedBlock uint64, isSyncing bool) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE sync_status SET last_indexed_block = $1, is_syncing = $2, updated_at = NOW()
		WHERE id = (SELECT id FROM sync_status ORDER BY id LIMIT 1)`,
		lastIndexedBlock, isSyncing)
	return err
}

func (d *DB) UpdateSyncStatusBlocks(ctx context.Context, verifiedBlock *uint64, finalizedBlock *uint64) error {
	_, err := d.pool.Exec(ctx, `
		UPDATE sync_status SET
			last_verified_block = COALESCE($1, last_verified_block),
			last_finalized_block = COALESCE($2, last_finalized_block),
			updated_at = NOW()
		WHERE id = (SELECT id FROM sync_status ORDER BY id LIMIT 1)`,
		verifiedBlock, finalizedBlock)
	return err
}

// Stats

func (d *DB) GetChainStats(ctx context.Context) (*types.ChainStats, error) {
	var stats types.ChainStats
	var avgBlockTime *float64
	err := d.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM blocks),
			(SELECT COUNT(*) FROM transactions),
			(SELECT COUNT(*) FROM address_stats),
			(SELECT COUNT(*) FROM tokens),
			(SELECT
				CASE WHEN COUNT(*) > 1
				THEN (MAX(timestamp) - MIN(timestamp))::float / NULLIF(COUNT(*) - 1, 0)
				ELSE NULL END
			FROM (SELECT timestamp FROM blocks ORDER BY number DESC LIMIT 100) recent_blocks)
		`).Scan(&stats.TotalBlocks, &stats.TotalTransactions, &stats.TotalAddresses, &stats.TotalTokens, &avgBlockTime)
	if avgBlockTime != nil {
		stats.AvgBlockTime = *avgBlockTime
	}
	return &stats, err
}

func (d *DB) GetTransactionHistory(ctx context.Context, intervalSeconds int, limit int) ([]types.TxHistoryPoint, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT
			(b.timestamp / $1) * $1 as interval_start,
			COUNT(t.hash) as tx_count
		FROM blocks b
		LEFT JOIN transactions t ON t.block_number = b.number
		WHERE b.timestamp > 0
		GROUP BY interval_start
		ORDER BY interval_start DESC
		LIMIT $2`, intervalSeconds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []types.TxHistoryPoint
	for rows.Next() {
		var p types.TxHistoryPoint
		if err := rows.Scan(&p.Timestamp, &p.Count); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}

	return points, rows.Err()
}

// Search suggestions

func (d *DB) SearchSuggestions(ctx context.Context, query string, limit int) ([]types.SearchSuggestion, error) {
	var suggestions []types.SearchSuggestion

	// If query is numeric, search for block numbers
	if isNumeric(query) {
		rows, err := d.pool.Query(ctx, `
			SELECT number FROM blocks
			WHERE number::text LIKE $1 || '%'
			ORDER BY number DESC LIMIT $2`, query, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var num uint64
			if err := rows.Scan(&num); err != nil {
				return nil, err
			}
			suggestions = append(suggestions, types.SearchSuggestion{
				Type:  "block",
				Value: fmt.Sprintf("%d", num),
				Label: fmt.Sprintf("Block #%d", num),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// If query looks like a hash (starts with 0x), search transactions
	if len(query) >= 2 && query[:2] == "0x" {
		rows, err := d.pool.Query(ctx, `
			SELECT hash FROM transactions
			WHERE LOWER(hash) LIKE LOWER($1) || '%'
			ORDER BY block_number DESC LIMIT $2`, query, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var hash string
			if err := rows.Scan(&hash); err != nil {
				return nil, err
			}
			suggestions = append(suggestions, types.SearchSuggestion{
				Type:  "transaction",
				Value: hash,
				Label: truncateHash(hash),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Search addresses
	if len(query) >= 2 && query[:2] == "0x" {
		rows, err := d.pool.Query(ctx, `
			SELECT address, tx_count FROM address_stats
			WHERE LOWER(address) LIKE LOWER($1) || '%'
			ORDER BY tx_count DESC LIMIT $2`, query, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var address string
			var txCount int
			if err := rows.Scan(&address, &txCount); err != nil {
				return nil, err
			}
			suggestions = append(suggestions, types.SearchSuggestion{
				Type:  "address",
				Value: address,
				Label: fmt.Sprintf("%s (%d txs)", truncateHash(address), txCount),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Search tokens by symbol or name
	if len(query) >= 1 {
		rows, err := d.pool.Query(ctx, `
			SELECT address, symbol, name FROM tokens
			WHERE LOWER(symbol) LIKE LOWER($1) || '%' OR LOWER(name) LIKE '%' || LOWER($1) || '%'
			ORDER BY holder_count DESC LIMIT $2`, query, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var address, symbol string
			var name *string
			if err := rows.Scan(&address, &symbol, &name); err != nil {
				return nil, err
			}
			label := symbol
			if name != nil && *name != "" {
				label = fmt.Sprintf("%s (%s)", symbol, *name)
			}
			suggestions = append(suggestions, types.SearchSuggestion{
				Type:  "token",
				Value: address,
				Label: label,
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return suggestions, nil
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func truncateHash(hash string) string {
	if len(hash) <= 16 {
		return hash
	}
	return hash[:10] + "..." + hash[len(hash)-6:]
}

// Gas tracker queries

// GasPercentiles holds the calculated gas percentile values
type GasPercentiles struct {
	SlowWei   *uint64
	NormalWei *uint64
	FastWei   *uint64
	BaseFee   *uint64
}

// GetGasPercentiles calculates gas price percentiles from recent transactions
func (d *DB) GetGasPercentiles(ctx context.Context, numBlocks int, slowPct, avgPct, fastPct float64) (*GasPercentiles, error) {
	var slow, normal, fast, baseFee *uint64

	// Query gas price percentiles from recent transactions
	// First try recent blocks, then fall back to recent transactions if no data
	query := `
		WITH recent_txs AS (
			SELECT t.gas_price
			FROM transactions t
			WHERE t.gas_price > 0
			ORDER BY t.block_number DESC
			LIMIT 10000
		)
		SELECT
			COALESCE(PERCENTILE_CONT($1) WITHIN GROUP (ORDER BY gas_price)::bigint, 0) as slow,
			COALESCE(PERCENTILE_CONT($2) WITHIN GROUP (ORDER BY gas_price)::bigint, 0) as normal,
			COALESCE(PERCENTILE_CONT($3) WITHIN GROUP (ORDER BY gas_price)::bigint, 0) as fast,
			(SELECT base_fee_per_gas FROM blocks WHERE base_fee_per_gas IS NOT NULL ORDER BY number DESC LIMIT 1) as base_fee
		FROM recent_txs
	`

	err := d.pool.QueryRow(ctx, query, slowPct, avgPct, fastPct).Scan(&slow, &normal, &fast, &baseFee)
	if err != nil {
		return nil, err
	}

	return &GasPercentiles{
		SlowWei:   slow,
		NormalWei: normal,
		FastWei:   fast,
		BaseFee:   baseFee,
	}, nil
}
