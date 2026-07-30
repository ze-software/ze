package mcp

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// specCacheableOperations is the operation list MCP 2026-07-28
// server/utilities/caching Section "Cacheable Results" names verbatim:
// "Servers MUST include caching hints on results with resultType: "complete"
// returned by the following operations".
//
// Written out here rather than derived from cacheTTLByMethod so the two can
// disagree, which is the whole point: the table is Ze's answer and this is the
// question.
var specCacheableOperations = []string{
	"server/discover",
	"tools/list",
	"prompts/list",
	"resources/list",
	"resources/templates/list",
	"resources/read",
}

// TestCacheableMethodsMatchSpecification covers the table's two failure
// directions at once.
//
// VALIDATES: every method this server dispatches AND the caching page lists has
// a cacheTTLByMethod entry, and every cacheTTLByMethod entry names a method
// this server both dispatches and the caching page lists.
// PREVENTS: a new cacheable method landing in the dispatch switch with no
// hints (a MUST breached silently), and an entry being added for a method the
// specification does not make cacheable -- notably tools/call, where the fields
// would be a conformance error rather than a harmless extra.
func TestCacheableMethodsMatchSpecification(t *testing.T) {
	dispatched := dispatchedMethods(t)

	for _, operation := range specCacheableOperations {
		_, implemented := dispatched[operation]
		_, hinted := cacheTTLByMethod[operation]
		switch {
		case implemented && !hinted:
			t.Errorf("%s is dispatched and is on the caching page's operation list, but has no cacheTTLByMethod entry", operation)
		case !implemented && hinted:
			t.Errorf("%s has a cacheTTLByMethod entry but is not dispatched; an unimplemented method returns an error, which carries no hints", operation)
		}
	}

	for method := range cacheTTLByMethod {
		if !slices.Contains(specCacheableOperations, method) {
			t.Errorf("cacheTTLByMethod names %s, which the caching page does not list as cacheable", method)
		}
		if _, implemented := dispatched[method]; !implemented {
			t.Errorf("cacheTTLByMethod names %s, which this server does not dispatch", method)
		}
	}

	// tools/call is the specific trap: it IS dispatched, it IS the busiest
	// method, and no shape of its result may carry a CACHING hint. The
	// CreateTaskResult shape does carry a ttlMs, but that is the tasks
	// extension's retention field and it never comes with a cacheScope; only a
	// cacheTTLByMethod entry can produce the pair. See the discriminator table
	// in caching.go and TestToolsCallCarriesNoCacheHints.
	if _, hinted := cacheTTLByMethod[methodToolsCall]; hinted {
		t.Errorf("%s has a cache-hint entry; interim input_required results carry no hints and MRTR retries MUST NOT be cached", methodToolsCall)
	}
}

// dispatchedMethods is the set of JSON-RPC methods dispatchMethod answers with a
// result rather than method-not-found.
//
// Read out of the dispatch switch itself (dispatch_surface_test.go), minus the
// cases that answer with an error. It used to be derived from
// resultBearingMethods, a hand-written table, which made the PREVENTS above a
// promise this test could not keep: a cacheable method added to the switch and
// not to the table came back implemented=false, hinted=false, and neither arm of
// the comparison fired. The table is now itself gated against the switch by
// TestResultBearingMethodsMatchDispatchSwitch, and this reads the switch
// directly so the caching verdict does not depend on that gate having run.
func dispatchedMethods(t *testing.T) map[string]struct{} {
	t.Helper()
	out := dispatchSwitchMethods(t)
	for method := range errorOnlyDispatchCases {
		delete(out, method)
	}
	return out
}

