// Design: docs/architecture/testing/ci-format.md — test runner CLI

package main

import (
	"context"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/test/peer"
)

// ze.test.bgp.port is a test-only env var: ze-test peer listens on it, and the
// ze-test harness sets it when launching ze as a BGP client (see cmd/ze-test/bgp.go).
// Private so it does not show up in `ze env list`.
var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.test.bgp.port",
	Type:        "int",
	Default:     "179",
	Description: "BGP TCP port used by ze-test peer and the ze-test runner (test infrastructure)",
	Private:     true,
})

func peerCmd() int {
	config, ok := parsePeerFlags()
	if config == nil {
		if ok {
			return 0 // Help or view was shown
		}
		return 1 // Error occurred
	}

	switch config.Mode {
	case peer.ModeSink:
		fmt.Print("\nsink mode - send us whatever, we can take it! :p\n\n")
	case peer.ModeEcho:
		fmt.Print("\necho mode - send us whatever, we can parrot it! :p\n\n")
	case peer.ModeCheck:
		if len(config.Expect) == 0 {
			fmt.Fprintln(os.Stderr, "no test data available to test against")
			return 1
		}
	case peer.ModeInject:
		fmt.Printf("\ninject mode - %d prefixes from %s via %s (AS %d)\n\n",
			config.Inject.Count, config.Inject.Prefix, config.Inject.NextHop, config.Inject.ASN)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
	}()

	p, err := peer.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create peer: %v\n", err)
		return 1
	}
	result := p.Run(ctx)
	cancel()

	fmt.Println()

	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "failed: %v\n", result.Error)
		return 1
	}

	if !result.Success {
		fmt.Fprintln(os.Stderr, "failed")
		return 1
	}

	fmt.Println("successful")
	return 0
}

// parsePeerFlags parses command-line flags and returns the config.
// Returns (nil, true) if help/view was shown (success, no action needed).
// Returns (nil, false) if an error occurred.
// Returns (config, true) if config is ready to use.
func parsePeerFlags() (*peer.Config, bool) {
	// Check for help first (os.Args shifted by main: ["peer", ...])
	if len(os.Args) > 1 && isHelpArg(os.Args[1]) {
		printPeerUsage()
		return nil, true
	}

	port := env.GetInt("ze.test.bgp.port", 179)

	config := &peer.Config{Output: os.Stdout}

	var view bool
	var mode string
	var injectPrefix, injectNextHop string
	var injectCount int
	var injectASN uint
	var injectDwell time.Duration

	fs := flag.NewFlagSet("peer", flag.ExitOnError)
	fs.IntVar(&config.Port, "port", port, "port to bind to")
	fs.StringVar(&config.BindAddr, "bind", "", "bind address (default 127.0.0.1, or ::1 with -ipv6)")
	fs.StringVar(&config.Dial, "dial", "", "dial host:port instead of listening (active BGP role; requires --mode inject)")
	fs.IntVar(&config.ASN, "asn", 0, "ASN to use (0 = extract from peer OPEN)")
	fs.StringVar(&mode, "mode", "check", "operation mode: check, sink, echo, inject")
	fs.BoolVar(&config.IPv6, "ipv6", false, "bind using IPv6")
	fs.BoolVar(&config.Decode, "decode", false, "decode messages to human-readable format")
	fs.BoolVar(&view, "view", false, "show expected packets and exit")
	fs.StringVar(&injectPrefix, "inject-prefix", "", "inject: base prefix (e.g. 10.0.0.0/24 or 2001:db8::/48)")
	fs.IntVar(&injectCount, "inject-count", 0, "inject: number of sequential prefixes to emit")
	fs.StringVar(&injectNextHop, "inject-nexthop", "", "inject: next-hop address (family must match --inject-prefix)")
	fs.UintVar(&injectASN, "inject-asn", 0, "inject: origin ASN for single-segment AS_SEQUENCE")
	fs.DurationVar(&injectDwell, "inject-dwell", 0, "inject: hold session this long after last byte (0 = until SIGTERM)")

	fs.Usage = printPeerUsage

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, false
	}

	// Parse mode (case-insensitive, warns on invalid)
	var valid bool
	config.Mode, valid = peer.ParseMode(mode)
	if !valid {
		fmt.Fprintf(os.Stderr, "warning: unknown mode %q, using %q\n", mode, config.Mode)
	}

	// Inject mode: parse + validate the spec.
	if config.Mode == peer.ModeInject {
		spec, err := buildInjectSpec(injectPrefix, injectCount, injectNextHop, injectASN, injectDwell)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return nil, false
		}
		config.Inject = spec
	}

	// Load check file if provided.
	if fs.NArg() > 0 {
		expect, fileConfig, err := peer.LoadExpectFile(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot load check file: %v\n", err)
			return nil, false
		}
		config.Expect = expect
		// Merge file config options.
		if fileConfig.IPv6 {
			config.IPv6 = true
		}
		if fileConfig.SendUnknownCapability {
			config.SendUnknownCapability = true
		}
		if fileConfig.SendDefaultRoute {
			config.SendDefaultRoute = true
		}
		if len(fileConfig.SendRoutes) > 0 {
			config.SendRoutes = append(config.SendRoutes, fileConfig.SendRoutes...)
		}
		if fileConfig.InspectOpenMessage {
			config.InspectOpenMessage = true
		}
		if fileConfig.SendUnknownMessage {
			config.SendUnknownMessage = true
		}
		if fileConfig.ASN != 0 {
			config.ASN = fileConfig.ASN
		}
		if fileConfig.TCPConnections > 0 {
			config.TCPConnections = fileConfig.TCPConnections
		}
		if len(fileConfig.CapabilityOverrides) > 0 {
			config.CapabilityOverrides = fileConfig.CapabilityOverrides
		}
		if fileConfig.BindAddr != "" && config.BindAddr == "" {
			config.BindAddr = fileConfig.BindAddr
		}
		if fileConfig.ConnMap != "" {
			config.ConnMap = fileConfig.ConnMap
		}
	}

	if view {
		fmt.Println("\nrules:")
		for i, rule := range config.Expect {
			fmt.Printf("       %02d  %s\n", i+1, rule)
		}
		return nil, true // View shown successfully
	}

	return config, true
}

