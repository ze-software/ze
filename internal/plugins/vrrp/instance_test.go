// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- per-instance worker (FSM executor) tests
//
// VALIDATES: the action-execution contract spec-vrrp-2 hands the engine -- every
// action value is executed, in order, with the right side effect; timer Gen is
// echoed back so the FSM can reject stale expiries; rx packets are decoded by
// the ENGINE (D-B) and mapped to reason labels.
// PREVENTS: holo's dead-counter and stale-advert bug classes, and the
// split-brain that an empty re-registration (instead of Unregister) would cause.

package vrrp

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
)

// fakeDeps records what the worker asked the outside world to do.
type fakeDeps struct {
	mu sync.Mutex

	adverts     []sentAdvert
	announces   int
	installs    []installCall
	removes     []string
	rxErrors    []string
	transitions []string
}

type sentAdvert struct {
	priority   uint8
	intervalMs int
}

type installCall struct {
	dev   string
	owner string
	cidrs []string
}

func (f *fakeDeps) deps() engineDeps {
	return engineDeps{
		sendAdvert: func(_ transport.InstanceKey, priority uint8, intervalMs int) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.adverts = append(f.adverts, sentAdvert{priority: priority, intervalMs: intervalMs})
			return nil
		},
		updateAdvert: func(transport.InstanceKey, transport.AdvertParams) error { return nil },
		announceMaster: func(transport.InstanceKey, []netip.Addr) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.announces++
		},
		installVIPs: func(dev, owner string, cidrs []string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.installs = append(f.installs, installCall{dev: dev, owner: owner, cidrs: cidrs})
			return nil
		},
		removeVIPs: func(owner string) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.removes = append(f.removes, owner)
		},
		recordRxError: func(_ transport.InstanceKey, reason string) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.rxErrors = append(f.rxErrors, reason)
		},
		emitState: func(_ GroupSpec, from, to fsm.State, reason string) {
			f.mu.Lock()
			defer f.mu.Unlock()
			var tb []byte
			tb = append(tb, from.String()...)
			tb = append(tb, "->"...)
			tb = append(tb, to.String()...)
			tb = append(tb, ':')
			tb = append(tb, reason...)
			f.transitions = append(f.transitions, string(tb))
		},
	}
}

func (f *fakeDeps) snapshot() fakeDeps {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fakeDeps{
		adverts:     append([]sentAdvert(nil), f.adverts...),
		announces:   f.announces,
		installs:    append([]installCall(nil), f.installs...),
		removes:     append([]string(nil), f.removes...),
		rxErrors:    append([]string(nil), f.rxErrors...),
		transitions: append([]string(nil), f.transitions...),
	}
}

func testSpec() GroupSpec {
	return GroupSpec{
		IfType:           "ethernet",
		Interface:        "eth0",
		Unit:             "0",
		ParentDevice:     "eth0", // plain unit: device == interface
		Family:           familyIPv4,
		Name:             "uplink",
		VRID:             10,
		VIPs:             []netip.Addr{netip.MustParseAddr("192.0.2.1")},
		Priority:         200,
		Preempt:          true,
		AdvertIntervalMs: 1000,
		Version:          versionV3,
	}
}

// newTestInstance builds a worker over a fake clock and fake deps, without
// starting run() (tests drive dispatch directly for determinism).
func newTestInstance(t *testing.T, spec GroupSpec) (*instance, *fakeDeps, *sim.FakeClock) {
	t.Helper()
	f := &fakeDeps{}
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	in := newInstance(spec, transport.InstanceKey{Interface: spec.Interface, VRID: spec.VRID, Family: packet.V4}, "zv4-2-10", clk, f.deps())
	return in, f, clk
}

