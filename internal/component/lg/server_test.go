package lg

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockDispatch returns a dispatcher that returns fixed JSON for known commands.
func mockDispatch() CommandDispatcher {
	return func(cmd string) (string, error) {
		switch {
		case cmd == "bgp status":
			return `{"router-id":"1.2.3.4","version":"test","start-time":"2026-01-01T00:00:00Z"}`, nil
		case cmd == "peer summary":
			return `[{"name":"peer1","peer-address":"10.0.0.1","remote-as":"65001","state":"established","routes-received":"100","routes-accepted":"95","routes-sent":"50"}]`, nil
		case strings.HasPrefix(cmd, "rib show"):
			return `{"routes":[{"prefix":"10.0.0.0/24","next-hop":"10.0.0.1","origin":"igp","as-path":[65001,65002],"local-preference":100,"med":0,"peer-address":"10.0.0.1"}]}`, nil
		case strings.HasPrefix(cmd, "rib best"):
			return `{"routes":[{"prefix":"10.0.0.0/24","next-hop":"10.0.0.1","origin":"igp","as-path":[65001],"local-preference":100}]}`, nil
		}
		return `{"error":"unknown command"}`, nil
	}
}

// doGet sends a GET request with context and returns the response.
func doGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest %s: %v", url, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// doPost sends a POST form request with context and returns the response.
func doPost(t *testing.T, client *http.Client, url, formData string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, strings.NewReader(formData))
	if err != nil {
		t.Fatalf("NewRequest POST %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func startTestServer(t *testing.T) (*LGServer, string, *http.Client) {
	t.Helper()

	srv, err := NewLGServer(LGConfig{
		ListenAddr: "127.0.0.1:0",
		Dispatch:   mockDispatch(),
	})
	if err != nil {
		t.Fatalf("NewLGServer: %v", err)
	}

	go func() {
		_ = srv.ListenAndServe(context.Background())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.WaitReady(ctx); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	return srv, "http://" + srv.Address(), client
}

func TestNewLGServer(t *testing.T) {
	// VALIDATES: AC-1 -- server created with config.
	// PREVENTS: nil dispatcher or empty address accepted.
	srv, err := NewLGServer(LGConfig{
		ListenAddr: "127.0.0.1:0",
		Dispatch:   mockDispatch(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewLGServerRequiresAddr(t *testing.T) {
	// VALIDATES: validation rejects empty address.
	_, err := NewLGServer(LGConfig{
		Dispatch: mockDispatch(),
	})
	if err == nil {
		t.Fatal("expected error for empty listen address")
	}
}

func TestNewLGServerRequiresDispatch(t *testing.T) {
	// VALIDATES: validation rejects nil dispatcher.
	_, err := NewLGServer(LGConfig{
		ListenAddr: "127.0.0.1:0",
	})
	if err == nil {
		t.Fatal("expected error for nil dispatcher")
	}
}

func TestLGServerDisabled(t *testing.T) {
	// VALIDATES: AC-2 -- no config means no server.
	// PREVENTS: server started without config.
	_, err := NewLGServer(LGConfig{})
	if err == nil {
		t.Fatal("expected error when config is empty")
	}
}

func TestLGServerPlainHTTP(t *testing.T) {
	// VALIDATES: AC-4 -- server uses plain HTTP when TLS is false.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/api/looking-glass/status")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLGServerTLSRequiresCert(t *testing.T) {
	// VALIDATES: AC-3 -- TLS enabled without cert fails.
	_, err := NewLGServer(LGConfig{
		ListenAddr: "127.0.0.1:0",
		TLS:        true,
		Dispatch:   mockDispatch(),
	})
	if err == nil {
		t.Fatal("expected error for TLS without cert/key")
	}
}

func TestLGServerRouting(t *testing.T) {
	// VALIDATES: AC-5, AC-6 -- mux routes to correct handlers.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	tests := []struct {
		path       string
		wantStatus int
		wantCT     string
	}{
		{"/api/looking-glass/status", 200, "application/json"},
		{"/api/looking-glass/protocols/bgp", 200, "application/json"},
		{"/api/looking-glass/routes/protocol/peer1", 200, "application/json"},
		{"/api/looking-glass/routes/table/ipv4%2Funicast", 200, "application/json"},
		{"/api/looking-glass/routes/search?prefix=10.0.0.0/24", 200, "application/json"},
		{"/api/looking-glass/nonexistent", 404, "application/json"},
		{"/lg/peers", 200, "text/html"},
		{"/lg/lookup", 200, "text/html"},
		{"/lg/peer/peer1", 200, "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp := doGet(t, client, base+tt.path)
			defer resp.Body.Close() //nolint:errcheck // test cleanup

			if resp.StatusCode != tt.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d (body: %s)", resp.StatusCode, tt.wantStatus, string(body))
			}

			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(ct, tt.wantCT) {
				t.Errorf("Content-Type = %q, want containing %q", ct, tt.wantCT)
			}
		})
	}
}

func TestLGServerGracefulShutdown(t *testing.T) {
	// VALIDATES: AC-7 -- graceful shutdown completes.
	srv, _, _ := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestAPIStatusResponse(t *testing.T) {
	// VALIDATES: AC-1 -- status returns router_id, server_time, version.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/api/looking-glass/status")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	status, ok := result["status"].(map[string]any)
	if !ok {
		t.Fatal("missing status field")
	}

	for _, key := range []string{"router_id", "server_time", "version"} {
		if _, ok := status[key]; !ok {
			t.Errorf("missing key %q in status response", key)
		}
	}
}

func TestAPIProtocolsResponse(t *testing.T) {
	// VALIDATES: AC-2 -- protocols returns peer list with expected fields.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/api/looking-glass/protocols/bgp")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	protocols, ok := result["protocols"].(map[string]any)
	if !ok {
		t.Fatal("missing protocols field")
	}

	if len(protocols) == 0 {
		t.Fatal("expected at least one protocol entry")
	}

	for _, v := range protocols {
		peer, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for _, key := range []string{"state", "neighbor_address", "neighbor_as"} {
			if _, ok := peer[key]; !ok {
				t.Errorf("missing key %q in protocol entry", key)
			}
		}
	}
}

func TestAPIRoutesResponse(t *testing.T) {
	// VALIDATES: AC-3 -- routes returns expected fields.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/api/looking-glass/routes/protocol/peer1")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	routes, ok := result["routes"].([]any)
	if !ok || len(routes) == 0 {
		t.Fatal("expected routes array with entries")
	}

	route, ok := routes[0].(map[string]any)
	if !ok {
		t.Fatal("route is not a map")
	}

	for _, key := range []string{"network", "gateway", "bgp"} {
		if _, ok := route[key]; !ok {
			t.Errorf("missing key %q in route", key)
		}
	}
}

func TestAPIUnknownPath(t *testing.T) {
	// VALIDATES: AC-10 -- unknown API path returns 404 JSON.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/api/looking-glass/nonexistent")
	defer resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestAPIContentType(t *testing.T) {
	// VALIDATES: AC-7 -- all API responses have application/json Content-Type.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	endpoints := []string{
		"/api/looking-glass/status",
		"/api/looking-glass/protocols/bgp",
		"/api/looking-glass/routes/protocol/peer1",
	}

	for _, ep := range endpoints {
		resp := doGet(t, client, base+ep)
		ct := resp.Header.Get("Content-Type")
		resp.Body.Close() //nolint:errcheck // test cleanup
		if !strings.Contains(ct, "application/json") {
			t.Errorf("%s: Content-Type = %q, want application/json", ep, ct)
		}
	}
}

func TestHTMXFragmentVsFullPage(t *testing.T) {
	// VALIDATES: AC-9, AC-10 from lg-3 -- HX-Request returns fragment.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Full page request (no HX-Request header).
	resp := doGet(t, client, base+"/lg/peers")
	fullBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck // test cleanup

	if !strings.Contains(string(fullBody), "<!DOCTYPE html>") {
		t.Error("full page response should contain <!DOCTYPE html>")
	}

	// Fragment request (with HX-Request header).
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/lg/peers", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("HX-Request", "true")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("GET with HX-Request: %v", err)
	}
	fragBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck // test cleanup

	if strings.Contains(string(fragBody), "<!DOCTYPE html>") {
		t.Error("fragment response should NOT contain <!DOCTYPE html>")
	}
}

func TestUILookupPost(t *testing.T) {
	// VALIDATES: POST /lg/lookup returns route results.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doPost(t, client, base+"/lg/lookup", "prefix=10.0.0.0/24")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "10.0.0.0/24") {
		t.Error("response should contain the queried prefix")
	}
}

func TestUIASPathSearchPost(t *testing.T) {
	// VALIDATES: POST /lg/search/aspath returns results.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doPost(t, client, base+"/lg/search/aspath", "pattern=65001")
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestUICommunitySearchPost(t *testing.T) {
	// VALIDATES: POST /lg/search/community returns results.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doPost(t, client, base+"/lg/search/community", "community=65000:100")
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAPIRoutesFilteredEndpoint(t *testing.T) {
	// VALIDATES: /api/looking-glass/routes/filtered/{name} returns JSON.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/api/looking-glass/routes/filtered/peer1")
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestGraphEndpoint(t *testing.T) {
	// VALIDATES: /lg/graph returns SVG.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/lg/graph?prefix=10.0.0.0/24")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "image/svg+xml") {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
	if !strings.Contains(string(body), "<svg") {
		t.Error("response should contain SVG element")
	}
}

func TestGraphEndpointMissingPrefix(t *testing.T) {
	// VALIDATES: /lg/graph without prefix returns 400.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/lg/graph")
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAssetsCSS(t *testing.T) {
	// VALIDATES: /lg/assets/style.css returns CSS.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/lg/assets/style.css")
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}

func TestAssetsUnknown(t *testing.T) {
	// VALIDATES: /lg/assets/unknown returns 404.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/lg/assets/evil.js")
	resp.Body.Close() //nolint:errcheck // test cleanup

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSecurityHeaders(t *testing.T) {
	// VALIDATES: security headers present on all responses.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	resp := doGet(t, client, base+"/api/looking-glass/status")
	resp.Body.Close() //nolint:errcheck // test cleanup

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for name, want := range headers {
		got := resp.Header.Get(name)
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Error("missing Content-Security-Policy header")
	}
}

func TestRouteDetailValidation(t *testing.T) {
	// VALIDATES: /lg/route/detail validates prefix parameter.
	srv, base, client := startTestServer(t)
	defer func() { _ = srv.Shutdown(context.Background()) }()

	// Missing prefix.
	resp := doGet(t, client, base+"/lg/route/detail")
	resp.Body.Close() //nolint:errcheck // test cleanup
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing prefix: status = %d, want 400", resp.StatusCode)
	}

	// Invalid prefix (injection attempt).
	resp = doGet(t, client, base+"/lg/route/detail?prefix=10.0.0.0/24%20rm%20-rf")
	resp.Body.Close() //nolint:errcheck // test cleanup
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("injection attempt: status = %d, want 400", resp.StatusCode)
	}
}
