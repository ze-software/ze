// Design: plan/learned/959-ospf-5-interface-ism.md -- NBMA + point-to-multipoint ISM/Hello tests.
package iface

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// dstSender records the unicast destination of every SendPacket so a test can assert
// the NBMA/PtMP unicast fan-out (which the shared fakeSender discards).
type dstSender struct {
	dsts     []netip.Addr
	payloads [][]byte
}

func (s *dstSender) SendPacket(_ string, dst netip.Addr, payload []byte) error {
	s.dsts = append(s.dsts, dst)
	s.payloads = append(s.payloads, append([]byte(nil), payload...))
	return nil
}
func (s *dstSender) JoinAllDRouters(string) error  { return nil }
func (s *dstSender) LeaveAllDRouters(string) error { return nil }

func (s *dstSender) sentTo(a netip.Addr) int {
	n := 0
	for _, d := range s.dsts {
		if d == a {
			n++
		}
	}
	return n
}

func nbmaConfig(t *testing.T) Config {
	cfg := baseConfig(t)
	cfg.NetworkType = NetworkNBMA
	cfg.PollInterval = 120
	return cfg
}

func v6NBMAConfig(t *testing.T) Config {
	cfg := nbmaConfig(t)
	cfg.IsV6 = true
	cfg.NetworkMask = [4]byte{}
	cfg.InterfaceAddress = [4]byte{}
	return cfg
}

// TestOSPFPtMPISMNoElection: a point-to-multipoint interface takes the point-to-point
// ISM state and never elects (AC-1, RFC 2328 sec 9.5).
func TestOSPFPtMPISMNoElection(t *testing.T) {
	cfg := baseConfig(t)
	cfg.NetworkType = NetworkPointToMultipoint
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StatePointToPoint {
		t.Fatalf("PtMP state = %s, want point-to-point", ifc.State())
	}
	ifc.forceWaitTimer()
	if ifc.DR() != (types.RouterID{}) || ifc.BDR() != (types.RouterID{}) {
		t.Fatalf("PtMP elected DR=%s BDR=%s, want none", ifc.DR(), ifc.BDR())
	}
}

func TestOSPFv3PtMPISMNoElection(t *testing.T) {
	cfg := baseConfig(t)
	cfg.NetworkType = NetworkPointToMultipoint
	cfg.IsV6 = true
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StatePointToPoint {
		t.Fatalf("v3 PtMP state = %s, want point-to-point", ifc.State())
	}
	ifc.forceWaitTimer()
	if ifc.DR() != (types.RouterID{}) {
		t.Fatalf("v3 PtMP elected DR=%s, want none", ifc.DR())
	}
}

// TestOSPFPtMPNoElection: a received Hello on a PtMP interface never triggers an
// election (AC-6, R-3).
func TestOSPFPtMPNoElection(t *testing.T) {
	cfg := baseConfig(t)
	cfg.NetworkType = NetworkPointToMultipoint
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	ifc.forceWaitTimer()
	if ifc.DR() != (types.RouterID{}) || ifc.BDR() != (types.RouterID{}) {
		t.Fatalf("PtMP elected after Hello: DR=%s BDR=%s", ifc.DR(), ifc.BDR())
	}
}

func TestOSPFv3PtMPNoElection(t *testing.T) {
	cfg := baseConfig(t)
	cfg.NetworkType = NetworkPointToMultipoint
	cfg.IsV6 = true
	cfg.NetworkMask = [4]byte{}
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	if reason := ifc.receiveHello(peer, netip.MustParseAddr("fe80::2"), helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("receiveHello: %s", reason)
	}
	ifc.forceWaitTimer()
	if ifc.DR() != (types.RouterID{}) {
		t.Fatalf("v3 PtMP elected after Hello: DR=%s", ifc.DR())
	}
}

// TestOSPFNBMAISMWaiting: an eligible NBMA interface enters Waiting; a priority-0 one
// goes straight to DROther (AC-9).
func TestOSPFNBMAISMWaiting(t *testing.T) {
	cfg := nbmaConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StateWaiting {
		t.Fatalf("eligible NBMA state = %s, want waiting", ifc.State())
	}

	cfg0 := nbmaConfig(t)
	cfg0.Priority = 0
	ifc0 := New(cfg0, &fakeSender{}, NopMetrics())
	ifc0.Start()
	defer ifc0.Stop()
	if ifc0.State() != StateDROther {
		t.Fatalf("priority-0 NBMA state = %s, want dr-other", ifc0.State())
	}
}

