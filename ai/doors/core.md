# The core door: shared libraries and runtime state

<!-- door-base: internal/core pkg/zefs -->

Read `ai/doors/system.md` first. This door covers:

- `internal/core/`
- the `pkg/zefs` store format
- `internal/core/statestore`
- the registries that every tier reads: families, env vars, event namespaces,
  and the core-leaf registries such as `internal/core/ipsecinventory`

## Where it sits

- **`internal/core/` is the leaf tier.** A core package is a library with no
  config-driven lifecycle and no registry side effect (`docs/architecture/module-tiers.md`,
  "The three tiers"). It imports neither `internal/component/` nor
  `internal/plugins/`. The `core-import-direction` check in `./le arch tier check`
  refuses a new upward import (`internal/le/arch/tier/gates.go` `coreDirectionGate`).
  The grandfathered pairs in `internal/le/arch/tier/testdata/core_import_baseline.txt`
  can only shrink.
- **Everything depends on it.** Components, plugins and the public SDK all import
  core. `pkg/plugin/sdk` and `pkg/plugin/rpc` import `internal/core/selector`,
  `internal/core/env` and `internal/core/textbuf`.
- **It cannot be compiled out.** A feature that a build can drop is not core: it
  is a gated component or plugin (`feature-gates.txt`, `module-tiers.md`, "Compile-out").
- **`pkg/zefs` is public.** It defines the on-disk ZeFS format and the key
  registry (`docs/architecture/zefs-format.md`).

## Who depends on it from outside

| Outside party | What they hold | Contract and page |
|---------------|----------------|-------------------|
| A Go plugin built outside this repository | Calls on `pkg/plugin/sdk` | A core type in an SDK signature is part of the SDK: `sdk.Plugin.UpdateRouteSel` takes a `*selector.Selector`. Go does not let another module import `internal/`, so that plugin reaches the type only through what `pkg/` exposes |
| A plugin in any language | A Stage 1 declaration and JSON events | Family names `<afi>/<safi>` with their AFI and SAFI numbers, which the engine forwards to `family.RegisterFamily` (`docs/architecture/plugin/plugin-system.md`, "Address family registration"). Event namespace and type names it subscribes to (`docs/architecture/api/process-protocol.md`, "Subscription Namespace", `docs/architecture/api/json-format.md`) |
| A plugin that persists state | `StateGet`, `StatePut` and the other state RPCs | Key bounds, status values and key ownership (`process-protocol.md`, "Persistent runtime state") |
| An operator | Env vars, `database.zefs` on disk | Env var names and their precedence (`docs/architecture/config/environment.md`). Key patterns that `ze data registered` lists (`zefs-format.md`, "Key Registry") |

A rename of a family, an event type, an env key or a zefs key changes each of
these contracts. It is not a refactor.

## Boundaries

- **Never import upward.** Where core needs something a component owns, the
  component registers into core. Do not add a row to the core import baseline.
- **Copy the core-leaf registry shape.** `internal/core/ipsecinventory` imports
  the standard library only. The IKE engine calls `Register` from `init()`, and a
  reader calls `Tunnels()` for value-type records. With no engine linked, it
  answers `ErrNotRegistered`, never an empty list (`docs/architecture/core-design.md`,
  section 19, and `internal/core/ipsecinventory/registry.go`). `internal/core/ikeprobe`
  has the same shape.
- **Runtime state goes through the daemon's one store.** In the daemon, use
  `statestore` with a key registered in `pkg/zefs/keys.go`
  (`ai/rules/architecture.md`). A plugin uses the SDK state RPCs, and its owning
  package grants its key patterns with `statestore.RegisterPluginKeys`
  (`process-protocol.md`, "Persistent runtime state"). `./le arch fs-persistence check`
  refuses a raw file write under the config or state directory.
