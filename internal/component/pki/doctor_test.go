// VALIDATES: the doctor check over the local certificate authority root reports
// absence, an unreadable pair and an approaching expiry, each under its own
// code, and is registered with those codes (AC-11).
// PREVENTS: a root that quietly disappears or expires under a fleet that still
// trusts it, and a code an operator cannot look up because the central code
// table lost the entry.
package pki

import (
	"io/fs"
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/zefs"
)

// caRootDiagnostic runs the check over one store and returns the single
// diagnostic it reported, failing the test when the count is not one.
func caRootDiagnostic(t *testing.T, store storage.Storage) diagnostic.Diagnostic {
	t.Helper()

	diags := checkCARoot(diagnostic.DoctorCheckContext{Store: store})
	if len(diags) != 1 {
		t.Fatalf("check reported %d diagnostics, want 1: %v", len(diags), diags)
	}
	return diags[0]
}

// atRootAge runs the check with the clock moved to age past the root's
// NotBefore, so the boundary cases are reached without minting a certificate
// the production path would never issue.
func atRootAge(t *testing.T, store storage.Storage, remaining time.Duration) []diagnostic.Diagnostic {
	t.Helper()

	root, err := LoadOrGenerateRoot(store)
	if err != nil {
		t.Fatalf("LoadOrGenerateRoot: %v", err)
	}
	at := root.Certificate().NotAfter.Add(-remaining)
	caRootNow = func() time.Time { return at }
	t.Cleanup(func() { caRootNow = time.Now })

	return checkCARoot(diagnostic.DoctorCheckContext{Store: store})
}

func TestCARootDoctorCheck(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		store, _ := newRootStore(t)

		diag := caRootDiagnostic(t, store)
		if diag.Code != codeCARootMissing {
			t.Fatalf("code = %q, want %q", diag.Code, codeCARootMissing)
		}
		if diag.Severity != diagnostic.SeverityWarning {
			t.Fatalf("severity = %q, want warning", diag.Severity)
		}
	})

	t.Run("half written", func(t *testing.T) {
		store, _ := newRootStore(t)
		if _, err := LoadOrGenerateRoot(store); err != nil {
			t.Fatalf("LoadOrGenerateRoot: %v", err)
		}
		if err := store.Remove(zefs.KeyCAKey.Pattern); err != nil {
			t.Fatalf("remove the stored root key: %v", err)
		}

		diag := caRootDiagnostic(t, store)
		if diag.Code != codeCARootInvalid {
			t.Fatalf("code = %q, want %q", diag.Code, codeCARootInvalid)
		}
		if diag.Severity != diagnostic.SeverityError {
			t.Fatalf("severity = %q, want error", diag.Severity)
		}
	})

	t.Run("unreadable", func(t *testing.T) {
		store, _ := newRootStore(t)
		for _, key := range []string{zefs.KeyCACert.Pattern, zefs.KeyCAKey.Pattern} {
			if err := store.WriteFile(key, []byte("not a certificate"), fs.FileMode(0o600)); err != nil {
				t.Fatalf("write %s: %v", key, err)
			}
		}

		diag := caRootDiagnostic(t, store)
		if diag.Code != codeCARootInvalid {
			t.Fatalf("code = %q, want %q", diag.Code, codeCARootInvalid)
		}
		if diag.Severity != diagnostic.SeverityError {
			t.Fatalf("severity = %q, want error", diag.Severity)
		}
	})

	t.Run("healthy", func(t *testing.T) {
		store, _ := newRootStore(t)
		if _, err := LoadOrGenerateRoot(store); err != nil {
			t.Fatalf("LoadOrGenerateRoot: %v", err)
		}

		diags := checkCARoot(diagnostic.DoctorCheckContext{Store: store})
		if len(diags) != 0 {
			t.Fatalf("a fresh root reported %v", diags)
		}
	})

	t.Run("the last second before the warning window", func(t *testing.T) {
		store, _ := newRootStore(t)

		diags := atRootAge(t, store, caRootExpiryWarnWindow)
		if len(diags) != 0 {
			t.Fatalf("exactly %v remaining reported %v", caRootExpiryWarnWindow, diags)
		}
	})

	t.Run("inside the warning window", func(t *testing.T) {
		store, _ := newRootStore(t)

		diags := atRootAge(t, store, caRootExpiryWarnWindow-time.Second)
		if len(diags) != 1 {
			t.Fatalf("check reported %d diagnostics, want 1: %v", len(diags), diags)
		}
		if diags[0].Code != codeCARootExpiry {
			t.Fatalf("code = %q, want %q", diags[0].Code, codeCARootExpiry)
		}
		if diags[0].Severity != diagnostic.SeverityWarning {
			t.Fatalf("severity = %q, want warning", diags[0].Severity)
		}
	})

	t.Run("expired", func(t *testing.T) {
		store, _ := newRootStore(t)

		diags := atRootAge(t, store, -time.Second)
		if len(diags) != 1 {
			t.Fatalf("check reported %d diagnostics, want 1: %v", len(diags), diags)
		}
		if diags[0].Code != codeCARootExpiry {
			t.Fatalf("code = %q, want %q", diags[0].Code, codeCARootExpiry)
		}
		if diags[0].Severity != diagnostic.SeverityError {
			t.Fatalf("severity = %q, want error", diags[0].Severity)
		}
	})

	t.Run("no store", func(t *testing.T) {
		diag := caRootDiagnostic(t, nil)
		if diag.Code != codeCARootInvalid {
			t.Fatalf("code = %q, want %q", diag.Code, codeCARootInvalid)
		}
	})
}

func TestCARootDoctorCheckRegistered(t *testing.T) {
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(caRootDoctorCheck.Phase)
	for i := range checks {
		if checks[i].Name == caRootDoctorCheck.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", caRootDoctorCheck.Name, caRootDoctorCheck.Phase)
	}

	// The codes are declared in a central table many sessions edit at once, so
	// assert they are REGISTERED rather than merely spelled here.
	diagnostic.RegisterBuiltinCodes()
	for _, code := range []string{codeCARootMissing, codeCARootExpiry, codeCARootInvalid} {
		if diagnostic.Lookup(code) == nil {
			t.Errorf("diagnostic code %q is not registered", code)
		}
		if !slices.Contains(found.Codes, code) {
			t.Errorf("the check does not declare code %q", code)
		}
	}
}