func TestOSPFv3NBMAISMWaiting(t *testing.T) {
	cfg := v6NBMAConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StateWaiting {
		t.Fatalf("eligible v3 NBMA state = %s, want waiting", ifc.State())
	}
}

// TestOSPFNBMAElection: NBMA reuses electDRBDR over heard neighbors, exactly like
// broadcast (AC-11, A-1/A-2).
func TestOSPFNBMAElection(t *testing.T) {
	cfg := nbmaConfig(t)
	peerAddr := netip.MustParseAddr("10.0.0.2")
	cfg.NBMANeighbors = []NBMANeighbor{{Address: peerAddr, Priority: 1}}
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	if reason := ifc.receiveHello(peer, peerAddr, helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("receiveHello: %s", reason)
	}
	ifc.forceWaitTimer()
	if ifc.DR() != peer || ifc.BDR() != cfg.RouterID {
		t.Fatalf("NBMA election DR=%s BDR=%s, want peer DR + self BDR", ifc.DR(), ifc.BDR())
	}
}

func TestOSPFv3NBMAElection(t *testing.T) {
	cfg := v6NBMAConfig(t)
	peer := rid(t, "10.0.0.2")
	cfg.NBMANeighbors = []NBMANeighbor{{RouterID: peer, LinkLocal: netip.MustParseAddr("fe80::2"), Priority: 1}}
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if reason := ifc.receiveHello(peer, netip.MustParseAddr("fe80::2"), helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("receiveHello: %s", reason)
	}
	ifc.forceWaitTimer()
	// The higher Router ID (10.0.0.2) wins DR; self (10.0.0.1) is BDR.
	if ifc.DR() != peer || ifc.BDR() != cfg.RouterID {
		t.Fatalf("v3 NBMA election DR=%s BDR=%s, want peer DR + self BDR", ifc.DR(), ifc.BDR())
	}
}

// TestOSPFNBMAUnicastHello: NBMA Hellos are unicast to each configured neighbor, never
// to the all-routers multicast group (AC-10, R-5).
func TestOSPFNBMAUnicastHello(t *testing.T) {
	cfg := nbmaConfig(t)
	n1 := netip.MustParseAddr("10.0.0.2")
	n2 := netip.MustParseAddr("10.0.0.3")
	cfg.NBMANeighbors = []NBMANeighbor{{Address: n1, Priority: 1}, {Address: n2, Priority: 1}}
	s := &dstSender{}
	ifc := New(cfg, s, NopMetrics())
	now := time.Unix(1000, 0)
	// Both neighbors have been heard, so both get a HelloInterval unicast.
	if reason := ifc.receiveHello(rid(t, "10.0.0.2"), n1, helloFor(cfg, cfg.RouterID), now); reason != "" {
		t.Fatalf("receiveHello n1: %s", reason)
	}
	if reason := ifc.receiveHello(rid(t, "10.0.0.3"), n2, helloFor(cfg, cfg.RouterID), now); reason != "" {
		t.Fatalf("receiveHello n2: %s", reason)
	}
	if err := ifc.sendHelloAt(now); err != nil {
		t.Fatal(err)
	}
	if s.sentTo(n1) != 1 || s.sentTo(n2) != 1 {
		t.Fatalf("NBMA Hello dsts = %v, want one each to n1/n2", s.dsts)
	}
	if s.sentTo(allSPFRouters) != 0 {
		t.Fatalf("NBMA sent a multicast Hello to %s", allSPFRouters)
	}
}

func TestOSPFv3NBMAUnicastHello(t *testing.T) {
	cfg := v6NBMAConfig(t)
	peer := rid(t, "10.0.0.2")
	ll := netip.MustParseAddr("fe80::2")
	cfg.NBMANeighbors = []NBMANeighbor{{RouterID: peer, LinkLocal: ll, Priority: 1}}
	s := &dstSender{}
	ifc := New(cfg, s, NopMetrics())
	now := time.Unix(1000, 0)
	if reason := ifc.receiveHello(peer, ll, helloFor(cfg, cfg.RouterID), now); reason != "" {
		t.Fatalf("receiveHello: %s", reason)
	}
	if err := ifc.sendHelloAt(now); err != nil {
		t.Fatal(err)
	}
	if s.sentTo(ll) != 1 {
		t.Fatalf("v3 NBMA Hello dsts = %v, want one to link-local %s", s.dsts, ll)
	}
	if s.sentTo(allSPFRoutersV6) != 0 {
		t.Fatalf("v3 NBMA sent a multicast Hello to %s", allSPFRoutersV6)
	}
}

