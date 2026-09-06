// Tests for the periodic BMP Statistics Report the `statistics-timeout` leaf
// configures.
//
// Related: statistics.go -- the ticker, the counters and the per-peer reports
// Related: bmp_events.go -- duplicateUpdate, which measures Stat Type 13
// Related: rfc8671_test.go -- startReloadEngine and liveCollectorSession, the
// config rail these tests reach the plugin through

package bmp

import (
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// statisticsUpdateBody is one UPDATE body, as a reactor event carries it: the
// bytes after the 19-byte BGP header. The dedup detector hashes exactly these,
// so two events carrying it are the duplicate pair RFC 7854 Stat Type 13 counts.
var statisticsUpdateBody = []byte{
	0x00, 0x00, // withdrawn routes length
	0x00, 0x14, // total path attribute length
	0x40, 0x01, 0x01, 0x00, // ORIGIN igp
	0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9, // AS_PATH 65001
	0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP 10.0.0.1
	0x18, 0x0a, 0x14, 0x1e, // 10.20.30.0/24
}

// statisticsUpdateOther is a second UPDATE body that differs from
// statisticsUpdateBody only in its NLRI, so a detector that hashed nothing and
// answered "duplicate" for every event fails the tests that use both.
var statisticsUpdateOther = []byte{
	0x00, 0x00,
	0x00, 0x14,
	0x40, 0x01, 0x01, 0x00,
	0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xfd, 0xe9,
	0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01,
	0x18, 0x0a, 0x14, 0x1f, // 10.20.31.0/24
}

// statisticsPeer is the monitored BGP peer every test here reports on, and the
// address updateEvent puts on the events it builds.
const statisticsPeer = "10.0.0.1"

// statisticsPlugin builds a plugin holding one established peer, one collector
// session writing into conn, and a running statistics configuration.
func statisticsPlugin(conn net.Conn, policy string, interval time.Duration) *BMPPlugin {
	bp := newPipeSender(conn, false)
	bp.routeMonitorPolicy = policy
	bp.statisticsInterval = interval
	bp.peerUps = map[string]*peerUpState{statisticsPeer: establishedPeer(statisticsPeer, 65001)}
	return bp
}

// duplicateCount reports the Stat Type 13 counter ze has measured for the
// monitored peer.
func duplicateCount(bp *BMPPlugin) uint32 {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	return bp.dedupCount[statisticsPeer]
}

// statCounter reads the 32-bit Counter carried for typ, and reports whether the
// report carried that type at all.
func statCounter(sr *statisticsReport, typ uint16) (uint32, bool) {
	for _, entry := range sr.Stats {
		if entry.Type != typ {
			continue
		}
		if len(entry.Value) != statCounterSize {
			return 0, false
		}
		return binary.BigEndian.Uint32(entry.Value), true
	}
	return 0, false
}

// awaitStatisticsReports reads want Statistics Reports off the collector socket,
// stepping over the Peer Down and Peer Up a configuration change owes the
// collector first, and reports how long the LAST one took to arrive after the
// one before it.
//
// The budget is what makes an absent report a failure rather than a hang, and
// the returned gap is what makes the reports PERIODIC rather than one report
// repeated by the test's own reads.
func awaitStatisticsReports(t *testing.T, conn net.Conn, want int, budget time.Duration) []time.Duration {
	t.Helper()

	deadline := time.After(budget)
	gaps := make([]time.Duration, 0, want)
	last := time.Now()
	for len(gaps) < want {
		select {
		case got := <-asyncRead(conn):
			if got.err != nil {
				t.Fatalf("the collector session was closed after %d reports: %v", len(gaps), got.err)
			}
			switch message := got.msg.(type) {
			case *statisticsReport:
				gaps = append(gaps, time.Since(last))
				last = time.Now()
			case *PeerDown, *PeerUp:
				// The bounce RFC 8671 Section 7.2 owes a behavior change.
			default:
				t.Fatalf("the collector was sent a %T, want a Statistics Report", message)
			}
		case <-deadline:
			t.Fatalf("only %d of %d Statistics Reports reached the collector in %s", len(gaps), want, budget)
		}
	}
	return gaps
}

// TestBMPDuplicateUpdateCountsReceivedRepeatsOnly holds duplicateUpdate to the
// wording of the counter it feeds. RFC 7854 Section 4.8: "Stat Type = 13:
// (32-bit Counter) Number of duplicate update messages received."
//
// Three properties, and the counter is wrong without each of them: a repeat in
// the received direction counts, a body seen once does not, and a repeat in the
// SENT direction does not, because a body ze advertised is not one ze received.
// VALIDATES: AC-4 -- a repeat RECEIVED counts, a repeat SENT does not.
// PREVENTS: a counter that reports repeats in either direction.
func TestBMPDuplicateUpdateCountsReceivedRepeatsOnly(t *testing.T) {
	bp := &BMPPlugin{dedupState: make(map[string]map[uint64]struct{}), stopCh: make(chan struct{})}

	if bp.duplicateUpdate(statisticsPeer, true, statisticsUpdateBody) {
		t.Fatal("the first received UPDATE was read as a duplicate")
	}
	if count := duplicateCount(bp); count != 0 {
		t.Errorf("counter = %d after one received UPDATE, want 0", count)
	}

	if !bp.duplicateUpdate(statisticsPeer, true, statisticsUpdateBody) {
		t.Fatal("the same body received twice was not read as a duplicate")
	}
	if count := duplicateCount(bp); count != 1 {
		t.Errorf("counter = %d after one repeat, want 1", count)
	}

	// A different body is not a repeat, so a detector that answered
	// "duplicate" for everything fails here rather than passing the line above.
	if bp.duplicateUpdate(statisticsPeer, true, statisticsUpdateOther) {
		t.Error("a body with different NLRI was read as a duplicate")
	}
	if count := duplicateCount(bp); count != 1 {
		t.Errorf("counter = %d after a distinct body, want 1", count)
	}

	// The SENT direction keeps its own hash set, so the body ze already
	// RECEIVED is new on the way out, and its repeat counts nothing.
	if bp.duplicateUpdate(statisticsPeer, false, statisticsUpdateBody) {
		t.Error("a body ze received was read as a duplicate of itself on the way out")
	}
	if !bp.duplicateUpdate(statisticsPeer, false, statisticsUpdateBody) {
		t.Error("the same body sent twice was not read as a duplicate")
	}
	if count := duplicateCount(bp); count != 1 {
		t.Errorf("counter = %d after two sent bodies, want 1: a sent repeat is not one ze received", count)
	}
}

// TestBMPStatisticsMeasuresUnderAPolicyThatStreamsNothing pins the reason
// handleSenderUpdate is called for a direction the route-monitoring policy
// filters out: the counter is measured there, and a counter that stopped being
// measured under `post-policy` would report a zero ze never took
// (ai/rules/principles.md).
//
// Nothing reaches the collector either way, which is the second half of it: the
// policy still decides what is STREAMED.
// VALIDATES: AC-6 -- the detector runs whatever the policy is, so it measures.
// PREVENTS: a report of zero duplicates ze never measured.
func TestBMPStatisticsMeasuresUnderAPolicyThatStreamsNothing(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := statisticsPlugin(client, policyPostPolicy, time.Minute)

	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))
	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))

	if count := duplicateCount(bp); count != 1 {
		t.Errorf("counter = %d, want 1: post-policy must still measure the received direction", count)
	}
	requireCollectorSilent(t, asyncRead(server), "post-policy received UPDATE")
}

