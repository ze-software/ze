package perf

import (
	"context"
	"io"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// VALIDATES: "End-to-end with trivial forwarder: send 10 routes, get metrics."
// PREVENTS: "Sender/receiver/metrics not integrated."
//
// TestRunSmallBenchmark is an end-to-end integration test that sends 10 routes
// through a trivial BGP forwarder and verifies that metrics are computed correctly.
func TestRunSmallBenchmark(t *testing.T) {
	t.Parallel()

	const routeCount = 10

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Start the test forwarder.
	fwd := newTestForwarder(t)

	fwdDone := make(chan struct{})

	go func() {
		defer close(fwdDone)
		fwd.Run(ctx)
	}()

	// 2. Generate routes.
	routes := GenerateIPv4Routes(42, routeCount)
	if len(routes) != routeCount {
		t.Fatalf("generated %d routes, want %d", len(routes), routeCount)
	}

	// 3. Dial two connections to the forwarder.
	// The forwarder accepts connections in order; connection ordering does not
	// matter because it forwards UPDATEs bidirectionally.
	var dialer net.Dialer

	senderConn, err := dialer.DialContext(ctx, "tcp", fwd.Addr())
	if err != nil {
		t.Fatalf("dial sender: %v", err)
	}
	defer func() { _ = senderConn.Close() }()

	receiverConn, err := dialer.DialContext(ctx, "tcp", fwd.Addr())
	if err != nil {
		t.Fatalf("dial receiver: %v", err)
	}
	defer func() { _ = receiverConn.Close() }()

	// Set TCP_NODELAY on both.
	if tc, ok := senderConn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	if tc, ok := receiverConn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	// 4. BGP handshake on both connections.
	senderCfg := SessionConfig{
		ASN:      65001,
		RouterID: netip.MustParseAddr("1.1.1.1"),
		HoldTime: 90,
		Family:   "ipv4/unicast",
	}

	receiverCfg := SessionConfig{
		ASN:      65002,
		RouterID: netip.MustParseAddr("2.2.2.2"),
		HoldTime: 90,
		Family:   "ipv4/unicast",
	}

	doBGPHandshake(t, senderConn, senderCfg)
	doBGPHandshake(t, receiverConn, receiverCfg)

	// 5. Start receiver goroutine.
	var mu sync.Mutex

	recvTimes := make(map[netip.Prefix]time.Time, routeCount)
	recvDone := make(chan struct{})

	go func() {
		defer close(recvDone)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_ = receiverConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

			msgType, msg, err := ReadMessage(receiverConn)
			if err != nil {
				if isTimeout(err) {
					// Check if we have all routes yet.
					mu.Lock()
					got := len(recvTimes)
					mu.Unlock()

					if got >= routeCount {
						return
					}

					continue
				}

				t.Logf("receiver read error: %v", err)

				return
			}

			if msgType != message.TypeUPDATE {
				continue
			}

			// Extract prefixes from the UPDATE body (after header).
			body := msg[message.HeaderLen:]
			prefixes := ExtractPrefixes(body)
			now := time.Now()

			mu.Lock()
			for _, p := range prefixes {
				if _, exists := recvTimes[p]; !exists {
					recvTimes[p] = now
				}
			}

			got := len(recvTimes)
			mu.Unlock()

			if got >= routeCount {
				return
			}
		}
	}()

	// 6. Send routes via sender, recording send times.
	sender := NewSender(SenderConfig{
		ASN:     65001,
		IsEBGP:  true,
		NextHop: netip.MustParseAddr("1.1.1.1"),
		Family:  "ipv4/unicast",
	})

	sendTimes := make(map[netip.Prefix]time.Time, routeCount)

	for _, prefix := range routes {
		updateBytes := sender.BuildRoute(prefix)
		if updateBytes == nil {
			t.Fatalf("BuildRoute returned nil for %s", prefix)
		}

		if err := WriteMessage(senderConn, updateBytes); err != nil {
			t.Fatalf("sending route %s: %v", prefix, err)
		}

		sendTimes[prefix] = time.Now()
	}

	// 7. Wait for receiver to get all routes (with timeout).
	select {
	case <-recvDone:
		// Receiver collected all routes.
	case <-ctx.Done():
		mu.Lock()
		got := len(recvTimes)
		mu.Unlock()
		t.Fatalf("timeout waiting for routes: received %d/%d", got, routeCount)
	}

	// 8. Compute and verify metrics.
	mu.Lock()
	defer mu.Unlock()

	if len(recvTimes) != routeCount {
		t.Fatalf("received %d routes, want %d", len(recvTimes), routeCount)
	}

	// Build per-route durations by joining send and receive maps.
	durations := make([]time.Duration, 0, routeCount)

	for prefix, sendTime := range sendTimes {
		recvTime, ok := recvTimes[prefix]
		if !ok {
			t.Fatalf("prefix %s was sent but not received", prefix)
		}

		d := recvTime.Sub(sendTime)
		if d < 0 {
			t.Fatalf("negative latency for %s: %v", prefix, d)
		}

		durations = append(durations, d)
	}

	// All latencies must be non-negative.
	for i, d := range durations {
		if d < 0 {
			t.Errorf("latency[%d] = %v, want >= 0", i, d)
		}
	}

	p50, p90, p99, pMax := CalculateLatencies(durations)

	// With 10 routes through loopback, latencies should be tiny but ordered.
	if p50 > p90 {
		t.Errorf("p50 (%d) > p90 (%d)", p50, p90)
	}

	if p90 > p99 {
		t.Errorf("p90 (%d) > p99 (%d)", p90, p99)
	}

	if p99 > pMax {
		t.Errorf("p99 (%d) > max (%d)", p99, pMax)
	}

	// 9. Build an IterationResult and verify fields are populated.
	recvTimestamps := make([]time.Time, 0, routeCount)
	for _, ts := range recvTimes {
		recvTimestamps = append(recvTimestamps, ts)
	}

	sort.Slice(recvTimestamps, func(i, j int) bool {
		return recvTimestamps[i].Before(recvTimestamps[j])
	})

	tpAvg, tpPeak := CalculateThroughput(recvTimestamps)

	result := IterationResult{
		RoutesReceived: len(recvTimes),
		LatencyP50Ms:   p50,
		LatencyP90Ms:   p90,
		LatencyP99Ms:   p99,
		LatencyMaxMs:   pMax,
		ThroughputAvg:  tpAvg,
		ThroughputPeak: tpPeak,
	}

	if result.RoutesReceived != routeCount {
		t.Errorf("IterationResult.RoutesReceived = %d, want %d", result.RoutesReceived, routeCount)
	}

	// 10. Cleanup.
	cancel()
	<-fwdDone
}

