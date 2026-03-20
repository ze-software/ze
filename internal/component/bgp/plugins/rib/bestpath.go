// Design: docs/architecture/plugin/rib-storage-design.md — best-path selection
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_attr_format.go — attribute formatting (asPathLength, firstASInPath shared concern)
// Related: rib_commands.go — extractCandidate, gatherCandidates
// Related: rib_pipeline_best.go — best-path pipeline for rib best commands
//
// Best-path selection per RFC 4271 §9.1.2 Decision Process Phase 2.
// Pure functions operating on extracted Candidate values — no pool dependency.
package rib

import (
	"bytes"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

// ORIGIN values per RFC 4271 §4.3.
const (
	OriginIGP        byte = 0
	OriginEGP        byte = 1
	OriginIncomplete byte = 2
)

// Candidate holds extracted attribute values for best-path comparison.
// Built from pool handles by the caller -- this struct has no pool dependency.
type Candidate struct {
	PeerAddr     string // peer IP address (tiebreak step 8)
	PeerASN      uint32 // peer's AS number
	LocalASN     uint32 // local AS number (0 = unknown)
	LocalPref    uint32 // LOCAL_PREF value (default 100 if absent)
	ASPathLen    int    // AS_PATH length (AS_SET counts as 1)
	FirstAS      uint32 // first AS in path (for MED neighbor comparison)
	Origin       byte   // ORIGIN: 0=IGP, 1=EGP, 2=INCOMPLETE
	MED          uint32 // MED value (default 0 if absent)
	OriginatorID string // ORIGINATOR_ID as IP string (RFC 4456, Router ID tiebreak)
	StaleLevel   uint8  // Route staleness level (0=fresh; plugin-defined higher levels)
}

// SelectBest selects the best route from a list of candidates.
// Returns nil if the list is empty.
// RFC 4271 §9.1.2: pairwise comparison through all decision steps.
func SelectBest(candidates []*Candidate) *Candidate {
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if ComparePair(c, best) < 0 {
			best = c
		}
	}
	return best
}

// ComparePair compares two candidates using RFC 4271 §9.1.2 Phase 2 steps,
// with stale-level depreference applied first.
// Returns -1 if a is better, 1 if b is better, 0 if equal (should not happen
// with peer address tiebreak, but returned for defensive correctness).
func ComparePair(a, b *Candidate) int {
	// Step 0: Stale-level depreference.
	// Routes at or above DepreferenceThreshold lose to routes below it.
	// Between two routes on the same side of the threshold, lower level wins.
	// Between two routes both below threshold, normal tiebreaking applies.
	aDepref := a.StaleLevel >= storage.DepreferenceThreshold
	bDepref := b.StaleLevel >= storage.DepreferenceThreshold
	if aDepref != bDepref {
		if !aDepref {
			return -1 // a is normal, b is deprioritized: a wins
		}
		return 1 // a is deprioritized, b is normal: b wins
	}
	// Both deprioritized: lower stale level wins
	if aDepref && a.StaleLevel != b.StaleLevel {
		if a.StaleLevel < b.StaleLevel {
			return -1
		}
		return 1
	}

	// Step 1: Highest LOCAL_PREF wins.
	// RFC 4271 §9.1.2: "the route with the highest degree of preference MUST be selected"
	if a.LocalPref != b.LocalPref {
		if a.LocalPref > b.LocalPref {
			return -1
		}
		return 1
	}

	// Step 2: Shortest AS_PATH wins.
	// RFC 4271 §9.1.2.2(a): "prefer the route with the shorter AS_PATH"
	if a.ASPathLen != b.ASPathLen {
		if a.ASPathLen < b.ASPathLen {
			return -1
		}
		return 1
	}

	// Step 3: Lowest ORIGIN wins (IGP < EGP < INCOMPLETE).
	// RFC 4271 §9.1.2.2(b): "prefer the route with the lowest Origin value"
	if a.Origin != b.Origin {
		if a.Origin < b.Origin {
			return -1
		}
		return 1
	}

	// Step 4: Lowest MED wins — only when same neighbor AS.
	// RFC 4271 §9.1.2.2(c): "prefer the route with the lower multi-exit discriminator"
	// "comparison is only performed between routes learned from the same neighboring AS"
	if a.FirstAS != 0 && b.FirstAS != 0 && a.FirstAS == b.FirstAS {
		if a.MED != b.MED {
			if a.MED < b.MED {
				return -1
			}
			return 1
		}
	}

	// Step 5: Prefer eBGP over iBGP.
	// RFC 4271 §9.1.2.2(d): "prefer externally learned routes"
	if a.LocalASN != 0 && b.LocalASN != 0 {
		aEBGP := a.PeerASN != a.LocalASN
		bEBGP := b.PeerASN != b.LocalASN
		if aEBGP != bEBGP {
			if aEBGP {
				return -1
			}
			return 1
		}
	}

	// Step 6: Lowest IGP cost to NEXT_HOP — deferred (requires IGP integration).

	// Step 7: Lowest Router ID (use ORIGINATOR_ID when present, RFC 4456).
	// RFC 4271: BGP Identifier is a 32-bit unsigned integer — compare as IP bytes, not strings.
	if a.OriginatorID != "" && b.OriginatorID != "" {
		if cmp := compareAddrs(a.OriginatorID, b.OriginatorID); cmp != 0 {
			return cmp
		}
	}

	// Step 8: Lowest peer address (final tiebreak).
	// RFC 4271 §9.1.2.2(g): "prefer the route received from the peer with the lowest BGP Identifier"
	if a.PeerAddr != b.PeerAddr {
		return compareAddrs(a.PeerAddr, b.PeerAddr)
	}

	return 0
}

// compareAddrs compares two IP address strings numerically.
// RFC 4271: BGP Identifier and peer address are 32-bit unsigned integers.
// String comparison gives wrong results across digit-count boundaries
// (e.g., "9.0.0.1" vs "10.0.0.1"). Falls back to string comparison if parsing fails.
func compareAddrs(a, b string) int {
	ipA := net.ParseIP(a)
	ipB := net.ParseIP(b)
	if ipA == nil || ipB == nil {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	return bytes.Compare(ipA.To16(), ipB.To16())
}

// asPathLength counts the number of ASes in an AS_PATH attribute value.
// RFC 4271 §9.1.2.2(a): AS_SET counts as 1 regardless of how many ASes it contains.
// Assumes 4-byte ASNs (ASN4 capability negotiated).
func asPathLength(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	length := 0
	offset := 0
	for offset+2 <= len(data) {
		segType := data[offset]
		count := int(data[offset+1])
		offset += 2
		if segType == 1 {
			// AS_SET: entire set counts as 1.
			length++
		} else {
			// AS_SEQUENCE (type 2) or other: each AS counts.
			length += count
		}
		offset += count * 4 // skip AS values (4 bytes each)
	}
	return length
}

// firstASInPath extracts the first AS number from an AS_PATH attribute value.
// Used for MED comparison: MED is only compared between routes from the same neighbor AS.
// Returns 0 if the path is empty or truncated.
func firstASInPath(data []byte) uint32 {
	// Minimum: type(1) + count(1) + one 4-byte ASN = 6 bytes.
	if len(data) < 6 {
		return 0
	}
	count := data[1]
	if count == 0 {
		return 0
	}
	return uint32(data[2])<<24 | uint32(data[3])<<16 | uint32(data[4])<<8 | uint32(data[5])
}
