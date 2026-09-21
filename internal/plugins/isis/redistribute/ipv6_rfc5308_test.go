// Design: docs/architecture/isis/isis-11-redistribution.md -- IPv6 redistribution into TLV 236.
// RFC: rfc/short/rfc5308.md -- Section 2 (the external bit of TLV 236).
package isisredistribute

import (
	"context"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
)

// RFC requirement: RFC5308-2-3 positive -- an IPv6 prefix distributed into IS-IS from
// another routing protocol is originated in TLV 236 with the external (X) bit set at every
// origination level (injectRouteV6, ipv6.go, builds PrefixInfoV6{External: true}).
func TestRFC5308RedistributedIPv6SetsExternalBit(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1, lsdb.Level2)
	c := NewConsumer(inj)
	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{Prefix: "2001:db8:9::/48", Source: "bgp"})

	for _, level := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		got := inj.snapshotV6(level)
		if len(got) != 1 {
			t.Fatalf("%s: got %d IPv6 entries, want 1", level, len(got))
		}
		if got[0].Prefix != netip.MustParsePrefix("2001:db8:9::/48") {
			t.Fatalf("%s: prefix = %v, want 2001:db8:9::/48", level, got[0].Prefix)
		}
		if !got[0].External {
			t.Fatalf("%s: external bit clear on a redistributed prefix: %+v", level, got[0])
		}
	}
}

// RFC requirement: RFC5308-2-3 negative -- an IPv6 prefix that IS-IS learns from the
// router's own connected interfaces was not distributed from another routing protocol, so
// its TLV 236 entry carries the external bit 0 (ConnectedPrefixInfosV6, ipv6.go).
func TestRFC5308ConnectedIPv6ClearsExternalBit(t *testing.T) {
	out := ConnectedPrefixInfosV6([]netip.Prefix{netip.MustParsePrefix("2001:db8::5/64")}, 7)
	if len(out) != 1 {
		t.Fatalf("got %d entries, want 1", len(out))
	}
	if out[0].Prefix != netip.MustParsePrefix("2001:db8::/64") {
		t.Fatalf("prefix = %v, want masked 2001:db8::/64", out[0].Prefix)
	}
	if out[0].External {
		t.Fatalf("external bit set on a connected prefix: %+v", out[0])
	}
}
