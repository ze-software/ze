// VALIDATES: RFC 5340 §2.5 / §4.1.2 -- an OSPFv3 virtual link's IP interface address is one of
// this router's own GLOBAL-scope IPv6 addresses; a router that advertises only a link-local in
// the transit area resolves no virtual-link endpoint at all.
// PREVENTS: a virtual link falling back to a link-local source address, which the intermediate
// transit-area hops cannot route.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// RFC requirement: RFC5340-4.1.2-2 negative -- the virtual link's IP interface address is never
// a link-local address: when this router advertises only fe80::1 in the transit area (the
// neighbor advertising a perfectly good global), no interface address resolves and the virtual
// link stays unusable rather than adopting the link-local (v6RouterGlobalAddr IsGlobalUnicast
// filter, virtuallink_v6.go:69; v6ResolveVirtualEndpointLocked requires both ends,
// virtuallink_v6.go:32-34).
func TestRFC5340VirtualLinkRefusesLocalLinkLocalInterfaceAddress(t *testing.T) {
	e := newV6OriginEngine()
	transit := vlArea(t, "0.0.0.1")
	self := vlRID(t, "10.0.0.1")
	neighbor := vlRID(t, "10.0.0.2")
	e.cfg.RouterID = self
	installV6IntraPrefix(t, e.lsdb, transit, self, "fe80::1/128")
	installV6IntraPrefix(t, e.lsdb, transit, neighbor, "2001:db8:2::2/128")

	rt := &virtualLinkRuntime{cfg: virtualLinkConfig{TransitArea: transit, RemoteRouterID: neighbor}}
	src, _, ok := e.v6ResolveVirtualEndpointLocked(rt)
	assert.False(t, ok, "a virtual link must not resolve when this router has no global-scope IPv6 address")
	assert.False(t, src.IsValid(), "a link-local must never be adopted as the virtual link's interface address")
}
