// VALIDATES: gokrazy reverse proxy injects Basic Auth, strips the /gokrazy
// prefix, rewrites absolute HTML/JS paths under /gokrazy, and returns 503 when
// the management socket is absent.
// PREVENTS: silent auth-injection regressions, broken relative links in the
// proxied gokrazy UI, and 502-masking of a missing management socket.
package gokrazy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// socketDir returns a directory short enough to hold a Unix socket path.
//
// t.TempDir() embeds the test name and a per-test counter, so on darwin it
// yields paths like
// /var/folders/<16>/<30>/T/TestRewriteResponseSkipsNonHTML1258320514/001/
// which leaves no room: sun_path is 104 bytes including the NUL, and the
// longest test here overflows it, so bind() fails with EINVAL ("invalid
// argument") before any assertion runs. Linux allows 108 and never noticed.
// A short prefix keeps the whole path well inside the limit on both.
func socketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gk")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// fakeGokrazy starts a fake gokrazy management server on a Unix socket in a
// temp dir and returns its socket path plus a pointer to the last request's
// captured Authorization header. The handler writes the supplied body/content
// type so response-rewrite behavior can be asserted end to end.
func fakeGokrazy(t *testing.T, contentType, body string) (socketPath string, gotAuth *string) {
	t.Helper()
	socketPath = filepath.Join(socketDir(t), "gokrazy.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	var mu sync.Mutex
	captured := ""
	gotAuth = &captured
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			captured = r.Header.Get("Authorization")
			mu.Unlock()
			if contentType != "" {
				w.Header().Set("Content-Type", contentType)
			}
			if _, err := io.WriteString(w, body); err != nil {
				return
			}
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return socketPath, gotAuth
}

// capturedResp is a fully-buffered response snapshot; doGet closes the body
// before returning so callers never leak it.
type capturedResp struct {
	code   int
	header http.Header
	body   string
}

func doGet(t *testing.T, h http.Handler, path string) capturedResp {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return capturedResp{code: res.StatusCode, header: res.Header, body: string(body)}
}

// TestGokrazyProxyAuthInjection proves the proxy forwards the injected Basic
// Auth header to the upstream gokrazy socket (AC-7: auth injection).
func TestGokrazyProxyAuthInjection(t *testing.T) {
	sock, gotAuth := fakeGokrazy(t, "text/plain", "ok")
	const auth = "Basic Z29rcmF6eTpzM2NyZXQ="
	h := handlerWithAuth(sock, auth)

	resp := doGet(t, h, "/gokrazy/status")
	if resp.code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.code)
	}
	if *gotAuth != auth {
		t.Fatalf("upstream Authorization = %q, want %q", *gotAuth, auth)
	}
}

// TestGokrazyProxyNoAuthWhenEmpty proves no Authorization header is forged
// when no gokrazy password is configured.
func TestGokrazyProxyNoAuthWhenEmpty(t *testing.T) {
	sock, gotAuth := fakeGokrazy(t, "text/plain", "ok")
	h := handlerWithAuth(sock, "")

	_ = doGet(t, h, "/gokrazy/status")
	if *gotAuth != "" {
		t.Fatalf("upstream Authorization = %q, want empty", *gotAuth)
	}
}

// TestProxyStripsPrefix proves the /gokrazy mount prefix is stripped before
// the request reaches the upstream socket.
func TestProxyStripsPrefix(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "gokrazy.sock")
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	var mu sync.Mutex
	gotPath := ""
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotPath = r.URL.Path
		mu.Unlock()
		if _, err := io.WriteString(w, "ok"); err != nil {
			return
		}
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	_ = doGet(t, handlerWithAuth(socketPath, ""), "/gokrazy/status")
	mu.Lock()
	defer mu.Unlock()
	if gotPath != "/status" {
		t.Fatalf("upstream path = %q, want /status", gotPath)
	}
}

// TestProxySocketMissing503 proves a missing gokrazy socket yields 503, not a
// generic 502 (AC-7: 503-on-missing-socket).
func TestProxySocketMissing503(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.sock")
	h := handlerWithAuth(missing, "")

	resp := doGet(t, h, "/gokrazy/status")
	if resp.code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 for missing socket", resp.code)
	}
}

// TestRewriteResponseHTMLPaths proves absolute HTML/JS paths are rewritten
// under the /gokrazy prefix and the permissive CSP is set (AC-7: path rewrite).
func TestRewriteResponseHTMLPaths(t *testing.T) {
	html := `<a href="/status">s</a><script src="/gokrazy.js"></script>` +
		`<form action="/stop"></form><script>new EventSource("/log?path=x")</script>`
	sock, _ := fakeGokrazy(t, "text/html", html)

	resp := doGet(t, handlerWithAuth(sock, ""), "/gokrazy/")
	got := resp.body
	if got == "" {
		t.Fatalf("proxy returned empty body")
	}

	for _, want := range []string{
		`href="/gokrazy/status"`,
		`src="/gokrazy/gokrazy.js"`,
		`action="/gokrazy/stop"`,
		`"/gokrazy/log?path=x"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rewritten body missing %q\nbody: %s", want, got)
		}
	}
	if csp := resp.header.Get("Content-Security-Policy"); !strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP = %q, want permissive (unsafe-inline)", csp)
	}
}

// TestRewriteResponseSkipsNonHTML proves non-HTML/JS bodies pass through
// untouched (only the CSP header is added).
func TestRewriteResponseSkipsNonHTML(t *testing.T) {
	const payload = `{"path":"/status"}`
	sock, _ := fakeGokrazy(t, "application/json", payload)

	resp := doGet(t, handlerWithAuth(sock, ""), "/gokrazy/api")
	if resp.code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.code)
	}
	if resp.body != payload {
		t.Fatalf("JSON body mutated: got %q, want %q", resp.body, payload)
	}
}

// TestRewriteAttr covers the pure attribute rewriter: bare absolute paths get
// the prefix, already-prefixed paths are protected from a double rewrite.
func TestRewriteAttr(t *testing.T) {
	cases := []struct {
		name, attr, in, want string
	}{
		{"bare href", "href", `href="/status"`, `href="/gokrazy/status"`},
		{"already prefixed", "href", `href="/gokrazy/status"`, `href="/gokrazy/status"`},
		{"src", "src", `src="/app.js"`, `src="/gokrazy/app.js"`},
		{"other attr untouched", "href", `data-x="/status"`, `data-x="/status"`},
		{"mixed", "href", `href="/a" href="/gokrazy/b"`, `href="/gokrazy/a" href="/gokrazy/b"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(rewriteAttr([]byte(tc.in), tc.attr))
			if got != tc.want {
				t.Fatalf("rewriteAttr(%q, %q) = %q, want %q", tc.in, tc.attr, got, tc.want)
			}
		})
	}
}

// TestFakeGokrazyResponds sanity-checks the Unix-socket fake used above.
func TestFakeGokrazyResponds(t *testing.T) {
	sock, _ := fakeGokrazy(t, "text/plain", "pong")
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "GET /ping HTTP/1.1\r\nHost: gokrazy\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatalf("write request: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(body, []byte("pong")) {
		t.Fatalf("body = %q, want pong", body)
	}
}
