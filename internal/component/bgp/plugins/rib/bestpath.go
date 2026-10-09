// Design: docs/architecture/plugin/rib-storage-design.md — best-path selection
// Overview: rib.go — RIB plugin core types and event handlers
// Related: rib_attr_format.go — attribute formatting (asPathLength, firstASInPath shared concern)
// Related: rib_commands.go — extractCandidate, gatherCandidatesLocked
// Related: rib_pipeline_best.go — best-path pipeline for show bgp rib best commands
// RFC: rfc/short/rfc4271.md -- Section 9.1.2.2 decision process
// Related: rib_bestchange.go — best-path change tracking and Bus publishing
//
// Best-path selection per RFC 4271 §9.1.2 Decision Process Phase 2.
// Comparisons use extracted values; gathered candidates retain the pool snapshot
// that supplies AS_PATH identity and the eventual winner's metadata.
package rib

import (
	"cmp"
	"fmt"
	"math"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/rib/igpcost"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/textbuf"
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
	BestStepAIGP                         // received AIGP plus distance to next hop
	BestStepASPathLen                    // 2 -- shortest AS_PATH
	BestStepOrigin                       // 3 -- lowest Origin (IGP < EGP < INCOMPLETE)
	BestStepMED                          // 4 -- lowest MED (same neighbor AS)
	BestStepEBGPOverIBGP                 // 5 -- prefer eBGP over iBGP
	BestStepIGPCost                      // lowest resolved interior distance
	BestStepRouterID                     // 7 -- lowest Router ID / ORIGINATOR_ID
	BestStepClusterList                  // 8 -- shortest CLUSTER_LIST (RFC 4456 Section 9)
	BestStepPeerAddr                     // 9 -- lowest peer address
	BestStepPathID                       // 10 -- lowest path identifier (Ze tie-break, not RFC text)
	BestStepEqual                        // no step resolved -- candidates are byte-for-byte identical
)

// String returns a stable, human-readable name for a decision step.
func (s BestStep) String() string {
	switch s {
	case BestStepStale:
		return "stale-level"
	case BestStepLocalPref:
		return "local-preference"
	case BestStepAIGP:
		return "aigp"
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
	case BestStepClusterList:
		return "cluster-list-length"
	case BestStepPeerAddr:
		return "peer-address"
	case BestStepPathID:
		return "path-id"
	case BestStepEqual:
		return "equal"
	default:
		panic("BUG: invalid best-path decision step")
	}
}

// ORIGIN value aliases for use within the rib package.
// These are typed attribute.Origin values, not raw bytes.
const (
	OriginIGP        = attribute.OriginIGP
	OriginEGP        = attribute.OriginEGP
	OriginIncomplete = attribute.OriginIncomplete
)

// Candidate holds extracted attribute values for best-path comparison.
// Gathered candidates also own their immutable entry and label snapshot until
// releaseCandidates; directly extracted candidates only borrow ASPathHandle.
type Candidate struct {
	PeerAddr           string           // peer IP address string (map keys, JSON, internPeer)
	PeerIP             netip.Addr       // parsed peer address (zero-alloc comparison)
	PeerASN            uint32           // peer's AS number
	LocalASN           uint32           // local AS number (0 = unknown)
	LocalPref          uint32           // LOCAL_PREF value (default 100 if absent)
	ASPathLen          int              // AS_PATH length (AS_SET counts as 1)
	FirstAS            uint32           // first AS in path (for MED neighbor comparison)
	Origin             attribute.Origin // ORIGIN: 0=IGP, 1=EGP, 2=INCOMPLETE
	MED                uint32           // MED value (default 0 if absent)
	IGPCost            uint64           // resolved interior distance; max when unavailable
	AIGP               uint64           // first received AIGP metric (not accumulated)
	HasAIGP            bool             // metric presence is distinct from metric zero
	OriginatorIP       netip.Addr       // ORIGINATOR_ID or Router ID (RFC 4456, zero-alloc comparison)
	ClusterListEntries uint16           // CLUSTER_ID count in the CLUSTER_LIST (RFC 4456 Section 9; 0 when the attribute is absent)
	StaleLevel         uint8            // Route staleness level (0=fresh; plugin-defined higher levels)
	ASPathHandle       attrpool.Handle  // AS_PATH pool handle (for content-equal multipath comparison)
	// PathID is the RFC 7911 Path Identifier the path was received under, and
	// AddPath says whether the peer's family is stored with ADD-PATH. Together
	// with the prefix they name the winning path. Its attributes and label
	// side-data come from the retained snapshot below, not another storage read.
	// PathID is zero and AddPath false without ADD-PATH.
	PathID  uint32
	AddPath bool
	// Route is the wire NLRI a non-CIDR path was received with, without its
	// path identifier (storage.PrefixPath.Route). Its route key drops the
	// labels, so the winner's labels are read from here. Empty for CIDR.
	Route string

	// Ownership transfers here from a retained storage.PrefixPath during gather.
	// Other Candidate constructors carry extracted comparison values only.
	entry       storage.RouteEntry
	labelHandle attrpool.Handle
}

