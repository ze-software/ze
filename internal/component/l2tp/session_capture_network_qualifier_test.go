// Design: docs/architecture/l2tp/subscriber-session-model.md -- explicit network-qualified counter capture.
package l2tp

import (
	"context"
	"errors"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// A cold selector can bind the original network without changing the existing
// logical-session capture contract. Native I/O/Control are the real admitted
// model owner, including a genuine same-worker LCP restart and new IPCP.
// MUTATION: omit the nonzero network qualifier in CaptureSession/currentLocked.
func TestNetworkQualifiedCaptureRejectsSameBirthNextNetwork(t *testing.T) {
	probe := newCapturedCounterProbe(t)
	snapshot, err := probe.captured.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	first := snapshot.NetworkLifetime
	if !first.IsValid() {
		t.Fatal("actual first assignment omitted its network lifetime")
	}
	selector := SessionSelector{SID: 42, TID: 100, Birth: probe.session.Birth(), NetworkLifetime: first}
	root := &Subsystem{reactors: []*l2tpReactor{probe.reactor}}
	original, err := root.CaptureSession(selector)
	if err != nil {
		t.Fatal(err)
	}
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) { return capturedCounterSample(), nil })
	if snapshot, err := original.Snapshot(); err != nil || snapshot.NetworkLifetime != first {
		t.Fatalf("qualified original snapshot=%+v,%v", snapshot, err)
	}
	if got, err := original.Counters(); err != nil || got != capturedCounterSample() {
		t.Fatalf("qualified original counters=%+v,%v", got, err)
	}
	withdrawal := probe.wire.beginRestart(t)
	probe.reactor.handlePPPEvent(withdrawal)
	probe.wire.finishRestart(t)
	probe.reactor.handlePPPEvent(probe.wire.opened)
	probe.wire.prepareNetwork(t, "qualified-next-network", ppp.IPResponseArgs{
		Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.43"),
	}, [8]byte{})
	probe.reactor.handlePPPEvent(probe.wire.assign(t, ppp.AddressFamilyIPv4))
	probe.reactor.handleSessionUp(probe.wire.nextUp(t))
	replacement, err := probe.captured.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	next := replacement.NetworkLifetime
	if !next.IsValid() || next == first {
		t.Fatal("actual LCP restart reused or omitted its next network lifetime")
	}
	var reads atomic.Uint64
	probe.owner.setCounterRead(func() (ppp.SessionCounters, error) { reads.Add(1); return capturedCounterSample(), nil })
	if _, err := original.Counters(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old network counter reader=%v", err)
	}
	if _, err := original.Snapshot(); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old network snapshot=%v", err)
	}
	if reads.Load() != 0 {
		t.Fatal("an old network reached the new network's native reader")
	}
	terminal, err := original.TerminalCounters(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	totals, err := terminal.Counters()
	if err != nil {
		t.Fatal(err)
	}
	if totals.RxBytes != capturedCounterSample().RxBytes {
		t.Fatal("the old receipt lost its frozen original totals")
	}
	if reads.Load() != 0 {
		t.Fatal("the completed old terminal receipt queried the new network")
	}
	if _, err := root.CaptureSession(selector); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("old network selector=%v", err)
	}
	if got, err := probe.captured.Counters(); err != nil || got != capturedCounterSample() {
		t.Fatalf("existing unqualified logical capture lost fresh readings=%+v,%v", got, err)
	}
	selector.NetworkLifetime = next
	current, err := root.CaptureSession(selector)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := current.Counters(); err != nil || got != capturedCounterSample() {
		t.Fatalf("replacement network lost its own positive read=%+v,%v", got, err)
	}
}
