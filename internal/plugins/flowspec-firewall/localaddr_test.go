package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLocalAddrTracking(t *testing.T) {
	la := newLocalAddrs()

	la.add(netip.MustParseAddr("10.0.0.1"))
	la.add(netip.MustParseAddr("192.168.1.1"))

	assert.True(t, la.containsWithin(netip.MustParsePrefix("10.0.0.0/24")))
	assert.True(t, la.containsWithin(netip.MustParsePrefix("192.168.1.0/24")))
	assert.False(t, la.containsWithin(netip.MustParsePrefix("172.16.0.0/16")))

	la.remove(netip.MustParseAddr("10.0.0.1"))
	assert.False(t, la.containsWithin(netip.MustParsePrefix("10.0.0.0/24")))
	assert.True(t, la.containsWithin(netip.MustParsePrefix("192.168.1.0/24")))
}

func TestHookSelectionTransit(t *testing.T) {
	la := newLocalAddrs()
	la.add(netip.MustParseAddr("10.0.0.1"))

	// Destination 172.16.0.0/24 is not local
	assert.False(t, la.containsWithin(netip.MustParsePrefix("172.16.0.0/24")))
}

func TestHookSelectionLocal(t *testing.T) {
	la := newLocalAddrs()
	la.add(netip.MustParseAddr("10.0.0.1"))

	// Destination 10.0.0.0/24 contains local address 10.0.0.1
	assert.True(t, la.containsWithin(netip.MustParsePrefix("10.0.0.0/24")))

	// Exact /32 match
	assert.True(t, la.containsWithin(netip.MustParsePrefix("10.0.0.1/32")))
}

func TestHookSelectionNoDestination(t *testing.T) {
	la := newLocalAddrs()
	la.add(netip.MustParseAddr("10.0.0.1"))

	// Invalid prefix (no destination component) should not match
	assert.False(t, la.containsWithin(netip.Prefix{}))
}

func TestHookReassignmentOnAddrChange(t *testing.T) {
	la := newLocalAddrs()
	pfx := netip.MustParsePrefix("10.0.0.0/24")

	// Not local initially
	assert.False(t, la.containsWithin(pfx))

	// Add a local address within the prefix
	la.add(netip.MustParseAddr("10.0.0.5"))
	assert.True(t, la.containsWithin(pfx))

	// Remove it
	la.remove(netip.MustParseAddr("10.0.0.5"))
	assert.False(t, la.containsWithin(pfx))
}

func TestAddrPayloadHandlers(t *testing.T) {
	la := newLocalAddrs()

	la.handleAddrAdded(`{"name":"eth0","unit":0,"index":2,"address":"10.0.0.1","prefix-length":24,"family":"ipv4","managed":true}`)
	assert.True(t, la.containsWithin(netip.MustParsePrefix("10.0.0.0/24")))

	la.handleAddrRemoved(`{"name":"eth0","unit":0,"index":2,"address":"10.0.0.1","prefix-length":24,"family":"ipv4","managed":true}`)
	assert.False(t, la.containsWithin(netip.MustParsePrefix("10.0.0.0/24")))

	// Invalid payload is silently ignored
	la.handleAddrAdded(42)
	la.handleAddrAdded(`invalid json`)
}
