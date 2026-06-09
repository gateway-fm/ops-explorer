package publicapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"explorer/internal/api"
	"explorer/internal/types"

	"explorer/pkg/eth/common"
	"github.com/go-chi/chi/v5"
)

const defaultLimit = 25

// --- Helpers (matching existing patterns) ---

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
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

func parsePage(r *http.Request) int {
	if p := r.URL.Query().Get("page"); p != "" {
		if page, err := strconv.Atoi(p); err == nil && page > 0 {
			return page
		}
	}
	return 1
}

func parsePageSize(r *http.Request) int {
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if pageSize, err := strconv.Atoi(ps); err == nil && pageSize > 0 && pageSize <= 100 {
			return pageSize
		}
	}
	return 25
}

// paginate trims an over-fetched (limit+1) slice and, when there is a next
// page, emits the cursor for it. BUG-7: the previous form set the cursor to
// strconv.Itoa(len(items)) — the page length, which is meaningless as a
// ?before= value (the API pages by block number). cursorFn extracts the real
// cursor (the last returned row's block number) so the ?before= round-trip
// pages forward with no dupes/gaps.
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

// computeTotalPages returns ceil(total/pageSize) in int64 (BUG-1: int(total)
// truncated the total to platform int before dividing, breaking on 32-bit for
// a total above math.MaxInt32). pageSize<=0 / total<=0 -> 0.
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

func offsetPaginated[T any](data []T, total int64, page, pageSize int) types.OffsetPaginatedResponse[T] {
	if data == nil {
		data = []T{}
	}
	return types.OffsetPaginatedResponse[T]{
		Data:       data,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: computeTotalPages(total, pageSize),
	}
}

// --- Health ---

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	_, err := s.provider.GetLatestBlockNumber(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeJSON(w, map[string]any{"status": "unhealthy", "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"status": "healthy"})
}

// --- Blocks ---

func (s *Server) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	blocks, err := s.provider.GetBlocks(r.Context(), limit+1, beforeBlock)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(blocks, limit, func(b types.Block) string {
		return strconv.FormatUint(b.Number, 10)
	}))
}

func (s *Server) handleGetLatestBlock(w http.ResponseWriter, r *http.Request) {
	blocks, err := s.provider.GetBlocks(r.Context(), 1, nil)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(blocks) == 0 {
		writeError(w, "no blocks found", http.StatusNotFound)
		return
	}

	writeJSON(w, blocks[0])
}

func (s *Server) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	numberStr := chi.URLParam(r, "number")
	number, err := strconv.ParseUint(numberStr, 10, 64)
	if err != nil {
		writeError(w, "invalid block number", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	block, err := s.provider.GetBlock(ctx, number)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if block == nil {
		writeError(w, "block not found", http.StatusNotFound)
		return
	}

	txs, err := s.provider.GetTransactionsByBlock(ctx, number)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"block":        block,
		"transactions": txs,
	})
}

// --- Transactions ---

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
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(txs, limit, func(tx types.Transaction) string {
		return strconv.FormatUint(tx.BlockNumber, 10)
	}))
}

func (s *Server) handleGetTransactionsPaginated(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	pageSize := parsePageSize(r)

	txs, total, err := s.provider.GetTransactionsPaginatedWithCategories(r.Context(), page, pageSize)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if txs == nil {
		txs = []types.Transaction{}
	}

	writeJSON(w, offsetPaginated(txs, total, page, pageSize))
}

func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	tx, err := s.provider.GetTransactionWithCategories(r.Context(), hash)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tx == nil {
		writeError(w, "transaction not found", http.StatusNotFound)
		return
	}

	if tx.TxType == types.TxTypeDeposit {
		deposit, err := s.provider.GetOPDeposit(r.Context(), tx.Hash)
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

func (s *Server) handleGetTransactionLogs(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	logs, err := s.provider.GetLogsByTransaction(r.Context(), hash)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []types.Log{}
	}

	writeJSON(w, logs)
}

func (s *Server) handleGetTransactionTransfers(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	transfers, err := s.provider.GetTransfersByTransaction(r.Context(), hash)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, transfers)
}

func (s *Server) handleGetTransactionInternalTxs(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !strings.HasPrefix(hash, "0x") {
		hash = "0x" + hash
	}

	internalTxs, err := s.provider.GetInternalTransactionsByTx(r.Context(), hash)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if internalTxs == nil {
		internalTxs = []types.InternalTransaction{}
	}

	writeJSON(w, internalTxs)
}

// --- Addresses ---

