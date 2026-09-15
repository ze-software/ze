// Design: docs/architecture/diagnostics/active-probes.md -- a refused DF probe reaches the payload
//
// VALIDATES: AC-3 and AC-4 on the ping payload: a probe the path or the
// cache refused for its size is a too-big row carrying the reported next-hop
// MTU, or reported=false with no MTU key, and the batch summary counts and
// reports it.
// PREVENTS: a refused DF probe timing out silently, a zero written as an
// MTU, a refusal quoting another process's probe resolving ours, and a
// refused send stopping the session.
//
// These tests drive runPingSession through the fake conn with the kernel's
// IP_RECVERR delivery modeled as it happens: a refusal from the network is
// queued and then makes one ordinary read fail with EMSGSIZE, and a refusal
// at send fails WriteTo with EMSGSIZE and queues the cached estimate. The
// kernel's own delivery is proven by
// internal/core/probe/errqueue_integration_linux_test.go.

package cmd

import (
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/test/sim"
)

// routerRefusal is the entry the kernel queues when a router answered a probe
// with Fragmentation Needed reporting mtu, quoting the probe (id, seq).
func routerRefusal(id, seq uint16, mtu uint32) probe.QueuedError {
	return probe.QueuedError{
		Outcome:  probe.ErrQueueMTUReported,
		MTU:      mtu,
		Errno:    syscall.EMSGSIZE,
		Offender: netip.MustParseAddr("192.0.2.254"),
		Echo:     probe.QuotedEcho{Present: true, ID: id, Seq: seq},
	}
}

// cachedRefusal is the entry the kernel queues when it refused a send
// against its cached path MTU: local, quoting nothing.
func cachedRefusal(mtu uint32) probe.QueuedError {
	return probe.QueuedError{Outcome: probe.ErrQueueMTUReported, MTU: mtu, Errno: syscall.EMSGSIZE, Local: true}
}

// TestPingRefusedByPathReportsNextHopMTU VALIDATES AC-3: a probe a router
// refused for its size resolves as too-big with the reported MTU and
// reported=true, rather than timing out silently.
func TestPingRefusedByPathReportsNextHopMTU(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, 1)
	defer cancel()

	w := <-fc.wrote
	fc.queueError(routerRefusal(testPID(), w.seq, 1400))
	fc.injectReadErr(syscall.EMSGSIZE)

	r := <-out
	if r[fieldStatus] != statusTooBig {
		t.Fatalf("status = %v, want %q", r[fieldStatus], statusTooBig)
	}
	if r[fieldNextHopMTUReported] != true {
		t.Fatalf("%s = %v, want true", fieldNextHopMTUReported, r[fieldNextHopMTUReported])
	}
	if r[fieldNextHopMTU] != 1400 {
		t.Fatalf("%s = %v, want 1400", fieldNextHopMTU, r[fieldNextHopMTU])
	}
	if r["seq"] != 0 {
		t.Fatalf("seq = %v, want 0", r["seq"])
	}
}

// TestPingRefusedWithZeroNextHopMTUReportsNoValue VALIDATES AC-4: a refusal
// whose next-hop MTU the probe layer could not use (RFC 1191's zero) is
// still a too-big row, says no value was reported, and carries no MTU key
// at all: a zero is never written as a value.
func TestPingRefusedWithZeroNextHopMTUReportsNoValue(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, 1)
	defer cancel()

	w := <-fc.wrote
	q := routerRefusal(testPID(), w.seq, 0)
	q.Outcome = probe.ErrQueueMTUUnreported
	fc.queueError(q)
	fc.injectReadErr(syscall.EMSGSIZE)

	r := <-out
	if r[fieldStatus] != statusTooBig {
		t.Fatalf("status = %v, want %q", r[fieldStatus], statusTooBig)
	}
	if r[fieldNextHopMTUReported] != false {
		t.Fatalf("%s = %v, want false", fieldNextHopMTUReported, r[fieldNextHopMTUReported])
	}
	if _, present := r[fieldNextHopMTU]; present {
		t.Fatalf("%s present on a row with no reported value: %v", fieldNextHopMTU, r[fieldNextHopMTU])
	}
}

// TestPingRefusalQuotingAnotherProbeIsIgnored proves the quoted identifier
// is matched before a refusal is believed: the kernel hands an unconnected
// raw socket every ICMP error quoting an ICMP datagram from this host, so an
// entry quoting another process's probe leaves ours in flight, and it times
// out as before. The test waits for the drain to have run before it expires
// the probe, so a wrong resolution cannot hide behind the timeout.
func TestPingRefusalQuotingAnotherProbeIsIgnored(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, 1)
	defer cancel()

	w := <-fc.wrote
	fc.queueError(routerRefusal(testPID()+1, w.seq, 1400))
	fc.injectReadErr(syscall.EMSGSIZE)
	<-fc.drained

	clk.Add(5 * time.Second)
	r := <-out
	if r[fieldStatus] != "timeout" {
		t.Fatalf("status = %v, want timeout: a refusal quoting another identifier resolved our probe", r[fieldStatus])
	}
}