// TestInstanceReadinessGatesStartup proves an instance does NOT start while its
// parent is down or address-less, and starts once the parent is usable.
//
// RFC 9568 Section 6.4.1: a virtual router only leaves Initialize on a Startup
// event. Advertising from a dead parent would claim mastership of a link we
// cannot actually serve.
func TestInstanceReadinessGatesStartup(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = true // would go straight to Master if it started
	f := &fakeDeps{}
	ready := &atomic.Bool{}
	deps := f.deps()
	deps.parentReady = func(string, string) bool { return ready.Load() }
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	in := newInstance(spec, transport.InstanceKey{Interface: "eth0", VRID: 10, Family: packet.V4}, "zv4-2-10", clk, deps)

	// Parent not ready: nothing happens.
	in.evaluateReadiness()
	if in.machine.State() != fsm.StateInitialize {
		t.Fatalf("state = %v with an unusable parent, want Initialize", in.machine.State())
	}
	if got := f.snapshot(); len(got.adverts) != 0 || len(got.installs) != 0 {
		t.Fatalf("an instance with an unusable parent must not advertise or install VIPs: adverts=%d installs=%d",
			len(got.adverts), len(got.installs))
	}

	// Parent becomes usable: the instance starts.
	ready.Store(true)
	in.evaluateReadiness()
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("state = %v after the parent became ready, want Master (owner)", in.machine.State())
	}
}

// TestInstanceParentDownStopsInstance proves a parent going down tears the
// virtual router down: VIPs released, timers canceled.
//
// This is THE reason readiness keys on the parent. Measured in QEMU
// (spec-vrrp-3 A-4, broken): the kernel leaves a macvlan's oper-state UP
// forever when its parent goes down, so watching the macvlan would never notice
// a dead link and this router would keep claiming mastership of a link it
// cannot serve.
func TestInstanceParentDownStopsInstance(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = true
	f := &fakeDeps{}
	ready := &atomic.Bool{}
	ready.Store(true)
	deps := f.deps()
	deps.parentReady = func(string, string) bool { return ready.Load() }
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	in := newInstance(spec, transport.InstanceKey{Interface: "eth0", VRID: 10, Family: packet.V4}, "zv4-2-10", clk, deps)

	in.evaluateReadiness()
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("state = %v, want Master before the parent fails", in.machine.State())
	}

	ready.Store(false)
	in.evaluateReadiness()

	if in.machine.State() != fsm.StateInitialize {
		t.Fatalf("state = %v after parent down, want Initialize", in.machine.State())
	}
	got := f.snapshot()
	if len(got.removes) == 0 {
		t.Error("parent down must release the virtual addresses")
	}
	if in.advert != nil || in.masterDown != nil {
		t.Error("parent down must cancel the timers")
	}
	// A Master that loses its parent still owes its peers a resignation.
	last := got.adverts[len(got.adverts)-1]
	if last.priority != 0 {
		t.Errorf("last advert priority = %d, want 0 (resignation on parent loss)", last.priority)
	}

	// The parent returns: the instance comes back without a recreate.
	ready.Store(true)
	in.evaluateReadiness()
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("state = %v after the parent returned, want Master", in.machine.State())
	}
}

// TestInstanceReadinessIsIdempotent proves repeated link events (the iface
// monitor fires on any change) do not restart a running instance.
func TestInstanceReadinessIsIdempotent(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = true
	f := &fakeDeps{}
	deps := f.deps()
	deps.parentReady = func(string, string) bool { return true }
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	in := newInstance(spec, transport.InstanceKey{Interface: "eth0", VRID: 10, Family: packet.V4}, "zv4-2-10", clk, deps)

	in.evaluateReadiness()
	first := len(f.snapshot().adverts)
	in.evaluateReadiness()
	in.evaluateReadiness()

	if got := len(f.snapshot().adverts); got != first {
		t.Fatalf("adverts = %d after repeated link events, want %d: a running instance must not restart", got, first)
	}
}

