// Design: (none -- new tool, predates documentation)
// Detail: session.go -- BGP message I/O and handshake
// Detail: sender.go -- UPDATE construction
// Detail: receiver.go -- prefix extraction from UPDATEs
// Detail: routes.go -- prefix generation
// Detail: metrics.go -- latency and throughput computation
// Detail: result.go -- Result type for output

package perf

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/msgtype"

	"github.com/ze-software/ze/internal/component/bgp/message"
)

var errBuildrouteReturnedNilForProbePrefix = errors.New("BuildRoute returned nil for probe prefix")

// rawMessage holds a raw BGP UPDATE message and its receive timestamp.
// Used to defer prefix extraction until after the receive loop completes,
// keeping parsing CPU out of the timing path.
type rawMessage struct {
	data []byte
	when time.Time
}

// batchRange maps a contiguous slice of prefixes to one UPDATE message.
type batchRange struct{ start, end int }

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
	PassiveListen  bool // Listen on port 179 for inbound DUT connections (requires root)
	SenderFirst    bool // Connect sender before receiver (measures RIB propagation)
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

		_, _ = fmt.Fprintf(progress, "%s\n", label) //nolint:errcheck // output

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
		WithdrawalMs:        agg.WithdrawalMs,
		WithdrawalStddevMs:  agg.WithdrawalStddevMs,
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
// Measurement isolation: all UPDATE wire bytes are pre-built before the send loop
// (no encoding CPU in the timing path), and the receiver buffers raw messages with
// timestamps without parsing (no decoding CPU in the timing path). Prefix extraction
// happens after the receiver is done. Buffered I/O (16 KB write, 64 KB read) matches
// ze production and minimizes syscall overhead.
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

	receiverAddr := net.JoinHostPort(cfg.DUTAddr, strconv.Itoa(receiverPort))
	senderDUTAddr := net.JoinHostPort(cfg.DUTAddr, strconv.Itoa(senderPort))

	type connResult struct {
		conn net.Conn
		err  error
	}

	var receiverConn net.Conn
	var senderConn net.Conn
	var receiverSetup time.Duration
	var sendStart time.Time
	var err error

	switch { //nolint:gocritic // ifElseChain: intentional, each branch has distinct connection setup logic
	case cfg.PassiveListen:
		receiverCh := make(chan connResult, 1)
		senderCh := make(chan connResult, 1)

		go func() {
			conn, err := connectBGP(ctx, cfg.ReceiverAddr, receiverAddr, cfg.ConnectTimeout, true)
			receiverCh <- connResult{conn: conn, err: err}
		}()

		go func() {
			conn, err := connectBGP(ctx, cfg.SenderAddr, senderDUTAddr, cfg.ConnectTimeout, true)
			senderCh <- connResult{conn: conn, err: err}
		}()

		senderRes := <-senderCh
		if senderRes.err != nil {
			receiverRes := <-receiverCh
			if receiverRes.conn != nil {
				receiverRes.conn.Close() //nolint:errcheck // cleanup
			}
			return IterationResult{}, fmt.Errorf("connecting sender: %w", senderRes.err)
		}
		senderConn = senderRes.conn

		receiverRes := <-receiverCh
		if receiverRes.err != nil {
			senderConn.Close() //nolint:errcheck // cleanup
			return IterationResult{}, fmt.Errorf("connecting receiver: %w", receiverRes.err)
		}
		receiverConn = receiverRes.conn
	case cfg.SenderFirst:
		senderConn, err = connectBGP(ctx, cfg.SenderAddr, senderDUTAddr, cfg.ConnectTimeout, false)
		if err != nil {
			return IterationResult{}, fmt.Errorf("connecting sender: %w", err)
		}
	default:
		receiverConn, err = connectBGP(ctx, cfg.ReceiverAddr, receiverAddr, cfg.ConnectTimeout, false)
		if err != nil {
			return IterationResult{}, fmt.Errorf("connecting receiver: %w", err)
		}

		senderConn, err = connectBGP(ctx, cfg.SenderAddr, senderDUTAddr, cfg.ConnectTimeout, false)
		if err != nil {
			receiverConn.Close() //nolint:errcheck // cleanup
			return IterationResult{}, fmt.Errorf("connecting sender: %w", err)
		}
	}

	if senderConn != nil {
		defer senderConn.Close() //nolint:errcheck // cleanup
	}

	senderCfg := SessionConfig{
		ASN:      uint32(cfg.SenderASN), //nolint:gosec // CLI-validated range
		RouterID: mustRouterID(cfg.SenderAddr),
		HoldTime: 90,
		Family:   cfg.Family,
	}

	// In receiver-first mode, handshake receiver before sender (matches
	// the test forwarder's accept-handshake-accept-handshake sequence).
	if !cfg.SenderFirst && !cfg.PassiveListen {
		receiverCfg := SessionConfig{
			ASN:      uint32(cfg.ReceiverASN), //nolint:gosec // CLI-validated range
			RouterID: mustRouterID(cfg.ReceiverAddr),
			HoldTime: 90,
			Family:   cfg.Family,
		}

		if err := receiverConn.SetDeadline(time.Now().Add(cfg.ConnectTimeout)); err != nil {
			return IterationResult{}, fmt.Errorf("setting receiver deadline: %w", err)
		}

		receiverSetup, err = DoHandshake(receiverConn, receiverCfg)
		if err != nil {
			return IterationResult{}, fmt.Errorf("receiver handshake: %w", err)
		}

		if err := receiverConn.SetDeadline(time.Time{}); err != nil {
			return IterationResult{}, fmt.Errorf("clearing receiver deadline: %w", err)
		}
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

	sender := NewSender(SenderConfig{
		ASN:     uint32(cfg.SenderASN), //nolint:gosec // CLI-validated range
		IsEBGP:  cfg.SenderASN != cfg.DUTASN,
		NextHop: parseAddr(cfg.SenderAddr),
		Family:  cfg.Family,
		ForceMP: cfg.ForceMP,
	})

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		probe := sender.BuildRoute(prefixes[0])
		if probe == nil {
			return IterationResult{}, errBuildrouteReturnedNilForProbePrefix
		}

		perNLRI := 1 + (prefixes[0].Bits()+7)/8
		overhead := len(probe) - perNLRI
		if overhead >= message.MaxMsgLen {
			return IterationResult{}, fmt.Errorf("attribute overhead %d exceeds max message size %d", overhead, message.MaxMsgLen)
		}
		batchSize = max((message.MaxMsgLen-overhead-1)/perNLRI, 1)
	}

	var batches []batchRange

	for i := 0; i < len(prefixes); i += batchSize {
		batches = append(batches, batchRange{i, min(i+batchSize, len(prefixes))})
	}

	prebuilt := make([][]byte, len(batches))

	totalSendBytes := 0

	for i, br := range batches {
		b := sender.BuildBatch(prefixes[br.start:br.end])
		if b == nil {
			return IterationResult{}, fmt.Errorf("BuildBatch returned nil for batch %d", i)
		}

		prebuilt[i] = b
		totalSendBytes += len(b)
	}

	sendSlab := make([]byte, totalSendBytes)
	off := 0

	for i, b := range prebuilt {
		copy(sendSlab[off:], b)
		prebuilt[i] = sendSlab[off : off+len(b)]
		off += len(b)
	}

	sendBufWriter := bufio.NewWriterSize(senderConn, 16384)
	sendTimes := make(map[netip.Prefix]time.Time, len(prefixes))
	recvSlab := make([]byte, len(prefixes)*128) //nolint:mnd // generous estimate
	rawMsgs := make([]rawMessage, 0, len(prefixes))

	recvCtx, recvCancel := context.WithCancel(ctx)
	defer recvCancel()

	recvDone := make(chan struct{})

	var recvWg sync.WaitGroup

	senderKaCtx, senderKaCancel := context.WithCancel(ctx)
	defer senderKaCancel()

	var senderKaWg sync.WaitGroup

	recvKaCtx, recvKaCancel := context.WithCancel(ctx)
	defer recvKaCancel()

	var recvKaWg sync.WaitGroup
	var recvBufReader *bufio.Reader

	if cfg.SenderFirst {
		// Sender-first: send routes, wait for ingestion, connect receiver, measure.
		if err := writeSendBatches(sendBufWriter, prebuilt, prefixes, batches, sendTimes); err != nil {
			return IterationResult{}, fmt.Errorf("sending batches: %w", err)
		}
		if err := sendBufWriter.Flush(); err != nil {
			return IterationResult{}, fmt.Errorf("flushing sender: %w", err)
		}

		senderKaWg.Go(func() {
			keepaliveLoop(senderKaCtx, senderConn)
		})

		if cfg.Warmup > 0 {
			select {
			case <-ctx.Done():
				return IterationResult{}, ctx.Err()
			case <-time.After(cfg.Warmup):
			}
		}

		// Connect receiver after RIB populated.
		if !cfg.PassiveListen {
			receiverConn, err = connectBGP(ctx, cfg.ReceiverAddr, receiverAddr, cfg.ConnectTimeout, false)
			if err != nil {
				return IterationResult{}, fmt.Errorf("connecting receiver: %w", err)
			}

			rcfg := SessionConfig{
				ASN:      uint32(cfg.ReceiverASN), //nolint:gosec // CLI-validated range
				RouterID: mustRouterID(cfg.ReceiverAddr),
				HoldTime: 90,
				Family:   cfg.Family,
			}

			if err := receiverConn.SetDeadline(time.Now().Add(cfg.ConnectTimeout)); err != nil {
				return IterationResult{}, fmt.Errorf("setting receiver deadline: %w", err)
			}

			receiverSetup, err = DoHandshake(receiverConn, rcfg)
			if err != nil {
				return IterationResult{}, fmt.Errorf("receiver handshake: %w", err)
			}

			if err := receiverConn.SetDeadline(time.Time{}); err != nil {
				return IterationResult{}, fmt.Errorf("clearing receiver deadline: %w", err)
			}

			recvBufReader = bufio.NewReaderSize(receiverConn, 65536)
		}

		recvKaWg.Go(func() {
			keepaliveLoop(recvKaCtx, receiverConn)
		})

		sendStart = time.Now()

		// Reset sendTimes to receiver-connect time so per-prefix latencies
		// measure RIB propagation, not the Phase 1 send + warmup overhead.
		for k := range sendTimes {
			sendTimes[k] = sendStart
		}

		recvWg.Go(func() {
			defer close(recvDone)
			rawMsgs, _ = receiveRaw(recvCtx, recvBufReader, receiverConn, len(prefixes), rawMsgs, recvSlab)
		})
	} else {
		// Receiver-first (default): warmup, start receiver, send routes, measure.
		recvBufReader = bufio.NewReaderSize(receiverConn, 65536)

		recvKaWg.Go(func() {
			keepaliveLoop(recvKaCtx, receiverConn)
		})

		if cfg.Warmup > 0 {
			select {
			case <-ctx.Done():
				return IterationResult{}, ctx.Err()
			case <-time.After(cfg.Warmup):
			}
		}

		recvWg.Go(func() {
			defer close(recvDone)
			rawMsgs, _ = receiveRaw(recvCtx, recvBufReader, receiverConn, len(prefixes), rawMsgs, recvSlab)
		})

		sendStart = time.Now()

		if err := writeSendBatches(sendBufWriter, prebuilt, prefixes, batches, sendTimes); err != nil {
			return IterationResult{}, fmt.Errorf("sending batches: %w", err)
		}
		if err := sendBufWriter.Flush(); err != nil {
			return IterationResult{}, fmt.Errorf("flushing sender: %w", err)
		}

		senderKaWg.Go(func() {
			keepaliveLoop(senderKaCtx, senderConn)
		})
	}

	// Wait for convergence (receiver idle) or timeout.
	deadline := time.NewTimer(cfg.Duration)
	defer deadline.Stop()

	select {
	case <-recvDone:
		// Receiver went idle -- all updates arrived.
	case <-deadline.C:
		// Timeout -- proceed with partial results.
	case <-ctx.Done():
		return IterationResult{}, ctx.Err()
	}

	// Stop receiver.
	recvCancel()
	recvWg.Wait()

	// Decode prefixes from buffered raw messages (post-idle, no timing impact).
	recvTimes := make(map[netip.Prefix]time.Time, len(prefixes))
	for _, raw := range rawMsgs {
		body := raw.data[message.HeaderLen:]
		for _, p := range ExtractPrefixes(body) {
			if _, exists := recvTimes[p]; !exists {
				recvTimes[p] = raw.when
			}
		}
	}

	// Compute metrics.
	// Only count prefixes that were both sent and received -- the DUT may
	// advertise its own routes (connected networks, loopback) which would
	// inflate len(recvTimes) beyond cfg.Routes and produce negative RoutesLost.
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

	// Stop sender keepalive before writing withdrawals to avoid concurrent writes.
	senderKaCancel()
	senderKaWg.Wait()

	// Send route withdrawals so the DUT can clean its RIB for the next iteration.
	withdrawalStart := time.Now()
	sendWithdrawals(senderConn, sender, prefixes, batches)

	// Drain forwarded withdrawal UPDATEs using the existing buffered reader.
	wdDrainCtx, wdDrainCancel := context.WithCancel(ctx)
	wdDone := make(chan struct{})
	var wdUpdates int
	var wdDrainWg sync.WaitGroup
	if receiverConn != nil && recvBufReader != nil {
		wdDrainWg.Go(func() {
			defer close(wdDone)
			wdUpdates = drainMessages(wdDrainCtx, recvBufReader, receiverConn)
		})
	} else {
		close(wdDone)
	}

	wdDeadline := time.NewTimer(cfg.Duration)
	defer wdDeadline.Stop()
	select {
	case <-wdDone:
	case <-wdDeadline.C:
	case <-ctx.Done():
	}
	withdrawalMs := 0
	if wdUpdates > 0 {
		withdrawalMs = int(time.Since(withdrawalStart) / time.Millisecond)
	}

	wdDrainCancel()
	wdDrainWg.Wait()

	// Stop receiver keepalive before sending Cease.
	recvKaCancel()
	recvKaWg.Wait()

	// Send NOTIFICATION Cease on both connections.
	cease := BuildCeaseNotification()
	_ = WriteMessage(senderConn, cease)
	if receiverConn != nil {
		_ = WriteMessage(receiverConn, cease)
	}

	// Give 100ms for NOTIFICATION to be sent before closing.
	time.Sleep(100 * time.Millisecond)

	if receiverConn != nil {
		receiverConn.Close() //nolint:errcheck // cleanup before next iteration
	}

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
		WithdrawalMs:      withdrawalMs,
		SessionSenderMs:   int(senderSetup / time.Millisecond),
		SessionReceiverMs: int(receiverSetup / time.Millisecond),
	}, nil
}

