// VALIDATES: record-run writes a run record only for an observed pass (D-6).
// PREVENTS: a failing, unselected, or mid-run-edited test reaching Supported.

package feature

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// passOutputs is what the two runners print for the fixture's items when they
// pass: the functional runner's per-test line and `go test -v`'s PASS line.
var passOutputs = map[string]string{
	"test/plugin/widget.ci":                      "6.7s     1/1  PASS  2  widget\npass  1/1  100.0%  6.7s\n",
	"internal/widget/widget_test.go::TestWidget": "=== RUN   TestWidget\n--- PASS: TestWidget (0.00s)\nPASS\n",
}

// fakeRunner answers outputs per item, exit 0 unless the item is in failing.
func fakeRunner(outputs map[string]string, failing ...string) runItem {
	return func(item string) (observation, error) {
		if slices.Contains(failing, item) {
			return observation{exited: false, output: "--- FAIL: " + item}, nil
		}
		return observation{exited: true, output: outputs[item]}, nil
	}
}

// recordedTree is the check fixture with its run record removed, which is the
// state record-run starts from for a feature nothing has run.
func recordedTree(t *testing.T) string {
	t.Helper()
	tree := fixtureTree(t, same)
	if err := os.Remove(filepath.Join(tree, runRecordRel("widget"))); err != nil {
		t.Fatal(err)
	}
	return tree
}

func requireNoRecord(t *testing.T, tree string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(tree, runRecordRel("widget"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused run wrote a record (stat: %v)", err)
	}
}

// TestRecordRunWritesWhatTheCheckAccepts: every item observed passing writes a
// record, and the check then lets the declaration stand at Supported.
func TestRecordRunWritesWhatTheCheckAccepts(t *testing.T) {
	tree := recordedTree(t)
	if _, err := recordRun(tree, "widget", fakeRunner(passOutputs), "abc", "2026-10-07"); err != nil {
		t.Fatal(err)
	}
	verdict := judgeOne(t, tree)
	if len(verdict.Refusals) > 0 {
		t.Fatalf("refusals after a recorded green run: %v", verdict.Refusals)
	}
	if verdict.Ceiling != LevelSupported {
		t.Fatalf("ceiling %s, want supported; unmet %v", verdict.Ceiling, verdict.Unmet)
	}
}

// TestRecordRunRefusesWhatItDidNotObserve: a failing item, a run whose output
// does not show the item passing (a -run pattern that selected nothing exits
// 0), and a runner error each write nothing.
func TestRecordRunRefusesWhatItDidNotObserve(t *testing.T) {
	cases := []struct {
		name string
		run  runItem
		want string
	}{
		{"failing item", fakeRunner(passOutputs, "test/plugin/widget.ci"), "the run failed"},
		{"go test selected nothing", fakeRunner(map[string]string{
			"test/plugin/widget.ci":                      passOutputs["test/plugin/widget.ci"],
			"internal/widget/widget_test.go::TestWidget": "testing: warning: no tests to run\nPASS\n",
		}), "no '--- PASS: TestWidget' line"},
		{"ci output names another test", fakeRunner(map[string]string{
			"test/plugin/widget.ci": "6.7s     1/1  PASS  2  other\n",
		}), "no PASS line for widget"},
		{"runner error", func(string) (observation, error) {
			return observation{}, errors.New("did not finish")
		}, "did not finish"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree := recordedTree(t)
			_, err := recordRun(tree, "widget", tc.run, "abc", "2026-10-07")
			if err == nil {
				t.Fatal("recorded a run it did not observe passing")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q, want it to contain %q", err, tc.want)
			}
			requireNoRecord(t, tree)
		})
	}
}

// TestRecordRunRefusesAFileChangedMidRun: a pass belongs to the content that
// ran, so a file edited during the run records nothing.
func TestRecordRunRefusesAFileChangedMidRun(t *testing.T) {
	tree := recordedTree(t)
	run := func(item string) (observation, error) {
		writeFile(t, tree, "test/plugin/widget.ci", "cmd=foreground:seq=1:exec=ze edited\n")
		return fakeRunner(passOutputs)(item)
	}
	_, err := recordRun(tree, "widget", run, "abc", "2026-10-07")
	if err == nil || !strings.Contains(err.Error(), "changed while it ran") {
		t.Fatalf("error %v, want a changed-while-it-ran refusal", err)
	}
	requireNoRecord(t, tree)
}

// TestCheckRefusesACIOutsideTheFunctionalRunner is A-7: a .ci in a directory no
// functional suite walks is refused even though the file exists.
func TestCheckRefusesACIOutsideTheFunctionalRunner(t *testing.T) {
	tree := fixtureTree(t, func(text string) string {
		return strings.ReplaceAll(text, "test/plugin/widget.ci", "test/nosuch/widget.ci")
	})
	writeFile(t, tree, "test/nosuch/widget.ci", "cmd=foreground:seq=1:exec=ze\n")
	verdict := judgeOne(t, tree)
	requireRefused(t, &verdict, "runs test/nosuch/")
}
