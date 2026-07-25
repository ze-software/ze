// Design: docs/architecture/mcp/overview.md -- MCP resources capability tests

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

func TestResources_ListWalksEmbeddedFS(t *testing.T) {
	resources := listResources()
	if len(resources) == 0 {
		t.Fatal("listResources returned empty; embedded FS has no files")
	}
	found := map[string]bool{}
	for _, r := range resources {
		uri, _ := r["uri"].(string)
		found[uri] = true
		if !strings.HasPrefix(uri, uiScheme) {
			t.Errorf("resource URI %q does not start with %q", uri, uiScheme)
		}
		if r["mimeType"] == nil || r["mimeType"] == "" {
			t.Errorf("resource %q has empty mimeType", uri)
		}
		if r["name"] == nil || r["name"] == "" {
			t.Errorf("resource %q has empty name", uri)
		}
	}
	for _, want := range []string{
		"ui://bgp-peer/index.html",
		"ui://bgp-peer/style.css",
		"ui://bgp-peer/app.js",
	} {
		if !found[want] {
			t.Errorf("expected resource %q not in list", want)
		}
	}
}

func TestResources_ReadHTML(t *testing.T) {
	content, err := readResource("ui://bgp-peer/index.html")
	if err != nil {
		t.Fatalf("readResource: %v", err)
	}
	if content["mimeType"] != "text/html" {
		t.Errorf("mimeType = %v, want text/html", content["mimeType"])
	}
	text, _ := content["text"].(string)
	if text == "" {
		t.Fatal("text field is empty for HTML resource")
	}
	if !strings.Contains(text, "<!DOCTYPE html>") {
		t.Error("HTML content does not contain DOCTYPE")
	}
	if content["blob"] != nil {
		t.Error("text resource should not have blob field")
	}
}

func TestResources_ReadCSS(t *testing.T) {
	content, err := readResource("ui://bgp-peer/style.css")
	if err != nil {
		t.Fatalf("readResource: %v", err)
	}
	if content["mimeType"] != "text/css" {
		t.Errorf("mimeType = %v, want text/css", content["mimeType"])
	}
	text, _ := content["text"].(string)
	if text == "" {
		t.Fatal("text field is empty for CSS resource")
	}
}

func TestResources_ReadNotFound(t *testing.T) {
	_, err := readResource("ui://nonexistent/file.html")
	if !errors.Is(err, errResourceNotFound) {
		t.Errorf("err = %v, want errResourceNotFound", err)
	}
}

func TestResources_ReadInvalidURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"missing scheme", "bgp-peer/index.html"},
		{"wrong scheme", "file://bgp-peer/index.html"},
		{"https scheme", "https://example.com/index.html"},
		{"bare scheme", "ui://"},
		{"uppercase scheme", "UI://bgp-peer/index.html"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readResource(tt.uri)
			if !errors.Is(err, errInvalidURI) {
				t.Errorf("readResource(%q) err = %v, want errInvalidURI", tt.uri, err)
			}
		})
	}
}

func TestResources_ReadTraversalRejected(t *testing.T) {
	traversals := []string{
		"ui://../../etc/passwd",
		"ui://../secrets",
		"ui://bgp-peer/../../../etc/shadow",
	}
	for _, uri := range traversals {
		t.Run(uri, func(t *testing.T) {
			_, err := readResource(uri)
			if !errors.Is(err, errInvalidURI) {
				t.Errorf("readResource(%q) err = %v, want errInvalidURI", uri, err)
			}
		})
	}
}

func TestResources_ReadBinaryUsesBlob(t *testing.T) {
	content, err := readResource("ui://bgp-peer/icon.png")
	if err != nil {
		t.Fatalf("readResource: %v", err)
	}
	if content["mimeType"] != "image/png" {
		t.Errorf("mimeType = %v, want image/png", content["mimeType"])
	}
	blob, _ := content["blob"].(string)
	if blob == "" {
		t.Fatal("blob field is empty for binary resource")
	}
	if content["text"] != nil {
		t.Error("binary resource should not have text field")
	}
}

func TestResources_ReadNoCapability(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	})
	defer cleanup()

	sid := initializeWithCapabilities(t, hs, map[string]any{"tools": map[string]any{}})

	body := `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"ui://bgp-peer/index.html"}}`
	resp := postWithSession(t, hs, sid, body)
	defer closeBody(t, resp.Body)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, _ := result["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("expected error response for resources/read without capability")
	}
	code, _ := errObj["code"].(float64)
	if code != -32601 {
		t.Errorf("error code = %v, want -32601", code)
	}
}