// TestInstanceRefreshesParentAddresses proves a link change re-resolves the
// parent's source address in the transport.
//
// RFC 9568 Section 7.2: an IPv4 advertisement is sourced from the sending
// interface's primary address. The transport caches it, so an address change on
// the parent must invalidate that cache or we would advertise from an address
// the interface no longer holds.
func TestInstanceRefreshesParentAddresses(t *testing.T) {
	spec := testSpec()
	f := &fakeDeps{}
	deps := f.deps()
	deps.parentReady = func(string, string) bool { return true }
	refreshed := &atomic.Int32{}
	deps.refreshAddresses = func(transport.InstanceKey) { refreshed.Add(1) }
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	in := newInstance(spec, transport.InstanceKey{Interface: "eth0", VRID: 10, Family: packet.V4}, "zv4-2-10", clk, deps)

	in.evaluateReadiness()
	if refreshed.Load() == 0 {
		t.Fatal("a link change must refresh the transport's cached parent address")
	}
}

// TestInstanceOwnerString pins the per-instance owner identity (D-1): a shared
// owner could not drop one instance's VIPs without dropping every instance's.
func TestInstanceOwnerString(t *testing.T) {
	in, _, _ := newTestInstance(t, testSpec())
	if in.own != "vrrp:zv4-2-10" {
		t.Fatalf("owner = %q, want vrrp:zv4-2-10", in.own)
	}
}

// TestInstanceStartupNonOwnerGoesBackup proves a non-owner arms the master-down
// timer and installs nothing (RFC 9568 Section 6.4.1).
func TestInstanceStartupNonOwnerGoesBackup(t *testing.T) {
	// A Backup installs no virtual address, so its virtual-MAC macvlan holds no VIP;
	// with no VIP the kernel answers no ARP for it and locally accepts nothing
	// addressed to it (the dataplane realization is proven under QEMU in
	// docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md). The Master contrast lives in
	// TestInstanceOwnerStartupGoesMaster.
	// RFC requirement: RFC3768-6.4.2-1 positive -- a Backup installs no VIP, so the kernel never answers ARP requests for the virtual address (doInstallVIPs runs only on Master, instance.go:369).
	// RFC requirement: RFC3768-6.4.2-2 positive -- with no VIP installed a Backup does not locally accept frames delivered to the virtual-MAC device.
	// RFC requirement: RFC3768-6.4.2-3 positive -- a Backup accepts no packets addressed to the virtual IP because the address is never installed on it.
	// RFC requirement: RFC3768-6.4.3-2 negative -- contrast: a Backup does NOT install the VIP, so it does not process traffic for the virtual MAC (a Master does).
	// RFC requirement: RFC3768-6.4.3-4 negative -- contrast: a Backup does NOT accept packets for the virtual IP (a Master installs the VIP and accepts).
	// RFC requirement: RFC9568-6.4.2-1 positive -- a Backup installs no virtual address, so the kernel answers no ARP request for it (doInstallVIPs runs only on an Active transition, instance.go:369)
	// RFC requirement: RFC9568-6.4.2-4 positive -- with no virtual address installed, frames delivered to the Virtual Router MAC device are not processed locally by a Backup
	// RFC requirement: RFC9568-6.4.2-5 positive -- a Backup accepts no packet addressed to the virtual IPvX address, because the address is never installed on it
	// RFC requirement: RFC9568-6.4.3-5 negative -- contrast: a Backup does NOT install the virtual address, so it does not process traffic for the Virtual Router MAC (an Active router does)
	// RFC requirement: RFC9568-6.4.3-6 negative -- contrast: a Backup does NOT accept packets addressed to the virtual IPvX address (the Active owner installs the address and accepts).
	in, f, _ := newTestInstance(t, testSpec())
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	got := f.snapshot()
	if len(got.installs) != 0 {
		t.Errorf("Backup must not install VIPs, got %+v", got.installs)
	}
	if len(got.adverts) != 0 {
		t.Errorf("Backup must not advertise, got %+v", got.adverts)
	}
	if in.masterDown == nil {
		t.Error("master-down timer must be armed in Backup")
	}
	if in.machine.State() != fsm.StateBackup {
		t.Errorf("state = %v, want Backup", in.machine.State())
	}
}

