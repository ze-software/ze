package dnsserver

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/miekg/dns"

	"codeberg.org/thomas-mangin/ze/internal/core/selfcert"
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
func TestDoTListener(t *testing.T) {
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), echoHandler("10.0.0.7"), Options{})
	err := mgr.ApplyListeners(true, Listeners{
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

func mustPack(t *testing.T, name string, qtype uint16) []byte {
	t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	wire, err := m.Pack()
	if err != nil {
		t.Fatalf("pack query: %v", err)
	}
	return wire
}

// VALIDATES: AC-4 -- a Manager with a DoH endpoint answers both POST and GET
// (RFC 8484) with application/dns-message, driving the same shared handler.
func TestDoHListener(t *testing.T) {
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), echoHandler("10.0.0.8"), Options{})
	err := mgr.ApplyListeners(true, Listeners{
		DoH:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	})
	if err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)
	client := dohClient(roots)

	// POST
	postReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		dohURL(port, nil), bytes.NewReader(mustPack(t, "post.test", dns.TypeA)))
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
	getReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		dohURL(port, mustPack(t, "get.test", dns.TypeA)), http.NoBody)
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
	if err := mgr.ApplyListeners(true, Listeners{
		DoH:       []Endpoint{{IP: netip.MustParseAddr("127.0.0.1"), Port: port}},
		TLSConfig: srvTLS,
	}); err != nil {
		t.Fatalf("ApplyListeners: %v", err)
	}
	t.Cleanup(mgr.Stop)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		dohURL(port, nil), bytes.NewReader(mustPack(t, "drop.test", dns.TypeA)))
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
func TestDoHMethodNotAllowed(t *testing.T) {
	port := freePort(t)
	srvTLS, roots := testTLSPair(t)
	mgr := New(testLogger(), echoHandler("10.0.0.8"), Options{})
	if err := mgr.ApplyListeners(true, Listeners{
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
	err := mgr.ApplyListeners(true, Listeners{
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
