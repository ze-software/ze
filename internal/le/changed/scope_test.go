package changed

import (
	"path/filepath"
	"testing"
)

// VALIDATES: `le changed packages` widens to ./... on EVERY route it cannot
// answer. Thus, a scoped stage never covers nothing while it reports success.
// PREVENTS: The regression measured in internal/le/changed/actions.go on
// 2026-08-26. ZE_VERIFY_SCOPE_PACKAGES names a readable path that is not a file.
// Then `cat` fails, but the script ignores the exit status. The recipe receives
// an empty package list with exit 0. The script's header requires every failure
// route to widen, but this route does not.

func TestAScopeFileThatCannotBeReadWidensToEveryPackage(t *testing.T) {
	// A DIRECTORY: the shell's `[ -r ]` test passes for it and `cat` then
	// fails, which is the exact shape the script answers nothing for.
	report, code := (Scope{Root: t.TempDir(), File: t.TempDir()}).Resolve(nil)
	if code != 0 {
		t.Fatalf("precomputed package widening exited %d", code)
	}

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
	report, code := (Scope{Root: t.TempDir(), File: filepath.Join(t.TempDir(), "absent")}).Resolve(nil)
	if code != 0 {
		t.Fatalf("missing precomputed package widening exited %d", code)
	}
	if !report.Widened {
		t.Fatalf("a missing scope file did not widen: %+v", report)
	}
}

func TestAScopeFileIsReadWhenNoArgumentAsksADifferentQuestion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "scope.txt", "./internal/core/env\n./cmd/ze\n")

	report, code := (Scope{Root: root, File: filepath.Join(root, "scope.txt")}).Resolve(nil)
	if code != 0 {
		t.Fatalf("precomputed package answer exited %d", code)
	}
	if report.Widened {
		t.Fatalf("a readable scope file widened: %+v", report)
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
	root := writeScopeFixture(t)
	writeFile(t, root, "scope.txt", "./precomputed\n")
	paths := scopePathsFile(t, "core/core.go")

	report, code := (Scope{
		Root: root,
		File: filepath.Join(root, "scope.txt"),
	}).Resolve([]string{"--depth=1", "--paths-from=" + paths})
	if code != 0 {
		t.Fatalf("native selector exited %d", code)
	}
	if len(report.Packages) != 2 || report.Packages[0] != "./core" || report.Packages[1] != "./mid" {
		t.Errorf("packages are %v, want [./core ./mid]", report.Packages)
	}
}

func TestTheScopeReportPreservesEveryPrintMode(t *testing.T) {
	report := ScopeReport{
		Packages: []string{"./cmd/ze", "./internal/core/env"},
		Tags:     []string{"ze_bgp", "ze_ssh"},
	}
	if text := report.Text(); text != "./cmd/ze\n./internal/core/env\n" {
		t.Errorf("default scope rendering is %q", text)
	}
	report.Print = "tags"
	if text := report.Text(); text != "ze_bgp\nze_ssh\n" {
		t.Errorf("tag rendering is %q", text)
	}
	report.Print = "both"
	want := "# packages\n./cmd/ze\n./internal/core/env\n# tags\nze_bgp\nze_ssh\n"
	if text := report.Text(); text != want {
		t.Errorf("both rendering is %q, want %q", text, want)
	}
}
