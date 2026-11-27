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
	ethcommon "explorer/pkg/eth/common"
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
// does not expand (arrays and tuples). A string or bytes value longer than 256 bytes
// is cut there and ends in "…".
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

	// key identifies a step by its raw data (call input, event topics and data), so
	// repeats merge only when they are byte-for-byte the same, whatever the decoder
	// shows. Empty for steps that never merge.
	key string
}

// Flow is the whole walkthrough for a transaction.
type Flow struct {
	TxHash       string        `json:"txHash"`
	Status       string        `json:"status"` // "success" or "failed"
	Currency     string        `json:"currency"`
	HasCallData  bool          `json:"hasCallData"` // internal calls were recorded for this transaction
	Participants []Participant `json:"participants"`
	Steps        []Step        `json:"steps"`
	TotalSteps   int           `json:"totalSteps"` // len(Steps), or when Truncated the calls and events in all, before merging
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

// maxRawItems bounds the calls and events Build turns into steps, merged repeats
// included, so decoding and wording a trace of thousands of identical calls costs a
// bounded amount of work. Ordering the calls and placing the events still covers the
// whole input, which the reads bound to types.MaxTxScopedRows rows of each kind.
const maxRawItems = 10_000

// Input is everything Build reads. Contracts and Tokens are keyed by lower-case address
// and may be missing entries: an address without metadata is shown by its short form.
type Input struct {
	// InternalUnavailable is set when the internal calls could not be read, as opposed
	// to there being none recorded.
	InternalUnavailable bool
	// Partial is set when the internal calls, logs or token transfers were cut at
	// types.MaxTxScopedRows, so counts are lower bounds.
	Partial bool

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
	static       bool // a staticcall or inside one: cannot emit logs
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
	moments := walk(frames)
	at := placeLogs(frames, moments, logs)

	transfersByLog := map[int][]types.TokenTransfer{}
	for _, t := range in.Transfers {
		transfersByLog[t.LogIndex] = append(transfersByLog[t.LogIndex], t)
	}

	// Steps are merged as they are built: a call or event whose raw data repeats the
	// previous step only bumps its count, without being decoded again. Building stops
	// one step past the cap, or after maxRawItems calls and events, so a huge trace
	// costs decoding work for about the steps returned, not for all of it.
	var steps []Step
	raw := 0
	stopped := false
	merged := func(key string, depth int) bool {
		if key == "" || len(steps) == 0 {
			return false
		}
		last := &steps[len(steps)-1]
		if last.key != key || last.Depth != depth {
			return false
		}
		last.Repeat = max(last.Repeat, 1) + 1
		return true
	}
	full := func() bool { return len(steps) > opts.MaxSteps || raw >= maxRawItems }
	li := 0
build:
	for mi, m := range moments {
		f := frames[m.frame]
		if m.enter {
			if full() {
				stopped = true
				break
			}
			raw++
			if k := frameKey(f); !merged(k, f.depth) {
				s := frameStep(f, n, opts.Currency)
				s.key = k
				steps = append(steps, s)
			}
			continue
		}
		// A log sits at the depth of the call running when it was emitted.
		for ; li < len(logs) && at[li] == mi; li++ {
			if full() {
				stopped = true
				break build
			}
			raw++
			l := logs[li]
			transfers := transfersByLog[l.LogIndex]
			if len(transfers) == 0 {
				if k := logKey(l); !merged(k, f.depth) {
					s := eventStep(l, n)
					s.Depth, s.key = f.depth, k
					steps = append(steps, s)
				}
				continue
			}
			// One log can carry a whole batch of transfers: build only what still fits.
			for _, s := range transferSteps(l, transfers, n, opts.MaxSteps+1-len(steps)) {
				s.Depth = f.depth
				steps = append(steps, s)
			}
		}
	}
	for i := range steps {
		if steps[i].Repeat > 1 {
			steps[i].Text += " (" + strconv.Itoa(steps[i].Repeat) + " times in a row)"
		}
	}

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
		stopped = true
	}
	if stopped {
		flow.Truncated = true
		flow.TotalSteps = rawSteps(frames, logs, transfersByLog)
	}
	for i := range steps {
		steps[i].Index = i
	}
	flow.Steps = steps
	flow.Participants = n.participants(steps)
	flow.Summary = summary(in.Tx, frames, logs, in.Transfers, n, opts.Currency, in.Partial)
	flow.Notes = notes(in, flow, len(logs) > 0, n)
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
			// The tracer repeats the caller's value on a delegatecall frame, and a
			// callcode sends it to the caller itself: no currency moves either way.
			f.value = new(big.Int)
		}
		f.failedAbove = frames[parent].failedAbove || frames[parent].err != ""
		f.static = frames[parent].static || f.callType == "staticcall"
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

