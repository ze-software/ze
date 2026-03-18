// Design: docs/architecture/testing/ci-format.md — test runner CLI

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/test/peer"
)

// Register ze.bgp.tcp.port so env.GetInt doesn't abort.
// The full ze.bgp.* prefix is registered in internal/component/config,
// but ze-test doesn't import that package.
var _ = env.MustRegister(env.EnvEntry{
	Key:         "ze.bgp.tcp.port",
	Type:        "int",
	Default:     "179",
	Description: "BGP TCP port (used by ze-test peer)",
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

	port := env.GetInt("ze.bgp.tcp.port", 179)

	config := &peer.Config{Output: os.Stdout}

	var view bool
	var mode string

	fs := flag.NewFlagSet("peer", flag.ExitOnError)
	fs.IntVar(&config.Port, "port", port, "port to bind to")
	fs.IntVar(&config.ASN, "asn", 0, "ASN to use (0 = extract from peer OPEN)")
	fs.StringVar(&mode, "mode", "check", "operation mode: check, sink, echo")
	fs.BoolVar(&config.IPv6, "ipv6", false, "bind using IPv6")
	fs.BoolVar(&config.Decode, "decode", false, "decode messages to human-readable format")
	fs.BoolVar(&view, "view", false, "show expected packets and exit")

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

func printPeerUsage() {
	fmt.Fprintf(os.Stderr, `Usage: ze-test peer [options] [expect-file]

BGP test peer for validating BGP implementations.

Modes:
  --mode check    Validate messages against expect-file (default)
  --mode sink     Accept any messages, reply keepalive
  --mode echo     Accept any messages, echo them back

Options:
  --port N        Port to bind (default: 179, or ze_bgp_tcp_port env)
  --asn N         ASN to use (0 = extract from peer OPEN)
  --ipv6          Bind using IPv6
  --decode        Decode messages to human-readable format
  --view          Show expected packets and exit

Examples:
  ze-test peer --mode sink --port 1790
  ze-test peer --mode echo --port 1790
  ze-test peer --port 1790 test/encode/basic.msg
  ze-test peer --view test/encode/basic.msg
`)
}
