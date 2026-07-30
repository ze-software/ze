package mcp

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Client-capability literals for the MCP Apps extension, one per row of the
// design's settings table.
const (
	capsUIBare       = `{"extensions":{"io.modelcontextprotocol/ui":{}}}`
	capsUIProfiled   = `{"extensions":{"io.modelcontextprotocol/ui":{"mimeTypes":["text/html;profile=mcp-app"]}}}`
	capsUIPlainHTML  = `{"extensions":{"io.modelcontextprotocol/ui":{"mimeTypes":["text/html"]}}}`
	capsUIOtherTypes = `{"extensions":{"io.modelcontextprotocol/ui":{"mimeTypes":["image/png","application/pdf"]}}}`
	capsOtherExt     = `{"extensions":{"io.modelcontextprotocol/tasks":{}}}`
)

// uiAnnotatedCommands is a command set whose `show bgp` group carries a UI
// bundle, so the generated descriptor has a _meta.ui object for the gate to
// admit or remove.
func uiAnnotatedCommands() []CommandInfo {
	return []CommandInfo{
		{
			Name: "show bgp peer list",
			Help: "List peers",
			UIResource: &UIResourceInfo{
				Path:        "bgp-peer/index.html",
				Permissions: "network",
				CSP:         "default-src 'self'",
			},
		},
		{Name: "show bgp peer detail", Help: "Peer details"},
		{Name: "show bgp rib status", Help: "RIB summary"},
		{Name: "show config dump", Help: "Dump config"},
	}
}

// TestUIExtensionSettingsGate covers the five-case settings table and its
// malformed neighbors.
//
// VALIDATES: clientSupportsUIApps answers per MCP 2026-07-28 basic/versioning
// Section "Extension Negotiation" -- an empty settings object is support, a
// mimeTypes list is a constraint, and base-type matching treats bare text/html
// as compatible with text/html;profile=mcp-app.
// PREVENTS: exact-string matching on "text/html;profile=mcp-app", which would
// refuse a host that renders Ze's bundle perfectly well; and a malformed
// settings object being read as support.
func TestUIExtensionSettingsGate(t *testing.T) {
	cases := []struct {
		name string
		caps string
		want bool
	}{
		{name: "no extensions member at all", caps: `{}`, want: false},
		{name: "extensions present, ui absent", caps: capsOtherExt, want: false},
		{name: "ui declared with empty settings", caps: capsUIBare, want: true},
		{name: "ui declared with settings but no mimeTypes", caps: `{"extensions":{"io.modelcontextprotocol/ui":{"other":1}}}`, want: true},
		{name: "mimeTypes carries the profiled html type", caps: capsUIProfiled, want: true},
		{name: "mimeTypes carries bare text/html", caps: capsUIPlainHTML, want: true},
		{name: "mimeTypes carries html among others", caps: `{"extensions":{"io.modelcontextprotocol/ui":{"mimeTypes":["image/png","TEXT/HTML ; charset=utf-8"]}}}`, want: true},
		{name: "mimeTypes carries no html type", caps: capsUIOtherTypes, want: false},
		{name: "mimeTypes is an empty list", caps: `{"extensions":{"io.modelcontextprotocol/ui":{"mimeTypes":[]}}}`, want: false},
		{name: "mimeTypes is not a list", caps: `{"extensions":{"io.modelcontextprotocol/ui":{"mimeTypes":"text/html"}}}`, want: false},
		{name: "ui value is not an object", caps: `{"extensions":{"io.modelcontextprotocol/ui":true}}`, want: false},
		{name: "extensions value is not an object", caps: `{"extensions":[]}`, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var caps map[string]any
			if err := json.Unmarshal([]byte(tc.caps), &caps); err != nil {
				t.Fatalf("caps %q: %v", tc.caps, err)
			}
			if got := clientSupportsUIApps(caps); got != tc.want {
				t.Errorf("clientSupportsUIApps(%s) = %v, want %v", tc.caps, got, tc.want)
			}
			// The same verdict must reach the parsed capability set, which is
			// what every handler actually reads.
			if got := parseClientCapabilities(caps).UIApps; got != tc.want {
				t.Errorf("parseClientCapabilities(%s).UIApps = %v, want %v", tc.caps, got, tc.want)
			}
		})
	}
}