// SelectBest selects the best route, or nil when the list is empty.
// The caller MUST own the slice as scratch: selection can reorder its pointers,
// but retains every pointer and does not modify Candidate values.
// RFC 4271 Section 9.1.2.2: "The criteria MUST be applied in the order specified."
// Selection preserves the RFC criteria order.
func SelectBest(candidates []*Candidate) *Candidate {
	best, _ := selectBestCandidates(candidates, nil)
	return best
}

// SelectMultipath extends SelectBest with post-selection equal-cost multipath
// (RFC 4271 §9.1.2 Decision Process Phase 2 extension): after the primary
// best path is chosen, another candidate that survived whole-set MED elimination
// and ties through the non-tiebreaker steps can join the multipath set up to the
// configured maximum.
//
// The returned primary is the exact same Candidate that SelectBest would
// return -- multipath siblings are the OTHER equal-cost paths. FIB/ECMP
// consumers should program `append([]*Candidate{primary}, siblings...)` as
// the full path set.
//
// Parameters:
//   - candidates:   full candidate set (as produced by gatherCandidatesLocked).
//   - maxPaths:     configured maximum-paths (from bgp/multipath); 0 and 1
//     both mean "single best, no multipath" and the siblings slice is nil.
//   - relaxASPath:  when false, a sibling must have byte-identical AS_PATH
//     to the primary (checked via the attrpool handle). When true, matching
//     AS_PATH length is sufficient -- the Cisco "as-path multipath-relax"
//     behavior.
//
// Returns nil primary if candidates is empty. The caller MUST own the slice as
// scratch, with the same pointer-retention contract as SelectBest.
func SelectMultipath(candidates []*Candidate, maxPaths uint32, relaxASPath bool) (primary *Candidate, siblings []*Candidate) {
	// RFC 4271 Section 9.1.2.2: only whole-set MED survivors can be siblings.
	primary, eligible := selectBestCandidates(candidates, nil)
	if primary == nil || maxPaths <= 1 {
		return primary, nil
	}
	// Siblings slice capped at maxPaths-1 since the primary counts as slot 0.
	// Cast down from uint32: multipath maximum-paths is YANG-bounded to 256.
	capacity := min(int(maxPaths)-1, eligible-1)
	if capacity <= 0 {
		return primary, nil
	}
	siblings = make([]*Candidate, 0, capacity)
	for _, c := range candidates[:eligible] {
		if c == primary {
			continue
		}
		if multipathEqual(primary, c, relaxASPath) {
			siblings = append(siblings, c)
			if len(siblings) == capacity {
				break
			}
		}
	}
	if len(siblings) == 0 {
		return primary, nil
	}
	return primary, siblings
}

// multipathEqual reports whether a and b tie through all "non-tiebreaker"
// best-path steps: LOCAL_PREF, AIGP, AS_PATH, Origin, MED, eBGP/iBGP,
// and interior distance. Router ID, CLUSTER_LIST length and peer address
// distinguish paths the earlier steps already found equal-cost.
//
// relaxASPath == false requires byte-identical AS_PATH. Because the attrpool
// deduplicates identical byte sequences to the same handle, two candidates
// with byte-equal paths have ASPathHandle == ASPathHandle and the check is
// O(1). When either handle is invalid (AS_PATH attribute absent from the
// route entry) the comparison falls back to length-only.
func multipathEqual(a, b *Candidate, relaxASPath bool) bool {
	// Step 1: LOCAL_PREF.
	if a.LocalPref != b.LocalPref {
		return false
	}
	if compareAIGP(a, b) != 0 || a.IGPCost != b.IGPCost {
		return false
	}
	// Step 2: AS_PATH length (always) and content (unless relaxed).
	if a.ASPathLen != b.ASPathLen {
		return false
	}
	if !relaxASPath && a.ASPathHandle.IsValid() && b.ASPathHandle.IsValid() {
		if a.ASPathHandle != b.ASPathHandle {
			return false
		}
	}
	// Step 3: Origin.
	if a.Origin != b.Origin {
		return false
	}
	// Step 4: MED, only when both routes share a neighbor AS.
	if sameNeighborAS(a, b) && a.MED != b.MED {
		return false
	}
	// Step 5: eBGP vs iBGP.
	if a.LocalASN != 0 && b.LocalASN != 0 {
		aEBGP := a.PeerASN != a.LocalASN
		bEBGP := b.PeerASN != b.LocalASN
		if aEBGP != bEBGP {
			return false
		}
	}
	return true
}

