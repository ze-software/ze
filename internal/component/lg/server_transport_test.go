// Design: docs/architecture/pki/tls-listeners.md -- each looking-glass listener serves HTTPS or HTTP

package lg

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"slices"
	"testing"
	"time"
)

// startTransportTestServer starts a looking glass on the given listeners and
// returns it once every one is bound. Empty PEM starts a server that holds no
// certificate, which NewLGServer accepts only when no listener serves TLS.
func startTransportTestServer(t *testing.T, certPEM, keyPEM []byte, listen ...ListenAddr) *LGServer {
	t.Helper()

	srv, err := NewLGServer(LGConfig{
		ListenAddrs: listen,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
		Dispatch:    mockDispatch(),
	})
	if err != nil {
		t.Fatalf("NewLGServer: %v", err)
	}

	go func() {
		_ = srv.ListenAndServe(context.Background())
	}()

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if err := srv.WaitReady(readyCtx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutCancel()
		if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
			t.Logf("shutdown: %v", shutErr)
		}
	})
	return srv
}

// insecureHTTPSClient returns a client that reads what a TLS listener answers
// without trusting its certificate: the tests assert on the answer.
func insecureHTTPSClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // the test asserts on the answer, not on trust
			DisableKeepAlives: true,
		},
	}
}

// plainHTTPClient returns a client that opens a new connection per request, so
// each request is accepted under the protocol the listener serves at that
// moment.
func plainHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{DisableKeepAlives: true},
	}
}

// statusOf returns the status of GET url, closing the body.
func statusOf(t *testing.T, client *http.Client, url string) int {
	t.Helper()

	resp := doGet(t, client, url)
	defer resp.Body.Close() //nolint:errcheck // test cleanup
	return resp.StatusCode
}

// requireNoTLS fails unless a TLS handshake with addr is refused, which is what
// a listener serving plain HTTP does to one.
func requireNoTLS(t *testing.T, addr, when string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // the test asserts that no handshake completes
		MinVersion:         tls.VersionTLS12,
	}}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("%s: a TLS handshake with %s completed, want a listener serving plain HTTP", when, addr)
	}
}

// freeLoopbackAddr returns a loopback address nothing listens on: it binds
// port 0, reads the port the kernel chose, and closes the listener.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("release the reserved port: %v", err)
	}
	return addr
}

const (
	statusPath  = "/api/looking-glass/status"
	rfc8522Path = "/.well-known/looking-glass/v1/routers"
)

func TestLGServerServesHTTPAndHTTPSAtOnce(t *testing.T) {
	// VALIDATES: one looking glass serves HTTP on one listener and HTTPS on
	// another at the same time, each answering on its own protocol, and the
	// RFC 8522 API is refused on the HTTP listener of that same server.
	// PREVENTS: one server-wide protocol, where a second listener silently takes
	// the first one's, and an RFC 8522 answer in the clear on a server that also
	// serves TLS.
	chainPEM, keyPEM := testChain(t, "both at once")
	srv := startTransportTestServer(t, chainPEM, keyPEM,
		ListenAddr{Addr: "127.0.0.1:0"},
		ListenAddr{Addr: "127.0.0.1:0", TLS: true},
	)
	addrs := srv.Addresses()
	if len(addrs) != 2 {
		t.Fatalf("bound %d listeners, want 2", len(addrs))
	}
	plainAddr, tlsAddr := addrs[0], addrs[1]

	if got := statusOf(t, plainHTTPClient(), "http://"+plainAddr+statusPath); got != http.StatusOK {
		t.Errorf("HTTP listener: status %d, want 200", got)
	}
	requireNoTLS(t, plainAddr, "the HTTP listener")

	if got := statusOf(t, insecureHTTPSClient(), "https://"+tlsAddr+statusPath); got != http.StatusOK {
		t.Errorf("HTTPS listener: status %d, want 200", got)
	}
	if name := handshakePeerCerts(t, tlsAddr)[0].Subject.CommonName; name != "both at once" {
		t.Errorf("HTTPS listener leaf: got %q, want the configured leaf", name)
	}

	// RFC 8522 Section 6 on each connection: HTTPS answers, HTTP is refused.
	if got := statusOf(t, insecureHTTPSClient(), "https://"+tlsAddr+rfc8522Path); got != http.StatusOK {
		t.Errorf("RFC 8522 over the HTTPS listener: status %d, want 200", got)
	}
	if got := statusOf(t, plainHTTPClient(), "http://"+plainAddr+rfc8522Path); got != http.StatusBadRequest {
		t.Errorf("RFC 8522 over the HTTP listener: status %d, want 400", got)
	}

	want := map[string]bool{plainAddr: false, tlsAddr: true}
	got := srv.ListenerTLS()
	if len(got) != len(want) || got[plainAddr] || !got[tlsAddr] {
		t.Errorf("ListenerTLS: got %v, want %v", got, want)
	}
}

