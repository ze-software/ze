// VALIDATES: spec-static-route-tag-reaches-no-consumer AC-3 -- the OSPF redistribution
// consumer hands the RouteEntry's tag to the ExternalInjector, for IPv4 and IPv6.
// PREVENTS: the tag reaching the consumer and being dropped at the injector seam, which
// looks from the LSA exactly like a producer that never set it.
package ospfredistribute

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
)

func TestOSPFRedistConsumerPassesRouteTag(t *testing.T) {
	f := newFakeInjector()
	c := NewConsumer(f)

	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "10.1.0.0/24", Source: "static", Tag: 4242,
	})
	c.InjectRoute(context.Background(), family.IPv4Unicast, configredist.RouteEntry{
		Prefix: "10.2.0.0/24", Source: "static",
	})

	assert.Equal(t, uint32(4242), f.tags[netip.MustParsePrefix("10.1.0.0/24")])
	assert.Equal(t, uint32(0), f.tags[netip.MustParsePrefix("10.2.0.0/24")], "an untagged route reaches the injector as zero")
}

func TestOSPFRedistConsumerPassesRouteTagIPv6(t *testing.T) {
	f6 := newFakeInjector()
	c := NewConsumer(newFakeInjector())
	c.SetV6Injector(f6)

	c.InjectRoute(context.Background(), family.IPv6Unicast, configredist.RouteEntry{
		Prefix: "2001:db8::/32", Source: "static", Tag: 99,
	})

	assert.Equal(t, uint32(99), f6.tags[netip.MustParsePrefix("2001:db8::/32")])
}