// TestOSPFNBMAPollAttempt: a silent (never-heard) neighbor is polled only every
// PollInterval; a heard neighbor is sent every HelloInterval tick (AC-10, A-9).
func TestOSPFNBMAPollAttempt(t *testing.T) {
	cfg := nbmaConfig(t)
	cfg.HelloInterval = 10
	cfg.PollInterval = 120
	heard := netip.MustParseAddr("10.0.0.2")
	silent := netip.MustParseAddr("10.0.0.3")
	cfg.NBMANeighbors = []NBMANeighbor{{Address: heard, Priority: 1}, {Address: silent, Priority: 1}}
	s := &dstSender{}
	ifc := New(cfg, s, NopMetrics())
	t0 := time.Unix(1000, 0)
	if reason := ifc.receiveHello(rid(t, "10.0.0.2"), heard, helloFor(cfg, cfg.RouterID), t0); reason != "" {
		t.Fatalf("receiveHello: %s", reason)
	}
	// t0: heard + first poll of silent.
	_ = ifc.sendHelloAt(t0)
	// t0+10s (< PollInterval): heard again, silent NOT re-polled.
	_ = ifc.sendHelloAt(t0.Add(10 * time.Second))
	// t0+120s (>= PollInterval): silent polled again.
	_ = ifc.sendHelloAt(t0.Add(120 * time.Second))
	if got := s.sentTo(heard); got != 3 {
		t.Fatalf("heard neighbor Hellos = %d, want 3 (one per tick)", got)
	}
	if got := s.sentTo(silent); got != 2 {
		t.Fatalf("silent neighbor polls = %d, want 2 (t0 and t0+PollInterval)", got)
	}
}

func TestOSPFv3NBMAPollAttempt(t *testing.T) {
	cfg := v6NBMAConfig(t)
	cfg.HelloInterval = 10
	cfg.PollInterval = 120
	peer := rid(t, "10.0.0.3")
	ll := netip.MustParseAddr("fe80::3")
	cfg.NBMANeighbors = []NBMANeighbor{{RouterID: peer, LinkLocal: ll, Priority: 1}}
	s := &dstSender{}
	ifc := New(cfg, s, NopMetrics())
	t0 := time.Unix(1000, 0)
	_ = ifc.sendHelloAt(t0)
	_ = ifc.sendHelloAt(t0.Add(10 * time.Second))
	_ = ifc.sendHelloAt(t0.Add(120 * time.Second))
	if got := s.sentTo(ll); got != 2 {
		t.Fatalf("v3 silent neighbor polls = %d, want 2", got)
	}
}

