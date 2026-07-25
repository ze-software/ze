// Design: docs/guide/command-catalogue.md -- traceroute output formatting
// Related: traceroute.go -- doTraceroute internal ICMP engine shared across all traceroute paths

package cmd

import (
	"io"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func printTracerouteResults(w io.Writer, target string, hops []map[string]any) {
	var tb textbuf.Buffer
	tb.Reset(256)

	tb.Str("traceroute to ").Str(target).Str(", ").Int(int64(len(hops))).Str(" hops\n")

	for _, hop := range hops {
		ttl, _ := hop["ttl"].(int)
		addr, _ := hop["addr"].(string)
		tb.Str("  ").Int(int64(ttl)).Str("  ").Str(addr)
		if rtt, ok := hop["rtt-ms"].(float64); ok {
			tb.Str("  ").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str(" ms")
		}
		tb.Byte('\n')
	}

	io.WriteString(w, tb.Slice()) //nolint:errcheck // stdout
}
