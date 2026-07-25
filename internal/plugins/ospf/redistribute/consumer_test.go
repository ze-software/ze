// VALIDATES: spec-ospf-10 redistribution consumer -- RouteEntry (connected/static/
// bgp) injected as Type 5 via the ExternalInjector seam, withdraw purges, malformed
// prefixes rejected, source label recovered on withdraw, self-import name match.
// PREVENTS: regressions where the consumer swallows a failed origination, mislabels
// the source, or originates a Type 5 for a malformed/non-IPv4 prefix.
package ospfredistribute

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
)

type fakeInjector struct {
	injected    map[netip.Prefix]string
	withdrawn   map[netip.Prefix]bool
	injectErr   error
	withdrawErr error
}

func newFakeInjector() *fakeInjector {
	return &fakeInjector{injected: map[netip.Prefix]string{}, withdrawn: map[netip.Prefix]bool{}}
}

func (f *fakeInjector) InjectExternal(p netip.Prefix, source string) error {
	if f.injectErr != nil {
		return f.injectErr
	}
	f.injected[p] = source
	return nil
}

func (f *fakeInjector) WithdrawExternal(p netip.Prefix) (bool, error) {
	if f.withdrawErr != nil {
		return false, f.withdrawErr
	}
	_, ok := f.injected[p]
	delete(f.injected, p)
	f.withdrawn[p] = true
	return ok, nil
}

func TestOSPFRedistConsumerName(t *testing.T) {
	c := NewConsumer(newFakeInjector())
	assert.Equal(t, "ospf", c.Name())
	// The consumer name MUST equal the source name so the generic loop-prevention
	// evaluator auto-rejects self-import (AC-13).
	assert.Equal(t, ConsumerName, c.Name())
}

func TestOSPFRedistConsumerConnected(t *testing.T) {
	f := newFakeInjector()
	c := NewConsumer(f)
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{Prefix: "10.1.0.0/24", Source: "connected"})
	pfx := netip.MustParsePrefix("10.1.0.0/24")
	assert.Equal(t, "connected", f.injected[pfx], "connected prefix injected as Type 5 with source label")
}

func TestOSPFRedistConsumerStatic(t *testing.T) {
	f := newFakeInjector()
	c := NewConsumer(f)
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{Prefix: "192.0.2.0/24", Source: "static"})
	assert.Equal(t, "static", f.injected[netip.MustParsePrefix("192.0.2.0/24")])
}

func TestOSPFRedistConsumerBGP(t *testing.T) {
	f := newFakeInjector()
	c := NewConsumer(f)
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{Prefix: "203.0.113.0/24", Source: "bgp"})
	assert.Equal(t, "bgp", f.injected[netip.MustParsePrefix("203.0.113.0/24")])
}

func TestOSPFRedistConsumerWithdraw(t *testing.T) {
	f := newFakeInjector()
	c := NewConsumer(f)
	ctx := context.Background()
	c.InjectRoute(ctx, family.IPv4Unicast, configredist.RouteEntry{Prefix: "10.2.0.0/16", Source: "connected"})
	c.WithdrawRoute(ctx, family.IPv4Unicast, "10.2.0.0/16")
	pfx := netip.MustParsePrefix("10.2.0.0/16")
	assert.True(t, f.withdrawn[pfx], "withdraw purges the Type 5")
	_, still := f.injected[pfx]
	assert.False(t, still, "prefix removed from the injected set")
}

func TestOSPFRedistConsumerLogsFailure(t *testing.T) {
	f := newFakeInjector()
	f.injectErr = errors.New("origination failed")
	c := NewConsumer(f)
	// Must not panic; a failed origination is not remembered, so a later withdraw
	// resolves source="unknown" (the bookkeeping was never recorded).
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{Prefix: "10.3.0.0/16", Source: "connected"})
	assert.Equal(t, "unknown", c.forgetSource(netip.MustParsePrefix("10.3.0.0/16")))
}

func TestOSPFRedistConsumerMalformedPrefix(t *testing.T) {
	f := newFakeInjector()
	c := NewConsumer(f)
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{Prefix: "not-a-prefix", Source: "connected"})
	assert.Empty(t, f.injected, "malformed prefix is rejected, no Type 5 originated")
}

