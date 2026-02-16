# Ze Project Memory

## YANG Module Naming Convention
- Config modules use `-conf` suffix: `ze-bgp-conf`, `ze-hub-conf`, `ze-plugin-conf`
- API modules will use `-api` suffix: `ze-bgp-api` (future)
- Wire format identifier `"format": "ze-bgp"` is separate from module names - does NOT change
- Socket names and binary names are also separate - do NOT change

## YANG Schema Package Pattern
- Each plugin/module has `schema/` subdirectory with `embed.go` + `.yang` file
- `embed.go` uses `//go:embed` to export YANG content as a string variable
- Import alias convention when multiple `schema` packages: `grschema`, `hostnameschema`, `bgpschema`, `hubschema`
- Core modules: `internal/yang/modules/` (ze-types, ze-extensions, ze-plugin-conf)
- Plugin modules: `internal/plugins/<name>/schema/` (ze-bgp-conf, ze-graceful-restart, etc.)
- Hub modules: `internal/hub/schema/` (ze-hub-conf)

## Key Mappings
- `MapPrefixToModule()` in `validator.go`: "bgp" → "ze-bgp-conf", "plugin" → "ze-plugin-conf"
- `FormatNamespace()`: strips "urn:" prefix, replaces ":" with "." (e.g., "urn:ze:bgp:conf" → "ze.bgp.conf")

## Flaky Tests Policy (BLOCKING — ZERO TOLERANCE)
- Flaky tests are NEVER acceptable — they indicate real bugs that will manifest in production
- Do NOT document-and-ignore flaky tests — that hides production bugs
- NEVER use words: "transient", "resource contention", "parallel execution noise", "not related to our changes"
- These phrases are SELF-DECEPTION. A test either passes or it doesn't.

### Mandatory Procedure When ANY Test Fails Then Passes on Retry
1. **STOP** — do not continue with current work
2. **Paste the full failure output** to the user
3. **Read the failing test source code** — understand what it tests
4. **Investigate root cause** — grep for races, check goroutine lifecycles, examine resource cleanup
5. **Report findings** to the user with: test name, failure output, source analysis, root cause theory
6. **User decides** whether to: fix now, create spec, or (only user can say this) accept the risk
7. **I do NOT get to decide** that a failure is "transient" — that word is banned from my vocabulary for test failures

### Resolved flaky tests:
- `TestSubsystemRPCProtocol` + `TestSubsystemShutdown` — FIXED (3d36df9): mock goroutine `defer cancel()` raced with engine's `completeProtocol` stage 5 response write
- `TestProcessWriteEvent`, `TestProcessReadCommand`, `TestProcessSendRequest` — GONE: removed during plugin RPC migration refactoring
- `TestInvokePluginForkPath` — NO LONGER FLAKY: 0/10 failures under full load (stabilized by RPC migration)
- Functional test "refresh" suite — NO LONGER FLAKY: passes consistently (stabilized by RPC migration)

### Known persistent failures (not flaky — real bugs):
- `custom-flowspec-plugin` functional test — FIXED: capability ordering was non-deterministic due to Go map iteration in `parseFamiliesFromTree` (config.go:164). Added `slices.Sort` on family keys before iterating.

## Family Registration Pattern
- Address families are registered DYNAMICALLY by plugins via `PluginRegistry.Register()` — NOT a static list
- `PluginRegistry.LookupFamily()` checks if a family has a registered decode plugin
- CLI `ze bgp decode` uses static `pluginFamilyMap` only because it has no runtime registry
- When validating family strings, check FORMAT (contains "/", non-empty parts) — never enumerate all families
- New families can be added by plugins without code changes

## Buffer-First Encoding (BLOCKING)
- Wire encoding MUST use `WriteTo(buf, off) int` — NEVER `make([]byte, N)` and return
- `Pack() []byte` is fully DELETED — no Pack methods exist on any wire type
- All per-type serialization uses `WriteTo`. Use "Write" (not "Pack") in comments when describing encoding.
- Higher-level assembly functions (`PackTo`, `PackFor`, `PackAttributesFor`) still exist — these orchestrate multiple `WriteTo` calls
- `wire.SessionBuffer` exists for reusable per-session buffers — use it in reactor/peer paths
- See `.claude/rules/buffer-first.md` for full rules and exceptions
- `/find-alloc` audits violations, `/fix-alloc file:line` fixes them

## Bash Timeout Rule
- Default bash timeout: 15000ms (15 seconds) for most commands
- Only use longer for genuinely long operations (`make verify`, `make unit-test`)
- ExaBGP functional tests complete in seconds — 15s is plenty
- Do NOT pass `--timeout` flags to test runners — use their defaults

## Linter Hook Behavior
- `auto_linter.sh` runs on every Edit/Write and may run goimports which removes unused imports
- When adding import + usage in separate edits, the import may be removed before usage is added
- Solution: add import and usage in the same edit, or accept the cascading lint errors as transient

## Config Pipeline (post-BGPConfig removal)
- BGPConfig struct fully eliminated — spec 226 done
- Config flow: file → Tree → ResolveBGPTree() → map[string]any → reactor.PeersFromTree()
- Route extraction: config.PeersFromConfigTree() with 3-layer template inheritance (globs → templates → peer)
- Template resolution shared between ResolveBGPTree (map-level) and PeersFromConfigTree (Tree-level)
- Key files: `config/resolve.go`, `config/peers.go`, `reactor/config.go`
- Route types (StaticRouteConfig etc.) remain in config package — can't move to reactor (import cycle)

## File Splits (reference)
- model.go split into 4: model.go, model_commands.go, model_render.go, model_load.go
- bgp.go split into 4: bgp.go, bgp_peer.go (DELETED), bgp_routes.go, bgp_util.go
