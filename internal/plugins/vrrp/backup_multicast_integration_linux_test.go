//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- Backup discards unicast VMAC traffic, not peer multicast.

package vrrp

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestVRRPBackupReceivesMulticastPeerAdvertisements installs the production
// Backup filter, then sends a VRRP multicast frame from the peer using the
// shared virtual MAC. The raw socket on the PRIVATE macvlan must receive it.
// TestVRRPBackupDoesNotForwardVirtualMACFrames covers the complementary drop.
// It runs unprivileged in a user namespace of its own (userns.Enter), where it
// holds CAP_NET_ADMIN and CAP_NET_RAW.
func TestVRRPBackupReceivesMulticastPeerAdvertisements(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	w := newGatewayWire(t)
	vmac := nonOwnerMacvlan(t, w, "zbmc4", 30)
	// These owned namespace settings MUST be restored before wire teardown.
	if err := applyDataplaneSysctls(w.parent.Attrs().Name, "zbmc4", familyIPv4); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// MUST restore the sysctls before the owned interfaces disappear.
		revertDataplaneSysctls(w.parent.Attrs().Name, "zbmc4", familyIPv4)
	})
	// This test's Backup-filter registration MUST be withdrawn during cleanup.
	if err := setBackupFilter("vrrp:zbmc4", "zbmc4"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// MUST withdraw only this test's registration before wire teardown.
		if err := clearBackupFilter("vrrp:zbmc4"); err != nil {
			t.Error(err)
		}
	})
	// The owned raw socket MUST be closed before its namespace is removed.
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, int(packet.ProtoNumber))
	if err != nil {
		t.Fatal(err)
	}
	// MUST release the descriptor on every subsequent test exit.
	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // best-effort socket cleanup
	if err := unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, "zbmc4"); err != nil {
		t.Fatal(err)
	}
	mreq := &unix.IPMreqn{Multiaddr: packet.MulticastV4.As4(), Ifindex: int32(gatewayLink(t, "zbmc4").Attrs().Index)}
	if err := unix.SetsockoptIPMreqn(fd, unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP, mreq); err != nil {
		t.Fatal(err)
	}
	if err := unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Sec: 3}); err != nil {
		t.Fatal(err)
	}

	var frame [128]byte
	copy(frame[:6], []byte{1, 0, 0x5e, 0, 0, 0x12})
	copy(frame[6:12], vmac)
	binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_IP)
	src := netip.MustParseAddr("192.0.2.20")
	adv := packet.Advertisement{Version: packet.VersionV3, Family: packet.V4, VRID: 30, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{nonOwnerVIP}}
	// RFC 9568 Sections 5.1 and 5.2.8: send an actual checked advertisement.
	n := adv.WriteTo(frame[:], 34)
	packet.FillChecksum(frame[:], 34, n, src, packet.MulticastV4, packet.ChecksumRFC9568)
	frame[14], frame[22], frame[23] = 0x45, 255, packet.ProtoNumber
	binary.BigEndian.PutUint16(frame[16:18], uint16(20+n))
	copy(frame[26:30], src.AsSlice())
	copy(frame[30:34], packet.MulticastV4.AsSlice())
	binary.BigEndian.PutUint16(frame[24:26], gatewayChecksum(frame[14:34]))
	if err := unix.Sendto(w.fd, frame[:34+n], 0, &unix.SockaddrLinklayer{Ifindex: w.peer.Attrs().Index}); err != nil {
		t.Fatal(err)
	}
	var received [128]byte
	count, _, err := unix.Recvfrom(fd, received[:], 0)
	if err != nil {
		t.Fatalf("Backup filter blocked peer multicast advertisement: %v", err)
	}
	if !bytes.Equal(received[:count], frame[14:34+n]) {
		t.Fatalf("received datagram = %x, want %x", received[:count], frame[14:34+n])
	}
}
