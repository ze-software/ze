package engine

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 7296 Section 3.6 mandates SHA-1 as the hash-and-url object identifier
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/slogutil"
)

const (
	wpcCAName   = "wpc-root"
	wpcCertName = "wpc-device"
	wpcIdentity = "gw.wpc.example"
)

// wpcChain builds a root and a device certificate separated by `depth` intermediates, and
// loads them into the PKI store the way pki config.go records them. It returns the DER of
// the whole path, leaf first, which is exactly the order RFC 7296 Section 3.6 requires on
// the wire.
//
// depth 3 gives the four-certificate maximum the section names.
func wpcChain(t *testing.T, depth int) (chain [][]byte) {
	t.Helper()

	newKey := func(what string) *ecdsa.PrivateKey {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("generate the %s key: %v", what, err)
		}
		return key
	}
	sign := func(what string, tmpl, parent *x509.Certificate, pub any, parentKey *ecdsa.PrivateKey) ([]byte, *x509.Certificate) {
		der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, pub, parentKey)
		if err != nil {
			t.Fatalf("create the %s certificate: %v", what, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			t.Fatalf("parse the %s certificate: %v", what, err)
		}
		return der, cert
	}
	caTmpl := func(serial int64, cn string) *x509.Certificate {
		return &x509.Certificate{
			SerialNumber:          big.NewInt(serial),
			Subject:               pkix.Name{CommonName: cn},
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(time.Hour),
			IsCA:                  true,
			BasicConstraintsValid: true,
			KeyUsage:              x509.KeyUsageCertSign,
		}
	}

	rootKey := newKey("root")
	rootTmplV := caTmpl(1, "wpc root")
	rootDER, rootCert := sign("root", rootTmplV, rootTmplV, &rootKey.PublicKey, rootKey)

	// Build downward from the root. issuerCert/issuerKey walk with each level.
	issuerCert, issuerKey := rootCert, rootKey
	inters := make([]*x509.Certificate, 0, depth)
	interDERs := make([][]byte, 0, depth)
	for i := range depth {
		key := newKey("intermediate")
		tmpl := caTmpl(int64(10+i), "wpc intermediate "+string(rune('A'+i)))
		der, cert := sign("intermediate", tmpl, issuerCert, &key.PublicKey, issuerKey)
		inters = append(inters, cert)
		interDERs = append(interDERs, der)
		issuerCert, issuerKey = cert, key
	}

	leafKey := newKey("leaf")
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(99),
		Subject:      pkix.Name{CommonName: wpcIdentity},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{wpcIdentity},
	}
	leafDER, leafCert := sign("leaf", leafTmpl, issuerCert, &leafKey.PublicKey, issuerKey)

	// pki records the intermediates from the issuer of the leaf outward, which is the
	// order the wire wants: leaf, its issuer, that issuer's issuer.
	orderedInters := make([]*x509.Certificate, 0, depth)
	orderedDERs := make([][]byte, 0, depth)
	for i := depth - 1; i >= 0; i-- {
		orderedInters = append(orderedInters, inters[i])
		orderedDERs = append(orderedDERs, interDERs[i])
	}

	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			wpcCAName: {Name: wpcCAName, Certificate: rootCert, Raw: rootDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			wpcCertName: {
				Name: wpcCertName, Certificate: leafCert, Raw: leafDER, PrivateKey: leafKey,
				Intermediates: orderedInters, RawIntermediates: orderedDERs,
			},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})

	chain = append(chain, leafDER)
	chain = append(chain, orderedDERs...)
	return chain
}

// wpcAuth is the X.509 configuration the fixture peers share.
func wpcAuth() ipsec.AuthConfig {
	return ipsec.AuthConfig{
		Mode:          ipsec.AuthX509,
		Certificate:   wpcCertName,
		CACertificate: wpcCAName,
		LocalID:       wpcIdentity,
		RemoteID:      wpcIdentity,
	}
}

// wpcCertPayloads turns DER blobs into received CERT payloads carrying encoding 4.
func wpcCertPayloads(ders ...[]byte) []*wire.PayloadCERT {
	out := make([]*wire.PayloadCERT, 0, len(ders))
	for _, der := range ders {
		out = append(out, &wire.PayloadCERT{CertEncoding: wire.CertEncodingX509Sig, CertData: der})
	}
	return out
}

func wpcQuietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// wpcServer serves one fixed body over http and counts the requests it answered. The
// counter is what proves a control acted BEFORE any connection was made.
func wpcServer(t *testing.T, body []byte) (srv *httptest.Server, hits *int) {
	t.Helper()
	count := 0
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count++
		if _, err := w.Write(body); err != nil {
			t.Errorf("serve the certificate body: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &count
}

// wpcHashAndURL builds the CertData of a Hash and URL payload (RFC 7296 Section 3.6). The
// CertData is the 20-octet SHA-1 of the replaced structure, followed by the URL that
// resolves to it.
func wpcHashAndURL(hash []byte, rawURL string) []byte {
	return append(append([]byte{}, hash...), rawURL...)
}

// wpcFreshCache empties the process-global lookup cache for the duration of one test.
//
// Without it a lookup test is answered by whatever an earlier test cached under the same
// hash, and it passes without a fetch ever happening. That is not hypothetical. The
// hash-comparison row below went green against a cache entry the loopback-allowance row
// had stored. That would have shipped a vacuous proof of the single most important control
// in this file.
func wpcFreshCache(t *testing.T) {
	t.Helper()
	resetCertURLCache()
	t.Cleanup(resetCertURLCache)
	t.Cleanup(func() { wpcAwaitIdleFetcher(t) })
}

// wpcAwaitCached blocks until the background worker has cached the object named by hash.
//
// It waits on the CONDITION, not on a duration (ai/rules/completion.md). The lookup
// is real network I/O to an httptest server. A fixed sleep makes a load-sensitive test.
func wpcAwaitCached(t *testing.T, hash []byte) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := lookupHashAndURL(hash); ok {
			return
		}
		time.Sleep(2 * time.Millisecond) // poll interval; the loop returns as soon as the cache holds the object
	}
	t.Fatal("the background hash-and-url worker never cached the object")
}

// wpcAwaitIdleFetcher blocks until no background lookup is running. One test's worker then
// cannot write the shared cache while the next test asserts on it.
func wpcAwaitIdleFetcher(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if certURLFetches.inFlight() == 0 {
			return
		}
		time.Sleep(2 * time.Millisecond) // poll interval; the loop returns as soon as the pending set drains
	}
	t.Error("a background hash-and-url worker never finished")
}

// VALIDATES: RFC7296-3.6-1. Ze can be configured to SEND four X.509 certificates and to
// ACCEPT four, and the bound on each is the operator's certificate-count.
// PREVENTS: the two opposite failures this row had. The send side could hold ONE
// intermediate, so no configuration reached three or four. The accept side had NO bound at
// all. A missing bound is not "configurable to accept up to four": it accepted four by
// accident of having no limit. It would have parsed a thousand into an x509.CertPool just
// as readily.
// RFC requirement: RFC7296-3.6-1 positive -- buildCertPayloads emits the leaf plus every
// configured intermediate, reaching four, and storeRemoteCerts accepts four in wire order.
// RFC requirement: RFC7296-3.6-1 negative -- a fifth payload is refused, and the SAME four
// are refused under certificate-count 2, so the bound is read from config and is not a constant.
func TestCcnCertificateCountReachesFourInBothDirections(t *testing.T) {
	chain := wpcChain(t, 3)
	if len(chain) != 4 {
		t.Fatalf("the fixture built a chain of %d, want 4", len(chain))
	}

	t.Run("send", func(t *testing.T) {
		sa := testSAWithKeys(t)
		sa.PeerCfg.Auth = wpcAuth()

		entries, err := buildCertPayloads(sa)
		if err != nil {
			t.Fatalf("buildCertPayloads: %v", err)
		}
		if len(entries) != 4 {
			t.Fatalf("buildCertPayloads emitted %d payloads, want the leaf and three intermediates",
				len(entries))
		}
		for i, e := range entries {
			cert, ok := e.Payload.(*wire.PayloadCERT)
			if !ok {
				t.Fatalf("payload %d is %T, not a CERT payload", i, e.Payload)
			}
			if cert.CertEncoding != wire.CertEncodingX509Sig {
				t.Errorf("payload %d carries encoding %d, want %d",
					i, cert.CertEncoding, wire.CertEncodingX509Sig)
			}
			if !bytes.Equal(cert.CertData, chain[i]) {
				t.Errorf("payload %d is not chain position %d", i, i)
			}
		}
	})

	t.Run("accept", func(t *testing.T) {
		sa := testSAWithKeys(t)
		sa.PeerCfg.Auth = wpcAuth()

		if err := storeRemoteCerts(sa, wpcCertPayloads(chain...), wpcQuietLogger()); err != nil {
			t.Fatalf("a four-certificate chain was refused: %v", err)
		}
		if !bytes.Equal(sa.RemoteCertRaw, chain[0]) {
			t.Error("the peer certificate is not the FIRST payload, so AUTH verifies against an issuer")
		}
		if len(sa.RemoteCertChainRaw) != 3 {
			t.Fatalf("the stored chain holds %d intermediates, want 3", len(sa.RemoteCertChainRaw))
		}
		for i, got := range sa.RemoteCertChainRaw {
			if !bytes.Equal(got, chain[i+1]) {
				t.Errorf("stored intermediate %d is not wire position %d", i, i+1)
			}
		}
	})
}

