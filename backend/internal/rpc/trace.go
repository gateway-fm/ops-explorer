package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"explorer/internal/log"
	explorerTypes "explorer/internal/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// CallFrame represents a single call frame in the trace
type CallFrame struct {
	Type    string       `json:"type"`
	From    string       `json:"from"`
	To      string       `json:"to,omitempty"`
	Value   *hexutil.Big `json:"value,omitempty"`
	Gas     hexutil.Uint64 `json:"gas,omitempty"`
	GasUsed hexutil.Uint64 `json:"gasUsed,omitempty"`
	Input   string       `json:"input,omitempty"`
	Output  string       `json:"output,omitempty"`
	Error   string       `json:"error,omitempty"`
	Calls   []CallFrame  `json:"calls,omitempty"`
}

// TraceConfig for debug_traceBlockByNumber
type TraceConfig struct {
	Tracer string `json:"tracer"`
}

// TraceBlockResult represents the result of tracing a block
type TraceBlockResult struct {
	TxHash string    `json:"txHash"`
	Result CallFrame `json:"result"`
}

// CheckTracingSupport checks if the RPC node supports tracing
func (c *Client) CheckTracingSupport(ctx context.Context) (bool, error) {
	// Try to call debug_traceBlockByNumber on block 0 to check support
	var result json.RawMessage
	err := c.raw.CallContext(ctx, &result, "debug_traceBlockByNumber", "0x0", &TraceConfig{Tracer: "callTracer"})
	if err != nil {
		// Check if it's a method not found error
		if strings.Contains(err.Error(), "method not found") ||
		   strings.Contains(err.Error(), "not supported") ||
		   strings.Contains(err.Error(), "does not exist") {
			return false, nil
		}
		// Other errors might indicate the block doesn't exist, but tracing is supported
		return true, nil
	}
	return true, nil
}

// TraceBlock fetches all traces for a block using debug_traceBlockByNumber
func (c *Client) TraceBlock(ctx context.Context, blockNumber uint64) (map[string]*CallFrame, error) {
	blockHex := fmt.Sprintf("0x%x", blockNumber)

	var results []struct {
		TxHash string    `json:"txHash"`
		Result CallFrame `json:"result"`
	}

	err := c.raw.CallContext(ctx, &results, "debug_traceBlockByNumber", blockHex, &TraceConfig{Tracer: "callTracer"})
	if err != nil {
		return nil, fmt.Errorf("debug_traceBlockByNumber failed: %w", err)
	}

	traces := make(map[string]*CallFrame)
	for _, r := range results {
		result := r.Result // Copy to avoid pointer issues
		traces[r.TxHash] = &result
	}

	return traces, nil
}

// TraceTransaction fetches traces for a single transaction
func (c *Client) TraceTransaction(ctx context.Context, txHash common.Hash) (*CallFrame, error) {
	var result CallFrame

	err := c.raw.CallContext(ctx, &result, "debug_traceTransaction", txHash.Hex(), &TraceConfig{Tracer: "callTracer"})
	if err != nil {
		return nil, fmt.Errorf("debug_traceTransaction failed: %w", err)
	}

	return &result, nil
}

// FlattenCallFrame converts a nested CallFrame into flat InternalTransaction records
func FlattenCallFrame(frame *CallFrame, txHash string, blockNumber uint64, timestamp *uint64) []*explorerTypes.InternalTransaction {
	var result []*explorerTypes.InternalTransaction
	flattenCallFrameRecursive(frame, txHash, blockNumber, timestamp, []int{}, &result)
	return result
}

