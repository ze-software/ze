// VALIDATES: a real-path test file counts exactly when a runner of the
// repository runs it, answered by each runner's own declaration (A-7), and
// record-run selects and observes it the way that runner does.
// PREVENTS: an .et, a `le test bgp` directory or a registered `le test <dir>`
// suite being refused although it runs, or a file nothing runs being accepted.

package feature

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// registeredSuiteDir is a `le test <dir>` command this test registers, standing
// in for a harness package such as internal/le/test/pppoe.
const registeredSuiteDir = "feature-fixture-suite"

func init() {
	leroot.Register(testTreeDir+" "+registeredSuiteDir, leroot.GroupSuite,
		func([]string) (any, int) { return nil, 0 }, registry.Meta{ShortHelp: "fixture suite", Description: "A suite command a feature test registers.",
			Mode: "offline", Section: registry.SectionTest})
}

// TestRunnerOfAnswersEveryRunnerKind asks runnerOf for one file of each runner
// kind and for files no runner walks.
func TestRunnerOfAnswersEveryRunnerKind(t *testing.T) {
	cases := []struct {
		rel      string
		words    []string
		selector string
		problem  string
	}{
		{rel: "test/plugin/widget.ci", words: []string{"bgp", "plugin"}, selector: "widget"},
		{rel: "test/editor/mode/widget.et", words: []string{"editor"}, selector: "test/editor/mode/widget.et"},
		{rel: "test/chaos-web/widget.ci", words: []string{"bgp", "chaos-web"}, selector: "widget"},
		{rel: "test/" + registeredSuiteDir + "/widget.ci", words: []string{registeredSuiteDir}, selector: "widget"},
		{rel: "test/plugin/widget.et", problem: "the editor runner walks only test/editor/"},
		{rel: "test/nosuch/widget.ci", problem: "runs test/nosuch/"},
		{rel: "test/plugin/deeper/widget.ci", problem: "which is where the .ci runners look"},
		{rel: "test/plugin/widget.txt", problem: "is neither a Go test"},
		{rel: "docs/widget.ci", problem: "is not under test/<dir>/"},
	}
	for _, c := range cases {
		runner, problem := runnerOf(c.rel)
		if c.problem != "" {
			if !strings.Contains(problem, c.problem) {
				t.Errorf("%s: problem %q, want one containing %q", c.rel, problem, c.problem)
			}
			continue
		}
		if problem != "" {
			t.Errorf("%s: refused: %s", c.rel, problem)
			continue
		}
		if !slices.Equal(runner.words, c.words) || runner.selector != c.selector {
			t.Errorf("%s: runner %v %q, want %v %q", c.rel, runner.words, runner.selector, c.words, c.selector)
		}
	}
}

// editorTree is the check fixture whose .ci item is replaced by an editor test,
// with its run record removed.
func editorTree(t *testing.T) string {
	t.Helper()
	tree := fixtureTree(t, func(text string) string {
		return strings.ReplaceAll(text, "test/plugin/widget.ci", editorItem)
	})
	writeFile(t, tree, editorItem, "input=type:text=set\n")
	if err := os.Remove(filepath.Join(tree, runRecordRel("widget"))); err != nil {
		t.Fatal(err)
	}
	return tree
}

const editorItem = "test/editor/mode/widget.et"

// TestCheckAcceptsAnEditorTest: an .et the editor runner walks is a real-path
// test, and record-run records it from the editor runner's PASS line, which
// names the file by its path.
func TestCheckAcceptsAnEditorTest(t *testing.T) {
	tree := editorTree(t)
	outputs := map[string]string{
		editorItem: "5.9s     1/1  PASS  100  " + editorItem + "\npass  1/1  100.0%  5.9s\n",
		"internal/widget/widget_test.go::TestWidget": passOutputs["internal/widget/widget_test.go::TestWidget"],
	}
	if _, err := recordRun(tree, "widget", runners{item: fakeRunner(outputs)}, "abc", "2026-10-07"); err != nil {
		t.Fatal(err)
	}
	verdict := judgeOne(t, tree)
	if len(verdict.Refusals) > 0 {
		t.Fatalf("refusals for an editor test: %v", verdict.Refusals)
	}
	if verdict.Ceiling != LevelSupported {
		t.Fatalf("ceiling %s, want supported; unmet %v", verdict.Ceiling, verdict.Unmet)
	}
}

// TestRecordRunRefusesAnEditorPassForAnotherFile: the editor runner's PASS line
// must name this file's path; the stem alone is another runner's selector.
func TestRecordRunRefusesAnEditorPassForAnotherFile(t *testing.T) {
	tree := editorTree(t)
	outputs := map[string]string{
		editorItem: "5.9s     1/1  PASS  100  widget\n",
		"internal/widget/widget_test.go::TestWidget": passOutputs["internal/widget/widget_test.go::TestWidget"],
	}
	_, err := recordRun(tree, "widget", runners{item: fakeRunner(outputs)}, "abc", "2026-10-07")
	if err == nil || !strings.Contains(err.Error(), "no PASS line for "+editorItem) {
		t.Fatalf("error %v, want a refusal naming %s", err, editorItem)
	}
	requireNoRecord(t, tree)
}