func TestOSPFRedistConsumerNonIPv4(t *testing.T) {
	f := newFakeInjector()
	c := NewConsumer(f)
	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{Prefix: "2001:db8::/32", Source: "connected"})
	assert.Empty(t, f.injected, "OSPFv2 is IPv4-only; IPv6 inject is a no-op")
}

func TestOSPFRedistConsumerV6Injector(t *testing.T) {
	// I8: with a wired v6 injector (SetV6Injector), an IPv6 redistribution forwards to it; the
	// cross-family guard (prefix.Addr().Is4() == want6) rejects an IPv4 prefix on the IPv6 path
	// and an IPv6 prefix on the IPv4 path, so a route never reaches the wrong-family injector.
	v4 := newFakeInjector()
	v6 := newFakeInjector()
	c := NewConsumer(v4)
	c.SetV6Injector(v6)
	ctx := context.Background()

	// IPv6 family + IPv6 prefix -> forwarded to the v6 injector only.
	c.InjectRoute(ctx, family.IPv6Unicast, configredist.RouteEntry{Prefix: "2001:db8:6::/48", Source: "bgp"})
	assert.Equal(t, "bgp", v6.injected[netip.MustParsePrefix("2001:db8:6::/48")], "IPv6 prefix injected via the v6 injector")
	assert.Empty(t, v4.injected, "IPv6 prefix must not reach the v4 injector")

	// IPv6 family + IPv4 prefix -> rejected by the cross-family guard.
	c.InjectRoute(ctx, family.IPv6Unicast, configredist.RouteEntry{Prefix: "10.9.0.0/16", Source: "connected"})
	if _, got := v6.injected[netip.MustParsePrefix("10.9.0.0/16")]; got {
		t.Fatal("an IPv4 prefix on the IPv6 path must be rejected (cross-family guard)")
	}

	// IPv4 family + IPv6 prefix -> rejected by the cross-family guard.
	c.InjectRoute(ctx, family.IPv4Unicast, configredist.RouteEntry{Prefix: "2001:db8:4::/48", Source: "connected"})
	if _, got := v4.injected[netip.MustParsePrefix("2001:db8:4::/48")]; got {
		t.Fatal("an IPv6 prefix on the IPv4 path must be rejected (cross-family guard)")
	}

	// Withdraw the v6 route through the v6 injector.
	c.WithdrawRoute(ctx, family.IPv6Unicast, "2001:db8:6::/48")
	assert.True(t, v6.withdrawn[netip.MustParsePrefix("2001:db8:6::/48")], "v6 withdraw purges via the v6 injector")
}

func TestOSPFRedistConsumer4in6Guard(t *testing.T) {
	// An IPv4-mapped IPv6 prefix (::ffff:a.b.c.d) must not slip past the family guard: it is
	// canonicalized to its IPv4 form, so the IPv6 injector rejects it (it is really IPv4) and the
	// IPv4 injector receives the unmapped v4 prefix.
	v4 := newFakeInjector()
	v6 := newFakeInjector()
	c := NewConsumer(v4)
	c.SetV6Injector(v6)
	ctx := context.Background()

	c.InjectRoute(ctx, family.IPv6Unicast, configredist.RouteEntry{Prefix: "::ffff:10.8.0.0/120", Source: "connected"})
	assert.Empty(t, v6.injected, "a 4-in-6 (IPv4-mapped) prefix must not be injected as IPv6")

	c.InjectRoute(ctx, family.IPv4Unicast, configredist.RouteEntry{Prefix: "::ffff:10.9.0.0/120", Source: "connected"})
	assert.Equal(t, "connected", v4.injected[netip.MustParsePrefix("10.9.0.0/24")], "a 4-in-6 prefix on the v4 path is injected as the unmapped /24")
}

func TestOSPFRedistSelfImportRejected(t *testing.T) {
	// Self-import rejection is enforced by the generic evaluator (origin ==
	// importing protocol). The precondition the consumer guarantees is that its name
	// equals the redistribution source name, both "ospf".
	require.Equal(t, "ospf", ConsumerName)
	RegisterOSPFSources()
	_, ok := configredist.LookupSource("ospf")
	assert.True(t, ok, "source 'ospf' registered; matches consumer name so self-import is auto-rejected")
}
