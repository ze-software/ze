// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- golden-byte GARP frame tests (darwin-safe)

package transport

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

func TestBuildGARPFrameGolden(t *testing.T) {
	// VALIDATES: AC-8 / R-1 -- exact 42-byte gratuitous ARP for vrid 10, VIP
	// 192.0.2.1, virtual MAC 00:00:5e:00:01:0a; tha == virtual MAC (errata
	// 7947/7949, orchestrator D-E). Byte layout from the spec.
	want := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // eth dst = broadcast
		0x00, 0x00, 0x5e, 0x00, 0x01, 0x0a, // eth src = virtual MAC
		0x08, 0x06, // ethertype ARP
		0x00, 0x01, 0x08, 0x00, 0x06, 0x04, // htype 1, ptype 0x0800, hlen 6, plen 4
		0x00, 0x01, // oper 1 (request)
		0x00, 0x00, 0x5e, 0x00, 0x01, 0x0a, // sha = virtual MAC
		0xc0, 0x00, 0x02, 0x01, // spa = 192.0.2.1
		0x00, 0x00, 0x5e, 0x00, 0x01, 0x0a, // tha = virtual MAC (D-E)
		0xc0, 0x00, 0x02, 0x01, // tpa = 192.0.2.1
	}

	buf := make([]byte, 64)
	vmac := packet.VirtualMAC(packet.V4, 10)
	n := buildGARP(buf, vmac, netip.MustParseAddr("192.0.2.1").As4())
	if n != GARPFrameLen {
		t.Fatalf("buildGARP returned %d, want %d", n, GARPFrameLen)
	}
	if !bytes.Equal(buf[:n], want) {
		t.Fatalf("GARP frame\n got % x\nwant % x", buf[:n], want)
	}
}

func TestBuildGARPFramePerVIP(t *testing.T) {
	// VALIDATES: AC-8 -- one frame per IPv4 VIP with the VIP in spa and tpa; an
	// empty VIP list produces zero frames.
	vmac := packet.VirtualMAC(packet.V4, 10)
	vips := []netip.Addr{
		netip.MustParseAddr("192.0.2.1"),
		netip.MustParseAddr("198.51.100.7"),
	}
	var frames [][]byte
	buf := make([]byte, 64)
	for _, vip := range vips {
		n := buildGARP(buf, vmac, vip.As4())
		frames = append(frames, append([]byte(nil), buf[:n]...))
	}
	if len(frames) != 2 {
		t.Fatalf("built %d frames, want 2", len(frames))
	}
	for i, vip := range vips {
		v4 := vip.As4()
		if !bytes.Equal(frames[i][28:32], v4[:]) || !bytes.Equal(frames[i][38:42], v4[:]) {
			t.Fatalf("frame %d spa/tpa != %v: % x", i, vip, frames[i])
		}
	}

	// Zero VIPs -> zero frames.
	var none [][]byte
	for _, vip := range []netip.Addr(nil) {
		n := buildGARP(buf, vmac, vip.As4())
		none = append(none, append([]byte(nil), buf[:n]...))
	}
	if len(none) != 0 {
		t.Fatalf("empty VIP list built %d frames, want 0", len(none))
	}
}
