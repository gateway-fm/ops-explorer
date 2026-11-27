package publicapi

// Markdown versions of the explorer pages, for AI agents. Each function fills
// an mdDoc from the same DataProvider reads the JSON API uses. They return
// *mdError for "the page does not exist / the input is malformed" and a plain
// error when the data source failed.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"explorer/internal/types"
	"explorer/pkg/eth/common"
	"explorer/pkg/log"

	"golang.org/x/sync/errgroup"
)

const (
	mdListSize      = 25  // rows on list pages and list sections
	mdHomeListSize  = 10  // latest blocks / transactions on the home page
	mdMaxTokenMeta  = 25  // token metadata lookups per page
	mdMaxLogs       = 50  // logs shown on a transaction page
	mdMaxBlockTxs   = 100 // transactions shown on a block page
	mdMaxInputBytes = 16 * 1024
)

// mdError is an expected, user-facing failure (404, 400) with a Markdown body.
type mdError struct {
	status int
	title  string
	detail string
}

func (e *mdError) Error() string { return e.title }

func mdNotFound(what string) error {
	return &mdError{status: 404, title: "Not found", detail: what}
}

func mdBadRequest(what string) error {
	return &mdError{status: 400, title: "Bad request", detail: what}
}

var txHashRe = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)

var txCategoryLabels = map[string]string{
	types.TxCategoryCoinTransfer:      "Coin transfer",
	types.TxCategoryContractCall:      "Contract call",
	types.TxCategoryContractCreation:  "Contract creation",
	types.TxCategoryTokenTransfer:     "Token transfer",
	types.TxCategorySystemTransaction: "System transaction",
}

func txTypeLabel(t int) string {
	switch t {
	case 0:
		return "0 (Legacy)"
	case 1:
		return "1 (Access List)"
	case 2:
		return "2 (EIP-1559)"
	case types.TxTypeDeposit:
		return "126 (System Deposit)"
	}
	return strconv.Itoa(t)
}

func txStatus(tx *types.Transaction) string {
	if tx.Status == 1 {
		return "Success"
	}
	s := "Failed"
	if tx.Error != nil && *tx.Error != "" {
		s += " (error: " + mdText(*tx.Error) + ")"
	}
	if tx.RevertReason != nil && *tx.RevertReason != "" {
		s += " (revert reason: " + mdText(*tx.RevertReason) + ")"
	}
	return s
}

func shortStatus(status int) string {
	if status == 1 {
		return "Success"
	}
	return "Failed"
}

func (s *Server) native(raw string) string {
	return formatUnits(raw, 18) + " " + s.site.currency()
}

// tokenLabel is "Name (SYMBOL)" escaped, or the address when nothing is known.
func tokenLabel(t *types.Token, addr string) string {
	if t == nil {
		return addr
	}
	name := strings.TrimSpace(optStr(t.Name))
	sym := strings.TrimSpace(t.Symbol)
	switch {
	case name != "" && sym != "":
		return mdText(name) + " (" + mdText(sym) + ")"
	case sym != "":
		return mdText(sym)
	case name != "":
		return mdText(name)
	}
	return addr
}

// tokenAmount formats a transfer/balance amount with the token's decimals and
// symbol. Without metadata the raw integer is shown and labelled as such.
func tokenAmount(raw string, t *types.Token) string {
	if t == nil {
		return formatUnits(raw, 0) + " (raw units)"
	}
	if t.TokenType == "ERC721" {
		return formatUnits(raw, 0)
	}
	out := formatUnits(raw, t.Decimals)
	if sym := strings.TrimSpace(t.Symbol); sym != "" {
		out += " " + mdText(sym)
	}
	return out
}

// tokenMeta looks up token metadata for up to mdMaxTokenMeta distinct
// addresses, in parallel. Missing or failed lookups are simply absent: a page
// still renders, with raw amounts.
func (s *Server) tokenMeta(ctx context.Context, addrs []string) map[string]*types.Token {
	seen := map[string]bool{}
	var uniq []string
	for _, a := range addrs {
		k := strings.ToLower(a)
		if a == "" || seen[k] {
			continue
		}
		seen[k] = true
		uniq = append(uniq, a)
		if len(uniq) == mdMaxTokenMeta {
			break
		}
	}
	out := make(map[string]*types.Token, len(uniq))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for _, a := range uniq {
		g.Go(func() error {
			t, err := s.provider.GetToken(gctx, a)
			if err == nil && t != nil {
				mu.Lock()
				out[strings.ToLower(a)] = t
				mu.Unlock()
			}
			return nil
		})
	}
	_ = g.Wait()
	return out
}

func metaFor(m map[string]*types.Token, addr string) *types.Token {
	return m[strings.ToLower(addr)]
}

func parseQueryInt(q url.Values, key string, def int) int {
	if v, err := strconv.Atoi(q.Get(key)); err == nil && v > 0 {
		return v
	}
	return def
}

// --- Home ---

