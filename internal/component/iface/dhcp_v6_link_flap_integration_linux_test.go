// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: rfc9915_dhcp_v6_link_move_integration_linux_test.go -- the leased lab (v6lmLease) these tests reuse
// Related: dhcp_v6_selector_renew_integration_linux_test.go -- the veth lab and the address lookup

//go:build linux

package iface_test

import (
	"errors"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/test/userns"
)

// v6lfSlack is how far below the granted lifetime the kernel's lifetime left
// may be when read: the time from the Reply to the read, with room to spare.
const v6lfSlack = 60 * time.Second

// TestDHCPv6CarrierFlapKeepsTheLeasedAddress flaps the carrier of the
// client's veth by setting the peer end down and up, while the lease holds an
// address and a delegated prefix.
//
// VALIDATES: after the flap the leased address is on the device, out of DAD,
// with finite lifetimes no longer than the lease granted, and the client sends
// a Rebind and no Decline.
// PREVENTS: a carrier loss read as a duplicate address, or an address left
// with a lifetime the lease never granted.
func TestDHCPv6CarrierFlapKeepsTheLeasedAddress(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := v6lmLease(t)

	h, err := netlink.NewHandleAt(lab.peerNS)
	if err != nil {
		t.Fatalf("netlink handle in the peer namespace: %v", err)
	}
	defer h.Close()
	peer, err := h.LinkByName(lab.peer)
	if err != nil {
		t.Fatalf("LinkByName(%s) in the peer namespace: %v", lab.peer, err)
	}
	if err := h.LinkSetDown(peer); err != nil {
		t.Fatalf("take %s down: %v", lab.peer, err)
	}
	lab.awaitLink(t, iface.LinkDown)
	if err := h.LinkSetUp(peer); err != nil {
		t.Fatalf("bring %s back up: %v", lab.peer, err)
	}
	lab.awaitLink(t, iface.LinkUp)

	lab.server.awaitRebindNoDecline(t, "after the carrier flap")
	v6lfRequireLeasedAddress(t, lab.device, "after the carrier flap")
}

// TestDHCPv6AdminFlapReinstallsTheLeasedAddress sets the client's own device
// down and up while the lease holds an address and a delegated prefix. The
// kernel deletes the address with the down.
//
// VALIDATES: the client installs the address again after the up, with finite
// lifetimes no longer than the lease granted, the kernel's DAD clears it, and
// the client sends a Rebind and no Decline.
// PREVENTS: every leased address declined to the server after an operator
// bounces the interface, or left off the device until the next Reply.
func TestDHCPv6AdminFlapReinstallsTheLeasedAddress(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	lab := v6lmLease(t)

	link, err := netlink.LinkByName(lab.device)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", lab.device, err)
	}
	if err := netlink.LinkSetDown(link); err != nil {
		t.Fatalf("take %s down: %v", lab.device, err)
	}
	lab.awaitLink(t, iface.LinkDown)
	if a := v6sbFindAddress(t, lab.device); a != nil {
		t.Fatalf("%s still holds %s while down: the kernel kept the address, so this test cannot see a reinstall", lab.device, v6sbLeased)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("bring %s back up: %v", lab.device, err)
	}
	lab.awaitLink(t, iface.LinkUp)

	lab.server.awaitRebindNoDecline(t, "after the admin flap")
	v6lfRequireLeasedAddress(t, lab.device, "after the admin flap")
}

