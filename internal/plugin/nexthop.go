package plugin

import "net/netip"

// NextHopPolicy specifies how next-hop is determined for a route.
type NextHopPolicy uint8

const (
	NextHopUnset    NextHopPolicy = iota // Zero value = invalid
	NextHopExplicit                      // Use configured IP address
	NextHopSelf                          // Use session's local address
)

// RouteNextHop encapsulates next-hop policy for route origination.
// Resolution happens at peer level where negotiated capabilities are known.
//
// RFC 4271 Section 5.1.3 - NEXT_HOP attribute.
// RFC 5549/8950 - Extended Next Hop Encoding (cross-family resolution).
type RouteNextHop struct {
	Policy NextHopPolicy
	Addr   netip.Addr // Valid only when Policy == NextHopExplicit
}

// NewNextHopExplicit creates a RouteNextHop with an explicit address.
// The addr can be invalid; callers should check IsValid() before use.
func NewNextHopExplicit(addr netip.Addr) RouteNextHop {
	return RouteNextHop{
		Policy: NextHopExplicit,
		Addr:   addr,
	}
}

// NewNextHopSelf creates a RouteNextHop that uses the session's local address.
// Resolution happens at peer level via resolveNextHop().
func NewNextHopSelf() RouteNextHop {
	return RouteNextHop{
		Policy: NextHopSelf,
	}
}

// IsSelf returns true if this next-hop uses the session's local address.
func (n RouteNextHop) IsSelf() bool {
	return n.Policy == NextHopSelf
}

// IsExplicit returns true if this next-hop uses an explicit address.
func (n RouteNextHop) IsExplicit() bool {
	return n.Policy == NextHopExplicit
}

// IsValid returns true if this RouteNextHop is usable.
// - Self is always valid (resolution happens at send time)
// - Explicit is valid only if Addr is valid
// - Unset is never valid.
func (n RouteNextHop) IsValid() bool {
	switch n.Policy {
	case NextHopSelf:
		return true
	case NextHopExplicit:
		return n.Addr.IsValid()
	case NextHopUnset:
		return false
	default:
		return false
	}
}

// String returns "self", the IP address, or "" for invalid.
func (n RouteNextHop) String() string {
	switch n.Policy {
	case NextHopSelf:
		return "self"
	case NextHopExplicit:
		if n.Addr.IsValid() {
			return n.Addr.String()
		}
		return ""
	case NextHopUnset:
		return ""
	default:
		return ""
	}
}
