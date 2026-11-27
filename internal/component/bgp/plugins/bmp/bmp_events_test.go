// Design: docs/guide/bmp.md -- BMP sender bounce replay-abort and delivery error reporting
// Related: bmp_events.go -- bounceMonitoredPeers and handleSender* write failure paths
// Related: bmp_locrib.go -- Loc-RIB Peer Up / EOR / Peer Down failure paths
// Related: metrics.go -- ze_bmp_message_drops_total

package bmp

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

type bmpTestPublisher struct {
	mu     sync.Mutex
	raises []sdk.Issue
}

func (p *bmpTestPublisher) RaiseIssue(_ context.Context, issue sdk.Issue) error {
	p.mu.Lock()
	p.raises = append(p.raises, issue)
	p.mu.Unlock()
	return nil
}

func (p *bmpTestPublisher) ClearIssue(_ context.Context, _, _ string) error {
	return nil
}

type bmpSyncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *bmpSyncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *bmpSyncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

type bmpTestCounter struct {
	mu  sync.Mutex
	val float64
}

func (c *bmpTestCounter) Inc() {
	c.mu.Lock()
	c.val++
	c.mu.Unlock()
}

func (c *bmpTestCounter) Add(v float64) {
	c.mu.Lock()
	c.val += v
	c.mu.Unlock()
}

type bmpTestCounterVec struct {
	mu     sync.Mutex
	counts map[string]*bmpTestCounter
}

func (v *bmpTestCounterVec) With(labels ...string) metrics.Counter {
	key := strings.Join(labels, "|")
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.counts == nil {
		v.counts = make(map[string]*bmpTestCounter)
	}
	c, ok := v.counts[key]
	if !ok {
		c = &bmpTestCounter{}
		v.counts[key] = c
	}
	return c
}

func (v *bmpTestCounterVec) Delete(labels ...string) bool {
	key := strings.Join(labels, "|")
	v.mu.Lock()
	defer v.mu.Unlock()
	_, ok := v.counts[key]
	delete(v.counts, key)
	return ok
}

func (v *bmpTestCounterVec) total() float64 {
	v.mu.Lock()
	defer v.mu.Unlock()
	var sum float64
	for _, c := range v.counts {
		c.mu.Lock()
		sum += c.val
		c.mu.Unlock()
	}
	return sum
}

