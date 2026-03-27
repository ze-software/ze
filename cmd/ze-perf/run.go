// Design: (none -- new tool, predates documentation)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/perf"
)

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("ze-perf run", flag.ContinueOnError)

	// DUT flags.
	dutAddr := fs.String("dut-addr", "", "DUT BGP address (required)")
	dutPort := fs.Int("dut-port", 179, "DUT BGP port")
	dutASN := fs.Int("dut-asn", 0, "DUT autonomous system number (required)")
	dutName := fs.String("dut-name", "unknown", "DUT implementation name")
	dutVersion := fs.String("dut-version", "", "DUT version string")

	// Sender/receiver flags.
	senderAddr := fs.String("sender-addr", "127.0.0.1", "Sender local address")
	senderASN := fs.Int("sender-asn", 65001, "Sender autonomous system number")
	senderPort := fs.Int("sender-port", 0, "DUT port for sender (0 = use --dut-port)")
	receiverAddr := fs.String("receiver-addr", "127.0.0.2", "Receiver local address")
	receiverASN := fs.Int("receiver-asn", 65002, "Receiver autonomous system number")
	receiverPort := fs.Int("receiver-port", 0, "DUT port for receiver (0 = use --dut-port)")

	// Benchmark flags.
	routes := fs.Int("routes", 1000, "Number of routes to inject")
	family := fs.String("family", "ipv4/unicast", "Address family (ipv4/unicast or ipv6/unicast)")
	forceMP := fs.Bool("force-mp", false, "Force MP_REACH_NLRI for IPv4 unicast")
	seed := fs.Uint64("seed", 0, "Deterministic seed (0 = random)")
	warmup := fs.Duration("warmup", 2*time.Second, "Warmup delay after session establishment")
	connectTimeout := fs.Duration("connect-timeout", 10*time.Second, "TCP connection timeout")
	duration := fs.Duration("duration", 60*time.Second, "Maximum time to wait for convergence per iteration")

	// Iteration flags.
	repeat := fs.Int("repeat", 5, "Number of benchmark iterations")
	warmupRuns := fs.Int("warmup-runs", 1, "Warmup iterations (discarded)")
	iterDelay := fs.Duration("iter-delay", 3*time.Second, "Delay between iterations")
	batchSize := fs.Int("batch-size", 0, "NLRIs per UPDATE (0 = auto-max within 4096 bytes, 1 = one prefix per UPDATE)")
	passiveListen := fs.Bool("passive-listen", false, "Listen on port 179 for inbound DUT connections (requires root)")

	// Output flags.
	jsonOutput := fs.Bool("json", false, "JSON output")
	output := fs.String("output", "", "Output file path (implies --json)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: ze-perf run [flags]

Run a BGP propagation benchmark against a device under test (DUT).

Examples:
  ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000
  ze-perf run --dut-addr 172.31.0.5 --dut-asn 65000 --dut-name gobgp --routes 10000 --json
  ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --family ipv6/unicast
  ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --force-mp --repeat 10

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// --output implies --json.
	if *output != "" {
		*jsonOutput = true
	}

	// Validate required flags.
	if *dutAddr == "" {
		fmt.Fprintf(os.Stderr, "error: --dut-addr is required\n")
		return 1
	}

	if *dutASN == 0 {
		fmt.Fprintf(os.Stderr, "error: --dut-asn is required\n")
		return 1
	}

	// Validate family.
	if *family != "ipv4/unicast" && *family != "ipv6/unicast" {
		fmt.Fprintf(os.Stderr, "error: --family must be ipv4/unicast or ipv6/unicast, got %q\n", *family)
		return 1
	}

	// --force-mp only valid with ipv4/unicast.
	if *forceMP && *family != "ipv4/unicast" {
		fmt.Fprintf(os.Stderr, "error: --force-mp is only valid with ipv4/unicast\n")
		return 1
	}

	// Validate numeric ranges.
	if *routes < 1 {
		fmt.Fprintf(os.Stderr, "error: --routes must be >= 1, got %d\n", *routes)
		return 1
	}

	if *repeat < 1 {
		fmt.Fprintf(os.Stderr, "error: --repeat must be >= 1, got %d\n", *repeat)
		return 1
	}

	if *duration < time.Second {
		fmt.Fprintf(os.Stderr, "error: --duration must be >= 1s, got %s\n", *duration)
		return 1
	}

	// Construct summary for display.
	parts := []string{
		fmt.Sprintf("dut=%s:%d (AS %d)", *dutAddr, *dutPort, *dutASN),
		fmt.Sprintf("routes=%d", *routes),
		fmt.Sprintf("family=%s", *family),
		fmt.Sprintf("repeat=%d", *repeat),
	}

	if *forceMP {
		parts = append(parts, "force-mp")
	}

	fmt.Fprintf(os.Stderr, "ze-perf run | %s\n", strings.Join(parts, " | "))

	// Build benchmark config.
	cfg := perf.BenchmarkConfig{
		DUTAddr:        *dutAddr,
		DUTPort:        *dutPort,
		DUTASN:         *dutASN,
		DUTName:        *dutName,
		DUTVersion:     *dutVersion,
		SenderAddr:     *senderAddr,
		SenderASN:      *senderASN,
		SenderPort:     *senderPort,
		ReceiverAddr:   *receiverAddr,
		ReceiverASN:    *receiverASN,
		ReceiverPort:   *receiverPort,
		Routes:         *routes,
		Family:         *family,
		ForceMP:        *forceMP,
		Seed:           *seed,
		Warmup:         *warmup,
		ConnectTimeout: *connectTimeout,
		Duration:       *duration,
		Repeat:         *repeat,
		WarmupRuns:     *warmupRuns,
		IterDelay:      *iterDelay,
		BatchSize:      *batchSize,
		PassiveListen:  *passiveListen,
	}

	// Set up context with signal cancellation.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	result, err := perf.RunBenchmark(ctx, cfg, os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	// Output results.
	if *jsonOutput {
		return writeJSONResult(&result, *output)
	}

	printHumanResult(&result)

	if result.RoutesLost > 0 {
		return 1
	}

	return 0
}

