package db

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"explorer/internal/types"
	"explorer/pkg/eth/common"

	"github.com/jackc/pgx/v5"
)

func addressKeys(address string) (canonical, legacy string) {
	canonical = common.HexToAddress(address).Hex()
	return canonical, strings.ToLower(canonical)
}

// GetContractVerification returns the verified-contract row for an address
// if one exists. Returns (nil, nil) when not verified. The caller (api
// handler) merges this with chain facts fetched from chain-indexer to
// produce a full types.Contract.
func (d *DB) GetContractVerification(ctx context.Context, address string) (*types.Contract, error) {
	canonical, legacy := addressKeys(address)
	var c types.Contract
	var abiBytes []byte
	err := d.pool.QueryRow(ctx, `
		SELECT address, contract_name, compiler_version, optimization_used,
		       optimization_runs, evm_version, license_type, source_code,
		       abi, constructor_args, verified_at
		FROM contract_verifications
		WHERE address IN ($1, $2)
		ORDER BY source_code IS NOT NULL DESC, address = $1 DESC
		LIMIT 1`, canonical, legacy).Scan(
		&c.Address, &c.ContractName, &c.CompilerVersion, &c.OptimizationUsed,
		&c.OptimizationRuns, &c.EVMVersion, &c.LicenseType, &c.SourceCode,
		&abiBytes, &c.ConstructorArgs, &c.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Address = canonical
	if len(abiBytes) > 0 {
		c.ABI = json.RawMessage(abiBytes)
	}
	c.IsVerified = true
	return &c, nil
}

// SetContractABI records (or overwrites) the ABI for an address without
// full source-code verification. Used when a user uploads an ABI-only
// record. address is the primary key; on conflict, ABI is replaced and
// verified_at is bumped.
func (d *DB) SetContractABI(ctx context.Context, address string, abi json.RawMessage) error {
	canonical, legacy := addressKeys(address)
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			DELETE FROM contract_verifications
			WHERE address = $1 AND address <> $2 AND source_code IS NULL
			  AND EXISTS (SELECT 1 FROM contract_verifications WHERE address = $2 AND source_code IS NOT NULL)`,
			canonical, legacy); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE contract_verifications SET address = $1
			WHERE address = $2 AND address <> $1
			  AND NOT EXISTS (SELECT 1 FROM contract_verifications WHERE address = $1)`,
			canonical, legacy); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO contract_verifications (address, abi, verified_at)
			VALUES ($1, $2, $3)
			ON CONFLICT (address) DO UPDATE
			  SET abi = EXCLUDED.abi,
			      verified_at = EXCLUDED.verified_at`,
			canonical, []byte(abi), time.Now().UTC()); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM contract_verifications WHERE address = $2 AND address <> $1`,
			canonical, legacy)
		return err
	})
}

// VerifyContract records a full verification — source + compiler settings
// + ABI. Any prior row for the same address is replaced.
func (d *DB) VerifyContract(
	ctx context.Context,
	address, contractName, compilerVersion string,
	optimizationUsed bool,
	sourceCode string,
	abi json.RawMessage,
	evmVersion, licenseType, constructorArgs string,
	optimizationRuns int,
) error {
	canonical, legacy := addressKeys(address)
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO contract_verifications (
			  address, contract_name, compiler_version, optimization_used,
			  optimization_runs, evm_version, license_type, source_code,
			  abi, constructor_args, verified_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (address) DO UPDATE SET
			  contract_name = EXCLUDED.contract_name,
			  compiler_version = EXCLUDED.compiler_version,
			  optimization_used = EXCLUDED.optimization_used,
			  optimization_runs = EXCLUDED.optimization_runs,
			  evm_version = EXCLUDED.evm_version,
			  license_type = EXCLUDED.license_type,
			  source_code = EXCLUDED.source_code,
			  abi = EXCLUDED.abi,
			  constructor_args = EXCLUDED.constructor_args,
			  verified_at = EXCLUDED.verified_at`,
			canonical, contractName, compilerVersion, optimizationUsed,
			optimizationRuns, evmVersion, licenseType, sourceCode,
			[]byte(abi), constructorArgs, time.Now().UTC()); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			DELETE FROM contract_verifications WHERE address = $2 AND address <> $1`,
			canonical, legacy)
		return err
	})
}
