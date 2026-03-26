// Design: (none -- new tool, predates documentation)
// Related: session.go -- BGP message I/O and handshake
// Related: sender.go -- UPDATE construction
// Related: receiver.go -- prefix extraction from UPDATEs
// Related: metrics.go -- latency and throughput computation
// Related: result.go -- Result type for output

package perf

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
)

// Address family constants for benchmark configuration.
const (
	FamilyIPv4Unicast = "ipv4/unicast"
	FamilyIPv6Unicast = "ipv6/unicast"
)

// BenchmarkConfig holds all parameters for a benchmark run.
type BenchmarkConfig struct {
	DUTAddr        string
	DUTPort        int
	DUTASN         int
	DUTName        string
	DUTVersion     string
	SenderAddr     string
	SenderASN      int
	SenderPort     int // 0 = use DUTPort
	ReceiverAddr   string
	ReceiverASN    int
	ReceiverPort   int // 0 = use DUTPort
	Routes         int
	Family         string
	ForceMP        bool
	Seed           uint64
	Warmup         time.Duration
	ConnectTimeout time.Duration
	Duration       time.Duration
	Repeat         int
	WarmupRuns     int
	IterDelay      time.Duration
	BatchSize      int
}

// RunBenchmark runs the full benchmark: generate routes, run iterations,
// remove outliers, aggregate, and return a Result. Progress messages are
// written to progress (typically os.Stderr).
//
//nolint:cyclop // Orchestrator function: sequential steps, not complex branching.
func RunBenchmark(ctx context.Context, cfg BenchmarkConfig, progress io.Writer) (Result, error) {
	seed := cfg.Seed
	if seed == 0 {
		s, err := cryptoRandUint64()
		if err != nil {
			return Result{}, fmt.Errorf("generating random seed: %w", err)
		}

		seed = s
	}

	prefixes, err := generatePrefixes(cfg.Family, seed, cfg.Routes)
	if err != nil {
		return Result{}, err
	}

	totalIterations := cfg.WarmupRuns + cfg.Repeat
	var collected []IterationResult

	for i := range totalIterations {
		isWarmup := i < cfg.WarmupRuns
		label := fmt.Sprintf("iteration %d/%d", i+1, totalIterations)

		if isWarmup {
			label += " (warmup)"
		}

		_, _ = fmt.Fprintf(progress, "%s\n", label)

		iter, err := runIteration(ctx, cfg, prefixes)
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", label, err)
		}

		if !isWarmup {
			collected = append(collected, iter)
		}

		// Delay between iterations (skip after last).
		if cfg.IterDelay > 0 && i < totalIterations-1 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(cfg.IterDelay):
			}
		}
	}

	kept := RemoveOutliers(collected)
	agg := Aggregate(kept)

	result := Result{
		DUTName:             cfg.DUTName,
		DUTVersion:          cfg.DUTVersion,
		DUTAddr:             cfg.DUTAddr,
		DUTPort:             cfg.DUTPort,
		DUTASN:              cfg.DUTASN,
		Routes:              cfg.Routes,
		Family:              cfg.Family,
		ForceMP:             cfg.ForceMP,
		Seed:                seed,
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
		Repeat:              cfg.Repeat,
		RepeatKept:          len(kept),
		WarmupRuns:          cfg.WarmupRuns,
		IterDelayMs:         int(cfg.IterDelay / time.Millisecond),
		SessionSetupMs:      SessionSetup{Sender: agg.SessionSenderMs, Receiver: agg.SessionReceiverMs},
		FirstRouteMs:        agg.FirstRouteMs,
		ConvergenceMs:       agg.ConvergenceMs,
		ConvergenceStddevMs: agg.ConvergenceStddevMs,
		RoutesSent:          cfg.Routes,
		RoutesReceived:      agg.RoutesReceived,
		RoutesLost:          cfg.Routes - agg.RoutesReceived,
		ThroughputAvg:       agg.ThroughputAvg,
		ThroughputAvgStddev: agg.ThroughputAvgStddev,
		ThroughputPeak:      agg.ThroughputPeak,
		LatencyP50Ms:        agg.LatencyP50Ms,
		LatencyP90Ms:        agg.LatencyP90Ms,
		LatencyP99Ms:        agg.LatencyP99Ms,
		LatencyP99StddevMs:  agg.LatencyP99StddevMs,
		LatencyMaxMs:        agg.LatencyMaxMs,
	}

	return result, nil
}