func (v *bmpTestCounterVec) value(labels ...string) float64 {
	key := strings.Join(labels, "|")
	v.mu.Lock()
	defer v.mu.Unlock()
	c, ok := v.counts[key]
	if !ok {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.val
}

type bmpTestMetrics struct {
	metrics.NopRegistry
	mu   sync.Mutex
	vecs map[string]*bmpTestCounterVec
}

func (m *bmpTestMetrics) CounterVec(name, _ string, _ []string) metrics.CounterVec {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.vecs == nil {
		m.vecs = make(map[string]*bmpTestCounterVec)
	}
	vec := &bmpTestCounterVec{}
	m.vecs[name] = vec
	return vec
}

func bmpEventPeerHeader(lastOctet byte) PeerHeader {
	var addr [16]byte
	addr[12], addr[13], addr[14], addr[15] = 192, 0, 2, lastOctet
	return PeerHeader{
		PeerType:  PeerTypeGlobal,
		Address:   addr,
		PeerAS:    65001,
		PeerBGPID: 0x01020300 | uint32(lastOctet),
	}
}

// TestBounceMonitoredPeersAbortsReplayOnPeerDownError verifies that
// bounceMonitoredPeers aborts the Adj-RIB replay and reports the failure when
// writePeerDownLocked fails with a non-disconnected write error, and counts a
// single drop without aborting replay when the collector is disconnected.
//
// VALIDATES: AC-3, AC-4 (bmp_events.go bounceMonitoredPeers RFC 8671 Section 7.2).
// PREVENTS: a failed Peer Down continuing into Peer Up or next peers with corrupt state.
func TestBounceMonitoredPeersAbortsReplayOnPeerDownError(t *testing.T) {
	t.Run("write_error_aborts_and_reports", func(t *testing.T) {
		reg := &bmpTestMetrics{}
		SetMetricsRegistry(reg)
		t.Cleanup(func() { SetMetricsRegistry(nil) })

		logs := &bmpSyncWriter{}
		prevLogger := logger()
		setLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
		t.Cleanup(func() { setLogger(prevLogger) })

		pub := &bmpTestPublisher{}
		q := issuequeue.Start(pub)
		t.Cleanup(q.Stop)

		conn := newRecordingConn()
		ss := newTestSession(t, "bounce-overflow", conn)
		// Smaller than a single Peer Down frame so enqueueLocked returns errQueueOverflow.
		ss.txLimit = 10

		bp := &BMPPlugin{
			issues: q,
			peerUps: map[string]*peerUpState{
				"192.0.2.1": {peer: bmpEventPeerHeader(1)},
				"192.0.2.2": {peer: bmpEventPeerHeader(2)},
			},
			dedupState: make(map[string]map[uint64]struct{}),
			dedupCount: make(map[string]uint32),
			stopCh:     make(chan struct{}),
		}

		bp.bounceMonitoredPeers([]*senderSession{ss})
		q.Flush()

		drops := reg.vecs[metricBMPMessageDrops]
		require.NotNil(t, drops, "ze_bmp_message_drops_total must be registered")
		assert.Equal(t, float64(1), drops.total(), "bounce must break on first Peer Down write error rather than continuing to second peer")

		out := logs.String()
		assert.Contains(t, out, "level=WARN")
		assert.Contains(t, out, "code="+codeDoctorBMPSenderDeliveryFailed)
		assert.Contains(t, out, "bounce-overflow")
		assert.Contains(t, out, fixShowBMPCollectors)
		assert.Contains(t, out, "report=queued")

		pub.mu.Lock()
		raises := append([]sdk.Issue(nil), pub.raises...)
		pub.mu.Unlock()
		require.Len(t, raises, 1)
		assert.Equal(t, codeDoctorBMPSenderDeliveryFailed, raises[0].Code)
		assert.Equal(t, rpc.ReportError, raises[0].Severity)
		assert.Equal(t, fixShowBMPCollectors, raises[0].Fix)
	})

	t.Run("disconnected_counts_drop_and_breaks", func(t *testing.T) {
		reg := &bmpTestMetrics{}
		SetMetricsRegistry(reg)
		t.Cleanup(func() { SetMetricsRegistry(nil) })

		ss := newTestSession(t, "bounce-disconnected", nil)
		bp := &BMPPlugin{
			peerUps: map[string]*peerUpState{
				"192.0.2.1": {peer: bmpEventPeerHeader(1)},
				"192.0.2.2": {peer: bmpEventPeerHeader(2)},
			},
			dedupState: make(map[string]map[uint64]struct{}),
			dedupCount: make(map[string]uint32),
			stopCh:     make(chan struct{}),
		}

		bp.bounceMonitoredPeers([]*senderSession{ss})

		drops := reg.vecs[metricBMPMessageDrops]
		require.NotNil(t, drops, "ze_bmp_message_drops_total must be registered")
		assert.Equal(t, float64(1), drops.total(), "disconnected bounce must count once and break")
	})
}

// TestBMPSenderDeliveryFailureWarnsCountsAndRateLimits verifies that BMP sender
// delivery failures across state, mirror, update, Loc-RIB, and statistics paths
// increment ze_bmp_message_drops_total, emit rate-limited WARNs with
// doctor-bmp-sender-delivery-failed, preserve locRIBUpSent reset, and stay
// silent during plugin shutdown.
//
// VALIDATES: AC-2, AC-4 (bmp_events.go, bmp_locrib.go, sender_drain.go, statistics.go).
// PREVENTS: collector write failures vanishing below WARN or flooding stderr.
func TestBMPSenderDeliveryFailureWarnsCountsAndRateLimits(t *testing.T) {
	reg := &bmpTestMetrics{}
	SetMetricsRegistry(reg)
	t.Cleanup(func() { SetMetricsRegistry(nil) })

	logs := &bmpSyncWriter{}
	prevLogger := logger()
	setLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { setLogger(prevLogger) })

	pub := &bmpTestPublisher{}
	q := issuequeue.Start(pub)
	t.Cleanup(q.Stop)

	ss := newTestSession(t, "down-collector", nil)
	peerHdr := bmpEventPeerHeader(10)
	bp := &BMPPlugin{
		issues:   q,
		senders:  []*senderSession{ss},
		identity: &localIdentity{asn: 65000, routerID: 0x01020304},
		peerUps: map[string]*peerUpState{
			"192.0.2.10": {peer: peerHdr},
		},
		dedupState: make(map[string]map[uint64]struct{}),
		dedupCount: map[string]uint32{"192.0.2.10": 3},
		stopCh:     make(chan struct{}),
	}

	// Peer Down twice to check rate-limiting per (collector, message).
	downEv := &rpc.StructuredEvent{PeerAddress: "192.0.2.10", State: rpc.SessionStateDown}
	cause := peerDownCause{peer: peerHdr, reason: PeerDownLocalNoNotify, reported: true}
	bp.handleSenderState(downEv, []*senderSession{ss}, cause)
	bp.handleSenderState(downEv, []*senderSession{ss}, cause)

	// Route Mirroring failure.
	mirrorEv := &rpc.StructuredEvent{
		PeerAddress: "192.0.2.10",
		PeerAS:      rpc.RemoteASOf(65001),
		RawMessage:  &bgptypes.RawMessage{Type: msgtype.TypeKEEPALIVE},
	}
	bp.handleSenderMirror(mirrorEv, []*senderSession{ss})

	// Loc-RIB Peer Up failure must reset locRIBUpSent to false and report.
	bp.ensureLocRIBPeerUp([]*senderSession{ss})
	assert.False(t, ss.locRIBUpSent.Load(), "locRIBUpSent must stay reset after write failure")

	// Loc-RIB End-of-RIB failure.
	bp.sendLocRIBEndOfRIB([]*senderSession{ss}, family.IPv4Unicast, peerHdr)

	// Statistics Report failure.
	bp.sendStatisticsReports()

	q.Flush()

	drops := reg.vecs[metricBMPMessageDrops]
	require.NotNil(t, drops)
	assert.GreaterOrEqual(t, drops.total(), float64(6), "all 6 failed sender writes must increment ze_bmp_message_drops_total")

	out := logs.String()
	assert.Contains(t, out, "level=WARN")
	assert.Contains(t, out, "code="+codeDoctorBMPSenderDeliveryFailed)
	assert.Contains(t, out, fixShowBMPCollectors)
	assert.Contains(t, out, "report=queued")

	pub.mu.Lock()
	raisesBeforeStop := len(pub.raises)
	pub.mu.Unlock()
	// 5 distinct message kinds (peer-down rate-limited from 2 to 1, route-mirroring, loc-rib-peer-up, loc-rib-eor, statistics).
	assert.Equal(t, 5, raisesBeforeStop, "expected 1 rate-limited raise per distinct message kind")

	// During plugin shutdown (stopCh closed), sendLocRIBPeerDown must stay silent.
	bp.locRIBUp = true
	close(bp.stopCh)
	bp.sendLocRIBPeerDown()
	q.Flush()

	pub.mu.Lock()
	raisesAfterStop := len(pub.raises)
	pub.mu.Unlock()
	assert.Equal(t, raisesBeforeStop, raisesAfterStop, "shutdown sendLocRIBPeerDown must not raise spurious doctor warnings")
}

