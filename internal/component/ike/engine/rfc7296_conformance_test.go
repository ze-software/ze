package engine

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

const cfmCAName = "cfm-root"

// cfmRSALeaf mints an RSA certificate of the requested modulus size, signed by a fresh
// authority loaded into the PKI store. The certificate carries the subject alternative
// names given.
//
// The KEY SIZE is a parameter rather than a constant, because RFC 7296 Section 4 names two
// sizes, 1024 and 2048. A fixture pinned to one of them proves nothing about the other.
// Nothing in ze imposes a floor or a ceiling. The only honest way to show that is a real
// signature round trip at each size, rather than a code read.
func cfmRSALeaf(
	t *testing.T, bits int, subject pkix.Name, dns, emails []string,
) (leafDER []byte, leafKey *rsa.PrivateKey) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate the authority key: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cfm root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create the authority certificate: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse the authority certificate: %v", err)
	}

	leafKey, err = rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("generate the %d-bit leaf key: %v", bits, err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber:   big.NewInt(2),
		Subject:        subject,
		NotBefore:      time.Now().Add(-time.Hour),
		NotAfter:       time.Now().Add(time.Hour),
		DNSNames:       dns,
		EmailAddresses: emails,
	}
	leafDER, err = x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create the %d-bit leaf certificate: %v", bits, err)
	}

	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			cfmCAName: {Name: cfmCAName, Certificate: caCert, Raw: caDER},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})
	return leafDER, leafKey
}

// cfmRSAAuth builds a valid RFC 7427 method 14 AUTH payload signed by an RSA key. A
// refusal therefore comes from the identity policy and never from the arithmetic.
func cfmRSAAuth(t *testing.T, sa *SA, key *rsa.PrivateKey) *wire.PayloadAUTH {
	t.Helper()
	octets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("compute the signed octets: %v", err)
	}
	digest := sha256.Sum256(octets)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign the digest: %v", err)
	}
	data := make([]byte, 0, 1+len(oidSHA256WithRSA)+len(sig))
	data = append(data, byte(len(oidSHA256WithRSA)))
	data = append(data, oidSHA256WithRSA...)
	data = append(data, sig...)
	return &wire.PayloadAUTH{AuthMethod: wire.AuthMethodDigitalSig, AuthData: data}
}

// cfmDN encodes a subject as the DER an ID_DER_ASN1_DN payload carries, and returns the
// rendering from RFC 4514 that an operator writes into remote-id.
func cfmDN(t *testing.T, subject pkix.Name) (der []byte, text string) {
	t.Helper()
	rdn := subject.ToRDNSequence()
	der, err := asn1.Marshal(rdn)
	if err != nil {
		t.Fatalf("marshal the distinguished name: %v", err)
	}
	return der, rdn.String()
}

