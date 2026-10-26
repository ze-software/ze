// RFC 3162 obligations on the NAS: what ze puts in an Access-Request and what
// it does with the IPv6 attributes of an Access-Accept.
//
// VALIDATES: an Access-Request sent from an IPv6 source names the NAS by
// NAS-Identifier, since ze sends no NAS-IPv6-Address (Section 2.1); the text
// of a Framed-IPv6-Route never stops the Access-Accept being honored
// (Section 2.5); a Framed-IPv6-Pool is ignored by a NAS without prefix pools
// (Section 2.6).
// PREVENTS: an Access-Request that names no NAS, an Access-Accept dropped
// because one route text did not parse, and a pool name changing what ze
// assigns.

package l2tpauthradius

import (
	"net"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// rfc3162FramedIPv6Pool is the Framed-IPv6-Pool attribute type (RFC 3162
// Section 2.6). The radius dictionary declares no constant for it because ze
// never reads it, which is what this file proves.
const rfc3162FramedIPv6Pool = 100

// RFC requirement: RFC3162-2.1-1 positive -- an Access-Request ze sends with an
// IPv6 source-address and no nas-identifier configured carries neither
// NAS-IP-Address nor NAS-IPv6-Address, and so carries a NAS-Identifier of
// non-zero length.
func TestRFC3162AccessRequestFromAnIPv6SourceNamesTheNAS(t *testing.T) {
	a, srv, resp := setupCapturingAuthIdentity(t, []byte("testing123"), "", net.ParseIP("2001:db8::7"))
	a.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte("password123"),
	}, resp.respond)
	resp.waitOne(t)

	pkt := decodeCaptured(t, srv)
	if pkt.FindAttr(radius.AttrNASIPAddress) != nil {
		t.Error("an IPv6 source-address is not an IPv4 NAS-IP-Address")
	}
	if pkt.FindAttr(radius.AttrNASIPv6Address) != nil {
		t.Fatal("ze sends no NAS-IPv6-Address; this test assumes it and must change if it does")
	}
	nasID := pkt.FindAttr(radius.AttrNASIdentifier)
	if nasID == nil {
		t.Fatal("neither NAS-IP-Address nor NAS-IPv6-Address is present, so NAS-Identifier MUST be")
	}
	if len(nasID) == 0 {
		t.Error("NAS-Identifier is text of length zero, which names no NAS")
	}
}

// RFC requirement: RFC3162-2.5-1 negative -- a Framed-IPv6-Route whose text does
// not parse is skipped, and the rest of the Access-Accept is still read: the
// Session-Timeout and the well-formed route beside it reach the session.
func TestRFC3162FramedIPv6RouteTextDoesNotStopTheAccept(t *testing.T) {
	resp := &radius.Packet{Attrs: []radius.Attr{
		{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(3600)},
		{Type: radius.AttrFramedIPv6Route, Value: radius.AttrString("not a route at all")},
		{Type: radius.AttrFramedIPv6Route, Value: radius.AttrString("2001:db8:5::/48 :: 1")},
	}}

	meta := extractAuthMetadata(resp)
	if meta == nil {
		t.Fatal("an unparseable route text discarded the whole Access-Accept profile")
	}
	if meta.SessionTimeout != 3600 {
		t.Errorf("SessionTimeout = %d, want 3600: the route text changed another attribute", meta.SessionTimeout)
	}
	if len(meta.FramedRoutes) != 1 {
		t.Fatalf("FramedRoutes = %v, want only the well-formed route", meta.FramedRoutes)
	}
	if got := meta.FramedRoutes[0].Prefix.String(); got != "2001:db8:5::/48" {
		t.Errorf("route prefix = %s, want 2001:db8:5::/48", got)
	}
}

// RFC requirement: RFC3162-2.6-1 positive -- ze supports no IPv6 prefix pools,
// and an Access-Accept carrying a Framed-IPv6-Pool yields exactly the profile
// the same Access-Accept yields without it.
func TestRFC3162FramedIPv6PoolIsIgnored(t *testing.T) {
	base := []radius.Attr{
		{Type: radius.AttrFramedIPAddress, Value: net.IPv4(10, 0, 0, 5).To4()},
		{Type: radius.AttrSessionTimeout, Value: radius.AttrUint32(600)},
	}
	withPool := append(append([]radius.Attr(nil), base...),
		radius.Attr{Type: rfc3162FramedIPv6Pool, Value: radius.AttrString("v6-pool-a")})

	want := extractAuthMetadata(&radius.Packet{Attrs: base})
	got := extractAuthMetadata(&radius.Packet{Attrs: withPool})
	if want == nil {
		t.Fatal("the profile without the pool attribute produced no metadata")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Framed-IPv6-Pool changed the profile:\n got %+v\nwant %+v", got, want)
	}
}
