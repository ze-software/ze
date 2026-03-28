# 448 — Handler Reorg: BGP Commands into Self-Contained Plugin Folders

## Objective

Move 28 BGP command handler RPCs from the monolithic `internal/component/bgp/handler/` into three self-contained plugin folders under `bgp/plugins/`, each with its own YANG schema, tests, and helpers. Deleting a folder must not break anything.

## Decisions

- **Three plugins by concern:** bgp-cmd-peer (21 RPCs: peer lifecycle, introspection, subscriptions), bgp-cmd-ops (6 RPCs: cache, commit, raw, refresh), bgp-cmd-update (2 RPCs: text/wire UPDATE parsing).
- **Not in `all/all.go`:** These packages use `pluginserver.RegisterRPCs()` (engine-side), not `registry.Register()` (SDK). Including them in `all/all.go` creates an import cycle: `process_test → all → bgp-cmd-ops → plugin/server → plugin/process`. Blank-imported from `reactor.go` and `cli/main.go` instead.
- **`doc.go` not `register.go`:** Renamed to prevent `gen-plugin-imports.go` from auto-discovering them into `all/all.go`. Only SDK plugins belong there.
- **`requireBGPReactor()` copied, not shared:** 33-line helper copied to each plugin's `require.go`. Self-containment > DRY for trivial code.
- **Rib meta-commands (help, command list, event list) moved to bgp-rib plugin** in the same effort, since they were the only handler/ RPCs that belonged to an SDK plugin.

## Patterns

- **Two registration mechanisms:** `registry.Register()` for SDK plugins (go in `all/all.go`, discovered by `gen-plugin-imports.go`). `pluginserver.RegisterRPCs()` for engine-side command handlers (blank-imported from entrypoints). Never mix them in the same auto-discovery path.
- **Import cycle prevention:** `all/all.go` can only contain imports whose transitive closure doesn't include `plugin/process` or `plugin/server`. SDK plugins import only `plugin/registry` (leaf package), so they're safe. Engine-side handlers import `plugin/server`, so they're not.
- **YANG schema wiring:** Each plugin's `doc.go` blank-imports its own `schema/` subpackage. The schema `register.go` calls `yang.RegisterModule()` in `init()`.
- **Test RPC counts are package-scoped:** `AllBuiltinRPCs()` returns only RPCs from packages imported by the test binary. The peer test sees 21, not 28. Integration coverage comes from functional tests that import everything via `all/all.go`.

## Gotchas

- **Import cycle was not predicted by the plan.** The plan assumed auto-discovery would replace manual blank imports. Reality: `plugin/server` → `plugin/process` chain means engine-side handlers can't go through `all/all.go`.
- **Rib command tree tests broke.** Moving rib meta-commands from handler/ to bgp-rib meant "rib" disappeared from `AllBuiltinRPCs()`. Tests in `cli/` and `run/` that expected "rib" as a top-level builtin command needed updating.
- **Protocol test count drift.** The bgp-rib protocol test expected 14 commands but the plugin now registers 17 (3 meta-commands added). Count assertions in protocol tests must be updated when commands are added.

## Files

- Created: `bgp/plugins/bgp-cmd-peer/` (14 files), `bgp-cmd-ops/` (15 files), `bgp-cmd-update/` (13 files)
- Deleted: `bgp/handler/` (16 files)
- Modified: `reactor/reactor.go`, `cmd/ze/cli/main.go` (blank imports), `plugin/server/*.go` (stale refs), `bgp-rib/protocol_test.go` (count), `cli/main_test.go`, `run/main_test.go` (remove rib expectations)