// VALIDATES: RFC7296-3.6-1. The received chain bound refuses, never truncates, and the
// limit it applies comes from certificate-count rather than from a constant.
// PREVENTS: a truncating cap. Truncation passes every count-based assertion while hiding
// from the operator that a limit was reached, and it makes WHICH certificates survive
// depend on the order an unauthenticated peer chose (ai/rules/protocol.md).
// RFC requirement: RFC7296-3.6-1 negative -- five payloads are refused at the default of
// four, four are refused at certificate-count 2, and the same four pass at 4.
func TestCcnCertificateCountIsBoundedAndConfigurable(t *testing.T) {
	chain := wpcChain(t, 3)
	log := wpcQuietLogger()

	t.Run("a fifth certificate is refused", func(t *testing.T) {
		sa := testSAWithKeys(t)
		sa.PeerCfg.Auth = wpcAuth()
		five := append(append([][]byte{}, chain...), chain[3])

		err := storeRemoteCerts(sa, wpcCertPayloads(five...), log)
		if err == nil {
			t.Fatal("a five-certificate chain was accepted at the default bound of four")
		}
		for _, want := range []string{"5", "4", "certificate-count"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal %q does not name %q", err, want)
			}
		}
		// Refusal, not truncation. A truncating implementation passes the count
		// assertion above and fails here.
		if len(sa.RemoteCertRaw) != 0 || len(sa.RemoteCertChainRaw) != 0 {
			t.Errorf("the refused chain was stored anyway: peer=%d intermediates=%d",
				len(sa.RemoteCertRaw), len(sa.RemoteCertChainRaw))
		}
	})

	t.Run("the bound is read from config", func(t *testing.T) {
		// The SAME four payloads, twice, differing only in certificate-count. Without
		// this pair a test asserting "five is refused" would pass against a hardcoded 4.
		low := testSAWithKeys(t)
		low.PeerCfg.Auth = wpcAuth()
		low.PeerCfg.Auth.CertificateCount = 2
		if err := storeRemoteCerts(low, wpcCertPayloads(chain...), log); err == nil {
			t.Error("four certificates were accepted under certificate-count 2, " +
				"so the bound is not read from config")
		}

		high := testSAWithKeys(t)
		high.PeerCfg.Auth = wpcAuth()
		high.PeerCfg.Auth.CertificateCount = 4
		if err := storeRemoteCerts(high, wpcCertPayloads(chain...), log); err != nil {
			t.Errorf("four certificates were refused under certificate-count 4, "+
				"so the bound is a blanket refusal rather than a filter: %v", err)
		}
	})
}

