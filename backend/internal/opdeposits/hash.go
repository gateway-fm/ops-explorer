package opdeposits

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// depositTxType is the EIP-2718 type byte for OP Stack deposit transactions.
const depositTxType = 0x7E

// ComputeSourceHash computes the sourceHash for a user-deposited transaction.
// sourceHash = keccak256(bytes32(0) || keccak256(l1BlockHash || bytes32(logIndex)))
func ComputeSourceHash(l1BlockHash common.Hash, logIndex uint64) common.Hash {
	// Inner hash: keccak256(l1BlockHash || bytes32(logIndex))
	var logIndexBytes [32]byte
	binary.BigEndian.PutUint64(logIndexBytes[24:], logIndex)

	innerInput := make([]byte, 64)
	copy(innerInput[:32], l1BlockHash.Bytes())
	copy(innerInput[32:], logIndexBytes[:])
	innerHash := crypto.Keccak256Hash(innerInput)

	// Outer hash: keccak256(bytes32(0) || innerHash)
	// Domain 0 = user deposit
	var domain [32]byte // all zeros = domain 0
	outerInput := make([]byte, 64)
	copy(outerInput[:32], domain[:])
	copy(outerInput[32:], innerHash.Bytes())

	return crypto.Keccak256Hash(outerInput)
}

// depositTxFields holds the fields for RLP encoding a deposit transaction.
type depositTxFields struct {
	SourceHash  common.Hash
	From        common.Address
	To          *common.Address // nil for contract creation
	Mint        *big.Int
	Value       *big.Int
	Gas         uint64
	IsSystemTx  bool
	Data        []byte
}

// ComputeL2DepositTxHash computes the L2 transaction hash from deposit event data.
// The L2 tx hash is keccak256(0x7E || RLP([sourceHash, from, to, mint, value, gas, isSystemTx, data]))
func ComputeL2DepositTxHash(sourceHash common.Hash, from common.Address, to *common.Address, mint, value *big.Int, gas uint64, isSystemTx bool, data []byte) (common.Hash, error) {
	if mint == nil {
		mint = big.NewInt(0)
	}
	if value == nil {
		value = big.NewInt(0)
	}

	fields := depositTxFields{
		SourceHash: sourceHash,
		From:       from,
		To:         to,
		Mint:       mint,
		Value:      value,
		Gas:        gas,
		IsSystemTx: isSystemTx,
		Data:       data,
	}

	// RLP encode the fields
	encoded, err := rlp.EncodeToBytes(fields)
	if err != nil {
		return common.Hash{}, err
	}

	// Prepend the deposit type byte
	txBytes := make([]byte, 1+len(encoded))
	txBytes[0] = depositTxType
	copy(txBytes[1:], encoded)

	return crypto.Keccak256Hash(txBytes), nil
}

// ParseOpaqueData parses the opaqueData from a TransactionDeposited event.
// opaqueData layout:
//   [0:32]   - msg.value (uint256)
//   [32:64]  - value (uint256)
//   [64:72]  - gasLimit (uint64)
//   [72:73]  - isCreation (bool, 0 or 1)
//   [73:]    - data (remaining bytes)
func ParseOpaqueData(opaqueData []byte) (msgValue, value *big.Int, gasLimit uint64, isCreation bool, data []byte, err error) {
	if len(opaqueData) < 73 {
		// Minimum: 32 (mint) + 32 (value) + 8 (gas) + 1 (isCreation) = 73
		return nil, nil, 0, false, nil, nil
	}

	msgValue = new(big.Int).SetBytes(opaqueData[0:32])
	value = new(big.Int).SetBytes(opaqueData[32:64])
	gasLimit = binary.BigEndian.Uint64(opaqueData[64:72])
	isCreation = opaqueData[72] == 1

	if len(opaqueData) > 73 {
		data = opaqueData[73:]
	}

	return msgValue, value, gasLimit, isCreation, data, nil
}
