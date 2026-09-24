// Design: docs/architecture/core-design.md -- IGP next-hop cost seam
// RFC: rfc/short/rfc7311.md -- Sections 3.4.3 and 4.2

// Package igpcost carries resolved next-hop distances from the unified RIB to
// route selection and advertisement without importing either component.
package igpcost

import (
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// Distance distinguishes an unknown route from a directly connected zero-cost
// route. Cost includes each recursive BGP route's received AIGP metric and the
// terminal IGP/static distance, never a BGP MED. MissingAIGP means a recursive
// BGP route omitted AIGP, so RFC 7311 Section 3.4.3 forbids carrying the attribute
// when changing next hop to self.
type Distance struct {
	Cost        uint64
	Resolved    bool
	MissingAIGP bool
}

// Func resolves the interior distance to addr.
type Func func(addr netip.Addr) Distance

var fnPtr atomic.Pointer[Func]

// Set registers the lookup. A nil function clears it.
func Set(fn Func) {
	if fn == nil {
		fnPtr.Store(nil)
		return
	}
	fnPtr.Store(&fn)
}

// Lookup uses the registered resolver, or the engine Loc-RIB when none is set.
func Lookup(addr netip.Addr) Distance {
	p := fnPtr.Load()
	if p == nil {
		return Resolve(locrib.Default(), addr)
	}
	return (*p)(addr)
}

// Add saturates rather than wrapping an accumulated distance.
func Add(a, b uint64) uint64 {
	if ^uint64(0)-a < b {
		return ^uint64(0)
	}
	return a + b
}

// Resolve follows received BGP AIGP and recursive static hops, then counts the
// terminal interior metric once. Missing reachability never yields a partial cost.
func Resolve(rib *locrib.RIB, addr netip.Addr) Distance {
	if rib == nil || !addr.IsValid() {
		return Distance{}
	}
	var distance Distance
	current := addr
	for range 8 {
		fam := family.IPv6Unicast
		if current.Is4() {
			fam = family.IPv4Unicast
		}
		path, _, found := rib.LPM(fam, current)
		if !found || path.RouteType.Discards() {
			return Distance{MissingAIGP: distance.MissingAIGP}
		}
		if !path.IsBGP && !path.MetricRecursive {
			distance.Cost = Add(distance.Cost, uint64(path.Metric))
			distance.Resolved = true
			return distance
		}
		if path.IsBGP {
			if !path.AIGPPresent {
				distance.MissingAIGP = true
			} else {
				distance.Cost = Add(distance.Cost, path.AIGP)
			}
		}
		if !path.NextHop.IsValid() || path.NextHop == current {
			return Distance{MissingAIGP: distance.MissingAIGP}
		}
		current = path.NextHop
	}
	return Distance{MissingAIGP: distance.MissingAIGP}
}
