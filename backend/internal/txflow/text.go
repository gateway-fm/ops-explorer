package txflow

import (
	"fmt"
	"math/big"
	"strings"

	"explorer/internal/types"
	ethcommon "explorer/pkg/eth/common"
)

// namer resolves addresses to names and ABIs from the contract and token metadata.
type namer struct {
	contracts map[string]*types.Contract
	tokens    map[string]*types.Token
	abis      map[string]*abiIndex
	sender    string
}

func newNamer(in Input) *namer {
	n := &namer{contracts: map[string]*types.Contract{}, tokens: map[string]*types.Token{}, abis: map[string]*abiIndex{}, sender: strings.ToLower(in.Tx.From)}
	for k, v := range in.Contracts {
		if v != nil {
			n.contracts[strings.ToLower(k)] = v
		}
	}
	for k, v := range in.Tokens {
		if v != nil {
			n.tokens[strings.ToLower(k)] = v
		}
	}
	return n
}

func (n *namer) contract(addr string) *types.Contract { return n.contracts[strings.ToLower(addr)] }
func (n *namer) token(addr string) *types.Token       { return n.tokens[strings.ToLower(addr)] }

func (n *namer) abi(addr string) *abiIndex {
	key := strings.ToLower(addr)
	if idx, ok := n.abis[key]; ok {
		return idx
	}
	idx := newABIIndex()
	if c := n.contracts[key]; c != nil && c.IsVerified {
		idx = parseABI(c.ABI)
	}
	n.abis[key] = idx
	return idx
}

func (n *namer) function(addr, selector string) (method, string, bool) {
	if m, ok := n.abi(addr).function(selector); ok {
		return m, DecodedVerified, true
	}
	if m, ok := common.function(selector); ok {
		return m, DecodedCommon, true
	}
	return method{}, "", false
}

func (n *namer) event(addr, topic0 string, indexed int) (method, string, bool) {
	if m, ok := n.abi(addr).event(topic0, indexed); ok {
		return m, DecodedVerified, true
	}
	if m, ok := common.event(topic0, indexed); ok {
		return m, DecodedCommon, true
	}
	return method{}, "", false
}

func isHexAddress(a string) bool { return ethcommon.IsHexAddress(a) }

func shortAddress(a string) string {
	if !isHexAddress(a) {
		return a
	}
	return a[:6] + "…" + a[len(a)-4:]
}

// label is the short name a participant lane carries.
func (n *namer) label(addr string) string {
	if addr == "" {
		return "new contract"
	}
	if t := n.token(addr); t != nil && t.Symbol != "" {
		return t.Symbol
	}
	if c := n.contract(addr); c != nil && c.IsVerified && c.ContractName != nil && *c.ContractName != "" {
		return *c.ContractName
	}
	return shortAddress(addr)
}

// name is how a sentence refers to an address.
func (n *namer) name(addr string) string {
	switch {
	case addr == "":
		return "a new contract"
	case strings.EqualFold(addr, zeroAddress):
		return "the zero address"
	case !isHexAddress(addr):
		return "a private address"
	}
	if t := n.token(addr); t != nil && t.Symbol != "" {
		return "the " + t.Symbol + " token"
	}
	if c := n.contract(addr); c != nil {
		if c.IsVerified && c.ContractName != nil && *c.ContractName != "" {
			return *c.ContractName
		}
		return "contract " + shortAddress(addr)
	}
	if strings.EqualFold(addr, n.sender) {
		return "the sender " + shortAddress(addr)
	}
	return shortAddress(addr)
}

func (n *namer) kind(addr string) string {
	switch {
	case !isHexAddress(addr) && addr != "":
		return "private"
	case n.token(addr) != nil:
		return "token"
	case n.contract(addr) != nil:
		return "contract"
	}
	return "account"
}

