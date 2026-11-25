// Design: docs/architecture/chaos-web-dashboard.md — property-based validation
//
// Package validation implements the expected/actual route state model
// for verifying route server propagation correctness.
package validation

import (
	"maps"
	"net/netip"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

type routeKey struct {
	family family.Family
	prefix netip.Prefix
}

func makeRouteKey(prefix netip.Prefix, name string) routeKey {
	if name != "" {
		return routeKey{family: lookupRouteFamily(name), prefix: prefix}
	}
	return routeKey{family: unicastFamily(prefix), prefix: prefix}
}

// Chaos can observe VPN and FlowSpec from an external target even when its
// own build did not register the corresponding plugins.
const (
	vpnFamilyName  = "mpls-vpn"
	flowFamilyName = "flow"
)

func lookupRouteFamily(name string) family.Family {
	if fam, ok := family.LookupFamily(name); ok {
		return fam
	}
	afiName, safiName, _ := strings.Cut(name, "/")
	afi, ok := family.LookupAFI(afiName)
	// refuse-by-default: guards makeRouteKey -- unrecognized family names stay keyed under the zero family so announced and received rows compare symmetrically
	if !ok {
		return family.Family{}
	}
	switch safiName {
	case vpnFamilyName:
		return family.Family{AFI: afi, SAFI: family.SAFIVPN}
	case flowFamilyName:
		return family.Family{AFI: afi, SAFI: family.SAFIFlowSpec}
	default:
		return family.Family{}
	}
}

func routeFamilyName(fam family.Family) string {
	var name string
	// The simulator uses these two names for optional families on every build.
	//exhaustive:ignore
	switch fam.SAFI {
	case 0:
		return ""
	case family.SAFIVPN:
		name = vpnFamilyName
	case family.SAFIFlowSpec:
		name = flowFamilyName
	default:
		return fam.String()
	}
	var b textbuf.Buffer
	return b.Str(fam.AFI.String()).Byte('/').Str(name).String()
}

func unicastFamily(prefix netip.Prefix) family.Family {
	if prefix.Addr().Is4() {
		return family.IPv4Unicast
	}
	if prefix.Addr().Is6() {
		return family.IPv6Unicast
	}
	return family.Family{}
}

func (r routeKey) label() string {
	if r.family == unicastFamily(r.prefix) {
		return r.prefix.String()
	}
	var b textbuf.Buffer
	return b.Str(routeFamilyName(r.family)).Byte(' ').Str(r.prefix.String()).String()
}

// PrefixSet stores routes distinguished by address family and prefix.
type PrefixSet struct {
	m map[routeKey]struct{}
}

// NewPrefixSet creates an empty prefix set.
func NewPrefixSet() *PrefixSet {
	return &PrefixSet{m: make(map[routeKey]struct{})}
}

// Add inserts a unicast prefix into the set.
func (s *PrefixSet) Add(p netip.Prefix) { s.m[makeRouteKey(p, "")] = struct{}{} }

func (s *PrefixSet) addAll(prefixes *PrefixSet) {
	maps.Copy(s.m, prefixes.m)
}

// Remove deletes a unicast prefix from the set.
func (s *PrefixSet) Remove(p netip.Prefix) { delete(s.m, makeRouteKey(p, "")) }

// Contains returns true if any family in the set contains the prefix.
func (s *PrefixSet) Contains(p netip.Prefix) bool {
	if _, ok := s.m[makeRouteKey(p, "")]; ok {
		return true
	}
	for route := range s.m {
		if route.prefix == p {
			return true
		}
	}
	return false
}

// Len returns the number of prefixes in the set.
func (s *PrefixSet) Len() int { return len(s.m) }

// All returns the unordered prefixes, one per route family and prefix pair.
func (s *PrefixSet) All() []netip.Prefix {
	result := make([]netip.Prefix, 0, len(s.m))
	for route := range s.m {
		result = append(result, route.prefix)
	}
	return result
}

// SortedStrings returns route descriptions sorted by prefix and family.
func (s *PrefixSet) SortedStrings() []string {
	routes := slices.Collect(maps.Keys(s.m))
	slices.SortFunc(routes, func(a, b routeKey) int {
		if c := a.prefix.Addr().Compare(b.prefix.Addr()); c != 0 {
			return c
		}
		if c := a.prefix.Bits() - b.prefix.Bits(); c != 0 {
			return c
		}
		if a.family.AFI != b.family.AFI {
			return int(a.family.AFI) - int(b.family.AFI)
		}
		return int(a.family.SAFI) - int(b.family.SAFI)
	})
	result := make([]string, len(routes))
	for i, route := range routes {
		result[i] = route.label()
	}
	return result
}

// Reachability reports whether the target exports a family from source to destination.
// A nil policy allows all pairs of distinct peers. An empty family denotes an
// announcement without family information. The policy must be safe for concurrent use.
type Reachability func(source, destination int, family string) bool

func (reachable Reachability) allows(source, destination int, family string) bool {
	return source != destination && (reachable == nil || reachable(source, destination, family))
}

// Model tracks the expected route state for all peers. After convergence, each
// established peer should have every route exported to it by another established peer.
type Model struct {
	// peerCount is the number of peers in the scenario.
	peerCount int

	// established tracks which peers have active sessions.
	established []bool

	// announced lists each source peer's prefixes by family.
	announced []map[family.Family]*PrefixSet

	reachable Reachability
}

// NewModel creates a validation model with full reachability between distinct peers.
func NewModel(n int) *Model {
	return NewModelWithReachability(n, nil)
}

// NewModelWithReachability creates a validation model for the target's export policy.
func NewModelWithReachability(n int, reachable Reachability) *Model {
	announced := make([]map[family.Family]*PrefixSet, n)
	for i := range n {
		announced[i] = make(map[family.Family]*PrefixSet)
	}
	return &Model{
		peerCount:   n,
		established: make([]bool, n),
		announced:   announced,
		reachable:   reachable,
	}
}

// SetEstablished records whether a peer's routes can be exchanged.
func (m *Model) SetEstablished(peer int, established bool) {
	if peer < 0 || peer >= m.peerCount {
		return
	}
	m.established[peer] = established
}

// Established reports whether the latest observation has a live session.
func (m *Model) Established(peer int) bool {
	return peer >= 0 && peer < m.peerCount && m.established[peer]
}

// Announce records a route the source sends to the target.
func (m *Model) Announce(source int, prefix netip.Prefix) {
	m.AnnounceFamily(source, prefix, "")
}

// AnnounceFamily records a route with its family so export eligibility can be checked.
func (m *Model) AnnounceFamily(source int, prefix netip.Prefix, name string) {
	m.announce(source, makeRouteKey(prefix, name))
}

func (m *Model) announce(source int, route routeKey) {
	if source < 0 || source >= m.peerCount {
		return
	}
	families := m.announced[source]
	prefixes := families[route.family]
	if prefixes == nil {
		prefixes = NewPrefixSet()
		families[route.family] = prefixes
	}
	prefixes.m[route] = struct{}{}
}

// Withdraw forgets a route sent by the source. Routes from other sources remain.
func (m *Model) Withdraw(source int, prefix netip.Prefix) {
	m.withdraw(source, makeRouteKey(prefix, ""))
}

func (m *Model) withdraw(source int, route routeKey) {
	if source < 0 || source >= m.peerCount {
		return
	}
	if prefixes := m.announced[source][route.family]; prefixes != nil {
		delete(prefixes.m, route)
	}
}

// Disconnect marks a peer as disconnected and removes all its announced routes.
func (m *Model) Disconnect(peer int) {
	if peer < 0 || peer >= m.peerCount {
		return
	}
	m.established[peer] = false
	clear(m.announced[peer])
}

// Expected returns the union of prefixes that established sources export to peer.
func (m *Model) Expected(peer int) *PrefixSet {
	result := NewPrefixSet()
	if peer < 0 || peer >= m.peerCount || !m.established[peer] {
		return result
	}
	for source, families := range m.announced {
		if !m.established[source] {
			continue
		}
		for fam, prefixes := range families {
			if m.reachable.allows(source, peer, routeFamilyName(fam)) {
				result.addAll(prefixes)
			}
		}
	}
	return result
}

// AnnouncedRoutes returns the set of routes a peer has announced.
func (m *Model) AnnouncedRoutes(peer int) *PrefixSet {
	result := NewPrefixSet()
	if peer < 0 || peer >= m.peerCount {
		return result
	}
	for _, prefixes := range m.announced[peer] {
		result.addAll(prefixes)
	}
	return result
}
