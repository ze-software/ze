# Memory Rationale

Why: `.claude/rules/memory.md`

## YANG Schema Package Pattern
- Each plugin/module has `schema/` subdirectory with `embed.go` + `.yang` file
- `embed.go` uses `//go:embed` to export YANG content as a string variable
- Import alias convention when multiple `schema` packages: `grschema`, `hostnameschema`, `bgpschema`
- Core modules: `internal/component/config/yang/modules/` (ze-types, ze-extensions, ze-hub-conf)
- Plugin infra schema: `internal/component/plugin/schema/` (ze-plugin-conf)
- BGP component schema: `internal/component/bgp/schema/` (ze-bgp-conf, ze-bgp-api)
- Plugin modules: `internal/plugins/<name>/schema/` (ze-graceful-restart, etc.)

## Key Mappings
- `MapPrefixToModule()` in `validator.go`: "bgp" -> "ze-bgp-conf", "plugin" -> "ze-plugin-conf"
- `FormatNamespace()`: strips "urn:" prefix, replaces ":" with "." (e.g., "urn:ze:bgp:conf" -> "ze.bgp.conf")

## Flaky Tests: Mandatory 7-Step Procedure
When ANY test fails then passes on retry:
1. STOP -- do not continue with current work
2. Paste the full failure output to the user
3. Read the failing test source code -- understand what it tests
4. Investigate root cause -- grep for races, check goroutine lifecycles, examine resource cleanup
5. Report findings to the user with: test name, failure output, source analysis, root cause theory
6. User decides whether to: fix now, create spec, or accept the risk
7. I do NOT get to decide that a failure is "transient" -- that word is banned

## Resolved Flaky Tests History
- `TestSubsystemRPCProtocol` + `TestSubsystemShutdown` -- FIXED (3d36df9): mock goroutine `defer cancel()` raced with engine's `completeProtocol` stage 5 response write
- `TestProcessWriteEvent`, `TestProcessReadCommand`, `TestProcessSendRequest` -- GONE: removed during plugin RPC migration refactoring
- `TestInvokePluginForkPath` -- NO LONGER FLAKY: 0/10 failures under full load (stabilized by RPC migration)
- Functional test "refresh" suite -- NO LONGER FLAKY: passes consistently (stabilized by RPC migration)
- `custom-flowspec-plugin` functional test -- FIXED: capability ordering was non-deterministic due to Go map iteration in `parseFamiliesFromTree` (config.go:164). Added `slices.Sort` on family keys before iterating.

## Config Pipeline (post-BGPConfig removal)
- BGPConfig struct fully eliminated -- spec 226 done
- Config flow: file -> Tree -> ResolveBGPTree() -> map[string]any -> reactor.PeersFromTree()
- Route extraction: config.PeersFromConfigTree() with 3-layer template inheritance (globs -> templates -> peer)
- Template resolution shared between ResolveBGPTree (map-level) and PeersFromConfigTree (Tree-level)
- Key files: `internal/component/bgp/config/resolve.go`, `internal/component/bgp/config/peers.go`, `internal/component/bgp/reactor/config.go`
- Route types (StaticRouteConfig etc.) remain in config package -- can't move to reactor (import cycle)

## File Splits (reference)
- model.go split into 4: model.go, model_commands.go, model_render.go, model_load.go
- bgp.go split into 4: bgp.go, bgp_peer.go (DELETED), bgp_routes.go, bgp_util.go

## Buffer-First Details
- `Pack() []byte` is fully DELETED -- no Pack methods exist on any wire type
- Higher-level assembly functions (`PackTo`, `PackFor`, `PackAttributesFor`) still exist -- these orchestrate multiple `WriteTo` calls
- `wire.SessionBuffer` exists for reusable per-session buffers -- use it in reactor/peer paths
- `/ze-find-alloc` audits violations, `/ze-fix-alloc file:line` fixes them
