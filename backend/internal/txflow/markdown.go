package txflow

import (
	"fmt"
	"strings"
)

// Markdown renders the flow as a numbered list of plain-language steps, indented by call
// depth, for text surfaces such as a page's Markdown version or an MCP tool result.
func (f *Flow) Markdown() string {
	var b strings.Builder
	b.WriteString("## Flow\n\n")
	b.WriteString(mdEscape(f.Summary) + "\n\n")
	for _, s := range f.Steps {
		indent := strings.Repeat("  ", min(s.Depth, 8))
		fmt.Fprintf(&b, "%s%d. %s\n", indent, s.Index+1, mdEscape(s.Text))
	}
	if len(f.Notes) > 0 {
		b.WriteString("\n")
		for _, n := range f.Notes {
			b.WriteString("> " + mdEscape(n) + "\n")
		}
	}
	return b.String()
}

// mdEscape keeps contract-supplied text (names, decoded strings) from breaking the list.
func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := strings.NewReplacer("\\", "\\\\", "*", "\\*", "_", "\\_", "`", "\\`", "[", "\\[", "]", "\\]")
	return r.Replace(s)
}
