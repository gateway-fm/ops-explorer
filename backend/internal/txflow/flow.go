// Package txflow turns one transaction into an ordered list of plain-language steps:
// the calls it made, the native-currency and token transfers, and the events it emitted.
//
// The model is built only from data the indexer already stored (the transaction,
// its internal calls, its logs and token transfers, and contract metadata). It never
// calls the node, so serving it cannot trigger a trace.
//
// Build is a pure function over that data; Load fetches the data and calls Build. Both
// are meant to be reused by every surface that explains a transaction: the Flow tab, the
// Markdown page and the MCP tool.
package txflow

import (
	"math/big"
	"sort"
	"strconv"
	"strings"

	"explorer/internal/types"
)

// Kind is what a step shows.
type Kind string

const (
	KindCall          Kind = "call"           // a contract call (possibly sending native currency)
	KindCreate        Kind = "create"         // a contract deployment
	KindValueTransfer Kind = "value_transfer" // native currency sent without calling a function
	KindTokenTransfer Kind = "token_transfer" // an ERC-20/721/1155 transfer
	KindEvent         Kind = "event"          // any other event a contract emitted
)

// Decoding sources for a step's method or event name.
const (
	DecodedVerified = "verified" // from the verified contract's ABI
	DecodedCommon   = "common"   // from the built-in list of standard token functions and events
)