// generatePrefixes creates route prefixes for the given family and seed.
func generatePrefixes(family string, seed uint64, count int) ([]netip.Prefix, error) {
	switch family {
	case FamilyIPv4Unicast:
		return GenerateIPv4Routes(seed, count), nil
	case FamilyIPv6Unicast:
		return GenerateIPv6Routes(seed, count), nil
	}

	return nil, fmt.Errorf("unsupported family: %s", family)
}

// runIteration executes a single benchmark iteration: connect sender and receiver,
// handshake, inject routes, wait for convergence, compute metrics.
//
//nolint:cyclop,funlen // Sequential orchestration of a multi-step benchmark iteration.
func runIteration(ctx context.Context, cfg BenchmarkConfig, prefixes []netip.Prefix) (IterationResult, error) {
	receiverPort := cfg.DUTPort
	if cfg.ReceiverPort > 0 {
		receiverPort = cfg.ReceiverPort
	}

	senderPort := cfg.DUTPort
	if cfg.SenderPort > 0 {
		senderPort = cfg.SenderPort
	}

	// Connect receiver first.
	receiverAddr := net.JoinHostPort(cfg.DUTAddr, fmt.Sprintf("%d", receiverPort))

	receiverConn, err := connectBGP(ctx, cfg.ReceiverAddr, receiverAddr, cfg.ConnectTimeout)
	if err != nil {
		return IterationResult{}, fmt.Errorf("connecting receiver: %w", err)
	}
	defer func() { _ = receiverConn.Close() }()

	receiverCfg := SessionConfig{
		ASN:      uint32(cfg.ReceiverASN), //nolint:gosec // CLI-validated range
		RouterID: mustRouterID(cfg.ReceiverAddr),
		HoldTime: 90,
		Family:   cfg.Family,
	}

	if err := receiverConn.SetDeadline(time.Now().Add(cfg.ConnectTimeout)); err != nil {
		return IterationResult{}, fmt.Errorf("setting receiver deadline: %w", err)
	}

	receiverSetup, err := DoHandshake(receiverConn, receiverCfg)
	if err != nil {
		return IterationResult{}, fmt.Errorf("receiver handshake: %w", err)
	}

	if err := receiverConn.SetDeadline(time.Time{}); err != nil {
		return IterationResult{}, fmt.Errorf("clearing receiver deadline: %w", err)
	}

	// Connect sender.
	senderDUTAddr := net.JoinHostPort(cfg.DUTAddr, fmt.Sprintf("%d", senderPort))

	senderConn, err := connectBGP(ctx, cfg.SenderAddr, senderDUTAddr, cfg.ConnectTimeout)
	if err != nil {
		return IterationResult{}, fmt.Errorf("connecting sender: %w", err)
	}
	defer func() { _ = senderConn.Close() }()

	senderCfg := SessionConfig{
		ASN:      uint32(cfg.SenderASN), //nolint:gosec // CLI-validated range
		RouterID: mustRouterID(cfg.SenderAddr),
		HoldTime: 90,
		Family:   cfg.Family,
	}

	if err := senderConn.SetDeadline(time.Now().Add(cfg.ConnectTimeout)); err != nil {
		return IterationResult{}, fmt.Errorf("setting sender deadline: %w", err)
	}

	senderSetup, err := DoHandshake(senderConn, senderCfg)
	if err != nil {
		return IterationResult{}, fmt.Errorf("sender handshake: %w", err)
	}

	if err := senderConn.SetDeadline(time.Time{}); err != nil {
		return IterationResult{}, fmt.Errorf("clearing sender deadline: %w", err)
	}

	// Start keepalive goroutines.
	kaCtx, kaCancel := context.WithCancel(ctx)
	defer kaCancel()

	var kaWg sync.WaitGroup

	kaWg.Go(func() {
		keepaliveLoop(kaCtx, senderConn)
	})

	kaWg.Go(func() {
		keepaliveLoop(kaCtx, receiverConn)
	})

	// Start receiver goroutine.
	var mu sync.Mutex

	recvTimes := make(map[netip.Prefix]time.Time, len(prefixes))
	allReceived := make(chan struct{})

	recvCtx, recvCancel := context.WithCancel(ctx)
	defer recvCancel()

	var recvWg sync.WaitGroup

	recvWg.Go(func() {
		receiveLoop(recvCtx, receiverConn, len(prefixes), &mu, recvTimes, allReceived)
	})

	// Wait warmup duration.
	if cfg.Warmup > 0 {
		select {
		case <-ctx.Done():
			return IterationResult{}, ctx.Err()
		case <-time.After(cfg.Warmup):
		}
	}

	// Send routes.
	sender := NewSender(SenderConfig{
		ASN:     uint32(cfg.SenderASN), //nolint:gosec // CLI-validated range
		IsEBGP:  cfg.SenderASN != cfg.DUTASN,
		NextHop: mustAddr(cfg.SenderAddr),
		Family:  cfg.Family,
		ForceMP: cfg.ForceMP,
	})

	sendTimes := make(map[netip.Prefix]time.Time, len(prefixes))
	sendStart := time.Now()

	for _, prefix := range prefixes {
		updateBytes := sender.BuildRoute(prefix)
		if updateBytes == nil {
			return IterationResult{}, fmt.Errorf("BuildRoute returned nil for %s", prefix)
		}

		if err := WriteMessage(senderConn, updateBytes); err != nil {
			return IterationResult{}, fmt.Errorf("sending route %s: %w", prefix, err)
		}

		sendTimes[prefix] = time.Now()
	}

	// Wait for convergence or timeout.
	deadline := time.NewTimer(cfg.Duration)
	defer deadline.Stop()

	select {
	case <-allReceived:
		// All prefixes received.
	case <-deadline.C:
		// Timeout -- proceed with partial results.
	case <-ctx.Done():
		return IterationResult{}, ctx.Err()
	}

	// Stop receiver.
	recvCancel()
	recvWg.Wait()

	// Compute metrics.
	// Only count prefixes that were both sent and received -- the DUT may
	// advertise its own routes (connected networks, loopback) which would
	// inflate len(recvTimes) beyond cfg.Routes and produce negative RoutesLost.
	mu.Lock()

	var durations []time.Duration

	var recvTimestamps []time.Time

	var firstRouteMs int

	for prefix, sendTime := range sendTimes {
		recvTime, ok := recvTimes[prefix]
		if !ok {
			continue
		}

		d := max(recvTime.Sub(sendTime), 0)
		durations = append(durations, d)
		recvTimestamps = append(recvTimestamps, recvTime)
	}

	received := len(durations)

	mu.Unlock()

	if len(recvTimestamps) > 0 {
		sort.Slice(recvTimestamps, func(i, j int) bool {
			return recvTimestamps[i].Before(recvTimestamps[j])
		})

		firstRouteMs = int(recvTimestamps[0].Sub(sendStart) / time.Millisecond)
	}

	convergenceMs := 0
	if len(recvTimestamps) > 0 {
		last := recvTimestamps[len(recvTimestamps)-1]
		convergenceMs = int(last.Sub(sendStart) / time.Millisecond)
	}

	p50, p90, p99, latMax := CalculateLatencies(durations)
	tpAvg, tpPeak := CalculateThroughput(recvTimestamps, time.Duration(convergenceMs)*time.Millisecond)

	// Send NOTIFICATION Cease on both connections.
	cease := BuildCeaseNotification()
	_ = WriteMessage(senderConn, cease)
	_ = WriteMessage(receiverConn, cease)

	// Give 100ms for NOTIFICATION to be sent before closing.
	time.Sleep(100 * time.Millisecond)

	// Stop keepalives.
	kaCancel()
	kaWg.Wait()

	return IterationResult{
		ConvergenceMs:     convergenceMs,
		FirstRouteMs:      firstRouteMs,
		ThroughputAvg:     tpAvg,
		ThroughputPeak:    tpPeak,
		LatencyP50Ms:      p50,
		LatencyP90Ms:      p90,
		LatencyP99Ms:      p99,
		LatencyMaxMs:      latMax,
		RoutesReceived:    received,
		SessionSenderMs:   int(senderSetup / time.Millisecond),
		SessionReceiverMs: int(receiverSetup / time.Millisecond),
	}, nil
}

