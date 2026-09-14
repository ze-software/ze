// VALIDATES: the doctor check over the certificates an operator pastes into the
// pki config block reports missing and expired material under doctor-pki-cert,
// and is registered with that code.
// PREVENTS: a CA or a certificate every TLS listener trusts failing at its first
// connection rather than in the readiness report.
//
// The two check cases arrived from internal/component/doctor with the check
// they drive, case for case.
package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// configCertDER mints one self-signed certificate with the given validity
// window and returns its DER bytes, the encoding the pki block carries.
func configCertDER(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	return der
}

// configCertDiagnostic runs the check over one tree and returns the diagnostic
// carrying code, failing the test when none does.
func configCertDiagnostic(t *testing.T, tree *config.Tree, code string) diagnostic.Diagnostic {
	t.Helper()
	diags := checkConfiguredCertificates(diagnostic.DoctorCheckContext{Tree: tree})
	for i := range diags {
		if diags[i].Code == code {
			return diags[i]
		}
	}
	t.Fatalf("missing diagnostic %s in %+v", code, diags)
	return diagnostic.Diagnostic{}
}

func TestCheckConfiguredCertificates_MissingCA(t *testing.T) {
	// VALIDATES: AC-9 PKI CA entries without certificate material return doctor-pki-cert.
	// PREVENTS: Certificate store gaps being missed until IPsec or TLS uses the CA.
	tree := config.NewTree()
	tree.GetOrCreateContainer("pki").AddListEntry("ca", "root", config.NewTree())

	diag := configCertDiagnostic(t, tree, diagnostic.CodeDoctorPKICert)
	if diag.Severity != diagnostic.SeverityError {
		t.Fatalf("severity = %q, want error", diag.Severity)
	}
}

func TestCheckConfiguredCertificates_ExpiredCA(t *testing.T) {
	// VALIDATES: AC-9 PKI CA certificate validity is checked, not just presence.
	// PREVENTS: Expired embedded CA certificates passing readiness checks.
	tree := config.NewTree()
	ca := config.NewTree()
	ca.Set("certificate", base64.StdEncoding.EncodeToString(configCertDER(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))))
	tree.GetOrCreateContainer("pki").AddListEntry("ca", "root", ca)

	diag := configCertDiagnostic(t, tree, diagnostic.CodeDoctorPKICert)
	if diag.Severity != diagnostic.SeverityError {
		t.Fatalf("severity = %q, want error", diag.Severity)
	}
}

func TestCheckConfiguredCertificates_SilentWithoutABlock(t *testing.T) {
	if diags := checkConfiguredCertificates(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no pki block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkConfiguredCertificates(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
}

func TestConfigCertDoctorCheckRegistered(t *testing.T) {
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(configCertDoctorCheck.Phase)
	for i := range checks {
		if checks[i].Name == configCertDoctorCheck.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", configCertDoctorCheck.Name, configCertDoctorCheck.Phase)
	}
	if found.Order != configCertDoctorCheck.Order {
		t.Fatalf("order = %d, want %d", found.Order, configCertDoctorCheck.Order)
	}

	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(diagnostic.CodeDoctorPKICert) == nil {
		t.Fatalf("diagnostic code %q is not registered", diagnostic.CodeDoctorPKICert)
	}
}
