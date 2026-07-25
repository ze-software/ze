// Design: ai/rules/pipe-completeness.md -- data-transform pipes in custom render paths
// Related: model_traceroute.go -- | log legend enrichment for monitor traceroute
// Related: model_ping.go -- | log target legend enrichment for monitor ping

package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// enrichAddr applies the | resolve and | origin data-transform pipes to a
// single address for display paths that bypass ApplyPipes (the | log
// renderers). Origin wins over resolve when both flags are set and origin
// data is available. Returns the address unchanged when no lookup succeeds.
func enrichAddr(addr string, resolve, origin bool) string {
	if addr == "*" || addr == "" {
		return addr
	}
	var tb textbuf.Buffer
	if origin {
		o := command.LookupOrigin(addr)
		if o.ASN > 0 && o.Name != "" {
			return tb.Str(addr).Byte(' ').Str(o.Name).String()
		}
		if o.ASN > 0 {
			return tb.Str(addr).Str(" AS").Int(int64(o.ASN)).String()
		}
	}
	if resolve {
		if name := command.ReverseLookup(addr); name != "" {
			return tb.Str(addr).Byte(' ').Str(name).String()
		}
	}
	return addr
}
