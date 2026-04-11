# 554 -- Named Service Listeners

## Context

Every service that accepted inbound connections had a named YANG list for its
listen endpoints (`list server { key "name"; ze:listener; uses zt:listener; }`)
but only SSH and the plugin hub actually honoured more than one entry. Web,
looking-glass, MCP, telemetry, and api-server silently took the first entry
and dropped the rest. The API engine (REST + gRPC) additionally modelled its
listener as a single `container server`, so it couldn't even express multi-bind
at the YANG level. Two smaller gaps: compound `ze.<svc>.listen=ip:port,ip:port`
env vars printed "only first endpoint used, multi-bind not yet supported" and
dropped the rest, and `CollectListeners` (port-conflict detection) didn't know
about `api-server` so REST + gRPC collisions were invisible at parse time.

The goal was to converge every service on the same shape so a user can write
two `server <name> { ... }` blocks and have both bound.

## Decisions

- **Pure-slice binder shape over SSH's "primary + extras" split.** SSH already
  binds multiple listeners but splits them into `listener` + `extraListeners`
  with the first doubling as the display address. The plugin hub uses a pure
  `[]Config` where every entry is equal. Every new binder in this spec (web,
  lg, mcp, telemetry, REST, gRPC) picked the pure-slice shape: own
  `configured []string` + `bound []string` under mu, bind everything upfront,
  then serve on every listener via sibling goroutines. The asymmetry of SSH's
  model meant a bind failure on the primary was fatal while a failure on an
  extra was merely logged — inconsistent with the all-or-nothing contract
  the spec wanted. SSH is explicitly out of scope for this spec; a follow-up
  can migrate it to the pure-slice shape once the new binders have miles.
- **All-or-nothing bind with rollback over "bind what you can".** Every
  binder iterates the configured addresses, binds each, and rolls back every
  already-bound listener if any bind fails (`closeAllListeners` helpers in
  web / lg; inline loops elsewhere). Serve goroutines never start on a
  partial set. AC-15 is verified by per-binder
  `TestXxx_BindFailureClosesPartialListeners` tests that squat on a port and
  assert the first port is released after the rollback.
- **Env var replaces the config slice wholesale.** When `ze.web.listen=a,b`
  is set, the config-file list is ignored entirely. Partial merge (append
  env entries to config entries) was rejected as surprising and hard to test.
  Precedence documented as env > CLI > config > YANG default in every
  service. AC-9 is encoded as the mechanical fact that `runYANGConfig` only
  reads from ExtractXxx when `len(<svc>Addrs) == 0`.
- **`knownListenerServices` kept as a static table over a schema-driven
  walker.** The spec's Design Insights flagged a schema walker keyed on
  `ze:listener` as an optional long-term cleanup: it would remove the
  hardcoded table and the per-service `enabled` leaf hints automatically.
  Not implemented because the table has 8 entries and the api-server
  addition was trivial, and because a schema walker needs to understand
  the surrounding context (plugin hub's `alwaysEnabled` variant, the
  `enabled` leaf position). AC-17 is explicitly marked skipped with a
  deferral destination ("follow-up spec if a 9th service needs coverage").
- **Shared `ServerEndpoint{Host, Port}` type with `Listen()` method.**
  Web, mcp, and lg share one type in `internal/component/config/loader_extract.go`.
  The api-server and telemetry binders keep their own endpoint types
  (`APIListenConfig`, `metrics.Endpoint`) because they carry transport-specific
  context (telemetry uses int ports for the http.Server, api types have
  existed since before this spec). No attempt to unify — YAGNI.
- **Empty-list fallback synthesizes one default entry from YANG refine.**
  Legacy configs like `web { enabled true; }` with no server block still
  work: the extractor reads the YANG refine defaults (`0.0.0.0:3443` for
  web, `0.0.0.0:8081` for REST, etc.) and emits a single ServerEndpoint.
  Picked this over "require at least one explicit server block" because
  it preserves backward compatibility and lets env vars stay a pure
  override.
