// Design: docs/guide/gnmi.md -- TLS: the transport serves TLS or nothing
// Related: doctor.go -- checkGNMITLSMaterial and its registration

package gnmi

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/selfcert"
)

// clearGNMIEnv empties the gNMI environment overrides for one test, so a value
// the developer's shell exports never decides which pair the check judges.
func clearGNMIEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ze.gnmi.enabled", "")
	t.Setenv("ze.gnmi.tls.cert", "")
	t.Setenv("ze.gnmi.tls.key", "")
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// gnmiTLSTree builds an environment gnmi block that asks for a listener when
// enabled is true, naming the given certificate and key paths. An empty path
// leaves its leaf unset.
func gnmiTLSTree(enabled bool, cert, key string) *zeconfig.Tree {
	tree := zeconfig.NewTree()
	block := tree.GetOrCreateContainer("environment").GetOrCreateContainer("gnmi")
	if enabled {
		block.Set("enabled", "true")
	}
	tls := block.GetOrCreateContainer("tls")
	if cert != "" {
		tls.Set("cert", cert)
	}
	if key != "" {
		tls.Set("key", key)
	}
	return tree
}

// runGNMITLSCheck runs the check over tree the way the doctor runner does.
func runGNMITLSCheck(tree *zeconfig.Tree) []diagnostic.Diagnostic {
	return checkGNMITLSMaterial(diagnostic.DoctorCheckContext{Tree: tree})
}

// TestGNMITLSMaterial drives the check over the pairs the hub hands Serve.
//
// VALIDATES: an unreadable certificate, a half pair and a certificate near
// expiry each give one doctor-gnmi-tls-material finding carrying the
// refusal's problem and its fix; the environment override is the pair judged;
// valid material, a block naming no TLS, and a gNMI that does not start give
// none.
// PREVENTS: gNMI refusing to start at boot over its TLS material with nothing
// in `ze doctor` naming the file or the fix (spec doctor-coverage-4 AC-10).
func TestGNMITLSMaterial(t *testing.T) {
	t.Run("unreadable certificate", func(t *testing.T) {
		clearGNMIEnv(t)
		dir := t.TempDir()
		_, keyFile := writeTestKeyPair(t, dir, "gnmi")
		missing := filepath.Join(dir, "absent.pem")

		diags := runGNMITLSCheck(gnmiTLSTree(true, missing, keyFile))

		_, want := loadTLSCredentials(missing, keyFile)
		requireOneGNMITLSFinding(t, diags, diagnostic.SeverityError)
		if !strings.Contains(diags[0].Message, missing) {
			t.Errorf("message %q does not name %s", diags[0].Message, missing)
		}
		if !strings.Contains(diags[0].Message, "cannot be read") {
			t.Errorf("message %q does not say the file cannot be read", diags[0].Message)
		}
		if !strings.Contains(diags[0].Help, "no such file") {
			t.Errorf("help %q does not carry the read error", diags[0].Help)
		}
		refusal, ok := errors.AsType[*opererr.Error](want)
		if !ok {
			t.Fatalf("loadTLSCredentials answered %v, want an operator error", want)
		}
		if diags[0].Fix != refusal.Fix() {
			t.Errorf("fix = %q, want the refusal's fix %q", diags[0].Fix, refusal.Fix())
		}
		if diags[0].Message != refusal.Problem() {
			t.Errorf("message = %q, want the refusal's problem %q", diags[0].Message, refusal.Problem())
		}
		if !strings.Contains(diags[0].Fix, "ze signal restart") {
			t.Errorf("fix %q does not name the restart", diags[0].Fix)
		}
	})

	t.Run("key without certificate", func(t *testing.T) {
		clearGNMIEnv(t)
		dir := t.TempDir()
		_, keyFile := writeTestKeyPair(t, dir, "gnmi")

		diags := runGNMITLSCheck(gnmiTLSTree(true, "", keyFile))

		requireOneGNMITLSFinding(t, diags, diagnostic.SeverityError)
		if !strings.Contains(diags[0].Message, "without a certificate file") {
			t.Errorf("message %q does not name the half pair", diags[0].Message)
		}
	})

	t.Run("environment override is the pair judged", func(t *testing.T) {
		clearGNMIEnv(t)
		dir := t.TempDir()
		certFile, keyFile := writeTestKeyPair(t, dir, "gnmi")
		missing := filepath.Join(dir, "override.pem")
		t.Setenv("ze.gnmi.tls.cert", missing)
		env.ResetCache()

		diags := runGNMITLSCheck(gnmiTLSTree(true, certFile, keyFile))

		requireOneGNMITLSFinding(t, diags, diagnostic.SeverityError)
		if !strings.Contains(diags[0].Message, missing) {
			t.Errorf("message %q does not name the override %s", diags[0].Message, missing)
		}
	})

	t.Run("certificate near expiry", func(t *testing.T) {
		clearGNMIEnv(t)
		dir := t.TempDir()
		certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1:0", nil, 24*time.Hour)
		if err != nil {
			t.Fatalf("generate certificate: %v", err)
		}
		certFile := filepath.Join(dir, "short.pem")
		keyFile := filepath.Join(dir, "short.key")
		if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
			t.Fatalf("write certificate: %v", err)
		}
		if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
			t.Fatalf("write key: %v", err)
		}

		diags := runGNMITLSCheck(gnmiTLSTree(true, certFile, keyFile))

		requireOneGNMITLSFinding(t, diags, diagnostic.SeverityWarning)
		if !strings.Contains(diags[0].Fix, certFile) {
			t.Errorf("fix %q does not name %s", diags[0].Fix, certFile)
		}
	})

	t.Run("valid material", func(t *testing.T) {
		clearGNMIEnv(t)
		certFile, keyFile := writeTestKeyPair(t, t.TempDir(), "gnmi")

		if diags := runGNMITLSCheck(gnmiTLSTree(true, certFile, keyFile)); len(diags) != 0 {
			t.Errorf("valid material: got %d findings, want none: %+v", len(diags), diags)
		}
	})

	t.Run("TLS not configured", func(t *testing.T) {
		clearGNMIEnv(t)

		if diags := runGNMITLSCheck(gnmiTLSTree(true, "", "")); len(diags) != 0 {
			t.Errorf("no TLS: got %d findings, want none: %+v", len(diags), diags)
		}
	})

	t.Run("gNMI does not start", func(t *testing.T) {
		clearGNMIEnv(t)
		missing := filepath.Join(t.TempDir(), "absent.pem")

		if diags := runGNMITLSCheck(gnmiTLSTree(false, missing, missing)); len(diags) != 0 {
			t.Errorf("dormant block: got %d findings, want none: %+v", len(diags), diags)
		}
	})

	t.Run("no tree", func(t *testing.T) {
		clearGNMIEnv(t)

		if diags := runGNMITLSCheck(nil); len(diags) != 0 {
			t.Errorf("nil tree: got %d findings, want none: %+v", len(diags), diags)
		}
	})
}

