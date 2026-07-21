package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"explorer/internal/rpc"
	"explorer/internal/types"
	"explorer/internal/version"
	"explorer/pkg/eth/common"
	"explorer/pkg/log"

	"github.com/go-chi/chi/v5"
)

const sourcifyAPIBase = "https://sourcify.dev/server"

const defaultLimit = 25

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.provider.GetChainStats(r.Context())
	if err != nil {
		// Return minimal stats rather than failing — privacyEnabled must still be
		// set correctly so the frontend shows the login button even when the
		// explorer data store is unavailable.
		stats = &types.ChainStats{}
	}
	stats.PrivacyEnabled = s.privacyClient != nil

	// In privacy mode, totalTransactions / totalAddresses flow through from
	// the privacy proxy (RBAC-scoped per caller). totalBlocks and
	// avgBlockTime are public chain-level facts, so we pull them straight
	// from the node via RPC instead of relying on the proxy's view.
	if stats.PrivacyEnabled && s.rpc != nil {
		if total, avg, ok := publicChainFactsFromRPC(r.Context(), s.rpc); ok {
			stats.TotalBlocks = total
			if avg > 0 {
				stats.AvgBlockTime = avg
			}
		}
	}
	writeJSON(w, stats)
}

// publicChainFactsFromRPC returns chain-height-based stats that the
// privacy proxy's /explorer/stats endpoint may not surface usefully.
// Calls go through the same privacy proxy (its RPC forwarder), so the
// proxy still applies its group policy on a per-method basis. If any
// call is denied or the sample is implausible, ok=false and the caller
// leaves both upstream fields untouched (all-or-nothing).
//
// Genesis (block 0) is deliberately excluded — dev chains (anvil,
// hardhat) frequently leave its timestamp at 0, which would otherwise
// turn the average into (now-0)/N — a wildly wrong number.
const (
	avgBlockSampleSize uint64  = 100
	avgBlockTimeMaxSec float64 = 3600 // 1h/block is the upper sanity bound
)

func publicChainFactsFromRPC(ctx context.Context, c *rpc.Client) (totalBlocks int64, avgBlockTime float64, ok bool) {
	latest, err := c.BlockNumber(ctx)
	if err != nil {
		log.Debug("public chain facts: eth_blockNumber denied or failed", "error", err)
		return 0, 0, false
	}
	// totalBlocks counts genesis as well.
	totalBlocks = int64(latest) + 1

	// Need at least block 1 to sample without touching genesis.
	if latest < 2 {
		return totalBlocks, 0, true
	}
	earliestSample := uint64(1)
	if latest > avgBlockSampleSize {
		earliestSample = latest - avgBlockSampleSize
	}
	window := latest - earliestSample

	latestTS, err := c.RawBlockTimestamp(ctx, latest)
	if err != nil {
		log.Debug("public chain facts: latest block timestamp denied or failed", "block", latest, "error", err)
		return 0, 0, false
	}
	earlierTS, err := c.RawBlockTimestamp(ctx, earliestSample)
	if err != nil {
		log.Debug("public chain facts: earlier block timestamp denied or failed", "block", earliestSample, "error", err)
		return 0, 0, false
	}
	if latestTS <= earlierTS {
		return totalBlocks, 0, true
	}
	avg := float64(latestTS-earlierTS) / float64(window)
	if avg <= 0 || avg > avgBlockTimeMaxSec {
		// Implausible — happens on dev chains that initialise early
		// block timestamps to 0. Skip the override and let upstream win.
		return totalBlocks, 0, true
	}
	return totalBlocks, avg, true
}

func (s *Server) handleGetPrice(w http.ResponseWriter, r *http.Request) {
	if s.price == nil {
		http.Error(w, "price service not available", http.StatusServiceUnavailable)
		return
	}

	priceData, err := s.price.GetPrice(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, priceData)
}

