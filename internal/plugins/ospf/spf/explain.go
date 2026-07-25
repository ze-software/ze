// Design: plan/learned/1052-ospf-ext-14-debug-introspection.md -- read-only SPF-explain snapshot.
// RFC: rfc/short/rfc2328.md (Section 16.4: intra-area > inter-area > external Type 1 >
// external Type 2 path preference, then lowest cost).
//
// ExplainSnapshot answers "why did this route win?" purely from the LAST computed result:
// the installed winner per prefix plus the retained candidate set, WITHOUT a recompute
// (R-3). It is address-family agnostic (the Computer is shared by both families); the engine
// tags the OSPFv3 result with its address family / Instance ID.

package spf

import (
	"sort"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ExplainCandidate is one candidate path considered for a prefix.
type ExplainCandidate struct {
	Type   string `json:"type"`
	Metric uint64 `json:"metric"`
	Origin string `json:"origin"`
	Area   string `json:"area"`
	Winner bool   `json:"winner"`
}

// ExplainEntry explains one installed prefix: the winner, its cost, and the Section 16.4
// tie-break that selected it over the other candidates.
type ExplainEntry struct {
	Prefix        string             `json:"prefix"`
	Area          string             `json:"area"`
	WinningType   string             `json:"winning-type"`
	WinningMetric uint64             `json:"winning-metric"`
	Origin        string             `json:"origin"`
	Reason        string             `json:"reason"`
	Candidates    []ExplainCandidate `json:"candidates"`
}

// runCount returns the number of completed SPF computations (read-only). The explain view
// never increments it, so a test can assert an explain call did not recompute.
func (c *Computer) runCount() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runs
}

// ExplainSnapshot returns the per-prefix explanation for the last SPF result. Read-only.
func (c *Computer) ExplainSnapshot() []ExplainEntry {
	c.mu.Lock()
	winners := append([]RouteEntry(nil), c.last...)
	cands := append([]RouteEntry(nil), c.lastCandidates...)
	c.mu.Unlock()
	return explainRoutes(winners, cands)
}

// explainRoutes is the pure explain logic over the winners + candidate set.
func explainRoutes(winners, candidates []RouteEntry) []ExplainEntry {
	byPrefix := map[string][]RouteEntry{}
	for _, cand := range candidates {
		key := cand.Prefix.String()
		byPrefix[key] = append(byPrefix[key], cand)
	}
	out := make([]ExplainEntry, 0, len(winners))
	for _, w := range winners {
		key := w.Prefix.String()
		cands := byPrefix[key]
		if len(cands) == 0 {
			cands = []RouteEntry{w}
		}
		entry := ExplainEntry{
			Prefix:        key,
			Area:          w.AreaID.String(),
			WinningType:   w.Type.String(),
			WinningMetric: w.Metric,
			Origin:        w.Origin.String(),
			Reason:        explainReason(w, cands),
		}
		for _, cand := range cands {
			entry.Candidates = append(entry.Candidates, ExplainCandidate{
				Type:   cand.Type.String(),
				Metric: cand.Metric,
				Origin: cand.Origin.String(),
				Area:   cand.AreaID.String(),
				Winner: cand.Type == w.Type && cand.Metric == w.Metric && cand.Origin == w.Origin,
			})
		}
		sort.SliceStable(entry.Candidates, func(i, j int) bool {
			return entry.Candidates[i].Metric < entry.Candidates[j].Metric
		})
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Prefix < out[j].Prefix })
	return out
}

// explainReason states the RFC 2328 Section 16.4 rule that selected the winner.
func explainReason(w RouteEntry, cands []RouteEntry) string {
	differentTypes := false
	moreThanOne := len(cands) > 1
	for _, cand := range cands {
		if routeTypeRank(cand.Type) != routeTypeRank(w.Type) {
			differentTypes = true
			break
		}
	}
	var tb textbuf.Buffer
	switch {
	case differentTypes:
		return tb.Str("path-type preference (RFC 2328 Section 16.4): ").Str(w.Type.String()).
			Str(" preferred over lower-preference candidates").String()
	case moreThanOne:
		return tb.Str("lowest cost ").Uint(w.Metric).Str(" among equal-preference ").
			Str(w.Type.String()).Str(" paths").String()
	default:
		return tb.Str("only path (").Str(w.Type.String()).Str(")").String()
	}
}
