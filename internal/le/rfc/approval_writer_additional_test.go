//go:build linux || darwin

// Design: docs/contributing/committing.md -- approval transactions preserve pending verdicts
// Related: approve.go -- real upsert; approval storage -- shared lock and exact prune
package rfc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

// Goal: a transaction retains the same process lock when the ledger inode is
// replaced, then releases it after either a successful callback or its error.
// Method: perform the existing real sibling-stage/rename operation while a
// distinct OS-lock handle probes the stable sidecar before and after rename.
func TestApprovalLedgerLockSpansReplacementAndReleases(t *testing.T) {
	callbackFailure := errors.New("deliberate approval callback failure")
	for _, question := range []struct {
		name    string
		failure error
	}{
		{name: "successful replacement"},
		{name: "failed callback", failure: callbackFailure},
	} {
		t.Run(question.name, func(t *testing.T) {
			full := filepath.Join(t.TempDir(), "pending.md")
			before := "the original pending rows\n"
			if err := os.WriteFile(full, []byte(before), 0o600); err != nil {
				t.Fatal(err)
			}
			var originalLock os.FileInfo
			err := withApprovalLedgerLock(full, func() error {
				if err := approvalRaceProbeLock(full, false); err != nil {
					return err
				}
				var err error
				originalLock, err = os.Stat(approvalLedgerLockPath(full))
				if err != nil {
					return err
				}
				if question.failure != nil {
					return question.failure
				}
				ledgerBefore, err := os.Stat(full)
				if err != nil {
					return err
				}
				if err := replaceFileAtomically(full, []byte("the replacement pending rows\n")); err != nil {
					return err
				}
				ledgerAfter, err := os.Stat(full)
				if err != nil {
					return err
				}
				if os.SameFile(ledgerBefore, ledgerAfter) {
					return errors.New("the control did not replace the actual ledger inode")
				}
				lockAfter, err := os.Stat(approvalLedgerLockPath(full))
				if err != nil {
					return err
				}
				if !os.SameFile(originalLock, lockAfter) {
					return errors.New("ledger replacement changed the stable process-lock inode")
				}
				return approvalRaceProbeLock(full, false)
			})
			if question.failure == nil {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if !errors.Is(err, question.failure) {
					t.Fatalf("transaction error = %v, want original callback error", err)
				}
				body, readErr := os.ReadFile(full)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if string(body) != before {
					t.Fatalf("callback refusal changed the ledger: %q", body)
				}
			}
			retainedLock, statErr := os.Stat(approvalLedgerLockPath(full))
			if statErr != nil {
				t.Fatalf("transaction removed its stable process-lock path: %v", statErr)
			}
			if !os.SameFile(originalLock, retainedLock) {
				t.Fatal("transaction replaced its stable process-lock inode after returning")
			}
			if err := approvalRaceProbeLock(full, true); err != nil {
				t.Fatalf("transaction retained its process lock after returning: %v", err)
			}
		})
	}
}

// Goal: both actual mutation APIs refuse a failed process-lock acquisition
// before changing any pending row. Method: their exact sidecar name is an
// owned directory, so opening it as a lock file must fail deterministically.
// Bypassing the transaction in either API makes that case succeed incorrectly.
func TestApprovalWritersRefuseUnavailableProcessLock(t *testing.T) {
	for _, writer := range []string{"approve", "prune"} {
		t.Run(writer, func(t *testing.T) {
			root := approvalWriterFixture(t)
			before := readApprovals(t, root)
			full := filepath.Join(root, filepath.FromSlash(ApprovalPath(approvalSession)))
			lockPath := approvalLedgerLockPath(full)
			// Only this fresh fixture can own the sidecar; no writer is active.
			if err := os.Remove(lockPath); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(lockPath, 0o700); err != nil {
				t.Fatal(err)
			}
			var err error
			if writer == "approve" {
				_, err = Approve(root, approvalSession, "pkg.TestX", "must refuse unavailable process lock")
			} else {
				err = PruneApprovals(root, approvalRaceFile(), []string{approvalRaceRow(t, before, "pkg.TestSeed")})
			}
			if err == nil {
				t.Fatalf("actual %s ignored its unavailable process lock", writer)
			}
			if after := readApprovals(t, root); after != before {
				t.Fatalf("refused %s changed earlier pending rows:\nbefore:\n%s\nafter:\n%s", writer, before, after)
			}
		})
	}
}