// buildInjectSpec validates and assembles an InjectSpec from the CLI flags.
// All inject-* flags are required when --mode inject is set.
func buildInjectSpec(prefixStr string, count int, nextHopStr string, asn uint, dwell time.Duration) (*peer.InjectSpec, error) {
	if prefixStr == "" || nextHopStr == "" || count <= 0 || asn == 0 {
		return nil, fmt.Errorf("--mode inject requires --inject-prefix, --inject-count (>0), --inject-nexthop, --inject-asn (>0)")
	}
	prefix, err := netip.ParsePrefix(prefixStr)
	if err != nil {
		return nil, fmt.Errorf("invalid --inject-prefix %q: %w", prefixStr, err)
	}
	nh, err := netip.ParseAddr(nextHopStr)
	if err != nil {
		return nil, fmt.Errorf("invalid --inject-nexthop %q: %w", nextHopStr, err)
	}
	if asn > (1<<32)-1 {
		return nil, fmt.Errorf("--inject-asn %d exceeds 32 bits", asn)
	}
	return &peer.InjectSpec{
		Prefix:   prefix,
		Count:    count,
		NextHop:  nh,
		ASN:      uint32(asn), //nolint:gosec // bounds checked above
		EndOfRIB: true,
		Dwell:    dwell,
	}, nil
}

func printPeerUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze-test peer [options] [expect-file]

BGP test peer for validating BGP implementations.

Modes:
  --mode check    Validate messages against expect-file (default)
  --mode sink     Accept any messages, reply keepalive
  --mode echo     Accept any messages, echo them back
  --mode inject   Stream a bulk UPDATE image after OPEN (stress tests)

Options:
  --port N           Port to bind (default: 179, or ze_test_bgp_port env)
  --asn N            ASN to use (0 = extract from peer OPEN)
  --ipv6             Bind using IPv6
  --decode           Decode messages to human-readable format
  --view             Show expected packets and exit

Inject options (all required when --mode inject):
  --inject-prefix P  Base prefix, e.g. 10.0.0.0/24 or 2001:db8::/48
  --inject-count N   Number of sequential prefixes to emit
  --inject-nexthop A Next-hop address (family must match --inject-prefix)
  --inject-asn N     Origin ASN (single-segment AS_SEQUENCE, 4-byte)
  --inject-dwell D   Hold the session open this long after the last byte
                     is written (default: until SIGTERM).
  --dial H:P         Act as the active BGP role: dial H:P instead of
                     listening. Sends OPEN first with minimal capabilities
                     (one MP-BGP family inferred from --inject-prefix,
                     plus 4-byte ASN). Combined with --mode inject only.

Examples:
  ze-test peer --mode sink --port 1790
  ze-test peer --mode echo --port 1790
  ze-test peer --port 1790 test/encode/basic.msg
  ze-test peer --view test/encode/basic.msg
  ze-test peer --mode inject --port 1790 \
      --inject-prefix 10.0.0.0/24 --inject-count 1000000 \
      --inject-nexthop 172.31.0.3 --inject-asn 65100
  ze-test peer --mode inject --dial 172.31.0.2:179 \
      --inject-prefix 10.0.0.0/24 --inject-count 1000000 \
      --inject-nexthop 172.31.0.3 --inject-asn 65100 --inject-dwell 60s
`)
}
