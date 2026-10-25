package le

import (
	"testing"

	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// migratedRefusal is one line a migrated area refused before its conversion
// onto an action table, with the code it answered then.
type migratedRefusal struct {
	area string
	line []string
	code int
}

// migratedRefusalsBatchA is the golden of the Phase 3 batch A areas, captured
// from the hand-rolled parsers before each conversion
// (plan/post-release/engineering/spec-le-cli-usability.md, AC-17). The bare
// line is in it because it answered the listing with 0 and ran no gate.
//
// The single-gate areas after doc yang-contract run their gate when bare
// (D-P3-bare), so no bare line of theirs is here: `help` is the line that
// answers their listing with 0. Their stray word answered 1 before the table
// and answers the usage code 2 since (D-P3-stray); the word after the verb is
// new with the verb.
var migratedRefusalsBatchA = []migratedRefusal{
	{"ai digest", []string{"help"}, 0},
	{"ai digest", []string{probeWord}, 2},
	{"ai digest", []string{"check", probeWord}, 2},
	{"arch iface-resolution", []string{"help"}, 0},
	{"arch iface-resolution", []string{probeWord}, 2},
	{"arch iface-resolution", []string{"check", probeWord}, 2},
	{"build gosum", []string{"help"}, 0},
	{"build gosum", []string{probeWord}, 2},
	{"build gosum", []string{"check", probeWord}, 2},
	{"build host-driver", []string{"help"}, 0},
	{"build host-driver", []string{probeWord}, 2},
	{"build host-driver", []string{"run", probeWord}, 2},
	{"cli grammar", []string{"help"}, 0},
	{"cli grammar", []string{probeWord}, 2},
	{"cli grammar", []string{"check", probeWord}, 2},
	{"cli list", []string{"help"}, 0},
	{"cli list", []string{probeWord}, 2},
	{"cli list", []string{"report", probeWord}, 2},
	{"cli ownership", []string{"help"}, 0},
	{"cli ownership", []string{probeWord}, 2},
	{"cli ownership", []string{"check", probeWord}, 2},
	{"config claims", []string{"help"}, 0},
	{"config claims", []string{probeWord}, 2},
	{"config claims", []string{"check", probeWord}, 2},
	{"doc consistency", []string{"help"}, 0},
	{"doc consistency", []string{probeWord}, 2},
	{"doc consistency", []string{"check", probeWord}, 2},
	{"doc check", nil, 0},
	{"doc check", []string{probeWord}, 2},
	{"doc check", []string{"links", probeWord}, 2},
	{"doc check", []string{"verify", probeWord}, 2},
	{"doc check", []string{"templ-output", probeWord}, 2},
	{"doc check", []string{"retired-commands", probeWord}, 2},
	{"doc yang-contract", nil, 0},
	{"doc yang-contract", []string{probeWord}, 2},
	{"doc yang-contract", []string{"command-contract", probeWord}, 2},
	{"doc yang-contract", []string{"doc-drift", probeWord}, 2},
	{"doc yang-contract", []string{"usage-contract", probeWord}, 2},
	{"doc yang-contract", []string{"help-shape", probeWord}, 2},
	{"doc yang-contract", []string{"pipe-operators-update", probeWord}, 2},
}

// TestMigratedAreaKeepsItsExitCodes holds every refusal of a migrated area to
// the code its golden row carries: the code it answered before its migration,
// or 2 where the table's parser now makes the refusal (D-P3-stray, D-P3-dash).
//
// VALIDATES: AC-17 for the Phase 3 areas of batches A, B and C (the rows above
// and in migrated_exit_codes_batchb_test.go and _batchc_test.go). Commit's
// rows are commit TestCommitRefusesWhatItsTableRefuses.
// PREVENTS: a conversion that changes what a script or a hook reads from an
// exit code, before Phase 5 decides the codes on purpose.
//
// The method runs each line through the registered handler, as leroot.Run
// does. Every line is refused before an action body runs, so nothing judges
// the tree. The batch A golden was proved against the parsers it replaced by
// running with -overlay pointing those areas at their HEAD files.
func TestMigratedAreaKeepsItsExitCodes(t *testing.T) {
	refusals := append(append(append([]migratedRefusal{}, migratedRefusalsBatchA...),
		migratedRefusalsBatchB...), migratedRefusalsBatchC...)
	for _, refusal := range refusals {
		handler := leroot.LookupCommand(refusal.area)
		if handler == nil {
			t.Errorf("area %q registers no handler", refusal.area)
			continue
		}
		if _, code := probeAnswer(judged(t, refusal.area, handler), refusal.line); code != refusal.code {
			t.Errorf("le %s %v answered %d, want %d", refusal.area, refusal.line, code, refusal.code)
		}
	}
}
