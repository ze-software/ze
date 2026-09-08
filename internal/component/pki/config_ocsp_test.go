// VALIDATES: a `pki certificate` block reads the `ocsp-response` leaf in both
// accepted forms, refuses a response about another certificate, and stores the
// DER the EAP-TLS authenticator staples to answer a Certificate Status Request.
// PREVENTS: an operator's OCSP response being parsed and then not reaching the
// consumer, and a response pasted under the wrong certificate reading as a
// working status.
package pki

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/ze-software/ze/internal/component/config"
)

// testOCSPResponseDER signs an OCSP response about the certificate given, with
// the CA's own key, which is the undelegated form RFC 6960 Section 4.2.2.2 names
// first.
func testOCSPResponseDER(t *testing.T, caKey any, caDER, certDER []byte) []byte {
	t.Helper()
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}
	signer, ok := caKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("ca key is %T, want *ecdsa.PrivateKey", caKey)
	}
	der, err := ocsp.CreateResponse(caCert, caCert, ocsp.Response{
		Status:       ocsp.Good,
		SerialNumber: cert.SerialNumber,
		ThisUpdate:   time.Now().Add(-time.Hour),
		NextUpdate:   time.Now().Add(24 * time.Hour),
	}, signer)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// makeCertTreeWithOCSP builds a config tree holding one `pki certificate` with
// the ocsp-response value given, and the ca that issued it.
func makeCertTreeWithOCSP(t *testing.T, caDER, certDER []byte, key any, ocspValue string) *config.Tree {
	t.Helper()
	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")

	caEntry := config.NewTree()
	caEntry.Set("certificate", base64.StdEncoding.EncodeToString(caDER))
	pkiContainer.AddListEntry("ca", "test-ca", caEntry)

	certEntry := config.NewTree()
	certEntry.Set("certificate", base64.StdEncoding.EncodeToString(certDER))
	privContainer := certEntry.GetOrCreateContainer("private")
	privContainer.Set("key", marshalKeyB64(t, key))
	if ocspValue != "" {
		certEntry.Set("ocsp-response", ocspValue)
	}
	pkiContainer.AddListEntry("certificate", "dev-1", certEntry)

	return tree
}

// TestParseCertificateOCSPResponseAcceptsPEMAndBase64 reads the leaf in both
// forms an operator arrives with.
func TestParseCertificateOCSPResponseAcceptsPEMAndBase64(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	key, certDER := testDeviceCertDER(t, caKey, caDER)
	responseDER := testOCSPResponseDER(t, caKey, caDER, certDER)

	cases := []struct {
		name  string
		value string
	}{
		{"pem", pemDocument(t, pemBlockOCSPResponse, responseDER)},
		{"base64 der", base64.StdEncoding.EncodeToString(responseDER)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(makeCertTreeWithOCSP(t, caDER, certDER, key, tc.value))
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			entry, ok := cfg.Certificates["dev-1"]
			if !ok {
				t.Fatal("certificate 'dev-1' not found")
			}
			if len(entry.RawOCSPResponse) == 0 {
				t.Fatal("the entry stored no OCSP response")
			}
			// The DER the responder signed is what the authenticator staples, so
			// what is stored has to be those exact bytes rather than a re-encoding.
			if !bytes.Equal(entry.RawOCSPResponse, responseDER) {
				t.Fatalf("the entry stored %d octets and the operator pasted %d",
					len(entry.RawOCSPResponse), len(responseDER))
			}
		})
	}
}

// TestCertificateOCSPResponseIsNilWhenNoneIsConfigured pins the state that makes
// an authenticator answer a Certificate Status Request with no status, which
// RFC 6066 Section 8 permits.
func TestCertificateOCSPResponseIsNilWhenNoneIsConfigured(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	key, certDER := testDeviceCertDER(t, caKey, caDER)

	cfg, err := ParseConfig(makeCertTreeWithOCSP(t, caDER, certDER, key, ""))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	entry, ok := cfg.Certificates["dev-1"]
	if !ok {
		t.Fatal("certificate 'dev-1' not found")
	}
	if entry.RawOCSPResponse != nil {
		t.Fatalf("an entry with no ocsp-response holds %d octets of response", len(entry.RawOCSPResponse))
	}
}

// TestParseCertificateOCSPResponseRefusesAnotherCertificatesResponse refuses a
// response the same CA signed about a different certificate.
//
// A response is about ONE certificate, named by serial number, so one pasted
// under the wrong entry answers nothing about the certificate ze presents. The
// refusal happens at config load, where the message can name the entry.
func TestParseCertificateOCSPResponseRefusesAnotherCertificatesResponse(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	key, certDER := testDeviceCertDER(t, caKey, caDER)
	otherResponse := testOCSPResponseDER(t, caKey, caDER, caDER)

	_, err := ParseConfig(makeCertTreeWithOCSP(t, caDER, certDER, key,
		base64.StdEncoding.EncodeToString(otherResponse)))
	if err == nil {
		t.Fatal("ParseConfig accepted an OCSP response about another certificate")
	}
	if !errors.Is(err, errPKIOCSPSerial) {
		t.Fatalf("ParseConfig refused with %v, want the wrong-certificate refusal", err)
	}
}

// TestParseCertificateOCSPResponseRefusesACertificatePastedIntoTheLeaf refuses
// bytes that are not an OCSP response at all.
//
// The certificate's own DER is the paste an operator makes by reaching for the
// wrong file, and it decodes as base64 and then fails to parse, which is the
// path this covers.
func TestParseCertificateOCSPResponseRefusesACertificatePastedIntoTheLeaf(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	key, certDER := testDeviceCertDER(t, caKey, caDER)

	_, err := ParseConfig(makeCertTreeWithOCSP(t, caDER, certDER, key,
		base64.StdEncoding.EncodeToString(certDER)))
	if err == nil {
		t.Fatal("ParseConfig accepted a certificate pasted into the ocsp-response leaf")
	}
	if !errors.Is(err, errPKIOCSPParse) {
		t.Fatalf("ParseConfig refused with %v, want the unparseable-response refusal", err)
	}
}

// TestParseCertificateOCSPResponseRefusesAWrongPEMLabel refuses a PEM document
// that is not an OCSP response.
func TestParseCertificateOCSPResponseRefusesAWrongPEMLabel(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	key, certDER := testDeviceCertDER(t, caKey, caDER)
	responseDER := testOCSPResponseDER(t, caKey, caDER, certDER)

	_, err := ParseConfig(makeCertTreeWithOCSP(t, caDER, certDER, key,
		pemDocument(t, pemBlockCertificate, responseDER)))
	if err == nil {
		t.Fatal("ParseConfig accepted an OCSP response wrapped in a CERTIFICATE PEM block")
	}
	if !errors.Is(err, errPKIOCSPPEMBlock) {
		t.Fatalf("ParseConfig refused with %v, want the wrong-PEM-label refusal", err)
	}
}
