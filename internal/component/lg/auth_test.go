package lg

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// startTokenTestServer starts a looking glass gated by the given bearer token
// ("" leaves it open) and returns its base URL and a client.
func startTokenTestServer(t *testing.T, token string) (string, *http.Client) {
	t.Helper()

	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []string{"127.0.0.1:0"},
		Dispatch:    mockDispatch(),
		Token:       token,
	})
	if err != nil {
		t.Fatalf("NewLGServer: %v", err)
	}
	t.Cleanup(func() { _ = srv.Shutdown(context.Background()) })

	go func() { _ = srv.ListenAndServe(context.Background()) }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	return "http://" + srv.Address(), &http.Client{Timeout: 10 * time.Second}
}

// getWithAuth issues a GET carrying the given Authorization header value; an
// empty header value sends no Authorization header at all.
func getWithAuth(t *testing.T, client *http.Client, url, authHeader string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest %s: %v", url, err)
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // test
	return resp.StatusCode
}

func TestLGTokenMiddleware(t *testing.T) {
	// VALIDATES: AC-5 -- when a looking-glass token is configured, EVERY /api/
	// and /lg/ route requires a matching bearer token. Driven through the
	// server's real HTTP entry point rather than the middleware function, per
	// ai/rules/evidence.md: a middleware nothing wraps the mux with
	// authenticates nothing.
	// PREVENTS: a token that gates the API but leaves the UI open, or a route
	// registered after the middleware and therefore never gated.
	const token = "lg-s3cret-token"
	base, client := startTokenTestServer(t, token)

	// Routes that must be gated. The two route families are listed because the
	// LG registers them on one mux and a wrapper applied to only one family is
	// the failure this test exists to catch.
	paths := []string{
		"/api/looking-glass/status",
		"/api/looking-glass/protocols/bgp",
		"/api/looking-glass/routes/filtered/peer1",
		"/api/looking-glass/routes/noexport/peer1",
		"/lg/peers",
		"/lg/search",
		"/lg/help",
	}

	for _, p := range paths {
		t.Run("no token rejected "+p, func(t *testing.T) {
			if got := getWithAuth(t, client, base+p, ""); got != http.StatusUnauthorized {
				t.Fatalf("%s without a token: got %d, want 401", p, got)
			}
		})
		t.Run("wrong token rejected "+p, func(t *testing.T) {
			if got := getWithAuth(t, client, base+p, "Bearer wrong-token"); got != http.StatusUnauthorized {
				t.Fatalf("%s with a wrong token: got %d, want 401", p, got)
			}
		})
		t.Run("correct token accepted "+p, func(t *testing.T) {
			if got := getWithAuth(t, client, base+p, "Bearer "+token); got == http.StatusUnauthorized {
				t.Fatalf("%s with the correct token was rejected", p)
			}
		})
	}

	t.Run("malformed authorization rejected", func(t *testing.T) {
		// Not "Bearer <token>": a bare token, a wrong scheme, and the prefix
		// alone must all fail rather than fall through to a prefix-strip that
		// yields an empty token.
		for _, hdr := range []string{token, "Basic " + token, "Bearer", "Bearer ", "Bearer " + token + "x"} {
			got := getWithAuth(t, client, base+"/api/looking-glass/status", hdr)
			if got != http.StatusUnauthorized {
				t.Fatalf("Authorization %q: got %d, want 401", hdr, got)
			}
		}
	})

	t.Run("scheme is case-insensitive", func(t *testing.T) {
		// RFC 7235 Section 2.1: the auth-scheme is case-insensitive. Rejecting
		// `bearer` would fail a conforming client for no security gain. The
		// token after the scheme stays case-sensitive.
		for _, hdr := range []string{"bearer " + token, "BEARER " + token, "BeArEr " + token} {
			if got := getWithAuth(t, client, base+"/api/looking-glass/status", hdr); got == http.StatusUnauthorized {
				t.Fatalf("Authorization %q must be accepted (RFC 7235 2.1 case-insensitive scheme)", hdr)
			}
		}
		if got := getWithAuth(t, client, base+"/api/looking-glass/status", "Bearer "+strings.ToUpper(token)); got != http.StatusUnauthorized {
			t.Fatalf("the token itself must stay case-sensitive, got %d", got)
		}
	})
}

func TestLGWithoutTokenStaysOpen(t *testing.T) {
	// VALIDATES: AC-5 -- the token is OPTIONAL. A looking glass is an
	// intentionally public read-only surface (Key Design Decisions row: LG is
	// exempt from the unauthenticated refusal), so an unset token must leave
	// every route reachable exactly as before.
	// PREVENTS: the auth gate defaulting on and breaking public looking glasses.
	base, client := startTokenTestServer(t, "")
	for _, p := range []string{"/api/looking-glass/status", "/lg/peers"} {
		if got := getWithAuth(t, client, base+p, ""); got == http.StatusUnauthorized {
			t.Fatalf("%s must stay open when no token is configured", p)
		}
	}
}