// VALIDATES: RFC7296-3.6-2. Both Hash and URL formats are reachable under configuration:
// encoding 12 for a lone certificate and encoding 13 for a bundle. Each carries the
// 20-octet SHA-1 of the structure it replaces, followed by the configured http URL.
// PREVENTS: a constant with no referent. CertEncodingHashURL was declared and never used,
// and encoding 13 was not declared at all. Ze therefore sent neither format and accepted
// neither.
// RFC requirement: RFC7296-3.6-2 positive -- buildCertPayloads emits encoding 12 and
// encoding 13 under hash-and-url, and the hash is SHA-1 over the exact structure replaced.
// RFC requirement: RFC7296-3.6-2 negative -- with the leaf absent ze emits encoding 4 only,
// advertises no HTTP_CERT_LOOKUP_SUPPORTED, and DROPS a received hash-and-url payload.
func TestChuBothHashAndURLFormatsAreConfigurable(t *testing.T) {
	const certURL = "http://pki.example/device.der"

	t.Run("encoding 12 replaces a lone certificate", func(t *testing.T) {
		chain := wpcChain(t, 0)
		sa := testSAWithKeys(t)
		sa.PeerCfg.Auth = wpcAuth()
		sa.PeerCfg.Auth.HashAndURL = true
		sa.PeerCfg.Auth.CertificateURL = certURL

		entries, err := buildCertPayloads(sa)
		if err != nil {
			t.Fatalf("buildCertPayloads: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("hash-and-url emitted %d payloads, want one", len(entries))
		}
		cert, ok := entries[0].Payload.(*wire.PayloadCERT)
		if !ok {
			t.Fatalf("payload is %T, not a CERT payload", entries[0].Payload)
		}
		if cert.CertEncoding != wire.CertEncodingHashURL {
			t.Fatalf("encoding = %d, want %d (Hash and URL of X.509 certificate)",
				cert.CertEncoding, wire.CertEncodingHashURL)
		}
		want := sha1.Sum(chain[0]) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
		if !bytes.Equal(cert.CertData[:wire.CertHashURLHashLen], want[:]) {
			t.Error("the 20-octet prefix is not the SHA-1 of the certificate it replaces")
		}
		if got := string(cert.CertData[wire.CertHashURLHashLen:]); got != certURL {
			t.Errorf("the URL that follows the hash is %q, want %q", got, certURL)
		}
	})

	t.Run("encoding 13 replaces a bundle", func(t *testing.T) {
		chain := wpcChain(t, 3)
		sa := testSAWithKeys(t)
		sa.PeerCfg.Auth = wpcAuth()
		sa.PeerCfg.Auth.HashAndURL = true
		sa.PeerCfg.Auth.CertificateURL = certURL

		entries, err := buildCertPayloads(sa)
		if err != nil {
			t.Fatalf("buildCertPayloads: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("hash-and-url emitted %d payloads, want one", len(entries))
		}
		cert, ok := entries[0].Payload.(*wire.PayloadCERT)
		if !ok {
			t.Fatalf("payload is %T, not a CERT payload", entries[0].Payload)
		}
		if cert.CertEncoding != wire.CertEncodingHashURLBundle {
			t.Fatalf("encoding = %d, want %d (Hash and URL of X.509 bundle)",
				cert.CertEncoding, wire.CertEncodingHashURLBundle)
		}
		bundle, err := encodeCertBundle(chain)
		if err != nil {
			t.Fatalf("encode the reference bundle: %v", err)
		}
		want := sha1.Sum(bundle) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
		if !bytes.Equal(cert.CertData[:wire.CertHashURLHashLen], want[:]) {
			t.Error("the 20-octet prefix is not the SHA-1 of the bundle it replaces")
		}
		// The bundle must round-trip to the same four certificates, in order, or the
		// URL resolves to something the peer cannot use.
		back, err := decodeCertBundle(bundle)
		if err != nil {
			t.Fatalf("decode the bundle ze would publish: %v", err)
		}
		if len(back) != 4 {
			t.Fatalf("the bundle holds %d certificates, want 4", len(back))
		}
		for i := range back {
			if !bytes.Equal(back[i], chain[i]) {
				t.Errorf("bundle position %d is not chain position %d", i, i)
			}
		}
	})
}

// VALIDATES: RFC7296-3.6-2. Hash and URL is OFF unless an operator turns it on, and with
// it off no lookup path is reachable at all.
// PREVENTS: the SSRF surface existing in the default configuration. Resolving a received
// payload fetches a URL chosen by a peer that is NOT yet authenticated, so the default is a
// security property rather than a preference. It is also the anti-vacuity guard for the
// positive above: it proves the encodings are caused by config and not emitted always.
// RFC requirement: RFC7296-3.6-2 negative -- with hash-and-url absent ze sends encoding 4,
// advertises no HTTP_CERT_LOOKUP_SUPPORTED, drops a received encoding-12 payload at the
// shared collection gate, and refuses to resolve one at the funnel.
// rfc-test-change-approved: 2026-07-31 owner standing approval for
// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, strengthening only. Mutation M13 (delete the
// hash-and-url gate inside resolveCertPayloads) left this test GREEN. The funnel assertion
// pointed at an unresolvable host, so the fetch failed on DNS and the refusal proved
// nothing about policy. The URL now names a live server whose bytes match the hash, with
// the destination allowance set, so a refusal can only be the gate.
func TestChuHashAndURLIsOffByDefault(t *testing.T) {
	wpcChain(t, 1)

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth = wpcAuth()
	if sa.PeerCfg.Auth.HashAndURL {
		t.Fatal("the fixture enabled hash-and-url, so this test proves nothing")
	}

	entries, err := buildCertPayloads(sa)
	if err != nil {
		t.Fatalf("buildCertPayloads: %v", err)
	}
	for i, e := range entries {
		cert, ok := e.Payload.(*wire.PayloadCERT)
		if !ok {
			t.Fatalf("payload %d is %T, not a CERT payload", i, e.Payload)
		}
		if cert.CertEncoding != wire.CertEncodingX509Sig {
			t.Errorf("payload %d carries encoding %d with hash-and-url off",
				i, cert.CertEncoding)
		}
	}

	if notify := hashAndURLNotify(sa); notify != nil {
		t.Error("ze advertised HTTP_CERT_LOOKUP_SUPPORTED with hash-and-url off, " +
			"so a conforming peer would send a payload ze must then fetch")
	}

	// The collection gate. Both IKE_AUTH walks call acceptedCertEncoding, so this is
	// the drop that keeps a non-conforming peer's payload out of the funnel.
	hashURL := &wire.PayloadCERT{
		CertEncoding: wire.CertEncodingHashURL,
		CertData:     wpcHashAndURL(make([]byte, wire.CertHashURLHashLen), "http://evil.example/x"),
	}
	if acceptedCertEncoding(sa, hashURL) {
		t.Error("a hash-and-url payload was collected with hash-and-url off")
	}
	bundleURL := &wire.PayloadCERT{
		CertEncoding: wire.CertEncodingHashURLBundle,
		CertData:     wpcHashAndURL(make([]byte, wire.CertHashURLHashLen), "http://evil.example/x"),
	}
	if acceptedCertEncoding(sa, bundleURL) {
		t.Error("a hash-and-url bundle payload was collected with hash-and-url off")
	}

	// The funnel gate. Even handed the payload directly, the resolver refuses rather
	// than fetching, so a gate that lived only in the collection loops is not enough.
	//
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, strengthening only. Mutation M13 (delete the
	// hash-and-url gate inside resolveCertPayloads) left the previous form of this
	// assertion GREEN. It named an unresolvable host, so the fetch failed on DNS and the
	// refusal proved nothing about policy.
	//
	// The URL now points at a LIVE server holding a certificate whose hash matches, and
	// the peer carries the loopback allowance. As a result, EVERY other control would let
	// this fetch succeed. A refusal here can therefore only be the gate.
	wpcFreshCache(t)
	reachable := wpcChain(t, 0)
	srv, hits := wpcServer(t, reachable[0])
	sum := sha1.Sum(reachable[0]) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
	live := &wire.PayloadCERT{
		CertEncoding: wire.CertEncodingHashURL,
		CertData:     wpcHashAndURL(sum[:], srv.URL),
	}
	off := testSAWithKeys(t)
	off.PeerCfg.Auth = wpcAuth()
	off.PeerCfg.Auth.CertificateURLAllow = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}
	if err := storeRemoteCerts(off, []*wire.PayloadCERT{live}, wpcQuietLogger()); err == nil {
		t.Error("the resolver accepted a hash-and-url payload with hash-and-url off, " +
			"so an unauthenticated peer could name a URL ze fetches")
	}
	if *hits != 0 {
		t.Errorf("ze contacted the peer's URL %d times with hash-and-url off, so the "+
			"pre-authentication fetch happens in the default configuration", *hits)
	}
	if len(off.RemoteCertRaw) != 0 {
		t.Error("a certificate fetched with hash-and-url off was stored on the SA")
	}

	// With the leaf ON, the same payload IS collected. Without this row an unconditional
	// refusal of encodings 12 and 13 would produce the same drop.
	on := testSAWithKeys(t)
	on.PeerCfg.Auth = wpcAuth()
	on.PeerCfg.Auth.HashAndURL = true
	on.PeerCfg.Auth.CertificateURL = "http://pki.example/device.der"
	if !acceptedCertEncoding(on, hashURL) {
		t.Error("a hash-and-url payload was dropped with hash-and-url ON, so the gate " +
			"is a blanket refusal rather than a config-driven one")
	}
	if hashAndURLNotify(on) == nil {
		t.Error("ze advertised no HTTP_CERT_LOOKUP_SUPPORTED with hash-and-url ON, " +
			"so a conforming peer never sends the format ze asked to support")
	}
}

// VALIDATES: RFC7296-3.6-3. Ze supports the http scheme for hash-and-URL lookup: a
// received encoding-12 payload naming an http URL is fetched, its SHA-1 is verified, and
// the resulting certificate is the one the peer named.
// PREVENTS: 3.6-2 going green on the send half alone while the LOOKUP -- which is what
// this row is about -- was never performed. certurl.go existed with no non-test caller, so
// no configuration made ze resolve a peer's certificate URL.
// RFC requirement: RFC7296-3.6-3 positive -- storeRemoteCerts resolves an http URL through
// fetchHashAndURL and stores the fetched DER as the peer certificate.
// RFC requirement: RFC7296-3.6-3 negative -- a scheme other than http is refused before any
// I/O, and bytes whose SHA-1 does not match the payload are refused before any parser sees them.
// rfc-test-change-approved: 2026-08-01 owner standing approval for
// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, strengthening only. The lookup no longer runs on
// the caller's goroutine (certurl.go, certURLFetcher), so the first call reports the object
// as pending and the RETRANSMISSION resolves it. This test now proves both halves: that the
// first call performs no fetch of its own, and that the object still arrives.
func TestChuHashURLLookupUsesHTTPAndVerifiesTheHash(t *testing.T) {
	wpcFreshCache(t)
	chain := wpcChain(t, 0)
	leafDER := chain[0]

	srv, hits := wpcServer(t, leafDER)

	// httptest binds 127.0.0.1, which the fetcher's deny list refuses on purpose. The
	// operator allowance is the supported way to permit a destination, and using it here
	// exercises that leaf as well.
	loopback := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

	sum := sha1.Sum(leafDER) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
	payload := &wire.PayloadCERT{
		CertEncoding: wire.CertEncodingHashURL,
		CertData:     wpcHashAndURL(sum[:], srv.URL),
	}

	sa := testSAWithKeys(t)
	sa.PeerCfg.Auth = wpcAuth()
	sa.PeerCfg.Auth.HashAndURL = true
	sa.PeerCfg.Auth.CertificateURL = "http://pki.example/device.der"
	sa.PeerCfg.Auth.CertificateURLAllow = loopback

	// rfc-test-change-approved: 2026-08-01 owner standing approval for
	// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, strengthening only.
	//
	// The first delivery finds nothing cached. It starts a worker and reports the object
	// as pending. It fetches NOTHING on this goroutine. An inline fetch here runs on the
	// shared dispatch goroutine that serves every IKE session.
	first := storeRemoteCerts(sa, []*wire.PayloadCERT{payload}, wpcQuietLogger())
	if !errors.Is(first, errCertURLPending) {
		t.Fatalf("the first delivery of an uncached hash-and-url payload returned %v, "+
			"want errCertURLPending; a lookup that answers inline blocks every other session", first)
	}
	if len(sa.RemoteCertRaw) != 0 {
		t.Error("a pending lookup wrote a peer certificate, so a later refusal cannot undo it")
	}

	// The peer retransmits (RFC 7296 Section 2.1). By then the worker has cached the object.
	wpcAwaitCached(t, sum[:])
	if err := storeRemoteCerts(sa, []*wire.PayloadCERT{payload}, wpcQuietLogger()); err != nil {
		t.Fatalf("the retransmitted http hash-and-url lookup failed: %v", err)
	}
	if *hits == 0 {
		t.Fatal("the server was never contacted, so no lookup was performed")
	}
	if !bytes.Equal(sa.RemoteCertRaw, leafDER) {
		t.Error("the stored peer certificate is not the DER the URL resolved to")
	}
	// The fetched bytes must reach x509 exactly as an inline payload would.
	if _, err := x509.ParseCertificate(sa.RemoteCertRaw); err != nil {
		t.Errorf("the fetched certificate does not parse: %v", err)
	}
}

// VALIDATES: RFC7296-3.6-3. Every control on the lookup holds, one row per control.
// PREVENTS: a bounded-looking fetcher that is missing a control. A single "a bad URL is
// refused" row passes while five of the seven controls are absent, so each is its own row
// (ai/rules/evidence.md).
// RFC requirement: RFC7296-3.6-3 negative -- the scheme allowlist, the size cap, the
// redirect refusal, the destination deny list and the hash comparison each refuse
// independently, and the hash is compared BEFORE any parser sees the bytes.
func TestChuHashURLLookupRefusesEverythingOutsideTheBound(t *testing.T) {
	wpcFreshCache(t)
	chain := wpcChain(t, 0)
	leafDER := chain[0]
	sum := sha1.Sum(leafDER) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
	loopback := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

	for _, scheme := range []string{"file:///etc/passwd", "ftp://host/x", "https://host/x"} {
		t.Run("scheme "+scheme+" is refused before any I/O", func(t *testing.T) {
			// RFC 7296 Section 3.6 makes http the required scheme and says other
			// schemes SHOULD NOT be used absent a document specifying them. https is
			// the most plausible one to add by accident, so it is pinned as refused.
			wpcFreshCache(t)
			_, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], scheme), loopback)
			if !errors.Is(err, errCertURLScheme) {
				t.Fatalf("%s was not refused by the scheme check: %v", scheme, err)
			}
		})
	}

	t.Run("an oversized body is refused", func(t *testing.T) {
		wpcFreshCache(t)
		srv, _ := wpcServer(t, make([]byte, certURLMaxBytes+4096))
		_, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], srv.URL), loopback)
		if !errors.Is(err, errCertURLTooLarge) {
			t.Fatalf("a body beyond the cap was not refused: %v", err)
		}
	})

	t.Run("a redirect is refused and the second server is never contacted", func(t *testing.T) {
		wpcFreshCache(t)
		second, secondHits := wpcServer(t, leafDER)
		first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, second.URL, http.StatusFound)
		}))
		t.Cleanup(first.Close)

		_, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], first.URL), loopback)
		if err == nil {
			t.Fatal("a redirect was followed")
		}
		if *secondHits != 0 {
			t.Errorf("the redirect target was contacted %d times, so the host check was "+
				"re-opened against a server the peer chose", *secondHits)
		}
	})

	t.Run("the metadata address is refused before connect", func(t *testing.T) {
		// rfc-test-change-approved: 2026-08-01 owner standing approval for
		// plan/learned/1313-rfcgate-1b-rfc7296-pilot.md, strengthening only.
		//
		// This row and the one below asserted err != nil only. 169.254.169.254 is
		// unroutable on a build host, and a loopback port can refuse a connection. Both
		// rows therefore passed on a NETWORK failure. Neither said anything about the deny
		// list. The fetch still fails with certURLDenied deleted, so a person CAN remove
		// the guard and neither row reports it. The refusal must name itself.
		//
		// No allowance, so the deny list governs.
		wpcFreshCache(t)
		_, err := fetchHashAndURL(t.Context(),
			wpcHashAndURL(sum[:], "http://169.254.169.254/latest/meta-data/"), nil)
		if !errors.Is(err, errCertURLBlocked) {
			t.Fatalf("the cloud metadata address returned %v, want errCertURLBlocked; a "+
				"routing or timeout failure is not the destination check firing", err)
		}
	})

	t.Run("loopback is refused without an operator allowance", func(t *testing.T) {
		wpcFreshCache(t)
		srv, _ := wpcServer(t, leafDER)
		_, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], srv.URL), nil)
		if !errors.Is(err, errCertURLBlocked) {
			t.Fatalf("a loopback destination with no certificate-url-allow entry returned "+
				"%v, want errCertURLBlocked", err)
		}
		// The same URL WITH the allowance succeeds, so the deny list is not a blanket
		// refusal of every destination.
		if _, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], srv.URL), loopback); err != nil {
			t.Fatalf("the operator allowance did not permit the destination: %v", err)
		}
	})

	t.Run("a valid but different certificate is refused by the hash", func(t *testing.T) {
		// This is the most important row. A fetcher that parsed first and compared the
		// hash afterwards passes every row above and still hands attacker-chosen bytes
		// to the X.509 parser.
		wpcFreshCache(t)
		other := wpcChain(t, 0)
		srv, _ := wpcServer(t, other[0])

		// The hash names the ORIGINAL certificate. The server serves a different one
		// that is itself perfectly valid DER.
		got, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], srv.URL), loopback)
		if !errors.Is(err, errCertURLHash) {
			t.Fatalf("bytes that do not match the peer's hash were accepted: %v", err)
		}
		if got != nil {
			t.Error("the fetcher returned bytes that failed the hash, so a caller could parse them")
		}
	})
}