// bestPathExplanation records each eliminated route against the candidate that
// removed it. A MED witness can later lose to a route from a different AS.
// Candidates and all step indices retain the caller's original order.
type bestPathExplanation struct {
	Candidates []*Candidate       // candidates in original input order
	Steps      []PairwiseStep     // one removal per non-winning candidate
	Winner     *Candidate         // final whole-set winner
	indices    map[*Candidate]int // original positions, explanation only
}

// PairwiseStep describes an elimination witness. Incumbent and challenger are
// ordered by their original input indices, not by a running-best tournament.
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
func SelectBestExplain(candidates []*Candidate) *bestPathExplanation {
	if len(candidates) == 0 {
		return nil
	}
	exp := &bestPathExplanation{
		Candidates: candidates,
		Steps:      make([]PairwiseStep, 0, len(candidates)-1),
		indices:    make(map[*Candidate]int, len(candidates)),
	}
	for i, candidate := range candidates {
		exp.indices[candidate] = i
	}
	// RFC 4271 Section 9.1.2.2: use the same elimination as the hot path,
	// with private scratch so the recorded indices remain the original ones.
	exp.Winner, _ = selectBestCandidates(slices.Clone(candidates), exp)
	return exp
}

// ComparePair compares two candidates using RFC 4271 §9.1.2 Phase 2 steps,
// with stale-level depreference applied first.
// Returns -1 if a is better, 1 if b is better, 0 if equal (should not happen
// with peer address tiebreak, but returned for defensive correctness).
func ComparePair(a, b *Candidate) int {
	result, _ := comparePair(a, b)
	return result
}

// compareBeforeMED compares the globally ordered criteria before conditional MED.
// RFC 4271 Section 9.1.2.2: "The criteria MUST be applied in the order specified."
// Earlier criteria eliminate candidates before conditional MED.
func compareBeforeMED(a, b *Candidate) (int, BestStep) {
	// Step 0: Stale-level depreference.
	aDepref := a.StaleLevel >= storage.DepreferenceThreshold
	bDepref := b.StaleLevel >= storage.DepreferenceThreshold
	if aDepref != bDepref {
		if !aDepref {
			return -1, BestStepStale
		}
		return 1, BestStepStale
	}
	if aDepref && a.StaleLevel != b.StaleLevel {
		if a.StaleLevel < b.StaleLevel {
			return -1, BestStepStale
		}
		return 1, BestStepStale
	}

	// Step 1: Highest LOCAL_PREF wins.
	if a.LocalPref != b.LocalPref {
		if a.LocalPref > b.LocalPref {
			return -1, BestStepLocalPref
		}
		return 1, BestStepLocalPref
	}

	// RFC 7311 Section 4.1: apply AIGP after LOCAL_PREF and before AS_PATH.
	if cmp := compareAIGP(a, b); cmp != 0 {
		return cmp, BestStepAIGP
	}

	// Step 2: Shortest AS_PATH wins.
	if a.ASPathLen != b.ASPathLen {
		if a.ASPathLen < b.ASPathLen {
			return -1, BestStepASPathLen
		}
		return 1, BestStepASPathLen
	}

	// Step 3: Lowest ORIGIN wins (IGP < EGP < INCOMPLETE).
	if a.Origin != b.Origin {
		if a.Origin < b.Origin {
			return -1, BestStepOrigin
		}
		return 1, BestStepOrigin
	}

	return 0, BestStepEqual
}

