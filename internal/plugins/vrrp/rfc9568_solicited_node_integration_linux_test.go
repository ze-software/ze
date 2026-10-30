//go:build linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the Active router's macvlan hears its addresses' solicitations before it holds them
// Related: solicitednode_linux.go -- joinSolicitedNode, leaveSolicitedNode, the memberships under test
// Related: instance.go -- doInstallVIPs and doRemoveVIPs, the promotion and demotion paths that take and drop them
// Related: owner_answer_integration_linux_test.go -- the ND resolve and capture helpers
package vrrp

import (
	"bufio"
	"encoding/hex"
	"io"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestVRRPActiveJoinsSolicitedNodeBeforeItsAddressIsInstalled checks that a
// router which has just become Active is a member of its IPv6 virtual
// address's Solicited-Node group before the address is installed, keeps the
// groups its address set shares across a reconfiguration, and answers the
// first solicitation once the address is usable.
//
// Method: a non-owner IPv6 instance runs the real FSM, the real promotion path
// (doInstallVIPs) and the real address-owner registry
// (iface.RegisterOwnedAddresses), with the product's Solicited-Node
// memberships, on a private macvlan carrying the product's IPv6 recipe. The
// registry installs nothing itself: in production the iface reconcile does,
// on its own worker, after the promotion has returned. So the moment the
// promotion returns the address is registered but absent, and the test asserts
// that the address's Solicited-Node group, 33:33:ff:00:00:60, is already in the
// macvlan's multicast list and has been synced onto the parent's, which the
// macvlan's filter update (macvlan_set_mac_lists) does when it runs. The
// commit path (reconfigure) then swaps 2001:db8::60 for 2001:db8::70 while the
// router is Active: the new group appears, the old one goes, and the group of
// fe80::1, which both sets hold, is sampled from a second goroutine throughout
// the swap and MUST never be missing; a swap back restores the first set. The
// test then demotes the router: the registration and the membership go with it, and
// since no reconcile ran, the group leaves the macvlan at once. A second
// promotion plays the reconcile: the test adds the addresses through the
// product's netlink backend, waits until the address is usable, and the host
// sends one multicast solicitation with no retry, whose answer MUST carry the
// Virtual Router MAC. The kernel's own membership for an address it removes
// is dropped later, on its own schedule, so the test never asserts on it.
//
// RFC requirement: RFC9568-6.4.3-2 positive -- when the promotion to Active returns, before the IPv6 virtual address is installed, the Virtual Router MAC macvlan holds that address's Solicited-Node group (33:33:ff:00:00:60) in its multicast list and the group is synced onto the parent; a reconfiguration while Active adds the new address's group, drops the removed one's and never drops the group of fe80::1, which both address sets hold (joinSolicitedNode solicitednode_linux.go, doInstallVIPs instance.go).
func TestVRRPActiveJoinsSolicitedNodeBeforeItsAddressIsInstalled(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	w := newGatewayWire(t)
	parent := w.parent.Attrs().Name
	physical := w.parent.Attrs().HardwareAddr
	vip := netip.MustParseAddr("2001:db8::60")
	group := [6]byte{0x33, 0x33, 0xff, 0x00, 0x00, 0x60}
	spec := testSpecV6()
	spec.Interface, spec.ParentDevice = parent, parent
	spec.VRID = 22
	spec.VIPs = []netip.Addr{netip.MustParseAddr("fe80::1"), vip}
	spec.realAddresses = []netip.Addr{netip.MustParseAddr("2001:db8::254")}
	spec.realPrefixes = []netip.Prefix{netip.MustParsePrefix("2001:db8::254/64")}

	const device = "zs6"
	vmac := packet.VirtualMAC(packet.V6, spec.VRID)
	if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
		Name: device, Parent: parent, MAC: macString(vmac),
		Mode: iface.MacvlanModePrivate, Alias: "ze:owned:solicited-node-proof",
	}); err != nil {
		t.Fatal(err)
	}
	if err := applyDataplaneSysctls(parent, device, familyIPv6); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { revertDataplaneSysctls(parent, device, familyIPv6) })
	ndFD := ownerSocket(t, w.peer, unix.ETH_P_IPV6)

	f := &fakeDeps{}
	deps := f.deps()
	deps.installVIPs = iface.RegisterOwnedAddresses
	deps.removeVIPs = iface.UnregisterOwnedAddresses
	deps.joinSolicitedNode = joinSolicitedNode
	deps.leaveSolicitedNode = leaveSolicitedNode
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	key := transport.InstanceKey{Interface: parent, VRID: spec.VRID, Family: packet.V6}
	in := newInstance(spec, key, device, clk, deps)
	t.Cleanup(func() {
		iface.UnregisterOwnedAddresses(in.own)
		if err := leaveSolicitedNode(in.own); err != nil {
			t.Errorf("leave at cleanup: %v", err)
		}
	})
	cidrs := spec.vipCIDRs(spec.VIPs)

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("state = %v after startup, want Backup", in.machine.State())
	}
	if devMcastHolds(t, device, group) {
		t.Fatalf("the Backup's %s is a member of %x, want no membership before promotion", device, group)
	}

	// Phase 1: the membership alone. No reconcile runs, so the kernel never
	// holds the address and every reference to the group is the instance's.
	promoteToActive(t, in, clk)
	// The promotion has returned. The address is registered, not installed.
	if !registryHolds(t, device, cidrs) {
		t.Fatalf("the promotion registered none of %v on %s with the address-owner registry", cidrs, device)
	}
	if deviceHoldsAddress(t, device, vip) {
		t.Fatalf("%s holds %s before the reconcile ran: the test no longer models the asynchronous install", device, vip)
	}
	if !devMcastHolds(t, device, group) {
		t.Fatalf("the Active router's %s is not a member of %x when the promotion returns, before its address %s is installed", device, group, vip)
	}
	if !devMcastHolds(t, parent, group) {
		t.Fatalf("%x is in %s's multicast list but not yet synced onto its parent %s: the macvlan's filter update has not run", group, device, parent)
	}

	// The reconfiguration: one address swapped while Active.
	shared := [6]byte{0x33, 0x33, 0xff, 0x00, 0x00, 0x01}
	added := [6]byte{0x33, 0x33, 0xff, 0x00, 0x00, 0x70}
	swapped := spec
	swapped.VIPs = []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::70")}
	if !devMcastHolds(t, device, shared) {
		t.Fatalf("the Active router's %s is not a member of %x, the group of fe80::1", device, shared)
	}
	samples, misses := sampleWhile(t, device, shared, func() { in.reconfigure(swapped) })
	if samples >= mcastSamplesMax {
		t.Fatalf("the sampler stopped at its cap of %d samples before the reconfiguration returned, so the samples do not cover the whole reconfiguration", mcastSamplesMax)
	}
	if misses != 0 {
		t.Fatalf("%x, held by both address sets, was missing from %s in %d of %d samples taken during the reconfiguration", shared, device, misses, samples)
	}
	if !devMcastHolds(t, device, added) {
		t.Fatalf("the reconfiguration to %v left %s without %x", swapped.VIPs, device, added)
	}
	if !devMcastHolds(t, parent, added) {
		t.Fatalf("the reconfiguration left %x off %s's parent %s", added, device, parent)
	}
	if devMcastHolds(t, device, group) {
		t.Fatalf("the reconfiguration removed %s, but %s is still a member of %x", vip, device, group)
	}
	in.reconfigure(spec)
	if !devMcastHolds(t, device, group) {
		t.Fatalf("the reconfiguration back to %v left %s without %x", spec.VIPs, device, group)
	}
	if devMcastHolds(t, device, added) {
		t.Fatalf("the reconfiguration back to %v left %s a member of %x", spec.VIPs, device, added)
	}

	in.dispatch(fsm.AdvertReceived{Priority: 254, SrcIP: netip.MustParseAddr("fe80::99"), IntervalMs: 1000, VIPCount: 2})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("state = %v after a higher-priority advertisement, want Backup", in.machine.State())
	}
	if registryHolds(t, device, cidrs) {
		t.Fatalf("the demotion left %v registered on %s", cidrs, device)
	}
	if devMcastHolds(t, device, group) {
		t.Fatalf("the demoted router's %s is still a member of %x", device, group)
	}

	// Phase 2: the first solicitation. The reconcile installs the addresses
	// after the promotion; the host solicits once, with no retry, as soon as
	// the address is usable.
	promoteToActive(t, in, clk)
	if !devMcastHolds(t, parent, group) {
		t.Fatalf("the second promotion left %x off %s's filter", group, parent)
	}
	for _, cidr := range cidrs {
		if err := w.backend.AddAddress(device, cidr); err != nil {
			t.Fatal(err)
		}
	}
	waitAddressUsable(t, device, vip)
	answers := ownerResolveND(t, w.peer, ndFD, vip)
	ownerExpect(t, "first solicitation after the address is installed", answers, physical, vmac[:], false)

	in.dispatch(fsm.Shutdown{})
	if registryHolds(t, device, cidrs) {
		t.Fatalf("the shutdown left %v registered on %s", cidrs, device)
	}
}