// TestPingRefusedAtSendReportsCachedEstimate VALIDATES the send side of
// AC-3 under honor-cache: the kernel refuses a probe larger than its cached
// path MTU with EMSGSIZE at WriteTo and queues the estimate. The probe is
// reported as too-big-cached with that estimate, and the session goes on to
// resolve every requested probe rather than stopping at the refusal. The
// first probe is answered normally so the ticker is known to exist before
// the test fires it.
func TestPingRefusedAtSendReportsCachedEstimate(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, 2)
	defer cancel()

	w := <-fc.wrote
	fc.injectReply(testPID(), w.seq)
	if r := <-out; r[fieldStatus] != "ok" {
		t.Fatalf("first probe status = %v, want ok", r[fieldStatus])
	}

	fc.setWriteErr(syscall.EMSGSIZE)
	fc.queueError(cachedRefusal(1400))
	clk.Add(time.Second)
	clk.FireTickers()

	r := <-out
	if r[fieldStatus] != statusTooBigCached {
		t.Fatalf("status = %v, want %q", r[fieldStatus], statusTooBigCached)
	}
	if r[fieldNextHopMTU] != 1400 {
		t.Fatalf("%s = %v, want 1400 (the cached estimate)", fieldNextHopMTU, r[fieldNextHopMTU])
	}
	if r["seq"] != 1 {
		t.Fatalf("seq = %v, want 1", r["seq"])
	}
	if _, open := <-out; open {
		t.Fatal("session did not end after every probe resolved")
	}
}

// TestPingSummaryCountsAndReportsRefusals proves the batch summary keeps a
// probe refused at send out of sent (it never reached the wire), keeps one a
// router refused in it, and carries the smallest reported next-hop MTU,
// which is the only value RFC 1191 Section 3 lets an estimate move to. A run
// with no refusal carries neither key.
func TestPingSummaryCountsAndReportsRefusals(t *testing.T) {
	replies := []map[string]any{
		{"seq": 0, fieldStatus: statusTooBig, fieldNextHopMTUReported: true, fieldNextHopMTU: 1400},
		{"seq": 1, fieldStatus: statusTooBigCached, fieldNextHopMTUReported: true, fieldNextHopMTU: 1300},
		{"seq": 2, fieldStatus: "timeout"},
	}
	res := summarizePingReplies(testDest(), replies)
	if res["sent"] != 2 {
		t.Errorf("sent = %v, want 2: the send the kernel refused never reached the wire", res["sent"])
	}
	if res[fieldNextHopMTU] != 1300 {
		t.Errorf("summary %s = %v, want 1300", fieldNextHopMTU, res[fieldNextHopMTU])
	}
	if res[fieldNextHopMTUReported] != true {
		t.Errorf("summary %s = %v, want true", fieldNextHopMTUReported, res[fieldNextHopMTUReported])
	}

	unreported := summarizePingReplies(testDest(), []map[string]any{{"seq": 0, fieldStatus: statusTooBig, fieldNextHopMTUReported: false}})
	if unreported[fieldNextHopMTUReported] != false {
		t.Errorf("summary over an unreported refusal: %s = %v, want false", fieldNextHopMTUReported, unreported[fieldNextHopMTUReported])
	}
	if _, present := unreported[fieldNextHopMTU]; present {
		t.Errorf("summary over an unreported refusal carries %s", fieldNextHopMTU)
	}

	none := summarizePingReplies(testDest(), []map[string]any{{"seq": 0, fieldStatus: "ok", "rtt-ms": 1.0}})
	if _, present := none[fieldNextHopMTUReported]; present {
		t.Errorf("a run with no refusal carries %s", fieldNextHopMTUReported)
	}
}

// TestPingRefusedSendKeepsTheRouterAnswerAhead PREVENTS a router's answer
// being dropped by the drain a refused send performs. Under honor-cache the
// two arrive together: the router refuses probe 0 and reports 1400, which
// shrinks the kernel's cache, and the cache then refuses probe 1 at send.
// When the ticker wins the race against the receiver's wake, the sender's
// drain is the first read of the queue and finds both entries. Probe 0 is
// too-big with the router's value, probe 1 too-big-cached with the cache's,
// and neither times out.
func TestPingRefusedSendKeepsTheRouterAnswerAhead(t *testing.T) {
	clk := sim.NewFakeClock(testEpoch)
	fc, out, cancel := startSession(clk, time.Second, 5*time.Second, 2)
	defer cancel()

	w := <-fc.wrote
	fc.queueError(routerRefusal(testPID(), w.seq, 1400))
	fc.setWriteErr(syscall.EMSGSIZE)
	fc.queueError(cachedRefusal(1400))
	clk.Add(time.Second)
	clk.FireTickers()

	first := <-out
	if first[fieldStatus] != statusTooBig {
		t.Fatalf("probe 0 status = %v, want %q (the router's answer was dropped)", first[fieldStatus], statusTooBig)
	}
	if first["seq"] != 0 {
		t.Fatalf("probe 0 seq = %v, want 0", first["seq"])
	}
	if first[fieldNextHopMTU] != 1400 {
		t.Fatalf("probe 0 %s = %v, want 1400", fieldNextHopMTU, first[fieldNextHopMTU])
	}
	second := <-out
	if second[fieldStatus] != statusTooBigCached {
		t.Fatalf("probe 1 status = %v, want %q", second[fieldStatus], statusTooBigCached)
	}
	if second["seq"] != 1 {
		t.Fatalf("probe 1 seq = %v, want 1", second["seq"])
	}
	if second[fieldNextHopMTU] != 1400 {
		t.Fatalf("probe 1 %s = %v, want 1400 (the cached estimate)", fieldNextHopMTU, second[fieldNextHopMTU])
	}
	if _, open := <-out; open {
		t.Fatal("session did not end after every probe resolved")
	}
}
