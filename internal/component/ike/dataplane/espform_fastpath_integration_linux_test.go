// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- one Child SA receives both ESP forms
// Related: espform_fastpath_linux.go -- espFormFastPath, the lifecycle under test
//
// VALIDATES: the fast path's attach lifecycle (AC-7 of
// spec-ipsec-dual-form-receive-spd-bypass): start loads the program and attaches it at
// the head of every Ethernet and L3 link, ahead of a program already there, leaves lo
// alone, refuses a tunnel by name, follows a link created later and a link deleted,
// writes and deletes watched SPIs in the map of their family, and stop detaches
// every link it attached and nothing else.
// PREVENTS: a link the subscription never attaches, a rewrite placed behind a program
// that answers TCX_PASS (it would never run), a TCX link left attached after the last
// forget, a stale map entry after a family change, and a stop that detaches another
// program's link.
// METHOD: a fresh network namespace on a locked thread, with a veth pair, a dummy, a
// TUN and an IPIP tunnel; the kernel's own TCX query (BPF_PROG_QUERY) reads what is
// attached, and the maps are read back by key.

//go:build linux

package dataplane

import (
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/link"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// Links the test creates in its own namespace.
const (
	fastPathTestVeth     = "zefp-veth0"
	fastPathTestVethPeer = "zefp-veth1"
	fastPathTestDummy    = "zefp-dummy0"
	fastPathTestLate     = "zefp-dummy1"
	fastPathTestTUN      = "zefp-tun0"
	fastPathTestTunnel   = "zefp-ipip0"
)

// fastPathTestWait bounds every wait for the worker to apply a link update.
const fastPathTestWait = 5 * time.Second

// TestESPFormFastPathAttachesEveryLink drives one fast path from start to stop over
// the links of a fresh namespace.
//
//ze:guest-root
func TestESPFormFastPathAttachesEveryLink(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: loading a SchedCLS program and attaching it through TCX need CAP_BPF and CAP_NET_ADMIN in the initial user namespace, which userns.Enter cannot grant; run `./le test qemu all`")
	}
	enterFastPathNetns(t)
	fastPathTestLinks(t)

	// A program already at the veth's ingress, answering TCX_PASS the way the
	// trafficusage counter does: the fast path must attach ahead of it.
	standIn, standInID := fastPathStandIn(t)
	veth := fastPathIfindex(t, fastPathTestVeth)
	standInLink, err := link.AttachTCX(link.TCXOptions{
		Interface: veth,
		Program:   standIn,
		Attach:    ebpf.AttachTCXIngress,
		Anchor:    link.Tail(),
	})
	if err != nil {
		t.Fatalf("attach the stand-in program to %s: %v", fastPathTestVeth, err)
	}
	t.Cleanup(func() { standInLink.Close() }) //nolint:errcheck // the namespace goes with the test

	f := newESPFormFastPath(nil)
	t.Cleanup(f.stop)
	f.start()

	st := waitForFastPath(t, f, "every link of the namespace attached or refused", func(st ESPFormFastPathState) bool {
		return fastPathHas(st.Attached, fastPathTestVeth, fastPathTestVethPeer, fastPathTestDummy, fastPathTestTUN) &&
			fastPathRefusal(st, fastPathTestTunnel) != nil
	})
	if st.Status != ESPFormFastPathAttached {
		t.Fatalf("status %s, want %s", st.Status, ESPFormFastPathAttached)
	}
	if slices.Contains(st.Attached, "lo") {
		t.Errorf("the fast path attached to lo: %v", st.Attached)
	}
	if slices.Contains(st.Attached, fastPathTestTunnel) {
		t.Errorf("the fast path attached to the tunnel %s, which it has no program for", fastPathTestTunnel)
	}
	if refusal := fastPathRefusal(st, fastPathTestTunnel); !strings.Contains(refusal.Reason, "ipip") {
		t.Errorf("the refusal of %s reads %q, want it to name the link type ipip", fastPathTestTunnel, refusal.Reason)
	}

	f.mu.Lock()
	ethernetID := fastPathProgramID(t, f.ethernet)
	f.mu.Unlock()
	dummy := fastPathIfindex(t, fastPathTestDummy)
	if got := fastPathIngress(t, dummy); !slices.Equal(got, []ebpf.ProgramID{ethernetID}) {
		t.Errorf("%s ingress runs %v, want exactly the fast path program %d", fastPathTestDummy, got, ethernetID)
	}
	if got := fastPathIngress(t, veth); !slices.Equal(got, []ebpf.ProgramID{ethernetID, standInID}) {
		t.Errorf("%s ingress runs %v, want the fast path program %d first, then the stand-in %d", fastPathTestVeth, got, ethernetID, standInID)
	}

	if err := netlink.LinkAdd(&netlink.Dummy{Name: fastPathTestLate}); err != nil {
		t.Fatalf("add %s: %v", fastPathTestLate, err)
	}
	waitForFastPath(t, f, "a link created after start attached", func(st ESPFormFastPathState) bool {
		return slices.Contains(st.Attached, fastPathTestLate)
	})
	late, err := netlink.LinkByName(fastPathTestLate)
	if err != nil {
		t.Fatalf("find %s: %v", fastPathTestLate, err)
	}
	if err := netlink.LinkDel(late); err != nil {
		t.Fatalf("delete %s: %v", fastPathTestLate, err)
	}
	waitForFastPath(t, f, "a deleted link forgotten", func(st ESPFormFastPathState) bool {
		return !slices.Contains(st.Attached, fastPathTestLate)
	})

	fastPathTestMaps(t, f)

	f.stop()
	if got := fastPathIngress(t, dummy); len(got) != 0 {
		t.Errorf("after stop %s ingress still runs %v, want nothing", fastPathTestDummy, got)
	}
	if got := fastPathIngress(t, veth); !slices.Equal(got, []ebpf.ProgramID{standInID}) {
		t.Errorf("after stop %s ingress runs %v, want only the stand-in %d", fastPathTestVeth, got, standInID)
	}
	st, err = f.state()
	if err != nil {
		t.Fatalf("read the state after stop: %v", err)
	}
	if st.Status != ESPFormFastPathIdle {
		t.Errorf("status after stop %s, want %s", st.Status, ESPFormFastPathIdle)
	}
	if len(st.Attached) != 0 {
		t.Errorf("after stop the state lists attached links %v, want none", st.Attached)
	}
	f.stop() // a second stop does nothing
}

