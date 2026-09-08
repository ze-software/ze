package engine

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: after the Child SA is up, ze re-checks the revocation status of the
// authenticator certificate its EAP-TLS peer accepted, over https, and brings the
// SA down when the responder reports it revoked (RFC 9190 Section 5.4, RFC 5216
// Section 5.4).
// PREVENTS: a certificate withdrawn while a tunnel is up staying trusted until
// the next authentication, and the check reaching a responder over a transport
// the RFC forbids.

// postauthRounds bounds the EAP-TLS conversation these tests drive. A completed
// exchange takes a handful of rounds and the bound stops a wedged harness from
// hanging the package.
const postauthRounds = 20

// ocspResponder is an https OCSP responder that answers every request with the
// status named, signed by the CA whose key it holds.
//
// It is an httptest TLS server, so its own certificate is trusted by the client
// it hands out and by nothing else. That is what the check is given, and it is
// why these tests prove the fetch went over TLS rather than over anything else.
func ocspResponder(
	t *testing.T,
	ca *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	serial int64,
	status int,
) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/ocsp-request" {
			t.Errorf("the responder received Content-Type %q, want application/ocsp-request", ct)
		}
		template := ocsp.Response{
			Status:       status,
			SerialNumber: big.NewInt(serial),
			ThisUpdate:   time.Now().Add(-time.Hour),
			NextUpdate:   time.Now().Add(time.Hour),
		}
		if status == ocsp.Revoked {
			template.RevokedAt = time.Now().Add(-time.Minute)
			template.RevocationReason = ocsp.KeyCompromise
		}
		der, err := ocsp.CreateResponse(ca, ca, template, caKey)
		if err != nil {
			t.Errorf("the responder could not sign its answer: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/ocsp-response")
		if _, err := w.Write(der); err != nil {
			t.Errorf("the responder could not write its answer: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// postauthSerial is the serial number of every certificate this file asks a
// responder about. One number keeps the certificate, the responder's answer and
// the assertion naming the same subject.
const postauthSerial int64 = 55

// postauthLeaf issues an end-entity certificate naming one OCSP responder URL,
// which is the authority information access extension the check reads.
func postauthLeaf(
	t *testing.T,
	cn string,
	ca *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	responderURL string,
) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(postauthSerial),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		OCSPServer:   []string{responderURL},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create the leaf certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse the leaf certificate: %v", err)
	}
	return cert, der, key
}

// postauthSA drives one real EAP-TLS exchange with the two config builders the
// daemon uses, and answers an SA holding the peer session that exchange
// produced.
//
// It is a REAL handshake rather than a hand-built chain, because what this file
// has to prove is that the chain the EAP-TLS peer accepted is the chain the
// later check reads. A chain assembled in the test would prove the check and
// skip the join.
func postauthSA(t *testing.T, serverCfg eap.MethodConfig, peerCfg *eap.PeerTLSConfig) *SA {
	t.Helper()

	sess, err := eap.NewSession(eap.TypeTLS, serverCfg)
	if err != nil {
		t.Fatalf("create the authenticator session: %v", err)
	}
	peer := eap.NewPeerSessionTLS("eap-tls-client", peerCfg)
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	req := sess.Begin()
	done := false
	for range postauthRounds {
		res := peer.Process(req)
		if res.Err != nil {
			t.Fatalf("the peer failed the exchange: %v", res.Err)
		}
		if res.Done {
			done = true
			break
		}
		if res.Response == nil {
			break
		}
		next := sess.Process(res.Response)
		if next == nil {
			break
		}
		req = next
	}
	if !done {
		t.Fatal("the EAP-TLS exchange did not complete, so there is no accepted chain to re-check")
	}

	return &SA{
		PeerName:    "branch",
		IsInitiator: true,
		PeerCfg:     ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{Mode: ipsec.AuthEAPTLS}},
		EAPSession:  peer,
	}
}

// postauthEnv is one completed EAP-TLS authentication plus the https client that
// reaches the responder the authenticator's certificate names.
type postauthEnv struct {
	sa     *SA
	client *http.Client
}

