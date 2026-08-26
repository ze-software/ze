package changed

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: `le changed packages` widens to ./... on EVERY route it cannot
// answer. Thus, a scoped stage never covers nothing while it reports success.
// PREVENTS: The regression measured in scripts/dev/changed-pkgs.sh on
// 2026-08-26. ZE_VERIFY_SCOPE_PACKAGES names a readable path that is not a file.
// Then `cat` fails, but the script ignores the exit status. The recipe receives
// an empty package list with exit 0. The script's header requires every failure
// route to widen, but this route does not.

func TestAScopeFileThatCannotBeReadWidensToEveryPackage(t *testing.T) {
	// A DIRECTORY: the shell's `[ -r ]` test passes for it and `cat` then
	// fails, which is the exact shape the script answers nothing for.
	report := Scope{Root: t.TempDir(), File: t.TempDir()}.Resolve(nil)

	if !report.Widened {
		t.Fatalf("an unreadable scope file did not widen: %+v", report)
	}
	if len(report.Packages) != 1 || report.Packages[0] != everyPackage {
		t.Errorf("packages are %v, want exactly [%s]", report.Packages, everyPackage)
	}
	if report.Reason == "" {
		t.Error("a widening carries no reason, so a reader cannot tell why the run grew")
	}
}

func TestAMissingScopeFileWidensToEveryPackage(t *testing.T) {
	report := Scope{Root: t.TempDir(), File: filepath.Join(t.TempDir(), "absent")}.Resolve(nil)
	if !report.Widened {
		t.Fatalf("a missing scope file did not widen: %+v", report)
	}
}

func TestAScopeFileIsReadWhenNoArgumentAsksADifferentQuestion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "scope.txt", "./internal/core/env\n./cmd/ze\n")
	rec := &recorder{}

	report := Scope{Root: root, File: filepath.Join(root, "scope.txt"), Run: rec.run}.Resolve(nil)

	if report.Widened {
		t.Fatalf("a readable scope file widened: %+v", report)
	}
	if len(rec.calls) != 0 {
		t.Errorf("%d commands ran, want 0: the precomputed answer was published for exactly this", len(rec.calls))
	}
	want := []string{"./internal/core/env", "./cmd/ze"}
	if len(report.Packages) != len(want) {
		t.Fatalf("%d packages, want exactly %d: %v", len(report.Packages), len(want), report.Packages)
	}
	for i, pkg := range want {
		if report.Packages[i] != pkg {
			t.Errorf("package %d is %q, want %q", i, report.Packages[i], pkg)
		}
	}
}

// An argument asks a different question than the one the run precomputed, so
// the precomputed answer must not be handed back for it.
func TestAnArgumentBypassesThePrecomputedAnswer(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "scope.txt", "./internal/core/env\n")
	rec := &recorder{answers: map[string]string{selectorCall(root, "--depth=1"): "./cmd/ze\n"}}

	report := Scope{Root: root, File: filepath.Join(root, "scope.txt"), Run: rec.run}.Resolve([]string{"--depth=1"})

	if len(rec.calls) != 1 {
		t.Fatalf("%d commands ran, want exactly 1 (the selector)", len(rec.calls))
	}
	if len(report.Packages) != 1 || report.Packages[0] != "./cmd/ze" {
		t.Errorf("packages are %v, want exactly [./cmd/ze]", report.Packages)
	}
}

func TestASelectorFailureWidensToEveryPackage(t *testing.T) {
	root := t.TempDir()
	rec := &recorder{fail: map[string]error{selectorCall(root): errors.New("selector: exit 1")}}

	report := Scope{Root: root, Run: rec.run}.Resolve(nil)

	if !report.Widened {
		t.Fatalf("a failing selector did not widen: %+v", report)
	}
	if len(report.Packages) != 1 || report.Packages[0] != everyPackage {
		t.Errorf("packages are %v, want exactly [%s]", report.Packages, everyPackage)
	}
}

// An empty answer is an ANSWER: no changed path is compiled by a Go package.
// Widening for it would make every non-Go commit verify the whole tree.
func TestAnEmptySelectorAnswerIsAnAnswerRatherThanAWidening(t *testing.T) {
	root := t.TempDir()
	rec := &recorder{answers: map[string]string{selectorCall(root): "\n"}}

	report := Scope{Root: root, Run: rec.run}.Resolve(nil)

	if report.Widened {
		t.Fatalf("an empty selector answer widened: %+v", report)
	}
	if len(report.Packages) != 0 {
		t.Errorf("packages are %v, want none", report.Packages)
	}
}

func TestTheScopeReportRendersOnePackagePerLine(t *testing.T) {
	report := ScopeReport{Packages: []string{"./cmd/ze", "./internal/core/env"}}
	if text := report.Text(); text != "./cmd/ze\n./internal/core/env\n" {
		t.Errorf("scope rendering is %q", text)
	}
	if text := (ScopeReport{}).Text(); text != "" {
		t.Errorf("an empty scope renders %q, want the empty string", text)
	}
}

// The selector remains a script that this repository carries. Thus, the command
// line must name the script exactly. With a typo, the selector does not answer.
// Every run then silently widens to the whole tree.
func TestTheSelectorIsNamedByAnAbsolutePathOfTheCheckout(t *testing.T) {
	root := t.TempDir()
	rec := &recorder{}
	Scope{Root: root, Run: rec.run}.Resolve(nil)

	if len(rec.calls) != 1 {
		t.Fatalf("%d commands ran, want exactly 1", len(rec.calls))
	}
	joined := strings.Join(rec.calls[0], " ")
	want := filepath.Join(root, selectorPath)
	if !strings.Contains(joined, want) {
		t.Errorf("selector command %q does not name %q", joined, want)
	}
}

// selectorCall spells the command the scope resolution runs, for the recorder's
// table.
func selectorCall(root string, extra ...string) string {
	argv := selectorArgv(root, extra)
	return strings.Join(argv, " ")
}