func (s *Server) mdHome(ctx context.Context, d *mdDoc, _ string, _ url.Values) error {
	var (
		stats  *types.ChainStats
		blocks []types.Block
		txs    []types.Transaction
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Optional: on failure the totals are reported as unavailable, never
		// as zeros an agent would read as facts.
		if st, err := s.provider.GetChainStats(gctx); err == nil {
			stats = st
		}
		return nil
	})
	g.Go(func() (err error) {
		blocks, err = s.provider.GetBlocks(gctx, mdHomeListSize, nil)
		return err
	})
	g.Go(func() (err error) {
		txs, err = s.provider.GetTransactionsWithCategories(gctx, mdHomeListSize, nil)
		return err
	})
	if err := g.Wait(); err != nil {
		return err
	}

	d.h1(s.site.name())
	d.para("> Markdown version of this block explorer, for AI agents. Every explorer page has a Markdown version at its URL followed by `/llm.md`. The index of pages is at [llms.txt](" + d.base + "/llms.txt).")
	d.h2("Network")
	chainID := ""
	if s.chainInfo != nil {
		if ci := s.chainInfo.Get(); ci != nil && ci.ChainIDDecimal != 0 {
			chainID = u64(ci.ChainIDDecimal)
		}
	}
	d.fields(append([]mdField{
		{"Chain ID", chainID},
		{"Native currency", s.site.currency()},
	}, statsFields(stats)...)...)
	if stats == nil {
		d.para(mdStatsUnavailable)
	}
	d.para("More: " + d.link("Blocks", "/blocks") + " · " + d.link("Transactions", "/transactions") + " · " + d.link("Tokens", "/tokens") + " · " + d.link("Statistics", "/stats"))

	d.h2("Latest blocks")
	s.blocksTable(d, blocks)
	d.h2("Latest transactions")
	s.txTable(d, txs, true)
	return nil
}

const mdStatsUnavailable = "Network totals are unavailable right now."

// statsFields lists the chain totals the indexer provides. TotalTokens is
// not among them (it is always zero there), so it is not shown.
func statsFields(st *types.ChainStats) []mdField {
	if st == nil {
		return nil
	}
	return []mdField{
		{"Total blocks", strconv.FormatInt(st.TotalBlocks, 10)},
		{"Total transactions", strconv.FormatInt(st.TotalTransactions, 10)},
		{"Total addresses", strconv.FormatInt(st.TotalAddresses, 10)},
		{"Average block time", strconv.FormatFloat(st.AvgBlockTime, 'f', 2, 64) + " s"},
	}
}

func (s *Server) blocksTable(d *mdDoc, blocks []types.Block) {
	if len(blocks) == 0 {
		d.para("No blocks yet.")
		return
	}
	rows := make([][]string, 0, len(blocks))
	for _, b := range blocks {
		rows = append(rows, []string{
			d.block(b.Number), mdTime(b.Timestamp), strconv.Itoa(b.TransactionCount),
			u64(b.GasUsed), d.addr(b.Miner),
		})
	}
	d.table([]string{"Block", "Time", "Transactions", "Gas used", "Miner"}, rows)
}

func (s *Server) txTable(d *mdDoc, txs []types.Transaction, withBlock bool) {
	if len(txs) == 0 {
		d.para("No transactions.")
		return
	}
	head := []string{"Hash"}
	if withBlock {
		head = append(head, "Block")
	}
	head = append(head, "Time", "From", "To", "Value", "Status")
	rows := make([][]string, 0, len(txs))
	for _, t := range txs {
		to := d.addr(optStr(t.To))
		if t.To == nil && t.ContractAddress != nil {
			to = "Contract creation: " + d.addr(*t.ContractAddress)
		}
		r := []string{d.tx(t.Hash)}
		if withBlock {
			r = append(r, d.block(t.BlockNumber))
		}
		r = append(r, mdTime(t.BlockTimestamp), d.addr(t.From), to, s.native(string(t.Value)), shortStatus(t.Status))
		rows = append(rows, r)
	}
	d.table(head, rows)
}

// --- Lists ---

func (s *Server) mdBlocks(ctx context.Context, d *mdDoc, _ string, q url.Values) error {
	var before *uint64
	if v := q.Get("before"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return mdBadRequest("`before` must be a block number.")
		}
		before = &n
	}
	blocks, err := s.provider.GetBlocks(ctx, mdListSize+1, before)
	if err != nil {
		return err
	}
	more := len(blocks) > mdListSize
	if more {
		blocks = blocks[:mdListSize]
	}
	d.h1("Blocks")
	if before != nil {
		d.para("Blocks before " + u64(*before) + ", newest first.")
	} else {
		d.para("Latest blocks, newest first.")
	}
	s.blocksTable(d, blocks)
	if more && len(blocks) > 0 {
		d.para("[Next page](" + d.urlQ("/blocks", "before="+u64(blocks[len(blocks)-1].Number)) + ")")
	}
	return nil
}

func (s *Server) mdTransactions(ctx context.Context, d *mdDoc, _ string, q url.Values) error {
	page := parseQueryInt(q, "page", 1)
	txs, total, err := s.provider.GetTransactionsPaginatedWithCategories(ctx, page, mdListSize)
	if err != nil {
		return err
	}
	pages := computeTotalPages(total, mdListSize)
	d.h1("Transactions")
	d.para(fmt.Sprintf("Latest transactions, newest first. Page %d of %d (%d transactions).", page, pages, total))
	s.txTable(d, txs, true)
	if page < pages {
		d.para("[Next page](" + d.urlQ("/transactions", "page="+strconv.Itoa(page+1)) + ")")
	}
	return nil
}