// waitAddressUsable waits, for at most two seconds, until the kernel holds addr
// on dev outside the tentative state: the kernel answers no solicitation for a
// tentative address, so a host's first solicitation can be answered only from
// then on. The bound is a deadline, not a retry of what the test asserts.
func waitAddressUsable(t *testing.T, dev string, addr netip.Addr) {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("look up %s: %v", dev, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err != nil {
			t.Fatalf("list the addresses of %s: %v", dev, err)
		}
		for _, a := range addrs {
			got, ok := netip.AddrFromSlice(a.IP)
			if !ok || got != addr {
				continue
			}
			if a.Flags&unix.IFA_F_TENTATIVE == 0 {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s did not hold %s outside the tentative state within two seconds", dev, addr)
}

// registryHolds reports whether the address-owner registry holds any of cidrs
// on dev for an owner, by asking it to register them for a probe owner: the
// registry refuses an address another owner holds on the same interface.
func registryHolds(t *testing.T, dev string, cidrs []string) bool {
	t.Helper()
	const probe = "solicited-node-probe"
	err := iface.RegisterOwnedAddresses(dev, probe, cidrs)
	if err == nil {
		iface.UnregisterOwnedAddresses(probe)
		return false
	}
	return true
}

// deviceHoldsAddress reports whether the kernel holds addr on dev.
func deviceHoldsAddress(t *testing.T, dev string, addr netip.Addr) bool {
	t.Helper()
	link, err := netlink.LinkByName(dev)
	if err != nil {
		t.Fatalf("look up %s: %v", dev, err)
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
	if err != nil {
		t.Fatalf("list the addresses of %s: %v", dev, err)
	}
	for _, a := range addrs {
		if got, ok := netip.AddrFromSlice(a.IP); ok && got == addr {
			return true
		}
	}
	return false
}

// devMcastHolds reports whether dev's link-layer multicast list holds group.
// It reads the calling thread's namespace (/proc/thread-self), because the
// test switched only its locked thread into the namespace under test.
func devMcastHolds(t *testing.T, dev string, group [6]byte) bool {
	t.Helper()
	file, err := os.Open("/proc/thread-self/net/dev_mcast")
	if err != nil {
		t.Fatalf("open the multicast lists: %v", err)
	}
	defer func() { _ = file.Close() }()
	held, err := mcastListHolds(file, dev, group)
	if err != nil {
		t.Fatalf("read the multicast lists: %v", err)
	}
	return held
}

// mcastListHolds reports whether the dev_mcast text in r lists group on dev.
func mcastListHolds(r io.Reader, dev string, group [6]byte) (bool, error) {
	want := hex.EncodeToString(group[:])
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// ifindex name users global-users address
		fields := strings.Fields(scanner.Text())
		if len(fields) != 5 {
			continue
		}
		if fields[1] == dev && fields[4] == want {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// mcastSamples is what sampleMcastList saw: how many reads of the multicast
// lists it made, in how many the group was missing, and the read error that
// stopped it, if one did.
type mcastSamples struct {
	samples, misses int
	err             error
}

// sampleWhile runs change while a second goroutine reads dev's multicast list
// over and over, and returns how many samples it took and in how many group
// was missing. The list is opened on the calling thread, which the test locked
// into the namespace under test: a proc/net file stays bound to the namespace
// it was opened in, so the sampler, on another thread, still reads this one.
// The first sample is taken before change runs; the sampler stops once change
// has returned, and at the latest after mcastSamplesMax samples.
func sampleWhile(t *testing.T, dev string, group [6]byte, change func()) (samples, misses int) {
	t.Helper()
	file, err := os.Open("/proc/thread-self/net/dev_mcast")
	if err != nil {
		t.Fatalf("open the multicast lists: %v", err)
	}
	defer func() { _ = file.Close() }()

	started, stop, done := make(chan struct{}), make(chan struct{}), make(chan mcastSamples, 1)
	go sampleMcastList(file, dev, group, started, stop, done)
	<-started
	change()
	close(stop)
	seen := <-done
	if seen.err != nil {
		t.Fatalf("sample the multicast lists: %v", seen.err)
	}
	return seen.samples, seen.misses
}

// mcastSamplesMax bounds sampleMcastList, so a sampler whose stop never comes
// still ends.
const mcastSamplesMax = 100000

// sampleMcastList is sampleWhile's sampler. It closes started once, after its
// first sample or its first failure, and sends one result on done when stop
// closes, when a read fails, or after mcastSamplesMax samples.
func sampleMcastList(file *os.File, dev string, group [6]byte, started chan<- struct{}, stop <-chan struct{}, done chan<- mcastSamples) {
	var seen mcastSamples
	defer func() { done <- seen }()
	defer func() {
		if seen.samples == 0 {
			close(started)
		}
	}()
	for range mcastSamplesMax {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			seen.err = err
			return
		}
		held, err := mcastListHolds(file, dev, group)
		if err != nil {
			seen.err = err
			return
		}
		seen.samples++
		if !held {
			seen.misses++
		}
		if seen.samples == 1 {
			close(started)
		}
		select {
		case <-stop:
			return
		default:
		}
	}
}

// TestSolicitedNodeJoinReachesTheBottomOfAVLANStack checks that a join on a
// macvlan stacked on two VLANs leaves the group in the multicast list of every
// device down to the bottom one when it returns.
//
// Method: in a private namespace, a VLAN on a veth, a second VLAN on the first,
// and the product's macvlan on the second. Each device's filter update syncs
// the groups onto the device below and queues that device's own update to the
// netdev work queue (__dev_set_rx_mode, net/core/dev_addr_lists.c), so a join
// that brings only the macvlan and its parent current leaves the veth's list
// without the group until the kernel's work runs. That work runs within the
// time this test takes to read the list (a walk cut to two devices passed 20 of
// 20 runs on Linux 7.2.9), so the read proves the result, not the walk's depth.
// What it does discriminate is the end of the walk at the veth, whose link is
// its peer in the same namespace: a walk that neither stops at a peer nor at a
// device it visited runs between the two ends past its depth bound and fails
// the join. The unit tests in solicitednode_walk_linux_test.go prove each stop
// and the depth on their own.
func TestSolicitedNodeJoinReachesTheBottomOfAVLANStack(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	w := newGatewayWire(t)
	bottom := w.parent.Attrs().Name
	outer := &netlink.Vlan{Name: "zq1", ParentIndex: w.parent.Attrs().Index, VlanId: 100}
	if err := netlink.LinkAdd(outer); err != nil {
		t.Fatalf("create VLAN zq1 on %s: %v", bottom, err)
	}
	if err := netlink.LinkSetUp(outer); err != nil {
		t.Fatalf("bring zq1 up: %v", err)
	}
	inner := &netlink.Vlan{Name: "zq2", ParentIndex: outer.Attrs().Index, VlanId: 200}
	if err := netlink.LinkAdd(inner); err != nil {
		t.Fatalf("create VLAN zq2 on zq1: %v", err)
	}
	if err := netlink.LinkSetUp(inner); err != nil {
		t.Fatalf("bring zq2 up: %v", err)
	}
	const device = "zs7"
	if err := w.backend.CreateMacvlanDevice(iface.MacvlanSpec{
		Name: device, Parent: "zq2", MAC: macString(packet.VirtualMAC(packet.V6, 23)),
		Mode: iface.MacvlanModePrivate, Alias: "ze:owned:solicited-node-stack-proof",
	}); err != nil {
		t.Fatal(err)
	}
	link, err := netlink.LinkByName(device)
	if err != nil {
		t.Fatalf("look up %s: %v", device, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("bring %s up: %v", device, err)
	}

	const owner = "solicited-node-stack"
	group := [6]byte{0x33, 0x33, 0xff, 0x00, 0x00, 0x61}
	t.Cleanup(func() {
		if err := leaveSolicitedNode(owner); err != nil {
			t.Errorf("leave at cleanup: %v", err)
		}
	})
	if err := joinSolicitedNode(owner, device, []netip.Addr{netip.MustParseAddr("2001:db8::61")}); err != nil {
		t.Fatalf("join on %s: %v", device, err)
	}
	// Bottom first: the kernel's queued work catches up on its own schedule, so
	// the device furthest from the join is the one read soonest after it.
	for _, dev := range []string{bottom, "zq1", "zq2", device} {
		if !devMcastHolds(t, dev, group) {
			t.Fatalf("the join on %s returned with %x not yet in %s's multicast list", device, group, dev)
		}
	}
}