func flattenCallFrameRecursive(frame *CallFrame, txHash string, blockNumber uint64, timestamp *uint64, path []int, result *[]*explorerTypes.InternalTransaction) {
	// Build trace address string (e.g., "0,1,2")
	traceAddr := ""
	if len(path) > 0 {
		parts := make([]string, len(path))
		for i, p := range path {
			parts[i] = strconv.Itoa(p)
		}
		traceAddr = strings.Join(parts, ",")
	}

	// Convert value to string
	value := "0"
	if frame.Value != nil {
		value = (*big.Int)(frame.Value).String()
	}

	// Determine call type
	callType := strings.ToLower(frame.Type)
	switch callType {
	case "call", "delegatecall", "staticcall", "create", "create2":
		// Valid call types
	default:
		callType = "call"
	}

	// Build internal transaction
	var to *string
	if frame.To != "" {
		toAddr := frame.To
		to = &toAddr
	}

	var gas, gasUsed *uint64
	if frame.Gas > 0 {
		g := uint64(frame.Gas)
		gas = &g
	}
	if frame.GasUsed > 0 {
		gu := uint64(frame.GasUsed)
		gasUsed = &gu
	}

	var input, output, errStr *string
	if frame.Input != "" && frame.Input != "0x" {
		input = &frame.Input
	}
	if frame.Output != "" && frame.Output != "0x" {
		output = &frame.Output
	}
	if frame.Error != "" {
		errStr = &frame.Error
	}

	internalTx := &explorerTypes.InternalTransaction{
		TxHash:       txHash,
		BlockNumber:  blockNumber,
		TraceAddress: traceAddr,
		From:         frame.From,
		To:           to,
		Value:        explorerTypes.JSONString(value),
		Gas:          gas,
		GasUsed:      gasUsed,
		Input:        input,
		Output:       output,
		CallType:     callType,
		Error:        errStr,
		Timestamp:    timestamp,
	}

	*result = append(*result, internalTx)

	// Recursively process child calls
	for i, child := range frame.Calls {
		childPath := append(append([]int{}, path...), i)
		flattenCallFrameRecursive(&child, txHash, blockNumber, timestamp, childPath, result)
	}
}

// TraceResult holds the result of a trace fetch
type TraceResult struct {
	TxHash       string
	InternalTxs  []*explorerTypes.InternalTransaction
	Err          error
}

// FetchTracesBatch fetches traces for multiple transactions in parallel with rate limiting
func (c *Client) FetchTracesBatch(ctx context.Context, txHashes []common.Hash, blockNumber uint64, timestamp uint64, workers int, rateLimit int) ([]*explorerTypes.InternalTransaction, error) {
	if len(txHashes) == 0 {
		return nil, nil
	}

	// Try block-level tracing first (more efficient)
	blockTraces, err := c.TraceBlock(ctx, blockNumber)
	if err == nil && len(blockTraces) > 0 {
		// Successfully got block traces
		var allInternalTxs []*explorerTypes.InternalTransaction
		ts := timestamp

		for _, txHash := range txHashes {
			frame, ok := blockTraces[txHash.Hex()]
			if !ok {
				continue
			}
			internalTxs := FlattenCallFrame(frame, txHash.Hex(), blockNumber, &ts)
			// Skip the root call (already represented by the transaction itself)
			if len(internalTxs) > 1 {
				allInternalTxs = append(allInternalTxs, internalTxs[1:]...)
			}
		}

		return allInternalTxs, nil
	}

	// Fallback to per-transaction tracing
	log.Warn("block tracing failed, falling back to per-tx tracing", "block", blockNumber, "error", err)

	limiter := rate.NewLimiter(rate.Limit(rateLimit), rateLimit/10+1)
	results := make([]*explorerTypes.InternalTransaction, 0)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(workers)

	ts := timestamp

	for _, hash := range txHashes {
		hash := hash
		g.Go(func() error {
			if err := limiter.Wait(ctx); err != nil {
				return nil // Don't fail the whole batch
			}

			frame, err := c.TraceTransaction(ctx, hash)
			if err != nil {
				log.Warn("failed to trace tx", "tx", hash.Hex(), "error", err)
				return nil
			}

			internalTxs := FlattenCallFrame(frame, hash.Hex(), blockNumber, &ts)
			// Skip the root call
			if len(internalTxs) > 1 {
				mu.Lock()
				results = append(results, internalTxs[1:]...)
				mu.Unlock()
			}

			return nil
		})
	}

	g.Wait()
	return results, nil
}
