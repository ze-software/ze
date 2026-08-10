package dnsserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/core/selfcert"
)

// testTLSPair returns a server tls.Config (self-signed, valid for 127.0.0.1) and
// a matching client root pool, so tests verify the certificate properly instead
// of disabling verification.
func testTLSPair(t *testing.T) (*tls.Config, *x509.CertPool) {
	t.Helper()
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames("127.0.0.1", []string{"localhost"}, time.Hour)
	if err != nil {
		t.Fatalf("generate cert: %v", err)
	}
	srv, err := selfcert.NewTLSConfig(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("server tls config: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPEM) {
		t.Fatalf("append cert to pool")
	}
	return srv, pool
}

func hostPort(port uint16) string {
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port)))
}

// VALIDATES: AC-3 -- a Manager with a DoT endpoint serves DNS-over-TLS (RFC
// 7858) to a real TLS client using the shared handler, and a verifying client
// (real cert check, no InsecureSkipVerify) succeeds.
// RFC requirement: RFC7858-3.1-1 positive -- the server listens on and accepts a TCP+TLS connection on its DoT port and answers (default DoT port 853, secure.go:39).
// RFC requirement: RFC7858-3.1-5 positive -- the client initiates a TLS handshake as the first data exchange (Net "tcp-tls"), then the query is answered.
// RFC requirement: RFC7858-3.1-6 positive -- (server role) the DoT port transports TLS-wrapped DNS rather than cleartext; the TLS exchange succeeds.
// RFC requirement: RFC7858-3.1-8 positive -- the server responds over the established TLS session; a TLS-wrapped query on the DoT port is answered.
// RFC requirement: RFC7858-3.3-1 positive -- the exchange uses the RFC 1035 4.2.2 two-octet length-prefixed framing (miekg/dns over the TLS listener); a mis-framed message would not round-trip.
// RFC requirement: RFC7858-8-1 positive -- a certificate-verifying client negotiating TLS 1.2 (the BCP 195 floor) completes the handshake and is answered.
func TestDoTListener(t *testing.T) {
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), echoHandler("10.0.0.7"), Options{})
	err := mgr.applyListeners(true, Listeners{
		DoT:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	})
	if err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("example.test"), dns.TypeA)
	c := &dns.Client{
		Net:       "tcp-tls",
		Timeout:   3 * time.Second,
		TLSConfig: &tls.Config{RootCAs: roots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12},
	}
	resp, _, err := c.Exchange(m, hostPort(port))
	if err != nil {
		t.Fatalf("DoT exchange: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok || a.A.String() != "10.0.0.7" {
		t.Fatalf("unexpected answer %v", resp.Answer[0])
	}
}

func dohClient(roots *x509.CertPool) *http.Client {
	return &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: roots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12},
		},
	}
}

func dohURL(port uint16, query []byte) string {
	u := url.URL{Scheme: "https", Host: hostPort(port), Path: DefaultDoHPath}
	if query != nil {
		q := u.Query()
		q.Set("dns", base64.RawURLEncoding.EncodeToString(query))
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func mustPack(t *testing.T, name string) []byte {
	t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeA)
	wire, err := m.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	return wire
}

// VALIDATES: AC-4 -- a Manager with a DoH endpoint answers both POST and GET
// (RFC 8484) with application/dns-message, driving the same shared handler.
// RFC requirement: RFC8484-4.1-2 positive -- the server implements BOTH POST and GET; each returns a well-formed application/dns-message answer.
// RFC requirement: RFC8484-5-1 positive -- the DoH endpoint is served over the https URI scheme (a TLS listener) and a certificate-verifying HTTPS client succeeds.
func TestDoHListener(t *testing.T) {
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), echoHandler("10.0.0.8"), Options{})
	err := mgr.applyListeners(true, Listeners{
		DoH:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	})
	if err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)
	client := dohClient(roots)

	// POST
	// RFC requirement: RFC8484-4.2-1 positive -- a POST carrying Content-Type application/dns-message is processed into a DNS answer.
	// RFC requirement: RFC8484-6-4 positive -- the POST body is the raw wire-format DNS message used directly (no base64 encoding), and the server answers it.
	postReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		dohURL(port, nil), bytes.NewReader(mustPack(t, "post.test")))
	if err != nil {
		t.Fatalf("new POST request: %v", err)
	}
	postReq.Header.Set("Content-Type", dohContentType)
	resp, err := client.Do(postReq)
	if err != nil {
		t.Fatalf("DoH POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	assertDoHAnswer(t, resp, "10.0.0.8")

	// GET
	// RFC requirement: RFC8484-6-2 positive -- the GET query carries the wire message base64url-encoded in the "dns" variable and is answered.
	// RFC requirement: RFC8484-6-3 positive -- the "dns" value is base64url WITHOUT padding (RawURLEncoding); the compliant unpadded request is accepted.
	getReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		dohURL(port, mustPack(t, "get.test")), http.NoBody)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("DoH GET: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	assertDoHAnswer(t, getResp, "10.0.0.8")
}

func assertDoHAnswer(t *testing.T, resp *http.Response, wantIP string) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// RFC requirement: RFC8484-5.4-1 positive -- the DoH response body uses the application/dns-message media type.
	if ct := resp.Header.Get("Content-Type"); ct != dohContentType {
		t.Fatalf("content-type = %q, want %q", ct, dohContentType)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	m := new(dns.Msg)
	if err := m.Unpack(body); err != nil {
		t.Fatalf("unpack response: %v", err)
	}
	if len(m.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(m.Answer))
	}
	a, ok := m.Answer[0].(*dns.A)
	if !ok || a.A.String() != wantIP {
		t.Fatalf("unexpected answer %v", m.Answer[0])
	}
}

