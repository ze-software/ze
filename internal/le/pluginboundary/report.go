// Design: docs/architecture/core-design.md -- the process-boundary guard's answers
//
// report.go holds what the actions of `le plugin-boundary` ANSWER, apart from
// what produced them.
//
// Each answer IS its rows, so each payload is a slice rather than a struct
// wrapping one: `| json` renders the array the script's --json rendered, and
// `| count` says how many. Each slice also renders ITSELF (Text), because a
// violation list with the remedy under it is what a person reads here.

package pluginboundary

import "github.com/ze-software/ze/internal/core/textbuf"

// Finding is one unguarded same-process-effect call site, and it is one ROW of
// the check's answer. The keys are the script's, unchanged.
type Finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Code string `json:"code"`
}

// Findings is the whole answer of one check.
type Findings []Finding

// Text renders the findings for a person: the count, one line per site, and the
// remedy. A run that found nothing renders the verdict the script printed. It
// ends in a newline.
func (f Findings) Text() string {
	var tb textbuf.Buffer
	if len(f) == 0 {
		return tb.Str("plugin-process-boundary: OK\n").String()
	}

	tb.Str("plugin-process-boundary: ").Int(int64(len(f))).
		Str(" unguarded same-process-effect call site(s):\n")
	for _, finding := range f {
		tb.Str("  ").Str(finding.File).Byte(':').Int(int64(finding.Line)).Str(": ").Str(finding.Code).Byte('\n')
	}
	tb.Byte('\n')
	tb.Str("This calls a same-process-effect function directly (bypassing DirectBridge/\n")
	tb.Str("DispatchCommand), which silently no-ops or never fires when the plugin runs as\n")
	tb.Str("an external subprocess (see as112/cos/traffic-usage/flow-export/ddos-detect for\n")
	tb.Str("precedent). Add a p.IsInternal() check (refuse to start if the call is the\n")
	tb.Str("plugin's core purpose) or a warnIfExternal(p.IsInternal()) helper (if the plugin\n")
	tb.Str("still provides real value external) somewhere in this plugin's own package.\n")
	return tb.String()
}

// RootList is what `le plugin-boundary roots` answers: the scan roots, derived
// from the composition-root generator rather than declared a second time.
//
// It is a named slice rather than a bare []string so it can render itself, one
// root per line, which is what the script's --print-roots printed.
type RootList []string

// Text renders one root per line, ending in a newline.
func (r RootList) Text() string {
	var tb textbuf.Buffer
	for _, root := range r {
		tb.Str(root).Byte('\n')
	}
	return tb.String()
}
