package infra_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"

	// plugin/all links every plugin, so every YANG module registers and the
	// derivation below reads the whole tree rather than this package's own
	// imports. It is not enough on its own: each plugin sits behind a feature
	// build tag, so a run without them links 18 conf modules instead of the
	// full set. The control below is what catches that.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// TestEveryValidatorSectionIsWalkedOrExcused closes the gap that let three dead
// ze:validate sections sit unnoticed. The goal is that a declared validator
// either RUNS or has a reason in code for why it does not; the method is to
// derive the declaring sections from the resolved model and subtract the two
// sets that account for them.
//
// It exists because nothing else can see this. An annotation that never runs is
// spelled exactly like one that does, and CheckAllValidatorsRegistered passes
// either way: it asks whether the validator function exists, never whether the
// walk reaches it. So the failure mode was silence, and `ze config validate`
// answered valid for a config its own model forbids.
//
// The control comes first and is the half that took a second attempt. A bare
// `go test` compiles no feature tags, so every gated plugin's YANG is absent,
// the derivation sees four declaring sections instead of ten, and a clean sheet
// means nothing. Asserting the recorded exclusions are PRESENT is what turns
// that into a red with an instruction rather than a false pass.
//
// Remove an entry from knownUnwalkedValidatorSections and this goes red naming
// that section. Add a plugin whose YANG declares a ze:validate under a new
// top-level section and it goes red too, which is the point: the next author is
// told, rather than shipping a rule that does nothing.
func TestEveryValidatorSectionIsWalkedOrExcused(t *testing.T) {
	cov, err := config.ValidatorSectionCoverage()
	if err != nil {
		t.Fatalf("derive the validator sections: %v", err)
	}

	for section := range config.KnownUnwalkedValidatorSections() {
		if cov.Declaring[section] {
			continue
		}
		t.Fatalf("this binary did not link the YANG that declares %q, so the "+
			"derivation covered %d declaring sections rather than the whole tree "+
			"and an empty answer would prove nothing. Run it with the feature "+
			"tags: ./le test-unit", section, len(cov.Declaring))
	}

	for _, section := range cov.Unaccounted {
		t.Errorf("top-level section %q declares a ze:validate that never runs: "+
			"ValidateCustomSections walks validatedSections only. Add it to that "+
			"list once you have measured what it newly refuses, or record why not "+
			"in knownUnwalkedValidatorSections (internal/component/config/validate_sections.go)",
			section)
	}
}
