# The plugin infrastructure door

<!-- door-base: internal/component/plugin pkg/plugin pkg/ze -->

Read `ai/doors/system.md` first. It already states the process boundary and
the public `pkg/` API. This door does not repeat them. It covers
`internal/component/plugin` (`registry`, `server`, `process`, `manager`, `all`,
`cli`), `pkg/plugin/sdk`, `pkg/plugin/rpc`, `pkg/ze`, the process protocol, and
the guide in `docs/plugin-development/`.

## Where it sits

| Package | Role | Page |
|---------|------|------|
| `plugin/registry` | The `Registration` each internal plugin hands `registry.Register` from `init()`. A leaf with no plugin dependencies | `docs/architecture/plugin/plugin-system.md`, "Where each layer lives" |
| `plugin/all` | The generated composition root: blank imports only | `plugin-system.md` |
| `plugin/process` | Starts a plugin internal (`startInternal`) or external (`startExternal`), and runs per-process event delivery | `plugin-system.md`, "The process boundary" |
| `plugin/server` | The startup coordinator, the engine-op registry, event dispatch, config delivery and reload | `ai/digests/plugin-transport.md` |
| `plugin/manager` | Implements `ze.PluginManager` | `docs/architecture/plugin/component-boundaries.md` |
| `pkg/plugin/sdk`, `pkg/plugin/rpc`, `pkg/ze` | The public SDK, the wire types with `MuxConn` and `DirectBridge`, the engine interfaces | `plugin-system.md`, `component-boundaries.md` |

It is infrastructure and cannot be compiled out. Outside the generated `all/`
files, no package under `internal/component/plugin/` imports
`internal/component/bgp/` (`component-boundaries.md`, "What this buys").

## Who depends on it from outside

| Outside party | Contract | Page |
|---------------|----------|------|
| A plugin in any language | The line format `#<id> <verb> [<json>]`, the method names in `internal/core/ipc/yang/ze-plugin-engine.yang` and `ze-plugin-callback.yang`, the five stages, and the JSON events | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md` |
| A forked plugin | TLS connect-back: `ZE_PLUGIN_HUB_HOST`, `ZE_PLUGIN_HUB_PORT`, `ZE_PLUGIN_HUB_TOKEN`, `ZE_PLUGIN_CA_PEM`, `ZE_PLUGIN_NAME`, then `#0 auth`. It never gets `DirectBridge` | `process-protocol.md`, "Mode 2: Subprocess (TLS connect-back)" |
| A plugin binary ze does not carry | Query mode: started with `ZE_PLUGIN_MODE=declare`, it writes its Stage 1 line to stdout and exits 0 | `process-protocol.md`, "Query Mode", and `ai/rules/plugins.md` |
| A Go plugin outside this repository | `sdk.NewFromEnv`, `sdk.NewFromTLSEnv`, `sdk.NewWithConn`, `sdk.Registration` (an alias of `rpc.DeclareRegistrationInput`), every `On*` callback, `sdk.RunOrDeclare`, and the `pkg/ze` interfaces | `docs/plugin-development/README.md`, `component-boundaries.md` |
| An operator | `plugin { external <name> { run ...; } }` (`internal/component/plugin/yang/ze-plugin-conf.yang`) | `docs/guide/plugins.md` |

A field or method name in `rpc` is wire, not Go: renaming a JSON tag breaks
every plugin written in another language. An external plugin's own YANG is not
compiled yet (`docs/plugin-development/schema.md`, "What works today").

## Boundaries

- **The engine never enumerates plugins.** Families, events, send types,
  commands and claims all arrive by declaration (`DESIGN-HISTORY.md`, Plugin
  system, the row `Plugin declares what it handles; engine never enumerates`).
- **One engine-op entry per RPC.** The JSON path, the direct path and the typed
  bridge setter all derive from its `engineOps` entry
  (`plugin/server/dispatch_registry.go`, and `ai/digests/plugin-transport.md`). Never
  add a second path for one op.
