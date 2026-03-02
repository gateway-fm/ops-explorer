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

// sourceHash = keccak256(bytes32(0) || keccak256(l1BlockHash || bytes32(logIndex)))
func ComputeSourceHash(l1BlockHash common.Hash, logIndex uint64) common.Hash {
	var logIndexBytes [32]byte
	binary.BigEndian.PutUint64(logIndexBytes[24:], logIndex)

	innerInput := make([]byte, 64)
	copy(innerInput[:32], l1BlockHash.Bytes())
	copy(innerInput[32:], logIndexBytes[:])
	innerHash := crypto.Keccak256Hash(innerInput)

	// Domain 0 = user deposit
	var domain [32]byte
	outerInput := make([]byte, 64)
	copy(outerInput[:32], domain[:])
	copy(outerInput[32:], innerHash.Bytes())

	return crypto.Keccak256Hash(outerInput)
}

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

// L2 tx hash = keccak256(0x7E || RLP([sourceHash, from, to, mint, value, gas, isSystemTx, data]))
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

	encoded, err := rlp.EncodeToBytes(fields)
	if err != nil {
		return common.Hash{}, err
	}

	txBytes := make([]byte, 1+len(encoded))
	txBytes[0] = depositTxType
	copy(txBytes[1:], encoded)

	return crypto.Keccak256Hash(txBytes), nil
}

// opaqueData layout:
//
//	[0:32]   - msg.value (uint256)
//	[32:64]  - value (uint256)
//	[64:72]  - gasLimit (uint64)
//	[72:73]  - isCreation (bool, 0 or 1)
//	[73:]    - data (remaining bytes)
func ParseOpaqueData(opaqueData []byte) (msgValue, value *big.Int, gasLimit uint64, isCreation bool, data []byte, err error) {
	if len(opaqueData) < 73 {
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