// TestCacheTTLConstantsAreInRange covers AC-14 and the spec's numeric bound.
//
// VALIDATES: every ttlMs this server can emit is an integer >= 0, and the two
// freshness constants are exactly the values the design fixed.
// PREVENTS: a negative ttlMs (MCP 2026-07-28 server/utilities/caching: "Servers
// MUST provide a ttlMs value that is >= 0"), a 0 that would make every result
// immediately stale and defeat SEP-2549 entirely, and an edit silently pinning
// a client for longer than the longest declared class.
func TestCacheTTLConstantsAreInRange(t *testing.T) {
	if ttlRegistryDerivedMs != 60000 {
		t.Errorf("ttlRegistryDerivedMs = %d, want 60000 (the documented 60 s reload window)", ttlRegistryDerivedMs)
	}
	if ttlEmbeddedAssetMs != 3600000 {
		t.Errorf("ttlEmbeddedAssetMs = %d, want 3600000", ttlEmbeddedAssetMs)
	}
	for method, ttl := range cacheTTLByMethod {
		if ttl < 0 {
			t.Errorf("%s ttlMs = %d, want >= 0", method, ttl)
		}
		if ttl == 0 {
			t.Errorf("%s ttlMs = 0, which means immediately stale; no Ze surface may emit it", method)
		}
		if ttl > ttlEmbeddedAssetMs {
			t.Errorf("%s ttlMs = %d, longer than the longest declared class %d", method, ttl, ttlEmbeddedAssetMs)
		}
	}
}

// TestCacheableResultsCarryHints covers AC-1 through AC-4 and AC-14.
//
// VALIDATES: each of the four implemented cacheable surfaces returns a result
// carrying its class's ttlMs and cacheScope "private", over the real HTTP
// transport.
// PREVENTS: the stamp being dropped for one surface while the others keep it,
// which is exactly what a per-handler call would allow.
func TestCacheableResultsCarryHints(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	cases := []struct {
		method string
		params string
		wantMs int
	}{
		{method: methodServerDiscover, wantMs: ttlRegistryDerivedMs},
		{method: methodToolsList, wantMs: ttlRegistryDerivedMs},
		{method: methodResourcesList, wantMs: ttlEmbeddedAssetMs},
		{method: methodResourcesRead, params: `{"uri":"ui://bgp-peer/index.html"}`, wantMs: ttlEmbeddedAssetMs},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			status, parsed := postMCP(t, hs, tc.method, capsNone, tc.params)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
			}
			result := resultOf(t, parsed)

			raw, present := result[resultKeyTTLMs]
			if !present {
				t.Fatalf("%s missing from %v; CacheableResult makes it non-optional", resultKeyTTLMs, result)
			}
			ttl, isNumber := raw.(float64)
			if !isNumber {
				t.Fatalf("%s = %v (%T), want a JSON number", resultKeyTTLMs, raw, raw)
			}
			if int(ttl) != tc.wantMs {
				t.Errorf("%s = %d, want %d", resultKeyTTLMs, int(ttl), tc.wantMs)
			}
			if ttl < 0 {
				t.Errorf("%s = %v, want >= 0", resultKeyTTLMs, ttl)
			}
			if result[resultKeyCacheScope] != cacheScopePrivate {
				t.Errorf("%s = %v, want %q", resultKeyCacheScope, result[resultKeyCacheScope], cacheScopePrivate)
			}
			// Hints ride alongside the envelope fields, not instead of them.
			if result["resultType"] != resultTypeComplete {
				t.Errorf("resultType = %v, want %q", result["resultType"], resultTypeComplete)
			}
		})
	}
}

