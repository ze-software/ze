// Design: docs/architecture/pki/tls-listeners.md -- stored web TLS pair doctor check tests
// Detail: doctor_material.go -- checkWebTLSMaterial and its registration
//
// These cases arrived from internal/component/doctor with the check they
// drive, case for case, and the last one drives it through the entry point an
// operator reaches.

package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/zefs"
)

// stubStorage overlays a filesystem store with in-memory files, so a test can
// stand in the blob store the web listener reads its pair from.
type stubStorage struct {
	storage.Storage
	data map[string][]byte
	// unreadable names files that exist in storage and fail to read, the state
	// a corrupt or unreadable file reaches. Exists says yes, ReadFile says no.
	unreadable map[string]error
}

func (s *stubStorage) ReadFile(name string) ([]byte, error) {
	if err, ok := s.unreadable[name]; ok {
		return nil, err
	}
	if d, ok := s.data[name]; ok {
		return d, nil
	}
	return s.Storage.ReadFile(name)
}

func (s *stubStorage) Exists(name string) bool {
	if _, ok := s.data[name]; ok {
		return true
	}
	if _, ok := s.unreadable[name]; ok {
		return true
	}
	return s.Storage.Exists(name)
}

// webMaterialCertPEM returns a self-signed certificate and the key that signed
// it, both PEM-encoded, so a test can supply material tls.X509KeyPair accepts.
func webMaterialCertPEM(t *testing.T, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func webEnabledTree() *zeconfig.Tree {
	tree := zeconfig.NewTree()
	tree.GetOrCreateContainer("environment").GetOrCreateContainer("web").Set("enabled", "true")
	return tree
}

func storedPair(cert, key []byte) *stubStorage {
	data := map[string][]byte{}
	if cert != nil {
		data[zefs.KeyWebCert.Pattern] = cert
	}
	if key != nil {
		data[zefs.KeyWebKey.Pattern] = key
	}
	return &stubStorage{Storage: storage.NewFilesystem(), data: data}
}

func TestCheckWebTLSMaterial_NoCerts(t *testing.T) {
	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree(), Store: storage.NewFilesystem()})
	assert.Empty(t, diags, "no blob certs should produce no diagnostics")
}

func TestCheckWebTLSMaterial_ExpiredCert(t *testing.T) {
	certPEM, keyPEM := webMaterialCertPEM(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))

	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree(), Store: storedPair(certPEM, keyPEM)})
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSExpired, diags[0].Code)
}

func TestCheckWebTLSMaterial_CertWithoutKey(t *testing.T) {
	certPEM, _ := webMaterialCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))

	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree(), Store: storedPair(certPEM, nil)})
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSMissing, diags[0].Code)
	assert.Contains(t, diags[0].Message, "key missing")
}

func TestCheckWebTLSMaterial_KeyWithoutCert(t *testing.T) {
	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree(), Store: storedPair(nil, []byte("key-data"))})
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSMissing, diags[0].Code)
	assert.Contains(t, diags[0].Message, "certificate missing")
}

func TestCheckWebTLSMaterial_Disabled(t *testing.T) {
	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: zeconfig.NewTree(), Store: storage.NewFilesystem()})
	assert.Empty(t, diags, "web not enabled should skip")
	assert.Empty(t, checkWebTLSMaterial(diagnostic.DoctorCheckContext{Store: storage.NewFilesystem()}), "nil tree")
}

func TestCheckWebTLSMaterial_MatchingPair(t *testing.T) {
	// VALIDATES: a certificate and the key that signed it produce no diagnostic.
	// PREVENTS: a pair check that reports every stored pair as unusable.
	certPEM, keyPEM := webMaterialCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))

	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree(), Store: storedPair(certPEM, keyPEM)})
	assert.Empty(t, diags, "a matching pair is not a finding")
}