// TestGNMITLSMaterialCheckRegistered proves the check reaches the doctor
// registry from this package's init(), declaring the code it reports.
//
// VALIDATES: gnmi-tls-material is registered in the post-config phase and
// lists doctor-gnmi-tls-material in Codes, which the doctor-coverage gate
// reads to admit the hub's refusal log line.
// PREVENTS: a check that exists in source and never runs.
func TestGNMITLSMaterialCheckRegistered(t *testing.T) {
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "gnmi-tls-material" {
			continue
		}
		if !slices.Contains(check.Codes, diagnostic.CodeDoctorGNMITLSMaterial) {
			t.Errorf("gnmi-tls-material Codes = %v, want %s", check.Codes, diagnostic.CodeDoctorGNMITLSMaterial)
		}
		return
	}
	t.Fatal("gnmi-tls-material is not registered in the post-config phase")
}

// requireOneGNMITLSFinding fails unless diags is one finding under the gNMI
// TLS code at the given severity, with a fix.
func requireOneGNMITLSFinding(t *testing.T, diags []diagnostic.Diagnostic, severity diagnostic.Severity) {
	t.Helper()
	if len(diags) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != diagnostic.CodeDoctorGNMITLSMaterial {
		t.Errorf("code = %q, want %q", diags[0].Code, diagnostic.CodeDoctorGNMITLSMaterial)
	}
	if diags[0].Severity != severity {
		t.Errorf("severity = %v, want %v", diags[0].Severity, severity)
	}
	if diags[0].Fix == "" {
		t.Error("finding carries no fix")
	}
}