// TestInstanceOwnerStartupGoesMaster proves the address owner takes over
// immediately: advert, VIP install, and failover announcement.
//
// RFC 9568 Section 6.4.1: an owner (priority 255) transitions straight to
// Active on startup.
func TestInstanceOwnerStartupGoesMaster(t *testing.T) {
	// A Master installs the VIP on its virtual-MAC macvlan (installs[0].dev is the
	// vMAC device), which is how ze makes the kernel forward/accept traffic for the
	// virtual MAC and virtual IP; the Backup contrast lives in
	// TestInstanceStartupNonOwnerGoesBackup.
	// RFC requirement: RFC3768-6.4.3-2 positive -- the Master installs the VIP on its virtual-MAC macvlan, so the kernel delivers/processes frames addressed to the virtual MAC (instance.go:369; createMacvlan register.go:329).
	// RFC requirement: RFC3768-6.4.3-4 positive -- the owner Master installs the VIP (a real parent address), so the kernel accepts packets addressed to the virtual IP.
	// RFC requirement: RFC3768-6.4.2-1 negative -- contrast: a Master DOES own the VIP on the vMAC device, so the Backup ARP-non-response is state-specific, not a blanket refusal.
	// RFC requirement: RFC3768-6.4.2-2 negative -- contrast: a Master DOES accept frames delivered to the virtual MAC (VIP installed).
	// RFC requirement: RFC3768-6.4.2-3 negative -- contrast: a Master DOES accept packets addressed to the virtual IP (VIP installed).
	// RFC requirement: RFC9568-6.4.3-5 positive -- the Active router installs the virtual address on its Virtual Router MAC macvlan, so the kernel processes frames addressed to that MAC (instance.go:369; createMacvlan register.go:329)
	// RFC requirement: RFC9568-6.4.3-6 positive -- the Active router that owns the address installs it, so packets addressed to the virtual IPvX address are accepted (instance.go:369)
	// RFC requirement: RFC9568-6.4.2-1 negative -- contrast: an Active router DOES own the virtual address on the vMAC device, so the Backup ARP silence is state-specific, not a blanket refusal
	// RFC requirement: RFC9568-6.4.2-4 negative -- contrast: an Active router DOES process frames delivered to the Virtual Router MAC (address installed)
	// RFC requirement: RFC9568-6.4.2-5 negative -- contrast: an Active router DOES accept packets addressed to the virtual IPvX address (address installed).
	spec := testSpec()
	spec.IsOwner = true
	in, f, _ := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	got := f.snapshot()
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("owner state = %v, want Master", in.machine.State())
	}
	if len(got.adverts) == 0 || got.adverts[0].priority != 255 {
		t.Errorf("owner must advertise with priority 255, got %+v", got.adverts)
	}
	if len(got.installs) != 1 || got.installs[0].owner != in.own || got.installs[0].dev != in.dev {
		t.Errorf("owner must install VIPs on its macvlan under its own owner string, got %+v", got.installs)
	}
	if got.announces != 1 {
		t.Errorf("announcements = %d, want 1 (GARP/NA burst on promotion)", got.announces)
	}
	if in.advert == nil {
		t.Error("advert timer must be armed in Master")
	}
}

// TestInstanceTimerGenEcho proves the worker echoes the arming action's Gen back
// to the FSM, and that a STALE expiry (an old Gen, as a late timer callback
// would deliver) is ignored rather than promoting the router.
func TestInstanceTimerGenEcho(t *testing.T) {
	in, f, clk := newTestInstance(t, testSpec())
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	// A stale expiry (Gen 0 was never armed) must not promote.
	in.dispatch(fsm.MasterDownExpired{Gen: 0})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("stale master-down expiry promoted the router: state = %v", in.machine.State())
	}
	if len(f.snapshot().adverts) != 0 {
		t.Fatal("stale expiry must not advertise")
	}

	// The real timer callback carries the armed Gen and DOES promote.
	clk.Add(10 * time.Second)
	deadline := time.After(2 * time.Second)
	for in.machine.State() != fsm.StateMaster {
		select {
		case ev := <-in.events:
			in.dispatch(ev)
		case <-deadline:
			t.Fatal("master-down timer never fired with a matching Gen")
		}
	}
	got := f.snapshot()
	if len(got.installs) != 1 {
		t.Errorf("promotion must install VIPs, got %+v", got.installs)
	}
	if got.announces != 1 {
		t.Errorf("promotion must announce failover, got %d", got.announces)
	}
}