// connectBGP establishes a BGP TCP connection by racing two strategies:
// 1. Dial out to remoteAddr from localAddr (standard client behavior)
// 2. Listen on localAddr:179 for inbound connection from the DUT
//
// Some implementations (e.g., rustbgpd) only respond to peers they can actively
// connect to. By also listening, ze-perf works with both active and passive DUTs.
// The first connection to succeed wins; the other is cleaned up.
func connectBGP(ctx context.Context, localAddr, remoteAddr string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start listener for inbound connections BEFORE dialing out.
	// This ensures DUTs that actively connect (e.g., rustbgpd) find a listener.
	listenAddr := net.JoinHostPort(localAddr, "179")

	var lc net.ListenConfig

	listener, listenErr := lc.Listen(ctx, "tcp", listenAddr)

	// Dial out to the DUT.
	local, err := net.ResolveTCPAddr("tcp", localAddr+":0")
	if err != nil {
		if listener != nil {
			listener.Close() //nolint:errcheck // cleanup
		}

		return nil, fmt.Errorf("resolving local address %s: %w", localAddr, err)
	}

	dialer := net.Dialer{LocalAddr: local, Timeout: timeout}

	conn, dialErr := dialer.DialContext(ctx, "tcp", remoteAddr)

	// Decide which connection path to use.
	switch {
	case dialErr == nil && listenErr != nil:
		// Dial succeeded, no listener. Use dialed connection directly.
		return setNoDelay(conn)

	case dialErr != nil && listenErr != nil:
		return nil, fmt.Errorf("connecting to %s: dial: %w, listen: %w", remoteAddr, dialErr, listenErr)

	case dialErr != nil && listener == nil:
		return nil, fmt.Errorf("dialing %s: %w", remoteAddr, dialErr)

	case dialErr == nil && listener != nil:
		// Dial succeeded at TCP level. Probe whether the DUT actually responds with BGP data.
		// Some DUTs (e.g., rustbgpd) accept TCP but don't send OPEN until their own outbound
		// connect succeeds. If the probe times out, fall back to the listened connection.
		if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err == nil { //nolint:mnd // probe timeout
			probe := make([]byte, 1)

			_, probeErr := conn.Read(probe)
			if probeErr == nil {
				// DUT responded. Replay the probed byte via prefixedConn.
				if err := conn.SetReadDeadline(time.Time{}); err != nil {
					conn.Close()     //nolint:errcheck // cleanup
					listener.Close() //nolint:errcheck // cleanup
					return nil, fmt.Errorf("clearing deadline: %w", err)
				}

				listener.Close() //nolint:errcheck // cleanup

				return &prefixedConn{Conn: conn, prefix: probe}, nil
			}

			// Dial connection is unresponsive. Close it and wait for inbound.
			conn.Close() //nolint:errcheck // switching to listener path
		}

	case dialErr != nil && listener != nil:
		// Dial failed but listener is up. Fall through to accept inbound.
	}

	// Wait for inbound connection on the listener.
	if listener != nil {
		defer listener.Close() //nolint:errcheck // cleanup

		inConn, err := listener.Accept()
		if err != nil {
			return nil, fmt.Errorf("accepting inbound from DUT: %w", err)
		}

		return setNoDelay(inConn)
	}

	return nil, fmt.Errorf("no connection to %s", remoteAddr)
}