// TestNonCacheableResultsCarryNoHints covers AC-15.
//
// VALIDATES: the COMPLETE and the INPUT_REQUIRED tools/call result, and every
// tasks/* result, carry neither ttlMs nor cacheScope. The task-shaped tools/call
// result is a separate case with a separate invariant and is covered by
// TestToolsCallCarriesNoCacheHints (streamable_test.go): it legitimately carries
// the tasks extension's own ttlMs and pollIntervalMs, and what must be absent
// there is cacheScope.
//
// The input_required subtest was added 2026-07-30: AC-15 names both shapes and
// only `complete` was driven, because resultBearingMethods calls ze_execute WITH
// a command. resultTypeInputRequired appeared nowhere in this file, so the half
// of the AC that the caching page states most explicitly -- an interim result
// "is not cacheable and carries no caching hints" -- was asserted by nothing at
// the wire level.
// PREVENTS: the hints being folded into the shared ok() responder "for
// consistency". tools/call is absent from the caching page's operation list,
// interim input_required results "are not cacheable and carry no caching
// hints", and MRTR retries "MUST NOT be cached" -- so a hint here is a
// conformance error, not a harmless extra field.
func TestNonCacheableResultsCarryNoHints(t *testing.T) {
	// taskCapableCommands supplies the `required` command createTestTask needs:
	// under the server-directed model the annotation is the only thing that
	// makes a call a task (D-1).
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
	defer cleanup()

	taskID := createTestTask(t, hs)

	for _, probe := range resultBearingMethods(taskID) {
		if _, cacheable := cacheTTLByMethod[probe.method]; cacheable {
			continue
		}
		t.Run(probe.method, func(t *testing.T) {
			status, parsed := postMCP(t, hs, probe.method, probe.caps, probe.params)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
			}
			result := resultOf(t, parsed)
			if raw, present := result[resultKeyTTLMs]; present {
				t.Errorf("%s = %v on a non-cacheable result", resultKeyTTLMs, raw)
			}
			if raw, present := result[resultKeyCacheScope]; present {
				t.Errorf("%s = %v on a non-cacheable result", resultKeyCacheScope, raw)
			}
		})
	}

	// The other half of AC-15: the INTERIM tools/call shape. ze_execute called
	// with no command by a client declaring form-mode elicitation is the one
	// input_required result this server can produce, and the caching page is
	// explicit that such a result "is not cacheable and carries no caching
	// hints". Driven over the same transport rather than through stampCacheHints
	// directly, so it covers the whole dispatch path the way the complete-shape
	// rows above do.
	t.Run(resultTypeInputRequired, func(t *testing.T) {
		status, parsed := postMCP(t, hs, methodToolsCall, capsElicitForm,
			`{"name":"ze_execute","arguments":{}}`)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
		}
		result := resultOf(t, parsed)
		// Guard the fixture: if this ever stops being an interim result the
		// assertions below would pass vacuously against a `complete` one.
		if got := result[resultTypeKey]; got != resultTypeInputRequired {
			t.Fatalf("resultType = %v, want %q -- the interim shape this case exists for was not produced (body %v)",
				got, resultTypeInputRequired, parsed)
		}
		if _, present := result[inputRequestsKey]; !present {
			t.Fatalf("interim result carries no %s: %v", inputRequestsKey, result)
		}
		for _, key := range []string{resultKeyTTLMs, resultKeyCacheScope} {
			if raw, present := result[key]; present {
				t.Errorf("%s = %v on an input_required result; an interim result is not cacheable", key, raw)
			}
		}
		// Nothing nested may smuggle one in either: the inputRequests entry is a
		// server-built object and would carry a stamped hint with it.
		encoded, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		for _, key := range []string{resultKeyTTLMs, resultKeyCacheScope} {
			if strings.Contains(string(encoded), `"`+key+`"`) {
				t.Errorf("interim result mentions %s somewhere: %s", key, encoded)
			}
		}
	})
}

// TestCacheScopeIsNeverPublic covers AC-5.
//
// VALIDATES: no surface this server dispatches emits cacheScope "public", read
// back off the wire rather than off the constant.
// PREVENTS: R-1. The caching page warns that a "public" result from an
// authenticated endpoint "may be shared outside of the initial requests
// authorization context", so a single "public" here would let a shared gateway
// serve one principal's tool list to another the moment the tool surface
// becomes scope-filtered.
func TestCacheScopeIsNeverPublic(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
	defer cleanup()

	taskID := createTestTask(t, hs)

	for _, probe := range resultBearingMethods(taskID) {
		t.Run(probe.method, func(t *testing.T) {
			status, parsed := postMCP(t, hs, probe.method, probe.caps, probe.params)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
			}
			result := resultOf(t, parsed)
			if scope, present := result[resultKeyCacheScope]; present && scope != cacheScopePrivate {
				t.Errorf("%s = %v, want %q; a non-private scope may be shared across authorization contexts",
					resultKeyCacheScope, scope, cacheScopePrivate)
			}
			// Nothing nested may smuggle one in either.
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if strings.Contains(string(encoded), `"cacheScope":"public"`) {
				t.Errorf("result carries a public cacheScope somewhere: %s", encoded)
			}
		})
	}
}