- **Twelve commits, not eight.** The spec originally listed 8 phases but
  every binder became its own commit so cherry-picks have single-package
  blast radius. Chunks 2 (api YANG) and 3 (Extract helpers) are the only
  load-bearing prerequisites; chunks 4-9 (one per binder) can land in any
  order after those two.

## Consequences

- **New services with ze:listener automatically get parse-time conflict
  detection**, but only if they're added to `knownListenerServices`. The
  hardcoded table is now a maintenance tax that grows one entry per new
  listener type; the schema walker from AC-17 is there to pick up when the
  table hits the wrong side of the cost/benefit curve. Name each new entry
  with the container path (e.g. `{name: "api-server-rest", containers:
  []string{"environment", "api-server", "rest"}}`).
- **Backward compatibility**: single-entry configs keep working unchanged.
  `server main { ip 0.0.0.0; port 3443; }` parses and binds exactly as
  before; the only difference is that `server admin { ... }` next to it
  now binds a second listener instead of being silently ignored.
- **Binder lifetime contract is uniform**: every service exposes
  `Addresses() []string` (plural) alongside the existing `Address()` that
  returns the first entry. Consumers that only care about the primary
  endpoint keep using `Address()`; multi-bind-aware code iterates
  `Addresses()`.
- **MCP and insecure-web sanitization is per-entry, not per-service.**
  MCP rewrites every non-loopback entry to `127.0.0.1` with a warning per
  rewrite. Insecure web forces every entry (not just the first) to
  `127.0.0.1`. This was a latent bug in the single-listener world because
  there was only one entry to rewrite; multi-listener makes it testable
  and `TestExtractWebConfig_InsecureForcesLoopback` pins the behaviour.
- **Telemetry bind order is alphabetical** when extracted from a
  `map[string]any` (post-`ToMap()`), because map iteration order is
  non-deterministic and `internal/core/metrics/` cannot import
  `component/config` to use `*Tree.GetListOrdered`. Alphabetical by YANG
  list key gives a deterministic substitute. Users who care about a
  specific order should name their entries `primary`, `secondary`, etc.
  Web / lg / mcp / api-server all use `*Tree.GetListOrdered` directly and
  preserve config declaration order.
- **gRPC `Serve(ctx)` lost the `addr` argument.** It was latent redundancy
  (the caller passed `cfg.GRPC.Listen()` twice — once to NewGRPCServer,
  once to Serve). The new signature reads from the stored `configured`
  slice, so `Serve(ctx)` takes one argument. Call-site update in
  `cmd/ze/hub/api.go` is the only consumer.
- **Future env-var-per-named-listener syntax is unnecessary.** The spec
  explicitly rejected `ze.<svc>.server.<name>.ip` / `.port` env vars. The
  compound `ze.<svc>.listen=ip:port,ip:port` form already expresses
  multi-bind, and named-per-entry env overrides would multiply the
  surface area without a concrete user need.

## Gotchas

- **British spelling tripped the misspell linter five times.** "synthesise",
  "behaviour". Every occurrence in a doc comment or test name triggered the
  auto-linter hook. Stick to American spelling in code and comments — the
  linter is project policy, not preference.
- **goimports strips "unused" imports during multi-edit flows.** When adding
  a multi-listener test that uses `errors`, `net`, `time`, `http`, `context`:
  adding the imports first (in one Edit) and the function usage second (in a
  separate Edit) causes goimports to strip the imports in between. Add the
  imports AND the usage in a single Edit call — the imports survive. This
  is a specific instance of the "aliased imports" rule in `go-standards.md`.
- **gRPC existing tests bypassed Serve()** with `srv.srv.Serve(ln)` on the
  inner `*grpc.Server` + their own listener. New `NewGRPCServer` validation
  requires `ListenAddrs` non-empty, so every test construction needs a
  placeholder `[]string{"127.0.0.1:0"}` even though the listener is never
  bound through the new path. Six test sites, all updated with a comment
  explaining why the placeholder is there.
- **The `-p` flag in `bin/ze-test bgp parse` is parallelism, not pattern.**
  Tests are selected by numeric code (or literal single characters like `z`
  once the codes overflow into the alphabet). `bgp parse -l` prints the
  full map; pipe it to `grep -E` to find the code for a named test.