// Participant is one address that appears in the flow; the diagram draws one lane per participant.
type Participant struct {
	Address  string `json:"address"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // "account", "contract", "token" or "private"
	Verified bool   `json:"verified,omitempty"`
}

// Arg is a decoded function or event argument. Value is empty for types the decoder
// does not expand (arrays and tuples).
type Arg struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TokenAmount describes the token side of a token-transfer step.
type TokenAmount struct {
	Address  string  `json:"address"`
	Symbol   string  `json:"symbol,omitempty"`
	Standard string  `json:"standard,omitempty"` // ERC20, ERC721, ERC1155
	Raw      string  `json:"raw"`                // amount in the token's smallest unit
	Amount   string  `json:"amount,omitempty"`   // Raw scaled by the token's decimals, when known
	TokenID  *string `json:"tokenId,omitempty"`
}

// Step is one entry of the walkthrough, in execution order.
type Step struct {
	Index int    `json:"index"`
	Kind  Kind   `json:"kind"`
	Depth int    `json:"depth"` // call depth: 0 is the transaction's own call
	From  string `json:"from"`
	To    string `json:"to"`

	CallType     string  `json:"callType,omitempty"`     // call, staticcall, delegatecall, create, create2
	ReadOnly     bool    `json:"readOnly,omitempty"`     // a staticcall: reads state, changes nothing
	TraceAddress *string `json:"traceAddress,omitempty"` // position in the call tree ("" for the top call)

	Method    string `json:"method,omitempty"`    // function or event name, or a selector/topic when unknown
	Signature string `json:"signature,omitempty"` // full signature when decoded
	Decoded   string `json:"decoded,omitempty"`   // DecodedVerified, DecodedCommon, or empty
	Args      []Arg  `json:"args,omitempty"`

	Value  string       `json:"value,omitempty"`  // native currency in wei, when non-zero
	Amount string       `json:"amount,omitempty"` // Value in whole units, e.g. "1.5"
	Token  *TokenAmount `json:"token,omitempty"`

	LogIndex *int `json:"logIndex,omitempty"`

	Error      string `json:"error,omitempty"`      // this call failed
	RolledBack bool   `json:"rolledBack,omitempty"` // a call above this one failed, so its effects were undone
	Repeat     int    `json:"repeat,omitempty"`     // >1 when this many identical steps in a row were merged

	Text string `json:"text"` // the step in plain words
}

// Flow is the whole walkthrough for a transaction.
type Flow struct {
	TxHash       string        `json:"txHash"`
	Status       string        `json:"status"` // "success" or "failed"
	Currency     string        `json:"currency"`
	HasCallData  bool          `json:"hasCallData"` // internal calls were recorded for this transaction
	Participants []Participant `json:"participants"`
	Steps        []Step        `json:"steps"`
	TotalSteps   int           `json:"totalSteps"` // steps before the cap (after merging repeats)
	Truncated    bool          `json:"truncated"`
	Summary      string        `json:"summary"`
	Notes        []string      `json:"notes,omitempty"`
}

// Options tunes Build.
type Options struct {
	// MaxSteps caps the steps returned; the rest are counted in TotalSteps. Zero means DefaultMaxSteps.
	MaxSteps int
	// Currency is the chain's native currency symbol. Empty means "ETH".
	Currency string
}

// DefaultMaxSteps keeps a large trace to a size a browser draws without stalling.
const DefaultMaxSteps = 250

// Input is everything Build reads. Contracts and Tokens are keyed by lower-case address
// and may be missing entries: an address without metadata is shown by its short form.
type Input struct {
	Tx        types.Transaction
	Internal  []types.InternalTransaction
	Logs      []types.Log
	Transfers []types.TokenTransfer
	Contracts map[string]*types.Contract
	Tokens    map[string]*types.Token
}

const zeroAddress = "0x0000000000000000000000000000000000000000"

// frame is one call in the call tree, in pre-order.
type frame struct {
	traceAddress string
	parent       int // index into frames, -1 for the root
	depth        int
	from, to     string
	ctx          string // the address whose storage and logs this frame acts for
	callType     string
	value        *big.Int
	input        string
	err          string
	failedAbove  bool // an ancestor failed
	subtreeEnd   int  // index of the last frame in this frame's subtree
}

// Build turns the transaction data into the ordered step model.
func Build(in Input, opts Options) *Flow {
	if opts.MaxSteps <= 0 {
		opts.MaxSteps = DefaultMaxSteps
	}
	if opts.Currency == "" {
		opts.Currency = "ETH"
	}
	n := newNamer(in)

	frames := buildFrames(in.Tx, in.Internal)
	logs := sortedLogs(in.Logs)
	slots, emitters := placeLogs(frames, logs)

	transfersByLog := map[int][]types.TokenTransfer{}
	for _, t := range in.Transfers {
		transfersByLog[t.LogIndex] = append(transfersByLog[t.LogIndex], t)
	}

	var steps []Step
	li := 0
	for fi, f := range frames {
		steps = append(steps, frameStep(f, n, opts.Currency))
		for li < len(logs) && slots[li] == fi {
			// A log sits at the depth of the call that emitted it, when known.
			depth := frames[fi].depth
			if e := emitters[li]; e >= 0 {
				depth = frames[e].depth
			}
			for _, s := range logSteps(logs[li], transfersByLog[logs[li].LogIndex], n) {
				s.Depth = depth
				steps = append(steps, s)
			}
			li++
		}
	}

	steps = mergeRepeats(steps)
	flow := &Flow{
		TxHash:      in.Tx.Hash,
		Status:      "success",
		Currency:    opts.Currency,
		HasCallData: len(frames) > 1,
		TotalSteps:  len(steps),
	}
	if in.Tx.Status != 1 {
		flow.Status = "failed"
	}
	if len(steps) > opts.MaxSteps {
		steps = steps[:opts.MaxSteps]
		flow.Truncated = true
	}
	for i := range steps {
		steps[i].Index = i
	}
	flow.Steps = steps
	flow.Participants = n.participants(steps)
	flow.Summary = summary(in.Tx, frames, logs, in.Transfers, n, opts.Currency)
	flow.Notes = notes(in.Tx, flow, len(logs) > 0, n)
	return flow
}

// buildFrames synthesises the top call from the transaction (the indexer stores only the
// sub-calls) and orders the stored sub-calls into a pre-order tree by trace address.
func buildFrames(tx types.Transaction, internal []types.InternalTransaction) []*frame {
	to := ""
	callType := "call"
	if tx.To != nil {
		to = *tx.To
	} else {
		callType = "create"
		if tx.ContractAddress != nil {
			to = *tx.ContractAddress
		}
	}
	root := &frame{
		traceAddress: "", parent: -1, from: tx.From, to: to, ctx: to, callType: callType,
		value: parseBig(string(tx.Value)), input: tx.InputData,
	}
	if tx.Status != 1 {
		root.err = "reverted"
		switch {
		case tx.RevertReason != nil && *tx.RevertReason != "":
			root.err = *tx.RevertReason
		case tx.Error != nil && *tx.Error != "":
			root.err = *tx.Error
		}
	}

	sorted := make([]types.InternalTransaction, 0, len(internal))
	for _, it := range internal {
		if it.TraceAddress != "" {
			sorted = append(sorted, it)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return compareTraceAddress(sorted[i].TraceAddress, sorted[j].TraceAddress) < 0 })

	frames := []*frame{root}
	byAddr := map[string]int{"": 0}
	for _, it := range sorted {
		parts := strings.Split(it.TraceAddress, ",")
		parent, ok := byAddr[strings.Join(parts[:len(parts)-1], ",")]
		if !ok {
			parent = 0
		}
		f := &frame{
			traceAddress: it.TraceAddress, parent: parent, depth: frames[parent].depth + 1,
			from: it.From, callType: strings.ToLower(it.CallType), value: parseBig(string(it.Value)),
		}
		if f.callType == "" {
			f.callType = "call"
		}
		if it.To != nil {
			f.to = *it.To
		}
		if it.Input != nil {
			f.input = *it.Input
		}
		if it.Error != nil {
			f.err = *it.Error
		}
		// A delegatecall runs another contract's code for the caller: logs it emits
		// carry the caller's address.
		f.ctx = f.to
		if f.callType == "delegatecall" || f.callType == "callcode" {
			f.ctx = frames[parent].ctx
		}
		f.failedAbove = frames[parent].failedAbove || frames[parent].err != ""
		byAddr[it.TraceAddress] = len(frames)
		frames = append(frames, f)
	}
	// Pre-order: a frame's subtree is contiguous, so it ends where the next frame at
	// the same or a shallower depth begins.
	for i := len(frames) - 1; i >= 0; i-- {
		frames[i].subtreeEnd = i
		for j := i + 1; j < len(frames) && frames[j].depth > frames[i].depth; j++ {
			frames[i].subtreeEnd = j
		}
	}
	return frames
}

func compareTraceAddress(a, b string) int {
	pa, pb := strings.Split(a, ","), strings.Split(b, ",")
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x - y
		}
	}
	return len(pa) - len(pb)
}

func sortedLogs(in []types.Log) []types.Log {
	out := make([]types.Log, 0, len(in))
	for _, l := range in {
		if !l.Removed {
			out = append(out, l)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LogIndex < out[j].LogIndex })
	return out
}

// placeLogs decides after which frame each log is shown. The stored trace does not record
// where inside a call a log was emitted, so this is an estimate that keeps two facts true:
// logs stay in log-index order, and each log sits inside a call made by (or on behalf of)
// the contract that emitted it. Within that call it is shown as late as those facts allow,
// i.e. after the sub-calls that came before the next log.
// It returns each log's slot and the frame that emitted it (-1 when unknown).
func placeLogs(frames []*frame, logs []types.Log) (slots, emitter []int) {
	slots = make([]int, len(logs))
	emitter = make([]int, len(logs))
	if len(logs) == 0 {
		return slots, emitter
	}
	cur := 0
	for i, l := range logs {
		e := openEmitter(frames, cur, l.Address)
		if e < 0 {
			for j := cur + 1; j < len(frames); j++ {
				if frames[j].err == "" && !frames[j].failedAbove && strings.EqualFold(frames[j].ctx, l.Address) {
					e, cur = j, j
					break
				}
			}
		}
		emitter[i] = e
		slots[i] = cur
	}
	// Push each log as late as its emitting call and the next log allow.
	limit := len(frames) - 1
	for i := len(logs) - 1; i >= 0; i-- {
		late := slots[i]
		if e := emitter[i]; e >= 0 {
			late = lastLive(frames, e, limit)
		}
		if late < slots[i] {
			late = slots[i]
		}
		slots[i] = late
		limit = late
	}
	return slots, emitter
}

// openEmitter returns the nearest call at or above frame cur that acts for addr, or -1.
func openEmitter(frames []*frame, cur int, addr string) int {
	for i := cur; i >= 0; i = frames[i].parent {
		if strings.EqualFold(frames[i].ctx, addr) {
			return i
		}
	}
	return -1
}

// lastLive is the last frame in e's subtree, not past limit, whose effects were kept
// (a failed call's logs are discarded, so no kept log follows inside it).
func lastLive(frames []*frame, e, limit int) int {
	end := frames[e].subtreeEnd
	if end > limit {
		end = limit
	}
	for end > e && (frames[end].err != "" || frames[end].failedAbove) && !frames[e].failedAbove {
		end--
	}
	return end
}

func parseBig(s string) *big.Int {
	v, ok := new(big.Int).SetString(strings.TrimSpace(s), 0)
	if !ok {
		v, ok = new(big.Int).SetString(strings.TrimSpace(s), 10)
	}
	if !ok {
		return new(big.Int)
	}
	return v
}

func frameStep(f *frame, n *namer, currency string) Step {
	ta := f.traceAddress
	s := Step{
		Kind: KindCall, Depth: f.depth, From: f.from, To: f.to,
		CallType: f.callType, ReadOnly: f.callType == "staticcall", TraceAddress: &ta,
		Error: f.err, RolledBack: f.failedAbove,
	}
	if f.value.Sign() > 0 {
		s.Value = f.value.String()
		s.Amount = formatUnits(f.value, 18)
	}
	input := strings.TrimPrefix(strings.TrimPrefix(f.input, "0x"), "0X")
	switch {
	case strings.HasPrefix(f.callType, "create"):
		s.Kind = KindCreate
	case len(input) < 8 && f.value.Sign() > 0:
		s.Kind = KindValueTransfer
	case len(input) >= 8:
		selector := "0x" + strings.ToLower(input[:8])
		s.Method = selector
		m, src, ok := n.function(f.to, selector)
		if !ok && (f.callType == "delegatecall" || f.callType == "callcode") {
			// An unverified implementation behind a verified proxy: the proxy's ABI
			// usually lists the same functions.
			m, src, ok = n.function(f.ctx, selector)
		}
		if ok {
			s.Method, s.Signature, s.Decoded = m.Name, m.Signature, src
			if data, err := hexBytes(input[8:]); err == nil {
				s.Args = decodeInputs(m.Inputs, data)
			}
		}
	}
	s.Text = stepText(s, n, currency)
	return s
}

func logSteps(l types.Log, transfers []types.TokenTransfer, n *namer) []Step {
	li := l.LogIndex
	if len(transfers) > 0 {
		out := make([]Step, 0, len(transfers))
		for _, t := range transfers {
			idx := li
			tok := &TokenAmount{Address: t.TokenAddress, Standard: t.TokenType, Raw: string(t.Value), TokenID: t.TokenID}
			if tk := n.token(t.TokenAddress); tk != nil {
				tok.Symbol = tk.Symbol
				if tok.Standard == "" {
					tok.Standard = tk.TokenType
				}
				if tok.Standard != "ERC721" {
					tok.Amount = formatUnits(parseBig(string(t.Value)), tk.Decimals)
				}
			}
			s := Step{Kind: KindTokenTransfer, From: t.From, To: t.To, Token: tok, LogIndex: &idx, Method: "Transfer"}
			s.Text = stepText(s, n, "")
			out = append(out, s)
		}
		return out
	}

	topics := []string{}
	for _, t := range []*string{l.Topic0, l.Topic1, l.Topic2, l.Topic3} {
		if t == nil {
			break
		}
		topics = append(topics, *t)
	}
	s := Step{Kind: KindEvent, From: l.Address, To: l.Address, LogIndex: &li}
	if len(topics) == 0 {
		s.Method = "anonymous event"
	} else {
		s.Method = topics[0]
		if m, src, ok := n.event(l.Address, topics[0], len(topics)-1); ok {
			s.Method, s.Signature, s.Decoded = m.Name, m.Signature, src
			if data, err := hexBytes(l.Data); err == nil {
				s.Args = decodeEvent(m, topics, data)
			}
		}
	}
	s.Text = stepText(s, n, "")
	return []Step{s}
}

// mergeRepeats folds runs of identical steps (same call to the same function, with no
// value and nothing in between) into one step with a Repeat count. Long traces often
// repeat the same read many times; merging keeps the diagram readable.
func mergeRepeats(steps []Step) []Step {
	out := make([]Step, 0, len(steps))
	for _, s := range steps {
		if len(out) > 0 && sameStep(out[len(out)-1], s) {
			last := &out[len(out)-1]
			if last.Repeat == 0 {
				last.Repeat = 1
			}
			last.Repeat++
			last.Args = nil
			continue
		}
		out = append(out, s)
	}
	for i := range out {
		if out[i].Repeat > 1 {
			out[i].Text += " (" + strconv.Itoa(out[i].Repeat) + " times in a row)"
		}
	}
	return out
}

func sameStep(a, b Step) bool {
	if a.Kind != b.Kind || (a.Kind != KindCall && a.Kind != KindEvent) {
		return false
	}
	return a.Depth == b.Depth && strings.EqualFold(a.From, b.From) && strings.EqualFold(a.To, b.To) &&
		a.Method == b.Method && a.CallType == b.CallType && a.Value == "" && b.Value == "" &&
		a.Error == "" && b.Error == "" && a.RolledBack == b.RolledBack
}
