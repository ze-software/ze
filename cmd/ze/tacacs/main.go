// Design: plan/learned/601-tacacs.md -- TACACS+ AAA operational CLI
//
// Package tacacs implements the `ze tacacs` offline subcommand.
//
// Purpose: surface per-server reachability for the TACACS+ servers named in
// a config file, without needing the daemon. This is the operator's "is the
// auth server even up" probe (AC-13).
package tacacs

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	tacacspkg "codeberg.org/thomas-mangin/ze/internal/component/tacacs"
)

// Run dispatches `ze tacacs <sub> [args]`. Returns an exit code.
func Run(args []string) int {
	if len(args) < 1 {
		usage()
		return 1
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "help", "-h", "--help": //nolint:goconst // consistent across cmd/
		usage()
		return 0
	case "show":
		return cmdShow(rest)
	default:
		fmt.Fprintf(os.Stderr, "error: unknown subcommand %q\n\n", sub)
		usage()
		return 1
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze tacacs",
		Summary: "Offline TACACS+ operational commands",
		Usage:   []string{"ze tacacs <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: []helpfmt.HelpEntry{
				{Name: "show <config>", Desc: "Probe each configured TACACS+ server and report reachability"},
			}},
		},
		Examples: []string{
			"ze tacacs show /etc/ze.conf",
			"ze tacacs show /etc/ze.conf --json",
		},
	}
	p.Write()
}

// probeResult is a single server's reachability probe outcome.
type probeResult struct {
	Address   string        `json:"address"`
	Port      uint16        `json:"port"`
	Reachable bool          `json:"reachable"`
	RTT       time.Duration `json:"rtt"`
	Error     string        `json:"error,omitempty"`
}

// Exit codes. Distinct so operator scripts can tell what went wrong.
const (
	exitOK         = 0 // at least one server reachable
	exitUsage      = 1 // missing/invalid arg, or no TACACS+ servers in config
	exitIOOrParse  = 2 // config file unreadable or YANG-invalid
	exitAllUnreach = 3 // every configured server failed the probe
)

// cmdShow parses a config file, extracts the TACACS+ server list, probes each
// server with a TCP connect bounded by the configured timeout, and prints a
// status table (or JSON).
func cmdShow(args []string) int {
	fs := flag.NewFlagSet("ze tacacs show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	jsonOut := fs.Bool("json", false, "JSON output")
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze tacacs show",
			Summary: "Probe configured TACACS+ servers and report reachability",
			Usage:   []string{"ze tacacs show <config-path> [--json]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Exit codes", Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: "At least one server reachable"},
					{Name: "1", Desc: "Usage error or no TACACS+ servers in config"},
					{Name: "2", Desc: "Cannot read or parse the config file"},
					{Name: "3", Desc: "All configured servers unreachable"},
				}},
			},
		}
		p.Write()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: config path required\n")
		fs.Usage()
		return exitUsage
	}
	configPath := fs.Arg(0)

	data, err := os.ReadFile(configPath) //nolint:gosec // operator-provided config path
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", configPath, err)
		return exitIOOrParse
	}

	tree, err := zeconfig.ParseTreeWithYANG(string(data), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: parse %s: %v\n", configPath, err)
		return exitIOOrParse
	}

	cfg := tacacspkg.ExtractConfig(tree)
	if !cfg.HasServers() {
		fmt.Fprintf(os.Stderr, "no TACACS+ servers configured in %s\n", configPath)
		return exitUsage
	}

	results := probeServers(cfg)

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(results); encErr != nil {
			fmt.Fprintf(os.Stderr, "error: encode json: %v\n", encErr)
			return exitIOOrParse
		}
		return showExitCode(results)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	// tabwriter buffers: Write errors here surface at Flush time, which is
	// checked. Individual Fprintln/Fprintf error returns are deliberately
	// ignored via fmt.Fprintln's no-op error path.
	if _, err := fmt.Fprintln(w, "ADDRESS\tPORT\tREACHABLE\tRTT\tERROR"); err != nil { //nolint:revive // unreachable, tabwriter buffers
		return exitIOOrParse
	}
	if _, err := fmt.Fprintln(w, "-------\t----\t---------\t---\t-----"); err != nil { //nolint:revive // unreachable
		return exitIOOrParse
	}
	for _, r := range results {
		reachable := "yes"
		if !r.Reachable {
			reachable = "no"
		}
		rtt := "-"
		if r.RTT > 0 {
			rtt = r.RTT.Round(time.Microsecond).String()
		}
		if _, err := fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", r.Address, r.Port, reachable, rtt, r.Error); err != nil { //nolint:revive // unreachable
			return exitIOOrParse
		}
	}
	if flushErr := w.Flush(); flushErr != nil {
		fmt.Fprintf(os.Stderr, "error: flush: %v\n", flushErr)
		return exitIOOrParse
	}

	return showExitCode(results)
}

// showExitCode returns 0 when at least one server is reachable, 3 otherwise.
// See the Exit codes block in cmdShow's usage for the full mapping.
func showExitCode(results []probeResult) int {
	for _, r := range results {
		if r.Reachable {
			return exitOK
		}
	}
	return exitAllUnreach
}

// probeServers probes each server serially with the configured per-server
// timeout. Serial (not parallel) because TACACS+ servers are typically a
// short ordered list and the probe is rarely latency-critical.
func probeServers(cfg tacacspkg.ExtractedConfig) []probeResult {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	results := make([]probeResult, 0, len(cfg.Servers))
	for _, srv := range cfg.Servers {
		_, portStr, splitErr := net.SplitHostPort(srv.Address)
		port := uint16(49)
		if splitErr == nil {
			if p, parseErr := strconv.ParseUint(portStr, 10, 16); parseErr == nil {
				port = uint16(p)
			}
		}

		r := probeResult{Address: srv.Address, Port: port}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		start := time.Now()
		var d net.Dialer
		conn, dialErr := d.DialContext(ctx, "tcp", srv.Address)
		r.RTT = time.Since(start)
		cancel()
		if dialErr != nil {
			r.Error = dialErr.Error()
		} else {
			r.Reachable = true
			if closeErr := conn.Close(); closeErr != nil {
				r.Error = "close: " + closeErr.Error()
			}
		}
		results = append(results, r)
	}
	return results
}