func TestLGServerTLSListenerRequiresCertificate(t *testing.T) {
	// VALIDATES: a server with one TLS listener among HTTP ones still needs the
	// certificate, and a server with none starts without one.
	// PREVENTS: a TLS listener that binds with nothing to present, which fails
	// every handshake while the server looks started.
	_, err := NewLGServer(LGConfig{
		ListenAddrs: []ListenAddr{{Addr: "127.0.0.1:0"}, {Addr: "127.0.0.1:0", TLS: true}},
		Dispatch:    mockDispatch(),
	})
	if err == nil {
		t.Fatal("a TLS listener with no certificate: got no error, want a refusal")
	}

	if _, err := NewLGServer(LGConfig{
		ListenAddrs: []ListenAddr{{Addr: "127.0.0.1:0"}},
		Dispatch:    mockDispatch(),
	}); err != nil {
		t.Fatalf("an all-HTTP server needs no certificate: %v", err)
	}
}

func TestLGServerUpdateListenerTLS(t *testing.T) {
	// VALIDATES: UpdateListenerTLS switches a bound listener between HTTP and
	// HTTPS on the socket it already has: the address does not change, the next
	// connection takes the new protocol, and a connection opened before the
	// switch keeps answering on the old one.
	// PREVENTS: a protocol change that needs a rebind (a gap in service and a
	// dropped viewer), and a switch to TLS on a server that has no certificate
	// to present.
	srv := startTransportTestServer(t, nil, nil, ListenAddr{Addr: "127.0.0.1:0"})
	addr := srv.Address()
	bound := srv.Addresses()

	if got := statusOf(t, plainHTTPClient(), "http://"+addr+statusPath); got != http.StatusOK {
		t.Fatalf("before any switch: HTTP status %d, want 200", got)
	}

	// No certificate yet: the switch is refused and the listener keeps HTTP.
	if err := srv.UpdateListenerTLS(map[string]bool{addr: true}); err == nil {
		t.Fatal("switching to TLS with no certificate: got no error, want a refusal")
	}
	if srv.ListenerTLS()[addr] {
		t.Fatal("a refused switch changed the listener's protocol")
	}
	if got := statusOf(t, plainHTTPClient(), "http://"+addr+statusPath); got != http.StatusOK {
		t.Fatalf("after the refused switch: HTTP status %d, want 200", got)
	}

	// A connection opened on HTTP, kept across the switch.
	var dialer net.Dialer
	open, err := dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	defer func() { _ = open.Close() }()
	openReader := bufio.NewReader(open)
	requireStatusOK(t, open, openReader, "on HTTP, before the switch")

	chainPEM, keyPEM := testChain(t, "switched on")
	if err := srv.UpdateTLSCertificate(chainPEM, keyPEM); err != nil {
		t.Fatalf("installing a certificate on an all-HTTP server: %v", err)
	}
	if err := srv.UpdateListenerTLS(map[string]bool{addr: true}); err != nil {
		t.Fatalf("switching to TLS: %v", err)
	}

	if name := handshakePeerCerts(t, addr)[0].Subject.CommonName; name != "switched on" {
		t.Errorf("after the switch to TLS: leaf %q, want the installed one", name)
	}
	if got := statusOf(t, insecureHTTPSClient(), "https://"+addr+statusPath); got != http.StatusOK {
		t.Errorf("after the switch to TLS: HTTPS status %d, want 200", got)
	}
	requireStatusOK(t, open, openReader, "on the HTTP connection opened before the switch")
	if got := srv.Addresses(); !slices.Equal(got, bound) {
		t.Errorf("the switch rebound the listener: %v, want %v", got, bound)
	}

	// And back: the next connection is plain HTTP again.
	if err := srv.UpdateListenerTLS(map[string]bool{addr: false}); err != nil {
		t.Fatalf("switching back to HTTP: %v", err)
	}
	if got := statusOf(t, plainHTTPClient(), "http://"+addr+statusPath); got != http.StatusOK {
		t.Errorf("after the switch back: HTTP status %d, want 200", got)
	}
	requireNoTLS(t, addr, "after the switch back to HTTP")
	if got := srv.Addresses(); !slices.Equal(got, bound) {
		t.Errorf("the switch back rebound the listener: %v, want %v", got, bound)
	}
}

