//go:build ze_ssh

// Design: docs/guide/authentication.md -- bootstrap credential failures remain visible to operators.
// Related: main.go -- bootPowerUsers; main_servers.go -- canonical local credential reader.

package hub

import (
	"bytes"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/zefs"
)

const bootstrapCredentialCode = "doctor-local-admin-credentials"
const bootstrapCredentialSubject = "local-admin"

func bootstrapCredentialDoctor(t *testing.T) diagnostic.DoctorCheck {
	t.Helper()
	// The binary registers builtin explanations before dispatching doctor.
	diagnostic.RegisterBuiltinCodes()
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePreConfig) {
		if check.Name != "hub-local-admin-credentials" {
			continue
		}
		if !slices.Contains(check.Codes, bootstrapCredentialCode) {
			t.Fatal("local credential doctor check does not declare its diagnostic code")
		}
		meta := diagnostic.Lookup(bootstrapCredentialCode)
		if meta == nil {
			t.Fatal("local credential diagnostic code has no explanation metadata")
		}
		if meta.Title == "" {
			t.Fatal("local credential diagnostic has no operator explanation")
		}
		return check
	}
	t.Fatal("local credential doctor check is absent from the pre-config registry")
	return diagnostic.DoctorCheck{}
}

func TestBootstrapCredentialDoctorUsesCanonicalReader(t *testing.T) {
	check := bootstrapCredentialDoctor(t)
	for _, tc := range []struct {
		name           string
		keys           string
		disabled       bool
		removePassword bool
		broken         bool
	}{
		{name: "password-only"},
		{name: "comments-only", keys: "# no keys\n"},
		{name: "malformed-key", keys: "INVALID-PRIVATE-PAYLOAD\n", broken: true},
		{name: "missing-password", removePassword: true, broken: true},
		{name: "disabled-malformed-key", keys: "INVALID-PRIVATE-PAYLOAD\n", disabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := bootstrapStore(t, tc.keys, "", tc.disabled)
			if tc.removePassword {
				if err := store.RemoveKey(zefs.KeyLocalAdminPassword.Pattern); err != nil {
					t.Fatal(err)
				}
			}
			if tc.broken {
				if _, err := usersFromStore(store); err == nil {
					t.Fatal("fixture must reach a canonical credential refusal")
				}
			}
			diags := check.Check(diagnostic.DoctorCheckContext{Store: store})
			if !tc.broken {
				if len(diags) != 0 {
					t.Fatalf("valid or explicitly disabled credentials were reported broken: %+v", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics for refused local credentials, want one", len(diags))
			}
			if diags[0].Code != bootstrapCredentialCode {
				t.Fatalf("wrong credential diagnostic: %s", diags[0].Code)
			}
			if diags[0].Severity != diagnostic.SeverityWarning {
				t.Fatalf("credential refusal severity = %v, want warning because configured users can still log in", diags[0].Severity)
			}
			if diags[0].Fix == "" {
				t.Fatal("credential refusal has no repair command")
			}
			if strings.Contains(diags[0].Message, "INVALID-PRIVATE-PAYLOAD") {
				t.Fatal("credential diagnostic disclosed the refused input")
			}
		})
	}
}

func TestBootstrapCredentialFailureRaisesCodedWarningAndClears(t *testing.T) {
	report.ClearWarning("hub", bootstrapCredentialCode, bootstrapCredentialSubject)
	t.Cleanup(func() {
		report.ClearWarning("hub", bootstrapCredentialCode, bootstrapCredentialSubject)
	})
	store := bootstrapStore(t, "INVALID-PRIVATE-PAYLOAD\n", "", false)
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))
	for range 2 {
		if users := bootPowerUsers(store, log); len(users) != 0 {
			t.Fatal("refused bootstrap credentials entered the accepted user input")
		}
	}
	if !strings.Contains(logs.String(), "code="+bootstrapCredentialCode) {
		t.Error("bootstrap refusal warning has no registered doctor code")
	}
	if !strings.Contains(logs.String(), "fix=") {
		t.Error("bootstrap refusal warning has no repair command")
	}
	if strings.Contains(logs.String(), "INVALID-PRIVATE-PAYLOAD") {
		t.Error("bootstrap refusal warning disclosed key input")
	}
	active := 0
	for _, issue := range report.Warnings() {
		if issue.Source != "hub" {
			continue
		}
		if issue.Code != bootstrapCredentialCode {
			continue
		}
		if issue.Subject != bootstrapCredentialSubject {
			t.Errorf("credential warning is not owned by its stable subject: %s", issue.Subject)
			continue
		}
		active++
		if issue.Fix == "" {
			t.Error("active credential warning has no repair command")
		}
	}
	if active != 1 {
		t.Errorf("refused bootstrap credentials raised %d active warnings, want one deduplicated warning", active)
	}
	if err := store.RemoveKey(zefs.KeyLocalAdminAuthorizedKeys.Pattern); err != nil {
		t.Fatal(err)
	}
	if users := bootPowerUsers(store, log); len(users) != 1 {
		t.Fatal("repairing optional keys did not restore password recovery")
	}
	_, present, _ := report.SourceWarnings("hub", bootstrapCredentialCode, bootstrapCredentialSubject)
	if present {
		t.Fatal("repaired local credentials retained a stale active warning")
	}
}
