package migration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestExtractStaticRoutesIPv4 verifies static→announce for IPv4 routes.
//
// VALIDATES: neighbor.static routes become peer.announce.ipv4.unicast.
//
// PREVENTS: IPv4 routes being lost during migration.
func TestExtractStaticRoutesIPv4(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
    static {
        route 10.0.0.0/8 next-hop self;
        route 192.168.0.0/16 next-hop 1.2.3.4;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	// neighbor should still exist (we only transform static→announce)
	neighbors := result.GetList("neighbor")
	require.Len(t, neighbors, 1)

	neighbor := neighbors["192.0.2.1"]
	require.NotNil(t, neighbor)

	// static should be gone
	static := neighbor.GetContainer("static")
	require.Nil(t, static, "static block should be removed")

	// announce.ipv4.unicast should have the routes
	announce := neighbor.GetContainer("announce")
	require.NotNil(t, announce, "announce block should exist")

	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4, "ipv4 block should exist")

	unicast := ipv4.GetList("unicast")
	require.Len(t, unicast, 2)

	// Check first route
	route1 := unicast["10.0.0.0/8"]
	require.NotNil(t, route1)
	nh, ok := route1.Get("next-hop")
	require.True(t, ok)
	require.Equal(t, "self", nh)

	// Check second route
	route2 := unicast["192.168.0.0/16"]
	require.NotNil(t, route2)
	nh, ok = route2.Get("next-hop")
	require.True(t, ok)
	require.Equal(t, "1.2.3.4", nh)
}

// TestExtractStaticRoutesIPv6 verifies static→announce for IPv6 routes.
//
// VALIDATES: IPv6 routes go to peer.announce.ipv6.unicast.
//
// PREVENTS: IPv6 routes being misclassified as IPv4.
func TestExtractStaticRoutesIPv6(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 2001:db8::/32 next-hop self;
        route ::ffff:10.0.0.0/104 next-hop 2001:db8::1;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, neighbor)

	// static should be gone
	require.Nil(t, neighbor.GetContainer("static"))

	// announce.ipv6.unicast should have the routes
	announce := neighbor.GetContainer("announce")
	require.NotNil(t, announce)

	ipv6 := announce.GetContainer("ipv6")
	require.NotNil(t, ipv6)

	unicast := ipv6.GetList("unicast")
	require.Len(t, unicast, 2)

	// Both routes should be in ipv6
	require.NotNil(t, unicast["2001:db8::/32"])
	require.NotNil(t, unicast["::ffff:10.0.0.0/104"])
}

// TestExtractStaticRoutesMixed verifies mixed IPv4/IPv6 routes.
//
// VALIDATES: Routes are correctly separated by AFI.
//
// PREVENTS: Mixed routes being placed in wrong AFI container.
func TestExtractStaticRoutesMixed(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 next-hop self;
        route 2001:db8::/32 next-hop self;
        route 172.16.0.0/12 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	announce := neighbor.GetContainer("announce")

	// Check IPv4
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	ipv4Unicast := ipv4.GetList("unicast")
	require.Len(t, ipv4Unicast, 2)
	require.NotNil(t, ipv4Unicast["10.0.0.0/8"])
	require.NotNil(t, ipv4Unicast["172.16.0.0/12"])

	// Check IPv6
	ipv6 := announce.GetContainer("ipv6")
	require.NotNil(t, ipv6)
	ipv6Unicast := ipv6.GetList("unicast")
	require.Len(t, ipv6Unicast, 1)
	require.NotNil(t, ipv6Unicast["2001:db8::/32"])
}

// TestExtractStaticRoutesMulticast verifies multicast route detection.
//
// VALIDATES: Multicast prefixes go to .<afi>.multicast.
//
// PREVENTS: Multicast routes being classified as unicast.
func TestExtractStaticRoutesMulticast(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 224.0.0.0/4 next-hop self;
        route ff02::1/128 next-hop self;
        route 10.0.0.0/8 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	announce := neighbor.GetContainer("announce")

	// IPv4 multicast
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	ipv4Mcast := ipv4.GetList("multicast")
	require.Len(t, ipv4Mcast, 1)
	require.NotNil(t, ipv4Mcast["224.0.0.0/4"])

	// IPv4 unicast
	ipv4Unicast := ipv4.GetList("unicast")
	require.Len(t, ipv4Unicast, 1)
	require.NotNil(t, ipv4Unicast["10.0.0.0/8"])

	// IPv6 multicast
	ipv6 := announce.GetContainer("ipv6")
	require.NotNil(t, ipv6)
	ipv6Mcast := ipv6.GetList("multicast")
	require.Len(t, ipv6Mcast, 1)
	require.NotNil(t, ipv6Mcast["ff02::1/128"])
}