// moment is one point in the walk of the call tree: a call starting (enter), or a gap
// between call boundaries, when frame is the innermost call running. A gap follows each
// call's start and each sub-call's return; a call emits its logs in its gaps.
type moment struct {
	frame int
	enter bool
}

// walk lists the moments of the call tree in execution order.
func walk(frames []*frame) []moment {
	out := make([]moment, 0, 2*len(frames))
	var open []int
	closeTo := func(i int) { // return from every open call whose subtree ends before frame i
		for len(open) > 1 && frames[open[len(open)-1]].subtreeEnd < i {
			open = open[:len(open)-1]
			out = append(out, moment{frame: open[len(open)-1]})
		}
	}
	for i := range frames {
		closeTo(i)
		open = append(open, i)
		out = append(out, moment{frame: i, enter: true}, moment{frame: i})
	}
	closeTo(len(frames))
	return out
}

// canEmit is whether a call can have emitted logs that were kept: it did not fail, no
// call above it failed, and it is not read-only.
func canEmit(f *frame) bool { return f.err == "" && !f.failedAbove && !f.static }

// placeLogs picks the gap (an index into moments) where each log is shown. The stored
// trace does not record where inside a call a log was emitted, so this is an estimate
// that keeps these facts true: logs stay in log-index order, and each sits in a gap of a
// kept, non-read-only call acting for the address that emitted it.
//
// A backward pass finds each log's latest gap that still leaves room for the logs after
// it. A forward pass then picks a call between the previous log's gap and that bound:
//   - the previous log's call, while more logs of this address remain than unused calls
//     that could still take them (that call emitted several, e.g. Approval and Transfer);
//   - else the earliest call no log used yet, so two calls to one token each keep
//     their own Transfer;
//   - else the earliest call that fits, which is the previous log's call when that one
//     acts for the same address.
//
// The log goes in the latest gap of that call that comes before the next unused call
// to the same address starts: after the sub-calls the call made first, but never past
// a nested or later call that is still waiting for its logs (a callback into the same
// contract). A call that only forwards to a delegatecall (a proxy, a multicall) starts
// as used, since the code it delegates to emits in its name.
//
// This assumes every kept, non-read-only call emits at least one log for its address,
// and that extra logs belong to the earlier call: when a call emits none, a log of an
// earlier call to the same contract is shown in it, and when every call emits several
// (two transferFrom calls, each with Approval and Transfer), the first call collects
// the extras. A log with no gap at all for its address stays at the previous log's gap.
func placeLogs(frames []*frame, moments []moment, logs []types.Log) []int {
	at := make([]int, len(logs))
	if len(logs) == 0 {
		return at
	}
	// The gaps of each emitting call, and per address the gaps of the calls acting for
	// it, both in walk order, so every lookup is a binary search.
	gapsOf := map[int][]int{}
	byAddr := map[string][]int{}
	for mi, m := range moments {
		if f := frames[m.frame]; !m.enter && canEmit(f) {
			gapsOf[m.frame] = append(gapsOf[m.frame], mi)
			k := strings.ToLower(f.ctx)
			byAddr[k] = append(byAddr[k], mi)
		}
	}
	latest := func(list []int, bound int) int { // the last entry <= bound, or -1
		if k := sort.SearchInts(list, bound+1) - 1; k >= 0 {
			return list[k]
		}
		return -1
	}
	// runEnd[a][k] is the index just past the run of byAddr[a] entries that belong to
	// the same call as entry k, so scans move call by call.
	runEnd := map[string][]int{}
	for a, c := range byAddr {
		r := make([]int, len(c))
		for k := len(c) - 1; k >= 0; k-- {
			r[k] = k + 1
			if k+1 < len(c) && moments[c[k+1]].frame == moments[c[k]].frame {
				r[k] = r[k+1]
			}
		}
		runEnd[a] = r
	}

	addrs := make([]string, len(logs))
	bound := make([]int, len(logs))
	limit := len(moments) - 1
	for i := len(logs) - 1; i >= 0; i-- {
		addrs[i] = strings.ToLower(logs[i].Address)
		bound[i] = latest(byAddr[addrs[i]], limit)
		if bound[i] >= 0 {
			limit = bound[i]
		}
	}

	used := forwarders(frames)

	// remaining[i] counts the placeable logs from i on with log i's address; lastBound
	// is the bound of an address's last placeable log.
	remaining := make([]int, len(logs))
	lastBound := map[string]int{}
	count := map[string]int{}
	for i := len(logs) - 1; i >= 0; i-- {
		if bound[i] < 0 {
			continue
		}
		a := addrs[i]
		count[a]++
		remaining[i] = count[a]
		if _, ok := lastBound[a]; !ok {
			lastBound[a] = bound[i]
		}
	}
	open := newOpenCalls(frames, gapsOf, used, lastBound)

	prev, prevFrame := 1, -1 // the gap after the top call starts
	for i := range logs {
		if bound[i] < 0 {
			at[i] = prev
			continue
		}
		a, c, re := addrs[i], byAddr[addrs[i]], runEnd[addrs[i]]
		alive := open.alive(a, prev)
		prevFits := prevFrame >= 0 && strings.EqualFold(frames[prevFrame].ctx, a) && latest(gapsOf[prevFrame], bound[i]) >= prev

		lo, hi := sort.SearchInts(c, prev), sort.SearchInts(c, bound[i]+1)
		pick := -1
		if prevFits && remaining[i] > alive {
			pick = prevFrame
		}
		for k := lo; pick < 0 && k < hi; k = re[k] {
			if f := moments[c[k]].frame; !used[f] {
				pick = f
			}
		}
		if pick < 0 {
			pick = moments[c[lo]].frame
		}

		gs := gapsOf[pick]
		first := gs[sort.SearchInts(gs, prev)]
		end := bound[i]
		for k := sort.SearchInts(c, first+1); k < hi; k = re[k] {
			if f := moments[c[k]].frame; f != pick && !used[f] {
				end = c[k] - 1
				break
			}
		}
		at[i] = latest(gs, end)
		used[pick] = true
		open.use(pick)
		prev, prevFrame = at[i], pick
	}
	return at
}

