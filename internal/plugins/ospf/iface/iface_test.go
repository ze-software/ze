// Design: docs/architecture/ospf/ospf-5-interface-ism.md -- ISM, Hello, and election tests
package iface

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type fakeSender struct {
	sent   [][]byte
	joined []string
	left   []string
}

func (s *fakeSender) SendPacket(_ string, _ netip.Addr, payload []byte) error {
	cp := append([]byte(nil), payload...)
	s.sent = append(s.sent, cp)
	return nil
}
func (s *fakeSender) JoinAllDRouters(name string) error {
	s.joined = append(s.joined, name)
	return nil
}
func (s *fakeSender) LeaveAllDRouters(name string) error { s.left = append(s.left, name); return nil }

type recordSink struct {
	states    []Snapshot
	dr        []Snapshot
	neighbors []Snapshot
}

func (s *recordSink) InterfaceStateChanged(snap Snapshot) { s.states = append(s.states, snap) }
func (s *recordSink) DRChanged(snap Snapshot)             { s.dr = append(s.dr, snap) }
func (s *recordSink) NeighborChanged(snap Snapshot)       { s.neighbors = append(s.neighbors, snap) }

func rid(t *testing.T, s string) types.RouterID {
	t.Helper()
	id, err := types.ParseRouterID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func area(t *testing.T, s string) types.AreaID {
	t.Helper()
	id, err := types.ParseAreaID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func baseConfig(t *testing.T) Config {
	return Config{
		Name:             "eth0",
		RouterID:         rid(t, "10.0.0.1"),
		AreaID:           area(t, "0"),
		AreaType:         "normal",
		NetworkType:      NetworkBroadcast,
		NetworkMask:      [4]byte{255, 255, 255, 0},
		InterfaceAddress: [4]byte{10, 0, 0, 1},
		Cost:             10,
		HelloInterval:    10,
		DeadInterval:     40,
		Priority:         1,
	}
}

func helloFor(cfg Config, neighbors ...types.RouterID) packet.Hello {
	return packet.Hello{
		NetworkMask:   cfg.NetworkMask,
		HelloInterval: cfg.HelloInterval,
		Options:       types.OptionE,
		Priority:      1,
		DeadInterval:  uint32(cfg.DeadInterval),
		DR:            [4]byte{},
		BDR:           [4]byte{},
		Neighbors:     neighbors,
	}
}

type recordingEncoder struct {
	called bool
	got    packet.Hello
}

func (e *recordingEncoder) EncodeHello(_ types.RouterID, _ types.AreaID, h packet.Hello) []byte {
	e.called = true
	e.got = h
	return []byte{0xAB}
}

// TestOSPFIfaceHelloEncoderSeam proves the interface builds the AF-neutral Hello
// and serializes it through the injected Encoder, so an OSPFv3 encoder can replace
// the default OSPFv2 one. PREVENTS: the send path being hard-wired to ospf/packet.
func TestOSPFIfaceHelloEncoderSeam(t *testing.T) {
	cfg := baseConfig(t)
	cfg.InterfaceID = 42 // OSPFv3 Interface ID: the encoder must receive it (RFC 5340 sec 3.4.3)
	i := New(cfg, &fakeSender{}, NopMetrics())
	rec := &recordingEncoder{}
	i.SetEncoder(rec)

	buf := i.buildHelloPacket()
	if !rec.called {
		t.Fatal("buildHelloPacket did not route through the injected Encoder")
	}
	if len(buf) != 1 || buf[0] != 0xAB {
		t.Fatalf("encoder output not returned: %v", buf)
	}
	if rec.got.NetworkMask != cfg.NetworkMask || rec.got.HelloInterval != cfg.HelloInterval {
		t.Fatalf("interface did not build the neutral Hello fields: %+v", rec.got)
	}
	if rec.got.InterfaceID != cfg.InterfaceID {
		t.Fatalf("Hello Interface ID = %d, want %d (must match the interface for the Router-LSA)", rec.got.InterfaceID, cfg.InterfaceID)
	}
}

func TestOSPFISMBroadcastUpToWaiting(t *testing.T) {
	ifc := New(baseConfig(t), &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StateWaiting {
		t.Fatalf("state = %s, want waiting", ifc.State())
	}
}

func TestOSPFISMP2PUpToPointToPoint(t *testing.T) {
	cfg := baseConfig(t)
	cfg.NetworkType = NetworkPointToPoint
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StatePointToPoint {
		t.Fatalf("state = %s, want point-to-point", ifc.State())
	}
	ifc.forceWaitTimer()
	if ifc.DR() != (types.RouterID{}) || ifc.BDR() != (types.RouterID{}) {
		t.Fatalf("p2p elected DR=%s BDR=%s", ifc.DR(), ifc.BDR())
	}
}

func TestOSPFISMLoopbackNoHello(t *testing.T) {
	cfg := baseConfig(t)
	cfg.NetworkType = NetworkLoopback
	sender := &fakeSender{}
	ifc := New(cfg, sender, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StateLoopback {
		t.Fatalf("state = %s, want loopback", ifc.State())
	}
	ifc.forceWaitTimer()
	if ifc.DR() != (types.RouterID{}) || ifc.BDR() != (types.RouterID{}) {
		t.Fatalf("loopback elected DR=%s BDR=%s", ifc.DR(), ifc.BDR())
	}
	if err := ifc.SendHello(); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("loopback sent %d Hellos", len(sender.sent))
	}
}

func TestOSPFDRElectionTwoNodes(t *testing.T) {
	cfg := baseConfig(t)
	sender := &fakeSender{}
	ifc := New(cfg, sender, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	ifc.forceWaitTimer()
	if ifc.DR() != peer || ifc.BDR() != cfg.RouterID || ifc.State() != StateBackup {
		t.Fatalf("DR=%s BDR=%s state=%s, want peer/local backup", ifc.DR(), ifc.BDR(), ifc.State())
	}
	if len(sender.joined) != 1 || sender.joined[0] != cfg.Name {
		t.Fatalf("AllDRouters joins = %v, want local backup to join", sender.joined)
	}
	if err := ifc.SendHello(); err != nil {
		t.Fatal(err)
	}
	p, err := packet.DecodePacket(sender.sent[0])
	if err != nil {
		t.Fatal(err)
	}
	if p.Hello.DR != [4]byte{10, 0, 0, 2} || p.Hello.BDR != cfg.InterfaceAddress {
		t.Fatalf("Hello DR=%v BDR=%v, want interface addresses", p.Hello.DR, p.Hello.BDR)
	}
}

func TestOSPFStopLeavesAllDRouters(t *testing.T) {
	cfg := baseConfig(t)
	sender := &fakeSender{}
	ifc := New(cfg, sender, NopMetrics())
	ifc.Start()
	peer := rid(t, "10.0.0.2")
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	ifc.forceWaitTimer()
	ifc.Stop()
	if len(sender.left) != 1 || sender.left[0] != cfg.Name {
		t.Fatalf("AllDRouters leaves = %v, want stopped backup to leave", sender.left)
	}
}

func TestOSPFInterfaceEvents(t *testing.T) {
	cfg := baseConfig(t)
	sink := &recordSink{}
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.SetEventSink(sink)
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	ifc.forceWaitTimer()
	if len(sink.states) == 0 || len(sink.neighbors) == 0 || len(sink.dr) == 0 {
		t.Fatalf("events states=%d neighbors=%d dr=%d, want all event types", len(sink.states), len(sink.neighbors), len(sink.dr))
	}
}

func TestOSPFISMBackupSeenShortCircuitsWait(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	h := helloFor(cfg, cfg.RouterID)
	h.BDR = [4]byte(peer)
	if reason := ifc.ReceiveHello(peer, h, time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if ifc.State() == StateWaiting || ifc.DR() == (types.RouterID{}) {
		t.Fatalf("backup seen did not elect: state=%s DR=%s BDR=%s", ifc.State(), ifc.DR(), ifc.BDR())
	}
}

func TestOSPFISMBackupSeenRequiresTwoWay(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	h := helloFor(cfg)
	h.BDR = [4]byte(peer)
	if reason := ifc.ReceiveHello(peer, h, time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if ifc.State() != StateWaiting || ifc.DR() != (types.RouterID{}) {
		t.Fatalf("one-way BackupSeen state=%s DR=%s, want still waiting", ifc.State(), ifc.DR())
	}
}

func TestOSPFISMBackupSeenIgnoresOtherRouterDR(t *testing.T) {
	// RFC 2328 sec 9.2: a Hello that names some OTHER router as DR (not the sender itself) with
	// no Backup DR must NOT end the Waiting state -- only the sender declaring ITSELF DR or BDR
	// does. The previous condition fired on any non-zero DR/BDR, ending the wait too early.
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	peer := rid(t, "10.0.0.2")
	other := rid(t, "10.0.0.9")
	h := helloFor(cfg, cfg.RouterID) // two-way (lists our Router ID)
	h.DR = [4]byte(other)            // names a third router as DR, no Backup DR
	if reason := ifc.ReceiveHello(peer, h, time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if ifc.State() != StateWaiting {
		t.Fatalf("BackupSeen fired for an other-router DR (state=%s); want still Waiting", ifc.State())
	}
}

func TestOSPFDRElectionSticky(t *testing.T) {
	local := Candidate{RouterID: rid(t, "10.0.0.1"), Priority: 1, TwoWay: true, Self: true}
	sitting := Candidate{RouterID: rid(t, "10.0.0.2"), Priority: 1, TwoWay: true}
	sitting.DeclaredDR = sitting.RouterID
	joiner := Candidate{RouterID: rid(t, "10.0.0.3"), Priority: 255, TwoWay: true}
	res := electDRBDR([]Candidate{local, sitting, joiner})
	if res.DR != sitting.RouterID {
		t.Fatalf("DR=%s, want sitting DR %s", res.DR, sitting.RouterID)
	}
}

func TestOSPFDRElectionHigherPriorityJoinsNoDisplace(t *testing.T) {
	local := Candidate{RouterID: rid(t, "10.0.0.1"), Priority: 1, TwoWay: true, Self: true}
	sitting := Candidate{RouterID: rid(t, "10.0.0.2"), Priority: 1, TwoWay: true}
	sitting.DeclaredDR = [4]byte(sitting.RouterID)
	joiner := Candidate{RouterID: rid(t, "10.0.0.3"), Priority: 255, TwoWay: true}
	res := electDRBDR([]Candidate{local, sitting, joiner})
	if res.DR != sitting.RouterID {
		t.Fatalf("DR=%s, want sitting DR %s", res.DR, sitting.RouterID)
	}
}

func TestOSPFDRElectionBDRPromotedOnDRLoss(t *testing.T) {
	bdr := Candidate{RouterID: rid(t, "10.0.0.2"), Priority: 1, TwoWay: true}
	bdr.DeclaredBDR = [4]byte(bdr.RouterID)
	other := Candidate{RouterID: rid(t, "10.0.0.1"), Priority: 1, TwoWay: true, Self: true}
	res := electDRBDR([]Candidate{bdr, other})
	if res.DR != bdr.RouterID || res.BDR != other.RouterID {
		t.Fatalf("DR=%s BDR=%s, want former BDR promoted and other backup", res.DR, res.BDR)
	}
}

func TestOSPFDRElectionPriorityZeroIneligible(t *testing.T) {
	zero := Candidate{RouterID: rid(t, "10.0.0.9"), Priority: 0, TwoWay: true}
	one := Candidate{RouterID: rid(t, "10.0.0.1"), Priority: 1, TwoWay: true, Self: true}
	res := electDRBDR([]Candidate{zero, one})
	if res.DR != one.RouterID || res.BDR != (types.RouterID{}) {
		t.Fatalf("election = DR %s BDR %s, want only priority-1 self", res.DR, res.BDR)
	}
}

func TestOSPFISMPriorityZeroLeavesWaitingAsDROther(t *testing.T) {
	cfg := baseConfig(t)
	cfg.Priority = 0
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StateDROther {
		t.Fatalf("initial priority-zero state = %s, want DROther", ifc.State())
	}
	ifc.forceWaitTimer()
	if ifc.State() != StateDROther || ifc.DR() != (types.RouterID{}) || ifc.BDR() != (types.RouterID{}) {
		t.Fatalf("state=%s DR=%s BDR=%s, want DROther with no elected roles", ifc.State(), ifc.DR(), ifc.BDR())
	}
}

func TestOSPFHelloTwoWayDetection(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")
	if reason := ifc.ReceiveHello(peer, helloFor(cfg), time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if ifc.neighbors[peer].TwoWay {
		t.Fatal("neighbor is two-way without our router-id in Hello")
	}
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), time.Now()); reason != "" {
		t.Fatalf("ReceiveHello two-way: %s", reason)
	}
	if !ifc.neighbors[peer].TwoWay {
		t.Fatal("neighbor not marked two-way")
	}
}

func TestOSPFHelloMismatches(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")
	h := helloFor(cfg)
	h.HelloInterval++
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != "hello-interval" {
		t.Fatalf("mismatch reason = %q, want hello-interval", got)
	}
	h = helloFor(cfg)
	h.NetworkMask = [4]byte{255, 255, 0, 0}
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != "network-mask" {
		t.Fatalf("mismatch reason = %q, want network-mask", got)
	}
	h = helloFor(cfg)
	h.DeadInterval++
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != "dead-interval" {
		t.Fatalf("mismatch reason = %q, want dead-interval", got)
	}
	h = helloFor(cfg)
	h.Options = 0
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != DropReasonOptionsE {
		t.Fatalf("mismatch reason = %q, want %s", got, DropReasonOptionsE)
	}
}

func TestOSPFHelloOriginationFields(t *testing.T) {
	cfg := baseConfig(t)
	sender := &fakeSender{}
	ifc := New(cfg, sender, NopMetrics())
	peer := rid(t, "10.0.0.2")
	if reason := ifc.ReceiveHello(peer, helloFor(cfg), time.Now()); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if err := ifc.SendHello(); err != nil {
		t.Fatalf("SendHello: %v", err)
	}
	p, err := packet.DecodePacket(sender.sent[0])
	if err != nil {
		t.Fatalf("DecodePacket: %v", err)
	}
	if p.Hello == nil || p.Hello.HelloInterval != cfg.HelloInterval || p.Hello.DeadInterval != uint32(cfg.DeadInterval) || !p.Hello.Options.Has(types.OptionE) || len(p.Hello.Neighbors) != 1 || p.Hello.Neighbors[0] != peer {
		t.Fatalf("hello = %+v", p.Hello)
	}
}

func TestOSPFHelloEbitSetNormalClearStub(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	p, err := packet.DecodePacket(ifc.buildHelloPacket())
	if err != nil {
		t.Fatal(err)
	}
	if !p.Hello.Options.Has(types.OptionE) {
		t.Fatal("normal area Hello missing E-bit")
	}
	cfg.AreaType = AreaStub
	ifc = New(cfg, &fakeSender{}, NopMetrics())
	p, err = packet.DecodePacket(ifc.buildHelloPacket())
	if err != nil {
		t.Fatal(err)
	}
	if p.Hello.Options.Has(types.OptionE) {
		t.Fatal("stub area Hello has E-bit")
	}
}

func TestOSPFPassiveInterfaceNoHello(t *testing.T) {
	cfg := baseConfig(t)
	cfg.Passive = true
	sender := &fakeSender{}
	ifc := New(cfg, sender, NopMetrics())
	ifc.Start()
	defer ifc.Stop()
	if ifc.State() != StateDown {
		t.Fatalf("passive state = %s, want down", ifc.State())
	}
	if err := ifc.SendHello(); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("passive sent %d Hellos", len(sender.sent))
	}
}

func TestOSPFNeighborInactivityReElection(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")
	now := time.Now()
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), now); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if removed := ifc.expireNeighbors(now.Add(41 * time.Second)); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if ifc.Snapshot().NeighborCount != 0 {
		t.Fatal("neighbor not expired")
	}
}

func TestOSPFNeighborInactivityDelayTracksDeadline(t *testing.T) {
	cfg := baseConfig(t)
	cfg.DeadInterval = 40
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")
	now := time.Unix(1000, 0)
	if reason := ifc.ReceiveHello(peer, helloFor(cfg, cfg.RouterID), now); reason != "" {
		t.Fatalf("ReceiveHello: %s", reason)
	}
	if got := ifc.nextInactivityDelay(now.Add(39*time.Second), time.Duration(cfg.DeadInterval)*time.Second); got != time.Second {
		t.Fatalf("next inactivity delay = %s, want 1s", got)
	}
}

func TestOSPFInterfaceSnapshot(t *testing.T) {
	cfg := baseConfig(t)
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	snap := ifc.Snapshot()
	if snap.Name != "eth0" || snap.Area != "0.0.0.0" || snap.HelloInterval != 10 || snap.DeadInterval != 40 || snap.Cost != 10 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.Passive {
		t.Fatalf("default interface snapshot must not be passive: %+v", snap)
	}

	// AC-3: `show ospf interface` must surface the passive flag, so the Snapshot the
	// renderer consumes carries it.
	pcfg := baseConfig(t)
	pcfg.Passive = true
	if psnap := New(pcfg, &fakeSender{}, NopMetrics()).Snapshot(); !psnap.Passive {
		t.Fatalf("passive interface snapshot must report passive: %+v", psnap)
	}
}

// TestOSPFv6InterfaceSkipsNetworkMaskCheck drives the AF-aware Hello validation: OSPFv3 Hellos
// carry an Interface ID, not a Network Mask, so a v6 interface must skip the OSPFv2-only mask
// match (RFC 2328 sec 10.5). The same zero-mask Hello on a v4 broadcast interface is dropped.
func TestOSPFv6InterfaceSkipsNetworkMaskCheck(t *testing.T) {
	cfg := baseConfig(t)
	h := helloFor(cfg)
	h.NetworkMask = [4]byte{} // an OSPFv3 Hello has no Network Mask

	cfg.IsV6 = true
	if reason := New(cfg, &fakeSender{}, NopMetrics()).validateHelloLocked(h); reason != "" {
		t.Fatalf("v6 interface must skip the Network Mask check, got drop %q", reason)
	}

	cfg.IsV6 = false
	if reason := New(cfg, &fakeSender{}, NopMetrics()).validateHelloLocked(h); reason != "network-mask" {
		t.Fatalf("v4 broadcast interface must drop a mismatched Network Mask, got %q", reason)
	}
}

// TestHelloCarriesInstanceID proves AC-5 / A-5 (RFC 6549 sec 3 on transmit): the default
// OSPFv2 Hello encoder built from Config.InstanceID stamps the engine's Instance ID into
// the common header (offset 14) of every Hello; the base instance 0 leaves it zero.
func TestHelloCarriesInstanceID(t *testing.T) {
	for _, id := range []uint8{0, 5, 255} {
		cfg := baseConfig(t)
		cfg.InstanceID = id
		i := New(cfg, &fakeSender{}, NopMetrics())
		h, _, err := packet.DecodeHeader(i.buildHelloPacket())
		if err != nil {
			t.Fatalf("id %d: DecodeHeader: %v", id, err)
		}
		if h.InstanceID != id {
			t.Fatalf("Hello Instance ID = %d, want %d", h.InstanceID, id)
		}
		if h.Type != packet.PacketTypeHello {
			t.Fatalf("id %d: decoded type = %v, want Hello", id, h.Type)
		}
	}
}