// writeJSONResult marshals the result to JSON and writes to stdout or a file.
func writeJSONResult(result *perf.Result, outputPath string) int {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshaling result: %v\n", err)
		return 1
	}

	data = append(data, '\n')

	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0o644); err != nil { //nolint:gosec // CLI tool writes user-specified output file
			fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", outputPath, err)
			return 1
		}

		fmt.Fprintf(os.Stderr, "results written to %s\n", outputPath)

		return 0
	}

	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing to stdout: %v\n", err)
		return 1
	}

	return 0
}

// printHumanResult prints a human-readable summary to stdout.
func printHumanResult(r *perf.Result) {
	fmt.Printf("DUT: %s", r.DUTName)

	if r.DUTVersion != "" {
		fmt.Printf(" %s", r.DUTVersion)
	}

	fmt.Printf(" @ %s:%d (AS %d)\n", r.DUTAddr, r.DUTPort, r.DUTASN)
	fmt.Printf("Routes: %d %s (seed %d)\n", r.Routes, r.Family, r.Seed)
	fmt.Printf("Iterations: %d measured, %d warmup, %d kept\n", r.Repeat, r.WarmupRuns, r.RepeatKept)

	fmt.Println()
	fmt.Println("Sessions:")
	fmt.Printf("  sender   -> established in %dms\n", r.SessionSetupMs.Sender)
	fmt.Printf("  receiver -> established in %dms\n", r.SessionSetupMs.Receiver)

	fmt.Println()
	fmt.Println("Propagation:")
	fmt.Printf("  first route:   %dms\n", r.FirstRouteMs)

	if r.ConvergenceStddevMs > 0 {
		fmt.Printf("  convergence:   %dms +/- %dms\n", r.ConvergenceMs, r.ConvergenceStddevMs)
	} else {
		fmt.Printf("  convergence:   %dms\n", r.ConvergenceMs)
	}

	if r.ThroughputAvgStddev > 0 {
		fmt.Printf("  throughput:    %d routes/sec +/- %d\n", r.ThroughputAvg, r.ThroughputAvgStddev)
	} else {
		fmt.Printf("  throughput:    %d routes/sec\n", r.ThroughputAvg)
	}

	fmt.Printf("  routes:        %d/%d (%d lost)\n", r.RoutesReceived, r.RoutesSent, r.RoutesLost)

	fmt.Println()
	fmt.Println("Latency distribution:")
	fmt.Printf("  p50:   %dms\n", r.LatencyP50Ms)
	fmt.Printf("  p90:   %dms\n", r.LatencyP90Ms)

	if r.LatencyP99StddevMs > 0 {
		fmt.Printf("  p99:   %dms +/- %dms\n", r.LatencyP99Ms, r.LatencyP99StddevMs)
	} else {
		fmt.Printf("  p99:   %dms\n", r.LatencyP99Ms)
	}

	fmt.Printf("  max:   %dms\n", r.LatencyMaxMs)
}