// forwarders marks the calls that hand their work to a kept delegatecall: a proxy
// passing its own input on, or a contract delegatecalling itself (a multicall). Their
// logs come from the code they delegate to, so placement treats them as already used.
// A delegatecall to other code with other input (a library, a Safe's MultiSend) does
// not count: the caller still runs and emits its own logs around it.
func forwarders(frames []*frame) map[int]bool {
	out := map[int]bool{}
	norm := func(s string) string { return strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")) }
	for _, f := range frames[1:] {
		if (f.callType != "delegatecall" && f.callType != "callcode") || !canEmit(f) {
			continue
		}
		p := frames[f.parent]
		if norm(f.input) == norm(p.input) || strings.EqualFold(f.to, p.ctx) {
			out[f.parent] = true
		}
	}
	return out
}

// openCalls counts, per address, the unused calls a remaining log could still go to:
// those that start before the address's last log is placed and end at or after the
// previous log. Each address's calls are ordered by their last gap, so the ones that
// ended before the previous log drop out as placement moves forward.
type openCalls struct {
	gapsOf  map[int][]int
	byAddr  map[string]*openList
	counted map[int]string // frame -> its address, while it is counted as open
}

type openList struct {
	frames      []int
	next, count int
}

func newOpenCalls(frames []*frame, gapsOf map[int][]int, used map[int]bool, lastBound map[string]int) *openCalls {
	o := &openCalls{gapsOf: gapsOf, byAddr: map[string]*openList{}, counted: map[int]string{}}
	for a := range lastBound {
		o.byAddr[a] = &openList{}
	}
	for fi, f := range frames {
		a := strings.ToLower(f.ctx)
		gs := gapsOf[fi]
		lb, ok := lastBound[a]
		if len(gs) == 0 || used[fi] || !ok || gs[0] > lb {
			continue
		}
		l := o.byAddr[a]
		l.frames = append(l.frames, fi)
		l.count++
		o.counted[fi] = a
	}
	for _, l := range o.byAddr {
		sort.SliceStable(l.frames, func(x, y int) bool {
			gx, gy := gapsOf[l.frames[x]], gapsOf[l.frames[y]]
			return gx[len(gx)-1] < gy[len(gy)-1]
		})
	}
	return o
}

// alive returns how many calls for address a are still open at gap prev.
func (o *openCalls) alive(a string, prev int) int {
	l := o.byAddr[a]
	for l.next < len(l.frames) {
		gs := o.gapsOf[l.frames[l.next]]
		if gs[len(gs)-1] >= prev {
			break
		}
		o.use(l.frames[l.next])
		l.next++
	}
	return l.count
}

// use stops counting call fi as open.
func (o *openCalls) use(fi int) {
	if a, ok := o.counted[fi]; ok {
		delete(o.counted, fi)
		o.byAddr[a].count--
	}
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
			s.Args = decodeInputs(m.Inputs, ethcommon.FromHex(input[8:]))
		}
	}
	s.Text = stepText(s, n, currency)
	return s
}