// VALIDATES: RFC7296-4-4. Ze can be configured to accept the whole conformance set.
// The set has two halves. The first is PKIX certificates signed by RSA keys of 1024 and
// 2048 bits. Under that half the identity passed is any of ID_FQDN, ID_RFC822_ADDR,
// ID_DER_ASN1_DN or ID_KEY_ID. The second is shared key authentication, where the identity
// is any of ID_KEY_ID, ID_FQDN or ID_RFC822_ADDR.
// PREVENTS: the row being read as satisfied from Appendix A's shortened text, which drops
// the ID-type clause entirely. Both ID types the clause adds were DENIED.
// certificateCarriesIdentity fell through to false for ID_KEY_ID. assertedIdentity
// reported ID_DER_ASN1_DN not comparable, so checkRemoteIdentity refused before any
// certificate was read.
// RFC requirement: RFC7296-4-4 positive -- an RSA-1024 and an RSA-2048 PKIX peer
// authenticate under each required identity type, and a shared-key peer under each of its three.
// RFC requirement: RFC7296-4-4 negative -- a certificate that does NOT carry the asserted
// identity is refused for every one of those types, so the acceptance names an identity
// rather than accepting whatever the authority issued.
func TestCfmConformanceConfigurationSetIsAcceptable(t *testing.T) {
	const (
		fqdn = "gw.cfm.example"
		mail = "ops@cfm.example"
		keyi = "\x00\xffopaque-key-id"
	)

	t.Run("pkix rsa 1024 with ID_FQDN", func(t *testing.T) {
		der, key := cfmRSALeaf(t, 1024, pkix.Name{CommonName: fqdn}, []string{fqdn}, nil)
		sa := ridCertSA(t, cfmCAName, fqdn)
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeFQDN, []byte(fqdn))
		if err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key)); err != nil {
			t.Fatalf("an RSA-1024 PKIX peer asserting ID_FQDN was refused: %v", err)
		}
	})

	t.Run("pkix rsa 2048 with ID_RFC822_ADDR", func(t *testing.T) {
		der, key := cfmRSALeaf(t, 2048, pkix.Name{CommonName: fqdn}, nil, []string{mail})
		sa := ridCertSA(t, cfmCAName, mail)
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeRFC822Addr, []byte(mail))
		if err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key)); err != nil {
			t.Fatalf("an RSA-2048 PKIX peer asserting ID_RFC822_ADDR was refused: %v", err)
		}
	})

	t.Run("pkix rsa 2048 with ID_DER_ASN1_DN", func(t *testing.T) {
		subject := pkix.Name{CommonName: "hq", Organization: []string{"Example"}}
		der, key := cfmRSALeaf(t, 2048, subject, []string{fqdn}, nil)
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatalf("parse the leaf certificate: %v", err)
		}
		_, text := cfmDN(t, subject)

		sa := ridCertSA(t, cfmCAName, text)
		sa.RemoteCertRaw = der
		// The payload carries the certificate's OWN RawSubject, which is what a
		// conforming peer asserts and what the binding compares against.
		ridAssert(sa, wire.IDTypeDERASN1DN, cert.RawSubject)
		if err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key)); err != nil {
			t.Fatalf("an RSA-2048 PKIX peer asserting ID_DER_ASN1_DN was refused: %v", err)
		}
	})

	t.Run("pkix rsa 2048 with ID_KEY_ID under remote-id-type key-id", func(t *testing.T) {
		der, key := cfmRSALeaf(t, 2048, pkix.Name{CommonName: fqdn}, []string{fqdn}, nil)
		sa := ridCertSA(t, cfmCAName, keyi)
		sa.PeerCfg.Auth.RemoteIDType = wire.IDTypeKeyID
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeKeyID, []byte(keyi))
		if err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key)); err != nil {
			t.Fatalf("an RSA-2048 PKIX peer asserting ID_KEY_ID was refused with "+
				"remote-id-type key-id set: %v", err)
		}
	})

	t.Run("shared key with each required identity type", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			idType uint8
			value  string
		}{
			{"ID_KEY_ID", wire.IDTypeKeyID, keyi},
			{"ID_FQDN", wire.IDTypeFQDN, fqdn},
			{"ID_RFC822_ADDR", wire.IDTypeRFC822Addr, mail},
		} {
			t.Run(tc.name, func(t *testing.T) {
				sa := testSAWithKeys(t)
				sa.PeerCfg.Auth.Mode = ipsec.AuthPreSharedSecret
				sa.PeerCfg.Auth.PSK = "cfm-shared-secret"
				sa.PeerCfg.Auth.RemoteID = tc.value
				ridAssert(sa, tc.idType, []byte(tc.value))
				if err := verifyRemoteAuth(sa, ridPSKAuth(t, sa)); err != nil {
					t.Fatalf("a shared-key peer asserting %s was refused: %v", tc.name, err)
				}
			})
		}
	})
}

