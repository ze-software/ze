package l2tpauthradius

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

func TestExtractFramedIP(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(10, 0, 0, 5).To4()},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	want := netip.MustParseAddr("10.0.0.5")
	if meta.FramedIP != want {
		t.Errorf("FramedIP = %v, want %v", meta.FramedIP, want)
	}
}

func TestExtractFramedIPRejectsMulticast(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(224, 0, 0, 1).To4()},
	}}
	meta := extractAuthMetadata(resp)
	if meta != nil {
		t.Error("multicast address should be rejected")
	}
}

func TestExtractFramedIPRejectsLoopback(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(127, 0, 0, 1).To4()},
	}}
	meta := extractAuthMetadata(resp)
	if meta != nil {
		t.Error("loopback address should be rejected")
	}
}

func TestExtractFramedIPRejectsBroadcast(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(255, 255, 255, 255).To4()},
	}}
	meta := extractAuthMetadata(resp)
	if meta != nil {
		t.Error("broadcast address should be rejected")
	}
}

func TestExtractFramedIPRejectsLinkLocal(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(169, 254, 1, 1).To4()},
	}}
	meta := extractAuthMetadata(resp)
	if meta != nil {
		t.Error("link-local address should be rejected")
	}
}

func TestExtractFramedIPShortValue(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: []byte{10, 0}},
	}}
	meta := extractAuthMetadata(resp)
	if meta != nil {
		t.Error("short Framed-IP-Address value should be ignored")
	}
}

func TestExtractFramedPool(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedPool, Value: radius.AttrString("gold")},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.FramedPool != "gold" {
		t.Errorf("FramedPool = %q, want %q", meta.FramedPool, "gold")
	}
}

func TestExtractSessionTimeout(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(3600)},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.SessionTimeout != 3600 {
		t.Errorf("SessionTimeout = %d, want 3600", meta.SessionTimeout)
	}
}

func TestExtractIdleTimeout(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrIdleTimeout, Value: radius.AttrUint32(300)},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.IdleTimeout != 300 {
		t.Errorf("IdleTimeout = %d, want 300", meta.IdleTimeout)
	}
}

func TestExtractFilterId(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFilterID, Value: radius.AttrString("rate:20M/5M")},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.FilterID != "rate:20M/5M" {
		t.Errorf("FilterID = %q, want %q", meta.FilterID, "rate:20M/5M")
	}
}

func TestExtractAcctInterimInterval(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrAcctInterimInterval, Value: radius.AttrUint32(60)},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.AcctInterimInterval != 60 {
		t.Errorf("AcctInterimInterval = %d, want 60", meta.AcctInterimInterval)
	}
}

func TestExtractFramedNetmask(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPNetmask, Value: net.IPv4Mask(255, 255, 255, 0)},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	want := net.IPv4Mask(255, 255, 255, 0)
	if !net.IP(meta.FramedNetmask).Equal(net.IP(want)) {
		t.Errorf("FramedNetmask = %v, want %v", meta.FramedNetmask, want)
	}
}

func TestExtractNoProfileAttributes(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrReplyMessage, Value: radius.AttrString("welcome")},
	}}
	meta := extractAuthMetadata(resp)
	if meta != nil {
		t.Error("expected nil metadata when no profile attributes present")
	}
}

func TestExtractMultipleAttributes(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(198, 51, 100, 10).To4()},
		{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(600)},
		{Type: radius.AttrFilterID, Value: radius.AttrString("10M")},
		{Type: radius.AttrAcctInterimInterval, Value: radius.AttrUint32(120)},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if meta.FramedIP != netip.MustParseAddr("198.51.100.10") {
		t.Errorf("FramedIP = %v", meta.FramedIP)
	}
	if meta.SessionTimeout != 600 {
		t.Errorf("SessionTimeout = %d", meta.SessionTimeout)
	}
	if meta.FilterID != "10M" {
		t.Errorf("FilterID = %q", meta.FilterID)
	}
	if meta.AcctInterimInterval != 120 {
		t.Errorf("AcctInterimInterval = %d", meta.AcctInterimInterval)
	}
}

// VALIDATES: AC-5 -- Framed-Route "10.0.0.0/8 0.0.0.0 1" parsed correctly.
func TestExtractFramedRoute(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedRoute, Value: radius.AttrString("10.0.0.0/8 0.0.0.0 1")},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if len(meta.FramedRoutes) != 1 {
		t.Fatalf("FramedRoutes len = %d, want 1", len(meta.FramedRoutes))
	}
	if meta.FramedRoutes[0].Prefix != netip.MustParsePrefix("10.0.0.0/8") {
		t.Errorf("prefix = %v, want 10.0.0.0/8", meta.FramedRoutes[0].Prefix)
	}
	if meta.FramedRoutes[0].Metric != 1 {
		t.Errorf("metric = %d, want 1", meta.FramedRoutes[0].Metric)
	}
}

