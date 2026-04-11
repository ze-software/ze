// Design: docs/architecture/plugin/rib-storage-design.md — best-path selection
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_attr_format.go — attribute formatting (asPathLength, firstASInPath shared concern)
// Related: rib_commands.go — extractCandidate, gatherCandidates
// Related: rib_pipeline_best.go — best-path pipeline for bgp rib show best commands
// Related: rib_bestchange.go — best-path change tracking and Bus publishing
//
// Best-path selection per RFC 4271 §9.1.2 Decision Process Phase 2.
// Pure functions operating on extracted Candidate values — no pool dependency.
package rib

import (
	"bytes"
	"fmt"
	"net"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/storage"
)

// BestStep identifies which stage of the RFC 4271 §9.1.2 decision process
// determined the result of a pairwise candidate comparison. Used by
// SelectBestExplain to narrate why one path beat another.
type BestStep uint8

// Decision steps. Numbering matches the comments in ComparePair below and is
// mostly the RFC 4271 Section 9.1.2.2 step order, with step 0 for the
// pre-RFC stale-level depreference that runs first in ze.
const (
	BestStepStale        BestStep = iota // 0 -- stale-level depreference (pre-RFC)
	BestStepLocalPref                    // 1 -- highest LOCAL_PREF
	BestStepASPathLen                    // 2 -- shortest AS_PATH
	BestStepOrigin                       // 3 -- lowest Origin (IGP < EGP < INCOMPLETE)
	BestStepMED                          // 4 -- lowest MED (same neighbor AS)
	BestStepEBGPOverIBGP                 // 5 -- prefer eBGP over iBGP
	BestStepIGPCost                      // 6 -- lowest IGP cost (deferred)
	BestStepRouterID                     // 7 -- lowest Router ID / ORIGINATOR_ID
	BestStepPeerAddr                     // 8 -- lowest peer address
	BestStepEqual                        // no step resolved -- candidates are byte-for-byte identical
)

// String returns a stable, human-readable name for a decision step.
func (s BestStep) String() string {
	switch s {
	case BestStepStale:
		return "stale-level"
	case BestStepLocalPref:
		return "local-preference"
	case BestStepASPathLen:
		return "as-path-length"
	case BestStepOrigin:
		return "origin"
	case BestStepMED:
		return "med"
	case BestStepEBGPOverIBGP:
		return "ebgp-over-ibgp"
	case BestStepIGPCost:
		return "igp-cost"
	case BestStepRouterID:
		return "router-id"
	case BestStepPeerAddr:
		return "peer-address"
	case BestStepEqual:
		return "equal"
	}
	return "unknown-step"
}

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

// BestPathExplanation captures the step-by-step decision trail of a
// SelectBestExplain call. Steps[i] describes the pairwise comparison between
// Candidates[i+1] and the "running best" that prevailed through step i. The
// final Winner is the last running best.
//
// Because SelectBest is a linear reduction (N-1 comparisons for N candidates),
// the explanation is likewise linear: there is no combinatorial blowup even
// for prefixes with dozens of candidates.
type BestPathExplanation struct {
	Candidates []*Candidate   // candidates in original (gatherCandidates) order
	Steps      []PairwiseStep // N-1 entries for N candidates
	Winner     *Candidate     // final running best after all steps
}

// PairwiseStep describes a single reduction step: the incumbent (running
// best), the challenger, which one won, and WHY (the decision step that
// resolved the comparison).
type PairwiseStep struct {
	IncumbentIdx  int      // index into Candidates
	ChallengerIdx int      // index into Candidates
	WinnerIdx     int      // either IncumbentIdx or ChallengerIdx
	Step          BestStep // decision step that resolved the comparison
	Reason        string   // short human-readable explanation, e.g. "200 > 100"
}

// SelectBestExplain runs the RFC 4271 §9.1.2 decision process and records a
// per-step narrative of how the winner emerged. This is the slow-path variant
// used by CLI "reason" queries; the hot-path best-path updates continue to
// use SelectBest which skips the bookkeeping.
//
// Returns nil if candidates is empty.
func SelectBestExplain(candidates []*Candidate) *BestPathExplanation {
	if len(candidates) == 0 {
		return nil
	}
	exp := &BestPathExplanation{
		Candidates: candidates,
		Steps:      make([]PairwiseStep, 0, max(0, len(candidates)-1)),
	}
	incumbentIdx := 0
	for i := 1; i < len(candidates); i++ {
		challenger := candidates[i]
		incumbent := candidates[incumbentIdx]
		cmp, step, reason := comparePairWithReason(challenger, incumbent)
		winnerIdx := incumbentIdx
		if cmp < 0 {
			winnerIdx = i
		}
		exp.Steps = append(exp.Steps, PairwiseStep{
			IncumbentIdx:  incumbentIdx,
			ChallengerIdx: i,
			WinnerIdx:     winnerIdx,
			Step:          step,
			Reason:        reason,
		})
		incumbentIdx = winnerIdx
	}
	exp.Winner = candidates[incumbentIdx]
	return exp
}

// ComparePair compares two candidates using RFC 4271 §9.1.2 Phase 2 steps,
// with stale-level depreference applied first.
// Returns -1 if a is better, 1 if b is better, 0 if equal (should not happen
// with peer address tiebreak, but returned for defensive correctness).
func ComparePair(a, b *Candidate) int {
	result, _, _ := comparePairWithReason(a, b)
	return result
}

