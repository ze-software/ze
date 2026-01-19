// Command zebgp-peer is a BGP test peer for validating BGP implementations.
//
// It can operate in several modes:
//   - sink: Accept any BGP messages, reply with keepalive
//   - echo: Accept any BGP messages, echo them back
//   - check: Validate received messages against expected patterns
//
// This is a Go port of ExaBGP's qa/sbin/bgp tool.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"codeberg.org/thomas-mangin/zebgp/pkg/testpeer"
)

func main() {
	config := parseFlags()

	if config == nil {
		return
	}

	switch config.Mode {
	case testpeer.ModeSink:
		fmt.Print("\nsink mode - send us whatever, we can take it! :p\n\n")
	case testpeer.ModeEcho:
		fmt.Print("\necho mode - send us whatever, we can parrot it! :p\n\n")
	case testpeer.ModeCheck:
		if len(config.Expect) == 0 {
			fmt.Println("no test data available to test against")
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	peer, err := testpeer.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create peer: %v\n", err)
		os.Exit(1)
	}
	result := peer.Run(ctx)
	cancel()

	fmt.Println()

	if result.Error != nil {
		fmt.Printf("failed: %v\n", result.Error)
		os.Exit(1)
	}

	if !result.Success {
		fmt.Println("failed")
		os.Exit(1)
	}

	fmt.Println("successful")
}

func parseFlags() *testpeer.Config {
	port := 179
	if p := os.Getenv("zebgp.tcp.port"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	if p := os.Getenv("zebgp_tcp_port"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	config := &testpeer.Config{Output: os.Stdout}

	var view bool
	var mode string
	flag.IntVar(&config.Port, "port", port, "port to bind to")
	flag.IntVar(&config.ASN, "asn", 0, "ASN to use (0 = extract from peer OPEN)")
	flag.StringVar(&mode, "mode", "check", "operation mode: check, sink, echo")
	flag.BoolVar(&config.IPv6, "ipv6", false, "bind using IPv6")
	flag.BoolVar(&config.Decode, "decode", false, "decode messages to human-readable format")
	flag.BoolVar(&view, "view", false, "show expected packets and exit")
	flag.Parse()

	// Parse mode (case-insensitive, warns on invalid)
	var valid bool
	config.Mode, valid = testpeer.ParseMode(mode)
	if !valid {
		fmt.Fprintf(os.Stderr, "warning: unknown mode %q, using %q\n", mode, config.Mode)
	}

	// Load check file if provided.
	if flag.NArg() > 0 {
		expect, fileConfig, err := testpeer.LoadExpectFile(flag.Arg(0))
		if err != nil {
			fmt.Printf("cannot load check file: %v\n", err)
			os.Exit(1)
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
	}

	if view {
		fmt.Println("\nrules:")
		for i, rule := range config.Expect {
			fmt.Printf("       %02d  %s\n", i+1, rule)
		}
		return nil
	}

	return config
}