// TestESPFormFastPathSecondSATakesOver proves that when the SA the fast path serves for
// an inbound SPI is forgotten, the next SA watched on that SPI takes the fast path's map
// over, and that the key goes once no SA carries the SPI.
//
// METHOD: in a fresh network namespace, a receiver is driven through Watch and Forget,
// the calls the XFRM backend makes, so the takeover runs where it runs in production
// (espFormReceiver.Forget, syncFastPath). A third SA on another SPI keeps the receiver,
// its fast path and the maps running after both SAs on the shared SPI are gone. The
// IPv4 map is read back by key.
// VALIDATES: with A and B watched on one SPI on two local addresses, the key holds A's
// peer and local address; after A is forgotten it holds B's; after B is forgotten the
// key is gone while the other SPI's key stays.
// PREVENTS: the SA left on a shared SPI rate-bounded on the complete path for the rest
// of its life because the fast path dropped the SPI with the first SA.
// MUTATION: in espFormReceiver.Forget, call r.fast.remove(spi) instead of
// r.syncFastPath(spi): after A is forgotten the key is gone, so the read of B's target
// fails.
func TestESPFormFastPathSecondSATakesOver(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: loading a SchedCLS program needs CAP_BPF and CAP_NET_ADMIN in the initial user namespace, which userns.Enter cannot grant; run `./le test qemu all`")
	}
	enterFastPathNetns(t)

	r := newESPFormReceiver(slogutil.DiscardLogger())
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("close the receiver: %v", err)
		}
	})
	const spi, other = 0x2001, 0x2002
	peerA, localA := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("192.0.2.1")
	peerB, localB := netip.MustParseAddr("198.51.100.8"), netip.MustParseAddr("192.0.2.2")
	peerO, localO := netip.MustParseAddr("198.51.100.9"), netip.MustParseAddr("192.0.2.3")

	if err := r.Watch(other, natTarget(peerO, localO)); err != nil {
		t.Fatalf("watch the SA on the other SPI: %v", err)
	}
	if status := r.fast.currentStatus(); status != ESPFormFastPathAttached {
		t.Fatalf("fast path status %s after the first Watch, want %s", status, ESPFormFastPathAttached)
	}
	if err := r.Watch(spi, natTarget(peerA, localA)); err != nil {
		t.Fatalf("watch A: %v", err)
	}
	if err := r.Watch(spi, natTarget(peerB, localB)); err != nil {
		t.Fatalf("watch B on the same SPI: %v", err)
	}
	r.fast.mu.Lock()
	watched4 := r.fast.watched4
	r.fast.mu.Unlock()
	holds := func(when string, key uint32, peer, local netip.Addr) {
		t.Helper()
		want, err := newESPFormFastPathValue4(peer, local, espFormUDPPort, espFormUDPPort)
		if err != nil {
			t.Fatalf("lay out the target of %v: %v", local, err)
		}
		var got espFormFastPathValue4
		if err := watched4.Lookup(newESPFormFastPathKey(key), &got); err != nil {
			t.Fatalf("%s: read SPI %#x back: %v", when, key, err)
		}
		if got != want {
			t.Errorf("%s: SPI %#x reads % x, want % x (peer %v, local %v)", when, key, got, want, peer, local)
		}
	}

	holds("with A and B watched", spi, peerA, localA)
	r.Forget(spi, localA)
	holds("after A was forgotten", spi, peerB, localB)
	r.Forget(spi, localB)
	fastPathAbsent(t, watched4, spi, "the IPv4 map after A and B were forgotten")
	holds("after A and B were forgotten", other, peerO, localO)
}

