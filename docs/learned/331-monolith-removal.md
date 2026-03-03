# 331 — Monolith Removal: Decompose internal/plugin/

## Objective

Break `internal/plugin/` (~31 files, ~18K LOC) into focused sub-packages with clear responsibility boundaries, without any behavior changes.

## Decisions

- Chose 3 sub-packages (ipc/, process/, server/) instead of the planned 8: the connected type graph through `*Server` forces handler/, startup/, reload/, schema/ into server/ — splitting them further creates cycles.
- Extracted `server/` before `process/` to avoid an import cycle: root still holds Server files that import Process; only after Server moves can process/ safely live in a sub-package.
- `rpc/` renamed to `ipc/` to avoid confusion with `pkg/plugin/rpc` (same leaf segment in different trees).
- `registration.go` stayed in root (not moved to startup/) — registration types are shared across root, process, and server.

## Patterns

- Bottom-up extraction: leaves first (no dependents), orchestrators last — prevents import cycles during migration.
- Verify with `go vet ./...` after each commit; cycles show as compilation errors, not linker errors.
- `all_import_test.go` files needed in root and process/ to trigger plugin `init()` registrations in tests.

## Gotchas

- Initial assumption: ~10 external importers. Reality: ~30 files across 6 packages (bgp/handler, bgp/server, bgp/reactor, bgp/format, hub, cmd/ze). Always grep before estimating.
- `gofmt -r` incorrectly qualified struct literal field names (e.g., `plugin.Process:` in composite literals) — hand-fix needed after bulk renames.
- Redundant `plugin` import aliases caused goimports lint failures (3-group import ordering required).
- `ServerConfig.RPCProviders` references a connected type graph (`Handler` → `CommandContext` → `*Server`) — the entire graph must move together.

## Files

- `internal/plugin/ipc/` — socketpair + PluginConn (2 source files)
- `internal/plugin/process/` — Process, ProcessManager, EventDelivery (6 files)
- `internal/plugin/server/` — Server, protocol startup, dispatch, handlers, schema, reload (40 files)
