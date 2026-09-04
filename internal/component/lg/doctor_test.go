// Design: docs/architecture/pki/tls-listeners.md -- looking-glass TLS certificate doctor check tests

package lg

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"slices"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// lgDoctorTree builds a config tree whose pki block defines "lan" (plus a
// keyless "no-key" entry) and whose environment.looking-glass block references
// `reference`. An empty enabled writes no enabled leaf at all, which is how an
// operator who starts the looking glass from ze.looking-glass.listen writes the
// block. An empty reference writes no certificate leaf.
func lgDoctorTree(t *testing.T, enabled, reference string, notAfter time.Time) *zeconfig.Tree {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ca key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "lg doctor ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create ca certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse ca certificate: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "lg doctor leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf certificate: %v", err)
	}
	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	b64 := base64.StdEncoding.EncodeToString

	tree := zeconfig.NewTree()
	pkiC := tree.GetOrCreateContainer("pki")
	caEntry := zeconfig.NewTree()
	caEntry.Set("certificate", b64(caDER))
	pkiC.AddListEntry("ca", "lg-ca", caEntry)

	certEntry := zeconfig.NewTree()
	certEntry.Set("certificate", b64(leafDER))
	certEntry.GetOrCreateContainer("private").Set("key", b64(leafKeyDER))
	pkiC.AddListEntry("certificate", "lan", certEntry)

	keyless := zeconfig.NewTree()
	keyless.Set("certificate", b64(leafDER))
	pkiC.AddListEntry("certificate", "no-key", keyless)

	envC := tree.GetOrCreateContainer("environment")
	lgC := envC.GetOrCreateContainer("looking-glass")
	if enabled != "" {
		lgC.Set("enabled", enabled)
	}
	if reference != "" {
		lgC.Set("certificate", reference)
	}
	return tree
}