// comparePair compares one pair, not a candidate set: conditional MED is not
// transitive across neighboring ASes. Set selection first removes MED losers.
// KEEP IN SYNC with comparePairWithReason below (same steps, adds reason strings).
func comparePair(a, b *Candidate) (int, BestStep) {
	// RFC 4271 Section 9.1.2.2: earlier criteria precede MED.
	if result, step := compareBeforeMED(a, b); result != 0 {
		return result, step
	}

	// Step 4: Lowest MED wins — only when same neighbor AS.
	// RFC 4271 Section 9.1.2.2 (c): "For IBGP-learned routes, the MULTI_EXIT_DISC
	// MUST be used in route comparisons that reach this step in the Decision Process."
	if sameNeighborAS(a, b) {
		if a.MED != b.MED {
			if a.MED < b.MED {
				return -1, BestStepMED
			}
			return 1, BestStepMED
		}
	}
	return compareAfterMED(a, b)
}

// compareAfterMED compares candidates that survived conditional MED.
// RFC 4271 Section 9.1.2.2(d): "If at least one of the candidate routes was
// received via EBGP, remove from consideration all routes that were received
// via IBGP."
// Later criteria compare only the surviving candidates.
func compareAfterMED(a, b *Candidate) (int, BestStep) {

	// Step 5: Prefer eBGP over iBGP.
	if a.LocalASN != 0 && b.LocalASN != 0 {
		aEBGP := a.PeerASN != a.LocalASN
		bEBGP := b.PeerASN != b.LocalASN
		if aEBGP != bEBGP {
			if aEBGP {
				return -1, BestStepEBGPOverIBGP
			}
			return 1, BestStepEBGPOverIBGP
		}
	}

	// RFC 4271 Section 9.1.2.2(e): "Remove from consideration any routes with
	// less-preferred interior cost.  The interior cost of a route is determined
	// by calculating the metric to the NEXT_HOP for the route using the Routing
	// Table.  If the NEXT_HOP hop for a route is reachable, but no cost can be
	// determined, then this step should be skipped (equivalently, consider all
	// routes to have equal costs)."
	// The skip never applies here: every Loc-RIB path carries a metric, so
	// igpcost.Resolve answers a cost for every reachable next hop, zero included.
	// An unresolved distance means the Loc-RIB does not reach the next hop
	// (no covering route, a discard route, a loop), and RFC 4271 Section 9.1.2
	// excludes such a route from Phase 2. extractCandidate gives it the maximum
	// cost so it loses this step; skipping the step instead would let it win on
	// BGP Identifier. Full exclusion is the recorded gap RFC4271-9.1.2-1.
	if a.IGPCost != b.IGPCost {
		if a.IGPCost < b.IGPCost {
			return -1, BestStepIGPCost
		}
		return 1, BestStepIGPCost
	}

	// Step 7: Lowest Router ID.
	if a.OriginatorIP.IsValid() && b.OriginatorIP.IsValid() {
		if cmp := a.OriginatorIP.Compare(b.OriginatorIP); cmp != 0 {
			return cmp, BestStepRouterID
		}
	}

	// Step 8: Shortest CLUSTER_LIST.
	// RFC 4456 Section 9: "the following rule SHOULD be inserted between Steps
	// f) and g): a BGP Speaker SHOULD prefer a route with the shorter
	// CLUSTER_LIST length. The CLUSTER_LIST length is zero if a route does not
	// carry the CLUSTER_LIST attribute."
	// Step f) is the router-id step above and step g) is the peer address
	// below, so this is the slot the RFC names.
	if a.ClusterListEntries != b.ClusterListEntries {
		if a.ClusterListEntries < b.ClusterListEntries {
			return -1, BestStepClusterList
		}
		return 1, BestStepClusterList
	}

	// Step 9: Lowest peer address.
	if a.PeerIP != b.PeerIP {
		return a.PeerIP.Compare(b.PeerIP), BestStepPeerAddr
	}

	// Step 10: lowest path identifier (final tiebreak). Two paths of one
	// ADD-PATH session that tie on every RFC 4271 step would otherwise be
	// ordered by where the store happened to keep them, so the elected path
	// could change with no change on the wire. This is Ze's choice, not RFC
	// text: RFC 7911 gives the identifier no rank.
	if a.PathID != b.PathID {
		return cmp.Compare(a.PathID, b.PathID), BestStepPathID
	}

	return 0, BestStepEqual
}

