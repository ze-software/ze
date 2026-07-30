# 1302 -- mcp2026-4-caching-apps

## Context

Phase 4 of the `2026-07-28` cutover, two additive conformance items Phase 1 left
out so the transport cutover stayed atomic.

**Cacheable results (SEP-2549):** `ttlMs` and `cacheScope` are **non-optional**
fields on `CacheableResult`, which `DiscoverResult` extends, required on
`tools/list`, `resources/list`, `resources/read` and `server/discover`. Ze
emitted neither.

**MCP Apps (SEP-1865):** Ze already shipped Apps in the pre-extension shape from
the `2026-01-26` draft, with `_meta.ui.resourceUri` pointing at `ui://` assets.
In `2026-07-28` Apps is a first-class extension, `io.modelcontextprotocol/ui`,
negotiated through the `extensions` capability map with a settings object.

## Decisions

- **Two compile-time TTL constants, no config surface.** 60 s for
  registry-derived surfaces (`tools/list`, `server/discover` -- the command list
  is re-read from the dispatcher on every call), 1 h for embedded assets
  (`resources/*` -- built once from `//go:embed`). The correct TTL is a function
  of a surface's mutability, which the code knows and an operator does not;
  exposing it would let someone set a lifetime contradicting the server's real
  invalidation behaviour with no way for Ze to detect the contradiction.
- **`cacheScope` is `"private"` unconditionally.** Ze's tool list *is* identical
  for every principal today, so `"public"` would be accurate -- and was rejected
  anyway. The endpoint is authenticated, `"public"` licenses sharing outside the
  request's authorization context, and the auth modes already carry per-identity
  scopes that a scope-filtered tool list would use. Nothing is lost by the
  stricter value and no MUST requires advertising `"public"`.
- **The stamp lives in a closed table consulted at one site**, not in `ok()`.
  `ok()` is shared by every result including `tools/call`, which must carry no
  hints, so a blanket stamp there would be a conformance error.
- **The `_meta.ui` gate went on the assembled tool list, not inside
  `buildToolDef`**, so it also covers `ToolProvider` descriptors -- otherwise a
  provider could emit `_meta.ui` to a non-declaring client and breach the
  extension fallback rule. Fallback is to omit, never to reject.

## Consequences

- Without `subscriptions/listen` there is no push invalidation, so `ttlMs` is the
  only lever: for up to 60 s after a config reload a client may still offer a
  removed tool. The specification sanctions this ("A server **MAY** provide
  `ttlMs` without advertising `listChanged`"), and the failure is
  self-correcting, since calling a removed tool produces the error that prompts a
  re-fetch. Documented as a Known Limitation with the bound named.
- `_meta.ui` field names needed no reshaping. Verified against the live extension
  page rather than the spec document: `resourceUri`, `csp` and `permissions` are
  field-for-field what Ze already emitted.

## Gotchas

- **`cacheScope: "public"` is made hard to reach rather than merely discouraged.**
  There is no `cacheScopePublic` constant, so emitting it requires typing a bare
  literal into a surface a test reads back over HTTP. A comment saying "do not"
  would not have survived a future refactor; the absence of the constant does.
- **`ttlMs` means two different things and the invariant was stated wrongly.**
  `CreateTaskResult` legitimately carries `ttlMs` and `pollIntervalMs` -- those
  are the tasks extension's *retention* fields -- while a caching hint is the
  `(ttlMs, cacheScope)` **pair**. The code was right; `caching.go`'s comment and
  its test both claimed `tools/call` carries no hints "in either result shape",
  which would have led a future reader to "fix" the task result by deleting a
  required field. Corrected with an explicit discriminator: no `cacheScope`
  means it is not a caching hint.
- **Determinism was already safe, and the reason is worth keeping.**
  `groupCommands` builds from maps but sorts on `prefix`, which is a map key and
  therefore unique -- a total order, so the non-stable `sort.Slice` cannot bite.
  No change was needed; tests were added to hold the line, since the property
  depends on an invariant a future edit could break silently.
- **A guard that derives from a hand-written list guards nothing.**
  `TestCacheableMethodsMatchSpecification` claimed to catch a new cacheable
  method landing in the dispatch switch without hints, but derived its method set
  from a hand-maintained table rather than from the switch. Proven vacuous by
  adding two methods to the switch and watching it pass. It now parses the
  dispatch switch from source, and five other tests that hung off the same
  un-derived list are gated with it.
- **An assertion for the wire must compare the wire.** The tools-order test
  compared tool *names*, while the AC asks for a byte-identical `tools` array
  including every enum and description -- the thing prompt-cache hits actually
  depend on. It now compares raw bytes and reports a SHA-256 prefix, so the
  comparison cannot silently stop happening.

## Files

- `internal/component/mcp/{caching,apps}.go` (new) + tests
- `internal/component/mcp/{discover,resources,streamable_tools,tools}.go`
- `internal/component/mcp/{dispatch_surface_test,tools_determinism_test}.go`
- `test/plugin/mcp-{tools-list-cache-hints,ui-extension-fallback,discover-ui-extension,tools-list-deterministic-order}.ci`
- `docs/architecture/mcp/overview.md`, `docs/guide/mcp/overview.md`, `docs/comparison.md`
