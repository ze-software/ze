// Design: docs/architecture/route-selection.md -- whole-set MED elimination.
// Related: bestpath.go -- candidate attributes and pairwise criteria.
package rib

import (
	"cmp"
	"slices"
)

// selectBestCandidates partitions caller-owned scratch, retaining every pointer.
// The returned count bounds the candidates that survived through MED, including
// any equal-cost multipath siblings. No Candidate value is modified.
//
// RFC 4271 Section 9.1.2.2: "The tie-breaking algorithm begins by considering
// all equally preferable routes to the same destination, and then selects
// routes to be removed from consideration." "The criteria MUST be applied in
// the order specified."
// Elimination operates on the whole candidate set.
func selectBestCandidates(candidates []*Candidate, explanation *bestPathExplanation) (*Candidate, int) {
	if len(candidates) == 0 {
		return nil, 0
	}
	if len(candidates) == 1 {
		return candidates[0], 1
	}

	// RFC 4271 Section 9.1.2.2(a,b): only earlier-criteria survivors reach MED.
	beforeMED := candidates[0]
	for _, candidate := range candidates[1:] {
		if result, _ := compareBeforeMED(candidate, beforeMED); result < 0 {
			beforeMED = candidate
		}
	}
	eligible := 0
	for i, candidate := range candidates {
		if result, _ := compareBeforeMED(candidate, beforeMED); result != 0 {
			if explanation != nil {
				explanation.remove(candidate, beforeMED)
			}
			continue
		}
		candidates[eligible], candidates[i] = candidates[i], candidates[eligible]
		eligible++
	}
	if eligible == 1 {
		return candidates[0], 1
	}

	// RFC 4271 Section 9.1.2.2(c): remove higher MEDs within each known AS.
	// Sorting only the surviving pointer slice bounds grouping by O(N log N)
	// without a per-election map, allocation, or quadratic peer scan.
	slices.SortFunc(candidates[:eligible], compareNeighborAS)
	eligible = retainLowestMED(candidates[:eligible], explanation)

	// RFC 4271 Section 9.1.2.2(d-g): MED losers cannot enter later tie-breaks.
	best := candidates[0]
	for _, candidate := range candidates[1:eligible] {
		winner, loser := best, candidate
		if result, _ := compareAfterMED(candidate, best); result < 0 {
			winner, loser = candidate, best
		}
		if explanation != nil {
			explanation.remove(loser, winner)
		}
		best = winner
	}
	return best, eligible
}

// compareNeighborAS groups routes without assigning preference to an AS number.
// RFC 4271 Section 9.1.2.2(c): "MULTI_EXIT_DISC is only comparable between routes
// learned from the same neighboring AS (the neighboring AS is determined from
// the AS_PATH attribute)."
// Grouping retains the neighboring-AS comparison boundary.
func compareNeighborAS(a, b *Candidate) int {
	return cmp.Compare(neighborAS(a), neighborAS(b))
}

// retainLowestMED retains every group minimum, not just one representative.
// Unknown AS zero is not a group: none of its routes can suppress another.
// RFC 4271 Section 9.1.2.2(c): "Remove from consideration routes with
// less-preferred MULTI_EXIT_DISC attributes."
// Each group retains all candidates tied at its lowest MED.
func retainLowestMED(candidates []*Candidate, explanation *bestPathExplanation) int {
	retained := 0
	for start := 0; start < len(candidates); {
		end := start + 1
		lowest := candidates[start]
		neighbor := neighborAS(lowest)
		if neighbor != 0 {
			for end < len(candidates) && neighborAS(candidates[end]) == neighbor {
				if candidates[end].MED < lowest.MED {
					lowest = candidates[end]
				}
				end++
			}
		}
		for i := start; i < end; i++ {
			candidate := candidates[i]
			if candidate.MED > lowest.MED {
				if explanation != nil {
					explanation.remove(candidate, lowest)
				}
				continue
			}
			candidates[retained], candidates[i] = candidates[i], candidates[retained]
			retained++
		}
		start = end
	}
	return retained
}

// remove records the actual elimination witness, which need not be the final
// winner. Keeping original indices makes the existing CLI shape meaningful.
// RFC 4271 Section 9.1.2.2: "The criteria MUST be applied in the order specified."
func (explanation *bestPathExplanation) remove(loser, winner *Candidate) {
	loserIdx, winnerIdx := explanation.indices[loser], explanation.indices[winner]
	incumbentIdx, challengerIdx := min(loserIdx, winnerIdx), max(loserIdx, winnerIdx)
	_, step, reason := comparePairWithReason(explanation.Candidates[challengerIdx], explanation.Candidates[incumbentIdx])
	explanation.Steps = append(explanation.Steps, PairwiseStep{
		IncumbentIdx: incumbentIdx, ChallengerIdx: challengerIdx,
		WinnerIdx: winnerIdx, Step: step, Reason: reason,
	})
}