// TestBMPRateLimitMapsBoundedAndPruned verifies that BMP's receiver inject and
// sender delivery rate-limit maps enforce their hard caps (1024 and 256) and
// prune entries on peer down, router disconnect, and collector removal.
//
// VALIDATES: AC-4 and Boundary Tests (receiver cap 1024, sender cap 256).
// PREVENTS: unbounded memory growth under peer or collector churn.
func TestBMPRateLimitMapsBoundedAndPruned(t *testing.T) {
	bp := &BMPPlugin{stopCh: make(chan struct{})}

	for i := range receiverInjectWarnCap + 25 {
		key := "198.51.100.1:11019:192.0.2." + strconv.Itoa(i)
		assert.True(t, bp.allowReceiverInjectWarn(key))
	}
	bp.warnMu.Lock()
	assert.LessOrEqual(t, len(bp.receiverInjectWarned), receiverInjectWarnCap)
	bp.warnMu.Unlock()

	bp.pruneReceiverInjectWarnPeer("198.51.100.1:11019:192.0.2.1000")
	assert.True(t, bp.allowReceiverInjectWarn("198.51.100.1:11019:192.0.2.1000"))

	bp.pruneReceiverInjectWarnRouter("198.51.100.1:11019")
	bp.warnMu.Lock()
	assert.Empty(t, bp.receiverInjectWarned)
	bp.warnMu.Unlock()

	for i := range senderWarnCap + 25 {
		assert.True(t, bp.allowSenderDeliveryWarn("col-"+strconv.Itoa(i), bmpMsgKindRouteMonitoring))
	}
	bp.warnMu.Lock()
	assert.LessOrEqual(t, len(bp.senderDeliveryWarned), senderWarnCap)
	bp.warnMu.Unlock()

	assert.True(t, bp.allowSenderDeliveryWarn("prune-me", bmpMsgKindPeerUp))
	assert.False(t, bp.allowSenderDeliveryWarn("prune-me", bmpMsgKindPeerUp))
	bp.pruneSenderDeliveryWarn("prune-me")
	assert.True(t, bp.allowSenderDeliveryWarn("prune-me", bmpMsgKindPeerUp))

	bp.clearSenderDeliveryWarn()
	bp.warnMu.Lock()
	assert.Empty(t, bp.senderDeliveryWarned)
	bp.warnMu.Unlock()
}

