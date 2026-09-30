// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS inside IKE_AUTH
// Related: rfc9190_postauth_test.go, rfc9190_crl_wiring_test.go -- the EAP-TLS PKI fixtures; child.go -- createFirstChildSA.
package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// dataCipherEAPTLS runs a complete EAP-TLS exchange, Ze as the authenticator, and
// returns the authenticator session holding the TLS session it negotiated. The
// handshake is TLS 1.3, whose every ciphersuite is an AEAD (AES-GCM, AES-CCM or
// ChaCha20-Poly1305, RFC 8446 Appendix B.4); none of them names AES-CBC or HMAC.
func dataCipherEAPTLS(t *testing.T) *eap.Session {
	t.Helper()
	ca, caDER, caKey := crlWiringCA(t, "data-cipher-ca")
	_, serverDER, serverKey := crlWiringLeaf(t, "data-cipher-server", 71, ca, caKey)
	_, clientDER, clientKey := crlWiringLeaf(t, "data-cipher-client", 72, ca, caKey)
	crlPEM := postauthCRLPEM(t, ca, caKey)

	sess, err := eap.NewSession(eap.TypeTLS, eap.MethodConfig{
		ServerCertPEM: certPEM(t, serverDER),
		ServerKeyPEM:  keyPEM(t, serverKey),
		CACertPEM:     certPEM(t, caDER),
		CRLPEM:        crlPEM,
		Resumption:    eap.NewResumption(time.Now, true),
	})
	if err != nil {
		t.Fatalf("create the authenticator session: %v", err)
	}
	peer := eap.NewPeerSessionTLS("eap-tls-client", &eap.PeerTLSConfig{
		CertPEM:   certPEM(t, clientDER),
		KeyPEM:    keyPEM(t, clientKey),
		CACertPEM: certPEM(t, caDER),
		CRLPEM:    crlPEM,
	})
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	req := sess.Begin()
	for range postauthRounds {
		res := peer.Process(req)
		if res.Err != nil {
			t.Fatalf("the peer failed the exchange: %v", res.Err)
		}
		if res.Done || res.Response == nil {
			break
		}
		next := sess.Process(res.Response)
		if next == nil {
			break
		}
		req = next
	}
	if !sess.Succeeded() {
		t.Fatal("the EAP-TLS exchange did not succeed")
	}
	return sess
}

// dataCipherChild installs the first Child SA of an IKE SA that authenticated its
// peer with that EAP-TLS session, as responder_eap.go leaves it (EAPSession and
// EAPMSK set), under the ESP proposal given, and returns the outbound SA installed.
func dataCipherChild(t *testing.T, sess *eap.Session, proposal ipsec.ESPProposal) (encAlgo, authAlgo string, aead bool) {
	t.Helper()
	_, resp, _ := establishPSK(t)
	resp.EAPSession = sess
	resp.EAPMSK = sess.MSK()
	group := testESPGroup()
	group.Proposals = []ipsec.ESPProposal{proposal}
	dp := &mockDP{}
	if _, err := createFirstChildSA(resp, group, resp.PeerCfg.LocalAddress, resp.PeerCfg.RemoteAddress,
		7, dp, slogutil.DiscardLogger()); err != nil {
		t.Fatalf("install the first Child SA: %v", err)
	}
	if len(dp.sas) == 0 {
		t.Fatal("no Child SA was installed")
	}
	out := dp.sas[len(dp.sas)-1]
	return out.EncAlgo, out.AuthAlgo, out.IsAEAD
}

// RFC requirement: RFC5216-2.4-4 positive -- "Since the ciphersuite negotiated within
// EAP-TLS applies only to the EAP conversation, TLS ciphersuite negotiation MUST NOT be
// used to negotiate the ciphersuites used to secure data." (rfc/full/rfc5216.txt,
// Section 2.4). After a real EAP-TLS authentication, which negotiated a TLS 1.3 AEAD
// suite, the ESP SA Ze installs for the data carries the transforms of the ESP proposal
// (AES-256-CBC with HMAC-SHA-256, not an AEAD), which no TLS 1.3 suite names.
//
// VALIDATES: the data ciphers come from the IKE Child SA negotiation, not from TLS.
// PREVENTS: an installed ESP transform taken from the EAP-TLS ciphersuite.
func TestRFC5216DataCipherComesFromTheESPProposal(t *testing.T) {
	sess := dataCipherEAPTLS(t)
	enc, auth, aead := dataCipherChild(t, sess, ipsec.ESPProposal{
		Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256,
	})
	if aead {
		t.Fatalf("installed ESP is an AEAD (%s), but the ESP proposal named AES-256-CBC with HMAC-SHA-256", enc)
	}
	if enc != "aes256" || auth != "sha256" {
		t.Fatalf("installed ESP = %s/%s, want the proposal's aes256/sha256", enc, auth)
	}
}

// RFC requirement: RFC5216-2.4-4 negative -- "Since the ciphersuite negotiated within
// EAP-TLS applies only to the EAP conversation, TLS ciphersuite negotiation MUST NOT be
// used to negotiate the ciphersuites used to secure data." (rfc/full/rfc5216.txt,
// Section 2.4). The input is forced toward the violation: one EAP-TLS session, whose
// negotiated suite is fixed, authenticates two IKE SAs whose ESP proposals differ. The
// installed data ciphers differ with the proposals, so the TLS session pins nothing.
//
// VALIDATES: the TLS session does not choose, or constrain, the ESP transforms.
// PREVENTS: data ciphers that follow the EAP-TLS negotiation whatever IKE agreed.
func TestRFC5216TLSSessionDoesNotChooseTheDataCipher(t *testing.T) {
	sess := dataCipherEAPTLS(t)
	encCBC, authCBC, aeadCBC := dataCipherChild(t, sess, ipsec.ESPProposal{
		Number: 1, Encryption: ipsec.EncryptionAES256, Hash: ipsec.HashSHA256,
	})
	encOther, authOther, aeadOther := dataCipherChild(t, sess, ipsec.ESPProposal{
		Number: 1, Encryption: ipsec.EncryptionAES128, Hash: ipsec.HashSHA512,
	})
	if aeadCBC || aeadOther {
		t.Fatalf("an AEAD was installed (aead=%v/%v) for proposals that named none", aeadCBC, aeadOther)
	}
	if encCBC == encOther && authCBC == authOther {
		t.Fatalf("both IKE SAs installed %s/%s under one TLS session, although their ESP proposals differ",
			encCBC, authCBC)
	}
	if encOther != "aes128" || authOther != "sha512" {
		t.Fatalf("installed ESP = %s/%s, want the second proposal's aes128/sha512", encOther, authOther)
	}
}
