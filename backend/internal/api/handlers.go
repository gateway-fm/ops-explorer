package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"explorer/internal/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-chi/chi/v5"
)

const sourcifyAPIBase = "https://sourcify.dev/server"

const defaultLimit = 25

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetChainStats(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
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
	if s.gasTracker == nil {
		http.Error(w, "gas tracker not available", http.StatusServiceUnavailable)
		return
	}

	prices, err := s.gasTracker.GetGasPrices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, prices)
}

func (s *Server) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	blocks, err := s.db.GetBlocks(r.Context(), limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(blocks, limit))
}

func (s *Server) handleGetLatestBlock(w http.ResponseWriter, r *http.Request) {
	blocks, err := s.db.GetBlocks(r.Context(), 1, nil)
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

	// Try database first
	block, err := s.db.GetBlock(ctx, number)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If not in DB, trigger on-demand indexing
	if block == nil {
		if err := s.indexer.IndexBlock(ctx, number); err != nil {
			http.Error(w, "block not found", http.StatusNotFound)
			return
		}
		// Now fetch from DB
		block, err = s.db.GetBlock(ctx, number)
		if err != nil || block == nil {
			http.Error(w, "block not found after indexing", http.StatusInternalServerError)
			return
		}
	}

	// Get transactions from DB
	txs, err := s.db.GetTransactionsByBlock(ctx, number)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"block":        block,
		"transactions": txs,
	})
}