// TestBMPStatisticsOffMeasuresNothingUnderAPolicyThatStreamsNothing is the
// negative of the test above: with no periodic report configured there is no
// counter to feed, so a received UPDATE the policy does not stream is not
// hashed at all.
// VALIDATES: AC-6 -- under `all` the received direction is streamed AND measured.
// PREVENTS: a detector wired only to the direction the policy filters out.
func TestBMPStatisticsMeasuresTheDirectionThePolicyAlsoStreams(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := statisticsPlugin(client, policyAll, time.Minute)

	// Both events on their own goroutine: the first writes Route Monitoring
	// into an unbuffered pipe, so it completes only once the read below drains
	// it. The second is the duplicate, and it writes nothing.
	done := make(chan struct{})
	go func() {
		defer close(done)
		bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))
		bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))
	}()

	got := <-asyncRead(server)
	if got.err != nil {
		t.Fatalf("read the Route Monitoring: %v", got.err)
	}
	if _, ok := got.msg.(*RouteMonitoring); !ok {
		t.Fatalf("the collector was sent a %T, want Route Monitoring", got.msg)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the second UPDATE was still being processed: it must be suppressed, not written")
	}
	if count := duplicateCount(bp); count != 1 {
		t.Errorf("counter = %d, want 1: the streamed direction is measured too", count)
	}
	requireCollectorSilent(t, asyncRead(server), "after the duplicate UPDATE")
}