// TestInstanceShutdownAsMasterSendsPriorityZero proves the resignation
// advertisement.
//
// RFC 9568 Section 6.4.3: the Active router sends a Priority-0 advertisement on
// shutdown so Backups promote immediately instead of waiting out master-down.
func TestInstanceShutdownAsMasterSendsPriorityZero(t *testing.T) {
	// RFC requirement: RFC3768-6.4.3-5 positive -- the executor of a Master Shutdown actually sends a Priority-0 ADVERTISEMENT and unregisters the VIP owner before Initialize (execute SendAdvertZeroPriority instance.go:307).
	// RFC requirement: RFC9568-6.4.3-8 positive -- the executor of an Active Shutdown actually sends a Priority-0 ADVERTISEMENT on the wire and releases the virtual address before Initialize (execute SendAdvertZeroPriority instance.go:307).
	spec := testSpec()
	spec.IsOwner = true
	in, f, _ := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	in.dispatch(fsm.Shutdown{})

	got := f.snapshot()
	last := got.adverts[len(got.adverts)-1]
	if last.priority != 0 {
		t.Fatalf("last advert priority = %d, want 0 (resignation)", last.priority)
	}
	if in.prio0Sent != 1 {
		t.Errorf("prio0Sent = %d, want 1 (engine-owned counter, D-F)", in.prio0Sent)
	}
	if len(got.removes) != 1 || got.removes[0] != in.own {
		t.Errorf("shutdown must unregister the owner (not re-register empty), got %+v", got.removes)
	}
}

// TestInstanceRxDecodeErrorMapsReason proves the engine decodes rx packets (D-B)
// and maps failures to the metrics reason label rather than dropping silently.
func TestInstanceRxDecodeErrorMapsReason(t *testing.T) {
	// RFC requirement: RFC3768-7.1-6 negative -- a packet failing any receive check is discarded: the reason is recorded and the packet never reaches the FSM (onPacket instance.go:458).
	// RFC requirement: RFC9568-7.1-6 negative -- a v3 packet failing a mandatory receive check is discarded: the reason is recorded and the packet never reaches the state machine (onPacket instance.go:458).
	in, f, _ := newTestInstance(t, testSpec())
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	// Truncated payload: fails the codec's first ladder row.
	in.onPacket(transport.RxItem{
		Meta:    packet.RxMeta{TTL: 255, Family: packet.V4, Src: netip.MustParseAddr("192.0.2.9"), Dst: packet.MulticastV4},
		Payload: []byte{0x31, 0x0a},
	})
	got := f.snapshot()
	if len(got.rxErrors) != 1 || got.rxErrors[0] == "" {
		t.Fatalf("malformed packet must record a reason, got %+v", got.rxErrors)
	}
}

// TestInstanceRxValidAdvertReachesFSM proves a well-formed advert from a higher
// priority peer is decoded and re-arms the Backup's master-down timer.
func TestInstanceRxValidAdvertReachesFSM(t *testing.T) {
	// RFC requirement: RFC3768-7.1-6 positive -- a packet passing every receive check is NOT discarded; it decodes and reaches the FSM (onPacket instance.go:457).
	// RFC requirement: RFC9568-7.1-6 positive -- a v3 packet passing every mandatory receive check is NOT discarded; it decodes and reaches the state machine (onPacket instance.go:457).
	in, _, _ := newTestInstance(t, testSpec())
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	adv := packet.Advertisement{
		Version:         packet.VersionV3,
		Family:          packet.V4,
		VRID:            10,
		Priority:        250, // higher than ours: we stay Backup
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{netip.MustParseAddr("192.0.2.1")},
	}
	var buf [packet.MaxLenV3v4]byte
	n := adv.WriteTo(buf[:], 0)
	src := netip.MustParseAddr("192.0.2.9")
	packet.FillChecksum(buf[:], 0, n, src, packet.MulticastV4)

	in.onPacket(transport.RxItem{
		Meta:    packet.RxMeta{TTL: 255, Family: packet.V4, Src: src, Dst: packet.MulticastV4},
		Payload: buf[:n],
	})

	select {
	case ev := <-in.events:
		got, ok := ev.(fsm.AdvertReceived)
		if !ok {
			t.Fatalf("event = %T, want fsm.AdvertReceived", ev)
		}
		if got.Priority != 250 || got.SrcIP != src || got.IntervalMs != 1000 {
			t.Fatalf("decoded advert wrong: %+v", got)
		}
	default:
		t.Fatal("valid advert produced no FSM event")
	}
}

