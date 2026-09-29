package transport

import (
	"bytes"
	"net/netip"
	"testing"
	"time"
)

// announcedFrames drives AnnounceMaster for vips on a fresh instance of spec
// and returns the frames the handle was asked to send once every VIP has had
// its burst.
func announcedFrames(t *testing.T, spec InstanceSpec, vips []netip.Addr) [][]byte {
	t.Helper()
	fb := &fakeBackend{}
	tr := New(fb)
	t.Cleanup(tr.Close)
	key, err := tr.OpenInstance(spec)
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	h := fb.last()
	tr.AnnounceMaster(key, vips)
	want := announceRepeatCount * len(vips)
	deadline := time.After(3 * time.Second)
	for h.announceCount() < want {
		select {
		case <-deadline:
			t.Fatalf("announced %d frames, want %d", h.announceCount(), want)
		default:
			time.Sleep(time.Millisecond)
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([][]byte(nil), h.announces...)
}

// TestAnnounceMasterFramesPerAddress proves the announcement a new Master
// sends: one gratuitous ARP per IPv4 address with the Virtual Router MAC as
// sender and target hardware address, and one unsolicited Neighbor
// Advertisement per IPv6 address with R set, S clear, O set and the Virtual
// Router MAC in the Target Link-Layer Address option.
//
// Method: AnnounceMaster runs over the fake backend for two IPv4 addresses
// and, separately, two IPv6 addresses. Every frame the handle is asked to send
// is checked field by field against literals (the MAC 00-00-5e-00-01/02-0a,
// the ARP and ICMPv6 constants), and the set of announced addresses must be
// exactly the Virtual Router's addresses.
//
// RFC requirement: RFC3768-6.4.2-5 positive -- the failover announcement is a broadcast gratuitous ARP request per virtual IPv4 address, sender and target hardware address the virtual router MAC 00-00-5e-00-01-{VRID}, sender and target protocol address the virtual address (buildGARP garp.go, AnnounceMaster transport.go).
// RFC requirement: RFC5798-6.4.2-7 positive -- the failover announcement is one gratuitous ARP per virtual IPv4 address carrying the virtual router MAC, and one unsolicited NA per virtual IPv6 address with R=1, S=0, O=1, Target the address and Target Link-Layer Address the virtual router MAC 00-00-5e-00-02-{VRID} (buildGARP garp.go, BuildNA na.go, AnnounceMaster transport.go).
// RFC requirement: RFC9568-6.4.2-7 positive -- the failover announcement is one gratuitous ARP per virtual IPv4 address with target link-layer address the Virtual Router MAC (erratum 7949), and one unsolicited NA per virtual IPv6 address with R=1, S=0, O=1, Target the address and Target Link-Layer Address the Virtual Router MAC (buildGARP garp.go, BuildNA na.go, AnnounceMaster transport.go).
func TestAnnounceMasterFramesPerAddress(t *testing.T) {
	vmac4 := []byte{0x00, 0x00, 0x5e, 0x00, 0x01, 10}
	vmac6 := []byte{0x00, 0x00, 0x5e, 0x00, 0x02, 10}

	vips4 := []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("198.51.100.7")}
	seen4 := map[netip.Addr]bool{}
	for _, f := range announcedFrames(t, v4Spec(), vips4) {
		if len(f) != 42 {
			t.Fatalf("GARP frame length %d, want 42", len(f))
		}
		wantHead := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		wantHead = append(wantHead, vmac4...)
		wantHead = append(wantHead, 0x08, 0x06, 0x00, 0x01, 0x08, 0x00, 0x06, 0x04, 0x00, 0x01)
		if !bytes.Equal(f[:22], wantHead) {
			t.Fatalf("GARP header\n got % x\nwant % x", f[:22], wantHead)
		}
		spa := netip.AddrFrom4([4]byte(f[28:32]))
		if !bytes.Equal(f[22:28], vmac4) || !bytes.Equal(f[32:38], vmac4) {
			t.Errorf("GARP for %v: sender/target hardware % x / % x, want % x", spa, f[22:28], f[32:38], vmac4)
		}
		if tpa := netip.AddrFrom4([4]byte(f[38:42])); tpa != spa {
			t.Errorf("GARP target protocol address %v, want the announced %v", tpa, spa)
		}
		seen4[spa] = true
	}
	if len(seen4) != len(vips4) || !seen4[vips4[0]] || !seen4[vips4[1]] {
		t.Errorf("GARP announced %v, want exactly %v", seen4, vips4)
	}

	vips6 := []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::7")}
	seen6 := map[netip.Addr]bool{}
	for _, f := range announcedFrames(t, v6Spec(), vips6) {
		if len(f) != 32 {
			t.Fatalf("NA length %d, want 32", len(f))
		}
		if f[0] != 136 || f[1] != 0 {
			t.Fatalf("NA type/code %d/%d, want 136/0", f[0], f[1])
		}
		if f[4] != 0xa0 || f[5] != 0 || f[6] != 0 || f[7] != 0 {
			t.Errorf("NA flags % x, want R=1 S=0 O=1 (a0 00 00 00)", f[4:8])
		}
		target := netip.AddrFrom16([16]byte(f[8:24]))
		if f[24] != 2 || f[25] != 1 || !bytes.Equal(f[26:32], vmac6) {
			t.Errorf("NA for %v: option % x, want type 2 len 1 MAC % x", target, f[24:32], vmac6)
		}
		seen6[target] = true
	}
	if len(seen6) != len(vips6) || !seen6[vips6[0]] || !seen6[vips6[1]] {
		t.Errorf("NA announced %v, want exactly %v", seen6, vips6)
	}
}