// TestBMPDrainLoopWriteFailureReportsAndResets verifies that a socket write
// error in senderSession.drainLoop clears the connection, resets the queue,
// and reports through reportWriteFailure -> bp.reportSenderWriteFailure.
//
// VALIDATES: AC-2, AC-4 (sender_drain.go drainLoop site #21).
// PREVENTS: drain socket write failures vanishing below WARN.
func TestBMPDrainLoopWriteFailureReportsAndResets(t *testing.T) {
	reg := &bmpTestMetrics{}
	SetMetricsRegistry(reg)
	t.Cleanup(func() { SetMetricsRegistry(nil) })

	logs := &bmpSyncWriter{}
	prevLogger := logger()
	setLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { setLogger(prevLogger) })

	pub := &bmpTestPublisher{}
	q := issuequeue.Start(pub)
	t.Cleanup(q.Stop)

	bp := &BMPPlugin{
		issues: q,
		stopCh: make(chan struct{}),
	}

	c1, c2 := net.Pipe()
	_ = c1.Close()
	_ = c2.Close()

	ss := newTestSession(t, "drain-fail", c1)
	reported := make(chan struct{}, 1)
	ss.reportWriteFailure = func(collector, message string, err error) {
		bp.reportSenderWriteFailure(collector, message, err)
		select {
		case reported <- struct{}{}:
		default:
		}
	}

	// Enqueue one frame; drainLoop wakes, hits write error on closed c1, resets, and reports.
	ss.writeMu.Lock()
	err := ss.enqueueLocked([]byte{Version, 0, 0, 0, 6, MsgInitiation})
	ss.writeMu.Unlock()
	require.NoError(t, err)

	select {
	case <-reported:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for drainLoop to report write failure")
	}

	ss.stop()
	ss.waitDrain()
	q.Flush()

	drops := reg.vecs[metricBMPMessageDrops]
	require.NotNil(t, drops)
	assert.Equal(t, float64(1), drops.value("drain-fail", bmpMsgKindDrain, dropReasonWriteError))

	out := logs.String()
	assert.Contains(t, out, "level=WARN")
	assert.Contains(t, out, "code="+codeDoctorBMPSenderDeliveryFailed)
	assert.Contains(t, out, "drain-fail")
	assert.Contains(t, out, fixShowBMPCollectors)

	pub.mu.Lock()
	raises := append([]sdk.Issue(nil), pub.raises...)
	pub.mu.Unlock()
	require.Len(t, raises, 1)
	assert.Equal(t, codeDoctorBMPSenderDeliveryFailed, raises[0].Code)
}
