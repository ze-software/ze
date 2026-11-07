// Design: docs/contributing/dependency-approval.md -- the owner-only approval of one use
// Overview: deps.go -- the ledger this writes
// Related: ../le/human/human.go -- RequireAuthorization, the terminal gate every owner action shares

package deps

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/le/human"
)

// Approval is what `deps approve` reports once it has written.
type Approval struct {
	Row      Row    `json:"row"`
	Ledger   string `json:"ledger"`
	Approved string `json:"approval-record"`
}

// admit refuses the approval unless Thomas's own terminal runs it, then
// refuses a row the ledger could not hold. It MUST run before any read or
// write of the ledger, so a refused call leaves no trace.
func admit(root string, row Row) error {
	operands := []string{keyPackage, row.Package, keyImporter, row.Importer, keyReason, row.Reason}
	if err := human.RequireAuthorization(root, "", area+" approve", operands); err != nil {
		return err
	}
	return validApproval(root, row)
}

// validApproval refuses a row that is malformed or names a package the gate
// would never ask about: the standard library and Ze's own packages need no
// approval, so a row for one would be dead.
func validApproval(root string, row Row) error {
	if err := ValidRow(row); err != nil {
		return err
	}
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // the checkout root this command was pointed at
	if err != nil {
		return err
	}
	module, err := ModulePath(string(goMod))
	if err != nil {
		return err
	}
	if !ThirdParty(module, row.Package) {
		return errors.New(row.Package + " is not a third-party package (the standard library and " + module + " need no approval)")
	}
	return nil
}

// Approve writes row into the ledger, replacing the reason of the same
// package and importer, and appends it to the approval record the commit gate
// reads. The caller MUST have admitted the row (admit) first.
func Approve(root string, row Row) (Approval, error) {
	ledgerPath := filepath.Join(root, LedgerRel)
	text, err := os.ReadFile(ledgerPath) //nolint:gosec // a checkout-relative constant path
	if err != nil {
		return Approval{}, errors.New("read the ledger " + LedgerRel + ": " + err.Error())
	}
	ledger, err := Parse(string(text))
	if err != nil {
		return Approval{}, err
	}
	if err := writeAtomically(ledgerPath, ledger.With(row).Render()); err != nil {
		return Approval{}, err
	}
	if err := appendRecord(filepath.Join(root, ApprovalsRel), RowLine(row)); err != nil {
		return Approval{}, err
	}
	return Approval{Row: row, Ledger: LedgerRel, Approved: ApprovalsRel}, nil
}

// writeAtomically replaces file with content through a rename, so a reader
// never meets half a ledger.
func writeAtomically(file, content string) error {
	temporary, err := os.CreateTemp(filepath.Dir(file), ".dependency-ledger-*")
	if err != nil {
		return err
	}

	name := temporary.Name()
	defer os.Remove(name) //nolint:errcheck // absent once renamed; a failed write leaves only this temporary file to clear

	if _, err := temporary.WriteString(content); err != nil {
		temporary.Close() //nolint:errcheck,gosec // the write error is the one reported
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil { //nolint:gosec // a tracked file every checkout reads
		return err
	}
	return os.Rename(name, file)
}

// appendRecord adds one line to the approval record, creating tmp/ and the
// file when absent.
func appendRecord(file, line string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return err
	}
	record, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // a checkout-relative constant path
	if err != nil {
		return err
	}
	if _, err := record.WriteString(line + "\n"); err != nil {
		record.Close() //nolint:errcheck,gosec // the write error is the one reported
		return err
	}
	return record.Close()
}

// Recorded answers whether the approval record holds row exactly. An absent
// record holds nothing; an unreadable one is an error, never "not recorded".
func Recorded(root string, row Row) (bool, error) {
	text, err := os.ReadFile(filepath.Join(root, ApprovalsRel)) //nolint:gosec // a checkout-relative constant path
	// refuse-by-default: guards Approve -- an absent record is the state before the first Approve, which recorded no row
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	want := RowLine(row)
	for _, line := range splitLines(string(text)) {
		if line == want {
			return true, nil
		}
	}
	return false, nil
}
