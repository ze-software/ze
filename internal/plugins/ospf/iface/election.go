// Design: docs/architecture/ospf/ospf-5-interface-ism.md -- RFC 2328 DR/BDR election
// RFC 2328 Section 9.4: "The Backup Designated Router is calculated first. Then the Designated Router is calculated."

package iface

import (
	"net/netip"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

type Candidate struct {
	RouterID types.RouterID
	// Address is the candidate's reachable source address (IPv4 for OSPFv2, IPv6
	// link-local for OSPFv3). Election compares the [4]byte identity derived from
	// it (candidateAddress) against the declared DR/BDR.
	Address     netip.Addr
	Priority    uint8
	TwoWay      bool
	DeclaredDR  [4]byte
	DeclaredBDR [4]byte
	Self        bool
}

type ElectionResult struct {
	DR  types.RouterID
	BDR types.RouterID
}

func electDRBDR(candidates []Candidate) ElectionResult {
	eligible := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if c.Priority == 0 || (!c.Self && !c.TwoWay) {
			continue
		}
		eligible = append(eligible, c)
	}
	if len(eligible) == 0 {
		return ElectionResult{}
	}

	bdr := chooseBDR(eligible, types.RouterID{})
	dr := chooseDeclaredDR(eligible)
	if dr == (types.RouterID{}) {
		dr = bdr
		bdr = chooseBDR(eligible, dr)
	}
	return ElectionResult{DR: dr, BDR: bdr}
}

func chooseDeclaredDR(candidates []Candidate) types.RouterID {
	var best Candidate
	for _, c := range candidates {
		if c.DeclaredDR != candidateAddress(c) {
			continue
		}
		if best.RouterID == (types.RouterID{}) || betterCandidate(c, best) {
			best = c
		}
	}
	return best.RouterID
}

func chooseBDR(candidates []Candidate, exclude types.RouterID) types.RouterID {
	var bestDeclared Candidate
	for _, c := range candidates {
		if c.RouterID == exclude || c.DeclaredDR == candidateAddress(c) || c.DeclaredBDR != candidateAddress(c) {
			continue
		}
		if bestDeclared.RouterID == (types.RouterID{}) || betterCandidate(c, bestDeclared) {
			bestDeclared = c
		}
	}
	if bestDeclared.RouterID != (types.RouterID{}) {
		return bestDeclared.RouterID
	}
	var best Candidate
	for _, c := range candidates {
		if c.RouterID == exclude || c.DeclaredDR == candidateAddress(c) {
			continue
		}
		if best.RouterID == (types.RouterID{}) || betterCandidate(c, best) {
			best = c
		}
	}
	return best.RouterID
}

// candidateAddress is the [4]byte election identity: the IPv4 interface address
// for OSPFv2, or the Router ID for OSPFv3 (whose reachable address is an IPv6
// link-local, not IPv4, and which declares DR/BDR by Router ID -- RFC 5340 sec 4.2).
func candidateAddress(c Candidate) [4]byte {
	if c.Address.Is4() {
		return c.Address.As4()
	}
	return [4]byte(c.RouterID)
}

func betterCandidate(a, b Candidate) bool {
	if a.Priority != b.Priority {
		return a.Priority > b.Priority
	}
	return compareRouterID(a.RouterID, b.RouterID) > 0
}

func compareRouterID(a, b types.RouterID) int {
	for i := range a {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}
