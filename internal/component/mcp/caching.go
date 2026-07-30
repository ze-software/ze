// Design: docs/architecture/mcp/overview.md -- cacheable result hints (ttlMs, cacheScope)
// Related: streamable_tools.go -- runMethod stamps every dispatched result through this table
// Related: discover.go -- server/discover is one of the four cacheable surfaces
// Related: resources.go -- resources/list and resources/read are the embedded-asset surfaces

// Cacheable results for MCP 2026-07-28 (SEP-2549).
//
// The revision lets a server tell a client how long a result stays fresh and
// who may cache it, so an agent stops re-listing tools on every turn. Two
// fields carry it, and both are non-optional members of CacheableResult:
// `ttlMs` and `cacheScope`.
//
// MCP 2026-07-28 server/utilities/caching Section "Cacheable Results":
// "Servers MUST include caching hints on results with resultType: "complete"
// returned by the following operations: server/discover, tools/list,
// prompts/list, resources/list, resources/templates/list, resources/read."
//
// Ze implements four of those six. prompts/list and resources/templates/list
// are not dispatched at all, and an unimplemented method answers with a
// JSON-RPC error rather than a result, so it carries no hints and breaches
// nothing.
//
// The hints are applied from ONE site (runMethod) driven by the table below,
// deliberately NOT from the shared ok() responder that stamps resultType and
// serverInfo. tools/call rides that same responder and must never carry a
// CACHING hint in any of its result shapes -- the same page says interim
// results with resultType: "input_required" "are not cacheable and carry no
// caching hints", and results produced by an MRTR retry "MUST NOT be cached".
// Folding cache hints into ok() would put them on tools/call, which is the one
// place where adding the fields "for consistency" is a conformance error.
//
// # `ttlMs` on a CreateTaskResult is NOT a caching hint. Do not "fix" it away.
//
// A tools/call answered with resultType: "task" carries `ttlMs` and
// `pollIntervalMs` (streamable_tools.go, createTask). That is correct and
// required: the io.modelcontextprotocol/tasks extension specifies that a server
// accepting a call as a task responds with "a Task object containing a unique
// taskId, initial status, ttlMs, and pollIntervalMs". Those are the extension's
// own RETENTION fields -- how long the handle stays pollable and how often to
// poll it -- produced by taskRegistry.retentionHints (tasks.go), which the
// caching table below has no connection to.
//
// The two are told apart by the pair, not by the spelling:
//
//	| Producer                      | ttlMs | cacheScope | pollIntervalMs | Means                    |
//	|-------------------------------|-------|------------|----------------|--------------------------|
//	| stampCacheHints (this file)   | yes   | yes        | no             | cache this result        |
//	| createTask (tasks extension)  | yes   | no         | yes            | poll this handle until   |
//
// `cacheScope` is therefore the discriminator, and it is the field to assert on:
// a caching hint is the (ttlMs, cacheScope) PAIR, and cacheTTLByMethod is the
// only thing that can ever emit it. A task result carrying `cacheScope` would be
// the real defect; a task result carrying `ttlMs` is the extension working.
// TestToolsCallCarriesNoCacheHints (streamable_test.go) pins both shapes.

package mcp

// Result keys for the two CacheableResult fields. MCP wire keys are camelCase,
// so these are the specification's spellings verbatim rather than Ze names.
const (
	resultKeyTTLMs      = "ttlMs"
	resultKeyCacheScope = "cacheScope"
)

// cacheScopePrivate is the ONLY cache scope this server emits, on every
// cacheable result, unconditionally.
//
// This is a security decision, not a formatting one, and it is deliberately
// expressed as one constant applied from one site rather than as a per-method
// field: there is no code path that could choose otherwise, so no future audit
// is owed when the tool surface becomes principal-dependent
// (ai/rules/fail-closed-guards.md). Note what is NOT in this file: there is no
// cacheScopePublic constant. Emitting "public" would require typing a bare
// string literal into a surface that TestCacheScopeIsNeverPublic reads back
// over HTTP, so the change fails a gate rather than passing a review.
//
// Why "private" when Ze's tool list is currently identical for every principal:
//
//   - The whole endpoint sits behind authentication. MCP 2026-07-28
//     server/utilities/caching Section "Security Considerations" warns that a
//     "public" result "may be shared between callers even if the Result is
//     coming from an authenticated endpoint. For example, the Result from an
//     authenticated tools/list call with a "public" cacheScope may be cached by
//     a client and may be shared outside of the initial requests authorization
//     context."
//   - The auth modes already carry per-identity scopes (BearerListEntry.Scopes,
//     Identity.Scopes), and Identity.HasScope (auth.go) is the exact hook a
//     scope-filtered tool list would use. MCP 2026-07-28 server/tools states
//     the tool set "MAY vary by the authorization presented on the request", so
//     a per-principal tool list is an anticipated design, not a hypothetical.
//     The day someone wires that up, a "public" tool list would already have
//     leaked one principal's commands to another through a shared gateway.
//   - Nothing is narrowed by choosing it. "private" is the strictly more
//     restrictive value and the specification imposes no obligation to
//     advertise "public"; the same page only says "public" "is appropriate for
//     lists of tools ... when they are identical for all users".
//
// The same page also requires that a paginated list keep one scope across every
// page ("Servers MUST apply the same cacheScope to all response pages for a
// given list request"), which one unconditional constant satisfies by
// construction.
const cacheScopePrivate = "private"

