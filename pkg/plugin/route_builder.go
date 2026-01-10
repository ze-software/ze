package plugin

import (
	"fmt"
	"net/netip"

	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/attribute"
	"codeberg.org/thomas-mangin/zebgp/pkg/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/pkg/rib"
)

// RouteBuilder constructs routes using attribute.Builder for wire-first encoding.
// This replaces the intermediate PathAttributes struct for building routes
// from user commands.
//
// Example usage:
//
//	rb := NewRouteBuilder()
//	rb.SetPrefix(netip.MustParsePrefix("10.0.0.0/24"))
//	rb.SetNextHop(netip.MustParseAddr("192.168.1.1"))
//	rb.SetFamily(nlri.IPv4Unicast)
//	rb.Attrs().SetOrigin(0).SetLocalPref(100)
//	route, err := rb.Build()
type RouteBuilder struct {
	prefix  netip.Prefix
	nextHop netip.Addr
	family  nlri.Family
	pathID  uint32
	attrs   *attribute.Builder
}

// NewRouteBuilder creates a new RouteBuilder.
func NewRouteBuilder() *RouteBuilder {
	return &RouteBuilder{
		attrs: attribute.NewBuilder(),
	}
}

// SetPrefix sets the route prefix.
func (rb *RouteBuilder) SetPrefix(prefix netip.Prefix) *RouteBuilder {
	rb.prefix = prefix
	return rb
}

// SetNextHop sets the next-hop address.
func (rb *RouteBuilder) SetNextHop(addr netip.Addr) *RouteBuilder {
	rb.nextHop = addr
	return rb
}

// SetFamily sets the address family.
func (rb *RouteBuilder) SetFamily(family nlri.Family) *RouteBuilder {
	rb.family = family
	return rb
}

// SetPathID sets the ADD-PATH path identifier.
func (rb *RouteBuilder) SetPathID(pathID uint32) *RouteBuilder {
	rb.pathID = pathID
	return rb
}

// Attrs returns the attribute builder for chained attribute setting.
func (rb *RouteBuilder) Attrs() *attribute.Builder {
	return rb.attrs
}

// Reset clears all state for builder reuse.
func (rb *RouteBuilder) Reset() {
	rb.prefix = netip.Prefix{}
	rb.nextHop = netip.Addr{}
	rb.family = nlri.Family{}
	rb.pathID = 0
	rb.attrs.Reset()
}

// Build creates a rib.Route from the accumulated state.
// Returns error if required fields (prefix, next-hop) are missing.
func (rb *RouteBuilder) Build() (*rib.Route, error) {
	// Validate required fields
	if !rb.prefix.IsValid() {
		return nil, fmt.Errorf("missing required prefix")
	}
	if !rb.nextHop.IsValid() {
		return nil, fmt.Errorf("missing required next-hop")
	}

	// Create NLRI based on family
	// For unicast/multicast, use INET; other SAFIs would need specific NLRI types
	family := rb.family
	if family.AFI == 0 {
		// Infer AFI from prefix (don't mutate builder state)
		if rb.prefix.Addr().Is4() {
			family = nlri.IPv4Unicast
		} else {
			family = nlri.IPv6Unicast
		}
	}

	var n nlri.NLRI
	switch family.SAFI {
	case nlri.SAFIUnicast, nlri.SAFIMulticast:
		n = nlri.NewINET(family, rb.prefix, rb.pathID)
	case nlri.SAFIMPLSLabel, nlri.SAFIEVPN, nlri.SAFIVPN, nlri.SAFIFlowSpec,
		nlri.SAFIFlowSpecVPN, nlri.SAFIMVPN, nlri.SAFIVPLS, nlri.SAFIMUP,
		nlri.SAFIRTC, nlri.SAFIBGPLinkState:
		// These SAFIs need specific NLRI types, not supported by RouteBuilder
		return nil, fmt.Errorf("SAFI %d not supported by RouteBuilder", family.SAFI)
	default:
		n = nlri.NewINET(family, rb.prefix, rb.pathID)
	}

	// Convert Builder to []attribute.Attribute
	attrs := rb.attrs.ToAttributes()

	// Get AS_PATH if set
	asPath := rb.attrs.ToASPath()

	// Create route
	return rib.NewRouteWithASPath(n, rb.nextHop, attrs, asPath), nil
}
