// VALIDATES: dependencyRefusal refuses a commit that adds a third-party import
// the ledger does not approve for its directory, or a ledger row no approval
// recorded, and admits approved, kept and dropped imports.
// PREVENTS: a third-party module reaching a new use without Thomas's approval
// (ai/META-DECISIONS.md, "Code and dependencies").
package commit

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/deps"
)

// dependencyBaseline commits root module/vendor metadata and the named files
// over a fresh fixture checkout. Named metadata overrides those defaults.
func dependencyBaseline(t *testing.T, files map[string]string) string {
	t.Helper()
	root := newCommitRepository(t)
	writeCommitFixture(t, root, deps.VendorModulesRel, "")
	runCommitGit(t, root, "add", "--", "go.mod", deps.VendorModulesRel)
	for path, content := range files {
		writeCommitFixture(t, root, path, content)
		runCommitGit(t, root, "add", "--", path)
	}
	runCommitGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t",
		"-c", "commit.gpgsign=false", "commit", "-q", "-m", "dependency baseline")
	return root
}

const (
	dependencyImporting = "package x\n\nimport _ \"example.org/a\"\n"
	dependencyRow       = "example.org/a\tinternal/x\tapproved\n"
)

// TestDependencyRefusalRefusesUnapprovedImport is AC-1: the refusal names the
// package, the importer and the command Thomas runs.
func TestDependencyRefusalRefusesUnapprovedImport(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{deps.LedgerRel: ""})
	writeCommitFixture(t, root, "internal/x/x.go", dependencyImporting)
	err := dependencyRefusal(root, []string{"internal/x/x.go"}, nil, nil, "HEAD")
	if err == nil {
		t.Fatal("a commit adding an unapproved third-party import was admitted")
	}
	for _, want := range []string{
		"internal/x/x.go adds a third-party import",
		"package:  example.org/a",
		"importer: internal/x",
		"./le deps approve package example.org/a importer internal/x reason",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal does not say %q:\n%v", want, err)
		}
	}
	writeCommitFixture(t, root, "internal/y/y_test.go", "package y\n\nimport _ \"example.org/a\"\n")
	writeCommitFixture(t, root, deps.LedgerRel, dependencyRow)
	writeCommitFixture(t, root, deps.ApprovalsRel, dependencyRow)
	err = dependencyRefusal(root, []string{"internal/x/x.go", "internal/y/y_test.go", deps.LedgerRel}, nil, nil, "HEAD")
	if err == nil || !strings.Contains(err.Error(), "importer: internal/y") {
		t.Fatalf("a test file importing a package approved only for another directory was admitted: %v", err)
	}
	if strings.Contains(err.Error(), "importer: internal/x") {
		t.Fatalf("the approved importer was refused:\n%v", err)
	}
}

// TestDependencyRefusalAdmitsApprovedKeptAndDroppedImports is AC-2 and AC-3.
func TestDependencyRefusalAdmitsApprovedKeptAndDroppedImports(t *testing.T) {
	root := dependencyBaseline(t, map[string]string{
		deps.LedgerRel:    dependencyRow,
		"internal/k/k.go": "package k\n\nimport _ \"example.org/kept\"\n",
	})
	writeCommitFixture(t, root, "internal/x/x.go", dependencyImporting)
	if err := dependencyRefusal(root, []string{"internal/x/x.go"}, nil, nil, "HEAD"); err != nil {
		t.Fatalf("an approved import was refused:\n%v", err)
	}
	writeCommitFixture(t, root, "internal/k/k.go", "package k\n\nimport (\n\t\"fmt\"\n\t_ \"example.org/kept\"\n)\n\nvar _ = fmt.Sprint\n")
	if err := dependencyRefusal(root, []string{"internal/k/k.go"}, nil, nil, "HEAD"); err != nil {
		t.Fatalf("a kept import was judged as added:\n%v", err)
	}
	writeCommitFixture(t, root, "internal/k/k.go", "package k\n")
	if err := dependencyRefusal(root, []string{"internal/k/k.go"}, nil, nil, "HEAD"); err != nil {
		t.Fatalf("a dropped import was refused:\n%v", err)
	}
	writeCommitFixture(t, root, "vendor/example.org/v/v.go", "package v\n\nimport _ \"example.org/other\"\n")
	if err := dependencyRefusal(root, []string{"vendor/example.org/v/v.go"}, nil, nil, "HEAD"); err != nil {
		t.Fatalf("a vendored package's own import was judged:\n%v", err)
	}
}

// TestDependencyRefusalRefusesUnrecordedLedgerRow is AC-4 and AC-9: a row typed
// into the ledger is refused until the approval record holds it, the seed is
// admitted once, and removing the ledger is refused.
func TestDependencyRefusalRefusesUnrecordedLedgerRow(t *testing.T) {
	seed := dependencyBaseline(t, nil)
	writeCommitFixture(t, seed, deps.LedgerRel, dependencyRow)
	if err := dependencyRefusal(seed, []string{deps.LedgerRel}, nil, nil, "HEAD"); err != nil {
		t.Fatalf("the seed commit creating the ledger was refused:\n%v", err)
	}

	root := dependencyBaseline(t, map[string]string{deps.LedgerRel: ""})
	writeCommitFixture(t, root, deps.LedgerRel, dependencyRow)
	err := dependencyRefusal(root, []string{deps.LedgerRel}, nil, nil, "HEAD")
	if err == nil || !strings.Contains(err.Error(), "gains a row "+deps.ApprovalsRel+" does not hold") {
		t.Fatalf("a hand-written ledger row was admitted: %v", err)
	}
	writeCommitFixture(t, root, deps.ApprovalsRel, "example.org/a\tinternal/x\tanother reason\n")
	if err := dependencyRefusal(root, []string{deps.LedgerRel}, nil, nil, "HEAD"); err == nil {
		t.Fatal("a row whose reason differs from the recorded approval was admitted")
	}
	writeCommitFixture(t, root, deps.ApprovalsRel, dependencyRow)
	if err := dependencyRefusal(root, []string{deps.LedgerRel}, nil, nil, "HEAD"); err != nil {
		t.Fatalf("a recorded approval was refused:\n%v", err)
	}
	err = dependencyRefusal(root, nil, []string{deps.LedgerRel}, nil, "HEAD")
	if err == nil || !strings.Contains(err.Error(), "cannot be removed") {
		t.Fatalf("removing the ledger was admitted: %v", err)
	}
}