// VALIDATES: AC-6 negative -- with no report configured, nothing is hashed.
// PREVENTS: hashing every received UPDATE for a counter nobody reads.
func TestBMPStatisticsOffMeasuresNothingUnderAPolicyThatStreamsNothing(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := statisticsPlugin(client, policyPostPolicy, 0)

	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))
	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))

	bp.mu.RLock()
	tracked := len(bp.dedupState)
	bp.mu.RUnlock()
	if tracked != 0 {
		t.Errorf("%d peers were hashed with the periodic report off, want 0", tracked)
	}
	requireCollectorSilent(t, asyncRead(server), "post-policy received UPDATE with statistics off")
}

// TestBMPStatisticsReportCarriesTheDuplicateCounter reads one round of reports
// off the collector socket and holds it to what RFC 7854 Section 4.8 defines.
//
// One report per established peer, carrying Stat Type 13 as a 4-byte counter
// whose value is the number of duplicates ze measured. Section 4.8: "SR
// messages are optional. However, if an SR message is transmitted, at least one
// statistic MUST be carried in it." The counter is that one statistic.
// VALIDATES: AC-3 -- one report per peer, Stat Type 13, O flag cleared.
// PREVENTS: an empty Statistics Report, and one carrying the O flag.
func TestBMPStatisticsReportCarriesTheDuplicateCounter(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := statisticsPlugin(client, policyPostPolicy, time.Minute)
	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))
	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))
	bp.handleStructuredEvent(updateEvent(rpc.DirectionReceived, statisticsUpdateBody))

	result := asyncRead(server)
	bp.sendStatisticsReports()

	got := <-result
	if got.err != nil {
		t.Fatalf("read the Statistics Report: %v", got.err)
	}
	sr, ok := got.msg.(*statisticsReport)
	if !ok {
		t.Fatalf("the collector was sent a %T, want a Statistics Report", got.msg)
	}
	if len(sr.Stats) == 0 {
		t.Fatal("the Statistics Report carried no statistic")
	}
	if address := peerAddressString(sr.Peer); address != statisticsPeer {
		t.Errorf("the report names peer %q, want %s", address, statisticsPeer)
	}
	// RFC 8671 Section 6.2: "Statistics report messages are not specific to
	// Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero."
	if sr.Peer.Flags&PeerFlagO != 0 {
		t.Errorf("per-peer flags = %#x, the O flag must be zero on a Statistics Report", sr.Peer.Flags)
	}
	count, carried := statCounter(sr, statTypeDuplicateUpdates)
	if !carried {
		t.Fatalf("the report carries no 4-byte Stat Type %d", statTypeDuplicateUpdates)
	}
	if count != 2 {
		t.Errorf("Stat Type %d = %d, want 2", statTypeDuplicateUpdates, count)
	}
}

// TestBMPStatisticsTimeoutRunsOneTickerAtATime holds setStatisticsTimeout to
// the promise its own doc comment makes: exactly one ticker is live, a reload
// that moves the interval replaces it, and zero leaves none behind.
//
// A second live ticker would send a collector two report streams at two
// intervals, which no reader of the configuration would expect.
// VALIDATES: AC-5 -- one report stream per configuration, replaced on a move.
// PREVENTS: two tickers, and a ticker that outlives the leaf that started it.
func TestBMPStatisticsTimeoutRunsOneTickerAtATime(t *testing.T) {
	bp := &BMPPlugin{dedupState: make(map[string]map[uint64]struct{}), stopCh: make(chan struct{})}
	t.Cleanup(func() {
		close(bp.stopCh)
		bp.sessions.Wait()
	})

	bp.setStatisticsTimeout(0)
	bp.mu.RLock()
	stop := bp.statisticsStop
	bp.mu.RUnlock()
	if stop != nil {
		t.Fatal("a zero timeout started a ticker")
	}

	bp.setStatisticsTimeout(30 * time.Second)
	bp.mu.RLock()
	first := bp.statisticsStop
	interval := bp.statisticsInterval
	bp.mu.RUnlock()
	if first == nil {
		t.Fatal("a nonzero timeout started no ticker")
	}
	if interval != 30*time.Second {
		t.Errorf("interval = %s, want 30s", interval)
	}

	bp.setStatisticsTimeout(60 * time.Second)
	bp.mu.RLock()
	second := bp.statisticsStop
	bp.mu.RUnlock()
	if second == nil || second == first {
		t.Fatal("moving the interval did not start a ticker of its own")
	}
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("the ticker the reload replaced was left running")
	}

	bp.setStatisticsTimeout(0)
	bp.mu.RLock()
	stop = bp.statisticsStop
	bp.mu.RUnlock()
	if stop != nil {
		t.Fatal("a zero timeout left a ticker installed")
	}
	select {
	case <-second:
	case <-time.After(time.Second):
		t.Fatal("a zero timeout did not stop the running ticker")
	}
}

