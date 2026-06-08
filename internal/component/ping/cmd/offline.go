// Design: docs/guide/command-catalogue.md -- offline ping diagnostic
// Related: ping.go -- doPing internal ICMP engine shared across all ping paths

package cmd

import (
	"errors"
	"flag"
	"io"
	"net/netip"
	"os"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/core/probe"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const (
	maxOSPingCount = 100000
)

// RunPing implements `ze ping <target>`: a bounded batch of ICMP echo
// requests using Ze's internal ICMP engine (no external ping binary).
func RunPing(args []string) int {
	const name = "ping"
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		var tb textbuf.Buffer
		tb.Str("Usage: ze ").Str(name).Str(" <target> [--count N] [--source IP]\n\n")
		tb.Str("Send ICMP echo-request to <target> using the internal ICMP engine.\n\n")
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
		fs.PrintDefaults()
	}
	var (
		count  int
		source string
	)
	fs.IntVar(&count, "count", defaultPingCount, "number of echo requests (1..100000)")
	fs.IntVar(&count, "c", defaultPingCount, "short form of --count")
	fs.StringVar(&source, "source", "", "source IP address to bind")
	fs.StringVar(&source, "s", "", "short form of --source")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	rest := fs.Args()
	if len(rest) == 0 {
		var tb textbuf.Buffer
		tb.Str(name).Str(": target is required\n")
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
		fs.Usage()
		return 1
	}
	if len(rest) > 1 {
		var tb textbuf.Buffer
		tb.Str(name).Str(": multiple targets not allowed\n")
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
		return 1
	}
	target := rest[0]
	if count < 1 || count > maxOSPingCount {
		var tb textbuf.Buffer
		tb.Str(name).Str(": --count must be in 1..").Int(int64(maxOSPingCount)).Byte('\n')
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
		return 1
	}

	if err := validateResolveTarget(target); err != nil {
		var tb textbuf.Buffer
		tb.Str(name).Str(": ").Err(err).Byte('\n')
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
		return 1
	}

	dest, err := probe.ResolveTarget(target)
	if err != nil {
		var tb textbuf.Buffer
		tb.Str(name).Str(": ").Err(err).Byte('\n')
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
		return 1
	}

	var opts pingOpts
	if source != "" {
		addr, parseErr := netip.ParseAddr(source)
		if parseErr != nil {
			var tb textbuf.Buffer
			tb.Str(name).Str(": invalid source IP ").Str(strconv.Quote(source)).Byte('\n')
			os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
			return 1
		}
		opts.source = addr
	}

	results, pingErr := doPing(dest, count, defaultPingTimeout, opts)
	if pingErr != nil {
		var tb textbuf.Buffer
		tb.Str(name).Str(": ").Err(pingErr).Byte('\n')
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // stderr
		return 1
	}

	printPingResults(os.Stdout, results)
	return 0
}

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
			status, _ := r["status"].(string)
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

	io.WriteString(w, tb.String()) //nolint:errcheck // stdout
}
