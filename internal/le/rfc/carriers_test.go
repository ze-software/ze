// VALIDATES: every carrier the vocabulary declares has a place in the reading
// order, so a kind added to the table cannot sort last in silence.
// PREVENTS: a published page ordering evidence by a sequence written beside the
// vocabulary rather than in it, which is a second declaration of a closed set.

package rfc

import "testing"

// TestEveryCarrierTheVocabularyDeclaresIsRanked holds the rank against the REAL
// carrier table rather than against the two ordered lists beside it.
//
// The two lists and the table are the same vocabulary written twice, and this
// is what stops them drifting: a kind or a tier that reaches a Carrier and not
// carrierKindOrder answers no rank, and a consumer would then place it by a
// rule of its own (ai/rules/principles.md).
func TestEveryCarrierTheVocabularyDeclaresIsRanked(t *testing.T) {
	table := carriersFor(FunctionalSuites(), map[string]string{})
	if len(table) == 0 {
		t.Fatal("the carrier table is empty, so this proves nothing")
	}
	seen := map[string]bool{}
	for _, carrier := range table {
		label := carrier.Kind + "/" + carrier.Tier
		seen[label] = true
		if _, ranked := CarrierRank(carrier.Kind, carrier.Tier); !ranked {
			t.Errorf("carrier %q declares %s, which the reading order does not rank",
				carrier.Name, label)
		}
		if rank, ranked := CarrierLabelRank(label); !ranked {
			t.Errorf("the label %q answers no rank", label)
		} else if want, _ := CarrierRank(carrier.Kind, carrier.Tier); rank != want {
			t.Errorf("the label %q ranks %d and its parts rank %d", label, rank, want)
		}
	}
	t.Logf("%d carriers over %d distinct kind/tier labels", len(table), len(seen))
}

// TestTheReadingOrderIsTotalAndAscending holds that the two vocabularies
// produce one strictly increasing sequence, kind first and tier within it.
//
// The property that matters to a reader: unit/verify precedes unit/nightly, and
// both precede every functional row.
func TestTheReadingOrderIsTotalAndAscending(t *testing.T) {
	last, first := 0, true
	for _, kind := range CarrierKinds() {
		for _, tier := range CarrierTiers() {
			rank, ranked := CarrierRank(kind, tier)
			if !ranked {
				t.Fatalf("%s/%s is in the vocabulary and answers no rank", kind, tier)
			}
			if !first && rank <= last {
				t.Errorf("%s/%s ranks %d, which does not follow %d", kind, tier, rank, last)
			}
			last, first = rank, false
		}
	}
	unitVerify, _ := CarrierRank(kindUnit, tierVerify)
	unitNightly, _ := CarrierRank(kindUnit, tierNightly)
	functionalVerify, _ := CarrierRank(kindFunctional, tierVerify)
	if unitVerify >= unitNightly || unitNightly >= functionalVerify {
		t.Errorf("the order reads %d, %d, %d: tier must sort inside kind",
			unitVerify, unitNightly, functionalVerify)
	}
	if _, ranked := CarrierRank("moon", tierVerify); ranked {
		t.Error("a kind outside the vocabulary answers a rank")
	}
	if _, ranked := CarrierLabelRank("unit"); ranked {
		t.Error("a label with no tier answers a rank")
	}
}
