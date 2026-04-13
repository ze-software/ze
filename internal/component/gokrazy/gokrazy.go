// Design: (none — new component for gokrazy management proxy)
//
// Package gokrazy provides a reverse proxy handler for the gokrazy
// management interface, mounted on ze's web server at /gokrazy/.
// Gokrazy listens on a Unix socket (/run/gokrazy-http.sock)
// unconditionally. This handler proxies to that socket, injects
// gokrazy's Basic Auth credentials, and rewrites absolute paths in
// HTML responses so links stay under the /gokrazy/ prefix.
//
// Reference: https://github.com/gokrazy/gokrazy
// Auth handler: gokrazy/authenticated.go
// Password source: gokrazy/gokrazy.go readConfigFile("gokr-pw.txt")
package gokrazy

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// Handler returns an http.Handler that reverse-proxies to the gokrazy
// management Unix socket. It strips the /gokrazy prefix, injects
// Basic Auth, and rewrites absolute paths in HTML responses. The
// caller wraps this with ze's auth middleware.
func Handler(socketPath string) http.Handler {
	if socketPath == "" {
		socketPath = "/run/gokrazy-http.sock"
	}

	logger := slogutil.Logger("gokrazy")

	password := readGokrazyPassword()
	var authHeader string
	if password != "" {
		creds := base64.StdEncoding.EncodeToString([]byte("gokrazy:" + password))
		authHeader = "Basic " + creds
	} else {
		logger.Info("gokrazy password not found, proxying without auth injection")
	}

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(&url.URL{Scheme: "http", Host: "gokrazy"})
			pr.Out.URL.Path = pr.In.URL.Path
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
			if authHeader != "" {
				pr.Out.Header.Set("Authorization", authHeader)
			}
		},
	}

	proxy.ModifyResponse = rewriteResponse
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if os.IsNotExist(err) {
			http.Error(w, "gokrazy management socket not found", http.StatusServiceUnavailable)
			return
		}
		logger.Warn("gokrazy proxy error", "error", err, "path", r.URL.Path)
		http.Error(w, "gokrazy management unavailable", http.StatusBadGateway)
	}

	stripped := http.StripPrefix("/gokrazy", proxy)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clear ze's restrictive CSP (set by addSecurityHeaders in auth
		// middleware) before the proxy writes. The proxy's ModifyResponse
		// sets a permissive CSP that allows gokrazy's inline scripts.
		w.Header().Del("Content-Security-Policy")
		stripped.ServeHTTP(w, r)
	})
}

// rewriteResponse rewrites absolute URL paths in HTML and JS responses
// so that links like href="/status" become href="/gokrazy/status" and
// JS strings like "/log?path=" become "/gokrazy/log?path=".
func rewriteResponse(resp *http.Response) error {
	// Set permissive CSP for gokrazy pages: gokrazy uses inline scripts
	// (e.g., setting ServiceName) that ze's default script-src 'self' blocks.
	// The handler wrapper clears ze's restrictive CSP from the response writer;
	// this sets the replacement on the upstream response headers.
	resp.Header.Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")

	ct := resp.Header.Get("Content-Type")
	isHTML := strings.HasPrefix(ct, "text/html")
	isJS := strings.HasPrefix(ct, "application/javascript") ||
		strings.HasPrefix(ct, "text/javascript")
	if !isHTML && !isJS {
		return nil
	}

	const maxBody = 10 << 20 // 10 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	_ = resp.Body.Close()
	if err != nil {
		return err
	}

	// HTML attribute rewrites (href, src, action).
	if isHTML {
		for _, attr := range []string{"href", "src", "action"} {
			body = rewriteAttr(body, attr)
		}
	}

	// JS string rewrites (inline <script> in HTML and external .js files):
	// EventSource("/log?...") and similar gokrazy API calls.
	// Only target known gokrazy API paths to avoid mangling arbitrary strings.
	if isHTML || isJS {
		for _, path := range []string{"/log?", "/status?", "/stop?", "/restart?", "/reboot", "/uploadtemp/"} {
			old := []byte(`"` + path)
			rewritten := []byte(`"/gokrazy` + path)
			body = bytes.ReplaceAll(body, old, rewritten)
		}
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

// rewriteAttr rewrites attr="/..." to attr="/gokrazy/..." while
// protecting already-prefixed paths from double-rewrite.
func rewriteAttr(body []byte, attr string) []byte {
	already := []byte(attr + `="/gokrazy/`)
	bare := []byte(attr + `="/`)
	rewritten := []byte(attr + `="/gokrazy/`)
	marker := []byte("\x00GK_" + attr + "\x00")
	body = bytes.ReplaceAll(body, already, marker)
	body = bytes.ReplaceAll(body, bare, rewritten)
	body = bytes.ReplaceAll(body, marker, already)
	return body
}

// readGokrazyPassword reads the HTTP password from the same locations
// gokrazy uses: /perm/gokr-pw.txt, /etc/gokr-pw.txt, /gokr-pw.txt.
// Returns empty string if no password file is found.
func readGokrazyPassword() string {
	for _, path := range []string{"/perm/gokr-pw.txt", "/etc/gokr-pw.txt", "/gokr-pw.txt"} {
		data, err := os.ReadFile(path) //nolint:gosec // paths are hardcoded constants
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}