// VALIDATES: a DoH query the handler declines to answer (send=false) maps to
// HTTP 403, not a hung connection.
func TestDoHRefusedYields403(t *testing.T) {
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	// drop never writes a reply (models as112 allow-from denial).
	drop := dns.HandlerFunc(func(_ dns.ResponseWriter, _ *dns.Msg) {})
	mgr := New(testLogger(), drop, Options{})
	if err := mgr.applyListeners(true, Listeners{
		DoH:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	}); err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		dohURL(port, nil), bytes.NewReader(mustPack(t, "drop.test")))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", dohContentType)
	resp, err := dohClient(roots).Do(req)
	if err != nil {
		t.Fatalf("DoH POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

// VALIDATES: an unsupported HTTP method returns 405.
// RFC requirement: RFC8484-4.1-2 negative -- a method that is neither POST nor GET (PUT) is rejected with 405, so "implement both methods" is not vacuously met by accepting everything.
func TestDoHMethodNotAllowed(t *testing.T) {
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), echoHandler("10.0.0.8"), Options{})
	if err := mgr.applyListeners(true, Listeners{
		DoH:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	}); err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, dohURL(port, nil), http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := dohClient(roots).Do(req)
	if err != nil {
		t.Fatalf("DoH PUT: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
}

// VALIDATES: the plain UDP+TCP path is unchanged and no-ops on an identical
// re-apply even through ApplyListeners (regression guard for the refactor).
func TestApplyListenersPlainNoop(t *testing.T) {
	port := freePort(t)
	mgr := New(testLogger(), echoHandler("10.0.0.9"), Options{})
	eps := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}}
	if err := mgr.Apply(true, eps); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	t.Cleanup(mgr.Stop)
	// Second identical Apply is a no-op (same signature): the query still works.
	if err := mgr.Apply(true, eps); err != nil {
		t.Fatalf("Apply (2nd): %v", err)
	}
	resp := exchangeA(t, "udp", hostPort(port), "noop.test")
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}
}

// VALIDATES: rotating the serving certificate changes the listener signature so
// a subsequent ApplyListeners rebinds instead of no-opping.
func TestListenerSigCertRotation(t *testing.T) {
	certA, _ := testTLSPair(t)
	certB, _ := testTLSPair(t)
	eps := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: 853}}
	sigA := listenersSig(true, Listeners{DoT: eps, TLSConfig: certA, DoHPath: DefaultDoHPath})
	sigB := listenersSig(true, Listeners{DoT: eps, TLSConfig: certB, DoHPath: DefaultDoHPath})
	if sigA == sigB {
		t.Fatalf("expected distinct signatures for rotated certs, both = %q", sigA)
	}
	sigSame := listenersSig(true, Listeners{DoT: eps, TLSConfig: certA, DoHPath: DefaultDoHPath})
	if sigA != sigSame {
		t.Fatalf("same cert produced different signatures: %q vs %q", sigA, sigSame)
	}
}

