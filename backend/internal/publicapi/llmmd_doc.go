package publicapi

// Markdown building blocks for the `/llm.md` pages: a small document builder,
// escaping for untrusted text, and the number/time formatting every page uses.
// Kept free of data fetching so the MCP tools can reuse the same output.

import (
	"math/big"
	"strconv"
	"strings"
	"time"
)

// mdDoc accumulates one Markdown page.
type mdDoc struct {
	b    strings.Builder
	base string // absolute origin ("https://host") or "" for root-relative links
}

func newMDDoc(base string) *mdDoc { return &mdDoc{base: base} }

func (d *mdDoc) String() string { return strings.TrimRight(d.b.String(), "\n") + "\n" }

func (d *mdDoc) line(s string) {
	d.b.WriteString(s)
	d.b.WriteByte('\n')
}

func (d *mdDoc) h1(s string)   { d.line("# " + s); d.line("") }
func (d *mdDoc) h2(s string)   { d.line("## " + s); d.line("") }
func (d *mdDoc) para(s string) { d.line(s); d.line("") }

// mdField is one "label: value" row. Rows with an empty value are skipped, so
// callers can pass optional facts without branching.
type mdField struct{ k, v string }

func (d *mdDoc) fields(rows ...mdField) {
	wrote := false
	for _, r := range rows {
		if r.v == "" {
			continue
		}
		d.line("- **" + r.k + ":** " + r.v)
		wrote = true
	}
	if wrote {
		d.line("")
	}
}

// table writes a pipe table. Cells must already be escaped (mdText) or be
// links built by the doc; a bare "|" would split the cell.
func (d *mdDoc) table(head []string, rows [][]string) {
	d.line("| " + strings.Join(head, " | ") + " |")
	sep := make([]string, len(head))
	for i := range sep {
		sep[i] = "---"
	}
	d.line("| " + strings.Join(sep, " | ") + " |")
	for _, r := range rows {
		d.line("| " + strings.Join(r, " | ") + " |")
	}
	d.line("")
}

// code writes a fenced block whose fence is longer than any backtick run in
// body, so untrusted source code cannot close the block early.
func (d *mdDoc) code(lang, body string) {
	fence := "```"
	for strings.Contains(body, fence) {
		fence += "`"
	}
	d.line(fence + lang)
	d.line(strings.TrimRight(body, "\n"))
	d.line(fence)
	d.line("")
}

// url is the Markdown URL of an explorer page. path is the HTML page path
// ("/tx/0x..."); "" or "/" is the home page.
func (d *mdDoc) url(path string) string {
	path = strings.TrimSuffix(path, "/")
	return d.base + path + "/llm.md"
}

// urlQ is url plus a query string (already encoded by the caller).
func (d *mdDoc) urlQ(path, query string) string {
	if query == "" {
		return d.url(path)
	}
	return d.url(path) + "?" + query
}

func (d *mdDoc) link(text, path string) string { return "[" + text + "](" + d.url(path) + ")" }

func (d *mdDoc) addr(a string) string {
	if a == "" {
		return ""
	}
	return d.link(a, "/address/"+a)
}

func (d *mdDoc) tx(h string) string {
	if h == "" {
		return ""
	}
	return d.link(h, "/tx/"+h)
}

func (d *mdDoc) block(n uint64) string {
	s := strconv.FormatUint(n, 10)
	return d.link(s, "/block/"+s)
}

// mdEscaper neutralises Markdown syntax in text that comes from the chain or
// from users (token names, revert reasons, contract names). Newlines become
// spaces so a value cannot start a new block.
var mdEscaper = strings.NewReplacer(
	`\`, `\\`, "`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`,
	"<", `\<`, ">", `\>`, "|", `\|`, "#", `\#`, "!", `\!`,
	"\r\n", " ", "\r", " ", "\n", " ",
)

func mdText(s string) string { return mdEscaper.Replace(strings.TrimSpace(s)) }

// formatUnits renders an integer amount (decimal, or 0x-hex) with the given
// number of decimals, trimming trailing zeros: ("1500000000000000000", 18) ->
// "1.5". An unparseable value is returned escaped, unchanged.
func formatUnits(raw string, decimals int) string {
	raw = strings.TrimSpace(raw)
	v, ok := new(big.Int).SetString(raw, 10)
	if !ok && (strings.HasPrefix(raw, "0x") || strings.HasPrefix(raw, "0X")) {
		v, ok = new(big.Int).SetString(raw[2:], 16)
	}
	if !ok {
		return mdText(raw)
	}
	sign := ""
	if v.Sign() < 0 {
		sign = "-"
		v.Abs(v)
	}
	s := v.String()
	if decimals <= 0 {
		return sign + s
	}
	if len(s) <= decimals {
		s = strings.Repeat("0", decimals-len(s)+1) + s
	}
	intPart, frac := s[:len(s)-decimals], strings.TrimRight(s[len(s)-decimals:], "0")
	if frac == "" {
		return sign + intPart
	}
	return sign + intPart + "." + frac
}

// mdTime renders a unix timestamp as UTC; 0 means unknown and renders "".
func mdTime(ts uint64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(int64(ts), 0).UTC().Format("2006-01-02 15:04:05 UTC")
}

func u64(v uint64) string { return strconv.FormatUint(v, 10) }

func optU64(p *uint64) string {
	if p == nil {
		return ""
	}
	return u64(*p)
}

func optStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// pct renders part/whole as a percentage with one decimal; "" when whole is 0.
func pct(part, whole uint64) string {
	if whole == 0 {
		return ""
	}
	return strconv.FormatFloat(float64(part)*100/float64(whole), 'f', 1, 64) + "%"
}
