package rpki

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeVRP(prefix string, maxLen uint8, asn uint32) VRP {
	_, ipnet, _ := net.ParseCIDR(prefix)
	return VRP{Prefix: *ipnet, MaxLength: maxLen, ASN: asn}
}

// TestROACacheAddAndCount verifies basic add and count operations.
//
// VALIDATES: VRPs are stored and counted correctly per family.
// PREVENTS: VRPs being lost or miscounted.
func TestROACacheAddAndCount(t *testing.T) {
	c := NewROACache()

	c.Add(makeVRP("10.0.0.0/8", 24, 65001))
	c.Add(makeVRP("192.168.0.0/16", 24, 65002))
	c.Add(makeVRP("2001:db8::/32", 48, 65003))

	v4, v6 := c.Count()
	assert.Equal(t, 2, v4, "should have 2 IPv4 VRPs")
	assert.Equal(t, 1, v6, "should have 1 IPv6 VRP")
}

// TestROACacheDeduplicate verifies duplicate VRPs are not stored.
//
// VALIDATES: Same VRP added twice results in count=1.
// PREVENTS: Duplicate VRPs inflating counts.
func TestROACacheDeduplicate(t *testing.T) {
	c := NewROACache()

	c.Add(makeVRP("10.0.0.0/8", 24, 65001))
	c.Add(makeVRP("10.0.0.0/8", 24, 65001)) // duplicate

	v4, _ := c.Count()
	assert.Equal(t, 1, v4, "duplicate VRP should not be stored twice")
}

// TestROACacheRemove verifies VRP removal.
//
// VALIDATES: Remove deletes matching VRP.
// PREVENTS: Stale VRPs persisting after withdrawal.
func TestROACacheRemove(t *testing.T) {
	c := NewROACache()

	vrp := makeVRP("10.0.0.0/8", 24, 65001)
	c.Add(vrp)

	v4, _ := c.Count()
	assert.Equal(t, 1, v4)

	c.Remove(vrp)
	v4, _ = c.Count()
	assert.Equal(t, 0, v4)
}

// TestROACacheFindCoveringExact verifies exact prefix match.
//
// VALIDATES: FindCovering returns VRPs for exact prefix match.
// PREVENTS: Exact matches being missed.
func TestROACacheFindCoveringExact(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	entries := c.FindCovering("10.0.0.0/8")
	require.Len(t, entries, 1)
	assert.Equal(t, uint32(65001), entries[0].ASN)
}

// TestROACacheFindCoveringLonger verifies covering prefix lookup for longer prefix.
//
// VALIDATES: VRP for /8 covers a /24 within its address space.
// PREVENTS: Covering-prefix lookup missing shorter VRPs.
func TestROACacheFindCoveringLonger(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	entries := c.FindCovering("10.1.2.0/24")
	require.Len(t, entries, 1)
	assert.Equal(t, uint32(65001), entries[0].ASN)
}

// TestROACacheFindCoveringNoMatch verifies non-matching prefix returns empty.
//
// VALIDATES: Non-covered prefix returns no VRPs.
// PREVENTS: False positives in covering lookup.
func TestROACacheFindCoveringNoMatch(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))

	entries := c.FindCovering("192.168.0.0/24")
	assert.Empty(t, entries)
}

// TestROACacheFindCoveringMultiple verifies multiple covering VRPs returned.
//
// VALIDATES: All covering VRPs returned when multiple exist.
// PREVENTS: Only first/shortest VRP being returned.
func TestROACacheFindCoveringMultiple(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))
	c.Add(makeVRP("10.0.0.0/16", 24, 65002))

	entries := c.FindCovering("10.0.1.0/24")
	assert.Len(t, entries, 2, "both /8 and /16 VRPs should cover /24")
}

// TestROACacheFindCoveringIPv6 verifies IPv6 covering prefix lookup.
//
// VALIDATES: IPv6 covering lookup works correctly.
// PREVENTS: IPv6 address handling bugs.
func TestROACacheFindCoveringIPv6(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("2001:db8::/32", 48, 65003))

	entries := c.FindCovering("2001:db8:1::/48")
	require.Len(t, entries, 1)
	assert.Equal(t, uint32(65003), entries[0].ASN)
}

// TestROACacheClear verifies clear removes all entries.
//
// VALIDATES: Clear empties both IPv4 and IPv6 tables.
// PREVENTS: Stale VRPs after cache clear.
func TestROACacheClear(t *testing.T) {
	c := NewROACache()
	c.Add(makeVRP("10.0.0.0/8", 24, 65001))
	c.Add(makeVRP("2001:db8::/32", 48, 65003))

	c.Clear()
	v4, v6 := c.Count()
	assert.Equal(t, 0, v4)
	assert.Equal(t, 0, v6)
}
