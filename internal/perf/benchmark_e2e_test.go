package perf_test

import (
	"bytes"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/perf"
	"codeberg.org/thomas-mangin/ze/internal/perf/report"
)

// VALIDATES: "Full pipeline: RunBenchmark -> Result -> Markdown/HTML/NDJSON round-trip."
// PREVENTS: "RunBenchmark result not usable by report generators or NDJSON serialization."
//
// TestBenchmarkEndToEnd exercises the entire benchmark pipeline: starts a test
// forwarder, runs RunBenchmark, verifies the Result, generates Markdown and HTML
// reports, and round-trips the Result through NDJSON serialization.
func TestBenchmarkEndToEnd(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Start the test forwarder.
	fwd := perf.NewTestForwarder(t)

	go func() {
		fwd.Run(ctx)
	}()

	// Extract host and port from the forwarder address.
	host, portStr, err := net.SplitHostPort(fwd.Addr())
	if err != nil {
		t.Fatalf("splitting forwarder address %q: %v", fwd.Addr(), err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parsing port %q: %v", portStr, err)
	}

	// 2. Run the benchmark.
	cfg := perf.BenchmarkConfig{
		DUTAddr:        host,
		DUTPort:        port,
		DUTASN:         65000,
		DUTName:        "test-forwarder",
		SenderAddr:     "127.0.0.1",
		SenderASN:      65001,
		ReceiverAddr:   "127.0.0.1",
		ReceiverASN:    65002,
		Routes:         10,
		Family:         "ipv4/unicast",
		Seed:           42,
		Warmup:         100 * time.Millisecond,
		ConnectTimeout: 5 * time.Second,
		Duration:       10 * time.Second,
		Repeat:         3,
		WarmupRuns:     1,
		IterDelay:      100 * time.Millisecond,
	}

	result, err := perf.RunBenchmark(ctx, cfg, io.Discard)
	if err != nil {
		t.Fatalf("RunBenchmark: %v", err)
	}

	// 3. Verify result fields.
	if result.DUTName != "test-forwarder" {
		t.Errorf("DUTName = %q, want %q", result.DUTName, "test-forwarder")
	}

	if result.Routes != 10 {
		t.Errorf("Routes = %d, want 10", result.Routes)
	}

	if result.RoutesReceived <= 0 {
		t.Errorf("RoutesReceived = %d, want > 0", result.RoutesReceived)
	}

	if result.RoutesSent != 10 {
		t.Errorf("RoutesSent = %d, want 10", result.RoutesSent)
	}

	// With 10 routes on loopback, sub-millisecond convergence is normal
	// and rounds to 0ms. Verify non-negative.
	if result.ConvergenceMs < 0 {
		t.Errorf("ConvergenceMs = %d, want >= 0", result.ConvergenceMs)
	}

	if result.Repeat != 3 {
		t.Errorf("Repeat = %d, want 3", result.Repeat)
	}

	if result.RepeatKept < 2 {
		t.Errorf("RepeatKept = %d, want >= 2", result.RepeatKept)
	}

	if result.Seed != 42 {
		t.Errorf("Seed = %d, want 42", result.Seed)
	}

	// 4. Generate Markdown report and verify content.
	var mdBuf bytes.Buffer
	if err := report.Markdown([]perf.Result{result}, &mdBuf); err != nil {
		t.Fatalf("Markdown: %v", err)
	}

	md := mdBuf.String()

	if !strings.Contains(md, "test-forwarder") {
		t.Errorf("Markdown output missing DUT name 'test-forwarder'")
	}

	if !strings.Contains(md, "|") {
		t.Errorf("Markdown output missing table delimiter '|'")
	}

	// 5. Generate HTML report and verify content.
	var htmlBuf bytes.Buffer
	if err := report.HTML([]perf.Result{result}, &htmlBuf); err != nil {
		t.Fatalf("HTML: %v", err)
	}

	htmlStr := htmlBuf.String()

	if !strings.Contains(htmlStr, "<html>") {
		t.Errorf("HTML output missing '<html>' tag")
	}

	if !strings.Contains(htmlStr, "test-forwarder") {
		t.Errorf("HTML output missing DUT name 'test-forwarder'")
	}

	// 6. NDJSON round-trip.
	var ndjsonBuf bytes.Buffer
	if err := perf.WriteNDJSON(&ndjsonBuf, result); err != nil {
		t.Fatalf("WriteNDJSON: %v", err)
	}

	readBack, err := perf.ReadNDJSON(&ndjsonBuf)
	if err != nil {
		t.Fatalf("ReadNDJSON: %v", err)
	}

	if len(readBack) != 1 {
		t.Fatalf("ReadNDJSON returned %d results, want 1", len(readBack))
	}

	rt := readBack[0]

	if rt.DUTName != result.DUTName {
		t.Errorf("round-trip DUTName = %q, want %q", rt.DUTName, result.DUTName)
	}

	if rt.Routes != result.Routes {
		t.Errorf("round-trip Routes = %d, want %d", rt.Routes, result.Routes)
	}

	if rt.Seed != result.Seed {
		t.Errorf("round-trip Seed = %d, want %d", rt.Seed, result.Seed)
	}

	if rt.ConvergenceMs != result.ConvergenceMs {
		t.Errorf("round-trip ConvergenceMs = %d, want %d", rt.ConvergenceMs, result.ConvergenceMs)
	}

	if rt.RoutesReceived != result.RoutesReceived {
		t.Errorf("round-trip RoutesReceived = %d, want %d", rt.RoutesReceived, result.RoutesReceived)
	}

	cancel()
}