func TestLGServerReconfigureBindsRecordedProtocol(t *testing.T) {
	// VALIDATES: an address Reconfigure binds serves the protocol recorded for
	// it: the one UpdateListenerTLS named, and the one it served when a
	// Reconfigure removed it (what a rollback puts back). An address with no
	// record is refused, and the call binds nothing and moves nothing.
	// PREVENTS: a listener a reload adds, or a rollback restores, taking a
	// protocol its entry never asked for, even for one connection: the server
	// used to choose TLS for an address with no record, so an HTTP entry a
	// reload added answered TLS until the hub named it.
	chainPEM, keyPEM := testChain(t, "reconfigure")
	srv := startTransportTestServer(t, chainPEM, keyPEM, ListenAddr{Addr: "127.0.0.1:0", TLS: true})
	first := srv.Address()
	ctx := context.Background()

	// Recorded as HTTP before it is bound.
	recorded := freeLoopbackAddr(t)
	if err := srv.UpdateListenerTLS(map[string]bool{recorded: false}); err != nil {
		t.Fatalf("recording the protocol of an unbound address: %v", err)
	}
	if err := srv.Reconfigure(ctx, []string{first, recorded}); err != nil {
		t.Fatalf("adding the recorded address: %v", err)
	}
	if got := statusOf(t, plainHTTPClient(), "http://"+recorded+statusPath); got != http.StatusOK {
		t.Errorf("the added HTTP listener: status %d, want 200", got)
	}
	requireNoTLS(t, recorded, "the added HTTP listener")
	if name := handshakePeerCerts(t, first)[0].Subject.CommonName; name != "reconfigure" {
		t.Errorf("the kept TLS listener: leaf %q, want the configured one", name)
	}

	// Removed, then put back as a rollback does: it serves HTTP again.
	if err := srv.Reconfigure(ctx, []string{first}); err != nil {
		t.Fatalf("removing the HTTP listener: %v", err)
	}
	if err := srv.Reconfigure(ctx, []string{first, recorded}); err != nil {
		t.Fatalf("putting the HTTP listener back: %v", err)
	}
	if got := statusOf(t, plainHTTPClient(), "http://"+recorded+statusPath); got != http.StatusOK {
		t.Errorf("the HTTP listener put back: status %d, want 200", got)
	}
	requireNoTLS(t, recorded, "the HTTP listener put back")

	// Never recorded: refused before anything is bound or closed.
	before := srv.Addresses()
	unrecorded := freeLoopbackAddr(t)
	err := srv.Reconfigure(ctx, []string{first, unrecorded})
	if !errors.Is(err, errLgServerNoRecordedProtocol) {
		t.Fatalf("adding an unrecorded address: got %v, want errLgServerNoRecordedProtocol", err)
	}
	if got := srv.Addresses(); !slices.Equal(got, before) {
		t.Errorf("a refused Reconfigure moved the listeners: %v, want %v", got, before)
	}
	var dialer net.Dialer
	if conn, dialErr := dialer.DialContext(ctx, "tcp", unrecorded); dialErr == nil {
		_ = conn.Close()
		t.Errorf("a refused Reconfigure bound the unrecorded address %s", unrecorded)
	}
	if got := statusOf(t, plainHTTPClient(), "http://"+recorded+statusPath); got != http.StatusOK {
		t.Errorf("the HTTP listener a refused Reconfigure would have removed: status %d, want 200", got)
	}
}

func TestLGUpdateTLSCertificateOnAllHTTPServer(t *testing.T) {
	// VALIDATES: a looking glass whose listeners all serve HTTP accepts a
	// certificate, which is the first half of a reload that switches one of
	// them to HTTPS, and keeps serving HTTP until a listener is switched.
	// PREVENTS: the refusal the server gave when it had one protocol for all
	// listeners, which would fail every reload that adds TLS to an HTTP looking
	// glass.
	srv := startTransportTestServer(t, nil, nil, ListenAddr{Addr: "127.0.0.1:0"})

	chainPEM, keyPEM := testChain(t, "held, not served")
	if err := srv.UpdateTLSCertificate(chainPEM, keyPEM); err != nil {
		t.Fatalf("a certificate on an all-HTTP server: %v", err)
	}
	if err := srv.UpdateTLSCertificate([]byte("not pem"), keyPEM); err == nil {
		t.Error("unparseable material on an all-HTTP server: got no error, want a refusal")
	}
	if got := statusOf(t, plainHTTPClient(), "http://"+srv.Address()+statusPath); got != http.StatusOK {
		t.Errorf("the HTTP listener after the install: status %d, want 200", got)
	}
	requireNoTLS(t, srv.Address(), "the HTTP listener after the install")
}