// VALIDATES: AC-3/AC-4 -- ApplyWithSecure brings up DoT via the shared cert
// helper with a self-signed fallback, the plain path still answers, and a
// second identical apply reuses the cached cert (no rebind churn) rather than
// regenerating it.
func TestApplyWithSecureSelfSigned(t *testing.T) {
	plainPort := freePort(t)
	dotPort := freePort(t)
	mgr := New(testLogger(), echoHandler("10.0.0.5"), Options{})
	plain := []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: plainPort}}
	sc := SecureConfig{DoTEnabled: true, DoTPort: dotPort, DoHPath: DefaultDoHPath}
	if err := mgr.ApplyWithSecure(true, plain, sc, testLogger()); err != nil {
		t.Fatalf("ApplyWithSecure: %v", err)
	}
	t.Cleanup(mgr.Stop)
	if mgr.selfSigned == nil {
		t.Fatalf("expected a cached self-signed certificate")
	}
	first := mgr.selfSigned

	// Build a client root pool from the cached self-signed cert and query DoT.
	pool := x509.NewCertPool()
	leaf, err := x509.ParseCertificate(mgr.selfSigned.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatalf("parse cached cert: %v", err)
	}
	pool.AddCert(leaf)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("sec.test"), dns.TypeA)
	c := &dns.Client{
		Net:       "tcp-tls",
		Timeout:   3 * time.Second,
		TLSConfig: &tls.Config{RootCAs: pool, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12},
	}
	resp, _, err := c.Exchange(m, hostPort(dotPort))
	if err != nil {
		t.Fatalf("DoT exchange via ApplyWithSecure: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 DoT answer, got %d", len(resp.Answer))
	}

	// Second identical apply: cert must not be regenerated (churn guard).
	if err := mgr.ApplyWithSecure(true, plain, sc, testLogger()); err != nil {
		t.Fatalf("ApplyWithSecure (2nd): %v", err)
	}
	if mgr.selfSigned != first {
		t.Fatalf("self-signed certificate regenerated on reload (rebind churn)")
	}
	plainResp := exchangeA(t, "udp", hostPort(plainPort), "sec-plain.test")
	if len(plainResp.Answer) != 1 {
		t.Fatalf("plain listener stopped answering, got %d", len(plainResp.Answer))
	}
}

// VALIDATES: AC-3/AC-4 -- ParseSecureLeaves reads the tls/doh containers from a
// plugin config node, applying native-mirror port validation and defaults.
func TestParseSecureLeaves(t *testing.T) {
	node := map[string]any{
		"tls": map[string]any{
			"enabled":     "true",
			"listen-port": "8853",
			"cert-file":   "/c.pem",
			"key-file":    "/k.pem",
		},
		"doh": map[string]any{
			"enabled":     "true",
			"listen-port": "8443",
			"path":        "/q",
		},
	}
	sc := DefaultSecureConfig()
	if err := ParseSecureLeaves(node, &sc, "test"); err != nil {
		t.Fatalf("ParseSecureLeaves: %v", err)
	}
	if !sc.DoTEnabled || sc.DoTPort != 8853 || sc.CertFile != "/c.pem" || sc.KeyFile != "/k.pem" {
		t.Fatalf("DoT parse wrong: %+v", sc)
	}
	if !sc.DoHEnabled || sc.DoHPort != 8443 || sc.DoHPath != "/q" {
		t.Fatalf("DoH parse wrong: %+v", sc)
	}
}

// VALIDATES: defaults are 853/443//dns-query and a missing node leaves them.
// RFC requirement: RFC7858-3.1-1 positive -- the default DoT listen port is 853 (the IANA "domain-s" port), so an unconfigured DoT server listens on 853.
func TestDefaultSecureConfig(t *testing.T) {
	sc := DefaultSecureConfig()
	if sc.DoTPort != DefaultDoTPort || sc.DoHPort != DefaultDoHPort || sc.DoHPath != DefaultDoHPath {
		t.Fatalf("defaults wrong: %+v", sc)
	}
	if err := ParseSecureLeaves(map[string]any{}, &sc, "test"); err != nil {
		t.Fatalf("empty node: %v", err)
	}
	if sc.DoTEnabled || sc.DoHEnabled {
		t.Fatalf("empty node enabled a listener: %+v", sc)
	}
}