// TestESPFormFastPathWarnsOfASharedSPIWhileAttached proves Watch warns of two SAs on one
// inbound SPI while the fast path is attached, once, naming both local addresses, the
// SA listing as its fix and the fast path's doctor code. It is the positive control of
// TestESPFormReceiverWarnsOfNoSharedSPIWhileUnavailable.
//
// METHOD: in a fresh network namespace, where the fast path loads, a receiver watches
// two SAs on one SPI on two local addresses, and its warnings are read.
// MUTATION: in espFormReceiver.Watch, test that the fast path is not attached rather
// than attached: no warning is logged and this test turns red.
// MUTATION: in espFormReceiver.Watch, drop the "fix" attribute: the warning carries no
// fix and this test turns red.
func TestESPFormFastPathWarnsOfASharedSPIWhileAttached(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: loading a SchedCLS program needs CAP_BPF and CAP_NET_ADMIN in the initial user namespace, which userns.Enter cannot grant; run `./le test qemu all`")
	}
	enterFastPathNetns(t)

	warnings := &espFormWarnings{}
	r := newESPFormReceiver(slog.New(warnings))
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("close the receiver: %v", err)
		}
	})
	const spi = 0x2003
	peer := netip.MustParseAddr("198.51.100.7")
	localA, localB := netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")
	if err := r.Watch(spi, natTarget(peer, localA)); err != nil {
		t.Fatalf("watch the first SA: %v", err)
	}
	if status := r.fast.currentStatus(); status != ESPFormFastPathAttached {
		t.Fatalf("fast path status %s after the first Watch, want %s", status, ESPFormFastPathAttached)
	}
	if got := warnings.attrs(espFormSharedSPIWarning); len(got) != 0 {
		t.Fatalf("the first Watch warned of a shared SPI: %v", got)
	}
	if err := r.Watch(spi, natTarget(peer, localB)); err != nil {
		t.Fatalf("watch the second SA on the SPI: %v", err)
	}
	got := warnings.attrs(espFormSharedSPIWarning)
	if len(got) != 1 {
		t.Fatalf("the second Watch warned of the shared SPI %d times, want once: %v", len(got), got)
	}
	for key, want := range map[string]string{
		"local": localB.String(), "fast-path-local": localA.String(),
		"fix": ESPFormFixListSAs, "code": diagnostic.CodeDoctorIPsecESPFormFastPath,
	} {
		if got[0][key] != want {
			t.Errorf("the warning's %s is %q, want %q", key, got[0][key], want)
		}
	}
}