// TestExtractStaticRoutesMPLSVPN verifies MPLS-VPN route detection.
//
// VALIDATES: Routes with rd go to .<afi>.mpls-vpn (SAFI 128).
//
// PREVENTS: L3VPN routes being classified as unicast.
func TestExtractStaticRoutesMPLSVPN(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 rd 65000:1 next-hop self;
        route 192.168.0.0/16 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	announce := neighbor.GetContainer("announce")
	ipv4 := announce.GetContainer("ipv4")

	// MPLS-VPN route (has rd)
	mplsVPN := ipv4.GetList("mpls-vpn")
	require.Len(t, mplsVPN, 1)
	require.NotNil(t, mplsVPN["10.0.0.0/8"])

	// Unicast route
	unicast := ipv4.GetList("unicast")
	require.Len(t, unicast, 1)
	require.NotNil(t, unicast["192.168.0.0/16"])
}

// TestExtractStaticRoutesLabeledUnicast verifies labeled unicast detection.
//
// VALIDATES: Routes with label only (no rd) go to .<afi>.nlri-mpls (SAFI 4).
// RFC 8277: Labeled unicast uses SAFI 4.
//
// PREVENTS: Labeled unicast routes being misclassified as L3VPN (SAFI 128).
func TestExtractStaticRoutesLabeledUnicast(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 172.16.0.0/12 label 100 next-hop self;
        route 2001:db8::/32 label 200 next-hop self;
        route 10.0.0.0/8 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	announce := neighbor.GetContainer("announce")

	// IPv4 labeled unicast (label only, no rd)
	ipv4 := announce.GetContainer("ipv4")
	nlriMpls := ipv4.GetList("nlri-mpls")
	require.Len(t, nlriMpls, 1)
	require.NotNil(t, nlriMpls["172.16.0.0/12"])

	// IPv4 unicast (no label, no rd)
	unicast := ipv4.GetList("unicast")
	require.Len(t, unicast, 1)
	require.NotNil(t, unicast["10.0.0.0/8"])

	// IPv6 labeled unicast
	ipv6 := announce.GetContainer("ipv6")
	ipv6NlriMpls := ipv6.GetList("nlri-mpls")
	require.Len(t, ipv6NlriMpls, 1)
	require.NotNil(t, ipv6NlriMpls["2001:db8::/32"])
}

// TestExtractStaticRoutesMergeExisting verifies merging with existing announce.
//
// VALIDATES: Static routes merge with existing announce block.
//
// PREVENTS: Existing announce routes being overwritten.
func TestExtractStaticRoutesMergeExisting(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 next-hop self;
    }
    announce {
        ipv4 {
            unicast 192.168.0.0/16 next-hop self;
        }
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	announce := neighbor.GetContainer("announce")
	ipv4 := announce.GetContainer("ipv4")
	unicast := ipv4.GetList("unicast")

	// Both routes should be present
	require.Len(t, unicast, 2)
	require.NotNil(t, unicast["10.0.0.0/8"])
	require.NotNil(t, unicast["192.168.0.0/16"])
}

// TestExtractStaticRoutesMultipleNeighbors verifies each neighbor keeps own routes.
//
// VALIDATES: Routes stay in their respective peer blocks.
//
// PREVENTS: Routes from one peer bleeding into another.
func TestExtractStaticRoutesMultipleNeighbors(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 next-hop self;
    }
}
neighbor 192.0.2.2 {
    local-as 65000;
    static {
        route 172.16.0.0/12 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbors := result.GetList("neighbor")
	require.Len(t, neighbors, 2)

	// First neighbor should only have 10.0.0.0/8
	n1 := neighbors["192.0.2.1"]
	a1 := n1.GetContainer("announce").GetContainer("ipv4").GetList("unicast")
	require.Len(t, a1, 1)
	require.NotNil(t, a1["10.0.0.0/8"])

	// Second neighbor should only have 172.16.0.0/12
	n2 := neighbors["192.0.2.2"]
	a2 := n2.GetContainer("announce").GetContainer("ipv4").GetList("unicast")
	require.Len(t, a2, 1)
	require.NotNil(t, a2["172.16.0.0/12"])
}

// TestExtractStaticRoutesPreservesAttributes verifies all route attributes are kept.
//
// VALIDATES: All route attributes (community, local-pref, etc.) are preserved.
//
// PREVENTS: Route attributes being lost during migration.
func TestExtractStaticRoutesPreservesAttributes(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 next-hop self local-preference 200 community 65000:100;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	route := neighbor.GetContainer("announce").GetContainer("ipv4").GetList("unicast")["10.0.0.0/8"]

	nh, ok := route.Get("next-hop")
	require.True(t, ok)
	require.Equal(t, "self", nh)

	lp, ok := route.Get("local-preference")
	require.True(t, ok)
	require.Equal(t, "200", lp)

	comm, ok := route.Get("community")
	require.True(t, ok)
	require.Equal(t, "65000:100", comm)
}

// TestExtractStaticRoutesNoStatic verifies no-op when static doesn't exist.
//
// VALIDATES: Neighbors without static blocks are unchanged.
//
// PREVENTS: Errors on configs without static blocks.
func TestExtractStaticRoutesNoStatic(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    peer-as 65001;
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, neighbor)

	// Should have no announce block
	announce := neighbor.GetContainer("announce")
	require.Nil(t, announce)

	// Should preserve other values
	localAs, ok := neighbor.Get("local-as")
	require.True(t, ok)
	require.Equal(t, "65000", localAs)
}