// VALIDATES: AC-9 -- Framed-IPv6-Route parsed correctly.
func TestExtractFramedIPv6Route(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedIPv6Route, Value: radius.AttrString("2001:db8::/32 :: 1")},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if len(meta.FramedRoutes) != 1 {
		t.Fatalf("FramedRoutes len = %d, want 1", len(meta.FramedRoutes))
	}
	if meta.FramedRoutes[0].Prefix != netip.MustParsePrefix("2001:db8::/32") {
		t.Errorf("prefix = %v, want 2001:db8::/32", meta.FramedRoutes[0].Prefix)
	}
	if meta.FramedRoutes[0].Metric != 1 {
		t.Errorf("metric = %d, want 1", meta.FramedRoutes[0].Metric)
	}
}

// VALIDATES: AC-8 -- multiple Framed-Route attributes all parsed.
func TestExtractMultipleFramedRoutes(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedRoute, Value: radius.AttrString("10.0.0.0/8 0.0.0.0 1")},
		{Type: radius.AttrFramedRoute, Value: radius.AttrString("172.16.0.0/12 0.0.0.0 2")},
		{Type: radius.AttrFramedIPv6Route, Value: radius.AttrString("2001:db8::/32 :: 5")},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if len(meta.FramedRoutes) != 3 {
		t.Fatalf("FramedRoutes len = %d, want 3", len(meta.FramedRoutes))
	}
}

func TestExtractFramedRouteNoMetric(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedRoute, Value: radius.AttrString("10.0.0.0/8 0.0.0.0")},
	}}
	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("expected metadata")
	}
	if len(meta.FramedRoutes) != 1 {
		t.Fatalf("FramedRoutes len = %d, want 1", len(meta.FramedRoutes))
	}
	if meta.FramedRoutes[0].Metric != 0 {
		t.Errorf("metric = %d, want 0", meta.FramedRoutes[0].Metric)
	}
}

func TestExtractFramedRouteMalformed(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrFramedRoute, Value: radius.AttrString("not-a-prefix")},
	}}
	meta := extractAuthMetadata(resp)
	if meta != nil {
		t.Error("malformed Framed-Route should not produce metadata")
	}
}

func TestParseFramedRoute(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		wantOK bool
		prefix string
		metric uint32
	}{
		{"basic", "10.0.0.0/8 0.0.0.0 1", true, "10.0.0.0/8", 1},
		{"no metric", "192.168.0.0/16 0.0.0.0", true, "192.168.0.0/16", 0},
		{"ipv6", "2001:db8::/32 :: 5", true, "2001:db8::/32", 5},
		{"max metric", "10.0.0.0/8 0.0.0.0 4294967295", true, "10.0.0.0/8", 4294967295},
		{"host masked", "10.0.0.1/8 0.0.0.0 0", true, "10.0.0.0/8", 0},
		{"empty", "", false, "", 0},
		{"one field", "10.0.0.0/8", false, "", 0},
		{"bad prefix", "not-a-prefix 0.0.0.0 1", false, "", 0},
		{"bad metric", "10.0.0.0/8 0.0.0.0 abc", false, "", 0},
		{"metric overflow", "10.0.0.0/8 0.0.0.0 99999999999", false, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, ok := parseFramedRoute(tt.text)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if r.Prefix != netip.MustParsePrefix(tt.prefix) {
				t.Errorf("prefix = %v, want %v", r.Prefix, tt.prefix)
			}
			if r.Metric != tt.metric {
				t.Errorf("metric = %d, want %d", r.Metric, tt.metric)
			}
		})
	}
}

func TestIsValidSubscriberIP(t *testing.T) {
	tests := []struct {
		name string
		addr netip.Addr
		want bool
	}{
		{"unicast", netip.MustParseAddr("10.0.0.1"), true},
		{"public", netip.MustParseAddr("198.51.100.1"), true},
		{"multicast", netip.MustParseAddr("224.0.0.1"), false},
		{"loopback", netip.MustParseAddr("127.0.0.1"), false},
		{"link-local", netip.MustParseAddr("169.254.1.1"), false},
		{"broadcast", netip.MustParseAddr("255.255.255.255"), false},
		{"unspecified", netip.MustParseAddr("0.0.0.0"), false},
		{"ipv6", netip.MustParseAddr("::1"), false},
		{"zero", netip.Addr{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSubscriberIP(tt.addr)
			if got != tt.want {
				t.Errorf("isValidSubscriberIP(%v) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}
