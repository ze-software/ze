# The hub and engine door: daemon startup and orchestration

<!-- door-base: cmd/ze internal/component/engine internal/component/hub internal/component/managed pkg/fleet pkg/ze -->

Read `ai/doors/system.md` first. This door covers:

- how the `ze` daemon boots, wires its parts, reloads and stops
- how a build flavor is chosen
- the managed (fleet) client and server

The step-by-step flow is in `ai/digests/hub-engine.md`. This page does not
repeat it.

## Where it sits

- **`cmd/ze` is one program with several personalities.** `ze` and `le` share
  one command engine. A normal `ze` build imports no `internal/le` package, and
  only the non-default `ze_le` file `cmd/ze/ze_le_register.go` links it
  (`docs/architecture/core-design.md`, "Command engine and composition roots").
- **One config, one parser, one runtime.** `ze start` reaches `cmd/ze/hub`
  `run`, which calls `runYANGConfig` for every config the schema accepts. The
  separate orchestrator in `internal/component/hub` was deleted on 2026-08-12.
  That directory now holds only the `ze-hub-conf.yang` schema
  (`docs/architecture/hub-architecture.md`, status header).
- **BGP is a subsystem, but it does not boot through `RegisterSubsystem`.**
  `engine.Engine` `RegisterSubsystem` takes L2TP and PPPoE today
  (`cmd/ze/hub/register_l2tp.go` `registerBNGSubsystems`). The BGP subsystem
  loads through the plugin server and attaches to `plugin.Coordinator` with
  `SetReactor` and `OnPostStartup` (`internal/component/bgp/plugin/register.go`).
  That is the wiring, not the design: BGP and its plugins stay one subsystem
  that `ze_bgp` compiles out.
- **The plugin server is the EventBus and does the forking.** `eng.Start` runs
  before any plugin process exists. The five startup phases run later, in
  `apiServer.StartWithContext` (`cmd/ze/hub/main.go`).
- **Managed configuration has two sides in this tree.** The client is
  `internal/component/managed` `RunManagedClient`. The hub side is
  `startManagedServer` (`cmd/ze/hub/managed_server.go`) over
  `pluginserver.NewManagedServer`. `pkg/fleet` holds the envelope types, the
  version hash and the RPC verbs both sides use.
- **Can it be compiled out?** `cmd/ze/hub` is always on. What it starts is not:
  an optional service registers a factory in `cmd/ze/hub/service_registry.go`
  from a `//go:build ze_<feature>` file, and `feature-gates.txt` declares every
  gated package (`docs/architecture/module-tiers.md`, "Compile-out").

## Who depends on it from outside

| Outside party | Contract | What breaks if it changes |
|---------------|----------|---------------------------|
| An operator | `ze start`, the signals (SIGHUP reloads, SIGTERM stops), the ready file, and the promise that what `ze config validate` accepts is what boots (`docs/architecture/behavior/signals.md`, `test/plugin/config-validate-agrees-with-boot.ci`) | A config that validates and then fails at start. A reload that silently applies nothing |
| A third-party plugin | The startup order and the five phases, the post-startup callback, and the `pkg/ze` interfaces `Engine`, `Subsystem`, `EventBus`, `ConfigProvider`, `PluginManager` (`docs/architecture/plugin/component-boundaries.md`, `docs/architecture/plugin-manager-wiring.md`) | Code built against `pkg/ze` stops compiling, or a plugin waits for a stage that never comes |
| A managed edge and its hub | The `pkg/fleet` envelopes and verbs, config fetch, `config-changed` push, heartbeat (`docs/architecture/fleet-config.md`, `docs/guide/fleet-config.md`) | An edge that cannot fetch its config, or a push it ignores |
| Whoever packages Ze | The build flavors: `distro` (`ze_core ze_distro`, `bin/ze`), `appliance` (`ze_core ze_appliance`), `setup` (`ze_setup` alone, its own dispatch in `setup_dispatch.go`), `host` (`ze_core ze_setup`) and the installer (`internal/le/repo/compiles/matrix.go` `buildMatrix`) | A flavor that no longer builds, or a gated feature linked into a build that must not carry it |

## Boundaries

- **No always-on file imports a gated package.** Reach a gated service through
  the construction registry in `cmd/ze/hub/service_registry.go` or a blank
  import in the generated `internal/component/plugin/all`. Never import it from
  `cmd/ze/hub` directly (`feature-gates.txt` header).
- **The hub and the Engine carry no BGP knowledge.** BGP reads its boot inputs
  back from `registry.BGPBootstrap`, set by `coordinator.SetBootstrap`, and
  reaches the Coordinator through `registry.CoordinatorAccessor`
  (`ai/digests/hub-engine.md`, steps 2 and 7). A new protocol uses the same
  seams. It does not add a BGP-style special case to `main.go`.
