package commit

// VALIDATES: a structural stage an interrupt cut short is reported as never
// judged and does not refuse the commit, while a stage that judged the tree
// and found it red still does.
// PREVENTS: the gate charging an interrupted stage's exit status as a red, so
// every commit carrying Go after a canceled verify needed structural-red-ok
// against a defect that did not exist.

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestAnInterruptedStructuralStageIsNeverJudgedNotCharged holds the reader
// over both spellings of an interrupted stage: the index the writer now
// produces (interrupted, not reached, no verdict) and an index written before
// the writer marked interruption (reached, exit 130, one generic group). Each
// leaves the stage out of Charged, names it in Unjudged, and gives a cause
// that says it was never judged and the command that judges it. Method: the
// same structural stage, recorded each way, against an empty scope.
func TestAnInterruptedStructuralStageIsNeverJudgedNotCharged(t *testing.T) {
	for name, index := range map[string]string{
		"marked": `{"stages":[{"stage":"doc wiring","reached":false,"exit-code":0,
			"interrupted":true,"outside-run":true,"rerun":"le doc wiring"}]}`,
		"exit 130": `{"stages":[{"stage":"doc wiring","reached":true,"exit-code":130,
			"groups":[{"group-id":"stage:doc wiring","kind":"generic","related":[],
			"summary":"context canceled","rerun":"le doc wiring"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeCommitFixture(t, root, "mine.txt", "mine\n")
			writeCommitFixture(t, root, "tmp/ze-verify-failures.json", index)

			reds := structuralGateReds(root, []string{"mine.txt"})

			if len(reds.Charged) != 0 {
				t.Fatalf("an interrupted stage was charged: %#v", reds)
			}
			if !slices.Equal(reds.Unjudged, []string{"doc wiring"}) || len(reds.UnjudgedCauses) != 1 {
				t.Fatalf("Unjudged = %#v, causes %#v, want doc wiring named once", reds.Unjudged, reds.UnjudgedCauses)
			}
			cause := reds.UnjudgedCauses[0]
			for _, want := range []string{"never judged", "interrupted", "judge it with: le doc wiring"} {
				if !strings.Contains(cause, want) {
					t.Errorf("cause %q does not say %q", cause, want)
				}
			}
			state := verificationState(root, []string{"mine.txt"})
			if state.State == verifyFresh || !strings.Contains(state.Detail, cause) {
				t.Errorf("verification state %#v does not report the unjudged stage", state)
			}
		})
	}
}

// TestAnUnreadableFailureIndexIsReportedNotClean holds that the gate never
// reads an index it could not parse, or a stage entry it could not parse, as
// clean: it charges nothing from it, since it holds no verdict, and it names
// what is wrong, why, and the command that rewrites it, in the verification
// state the commit reports. A missing index is legitimately empty and reports
// nothing. Method: a malformed index, an index with one malformed stage entry,
// and no index at all.
func TestAnUnreadableFailureIndexIsReportedNotClean(t *testing.T) {
	for name, tc := range map[string]struct {
		index string
		want  bool
	}{
		"malformed index":       {index: `{"stages":[`, want: true},
		"malformed stage entry": {index: `{"stages":[{"stage":7}]}`, want: true},
		"no index":              {want: false},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeCommitFixture(t, root, "mine.txt", "mine\n")
			if tc.index != "" {
				writeCommitFixture(t, root, "tmp/ze-verify-failures.json", tc.index)
			}

			reds := structuralGateReds(root, []string{"mine.txt"})

			if len(reds.Charged) != 0 {
				t.Fatalf("an unreadable index was charged: %#v", reds)
			}
			if got := len(reds.Unreadable) != 0; got != tc.want {
				t.Fatalf("Unreadable = %#v, want reported %v", reds.Unreadable, tc.want)
			}
			if !tc.want {
				return
			}
			for _, want := range []string{"tmp/ze-verify-failures.json", "no verdict", "le verify"} {
				if !strings.Contains(reds.Unreadable[0], want) {
					t.Errorf("unreadable cause %q does not say %q", reds.Unreadable[0], want)
				}
			}
			state := verificationState(root, []string{"mine.txt"})
			if !strings.Contains(state.Detail, reds.Unreadable[0]) {
				t.Errorf("verification state %#v does not report the unreadable index", state)
			}
		})
	}
}

// TestACurrentFormatExit130IsCharged holds that the exit-130 reading is only
// for an index the old writer produced: the current writer never gives an
// interrupted stage an exit, so 130 in its index is the stage's own red (a
// child SIGINTed under a shell, say) and is charged. Method: the legacy
// record's stage, in an index that carries the current format.
func TestACurrentFormatExit130IsCharged(t *testing.T) {
	root := t.TempDir()
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	writeCommitFixture(t, root, "tmp/ze-verify-failures.json", `{"format":`+
		strconv.Itoa(verifyengine.FailureIndexFormat)+`,"stages":[{"stage":"doc wiring",
		"reached":true,"exit-code":130,"groups":[{"group-id":"stage:doc wiring","kind":"generic",
		"related":[],"summary":"action exited 130","rerun":"le doc wiring"}]}]}`)

	reds := structuralGateReds(root, []string{"mine.txt"})

	if !slices.Equal(reds.Charged, []string{"doc wiring"}) || len(reds.Unjudged) != 0 {
		t.Fatalf("structuralGateReds = %#v, want doc wiring charged and nothing unjudged", reds)
	}
}

// TestCreatePassesAnInterruptedStageAndRefusesARealRed drives the commit gate
// itself: an index whose only structural entry is an interrupted stage lets a
// dry-run commit through with no override, and the same stage recorded as a
// genuine red (exit 1) is refused. Method: Create over the two indexes.
func TestCreatePassesAnInterruptedStageAndRefusesARealRed(t *testing.T) {
	root := newCommitRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "interrupted-structural-fixture")
	writeCommitFixture(t, root, "mine.txt", "mine\n")
	options := Options{Subject: "interrupted stage", Files: []string{"mine.txt"}, DryRun: true}

	writeCommitFixture(t, root, "tmp/ze-verify-failures.json", `{"stages":[{
		"stage":"doc wiring","reached":true,"exit-code":130,
		"groups":[{"group-id":"stage:doc wiring","kind":"generic","related":[],
		"summary":"context canceled","rerun":"le doc wiring"}]}]}`)
	prepared, err := Create(root, &options)
	if err != nil {
		t.Fatalf("Create refused an interrupted stage that holds no verdict: %v", err)
	}
	if !strings.Contains(prepared.Verify.Detail, "doc wiring: never judged") {
		t.Errorf("the commit report does not name the unjudged stage: %q", prepared.Verify.Detail)
	}

	writeCommitFixture(t, root, "tmp/ze-verify-failures.json", `{"stages":[{
		"stage":"doc wiring","reached":true,"exit-code":1,
		"groups":[{"group-id":"stage:doc wiring","kind":"generic","related":[],
		"summary":"action exited 1","rerun":"le doc wiring"}]}]}`)
	if _, err := Create(root, &options); err == nil || !strings.Contains(err.Error(), "deterministic structural gate") {
		t.Fatalf("Create over a genuine red = %v, want the structural refusal", err)
	}
}