// setNoDelay enables TCP_NODELAY on a connection.
func setNoDelay(conn net.Conn) (net.Conn, error) {
	if tc, ok := conn.(*net.TCPConn); ok {
		if err := tc.SetNoDelay(true); err != nil {
			return nil, fmt.Errorf("setting TCP_NODELAY: %w", err)
		}
	}

	return conn, nil
}

// prefixedConn wraps a net.Conn and prepends previously-read bytes to the next Read.
// Used when a probe byte was read to test connection liveness.
type prefixedConn struct {
	net.Conn
	prefix []byte
	done   bool
}

func (c *prefixedConn) Read(b []byte) (int, error) {
	if !c.done && len(c.prefix) > 0 {
		n := copy(b, c.prefix)
		c.prefix = c.prefix[n:]

		if len(c.prefix) == 0 {
			c.done = true
		}

		// If there's room, read more from the underlying connection.
		if n < len(b) {
			m, err := c.Conn.Read(b[n:])
			return n + m, err
		}

		return n, nil
	}

	return c.Conn.Read(b)
}

// keepaliveLoop sends KEEPALIVE messages every 30 seconds until ctx is canceled.
func keepaliveLoop(ctx context.Context, conn net.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ka := BuildKeepalive()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := WriteMessage(conn, ka); err != nil {
				return
			}
		}
	}
}

