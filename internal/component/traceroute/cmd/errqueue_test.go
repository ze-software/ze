// Design: docs/architecture/diagnostics/active-probes.md -- a refused DF probe reaches the hop record
//
// VALIDATES: AC-3 and AC-4 on the traceroute payload: a probe the path or
// the cache refused for its size records the router that answered and the
// reported next-hop MTU on its hop, or reported=false with no MTU key, and
// the trace ends at that hop.
// PREVENTS: a refused DF probe reading as a timeout, a zero written as an
// MTU, a refusal quoting another process's probe recording our hop, and a
// trace that keeps probing past a router that refused the size.
//
// The kernel's delivery is modeled as it happens under IP_RECVERR: the
// ordinary read fails once with EMSGSIZE and the queue holds the entry. The
// queue itself is scripted through the newTTLConn seam, and the
// kernel's own delivery is proven by
// internal/core/probe/errqueue_integration_linux_test.go.

package cmd

import (
	"context"
	"net"
	"net/netip"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/probe"
)

// refusingConn is a ttlSetter whose first read fails with EMSGSIZE, the wake
// a queued refusal produces, and whose later reads hit the deadline. Its
// error queue hands over the scripted entries once and is then empty, as the
// kernel's queue is. writeErr, when set, is what every WriteTo answers.
type refusingConn struct {
	reads    int
	writeErr error
	queue    []probe.QueuedError
}

func (c *refusingConn) Identifier() uint16          { return tracePID() }
func (c *refusingConn) SetTTL(int) error            { return nil }
func (c *refusingConn) SetDeadline(time.Time) error { return nil }
func (c *refusingConn) Close() error                { return nil }
func (c *refusingConn) ReadFrom(_ []byte) (int, net.Addr, error) {
	c.reads++
	if c.reads == 1 {
		return 0, nil, syscall.EMSGSIZE
	}
	return 0, nil, os.ErrDeadlineExceeded
}

func (c *refusingConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(p), nil
}

func (c *refusingConn) DrainErrors(visit func(probe.QueuedError)) error {
	for _, q := range c.queue {
		visit(q)
	}
	c.queue = nil
	return nil
}

func tracePID() uint16 { return uint16(os.Getpid() & 0xffff) }

func routerRefusal(id, seq uint16, mtu uint32) probe.QueuedError {
	return probe.QueuedError{
		Outcome:  probe.ErrQueueMTUReported,
		MTU:      mtu,
		Errno:    syscall.EMSGSIZE,
		Offender: netip.MustParseAddr("192.0.2.254"),
		Echo:     probe.QuotedEcho{Present: true, ID: id, Seq: seq},
	}
}

// traceWithDF runs one honor-cache trace over conn, standing it in for the
// TTL wrapper the real socket would get.
func traceWithDF(t *testing.T, conn ttlSetter) []map[string]any {
	t.Helper()
	realOpener := openProbeConn
	openProbeConn = func(context.Context, probe.Family, netip.Addr, probe.DFMode) (probeSocket, error) {
		return deadProbeConn{}, nil
	}
	realTTLConn := newTTLConn
	newTTLConn = func(probeSocket, bool) ttlSetter { return conn }
	t.Cleanup(func() {
		openProbeConn = realOpener
		newTTLConn = realTTLConn
	})
	hops, err := doTracerouteCtx(context.Background(), netip.MustParseAddr("192.0.2.1"), 5, time.Second, 1, tracerouteOpts{df: probe.DFHonorCache})
	if err != nil {
		t.Fatalf("doTracerouteCtx: %v", err)
	}
	return hops
}

