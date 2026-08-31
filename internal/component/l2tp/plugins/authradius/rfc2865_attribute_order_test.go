// RFC 2865 Section 5 attribute order independence, on the client's read path.
//
// VALIDATES: ze reads an Access-Accept by attribute TYPE, so one attribute set
// delivered in two different orders yields one subscriber profile.
// PREVENTS: a read keyed on position, which answers differently for two servers
// that send the same attributes in a different order.
//
// The path exercised is the one a subscriber login takes: the server's bytes
// reach radius.Decode, and doRADIUS hands the decoded packet to
// extractAuthMetadata. The producers exercised are radius.Packet.Decode and
// radius.Packet.FindAttr.

package l2tpauthradius

import (
	"bytes"
	"net"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/radius"
)

// accessAcceptProfile is one Access-Accept attribute set: six attributes of six
// different types, every one of which extractAuthMetadata reads. Six distinct
// types is what makes the reversal below change the type at every position.
func accessAcceptProfile() []radius.Attr {
	return []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: []byte{192, 0, 2, 10}},
		{Type: radius.AttrFramedIPNetmask, Value: []byte{255, 255, 255, 0}},
		{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(3600)},
		{Type: radius.AttrIdleTimeout, Value: radius.AttrUint32(600)},
		{Type: radius.AttrFilterID, Value: radius.AttrString("subscriber-in")},
		{Type: radius.AttrFramedPool, Value: radius.AttrString("pool-a")},
	}
}

// reversedAttrs answers the same attributes in the opposite order.
func reversedAttrs(attrs []radius.Attr) []radius.Attr {
	out := make([]radius.Attr, len(attrs))
	for index, attr := range attrs {
		out[len(attrs)-1-index] = attr
	}
	return out
}

// wireRoundTrip puts one attribute set on the wire and reads it back, so each
// case sees what the client sees rather than the slice the test built.
func wireRoundTrip(t *testing.T, attrs []radius.Attr) *radius.Packet {
	t.Helper()
	pkt := &radius.Packet{Code: radius.CodeAccessAccept, Identifier: 7, Attrs: attrs}
	buf := make([]byte, radius.MaxPacketLen)
	written, err := pkt.EncodeTo(buf, 0)
	if err != nil {
		t.Fatalf("encode Access-Accept: %v", err)
	}
	decoded, err := radius.Decode(buf[:written])
	if err != nil {
		t.Fatalf("decode Access-Accept: %v", err)
	}
	return decoded
}

// RFC requirement: RFC2865-5-4 positive -- six attributes of six different
// types, sent forward and sent reversed, extract to the same subscriber
// profile: the same framed address, netmask, pool, session timeout, idle
// timeout and Filter-Id, with every other field of the profile still zero.
func TestRFC2865AccessAcceptExtractionIsOrderIndependent(t *testing.T) {
	want := l2tp.AuthMetadata{
		FramedIP:       netip.AddrFrom4([4]byte{192, 0, 2, 10}),
		FramedNetmask:  net.IPv4Mask(255, 255, 255, 0),
		FramedPool:     "pool-a",
		SessionTimeout: 3600,
		IdleTimeout:    600,
		FilterID:       "subscriber-in",
	}

	orders := []struct {
		name  string
		attrs []radius.Attr
	}{
		{"forward", accessAcceptProfile()},
		{"reversed", reversedAttrs(accessAcceptProfile())},
	}

	for _, order := range orders {
		t.Run(order.name, func(t *testing.T) {
			meta := extractAuthMetadata(wireRoundTrip(t, order.attrs))
			if meta == nil {
				t.Fatal("an Access-Accept carrying six profile attributes extracted no profile")
			}
			if !reflect.DeepEqual(*meta, want) {
				t.Errorf("profile: got %+v, want %+v", *meta, want)
			}
		})
	}
}

// RFC requirement: RFC2865-5-4 negative -- the reversal the case above relies on
// is a real one, so the sameness it reports is not sameness of two identical
// packets. Every one of the six positions carries a different attribute type in
// the two decoded packets, so a read keyed on position answers differently for
// each of them, while the type-keyed read answers Framed-IP-Address 192.0.2.10
// for both.
func TestRFC2865AttributeOrderIsObservableByPosition(t *testing.T) {
	forward := wireRoundTrip(t, accessAcceptProfile())
	reversed := wireRoundTrip(t, reversedAttrs(accessAcceptProfile()))

	if len(forward.Attrs) != len(reversed.Attrs) {
		t.Fatalf("attribute count: forward %d, reversed %d", len(forward.Attrs), len(reversed.Attrs))
	}
	for position := range forward.Attrs {
		if forward.Attrs[position].Type == reversed.Attrs[position].Type {
			t.Errorf("position %d carries type %d in both orders, so a positional read "+
				"would not tell the two packets apart there",
				position, forward.Attrs[position].Type)
		}
	}

	wantAddr := []byte{192, 0, 2, 10}
	if !bytes.Equal(forward.FindAttr(radius.AttrFramedIPAddress), wantAddr) {
		t.Errorf("forward Framed-IP-Address: got %v, want %v",
			forward.FindAttr(radius.AttrFramedIPAddress), wantAddr)
	}
	if !bytes.Equal(reversed.FindAttr(radius.AttrFramedIPAddress), wantAddr) {
		t.Errorf("reversed Framed-IP-Address: got %v, want %v",
			reversed.FindAttr(radius.AttrFramedIPAddress), wantAddr)
	}
}