// TestInstanceV2AddressListMismatchDrops proves the v2-only address-list check.
//
// RFC 3768 Section 7.1: a VRRPv2 router MUST discard an advertisement whose
// address list differs from its own. VRRPv3 dropped this, so v3 must NOT drop.
func TestInstanceV2AddressListMismatchDrops(t *testing.T) {
	// RFC requirement: RFC3768-7.1-7 negative -- a v2 advert whose address list differs from the configured VIPs is dropped and never fed to the FSM (onPacket instance.go:473; addressListMatches instance.go:490). Note: ze applies the drop uniformly, stricter than the RFC carve-out that lets a Priority-255 owner-sender continue on mismatch.
	spec := testSpec()
	spec.Version = versionV2
	in, f, _ := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	other := packet.Advertisement{
		Version:         packet.VersionV2,
		Family:          packet.V4,
		VRID:            10,
		Priority:        250,
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{netip.MustParseAddr("192.0.2.77")}, // not ours
	}
	var buf [packet.MaxLenV2]byte
	n := other.WriteTo(buf[:], 0)
	src := netip.MustParseAddr("192.0.2.9")
	packet.FillChecksum(buf[:], 0, n, src, packet.MulticastV4)

	in.onPacket(transport.RxItem{
		Meta:    packet.RxMeta{TTL: 255, Family: packet.V4, Src: src, Dst: packet.MulticastV4},
		Payload: buf[:n],
	})

	got := f.snapshot()
	if len(got.rxErrors) != 1 || got.rxErrors[0] != packet.ReasonAddressList {
		t.Fatalf("v2 address-list mismatch must record %q, got %+v", packet.ReasonAddressList, got.rxErrors)
	}
	select {
	case ev := <-in.events:
		t.Fatalf("mismatched v2 advert must not reach the FSM, got %T", ev)
	default:
	}
}

// TestInstanceV2AddressListMatchReachesFSM is the positive of the v2 address-list
// check: an advert whose address list matches the configured VIPs passes the check
// and is delivered to the FSM.
func TestInstanceV2AddressListMatchReachesFSM(t *testing.T) {
	// RFC requirement: RFC3768-7.1-7 positive -- a v2 advert whose address list matches the configured VIPs passes the address-list check and reaches the FSM (onPacket instance.go:473; addressListMatches instance.go:490).
	spec := testSpec()
	spec.Version = versionV2
	in, f, _ := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	adv := packet.Advertisement{
		Version:         packet.VersionV2,
		Family:          packet.V4,
		VRID:            10,
		Priority:        250, // higher than ours: we stay Backup and the advert is processed
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{netip.MustParseAddr("192.0.2.1")}, // matches testSpec VIPs
	}
	var buf [packet.MaxLenV2]byte
	n := adv.WriteTo(buf[:], 0)
	src := netip.MustParseAddr("192.0.2.9")
	packet.FillChecksum(buf[:], 0, n, src, packet.MulticastV4)

	in.onPacket(transport.RxItem{
		Meta:    packet.RxMeta{TTL: 255, Family: packet.V4, Src: src, Dst: packet.MulticastV4},
		Payload: buf[:n],
	})

	if got := f.snapshot(); len(got.rxErrors) != 0 {
		t.Fatalf("matching v2 address list must not record an rx error, got %+v", got.rxErrors)
	}
	select {
	case ev := <-in.events:
		if _, ok := ev.(fsm.AdvertReceived); !ok {
			t.Fatalf("event = %T, want fsm.AdvertReceived", ev)
		}
	default:
		t.Fatal("matching v2 advert produced no FSM event")
	}
}