func (s *Server) handleGetTransactions(w http.ResponseWriter, r *http.Request) {
	// Check if using page-based pagination
	pageStr := r.URL.Query().Get("page")
	if pageStr != "" {
		s.handleGetTransactionsPaginated(w, r)
		return
	}

	// Cursor-based pagination (legacy)
	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	// Use category-aware query
	txs, err := s.db.GetTransactionsWithCategories(r.Context(), limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(txs, limit))
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

	// Use category-aware query
	txs, total, err := s.db.GetTransactionsPaginatedWithCategories(r.Context(), page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if txs == nil {
		txs = []types.Transaction{}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

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

	// Try database first with category-aware query
	tx, err := s.db.GetTransactionWithCategories(ctx, hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If not in DB, try to find and index the block containing this tx
	if tx == nil {
		// Get tx from RPC to find its block number
		rpcTx, _, err := s.rpc.TransactionByHash(ctx, common.HexToHash(hash))
		if err != nil {
			http.Error(w, "transaction not found", http.StatusNotFound)
			return
		}

		// Get receipt to find block number
		receipt, err := s.rpc.TransactionReceipt(ctx, common.HexToHash(hash))
		if err != nil || receipt.BlockNumber == nil {
			http.Error(w, "transaction not found or pending", http.StatusNotFound)
			return
		}

		// Trigger on-demand indexing of the block
		if err := s.indexer.IndexBlock(ctx, receipt.BlockNumber.Uint64()); err != nil {
			http.Error(w, "failed to index block: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Now fetch from DB with categories
		tx, err = s.db.GetTransactionWithCategories(ctx, rpcTx.Hash().Hex())
		if err != nil || tx == nil {
			http.Error(w, "transaction not found after indexing", http.StatusInternalServerError)
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

	stats, err := s.db.GetAddressStats(ctx, address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balance, err := s.rpc.GetBalance(ctx, common.HexToAddress(address))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	code, err := s.rpc.GetCode(ctx, common.HexToAddress(address))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info := &types.AddressInfo{
		Address:    address,
		Balance:    types.JSONString(balance.String()),
		TxCount:    stats.TxCount,
		IsContract: len(code) > 0,
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

	txs, err := s.db.GetTransactionsByAddress(r.Context(), address, limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(txs, limit))
}

func (s *Server) handleGetContract(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	contract, err := s.db.GetContract(r.Context(), address)
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

	// Validate that ABI is valid JSON array
	var abiArray []json.RawMessage
	if err := json.Unmarshal(req.ABI, &abiArray); err != nil {
		http.Error(w, "abi must be a valid JSON array", http.StatusBadRequest)
		return
	}

	if err := s.db.SetContractABI(r.Context(), address, req.ABI); err != nil {
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

	transfers, err := s.db.GetTransfersByAddress(r.Context(), address, limit+1, beforeBlock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(transfers, limit))
}

func (s *Server) handleGetTransactionTransfers(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	transfers, err := s.db.GetTransfersByTransaction(r.Context(), hash)
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

	logs, err := s.db.GetLogsByTransaction(r.Context(), hash)
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

	// Get accounts from database
	accounts, total, err := s.db.GetAccountsPaginated(ctx, page, pageSize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch balances and contract status for each account
	var accountList []types.AccountListItem
	for _, acc := range accounts {
		balance, err := s.rpc.GetBalance(ctx, common.HexToAddress(acc.Address))
		if err != nil {
			balance = big.NewInt(0)
		}
		isContract, _ := s.db.IsContract(ctx, acc.Address)
		accountList = append(accountList, types.AccountListItem{
			Address:    acc.Address,
			Balance:    types.JSONString(balance.String()),
			TxCount:    acc.TxCount,
			IsContract: isContract,
		})
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, types.OffsetPaginatedResponse[types.AccountListItem]{
		Data:       accountList,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func (s *Server) handleGetTransactionHistory(w http.ResponseWriter, r *http.Request) {
	// Default: 14 days of data, grouped by day
	// For dev environments with recent data, use smaller intervals
	intervalStr := r.URL.Query().Get("interval")
	limitStr := r.URL.Query().Get("limit")

	interval := 60 // Default: 1 minute intervals for dev
	if intervalStr != "" {
		if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
			interval = i
		}
	}

	limit := 30 // Default: 30 data points
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	history, err := s.db.GetTransactionHistory(r.Context(), interval, limit)
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

	suggestions, err := s.db.SearchSuggestions(r.Context(), q, 10)
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

	// Try as block number
	if num, err := strconv.ParseUint(q, 10, 64); err == nil {
		block, err := s.db.GetBlock(ctx, num)
		if err == nil && block != nil {
			writeJSON(w, map[string]any{"type": "block", "data": block})
			return
		}
		// Try on-demand indexing
		if err := s.indexer.IndexBlock(ctx, num); err == nil {
			block, _ = s.db.GetBlock(ctx, num)
			if block != nil {
				writeJSON(w, map[string]any{"type": "block", "data": block})
				return
			}
		}
	}

	// Try as transaction hash
	if strings.HasPrefix(q, "0x") && len(q) == 66 {
		tx, err := s.db.GetTransaction(ctx, q)
		if err == nil && tx != nil {
			writeJSON(w, map[string]any{"type": "transaction", "data": tx})
			return
		}
		// Try to find and index the block containing this tx
		receipt, err := s.rpc.TransactionReceipt(ctx, common.HexToHash(q))
		if err == nil && receipt.BlockNumber != nil {
			if err := s.indexer.IndexBlock(ctx, receipt.BlockNumber.Uint64()); err == nil {
				tx, _ = s.db.GetTransaction(ctx, q)
				if tx != nil {
					writeJSON(w, map[string]any{"type": "transaction", "data": tx})
					return
				}
			}
		}
	}

	// Try as address
	if common.IsHexAddress(q) {
		address := common.HexToAddress(q).Hex()
		stats, _ := s.db.GetAddressStats(ctx, address)
		writeJSON(w, map[string]any{"type": "address", "data": stats})
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

// Helpers

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

func parseBeforeBlock(r *http.Request) *uint64 {
	if b := r.URL.Query().Get("before"); b != "" {
		if before, err := strconv.ParseUint(b, 10, 64); err == nil {
			return &before
		}
	}
	return nil
}

func paginate[T any](items []T, limit int) types.PaginatedResponse[T] {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	var nextCursor *string
	if hasMore && len(items) > 0 {
		// For simplicity, use the last item's index as cursor hint
		// In production, you'd want a proper cursor
		cursor := strconv.Itoa(len(items))
		nextCursor = &cursor
	}

	return types.PaginatedResponse[T]{
		Data:       items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

// Health check endpoints

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check database connectivity
	_, err := s.db.GetLatestBlockNumber(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"status": "unhealthy", "database": "down", "error": err.Error()})
		return
	}

	// Check RPC connectivity
	_, err = s.rpc.BlockNumber(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"status": "unhealthy", "rpc": "down", "error": err.Error()})
		return
	}

	writeJSON(w, map[string]any{"status": "healthy", "database": "up", "rpc": "up"})
}

func (s *Server) handleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	// Simple liveness check - just return OK if the server is running
	writeJSON(w, map[string]any{"status": "alive"})
}

func (s *Server) handleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check database connectivity
	latestBlock, err := s.db.GetLatestBlockNumber(ctx)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"ready": false, "reason": "database unavailable"})
		return
	}

	// Check if we have at least one block indexed
	if latestBlock == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"ready": false, "reason": "no blocks indexed yet"})
		return
	}

	writeJSON(w, map[string]any{"ready": true, "lastIndexedBlock": latestBlock})
}

// Token endpoints

func (s *Server) handleGetTokens(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")
	tokenType := r.URL.Query().Get("type")

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
	tokens, total, err := s.db.GetTokens(r.Context(), pageSize, offset, tokenType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if tokens == nil {
		tokens = []types.Token{}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

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

	token, err := s.db.GetToken(r.Context(), address)
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
	holders, total, err := s.db.GetTokenHolders(r.Context(), address, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if holders == nil {
		holders = []types.TokenHolder{}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, types.OffsetPaginatedResponse[types.TokenHolder]{
		Data:       holders,
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
	transfers, total, err := s.db.GetTransfersByToken(r.Context(), address, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if transfers == nil {
		transfers = []types.TokenTransfer{}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, types.OffsetPaginatedResponse[types.TokenTransfer]{
		Data:       transfers,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
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

	offset := (page - 1) * pageSize
	transfers, total, err := s.db.GetAllTransfers(r.Context(), pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if transfers == nil {
		transfers = []types.TokenTransfer{}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, types.OffsetPaginatedResponse[types.TokenTransfer]{
		Data:       transfers,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// Internal transaction endpoints

func (s *Server) handleGetTransactionInternalTxs(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	internalTxs, err := s.db.GetInternalTransactionsByTx(r.Context(), hash)
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
	internalTxs, total, err := s.db.GetInternalTransactionsByAddress(r.Context(), address, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if internalTxs == nil {
		internalTxs = []types.InternalTransaction{}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, types.OffsetPaginatedResponse[types.InternalTransaction]{
		Data:       internalTxs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// Log filtering endpoint

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

	logs, err := s.db.GetLogs(r.Context(), address, topic0, fromBlock, toBlock, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []types.Log{}
	}

	writeJSON(w, logs)
}

// Sync status endpoint

func (s *Server) handleGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.db.GetSyncStatus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get latest block on chain for comparison
	latestOnChain, err := s.rpc.BlockNumber(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"syncStatus":        status,
		"latestChainBlock":  latestOnChain,
		"blocksRemaining":   int64(latestOnChain) - int64(status.LastIndexedBlock),
		"isSynced":          status.LastIndexedBlock >= latestOnChain,
	})
}

// Address logs endpoint

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
	logs, total, err := s.db.GetLogsByAddress(r.Context(), address, pageSize, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []types.Log{}
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	writeJSON(w, types.OffsetPaginatedResponse[types.Log]{
		Data:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// Address token balances endpoint

func (s *Server) handleGetAddressTokenBalances(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	balances, err := s.db.GetTokenBalances(r.Context(), address)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if balances == nil {
		balances = []types.Balance{}
	}

	writeJSON(w, balances)
}

// Sourcify integration endpoints

// handleFetchSourcify fetches verified contract info from Sourcify
func (s *Server) handleFetchSourcify(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	chainIDStr := r.URL.Query().Get("chainId")
	if chainIDStr == "" {
		chainIDStr = "1" // Default to Ethereum mainnet
	}

	// Fetch from Sourcify
	url := fmt.Sprintf("%s/files/%s/%s", sourcifyAPIBase, chainIDStr, address)
	resp, err := http.Get(url)
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

	// Parse Sourcify response
	var sourcifyFiles []struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sourcifyFiles); err != nil {
		http.Error(w, "failed to parse Sourcify response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Extract ABI and source code
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

	// Update contract in database
	if err := s.db.VerifyContract(r.Context(), address, contractName, compilerVersion, false, sourceCode, abi, ""); err != nil {
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

// isLocalChain checks if a chain ID is a local/development chain
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

// handleVerifySourcify submits a contract for verification to Sourcify
func (s *Server) handleVerifySourcify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address         string          `json:"address"`
		ChainID         string          `json:"chainId"`
		SourceCode      string          `json:"sourceCode"`
		ContractName    string          `json:"contractName"`
		CompilerVersion string          `json:"compilerVersion"`
		OptimizationUsed bool           `json:"optimizationUsed"`
		Runs            int             `json:"runs"`
		ABI             json.RawMessage `json:"abi,omitempty"` // Optional: direct ABI for local chains
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

	// Handle local/development chains without Sourcify
	if isLocalChain(req.ChainID) {
		// For local chains, store verification directly without Sourcify
		// If ABI is provided, use it; otherwise store with empty ABI
		var abi json.RawMessage
		if len(req.ABI) > 0 {
			abi = req.ABI
		} else {
			// Store empty array as placeholder - user can update ABI separately
			abi = json.RawMessage("[]")
		}

		if err := s.db.VerifyContract(r.Context(), req.Address, req.ContractName, req.CompilerVersion, req.OptimizationUsed, req.SourceCode, abi, ""); err != nil {
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

	// Build Sourcify verification request
	sourcifyReq := map[string]any{
		"address":  req.Address,
		"chain":    req.ChainID,
		"files": map[string]string{
			req.ContractName + ".sol": req.SourceCode,
		},
	}

	body, _ := json.Marshal(sourcifyReq)
	resp, err := http.Post(sourcifyAPIBase+"/verify", "application/json", bytes.NewReader(body))
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

	// Fetch the verified contract info from Sourcify
	fetchURL := fmt.Sprintf("%s/files/%s/%s", sourcifyAPIBase, req.ChainID, common.HexToAddress(req.Address).Hex())
	fetchResp, err := http.Get(fetchURL)
	if err == nil && fetchResp.StatusCode == 200 {
		defer fetchResp.Body.Close()
		// Parse and save the verification result
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
				s.db.VerifyContract(r.Context(), req.Address, req.ContractName, compilerVersion, req.OptimizationUsed, req.SourceCode, abi, "")
			}
		}
	}

	writeJSON(w, map[string]any{
		"success": true,
		"status":  status,
		"address": req.Address,
	})
}

// handleVerifyContract handles contract verification using local solc or Sourcify fallback
func (s *Server) handleVerifyContract(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	// Check if we have a local verifier
	if s.verifier != nil {
		// Try local verification first
		result, err := s.verifier.VerifyFromJSON(r.Context(), body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If local verification succeeded or returned an error, use that result directly.
		// Only fall through to Sourcify if the compiler version is not available locally.
		if result.Success || (result.Error != "" && !strings.Contains(result.Error, "compiler version")) {
			writeJSON(w, result)
			return
		}
	}

	// Fall back to Sourcify verification - restore the body first
	r.Body = io.NopCloser(bytes.NewReader(body))
	s.handleVerifySourcify(w, r)
}

// handleListCompilers returns the list of available solc compiler versions
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

// handleCheckSourcify checks if a contract is verified on Sourcify
func (s *Server) handleCheckSourcify(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		http.Error(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	chainIDStr := r.URL.Query().Get("chainId")
	if chainIDStr == "" {
		chainIDStr = "1"
	}

	// For local chains, check our database instead of Sourcify
	if isLocalChain(chainIDStr) {
		contract, err := s.db.GetContract(r.Context(), address)
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

	// Check with Sourcify
	url := fmt.Sprintf("%s/check-by-addresses?addresses=%s&chainIds=%s", sourcifyAPIBase, address, chainIDStr)
	resp, err := http.Get(url)
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
