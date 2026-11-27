package txflow

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"

	ethcommon "explorer/pkg/eth/common"
	"explorer/pkg/eth/crypto"
)

// abiParam is one input of a function or event in a contract ABI.
type abiParam struct {
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Indexed    bool       `json:"indexed"`
	Components []abiParam `json:"components"`
}

type abiEntry struct {
	Type   string     `json:"type"`
	Name   string     `json:"name"`
	Inputs []abiParam `json:"inputs"`
}

// method is a function or event resolved from an ABI, keyed by its selector or topic.
type method struct {
	Name      string
	Signature string
	Inputs    []abiParam
}

// abiIndex maps 4-byte selectors (functions) and topic0 hashes (events) to their definitions.
// Events are keyed by topic0 plus the number of indexed inputs, because ERC-20 and ERC-721
// Transfer share a topic0 and differ only in which inputs are indexed.
type abiIndex struct {
	functions map[string]method
	events    map[string]method
}

func newABIIndex() *abiIndex {
	return &abiIndex{functions: map[string]method{}, events: map[string]method{}}
}

// parseABI reads a JSON ABI. A malformed ABI yields an empty index, never an error:
// the flow then falls back to selectors, as for an unverified contract.
func parseABI(raw json.RawMessage) *abiIndex {
	idx := newABIIndex()
	if len(raw) == 0 {
		return idx
	}
	var entries []abiEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		// Some sources store the ABI as a JSON string holding the array.
		var s string
		if json.Unmarshal(raw, &s) != nil || json.Unmarshal([]byte(s), &entries) != nil {
			return idx
		}
	}
	for _, e := range entries {
		idx.add(e)
	}
	return idx
}

func (idx *abiIndex) add(e abiEntry) {
	if e.Name == "" {
		return
	}
	types := make([]string, len(e.Inputs))
	for i, in := range e.Inputs {
		types[i] = canonicalType(in)
	}
	sig := e.Name + "(" + strings.Join(types, ",") + ")"
	hash := crypto.Keccak256([]byte(sig))
	m := method{Name: e.Name, Signature: sig, Inputs: e.Inputs}
	switch e.Type {
	case "function", "":
		idx.functions["0x"+hex.EncodeToString(hash[:4])] = m
	case "event":
		idx.events[eventKey("0x"+hex.EncodeToString(hash), indexedCount(e.Inputs))] = m
	}
}

func indexedCount(in []abiParam) int {
	n := 0
	for _, p := range in {
		if p.Indexed {
			n++
		}
	}
	return n
}

func eventKey(topic0 string, indexed int) string {
	return strings.ToLower(topic0) + "/" + strconv.Itoa(indexed)
}

func (idx *abiIndex) function(selector string) (method, bool) {
	m, ok := idx.functions[strings.ToLower(selector)]
	return m, ok
}

func (idx *abiIndex) event(topic0 string, indexed int) (method, bool) {
	m, ok := idx.events[eventKey(topic0, indexed)]
	return m, ok
}

// canonicalType renders a parameter type as it appears in a signature: tuples
// expand to their component types, e.g. "tuple[]" -> "(address,uint256)[]".
func canonicalType(p abiParam) string {
	if !strings.HasPrefix(p.Type, "tuple") {
		return p.Type
	}
	parts := make([]string, len(p.Components))
	for i, c := range p.Components {
		parts[i] = canonicalType(c)
	}
	return "(" + strings.Join(parts, ",") + ")" + strings.TrimPrefix(p.Type, "tuple")
}

