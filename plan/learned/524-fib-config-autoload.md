# 524 -- Config-Driven Plugin Auto-Loading

## Context

The FIB pipeline (spec fib-0-umbrella) introduced three new plugins: sysrib, fib-kernel, fib-p4. Users had to manually declare them in `plugin { internal sysrib { } internal fib-kernel { } }`, exposing internal infrastructure names. The user's intent is "I want route installation" expressed as `fib { kernel { } }` in config -- not "start these three internal plugins in dependency order."

The same problem exists for all components: BGP, interface, DNS. Nothing should run unless the user's config asks for it. Config presence is the universal trigger.

## Decisions

- Chose convention-based auto-loading (config container present = start matching plugin) over a `ze:load` YANG extension. The YANG is already loaded at compile time via `init()` blank imports. Adding an annotation for something the system can infer from config presence is redundant. Considered and rejected `ze:load` because it required a new extension, a tree walker, and per-node annotation for something that `ConfigRoots` already solves.
- Chose `ConfigRoots` with dot-separated paths (e.g., `"fib.kernel"`) over top-level root names only. `fib { kernel { } }` and `fib { p4 { } }` are different plugins sharing a parent container -- root-only matching can't distinguish them.
- Moved sysrib admin-distance config under `fib { admin-distance { } }` (owned by ze-fib-conf.yang) instead of keeping a separate sysrib YANG container. sysrib is internal infrastructure loaded via Dependencies, not user-facing. Users configure admin distance where they think about it: under `fib`. Note: the admin-distance YANG block has defaults but no plugin currently reads them at runtime -- sysrib gets priority from Bus events. The YANG serves as documentation and validation of allowed values.
- fib-p4 augments the `fib` container from ze-fib-conf.yang rather than defining its own top-level container. This means `fib { }` is a shared namespace for all FIB backends.
- Auto-stop on config removal: when a config section is removed and committed, the matching plugin is stopped and removed from the ProcessManager. Symmetry with auto-load. Explicitly configured plugins are never auto-stopped. Stop runs before load in the reload path to avoid start-then-immediately-stop races.
- Orphaned dependencies are stopped transitively: if fib-kernel is stopped and sysrib has no other dependents, sysrib is stopped too. The cleanup loops until no new orphans are found, handling dependency chains of any depth.

## Consequences

- Users write `fib { kernel { } }` and routes appear in the kernel. No plugin boilerplate.
- The `ConfigRoots` field on `Registration` is now load-bearing for the FIB pipeline. Any plugin can use the same mechanism by setting `ConfigRoots` and having a YANG container at that path.
- `CollectContainerPaths(tree)` walks the full config tree recursively, producing dot-separated paths. This runs once at startup and once per reload -- not a hot path.
- Config reload auto-loads new plugins AND auto-stops removed ones. The full lifecycle is config-driven. Stopped processes are removed from the ProcessManager via `RemoveProcess()` to prevent stale entries from causing reload verify failures.
- BGP and iface are still always-started (hardcoded in hub/main.go). Making them conditional uses the same `ConfigRoots` mechanism but requires refactoring the startup sequence. Follow-up work, same pattern.
- The startup is now five-phase: (1) explicit, (2) config paths, (3) families, (4) event types, (5) send types.
- `s.coordinator` on the Server struct is now protected by `coordinatorMu` to prevent a data race between ad-hoc SSH plugin sessions and the startup/reload paths.

## Gotchas

- `diff.added` in the reload path is `map[string]any`, not `[]string`. The keys are root names, values are the config data. Extract keys before passing to the auto-load function.
- `ConfiguredPaths` not `ConfiguredRoots` -- the name matters. These are dot-separated paths (`fib.kernel`), not just top-level roots (`fib`).
- Dotted diff keys (e.g., `"fib.kernel"`) are NOT flat keys in the nested config map. `navigateNestedMap()` splits by `"."` and descends. Using the dotted string as a direct map lookup returns nil.
- `collectContainerMapPaths` must only descend into `map[string]any` children (containers), not leaf values. The startup path (`CollectContainerPaths`) walks typed Tree containers; the reload path walks `map[string]any` from `ToMap()`. Filtering to map children only ensures consistent path sets.
- `parentRemoved` checks dot boundaries to avoid false substring matches. Removing `"fib"` matches `"fib.kernel"` but not `"fibula"`.
- `AddAPIProcessCount` must be compensated (decremented) if `runPluginPhase` fails, or the reactor's "all plugins ready" signal blocks forever.
- `ResolveDependencies` failure should abort auto-load, not fall back to loading without dependencies. Loading a plugin without its declared dependencies causes broken state.
- Explicitly configured plugins must be excluded from both auto-stop and orphan cleanup. The auto-load path already skips them; the stop path must mirror this guard.
- `s.coordinator` was a plain struct field accessed from multiple goroutines (startup, ad-hoc SSH sessions, reload). Added `coordinatorMu` mutex. The `stageTransition` function copies to a local variable under the lock to avoid holding the mutex during barrier waits.

## Files

- `internal/component/config/tree.go` -- `CollectContainerPaths` recursive path walker
- `internal/component/plugin/server/startup.go` -- Phase 2 config-path auto-loading, coordinator mutex
- `internal/component/plugin/server/startup_autoload.go` -- `getConfigPathPlugins`, `autoLoadForNewConfigPaths`, `autoStopForRemovedConfigPaths`, `stopOrphanedDependencies`, `navigateNestedMap`, `collectContainerMapPaths`
- `internal/component/plugin/server/reload.go` -- stop-before-load hooks in `reloadConfig`
- `internal/component/plugin/server/adhoc.go` -- coordinator mutex for ad-hoc sessions
- `internal/component/plugin/server/server.go` -- `coordinatorMu` field
- `internal/component/plugin/server/config.go` -- `ConfiguredPaths` field
- `internal/component/plugin/process/manager.go` -- `RemoveProcess` method
- `internal/component/bgp/reactor/reactor.go` -- `ConfiguredPaths` pass-through
- `internal/component/bgp/config/loader_create.go` -- populate `ConfiguredPaths`
- `internal/plugins/fibkernel/register.go` -- `ConfigRoots: ["fib.kernel"]`, `Dependencies: ["sysrib"]`
- `internal/plugins/fibp4/register.go` -- `ConfigRoots: ["fib.p4"]`, `Dependencies: ["sysrib"]`
- `internal/plugins/fibkernel/schema/ze-fib-conf.yang` -- restructured: `fib { admin-distance { } kernel { } }`
- `internal/plugins/fibp4/schema/ze-fib-p4-conf.yang` -- augments `fib` with `p4 { }`
- `internal/plugins/sysrib/schema/ze-sysrib-conf.yang` -- emptied (internal, no user-facing containers)