// TestInstanceReconfigureDoesNotRestart proves a config change re-applies to the
// running FSM instead of tearing the instance down (a restart would drop
// mastership and blackhole traffic for a full master-down interval).
//
// reconfigure applies SYNCHRONOUSLY under the instance lock rather than queuing
// an event: spec and FSM config must change together, or an advert sent between
// the two would mix priorities from one config with VIPs from the other.
func TestInstanceReconfigureDoesNotRestart(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = true
	in, _, _ := newTestInstance(t, spec)
	in.startup()
	before := in.machine.State()

	next := spec
	next.Priority = 150
	in.reconfigure(next)

	if in.machine.State() != before {
		t.Fatalf("reconfigure changed state %v -> %v; it must re-apply in place", before, in.machine.State())
	}
	if in.spec.Priority != 150 {
		t.Errorf("spec not updated: priority = %d, want 150", in.spec.Priority)
	}
	select {
	case ev := <-in.events:
		t.Fatalf("reconfigure must apply synchronously, not queue %T", ev)
	default:
	}
}

// testSpecV6 is the IPv6 counterpart of testSpec: a VRRPv3 group whose first
// virtual address is the Virtual Router's link-local (RFC 9568 Section 5.2.9).
func testSpecV6() GroupSpec {
	s := testSpec()
	s.Family = familyIPv6
	s.VIPs = []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::1")}
	return s
}

// TestInstanceIPv6VIPLivesOnVirtualMACDevice proves where an IPv6 virtual
// address lives, which is what decides who answers Neighbor Solicitations for
// it: only an Active router installs it, and it is installed on the Virtual
// Router MAC macvlan, never on the parent (whose MAC is the physical one).
//
// RFC requirement: RFC9568-6.4.2-2 positive -- a Backup installs no IPv6 virtual address, so the kernel answers no Neighbor Solicitation for it (doInstallVIPs runs only on an Active transition, instance.go:369)
// RFC requirement: RFC9568-6.4.2-2 negative -- contrast: an Active router DOES install the address, so the Backup ND silence is state-specific, not a blanket refusal
// RFC requirement: RFC9568-6.4.3-2 positive -- the Active router installs the IPv6 virtual address on the vMAC macvlan, which is what makes the kernel join that address's Solicited-Node multicast group on that device (instance.go:369; createMacvlan register.go:329)
// RFC requirement: RFC9568-8.2.2-1 positive -- the virtual address is installed on the Virtual Router MAC device (dev == in.dev) and never on the parent, so a Neighbor Advertisement for it can only carry the virtual MAC (instance.go:369).
func TestInstanceIPv6VIPLivesOnVirtualMACDevice(t *testing.T) {
	// Backup: nothing installed.
	in, f, _ := newTestInstance(t, testSpecV6())
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("state = %v, want Backup", in.machine.State())
	}
	if got := f.snapshot(); len(got.installs) != 0 {
		t.Fatalf("a Backup must install no IPv6 virtual address, got %+v", got.installs)
	}

	// Active (address owner): installed, on the virtual-MAC device only.
	spec := testSpecV6()
	spec.IsOwner = true
	inM, fM, _ := newTestInstance(t, spec)
	inM.dispatch(fsm.Startup{Config: inM.fsmConfig()})
	if inM.machine.State() != fsm.StateMaster {
		t.Fatalf("owner state = %v, want Master", inM.machine.State())
	}
	got := fM.snapshot()
	if len(got.installs) != 1 {
		t.Fatalf("the Active router must install its IPv6 virtual addresses, got %+v", got.installs)
	}
	if got.installs[0].dev != inM.dev {
		t.Fatalf("install device = %q, want the virtual-MAC macvlan %q (never the parent)", got.installs[0].dev, inM.dev)
	}
	if got.installs[0].dev == spec.ParentDevice {
		t.Fatalf("the IPv6 virtual address must never be installed on the parent %q", spec.ParentDevice)
	}
	if len(got.installs[0].cidrs) != len(spec.VIPs) {
		t.Fatalf("installed %d addresses, want %d", len(got.installs[0].cidrs), len(spec.VIPs))
	}
}

