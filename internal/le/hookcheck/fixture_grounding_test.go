package hookcheck

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/le/hookruntime"
)

// ungroundedCategories are the fixture categories whose verdict RESTATES the
// rule instead of asking the producer for it. Each one is a place where the
// fixtures can go on passing after the producer stops doing what they describe.
//
// It is a ratchet, not an allowance. A category may leave this list, never join
// it, and a category that leaves must not come back.
//
// Grounded means ONE thing: the verdict reaches hookruntime, the package that
// runs in the hook. It does not mean the verdict calls a function. The first
// version of this guard counted any non-string-library call as grounding and
// reported five bound categories; every one of those five called a copy of the
// producer's rule living in THIS package (sourceKind, safeSessionID,
// journalCells and two more). A copy drifts exactly like an inline
// restatement, so the true count was zero of twenty-five.
//
// The failure this prevents has already happened. `eae2825926` replaced the
// Python hooks with Go, and two design fixtures did not come across: the
// design-gate category claimed writeDesignEvidence matches the KIND of source
// read against the kind of file written, and design-ref claimed a hook demands
// a `// Design:` header. The gate matches no kinds, and no hook demands that
// header. Fifty-two design-gate fixtures stayed green throughout, and
// plan/spec-finish-ci-coverage.md cited them as its proof.
//
//nolint:gochecknoglobals // a ratchet baseline is data, and it is read by one test
var ungroundedCategories = []string{
	categoryCommitGate,
	categoryDelegation,
	categoryDelegationReminder,
	categoryDesignGate,
	categoryDesignRef,
	categoryDraftIncubator,
	categoryJournalRowShape,
	categoryMarkSourceRead,
	categoryPhaseGates,
	categoryRFCChangedLedger,
	categoryRFCTestGuard,
	categoryScriptWeakeningArms,
	categorySessionID,
	categorySessionState,
	categorySessionStateLocation,
	categorySubagentContext,
	categoryTestFirst,
	categoryValidateSpec,
	categoryWeakenedHatch,
}

// TestEveryFixtureCategoryReachesItsProducer holds the rule that a fixture
// naming a producer must ASK it, because a fixture that restates the rule
// cannot go red when the producer stops following it.
//
// VALIDATES: each category is either bound through hookruntime.Probe, which
// runs the check itself, or named on the ungroundedCategories baseline. There
// is no third state.
// PREVENTS: the design-gate failure recorded in
// plan/journal/refactor-removes-feature.md, where a migration kept 52 fixture
// names, dropped the behavior they exercised, and left a green suite that a
// spec cited as proof.
func TestEveryFixtureCategoryReachesItsProducer(t *testing.T) {
	for _, category := range &fixtureCategories {
		_, bound := categoryProbes[category.name]
		listed := slices.Contains(ungroundedCategories, category.name)
		switch {
		case bound && listed:
			t.Errorf("category %q now asks its producer, so remove it from ungroundedCategories: "+
				"the ratchet only tightens", category.name)
		case !bound && !listed:
			t.Errorf("category %q decides its verdict without reaching hookruntime, so it restates "+
				"%s rather than asking it. A restatement cannot go red when the producer stops "+
				"following the rule, which is how 52 design-gate fixtures stayed green through a "+
				"rewrite that dropped what they proved. Add a categoryProbes entry, or state why "+
				"none is possible by adding the category to ungroundedCategories",
				category.name, category.evidence)
		}
	}

	for _, name := range ungroundedCategories {
		if !slices.ContainsFunc(fixtureCategories[:], func(c fixtureCategory) bool { return c.name == name }) {
			t.Errorf("ungroundedCategories names %q, which is not a fixture category: "+
				"a baseline that outlives its subject stops being read", name)
		}
	}
}

// TestEveryProbeNamesARegisteredCheck fails when a probe names a check that
// hookruntime does not register. Without it a renamed producer would leave the
// probe answering "not found", which reads as a refusal and would fail the
// selftest with a message about the fixture rather than about the rename.
func TestEveryProbeNamesARegisteredCheck(t *testing.T) {
	for category, probe := range categoryProbes {
		_, _, found := hookruntime.Probe(probe.check, hookruntime.Payload{ToolName: probe.tool})
		if !found {
			t.Errorf("category %q probes %q, which hookruntime registers under no name: "+
				"the producer was renamed or moved out of nativeHookActions", category, probe.check)
		}
	}
}

// TestEveryProbeSeparatesItsAllowFromItsRefusal runs each bound category's two
// values through the real check and fails when the producer does not tell them
// apart. checkFixtureCategory makes the same call at selftest time; this runs
// it in the unit gate, where the failure names the category.
func TestEveryProbeSeparatesItsAllowFromItsRefusal(t *testing.T) {
	for _, category := range &fixtureCategories {
		probe, bound := categoryProbes[category.name]
		if !bound {
			continue
		}
		if !probeVerdict(probe, category.allow) {
			t.Errorf("category %q: %s refused the value the fixtures call allowed", category.name, probe.check)
		}
		if probeVerdict(probe, category.refuse) {
			t.Errorf("category %q: %s allowed the value the fixtures call refused", category.name, probe.check)
		}
	}
}
