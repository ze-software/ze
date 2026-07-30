// Design: docs/architecture/mcp/overview.md -- cacheable result hints (ttlMs, cacheScope)
// Related: streamable_tools.go -- runMethod stamps every dispatched result through this table
// Related: discover.go -- server/discover is one of the four cacheable surfaces
// Related: resources.go -- resources/list and resources/read are the embedded-asset surfaces

// Cacheable results for MCP 2026-07-28 (SEP-2549).
//
// The revision lets a server tell a client how long a result stays fresh and
// who can cache it. An agent therefore stops re-listing tools on every turn.
// Two fields carry the hint, and both are non-optional members of
// CacheableResult: `ttlMs` and `cacheScope`.
//
// MCP 2026-07-28 server/utilities/caching Section "Cacheable Results":
// "Servers MUST include caching hints on results with resultType: "complete"
// returned by the following operations: server/discover, tools/list,
// prompts/list, resources/list, resources/templates/list, resources/read."
//
// Ze implements four of those six. Ze dispatches neither prompts/list nor
// resources/templates/list. An unimplemented method answers with a JSON-RPC
// error rather than a result. That answer carries no hints, and it breaches
// nothing.
//
// One site applies the hints: runMethod, driven by the table below. The shared
// ok() responder, which stamps resultType and serverInfo, deliberately does
// NOT apply them.
//
// tools/call rides that same ok() responder. And tools/call must never carry a
// CACHING hint in any of its result shapes. The same page says that interim
// results with resultType: "input_required" "are not cacheable and carry no
// caching hints". It also says that results produced by an MRTR retry "MUST
// NOT be cached".
//
// Cache hints inside ok() would therefore reach tools/call. That is the one
// place where the fields added "for consistency" become a conformance error.
//
// # `ttlMs` on a CreateTaskResult is NOT a caching hint. Do not "fix" it away.
//
// A tools/call answered with resultType: "task" carries `ttlMs` and
// `pollIntervalMs` (streamable_tools.go, createTask). That is correct and
// required. The io.modelcontextprotocol/tasks extension specifies that a
// server which accepts a call as a task responds with "a Task object
// containing a unique taskId, initial status, ttlMs, and pollIntervalMs".
//
// Those two fields are the extension's own RETENTION fields. `ttlMs` says how
// long the handle stays pollable, and `pollIntervalMs` says how often to poll
// it. taskRegistry.retentionHints (tasks.go) produces the pair, and that
// function has no connection to the caching table below.
//
// The two are told apart by the pair, not by the spelling:
//
//	| Producer                      | ttlMs | cacheScope | pollIntervalMs | Means                    |
//	|-------------------------------|-------|------------|----------------|--------------------------|
//	| stampCacheHints (this file)   | yes   | yes        | no             | cache this result        |
//	| createTask (tasks extension)  | yes   | no         | yes            | poll this handle until   |
//
// `cacheScope` is therefore the discriminator, and it is the field to assert
// on. A caching hint is the (ttlMs, cacheScope) PAIR, and cacheTTLByMethod is
// the only thing that can ever emit that pair. A task result carrying
// `cacheScope` would be the real defect. A task result carrying `ttlMs` is the
// extension working.
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
// This is a security decision, not a formatting one. Ze expresses it as one
// constant applied from one site, and not as a per-method field. No code path
// can therefore choose another scope. And no future audit is owed when the
// tool surface becomes principal-dependent (ai/rules/fail-closed-guards.md).
//
// Note what is NOT in this file: there is no cacheScopePublic constant. To
// emit "public", an author must type a bare string literal into a surface that
// TestCacheScopeIsNeverPublic reads back over HTTP. The change therefore fails
// a gate rather than pass a review.
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
//
//   - The auth modes already carry per-identity scopes (BearerListEntry.Scopes,
//     Identity.Scopes), and Identity.HasScope (auth.go) is the exact hook a
//     scope-filtered tool list would use. MCP 2026-07-28 server/tools states
//     the tool set "MAY vary by the authorization presented on the request", so
//     a per-principal tool list is an anticipated design, not a hypothetical.
//     On the day that someone implements the scope filter, a "public" tool list
//     leaks one principal's commands to another through a shared gateway.
//
//   - Nothing is narrowed by the choice. "private" is the strictly more
//     restrictive value, and the specification imposes no obligation to
//     advertise "public". The same page only says "public" "is appropriate for
//     lists of tools ... when they are identical for all users".
//
// The same page also requires that a paginated list keep one scope across every
// page ("Servers MUST apply the same cacheScope to all response pages for a
// given list request"), which one unconditional constant satisfies by
// construction.
const cacheScopePrivate = "private"

