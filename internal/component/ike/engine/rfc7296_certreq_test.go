package engine

// RFC naming: untagged -- tests Ze's configured-CA CERTREQ request policy and PSK omission for strongSwan interoperability; no whole RFC requirement is claimed.

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // the CERTREQ authority is SHA-1 (RFC 7296 Section 3.7)
	"testing"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// certReqsOf answers every CERTREQ payload of a decrypted payload chain.
func certReqsOf(inner []wire.PayloadEntry) []*wire.PayloadCERTREQ {
	var out []*wire.PayloadCERTREQ
	for i := range inner {
		if req, ok := inner[i].Payload.(*wire.PayloadCERTREQ); ok {
			out = append(out, req)
		}
	}
	return out
}

// VALIDATES: an X.509 initiator that holds a CA for its peer asks for the peer's
// certificate in its first IKE_AUTH request, naming that CA by the SHA-1 of its
// public key. RFC 7296 Section 3.7: CERTREQ "can appear in the IKE_INIT_SA response
// and/or the IKE_AUTH request".
// PREVENTS: the home-CPE lab's mgmt tunnel never coming up. strongSwan sends its
// certificate only when asked (`send_cert = ifasked`, ike_cert_post.c build_certs
// checks COND_CERTREQ_SEEN), so a request without CERTREQ got an IKE_AUTH response
// without CERT, and getRemoteCert refused it with "no remote certificate received".
// The request is decrypted with the responder's keys and walked, so the test reads
// what goes on the wire, not what the builder returned.
func TestX509AuthRequestCarriesCertReq(t *testing.T) {
	wpcChain(t, 0)
	ini, resp, _ := establishPSK(t)
	ini.PeerCfg.Auth = wpcAuth()

	raw, err := buildAuthRequest(ini)
	if err != nil {
		t.Fatalf("buildAuthRequest: %v", err)
	}
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt IKE_AUTH request: %v", err)
	}
	reqs := certReqsOf(inner)
	if len(reqs) != 1 {
		t.Fatalf("the X.509 IKE_AUTH request carries %d CERTREQ payloads, want 1", len(reqs))
	}
	ca := pki.GetCA(wpcCAName)
	if ca == nil {
		t.Fatalf("the fixture CA %s is not loaded", wpcCAName)
	}
	want := sha1.Sum(ca.Certificate.RawSubjectPublicKeyInfo) //nolint:gosec // RFC 7296 Section 3.7
	if reqs[0].CertEncoding != wire.CertEncodingX509Sig {
		t.Errorf("CERTREQ encoding %d, want %d", reqs[0].CertEncoding, wire.CertEncodingX509Sig)
	}
	if !bytes.Equal(reqs[0].CertAuthority, want[:]) {
		t.Errorf("CERTREQ names authority %x, want the configured CA's key hash %x", reqs[0].CertAuthority, want)
	}
}

// VALIDATES: a pre-shared-secret initiator, which holds no CA, sends no CERTREQ.
// PREVENTS: asking a PSK peer for a certificate nothing would verify.
func TestPSKAuthRequestCarriesNoCertReq(t *testing.T) {
	ini, resp, _ := establishPSK(t)

	raw, err := buildAuthRequest(ini)
	if err != nil {
		t.Fatalf("buildAuthRequest: %v", err)
	}
	inner, err := decryptAndParse(resp, parseMsg(t, raw), raw)
	if err != nil {
		t.Fatalf("decrypt IKE_AUTH request: %v", err)
	}
	if reqs := certReqsOf(inner); len(reqs) != 0 {
		t.Fatalf("the PSK IKE_AUTH request carries %d CERTREQ payloads, want none", len(reqs))
	}
}
