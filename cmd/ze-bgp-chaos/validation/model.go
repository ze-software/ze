// Package validation implements the expected/actual route state model
// for verifying route reflector propagation correctness.
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

// Model tracks the expected route state for all peers.
//
// Core invariant: after convergence, established peer P should have received
// exactly the routes from all other established peers Q (where Q != P).
type Model struct {
	// peerCount is the number of peers in the scenario.
	peerCount int

	// established tracks which peers have active sessions.
	established []bool

	// announced tracks which routes each peer has announced.
	// announced[peerIndex] = set of prefixes peer has announced.
	announced []*PrefixSet
}

// NewModel creates a new validation model for n peers.
func NewModel(n int) *Model {
	announced := make([]*PrefixSet, n)
	for i := range n {
		announced[i] = NewPrefixSet()
	}
	return &Model{
		peerCount:   n,
		established: make([]bool, n),
		announced:   announced,
	}
}

// SetEstablished marks a peer as established or not.
// When a peer becomes established, its expected set is rebuilt.
func (m *Model) SetEstablished(peer int, established bool) {
	if peer < 0 || peer >= m.peerCount {
		return
	}
	m.established[peer] = established
}

// Announce records that a peer has announced a route.
// The route becomes expected at all other established peers.
func (m *Model) Announce(source int, prefix netip.Prefix) {
	if source < 0 || source >= m.peerCount {
		return
	}
	m.announced[source].Add(prefix)
}

// Withdraw records that a peer has withdrawn a route.
// The route is removed from all other peers' expected sets.
func (m *Model) Withdraw(source int, prefix netip.Prefix) {
	if source < 0 || source >= m.peerCount {
		return
	}
	m.announced[source].Remove(prefix)
}

// Disconnect marks a peer as disconnected and removes all its announced routes.
func (m *Model) Disconnect(peer int) {
	if peer < 0 || peer >= m.peerCount {
		return
	}
	m.established[peer] = false
	m.announced[peer] = NewPrefixSet()
}

// Expected computes the set of prefixes that peer should have received
// from the route reflector. This is the union of all routes announced by
// other established peers.
func (m *Model) Expected(peer int) *PrefixSet {
	result := NewPrefixSet()
	if peer < 0 || peer >= m.peerCount || !m.established[peer] {
		return result
	}
	for source := range m.peerCount {
		if source == peer {
			continue
		}
		if !m.established[source] {
			continue
		}
		for _, prefix := range m.announced[source].All() {
			result.Add(prefix)
		}
	}
	return result
}

// AnnouncedRoutes returns the set of routes a peer has announced.
func (m *Model) AnnouncedRoutes(peer int) *PrefixSet {
	if peer < 0 || peer >= m.peerCount {
		return NewPrefixSet()
	}
	return m.announced[peer]
}