// writeSendBatches writes pre-built UPDATE batches to the sender, recording per-prefix send times.
func writeSendBatches(w *bufio.Writer, prebuilt [][]byte, prefixes []netip.Prefix, batches []batchRange, sendTimes map[netip.Prefix]time.Time) error {
	for i, br := range batches {
		if _, err := w.Write(prebuilt[i]); err != nil {
			return err
		}
		now := time.Now()
		for _, prefix := range prefixes[br.start:br.end] {
			sendTimes[prefix] = now
		}
	}
	return nil
}

// drainMessages reads and discards BGP messages until the connection goes idle
// (no UPDATE messages for 500ms) or ctx is canceled. Returns the number of
// UPDATE messages drained.
func drainMessages(ctx context.Context, r io.Reader, conn net.Conn) int {
	hdr := make([]byte, message.HeaderLen)
	discard := make([]byte, message.MaxMsgLen)
	updates := 0
	for {
		if err := ctx.Err(); err != nil {
			return updates
		}
		if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
			return updates
		}
		if _, err := io.ReadFull(r, hdr); err != nil {
			return updates
		}
		msgType := msgtype.MessageType(hdr[18])
		msgLen := int(binary.BigEndian.Uint16(hdr[16:18]))
		if msgLen < message.HeaderLen {
			return updates
		}
		bodyLen := msgLen - message.HeaderLen
		if bodyLen > 0 && bodyLen <= len(discard) {
			if _, err := io.ReadFull(r, discard[:bodyLen]); err != nil {
				return updates
			}
		}
		if msgType == msgtype.TypeUPDATE {
			updates++
		}
	}
}

