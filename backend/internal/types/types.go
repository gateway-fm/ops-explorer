package types

import (
	"encoding/json"
	"math/big"
	"strings"
	"time"
)

// BigInt wraps big.Int to serialize as a string in JSON
type BigInt struct {
	i *big.Int
}

func NewBigInt(i *big.Int) BigInt {
	if i == nil {
		return BigInt{i: big.NewInt(0)}
	}
	return BigInt{i: i}
}

func (b BigInt) MarshalJSON() ([]byte, error) {
	if b.i == nil {
		return json.Marshal("0")
	}
	return json.Marshal(b.i.String())
}

func (b BigInt) String() string {
	if b.i == nil {
		return "0"
	}
	return b.i.String()
}

// JSONString is a string that always serializes as a JSON string
type JSONString string

func (s JSONString) MarshalJSON() ([]byte, error) {
	escaped := string(s)
	escaped = strings.ReplaceAll(escaped, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return []byte(`"` + escaped + `"`), nil
}

// Block represents a blockchain block
type Block struct {
	Number           uint64    `json:"number"`
	Hash             string    `json:"hash"`
	ParentHash       string    `json:"parentHash"`
	Timestamp        uint64    `json:"timestamp"`
	GasUsed          uint64    `json:"gasUsed"`
	GasLimit         uint64    `json:"gasLimit"`
	BaseFeePerGas    *uint64   `json:"baseFeePerGas,omitempty"`
	TransactionCount int       `json:"transactionCount"`
	// Additional block fields (Blockscout parity)
	Size             uint64    `json:"size"`
	Difficulty       string    `json:"difficulty"`       // Stored as string for large values
	TotalDifficulty  string    `json:"totalDifficulty"`  // Cumulative difficulty
	Nonce            string    `json:"nonce"`            // 8-byte hex string
	Miner            string    `json:"miner"`            // Block producer/validator address
	ExtraData        string    `json:"extraData"`        // Arbitrary data (hex encoded)
	StateRoot        string    `json:"stateRoot"`        // Merkle root of state trie
	TransactionsRoot string    `json:"transactionsRoot"` // Merkle root of transactions
	ReceiptsRoot     string    `json:"receiptsRoot"`     // Merkle root of receipts
	CreatedAt        time.Time `json:"createdAt"`
}

// Transaction represents a blockchain transaction
type Transaction struct {
	Hash                  string     `json:"hash"`
	BlockNumber           uint64     `json:"blockNumber"`
	BlockTimestamp        uint64     `json:"blockTimestamp,omitempty"`
	TxIndex               int        `json:"txIndex"`
	From                  string     `json:"from"`
	To                    *string    `json:"to"`
	ContractAddress       *string    `json:"contractAddress,omitempty"`
	Value                 JSONString `json:"value"`
	GasUsed               uint64     `json:"gasUsed"`
	GasPrice              uint64     `json:"gasPrice"`
	GasLimit              *uint64    `json:"gasLimit,omitempty"`
	MaxFeePerGas          *uint64    `json:"maxFeePerGas,omitempty"`
	MaxPriorityFeePerGas  *uint64    `json:"maxPriorityFeePerGas,omitempty"`
	Nonce                 *uint64    `json:"nonce,omitempty"`
	TxType                int        `json:"txType"`
	InputData             string     `json:"inputData"`
	Status                int        `json:"status"`
	Error                 *string    `json:"error,omitempty"`
	RevertReason          *string    `json:"revertReason,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	// Computed transaction categories (coin_transfer, contract_call, contract_creation, token_transfer)
	TxCategories          []string   `json:"txCategories,omitempty"`
	TokenTransferCount    int        `json:"tokenTransferCount,omitempty"`
}

// Token represents an ERC20/ERC721 token
type Token struct {
	Address            string     `json:"address"`
	Symbol             string     `json:"symbol"`
	Name               *string    `json:"name,omitempty"`
	Decimals           int        `json:"decimals"`
	TokenType          string     `json:"tokenType"` // ERC20, ERC721, NATIVE
	TotalSupply        *string    `json:"totalSupply,omitempty"`
	HolderCount        int        `json:"holderCount"`
	TransferCount      int        `json:"transferCount"`
	USDPrice           *float64   `json:"usdPrice,omitempty"`
	IconURL            *string    `json:"iconUrl,omitempty"`
	L1Address          *string    `json:"l1Address,omitempty"`
	BlockNumber        uint64     `json:"blockNumber"`
	CreationTx         *string    `json:"creationTx,omitempty"`
	OffChainUpdatedAt  *time.Time `json:"offChainUpdatedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
}

// TokenTransfer represents an ERC20/ERC721 token transfer
type TokenTransfer struct {
	ID           int64      `json:"id"`
	TxHash       string     `json:"txHash"`
	LogIndex     int        `json:"logIndex"`
	TokenAddress string     `json:"tokenAddress"`
	From         string     `json:"from"`
	To           string     `json:"to"`
	Value        JSONString `json:"value"`
	BlockNumber  uint64     `json:"blockNumber"`
	Timestamp    *uint64    `json:"timestamp,omitempty"`
	TransferType string     `json:"transferType"` // transfer, mint, burn, deposit, withdrawal
	TokenType    string     `json:"tokenType"`    // ERC20, ERC721
	TokenID      *string    `json:"tokenId,omitempty"` // For NFTs
	IsInternal   bool       `json:"isInternal"`
}

// Balance represents a token balance at a specific block
type Balance struct {
	Address      string     `json:"address"`
	TokenAddress string     `json:"tokenAddress"`
	BlockNumber  uint64     `json:"blockNumber"`
	Balance      JSONString `json:"balance"`
}

// Counter represents a pre-computed count for an address
type Counter struct {
	Address     string    `json:"address"`
	CounterType string    `json:"counterType"`
	Count       int64     `json:"count"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// AddressStats represents statistics for an address
type AddressStats struct {
	Address            string    `json:"address"`
	TxCount            int       `json:"txCount"`
	InternalTxCount    int       `json:"internalTxCount"`
	TokenTransferCount int       `json:"tokenTransferCount"`
	FirstSeen          *uint64   `json:"firstSeen"`
	LastSeen           *uint64   `json:"lastSeen"`
	IsContract         bool      `json:"isContract"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// Contract represents a deployed smart contract
type Contract struct {
	Address          string          `json:"address"`
	Bytecode         string          `json:"bytecode"`
	BytecodeHash     *string         `json:"bytecodeHash,omitempty"`
	Creator          string          `json:"creator"`
	CreationTx       string          `json:"creationTx"`
	BlockNumber      uint64          `json:"blockNumber"`
	IsVerified       bool            `json:"isVerified"`
	ContractName     *string         `json:"contractName,omitempty"`
	CompilerVersion  *string         `json:"compilerVersion,omitempty"`
	OptimizationUsed *bool           `json:"optimizationUsed,omitempty"`
	EVMVersion       *string         `json:"evmVersion,omitempty"`
	SourceCode       *string         `json:"sourceCode,omitempty"`
	ABI              json.RawMessage `json:"abi,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
}

// Log represents an event log
type Log struct {
	ID          int64   `json:"id"`
	TxHash      string  `json:"txHash"`
	LogIndex    int     `json:"logIndex"`
	Address     string  `json:"address"`
	Topic0      *string `json:"topic0"`
	Topic1      *string `json:"topic1"`
	Topic2      *string `json:"topic2"`
	Topic3      *string `json:"topic3"`
	Data        string  `json:"data"`
	BlockNumber uint64  `json:"blockNumber"`
	Timestamp   *uint64 `json:"timestamp,omitempty"`
	Removed     bool    `json:"removed"`
}

// InternalTransaction represents an internal transaction from traces
type InternalTransaction struct {
	ID           int64      `json:"id"`
	TxHash       string     `json:"txHash"`
	BlockNumber  uint64     `json:"blockNumber"`
	TraceAddress string     `json:"traceAddress"`
	From         string     `json:"from"`
	To           *string    `json:"to"`
	Value        JSONString `json:"value"`
	Gas          *uint64    `json:"gas,omitempty"`
	GasUsed      *uint64    `json:"gasUsed,omitempty"`
	Input        *string    `json:"input,omitempty"`
	Output       *string    `json:"output,omitempty"`
	CallType     string     `json:"callType"` // call, delegatecall, staticcall, create, create2
	Error        *string    `json:"error,omitempty"`
	Timestamp    *uint64    `json:"timestamp,omitempty"`
}

// SyncStatus represents the indexer sync status
type SyncStatus struct {
	ID                 int64     `json:"id"`
	LastIndexedBlock   uint64    `json:"lastIndexedBlock"`
	LastVerifiedBlock  *uint64   `json:"lastVerifiedBlock,omitempty"`
	LastFinalizedBlock *uint64   `json:"lastFinalizedBlock,omitempty"`
	IsSyncing          bool      `json:"isSyncing"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// AddressInfo combines address stats with live balance
type AddressInfo struct {
	Address            string     `json:"address"`
	Balance            JSONString `json:"balance"`
	TxCount            int        `json:"txCount"`
	InternalTxCount    int        `json:"internalTxCount"`
	TokenTransferCount int        `json:"tokenTransferCount"`
	IsContract         bool       `json:"isContract"`
}

// ChainStats represents overall chain statistics
type ChainStats struct {
	TotalBlocks       int64   `json:"totalBlocks"`
	TotalTransactions int64   `json:"totalTransactions"`
	TotalAddresses    int64   `json:"totalAddresses"`
	TotalTokens       int64   `json:"totalTokens"`
	AvgBlockTime      float64 `json:"avgBlockTime"`
}

// PaginatedResponse for cursor-based pagination
type PaginatedResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"nextCursor,omitempty"`
	HasMore    bool    `json:"hasMore"`
}

// OffsetPaginatedResponse for offset-based pagination
type OffsetPaginatedResponse[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	TotalPages int   `json:"totalPages"`
}

// AccountListItem for top accounts page
type AccountListItem struct {
	Address    string     `json:"address"`
	Balance    JSONString `json:"balance"`
	TxCount    int        `json:"txCount"`
	IsContract bool       `json:"isContract"`
}

// Search types
type SearchSuggestion struct {
	Type  string `json:"type"`  // block, transaction, address, token
	Value string `json:"value"`
	Label string `json:"label"`
}

type SearchSuggestionsResponse struct {
	Query       string             `json:"query"`
	Suggestions []SearchSuggestion `json:"suggestions"`
}

// Transaction history data point for charts
type TxHistoryPoint struct {
	Timestamp uint64 `json:"timestamp"`
	Count     int64  `json:"count"`
}

// TokenHolder represents a token holder
type TokenHolder struct {
	Address     string     `json:"address"`
	Balance     JSONString `json:"balance"`
	Percentage  float64    `json:"percentage"`
	IsContract  bool       `json:"isContract"`
}

// TransferType constants
const (
	TransferTypeTransfer   = "transfer"
	TransferTypeMint       = "mint"
	TransferTypeBurn       = "burn"
	TransferTypeDeposit    = "deposit"
	TransferTypeWithdrawal = "withdrawal"
)

// TokenType constants
const (
	TokenTypeERC20  = "ERC20"
	TokenTypeERC721 = "ERC721"
	TokenTypeNative = "NATIVE"
)

// CallType constants for internal transactions
const (
	CallTypeCall         = "call"
	CallTypeDelegateCall = "delegatecall"
	CallTypeStaticCall   = "staticcall"
	CallTypeCreate       = "create"
	CallTypeCreate2      = "create2"
)

// TxCategory constants for transaction categorization
const (
	TxCategoryCoinTransfer      = "coin_transfer"      // Native coin was transferred (value > 0)
	TxCategoryContractCall      = "contract_call"      // Called a contract with input data
	TxCategoryContractCreation  = "contract_creation"  // Deployed a new contract
	TxCategoryTokenTransfer     = "token_transfer"     // Involved ERC20/ERC721 transfers
)