// comparePairWithReason runs the full RFC 4271 §9.1.2 decision process and
// additionally reports the deciding step and a short textual reason. Used by
// SelectBestExplain; ComparePair is a thin wrapper that discards the narrative.
func comparePairWithReason(a, b *Candidate) (int, BestStep, string) {
	// Step 0: Stale-level depreference.
	// Routes at or above DepreferenceThreshold lose to routes below it.
	// Between two routes on the same side of the threshold, lower level wins.
	// Between two routes both below threshold, normal tiebreaking applies.
	aDepref := a.StaleLevel >= storage.DepreferenceThreshold
	bDepref := b.StaleLevel >= storage.DepreferenceThreshold
	if aDepref != bDepref {
		reason := fmt.Sprintf("stale-level %d vs %d (threshold %d)", a.StaleLevel, b.StaleLevel, storage.DepreferenceThreshold)
		if !aDepref {
			return -1, BestStepStale, reason
		}
		return 1, BestStepStale, reason
	}
	// Both deprioritized: lower stale level wins
	if aDepref && a.StaleLevel != b.StaleLevel {
		reason := fmt.Sprintf("stale-level %d vs %d", a.StaleLevel, b.StaleLevel)
		if a.StaleLevel < b.StaleLevel {
			return -1, BestStepStale, reason
		}
		return 1, BestStepStale, reason
	}

	// Step 1: Highest LOCAL_PREF wins.
	// RFC 4271 §9.1.2: "the route with the highest degree of preference MUST be selected"
	if a.LocalPref != b.LocalPref {
		reason := fmt.Sprintf("local-preference %d vs %d", a.LocalPref, b.LocalPref)
		if a.LocalPref > b.LocalPref {
			return -1, BestStepLocalPref, reason
		}
		return 1, BestStepLocalPref, reason
	}

	// Step 2: Shortest AS_PATH wins.
	// RFC 4271 §9.1.2.2(a): "prefer the route with the shorter AS_PATH"
	if a.ASPathLen != b.ASPathLen {
		reason := fmt.Sprintf("as-path-length %d vs %d", a.ASPathLen, b.ASPathLen)
		if a.ASPathLen < b.ASPathLen {
			return -1, BestStepASPathLen, reason
		}
		return 1, BestStepASPathLen, reason
	}

	// Step 3: Lowest ORIGIN wins (IGP < EGP < INCOMPLETE).
	// RFC 4271 §9.1.2.2(b): "prefer the route with the lowest Origin value"
	if a.Origin != b.Origin {
		reason := fmt.Sprintf("origin %d vs %d", a.Origin, b.Origin)
		if a.Origin < b.Origin {
			return -1, BestStepOrigin, reason
		}
		return 1, BestStepOrigin, reason
	}

	// Step 4: Lowest MED wins — only when same neighbor AS.
	// RFC 4271 §9.1.2.2(c): "prefer the route with the lower multi-exit discriminator"
	// "comparison is only performed between routes learned from the same neighboring AS"
	if a.FirstAS != 0 && b.FirstAS != 0 && a.FirstAS == b.FirstAS {
		if a.MED != b.MED {
			reason := fmt.Sprintf("med %d vs %d (same neighbor AS %d)", a.MED, b.MED, a.FirstAS)
			if a.MED < b.MED {
				return -1, BestStepMED, reason
			}
			return 1, BestStepMED, reason
		}
	}

	// Step 5: Prefer eBGP over iBGP.
	// RFC 4271 §9.1.2.2(d): "prefer externally learned routes"
	if a.LocalASN != 0 && b.LocalASN != 0 {
		aEBGP := a.PeerASN != a.LocalASN
		bEBGP := b.PeerASN != b.LocalASN
		if aEBGP != bEBGP {
			reason := fmt.Sprintf("ebgp-over-ibgp (%q vs %q)", ebgpLabel(aEBGP), ebgpLabel(bEBGP))
			if aEBGP {
				return -1, BestStepEBGPOverIBGP, reason
			}
			return 1, BestStepEBGPOverIBGP, reason
		}
	}

	// Step 6: Lowest IGP cost to NEXT_HOP — deferred (requires IGP integration).

	// Step 7: Lowest Router ID (use ORIGINATOR_ID when present, RFC 4456).
	// RFC 4271: BGP Identifier is a 32-bit unsigned integer — compare as IP bytes, not strings.
	if a.OriginatorID != "" && b.OriginatorID != "" {
		if cmp := compareAddrs(a.OriginatorID, b.OriginatorID); cmp != 0 {
			return cmp, BestStepRouterID, fmt.Sprintf("router-id %s vs %s", a.OriginatorID, b.OriginatorID)
		}
	}

	// Step 8: Lowest peer address (final tiebreak).
	// RFC 4271 §9.1.2.2(g): "prefer the route received from the peer with the lowest BGP Identifier"
	if a.PeerAddr != b.PeerAddr {
		return compareAddrs(a.PeerAddr, b.PeerAddr), BestStepPeerAddr,
			fmt.Sprintf("peer-address %s vs %s", a.PeerAddr, b.PeerAddr)
	}

	return 0, BestStepEqual, "identical candidates"
}

// Protocol-type labels used by both reason narration and best-path change
// events. Kept as package-level constants so there is a single source of
// truth for consumers that match JSON field values.
const (
	protocolTypeEBGP = "ebgp"
	protocolTypeIBGP = "ibgp"
)

// ebgpLabel returns the protocol-type label for the boolean. Kept small so
// the comparePairWithReason hot path stays inlineable.
func ebgpLabel(isEBGP bool) string {
	if isEBGP {
		return protocolTypeEBGP
	}
	return protocolTypeIBGP
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