// frameKey is a call's raw identity for merging repeats.
func frameKey(f *frame) string {
	return strings.Join([]string{"call", f.callType, strings.ToLower(f.from), strings.ToLower(f.to),
		f.value.String(), strings.ToLower(f.input), f.err, strconv.FormatBool(f.failedAbove)}, "|")
}

// logKey is an event's raw identity for merging repeats.
func logKey(l types.Log) string {
	parts := []string{"log", strings.ToLower(l.Address)}
	for _, t := range []*string{l.Topic0, l.Topic1, l.Topic2, l.Topic3} {
		if t == nil {
			break
		}
		parts = append(parts, strings.ToLower(*t))
	}
	return strings.Join(append(parts, strings.ToLower(l.Data)), "|")
}

// transferSteps builds up to limit token-transfer steps for one log.
func transferSteps(l types.Log, transfers []types.TokenTransfer, n *namer, limit int) []Step {
	li := l.LogIndex
	if len(transfers) > limit {
		transfers = transfers[:max(limit, 0)]
	}
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

// eventStep builds the step for a log that carries no token transfer.
func eventStep(l types.Log, n *namer) Step {
	li := l.LogIndex
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
			s.Args = decodeEvent(m, topics, ethcommon.FromHex(l.Data))
		}
	}
	s.Text = stepText(s, n, "")
	return s
}

// rawSteps counts the steps a flow has before merging: one per call, and one per log
// (or per token transfer the log carries).
func rawSteps(frames []*frame, logs []types.Log, transfersByLog map[int][]types.TokenTransfer) int {
	total := len(frames)
	for _, l := range logs {
		total += max(1, len(transfersByLog[l.LogIndex]))
	}
	return total
}
