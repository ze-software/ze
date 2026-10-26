// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- peer identity and certificates
// Related: remote_id_test.go -- the remote identity checks over whole certificates
// Related: rfc9190_postauth_test.go -- the OCSP check and its fixtures
//
// VALIDATES: two RFC 5280 obligations Ze holds as a certificate user: a DNS name is
// compared case-insensitively and exactly, and obtaining revocation information from an
// https URI in a certificate never recurses into the responder's own certificate.
// PREVENTS: a peer identity that matches on a suffix or by Unicode folding, and an OCSP
// fetch that chases the responder's certificate URIs without end.

package engine

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ocsp"
)

// TestRFC5280DNSNamesCompareCaseInsensitiveAndExact checks the comparison the remote
// identity check uses between an ID_FQDN and a certificate's dNSName.
//
// Method: pairs that differ only in ASCII case compare equal, and pairs that differ by a
// label, by a character, or by a Unicode case fold compare unequal.
func TestRFC5280DNSNamesCompareCaseInsensitiveAndExact(t *testing.T) {
	// RFC requirement: RFC5280-7.2-2 positive -- DNS names that differ only in ASCII case
	// compare equal.
	for _, pair := range [][2]string{
		{"vpn.example.net", "VPN.EXAMPLE.NET"},
		{"Vpn.Example.Net", "vpn.example.net"},
	} {
		if !asciiEqualFold(pair[0], pair[1]) {
			t.Errorf("%q and %q differ only in case and compared unequal", pair[0], pair[1])
		}
	}
	// RFC requirement: RFC5280-7.2-2 negative -- the match is exact on the entire name:
	// an added label, a missing character, or a Unicode fold (the Kelvin sign for k) is
	// a different name.
	for _, pair := range [][2]string{
		{"a.vpn.example.net", "vpn.example.net"},
		{"vpn.example.net", "vpn.example.ne"},
		{"vpn.example.net", "vpn.example.nex"},
		{"kvpn.example.net", "Kvpn.example.net"},
	} {
		if asciiEqualFold(pair[0], pair[1]) {
			t.Errorf("%q and %q are different names and compared equal", pair[0], pair[1])
		}
	}
}

// relyingPartyResponderTLS mints a TLS certificate for 127.0.0.1, signed by ca, whose
// authorityInfoAccess names next as its own OCSP responder.
func relyingPartyResponderTLS(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, next string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the responder key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(91),
		Subject:      pkix.Name{CommonName: "ocsp-responder"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		OCSPServer:   []string{next},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create the responder certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// TestRFC5280OCSPFetchDoesNotRecurseIntoTheResponderCertificate checks that validating
// the https responder's own certificate obtains nothing further.
//
// Method: the IKE authenticator's leaf names responder A, an https server. A's TLS
// certificate in turn names responder B in its authorityInfoAccess, and B counts every
// request. The check runs with ocspClient, Ze's own client, given only a root pool that
// trusts A's issuer. A answers good, and B must see no request: Ze validates A's
// certificate without obtaining what A's certificate points to, so the chain of URIs ends
// at its first link.
//
// RFC requirement: RFC5280-8-1 positive -- validating the https OCSP responder's server
// certificate does not fetch the OCSP URI that certificate names: one check reaches the
// first responder only, so no recursion can start.
func TestRFC5280OCSPFetchDoesNotRecurseIntoTheResponderCertificate(t *testing.T) {
	ca, _, caKey := crlWiringCA(t, "postauth-ca")

	var nextHits atomic.Int64
	next := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextHits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(next.Close)

	answering := ocspResponder(t, ca, caKey, postauthSerial, ocsp.Good)
	first := httptest.NewUnstartedServer(answering.Config.Handler)
	first.TLS = &tls.Config{
		Certificates: []tls.Certificate{relyingPartyResponderTLS(t, ca, caKey, next.URL)},
		MinVersion:   tls.VersionTLS12,
	}
	first.StartTLS()
	t.Cleanup(first.Close)

	leaf, _, _ := postauthLeaf(t, "postauth-node", ca, caKey, first.URL)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	client := ocspClient()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("ocspClient's transport is %T, want *http.Transport", client.Transport)
	}
	transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}

	status, err := checkServerChainStatus(context.Background(),
		[][]*x509.Certificate{{leaf, ca}}, client, time.Now())
	if err != nil {
		t.Fatalf("the check failed against the https responder: %v", err)
	}
	if status != statusGood {
		t.Fatalf("the check reported status %d, want the good status", status)
	}
	if hits := nextHits.Load(); hits != 0 {
		t.Errorf("validating the responder's certificate fetched the URI it names %d times, want none", hits)
	}
}