// VALIDATES: RFC7296-3.6-3. The lookup cache is content-addressed. It is keyed by the
// SHA-1 the peer sent, never by the URL. A second fetch of the same object therefore costs
// no request, and a URL that changes what it serves cannot poison an entry.
// PREVENTS: a URL-keyed cache. RFC 7296 Section 3.6 names caching as the point of the
// feature. A URL key would let a server answer the first request honestly and be cached.
// Its entry would then be reused for a DIFFERENT hash later.
// RFC requirement: RFC7296-3.6-3 positive -- a repeat lookup of one hash is served from
// cache without contacting the server, and the bytes still satisfy the hash.
func TestChuHashURLLookupCacheIsContentAddressed(t *testing.T) {
	wpcFreshCache(t)
	chain := wpcChain(t, 0)
	leafDER := chain[0]
	sum := sha1.Sum(leafDER) //nolint:gosec // RFC 7296 Section 3.6 names SHA-1
	loopback := []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

	srv, hits := wpcServer(t, leafDER)

	first, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], srv.URL), loopback)
	if err != nil {
		t.Fatalf("the first lookup failed: %v", err)
	}
	if *hits != 1 {
		t.Fatalf("the first lookup made %d requests, want one", *hits)
	}

	// The SAME hash from a DIFFERENT URL. A content-addressed cache answers it without
	// any request at all, because the hash names the object rather than its location.
	second, err := fetchHashAndURL(t.Context(),
		wpcHashAndURL(sum[:], "http://other.example/elsewhere.der"), loopback)
	if err != nil {
		t.Fatalf("the cached lookup failed: %v", err)
	}
	if *hits != 1 {
		t.Errorf("the repeat lookup made %d requests in total, so the cache is not "+
			"keyed by the hash", *hits)
	}
	if !bytes.Equal(first, second) || !bytes.Equal(second, leafDER) {
		t.Error("the cached bytes are not the object the hash names")
	}

	// After a reset the object is fetched again. That proves the two assertions above were
	// about the CACHE, and not about a fetcher that only ever issues one request.
	resetCertURLCache()
	if _, err := fetchHashAndURL(t.Context(), wpcHashAndURL(sum[:], srv.URL), loopback); err != nil {
		t.Fatalf("the lookup after a cache reset failed: %v", err)
	}
	if *hits != 2 {
		t.Errorf("the post-reset lookup made %d requests in total, want two", *hits)
	}
}