// comparePairWithReason runs the full RFC 4271 §9.1.2 decision process and
// additionally reports the deciding step and a short textual reason. Used by
// SelectBestExplain; the hot-path comparePair skips the narrative.
// KEEP IN SYNC with comparePair above (same steps, without reason strings).
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

	if cmp := compareAIGP(a, b); cmp != 0 {
		return cmp, BestStepAIGP, fmt.Sprintf("aigp %t/%d + %d vs %t/%d + %d",
			a.HasAIGP, a.AIGP, a.IGPCost, b.HasAIGP, b.AIGP, b.IGPCost)
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
	if sameNeighborAS(a, b) {
		if a.MED != b.MED {
			reason := fmt.Sprintf("med %d vs %d (same neighbor AS %d)", a.MED, b.MED, neighborAS(a))
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

	// RFC 4271 Section 9.1.2.2(e): "Remove from consideration any routes with
	// less-preferred interior cost." Same step as compareAfterMED, which says why
	// an unresolved next hop ranks last rather than skipping the step.
	if a.IGPCost != b.IGPCost {
		reason := fmt.Sprintf("igp-cost %d vs %d", a.IGPCost, b.IGPCost)
		if a.IGPCost < b.IGPCost {
			return -1, BestStepIGPCost, reason
		}
		return 1, BestStepIGPCost, reason
	}

	// Step 7: Lowest Router ID (use ORIGINATOR_ID when present, RFC 4456).
	// RFC 4271: BGP Identifier is a 32-bit unsigned integer — compare as IP bytes, not strings.
	if a.OriginatorIP.IsValid() && b.OriginatorIP.IsValid() {
		if cmp := a.OriginatorIP.Compare(b.OriginatorIP); cmp != 0 {
			return cmp, BestStepRouterID, fmt.Sprintf("router-id %s vs %s", a.OriginatorIP, b.OriginatorIP)
		}
	}

	// Step 8: Shortest CLUSTER_LIST.
	// RFC 4456 Section 9: "the following rule SHOULD be inserted between Steps
	// f) and g): a BGP Speaker SHOULD prefer a route with the shorter
	// CLUSTER_LIST length. The CLUSTER_LIST length is zero if a route does not
	// carry the CLUSTER_LIST attribute."
	if a.ClusterListEntries != b.ClusterListEntries {
		reason := "cluster-list-length " + textbuf.StringUint32(uint32(a.ClusterListEntries)) +
			" vs " + textbuf.StringUint32(uint32(b.ClusterListEntries))
		if a.ClusterListEntries < b.ClusterListEntries {
			return -1, BestStepClusterList, reason
		}
		return 1, BestStepClusterList, reason
	}

	// Step 9: Lowest peer address (final tiebreak).
	// RFC 4271 §9.1.2.2(g): "prefer the route received from the peer with the lowest BGP Identifier"
	if a.PeerIP != b.PeerIP {
		return a.PeerIP.Compare(b.PeerIP), BestStepPeerAddr,
			fmt.Sprintf("peer-address %s vs %s", a.PeerAddr, b.PeerAddr)
	}

	// Step 10: lowest path identifier, the same Ze tie-break comparePair makes.
	if a.PathID != b.PathID {
		var tb textbuf.Buffer
		return cmp.Compare(a.PathID, b.PathID), BestStepPathID,
			tb.Str("path-id ").Uint32(a.PathID).Str(" vs ").Uint32(b.PathID).String()
	}

	return 0, BestStepEqual, "identical candidates"
}

// Protocol-type labels used by both reason narration and best-path change
// events. Kept as package-level constants so there is a single source of
// truth for consumers that match JSON field values.
// ebgpLabel returns the BGP protocol type for the boolean. Kept small so
// the comparePairWithReason hot path stays inlineable.
func ebgpLabel(isEBGP bool) routeaction.ProtocolType {
	if isEBGP {
		return routeaction.ProtocolEBGP
	}
	return routeaction.ProtocolIBGP
}

// compareAIGP first eliminates candidates without a received metric, then
// compares received metric plus the resolved interior distance. Saturation is
// essential: wrapping makes the largest path appear to be the shortest.
func compareAIGP(a, b *Candidate) int {
	if a.HasAIGP != b.HasAIGP {
		if a.HasAIGP {
			return -1
		}
		return 1
	}
	if !a.HasAIGP {
		return 0
	}
	aMetric := igpcost.Add(a.AIGP, a.IGPCost)
	bMetric := igpcost.Add(b.AIGP, b.IGPCost)
	if aMetric < bMetric {
		return -1
	}
	if aMetric > bMetric {
		return 1
	}
	return 0
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

// neighborAS is the neighbor AS the MED step compares, or 0 when it is unknown.
//
// RFC 4271 Section 9.1.2.2 (c): "If the route is learned via IBGP, and the other IBGP
// speaker either (a) originated the route, or (b) created the route by aggregation and
// the AS_PATH attribute of the aggregate route is either empty or begins with an
// AS_SET, it is the local AS."
//
// The leftmost AS stands for the neighbor AS whenever the AS_PATH carries one. An
// IBGP-learned route with an empty AS_PATH was originated inside the local AS, so its
// neighbor AS is LocalASN. An EBGP route with no leftmost AS, or a candidate whose
// session class is unknown (LocalASN 0), has no neighbor AS, and 0 keeps it out of
// every MED comparison.
func neighborAS(c *Candidate) uint32 {
	if c.FirstAS != 0 {
		return c.FirstAS
	}
	if c.LocalASN == 0 {
		return 0
	}
	if c.PeerASN != c.LocalASN {
		return 0
	}
	return c.LocalASN
}

// sameNeighborAS reports whether a and b were learned from one known neighbor AS, the
// condition RFC 4271 Section 9.1.2.2 (c) puts on comparing their MED.
func sameNeighborAS(a, b *Candidate) bool {
	neighbor := neighborAS(a)
	if neighbor == 0 {
		return false
	}
	return neighbor == neighborAS(b)
}

// firstASInPath extracts the first AS number from an AS_PATH attribute value.
// Used for MED comparison: MED is only compared between routes from the same neighbor AS.
// Returns 0 if the path is empty or truncated, or begins with an AS_SET.
func firstASInPath(data []byte) uint32 {
	// Minimum: type(1) + count(1) + one 4-byte ASN = 6 bytes.
	if len(data) < 6 {
		return 0
	}
	// RFC 4271 Section 9.1.2.2 (c): "If the route is learned via IBGP, and the
	// other IBGP speaker ... (b) created the route by aggregation and the
	// AS_PATH attribute of the aggregate route is either empty or begins with
	// an AS_SET, it is the local AS." An AS_SET is unordered, so its first
	// member names no neighbor AS: answer 0, which neighborAS turns into the
	// local AS for an IBGP route and into "no neighbor AS" (no MED comparison)
	// for an EBGP one.
	if attribute.ASPathSegmentType(data[0]) == attribute.ASSet {
		return 0
	}
	count := data[1]
	if count == 0 {
		return 0
	}
	return uint32(data[2])<<24 | uint32(data[3])<<16 | uint32(data[4])<<8 | uint32(data[5])
}

// clusterIDOctets is the width of one CLUSTER_ID in a CLUSTER_LIST value.
// RFC 4456 Section 8: "It is a sequence of CLUSTER_ID values representing the
// reflection path that the route has passed.".
const clusterIDOctets = 4

// clusterListEntriesMax saturates a CLUSTER_LIST count. It is the widest
// uint16, so a list too long to count and a list this speaker cannot read both
// sort LAST at the CLUSTER_LIST step rather than winning it.
const clusterListEntriesMax = math.MaxUint16

// clusterListEntries counts the CLUSTER_IDs in a CLUSTER_LIST attribute value.
// The unit is entries, not octets: RFC 4456 Section 9 calls this the
// "CLUSTER_LIST length", and a CLUSTER_ID is four octets wide.
//
// The count is uint16 rather than uint8 because the attribute is not bounded to
// 255 entries: a BGP path attribute value is at most 65535 octets (RFC 4271
// Section 4.3, extended length), so a CLUSTER_LIST can carry 16383 CLUSTER_IDs
// and a uint8 would wrap a long list down to a short one -- the worst direction
// to be wrong in, because a wrapped count wins the comparison it should lose.
// A value longer than any wire message saturates instead of wrapping, so an
// over-long list sorts last.
//
// A trailing partial CLUSTER_ID is not counted. ParseClusterList rejects a
// value whose length is not a multiple of four, and counting a fragment here
// would claim an ID the wire did not carry.
func clusterListEntries(data []byte) uint16 {
	entries := len(data) / clusterIDOctets
	if entries > clusterListEntriesMax {
		return clusterListEntriesMax
	}
	return uint16(entries)
}
