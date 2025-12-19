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

	"github.com/exa-networks/zebgp/pkg/testpeer"
)

func main() {
	config := parseFlags()

	if config == nil {
		return
	}

	switch {
	case config.Sink:
		fmt.Print("\nsink mode - send us whatever, we can take it! :p\n\n")
	case config.Echo:
		fmt.Print("\necho mode - send us whatever, we can parrot it! :p\n\n")
	case len(config.Expect) == 0:
		fmt.Println("no test data available to test against")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	peer := testpeer.New(config)
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
	if p := os.Getenv("exabgp.tcp.port"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	if p := os.Getenv("exabgp_tcp_port"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	config := &testpeer.Config{Output: os.Stdout}

	var view bool
	flag.IntVar(&config.Port, "port", port, "port to bind to")
	flag.IntVar(&config.ASN, "asn", 0, "ASN to use (0 = extract from peer OPEN)")
	flag.BoolVar(&config.Sink, "sink", false, "accept any BGP messages, reply with keepalive")
	flag.BoolVar(&config.Echo, "echo", false, "accept any BGP messages, echo them back")
	flag.BoolVar(&config.IPv6, "ipv6", false, "bind using IPv6")
	flag.BoolVar(&view, "view", false, "show expected packets and exit")
	flag.Parse()

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