// wpcRoleStates drives one IKE_AUTH carrying the given CERT payloads through BOTH roles
// and reports the state each SA reached.
//
// Both roles are driven because the chain bound is enforced in one place, storeRemoteCerts.
// Its ERROR is handled in two places: the initiator walk in fsm.go and the responder walk
// in responder.go. A test that drove only one role would leave the other call site free to
// drop the error. The peer would then carry on past a refused chain.
func wpcRoleStates(t *testing.T, auth ipsec.AuthConfig, ders ...[]byte) (responder, initiator SAState) {
	t.Helper()
	log := slogutil.DiscardLogger()

	ini, resp, ps := autSAInitPair(t, auth)
	raw := rccAuthRequest(t, ini, ders...)
	ps.handleAuthRequest(resp, parseMsg(t, raw), raw, nil, nil, log)
	responder = resp.State

	ini2, resp2, _ := autSAInitPair(t, auth)
	ini2.State = StateAuthSent
	raw2 := rccAuthResponse(t, resp2, ders...)
	handleAuthResponse(ini2, parseMsg(t, raw2), raw2, nil, nil, log)
	initiator = ini2.State

	return responder, initiator
}

// VALIDATES: RFC7296-3.6-1. A chain longer than certificate-count kills the SA on BOTH the
// responder and the initiator path. The bound is therefore enforced where a real IKE_AUTH
// reaches it, rather than only where a unit test calls the funnel directly.
// PREVENTS: the second-producer failure. storeRemoteCerts is one function and its error is
// handled at two call sites (fsm.go and responder.go). Dropping the error at either left
// every test in this package green. The bound existed, and one role walked past it
// (ai/rules/completion.md).
// RFC requirement: RFC7296-3.6-1 negative -- an over-long chain moves the SA to StateDead
// through the real IKE_AUTH walk, on both roles, and the same chain within the bound does not.
func TestCcnOverlongChainKillsTheSAOnBothRoles(t *testing.T) {
	chain := wpcChain(t, 3)
	auth := wpcAuth()

	// certificate-count 2 with a four-certificate chain, so the refusal is the BOUND
	// rather than a malformed message. The peer config is shared by both roles.
	bounded := auth
	bounded.CertificateCount = 2

	respState, iniState := wpcRoleStates(t, bounded, chain...)
	if respState != StateDead {
		t.Errorf("the responder reached %v on a chain of %d with certificate-count 2, "+
			"want StateDead: the refusal was dropped at the responder call site",
			respState, len(chain))
	}
	if iniState != StateDead {
		t.Errorf("the initiator reached %v on a chain of %d with certificate-count 2, "+
			"want StateDead: the refusal was dropped at the initiator call site",
			iniState, len(chain))
	}

	// The SAME message within the bound must NOT die, or the assertions above would pass
	// against a walk that kills every certificate-bearing IKE_AUTH.
	within := auth
	within.CertificateCount = 4
	okResp, okIni := wpcRoleStates(t, within, chain...)
	if okResp == StateDead {
		t.Error("the responder killed a chain of four with certificate-count 4, " +
			"so the bound refuses everything rather than what exceeds it")
	}
	if okIni == StateDead {
		t.Error("the initiator killed a chain of four with certificate-count 4, " +
			"so the bound refuses everything rather than what exceeds it")
	}

	// The discriminating half, and the reason this test drives a PSK peer as well.
	//
	// For an X.509 peer the SA dies either way. A refused chain stores nothing.
	// getRemoteCert therefore fails a moment later with "no remote certificate received",
	// and the state assertions above cannot tell a handled refusal from a dropped one.
	//
	// A PRE-SHARED-SECRET peer needs no certificate at all, so nothing downstream objects
	// to an empty chain. The refusal is the ONLY thing that can stop it. That makes this
	// the assertion that separates the two call sites handling the error from either of
	// them ignoring it.
	psk := ipsec.AuthConfig{
		Mode:             ipsec.AuthPreSharedSecret,
		PSK:              "wpc-shared-secret",
		LocalID:          wpcIdentity,
		RemoteID:         wpcIdentity,
		CertificateCount: 2,
	}
	pskResp, pskIni := wpcRoleStates(t, psk, chain...)
	if pskResp != StateDead {
		t.Errorf("a shared-key responder reached %v on a chain of %d with "+
			"certificate-count 2, so the responder call site ignored the refusal",
			pskResp, len(chain))
	}
	if pskIni != StateDead {
		t.Errorf("a shared-key initiator reached %v on a chain of %d with "+
			"certificate-count 2, so the initiator call site ignored the refusal",
			pskIni, len(chain))
	}
}