- **The SDK stays generic.** A new callback is one `On*` method in
  `sdk_callbacks.go`. The event loops do not change (`plugin-system.md`, "The SDK
  stays generic").
- **Every feature works over both transports.** `DirectBridge` is an in-process
  shortcut of the same protocol, and a forked plugin always takes the RPC path.
- **A role one plugin takes from another is a claim, not an import.** Declare
  `Claims`. The other plugin reads `ClaimActive` in `OnConfigure`
  (`plugin-system.md`, "Exclusive role claims").
- **No plugin imports another plugin's implementation.** `./le arch tier check`
  enforces ownership (`docs/architecture/module-tiers.md`, "Plugin import ownership").

## Read before code

| Read | The question it answers |
|------|-------------------------|
| `docs/architecture/plugin/plugin-system.md` | Registration fields, startup phases and callbacks, claims, the process boundary, communication patterns |
| `docs/architecture/api/process-protocol.md` | Wire format, the five stages, query mode, tier-ordered startup, shutdown, runtime RPCs, event delivery |
| `docs/architecture/plugin/component-boundaries.md` | The `pkg/ze` interfaces and why they are public |
| `docs/architecture/api/ipc_protocol.md` | How answer lines are framed |
| `docs/plugin-development/` | What an outside author is told: `README.md`, `protocol.md`, `handlers.md`, `commands.md`, `schema.md`, `testing.md` |
| `docs/architecture/plugin/feature-gates.md` | How a plugin is compiled out |
| `ai/digests/plugin-transport.md` | Event down, RPC up, the bridge wiring order |
| `ai/digests/api-ipc.md` | The IPC and `MuxConn` transport |
| `ai/patterns/plugin.md`, `ai/patterns/registration.md` | The structure of a new plugin and of a registry |
| `ai/rules/plugins.md` | Ownership, YANG for every RPC, declare-mode purity, value types |
| `ai/rules/goroutine-lifecycle.md` | Workers a plugin starts |

## Traps

- **An engine call during startup waits at the barrier.** A plugin that calls
  the engine, or waits for another plugin, from its config stage stalls or is
  refused. Source: `plan/journal/plugin-startup-barrier-deadlock.md`, 2026-08-10
  and 2026-09-06 rows. State RPCs are refused until the handshake ends
  (`process-protocol.md`, "Persistent runtime state").
- **A cross-plugin command from `OnStarted` fails with "unknown command".** Use
  `OnAllPluginsReady`, but never resolve a role there (`plugin-system.md`,
  "Startup phases and callbacks", "Exclusive role claims").
- **One plugin failure stops the startup tier, by owner ruling.** Do not propose
  making it survivable, and make every server error path call `PluginFailed`
  and `proc.Stop()`. Source: `DESIGN-HISTORY.md`, Plugin system, load-bearing
  invariants.
- **A command declared to one reader only.** `Registration.Commands` and the
  `sdk.Registration` a runner passes to `p.Run` must agree, or the catalog and
  the daemon differ (`plugin-system.md`, "The process boundary").
- **SDK env keys read where the SDK is not linked end the process.** Source:
  `plan/journal/env-key-read-but-never-registered.md`, 2026-09-04 and 2026-09-22 rows.
- **A name off the wire is looked up, never registered.** Registering
  wire-supplied names would fill a process-global registry. A comment that said
  otherwise survived beside the code. Source:
  `plan/journal/comment-describes-superseded-behaviour.md`, 2026-09-08 row.
- **Know which mode a plugin runs in before you describe it.** A page said IKE
  ran as a subprocess when it runs in the daemon through `DirectBridge`. Source:
  `comment-describes-superseded-behaviour.md`, 2026-09-10 row.
- **A request line is framed by its newline,** so a payload must be compact
  `json.Marshal` output. Source: `DESIGN-HISTORY.md`, Plugin system, invariant 397.
- **A test that mutates the registry** takes `registry.Snapshot()` and restores
  it in `t.Cleanup`. Source: `RECURRING-PATTERNS.md`, "Package-level registry
  contamination across tests".
- **`DESIGN-HISTORY.md` "Plugin startup needs TWO socket pairs" is history.** The
  protocol runs over one bidirectional connection (`process-protocol.md`,
  "Overview". `Registration.RunEngine` takes one `net.Conn`).

## Gates

| Command | What it judges |
|---------|----------------|
| `./le plugin boundary check` | Known in-process calls that break when the plugin runs forked (a heuristic) |
| `./le plugin declarations check` | Stage 1 commands match the registration |
| `./le plugin imports check` | The generated composition root is current |
| `./le arch tier check` | Placement and plugin import ownership |
| `./le doc yang-contract command-contract` | Every YANG command has a handler |
| `./le mock plugin-external <name>` | Runs a registered engine through real TLS connect-back (`docs/architecture/testing/ci-format.md`) |
| `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...` | The scoped package tests |
| `./le doc check verify` | Source anchors in any page you edited |