func (s *Server) mdTokens(ctx context.Context, d *mdDoc, _ string, q url.Values) error {
	page := parseQueryInt(q, "page", 1)
	tokens, total, err := s.provider.GetTokens(ctx, mdListSize, (page-1)*mdListSize, "", "")
	if err != nil {
		return err
	}
	pages := computeTotalPages(total, mdListSize)
	d.h1("Tokens")
	d.para(fmt.Sprintf("Page %d of %d (%d tokens).", page, pages, total))
	if len(tokens) == 0 {
		d.para("No tokens.")
	} else {
		rows := make([][]string, 0, len(tokens))
		for i := range tokens {
			t := &tokens[i]
			supply := ""
			if t.TotalSupply != nil {
				supply = tokenAmount(*t.TotalSupply, t)
			}
			rows = append(rows, []string{
				d.link(tokenLabel(t, t.Address), "/token/"+t.Address), t.Address, mdText(t.TokenType),
				strconv.Itoa(t.HolderCount), strconv.Itoa(t.TransferCount), supply,
			})
		}
		d.table([]string{"Token", "Contract", "Type", "Holders", "Transfers", "Total supply"}, rows)
	}
	if page < pages {
		d.para("[Next page](" + d.urlQ("/tokens", "page="+strconv.Itoa(page+1)) + ")")
	}
	return nil
}

// --- Block ---

func (s *Server) mdBlock(ctx context.Context, d *mdDoc, id string, _ url.Values) error {
	var (
		block *types.Block
		err   error
	)
	switch {
	case txHashRe.MatchString(id):
		block, err = s.provider.GetBlockByHash(ctx, strings.ToLower(id))
	default:
		n, perr := strconv.ParseUint(id, 10, 64)
		if perr != nil {
			return mdBadRequest("A block is a decimal number or a 0x block hash.")
		}
		block, err = s.provider.GetBlock(ctx, n)
	}
	if err != nil {
		return err
	}
	if block == nil {
		return mdNotFound("No block " + mdText(id) + " on this chain (yet).")
	}
	txs, err := s.provider.GetTransactionsByBlock(ctx, block.Number)
	if err != nil {
		return err
	}

	d.h1("Block " + u64(block.Number))
	parent := ""
	if block.Number > 0 {
		parent = d.link(block.ParentHash, "/block/"+u64(block.Number-1))
	}
	baseFee := ""
	if block.BaseFeePerGas != nil {
		baseFee = u64(*block.BaseFeePerGas) + " wei"
	}
	gasUsed := u64(block.GasUsed)
	if p := pct(block.GasUsed, block.GasLimit); p != "" {
		gasUsed += " (" + p + " of limit)"
	}
	d.fields(
		mdField{"Block height", u64(block.Number)},
		mdField{"Hash", block.Hash},
		mdField{"Parent hash", parent},
		mdField{"Timestamp", mdTime(block.Timestamp)},
		mdField{"Transactions", strconv.Itoa(block.TransactionCount)},
		mdField{"Miner", d.addr(block.Miner)},
		mdField{"Gas used", gasUsed},
		mdField{"Gas limit", u64(block.GasLimit)},
		mdField{"Base fee", baseFee},
		mdField{"Size", u64(block.Size) + " bytes"},
		mdField{"Difficulty", mdText(block.Difficulty)},
		mdField{"Total difficulty", mdText(block.TotalDifficulty)},
		mdField{"Nonce", mdText(block.Nonce)},
		mdField{"Extra data", mdText(block.ExtraData)},
		mdField{"State root", mdText(block.StateRoot)},
		mdField{"Transactions root", mdText(block.TransactionsRoot)},
		mdField{"Receipts root", mdText(block.ReceiptsRoot)},
	)
	nav := []string{}
	if block.Number > 0 {
		nav = append(nav, d.link("Previous block", "/block/"+u64(block.Number-1)))
	}
	nav = append(nav, d.link("Next block", "/block/"+u64(block.Number+1)))
	d.para(strings.Join(nav, " · "))

	// The header count is the total; the list may be one indexer page.
	total := max(block.TransactionCount, len(txs))
	d.h2(fmt.Sprintf("Transactions (%d)", total))
	shown := txs
	if len(shown) > mdMaxBlockTxs {
		shown = shown[:mdMaxBlockTxs]
	}
	s.txTable(d, shown, false)
	if len(shown) < total {
		d.para(fmt.Sprintf("Showing the first %d of %d transactions.", len(shown), total))
	}
	return nil
}

// --- Transaction ---