// The two freshness classes, one per surface mutability class. Both are
// compile-time constants on purpose. The right lifetime is a function of how
// the underlying data changes, and the code knows that where an operator does
// not.
//
// An operator can set a YANG leaf or an env var to a value that contradicts
// the server's real invalidation behavior. One example is a one-hour tool-list
// TTL on a list that changes at every config reload. Ze has no way to find or
// reject that contradiction, which is the failure ai/rules/exact-or-reject.md
// exists to prevent.
const (
	// ttlRegistryDerivedMs is the freshness of a surface assembled from the
	// command registry. Ze re-reads that registry from the dispatcher on every
	// call. The surface therefore changes when a plugin registers, and it
	// changes when a config reload lands.
	//
	// The value is 60 s rather than the 300000 of the specification example.
	// Without subscriptions/listen this TTL is the only invalidation lever Ze
	// has. It therefore also bounds how long a client can still offer a command
	// that a reload removed. A minute is long enough to stop an agent
	// re-listing tools every turn. And a minute is short enough that an
	// operator sees a reload while they are still watching.
	ttlRegistryDerivedMs = 60000

	// ttlEmbeddedAssetMs is the freshness of a surface served from the embedded
	// UI filesystem, which is fixed for the binary's lifetime (//go:embed, and
	// cachedResources is built once at construction).
	//
	// The value is one hour rather than a day, even though the bytes cannot
	// change while the process runs. The bytes DO change across an upgrade. And
	// a stale UI bundle, unlike a stale tool list, produces no error for a
	// client to recover from. It renders the old panel instead. The surface
	// with the more "immutable" contents therefore gets the shorter practical
	// ceiling.
	ttlEmbeddedAssetMs = 3600000
)

// cacheTTLByMethod is the closed table of methods whose results carry cache
// hints, mapping each to its freshness class. Membership is the whole decision:
// a method absent from this table carries no `ttlMs` and no `cacheScope`.
//
// The table exists so a new method cannot silently miss the stamp, and so a new
// method cannot silently GAIN it either. tools/call and every tasks/* method
// are absent deliberately, not by omission. That includes the tools/call that
// answers with a CreateTaskResult. Its `ttlMs` comes from the tasks extension
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
// One place calls stampCacheHints, as dispatch returns. No handler can
// therefore forget it, and no handler can add hints the table does not
// sanction.
//
// Errors are skipped: an error response carries no `result` object at all, and
// the caching page's requirement is on results. The result map is mutated in
// place. That is safe because ok() already copied the map away from whatever
// the handler owned before it returned.
func stampCacheHints(method string, resp *response) *response {
	ttlMs, cacheable := cacheTTLByMethod[method]
	if !cacheable {
		return resp
	}
	if resp == nil || resp.Error != nil {
		return resp
	}
	// Always true on the success path: ok() is the only constructor of a
	// successful result, and it always builds a map[string]any. A guard rather
	// than an assertion keeps a hypothetical bug from a panic on a live
	// request. TestCacheableResultsCarryHints drives all four methods over
	// HTTP. A result that stopped being an object would therefore fail a gate
	// rather than quietly drop a MUST field.
	result, isObject := resp.Result.(map[string]any)
	if !isObject {
		return resp
	}
	// The resultType gate, not merely the method gate. The quoted MUST above is
	// scoped to results "with resultType: \"complete\"", and the same page says
	// an interim `input_required` result "is not cacheable and carries no
	// caching hints".
	//
	// Method membership alone cannot express that. resources/read sits in
	// cacheTTLByMethod AND in permitsInputRequired (mrtr.go). On the day
	// resources/read learns to ask for input, the table would stamp a caching
	// hint onto the prompt. That day is unreachable today, because
	// resourcesRead has no elicitation. And that is exactly when the guard is
	// cheap to add and free to keep.
	if kind, _ := result[resultTypeKey].(string); kind != resultTypeComplete {
		return resp
	}
	result[resultKeyTTLMs] = ttlMs
	result[resultKeyCacheScope] = cacheScopePrivate
	return resp
}
