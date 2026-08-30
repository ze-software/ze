// Design: docs/guide/command-catalogue.md -- ping output formatting
// Related: ping.go -- doPing internal ICMP engine shared across all ping paths

package cmd

import (
	"io"
	"strconv"

	"github.com/ze-software/ze/internal/core/textbuf"
)

func printPingResults(w io.Writer, results map[string]any) {
	var tb textbuf.Buffer
	tb.Reset(256)

	dest, _ := results["destination"].(string)
	sent, _ := results["sent"].(int)
	received, _ := results["received"].(int)
	loss, _ := results["loss-percent"].(float64)

	tb.Str("PING ").Str(dest).Str(": ").Int(int64(sent)).Str(" packets sent, ")
	tb.Int(int64(received)).Str(" received, ")
	tb.Str(strconv.FormatFloat(loss, 'f', 1, 64)).Str("% loss\n")

	if replies, ok := results["replies"].([]map[string]any); ok {
		for _, r := range replies {
			seq, _ := r["seq"].(int)
			status, _ := r[fieldStatus].(string)
			tb.Str("  seq=").Int(int64(seq))
			if status == "ok" {
				rtt, _ := r["rtt-ms"].(float64)
				tb.Str("  rtt=").Str(strconv.FormatFloat(rtt, 'f', 3, 64)).Str("ms\n")
			} else {
				tb.Str("  ").Str(status).Byte('\n')
			}
		}
	}

	if minRTT, ok := results["min-rtt-ms"].(float64); ok {
		avgRTT, _ := results["avg-rtt-ms"].(float64)
		maxRTT, _ := results["max-rtt-ms"].(float64)
		tb.Str("rtt min/avg/max = ")
		tb.Str(strconv.FormatFloat(minRTT, 'f', 3, 64)).Byte('/')
		tb.Str(strconv.FormatFloat(avgRTT, 'f', 3, 64)).Byte('/')
		tb.Str(strconv.FormatFloat(maxRTT, 'f', 3, 64)).Str(" ms\n")
	}

	io.WriteString(w, tb.Slice()) //nolint:errcheck // stdout
}