// fastPathTestMaps writes a v4 and a v6 SPI, moves the v4 one to IPv6, and removes
// both, reading each map back by key.
func fastPathTestMaps(t *testing.T, f *espFormFastPath) {
	t.Helper()
	const spi4, spi6 = 0x1001, 0x1002
	peer4, local4 := netip.MustParseAddr("198.51.100.7"), netip.MustParseAddr("192.0.2.1")
	peer6, local6 := netip.MustParseAddr("2001:db8::7"), netip.MustParseAddr("2001:db8::1")

	if err := f.set(spi4, peer4, local4, 4500, 4501); err != nil {
		t.Fatalf("set an IPv4 SPI: %v", err)
	}
	if err := f.set(spi6, peer6, local6, 4502, 4503); err != nil {
		t.Fatalf("set an IPv6 SPI: %v", err)
	}
	want4, err := newESPFormFastPathValue4(peer4, local4, 4500, 4501)
	if err != nil {
		t.Fatalf("lay out the IPv4 target: %v", err)
	}
	want6, err := newESPFormFastPathValue6(peer6, local6, 4502, 4503)
	if err != nil {
		t.Fatalf("lay out the IPv6 target: %v", err)
	}

	f.mu.Lock()
	watched4, watched6 := f.watched4, f.watched6
	f.mu.Unlock()

	var got4 espFormFastPathValue4
	if err := watched4.Lookup(newESPFormFastPathKey(spi4), &got4); err != nil {
		t.Fatalf("read the IPv4 SPI back: %v", err)
	}
	if got4 != want4 {
		t.Errorf("IPv4 SPI reads % x, want % x", got4, want4)
	}
	var got6 espFormFastPathValue6
	if err := watched6.Lookup(newESPFormFastPathKey(spi6), &got6); err != nil {
		t.Fatalf("read the IPv6 SPI back: %v", err)
	}
	if got6 != want6 {
		t.Errorf("IPv6 SPI reads % x, want % x", got6, want6)
	}
	fastPathAbsent(t, watched6, spi4, "the IPv6 map before the move")

	// A move to the other family leaves the SPI in one map only.
	if err := f.set(spi4, peer6, local6, 4500, 4501); err != nil {
		t.Fatalf("move the IPv4 SPI to IPv6: %v", err)
	}
	fastPathAbsent(t, watched4, spi4, "the IPv4 map after the move")
	if err := watched6.Lookup(newESPFormFastPathKey(spi4), &got6); err != nil {
		t.Errorf("the moved SPI is not in the IPv6 map: %v", err)
	}

	f.remove(spi4)
	f.remove(spi6)
	f.remove(0x1003) // never watched: nothing to delete, no failure
	fastPathAbsent(t, watched4, spi4, "the IPv4 map after remove")
	fastPathAbsent(t, watched6, spi4, "the IPv6 map after remove")
	fastPathAbsent(t, watched6, spi6, "the IPv6 map after remove")
}

// fastPathAbsent fails unless spi has no entry in watched.
func fastPathAbsent(t *testing.T, watched *ebpf.Map, spi uint32, where string) {
	t.Helper()
	value := make([]byte, watched.ValueSize())
	err := watched.Lookup(newESPFormFastPathKey(spi), value)
	if !errors.Is(err, ebpf.ErrKeyNotExist) {
		t.Errorf("SPI %#08x in %s: lookup answered %v, want no entry", spi, where, err)
	}
}

// enterFastPathNetns moves the test's goroutine, locked to its thread, into a fresh
// network namespace with lo up, and returns it to its own at cleanup, the way
// enterGuardNetns does.
func enterFastPathNetns(t *testing.T) {
	t.Helper()
	runtime.LockOSThread()
	origin, err := netns.Get()
	if err != nil {
		runtime.UnlockOSThread()
		t.Fatalf("read the thread's network namespace: %v", err)
	}
	ns, err := netns.New()
	if err != nil {
		origin.Close() //nolint:errcheck // the unshare already failed; that error is the one reported
		runtime.UnlockOSThread()
		t.Fatalf("make a network namespace: %v", err)
	}
	t.Cleanup(func() {
		ns.Close() //nolint:errcheck // the namespace goes with its last reference
		if err := netns.Set(origin); err != nil {
			// The thread stays locked, so the runtime retires it with the namespace.
			t.Errorf("return the thread to its network namespace: %v", err)
			return
		}
		origin.Close() //nolint:errcheck // restored above; the handle is spent
		runtime.UnlockOSThread()
	})

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("find lo: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("bring lo up: %v", err)
	}
}

