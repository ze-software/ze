package message

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// mustBuildUnicast is BuildUnicast for a test whose route has a usable next
// hop: a refusal fails the test.
func mustBuildUnicast(t *testing.T, ub *UpdateBuilder, p *UnicastParams) *Update {
	t.Helper()
	upd, err := ub.BuildUnicast(p)
	if err != nil {
		t.Fatalf("BuildUnicast(%s via %s): %v", p.Prefix, p.NextHop, err)
	}
	return upd
}

// TestBuildUnicastRefusesIPv4RouteWithNoUsableNextHop proves BuildUnicast never
// produces an UPDATE whose NLRI field holds an IPv4 route with no NEXT_HOP.
//
// VALIDATES: an IPv4 unicast route whose next hop is IPv6 without Extended Next
// Hop, or is unset, is refused with ErrUnicastNextHopUnusable and no Update.
// PREVENTS: the prefix written into the body NLRI field with no NEXT_HOP
// attribute, which `ze bgp encode` printed before the refusal existed.
//
// RFC requirement: RFC4271-5-2 negative -- BuildUnicast refuses (ErrUnicastNextHopUnusable, nil Update) an IPv4 unicast route whose next hop is IPv6 without Extended Next Hop, or unset, rather than build an UPDATE with NLRI and no NEXT_HOP.
func TestBuildUnicastRefusesIPv4RouteWithNoUsableNextHop(t *testing.T) {
	tests := []struct {
		name    string
		nextHop netip.Addr
	}{
		{name: "ipv6 next hop without extended next hop", nextHop: netip.MustParseAddr("2001:db8::1")},
		{name: "no next hop", nextHop: netip.Addr{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := &UnicastParams{
				Prefix:  netip.MustParsePrefix("198.18.0.0/15"),
				NextHop: tt.nextHop,
				Origin:  attribute.OriginIGP,
			}
			upd, err := NewUpdateBuilder(65001, false, true, false).BuildUnicast(params)
			if !errors.Is(err, ErrUnicastNextHopUnusable) {
				t.Fatalf("err = %v, want ErrUnicastNextHopUnusable", err)
			}
			if upd != nil {
				t.Fatalf("a refused route still returned an Update: %+v", upd)
			}
		})
	}
}

// TestBuildUnicastIPv4NextHopWithExtendedNextHopKeepsNextHop proves the
// Extended Next Hop flag does not drop NEXT_HOP from an IPv4 route whose next
// hop is IPv4.
//
// VALIDATES: the route still goes in the body NLRI field, and ORIGIN, AS_PATH
// and NEXT_HOP are all present.
// PREVENTS: the NEXT_HOP arm skipping the attribute whenever the flag is set,
// although RFC 8950 moves only an IPv6 next hop into MP_REACH_NLRI.
func TestBuildUnicastIPv4NextHopWithExtendedNextHopKeepsNextHop(t *testing.T) {
	params := &UnicastParams{
		Prefix:             netip.MustParsePrefix("198.18.0.0/15"),
		NextHop:            netip.MustParseAddr("192.0.2.1"),
		Origin:             attribute.OriginIGP,
		UseExtendedNextHop: true,
	}
	upd := mustBuildUnicast(t, NewUpdateBuilder(65001, false, true, false), params)
	if len(upd.NLRI) == 0 {
		t.Fatal("an IPv4 route with an IPv4 next hop left the body NLRI field empty")
	}
	requireMandatoryWithNLRI(t, upd, params.Prefix.String(), false)
}