func TestCheckWebTLSMaterial_MismatchedPair(t *testing.T) {
	// VALIDATES: a stored certificate and key that do not load as a TLS pair are
	// reported doctor-tls-invalid, and the message carries no key material.
	// PREVENTS: a mismatched pair written by a ze that predates the replace-cert
	// validation, which starts no web listener and which no check mentions.
	certPEM, _ := webMaterialCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))
	_, foreignKeyPEM := webMaterialCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))

	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree(), Store: storedPair(certPEM, foreignKeyPEM)})
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSInvalid, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "not a usable pair")

	for line := range strings.SplitSeq(strings.TrimSpace(string(foreignKeyPEM)), "\n") {
		if strings.HasPrefix(line, "-----") {
			continue
		}
		assert.NotContains(t, diags[0].Message, line, "the message carries key material")
	}
}

func TestCheckWebTLSMaterial_UnreadableKey(t *testing.T) {
	// VALIDATES: a key that exists and fails to read is reported as unreadable.
	// PREVENTS: reading the key collapsing a read failure into "key missing",
	// which names the wrong file and sends the operator to the wrong fix.
	certPEM, _ := webMaterialCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))

	store := storedPair(certPEM, nil)
	store.unreadable = map[string]error{zefs.KeyWebKey.Pattern: errors.New("input/output error")}
	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree(), Store: store})
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSInvalid, diags[0].Code)
	assert.Contains(t, diags[0].Message, "cannot be read")
	assert.NotContains(t, diags[0].Message, "missing", "an unreadable key is present, not missing")
}

func TestCheckWebTLSMaterial_NoStore(t *testing.T) {
	// VALIDATES: a run that resolved no storage says so rather than answering
	// "no pair stored", which is what a silent nil would read as.
	diags := checkWebTLSMaterial(diagnostic.DoctorCheckContext{Tree: webEnabledTree()})
	require.Len(t, diags, 1)
	assert.Equal(t, diagnostic.CodeDoctorTLSInvalid, diags[0].Code)
	assert.Contains(t, diags[0].Message, "no storage")
}

// TestWebTLSMaterialDoctorCheckRegistered asks the registry the doctor runner
// reads whether it holds this component's check, at the phase and order the
// declaration states, and whether every code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register.go installed the check and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestWebTLSMaterialDoctorCheckRegistered(t *testing.T) {
	want := webTLSMaterialDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	require.NotNil(t, found, "doctor check %q is not registered for phase %q", want.Name, want.Phase)
	assert.Equal(t, want.Order, found.Order)
	assert.Equal(t, want.Component, found.Component)

	diagnostic.RegisterBuiltinCodes()
	for _, code := range want.Codes {
		assert.NotNil(t, diagnostic.Lookup(code), "diagnostic code %q is not registered", code)
	}
}

// TestDoctorReportsUnusableWebTLSPair drives the check from the entry point an
// operator reaches: the doctor provider `ze doctor` and `show doctor` both run,
// over a config file and a filesystem store.
//
// VALIDATES: `ze doctor` reaches the pair check through the registry.
// PREVENTS: a check proven only by a unit test that calls it directly, while
// the runner never reaches it and the operator never sees the finding.
func TestDoctorReportsUnusableWebTLSPair(t *testing.T) {
	certPEM, _ := webMaterialCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))
	_, foreignKeyPEM := webMaterialCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))

	previous := env.Get("ze.storage.blob")
	require.NoError(t, env.Set("ze.storage.blob", "false"))
	t.Cleanup(func() {
		require.NoError(t, env.Set("ze.storage.blob", previous))
	})

	// Filesystem storage resolves a key pattern against the working directory.
	root := t.TempDir()
	cfgPath := filepath.Join(root, "ze.conf")
	require.NoError(t, os.WriteFile(cfgPath, []byte("environment {\n\tweb {\n\t\tenabled true\n\t}\n}\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(zefs.KeyWebCert.Pattern)), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, zefs.KeyWebCert.Pattern), certPEM, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, zefs.KeyWebKey.Pattern), foreignKeyPEM, 0o600))
	t.Chdir(root)

	diags := diagnostic.RunDoctorChecks(cfgPath)
	require.NotEmpty(t, diags, "the doctor provider ran no check: is internal/component/doctor linked into this binary?")

	found := false
	for i := range diags {
		if diags[i].Code == diagnostic.CodeDoctorTLSInvalid && strings.Contains(diags[i].Message, "not a usable pair") {
			found = true
			break
		}
	}
	assert.True(t, found, "ze doctor did not report the stored web pair as unusable: %v", codes(diags))
}
