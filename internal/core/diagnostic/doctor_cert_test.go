// Design: docs/features/ai-first.md -- the certificate material a registered doctor check reads
// Detail: doctor_cert.go -- DoctorCertPair and DoctorCertExpiry
//
// These cases arrived from internal/component/doctor with the two functions
// they drive, case for case.

package diagnostic

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCertPEM mints one self-signed certificate with the given validity window
// and returns it PEM-encoded.
func testCertPEM(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestDoctorCertExpiry_Valid(t *testing.T) {
	certPEM := testCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))
	assert.Empty(t, DoctorCertExpiry("test", "/test/cert.pem", certPEM))
}

func TestDoctorCertExpiry_Expired(t *testing.T) {
	certPEM := testCertPEM(t, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))
	diags := DoctorCertExpiry("test", "/test/cert.pem", certPEM)
	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorTLSExpired, diags[0].Code)
	assert.Equal(t, SeverityError, diags[0].Severity)
}

func TestDoctorCertExpiry_NotYetValid(t *testing.T) {
	certPEM := testCertPEM(t, time.Now().Add(24*time.Hour), time.Now().Add(365*24*time.Hour))
	diags := DoctorCertExpiry("test", "/test/cert.pem", certPEM)
	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorTLSExpired, diags[0].Code)
	assert.Contains(t, diags[0].Message, "not yet valid")
}

func TestDoctorCertExpiry_ExpiringSoon(t *testing.T) {
	certPEM := testCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(15*24*time.Hour))
	diags := DoctorCertExpiry("test", "/test/cert.pem", certPEM)
	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorTLSExpired, diags[0].Code)
	assert.Equal(t, SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "expires in")
}

func TestDoctorCertExpiry_InvalidPEM(t *testing.T) {
	diags := DoctorCertExpiry("test", "/test/cert.pem", []byte("not-pem"))
	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorTLSInvalid, diags[0].Code)
	assert.Contains(t, diags[0].Message, "not valid PEM")
}

func TestDoctorCertExpiry_BadDER(t *testing.T) {
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not-der")})
	diags := DoctorCertExpiry("test", "/test/cert.pem", pemData)
	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorTLSInvalid, diags[0].Code)
	assert.Contains(t, diags[0].Message, "cannot parse certificate")
}

func TestDoctorCertPair_KeyMissing(t *testing.T) {
	dir := t.TempDir()
	diags := DoctorCertPair("test", "", "/nonexistent/key.pem", dir)
	require.Len(t, diags, 1)
	assert.Equal(t, CodeDoctorTLSMissing, diags[0].Code)
	assert.Contains(t, diags[0].Message, "key not found")
}

// TestDoctorCertPair_Unconfigured pins the pair nobody set: it is not a
// finding, because the surface then serves without operator material.
func TestDoctorCertPair_Unconfigured(t *testing.T) {
	assert.Empty(t, DoctorCertPair("test", "", "", t.TempDir()))
}
