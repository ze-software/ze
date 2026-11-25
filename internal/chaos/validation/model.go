// Design: docs/architecture/chaos-web-dashboard.md — property-based validation
//
// Package validation implements the expected/actual route state model
// for verifying route server propagation correctness.
package validation

import (
	"net/netip"
	"slices"
)

// PrefixSet is a set of IPv4/IPv6 prefixes.
type PrefixSet struct {
	m map[netip.Prefix]struct{}
}

// NewPrefixSet creates an empty prefix set.
func NewPrefixSet() *PrefixSet {
	return &PrefixSet{m: make(map[netip.Prefix]struct{})}
}

// Add inserts a prefix into the set.
func (s *PrefixSet) Add(p netip.Prefix) { s.m[p] = struct{}{} }

func (s *PrefixSet) addAll(prefixes *PrefixSet) {
	for prefix := range prefixes.m {
		s.Add(prefix)
	}
}

// Remove deletes a prefix from the set.
func (s *PrefixSet) Remove(p netip.Prefix) { delete(s.m, p) }

// Contains returns true if the prefix is in the set.
func (s *PrefixSet) Contains(p netip.Prefix) bool {
	_, ok := s.m[p]
	return ok
}

// Len returns the number of prefixes in the set.
func (s *PrefixSet) Len() int { return len(s.m) }

// All returns all prefixes in the set (unordered).
func (s *PrefixSet) All() []netip.Prefix {
	result := make([]netip.Prefix, 0, len(s.m))
	for p := range s.m {
		result = append(result, p)
	}
	return result
}

// SortedStrings returns all prefixes as sorted string representations.
// Sorted by address first, then prefix length.
func (s *PrefixSet) SortedStrings() []string {
	prefixes := s.All()
	slices.SortFunc(prefixes, func(a, b netip.Prefix) int {
		if c := a.Addr().Compare(b.Addr()); c != 0 {
			return c
		}
		return a.Bits() - b.Bits()
	})
	result := make([]string, len(prefixes))
	for i, p := range prefixes {
		result[i] = p.String()
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
	announced []map[string]*PrefixSet

	reachable Reachability
}

// NewModel creates a validation model with full reachability between distinct peers.
func NewModel(n int) *Model {
	return NewModelWithReachability(n, nil)
}

// NewModelWithReachability creates a validation model for the target's export policy.
func NewModelWithReachability(n int, reachable Reachability) *Model {
	announced := make([]map[string]*PrefixSet, n)
	for i := range n {
		announced[i] = make(map[string]*PrefixSet)
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
func (m *Model) AnnounceFamily(source int, prefix netip.Prefix, family string) {
	if source < 0 || source >= m.peerCount {
		return
	}
	families := m.announced[source]
	prefixes := families[family]
	if prefixes == nil {
		prefixes = NewPrefixSet()
		families[family] = prefixes
	}
	prefixes.Add(prefix)
}

// Withdraw forgets a route sent by the source. Routes from other sources remain.
func (m *Model) Withdraw(source int, prefix netip.Prefix) {
	if source < 0 || source >= m.peerCount {
		return
	}
	for _, prefixes := range m.announced[source] {
		prefixes.Remove(prefix)
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
		for family, prefixes := range families {
			if m.reachable.allows(source, peer, family) {
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
