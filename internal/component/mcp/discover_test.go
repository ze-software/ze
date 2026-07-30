package mcp

import (
	"net/http"
	"testing"
)

// TestServerDiscoverShape covers AC-3.
//
// VALIDATES: server/discover returns supportedVersions, capabilities,
// instructions, resultType "complete", and serverInfo under the result's
// _meta -- not at the top of result.
// PREVENTS: shipping the pre-cutover InitializeResult shape (serverInfo at the
// top level, protocolVersion instead of supportedVersions) under a new method
// name.
func TestServerDiscoverShape(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodServerDiscover, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
	result, ok := parsed["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result object in %v", parsed)
	}

	if result["resultType"] != resultTypeComplete {
		t.Errorf("resultType = %v, want %q", result["resultType"], resultTypeComplete)
	}

	versions, ok := result["supportedVersions"].([]any)
	if !ok {
		t.Fatalf("supportedVersions = %v, want an array", result["supportedVersions"])
	}
	if len(versions) != 1 || versions[0] != ProtocolVersion {
		t.Errorf("supportedVersions = %v, want [%q]", versions, ProtocolVersion)
	}

	caps, ok := result["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities = %v, want an object", result["capabilities"])
	}
	for _, key := range []string{"tools", "resources", "extensions"} {
		if _, present := caps[key].(map[string]any); !present {
			t.Errorf("capabilities.%s missing or not an object: %v", key, caps)
		}
	}

	// The MCP Apps extension is advertised, with the empty settings object the
	// specification defines as "support with no additional settings".
	extensions, _ := caps["extensions"].(map[string]any)
	uiSettings, advertised := extensions[extensionUI].(map[string]any)
	if !advertised {
		t.Errorf("capabilities.extensions[%q] missing or not an object: %v", extensionUI, extensions)
	}
	if len(uiSettings) != 0 {
		t.Errorf("capabilities.extensions[%q] = %v, want an empty settings object", extensionUI, uiSettings)
	}

	if instructions, _ := result["instructions"].(string); instructions == "" {
		t.Error("instructions missing or empty")
	}

	// serverInfo belongs under result._meta, NOT at the top of result.
	if _, atTop := result["serverInfo"]; atTop {
		t.Error("serverInfo at the top of result; it belongs under result._meta")
	}
	meta, ok := result[metaKey].(map[string]any)
	if !ok {
		t.Fatalf("result._meta = %v, want an object", result[metaKey])
	}
	info, ok := meta[metaKeyServerInfo].(map[string]any)
	if !ok {
		t.Fatalf("result._meta[%q] = %v, want an object", metaKeyServerInfo, meta[metaKeyServerInfo])
	}
	if info["name"] != defaultServerName {
		t.Errorf("serverInfo.name = %v, want %q", info["name"], defaultServerName)
	}
	if info["version"] != serverVersion {
		t.Errorf("serverInfo.version = %v, want %q", info["version"], serverVersion)
	}

	// DiscoverResult extends CacheableResult, whose ttlMs and cacheScope are
	// both NON-optional. server/discover is registry-derived, so it carries the
	// same 60 s freshness as tools/list.
	if ttl, _ := result[resultKeyTTLMs].(float64); int(ttl) != ttlRegistryDerivedMs {
		t.Errorf("%s = %v, want %d", resultKeyTTLMs, result[resultKeyTTLMs], ttlRegistryDerivedMs)
	}
	if result[resultKeyCacheScope] != cacheScopePrivate {
		t.Errorf("%s = %v, want %q", resultKeyCacheScope, result[resultKeyCacheScope], cacheScopePrivate)
	}
}

// VALIDATES: in Provider mode server/discover reports the provider's name.
// PREVENTS: a ze-chaos listener identifying itself as the ze daemon.
func TestServerDiscoverProviderName(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Provider: fakeProvider{}})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodServerDiscover, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
	result, _ := parsed["result"].(map[string]any)
	meta, _ := result[metaKey].(map[string]any)
	info, ok := meta[metaKeyServerInfo].(map[string]any)
	if !ok {
		t.Fatalf("result._meta[%q] missing: %v", metaKeyServerInfo, result)
	}
	want := fakeProvider{}.ServerName()
	if info["name"] != want {
		t.Errorf("serverInfo.name = %v, want %q", info["name"], want)
	}
}

// TestDiscoverAdvertisesTasksExtension covers AC-11.
//
// VALIDATES: server/discover names io.modelcontextprotocol/tasks in
// capabilities.extensions, with an empty settings object.
// PREVENTS: the inconsistency this phase closes. The server advertised an empty
// extension set and already served tasks/*, so it claimed non-support for what
// it served. This row also underwrites the wire contract. MCP 2026-07-28
// basic/index says a client's legal ResultType set is the core set plus "any
// additional values of supported extensions that are advertised via
// capabilities". So resultType "task" is only interpretable because this row
// exists.
func TestDiscoverAdvertisesTasksExtension(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodServerDiscover, capsNone, "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
	caps, ok := resultOf(t, parsed)["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("no capabilities object: %v", parsed)
	}
	extensions, ok := caps["extensions"].(map[string]any)
	if !ok {
		t.Fatalf("capabilities.extensions missing or not an object: %v", caps)
	}
	settings, advertised := extensions[extensionTasks].(map[string]any)
	if !advertised {
		t.Fatalf("capabilities.extensions[%q] missing or not an object: %v", extensionTasks, extensions)
	}
	if len(settings) != 0 {
		t.Errorf("capabilities.extensions[%q] = %v, want an empty settings object", extensionTasks, settings)
	}
}

// TestBareTasksMemberIsNotAnExtensionDeclaration pins the negotiation cutover.
//
// VALIDATES: the 2025-11-25 bare `tasks` capability member no longer declares
// task support. Only the io.modelcontextprotocol/tasks identifier under
// `extensions` does.
// PREVENTS: an unsolicited task handle pushed at a legacy client. Under the old
// model that client asked for each task itself. Under the server-directed model
// (D-1), a server that honored its stale declaration would hand it a resultType
// it never agreed to receive. `tasks` is not a ClientCapabilities member in
// this revision in any case.
func TestBareTasksMemberIsNotAnExtensionDeclaration(t *testing.T) {
	if got := parseClientCapabilities(map[string]any{"tasks": map[string]any{}}); got.Tasks {
		t.Errorf(`bare {"tasks":{}} declared task support, want it ignored`)
	}
	extensionForm := map[string]any{
		"extensions": map[string]any{extensionTasks: map[string]any{}},
	}
	if got := parseClientCapabilities(extensionForm); !got.Tasks {
		t.Errorf("extension declaration did not register task support: %v", extensionForm)
	}
}