- **Untracked spec file on main blocked the cherry-pick of Chunk 1.** The
  spec file existed on main as an untracked copy (I wrote it to main before
  entering the worktree) and the chunk-1 commit tried to add the same file.
  Resolution: `diff -q` to confirm byte-for-byte identical, then `rm` the
  untracked copy on main, then retry. Next time: either commit the spec on
  main first so it enters the worktree as part of HEAD, or create the spec
  inside the worktree from the start.
- **Per-binder goroutine-per-listener is NOT a hot-path per-event goroutine.**
  The goroutine lifecycle rule (`rules/goroutine-lifecycle.md`) forbids
  per-event goroutines in hot paths and requires long-lived workers. The new
  `for _, ln := range listeners { go func(...) ... }()` in each binder is
  per-lifecycle (one goroutine per bound listener, runs for the daemon's
  entire lifetime), not per-event. This is the same pattern SSH already
  uses in `internal/component/ssh/ssh.go` for `extraListeners`.
- **`CollectListeners` enabled-gate lives on the LAST container in the
  path.** For `api-server.rest` the enabled leaf is on `rest`, not on
  `api-server`. The existing walker already handles this correctly because
  it checks `container.Get("enabled")` at the final node in the containers
  slice; I just had to make sure the 3-level path (`environment` ->
  `api-server` -> `rest`) landed in the table with the rest container at
  the end.

## Files

- `internal/component/api/schema/ze-api-conf.yang` -- YANG container -> list
- `internal/component/config/loader_extract.go` -- slice-returning extractors
- `internal/component/config/loader_extract_test.go` -- 14 extractor tests (created)
- `internal/component/config/listener.go` -- api-server entries in knownListenerServices
- `internal/component/config/listener_test.go` -- 3 api-server collection tests
- `internal/core/metrics/server.go` -- TelemetryConfig struct + multi-listener Start
- `internal/core/metrics/server_test.go` -- 2 multi-listener tests
- `internal/component/web/server.go` -- multi-listener WebServer
- `internal/component/web/server_test.go` -- 3 new tests
- `internal/component/lg/server.go` -- multi-listener LGServer
- `internal/component/lg/server_test.go` -- 2 new tests
- `cmd/ze/hub/mcp.go` -- multi-listener startMCPServer
- `cmd/ze/hub/mcp_test.go` -- 3 new tests (created)
- `internal/component/api/rest/server.go` -- multi-listener RESTServer
- `internal/component/api/rest/server_test.go` -- 3 new tests
- `internal/component/api/grpc/server.go` -- multi-listener GRPCServer, Serve(ctx)
- `internal/component/api/grpc/server_test.go` -- 3 new tests + 6 placeholder renames
- `cmd/ze/hub/main.go` -- runYANGConfig rewrite, startWebServer / startLGServer slice signatures, endpointsToAddrs helper
- `cmd/ze/hub/api.go` -- REST + gRPC slice forwarding, serveGRPC(srv) without addr arg
- `internal/component/bgp/config/loader_create.go` -- telemetry call site for TelemetryConfig struct
- `test/parse/web-multi-listener.ci` -- AC-1 (created)
- `test/parse/lg-multi-listener.ci` -- AC-2 (created)
- `test/parse/mcp-multi-listener.ci` -- AC-3 (created)
- `test/parse/telemetry-multi-listener.ci` -- AC-4 (created)
- `test/parse/api-rest-multi-listener.ci` -- AC-5 (created)
- `test/parse/api-grpc-multi-listener.ci` -- AC-6 (created)
- `test/parse/listener-conflict-web-lg.ci` -- AC-11 between different services (created)
- `test/parse/listener-conflict-api.ci` -- AC-11 + AC-16 for api-server (created)
- `test/plugin/rest-api-commands.ci` -- unnamed server -> server main rename
- `docs/guide/configuration.md` -- new Named Listeners subsection under Environment Block
- `docs/features.md` -- new Named Service Listeners row + REST/gRPC row rewrite
- `docs/architecture/config/syntax.md` -- ze:listener extension row rewrite