// VALIDATES: boundary -- a zero port is rejected by ParseSecureLeaves.
func TestParseSecureLeavesPortZero(t *testing.T) {
	sc := DefaultSecureConfig()
	if err := ParseSecureLeaves(map[string]any{"tls": map[string]any{"listen-port": "0"}}, &sc, "test"); err == nil {
		t.Fatal("tls port 0 accepted, want rejected")
	}
	sc = DefaultSecureConfig()
	if err := ParseSecureLeaves(map[string]any{"doh": map[string]any{"listen-port": "0"}}, &sc, "test"); err == nil {
		t.Fatal("doh port 0 accepted, want rejected")
	}
}

// VALIDATES: DoT/DoH requested without TLS material does not error the whole
// apply; secure listeners are skipped but plain ones still come up.
func TestSecureWithoutTLSSkipsSecure(t *testing.T) {
	port := freePort(t)
	mgr := New(testLogger(), echoHandler("10.0.0.9"), Options{})
	err := mgr.applyListeners(true, Listeners{
		Plain: []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		DoT:   []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: freePort(t)}},
	})
	if err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)
	resp := exchangeA(t, "udp", hostPort(port), "plainonly.test")
	if len(resp.Answer) != 1 {
		t.Fatalf("expected plain listener to answer, got %d answers", len(resp.Answer))
	}
}

// startDoH brings up a DoH-only Manager serving handler on a fresh port and
// returns the bound port plus a certificate-verifying HTTPS client. It reuses
// the same listener setup as TestDoHListener so the RFC 8484 request/response
// tests below do not each re-spell it.
func startDoH(t *testing.T, handler dns.Handler) (uint16, *http.Client) {
	t.Helper()
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), handler, Options{})
	if err := mgr.applyListeners(true, Listeners{
		DoH:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	}); err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)
	return port, dohClient(roots)
}

// dohPost issues a DoH POST to the default path with the given Content-Type
// (skipped when empty) and raw body.
func dohPost(t *testing.T, client *http.Client, port uint16, contentType string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		dohURL(port, nil), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new POST request: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DoH POST: %v", err)
	}
	return resp
}

// dohGet issues a DoH GET to a fully-formed URL.
func dohGet(t *testing.T, client *http.Client, rawURL string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		t.Fatalf("new GET request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DoH GET: %v", err)
	}
	return resp
}

// VALIDATES: a POST whose Content-Type is not application/dns-message is
// rejected with 415 rather than processed, pinning "process
// application/dns-message requests" to that one media type.
// RFC requirement: RFC8484-4.2-1 negative -- a POST body sent with a non application/dns-message Content-Type is refused (415), not processed as a DNS query.
// RFC requirement: RFC8484-5.4-1 negative -- text/plain is not honored as the DoH media type; only application/dns-message is supported.
func TestDoHRejectsWrongContentType(t *testing.T) {
	port, client := startDoH(t, echoHandler("10.0.0.8"))
	resp := dohPost(t, client, port, "text/plain", mustPack(t, "wrongct.test"))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", resp.StatusCode)
	}
}

// VALIDATES: a GET missing the "dns" variable, and a GET whose "dns" value is
// not valid base64url, are each rejected with 400 -- the base64url "dns"
// variable is required and validated, not merely tolerated.
// RFC requirement: RFC8484-6-2 negative -- a GET missing the "dns" variable, or carrying a non-base64url value, is rejected (400) rather than answered.
func TestDoHGetRejectsBadDNSParam(t *testing.T) {
	port, client := startDoH(t, echoHandler("10.0.0.8"))

	// Missing dns variable.
	missing := url.URL{Scheme: "https", Host: hostPort(port), Path: DefaultDoHPath}
	resp := dohGet(t, client, missing.String())
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing dns: status = %d, want 400", resp.StatusCode)
	}

	// Present but not valid base64url ('*' is outside the base64url alphabet).
	bad := url.URL{Scheme: "https", Host: hostPort(port), Path: DefaultDoHPath}
	q := bad.Query()
	q.Set("dns", "not*valid*base64url")
	bad.RawQuery = q.Encode()
	badResp := dohGet(t, client, bad.String())
	defer func() { _ = badResp.Body.Close() }()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid base64url: status = %d, want 400", badResp.StatusCode)
	}
}