// participants lists each address in the steps once, in order of first appearance.
func (n *namer) participants(steps []Step) []Participant {
	seen := map[string]bool{}
	out := []Participant{}
	add := func(a string) {
		k := strings.ToLower(a)
		if seen[k] {
			return
		}
		seen[k] = true
		p := Participant{Address: a, Label: n.label(a), Kind: n.kind(a)}
		if c := n.contract(a); c != nil && c.IsVerified {
			p.Verified = true
		}
		out = append(out, p)
	}
	for _, s := range steps {
		add(s.From)
		add(s.To)
	}
	return out
}

func methodLabel(s Step) string {
	if s.Decoded != "" {
		return s.Method
	}
	if strings.HasPrefix(s.Method, "0x") && len(s.Method) == 10 {
		return "function " + s.Method
	}
	if strings.HasPrefix(s.Method, "0x") && len(s.Method) > 12 {
		return "event " + s.Method[:10] + "…"
	}
	return s.Method
}

func stepText(s Step, n *namer, currency string) string {
	var b strings.Builder
	from, to := n.name(s.From), n.name(s.To)
	value := ""
	if s.Value != "" {
		value = s.Amount + " " + currency
	}
	switch s.Kind {
	case KindCreate:
		fmt.Fprintf(&b, "%s created a new contract", capitalize(from))
		if s.To != "" {
			fmt.Fprintf(&b, " at %s", shortAddress(s.To))
		}
		if value != "" {
			fmt.Fprintf(&b, " and funded it with %s", value)
		}
	case KindValueTransfer:
		fmt.Fprintf(&b, "%s sent %s to %s", capitalize(from), value, to)
	case KindCall:
		m := methodLabel(s)
		switch {
		case s.Method == "" && (s.CallType == "delegatecall" || s.CallType == "callcode"):
			fmt.Fprintf(&b, "%s ran the code of %s", capitalize(from), to)
		case s.Method == "":
			fmt.Fprintf(&b, "%s called %s with no function", capitalize(from), to)
		case s.CallType == "delegatecall" || s.CallType == "callcode":
			fmt.Fprintf(&b, "%s ran %s using the code of %s", capitalize(from), m, to)
		case s.ReadOnly:
			fmt.Fprintf(&b, "%s read %s from %s", capitalize(from), m, to)
		default:
			fmt.Fprintf(&b, "%s called %s on %s", capitalize(from), m, to)
		}
		if value != "" {
			fmt.Fprintf(&b, ", sending %s", value)
		}
	case KindTokenTransfer:
		b.WriteString(tokenText(s, n))
	case KindEvent:
		fmt.Fprintf(&b, "%s reported %s", capitalize(from), methodLabel(s))
		if args := shortArgs(s.Args, n); args != "" {
			fmt.Fprintf(&b, " (%s)", args)
		}
	}
	if s.Error != "" {
		fmt.Fprintf(&b, ". This call failed: %s", s.Error)
	} else if s.RolledBack {
		b.WriteString(". Undone, because a call above it failed")
	}
	return b.String()
}

func tokenText(s Step, n *namer) string {
	t := s.Token
	sym := t.Symbol
	if sym == "" {
		sym = "tokens of " + shortAddress(t.Address)
	}
	what := ""
	switch {
	case t.Standard == "ERC721" && t.TokenID != nil:
		what = sym + " #" + *t.TokenID
	case t.Standard == "ERC1155" && t.TokenID != nil:
		what = amountOrRaw(t) + " of " + sym + " #" + *t.TokenID
	default:
		what = amountOrRaw(t) + " " + sym
	}
	switch {
	case strings.EqualFold(s.From, zeroAddress):
		return fmt.Sprintf("%s were created (minted) for %s", capitalize(what), n.name(s.To))
	case strings.EqualFold(s.To, zeroAddress):
		return fmt.Sprintf("%s destroyed (burned) %s", capitalize(n.name(s.From)), what)
	}
	return fmt.Sprintf("%s sent %s to %s", capitalize(n.name(s.From)), what, n.name(s.To))
}