// receiveLoop reads BGP messages from conn, extracting prefixes from UPDATEs.
// Signals allReceived when expectedCount prefixes have been collected.
// Stops on context cancellation or connection error.
func receiveLoop(
	ctx context.Context,
	conn net.Conn,
	expectedCount int,
	mu *sync.Mutex,
	recvTimes map[netip.Prefix]time.Time,
	allReceived chan struct{},
) {
	for {
		// Check for cancellation before each read.
		if ctx.Err() != nil {
			return
		}

		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

		msgType, msg, err := ReadMessage(conn)
		if err != nil {
			if isNetTimeout(err) {
				mu.Lock()
				got := len(recvTimes)
				mu.Unlock()

				if got >= expectedCount {
					close(allReceived)
					return
				}

				continue
			}

			// Connection closed or other error -- stop gracefully.
			return
		}

		if msgType != message.TypeUPDATE {
			continue
		}

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

		if got >= expectedCount {
			close(allReceived)
			return
		}
	}
}

// isNetTimeout reports whether err is a network timeout error.
func isNetTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout()
	}

	return false
}

// mustAddr parses an IP address string, returning the zero address on failure.
func mustAddr(s string) netip.Addr {
	addr, _ := netip.ParseAddr(s)
	return addr
}

// mustRouterID derives a router ID from an address string.
// For IPv4, uses the address directly. For IPv6, uses 0.0.0.N based on the last byte.
func mustRouterID(s string) netip.Addr {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.MustParseAddr("0.0.0.1")
	}

	if addr.Is4() {
		return addr
	}

	// For IPv6, derive a router ID from the last byte.
	raw := addr.As16()
	last := raw[15]
	if last == 0 {
		last = 1
	}

	return netip.AddrFrom4([4]byte{0, 0, 0, last})
}

// cryptoRandUint64 returns a cryptographically random uint64.
func cryptoRandUint64() (uint64, error) {
	n, err := rand.Int(rand.Reader, new(big.Int).SetUint64(^uint64(0)))
	if err != nil {
		return 0, err
	}

	return n.Uint64(), nil
}