// sendWithdrawals sends route withdrawals for all prefixes to clear the DUT's RIB.
// Best-effort: errors are ignored since this is cleanup before session teardown.
func sendWithdrawals(conn net.Conn, sender *Sender, prefixes []netip.Prefix, batches []batchRange) {
	w := bufio.NewWriterSize(conn, 16384)
	for _, br := range batches {
		wd := sender.BuildWithdrawBatch(prefixes[br.start:br.end])
		if wd == nil {
			continue
		}
		if _, err := w.Write(wd); err != nil {
			return
		}
	}
	if err := w.Flush(); err != nil {
		return
	}
}

// connectBGP establishes a BGP TCP connection.
//
// When passiveListen is false, ze-perf dials out to remoteAddr from localAddr.
// When passiveListen is true, ze-perf listens on localAddr:179 and waits for
// the DUT to initiate the session. This matches the CLI flag semantics and
// avoids opening a competing outbound connection that can trigger collision
// handling in implementations that actively connect to their peers.
func connectBGP(ctx context.Context, localAddr, remoteAddr string, timeout time.Duration, passiveListen bool) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Resolve local address for dialing.
	local, err := net.ResolveTCPAddr("tcp", localAddr+":0")
	if err != nil {
		return nil, fmt.Errorf("resolving local address %s: %w", localAddr, err)
	}

	// When passive listen is disabled, use dial-only path.
	if !passiveListen {
		dialer := net.Dialer{LocalAddr: local, Timeout: timeout}

		conn, dialErr := dialer.DialContext(ctx, "tcp", remoteAddr)
		if dialErr != nil {
			return nil, fmt.Errorf("dialing %s: %w", remoteAddr, dialErr)
		}

		return tuneTCP(conn)
	}

	// Passive-listen mode waits for the DUT to connect to us on the well-known
	// BGP port. This requires root or CAP_NET_BIND_SERVICE.
	listenAddr := net.JoinHostPort(localAddr, "179")

	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", listenAddr, err)
	}
	defer listener.Close() //nolint:errcheck // cleanup

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	inConn, err := listener.Accept()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("accepting inbound from DUT: %w", ctx.Err())
		}
		return nil, fmt.Errorf("accepting inbound from DUT: %w", err)
	}

	return tuneTCP(inConn)
}

