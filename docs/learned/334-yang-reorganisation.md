# 334 — YANG Schema Reorganisation

## Objective

Reorganise YANG schemas so each module lives with the package that owns it, replace scattered manual `AddModuleFromText()` calls with init()-based registration, and move config/yang packages into the component layer.

## Decisions

- Used YANG `augment` (not `schema.Define()`) for splitting environment containers: `schema.Define()` replaces the existing container, augment extends it — they are not interchangeable.
- `ze-hub-conf.yang` stays in `internal/component/hub/schema/` (moved from yang/modules/) — it's a domain definition owned by hub, not a YANG library file.
- `LoadEmbedded()` reduced to ze-extensions.yang and ze-types.yang only — true YANG library files that are bootstrap dependencies for all other modules.
- `yang/registry/` sub-package was created then merged back into `yang` package — the import cycle it was meant to break did not actually exist.
- IPC protocol schemas (ze-plugin-callback.yang, ze-plugin-engine.yang) moved to `internal/ipc/schema/` — they define the plugin protocol, not BGP or hub.
- Phase 5 (dead environment field cleanup) deferred to `docs/plan/spec-config-bgp-separation.md` — fields are BGP-specific, better handled during BGP config separation.

## Patterns

- goyang `Parse()` is order-independent; only `Resolve()` needs all modules loaded — init()-registered modules can be collected in any order before resolving.
- Init-based registration (same pattern as `internal/component/plugin/registry/`) scales to any number of YANG modules without touching the loader.
- Each schema package gets a `register.go` file with an `init()` call — exempt from `block-init-register.sh` hook.
- `all_import_test.go` files needed in test packages to trigger `init()` registrations (blank imports don't fire in test binaries unless something in the test package chain imports them).

## Gotchas

- `GetAllInternalPluginYANG()` was removed from config loading path — it caused duplicate module errors when modules were registered via init() AND loaded manually.
- Plugin-conf imports hub-conf (augments it), so hub-conf must be part of the embedded/bootstrap set, not just a registered module loaded in any order.
- 10 of 14 environment fields are dead code (never consumed at runtime) — audit before assuming config fields are used.

## Files

- `internal/component/bgp/schema/` — ze-bgp-conf.yang, ze-bgp-api.yang, embed.go, register.go (moved from plugins/bgp/schema/)
- `internal/component/plugin/schema/` — ze-plugin-conf.yang, embed.go, register.go (moved from yang/modules/)
- `internal/component/hub/schema/` — ze-hub-conf.yang, embed.go, register.go
- `internal/ipc/schema/` — ze-plugin-callback.yang, ze-plugin-engine.yang, register.go
- `internal/component/config/` — config/ and yang/ merged here (31 + 12 importers updated)
- `internal/component/config/yang/` — former internal/yang/