// TestInstanceDelaysAnnounceUntilParentUsable proves the boot ordering VRRP
// depends on: no LAN announcement leaves this router until the virtual address
// and the Virtual Router MAC device are both in place, and when they are, the
// address is installed BEFORE the announcement is emitted.
//
// RFC requirement: RFC9568-8.1.2-2 positive -- for an IPv4 group the gratuitous ARP (AnnounceFailover) is emitted only after the virtual address is installed on the vMAC device, which the engine created and waited for before the instance existed (execute instance.go:312-317; waitDevicePresent register.go:395)
// RFC requirement: RFC9568-8.1.2-2 negative -- while the parent holds no usable IPv4 address the instance never starts, so no gratuitous ARP is emitted at boot (evaluateReadiness instance.go:218-238; parentReady register.go:436)
// RFC requirement: RFC9568-8.2.2-4 positive -- for an IPv6 group the unsolicited Neighbor Advertisement is emitted only after the virtual address is installed on the vMAC device (execute instance.go:312-317)
// RFC requirement: RFC9568-8.2.2-4 negative -- while the parent holds no usable IPv6 address the instance never starts, so no ND message is emitted at boot (evaluateReadiness instance.go:218-238; parentReady register.go:436).
func TestInstanceDelaysAnnounceUntilParentUsable(t *testing.T) {
	cases := []struct {
		name   string
		spec   GroupSpec
		family uint8
	}{
		{"ipv4", testSpec(), packet.V4},
		{"ipv6", testSpecV6(), packet.V6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.spec
			spec.IsOwner = true // an owner announces immediately once it starts

			var mu sync.Mutex
			var order []string
			f := &fakeDeps{}
			deps := f.deps()
			install, announce := deps.installVIPs, deps.announceMaster
			deps.installVIPs = func(dev, owner string, cidrs []string) error {
				mu.Lock()
				order = append(order, "install")
				mu.Unlock()
				return install(dev, owner, cidrs)
			}
			deps.announceMaster = func(key transport.InstanceKey, vips []netip.Addr) {
				mu.Lock()
				order = append(order, "announce")
				mu.Unlock()
				announce(key, vips)
			}
			ready := &atomic.Bool{}
			deps.parentReady = func(string, string) bool { return ready.Load() }

			clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
			in := newInstance(spec, transport.InstanceKey{Interface: spec.Interface, VRID: spec.VRID, Family: tc.family}, "zv-2-10", clk, deps)

			in.evaluateReadiness()
			mu.Lock()
			pre := append([]string(nil), order...)
			mu.Unlock()
			if len(pre) != 0 {
				t.Fatalf("nothing may be announced before the parent holds a usable address, got %v", pre)
			}

			ready.Store(true)
			in.evaluateReadiness()
			mu.Lock()
			post := append([]string(nil), order...)
			mu.Unlock()
			if len(post) < 2 || post[0] != "install" || post[1] != "announce" {
				t.Fatalf("announcement order = %v, want the address installed before it is announced", post)
			}
		})
	}
}

// TestInstanceSnapshotReportsEffectivePriority proves the show view reports both
// the configured and the effective priority for an owner (the RFC forces 255).
func TestInstanceSnapshotReportsEffectivePriority(t *testing.T) {
	spec := testSpec()
	spec.IsOwner = true
	spec.Priority = 120
	in, _, _ := newTestInstance(t, spec)
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})

	v := in.snapshot()
	if v.Priority != 120 {
		t.Errorf("configured priority = %d, want 120", v.Priority)
	}
	if v.EffectivePriority != 255 {
		t.Errorf("effective priority = %d, want 255 for the owner", v.EffectivePriority)
	}
	if !v.IsOwner || v.State != "master" {
		t.Errorf("snapshot wrong: %+v", v)
	}
}
