package mcp

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestASMetadataURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://as.example", "https://as.example/.well-known/oauth-authorization-server"},
		{"https://as.example/", "https://as.example/.well-known/oauth-authorization-server"},
		{"https://as.example//", "https://as.example/.well-known/oauth-authorization-server"},
		{"https://as.example/realm/x", "https://as.example/realm/x/.well-known/oauth-authorization-server"},
	}
	for _, tc := range cases {
		if got := asMetadataURL(tc.in); got != tc.want {
			t.Fatalf("asMetadataURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFetchASMetadata_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		body, err := json.Marshal(map[string]any{
			"issuer":   "https://as.example/",
			"jwks_uri": "https://as.example/jwks",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write: %v", werr)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	md, err := fetchASMetadata(ctx, nil, srv.URL)
	if err != nil {
		t.Fatalf("fetchASMetadata: %v", err)
	}
	if md.Issuer != "https://as.example/" {
		t.Fatalf("issuer = %q", md.Issuer)
	}
	if md.JWKSURI != "https://as.example/jwks" {
		t.Fatalf("jwks_uri = %q", md.JWKSURI)
	}
}

func TestFetchASMetadata_MissingIssuer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, _ := json.Marshal(map[string]any{"jwks_uri": "https://as.example/jwks"})
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write: %v", werr)
		}
	}))
	defer srv.Close()

	_, err := fetchASMetadata(t.Context(), nil, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "missing issuer") {
		t.Fatalf("expected missing issuer, got %v", err)
	}
}

func TestFetchASMetadata_MissingJWKSURI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, _ := json.Marshal(map[string]any{"issuer": "https://as.example/"})
		if _, werr := w.Write(body); werr != nil {
			t.Logf("write: %v", werr)
		}
	}))
	defer srv.Close()

	_, err := fetchASMetadata(t.Context(), nil, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "missing jwks_uri") {
		t.Fatalf("expected missing jwks_uri, got %v", err)
	}
}

func TestFetchASMetadata_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchASMetadata(t.Context(), nil, srv.URL)
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status 500 error, got %v", err)
	}
}

func TestFetchASMetadata_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, werr := w.Write([]byte("not json")); werr != nil {
			t.Logf("write: %v", werr)
		}
	}))
	defer srv.Close()

	_, err := fetchASMetadata(t.Context(), nil, srv.URL)
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestFetchASMetadata_OversizeBody(t *testing.T) {
	big := make([]byte, maxASMetadataSize+100)
	for i := range big {
		big[i] = 'a'
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, werr := w.Write(big); werr != nil {
			t.Logf("write: %v", werr)
		}
	}))
	defer srv.Close()

	_, err := fetchASMetadata(t.Context(), nil, srv.URL)
	if err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestFetchASMetadata_HonorsCustomClientTimeout(t *testing.T) {
	// Open a TCP listener that accepts but never writes; the HTTP client
	// must time out on its own. Using httptest.NewServer here would dead-
	// lock in t.Cleanup because Server.Close waits for in-flight handlers.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		if cerr := ln.Close(); cerr != nil {
			t.Logf("close listener: %v", cerr)
		}
	})
	go func() {
		conn, aerr := ln.Accept()
		if aerr != nil {
			return
		}
		<-t.Context().Done()
		if cerr := conn.Close(); cerr != nil {
			t.Logf("close conn: %v", cerr)
		}
	}()

	client := &http.Client{Timeout: 250 * time.Millisecond}
	baseURL := "http://" + ln.Addr().String()
	_, err = fetchASMetadata(t.Context(), client, baseURL)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestStringField(t *testing.T) {
	m := map[string]any{
		"str": "hello",
		"num": 42,
		"nil": nil,
	}
	if got := stringField(m, "str"); got != "hello" {
		t.Fatalf("str = %q", got)
	}
	if got := stringField(m, "num"); got != "" {
		t.Fatalf("num coerced to %q, want empty", got)
	}
	if got := stringField(m, "nil"); got != "" {
		t.Fatalf("nil coerced to %q, want empty", got)
	}
	if got := stringField(m, "absent"); got != "" {
		t.Fatalf("absent = %q, want empty", got)
	}
}
