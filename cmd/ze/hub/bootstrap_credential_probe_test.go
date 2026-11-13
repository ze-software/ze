//go:build ze_ssh

// Design: docs/guide/authentication.md -- an unavailable credential probe is not a healthy account.
// Related: bootstrap_credential_diagnostic_test.go -- registered canonical-reader checks.

package hub

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestBootstrapCredentialUnavailableStoreIsReported(t *testing.T) {
	check := bootstrapCredentialDoctor(t)
	diags := check.Check(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("unavailable credential store returned %d diagnostics, want one unknown result", len(diags))
	}
	if diags[0].Code != bootstrapCredentialCode {
		t.Fatalf("unavailable store code = %s, want %s", diags[0].Code, bootstrapCredentialCode)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("unavailable store severity = %s, want warning", diags[0].Severity)
	}
	if diags[0].Message == "" {
		t.Fatal("unavailable probe returned no explanation")
	}
	if diags[0].Fix == "" {
		t.Fatal("unavailable probe returned no next command")
	}
}