// TestExtractStaticRoutesNil verifies nil tree handling.
//
// VALIDATES: Nil tree returns error.
//
// PREVENTS: Nil pointer dereference.
func TestExtractStaticRoutesNil(t *testing.T) {
	result, err := ExtractStaticRoutes(nil)
	require.ErrorIs(t, err, ErrNilTree)
	require.Nil(t, result)
}

// TestExtractStaticRoutesPreservesOrder verifies route order is preserved.
//
// VALIDATES: Routes appear in same order as in static block.
//
// PREVENTS: Route order being scrambled.
func TestExtractStaticRoutesPreservesOrder(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    static {
        route 10.0.0.0/8 next-hop self;
        route 172.16.0.0/12 next-hop self;
        route 192.168.0.0/16 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	ipv4 := neighbor.GetContainer("announce").GetContainer("ipv4")
	unicast := ipv4.GetListOrdered("unicast")

	require.Len(t, unicast, 3)
	require.Equal(t, "10.0.0.0/8", unicast[0].Key)
	require.Equal(t, "172.16.0.0/12", unicast[1].Key)
	require.Equal(t, "192.168.0.0/16", unicast[2].Key)
}

// TestExtractStaticRoutesTemplateGroup verifies template.group static routes.
//
// VALIDATES: template.group.static routes become template.group.announce.
//
// PREVENTS: Template static routes being skipped.
func TestExtractStaticRoutesTemplateGroup(t *testing.T) {
	input := `
template {
    group vpn-customers {
        static {
            route 10.0.0.0/8 next-hop self;
            route 2001:db8::/32 next-hop self;
        }
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	tmpl := result.GetContainer("template")
	require.NotNil(t, tmpl)

	group := tmpl.GetList("group")["vpn-customers"]
	require.NotNil(t, group)

	// static should be gone
	require.Nil(t, group.GetContainer("static"))

	// announce should exist
	announce := group.GetContainer("announce")
	require.NotNil(t, announce)

	// IPv4
	ipv4 := announce.GetContainer("ipv4")
	require.NotNil(t, ipv4)
	require.Len(t, ipv4.GetList("unicast"), 1)

	// IPv6
	ipv6 := announce.GetContainer("ipv6")
	require.NotNil(t, ipv6)
	require.Len(t, ipv6.GetList("unicast"), 1)
}

// TestExtractStaticRoutesDoesNotMutateOriginal verifies original is unchanged.
//
// VALIDATES: ExtractStaticRoutes clones before modifying.
//
// PREVENTS: Original config corruption.
func TestExtractStaticRoutesDoesNotMutateOriginal(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
        route 10.0.0.0/8 next-hop self;
    }
}
`
	tree := parseWithBGPSchema(t, input)

	// Migrate
	_, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	// Original should still have static
	neighbor := tree.GetList("neighbor")["192.0.2.1"]
	static := neighbor.GetContainer("static")
	require.NotNil(t, static, "original should still have static block")
}

// TestExtractStaticRoutesEmptyStatic verifies empty static block handling.
//
// VALIDATES: Empty static block is removed, no announce created.
//
// PREVENTS: Empty announce blocks from empty static.
func TestExtractStaticRoutesEmptyStatic(t *testing.T) {
	input := `
neighbor 192.0.2.1 {
    local-as 65000;
    static {
    }
}
`
	tree := parseWithBGPSchema(t, input)

	result, err := ExtractStaticRoutes(tree)
	require.NoError(t, err)

	neighbor := result.GetList("neighbor")["192.0.2.1"]
	require.NotNil(t, neighbor)

	// static should be gone
	require.Nil(t, neighbor.GetContainer("static"))

	// announce should NOT exist (no routes to add)
	require.Nil(t, neighbor.GetContainer("announce"), "empty static should not create announce")
}