func lgDiagCodes(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

func lgDiagSeverities(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for i := range diags {
		out = append(out, string(diags[i].Severity))
	}
	return out
}

func TestLGTLSDoctorCheck(t *testing.T) {
	// VALIDATES: AC-9 -- `ze doctor` reports a broken
	// environment.looking-glass.certificate reference before the operator
	// deploys it.
	// PREVENTS: learning about a typo'd certificate name from a refused daemon
	// start, on the one listener Ze publishes to strangers.
	now := time.Now()
	far := now.Add(365 * 24 * time.Hour)

	t.Run("healthy reference is clean", func(t *testing.T) {
		if diags := lgTLSDiagnostic(lgDoctorTree(t, "true", "lan", far), now); len(diags) != 0 {
			t.Fatalf("a resolvable certificate must report nothing, got %v", lgDiagCodes(diags))
		}
	})

	t.Run("no reference is clean", func(t *testing.T) {
		// The self-signed default is not a problem to report.
		if diags := lgTLSDiagnostic(lgDoctorTree(t, "true", "", far), now); len(diags) != 0 {
			t.Fatalf("an unset certificate leaf must report nothing, got %v", lgDiagCodes(diags))
		}
	})

	t.Run("missing entry", func(t *testing.T) {
		diags := lgTLSDiagnostic(lgDoctorTree(t, "true", "typo", far), now)
		if got := lgDiagCodes(diags); !slices.Equal(got, []string{"doctor-tls-reference"}) {
			t.Fatalf("codes: got %v, want [doctor-tls-reference]", got)
		}
		if got := lgDiagSeverities(diags); !slices.Equal(got, []string{"error"}) {
			t.Fatalf("severities: got %v, want [error]", got)
		}
		if !strings.HasPrefix(diags[0].Message, "environment.looking-glass.certificate:") {
			t.Fatalf("the message must name the leaf the operator has to fix, got %q", diags[0].Message)
		}
		if !strings.Contains(diags[0].Message, "typo") {
			t.Fatalf("the message must name the unresolved certificate, got %q", diags[0].Message)
		}
	})

	t.Run("keyless entry", func(t *testing.T) {
		diags := lgTLSDiagnostic(lgDoctorTree(t, "true", "no-key", far), now)
		if got := lgDiagCodes(diags); !slices.Equal(got, []string{"doctor-tls-reference"}) {
			t.Fatalf("codes: got %v, want [doctor-tls-reference]", got)
		}
	})

	t.Run("expired certificate", func(t *testing.T) {
		diags := lgTLSDiagnostic(lgDoctorTree(t, "true", "lan", now.Add(-time.Hour)), now)
		if got := lgDiagCodes(diags); !slices.Contains(got, "doctor-tls-expired") {
			t.Fatalf("codes: got %v, want one doctor-tls-expired", got)
		}
		if got := lgDiagSeverities(diags); !slices.Contains(got, "error") {
			t.Fatalf("severities: got %v, want one error", got)
		}
	})

	t.Run("expiring inside the warning window", func(t *testing.T) {
		diags := lgTLSDiagnostic(lgDoctorTree(t, "true", "lan", now.Add(10*24*time.Hour)), now)
		if got := lgDiagCodes(diags); !slices.Contains(got, "doctor-tls-expired") {
			t.Fatalf("codes: got %v, want one doctor-tls-expired", got)
		}
		if got := lgDiagSeverities(diags); !slices.Equal(got, []string{"warning"}) {
			t.Fatalf("severities: got %v, want [warning]", got)
		}
	})

	t.Run("a disabled block is still checked", func(t *testing.T) {
		// ze.looking-glass.listen and ze.looking-glass.enabled both start the
		// listener without an `enabled true` leaf, and it would serve this
		// certificate. Reading ExtractLGConfig here would stay silent for
		// exactly those deployments.
		diags := lgTLSDiagnostic(lgDoctorTree(t, "false", "typo", far), now)
		if got := lgDiagCodes(diags); !slices.Equal(got, []string{"doctor-tls-reference"}) {
			t.Fatalf("codes: got %v, want [doctor-tls-reference]", got)
		}
	})

	t.Run("a block with no enabled leaf is still checked", func(t *testing.T) {
		diags := lgTLSDiagnostic(lgDoctorTree(t, "", "typo", far), now)
		if got := lgDiagCodes(diags); !slices.Equal(got, []string{"doctor-tls-reference"}) {
			t.Fatalf("codes: got %v, want [doctor-tls-reference]", got)
		}
	})

	t.Run("no looking-glass block at all", func(t *testing.T) {
		if diags := lgTLSDiagnostic(zeconfig.NewTree(), now); len(diags) != 0 {
			t.Fatalf("an empty tree must report nothing, got %v", lgDiagCodes(diags))
		}
		if diags := lgTLSDiagnostic(nil, now); len(diags) != 0 {
			t.Fatalf("a nil tree must report nothing, got %v", lgDiagCodes(diags))
		}
	})

	t.Run("a tree of another type is not checked", func(t *testing.T) {
		// The registry hands the check an `any`. A tree it cannot read must not
		// produce a verdict about a certificate it never saw.
		diags := checkLGTLSCertificate(diagnostic.DoctorCheckContext{Tree: "not a tree"})
		if len(diags) != 0 {
			t.Fatalf("an unreadable tree must report nothing, got %v", lgDiagCodes(diags))
		}
	})
}

func TestLGTLSDoctorCheckRegistered(t *testing.T) {
	// The check must be reachable from `ze doctor`, not merely defined.
	found := false
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "lg-tls-certificate" {
			continue
		}
		found = true
		if check.Component != "lg" {
			t.Fatalf("component: got %q, want lg", check.Component)
		}
		if !slices.Contains(check.Codes, "doctor-tls-reference") {
			t.Fatalf("codes: got %v, want one doctor-tls-reference", check.Codes)
		}
		if !slices.Contains(check.Codes, "doctor-tls-expired") {
			t.Fatalf("codes: got %v, want one doctor-tls-expired", check.Codes)
		}
	}
	if !found {
		t.Fatal("lg-tls-certificate must be in the doctor check registry")
	}
}