func (s *Server) handleGetAddress(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	ctx := r.Context()

	stats, err := s.provider.GetAddressStats(ctx, address)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	balance, err := s.provider.GetBalance(ctx, address)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	code, err := s.provider.GetCode(ctx, address)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info := &types.AddressInfo{
		Address:    address,
		Balance:    *balance,
		TxCount:    stats.TxCount,
		IsContract: len(code) > 0 && string(code) != "0x",
	}

	writeJSON(w, info)
}

func (s *Server) handleGetAddressTransactions(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	limit := parseLimit(r)
	beforeBlock := parseBeforeBlock(r)

	txs, err := s.provider.GetTransactionsByAddress(r.Context(), address, limit+1, beforeBlock)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, paginate(txs, limit, func(tx types.Transaction) string {
		return strconv.FormatUint(tx.BlockNumber, 10)
	}))
}

func (s *Server) handleGetAddressTransfers(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	page := parsePage(r)
	pageSize := parsePageSize(r)
	offset := (page - 1) * pageSize

	// BUG-pub: the offset was previously computed and then discarded (`_ =
	// offset`), so ?page=2 returned the same rows as ?page=1. GetTransfersByAddress
	// only exposes cursor pagination, so to honor ?page= we over-fetch
	// offset+pageSize+1 rows and skip the first `offset`. (The +1 sentinel still
	// drives HasMore.) For deep offsets the ?before= cursor remains preferable;
	// this keeps page/pageSize correct for the documented public-API contract.
	fetch := offset + pageSize + 1
	transfers, err := s.provider.GetTransfersByAddress(r.Context(), address, fetch, nil)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if offset < len(transfers) {
		transfers = transfers[offset:]
	} else {
		transfers = transfers[:0]
	}

	writeJSON(w, paginate(transfers, pageSize, func(tt types.TokenTransfer) string {
		return strconv.FormatUint(tt.BlockNumber, 10)
	}))
}

func (s *Server) handleGetAddressTokenBalances(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	balances, err := s.provider.GetTokenBalances(r.Context(), address)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if balances == nil {
		balances = []types.Balance{}
	}

	writeJSON(w, balances)
}

func (s *Server) handleGetAddressInternalTxs(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	page := parsePage(r)
	pageSize := parsePageSize(r)
	offset := (page - 1) * pageSize

	internalTxs, total, err := s.provider.GetInternalTransactionsByAddress(r.Context(), address, pageSize, offset)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if internalTxs == nil {
		internalTxs = []types.InternalTransaction{}
	}

	writeJSON(w, offsetPaginated(internalTxs, total, page, pageSize))
}

func (s *Server) handleGetAddressLogs(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	page := parsePage(r)
	pageSize := parsePageSize(r)
	offset := (page - 1) * pageSize

	logs, total, err := s.provider.GetLogsByAddress(r.Context(), address, pageSize, offset)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil {
		logs = []types.Log{}
	}

	writeJSON(w, offsetPaginated(logs, total, page, pageSize))
}

func (s *Server) handleGetContract(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	contract, err := s.provider.GetContract(r.Context(), address)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if contract == nil {
		writeError(w, "contract not found", http.StatusNotFound)
		return
	}

	writeJSON(w, contract)
}

// --- Tokens ---

func (s *Server) handleGetTokens(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	pageSize := parsePageSize(r)
	tokenType := r.URL.Query().Get("type")
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	offset := (page - 1) * pageSize
	tokens, total, err := s.provider.GetTokens(r.Context(), pageSize, offset, tokenType, search)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if tokens == nil {
		tokens = []types.Token{}
	}

	writeJSON(w, offsetPaginated(tokens, total, page, pageSize))
}

func (s *Server) handleGetToken(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	token, err := s.provider.GetToken(r.Context(), address)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if token == nil {
		writeError(w, "token not found", http.StatusNotFound)
		return
	}

	writeJSON(w, token)
}

func (s *Server) handleGetTokenHolders(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	page := parsePage(r)
	pageSize := parsePageSize(r)
	offset := (page - 1) * pageSize

	holders, total, err := s.provider.GetTokenHolders(r.Context(), address, pageSize, offset)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if holders == nil {
		holders = []types.TokenHolder{}
	}

	writeJSON(w, offsetPaginated(holders, total, page, pageSize))
}

func (s *Server) handleGetTokenTransfers(w http.ResponseWriter, r *http.Request) {
	address := chi.URLParam(r, "address")
	if !common.IsHexAddress(address) {
		writeError(w, "invalid address", http.StatusBadRequest)
		return
	}
	address = common.HexToAddress(address).Hex()

	page := parsePage(r)
	pageSize := parsePageSize(r)
	offset := (page - 1) * pageSize

	transfers, total, err := s.provider.GetTransfersByToken(r.Context(), address, pageSize, offset)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if transfers == nil {
		transfers = []types.TokenTransfer{}
	}

	writeJSON(w, offsetPaginated(transfers, total, page, pageSize))
}