// VALIDATES: RFC7296-4-4. The acceptance above names an identity. A certificate that does
// not carry the asserted identity is refused for every required type, and ID_KEY_ID is
// refused unless the operator opted in.
// PREVENTS: the positive passing against a check that accepts every certificate the
// authority issued, which is exactly the state remote-id exists to fix. It also pins the
// unset-remote-id warning. Without that warning the fail-open path goes silent, and this
// row's evidence becomes indistinguishable from having no check at all.
// RFC requirement: RFC7296-4-4 negative -- a mismatched certificate is refused for
// ID_FQDN, ID_RFC822_ADDR and ID_DER_ASN1_DN, and ID_KEY_ID is refused without
// remote-id-type key-id, so the acceptance is a binding rather than an absence of one.
func TestCfmConformanceSetDoesNotAcceptWhatItMustNot(t *testing.T) {
	const (
		fqdn  = "gw.cfm.example"
		other = "attacker.cfm.example"
		mail  = "ops@cfm.example"
		keyi  = "\x00\xffopaque-key-id"
	)

	t.Run("a certificate carrying another name is refused", func(t *testing.T) {
		// The certificate is validly issued by the SAME authority and its signature is
		// good. Only the identity differs.
		der, key := cfmRSALeaf(t, 2048, pkix.Name{CommonName: other}, []string{other}, nil)
		sa := ridCertSA(t, cfmCAName, fqdn)
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeFQDN, []byte(fqdn))
		if err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key)); err == nil {
			t.Fatal("a certificate from the authority authenticated as an identity it does not carry")
		}
	})

	t.Run("a certificate carrying another mail address is refused", func(t *testing.T) {
		der, key := cfmRSALeaf(t, 2048, pkix.Name{CommonName: fqdn}, nil, []string{"other@cfm.example"})
		sa := ridCertSA(t, cfmCAName, mail)
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeRFC822Addr, []byte(mail))
		if err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key)); err == nil {
			t.Fatal("a certificate authenticated as a mail address it does not carry")
		}
	})

	t.Run("a distinguished name that is not the certificate subject is refused", func(t *testing.T) {
		// The asserted DN is well formed and equals remote-id, so the POLICY half
		// passes. The certificate's own subject is different, so the BINDING half must
		// refuse. Without this row a DN arm that compares nothing at all would still pass.
		subject := pkix.Name{CommonName: "hq", Organization: []string{"Example"}}
		der, key := cfmRSALeaf(t, 2048, pkix.Name{CommonName: "somewhere-else"}, []string{fqdn}, nil)
		asserted, text := cfmDN(t, subject)

		sa := ridCertSA(t, cfmCAName, text)
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeDERASN1DN, asserted)
		if err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key)); err == nil {
			t.Fatal("a distinguished name that is not the certificate subject authenticated")
		}
	})

	t.Run("ID_KEY_ID is refused without remote-id-type key-id", func(t *testing.T) {
		// The identical configuration that PASSES in the positive, minus the opt-in.
		// An opaque key id corresponds to no certificate field, so ze denies rather
		// than guessing (ai/rules/fail-closed-guards.md).
		der, key := cfmRSALeaf(t, 2048, pkix.Name{CommonName: fqdn}, []string{fqdn}, nil)
		sa := ridCertSA(t, cfmCAName, keyi)
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeKeyID, []byte(keyi))
		err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key))
		if err == nil {
			t.Fatal("a PKIX peer asserting ID_KEY_ID authenticated with no remote-id-type set")
		}
		if !strings.Contains(err.Error(), "remote-id-type key-id") {
			t.Errorf("the refusal %q does not name the leaf that would permit it", err)
		}
	})

	t.Run("remote-id-type refuses a type the operator did not name", func(t *testing.T) {
		// remote-id-type rfc822-address closes the widening that let ID_FQDN carrying
		// the same text satisfy a mail-address remote-id.
		der, key := cfmRSALeaf(t, 2048, pkix.Name{CommonName: mail}, []string{mail}, []string{mail})
		sa := ridCertSA(t, cfmCAName, mail)
		sa.PeerCfg.Auth.RemoteIDType = wire.IDTypeRFC822Addr
		sa.RemoteCertRaw = der
		ridAssert(sa, wire.IDTypeFQDN, []byte(mail))
		err := verifyRemoteAuth(sa, cfmRSAAuth(t, sa, key))
		if err == nil {
			t.Fatal("ID_FQDN satisfied a peer pinned to ID_RFC822_ADDR")
		}
		if !strings.Contains(err.Error(), "ID_RFC822_ADDR") {
			t.Errorf("the refusal %q does not name the pinned type", err)
		}

		// The SAME peer asserting the pinned type passes, so the pin is a filter and
		// not a blanket refusal.
		ok := ridCertSA(t, cfmCAName, mail)
		ok.PeerCfg.Auth.RemoteIDType = wire.IDTypeRFC822Addr
		ok.RemoteCertRaw = der
		ridAssert(ok, wire.IDTypeRFC822Addr, []byte(mail))
		if err := verifyRemoteAuth(ok, cfmRSAAuth(t, ok, key)); err != nil {
			t.Errorf("the pinned type was refused, so remote-id-type denies everything: %v", err)
		}
	})
}
