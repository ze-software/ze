# The system door: what every change in Ze must respect

<!-- door-base: pkg cmd/ze internal -->

Read this before the door of the subsystem you work in. Every subsystem door
assumes it. It names the design facts that code written from habit breaks, and
the page that owns each one. Where this page and its source differ, the source
wins and this page is the defect.

## Ze is a platform, and other people's code runs against it

Ze is not a closed program whose every caller sits in this repository. Five
kinds of outside party depend on it, and none of them can be fixed by editing
`internal/`.

| Outside party | What they hold | The contract they rely on |
|---------------|----------------|---------------------------|
| A third-party plugin, in Go or any other language | A process Ze forks from `plugin { external <name> { run ...; } }`, or one built on `pkg/plugin/sdk` | The process protocol: YANG RPC over one connection (request lines newline-framed, answer lines width-framed), the five-stage startup, the JSON event format (`docs/architecture/api/process-protocol.md`, `docs/plugin-development/`) |
| An ExaBGP script an operator migrated | A script written against ExaBGP's process API | The ExaBGP bridge and the text command grammar (`docs/architecture/exabgp-bridge.md`, `docs/architecture/api/commands.md`) |
| An API client: REST, gRPC, gNMI, MCP, birdwatcher | A program speaking to a listener | The command registry and its declared answer shapes, the YANG schema, the JSON format (`docs/architecture/api/`) |
| An operator | A config file and their muscle memory at the CLI | The YANG schema, the command grammar, the error text (`ai/rules/config.md`, `ai/rules/cli.md`, `ai/rules/user-facing-errors.md`) |
| A protocol peer | Another router | The RFC, then ExaBGP API compatibility, then ExaBGP's behavior, in that order (`ai/rules/rfc-compliance.md`) |

**`pkg/` is the public API.** `pkg/plugin/sdk`, `pkg/plugin/rpc`, `pkg/ze`,
`pkg/zefs` and `pkg/fleet` are imported by code outside this repository. The
engine interfaces (`ze.Engine`, `ze.ConfigProvider`, `ze.PluginManager`,
`ze.Subsystem`, `ze.EventBus`) live there rather than in `internal/` because
external plugins depend on them (`docs/architecture/plugin/component-boundaries.md`).

A change under `pkg/` is a change to a published contract. A type, a field, a
callback name and the bytes on the wire all count. The SDK stays generic and
carries no plugin-specific code (`docs/architecture/plugin/plugin-system.md`,
"The SDK stays generic"). Ze is pre-release, so a contract can still change.
It changes on purpose, with its page, never as a side effect.

## A plugin can run in another process

Every plugin, ours included, runs either internal (a goroutine in the daemon) or
external (a forked subprocess that talks only over TLS and RPC). The SDK hides
which. Code that only works internal is broken.

- **A plain Go call into another component's package is the classic defect.** It
  works internal because memory is shared. Run external, it mutates the
  subprocess's own disconnected copy, with no error and no log line, so the
  feature silently never works (`plugin-system.md`, "The process boundary").
  `./le plugin boundary check` catches the known calls. It is a heuristic, not
  a proof.
- **Reach another unit through the boundary that survives a process split:**
  - the EventBus for one-to-many notification with no answer
  - DirectBridge for a typed request and response
  - the text command dispatcher for the slow path an external caller uses
    (`plugin-system.md`, "Communication patterns")

  DirectBridge is an in-process optimization of the same protocol, not a second
  protocol: an external plugin gets the same behavior over RPC.
- **What crosses a boundary is a self-contained value.** No pointer into data
  another plugin or component allocated, even when its package is shared core
  (`plugin-system.md`, "Cross-boundary value types").
- **A declaration is a pure value.** A plugin queried in declare mode
  (`ze.plugin.mode=declare`, spelled `ZE_PLUGIN_MODE` in the environment)
  starts nothing (`ai/rules/plugins.md`).

## The core knows nothing, and everything registers

- **BGP is a subsystem, and the engine has no BGP knowledge.** The BGP
  subsystem owns TCP, the FSM and the wire. Its plugins (`bgp-rib`, `bgp-rs`,
  `bgp-gr` and the rest under `internal/component/bgp/plugins`) extend it, and
  each one can be removed. The whole subsystem and its plugins compile out
  with `ze_bgp`. A subsystem is not a plugin, and a plugin is not a subsystem
  (`component-boundaries.md`). The mechanism today: the BGP reactor attaches to
  the coordinator from `internal/component/bgp/plugin/register.go`, and only
  L2TP and PPPoE go through `Engine.RegisterSubsystem`
  (`ai/doors/hub-engine.md`).
- **A compile tag is a boundary, and code MUST NOT reach across it.** A
  `ze_<feature>` tag in `feature-gates.txt` removes a package and everything
  only it imports.

  - Always-on code MUST NOT import a gated package.
  - Code reaches a gated feature only through a registry or a seam that the
    feature fills from its own `init()`. A build without the tag then still
    compiles, and it answers that the feature is absent.
  - A gate that depends on another gate (BMP needs `ze_bgp`) declares it.
    Read "Dependent gates and dependent files" in
    `docs/architecture/plugin/feature-gates.md` before you add one.
  - `./le arch tier check` refuses the import. The generated
    `plugin/all/all_ze_<tag>.go` files own the blank imports
    (`docs/architecture/module-tiers.md`, "Compile-out").
  - An untagged test does not link a gated package. Run it with its tag.
