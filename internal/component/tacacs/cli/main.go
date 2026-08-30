// Design: docs/architecture/aaa-tacacs.md -- TACACS+ AAA operational CLI
//
// Package tacacs implements the `ze tacacs` offline subcommand.
//
// Purpose: surface per-server reachability for the TACACS+ servers named in
// a config file, without needing the daemon. This is the operator's "is the
// auth server even up" probe (AC-13).
//
// The probe rows are rendered in the configured default format, through the
// renderer every locally answered command uses. The `--json` flag it carried
// before was a second, hand-written rendering of the same rows
// (ai/rules/cli.md).
package cli

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	zeconfig "github.com/ze-software/ze/internal/component/config"
	tacacspkg "github.com/ze-software/ze/internal/component/tacacs"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

// keyServers is the envelope key the probe rows travel under, so a reader sees
// one shape whichever way the probe was spelled.
const keyServers = "servers"

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
		fmt.Fprintln(os.Stderr, "error: unknown subcommand "+strconv.Quote(sub)+"\n")
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
		},
	}
	p.WriteErr()
}

// probeResult is a single server's reachability probe outcome, and one row of
// the answer.
//
// RTT is the probe duration as a string, rounded to the microsecond: a person
// reads `1.234ms` in the table, and a time.Duration reaches a table renderer as
// a bare nanosecond count.
type probeResult struct {
	Address   string `json:"address"`
	Port      uint16 `json:"port"`
	Reachable bool   `json:"reachable"`
	RTT       string `json:"rtt"`
	Error     string `json:"error,omitempty"`
}

// Exit codes. Distinct so operator scripts can tell what went wrong.
const (
	exitOK         = 0 // at least one server reachable
	exitUsage      = 1 // missing/invalid arg, or no TACACS+ servers in config
	exitIOOrParse  = 2 // config file unreadable or YANG-invalid
	exitAllUnreach = 3 // every configured server failed the probe
)

// cmdShow renders the probe for the `ze tacacs show <config>` spelling and
// answers the reachability verdict as its exit code.
//
// The rendering goes through command.RenderLocalAnswer, so the probe prints in
// the configured default format like every other locally answered command.
func cmdShow(args []string) int {
	fs := flag.NewFlagSet("ze tacacs show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze tacacs show",
			Summary: "Probe configured TACACS+ servers and report reachability",
			Usage:   []string{"ze tacacs show <config-path>"},
			Sections: []helpfmt.HelpSection{
				{Title: "Exit codes", Entries: []helpfmt.HelpEntry{
					{Name: "0", Desc: "At least one server reachable"},
					{Name: "1", Desc: "Usage error or no TACACS+ servers in config"},
					{Name: "2", Desc: "Cannot read or parse the config file"},
					{Name: "3", Desc: "All configured servers unreachable"},
				}},
			},
			Examples: []string{
				"ze tacacs show /etc/ze.conf",
			},
		}
		p.WriteErr()
	}
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	results, code := probeConfig(fs.Args())
	if code != exitOK {
		return code
	}
	// The command path is what the renderer reads a declared column order and
	// the default format from. This command declares no column order, so the
	// rows come out in field order.
	if renderCode := command.RenderLocalAnswer("tacacs show", serversAnswer(results)); renderCode != 0 {
		fmt.Fprintln(os.Stderr, "error: the probe could not be rendered")
		return exitIOOrParse
	}
	return showExitCode(results)
}

// serversAnswer wraps the probe rows in the envelope the renderer reads.
func serversAnswer(results []probeResult) map[string]any {
	return map[string]any{keyServers: results}
}

// probeConfig reads the config path the operator named, extracts its TACACS+
// servers, and probes each one. The int is an exit code, and exitOK means the
// rows are the answer.
func probeConfig(args []string) ([]probeResult, int) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "error: config path required")
		return nil, exitUsage
	}
	configPath := args[0]

	data, err := cliio.ReadFile(configPath) // "-" reads stdin
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: read "+configPath+":", err)
		return nil, exitIOOrParse
	}

	tree, err := zeconfig.ParseTreeWithYANG(string(data), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: parse "+configPath+":", err)
		return nil, exitIOOrParse
	}

	cfg := tacacspkg.ExtractConfig(tree)
	if !cfg.HasServers() {
		fmt.Fprintln(os.Stderr, "no TACACS+ servers configured in "+configPath)
		return nil, exitUsage
	}

	return probeServers(cfg), exitOK
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
		r.RTT = time.Since(start).Round(time.Microsecond).String()
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
