// Design: docs/architecture/isis/isis-11-redistribution.md -- first injection of a prefix.
// RFC: rfc/short/rfc5305.md -- Section 4.1 (the up/down bit at first injection).
package isisredistribute

import (
	"context"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
)

// RFC requirement: RFC5305-4.1-2 positive -- a prefix first injected into IS-IS by
// redistribution is originated in TLV 135 with the up/down bit 0 at every origination
// level (InjectRoute, consumer.go, builds PrefixInfo{UpDown: false}).
func TestRFC5305FirstInjectionClearsUpDown(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1, lsdb.Level2)
	c := NewConsumer(inj)
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{Prefix: "198.51.100.0/24", Source: "static"})

	for _, level := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		got := inj.snapshot(level)
		if len(got) != 1 {
			t.Fatalf("%s: got %d entries, want 1", level, len(got))
		}
		if got[0].Prefix != netip.MustParsePrefix("198.51.100.0/24") {
			t.Fatalf("%s: prefix = %v, want 198.51.100.0/24", level, got[0].Prefix)
		}
		if got[0].UpDown {
			t.Fatalf("%s: up/down bit set on first injection: %+v", level, got[0])
		}
	}
}

// RFC requirement: RFC5305-4.1-2 negative -- a connected prefix first injected from the
// router's own interfaces is also originated with the up/down bit 0, so the clear bit is
// the injection contract and not a property of one source (ConnectedPrefixInfos,
// source.go); the bit is set only later, by the L2->L1 leak in spf.LeakPrefixes.
func TestRFC5305ConnectedInjectionClearsUpDown(t *testing.T) {
	out := ConnectedPrefixInfos([]netip.Prefix{netip.MustParsePrefix("192.0.2.9/24")}, 10)
	if len(out) != 1 {
		t.Fatalf("got %d entries, want 1", len(out))
	}
	if out[0].Prefix != netip.MustParsePrefix("192.0.2.0/24") {
		t.Fatalf("prefix = %v, want masked 192.0.2.0/24", out[0].Prefix)
	}
	if out[0].UpDown {
		t.Fatalf("up/down bit set on a connected prefix's first injection: %+v", out[0])
	}
}