// VALIDATES: "Benchmark handles timeout when DUT drops UPDATEs."
// PREVENTS: "RunBenchmark hangs or panics when convergence is not reached."
//
// TestBenchmarkTimeout verifies that RunBenchmark completes gracefully when
// the DUT accepts connections and handshakes but never forwards UPDATEs,
// producing a result with lost routes.
func TestBenchmarkTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Start the sink forwarder (does handshake, drops all UPDATEs).
	sink := newTestSinkForwarder(t)

	go func() {
		sink.Run(ctx)
	}()

	// Extract host and port.
	host, portStr, err := net.SplitHostPort(sink.Addr())
	if err != nil {
		t.Fatalf("splitting sink address %q: %v", sink.Addr(), err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parsing port %q: %v", portStr, err)
	}

	// 2. Run the benchmark with a short duration so it times out quickly.
	cfg := BenchmarkConfig{
		DUTAddr:        host,
		DUTPort:        port,
		DUTASN:         65000,
		DUTName:        "sink-forwarder",
		SenderAddr:     "127.0.0.1",
		SenderASN:      65001,
		ReceiverAddr:   "127.0.0.1",
		ReceiverASN:    65002,
		Routes:         10,
		Family:         "ipv4/unicast",
		Seed:           99,
		Warmup:         0,
		ConnectTimeout: 5 * time.Second,
		Duration:       2 * time.Second,
		Repeat:         1,
		WarmupRuns:     0,
		IterDelay:      0,
	}

	result, err := RunBenchmark(ctx, cfg, io.Discard)
	if err != nil {
		t.Fatalf("RunBenchmark: %v", err)
	}

	// 3. Verify: routes were sent but not all received.
	if result.RoutesSent != 10 {
		t.Errorf("RoutesSent = %d, want 10", result.RoutesSent)
	}

	if result.RoutesReceived >= result.RoutesSent {
		t.Errorf("RoutesReceived = %d, want < RoutesSent (%d) since DUT drops UPDATEs",
			result.RoutesReceived, result.RoutesSent)
	}

	if result.RoutesLost <= 0 {
		t.Errorf("RoutesLost = %d, want > 0", result.RoutesLost)
	}

	cancel()
}