- **`ze` never links `internal/le`.** `TestNormalZeLinksNoInternalLe`
  (`cmd/ze/ze_le_personality_test.go`) measures it over every shipped flavor.
- **One reload path.** SIGHUP and a managed push both reach `doReload`
  (`cmd/ze/hub/main_reload.go`). A new trigger joins it rather than building a
  second path.
- **`Coordinator.postStartup` holds one closure** (`internal/component/plugin/coordinator.go`
  `OnPostStartup`). A second caller replaces BGP's deferred peer start.

## Read before code

| Read | It answers |
|------|------------|
| `ai/digests/hub-engine.md` | The boot, reload, managed and shutdown flow, step by step, with the stale pages it supersedes |
| `ai/digests/plugin-transport.md` | What happens inside one plugin's five-stage handshake and on the bus |
| `docs/architecture/plugin/component-boundaries.md` | Why the interfaces live in `pkg/ze`, and what a subsystem is |
| `docs/architecture/plugin-manager-wiring.md` | Why the process spawn and the protocol handshake are two phases |
| `docs/architecture/fleet-config.md` | The managed client and hub protocol, and the blob layout the hub reads |
| `docs/architecture/core-design.md`, "Command engine and composition roots" and section 18 | The `ze`/`le` split, and the config transaction a reload drives |
| `docs/architecture/module-tiers.md` and `docs/architecture/plugin/feature-gates.md` | How a feature compiles out, and what `feature-gates.txt` drives |
| `docs/architecture/hub-architecture.md`, `docs/architecture/subsystem-wiring.md` | The plugin model only. Both say in their header that the process model they draw is not the live code |
| `ai/patterns/registration.md` | How a new service or subsystem registers instead of being named in `main.go` |
| `ai/rules/architecture.md`, `ai/rules/plugins.md` | Where a new package goes, and the compile-out obligations |

## Traps

- **A hook wired late is missing for a fast plugin.** The shutdown fallback was
  set near the end of startup, but a plugin can dispatch as soon as it
  handshakes, so `request shutdown` found no handler
  (`plan/journal/session-up-before-its-subscriber.md`, 2026-09-07).
- **The ready file is not a barrier for post-startup work.**
  `sendPostStartupToAll` does not wait, so a test that treated `daemon.ready` as
  "every plugin was told" raced its own stop
  (`plan/journal/false-synchronization-claim.md`, 2026-08-12).
- **Something built only at startup ignores every reload.** `swapAAABundle`
  had only boot-time callers, so a changed RADIUS or TACACS+ server was accepted
  and applied to nothing (`plan/journal/invariant-enforced-by-an-absent-call-site.md`,
  2026-08-23). The opposite also happened: a reload rebuilt and swapped a live
  bundle (`plan/journal/component-rebuilt-during-reload.md`, 2026-08-24).
- **A check that runs only at start passes validate.** `l2tp.ExtractParameters`
  is called only by `registerBNGSubsystems`, so its relay and dial-source checks
  refuse at start, never at `ze config validate`
  (`plan/journal/validate-accepts-what-start-refuses.md`, 2026-10-17).
- **An untagged test does not see BGP.** The BGP schema and command plugins
  link only through `all_ze_bgp.go`, so untagged tests fail with "unknown path
  element: bgp" (`plan/journal/feature-test-missing-build-tag.md`, 2026-10-18).
- **One plugin failure aborts the whole startup tier, by design**, and an
  internal plugin can deadlock the five-stage startup when both sides wait
  (`plan/learned/DESIGN-HISTORY.md`, "Plugin system: architecture",
  load-bearing invariants).
- **A new plugin breaks two hand-kept lists**: `TestAvailablePlugins`
  (`cmd/ze/main_test.go`) and `TestAllPluginsRegistered`
  (`internal/component/plugin/all/all_test.go`)
  (`plan/learned/RECURRING-PATTERNS.md`, "Plugin list hardcoded in two test files").

## Gates

| Command | It judges |
|---------|-----------|
| `./le repo compiles check` | The tree git holds compiles in every flavor of `buildMatrix` |
| `./le repo feature-tags check` | The tag lists derived from `feature-gates.txt` are current |
| `./le arch tier check` | Tier placement, and no always-on import of a gated package |
| `./le plugin imports check` | The generated composition root is current |
| `./le test functional reload`, `./le test managed`, `./le test functional plugin` | Reload, managed config, and plugin startup through the real daemon |
| `./le job run label <label> command go test -tags <tags> <package>` | The scoped package test. Add `ze_bgp` when the test needs the BGP schema (`docs/contributing/running-commands.md`) |
