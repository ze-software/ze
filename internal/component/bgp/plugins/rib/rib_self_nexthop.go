// Design: docs/architecture/plugin/rib-storage-design.md -- best-path candidate eligibility
// RFC: rfc/short/rfc4271.md -- a route with this speaker as its next hop is not installed (Section 5.1.3)
// Related: rib.go -- peerMetadata.LocalAddress, where the address arrives
// Related: rib_commands.go -- gatherCandidatesLocked, the one caller of the test below
// Related: rib_bestchange.go -- entryNextHopAddr, which reads the address a route names
package rib

import (
	"net/netip"
	"slices"
)

// parseLocalAddr turns the local address an event states into a typed address.
// An empty or unparseable value yields the zero Addr, which names no address
// rather than a wrong one: isSelfNextHop then matches nothing for that session,
// which is what "this event did not say" must mean.
func parseLocalAddr(s string) netip.Addr {
	if s == "" {
		return netip.Addr{}
	}
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

// refreshSelfNextHopsLocked rebuilds the set of addresses this speaker answers
// to as a next hop, from the local end of every session peerMeta holds.
//
// Caller MUST hold r.peerMu for writing. Called from every site that writes or
// deletes a peerMeta entry, because those are exactly the moments the answer can
// change: a session coming up adds an address, a session going down removes the
// last user of one.
//
// The slice is published whole rather than mutated in place, so a reader holding
// the previous pointer keeps a consistent set for the duration of its scan.
// Cold path: once per session state change, never per route.
func (r *RIBManager) refreshSelfNextHopsLocked() {
	var addrs []netip.Addr
	for _, meta := range r.peerMeta {
		if meta == nil || !meta.LocalAddress.IsValid() {
			continue
		}
		a := meta.LocalAddress.Unmap()
		if !slices.Contains(addrs, a) {
			addrs = append(addrs, a)
		}
	}
	r.selfNextHops.Store(&addrs)
}

// isSelfNextHop reports whether addr is one of this speaker's own addresses.
//
// RFC 4271 Section 6.3 defines the address this asks about: "It MUST NOT be the
// IP address of the receiving speaker." A route naming one tells this speaker to
// forward through itself.
//
// An empty set answers false, and that is deliberate rather than a fail-open: the
// set is empty only while no session has reported its local end, and in that state
// this speaker has no address a peer could name. It is NOT the answer to a failed
// read, because there is no read to fail -- the set is derived from state this
// process already holds.
//
// Allocation-free: a linear scan over a slice that holds one entry per distinct
// session-local address.
// It takes the set the caller has already loaded rather than loading it, so a
// scan over many candidates judges them all against the same set
// (gatherCandidatesLocked, rib_commands.go).
func isSelfNextHop(self *[]netip.Addr, addr netip.Addr) bool {
	if self == nil || len(*self) == 0 || !addr.IsValid() {
		return false
	}
	return slices.Contains(*self, addr.Unmap())
}