// Goal: post-publication cleanup does not create a ledger that never existed.
// Method: prune a captured row in a fresh fixture directory and then check the
// ledger itself, allowing a retained stable sidecar but no invented ledger.
func TestPruneApprovalsLeavesMissingLedgerMissing(t *testing.T) {
	root := t.TempDir()
	file := approvalRaceFile()
	full := filepath.Join(root, filepath.FromSlash(file.Path))
	if err := PruneApprovals(root, file, []string{"| pkg.TestX | captured old row |  | none..none |"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(full); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing ledger after prune: %v, want it still missing", err)
	}
}

// Goal: spending an exact older row cannot erase a fresh verdict for the same
// unit. Method: capture the actual old row, renew through real Approve, prune
// only the captured text, and finally spend the exact fresh row as a control.
func TestPruneApprovalsKeepsRenewedSameUnitVerdict(t *testing.T) {
	root := approvalWriterFixture(t)
	if _, err := Approve(root, approvalSession, "pkg.TestX", "older captured verdict"); err != nil {
		t.Fatal(err)
	}
	old := approvalRaceRow(t, readApprovals(t, root), "pkg.TestX")
	if _, err := Approve(root, approvalSession, "pkg.TestX", "fresh independently renewed verdict"); err != nil {
		t.Fatal(err)
	}
	if _, err := Approve(root, approvalSession, "pkg.TestY", "unrelated pending verdict"); err != nil {
		t.Fatal(err)
	}
	before := readApprovals(t, root)
	fresh := approvalRaceRow(t, before, "pkg.TestX")
	if fresh == old {
		t.Fatal("renewal control did not produce a distinct exact row")
	}
	if err := PruneApprovals(root, approvalRaceFile(), []string{old}); err != nil {
		t.Fatal(err)
	}
	if after := readApprovals(t, root); after != before {
		t.Fatalf("old consumed row changed renewed or unrelated verdicts:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if err := PruneApprovals(root, approvalRaceFile(), []string{fresh}); err != nil {
		t.Fatal(err)
	}
	after := readApprovals(t, root)
	if strings.Contains(after, "| pkg.TestX |") {
		t.Fatalf("exact fresh consumed row survived its actual prune:\n%s", after)
	}
	for _, unit := range []string{"pkg.TestSeed", "pkg.TestY"} {
		if count := strings.Count(after, "| "+unit+" |"); count != 1 {
			t.Fatalf("pruning TestX left %d rows for unrelated %s:\n%s", count, unit, after)
		}
	}
	if !strings.Contains(after, ApprovalTableHeader) {
		t.Fatalf("exact-row prune lost the ledger header:\n%s", after)
	}
}

func approvalRaceFile() ApprovalFile {
	return ApprovalFile{Path: ApprovalPath(approvalSession), Header: approvalHeader, Command: "rfc approve"}
}

func approvalRaceRow(t *testing.T, content, unit string) string {
	t.Helper()
	var row string
	for line := range strings.SplitSeq(content, "\n") {
		if !strings.HasPrefix(line, "| "+unit+" |") {
			continue
		}
		if row != "" {
			t.Fatalf("fixture has duplicate rows for %s", unit)
		}
		row = line
	}
	if row == "" {
		t.Fatalf("fixture has no actual approval row for %s", unit)
	}
	return row
}

func approvalRaceProbeLock(full string, wantAcquired bool) (result error) {
	other := flock.New(approvalLedgerLockPath(full), flock.SetPermissions(0o600))
	acquired, err := other.TryLock()
	if err != nil {
		return err
	}
	if acquired {
		defer func() { result = errors.Join(result, other.Unlock()) }()
	}
	if acquired != wantAcquired {
		return fmt.Errorf("independent process-lock handle acquired=%t, want %t", acquired, wantAcquired)
	}
	return nil
}
