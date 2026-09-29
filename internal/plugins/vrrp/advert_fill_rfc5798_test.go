// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- transmit path
// Related: instance.go -- doSendAdvert, which fills the advertisement from state
// Related: groups.go -- EffectivePriority, the priority the state carries
//
// VALIDATES: the advertisement handed to the transport is filled from the
// Virtual Router's current state, and the encoded packet carries a checksum a
// receiver verifies.
// PREVENTS: a transmit path that fills the configured priority where the
// effective one applies, or that encodes without computing the checksum.
package vrrp

import (
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestTransmittedAdvertFilledFromInstanceState proves the RFC 5798 Section
// 7.2 transmit steps: fill the packet fields from the virtual router state,
// then compute the checksum.
//
// Method: an Active instance with two virtual addresses and two tracked
// interfaces captures every AdvertParams it hands to the transport. With all
// tracked interfaces up the last one must carry version 3, priority 200, the
// 1000 ms interval and both addresses in order; after eth1 (decrement 50)
// goes down it must carry priority 150, the effective priority, not the
// configured 200. That last advertisement is then encoded as the transport
// encodes it (WriteTo, FillChecksum with the parent primary as source) and
// must carry those fields on the wire and decode with its checksum verified.
//
// RFC requirement: RFC5798-7.2-1 positive -- the advertisement handed to the transport carries version 3, the effective priority (200, then 150 after a tracked interface fails), the 1000 ms interval and both virtual addresses, and its encoding carries those octets and a checksum that verifies on decode (doSendAdvert instance.go, WriteTo packet/packet.go, FillChecksum packet/checksum.go).
func TestTransmittedAdvertFilledFromInstanceState(t *testing.T) {
	spec := trackingSpec()
	spec.VIPs = []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("192.0.2.2")}
	l := newLinks("eth1", "wan0")

	var sent []transport.AdvertParams
	f := &fakeDeps{}
	deps := f.deps()
	deps.linkUp = l.linkUp
	deps.parentReady = func(string, string) bool { return true }
	deps.updateAdvert = func(_ transport.InstanceKey, p transport.AdvertParams) error {
		p.VIPs = slices.Clone(p.VIPs)
		sent = append(sent, p)
		return nil
	}
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	in := newInstance(spec, transport.InstanceKey{Interface: spec.Interface, VRID: spec.VRID, Family: packet.V4}, "zv4-2-10", clk, deps)
	in.startup()
	promoteToActive(t, in, clk)

	check := func(priority uint8) transport.AdvertParams {
		t.Helper()
		if len(sent) == 0 {
			t.Fatal("no advertisement reached the transport")
		}
		p := sent[len(sent)-1]
		if p.Version != packet.VersionV3 || p.Priority != priority || p.AdverIntervalMS != 1000 || !slices.Equal(p.VIPs, spec.VIPs) {
			t.Fatalf("advert params = %+v, want version 3, priority %d, 1000 ms, VIPs %v", p, priority, spec.VIPs)
		}
		return p
	}
	check(200)
	l.set("eth1", false)
	in.evaluateTracking()
	p := check(150)

	adv := packet.Advertisement{Version: p.Version, Family: packet.V4, VRID: spec.VRID, Priority: p.Priority, AdverIntervalMS: p.AdverIntervalMS, VIPs: p.VIPs}
	buf := make([]byte, packet.MaxLenV3v4)
	n := adv.WriteTo(buf, 0)
	source := netip.MustParseAddr("192.0.2.251")
	packet.FillChecksum(buf, 0, n, source, packet.MulticastV4)
	want := []byte{0x31, 10, 150, 2, 0x00, 0x64}
	if !slices.Equal(buf[:6], want) || !slices.Equal(buf[8:n], []byte{192, 0, 2, 1, 192, 0, 2, 2}) {
		t.Fatalf("encoded advert % x, want header % x .. and addresses 192.0.2.1, 192.0.2.2", buf[:n], want)
	}
	lookup := func(uint8) (packet.Local, bool) {
		return packet.Local{Version: packet.VersionV3, AdverIntervalMS: 1000}, true
	}
	meta := packet.RxMeta{TTL: 255, Src: source, Dst: packet.MulticastV4, Family: packet.V4}
	if _, err := packet.Decode(buf[:n], meta, lookup); err != nil {
		t.Fatalf("the encoded advert does not verify: %v", err)
	}
}
