// Design: plan/immediate/spec-fib-depth.md -- recursive next-hop resolution
// Related: sysrib.go -- sysRIB calls nhResolver after best-path selection

package sysrib

import (
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/igpcost"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

const maxRecursionDepth = 8

// ResolvedNH is the result of resolving a possibly-recursive next-hop.
type ResolvedNH struct {
	DirectNH  netip.Addr
	Interface string
	OnLink    bool
	Metric    uint32
	Resolved  bool
}

// nhResolver resolves recursive next-hops using the Loc-RIB's LPM. A next-hop
// is "recursive" when it is not directly connected: the covering prefix in
// the Loc-RIB itself has a next-hop that must be resolved, forming a chain.
// Resolution terminates when a connected route (zero next-hop) is found or
// maxRecursionDepth is reached.
type nhResolver struct {
	rib *locrib.RIB
	mu  sync.RWMutex

	// tracking maps next-hop addresses to the set of prefixes that depend on
	// them. When a next-hop's reachability or cost changes, all dependent
	// prefixes must be re-evaluated.
	tracking map[netip.Addr]map[netip.Prefix]struct{}
}

func newNHResolver(rib *locrib.RIB) *nhResolver {
	return &nhResolver{
		rib:      rib,
		tracking: make(map[netip.Addr]map[netip.Prefix]struct{}),
	}
}

// Resolve performs recursive next-hop resolution for addr. Returns the
// directly-connected next-hop and the accumulated IGP metric, or
// Resolved=false if the next-hop is unreachable.
func (r *nhResolver) Resolve(addr netip.Addr) ResolvedNH {
	if !addr.IsValid() {
		return ResolvedNH{}
	}

	var totalMetric uint32

	current := addr
	for range maxRecursionDepth {
		path, _, found := r.rib.LPM(familyForAddr(current), current)
		if !found {
			return ResolvedNH{}
		}
		if path.RouteType.Discards() {
			return ResolvedNH{}
		}
		totalMetric += path.Metric

		if !path.NextHop.IsValid() {
			// Connected route: current is the directly-reachable NH.
			return ResolvedNH{
				DirectNH: current, Interface: path.Interface,
				Metric: totalMetric, Resolved: true,
			}
		}
		if path.OnLink {
			return ResolvedNH{
				DirectNH: path.NextHop, Interface: path.Interface, OnLink: true,
				Metric: totalMetric, Resolved: true,
			}
		}

		if path.NextHop == current {
			// Self-referencing route prevents infinite loop.
			return ResolvedNH{}
		}

		current = path.NextHop
	}

	return ResolvedNH{}
}

// Track registers that prefix depends on nextHop for resolution. When
// nextHop changes (reachability or metric), the caller should re-evaluate
// prefix.
func (r *nhResolver) Track(nextHop netip.Addr, prefix netip.Prefix) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tracking[nextHop] == nil {
		r.tracking[nextHop] = make(map[netip.Prefix]struct{})
	}
	r.tracking[nextHop][prefix] = struct{}{}
}

// Untrack removes the dependency of prefix on nextHop.
func (r *nhResolver) Untrack(nextHop netip.Addr, prefix netip.Prefix) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if deps, ok := r.tracking[nextHop]; ok {
		delete(deps, prefix)
		if len(deps) == 0 {
			delete(r.tracking, nextHop)
		}
	}
}

// Dependents returns all prefixes that depend on the given next-hop.
func (r *nhResolver) Dependents(nextHop netip.Addr) []netip.Prefix {
	r.mu.RLock()
	defer r.mu.RUnlock()
	deps := r.tracking[nextHop]
	if len(deps) == 0 {
		return nil
	}
	result := make([]netip.Prefix, 0, len(deps))
	for pfx := range deps {
		result = append(result, pfx)
	}
	return result
}

// IGPMetric follows RFC 7311 Section 3.4.3's recursive BGP resolution. An IGP
// route's metric already spans its whole path; following its forwarding gateway
// and adding another metric would count links twice.
func (r *nhResolver) IGPMetric(addr netip.Addr) igpcost.Distance {
	return igpcost.Resolve(r.rib, addr)
}

// CoveredNHs returns tracked roots whose recursive chain intersects prefix.
// Tracking only the root address would miss a terminal covering-route change.
func (r *nhResolver) CoveredNHs(prefix netip.Prefix) []netip.Addr {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []netip.Addr
	for nh := range r.tracking {
		if r.chainCovered(nh, prefix) {
			result = append(result, nh)
		}
	}
	return result
}

func (r *nhResolver) chainCovered(current netip.Addr, prefix netip.Prefix) bool {
	for range maxRecursionDepth {
		if prefix.Contains(current) {
			return true
		}
		path, _, found := r.rib.LPM(familyForAddr(current), current)
		if !found || path.RouteType.Discards() || path.OnLink || !path.NextHop.IsValid() || path.NextHop == current {
			return false
		}
		current = path.NextHop
	}
	return false
}

func familyForAddr(addr netip.Addr) family.Family {
	if addr.Is4() {
		return family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}
	}
	return family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}
}

func familyForPrefix(p netip.Prefix) family.Family {
	return familyForAddr(p.Addr())
}
