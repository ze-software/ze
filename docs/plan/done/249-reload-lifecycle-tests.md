# Spec: reload-lifecycle-tests (UMBRELLA — COMPLETED)

## Task

Build the VyOS-style config reload pipeline: config change → YANG validate → plugin verify → save → plugin apply(diff). The pipeline is **generic** — it works with `map[string]any` config trees driven by YANG schemas, not BGP-specific types. Each plugin (including the BGP reactor) interprets its own config section independently.

## Sub-Specs (all completed)

| Order | Done | Scope |
|-------|------|-------|
| 1 | `done/222-config-reload-1-rpc.md` | RPC types for config-verify and config-apply |
| 2 | `done/223-config-reload-2-coordinator.md` | Reload coordinator orchestrating verify→apply |
| 3 | `done/230-config-reload-3-sighup.md` | SIGHUP → coordinator + reactor refactor |
| 4 | `done/224-config-reload-4-editor.md` | Editor commit → save + reload |
| 5 | `done/225-config-reload-5-e2e.md` | End-to-end functional tests |
| 6 | `done/226-config-reload-6-remove-bgpconfig.md` | Remove BGPConfig typed intermediate, reactor parses from tree |
| 7 | `done/227-config-reload-7-coordinator-hardening.md` | Crash detection, apply error propagation, daemon reload wiring |
| 8 | `done/228-config-reload-8-daemon-rpcs.md` | Move daemon-* RPCs from ze-bgp to ze-system module |

## Related Specs

- `done/234-reload-peer-add-remove.md` — peer add/remove via reload
- `done/238-hub-orchestrator-reload.md` — hub orchestrator reload integration

## Reference: Architecture Docs

- `docs/architecture/core-design.md` — system components, plugin protocol
- `docs/architecture/config/yang-config-design.md` — VyOS handler interface, Diff struct, commit flow
- `docs/architecture/config/vyos-research.md` — VyOS architecture analysis
- `docs/plan/done/155-hub-phase4-verify-apply.md` — two-phase verify/apply protocol
- `docs/plan/done/185-config-json-delivery.md` — config JSON delivery, WantsConfigRoots, reload format

## Reference: Key Insights

- The pipeline is generic: coordinator works with `map[string]any`, never touches `BGPConfig`
- Plugin SDK (`pkg/plugin/plugin.go`) has verify/apply handlers that are never invoked by the engine
- `DiffMaps()` in `internal/config/diff.go` is ready to use for computing config deltas
- Plugins declare `WantsConfigRoots` — coordinator only sends verify/apply to relevant plugins
- Reactor is called directly by coordinator (not via RPC) since it IS the engine

## Reference: Data Flow

### Entry Points
- **SIGHUP:** OS signal received by SignalHandler, triggers ReloadFromDisk()
- **Editor commit:** cmdCommit() in model_commands.go, triggers coordinator after save

### Transformation Path
1. Parse new config file → `map[string]any` tree (generic, YANG-driven)
2. YANG validate the new tree
3. `DiffMaps(running, new)` → `ConfigDiff{Added, Removed, Changed}`
4. If diff empty → no-op return
5. Build per-root sections from diff for each plugin's `WantsConfigRoots`
6. **Verify phase:** Send `config-verify` RPC to each affected plugin + call reactor verify
7. If ANY verify fails → abort, return error, keep running config
8. **Apply phase:** Send `config-apply` RPC with diff to each plugin + call reactor apply
9. Reactor apply: convert `map[string]any` → peer settings, diff peers, add/remove
10. Update running config reference

### Boundaries Crossed
| Boundary | How |
|----------|-----|
| Engine ↔ Plugin | `config-verify` / `config-apply` RPCs via Socket B |
| Coordinator ↔ Reactor | Direct function call (VerifyConfig, ApplyConfigDiff) |
| Signal ↔ Coordinator | OnReload callback → ReloadFromDisk() |
| Editor ↔ Engine | Save to disk + reload command |

### Integration Points
- `deliverConfigRPC()` in server.go — pattern for sending per-root config to plugins
- `extractConfigSubtree()` in server.go — extracts per-root sections from tree
- `DiffMaps()` in diff.go — computes config deltas
- `GetConfigTree()` on reactor — returns running config as `map[string]any`
- `WantsConfigRoots` in DeclareRegistrationInput — plugins declare interest

## Reference: Files Modified/Created

- `pkg/plugin/rpc/types.go` — config-verify/apply RPC types
- `internal/plugin/rpc_plugin.go` — SendConfigVerify/SendConfigApply
- `pkg/plugin/sdk/sdk.go` — config-verify/apply dispatch + OnConfig handlers
- `internal/plugin/server.go` — coordinator wiring
- `internal/plugins/bgp/reactor/reactor.go` — VerifyConfig/ApplyConfigDiff (replaced Reload())
- `internal/config/editor/model_commands.go` — commit triggers reload
- `internal/plugin/reload.go` — reload coordinator (created)
- `internal/plugin/reload_test.go` — coordinator tests (created)

## Reference: Design Decisions

- No premature abstraction (generic pipeline, not BGP-specific)
- No speculative features (only what's needed for reload)
- Single responsibility (coordinator orchestrates, plugins verify/apply independently)
- Explicit behavior (verify must pass before apply, clear error messages)
- Minimal coupling (coordinator uses map[string]any, plugins interpret own sections)
- Follows existing RPC/SDK patterns exactly
