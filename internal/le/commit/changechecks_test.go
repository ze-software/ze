// VALIDATES: `./le commit create` runs every check registered in the
// changecheck seam over the commit it prepares, refusing a commit whose carried
// content adds what a check refuses and admitting one that adds nothing, with
// Before read from the parent and After from the carried content.
// PREVENTS: a change-set check that exists as a command no commit runs, and a
// seam that judges the tree's old instances instead of the commit's additions.
package commit

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// fixtureMarker is what the fixture check refuses when a commit adds it.
const fixtureMarker = "FIXTURE-REFUSED"

// fixtureCheck refuses each carried path whose After side holds fixtureMarker
// more often than its Before side: an addition, never an instance the parent
// already held.
func fixtureCheck() changecheck.Check {
	return changecheck.Check{Name: "fixture check", Judge: func(change changecheck.Change) (string, error) {
		var refused []string
		for _, path := range change.Paths {
			after, _, err := change.After(path)
			if err != nil {
				return "", err
			}
			before, _, err := change.Before(path)
			if err != nil {
				return "", err
			}
			if strings.Count(after, fixtureMarker) > strings.Count(before, fixtureMarker) {
				refused = append(refused, path+" adds "+fixtureMarker)
			}
		}
		return strings.Join(refused, "\n"), nil
	}}
}

// TestChangeChecksRefuseAStagedAddition holds that a registered check refuses
// a carried file that adds what it refuses, and that the refusal names the
// check and its text.
func TestChangeChecksRefuseAStagedAddition(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{"internal/x/x.go": "package x\n// " + fixtureMarker + " held by the parent\n"})
	writeCommitFixture(t, root, "internal/x/x.go", "package x\n// "+fixtureMarker+" held by the parent\n// "+fixtureMarker+" added\n")

	err := runChangeChecks(root, []string{"internal/x/x.go"}, nil, nil, "HEAD", []changecheck.Check{fixtureCheck()})
	if err == nil {
		t.Fatal("a commit adding what a registered check refuses was admitted")
	}
	for _, want := range []string{"./le fixture check refused what this commit adds", "internal/x/x.go adds " + fixtureMarker} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not say %q:\n%v", want, err)
		}
	}
}

// TestChangeChecksAdmitACleanCommit holds that a carried file that only keeps
// what its parent held is admitted: the seam judges additions, not the tree.
func TestChangeChecksAdmitACleanCommit(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{"internal/x/x.go": "package x\n// " + fixtureMarker + " held by the parent\n"})
	writeCommitFixture(t, root, "internal/x/x.go", "package x\n// "+fixtureMarker+" held by the parent\n// a new comment\n")

	if err := runChangeChecks(root, []string{"internal/x/x.go"}, nil, nil, "HEAD", []changecheck.Check{fixtureCheck()}); err != nil {
		t.Fatalf("a commit adding nothing a check refuses was refused: %v", err)
	}
}

// TestChangeChecksAnswerACheckThatCannotJudge holds that a check whose judge
// fails stops the commit as a run that could not judge, never as an accept.
func TestChangeChecksAnswerACheckThatCannotJudge(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{"internal/x/x.go": "package x\n"})
	failing := changecheck.Check{Name: "failing check", Judge: func(changecheck.Change) (string, error) {
		return "", errors.New("the fixture judge could not read the tree")
	}}
	err := runChangeChecks(root, []string{"internal/x/x.go"}, nil, nil, "HEAD", []changecheck.Check{failing})
	if err == nil {
		t.Fatal("a check that could not judge admitted the commit")
	}
	if exitCodeOf(err) != 3 {
		t.Errorf("exit code = %d, want 3 (could not run): %v", exitCodeOf(err), err)
	}
	if !errors.Is(err, changecheck.ErrCouldNotJudge) {
		t.Errorf("error does not wrap ErrCouldNotJudge: %v", err)
	}
}

// TestChangeCheckRegistryNamesItsChecksSorted holds the seam's registry:
// Register adds, Registered answers sorted by name, and Snapshot restores.
func TestChangeCheckRegistryNamesItsChecksSorted(t *testing.T) {
	t.Cleanup(changecheck.Snapshot())
	before := len(changecheck.Registered())
	changecheck.Register(changecheck.Check{Name: "zz fixture", Judge: fixtureCheck().Judge})
	changecheck.Register(changecheck.Check{Name: "aa fixture", Judge: fixtureCheck().Judge})
	registered := changecheck.Registered()
	if len(registered) != before+2 {
		t.Fatalf("registered = %d, want %d", len(registered), before+2)
	}
	for position := 1; position < len(registered); position++ {
		if registered[position-1].Name > registered[position].Name {
			t.Fatalf("registered checks are not sorted: %q before %q", registered[position-1].Name, registered[position].Name)
		}
	}
}
