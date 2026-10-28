//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- peer advertisements share the local virtual MAC.

package transport

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestIntegrationPrivateMacvlanReceivesPeerVMAC sends a valid peer advertisement
// with the same source MAC as the local PRIVATE macvlan, as a real VRRP peer
// does. Linux redirects this multicast to the macvlan, not the parent. Both
// address families must deliver the packet with the configured parent's identity.
//
// It runs unprivileged in a user namespace of its own (userns.Enter), where it
// holds CAP_NET_ADMIN and CAP_NET_RAW.
func TestIntegrationPrivateMacvlanReceivesPeerVMAC(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	for _, family := range []uint8{packet.V4, packet.V6} {
		t.Run(fmt.Sprintf("IPv%d", family), func(t *testing.T) {
			lab := setupLab(t, family)
			link, err := netlink.LinkByName(lab.macvlan)
			if err != nil {
				t.Fatal(err)
			}
			if err := netlink.LinkSetMacvlanMode(link, netlink.MACVLAN_MODE_PRIVATE); err != nil {
				t.Fatal(err)
			}
			tr, key := openTransportInstance(t, lab, family)
			parent, err := netlink.LinkByName(lab.parent)
			if err != nil {
				t.Fatal(err)
			}

			var frame [128]byte
			copy(frame[6:12], lab.vmac[:])
			src := netip.MustParseAddr("192.0.2.2")
			dst := packet.MulticastV4
			vip := netip.MustParseAddr("192.0.2.1")
			offset := 14 + ipv4HeaderLen
			copy(frame[:6], []byte{1, 0, 0x5e, 0, 0, 0x12})
			binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_IP)
			if family == packet.V6 {
				src = netip.MustParseAddr("fe80::2")
				dst = packet.MulticastV6
				vip = netip.MustParseAddr("fe80::1")
				offset = 14 + 40
				copy(frame[:6], []byte{0x33, 0x33, 0, 0, 0, 0x12})
				binary.BigEndian.PutUint16(frame[12:14], unix.ETH_P_IPV6)
			}
			adv := packet.Advertisement{Version: packet.VersionV3, Family: family, VRID: testVRID, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{vip}}
			// RFC 9568 Sections 5.1 and 5.2.8: real headers and checksum, not a decoder stub.
			n := adv.WriteTo(frame[:], offset)
			packet.FillChecksum(frame[:], offset, n, src, dst, packet.ChecksumRFC9568)
			if family == packet.V4 {
				buildIPv4Header(frame[14:], src.As4(), dst.As4())
				binary.BigEndian.PutUint16(frame[16:18], uint16(ipv4HeaderLen+n))
				var sum uint32
				for i := 14; i < offset; i += 2 {
					sum += uint32(binary.BigEndian.Uint16(frame[i : i+2]))
				}
				binary.BigEndian.PutUint16(frame[24:26], ^foldSum(sum))
			} else {
				frame[14] = 0x60
				binary.BigEndian.PutUint16(frame[18:20], uint16(n))
				frame[20], frame[21] = packet.ProtoNumber, 255
				copy(frame[22:38], src.AsSlice())
				copy(frame[38:54], dst.AsSlice())
			}
			if err := unix.Sendto(lab.captureFD, frame[:offset+n], 0, &unix.SockaddrLinklayer{Ifindex: lab.peerIf}); err != nil {
				t.Fatalf("send peer VMAC advertisement: %v", err)
			}
			select {
			case item := <-tr.Receive():
				if item.Key != key {
					t.Fatalf("instance = %+v, want %+v", item.Key, key)
				}
				if item.Meta.IfIndex != parent.Attrs().Index {
					t.Fatalf("receiving identity = %d, want parent %d", item.Meta.IfIndex, parent.Attrs().Index)
				}
				if item.Meta.Src != src {
					t.Fatalf("source = %s, want %s", item.Meta.Src, src)
				}
				if item.Meta.TTL != 255 {
					t.Fatalf("TTL = %d, want 255", item.Meta.TTL)
				}
				if !bytes.Equal(item.Payload, frame[offset:offset+n]) {
					t.Fatalf("payload = %x, want %x", item.Payload, frame[offset:offset+n])
				}
			case <-time.After(3 * time.Second):
				t.Fatal("peer VMAC advertisement never reached the PRIVATE macvlan transport")
			}
		})
	}
}
