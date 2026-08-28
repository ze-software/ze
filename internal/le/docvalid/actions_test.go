// The command surface's tests: the action table, what a bare call answers, how
// a wrong word is refused, and the shape the engine was told to expect.

package docvalid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/leroot"
)

// VALIDATES: each native action has one explicit unique verb and implementation.
func TestActionTableDeclaresThreeNativeVerbs(t *testing.T) {
	want := map[string]bool{
		"command-contract":      true,
		"doc-drift":             true,
		"pipe-operators-update": true,
	}
	if len(actions) != len(want) {
		t.Fatalf("the table holds %d actions", len(actions))
	}
	for _, action := range actions {
		if !want[action.verb] {
			t.Errorf("the table holds an unknown action: %s", action.verb)
		}
		if action.why == "" || action.answer == nil {
			t.Errorf("action %s is incomplete", action.verb)
		}
	}
}

// VALIDATES: exactly one action is marked as writing, and it is the one that
// rewrites the generated table.
// PREVENTS: a developer running a writer from a listing that calls it a check.
func TestOnlyTheGeneratorIsMarkedAsWriting(t *testing.T) {
	writers := make([]string, 0, 1)
	for _, row := range Actions().Actions {
		if row.Writes {
			writers = append(writers, row.Verb)
		}
	}
	if len(writers) != 1 || writers[0] != "pipe-operators-update" {
		t.Fatalf("the writing actions are %v", writers)
	}
}

// VALIDATES: the bare command answers the action listing, marked.
// PREVENTS: the listing losing the marker a reader picks an action by.
func TestBareCommandAnswersTheListing(t *testing.T) {
	payload, code := Answer(nil)
	if code != 0 {
		t.Fatalf("the listing exited %d", code)
	}
	list, ok := payload.(ActionList)
	if !ok {
		t.Fatalf("the listing answered %T", payload)
	}
	if list.Area != area || len(list.Actions) != len(actions) {
		t.Fatalf("the listing answered %d actions of %q", len(list.Actions), list.Area)
	}

	text := list.Text()
	for _, want := range []string{"docvalid:", "doc-drift", "checks", "writes"} {
		if !strings.Contains(text, want) {
			t.Errorf("the listing does not hold %q:\n%s", want, text)
		}
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "pipe-operators-update ") && !strings.Contains(line, "writes") {
			t.Errorf("the generator is not marked as writing: %q", line)
		}
	}
}

// VALIDATES: help's one-line hint is derived from the action table.
// PREVENTS: a hand-written Subs line disagreeing with the listing about which
// action writes.
func TestHelpAndTheListingAgreeAboutWhatWrites(t *testing.T) {
	subs := Subs()
	for _, row := range Actions().Actions {
		if !strings.Contains(subs, row.Verb) {
			t.Errorf("help does not name %q: %q", row.Verb, subs)
		}
		if row.Writes && !strings.Contains(subs, row.Verb+" (writes)") { //nolint:goconst // the marker is spelled once beside the verb it belongs to
			t.Errorf("help does not mark %q as writing: %q", row.Verb, subs)
		}
	}
}

// VALIDATES: malformed command input answers the usage status 2.
func TestRefusalsAnswerUsageCode(t *testing.T) {
	if _, code := Answer([]string{"no-such-action"}); code != 2 {
		t.Errorf("an unknown action answered %d, want 2", code)
	}
	if _, code := Answer([]string{"doc-drift", "extra"}); code != 2 {
		t.Errorf("a value after an action answered %d, want 2", code)
	}
	if payload, _ := Answer([]string{"no-such-action"}); payload != nil {
		t.Errorf("a refusal answered a payload: %v", payload)
	}
}