func amountOrRaw(t *TokenAmount) string {
	if t.Amount != "" {
		return t.Amount
	}
	return t.Raw + " units"
}

// shortArgs renders up to three simple event arguments, naming addresses.
func shortArgs(args []Arg, n *namer) string {
	var parts []string
	for _, a := range args {
		if a.Value == "" || len(parts) == 3 {
			continue
		}
		v := a.Value
		if a.Type == "address" {
			v = n.name(v)
		} else if len(v) > 24 {
			v = v[:21] + "…"
		}
		name := a.Name
		if name == "" {
			name = a.Type
		}
		parts = append(parts, name+": "+v)
	}
	if len(parts) < countSimple(args) {
		parts = append(parts, "…")
	}
	return strings.Join(parts, ", ")
}

func countSimple(args []Arg) int {
	c := 0
	for _, a := range args {
		if a.Value != "" {
			c++
		}
	}
	return c
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// formatUnits scales v down by decimals and trims trailing zeros: 1500000 with 6 -> "1.5".
// It returns "" for a decimal count outside 0..77 (no uint256 needs more), so callers
// fall back to the raw amount rather than show raw units as whole tokens.
func formatUnits(v *big.Int, decimals int) string {
	if v == nil {
		return "0"
	}
	if decimals < 0 || decimals > 77 {
		return ""
	}
	if decimals == 0 {
		return v.String()
	}
	neg := v.Sign() < 0
	abs := new(big.Int).Abs(v)
	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	whole, frac := new(big.Int).QuoRem(abs, unit, new(big.Int))
	out := whole.String()
	if frac.Sign() != 0 {
		fs := fmt.Sprintf("%0*s", decimals, frac.String())
		out += "." + strings.TrimRight(fs, "0")
	}
	if neg {
		out = "-" + out
	}
	return out
}

// summary is one sentence on what the transaction did.
func summary(tx types.Transaction, frames []*frame, logs []types.Log, transfers []types.TokenTransfer, n *namer, currency string, partial bool) string {
	root := frameStep(frames[0], n, currency)
	var b strings.Builder
	b.WriteString(root.Text)
	if tx.Status == 1 {
		calls := len(frames) - 1
		if calls > 0 || len(transfers) > 0 || len(logs) > 0 {
			var parts []string
			if calls > 0 {
				parts = append(parts, plural(calls, "internal call"))
			}
			if len(transfers) > 0 {
				parts = append(parts, plural(len(transfers), "token transfer"))
			}
			if len(logs) > 0 {
				parts = append(parts, plural(len(logs), "event"))
			}
			led := ". It led to "
			if partial {
				led += "at least "
			}
			b.WriteString(led + joinList(parts))
		}
	}
	b.WriteString(".")
	return b.String()
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func joinList(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func notes(in Input, f *Flow, hasLogs bool, n *namer) []string {
	tx := in.Tx
	var out []string
	if f.Truncated {
		out = append(out, fmt.Sprintf("Showing the first %d steps; the transaction has %d calls and events in all.", len(f.Steps), f.TotalSteps))
	}
	if in.Partial {
		out = append(out, fmt.Sprintf("Only the first %d internal calls, events or token transfers of this transaction were read, so this flow and its counts are partial.", types.MaxTxScopedRows))
	}
	calledContract := tx.To != nil && n.contract(*tx.To) != nil || len(strings.TrimPrefix(tx.InputData, "0x")) >= 8
	switch {
	case in.InternalUnavailable:
		out = append(out, "Internal calls could not be read; showing events and token transfers only.")
	case !f.HasCallData && calledContract:
		out = append(out, "No internal calls are recorded for this transaction: either it made none, or call tracing was off when its block was indexed. Events and token transfers are still shown.")
	}
	if f.HasCallData && hasLogs {
		out = append(out, "Events are shown inside a call made by or for the contract that emitted them. Their exact position is not recorded, so it is estimated from the order of events and calls.")
	}
	return out
}
