// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- the signer hook and the socket send.
// Related: transport.go -- SendPacket and SendPacketRouted run the signer, then hand the bytes to the socket.

package transport

import (
	"bytes"
	"crypto/md5" //nolint:gosec // G501: RFC 2328 Appendix D.4.3 keyed MD5 is the algorithm under test.
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
)

// VALIDATES: RFC 2328 Appendix D.3, the appended digest leaves the OSPF Packet Length alone and
// reaches the socket, so the IP datagram the kernel builds around it counts it.
// PREVENTS: a send path that trims the signed packet back to its OSPF Packet Length.

// digestTestHello is a 44-octet OSPFv2 Hello (24-octet header, 20-octet body) with AuType 2
// and a zero checksum, as the engine's signer prepares it before packet.Sign.
func digestTestHello() []byte {
	pkt := make([]byte, 44)
	pkt[0], pkt[1] = 2, 1 // version 2, Hello
	binary.BigEndian.PutUint16(pkt[2:4], 44)
	copy(pkt[4:8], []byte{1, 1, 1, 1})         // Router ID
	pkt[15] = 2                                // AuType 2, cryptographic
	copy(pkt[24:28], []byte{255, 255, 255, 0}) // network mask
	binary.BigEndian.PutUint16(pkt[28:30], 10) // HelloInterval
	pkt[31] = 1                                // Rtr Pri
	binary.BigEndian.PutUint32(pkt[32:36], 40) // RouterDeadInterval
	return pkt
}

// RFC requirement: RFC2328-D.3-1 positive -- the message digest appended to the OSPF packet is not included in the OSPF header's packet length, but is included in what the transport hands to the socket, from which the kernel builds the IP header length: through SendPacket and SendPacketRouted with the cryptographic signer (packet.Sign, keyed MD5), the socket receives Packet Length + 16 octets, the Packet Length field still reads 44, and the 16 trailing octets equal an independent RFC 2328 D.4.3 MD5 over the 44 octets and the 16-octet padded key.
func TestRFC2328DigestCountedInIPLengthNotPacketLength(t *testing.T) {
	key := packet.AuthKey{KeyID: 7, Algorithm: packet.AuthMD5, Secret: []byte("ze-md5-key")}
	dst := netip.MustParseAddr("224.0.0.5")
	for _, routed := range []bool{false, true} {
		name := "link-local"
		if routed {
			name = "routed"
		}
		t.Run(name, func(t *testing.T) {
			fb := newFakeBackend()
			tr := New(fb)
			tr.EnableInterface("eth0")
			if err := tr.HandleLinkUp("eth0"); err != nil {
				t.Fatalf("HandleLinkUp: %v", err)
			}
			tr.SetSigner(func(_ string, wire []byte) []byte {
				signed, err := packet.Sign(wire, packet.AuTypeCryptographic, key, 0x01020304, [4]byte{})
				if err != nil {
					t.Errorf("Sign: %v", err)
					return nil
				}
				return signed
			})
			var err error
			if routed {
				err = tr.SendPacketRouted("eth0", netip.MustParseAddr("192.0.2.9"), netip.Addr{}, digestTestHello())
			} else {
				err = tr.SendPacket("eth0", dst, digestTestHello())
			}
			if err != nil {
				t.Fatalf("send: %v", err)
			}
			h := fb.handles["eth0"]
			sends := h.sends
			if routed {
				sends = h.routed
			}
			if len(sends) != 1 {
				t.Fatalf("socket sends = %d, want 1", len(sends))
			}
			wire := sends[0].payload
			if len(wire) != 44+md5.Size {
				t.Fatalf("socket payload is %d octets, want 44 + %d (the digest is part of the IP datagram)", len(wire), md5.Size)
			}
			if got := binary.BigEndian.Uint16(wire[2:4]); got != 44 {
				t.Fatalf("OSPF Packet Length = %d, want 44 (the digest is not counted)", got)
			}
			var padded [16]byte
			copy(padded[:], key.Secret)
			want := md5.Sum(append(append([]byte(nil), wire[:44]...), padded[:]...)) //nolint:gosec // G401: the RFC 2328 algorithm under test.
			if !bytes.Equal(wire[44:], want[:]) {
				t.Fatalf("trailing octets = %x, want the RFC 2328 D.4.3 digest %x", wire[44:], want)
			}
		})
	}
}
