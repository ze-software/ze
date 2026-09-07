// VALIDATES: a `pki ca` block reads the `crl` leaf-list in both accepted forms,
// refuses a list another CA signed, and hands what it stored back as PEM for the
// EAP-TLS revocation check RFC 9190 Section 5.4 requires.
// PREVENTS: an operator's revocation list being parsed and then not reaching the
// consumer, and a list pasted under the wrong ca reading as a working source.
package pki

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

// testCRLDER issues a revocation list from the CA whose DER and key are given,
// naming the serial numbers to revoke.
func testCRLDER(t *testing.T, caKey any, caDER []byte, revoked ...int64) []byte {
	t.Helper()
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]x509.RevocationListEntry, 0, len(revoked))
	for _, serial := range revoked {
		entries = append(entries, x509.RevocationListEntry{
			SerialNumber:   big.NewInt(serial),
			RevocationTime: time.Now().Add(-time.Minute),
		})
	}
	signer, ok := caKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("ca key is %T, want *ecdsa.PrivateKey", caKey)
	}
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now().Add(-time.Hour),
		NextUpdate:                time.Now().Add(24 * time.Hour),
		RevokedCertificateEntries: entries,
	}, caCert, signer)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

// makeCATree builds a config tree holding one `pki ca` with the crl values given.
func makeCATree(t *testing.T, caDER []byte, crlValues []string) *config.Tree {
	t.Helper()
	tree := config.NewTree()
	pkiContainer := tree.GetOrCreateContainer("pki")
	caEntry := config.NewTree()
	caEntry.Set("certificate", base64.StdEncoding.EncodeToString(caDER))
	if len(crlValues) > 0 {
		caEntry.SetSlice("crl", crlValues)
	}
	pkiContainer.AddListEntry("ca", "test-ca", caEntry)
	return tree
}

func TestParseCACRLAcceptsPEMAndBase64(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	crlDER := testCRLDER(t, caKey, caDER, 42)

	cases := []struct {
		name  string
		value string
	}{
		{"pem", pemDocument(t, pemBlockCRL, crlDER)},
		{"base64 der", base64.StdEncoding.EncodeToString(crlDER)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(makeCATree(t, caDER, []string{tc.value}))
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			ca := cfg.CACerts["test-ca"]
			if len(ca.CRLs) != 1 {
				t.Fatalf("revocation lists parsed = %d, want 1", len(ca.CRLs))
			}
			entries := ca.CRLs[0].RevokedCertificateEntries
			if len(entries) != 1 || entries[0].SerialNumber.Int64() != 42 {
				t.Fatalf("the parsed list does not carry serial 42: %v", entries)
			}
		})
	}
}

// TestCACRLPEMRoundTripsToTheConsumer is the wiring assertion: what the operator
// pasted is what the EAP-TLS revocation check receives. The two `crl` entries
// come back as two PEM documents, and re-parsing them yields the same serial
// numbers.
func TestCACRLPEMRoundTripsToTheConsumer(t *testing.T) {
	caKey, caDER := testCACertDER(t)
	first := testCRLDER(t, caKey, caDER, 7)
	second := testCRLDER(t, caKey, caDER, 9)

	cfg, err := ParseConfig(makeCATree(t, caDER, []string{
		pemDocument(t, pemBlockCRL, first),
		base64.StdEncoding.EncodeToString(second),
	}))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	out := cfg.CACerts["test-ca"].CRLPEM()
	if out == nil {
		t.Fatal("a CA carrying two revocation lists answered no PEM")
	}

	var serials []int64
	rest := out
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != pemBlockCRL {
			t.Fatalf("PEM block is %q, want %q", block.Type, pemBlockCRL)
		}
		list, pErr := x509.ParseRevocationList(block.Bytes)
		if pErr != nil {
			t.Fatalf("re-parse the emitted list: %v", pErr)
		}
		for _, entry := range list.RevokedCertificateEntries {
			serials = append(serials, entry.SerialNumber.Int64())
		}
	}
	if len(serials) != 2 || serials[0] != 7 || serials[1] != 9 {
		t.Fatalf("emitted serials = %v, want [7 9]", serials)
	}
}

// TestCACRLPEMIsNilWhenNoListIsConfigured pins the distinction the consumer
// turns on: a CA with no list answers nil, which is not the same as a list that
// revokes nothing.
func TestCACRLPEMIsNilWhenNoListIsConfigured(t *testing.T) {
	_, caDER := testCACertDER(t)

	cfg, err := ParseConfig(makeCATree(t, caDER, nil))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if out := cfg.CACerts["test-ca"].CRLPEM(); out != nil {
		t.Fatalf("a CA with no crl answered %d octets of PEM, want nil", len(out))
	}
}

// TestParseCACRLRefusesAnotherCAsList: a list this CA did not sign cannot answer
// for the certificates it issued, so it is refused at load rather than stored.
func TestParseCACRLRefusesAnotherCAsList(t *testing.T) {
	_, caDER := testCACertDER(t)
	otherKey, otherDER := testCACertDER(t)
	foreign := testCRLDER(t, otherKey, otherDER, 42)

	_, err := ParseConfig(makeCATree(t, caDER, []string{pemDocument(t, pemBlockCRL, foreign)}))
	if err == nil {
		t.Fatal("a revocation list signed by another CA was accepted")
	}
	if !errors.Is(err, errPKICRLIssuer) {
		t.Fatalf("refused with %v, want errPKICRLIssuer", err)
	}
}

// TestParseCACRLRefusesACertificatePastedIntoTheCRLLeaf keeps the two leaves
// from swallowing each other's material: a CERTIFICATE block in `crl` names the
// label it found.
func TestParseCACRLRefusesACertificatePastedIntoTheCRLLeaf(t *testing.T) {
	_, caDER := testCACertDER(t)

	_, err := ParseConfig(makeCATree(t, caDER, []string{pemDocument(t, pemBlockCertificate, caDER)}))
	if err == nil {
		t.Fatal("a CERTIFICATE PEM block was accepted as a revocation list")
	}
	if !errors.Is(err, errPKICRLPEMBlock) {
		t.Fatalf("refused with %v, want errPKICRLPEMBlock", err)
	}
}