// TestDHCPv6AdminDownDuringDADBeforeItsEventNotDeclined sets the client's
// own device down while the kernel still holds the leased address
// tentative, with the netlink monitor not yet running, so no link event can
// reach the client: its DAD poll reads the device before it holds the down,
// which is the race a running monitor leaves open for a few milliseconds.
// The monitor then starts and the device comes back up.
//
// VALIDATES: the address the kernel deleted with the down is not declined,
// because the read that found it gone reports the device down; the up that
// follows completes a move although no down event was ever delivered, so
// the client installs the address again and sends a Rebind and no Decline.
// PREVENTS: a unique address declined to the server because the DAD poll
// outran the LinkDown event (journal row of 2026-10-19 in
// plan/journal/reconciler-repairs-against-a-stale-snapshot.md).
func TestDHCPv6AdminDownDuringDADBeforeItsEventNotDeclined(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	device, peer, peerNS, server := v6sbSetupLabNS(t)
	iface.SetResolverMappingForTest(map[string]string{v6sbLogical: device})
	t.Cleanup(func() { iface.SetResolverMappingForTest(nil) })
	// Ten probes hold the leased address tentative for about ten seconds, so
	// the down below lands inside DAD on any schedule.
	dadTransmits := "/proc/sys/net/ipv6/conf/" + device + "/dad_transmits"
	if err := os.WriteFile(dadTransmits, []byte("10"), 0o644); err != nil {
		t.Fatalf("lengthen DAD on %s: %v", device, err)
	}

	bus := newV6lmBus()
	t.Cleanup(iface.BindResolverEventsForTest(bus))
	links, cancel := iface.Subscribe(v6sbLogical)
	t.Cleanup(cancel)
	factory := iface.DHCPClientFactoryForTest()
	if factory == nil {
		t.Fatal("no DHCP client factory is installed: the ifacedhcp blank import did not run its init")
	}
	client, err := factory(v6sbLogical, "0", &v6sbBus{}, false, true, "", "", true, 56, "", 0)
	if err != nil {
		t.Fatalf("start the DHCPv6 client on %s: %v", v6sbLogical, err)
	}
	var stopOnce sync.Once
	t.Cleanup(func() { stopOnce.Do(client.Stop) })
	solicit := server.await(t, "the Solicit", dhcpv6.MessageTypeSolicit)
	server.replyLeasePD(t, solicit)

	deadline := time.Now().Add(v6sbKernelWait)
	var a *netlink.Addr
	for a == nil && time.Now().Before(deadline) {
		a = v6sbFindAddress(t, device)
		if a == nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if a == nil {
		t.Fatalf("%s never received %s after the Reply", device, v6sbLeased)
	}
	if a.Flags&unix.IFA_F_TENTATIVE == 0 {
		t.Fatalf("%s holds %s out of DAD already (flags %#x): the down cannot land inside DAD", device, v6sbLeased, a.Flags)
	}
	link, err := netlink.LinkByName(device)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", device, err)
	}
	if err := netlink.LinkSetDown(link); err != nil {
		t.Fatalf("take %s down: %v", device, err)
	}
	if a := v6sbFindAddress(t, device); a != nil {
		t.Fatalf("%s still holds %s while down: the kernel kept the address, so this test cannot see the race", device, v6sbLeased)
	}
	server.awaitNothing(t, "with the device down and no link event delivered", v6lmQuiet)

	if err := os.WriteFile(dadTransmits, []byte("1"), 0o644); err != nil {
		t.Fatalf("restore DAD on %s: %v", device, err)
	}
	backend := iface.GetBackend()
	if err := backend.StartMonitor(bus); err != nil {
		t.Fatalf("start the netlink monitor: %v", err)
	}
	t.Cleanup(backend.StopMonitor)
	if err := netlink.LinkSetUp(link); err != nil {
		t.Fatalf("bring %s back up: %v", device, err)
	}
	lab := &v6lmLab{device: device, peer: peer, peerNS: peerNS, server: server,
		clientID: solicit.msg.Options.ClientID(), links: links}
	lab.awaitLink(t, iface.LinkUp)

	server.awaitRebindNoDecline(t, "after the up")
	v6lfRequireLeasedAddress(t, device, "after the up")
}

// awaitRebindNoDecline reads every client message for v6lmQuiet past the
// first Rebind, failing on a Decline or on anything but a Rebind. No Reply is
// sent, so the Rebind retransmits.
func (s *v6sbServer) awaitRebindNoDecline(t *testing.T, label string) {
	t.Helper()
	s.await(t, label+": the link-change Rebind", dhcpv6.MessageTypeRebind)
	if err := s.conn.SetReadDeadline(time.Now().Add(v6lmQuiet)); err != nil {
		t.Fatalf("set the server read deadline: %v", err)
	}
	buf := make([]byte, 1500)
	// The bound: the read deadline.
	for {
		n, from, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return
			}
			t.Fatalf("%s: read: %v", label, err)
		}
		msg, err := dhcpv6.MessageFromBytes(buf[:n])
		if err != nil {
			t.Fatalf("%s: undecodable client message from %v: %v", label, from, err)
		}
		if msg.MessageType != dhcpv6.MessageTypeRebind {
			t.Fatalf("%s: the client sent %s, want only Rebind retransmissions", label, msg.MessageType)
		}
	}
}

// v6lfRequireLeasedAddress waits for the leased address to be on device and
// out of DAD, then checks its lifetimes against the lease of v6lmLease:
// preferred 3*v6lmLong, valid 4*v6lmLong.
func v6lfRequireLeasedAddress(t *testing.T, device, label string) {
	t.Helper()
	deadline := time.Now().Add(v6sbKernelWait)
	var a *netlink.Addr
	for time.Now().Before(deadline) {
		a = v6sbFindAddress(t, device)
		if a != nil && a.Flags&unix.IFA_F_DADFAILED != 0 {
			t.Fatalf("%s: duplicate address detection failed for %s on %s", label, v6sbLeased, device)
		}
		if a != nil && a.Flags&unix.IFA_F_TENTATIVE == 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if a == nil {
		t.Fatalf("%s: %s does not hold %s within %v", label, device, v6sbLeased, v6sbKernelWait)
	}
	if a.Flags&unix.IFA_F_TENTATIVE != 0 {
		t.Fatalf("%s: %s is still tentative on %s after %v", label, v6sbLeased, device, v6sbKernelWait)
	}
	if a.Flags&unix.IFA_F_PERMANENT != 0 {
		t.Errorf("%s: %s is permanent (flags %#x), want the lease's finite lifetimes", label, v6sbLeased, a.Flags)
	}
	validMax := int(4 * v6lmLong / time.Second)
	validMin := int((4*v6lmLong - v6lfSlack) / time.Second)
	if a.ValidLft < validMin || a.ValidLft > validMax {
		t.Errorf("%s: %s valid lifetime left = %ds, want %d..%ds", label, v6sbLeased, a.ValidLft, validMin, validMax)
	}
	preferredMax := int(3 * v6lmLong / time.Second)
	preferredMin := int((3*v6lmLong - v6lfSlack) / time.Second)
	if a.PreferedLft < preferredMin || a.PreferedLft > preferredMax {
		t.Errorf("%s: %s preferred lifetime left = %ds, want %d..%ds", label, v6sbLeased, a.PreferedLft, preferredMin, preferredMax)
	}
}
