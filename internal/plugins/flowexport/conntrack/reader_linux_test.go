// VALIDATES: convertFlow translates a vishvananda ConntrackFlow into a FlowEntry
// (5-tuple copied, byte/packet counts summed across both directions, Mark and
// TCP state carried, CTA timestamps mapped with a now-fallback for LastSeen) and
// ipToAddr normalizes net.IP into an unmapped netip.Addr, rejecting nil.
// PREVENTS: a dump-path flow record silently dropping the reply-direction
// counters, losing the conntrack TCP state the DDoS characterizer reads, or
// leaking a v4-mapped-v6 address into the IPv4 enrichment trie.

//go:build linux

package conntrack

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

func TestConvertFlowIPv4(t *testing.T) {
	start := time.Unix(1_600_000_000, 0)
	stop := time.Unix(1_600_000_050, 0)

	f := &netlink.ConntrackFlow{}
	f.Forward.SrcIP = net.ParseIP("192.0.2.1")
	f.Forward.DstIP = net.ParseIP("198.51.100.2")
	f.Forward.SrcPort = 1234
	f.Forward.DstPort = 443
	f.Forward.Protocol = 6
	f.Forward.Bytes = 1000
	f.Forward.Packets = 10
	f.Reverse.Bytes = 500
	f.Reverse.Packets = 5
	f.Mark = 0x42
	f.ProtoInfo = &netlink.ProtoInfoTCP{State: 3} // ESTABLISHED
	f.TimeStart = uint64(start.UnixNano())
	f.TimeStop = uint64(stop.UnixNano())

	e, ok := convertFlow(f)
	if !ok {
		t.Fatal("convertFlow returned false for a valid v4 flow")
	}
	if e.SrcAddr != netip.MustParseAddr("192.0.2.1") || e.DstAddr != netip.MustParseAddr("198.51.100.2") {
		t.Errorf("addrs = %v/%v, want 192.0.2.1/198.51.100.2", e.SrcAddr, e.DstAddr)
	}
	if e.SrcPort != 1234 || e.DstPort != 443 || e.Protocol != 6 {
		t.Errorf("tuple = %d/%d proto %d, want 1234/443 proto 6", e.SrcPort, e.DstPort, e.Protocol)
	}
	if e.Bytes != 1500 {
		t.Errorf("bytes = %d, want 1500 (fwd 1000 + rev 500)", e.Bytes)
	}
	if e.Packets != 15 {
		t.Errorf("packets = %d, want 15 (fwd 10 + rev 5)", e.Packets)
	}
	if e.Mark != 0x42 {
		t.Errorf("mark = %#x, want 0x42", e.Mark)
	}
	if e.TCPState != 3 {
		t.Errorf("TCPState = %d, want 3 (ESTABLISHED)", e.TCPState)
	}
	if !e.StartTime.Equal(start) {
		t.Errorf("StartTime = %v, want %v", e.StartTime, start)
	}
	if !e.LastSeen.Equal(stop) {
		t.Errorf("LastSeen = %v, want %v (from TimeStop)", e.LastSeen, stop)
	}
}

func TestConvertFlowIPv6NowFallback(t *testing.T) {
	f := &netlink.ConntrackFlow{}
	f.Forward.SrcIP = net.ParseIP("2001:db8::1")
	f.Forward.DstIP = net.ParseIP("2001:db8::2")
	f.Forward.SrcPort = 53
	f.Forward.DstPort = 5353
	f.Forward.Protocol = 17
	f.Forward.Bytes = 200
	f.Forward.Packets = 2
	// No TimeStop: LastSeen must fall back to now, not stay zero.

	before := time.Now()
	e, ok := convertFlow(f)
	if !ok {
		t.Fatal("convertFlow returned false for a valid v6 flow")
	}
	if e.SrcAddr != netip.MustParseAddr("2001:db8::1") || e.DstAddr != netip.MustParseAddr("2001:db8::2") {
		t.Errorf("addrs = %v/%v", e.SrcAddr, e.DstAddr)
	}
	if e.Protocol != 17 || e.Bytes != 200 || e.Packets != 2 {
		t.Errorf("proto/counters = %d/%d/%d, want 17/200/2", e.Protocol, e.Bytes, e.Packets)
	}
	if e.TCPState != 0 {
		t.Errorf("TCPState = %d, want 0 for a UDP flow with no ProtoInfo", e.TCPState)
	}
	if e.LastSeen.Before(before) {
		t.Errorf("LastSeen = %v, want now-fallback (>= %v)", e.LastSeen, before)
	}
	if !e.StartTime.IsZero() {
		t.Errorf("StartTime = %v, want zero when TimeStart is 0", e.StartTime)
	}
}

func TestConvertFlowMissingAddr(t *testing.T) {
	// Missing source address: unconvertible.
	f := &netlink.ConntrackFlow{}
	f.Forward.DstIP = net.ParseIP("198.51.100.2")
	if _, ok := convertFlow(f); ok {
		t.Error("convertFlow should return false when SrcIP is nil")
	}

	// Missing destination address: unconvertible.
	f2 := &netlink.ConntrackFlow{}
	f2.Forward.SrcIP = net.ParseIP("192.0.2.1")
	if _, ok := convertFlow(f2); ok {
		t.Error("convertFlow should return false when DstIP is nil")
	}
}

func TestIPToAddr(t *testing.T) {
	if _, ok := ipToAddr(nil); ok {
		t.Error("ipToAddr(nil) should return false")
	}

	v4, ok := ipToAddr(net.ParseIP("192.0.2.7"))
	if !ok || v4 != netip.MustParseAddr("192.0.2.7") {
		t.Errorf("ipToAddr(v4) = %v, %v; want 192.0.2.7, true", v4, ok)
	}
	if !v4.Is4() {
		t.Errorf("ipToAddr(v4) = %v, want an Is4 (unmapped) address", v4)
	}

	v6, ok := ipToAddr(net.ParseIP("2001:db8::9"))
	if !ok || v6 != netip.MustParseAddr("2001:db8::9") {
		t.Errorf("ipToAddr(v6) = %v, %v; want 2001:db8::9, true", v6, ok)
	}

	// A v4-mapped-v6 input must come back as a pure IPv4 address (Unmap).
	mapped := net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 0, 2, 55}
	got, ok := ipToAddr(mapped)
	if !ok {
		t.Fatal("ipToAddr(v4-mapped) returned false")
	}
	if !got.Is4() || got != netip.MustParseAddr("192.0.2.55") {
		t.Errorf("ipToAddr(v4-mapped) = %v, want unmapped 192.0.2.55", got)
	}
}