// pointAtFixture makes lepath.Root() answer dir for the rest of this test.
//
// env.Get reads os.Environ() once per process and caches it, so a t.Setenv
// alone leaves lepath.Root() answering the real checkout (the cost of learning
// this the other way is three whole-tree walks under -race).
func pointAtFixture(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", dir)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// VALIDATES: each gate's action answers the gate's own verdict as its exit
// code, over the tree ZE_REPO_ROOT names.
// PREVENTS: a gate that reports its findings and exits 0, which is a gate that
// blocks nothing. Nothing else reaches these three functions: the package's
// other tests call Drift and Validate directly, so the mapping from a report to
// an exit code would otherwise be unproven.
func TestEachActionAnswersItsOwnVerdict(t *testing.T) {
	t.Run("drift answers 1 over a tree that drifts", func(t *testing.T) {
		root := t.TempDir()
		writeDoc(t, root, "docs/architecture/api/text-parser.md",
			"# Text Parser\n\nAll functions allocate via `strings.Fields()`.\n")
		pointAtFixture(t, root)

		payload, code := Answer([]string{"doc-drift"})
		if code != 1 {
			t.Fatalf("a drifting tree answered %d", code)
		}
		report, ok := payload.(DriftReport)
		if !ok || len(report.Issues) == 0 {
			t.Fatalf("the action answered %T with no finding", payload)
		}
	})

	t.Run("drift answers 0 over a tree that claims nothing", func(t *testing.T) {
		pointAtFixture(t, t.TempDir())

		payload, code := Answer([]string{"doc-drift"})
		if code != 0 {
			t.Fatalf("a tree with no document answered %d: %v", code, payload)
		}
	})

	t.Run("the contract answers 1 over a tree that registers nothing", func(t *testing.T) {
		root := t.TempDir()
		writeDoc(t, root, "cmd/ze/main.go", "package main\n\nfunc main() {}\n")
		pointAtFixture(t, root)

		payload, code := Answer([]string{"command-contract"})
		if code != 1 {
			t.Fatalf("a tree whose commands have no local handler answered %d", code)
		}
		result, ok := payload.(ValidationResult)
		if !ok || len(result.OrphanYANG) == 0 {
			t.Fatalf("the action answered %T with no orphan", payload)
		}
	})

	t.Run("the generator writes the table and names it", func(t *testing.T) {
		root := t.TempDir()
		writeDoc(t, root, pipeOperatorReferencePath, "# stale\n")
		pointAtFixture(t, root)

		payload, code := Answer([]string{"pipe-operators-update"})
		if code != 0 {
			t.Fatalf("the generator answered %d", code)
		}
		report, ok := payload.(WriteReport)
		if !ok || report.Path != pipeOperatorReferencePath {
			t.Fatalf("the generator answered %v", payload)
		}
		body, err := os.ReadFile(filepath.Join(root, pipeOperatorReferencePath))
		if err != nil {
			t.Fatalf("read the written table: %v", err)
		}
		if strings.Contains(string(body), "stale") {
			t.Fatal("the generator did not overwrite the stale table")
		}
	})
}

// VALIDATES: the gate's verdict reads BOTH orphan directions.
// PREVENTS: a handler nobody declared in YANG passing as a satisfied contract.
// That half cannot be reached from a fixture tree, because the handlers come
// from this process's registry (contract.go, contractSatisfied).
func TestTheVerdictReadsBothDirections(t *testing.T) {
	orphanNode := []CommandEntry{{WireMethod: "a:b"}}
	for _, tc := range []struct {
		name     string
		yang     []CommandEntry
		handlers []string
		want     bool
	}{
		{name: "nothing orphaned", want: true},
		{name: "a node with no handler", yang: orphanNode},
		{name: "a handler with no node", handlers: []string{"c:d"}},
		{name: "one of each", yang: orphanNode, handlers: []string{"c:d"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := contractSatisfied(tc.yang, tc.handlers); got != tc.want {
				t.Fatalf("the verdict is %v, want %v", got, tc.want)
			}
		})
	}
}

// VALIDATES: le owns this command, and the engine holds the shape it declared.
// PREVENTS: a tool that registered nothing, and a `| count` that walks the
// whole tree before the engine refuses it.
func TestTheCommandIsRegisteredWithItsShape(t *testing.T) {
	if !leroot.Owns(area) {
		t.Fatalf("le does not own %q, so nothing dispatches to it", area)
	}
	shape, declared := command.ShapeForCommand(leroot.CommandPath(area))
	if !declared {
		t.Fatal("the command declared no answer shape")
	}
	// ShapeDoc, because the contract answer carries several row sets: the
	// commands, the handlers, the local handlers and three orphan lists.
	if shape != command.ShapeDoc {
		t.Fatalf("the command declared the shape %v, want ShapeDoc", shape)
	}
}