// TestUIMetadataGatedOnExtensionSettings covers AC-8, AC-9 and AC-11 over the
// real transport.
//
// VALIDATES: a UI-annotated tool carries _meta.ui with resourceUri, permissions
// and csp when the client declared the extension compatibly, and omits _meta
// entirely when it did not -- while staying listed either way.
// PREVENTS: the fallback becoming a rejection. MCP 2026-07-28 basic/versioning
// permits exactly two fallbacks, "revert to core protocol behavior or reject
// the request"; rejecting a whole tools/list would break every non-Apps client.
func TestUIMetadataGatedOnExtensionSettings(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: uiAnnotatedCommands})
	defer cleanup()

	cases := []struct {
		name    string
		caps    string
		wantUI  bool
		comment string
	}{
		{name: "extension absent", caps: capsNone, wantUI: false},
		{name: "other extension only", caps: capsOtherExt, wantUI: false},
		{name: "empty settings", caps: capsUIBare, wantUI: true},
		{name: "profiled html", caps: capsUIProfiled, wantUI: true},
		{name: "bare html", caps: capsUIPlainHTML, wantUI: true},
		{name: "incompatible mime types", caps: capsUIOtherTypes, wantUI: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, parsed := postMCP(t, hs, methodToolsList, tc.caps, "")
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v); the fallback is omission, never rejection", status, parsed)
			}
			tool := toolNamed(t, parsed, "ze_show_bgp")

			meta, hasMeta := tool[metaKey].(map[string]any)
			if !tc.wantUI {
				if hasMeta {
					t.Fatalf("descriptor carries _meta = %v; want it omitted entirely", meta)
				}
				return
			}
			if !hasMeta {
				t.Fatalf("descriptor missing _meta: %v", tool)
			}
			ui, hasUI := meta[metaKeyUI].(map[string]any)
			if !hasUI {
				t.Fatalf("_meta missing %q: %v", metaKeyUI, meta)
			}
			if uri, _ := ui["resourceUri"].(string); uri != "ui://bgp-peer/index.html" {
				t.Errorf("resourceUri = %v, want ui://bgp-peer/index.html", ui["resourceUri"])
			}
			perms, _ := ui["permissions"].([]any)
			if len(perms) != 1 || perms[0] != "network" {
				t.Errorf("permissions = %v, want [network]", ui["permissions"])
			}
			if csp, _ := ui["csp"].(string); csp != "default-src 'self'" {
				t.Errorf("csp = %v, want default-src 'self'", ui["csp"])
			}
		})
	}
}

// VALIDATES: gating _meta.ui off changes nothing else about the tool list --
// same tools, same names, same order, still callable.
// PREVENTS: the gate being implemented as "skip the tool", which would hide
// every UI-annotated command from a non-Apps host instead of hiding its panel.
func TestUIGateRemovesOnlyTheUIMetadata(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: uiAnnotatedCommands})
	defer cleanup()

	_, withUI := postMCP(t, hs, methodToolsList, capsUIBare, "")
	_, withoutUI := postMCP(t, hs, methodToolsList, capsNone, "")

	namesWith := toolNames(t, withUI)
	namesWithout := toolNames(t, withoutUI)
	if len(namesWith) != len(namesWithout) {
		t.Fatalf("tool count differs: %d with the extension, %d without", len(namesWith), len(namesWithout))
	}
	for i := range namesWith {
		if namesWith[i] != namesWithout[i] {
			t.Errorf("tool %d: %q with the extension, %q without", i, namesWith[i], namesWithout[i])
		}
	}

	// And the gated tool still dispatches.
	status, parsed := postMCP(t, hs, methodToolsCall, capsNone,
		`{"name":"ze_show_bgp","arguments":{"action":"peer list"}}`)
	if status != http.StatusOK {
		t.Fatalf("tools/call on a gated tool: status = %d (body %v)", status, parsed)
	}
	if _, isError := parsed["error"]; isError {
		t.Errorf("tools/call on a gated tool failed: %v", parsed["error"])
	}
}