// common holds a few standard token functions and events, used when a contract is not
// verified, so the most frequent steps still read as words.
var common = func() *abiIndex {
	idx := newABIIndex()
	addr := func(n string, indexed bool) abiParam { return abiParam{Name: n, Type: "address", Indexed: indexed} }
	u256 := func(n string, indexed bool) abiParam { return abiParam{Name: n, Type: "uint256", Indexed: indexed} }
	fn := func(name string, in ...abiParam) { idx.add(abiEntry{Type: "function", Name: name, Inputs: in}) }
	ev := func(name string, in ...abiParam) { idx.add(abiEntry{Type: "event", Name: name, Inputs: in}) }

	fn("transfer", addr("to", false), u256("amount", false))
	fn("transferFrom", addr("from", false), addr("to", false), u256("amount", false))
	fn("approve", addr("spender", false), u256("amount", false))
	fn("balanceOf", addr("account", false))
	fn("allowance", addr("owner", false), addr("spender", false))
	fn("totalSupply")
	fn("decimals")
	fn("symbol")
	fn("name")
	fn("mint", addr("to", false), u256("amount", false))
	fn("burn", u256("amount", false))
	fn("deposit")
	fn("withdraw", u256("amount", false))
	fn("safeTransferFrom", addr("from", false), addr("to", false), u256("tokenId", false))
	fn("ownerOf", u256("tokenId", false))

	ev("Transfer", addr("from", true), addr("to", true), u256("value", false))
	ev("Transfer", addr("from", true), addr("to", true), u256("tokenId", true))
	ev("Approval", addr("owner", true), addr("spender", true), u256("value", false))
	ev("Approval", addr("owner", true), addr("approved", true), u256("tokenId", true))
	ev("ApprovalForAll", addr("owner", true), addr("operator", true), abiParam{Name: "approved", Type: "bool"})
	ev("Deposit", addr("account", true), u256("amount", false))
	ev("Withdrawal", addr("account", true), u256("amount", false))
	ev("OwnershipTransferred", addr("previousOwner", true), addr("newOwner", true))
	return idx
}()

// decodeInputs decodes ABI-encoded values. Simple types (address, bool, integers, fixed
// bytes, string, bytes) get a value; arrays and tuples keep their type with an empty value.
// Decoding stops at the first value the data cannot hold, so a short or malformed input
// yields fewer arguments rather than an error.
func decodeInputs(params []abiParam, data []byte) []Arg {
	args := make([]Arg, 0, len(params))
	slot := 0
	for _, p := range params {
		size, ok := headSlots(p)
		if !ok {
			break
		}
		if (slot+size)*32 > len(data) {
			break
		}
		v := ""
		if isElementary(p.Type) {
			// Arrays and tuples keep an empty value: their head word is an offset or
			// only their first element, never the whole value.
			v = decodeAt(p.Type, data, slot)
		}
		args = append(args, Arg{Name: p.Name, Type: canonicalType(p), Value: v})
		slot += size
	}
	return args
}

// decodeEvent decodes an event's indexed inputs from topics and the rest from data.
func decodeEvent(m method, topics []string, data []byte) []Arg {
	var nonIndexed []abiParam
	for _, p := range m.Inputs {
		if !p.Indexed {
			nonIndexed = append(nonIndexed, p)
		}
	}
	fromData := decodeInputs(nonIndexed, data)

	args := make([]Arg, 0, len(m.Inputs))
	ti, di := 1, 0
	for _, p := range m.Inputs {
		if p.Indexed {
			if ti >= len(topics) {
				break
			}
			raw := ethcommon.FromHex(topics[ti])
			ti++
			v := ""
			if len(raw) == 32 {
				if isElementary(p.Type) && !isDynamic(p.Type) {
					v = decodeAt(p.Type, raw, 0)
				} else {
					// An indexed string, bytes, array or tuple is stored as its hash.
					v = "0x" + hex.EncodeToString(raw)
				}
			}
			args = append(args, Arg{Name: p.Name, Type: canonicalType(p), Value: v})
			continue
		}
		if di < len(fromData) {
			args = append(args, fromData[di])
			di++
		}
	}
	return args
}

func isDynamic(t string) bool {
	return t == "string" || t == "bytes" || strings.HasSuffix(t, "[]")
}

func isElementary(t string) bool {
	if strings.Contains(t, "[") || strings.HasPrefix(t, "tuple") {
		return false
	}
	return t == "address" || t == "bool" || t == "string" || t == "bytes" ||
		strings.HasPrefix(t, "uint") || strings.HasPrefix(t, "int") || strings.HasPrefix(t, "bytes")
}