func TestResources_ListNoCapability(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	})
	defer cleanup()

	sid := initializeWithCapabilities(t, hs, map[string]any{"tools": map[string]any{}})

	body := `{"jsonrpc":"2.0","id":2,"method":"resources/list"}`
	resp := postWithSession(t, hs, sid, body)
	defer closeBody(t, resp.Body)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, _ := result["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("expected error response for resources/list without capability")
	}
	code, _ := errObj["code"].(float64)
	if code != -32601 {
		t.Errorf("error code = %v, want -32601", code)
	}
}

func TestResources_ListWithCapability(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	})
	defer cleanup()

	sid := initializeWithCapabilities(t, hs, map[string]any{"resources": map[string]any{}})

	body := `{"jsonrpc":"2.0","id":2,"method":"resources/list"}`
	resp := postWithSession(t, hs, sid, body)
	defer closeBody(t, resp.Body)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["error"] != nil {
		t.Fatalf("unexpected error: %v", result["error"])
	}
	resResult, _ := result["result"].(map[string]any)
	resources, _ := resResult["resources"].([]any)
	if len(resources) == 0 {
		t.Fatal("resources/list returned empty with capability declared")
	}
}

func TestResources_ReadWithCapability(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	})
	defer cleanup()

	sid := initializeWithCapabilities(t, hs, map[string]any{"resources": map[string]any{}})

	body := `{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"ui://bgp-peer/index.html"}}`
	resp := postWithSession(t, hs, sid, body)
	defer closeBody(t, resp.Body)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["error"] != nil {
		t.Fatalf("unexpected error: %v", result["error"])
	}
	resResult, _ := result["result"].(map[string]any)
	contents, _ := resResult["contents"].([]any)
	if len(contents) == 0 {
		t.Fatal("resources/read returned empty contents")
	}
	first, _ := contents[0].(map[string]any)
	if first["mimeType"] != "text/html" {
		t.Errorf("mimeType = %v, want text/html", first["mimeType"])
	}
	text, _ := first["text"].(string)
	if !strings.Contains(text, "<!DOCTYPE html>") {
		t.Error("returned text does not contain DOCTYPE")
	}
}

func TestInitialize_ServerAdvertisesResources(t *testing.T) {
	_, hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	})
	defer cleanup()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	res, _ := result["result"].(map[string]any)
	caps, _ := res["capabilities"].(map[string]any)
	if caps["resources"] == nil {
		t.Error("server capabilities missing 'resources'")
	}
}

func TestInitialize_ClientCapabilityResources(t *testing.T) {
	srv, hs, cleanup := newTestStreamable(t, StreamableConfig{
		Dispatch: func(_ context.Context, _ plugin.CallerIdentity, _ string) (*plugin.Response, error) {
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok")), nil
		},
	})
	defer cleanup()

	sid := initializeWithCapabilities(t, hs, map[string]any{"resources": map[string]any{}})
	sess, ok := srv.registry.Get(sid)
	if !ok {
		t.Fatalf("session %q not found", sid)
	}
	if !sess.ClientSupportsResources() {
		t.Error("ClientSupportsResources() = false, want true")
	}
}

func TestMimeType_Sniffing(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"index.html", "text/html"},
		{"style.css", "text/css"},
		{"app.js", "application/javascript"},
		{"data.json", "application/json"},
		{"icon.svg", "image/svg+xml"},
		{"photo.png", "image/png"},
		{"photo.jpg", "image/jpeg"},
		{"unknown.xyz", "application/octet-stream"},
		{"noext", "application/octet-stream"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sniffMIME(tt.name)
			if got != tt.want {
				t.Errorf("sniffMIME(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func initializeWithCapabilities(t *testing.T, hs *httptest.Server, caps map[string]any) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    caps,
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer closeBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	return resp.Header.Get("Mcp-Session-Id")
}

func postWithSession(t *testing.T, hs *httptest.Server, sid, body string) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hs.URL+Endpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mcp-Session-Id", sid)
	req.Header.Set("MCP-Protocol-Version", ProtocolVersion)
	resp, err := hs.Client().Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}