// --- Token Transfers ---

func (s *Server) handleGetAllTransfers(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	pageSize := parsePageSize(r)
	offset := (page - 1) * pageSize
	tokenType := api.NormalizeTokenType(r.URL.Query().Get("type"))

	transfers, total, err := s.provider.GetAllTransfers(r.Context(), tokenType, pageSize, offset)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if transfers == nil {
		transfers = []types.TokenTransfer{}
	}

	writeJSON(w, offsetPaginated(transfers, total, page, pageSize))
}

// --- Accounts ---

func (s *Server) handleGetAccounts(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	pageSize := parsePageSize(r)
	ctx := r.Context()

	accounts, total, err := s.provider.GetAccountsPaginated(ctx, page, pageSize)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
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

	if accountList == nil {
		accountList = []types.AccountListItem{}
	}

	writeJSON(w, offsetPaginated(accountList, total, page, pageSize))
}

// --- Search ---

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, "query required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Try as block number
	if num, err := strconv.ParseUint(q, 10, 64); err == nil {
		block, err := s.provider.GetBlock(ctx, num)
		if err == nil && block != nil {
			writeJSON(w, map[string]any{"type": "block", "data": block})
			return
		}
	}

	// Try as transaction hash
	if strings.HasPrefix(q, "0x") && len(q) == 66 {
		tx, err := s.provider.GetTransaction(ctx, q)
		if err == nil && tx != nil {
			writeJSON(w, map[string]any{"type": "transaction", "data": tx})
			return
		}
	}

	// Try as address
	if common.IsHexAddress(q) {
		address := common.HexToAddress(q).Hex()
		stats, _ := s.provider.GetAddressStats(ctx, address)
		writeJSON(w, map[string]any{"type": "address", "data": stats})
		return
	}

	writeError(w, "not found", http.StatusNotFound)
}

func (s *Server) handleSearchSuggestions(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) < 1 {
		writeJSON(w, types.SearchSuggestionsResponse{Query: q, Suggestions: []types.SearchSuggestion{}})
		return
	}

	suggestions, err := s.provider.SearchSuggestions(r.Context(), q, 10)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
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

// --- Stats ---

func (s *Server) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.provider.GetChainStats(r.Context())
	if err != nil {
		stats = &types.ChainStats{}
	}
	writeJSON(w, stats)
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
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if history == nil {
		history = []types.TxHistoryPoint{}
	}

	writeJSON(w, history)
}

func (s *Server) handleGetGasPrices(w http.ResponseWriter, r *http.Request) {
	slow, normal, fast, baseFee, err := s.provider.GetGasPrices(r.Context(), 20)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	d := func(p *uint64) uint64 {
		if p == nil {
			return 0
		}
		return *p
	}
	writeJSON(w, map[string]any{
		"slow":    d(slow),
		"normal":  d(normal),
		"fast":    d(fast),
		"baseFee": d(baseFee),
	})
}

func (s *Server) handleGetChainInfo(w http.ResponseWriter, r *http.Request) {
	if s.chainInfo == nil {
		writeError(w, "chain info not available", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, s.chainInfo.Get())
}

func (s *Server) handleGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.provider.GetSyncStatus(r.Context())
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	latestOnChain, err := s.provider.GetLatestBlockNumber(r.Context())
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// blocksRemaining can never be negative. When the indexer is ahead of this
	// chain-height read (a normal transient race), floor at 0 rather than emit a
	// negative count that breaks progress UIs.
	var blocksRemaining int64
	if latestOnChain > status.LastIndexedBlock {
		blocksRemaining = int64(latestOnChain - status.LastIndexedBlock)
	}

	writeJSON(w, map[string]any{
		"syncStatus":       status,
		"latestChainBlock": latestOnChain,
		"blocksRemaining":  blocksRemaining,
		"isSynced":         status.LastIndexedBlock >= latestOnChain,
	})
}

// --- Charts ---

func (s *Server) handleGetChartLines(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, api.GetChartLineDefinitions())
}

func (s *Server) handleGetChartLine(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	infoMap := api.GetChartLineInfoMap()
	info, ok := infoMap[id]
	if !ok {
		writeError(w, "unknown chart id", http.StatusNotFound)
		return
	}

	from, to := parseChartDateRange(r)

	stats, err := s.provider.GetDailyStats(r.Context(), from, to)
	if err != nil {
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	points := api.ExtractChartDataPoints(id, stats)

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
		writeError(w, err.Error(), http.StatusInternalServerError)
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

// --- OpenAPI ---

func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, buildOpenAPISpec())
}
