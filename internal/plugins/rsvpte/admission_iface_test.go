// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- multi-interface admission mapping
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

// VALIDATES: with a single configured interface, admission resolves to it
// regardless of the neighbor (no prefix needed) -- the original behavior.
func TestAdmissionIfaceSingle(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.1", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 8e9}}
	})
	iface, ok := e.admissionInterface(netip.MustParseAddr("203.0.113.7"))
	assert.True(t, ok)
	assert.Equal(t, "eth0", iface)
}

// VALIDATES: with several interfaces, the neighbor is mapped to the interface
// whose configured prefix contains it -- the multi-interface gap this fixes.
func TestAdmissionIfaceMultiMapsByPrefix(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.1", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", MaxBW: 10e9, MaxReservableBW: 8e9, Prefix: netip.MustParsePrefix("10.0.0.4/30")},
			{Name: "eth1", MaxBW: 10e9, MaxReservableBW: 8e9, Prefix: netip.MustParsePrefix("10.0.0.8/30")},
		}
	})

	iface, ok := e.admissionInterface(netip.MustParseAddr("10.0.0.5"))
	assert.True(t, ok)
	assert.Equal(t, "eth0", iface, "neighbor in eth0's /30")

	iface, ok = e.admissionInterface(netip.MustParseAddr("10.0.0.9"))
	assert.True(t, ok)
	assert.Equal(t, "eth1", iface, "neighbor in eth1's /30")
}

// VALIDATES: a neighbor outside every configured prefix is unresolvable, so
// admission is skipped rather than charged to the wrong link.
func TestAdmissionIfaceMultiNoMatch(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.1", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{
			{Name: "eth0", Prefix: netip.MustParsePrefix("10.0.0.4/30")},
			{Name: "eth1", Prefix: netip.MustParsePrefix("10.0.0.8/30")},
		}
	})
	_, ok := e.admissionInterface(netip.MustParseAddr("192.0.2.1"))
	assert.False(t, ok)
}

// VALIDATES: multiple interfaces without prefixes cannot be resolved (the
// operator must declare addresses) -- no silent mis-charging.
func TestAdmissionIfaceMultiNoPrefixes(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.1", func(c *rsvpteConfig) {
		c.Interfaces = []ifaceConfig{{Name: "eth0"}, {Name: "eth1"}}
	})
	_, ok := e.admissionInterface(netip.MustParseAddr("10.0.0.5"))
	assert.False(t, ok)
}