// VALIDATES: a GET whose "dns" value carries base64 padding ('=') is rejected
// with 400 -- the decoder is RawURLEncoding, which forbids padding, so a padded
// value cannot be answered.
// RFC requirement: RFC8484-6-3 negative -- a "dns" value that INCLUDES base64url padding ('=') is rejected (400); padded encodings are not accepted.
func TestDoHGetRejectsPaddedDNSParam(t *testing.T) {
	port, client := startDoH(t, echoHandler("10.0.0.8"))
	query := mustPack(t, "padded.test")
	padded := base64.URLEncoding.EncodeToString(query) // URLEncoding, unlike RawURLEncoding, appends '=' padding.
	if !strings.Contains(padded, "=") {
		t.Fatalf("expected a padded encoding to contain '=', got %q", padded)
	}
	u := url.URL{Scheme: "https", Host: hostPort(port), Path: DefaultDoHPath}
	q := u.Query()
	q.Set("dns", padded)
	u.RawQuery = q.Encode()
	resp := dohGet(t, client, u.String())
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a padded dns value", resp.StatusCode)
	}
}

// VALIDATES: the Cache-Control max-age of a DoH response equals the smallest
// TTL in the Answer section, so the HTTP freshness lifetime never exceeds it.
// RFC requirement: RFC8484-5.1-1 positive -- with answers at TTL 300 and 30, the response's Cache-Control max-age equals the smallest Answer-section TTL (30).
func TestDoHCacheControlMatchesSmallestTTL(t *testing.T) {
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.Answer = append(m.Answer,
			&dns.A{Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300}, A: net.ParseIP("10.0.0.1")},
			&dns.A{Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 30}, A: net.ParseIP("10.0.0.2")},
		)
		_ = w.WriteMsg(m)
	})
	port, client := startDoH(t, handler)
	resp := dohPost(t, client, port, dohContentType, mustPack(t, "ttl.test"))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Cache-Control"); got != "max-age=30" {
		t.Fatalf("Cache-Control = %q, want %q (smallest Answer TTL)", got, "max-age=30")
	}
}

// VALIDATES: a DoH query advertising a tiny EDNS UDP payload size (512) whose
// reply far exceeds it still returns the FULL response, untruncated -- the DoH
// path ignores the advertised EDNS UDP size rather than shrinking or setting TC.
// RFC requirement: RFC8484-6-1 positive -- a query advertising EDNS UDP size 512 with a >512-byte reply returns every answer with TC=0; the advertised UDP size is ignored.
func TestDoHIgnoresEDNSUDPSize(t *testing.T) {
	const answers = 50
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		for i := range answers {
			m.Answer = append(m.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.IPv4(10, 0, 0, byte(i)),
			})
		}
		_ = w.WriteMsg(m)
	})
	port, client := startDoH(t, handler)

	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn("ednssize.test"), dns.TypeA)
	q.SetEdns0(512, false) // advertise a tiny UDP payload size the server must ignore
	wire, err := q.Pack()
	if err != nil {
		t.Fatalf("pack EDNS query: %v", err)
	}
	resp := dohPost(t, client, port, dohContentType, wire)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) <= 512 {
		t.Fatalf("response is %d bytes; expected it to exceed the advertised 512-byte UDP size", len(body))
	}
	m := new(dns.Msg)
	if err := m.Unpack(body); err != nil {
		t.Fatalf("unpack response: %v", err)
	}
	if m.Truncated {
		t.Fatalf("response has TC=1; the DoH path must not truncate on the advertised EDNS UDP size")
	}
	if len(m.Answer) != answers {
		t.Fatalf("got %d answers, want %d (full response, untruncated)", len(m.Answer), answers)
	}
}

// startDoT brings up a DoT-only Manager serving handler on a fresh port and
// returns the bound port plus a client root pool that verifies the server cert.
func startDoT(t *testing.T, handler dns.Handler) (uint16, *x509.CertPool) {
	t.Helper()
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), handler, Options{})
	if err := mgr.applyListeners(true, Listeners{
		DoT:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	}); err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)
	return port, roots
}