// VALIDATES: an error response carries no cache hints even when the method is a
// cacheable one.
// PREVENTS: caching a failure. The caching page puts the requirement on
// results; an error carries no result object, and a client that cached a
// resources/read failure for an hour would keep serving it after the cause was
// fixed.
func TestCacheHintsAbsentFromErrorResponses(t *testing.T) {
	hs, cleanup := newTestStreamable(t, StreamableConfig{})
	defer cleanup()

	status, parsed := postMCP(t, hs, methodResourcesRead, capsNone, `{"uri":"ui://does-not-exist.html"}`)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %v)", status, parsed)
	}
	rpcErrorOf(t, parsed)
	if _, present := parsed["result"]; present {
		t.Fatalf("error response carries a result: %v", parsed)
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, key := range []string{resultKeyTTLMs, resultKeyCacheScope} {
		if strings.Contains(string(encoded), key) {
			t.Errorf("error response mentions %s: %s", key, encoded)
		}
	}
}

// VALIDATES: stampCacheHints leaves a handler's own _meta and payload keys
// untouched while adding the two fields.
// PREVENTS: the stamp overwriting a result key, which a map write would do
// silently.
//
// The fixture carries resultType "complete" because that is what ok() -- the
// only constructor of a successful result -- always produces, and because
// stampCacheHints now gates on it (see TestStampCacheHintsSkipsInterimResults).
func TestStampCacheHintsPreservesPayload(t *testing.T) {
	resp := &response{
		JSONRPC: "2.0",
		Result: map[string]any{
			resultTypeKey: resultTypeComplete,
			"tools":       []map[string]any{{"name": "ze_execute"}},
			metaKey:       map[string]any{"x": 1},
		},
	}
	stampCacheHints(methodToolsList, resp)

	result, _ := resp.Result.(map[string]any)
	if _, present := result["tools"]; !present {
		t.Error("payload key `tools` lost")
	}
	if _, present := result[metaKey]; !present {
		t.Error("_meta lost")
	}
	if result[resultKeyTTLMs] != ttlRegistryDerivedMs {
		t.Errorf("%s = %v, want %d", resultKeyTTLMs, result[resultKeyTTLMs], ttlRegistryDerivedMs)
	}
	if result[resultKeyCacheScope] != cacheScopePrivate {
		t.Errorf("%s = %v, want %q", resultKeyCacheScope, result[resultKeyCacheScope], cacheScopePrivate)
	}
}

// TestStampCacheHintsSkipsInterimResults closes the one-line hole a review
// found in the stamp's decision procedure.
//
// The quoted MUST is scoped to results "with resultType: \"complete\"", and the
// same page says an interim input_required result "is not cacheable and carries
// no caching hints". The stamp keyed solely on method membership, which cannot
// express that: resources/read is in cacheTTLByMethod AND in
// permitsInputRequired (mrtr.go), so the day resourcesRead learns to ask for
// input, the table would have stamped caching hints onto a prompt.
//
// VALIDATES: a result whose resultType is not "complete" gets neither field,
// even on a method the caching table names.
// PREVENTS: the resultType check being removed as unreachable. It is
// unreachable TODAY, which is what makes it free; the pairing of a cacheable
// method with an input-required-permitted method is what makes it necessary.
func TestStampCacheHintsSkipsInterimResults(t *testing.T) {
	// The pairing that makes this reachable at all. If it ever stops holding the
	// guard is merely harmless, but this is the fact worth pinning.
	if _, cacheable := cacheTTLByMethod[methodResourcesRead]; !cacheable || !permitsInputRequired(methodResourcesRead) {
		t.Fatalf("%s cacheable=%v inputRequiredPermitted=%v: the overlap this guard covers is gone",
			methodResourcesRead, cacheable, permitsInputRequired(methodResourcesRead))
	}

	for _, resultType := range []string{resultTypeInputRequired, resultTypeTask} {
		t.Run(resultType, func(t *testing.T) {
			resp := &response{
				JSONRPC: "2.0",
				Result:  map[string]any{resultTypeKey: resultType},
			}
			stampCacheHints(methodResourcesRead, resp)

			result, _ := resp.Result.(map[string]any)
			for _, key := range []string{resultKeyTTLMs, resultKeyCacheScope} {
				if raw, present := result[key]; present {
					t.Errorf("%s = %v on a %q result; only a complete result carries caching hints", key, raw, resultType)
				}
			}
		})
	}
}
