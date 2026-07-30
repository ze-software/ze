# 1302 -- mcp2026-4-caching-apps

## Context

Phase 4 of the `2026-07-28` cutover. Phase 1 excluded two additive conformance
items, so the transport cutover stayed atomic.

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
  of a surface's mutability. The code knows that mutability and an operator does
  not. A config surface would let someone set a lifetime that contradicts the
  server's real invalidation behavior, and Ze cannot detect that contradiction.
- **`cacheScope` is `"private"` unconditionally.** Ze's tool list *is* identical
  for every principal today, so `"public"` would be accurate -- and was rejected
  anyway. The endpoint is authenticated, and `"public"` licenses a copy outside
  the request's authorization context. The auth modes already carry per-identity
  scopes that a scope-filtered tool list would use. Nothing is lost by the
  stricter value, and no MUST requires `"public"`.
- **The stamp lives in a closed table consulted at one site**, not in `ok()`.
  `ok()` is shared by every result including `tools/call`, which must carry no
  hints, so a blanket stamp there would be a conformance error.
- **The `_meta.ui` gate went on the assembled tool list, not inside
  `buildToolDef`**, so it also covers `ToolProvider` descriptors. Inside
  `buildToolDef`, a provider can emit `_meta.ui` to a non-declaring client and
  breach the extension fallback rule. Fallback is to omit, never to reject.

## Consequences

- Without `subscriptions/listen` there is no push invalidation, so `ttlMs` is the
  only lever. For up to 60 s after a config reload, a client can still offer a
  removed tool. The specification sanctions this:
  <!-- The next line is a verbatim MCP specification sentence, kept as evidence. -->
  <!-- ste: ignore -->
  "A server **MAY** provide `ttlMs` without advertising `listChanged`."
  The failure is self-correcting, because a call to a removed tool produces the
  error that prompts a re-fetch. The bound is documented as a Known Limitation.
- `_meta.ui` field names needed no change. They were verified against the live
  extension page rather than the spec document. `resourceUri`, `csp` and
  `permissions` are field-for-field what Ze already emitted.

## Gotchas

- **`cacheScope: "public"` is hard to reach, not merely discouraged.** There is
  no `cacheScopePublic` constant, so a bare literal must be typed into a surface
  that a test reads over HTTP. A comment that says "do not" would not have
  survived a future refactor. The absence of the constant does survive.
- **`ttlMs` means two different things, and the invariant was stated wrongly.**
  `CreateTaskResult` legitimately carries `ttlMs` and `pollIntervalMs`, which are
  the tasks extension's *retention* fields. A caching hint is the
  `(ttlMs, cacheScope)` **pair**. The code was right, but `caching.go`'s comment
  and its test both claimed `tools/call` carries no hints "in either result
  shape". That claim would have led a future reader to "fix" the task result and
  delete a required field. An explicit discriminator corrects it: no `cacheScope`
  means it is not a caching hint.
- **Determinism was already safe, and the reason is worth keeping.**
  `groupCommands` builds from maps but sorts on `prefix`. A `prefix` is a map key
  and is therefore unique, so the sort has a total order and the non-stable
  `sort.Slice` is safe. No change was needed. Tests were added anyway, because
  the property depends on an invariant that a future edit can break in silence.
- **A guard that derives from a hand-written list guards nothing.**
  `TestCacheableMethodsMatchSpecification` claimed to catch a new cacheable
  method that reaches the dispatch switch with no hints. But it derived its
  method set from a hand-maintained table rather than from the switch. Two extra
  methods were added to the switch, the test still passed, and that proved it
  vacuous. It now parses the dispatch switch from source, and five other tests
  that read the same un-derived list are gated with it.
- **An assertion for the wire must compare the wire.** The tools-order test
  compared tool *names*. But the AC asks for a byte-identical `tools` array with
  every enum and description, and that is what a prompt-cache hit depends on. It
  now compares raw bytes and reports a SHA-256 prefix, so the comparison cannot
  stop in silence.

## Files

- `internal/component/mcp/{caching,apps}.go` (new) + tests
- `internal/component/mcp/{discover,resources,streamable_tools,tools}.go`
- `internal/component/mcp/{dispatch_surface_test,tools_determinism_test}.go`
- `test/plugin/mcp-{tools-list-cache-hints,ui-extension-fallback,discover-ui-extension,tools-list-deterministic-order}.ci`
- `docs/architecture/mcp/overview.md`, `docs/guide/mcp/overview.md`, `docs/comparison.md`