// VALIDATES: the DoT port is TLS-only. A cleartext DNS-over-TCP query (RFC 1035
// length-prefixed, no TLS handshake) sent to the DoT listener is never answered:
// the server reads the first bytes as a TLS record, the handshake fails, and no
// cleartext DNS response comes back.
// RFC requirement: RFC7858-3.1-5 negative -- the first data exchange must be a TLS handshake; a connection whose first bytes are cleartext DNS (not a ClientHello) gets no DNS answer.
// RFC requirement: RFC7858-3.1-6 negative -- (server role) the DoT port is not used to carry cleartext DNS; a cleartext query to it is not answered.
// RFC requirement: RFC7858-3.1-8 negative -- the server does not respond to a cleartext DNS message on the DoT port, including after the failed TLS handshake.
func TestDoTRefusesCleartext(t *testing.T) {
	port, _ := startDoT(t, echoHandler("10.0.0.7"))

	// Dial raw TCP (no TLS) and send a length-prefixed cleartext query, exactly as
	// an RFC 7766 DNS-over-TCP client would.
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.DialContext(context.Background(), "tcp", hostPort(port))
	if err != nil {
		t.Fatalf("dial DoT port: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	q := new(dns.Msg)
	q.SetQuestion(dns.Fqdn("cleartext.test"), dns.TypeA)
	wire, err := q.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	framed := make([]byte, 2+len(wire))
	binary.BigEndian.PutUint16(framed[:2], uint16(len(wire)))
	copy(framed[2:], wire)
	if _, werr := conn.Write(framed); werr != nil {
		// A write failure (server already dropped the non-TLS connection) is itself
		// proof the cleartext query was refused.
		return
	}

	// Whatever comes back must NOT be a well-formed DNS reply to our query: a TLS
	// server answers a non-ClientHello with a TLS alert or closes the connection.
	buf := make([]byte, 512)
	n, rerr := conn.Read(buf)
	if rerr != nil {
		return // connection closed / errored: no cleartext answer, as required.
	}
	if n > 2 {
		msg := new(dns.Msg)
		if uerr := msg.Unpack(buf[2:n]); uerr == nil && msg.Id == q.Id && len(msg.Question) > 0 {
			t.Fatalf("server answered a cleartext DNS query on the DoT port (%d bytes); it must not", n)
		}
	}
}

// VALIDATES: the DoT listener enforces the BCP 195 TLS floor -- a client that
// offers only TLS 1.1 (below the TLS 1.2 minimum ze configures, secure.go via
// selfcert.NewTLSConfig MinVersion) fails the handshake and gets no answer,
// while the TLS 1.2 client in TestDoTListener succeeds.
// RFC requirement: RFC7858-8-1 negative -- a client capped below TLS 1.2 (max TLS 1.1) is refused, so the BCP 195 "TLS 1.2 or higher" floor is enforced, not merely documented.
func TestDoTRejectsBelowTLS12(t *testing.T) {
	port, roots := startDoT(t, echoHandler("10.0.0.7"))

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("oldtls.test"), dns.TypeA)
	c := &dns.Client{
		Net:     "tcp-tls",
		Timeout: 3 * time.Second,
		TLSConfig: &tls.Config{
			RootCAs:    roots,
			ServerName: "127.0.0.1",
			MinVersion: tls.VersionTLS10,
			MaxVersion: tls.VersionTLS11,
		},
	}
	if _, _, err := c.Exchange(m, hostPort(port)); err == nil {
		t.Fatal("DoT exchange with a TLS 1.1-capped client succeeded; the server must enforce the TLS 1.2 minimum (BCP 195)")
	}
}

// VALIDATES: the DoT server is robust to an idle connection being terminated by
// the client. After a client opens a DoT connection, queries, and abruptly
// closes it while idle, the server keeps serving and answers a fresh connection.
// RFC requirement: RFC7858-3.4-5 positive -- a client that abruptly terminates an idle DoT connection does not disturb the server; a subsequent connection is answered.
func TestDoTRobustToIdleConnectionClose(t *testing.T) {
	port, roots := startDoT(t, echoHandler("10.0.0.7"))

	tlsCfg := &tls.Config{RootCAs: roots, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	c := &dns.Client{Net: "tcp-tls", Timeout: 3 * time.Second, TLSConfig: tlsCfg}

	// First connection: query, then abruptly close the now-idle connection.
	conn, err := c.Dial(hostPort(port))
	if err != nil {
		t.Fatalf("dial DoT: %v", err)
	}
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("idle1.test"), dns.TypeA)
	if _, _, err := c.ExchangeWithConn(m, conn); err != nil {
		t.Fatalf("first DoT exchange: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close idle conn: %v", err)
	}

	// The server must still answer a brand-new connection.
	m2 := new(dns.Msg)
	m2.SetQuestion(dns.Fqdn("idle2.test"), dns.TypeA)
	resp, _, err := c.Exchange(m2, hostPort(port))
	if err != nil {
		t.Fatalf("DoT exchange after idle-connection close: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer after reconnect, got %d", len(resp.Answer))
	}
}