// headSlots is how many 32-byte head slots a parameter takes. Dynamic values take one
// (an offset). Static tuples and fixed arrays take the sum of their parts.
func headSlots(p abiParam) (int, bool) {
	t := p.Type
	if strings.HasSuffix(t, "]") {
		open := strings.LastIndex(t, "[")
		if open < 0 {
			return 0, false
		}
		dim := t[open+1 : len(t)-1]
		if dim == "" {
			return 1, true
		}
		n, err := strconv.Atoi(dim)
		if err != nil || n <= 0 || n > 1024 {
			return 0, false
		}
		inner := p
		inner.Type = t[:open]
		if dynamicParam(inner) {
			return 1, true
		}
		s, ok := headSlots(inner)
		if !ok || s > maxHeadSlots/n {
			return 0, false
		}
		return n * s, true
	}
	if t == "tuple" {
		if dynamicParam(p) {
			return 1, true
		}
		total := 0
		for _, c := range p.Components {
			s, ok := headSlots(c)
			if !ok || total+s > maxHeadSlots {
				return 0, false
			}
			total += s
		}
		return total, true
	}
	return 1, true
}

func dynamicParam(p abiParam) bool {
	t := p.Type
	if t == "string" || t == "bytes" || strings.HasSuffix(t, "[]") {
		return true
	}
	if strings.HasSuffix(t, "]") {
		inner := p
		inner.Type = t[:strings.LastIndex(t, "[")]
		return dynamicParam(inner)
	}
	if t == "tuple" {
		for _, c := range p.Components {
			if dynamicParam(c) {
				return true
			}
		}
	}
	return false
}

const maxDecodedBytes = 256

// maxHeadSlots bounds the head size of one static parameter (nested fixed arrays
// multiply), so slot arithmetic cannot overflow; a larger parameter is not decoded.
const maxHeadSlots = 1 << 16

func decodeAt(t string, data []byte, slot int) string {
	word := data[slot*32 : slot*32+32]
	switch {
	case t == "address":
		return "0x" + hex.EncodeToString(word[12:])
	case t == "bool":
		if new(big.Int).SetBytes(word).Sign() != 0 {
			return "true"
		}
		return "false"
	case strings.HasPrefix(t, "uint"):
		return new(big.Int).SetBytes(word).String()
	case strings.HasPrefix(t, "int"):
		v := new(big.Int).SetBytes(word)
		if word[0]&0x80 != 0 {
			v.Sub(v, new(big.Int).Lsh(big.NewInt(1), 256))
		}
		return v.String()
	case t == "string" || t == "bytes":
		b, ok := dynamicBytes(data, word)
		if !ok {
			return ""
		}
		if t == "string" {
			return string(b)
		}
		return "0x" + hex.EncodeToString(b)
	case strings.HasPrefix(t, "bytes"):
		n, err := strconv.Atoi(strings.TrimPrefix(t, "bytes"))
		if err != nil || n < 1 || n > 32 {
			return ""
		}
		return "0x" + hex.EncodeToString(word[:n])
	}
	return ""
}

// dynamicBytes reads a string or bytes value whose head word holds its offset. Long
// values are cut to maxDecodedBytes so one argument cannot bloat the response.
//
// Offsets and lengths are 256-bit words from untrusted data, so each is compared
// against the bytes actually left before it is converted or added to anything.
func dynamicBytes(data, head []byte) ([]byte, bool) {
	off := new(big.Int).SetBytes(head)
	if off.Cmp(big.NewInt(int64(len(data)))) >= 0 {
		return nil, false
	}
	o := int(off.Int64())
	if len(data)-o < 32 {
		return nil, false
	}
	ln := new(big.Int).SetBytes(data[o : o+32])
	if ln.Cmp(big.NewInt(int64(len(data)-o-32))) > 0 {
		return nil, false
	}
	n := int(ln.Int64())
	if n > maxDecodedBytes {
		n = maxDecodedBytes
	}
	return data[o+32 : o+32+n], true
}