// VALIDATES: a ToolProvider's descriptors go through the same gate, and the
// provider's own maps and slice are not mutated by it.
// PREVENTS: a provider-supplied _meta.ui reaching a client that never declared
// the extension (the gate lives in allTools precisely so it covers this
// origin), and the gate corrupting a provider that returns the same maps on
// every call.
func TestUIGateAppliesToProviderTools(t *testing.T) {
	provider := &uiProvider{}
	s := &Streamable{cfg: StreamableConfig{Provider: provider}}

	gated := s.allTools(clientCapabilities{})
	if len(gated) != 1 {
		t.Fatalf("got %d tools, want 1", len(gated))
	}
	if _, present := gated[0][metaKey]; present {
		t.Errorf("provider descriptor kept _meta through a closed gate: %v", gated[0])
	}

	ungated := s.allTools(clientCapabilities{UIApps: true})
	if _, present := ungated[0][metaKey]; !present {
		t.Errorf("provider descriptor lost _meta through an open gate: %v", ungated[0])
	}

	// The provider still owns intact descriptors: the gate copied rather than
	// edited in place.
	original := provider.Tools()
	meta, present := original[0][metaKey].(map[string]any)
	if !present {
		t.Fatalf("gate mutated the provider's own descriptor: %v", original[0])
	}
	if _, hasUI := meta[metaKeyUI]; !hasUI {
		t.Errorf("gate mutated the provider's own _meta: %v", meta)
	}
}

// VALIDATES: _meta survives the gate when it carries members other than `ui`.
// PREVENTS: the gate deleting a future _meta member as collateral.
func TestUIGateKeepsOtherMetaMembers(t *testing.T) {
	tools := []map[string]any{{
		"name":  "ze_thing",
		metaKey: map[string]any{metaKeyUI: map[string]any{"resourceUri": "ui://x/index.html"}, "other": "keep"},
	}}
	gated := gateUIMeta(tools, false)

	meta, present := gated[0][metaKey].(map[string]any)
	if !present {
		t.Fatalf("_meta dropped even though it carried another member: %v", gated[0])
	}
	if _, hasUI := meta[metaKeyUI]; hasUI {
		t.Errorf("_meta kept %q: %v", metaKeyUI, meta)
	}
	if meta["other"] != "keep" {
		t.Errorf("_meta.other = %v, want keep", meta["other"])
	}
}

// VALIDATES: an open gate returns the caller's slice untouched, allocating
// nothing.
// PREVENTS: a needless per-request copy of every descriptor on the common path.
func TestUIGateOpenReturnsInputUnchanged(t *testing.T) {
	tools := []map[string]any{{"name": "ze_thing", metaKey: map[string]any{metaKeyUI: map[string]any{}}}}
	if gated := gateUIMeta(tools, true); &gated[0] != &tools[0] {
		t.Error("open gate copied the slice; it should return the input as-is")
	}
	plain := []map[string]any{{"name": "ze_thing"}}
	if gated := gateUIMeta(plain, false); &gated[0] != &plain[0] {
		t.Error("closed gate copied a slice holding no _meta.ui; nothing needed changing")
	}
}

// uiProvider is a ToolProvider whose single descriptor carries _meta.ui, and
// which returns the SAME maps on every call, as a real provider holding a
// prebuilt tool list would.
type uiProvider struct {
	tools []map[string]any
}

func (p *uiProvider) ServerName() string { return "ui-provider" }

func (p *uiProvider) Tools() []map[string]any {
	if p.tools == nil {
		p.tools = []map[string]any{{
			"name":  "provider_ui_tool",
			metaKey: map[string]any{metaKeyUI: map[string]any{"resourceUri": "ui://provider/index.html"}},
		}}
	}
	return p.tools
}

func (p *uiProvider) CallTool(string, json.RawMessage) map[string]any { return nil }

// toolNamed pulls one tool descriptor out of a tools/list response.
func toolNamed(t *testing.T, parsed map[string]any, name string) map[string]any {
	t.Helper()
	for _, tool := range toolList(t, parsed) {
		if tool["name"] == name {
			return tool
		}
	}
	t.Fatalf("tool %q absent from %v", name, toolNames(t, parsed))
	return nil
}

// toolList decodes the tools array of a tools/list response.
func toolList(t *testing.T, parsed map[string]any) []map[string]any {
	t.Helper()
	result := resultOf(t, parsed)
	raw, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("result.tools = %v, want an array", result["tools"])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		tool, isObject := entry.(map[string]any)
		if !isObject {
			t.Fatalf("tool entry %v is not an object", entry)
		}
		out = append(out, tool)
	}
	return out
}

// toolNames returns the tool names of a tools/list response, in wire order.
func toolNames(t *testing.T, parsed map[string]any) []string {
	t.Helper()
	tools := toolList(t, parsed)
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		names = append(names, name)
	}
	return names
}