// TestBMPStatisticsTimeoutParsesEveryValueTheLeafAccepts walks the range
// `ze-bmp-conf.yang` gives statistics-timeout, `uint16` with `range
// "0..65535"` and `units "seconds"`, plus the two values the config tree
// cannot deliver as a number.
//
// A value ze cannot read sends NO report rather than one at an interval nobody
// asked for, which is the fallback parseUint16 is given.
// VALIDATES: AC-7 -- the whole uint16 range, and the values ze cannot read.
// PREVENTS: a garbled leaf starting a stream at an interval nobody configured.
func TestBMPStatisticsTimeoutParsesEveryValueTheLeafAccepts(t *testing.T) {
	cases := []struct {
		leaf string
		want time.Duration
	}{
		{"0", 0},
		{"1", time.Second},
		{"65535", 65535 * time.Second},
		{"65536", 0},
		{"", 0},
		{"twenty", 0},
	}
	for _, one := range cases {
		t.Run(one.leaf, func(t *testing.T) {
			bp := &BMPPlugin{dedupState: make(map[string]map[uint64]struct{}), stopCh: make(chan struct{})}
			t.Cleanup(func() {
				close(bp.stopCh)
				bp.sessions.Wait()
			})

			bp.applySenderConfig(defaultSenderConfig(), &senderConfig{
				RouteMonitoringPolicy: policyAll,
				StatisticsTimeout:     one.leaf,
			})

			bp.mu.RLock()
			interval := bp.statisticsInterval
			bp.mu.RUnlock()
			if interval != one.want {
				t.Errorf("statistics-timeout %q installed %s, want %s", one.leaf, interval, one.want)
			}
		})
	}
}

// RFC requirement: RFC7854-x-15 positive -- "Transmission of SR messages could be timer
// triggered or event driven ... It is left to the implementation to determine
// transmission timings -- however, configuration control should be provided of the timer
// and/or threshold values" (RFC 7854 Section 4.8). `statistics-timeout` is that control,
// and this is the whole path from the leaf an operator commits to the bytes a collector
// reads: the reload rail delivers the configuration, the ticker it starts fires, and TWO
// reports arrive one interval apart.
//
// Two rather than one, because "periodically" is what the requirement says. A single
// report proves a report was sent and says nothing about a timer: an implementation that
// emitted one report on a configuration change would pass a one-report assertion.
//
// The reload is delivered over the engine's own rail (config-verify then config-apply),
// so a plugin that read the leaf but filed no reload callback fails here.
func TestRFC7854StatisticsTimeoutSendsPeriodicReports(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		stopCh:     make(chan struct{}),
		dedupState: make(map[string]map[uint64]struct{}),
		peerUps:    map[string]*peerUpState{"10.0.0.1": establishedPeer("10.0.0.1", 65001)},
	}
	engine := startReloadEngine(t, bp, "")
	liveCollectorSession(t, bp, engine, client, map[string]any{
		"route-monitoring-policy": policyAll,
		"statistics-timeout":      statisticsTimeoutOff,
		"collector":               testCollectorEntry(),
	})

	// The one leaf that moves is the timeout. The collector set and the policy
	// are byte for byte what the session already runs under.
	config := senderJSON(t, map[string]any{
		"route-monitoring-policy": policyAll,
		"statistics-timeout":      "1",
		"collector":               testCollectorEntry(),
	})

	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(config) }()

	gaps := awaitStatisticsReports(t, server, 2, 10*time.Second)
	awaitReload(t, done)

	// The second report is what the TIMER produced, so its gap is the one that
	// can tell a timer from a one-shot. Half the interval is the floor a
	// report emitted by the reload itself would fall under.
	if gaps[1] < 500*time.Millisecond {
		t.Errorf("the second report arrived %s after the first, want about the 1s interval", gaps[1])
	}
}