// tuneTCP applies the same socket options ze uses in production:
// - TCP_NODELAY: BGP messages are application-framed, Nagle only adds latency.
// - IP_TOS/IPV6_TCLASS = 0xC0 (DSCP CS6, Internet Control): RFC 4271 S5.1.
func tuneTCP(conn net.Conn) (net.Conn, error) {
	tc, ok := conn.(*net.TCPConn)
	if !ok {
		return conn, nil
	}

	if err := tc.SetNoDelay(true); err != nil {
		return nil, fmt.Errorf("setting TCP_NODELAY: %w", err)
	}

	// Best-effort DSCP CS6 marking. Failures are non-fatal: some containers
	// and OS configurations restrict setsockopt for TOS/TCLASS.
	if raw, err := tc.SyscallConn(); err == nil {
		_ = raw.Control(func(fd uintptr) {
			if addr, ok := tc.RemoteAddr().(*net.TCPAddr); ok {
				if addr.IP.To4() != nil {
					_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TOS, 0xC0)
				} else {
					_ = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_TCLASS, 0xC0)
				}
			}
		})
	}

	return conn, nil
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

// receiveRaw reads BGP messages from r, buffering UPDATE messages with their
// receive timestamps. Only cheap prefix counting (no netip.Prefix construction,
// no map, no mutex) is done for convergence detection. Full prefix extraction
// happens after this function returns.
//
// Messages are read into the pre-allocated slab for cache locality. If the slab
// is exhausted, falls back to heap allocation transparently. Slab offset advances
// forward-only, so message sub-slices never alias each other.
//
// Assumes ADD-PATH (RFC 7911) is not negotiated; prefix counts will be incorrect
// if the DUT sends path IDs.
//
// Returns msgs and nil on successful convergence or context cancellation.
// Returns msgs and a non-nil error on connection/protocol failure (partial results).
// The conn parameter is used only for setting read deadlines; all reads go
// through the buffered reader r.
func receiveRaw(
	ctx context.Context, r io.Reader, conn net.Conn,
	expectedPrefixes int, msgs []rawMessage, slab []byte,
) ([]rawMessage, error) {
	prefixCount := 0
	hdr := make([]byte, message.HeaderLen) // Reused across all reads.
	slabOff := 0

	for {
		if err := ctx.Err(); err != nil {
			return msgs, err
		}

		// Short deadline once all expected prefixes arrived (drain stragglers).
		// Normal deadline otherwise (cancellation checking).
		timeout := 200 * time.Millisecond
		if prefixCount >= expectedPrefixes {
			timeout = 100 * time.Millisecond
		}

		_ = conn.SetReadDeadline(time.Now().Add(timeout))

		var msgType msgtype.MessageType
		var msg []byte
		var err error

		msgType, msg, slabOff, err = readMessageSlab(r, hdr, slab, slabOff)
		if err != nil {
			if isNetTimeout(err) {
				if prefixCount >= expectedPrefixes {
					return msgs, nil // All expected prefixes collected.
				}

				continue
			}

			return msgs, fmt.Errorf("receive: %w", err)
		}

		if msgType != msgtype.TypeUPDATE {
			continue
		}

		msgs = append(msgs, rawMessage{data: msg, when: time.Now()})
		prefixCount += CountPrefixes(msg[message.HeaderLen:])
	}
}