// TestTracerouteRefusedByPathRecordsNextHopMTU VALIDATES AC-3: the hop a
// router refused names that router, carries the reported MTU with
// reported=true, and ends the trace.
func TestTracerouteRefusedByPathRecordsNextHopMTU(t *testing.T) {
	hops := traceWithDF(t, &refusingConn{queue: []probe.QueuedError{routerRefusal(tracePID(), 0, 1400)}})
	if len(hops) != 1 {
		t.Fatalf("hops = %d, want 1: the trace must end at the router that refused the size", len(hops))
	}
	hop := hops[0]
	if hop["addr"] != "192.0.2.254" {
		t.Errorf("addr = %v, want the offender 192.0.2.254", hop["addr"])
	}
	if hop[probe.FieldNextHopMTUReported] != true {
		t.Errorf("%s = %v, want true", probe.FieldNextHopMTUReported, hop[probe.FieldNextHopMTUReported])
	}
	if hop[probe.FieldNextHopMTU] != 1400 {
		t.Errorf("%s = %v, want 1400", probe.FieldNextHopMTU, hop[probe.FieldNextHopMTU])
	}
	if hop["rtt-ms"] == nil {
		t.Errorf("rtt-ms is nil on a hop that answered")
	}
}

// TestTracerouteRefusedWithoutValueRecordsNoMTU VALIDATES AC-4: a refusal
// with no usable value still records the hop with reported=false and no
// MTU key.
func TestTracerouteRefusedWithoutValueRecordsNoMTU(t *testing.T) {
	q := routerRefusal(tracePID(), 0, 0)
	q.Outcome = probe.ErrQueueMTUUnreported
	hops := traceWithDF(t, &refusingConn{queue: []probe.QueuedError{q}})
	if len(hops) != 1 {
		t.Fatalf("hops = %d, want 1", len(hops))
	}
	if hops[0][probe.FieldNextHopMTUReported] != false {
		t.Errorf("%s = %v, want false", probe.FieldNextHopMTUReported, hops[0][probe.FieldNextHopMTUReported])
	}
	if _, present := hops[0][probe.FieldNextHopMTU]; present {
		t.Errorf("%s present with no reported value", probe.FieldNextHopMTU)
	}
}

// TestTracerouteRefusalQuotingAnotherProbeIsIgnored proves the quoted
// identifier is matched before the hop is recorded: an entry quoting another
// process's probe leaves the hop unanswered and the trace running to its
// maximum.
func TestTracerouteRefusalQuotingAnotherProbeIsIgnored(t *testing.T) {
	hops := traceWithDF(t, &refusingConn{queue: []probe.QueuedError{routerRefusal(tracePID()+1, 0, 1400)}})
	if len(hops) != 5 {
		t.Fatalf("hops = %d, want 5: a refusal of another probe must not end the trace", len(hops))
	}
	if hops[0]["addr"] != "*" {
		t.Errorf("hop 1 addr = %v, want *", hops[0]["addr"])
	}
	if _, present := hops[0][probe.FieldNextHopMTUReported]; present {
		t.Errorf("hop 1 carries %s from another probe's refusal", probe.FieldNextHopMTUReported)
	}
}

// TestTracerouteRefusedAtSendRecordsCachedEstimate VALIDATES the send side
// under honor-cache: a send the kernel refused against its cached path MTU
// records the estimate on a hop that names no router, and ends the trace.
func TestTracerouteRefusedAtSendRecordsCachedEstimate(t *testing.T) {
	local := probe.QueuedError{Outcome: probe.ErrQueueMTUReported, MTU: 1400, Errno: syscall.EMSGSIZE, Local: true}
	hops := traceWithDF(t, &refusingConn{writeErr: syscall.EMSGSIZE, queue: []probe.QueuedError{local}})
	if len(hops) != 1 {
		t.Fatalf("hops = %d, want 1", len(hops))
	}
	hop := hops[0]
	if hop["addr"] != "*" {
		t.Errorf("addr = %v, want *: nothing reached the wire", hop["addr"])
	}
	if hop[probe.FieldNextHopMTU] != 1400 {
		t.Errorf("%s = %v, want 1400 (the cached estimate)", probe.FieldNextHopMTU, hop[probe.FieldNextHopMTU])
	}
}
