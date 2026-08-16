// four capability-gate tests are replaced, not dropped, because the
// gate they asserted was itself the defect. `resources` is a member of
// *ServerCapabilities*. The five ClientCapabilities members in MCP 2026-07-28
// are `experimental`, `roots`, `sampling`, `elicitation` and `extensions`. No
// conformant client can declare `resources`, so the gate refused every
// conformant caller. At the same time server/discover advertised the
// capability, and tools/list published `_meta.ui.resourceUri` that points at
// these assets.
//
// Removed: TestResources_NilSessionDeniesRatherThanPanics and
// TestInitialize_ClientCapabilityResources (their subjects, the *session
// pointer and the initialize handshake, no longer exist), plus
// TestResourcesRejectWithoutCapability and TestResourcesDenyOnZeroCapability-
// Value (they asserted the removed gate), plus TestResources_ListWithCapability
// and TestResources_ReadWithCapability (a capability the client cannot declare
// cannot be the precondition they named). Their assertions are folded into
// TestResourcesServedWithoutClientCapability and
// TestResourcesServedForEveryCapabilityShape below. Those two assert strictly
// more: the same list/read content across three declared-capability shapes
// instead of one, plus the resultType envelope. Per-request capability PARSING
// is asserted in meta_test.go TestParseRequestMeta. The surviving -32021 gate
// (the Tasks extension, a real client capability) is asserted in
// streamable_test.go TestStreamable_ToolsCallTaskWithoutCapability.

// Design: docs/architecture/mcp/overview.md -- MCP resources capability tests

package mcp

import (
	"errors"
	"net/http"
	"strings"
	"testing"
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

// TestResourcesServedWithoutClientCapability covers AC-13 as corrected: the
// client-capability gate the AC described is the defect, not the requirement
// (see the test-relax note at the top of this file).
//
// VALIDATES: a client that sends the conformant `clientCapabilities: {}` is
// served resources/list and resources/read. The answer is HTTP 200 and a
// "complete" result that carries real content. That empty object is the only
// shape a conformant client CAN send, because `resources` is a
// ServerCapabilities member.
// PREVENTS: a return of the client-capability gate. The gate closed a loop that
// was broken end to end. server/discover advertised `capabilities.resources`,
// and tools/list published `_meta.ui.resourceUri` that points at these very
// assets. resources/read then refused every conformant caller with -32021, and
// that error's data.requiredCapabilities was not even a legal
// ClientCapabilities value.
func TestResourcesServedWithoutClientCapability(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	t.Run(methodResourcesList, func(t *testing.T) {
		status, parsed := postMCP(t, hs, methodResourcesList, capsNone, "")
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
		}
		res := resultOf(t, parsed)
		if res["resultType"] != resultTypeComplete {
			t.Fatalf("resultType = %v, want %q", res["resultType"], resultTypeComplete)
		}
		resources, _ := res["resources"].([]any)
		if len(resources) == 0 {
			t.Fatal("resources/list returned empty for a {}-capability client")
		}
	})

	t.Run(methodResourcesRead, func(t *testing.T) {
		status, parsed := postMCP(t, hs, methodResourcesRead, capsNone,
			`{"uri":"ui://bgp-peer/index.html"}`)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
		}
		res := resultOf(t, parsed)
		if res["resultType"] != resultTypeComplete {
			t.Fatalf("resultType = %v, want %q", res["resultType"], resultTypeComplete)
		}
		contents, _ := res["contents"].([]any)
		if len(contents) == 0 {
			t.Fatal("resources/read returned empty contents")
		}
		first, _ := contents[0].(map[string]any)
		if first["mimeType"] != "text/html" {
			t.Fatalf("mimeType = %v, want text/html", first["mimeType"])
		}
		text, _ := first["text"].(string)
		if !strings.Contains(text, "<!DOCTYPE html>") {
			t.Fatal("returned text does not contain DOCTYPE")
		}
	})
}

// VALIDATES: what resources/list and resources/read answer does not depend on
// what the request declared in clientCapabilities -- an empty object, the tasks
// extension, and even a stray server-capability name all get the same result.
// PREVENTS: a capability gate reappearing on either handler under any name. A
// gate keyed on ANY declared value would make one of these rows diverge.
func TestResourcesServedForEveryCapabilityShape(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	shapes := []struct {
		name string
		caps string
	}{
		{"empty object", capsNone},
		{"tasks extension only", capsTasks},
		{"a server capability name the client cannot legally declare", `{"resources":{}}`},
	}

	var wantList int
	var wantText string
	for i, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			status, parsed := postMCP(t, hs, methodResourcesList, shape.caps, "")
			if status != http.StatusOK {
				t.Fatalf("resources/list status = %d, want 200 (body %v)", status, parsed)
			}
			resources, _ := resultOf(t, parsed)["resources"].([]any)
			if len(resources) == 0 {
				t.Fatal("resources/list returned empty")
			}

			status, parsed = postMCP(t, hs, methodResourcesRead, shape.caps,
				`{"uri":"ui://bgp-peer/index.html"}`)
			if status != http.StatusOK {
				t.Fatalf("resources/read status = %d, want 200 (body %v)", status, parsed)
			}
			contents, _ := resultOf(t, parsed)["contents"].([]any)
			if len(contents) == 0 {
				t.Fatal("resources/read returned empty contents")
			}
			first, _ := contents[0].(map[string]any)
			text, _ := first["text"].(string)

			if i == 0 {
				wantList, wantText = len(resources), text
				return
			}
			if len(resources) != wantList {
				t.Fatalf("resources/list returned %d entries, want %d -- the answer varies with declared capabilities", len(resources), wantList)
			}
			if text != wantText {
				t.Fatal("resources/read returned different content -- the answer varies with declared capabilities")
			}
		})
	}
}

// VALIDATES: server/discover advertises the resources capability, which is what
// tells a client it can declare and use it.
// PREVENTS: serving resources the client is never told about.
func TestServerDiscoverAdvertisesResources(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodServerDiscover, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
	res := resultOf(t, parsed)
	caps, _ := res["capabilities"].(map[string]any)
	if caps["resources"] == nil {
		t.Error("server capabilities missing 'resources'")
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