// readMessageSlab reads a BGP message into the slab at the given offset.
// If the slab has insufficient space, falls back to heap allocation.
// Returns the message type, message bytes, new slab offset, and any error.
func readMessageSlab(
	r io.Reader, hdr []byte, slab []byte, slabOff int,
) (msgtype.MessageType, []byte, int, error) {
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, slabOff, fmt.Errorf("reading header: %w", err)
	}

	msgLen := int(binary.BigEndian.Uint16(hdr[16:18]))
	if msgLen < message.HeaderLen {
		return 0, nil, slabOff, fmt.Errorf("invalid message length: %d", msgLen)
	}

	if msgLen > message.MaxMsgLen {
		return 0, nil, slabOff, fmt.Errorf("message length %d exceeds RFC 4271 limit %d", msgLen, message.MaxMsgLen)
	}

	// Try slab; fall back to heap if exhausted.
	var msg []byte
	if slabOff+msgLen <= len(slab) {
		msg = slab[slabOff : slabOff+msgLen]
		slabOff += msgLen
	} else {
		msg = make([]byte, msgLen)
	}

	copy(msg[:message.HeaderLen], hdr)

	if msgLen > message.HeaderLen {
		if _, err := io.ReadFull(r, msg[message.HeaderLen:]); err != nil {
			return 0, nil, slabOff, fmt.Errorf("reading body: %w", err)
		}
	}

	return msgtype.MessageType(hdr[18]), msg, slabOff, nil
}

// isNetTimeout reports whether err is a network timeout error.
func isNetTimeout(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return ne.Timeout()
	}

	return false
}

// parseAddr parses an IP address string, returning the zero address on failure.
func parseAddr(s string) netip.Addr {
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