func (s *Server) handleGetGasPrices(w http.ResponseWriter, r *http.Request) {
	if !s.gasPricesEnabled {
		writeJSON(w, map[string]any{
			"enabled":   false,
			"slow":      nil,
			"normal":    nil,
			"fast":      nil,
			"updatedAt": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	slow, normal, fast, baseFee, err := s.provider.GetGasPrices(r.Context(), 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"enabled":   true,
		"slow":      gweiPriceFromWei(slow),
		"normal":    gweiPriceFromWei(normal),
		"fast":      gweiPriceFromWei(fast),
		"baseFee":   gweiPriceFromWei(baseFee),
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func gweiPriceFromWei(wei *uint64) map[string]any {
	if wei == nil {
		return nil
	}
	return map[string]any{"price": float64(*wei) / 1e9}
}

func (s *Server) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	blocks, err := s.provider.GetBlocks(r.Context(), limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := paginate(blocks, limit, func(b types.Block) string {
		return strconv.FormatUint(b.Number, 10)
	})
	resp.AddressInfo = s.enrichBlockAddresses(r.Context(), resp.Data)
	writeJSON(w, resp)
}

func (s *Server) enrichBlockAddresses(ctx context.Context, blocks []types.Block) map[string]types.RowAddressInfo {
	addrs := make([]string, 0, len(blocks))
	for _, b := range blocks {
		if b.Miner != "" {
			addrs = append(addrs, b.Miner)
		}
	}
	return s.lookupAddressInfo(ctx, addrs)
}

func (s *Server) handleGetLatestBlock(w http.ResponseWriter, r *http.Request) {
	blocks, err := s.provider.GetBlocks(r.Context(), 1, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(blocks) == 0 {
		http.Error(w, "no blocks found", http.StatusNotFound)
		return
	}

	writeJSON(w, blocks[0])
}

func (s *Server) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	numberStr := chi.URLParam(r, "number")
	number, err := strconv.ParseUint(numberStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid block number", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	block, err := s.provider.GetBlock(ctx, number)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if block == nil {
		if err := s.provider.IndexBlock(ctx, number); err != nil {
			http.Error(w, "block not found", http.StatusNotFound)
			return
		}
		block, err = s.provider.GetBlock(ctx, number)
		if err != nil || block == nil {
			http.Error(w, "block not found after indexing", http.StatusInternalServerError)
			return
		}
	}

	txs, err := s.provider.GetTransactionsByBlock(ctx, number)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"block":        block,
		"transactions": txs,
	})
}

func (s *Server) handleGetBlockInternalTxs(w http.ResponseWriter, r *http.Request) {
	numberStr := chi.URLParam(r, "number")
	number, err := strconv.ParseUint(numberStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid block number", http.StatusBadRequest)
		return
	}

	internalTxs, err := s.provider.GetInternalTransactionsByBlock(r.Context(), number)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if internalTxs == nil {
		internalTxs = []types.InternalTransaction{}
	}

	writeJSON(w, internalTxs)
}

func (s *Server) handleGetTransactions(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	if pageStr != "" {
		s.handleGetTransactionsPaginated(w, r)
		return
	}

	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	txs, err := s.provider.GetTransactionsWithCategories(r.Context(), limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	resp := paginate(txs, limit, func(tx types.Transaction) string {
		return strconv.FormatUint(tx.BlockNumber, 10)
	})
	resp.AddressInfo = s.enrichTxAddresses(r.Context(), resp.Data)
	writeJSON(w, resp)
}

func (s *Server) enrichTxAddresses(ctx context.Context, txs []types.Transaction) map[string]types.RowAddressInfo {
	addrs := make([]string, 0, 3*len(txs))
	for _, tx := range txs {
		if tx.From != "" {
			addrs = append(addrs, tx.From)
		}
		if tx.To != nil && *tx.To != "" {
			addrs = append(addrs, *tx.To)
		}
		if tx.ContractAddress != nil && *tx.ContractAddress != "" {
			addrs = append(addrs, *tx.ContractAddress)
		}
	}
	return s.lookupAddressInfo(ctx, addrs)
}

// lookupAddressInfo returns one entry per unique address — including EOAs
// as {isContract: false} — so the frontend can distinguish "enrichment ran,
// EOA" from "no enrichment". Fan-out is deduped + cached by CachingProvider.
func (s *Server) lookupAddressInfo(ctx context.Context, addrs []string) map[string]types.RowAddressInfo {
	if len(addrs) == 0 {
		return nil
	}

	unique := make(map[string]struct{}, len(addrs))
	for _, a := range addrs {
		unique[strings.ToLower(a)] = struct{}{}
	}

	type entry struct {
		key  string
		info types.RowAddressInfo
	}
	resCh := make(chan entry, len(unique))
	var wg sync.WaitGroup
	for addr := range unique {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			contract, err := s.provider.GetContract(ctx, addr)
			info := types.RowAddressInfo{}
			if err == nil && contract != nil {
				info.IsContract = true
				if contract.IsVerified && contract.ContractName != nil {
					info.Name = *contract.ContractName
				}
			}
			resCh <- entry{key: addr, info: info}
		}(addr)
	}
	wg.Wait()
	close(resCh)

	out := make(map[string]types.RowAddressInfo, len(unique))
	for e := range resCh {
		out[e.key] = e.info
	}
	return out
}

func (s *Server) handleGetTransactionsPaginated(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	txs, total, err := s.provider.GetTransactionsPaginatedWithCategories(r.Context(), page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if txs == nil {
		txs = []types.Transaction{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.Transaction]{
		Data:       txs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	ctx := r.Context()

	tx, err := s.provider.GetTransactionWithCategories(ctx, hash)
	if err != nil {
		if errors.Is(err, errProviderNotFound) {
			// In privacy mode a 404 is deliberately ambiguous: the transaction
			// may exist but be outside the caller's visibility. Do not probe the
			// node or trigger reindexing, which would turn that opaque denial into
			// an existence/timing oracle.
			if _, callerScoped := s.provider.(CallerScopedProvider); callerScoped {
				http.Error(w, "transaction not found", http.StatusNotFound)
				return
			}
			tx = nil
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if tx == nil {
		_, blockNumber, err := s.provider.GetTransactionByHashRPC(ctx, hash)
		if err != nil || blockNumber == nil {
			http.Error(w, "transaction not found", http.StatusNotFound)
			return
		}

		if err := s.provider.IndexBlock(ctx, *blockNumber); err != nil {
			http.Error(w, "failed to index block: "+err.Error(), http.StatusInternalServerError)
			return
		}

		tx, err = s.provider.GetTransactionWithCategories(ctx, hash)
		if err != nil || tx == nil {
			http.Error(w, "transaction not found after indexing", http.StatusInternalServerError)
			return
		}
	}

	if tx.TxType == types.TxTypeDeposit {
		deposit, err := s.provider.GetOPDeposit(ctx, tx.Hash)
		if err == nil && deposit != nil {
			writeJSON(w, types.TransactionWithDeposit{
				Transaction: *tx,
				OPDeposit:   deposit,
			})
			return
		}
	}

	writeJSON(w, tx)
}

func (s *Server) handleGetAddress(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	ctx := r.Context()

	stats, err := s.provider.GetAddressStats(ctx, address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balance, err := s.provider.GetBalance(ctx, address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	code, err := s.provider.GetCode(ctx, address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info := &types.AddressInfo{
		Address:            address,
		Balance:            *balance,
		TxCount:            stats.TxCount,
		InternalTxCount:    stats.InternalTxCount,
		TokenTransferCount: stats.TokenTransferCount,
		IsContract:         len(code) > 0 && string(code) != "0x",
	}

	writeJSON(w, info)
}

func (s *Server) handleGetAddressTransactions(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	txs, err := s.provider.GetTransactionsByAddress(r.Context(), address, limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(txs, limit, func(tx types.Transaction) string {
		return strconv.FormatUint(tx.BlockNumber, 10)
	}))
}

func (s *Server) handleGetContract(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	contract, err := s.provider.GetContract(r.Context(), address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if contract == nil {
		http.Error(w, "contract not found", http.StatusNotFound)
		return
	}

	writeJSON(w, contract)
}

func (s *Server) handleUpdateContractABI(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	var req struct {
		ABI json.RawMessage `json:"abi"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.ABI) == 0 {
		http.Error(w, "abi is required", http.StatusBadRequest)
		return
	}

	var abiArray []json.RawMessage
	if err := json.Unmarshal(req.ABI, &abiArray); err != nil {
		http.Error(w, "abi must be a valid JSON array", http.StatusBadRequest)
		return
	}

	if err := s.provider.UpdateContractABI(r.Context(), address, req.ABI); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{"success": true, "address": address})
}

func (s *Server) handleGetAddressTransfers(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	transfers, err := s.provider.GetTransfersByAddress(r.Context(), address, limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(transfers, limit, func(tt types.TokenTransfer) string {
		return strconv.FormatUint(tt.BlockNumber, 10)
	}))
}

func (s *Server) handleGetTransactionTransfers(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	transfers, err := s.provider.GetTransfersByTransaction(r.Context(), hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, transfers)
}

func (s *Server) handleGetTransactionLogs(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	logs, err := s.provider.GetLogsByTransaction(r.Context(), hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []types.Log{}
	}

	writeJSON(w, logs)
}

func (s *Server) handleGetAccounts(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	ctx := r.Context()

	accounts, total, err := s.provider.GetAccountsPaginated(ctx, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var accountList []types.AccountListItem
	for _, acc := range accounts {
		balance, err := s.provider.GetBalance(ctx, acc.Address)
		if err != nil {
			zero := types.JSONString("0")
			balance = &zero
		}
		isContract, _ := s.provider.IsContract(ctx, acc.Address)
		accountList = append(accountList, types.AccountListItem{
			Address:    acc.Address,
			Balance:    *balance,
			TxCount:    acc.TxCount,
			IsContract: isContract,
		})
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.AccountListItem]{
		Data:       accountList,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetTransactionHistory(w http.ResponseWriter, r *http.Request) {
	intervalStr := r.URL.Query().Get("interval")
	limitStr := r.URL.Query().Get("limit")

	interval := 60
	if intervalStr != "" {
		if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
			interval = i
		}
	}

	limit := 30
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	history, err := s.provider.GetTransactionHistory(r.Context(), interval, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if history == nil {
		history = []types.TxHistoryPoint{}
	}

	writeJSON(w, history)
}

func (s *Server) handleSearchSuggestions(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) < 1 {
		writeJSON(w, types.SearchSuggestionsResponse{Query: q, Suggestions: []types.SearchSuggestion{}})
		return
	}

	suggestions, err := s.provider.SearchSuggestions(r.Context(), q, 10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if suggestions == nil {
		suggestions = []types.SearchSuggestion{}
	}

	writeJSON(w, types.SearchSuggestionsResponse{
		Query:       q,
		Suggestions: suggestions,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "query required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if num, err := strconv.ParseUint(q, 10, 64); err == nil {
		block, err := s.provider.GetBlock(ctx, num)
		if err == nil && block != nil {
			writeJSON(w, map[string]any{"type": "block", "data": block})
			return
		}
		if err := s.provider.IndexBlock(ctx, num); err == nil {
			block, _ = s.provider.GetBlock(ctx, num)
			if block != nil {
				writeJSON(w, map[string]any{"type": "block", "data": block})
				return
			}
		}
	}

	if strings.HasPrefix(q, "0x") && len(q) == 66 {
		tx, err := s.provider.GetTransaction(ctx, q)
		if err == nil && tx != nil {
			writeJSON(w, map[string]any{"type": "transaction", "data": tx})
			return
		}
		_, blockNumber, err := s.provider.GetTransactionByHashRPC(ctx, q)
		if err == nil && blockNumber != nil {
			if err := s.provider.IndexBlock(ctx, *blockNumber); err == nil {
				tx, _ = s.provider.GetTransaction(ctx, q)
				if tx != nil {
					writeJSON(w, map[string]any{"type": "transaction", "data": tx})
					return
				}
			}
		}
	}

	if common.IsHexAddress(q) {
		address := common.HexToAddress(q).Hex()

		// No separate visibility check needed — in privacy mode the
		// ProxyDataProvider already filters results through the proxy.
		// If GetAddressStats returns data, the user is allowed to see it.
		stats, _ := s.provider.GetAddressStats(ctx, address)
		writeJSON(w, map[string]any{"type": "address", "data": stats})
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func (s *Server) handleGetChainInfo(w http.ResponseWriter, r *http.Request) {
	if s.chainInfo == nil {
		http.Error(w, "chain info not available", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, s.chainInfo.Get())
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func parseLimit(r *http.Request) int {
	if l := r.URL.Query().Get("limit"); l != "" {
		if limit, err := strconv.Atoi(l); err == nil && limit > 0 && limit <= 100 {
			return limit
		}
	}
	return defaultLimit
}

func parseOffset(r *http.Request) int {
	if o := r.URL.Query().Get("offset"); o != "" {
		if offset, err := strconv.Atoi(o); err == nil && offset >= 0 {
			return offset
		}
	}
	return 0
}

func parseBeforeBlock(r *http.Request) *uint64 {
	if b := r.URL.Query().Get("before"); b != "" {
		if before, err := strconv.ParseUint(b, 10, 64); err == nil {
			return &before
		}
	}
	return nil
}

// paginate trims an over-fetched (limit+1) slice and emits the next-page
// cursor. BUG-7: the cursor was strconv.Itoa(len(items)) — the page length,
// useless as a ?before= value (paging is by block number). cursorFn extracts
// the real cursor (the last returned row's block number) so ?before= pages
// forward with no dupes/gaps.
func paginate[T any](items []T, limit int, cursorFn func(T) string) types.PaginatedResponse[T] {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor *string
	if hasMore && len(items) > 0 {
		cursor := cursorFn(items[len(items)-1])
		nextCursor = &cursor
	}

	return types.PaginatedResponse[T]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	_, err := s.provider.GetLatestBlockNumber(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"status": "unhealthy", "database": "down", "error": err.Error()})
		return
	}

	_, err = s.provider.GetLatestBlockNumber(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"status": "unhealthy", "rpc": "down", "error": err.Error()})
		return
	}

	writeJSON(w, map[string]any{"status": "healthy", "database": "up", "rpc": "up"})
}

func (s *Server) handleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"status": "alive"})
}

// handleVersion returns the build identity (version / commit / build time) of
// the running binary, injected via -ldflags at build time. It is unauthenticated
// by design: the explorer's own build version is surfaced in the UI footer (as
// is conventional for block explorers) and exposes no chain data.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"version":   version.Version,
		"commit":    version.Commit,
		"buildTime": version.BuildTime,
	})
}

func (s *Server) handleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	latestBlock, err := s.provider.GetLatestBlockNumber(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"ready": false, "reason": "database unavailable"})
		return
	}

	if latestBlock == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"ready": false, "reason": "no blocks indexed yet"})
		return
	}

	writeJSON(w, map[string]any{"ready": true, "lastIndexedBlock": latestBlock})
}

func (s *Server) handleGetTokens(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")
	tokenType := NormalizeTokenType(r.URL.Query().Get("type"))
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize
	tokens, total, err := s.provider.GetTokens(r.Context(), pageSize, offset, tokenType, search)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if tokens == nil {
		tokens = []types.Token{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.Token]{
		Data:       tokens,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetToken(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	token, err := s.provider.GetToken(r.Context(), address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if token == nil {
		http.Error(w, "token not found", http.StatusNotFound)
		return
	}

	writeJSON(w, token)
}

func (s *Server) handleGetTokenHolders(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize
	holders, total, err := s.provider.GetTokenHolders(r.Context(), address, pageSize, offset)
	if err != nil {
		log.Warn("token holders lookup failed", "error", err) // P-3: address omitted (PII in privacy mode)
		http.Error(w, "failed to get token holders", http.StatusInternalServerError)
		return
	}

	if holders == nil {
		holders = []types.TokenHolder{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.TokenHolder]{
		Data:       holders,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetTokenInventory(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Optional: filter to a single token id (powers the per-NFT detail page).
	tokenID := r.URL.Query().Get("tokenId")

	offset := (page - 1) * pageSize
	items, total, err := s.provider.GetTokenInventory(r.Context(), address, tokenID, pageSize, offset)
	if err != nil {
		log.Warn("token inventory lookup failed", "error", err) // P-3: address omitted (PII in privacy mode)
		http.Error(w, "failed to get token inventory", http.StatusInternalServerError)
		return
	}

	if items == nil {
		items = []types.TokenInventoryItem{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.TokenInventoryItem]{
		Data:       items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetTokenTransfers(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize
	transfers, total, err := s.provider.GetTransfersByToken(r.Context(), address, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if transfers == nil {
		transfers = []types.TokenTransfer{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.TokenTransfer]{
		Data:       transfers,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// NormalizeTokenType canonicalises a user-supplied token-standard filter to
// "ERC20"/"ERC721"/"ERC1155", or "" for anything unrecognised (= all).
func NormalizeTokenType(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "ERC20":
		return "ERC20"
	case "ERC721":
		return "ERC721"
	case "ERC1155":
		return "ERC1155"
	default:
		return ""
	}
}

func (s *Server) handleGetAllTransfers(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Optional token-standard filter (ERC20/ERC721/ERC1155); empty = all.
	tokenType := NormalizeTokenType(r.URL.Query().Get("type"))

	offset := (page - 1) * pageSize
	transfers, total, err := s.provider.GetAllTransfers(r.Context(), tokenType, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if transfers == nil {
		transfers = []types.TokenTransfer{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.TokenTransfer]{
		Data:       transfers,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetTransactionInternalTxs(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	internalTxs, err := s.provider.GetInternalTransactionsByTx(r.Context(), hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if internalTxs == nil {
		internalTxs = []types.InternalTransaction{}
	}

	writeJSON(w, internalTxs)
}

func (s *Server) handleGetAddressInternalTxs(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize
	internalTxs, total, err := s.provider.GetInternalTransactionsByAddress(r.Context(), address, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if internalTxs == nil {
		internalTxs = []types.InternalTransaction{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.InternalTransaction]{
		Data:       internalTxs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	addressStr := r.URL.Query().Get("address")
	topic0Str := r.URL.Query().Get("topic0")
	fromBlockStr := r.URL.Query().Get("fromBlock")
	toBlockStr := r.URL.Query().Get("toBlock")
	limitStr := r.URL.Query().Get("limit")

	var address, topic0 *string
	var fromBlock, toBlock *uint64

	if addressStr != "" && common.IsHexAddress(addressStr) {
		addr := common.HexToAddress(addressStr).Hex()
		address = &addr
	}
	if topic0Str != "" {
		topic0 = &topic0Str
	}
	if fromBlockStr != "" {
		if fb, err := strconv.ParseUint(fromBlockStr, 10, 64); err == nil {
			fromBlock = &fb
		}
	}
	if toBlockStr != "" {
		if tb, err := strconv.ParseUint(toBlockStr, 10, 64); err == nil {
			toBlock = &tb
		}
	}

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	logs, err := s.provider.GetLogs(r.Context(), address, topic0, fromBlock, toBlock, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []types.Log{}
	}

	writeJSON(w, logs)
}

func (s *Server) handleGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.provider.GetSyncStatus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	latestOnChain, err := s.provider.GetLatestBlockNumber(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"syncStatus":       status,
		"latestChainBlock": latestOnChain,
		"blocksRemaining":  int64(latestOnChain) - int64(status.LastIndexedBlock),
		"isSynced":         status.LastIndexedBlock >= latestOnChain,
	})
}

func (s *Server) handleGetAddressLogs(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 25
	if pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	offset := (page - 1) * pageSize
	logs, total, err := s.provider.GetLogsByAddress(r.Context(), address, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []types.Log{}
	}

	totalPages := computeTotalPages(total, pageSize)

	writeJSON(w, types.OffsetPaginatedResponse[types.Log]{
		Data:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetAddressTokenBalances(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	balances, err := s.provider.GetTokenBalances(r.Context(), address)
	if err != nil {
		log.Warn("token balances lookup failed", "error", err) // P-3: address omitted (PII in privacy mode)
		http.Error(w, "failed to get token balances", http.StatusInternalServerError)
		return
	}

	if balances == nil {
		balances = []types.Balance{}
	}

	writeJSON(w, balances)
}

func (s *Server) handleFetchSourcify(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	chainIDStr := r.URL.Query().Get("chainId")
	if chainIDStr == "" {
		// Use the explorer's own chain ID from RPC
		chainID, err := s.provider.GetChainID(r.Context())
		if err == nil && chainID > 0 {
			chainIDStr = strconv.FormatUint(chainID, 10)
		} else {
			chainIDStr = "1"
		}
	}

	// Validate chainIDStr to prevent SSRF
	if _, err := strconv.ParseUint(chainIDStr, 10, 64); err != nil {
		http.Error(w, "invalid chainId", http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("%s/files/%s/%s", s.sourcifyBase(), chainIDStr, address)
	resp, err := s.sourcifyClient().Get(url)
	if err != nil {
		http.Error(w, "failed to fetch from Sourcify: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		http.Error(w, "contract not verified on Sourcify", http.StatusNotFound)
		return
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		http.Error(w, fmt.Sprintf("Sourcify error: %s", string(body)), resp.StatusCode)
		return
	}

	var sourcifyFiles []struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sourcifyFiles); err != nil {
		http.Error(w, "failed to parse Sourcify response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var abi json.RawMessage
	var sourceCode string
	var contractName string
	var compilerVersion string

	for _, file := range sourcifyFiles {
		if file.Name == "metadata.json" {
			var metadata struct {
				Output struct {
					ABI json.RawMessage `json:"abi"`
				} `json:"output"`
				Settings struct {
					CompilationTarget map[string]string `json:"compilationTarget"`
				} `json:"settings"`
				Compiler struct {
					Version string `json:"version"`
				} `json:"compiler"`
			}
			if err := json.Unmarshal([]byte(file.Content), &metadata); err == nil {
				abi = metadata.Output.ABI
				compilerVersion = metadata.Compiler.Version
				for _, name := range metadata.Settings.CompilationTarget {
					contractName = name
					break
				}
			}
		}
		if strings.HasSuffix(file.Name, ".sol") && sourceCode == "" {
			sourceCode = file.Content
		}
	}

	if len(abi) == 0 {
		http.Error(w, "no ABI found in Sourcify response", http.StatusNotFound)
		return
	}

	if err := s.provider.VerifyContract(r.Context(), address, contractName, compilerVersion, false, sourceCode, abi, "", "", "", 0); err != nil {
		http.Error(w, "failed to save contract: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"success":         true,
		"address":         address,
		"contractName":    contractName,
		"compilerVersion": compilerVersion,
		"abiLength":       len(abi),
	})
}

func isLocalChain(chainID string) bool {
	localChains := map[string]bool{
		"31337": true, // Hardhat/Anvil
		"1337":  true, // Ganache
		"1001":  true, // Local zkEVM
		"1002":  true, // Local zkEVM
		"1003":  true, // Local zkEVM
	}
	return localChains[chainID]
}

func (s *Server) handleVerifySourcify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address          string          `json:"address"`
		ChainID          string          `json:"chainId"`
		SourceCode       string          `json:"sourceCode"`
		ContractName     string          `json:"contractName"`
		CompilerVersion  string          `json:"compilerVersion"`
		OptimizationUsed bool            `json:"optimizationUsed"`
		Runs             int             `json:"runs"`
		ABI              json.RawMessage `json:"abi,omitempty"` // Optional: direct ABI for local chains
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !common.IsHexAddress(req.Address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}

	if req.SourceCode == "" || req.ContractName == "" {
		http.Error(w, "sourceCode and contractName are required", http.StatusBadRequest)
		return
	}

	if req.ChainID == "" {
		req.ChainID = "1"
	}

	// For local/dev chains, store verification directly without Sourcify
	if isLocalChain(req.ChainID) {
		var abi json.RawMessage
		if len(req.ABI) > 0 {
			abi = req.ABI
		} else {
			abi = json.RawMessage("[]")
		}

		if err := s.provider.VerifyContract(r.Context(), req.Address, req.ContractName, req.CompilerVersion, req.OptimizationUsed, req.SourceCode, abi, "", "", "", 0); err != nil {
			http.Error(w, "failed to store verification: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{
			"success": true,
			"status":  "local",
			"address": req.Address,
			"message": "Contract verified locally (development chain)",
		})
		return
	}

	sourcifyReq := map[string]any{
		"address": req.Address,
		"chain":   req.ChainID,
		"files": map[string]string{
			req.ContractName + ".sol": req.SourceCode,
		},
	}

	body, _ := json.Marshal(sourcifyReq)
	resp, err := s.sourcifyClient().Post(s.sourcifyBase()+"/verify", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to submit to Sourcify: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		http.Error(w, fmt.Sprintf("Sourcify verification failed: %s", string(respBody)), resp.StatusCode)
		return
	}

	var result struct {
		Result []struct {
			Address string `json:"address"`
			Status  string `json:"status"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		http.Error(w, "failed to parse Sourcify response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(result.Result) == 0 {
		http.Error(w, "no verification result returned", http.StatusInternalServerError)
		return
	}

	status := result.Result[0].Status
	if status != "perfect" && status != "partial" {
		http.Error(w, fmt.Sprintf("verification failed: %s", status), http.StatusBadRequest)
		return
	}

	fetchURL := fmt.Sprintf("%s/files/%s/%s", s.sourcifyBase(), req.ChainID, common.HexToAddress(req.Address).Hex())
	fetchResp, err := s.sourcifyClient().Get(fetchURL)
	if err == nil && fetchResp.StatusCode == 200 {
		defer fetchResp.Body.Close()
		var sourcifyFiles []struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if json.NewDecoder(fetchResp.Body).Decode(&sourcifyFiles) == nil {
			var abi json.RawMessage
			var compilerVersion string
			for _, file := range sourcifyFiles {
				if file.Name == "metadata.json" {
					var metadata struct {
						Output struct {
							ABI json.RawMessage `json:"abi"`
						} `json:"output"`
						Compiler struct {
							Version string `json:"version"`
						} `json:"compiler"`
					}
					if json.Unmarshal([]byte(file.Content), &metadata) == nil {
						abi = metadata.Output.ABI
						compilerVersion = metadata.Compiler.Version
					}
				}
			}
			if len(abi) > 0 {
				s.provider.VerifyContract(r.Context(), req.Address, req.ContractName, compilerVersion, req.OptimizationUsed, req.SourceCode, abi, "", "", "", 0)
			}
		}
	}

	writeJSON(w, map[string]any{
		"success": true,
		"status":  status,
		"address": req.Address,
	})
}

func (s *Server) handleVerifyContract(w http.ResponseWriter, r *http.Request) {
	if s.verifier == nil {
		http.Error(w, "local verification not configured (solc not available)", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	result, err := s.verifier.VerifyFromJSON(r.Context(), body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (s *Server) handleVerifyStandardJSON(w http.ResponseWriter, r *http.Request) {
	if s.verifier == nil {
		http.Error(w, "local verification not configured (solc not available)", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	result, err := s.verifier.VerifyStandardJSON(r.Context(), body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (s *Server) handleListCompilers(w http.ResponseWriter, r *http.Request) {
	if s.verifier == nil {
		writeJSON(w, map[string]any{
			"versions": []string{},
			"message":  "local verification not configured",
		})
		return
	}

	versions := s.verifier.ListCompilers()
	writeJSON(w, map[string]any{
		"versions": versions,
	})
}

func (s *Server) handleCheckSourcify(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	chainIDStr := r.URL.Query().Get("chainId")
	if chainIDStr == "" {
		// Use the explorer's own chain ID from RPC
		chainID, err := s.provider.GetChainID(r.Context())
		if err == nil && chainID > 0 {
			chainIDStr = strconv.FormatUint(chainID, 10)
		} else {
			chainIDStr = "1"
		}
	}

	if isLocalChain(chainIDStr) {
		contract, err := s.provider.GetContract(r.Context(), address)
		if err != nil {
			http.Error(w, "failed to check contract: "+err.Error(), http.StatusInternalServerError)
			return
		}

		isVerified := contract != nil && contract.IsVerified
		status := "not verified"
		if isVerified {
			status = "local"
		}

		writeJSON(w, map[string]any{
			"address":    address,
			"chainId":    chainIDStr,
			"isVerified": isVerified,
			"status":     status,
		})
		return
	}

	// Validate chainIDStr to prevent SSRF
	if _, err := strconv.ParseUint(chainIDStr, 10, 64); err != nil {
		http.Error(w, "invalid chainId", http.StatusBadRequest)
		return
	}

	url := fmt.Sprintf("%s/check-by-addresses?addresses=%s&chainIds=%s", s.sourcifyBase(), address, chainIDStr)
	resp, err := s.sourcifyClient().Get(url)
	if err != nil {
		http.Error(w, "failed to check Sourcify: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var result []struct {
		Address  string `json:"address"`
		ChainIds []struct {
			ChainId string `json:"chainId"`
			Status  string `json:"status"`
		} `json:"chainIds"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		http.Error(w, "failed to parse Sourcify response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	isVerified := false
	status := "not verified"
	if len(result) > 0 && len(result[0].ChainIds) > 0 {
		status = result[0].ChainIds[0].Status
		isVerified = status == "perfect" || status == "partial"
	}

	writeJSON(w, map[string]any{
		"address":    address,
		"chainId":    chainIDStr,
		"isVerified": isVerified,
		"status":     status,
	})
}

// --- Charts ---

// GetChartLineDefinitions returns all available chart line definitions.
func GetChartLineDefinitions() []types.ChartLineInfo {
	return chartLineDefinitions
}

// GetChartLineInfoMap returns a map of chart ID to ChartLineInfo.
func GetChartLineInfoMap() map[string]types.ChartLineInfo {
	return chartLineInfoMap()
}

// ExtractChartDataPoints maps daily stats to chart data points for the given chart ID.
func ExtractChartDataPoints(id string, stats []types.DailyStats) []types.ChartDataPoint {
	return extractChartDataPoints(id, stats)
}

// computeTotalPages returns ceil(total/pageSize). BUG-1: the previous inline
// form `int(total)/pageSize` narrowed an int64 total to platform int BEFORE
// dividing, truncating (and possibly going negative) on 32-bit builds for a
// total above math.MaxInt32 — leaving TotalPages inconsistent with the int64
// Total in the same envelope. Doing the division in int64 and narrowing only
// the small quotient is platform-independent. pageSize<=0 yields 0 (handlers
// clamp it; this is a defensive guard against divide-by-zero).
func computeTotalPages(total int64, pageSize int) int {
	if pageSize <= 0 || total <= 0 {
		return 0
	}
	ps := int64(pageSize)
	pages := total / ps
	if total%ps != 0 {
		pages++
	}
	return int(pages)
}

var chartLineDefinitions = []types.ChartLineInfo{
	{ID: "active_accounts", Title: "Active accounts", Description: "Daily number of active addresses", Section: "accounts"},
	{ID: "new_accounts", Title: "New accounts", Description: "Daily number of new addresses", Section: "accounts"},
	{ID: "accounts_growth", Title: "Accounts growth", Description: "Cumulative number of addresses", Section: "accounts"},
	{ID: "new_txns", Title: "New transactions", Description: "Daily number of transactions", Section: "transactions"},
	{ID: "txns_growth", Title: "Transactions growth", Description: "Cumulative number of transactions", Section: "transactions"},
	{ID: "avg_txn_fee", Title: "Average transaction fee", Description: "Daily average gas price in Gwei", Units: "Gwei", Section: "transactions"},
	{ID: "txns_success_rate", Title: "Transaction success rate", Description: "Daily transaction success rate", Units: "%", Section: "transactions"},
	{ID: "new_blocks", Title: "New blocks", Description: "Daily number of new blocks", Section: "blocks"},
	{ID: "avg_block_size", Title: "Average block size", Description: "Daily average block size in bytes", Units: "bytes", Section: "blocks"},
	{ID: "avg_block_time", Title: "Average block time", Description: "Daily average block time in seconds", Units: "s", Section: "blocks"},
	{ID: "avg_gas_price", Title: "Average gas price", Description: "Daily average gas price in Gwei", Units: "Gwei", Section: "gas"},
	{ID: "avg_gas_used", Title: "Average gas used per block", Description: "Daily average gas used per block", Section: "gas"},
	{ID: "gas_used_growth", Title: "Gas used growth", Description: "Cumulative gas used", Section: "gas"},
	{ID: "new_contracts", Title: "New contracts", Description: "Daily number of new verified contracts", Section: "contracts"},
	{ID: "contracts_growth", Title: "Contracts growth", Description: "Cumulative number of contracts", Section: "contracts"},
	{ID: "new_token_transfers", Title: "New token transfers", Description: "Daily number of token transfers", Section: "tokens"},
}

func chartLineInfoMap() map[string]types.ChartLineInfo {
	m := make(map[string]types.ChartLineInfo, len(chartLineDefinitions))
	for _, info := range chartLineDefinitions {
		m[info.ID] = info
	}
	return m
}

func extractChartDataPoints(id string, stats []types.DailyStats) []types.ChartDataPoint {
	points := make([]types.ChartDataPoint, 0, len(stats))
	var cumulativeGas float64
	for _, s := range stats {
		var val float64
		switch id {
		case "active_accounts":
			val = float64(s.ActiveAddresses)
		case "new_accounts":
			val = float64(s.NewAddresses)
		case "accounts_growth":
			val = float64(s.CumulativeAddresses)
		case "new_txns":
			val = float64(s.TotalTransactions)
		case "txns_growth":
			val = float64(s.CumulativeTransactions)
		case "avg_txn_fee":
			val = float64(s.AvgGasPrice) / 1e9
		case "txns_success_rate":
			if s.TotalTransactions > 0 {
				val = float64(s.SuccessfulTxs) / float64(s.TotalTransactions) * 100
			}
		case "new_blocks":
			val = float64(s.TotalBlocks)
		case "avg_block_size":
			val = float64(s.AvgBlockSize)
		case "avg_block_time":
			val = s.AvgBlockTime
		case "avg_gas_price":
			val = float64(s.AvgGasPrice) / 1e9
		case "avg_gas_used":
			if s.TotalBlocks > 0 {
				val = float64(s.TotalGasUsed) / float64(s.TotalBlocks)
			}
		case "gas_used_growth":
			cumulativeGas += float64(s.TotalGasUsed)
			val = cumulativeGas
		case "new_contracts":
			val = float64(s.NewContracts)
		case "contracts_growth":
			val = float64(s.CumulativeContracts)
		case "new_token_transfers":
			val = float64(s.TokenTransferCount)
		}
		points = append(points, types.ChartDataPoint{Date: s.Date, Value: val})
	}
	return points
}

func (s *Server) handleGetChartLines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, chartLineDefinitions)
}

func (s *Server) handleGetChartLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	infoMap := chartLineInfoMap()
	info, ok := infoMap[id]
	if !ok {
		http.Error(w, "unknown chart id", http.StatusNotFound)
		return
	}

	from, to := parseChartDateRange(r)

	stats, err := s.provider.GetDailyStats(r.Context(), from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	points := extractChartDataPoints(id, stats)

	writeJSON(w, types.ChartLineResponse{
		Info:  info,
		Chart: points,
	})
}

func (s *Server) handleGetChartCounters(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterday := today.AddDate(0, 0, -1)

	stats, err := s.provider.GetDailyStats(r.Context(), yesterday, today)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	counters := map[string]any{
		"totalTransactions":   int64(0),
		"totalAddresses":      int64(0),
		"totalContracts":      int64(0),
		"todayTransactions":   0,
		"todayBlocks":         0,
		"todayTokenTransfers": 0,
		"averageBlockTime":    0.0,
	}

	for _, s := range stats {
		if s.Date == today.Format("2006-01-02") {
			counters["todayTransactions"] = s.TotalTransactions
			counters["todayBlocks"] = s.TotalBlocks
			counters["todayTokenTransfers"] = s.TokenTransferCount
			counters["averageBlockTime"] = s.AvgBlockTime
			counters["totalTransactions"] = s.CumulativeTransactions
			counters["totalAddresses"] = s.CumulativeAddresses
			counters["totalContracts"] = s.CumulativeContracts
		} else if s.Date == yesterday.Format("2006-01-02") {
			// Use yesterday's cumulative as fallback if today not yet computed
			if counters["totalTransactions"] == int64(0) {
				counters["totalTransactions"] = s.CumulativeTransactions
				counters["totalAddresses"] = s.CumulativeAddresses
				counters["totalContracts"] = s.CumulativeContracts
			}
		}
	}

	writeJSON(w, counters)
}

func parseChartDateRange(r *http.Request) (time.Time, time.Time) {
	now := time.Now().UTC()
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -30)

	if f := r.URL.Query().Get("from"); f != "" {
		if parsed, err := time.Parse("2006-01-02", f); err == nil {
			from = parsed
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			to = parsed
		}
	}
	return from, to
}