// RFC requirement: RFC7854-x-15 negative -- the same section makes the reports optional
// and puts them under configuration control: "SR messages are optional." (RFC 7854
// Section 4.8). `statistics-timeout 0` is the value `ze-bmp-conf.yang` documents as
// "0 = disabled", so a collector configured that way reads no Statistics Report at all.
//
// Everything else is the setup of the positive above: the same rail, the same live
// collector, the same established peer, and a reload that changes behavior. Only the
// leaf differs, so a plugin that sent reports on a schedule of its own, or one report per
// configuration change, fails here.
func TestRFC7854StatisticsTimeoutZeroSendsNoReport(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := &BMPPlugin{
		stopCh:     make(chan struct{}),
		dedupState: make(map[string]map[uint64]struct{}),
		peerUps:    map[string]*peerUpState{"10.0.0.1": establishedPeer("10.0.0.1", 65001)},
	}
	engine := startReloadEngine(t, bp, "")
	liveCollectorSession(t, bp, engine, client, map[string]any{
		"route-monitoring-policy": policyAll,
		"statistics-timeout":      statisticsTimeoutOff,
		"collector":               testCollectorEntry(),
	})

	config := senderJSON(t, map[string]any{
		"route-monitoring-policy": policyPostPolicy,
		"statistics-timeout":      statisticsTimeoutOff,
		"collector":               testCollectorEntry(),
	})

	done := make(chan error, 1)
	go func() { done <- engine.reloadBGP(config) }()

	// The behavior change owes the peer a Peer Down and a Peer Up, and nothing
	// else. The window is longer than two intervals of the positive test, so a
	// ticker running at the default would have written twice by now.
	deadline := time.After(3 * time.Second)
	for range 2 {
		select {
		case got := <-asyncRead(server):
			if got.err != nil {
				t.Fatalf("the collector session was closed: %v", got.err)
			}
			switch got.msg.(type) {
			case *PeerDown, *PeerUp:
			default:
				t.Fatalf("the collector was sent a %T with the periodic report disabled", got.msg)
			}
		case <-deadline:
			t.Fatal("the behavior change never bounced the peer")
		}
	}
	awaitReload(t, done)

	select {
	case got := <-asyncRead(server):
		if got.err != nil {
			t.Fatalf("the collector session was closed: %v", got.err)
		}
		t.Fatalf("the collector was sent a %T with statistics-timeout 0", got.msg)
	case <-time.After(2500 * time.Millisecond):
	}
}

// RFC requirement: RFC8671-6.2-1 positive -- "Statistics report messages are not
// specific to Adj-RIB-In or Adj-RIB-Out and MUST have the O flag set to zero" (RFC 8671
// Section 6.2). The peer reported on here is monitored for Adj-RIB-Out, so the per-peer
// header ze holds for it carries the O flag, and the report ze puts on the wire carries
// that flag as zero.
//
// The report is read off the collector socket after the production emission path built
// it, which is what makes this a conformance assertion rather than an encoder unit test:
// until the `statistics-timeout` timer existed, ze transmitted no Statistics Report and
// nothing exercised the obligation, so RFC8671-6.2-1 was a {gap} whatever the encoder
// did (owner ruling, 2026-08-31).
//
// The L flag is asserted to SURVIVE. Clearing the whole flags byte would satisfy the
// O flag and lose the post-policy bit the same header owes, and this is the assertion
// that tells the two apart.
func TestRFC8671StatisticsReportOnTheWireClearsTheOFlag(t *testing.T) {
	server, client := net.Pipe()
	defer closeLog(server, "server")
	defer closeLog(client, "client")

	bp := statisticsPlugin(client, policyPostPolicy, time.Minute)
	bp.peerUps[statisticsPeer].peer.Flags |= PeerFlagO | PeerFlagL

	result := asyncRead(server)
	bp.sendStatisticsReports()

	got := <-result
	if got.err != nil {
		t.Fatalf("read the Statistics Report: %v", got.err)
	}
	sr, ok := got.msg.(*statisticsReport)
	if !ok {
		t.Fatalf("the collector was sent a %T, want a Statistics Report", got.msg)
	}
	if sr.Peer.Flags&PeerFlagO != 0 {
		t.Errorf("per-peer flags = %#x, the O flag must be zero on a Statistics Report", sr.Peer.Flags)
	}
	if sr.Peer.Flags&PeerFlagL == 0 {
		t.Errorf("per-peer flags = %#x, clearing the O flag must leave the L flag alone", sr.Peer.Flags)
	}
}
