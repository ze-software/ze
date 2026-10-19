// Design: docs/features/interfaces.md -- DHCPv6 client lifecycle
// Related: rfc9915_dhcp_v6_link_move_integration_linux_test.go -- the leased lab (v6lmLease) these tests reuse
// Related: dhcp_v6_selector_renew_integration_linux_test.go -- the veth lab and the address lookup

//go:build integration && linux

package iface_test

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv6"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
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