// postauthCRLPEM issues an empty revocation list from the CA, as PEM.
//
// The handshake below is TLS 1.3, and RFC 9190 Section 5.4 makes the revocation
// check mandatory there, so both roles need a source. A list that revokes
// nothing is the answer a CA publishes while it has withdrawn nothing, and it
// leaves the post-authentication check as the only thing these tests measure.
func postauthCRLPEM(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: time.Now().Add(-time.Hour),
		NextUpdate: time.Now().Add(time.Hour),
	}, ca, caKey)
	if err != nil {
		t.Fatalf("create the revocation list: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "X509 CRL", Bytes: der})
}

// certPEM and keyPEM encode what the EAP configs take.
func certPEM(t *testing.T, der []byte) []byte {
	t.Helper()
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func keyPEM(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal the private key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// newPostauthEnv authenticates one EAP-TLS peer against an authenticator whose
// certificate names an https OCSP responder, and answers what the check needs.
func newPostauthEnv(t *testing.T, status int) postauthEnv {
	t.Helper()
	ca, caDER, caKey := crlWiringCA(t, "postauth-ca")
	srv := ocspResponder(t, ca, caKey, postauthSerial, status)
	_, serverDER, serverKey := postauthLeaf(t, "postauth-server", ca, caKey, srv.URL)
	_, clientDER, clientKey := crlWiringLeaf(t, "postauth-client", 56, ca, caKey)

	crlPEM := postauthCRLPEM(t, ca, caKey)
	serverCfg := eap.MethodConfig{
		ServerCertPEM: certPEM(t, serverDER),
		ServerKeyPEM:  keyPEM(t, serverKey),
		CACertPEM:     certPEM(t, caDER),
		CRLPEM:        crlPEM,
		Resumption:    eap.NewResumption(time.Now, true),
	}
	peerCfg := &eap.PeerTLSConfig{
		CertPEM:   certPEM(t, clientDER),
		KeyPEM:    keyPEM(t, clientKey),
		CACertPEM: certPEM(t, caDER),
		CRLPEM:    crlPEM,
	}

	return postauthEnv{sa: postauthSA(t, serverCfg, peerCfg), client: srv.Client()}
}

// TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked drives a
// complete EAP-TLS exchange and then asks the authenticator's own responder
// about the certificate it presented.
//
// RFC requirement: RFC9190-5.4-4 positive -- the peer checks for certificate
// revocation after authentication completes: with the Child SA up and a
// responder reporting the authenticator certificate revoked, the check reports a
// verdict, which is what the owner loop turns into a closed SA.
//
// RFC requirement: RFC5216-5.4-2 positive -- the same behavior is what RFC 5216
// Section 5.4 asks for, with no TLS version attached: the status is re-read after
// the handshake rather than only during it.
func TestPostAuthenticationCheckClosesTheSAWhenTheResponderReportsRevoked(t *testing.T) {
	env := newPostauthEnv(t, ocsp.Revoked)

	recheck := startServerCertRecheck(env.sa, env.client, slogutil.DiscardLogger())
	if recheck == nil {
		t.Fatal("no check was started for an SA whose EAP-TLS peer accepted a chain")
	}
	<-recheck.done

	select {
	case err := <-recheck.verdict():
		if err == nil {
			t.Fatal("the check reported a verdict with no reason")
		}
	default:
		t.Fatal("the responder reported the authenticator certificate revoked and the check reported no verdict")
	}
}

// TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood is the
// counterpart of the test above.
//
// RFC requirement: RFC9190-5.4-4 negative -- a check that closed every SA would
// satisfy the positive test above and make every tunnel unusable. With the same
// responder reporting the certificate good, the check finishes and reports no
// verdict, so the SA stays up.
//
// RFC requirement: RFC5216-5.4-2 negative -- the same, for the obligation RFC
// 5216 Section 5.4 states without a version condition.
func TestPostAuthenticationCheckLeavesTheSAUpWhenTheResponderReportsGood(t *testing.T) {
	env := newPostauthEnv(t, ocsp.Good)

	recheck := startServerCertRecheck(env.sa, env.client, slogutil.DiscardLogger())
	if recheck == nil {
		t.Fatal("no check was started for an SA whose EAP-TLS peer accepted a chain")
	}
	<-recheck.done

	select {
	case err := <-recheck.verdict():
		t.Fatalf("the check closed an SA whose responder reported the certificate good: %v", err)
	default:
	}
}

// TestPostAuthenticationCheckIsNotStartedWithoutAnEAPTLSPeerSession pins the two
// states that have nothing to check.
//
// A responder-role SA validated the client chain inside its own handshake and
// needs no network for it, and a password EAP method presents no certificate at
// all. Neither is an error, and neither may start a worker.
func TestPostAuthenticationCheckIsNotStartedWithoutAnEAPTLSPeerSession(t *testing.T) {
	log := slogutil.DiscardLogger()

	if r := startServerCertRecheck(&SA{PeerName: "branch"}, ocspClient(), log); r != nil {
		t.Fatal("a check was started for an SA with no EAP session")
	}

	sa := &SA{PeerName: "branch", EAPSession: eap.NewPeerSession(eap.TypeMSCHAPv2, "user", "secret")}
	if r := startServerCertRecheck(sa, ocspClient(), log); r != nil {
		t.Fatal("a check was started for an SA whose EAP method presents no certificate")
	}
}

// TestPostAuthenticationCheckRefusesAnInsecureResponderURL points the
// certificate's authority information access at an http responder.
//
// RFC requirement: RFC9190-5.4-5 positive -- "An EAP peer MUST use a secure
// transport to verify the revocation status of the server certificate." The
// check refuses the http url before it opens any connection, so it obtains no
// answer and reports the status unchecked with a reason naming the transport.
func TestPostAuthenticationCheckRefusesAnInsecureResponderURL(t *testing.T) {
	ca, _, caKey := crlWiringCA(t, "postauth-ca")
	leaf, _, _ := postauthLeaf(t, "postauth-node", ca, caKey, "http://ocsp.example.com/")

	status, err := checkServerChainStatus(context.Background(),
		[][]*x509.Certificate{{leaf, ca}}, ocspClient(), time.Now())

	if status != statusUnchecked {
		t.Fatalf("the check reported status %d for an http responder, want the unchecked status", status)
	}
	if err == nil {
		t.Fatal("the check refused the http responder without recording a reason")
	}
	if !errors.Is(err, errOCSPScheme) {
		t.Fatalf("the check refused for %q, which does not name the insecure transport", err)
	}
}

// TestPostAuthenticationCheckReadsAnHTTPSResponder is the counterpart of the
// test above.
//
// RFC requirement: RFC9190-5.4-5 negative -- a check that refused every
// responder url would satisfy the positive test above and never verify anything.
// The same certificate, naming an https responder, is asked and answered.
func TestPostAuthenticationCheckReadsAnHTTPSResponder(t *testing.T) {
	ca, _, caKey := crlWiringCA(t, "postauth-ca")
	srv := ocspResponder(t, ca, caKey, postauthSerial, ocsp.Good)
	leaf, _, _ := postauthLeaf(t, "postauth-node", ca, caKey, srv.URL)

	status, err := checkServerChainStatus(context.Background(),
		[][]*x509.Certificate{{leaf, ca}}, srv.Client(), time.Now())

	if err != nil {
		t.Fatalf("the check failed against an https responder: %v", err)
	}
	if status != statusGood {
		t.Fatalf("the check reported status %d after an https responder answered good, want the good status", status)
	}
}

// TestPostAuthenticationCheckReportsUncheckedWithNoResponder pins the third
// outcome: a certificate that names no responder at all leaves the status
// unchecked rather than good.
func TestPostAuthenticationCheckReportsUncheckedWithNoResponder(t *testing.T) {
	ca, _, caKey := crlWiringCA(t, "postauth-ca")
	leaf, _, _ := crlWiringLeaf(t, "postauth-node", postauthSerial, ca, caKey)

	status, err := checkServerChainStatus(context.Background(),
		[][]*x509.Certificate{{leaf, ca}}, ocspClient(), time.Now())

	if status != statusUnchecked {
		t.Fatalf("the check reported status %d for a certificate naming no responder, want the unchecked status", status)
	}
	if !errors.Is(err, errOCSPNoResponder) {
		t.Fatalf("the check reported %q, which does not name the missing responder", err)
	}
}

// TestServerCertRecheckStopReleasesAFetchInFlight drives the check against a
// responder that never answers, and stops it.
//
// The worker is a goroutine with an owner and a stop path, and stop is the path:
// it cancels the request in flight and returns only once the worker has ended. A
// test that hung here would be reporting a leaked goroutine for every SA that
// ends while its responder is slow.
func TestServerCertRecheckStopReleasesAFetchInFlight(t *testing.T) {
	ca, _, caKey := crlWiringCA(t, "postauth-ca")

	released := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-released
	}))
	t.Cleanup(func() {
		close(released)
		srv.Close()
	})

	leaf, _, _ := postauthLeaf(t, "postauth-node", ca, caKey, srv.URL)
	r := &serverCertRecheck{revoked: make(chan error, 1), done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.run(ctx, [][]*x509.Certificate{{leaf, ca}}, "branch", srv.Client(), slogutil.DiscardLogger())

	r.stop()

	select {
	case err := <-r.verdict():
		t.Fatalf("a canceled check reported a verdict: %v", err)
	default:
	}
}
