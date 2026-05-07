package l2tpauthradius

import (
	"net"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/radius"
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