// TestOSPFNBMAStartHelloPriorityZero: when this router becomes DR/BDR it sends a Start
// Hello to a priority-0 neighbor (AC-12, RFC 2328 sec 9.4 step 6).
func TestOSPFNBMAStartHelloPriorityZero(t *testing.T) {
	cfg := nbmaConfig(t)
	zero := netip.MustParseAddr("10.0.0.9")
	cfg.NBMANeighbors = []NBMANeighbor{{Address: zero, Priority: 0}}
	s := &dstSender{}
	ifc := New(cfg, s, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	// A lone eligible router elects itself DR, triggering the step-6 Start Hello.
	ifc.forceWaitTimer()
	if ifc.State() != StateDR {
		t.Fatalf("state = %s, want dr (lone eligible router)", ifc.State())
	}
	if s.sentTo(zero) == 0 {
		t.Fatalf("no Start Hello sent to priority-0 neighbor %s; dsts=%v", zero, s.dsts)
	}
}

func TestOSPFv3NBMAStartHelloPriorityZero(t *testing.T) {
	cfg := v6NBMAConfig(t)
	zero := rid(t, "10.0.0.9")
	ll := netip.MustParseAddr("fe80::9")
	cfg.NBMANeighbors = []NBMANeighbor{{RouterID: zero, LinkLocal: ll, Priority: 0}}
	s := &dstSender{}
	ifc := New(cfg, s, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	ifc.forceWaitTimer()
	if ifc.State() != StateDR {
		t.Fatalf("v3 state = %s, want dr", ifc.State())
	}
	if s.sentTo(ll) == 0 {
		t.Fatalf("no v3 Start Hello sent to priority-0 neighbor link-local %s; dsts=%v", ll, s.dsts)
	}
}

// TestOSPFNBMAPriorityZeroHelloGating pins RFC 2328 sec 9.5.1: a priority-0 (ineligible)
// NBMA neighbor is a periodic/poll Hello target only while this router is DR or BDR. On a
// DROther interface it gets no Hello target (heard or silent), while an eligible neighbor
// is still polled; on becoming DR the priority-0 neighbors are Start-Hello targets; and a
// DR interface polls a priority-0 neighbor from the periodic path too.
func TestOSPFNBMAPriorityZeroHelloGating(t *testing.T) {
	cfg := nbmaConfig(t)
	heardZero := netip.MustParseAddr("10.0.0.8")
	silentZero := netip.MustParseAddr("10.0.0.9")
	elig := netip.MustParseAddr("10.0.0.2")
	cfg.NBMANeighbors = []NBMANeighbor{
		{Address: heardZero, Priority: 0},
		{Address: silentZero, Priority: 0},
		{Address: elig, Priority: 1},
	}
	now := time.Unix(1000, 0)

	// DROther: neither priority-0 neighbor (even a heard one) is a Hello target; the
	// eligible neighbor is still polled.
	other := New(cfg, &fakeSender{}, NopMetrics())
	other.neighbors[rid(t, "10.0.0.8")] = Neighbor{RouterID: rid(t, "10.0.0.8"), Address: heardZero}
	other.state = StateDROther
	got := other.helloTargetsLocked(now)
	if slices.Contains(got, heardZero) || slices.Contains(got, silentZero) {
		t.Fatalf("DROther Hello targets = %v, want no priority-0 neighbor", got)
	}
	if !slices.Contains(got, elig) {
		t.Fatalf("DROther Hello targets = %v, want eligible neighbor %s polled", got, elig)
	}

	// On becoming DR the priority-0 neighbors receive the one-shot Start Hello (sec 9.4 step 6).
	other.state = StateDR
	starts := other.startHelloTargetsLocked(true)
	if !slices.Contains(starts, heardZero) || !slices.Contains(starts, silentZero) {
		t.Fatalf("Start-Hello targets = %v, want both priority-0 neighbors", starts)
	}

	// DR: a priority-0 silent neighbor is now polled by the periodic Hello path.
	dr := New(cfg, &fakeSender{}, NopMetrics())
	dr.state = StateDR
	got = dr.helloTargetsLocked(now)
	if !slices.Contains(got, silentZero) {
		t.Fatalf("DR Hello targets = %v, want priority-0 neighbor %s polled", got, silentZero)
	}
}

// TestOSPFNBMAPollBoundary exercises the PollInterval boundary: exactly at the interval
// the silent neighbor is re-polled, one second short it is not.
func TestOSPFNBMAPollBoundary(t *testing.T) {
	cfg := nbmaConfig(t)
	cfg.PollInterval = 120
	silent := netip.MustParseAddr("10.0.0.3")
	cfg.NBMANeighbors = []NBMANeighbor{{Address: silent, Priority: 1}}
	s := &dstSender{}
	ifc := New(cfg, s, NopMetrics())
	t0 := time.Unix(1000, 0)
	_ = ifc.sendHelloAt(t0)                        // poll 1
	_ = ifc.sendHelloAt(t0.Add(119 * time.Second)) // 1s short: no poll
	if got := s.sentTo(silent); got != 1 {
		t.Fatalf("polls one second short of PollInterval = %d, want 1", got)
	}
	_ = ifc.sendHelloAt(t0.Add(120 * time.Second)) // exactly PollInterval: poll 2
	if got := s.sentTo(silent); got != 2 {
		t.Fatalf("polls at PollInterval = %d, want 2", got)
	}
}
