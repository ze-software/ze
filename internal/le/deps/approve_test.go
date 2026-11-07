// VALIDATES: deps approve refuses without Thomas's terminal before any write,
// and from a terminal writes the ledger row and the approval record the
// commit gate matches.
// PREVENTS: an agent issuing an approval, and an approval the gate cannot see.
package deps

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// approveFixture writes a checkout with go.mod and an empty ledger.
func approveFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/ze-software/ze\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, LedgerRel), []byte(Ledger{}.Render()), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestApproveRefusesWithoutTerminal pipes stdin and stdout: the owner gate
// refuses, and neither the ledger nor the record changes.
func TestApproveRefusesWithoutTerminal(t *testing.T) {
	root := approveFixture(t)
	humantest.Streams(t, "piped-both")
	before, err := os.ReadFile(filepath.Join(root, LedgerRel))
	if err != nil {
		t.Fatal(err)
	}
	err = admit(root, Row{Package: "github.com/cloudflare/circl/sign/ed448", Importer: "internal/core/ed448", Reason: "Ed448 AUTH"})
	if err == nil {
		t.Fatal("deps approve was admitted with no interactive terminal")
	}
	after, readErr := os.ReadFile(filepath.Join(root, LedgerRel))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("a refused approval changed the ledger")
	}
	if _, statErr := os.Stat(filepath.Join(root, ApprovalsRel)); !os.IsNotExist(statErr) {
		t.Fatalf("a refused approval wrote the record: %v", statErr)
	}
}

// TestApproveWritesRowAndRecord writes one row twice: the ledger holds it once
// with the second reason, and the record holds the exact row Recorded matches.
func TestApproveWritesRowAndRecord(t *testing.T) {
	root := approveFixture(t)
	row := Row{Package: "github.com/cloudflare/circl/sign/ed448", Importer: "internal/core/ed448", Reason: "Ed448 AUTH"}
	humantest.Streams(t, "terminal")
	if err := admit(root, row); err != nil {
		t.Fatalf("admit from a terminal: %v", err)
	}
	if _, err := Approve(root, Row{Package: row.Package, Importer: row.Importer, Reason: "first"}); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if _, err := Approve(root, row); err != nil {
		t.Fatalf("Approve again: %v", err)
	}
	text, err := os.ReadFile(filepath.Join(root, LedgerRel))
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := Parse(string(text))
	if err != nil {
		t.Fatalf("the written ledger does not parse: %v", err)
	}
	if rows := ledger.Rows(); len(rows) != 1 || rows[0] != row {
		t.Fatalf("ledger rows: got %+v, want one %+v", rows, row)
	}
	recorded, err := Recorded(root, row)
	if err != nil || !recorded {
		t.Fatalf("Recorded: %v, %v", recorded, err)
	}
	other := Row{Package: row.Package, Importer: "internal/component/ike/engine", Reason: row.Reason}
	if recorded, _ := Recorded(root, other); recorded {
		t.Fatal("an importer never approved reads as recorded")
	}
}

// TestValidApprovalRefusesFirstParty refuses a row the gate would never read.
func TestValidApprovalRefusesFirstParty(t *testing.T) {
	root := approveFixture(t)
	for _, pkg := range []string{"fmt", "github.com/ze-software/ze/internal/core/env"} {
		err := validApproval(root, Row{Package: pkg, Importer: "internal/x", Reason: "r"})
		if err == nil || !strings.Contains(err.Error(), "not a third-party package") {
			t.Errorf("%s: want a not-third-party refusal, got %v", pkg, err)
		}
	}
}