- `internal/component/plugin/` imports nothing from `internal/component/bgp/`
  (`component-boundaries.md`), apart from the blank imports in the generated
  composition root `plugin/all`. `plugin/server` does import `iface` for
  autoload, which section 19 of `core-design.md` does not allow: a known gap,
  not a precedent.
- **A feature registers from `init()` in its own `register.go` and is
  discovered.** It never needs an edit to a switch, a factory, a field list, a
  help string or a completion table elsewhere (`ai/rules/principles.md`,
  `ai/patterns/registration.md`). The composition root
  `internal/component/plugin/all/all.go` is generated by `./le repo generate`.
  A blank import is the only coupling.
- **A plugin owns its whole surface, and removing it removes every feature it
  provides while the rest keeps working** (`ai/rules/plugins.md`). A
  compile-out-able feature is declared once in `feature-gates.txt`, and
  always-on code never imports it (`docs/architecture/module-tiers.md`).
- **Where a package lives follows dependency direction**:
  - a config-driven engine others depend on is a component (`internal/component/`)
  - one nothing depends on is an edge plugin (`internal/plugins/`, or
    `<subsystem>/plugins/` for a sub-plugin)
  - a package with no config lifecycle and no registry side effect is core
    (`internal/core/`)

  `./le arch tier check` enforces it
  (`ai/rules/architecture.md`). Allowed imports between components are the
  table in `docs/architecture/core-design.md`, section 19.

## One schema, one command registry, every interface

- **Config is YANG-modeled, and `ConfigProvider` is its single authority** for
  the editor, the web UI, subsystems and plugins. A config change is a
  transaction of stream events (verify, apply, rollback) that each plugin
  answers for its own roots (`docs/architecture/config/transaction-protocol.md`).
  A backend that cannot apply a config exactly refuses it at verify. It never
  approximates (`ai/rules/architecture.md`, "Design Principles", and
  `ai/rationale/exact-or-reject.md`).
- **Every RPC carries a YANG registration.** There is no command module
  category: whatever has RPCs is a plugin (`ai/rules/plugins.md`). Dispatch is
  registration-based at every level, never a switch.
- **A command is declared once and reaches every interface.** The CLI, SSH, the
  web UI, REST, gRPC and MCP dispatch into the same command registry. MCP
  derives its tools from it at call time, so a new command appears with no
  edit to the interface packages (`ai/digests/mcp.md`,
  `docs/architecture/api/commands.md`). Adding a command to one interface by
  hand is the defect.

  The one exception is a local-data command
  (`command.MustRegisterLocalData`). It is answered in the caller's own
  process, from a registry that `init()` filled. It has no daemon side, and so
  reaches the local CLI only (`commands.md`, "Local-data commands"). gNMI and the
  metrics exporters serve the config tree and the metrics registry, not
  commands.
- **Every message a user sees names the problem, why, and the exact fix**, on
  every interface (`ai/rules/user-facing-errors.md`).

## Shared machinery you use rather than rebuild

| Need | Use | Page |
|------|-----|------|
| Persist runtime state | In the daemon, `statestore` under a key in `pkg/zefs/keys.go`, never `os.WriteFile`. In a plugin, the SDK's `StateGet`/`StatePut`/`StateIncrement`: an external process has no store, and `statestore.Put` there answers `false, nil` and saves nothing | `ai/rules/architecture.md`, `docs/architecture/zefs-format.md`, `process-protocol.md` "Persistent runtime state" |
| An address family | `family.MustRegister` in `internal/core/family` | `plugin-system.md`, "Address family registration" |
| An env var or a log key | `env.MustRegister`, `slogutil.Logger` | `ai/rules/config.md`, `ai/rules/go-standards.md` |
| Encode bytes | `WriteTo(buf, off) int` into a pooled buffer, no `make` where a pool exists | `ai/rules/performance.md`, `docs/architecture/buffer-architecture.md` |
| A goroutine | A long-lived worker on a channel, never one per event | `ai/rules/goroutine-lifecycle.md` |
| A readiness check | A doctor check registered by the package that owns the dependency | `plugin-system.md`, "Proximity" |
| Markup | A `.templ` file, no inline script or style | `ai/rules/architecture.md` |

## Before you write code in a subsystem

1. Open that subsystem's door (`ai/doors/README.md`). It names the pages, the
   digest, the pattern and the gates for that area.
2. Ask the four questions this page exists for. Who outside the repository
   reaches what I am changing? Does it still work if the plugin runs in another
   process? Am I adding a registration or editing a central list? Which page
   becomes wrong, so that I edit it in the same change?
3. Grep for the pattern that already exists before you design one
   (`ai/rules/architecture.md`).
