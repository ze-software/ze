// Design: docs/architecture/system-architecture.md -- ze-perf run subcommand

package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/perf"
	"github.com/ze-software/ze/internal/perf/report"
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
	fam := fs.String("family", "ipv4/unicast", "Address family (ipv4/unicast or ipv6/unicast)")
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
	senderFirst := fs.Bool("sender-first", false, "Connect sender before receiver (measures RIB propagation instead of live forwarding)")

	// Output flags.
	jsonOutput := fs.Bool("json", false, "JSON output")
	output := fs.String("output", "", "Output file path (implies --json)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: ze-perf run [flags]\n\nRun a BGP propagation benchmark against a device under test (DUT).\n\nExamples:\n  ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000\n  ze-perf run --dut-addr 172.31.0.5 --dut-asn 65000 --dut-name gobgp --routes 10000 --json\n  ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --family ipv6/unicast\n  ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --force-mp --repeat 10\n\nFlags:\n")
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
	if *fam != "ipv4/unicast" && *fam != "ipv6/unicast" {
		fmt.Fprintf(os.Stderr, "error: --family must be ipv4/unicast or ipv6/unicast, got %q\n", *fam)
		return 1
	}

	// --force-mp only valid with ipv4/unicast.
	if *forceMP && *fam != "ipv4/unicast" {
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
		fmt.Sprintf("family=%s", *fam),
		fmt.Sprintf("repeat=%d", *repeat),
	}

	if *forceMP {
		parts = append(parts, "force-mp")
	}

	fmt.Fprintf(os.Stderr, "ze-perf run | %s\n", textbuf.Join(parts, " | "))

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
		Family:         *fam,
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
		SenderFirst:    *senderFirst,
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

	report.PrintHuman(os.Stdout, &result)

	if result.RoutesLost > 0 {
		return 1
	}

	return 0
}

func writeJSONResult(result *perf.Result, outputPath string) int {
	if outputPath != "" {
		f, err := cliio.Create(outputPath) // "-" writes stdout
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: creating %s: %v\n", outputPath, err)
			return 1
		}
		if err := report.WriteJSON(f, result); err != nil {
			f.Close() //nolint:errcheck,gosec // already reporting error
			fmt.Fprintf(os.Stderr, "error: writing %s: %v\n", outputPath, err)
			return 1
		}
		if err := f.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error: closing %s: %v\n", outputPath, err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "results written to %s\n", outputPath)
		return 0
	}

	if err := report.WriteJSON(os.Stdout, result); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing to stdout: %v\n", err)
		return 1
	}
	return 0
}