// The two freshness classes, one per surface mutability class. Both are
// compile-time constants on purpose: the right lifetime is a function of how
// the underlying data changes, which the code knows and an operator does not.
// A YANG leaf or env var could be set to a value contradicting the server's
// real invalidation behavior (a one-hour tool-list TTL on a list that changes
// at every config reload) with no way for Ze to detect or reject the
// contradiction, which is the failure ai/rules/exact-or-reject.md exists to
// prevent.
const (
	// ttlRegistryDerivedMs is the freshness of a surface assembled from the
	// command registry, which is re-read from the dispatcher on every call and
	// therefore changes when a plugin registers or a config reload lands.
	//
	// 60 s rather than the specification example's 300000: without
	// subscriptions/listen this TTL is the only invalidation lever Ze has, so
	// it doubles as the bound on how long a client may keep offering a command
	// a reload removed. A minute is long enough to stop an agent re-listing
	// tools every turn and short enough that an operator sees a reload
	// reflected while still watching.
	ttlRegistryDerivedMs = 60000

	// ttlEmbeddedAssetMs is the freshness of a surface served from the embedded
	// UI filesystem, which is fixed for the binary's lifetime (//go:embed, and
	// cachedResources is built once at construction).
	//
	// One hour rather than a day, even though the bytes cannot change while the
	// process runs: they DO change across an upgrade, and unlike a stale tool
	// list a stale UI bundle produces no error for a client to recover from. It
	// simply renders the old panel. The surface with the more "immutable"
	// contents therefore gets the shorter practical ceiling.
	ttlEmbeddedAssetMs = 3600000
)

// cacheTTLByMethod is the closed table of methods whose results carry cache
// hints, mapping each to its freshness class. Membership is the whole decision:
// a method absent from this table carries no `ttlMs` and no `cacheScope`.
//
// The table exists so a new method cannot silently miss the stamp, and so a new
// method cannot silently GAIN it either. tools/call and every tasks/* method
// are absent deliberately, not by omission -- including the tools/call that
// answers with a CreateTaskResult, whose `ttlMs` comes from the tasks extension
// and not from here (see the discriminator table at the top of this file).
// TestCacheableMethodsMatchSpecification pins both directions against the
// specification's operation list and against the dispatch switch.
var cacheTTLByMethod = map[string]int{
	methodServerDiscover: ttlRegistryDerivedMs,
	methodToolsList:      ttlRegistryDerivedMs,
	methodResourcesList:  ttlEmbeddedAssetMs,
	methodResourcesRead:  ttlEmbeddedAssetMs,
}

// stampCacheHints adds the CacheableResult fields to a dispatched response when
// the method is one of the cacheable operations AND the result is a finished
// one.
//
// Called from exactly one place, on the way out of dispatch, so no handler can
// forget it and no handler can add hints the table does not sanction.
//
// Errors are skipped: an error response carries no `result` object at all, and
// the caching page's requirement is on results. The result map is mutated in
// place, which is safe because ok() already copied it away from whatever the
// handler owned before returning.
func stampCacheHints(method string, resp *response) *response {
	ttlMs, cacheable := cacheTTLByMethod[method]
	if !cacheable {
		return resp
	}
	if resp == nil || resp.Error != nil {
		return resp
	}
	// Always true on the success path: ok() is the only constructor of a
	// successful result and it always builds a map[string]any. Guarding rather
	// than asserting keeps a hypothetical bug from panicking a live request;
	// TestCacheableResultsCarryHints drives all four methods over HTTP, so a
	// result that stopped being an object would fail a gate rather than quietly
	// drop a MUST field.
	result, isObject := resp.Result.(map[string]any)
	if !isObject {
		return resp
	}
	// The resultType gate, not merely the method gate. The quoted MUST above is
	// scoped to results "with resultType: \"complete\"", and the same page says
	// an interim `input_required` result "is not cacheable and carries no
	// caching hints". Method membership alone cannot express that: resources/read
	// sits in cacheTTLByMethod AND in permitsInputRequired (mrtr.go), so the day
	// it learns to ask for input the table would happily stamp a caching hint
	// onto the prompt. Unreachable today -- resourcesRead has no elicitation --
	// which is exactly when the guard is cheap to add and free to keep.
	if kind, _ := result[resultTypeKey].(string); kind != resultTypeComplete {
		return resp
	}
	result[resultKeyTTLMs] = ttlMs
	result[resultKeyCacheScope] = cacheScopePrivate
	return resp
}
