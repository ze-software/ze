// Design: plan/spec-pki-full-chain.md -- web TLS certificate doctor check tests

package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"

	"github.com/stretchr/testify/require"
)

// webDoctorTree builds a config tree with a pki block defining "lan" (plus a
// keyless "no-key" entry) and an environment.web block referencing `reference`.
// enabled controls the web block's enabled leaf so the settings-not-addresses
// behavior can be asserted.
func webDoctorTree(t *testing.T, enabled, reference string, notAfter time.Time) *zeconfig.Tree {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "web doctor ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "web doctor leaf"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	require.NoError(t, err)
	leafKeyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	require.NoError(t, err)

	b64 := base64.StdEncoding.EncodeToString

	tree := zeconfig.NewTree()
	pkiC := tree.GetOrCreateContainer("pki")
	caEntry := zeconfig.NewTree()
	caEntry.Set("certificate", b64(caDER))
	pkiC.AddListEntry("ca", "web-ca", caEntry)

	certEntry := zeconfig.NewTree()
	certEntry.Set("certificate", b64(leafDER))
	certEntry.GetOrCreateContainer("private").Set("key", b64(leafKeyDER))
	pkiC.AddListEntry("certificate", "lan", certEntry)

	keyless := zeconfig.NewTree()
	keyless.Set("certificate", b64(leafDER))
	pkiC.AddListEntry("certificate", "no-key", keyless)

	envC := tree.GetOrCreateContainer("environment")
	web := envC.GetOrCreateContainer("web")
	web.Set("enabled", enabled)
	if reference != "" {
		web.Set("certificate", reference)
	}
	return tree
}

func codes(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for i := range diags {
		out = append(out, diags[i].Code)
	}
	return out
}

func severities(diags []diagnostic.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for i := range diags {
		out = append(out, string(diags[i].Severity))
	}
	return out
}

func TestWebTLSDoctorCheck(t *testing.T) {
	// VALIDATES: AC-8 for the web surface -- `ze doctor` reports a broken
	// environment.web.certificate reference before the operator deploys it.
	// PREVENTS: discovering a typo'd certificate name as a refused daemon start
	// in production, which is what the fail-closed startup gate would otherwise
	// be the first sign of.
	now := time.Now()
	far := now.Add(365 * 24 * time.Hour)

	t.Run("healthy reference is clean", func(t *testing.T) {
		require.Empty(t, webTLSDiagnostic(webDoctorTree(t, "true", "lan", far), now))
	})

	t.Run("no reference is clean", func(t *testing.T) {
		// The self-signed default is not a problem to report.
		require.Empty(t, webTLSDiagnostic(webDoctorTree(t, "true", "", far), now))
	})

	t.Run("missing entry", func(t *testing.T) {
		diags := webTLSDiagnostic(webDoctorTree(t, "true", "typo", far), now)
		require.Equal(t, []string{"doctor-tls-reference"}, codes(diags))
		require.Equal(t, []string{"error"}, severities(diags))
		require.True(t, strings.HasPrefix(diags[0].Message, "environment.web.certificate:"),
			"the message must name the leaf the operator has to fix, got %q", diags[0].Message)
		require.Contains(t, diags[0].Message, "typo")
	})

	t.Run("keyless entry", func(t *testing.T) {
		diags := webTLSDiagnostic(webDoctorTree(t, "true", "no-key", far), now)
		require.Equal(t, []string{"doctor-tls-reference"}, codes(diags))
	})

	t.Run("expired certificate", func(t *testing.T) {
		diags := webTLSDiagnostic(webDoctorTree(t, "true", "lan", now.Add(-time.Hour)), now)
		require.Contains(t, codes(diags), "doctor-tls-expired")
	})

	t.Run("expiring inside the warning window", func(t *testing.T) {
		diags := webTLSDiagnostic(webDoctorTree(t, "true", "lan", now.Add(10*24*time.Hour)), now)
		require.Contains(t, codes(diags), "doctor-tls-expired")
		require.Equal(t, []string{"warning"}, severities(diags))
	})

	t.Run("a disabled block is still checked", func(t *testing.T) {
		// The listener may be started by --web, ze.web.listen, or
		// ze.web.enabled, all of which would serve this certificate. Skipping
		// the check here is the 1327 gate in diagnostic form.
		diags := webTLSDiagnostic(webDoctorTree(t, "false", "typo", far), now)
		require.Equal(t, []string{"doctor-tls-reference"}, codes(diags))
	})

	t.Run("no web block at all", func(t *testing.T) {
		require.Empty(t, webTLSDiagnostic(zeconfig.NewTree(), now))
		require.Empty(t, webTLSDiagnostic(nil, now))
	})
}

func TestWebTLSDoctorCheckRegistered(t *testing.T) {
	// The check must be reachable from `ze doctor`, not merely defined.
	found := false
	checks := diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig)
	for i := range checks {
		if checks[i].Name == "web-tls-certificate" {
			found = true
			require.Contains(t, checks[i].Codes, "doctor-tls-reference")
		}
	}
	require.True(t, found, "web-tls-certificate must be in the doctor check registry")
}