// fastPathTestLinks creates the veth pair and the dummy (Ethernet), the TUN (none) and
// the IPIP tunnel (no variant).
func fastPathTestLinks(t *testing.T) {
	t.Helper()
	if err := netlink.LinkAdd(&netlink.Veth{Name: fastPathTestVeth, PeerName: fastPathTestVethPeer}); err != nil {
		t.Fatalf("add the veth pair %s/%s: %v", fastPathTestVeth, fastPathTestVethPeer, err)
	}
	if err := netlink.LinkAdd(&netlink.Dummy{Name: fastPathTestDummy}); err != nil {
		t.Fatalf("add %s: %v", fastPathTestDummy, err)
	}
	tun := &netlink.Tuntap{
		Name:  fastPathTestTUN,
		Mode:  netlink.TUNTAP_MODE_TUN,
		Flags: netlink.TUNTAP_DEFAULTS | netlink.TUNTAP_NO_PI,
	}
	if err := netlink.LinkAdd(tun); err != nil {
		t.Fatalf("add the TUN %s: %v", fastPathTestTUN, err)
	}
	for _, fd := range tun.Fds {
		fd.Close() //nolint:errcheck // the device is persistent; the queue descriptor is not needed
	}
	tunnel := &netlink.Iptun{
		Name:   fastPathTestTunnel,
		Local:  net.ParseIP("192.0.2.1"),
		Remote: net.ParseIP("192.0.2.2"),
	}
	if err := netlink.LinkAdd(tunnel); err != nil {
		t.Fatalf("add the IPIP tunnel %s: %v", fastPathTestTunnel, err)
	}
	for _, name := range []string{fastPathTestVeth, fastPathTestVethPeer, fastPathTestDummy, fastPathTestTUN, fastPathTestTunnel} {
		l, err := netlink.LinkByName(name)
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if err := netlink.LinkSetUp(l); err != nil {
			t.Fatalf("bring %s up: %v", name, err)
		}
	}
}

// fastPathStandIn loads a SchedCLS program that answers TCX_PASS.
func fastPathStandIn(t *testing.T) (*ebpf.Program, ebpf.ProgramID) {
	t.Helper()
	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Name:    "ze_fp_standin",
		Type:    ebpf.SchedCLS,
		License: "GPL",
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, 0), // TCX_PASS
			asm.Return(),
		},
	})
	if err != nil {
		t.Fatalf("load the stand-in program: %v", err)
	}
	t.Cleanup(func() { prog.Close() }) //nolint:errcheck // the test is done with it
	return prog, fastPathProgramID(t, prog)
}

// fastPathProgramID answers the kernel's id of prog.
func fastPathProgramID(t *testing.T, prog *ebpf.Program) ebpf.ProgramID {
	t.Helper()
	info, err := prog.Info()
	if err != nil {
		t.Fatalf("read the program info: %v", err)
	}
	id, ok := info.ID()
	if !ok {
		t.Fatal("the kernel reports no program id")
	}
	return id
}

// fastPathIngress answers the programs at ifindex's TCX ingress, in the order they run.
func fastPathIngress(t *testing.T, ifindex int) []ebpf.ProgramID {
	t.Helper()
	result, err := link.QueryPrograms(link.QueryOptions{Target: ifindex, Attach: ebpf.AttachTCXIngress})
	if err != nil {
		t.Fatalf("query the TCX ingress of interface %d: %v", ifindex, err)
	}
	ids := make([]ebpf.ProgramID, 0, len(result.Programs))
	for _, p := range result.Programs {
		ids = append(ids, p.ID)
	}
	return ids
}

// fastPathIfindex answers the index of the named link.
func fastPathIfindex(t *testing.T, name string) int {
	t.Helper()
	l, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("find %s: %v", name, err)
	}
	return l.Attrs().Index
}

// waitForFastPath polls the state until done holds, and fails after fastPathTestWait.
// The worker applies link updates on its own goroutine, so the test waits on the
// state it produces.
func waitForFastPath(t *testing.T, f *espFormFastPath, what string, done func(ESPFormFastPathState) bool) ESPFormFastPathState {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(fastPathTestWait)
	for {
		st, err := f.state()
		if err != nil {
			t.Fatalf("read the fast path state: %v", err)
		}
		if done(st) {
			return st
		}
		if st.Status != ESPFormFastPathAttached {
			t.Fatalf("waiting for %s: status %s (cause %q), want %s", what, st.Status, st.Cause, ESPFormFastPathAttached)
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("waiting for %s: after %s the state is %+v", what, fastPathTestWait, st)
		}
	}
}

// fastPathHas reports whether every name is in attached.
func fastPathHas(attached []string, names ...string) bool {
	for _, name := range names {
		if !slices.Contains(attached, name) {
			return false
		}
	}
	return true
}

// fastPathRefusal answers the refusal of the named link, or nil.
func fastPathRefusal(st ESPFormFastPathState, name string) *ESPFormLinkRefusal {
	for i := range st.Refused {
		if st.Refused[i].Link == name {
			return &st.Refused[i]
		}
	}
	return nil
}
