// Design: docs/architecture/isis/isis-12-ipv6.md TDD plan -- IPv6 redistribution both ways.
//
// VALIDATES: the consumer turns AFI=2 imports into TLV 236 entries with the
// external bit set and link-local rejected (AC-6, RFC 5308 sec 2); the source
// emits IS-IS IPv6 SPF routes as an AFI=2 redistevents batch (AC-5); the
// connected IPv6 helper builds TLV 236 PrefixInfoV6.

package isisredistribute

import (
	"context"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/plugins/isis/lsdb"
	"github.com/ze-software/ze/internal/plugins/isis/spf"
)

// TestISISRedistConsumerIPv6 -- a connected/BGP IPv6 import (AFI=2) becomes a
// TLV 236 entry with the external bit set on every origination level (AC-6).
func TestISISRedistConsumerIPv6(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1, lsdb.Level2)
	c := NewConsumer(inj)

	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{
		Prefix: "2001:db8:7::/64",
		Source: "bgp",
	})

	for _, level := range []lsdb.Level{lsdb.Level1, lsdb.Level2} {
		got := inj.snapshotV6(level)
		if len(got) != 1 {
			t.Fatalf("%s: got %d IPv6 prefixes, want 1", level, len(got))
		}
		e := got[0]
		if e.Prefix != netip.MustParsePrefix("2001:db8:7::/64") {
			t.Errorf("%s: prefix = %v, want 2001:db8:7::/64", level, e.Prefix)
		}
		if !e.External {
			t.Errorf("%s: redistributed IPv6 must set the external bit (RFC 5308 sec 2)", level)
		}
		if e.UpDown {
			t.Errorf("%s: up/down bit must be clear on injection (RFC 2966)", level)
		}
		if e.Metric.Value() != DefaultRedistMetric {
			t.Errorf("%s: metric = %d, want %d", level, e.Metric.Value(), DefaultRedistMetric)
		}
	}
}

// TestISISRedistConsumerIPv6LinkLocalRejected -- a link-local IPv6 import is NOT
// injected into TLV 236 (RFC 5308 sec 2).
func TestISISRedistConsumerIPv6LinkLocalRejected(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	c := NewConsumer(inj)
	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{
		Prefix: "fe80::/64",
		Source: "connected",
	})
	if got := inj.snapshotV6(lsdb.Level1); len(got) != 0 {
		t.Errorf("link-local IPv6 prefix must not be injected into TLV 236, got %+v", got)
	}
}

// TestISISRedistConsumerIPv6Withdraw -- a withdraw removes the TLV 236 entry.
func TestISISRedistConsumerIPv6Withdraw(t *testing.T) {
	inj := newFakeInjector(lsdb.Level1)
	c := NewConsumer(inj)
	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{Prefix: "2001:db8:9::/64", Source: "bgp"})
	if len(inj.snapshotV6(lsdb.Level1)) != 1 {
		t.Fatal("inject did not record the IPv6 prefix")
	}
	c.WithdrawRoute(context.Background(), family.IPv6Unicast, "2001:db8:9::/64")
	if got := inj.snapshotV6(lsdb.Level1); len(got) != 0 {
		t.Errorf("withdraw left %d IPv6 prefixes, want 0", len(got))
	}
}

// TestISISRedistSourceIPv6 -- the source emits IS-IS IPv6 SPF routes as a single
// redistevents batch at AFI=2 (AC-5).
func TestISISRedistSourceIPv6(t *testing.T) {
	delta := spf.RouteDelta{
		Added: []spf.RouteEntry{{
			Prefix:   netip.MustParsePrefix("2001:db8:1::/64"),
			Metric:   10,
			Level:    spf.Level1,
			NextHops: []spf.NextHop{{Addr: netip.MustParseAddr("fe80::1"), Interface: "eth0"}},
		}},
	}

	var captured []captureBatch
	emitDeltaFamily(delta, testProtocolID(), family.AFIIPv6, func(b *redistevents.RouteChangeBatch) {
		cb := captureBatch{protocol: b.Protocol, afi: b.AFI, safi: b.SAFI}
		cb.entries = append(cb.entries, b.Entries...)
		captured = append(captured, cb)
	})

	if len(captured) != 1 {
		t.Fatalf("got %d batches, want 1", len(captured))
	}
	b := captured[0]
	if b.afi != uint16(family.AFIIPv6) || b.safi != uint8(family.SAFIUnicast) {
		t.Fatalf("batch family = afi %d safi %d, want ipv6/unicast", b.afi, b.safi)
	}
	if len(b.entries) != 1 || b.entries[0].Prefix != netip.MustParsePrefix("2001:db8:1::/64") {
		t.Fatalf("batch entries = %+v, want one IPv6 add", b.entries)
	}
	if b.entries[0].Action != redistevents.ActionAdd {
		t.Errorf("action = %v, want add", b.entries[0].Action)
	}
}

// TestISISConnectedAdvertiseV6 -- enabled interface IPv6 prefixes become internal
// TLV 236 PrefixInfoV6 (masked, up/down + external 0); link-local dropped.
func TestISISConnectedAdvertiseV6(t *testing.T) {
	in := []netip.Prefix{
		netip.MustParsePrefix("2001:db8::5/64"), // host addr; masked network advertised
		netip.MustParsePrefix("fe80::1/64"),     // link-local, dropped
	}
	out := ConnectedPrefixInfosV6(in, 7)
	if len(out) != 1 {
		t.Fatalf("got %d prefixes, want 1 (link-local dropped)", len(out))
	}
	if out[0].Prefix != netip.MustParsePrefix("2001:db8::/64") {
		t.Errorf("prefix = %v, want masked 2001:db8::/64", out[0].Prefix)
	}
	if out[0].Metric.Value() != 7 || out[0].UpDown || out[0].External {
		t.Errorf("connected entry = %+v, want metric 7 up/down=false external=false", out[0])
	}
}
