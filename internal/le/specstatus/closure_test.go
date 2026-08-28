package specstatus

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestClosureTurnsRedOnlyForCommittedExactEvidence(t *testing.T) {
	// VALIDATES: an in-progress non-umbrella spec with an exact tracked learned
	// summary is completed-not-closed, while the same on-disk summary without
	// index membership is green.
	// PREVENTS: an uncommitted closure draft wedging an active implementation.
	root := t.TempDir()
	path := filepath.Join(root, "plan", "spec-widget.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "| Field | Value |\n|-------|-------|\n| Status | in-progress |\n\n## Goal\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	learned := []learnedFile{{path: "plan/learned/900-widget.md", slug: "widget", tokens: []string{"widget"}}}
	green, err := inspectClosureSpec(root, path, learned, map[string]bool{}, nil, map[string]bool{"widget": true})
	if err != nil {
		t.Fatal(err)
	}
	if green.CompletedNotClosed {
		t.Fatalf("untracked evidence made closure red: %#v", green)
	}
	red, err := inspectClosureSpec(root, path, learned, map[string]bool{"plan/learned/900-widget.md": true}, nil, map[string]bool{"widget": true})
	if err != nil {
		t.Fatal(err)
	}
	if !red.CompletedNotClosed {
		t.Fatalf("tracked exact evidence = %#v, want red", red)
	}
	if red.Evidence != "plan/learned/900-widget.md" {
		t.Fatalf("tracked exact evidence = %#v", red)
	}
}

func TestJournalEvidenceNeedsAFinishedReviewGate(t *testing.T) {
	// VALIDATES: a committed journal row is closure evidence only after the
	// Review Gate has no open close checkbox.
	// PREVENTS: the first mid-work problem journal row blocking every stop.
	root := t.TempDir()
	path := filepath.Join(root, "plan", "spec-widget.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	base := "| Field | Value |\n|-------|-------|\n| Status | in-progress |\n\n## Goal\n"
	open := base + "\n## Review Gate\n- [ ] re-run shows 0 BLOCKER, 0 ISSUE\n"
	if err := os.WriteFile(path, []byte(open), 0o600); err != nil {
		t.Fatal(err)
	}
	evidence := map[string]string{"widget": "plan/journal/gate.md"}
	one, err := inspectClosureSpec(root, path, nil, nil, evidence, map[string]bool{"widget": true})
	if err != nil {
		t.Fatal(err)
	}
	if one.CompletedNotClosed {
		t.Fatal("unfinished review gate accepted journal evidence")
	}
	if err := os.WriteFile(path, []byte(base+"\n## Review Gate\n- [x] re-run shows 0 BLOCKER, 0 ISSUE\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	one, err = inspectClosureSpec(root, path, nil, nil, evidence, map[string]bool{"widget": true})
	if err != nil {
		t.Fatal(err)
	}
	if !one.CompletedNotClosed {
		t.Fatal("finished review gate did not accept journal evidence")
	}
}

func TestClosureConsumesNormalizedJournalEvidence(t *testing.T) {
	// VALIDATES: closure keys committed journal evidence by each canonical stem
	// supplied by the journal parser.
	// PREVENTS: a normalized closure signal disappearing during classification.
	evidence := map[string]string{
		"other-spec": "plan/journal/closure.md",
		"widget":     "plan/journal/closure.md",
	}

	root := t.TempDir()
	path := filepath.Join(root, "plan", "spec-widget.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	body := "| Field | Value |\n|-------|-------|\n| Status | in-progress |\n\n## Review Gate\n- [x] re-run shows 0 BLOCKER, 0 ISSUE\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	one, err := inspectClosureSpec(root, path, nil, nil, evidence, map[string]bool{"widget": true})
	if err != nil {
		t.Fatal(err)
	}
	if !one.CompletedNotClosed {
		t.Fatalf("normalized journal evidence = %#v, want completed-not-closed", one)
	}
	if one.JournalMatch != "plan/journal/closure.md" {
		t.Fatalf("normalized journal evidence = %#v", one)
	}
}

func TestMalformedGitIndexIsRefused(t *testing.T) {
	// VALIDATES: closure cannot report no committed evidence when the index is
	// malformed and therefore unreadable.
	// PREVENTS: a corrupt state source turning into a false green closure check.
	if _, err := readIndexPaths(bytes.NewBufferString("not an index")); err == nil {
		t.Fatal("readIndexPaths accepted malformed state")
	}
}

func TestSpecStatusCommandMapsAnAbsentClosureCheck(t *testing.T) {
	// VALIDATES: the grouped native status command maps the former --spec
	// closure check and preserves its clean answer for an already-removed spec.
	// PREVENTS: the Stop hook receiving a usage failure after native cutover.
	root := t.TempDir()
	action, spec := parseClosureAction([]string{"closure", "list"})
	if action != closureList || spec != "" {
		t.Fatalf("closure list mapping = (%d, %q)", action, spec)
	}
	action, spec = parseClosureAction([]string{"closure", "check", "spec", "removed"})
	if action != closureCheck || spec != "removed" {
		t.Fatalf("closure check mapping = (%d, %q)", action, spec)
	}
	setClosureAnswerRoot(t, root)
	payload, code := Answer([]string{"closure", "check", "spec", "removed"})
	report, ok := payload.(ClosureReport)
	if code != 0 {
		t.Fatalf("closure check = (%#v, %d)", payload, code)
	}
	if !ok {
		t.Fatalf("closure check payload = %T", payload)
	}
	if len(report) != 0 {
		t.Fatalf("closure check = %#v, want empty", report)
	}
	if _, code := Answer([]string{"closure", "check", "removed"}); code != 2 {
		t.Fatalf("closure check accepted a value without the spec selector, code %d", code)
	}
}

func setClosureAnswerRoot(t *testing.T, root string) {
	t.Helper()
	old, present := os.LookupEnv("ZE_REPO_ROOT")
	if err := os.Setenv("ZE_REPO_ROOT", root); err != nil {
		t.Fatal(err)
	}
	env.ResetCache()
	t.Cleanup(func() {
		var err error
		if present {
			err = os.Setenv("ZE_REPO_ROOT", old)
		} else {
			err = os.Unsetenv("ZE_REPO_ROOT")
		}
		if err != nil {
			t.Errorf("restore ZE_REPO_ROOT: %v", err)
		}
		env.ResetCache()
	})
}