- **A registry holds value types and is the only list.** Cross-boundary values
  carry no pointer into producer data (`plugin-system.md`, "Cross-boundary value
  types"). Code that copies what a registry holds into a literal or a switch is
  refused by `./le arch enumeration check`.
- **Events register their payload type once.** Typed handles come from
  `events.Register[T]` and `events.RegisterSignal` (`internal/core/events/typed.go`).
  Namespaces and plugin event types come from `RegisterNamespace` and
  `RegisterEventType` (`internal/core/events/events.go`). The delivery path is in
  `ai/digests/plugin-transport.md`.

## Read before code

| Read | The question it answers |
|------|-------------------------|
| `docs/architecture/module-tiers.md` | Does this package belong in core, and what does the tier gate refuse? |
| `docs/architecture/core-design.md`, section 19 | Which component is allowed to import which, and how a leaf registry replaces an import |
| `docs/architecture/zefs-format.md` | Key registry, in-process locking, "Runtime state through statestore" |
| `docs/architecture/storage-backends.md` | Who owns the live store and its lifetime lock |
| `docs/architecture/config/environment.md` | Env precedence, where keys register, why an unregistered key aborts |
| `docs/architecture/plugin/plugin-system.md` | Family registration, cross-boundary value types |
| `docs/architecture/api/process-protocol.md` | State RPCs, event subscription namespace |
| `ai/digests/plugin-transport.md` | How an `Event[T]` emit reaches in-process and external subscribers (core has no digest of its own) |
| `ai/patterns/registration.md` | The shape every registry follows |
| `ai/rules/architecture.md` | Placement, and the `statestore` rule |
| `ai/rules/config.md` | When a setting can be env-only |
| `ai/rules/go-standards.md` | Env access, logging, constructors |

## Traps

- **A duplicate env key wins by link order.** `env.MustRegister`
  (`internal/core/env/registry.go`) overwrites an existing key with no check, so
  the last `init()` wins, and the winner differs per binary.
  Source: `plan/learned/RECURRING-PATTERNS.md`, "Env var registered in two places
  drifts", and `plan/journal/env-var-double-registration.md`.
- **A key read where its registration is not linked ends the process.**
  `env.Get` on an unregistered key calls `mustBeRegistered`, which exits 2
  (`internal/core/env/env.go`). It has happened to SDK keys read by
  `internal/component/plugin/cli` and `internal/core/rib/locrib`, and to
  `ze.storage.blob` read by `internal/core/resolve`. Register a key beside its
  reader. Source: `plan/journal/env-key-read-but-never-registered.md`.
- **`statestore.Put` reports nothing when there is no store.** With no store set
  it returns `false, nil` (`internal/core/statestore/statestore.go` `Put`), which is
  the case in a plugin process the daemon forked. A caller that needs an
  acknowledgement uses `Write`, `Read`, `Delete` or `Increment`, or the SDK
  state RPCs in a plugin.
- **ZeFS excludes only within one process.** The blob mutex does not stop a
  second process (`zefs-format.md`, "Single-process ownership").
  Source: `plan/journal/store-serializes-in-process-only.md`.
- **Tests can share the developer's real store.** Unless `ze.config.dir` is set,
  `DefaultConfigDir` resolves from the binary path, so a test can read and
  write state another test left.
  Source: `plan/journal/suite-shares-one-persistent-store.md`.
- **A test that mutates a package-level registry poisons the next test.** Take a
  snapshot before the first mutation and restore it in `t.Cleanup`.
  Source: `RECURRING-PATTERNS.md`, "Package-level registry contamination across
  tests", and `plan/journal/registry-contamination.md`.
- **A one-slot registry is owned by link order.** Two plugins claimed the PPP
  auth handler slot from `init()`, and the last one linked won.
  Source: `plan/journal/registry-contamination.md`, 2026-08-13 row.
- **Validate a registered set by format, never by membership.** Families are
  registered at run time, so a static membership list is wrong by construction.
  Source: `plan/learned/DESIGN-HISTORY.md`, Configuration, load-bearing invariants (215).

## Gates

| Command | What it judges |
|---------|----------------|
| `./le arch tier check` | Core import direction, placement, compile-out |
| `./le arch fs-persistence check` | No raw runtime-state file writes |
| `./le arch enumeration check` | No literal copy of what a registry holds |
| `./le job run label core command go test -race ./internal/core/<pkg>/` | The scoped package test (`ai/rules/commands.md`) |
| `./le doc check all` | Source anchors in any page you edited |