func (s *Server) mdTransaction(ctx context.Context, d *mdDoc, id string, _ url.Values) error {
	if !txHashRe.MatchString(id) {
		return mdBadRequest("A transaction hash is 0x followed by 64 hex characters.")
	}
	id = strings.ToLower(id)
	tx, err := s.provider.GetTransactionWithCategories(ctx, id)
	if err != nil {
		return err
	}
	if tx == nil {
		return mdNotFound("No transaction " + id + " on this chain (yet).")
	}

	var (
		transfers []types.TokenTransfer
		internal  []types.InternalTransaction
		logs      []types.Log
		deposit   *types.OPDeposit
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { transfers, err = s.provider.GetTransfersByTransaction(gctx, tx.Hash); return err })
	g.Go(func() (err error) { internal, err = s.provider.GetInternalTransactionsByTx(gctx, tx.Hash); return err })
	g.Go(func() (err error) { logs, err = s.provider.GetLogsByTransaction(gctx, tx.Hash); return err })
	if tx.TxType == types.TxTypeDeposit {
		g.Go(func() error {
			// Same as the JSON API: deposit details are optional extras.
			if dep, err := s.provider.GetOPDeposit(gctx, tx.Hash); err == nil {
				deposit = dep
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	tokAddrs := make([]string, 0, len(transfers))
	for _, t := range transfers {
		tokAddrs = append(tokAddrs, t.TokenAddress)
	}
	meta := s.tokenMeta(ctx, tokAddrs)

	d.h1("Transaction " + tx.Hash)

	actions := make([]string, 0, len(tx.TxCategories))
	for _, c := range tx.TxCategories {
		if l, ok := txCategoryLabels[c]; ok {
			actions = append(actions, l)
		} else {
			actions = append(actions, mdText(c))
		}
	}
	to := d.addr(optStr(tx.To))
	created := ""
	if tx.ContractAddress != nil && *tx.ContractAddress != "" {
		created = d.addr(*tx.ContractAddress)
	}
	fee := ""
	if tx.GasUsed > 0 || tx.GasPrice > 0 {
		f := new(big.Int).Mul(new(big.Int).SetUint64(tx.GasUsed), new(big.Int).SetUint64(tx.GasPrice))
		fee = s.native(f.String())
	}
	gasUsed := u64(tx.GasUsed)
	if tx.GasLimit != nil {
		if p := pct(tx.GasUsed, *tx.GasLimit); p != "" {
			gasUsed += " (" + p + " of limit)"
		}
	}
	maxFee, maxPrio := "", ""
	if tx.MaxFeePerGas != nil {
		maxFee = u64(*tx.MaxFeePerGas) + " wei"
	}
	if tx.MaxPriorityFeePerGas != nil {
		maxPrio = u64(*tx.MaxPriorityFeePerGas) + " wei"
	}
	method := ""
	if in := strings.TrimSpace(tx.InputData); len(in) >= 10 && strings.HasPrefix(in, "0x") {
		method = "`" + strings.ToLower(in[:10]) + "` (function selector)"
	}
	d.fields(
		mdField{"Status", txStatus(tx)},
		mdField{"Action", strings.Join(actions, ", ")},
		mdField{"Block", d.block(tx.BlockNumber)},
		mdField{"Timestamp", mdTime(tx.BlockTimestamp)},
		mdField{"From", d.addr(tx.From)},
		mdField{"To", to},
		mdField{"Contract created", created},
		mdField{"Value", s.native(string(tx.Value)) + " (" + formatUnits(string(tx.Value), 0) + " wei)"},
		mdField{"Transaction fee", fee},
		mdField{"Gas price", u64(tx.GasPrice) + " wei"},
		mdField{"Gas limit", optU64(tx.GasLimit)},
		mdField{"Gas used", gasUsed},
		mdField{"Max fee per gas", maxFee},
		mdField{"Max priority fee per gas", maxPrio},
		mdField{"Transaction type", txTypeLabel(tx.TxType)},
		mdField{"Nonce", optU64(tx.Nonce)},
		mdField{"Position in block", strconv.Itoa(tx.TxIndex)},
		mdField{"Method", method},
	)

	if deposit != nil {
		d.h2("Deposit from the parent chain")
		d.fields(
			mdField{"Parent-chain block", u64(deposit.L1BlockNumber)},
			mdField{"Parent-chain timestamp", mdTime(derefU64(deposit.L1BlockTimestamp))},
			mdField{"Parent-chain transaction", mdText(deposit.L1TxHash)},
			mdField{"Parent-chain origin", mdText(deposit.L1TxOrigin)},
		)
	}

	d.h2(fmt.Sprintf("Token transfers (%d)", len(transfers)))
	if len(transfers) > 0 {
		s.transfersTable(d, transfers, meta, false)
	} else {
		d.para("None.")
	}

	d.h2(fmt.Sprintf("Internal transactions (%d)", len(internal)))
	if len(internal) > 0 {
		rows := make([][]string, 0, len(internal))
		for _, it := range internal {
			rows = append(rows, []string{
				mdText(it.CallType), d.addr(it.From), d.addr(optStr(it.To)),
				s.native(string(it.Value)), mdText(optStr(it.Error)),
			})
		}
		d.table([]string{"Type", "From", "To", "Value", "Error"}, rows)
	} else {
		d.para("None.")
	}

	d.h2(fmt.Sprintf("Logs (%d)", len(logs)))
	if len(logs) == 0 {
		d.para("None.")
	}
	for i, l := range logs {
		if i == mdMaxLogs {
			d.para(fmt.Sprintf("Showing the first %d of %d logs.", mdMaxLogs, len(logs)))
			break
		}
		d.line(fmt.Sprintf("### Log %d", l.LogIndex))
		d.line("")
		d.fields(
			mdField{"Address", d.addr(l.Address)},
			mdField{"Topics", logTopics(l)},
			mdField{"Data", "`" + mdCodeSpan(l.Data) + "`"},
		)
	}

	d.h2("Input data")
	in := strings.TrimSpace(tx.InputData)
	if in == "" || in == "0x" {
		d.para("None.")
	} else {
		if len(in) > mdMaxInputBytes*2 {
			d.para(fmt.Sprintf("First %d of %d bytes.", mdMaxInputBytes, (len(in)-2)/2))
			in = in[:mdMaxInputBytes*2+2]
		}
		d.code("", in)
	}
	return nil
}

// transfersTable renders token transfers. withTx adds a transaction column
// (lists of transfers across transactions).
func (s *Server) transfersTable(d *mdDoc, transfers []types.TokenTransfer, meta map[string]*types.Token, withTx bool) {
	head := []string{}
	if withTx {
		head = append(head, "Transaction", "Block")
	}
	head = append(head, "Token", "From", "To", "Amount")
	rows := make([][]string, 0, len(transfers))
	for _, t := range transfers {
		m := metaFor(meta, t.TokenAddress)
		amount := tokenAmount(string(t.Value), m)
		if t.TokenID != nil && *t.TokenID != "" {
			amount = "token ID " + mdText(*t.TokenID)
			if t.TokenType == "ERC1155" {
				amount = formatUnits(string(t.Value), 0) + " × " + amount
			}
		}
		r := []string{}
		if withTx {
			r = append(r, d.tx(t.TxHash), d.block(t.BlockNumber))
		}
		r = append(r, d.link(tokenLabel(m, t.TokenAddress), "/token/"+t.TokenAddress), d.addr(t.From), d.addr(t.To), amount)
		rows = append(rows, r)
	}
	d.table(head, rows)
}

// --- Address ---

func (s *Server) mdAddress(ctx context.Context, d *mdDoc, id string, q url.Values) error {
	if !common.IsHexAddress(id) {
		return mdBadRequest("An address is 0x followed by 40 hex characters.")
	}
	address := common.HexToAddress(id).Hex()
	switch q.Get("tab") {
	case "contract":
		return s.mdContract(ctx, d, address)
	case "transfers":
		return s.mdAddressTransfers(ctx, d, address, q)
	case "internal":
		return s.mdAddressInternal(ctx, d, address, q)
	case "logs":
		return s.mdAddressLogs(ctx, d, address, q)
	case "", "details", "transactions", "tokens":
	default:
		return mdBadRequest("This address tab has no Markdown version. Tabs: `transactions` (default), `transfers`, `internal`, `logs`, `contract`.")
	}
	cursor := q.Get("cursor")

	var (
		stats    *types.AddressStats
		balance  *types.JSONString
		code     []byte
		balances []types.Balance
		txs      []types.Transaction
		next     string
		token    *types.Token
		contract *types.Contract
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { stats, err = s.provider.GetAddressStats(gctx, address); return err })
	g.Go(func() (err error) { balance, err = s.provider.GetBalance(gctx, address); return err })
	g.Go(func() (err error) { code, err = s.provider.GetCode(gctx, address); return err })
	g.Go(func() (err error) { balances, err = s.provider.GetTokenBalances(gctx, address); return err })
	g.Go(func() (err error) {
		txs, next, err = s.provider.GetTransactionsByAddress(gctx, address, mdListSize, cursor, nil)
		return err
	})
	// Optional: whether this address is a token contract.
	g.Go(func() error {
		if t, err := s.provider.GetToken(gctx, address); err == nil {
			token = t
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}
	isContract := len(code) > 0 && string(code) != "0x"
	var contractErr error
	if isContract {
		// The indexer answers (nil, nil) for "no contract record"; an error
		// is a failed read, reported as unknown rather than "not verified".
		contract, contractErr = s.provider.GetContract(ctx, address)
		if contractErr != nil {
			log.Warn("llm.md contract read failed", "address", address, "error", contractErr)
		}
	}
	if stats == nil {
		stats = &types.AddressStats{}
	}

	kind := "Wallet (externally owned account)"
	if isContract {
		kind = "Contract"
	}
	bal := "0 " + s.site.currency()
	if balance != nil {
		bal = s.native(string(*balance))
	}
	d.h1("Address " + address)
	tokenLink := ""
	if token != nil {
		tokenLink = d.link(tokenLabel(token, address), "/token/"+address)
	}
	d.fields(
		mdField{"Type", kind},
		mdField{"Token", tokenLink},
		mdField{"Balance", bal},
		mdField{"Transactions", strconv.Itoa(stats.TxCount)},
		mdField{"Token transfers", strconv.Itoa(stats.TokenTransferCount)},
		mdField{"Internal transactions", strconv.Itoa(stats.InternalTxCount)},
	)

	if isContract {
		d.h2("Contract")
		switch {
		case contractErr != nil:
			d.fields(mdField{"Verified", "Unknown (contract data is unavailable right now)"})
		case contract != nil && contract.IsVerified:
			d.fields(
				mdField{"Verified", "Yes"},
				mdField{"Name", mdText(optStr(contract.ContractName))},
				mdField{"Compiler", mdText(optStr(contract.CompilerVersion))},
				mdField{"Creator", d.addr(contract.Creator)},
				mdField{"Creation transaction", d.tx(contract.CreationTx)},
			)
			if fns := abiSignatures(contract.ABI); len(fns) > 0 {
				d.para("Functions and events: " + strings.Join(fns, ", "))
			}
		default:
			creator, creationTx := "", ""
			if contract != nil {
				creator, creationTx = d.addr(contract.Creator), d.tx(contract.CreationTx)
			}
			d.fields(
				mdField{"Verified", "No"},
				mdField{"Creator", creator},
				mdField{"Creation transaction", creationTx},
			)
		}
		d.para("Source code, ABI and bytecode: [contract details](" + d.urlQ("/address/"+address, "tab=contract") + ")")
	}

	// Token holdings: non-zero balances, at most mdMaxTokenMeta of them.
	held := make([]types.Balance, 0, len(balances))
	for _, b := range balances {
		if v := strings.TrimLeft(string(b.Balance), "0"); v != "" && v != "x" {
			held = append(held, b)
		}
	}
	d.h2(fmt.Sprintf("Token holdings (%d)", len(held)))
	if len(held) == 0 {
		d.para("None.")
	} else {
		shown := held
		if len(shown) > mdMaxTokenMeta {
			shown = shown[:mdMaxTokenMeta]
		}
		addrs := make([]string, 0, len(shown))
		for _, b := range shown {
			addrs = append(addrs, b.TokenAddress)
		}
		meta := s.tokenMeta(ctx, addrs)
		rows := make([][]string, 0, len(shown))
		for _, b := range shown {
			m := metaFor(meta, b.TokenAddress)
			typ := ""
			if m != nil {
				typ = mdText(m.TokenType)
			}
			rows = append(rows, []string{d.link(tokenLabel(m, b.TokenAddress), "/token/"+b.TokenAddress), typ, tokenAmount(string(b.Balance), m)})
		}
		d.table([]string{"Token", "Type", "Balance"}, rows)
		if len(held) > len(shown) {
			d.para(fmt.Sprintf("Showing %d of %d tokens.", len(shown), len(held)))
		}
	}

	if cursor != "" {
		d.h2("Transactions (older page)")
	} else {
		d.h2("Recent transactions")
	}
	s.txTable(d, txs, true)
	if next != "" {
		d.para("[Older transactions](" + d.urlQ("/address/"+address, "cursor="+url.QueryEscape(next)) + ")")
	}
	base := "/address/" + address
	d.para("More: [Token transfers](" + d.urlQ(base, "tab=transfers") + ") · [Internal transactions](" +
		d.urlQ(base, "tab=internal") + ") · [Logs](" + d.urlQ(base, "tab=logs") + ")")
	return nil
}

func (s *Server) mdAddressTransfers(ctx context.Context, d *mdDoc, address string, q url.Values) error {
	cursor := q.Get("cursor")
	transfers, next, err := s.provider.GetTransfersByAddress(ctx, address, mdListSize, cursor, nil)
	if err != nil {
		return err
	}
	addrs := make([]string, 0, len(transfers))
	for _, t := range transfers {
		addrs = append(addrs, t.TokenAddress)
	}
	meta := s.tokenMeta(ctx, addrs)
	d.h1("Token transfers of " + address)
	d.para("Address: " + d.addr(address) + ". Newest first.")
	if len(transfers) == 0 {
		d.para("None.")
	} else {
		s.transfersTable(d, transfers, meta, true)
	}
	if next != "" {
		d.para("[Older transfers](" + d.urlQ("/address/"+address, "tab=transfers&cursor="+url.QueryEscape(next)) + ")")
	}
	return nil
}

func (s *Server) mdAddressInternal(ctx context.Context, d *mdDoc, address string, q url.Values) error {
	page := parseQueryInt(q, "page", 1)
	items, total, err := s.provider.GetInternalTransactionsByAddress(ctx, address, mdListSize, (page-1)*mdListSize)
	if err != nil {
		return err
	}
	pages := computeTotalPages(total, mdListSize)
	d.h1("Internal transactions of " + address)
	d.para(fmt.Sprintf("Address: %s. Page %d of %d (%d internal transactions).", d.addr(address), page, pages, total))
	if len(items) == 0 {
		d.para("None.")
	} else {
		rows := make([][]string, 0, len(items))
		for _, it := range items {
			rows = append(rows, []string{
				d.tx(it.TxHash), d.block(it.BlockNumber), mdText(it.CallType), d.addr(it.From), d.addr(optStr(it.To)),
				s.native(string(it.Value)), mdText(optStr(it.Error)),
			})
		}
		d.table([]string{"Transaction", "Block", "Type", "From", "To", "Value", "Error"}, rows)
	}
	if page < pages {
		d.para("[Next page](" + d.urlQ("/address/"+address, "tab=internal&page="+strconv.Itoa(page+1)) + ")")
	}
	return nil
}

func (s *Server) mdAddressLogs(ctx context.Context, d *mdDoc, address string, q url.Values) error {
	page := parseQueryInt(q, "page", 1)
	logs, total, err := s.provider.GetLogsByAddress(ctx, address, mdListSize, (page-1)*mdListSize)
	if err != nil {
		return err
	}
	pages := computeTotalPages(total, mdListSize)
	d.h1("Logs emitted by " + address)
	d.para(fmt.Sprintf("Address: %s. Page %d of %d (%d logs).", d.addr(address), page, pages, total))
	if len(logs) == 0 {
		d.para("None.")
	} else {
		rows := make([][]string, 0, len(logs))
		for _, l := range logs {
			rows = append(rows, []string{d.tx(l.TxHash), d.block(l.BlockNumber), strconv.Itoa(l.LogIndex), logTopics(l), "`" + mdCodeSpan(l.Data) + "`"})
		}
		d.table([]string{"Transaction", "Block", "Log index", "Topics", "Data"}, rows)
	}
	if page < pages {
		d.para("[Next page](" + d.urlQ("/address/"+address, "tab=logs&page="+strconv.Itoa(page+1)) + ")")
	}
	return nil
}

// logTopics renders a log's non-empty topics as code spans.
func logTopics(l types.Log) string {
	topics := []string{}
	for _, t := range []*string{l.Topic0, l.Topic1, l.Topic2, l.Topic3} {
		if t != nil && *t != "" {
			topics = append(topics, "`"+mdCodeSpan(*t)+"`")
		}
	}
	return strings.Join(topics, ", ")
}

func (s *Server) mdContract(ctx context.Context, d *mdDoc, address string) error {
	c, err := s.provider.GetContract(ctx, address)
	if err != nil {
		return err
	}
	if c == nil {
		return mdNotFound("No contract at " + address + ".")
	}
	d.h1("Contract " + address)
	verified := "No"
	if c.IsVerified {
		verified = "Yes"
	}
	opt := ""
	if c.OptimizationUsed != nil {
		opt = "No"
		if *c.OptimizationUsed {
			opt = "Yes"
			if c.OptimizationRuns != nil {
				opt += fmt.Sprintf(" (%d runs)", *c.OptimizationRuns)
			}
		}
	}
	d.fields(
		mdField{"Address", d.addr(address)},
		mdField{"Verified", verified},
		mdField{"Name", mdText(optStr(c.ContractName))},
		mdField{"Compiler", mdText(optStr(c.CompilerVersion))},
		mdField{"Optimization", opt},
		mdField{"EVM version", mdText(optStr(c.EVMVersion))},
		mdField{"License", mdText(optStr(c.LicenseType))},
		mdField{"Creator", d.addr(c.Creator)},
		mdField{"Creation transaction", d.tx(c.CreationTx)},
		mdField{"Created in block", nonZeroBlock(d, c.BlockNumber)},
	)
	if args := optStr(c.ConstructorArgs); args != "" {
		d.h2("Constructor arguments")
		d.code("", args)
	}
	if len(c.ABI) > 0 && string(c.ABI) != "null" {
		d.h2("ABI")
		d.code("json", compactJSON(c.ABI))
	}
	if src := optStr(c.SourceCode); src != "" {
		d.h2("Source code")
		d.code("solidity", src)
	}
	if c.Bytecode != "" && c.Bytecode != "0x" {
		d.h2("Deployed bytecode")
		d.code("", c.Bytecode)
	}
	return nil
}

func nonZeroBlock(d *mdDoc, n uint64) string {
	if n == 0 {
		return ""
	}
	return d.block(n)
}

func compactJSON(raw json.RawMessage) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}

// abiSignatures lists "function name(types)" / "event name(types)" entries of
// an ABI, escaped for Markdown. Invalid ABI yields nil.
func abiSignatures(raw json.RawMessage) []string {
	var entries []struct {
		Type   string `json:"type"`
		Name   string `json:"name"`
		Inputs []struct {
			Type string `json:"type"`
		} `json:"inputs"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &entries) != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		if (e.Type != "function" && e.Type != "event") || e.Name == "" {
			continue
		}
		ts := make([]string, 0, len(e.Inputs))
		for _, in := range e.Inputs {
			ts = append(ts, in.Type)
		}
		out = append(out, "`"+mdCodeSpan(e.Type+" "+e.Name+"("+strings.Join(ts, ",")+")")+"`")
	}
	return out
}

// mdCodeSpan makes text safe inside a single-backtick code span.
func mdCodeSpan(s string) string {
	s = strings.ReplaceAll(s, "`", "'")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "\r", " ")
}

// --- Token ---

func (s *Server) mdToken(ctx context.Context, d *mdDoc, id string, q url.Values) error {
	if !common.IsHexAddress(id) {
		return mdBadRequest("A token is identified by its contract address: 0x followed by 40 hex characters.")
	}
	address := common.HexToAddress(id).Hex()
	t, err := s.provider.GetToken(ctx, address)
	if err != nil {
		return err
	}
	if t == nil {
		return mdNotFound("No token at " + address + ". If it is an ordinary address, see " + d.addr(address) + ".")
	}
	// One ?page= pages both lists, like the JSON API's offset paging.
	page := parseQueryInt(q, "page", 1)
	offset := (page - 1) * mdListSize
	var (
		holders                  []types.TokenHolder
		transfers                []types.TokenTransfer
		holderTotal, transferTot int64
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		holders, holderTotal, err = s.provider.GetTokenHolders(gctx, address, mdListSize, offset)
		return err
	})
	g.Go(func() (err error) {
		transfers, transferTot, err = s.provider.GetTransfersByToken(gctx, address, mdListSize, offset)
		return err
	})
	if err := g.Wait(); err != nil {
		return err
	}

	d.h1("Token " + tokenLabel(t, address))
	supply := ""
	if t.TotalSupply != nil {
		supply = tokenAmount(*t.TotalSupply, t)
	}
	price := ""
	if t.USDPrice != nil {
		price = "$" + strconv.FormatFloat(*t.USDPrice, 'f', -1, 64)
	}
	d.fields(
		mdField{"Contract", d.addr(address)},
		mdField{"Name", mdText(optStr(t.Name))},
		mdField{"Symbol", mdText(t.Symbol)},
		mdField{"Type", mdText(t.TokenType)},
		mdField{"Decimals", strconv.Itoa(t.Decimals)},
		mdField{"Total supply", supply},
		mdField{"Holders", strconv.Itoa(t.HolderCount)},
		mdField{"Transfers", strconv.Itoa(t.TransferCount)},
		mdField{"Price", price},
		mdField{"Parent-chain address", mdText(optStr(t.L1Address))},
		mdField{"Created in block", nonZeroBlock(d, t.BlockNumber)},
		mdField{"Creation transaction", d.tx(optStr(t.CreationTx))},
	)

	holderPages := computeTotalPages(holderTotal, mdListSize)
	transferPages := computeTotalPages(transferTot, mdListSize)

	d.h2(fmt.Sprintf("Holders, largest first (page %d of %d, %d holders)", page, holderPages, holderTotal))
	if len(holders) == 0 {
		d.para("None.")
	} else {
		rows := make([][]string, 0, len(holders))
		for _, h := range holders {
			rows = append(rows, []string{d.addr(h.Address), tokenAmount(string(h.Balance), t), supplyShare(string(h.Balance), t)})
		}
		d.table([]string{"Address", "Balance", "Share of supply"}, rows)
	}

	d.h2(fmt.Sprintf("Transfers, newest first (page %d of %d, %d transfers)", page, transferPages, transferTot))
	if len(transfers) == 0 {
		d.para("None.")
	} else {
		s.transfersTable(d, transfers, map[string]*types.Token{strings.ToLower(address): t}, true)
	}
	if page < max(holderPages, transferPages) {
		d.para("[Next page](" + d.urlQ("/token/"+address, "page="+strconv.Itoa(page+1)) + ")")
	}
	return nil
}

// supplyShare is balance as a percentage of the token's total supply, or ""
// when the supply is unknown or zero. The indexer does not send a share.
func supplyShare(balance string, t *types.Token) string {
	if t == nil || t.TotalSupply == nil {
		return ""
	}
	sup, ok1 := new(big.Int).SetString(strings.TrimSpace(*t.TotalSupply), 10)
	bal, ok2 := new(big.Int).SetString(strings.TrimSpace(balance), 10)
	if !ok1 || !ok2 || sup.Sign() <= 0 {
		return ""
	}
	r := new(big.Rat).SetFrac(new(big.Int).Mul(bal, big.NewInt(100)), sup)
	return r.FloatString(2) + "%"
}

// --- Stats ---

func (s *Server) mdStats(ctx context.Context, d *mdDoc, _ string, _ url.Values) error {
	var (
		stats *types.ChainStats
		daily []types.DailyStats
		gas   [4]*uint64
	)
	now := time.Now().UTC()
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -30)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		// Optional: on failure the totals are reported as unavailable, never
		// as zeros an agent would read as facts.
		if st, err := s.provider.GetChainStats(gctx); err == nil {
			stats = st
		}
		return nil
	})
	g.Go(func() (err error) { daily, err = s.provider.GetDailyStats(gctx, from, to); return err })
	g.Go(func() error {
		// Optional, like the gas tracker: a failed read leaves the section out.
		slow, normal, fast, base, err := s.provider.GetGasPrices(gctx, 20)
		if err == nil {
			gas = [4]*uint64{slow, normal, fast, base}
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}

	d.h1("Chain statistics")
	d.fields(statsFields(stats)...)
	if stats == nil {
		d.para(mdStatsUnavailable)
	}

	if gas[1] != nil {
		d.h2("Gas prices")
		gw := func(p *uint64) string {
			if p == nil {
				return ""
			}
			return formatUnits(u64(*p), 9) + " gwei"
		}
		d.fields(
			mdField{"Slow", gw(gas[0])},
			mdField{"Normal", gw(gas[1])},
			mdField{"Fast", gw(gas[2])},
			mdField{"Base fee", gw(gas[3])},
		)
	}

	d.h2("Daily activity (last 30 days, UTC)")
	if len(daily) == 0 {
		d.para("No daily statistics yet.")
		return nil
	}
	sort.Slice(daily, func(i, j int) bool { return daily[i].Date > daily[j].Date })
	// Only the columns the indexer fills in; the others in DailyStats are
	// always zero there and would read as measured zeros.
	rows := make([][]string, 0, len(daily))
	for _, ds := range daily {
		rows = append(rows, []string{
			mdText(ds.Date), strconv.Itoa(ds.TotalTransactions), strconv.Itoa(ds.TotalBlocks),
			strconv.FormatInt(ds.TotalGasUsed, 10), s.native(strconv.FormatInt(ds.AvgGasPrice, 10)),
			strconv.Itoa(ds.ActiveAddresses), strconv.Itoa(ds.NewAddresses),
		})
	}
	d.table([]string{"Date", "Transactions", "Blocks", "Gas used", "Average fee per transaction", "Active addresses", "New addresses"}, rows)
	return nil
}

func derefU64(p *uint64) uint64 {
	if p == nil {
		return 0
	}
	return *p
}
