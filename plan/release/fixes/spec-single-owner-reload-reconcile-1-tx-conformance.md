# Spec: single-owner-reload-reconcile-1-tx-conformance

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A reload that adds a new config root bypasses its owner's in-process verifier before starting the plugin, and no conformance test forces every config-apply participant to round-trip apply, change, rollback and restart against one owner record. |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Parent umbrella: `plan/release/fixes/spec-single-owner-reload-reconcile.md` (Family S3, part 2 of 3).

## Task

**Root cause.** Two reload-wide invariants remain unguarded at HEAD:

1. `Server.reloadConfig` (`internal/component/plugin/server/reload.go`) calls `autoLoadForReload` before `runTxCoordinator`. When a reload adds a config root whose plugin was not running at boot (such as `traffic`), the newly auto-loaded plugin starts with Stage 2 `OnConfigure` before any `InProcessConfigVerifier` or transaction `OnConfigVerify` runs, so an unsupported configuration can mutate the dataplane or pass without running the verifier that `ze config validate` and editor commit run (`config.VerifyPluginConfigMapTransition`).
2. No registered conformance suite iterates every reload participant declared in `registry.All()` to prove that apply A, apply B, rollback to A and restart produce the same observed state as a fresh apply of each tree. A new config-owning plugin can register without proving round-trip parity.

**Goal.**

1. Run `config.VerifyPluginConfigMapTransition(running, newTree)` in `Server.reloadConfig` before `autoLoadForReload` starts any plugin, refusing invalid newly added roots on reload without changing `LoadConfigForStart` (preserving the 2026-11-26 "Ze always starts" rule).
2. Add a conformance fixture registration on `registry.Registration` (or a sibling registration seam in `internal/component/plugin/registry`) and a conformance test in `internal/component/plugin/all/reload_conformance_test.go` (`TestEveryParticipantRoundTripsApplyRollbackRestart`) that iterates every registered plugin declaring `ConfigRoots`. Any participant declaring no fixture fails the test by name; every participant declaring a fixture is driven through apply A, apply B, rollback to A, and restart, asserting observed state equals a fresh apply of the expected tree at every step.

**Why it matters for shipping.** Operators adding a subsystem for the first time via reload must get the exact refusal `ze config validate` gives before any plugin starts or touches the dataplane, and every config-owning plugin must prove rollback and restart parity automatically without a hand-kept test list.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, registration over enumeration
- [ ] `ai/doors/config-yang.md` - YANG config, `VerifyPluginConfigMapTransition`, transactions
- [ ] `ai/doors/plugin-infra.md` - `registry.Registration`, `plugin/all`, `Server.reloadConfig`, `RefuseChangedDeclarations`
- [ ] `ai/doors/le-tooling.md` - test conventions
- [ ] `docs/architecture/config/transaction-protocol.md` - verify, apply, rollback, reload sections
- [ ] `docs/architecture/config/yang-config-design.md` - validation layers, `InProcessConfigVerifier`
- [ ] `docs/architecture/plugin/plugin-system.md` - `Registration` fields, startup phases
- [ ] `docs/architecture/core-design.md` - Section 18 and `## Design decisions` (2026-11-26 "Ze always starts": reload refuses invalid config as a whole while `ze start` stays unaffected)

## Current Behavior (MANDATORY)

Verified at the producers on 2026-11-26.

**Source files read:**
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` runs `VerifyConfig` (reactor), `ExtractBusConfigFromMap`, then `autoLoadForReload` without calling `config.VerifyPluginConfigMapTransition(running, newTree)`.
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfigMapTransition` runs every plugin's `InProcessConfigVerifier` for roots present in the candidate or deleted from the previous tree, whether or not the plugin is currently running.
- [ ] `internal/component/config/loader.go` - `LoadConfigForStart` excludes invalid service blocks so `ze start` always starts; `LoadConfig` runs custom validators, data-plane verdict and redistribute rules but does not run `VerifyPluginConfigMapTransition`.
- [ ] `internal/component/plugin/registry/registry.go` - `Registration` declares `ConfigRoots`, `ConfigReads` and `InProcessConfigVerifier`, with no reload conformance fixture field.
- [ ] `internal/component/plugin/all/registrations_held_test.go` - house pattern for iterating the full compiled-in registry under `internal/component/plugin/all` without import cycles.

**Behavior to preserve:**
- `LoadConfigForStart` ("Ze always starts", 2026-11-26) is untouched: no new refusal is added to `ze start`.
- `RefuseChangedDeclarations` stays the sole policy for external plugin declarations on reload (owner decision 2026-11-24; AC-10 dropped).
- Existing `TxCoordinator` verify, apply, rollback and operation-graph ordering stay unchanged.
- Compiler-enforced single-writer state follows `spec-single-owner-lifecycle.md` decision D1 (unexported fields in a dedicated type/package, no annotation gate).

**Behavior to change:**
- `Server.reloadConfig` runs `config.VerifyPluginConfigMapTransition(running, newTree)` before `autoLoadForReload`, refusing any invalid candidate root (including newly added roots whose plugin is not running yet) before starting any plugin.
- Every registered plugin with `ConfigRoots` declares a reload conformance fixture (or explicit harness factory) exercised by `TestEveryParticipantRoundTripsApplyRollbackRestart` in `internal/component/plugin/all/reload_conformance_test.go`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| In-process plugin config verifier | `internal/component/config/plugin_verify.go` `VerifyPluginConfigMapTransition`, `JoinPluginConfigVerifyErrors` | Runs every registered plugin's `InProcessConfigVerifier` over candidate and deleted roots without needing a live plugin process; called by `ze config validate`, `ValidateSemantics`, and `cmd/ze/hub/api.go` | Extends: `Server.reloadConfig` calls `VerifyPluginConfigMapTransition(running, newTree)` before `autoLoadForReload`, closing the reload gap for newly added roots |
| Reload coordinator and auto-load | `internal/component/plugin/server/reload.go` `Server.reloadConfig`, `autoLoadForReload` | Diffs running vs candidate, verifies reactor and bus config, auto-loads plugins for newly added roots, then runs `TxCoordinator` | Extends: adds the pre-autoload `VerifyPluginConfigMapTransition` gate right after `ExtractBusConfigFromMap` |
| Plugin registration | `internal/component/plugin/registry/registry.go` `Registration` (`ConfigRoots`, `ConfigReads`, `InProcessConfigVerifier`) | Declares every compiled-in config-owning plugin; iterated by tests in `internal/component/plugin/all/` | Extends: adds a `ReloadConformance` fixture registration seam so every config-owning participant is discovered by registration without a hand-kept list |
| Config transaction coordinator and SDK tx gate | `internal/component/config/transaction/orchestrator.go` `TxCoordinator`; `pkg/plugin/sdk/config_tx_gate.go` `configTxGate` | Drives verify, apply, commit and rollback events over live participants | Does not replace: the conformance harness drives each participant through its existing config lifecycle (configure, verify, apply, rollback, restart) |
| External declaration refusal on reload | `cmd/ze/hub/main_reload.go` `diskConfigLoaders`; `internal/component/plugin/server/reload_declarations.go` `RefuseChangedDeclarations` | Refuses reloads that change external plugin declarations (owner decision 2026-11-24) | Does not apply: AC-10 of the umbrella was dropped and superseded by this mechanism |
| Compiler-enforced single writer | `plan/release/fixes/spec-single-owner-lifecycle.md` D1; `docs/contributing/ze-go-style.md` "One type per lifecycle state" | Unexported fields in a package/type of its own enforce a single writer at compile time | Extends: conformance fixtures assert round-trip parity without adding an annotation gate |

## Data Flow (MANDATORY)

### Entry Point
- SIGHUP / CLI / API / managed reload entering `Server.reloadConfig` (`internal/component/plugin/server/reload.go`).
- `go test` running `TestEveryParticipantRoundTripsApplyRollbackRestart` in `internal/component/plugin/all/reload_conformance_test.go`.

### Transformation Path
1. `Server.reloadConfig` computes `diff := config.DiffMaps(running, newTree)`.
2. After `s.reactor.VerifyConfig(newTree)` and `plugin.ExtractBusConfigFromMap(newTree)`, and BEFORE `autoLoadForReload`, `reloadConfig` calls `config.VerifyPluginConfigMapTransition(running, newTree)`.
3. If any verifier returns an error, `reloadConfig` returns the joined error immediately (via `config.JoinPluginConfigVerifyErrors`), raises reload refused under `CodeDoctorConfigReloadRefused`, starts no new plugin, and leaves running state untouched.
4. In `TestEveryParticipantRoundTripsApplyRollbackRestart`, the test walks `registry.All()` for every registration with non-empty `ConfigRoots`.
5. If a participant has no registered conformance fixture, the test fails naming that plugin.
6. For each participant with a fixture, the harness runs: fresh apply of tree A (recording baseline state A), fresh apply of tree B (recording baseline state B), then on one instance: apply A -> assert state equals baseline A -> apply B -> assert state equals baseline B -> rollback to A -> assert state equals baseline A -> restart fresh instance with tree A -> assert state equals baseline A.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `plugin/server` -> `config` | Calls `config.VerifyPluginConfigMapTransition(running, newTree)` (import already exists in `reload.go`) | Yes, existing import |
| `plugin/all` test -> `plugin/registry` | Reads `registry.All()` and each participant's registered conformance fixture | Yes, same pattern as `all_test.go` |

### Changed-Wire Census

No wire format, RPC message, EventBus event, YANG leaf, or persisted state format changes in this spec.

**Search evidence:** `internal/component/plugin/server/reload.go` calls the existing in-process `config.VerifyPluginConfigMapTransition`; `internal/component/plugin/registry/registry.go` adds a Go-only test fixture registration hook that never crosses `pkg/plugin/rpc` or Stage 1 declarations; `internal/component/plugin/all/reload_conformance_test.go` is test-only.

### Integration Points
- `internal/component/plugin/server/reload.go`: pre-autoload verification gate.
- `internal/component/plugin/registry/registry.go`: `ReloadConformance` fixture type and registration helper (keeps `pkg/plugin/rpc` untouched and generic).
- `internal/component/plugin/all/reload_conformance_test.go`: registered population conformance test (avoids the `plugin/all -> config/transaction` import cycle).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | Reload calls the exact `config.VerifyPluginConfigMapTransition` used by `ze config validate` and API commit |
| No unintended coupling | Yes | `plugin/server` already imports `config`; conformance test lives in `plugin/all` where the composition root already lives |
| No duplicated functionality | Yes | Reuses `VerifyPluginConfigMapTransition` instead of adding a second reload verifier walk |
| Zero-copy preserved where applicable | Yes | Reload verification is control-plane only, off the packet/UPDATE path |
| Registration over hardcoding, outbound | Yes | Each participant registers its own fixture in its package; no central switch |
| Registration over hardcoding, inbound | Yes | `TestEveryParticipantRoundTripsApplyRollbackRestart` derives its population from `registry.All()` |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `Server.ReloadConfig` adding a root whose plugin is not running yet | -> | `config.VerifyPluginConfigMapTransition(running, newTree)` before `autoLoadForReload` | `TestReloadConfigVerifiesNewlyAddedRootBeforeAutoLoad` (`internal/component/plugin/server/reload_test.go`) |
| `registry.All()` walk over all `ConfigRoots` participants | -> | registered conformance fixture round-trip runner | `TestEveryParticipantRoundTripsApplyRollbackRestart` (`internal/component/plugin/all/reload_conformance_test.go`) |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A plugin registers with non-empty `ConfigRoots` and declares no reload conformance fixture | `TestEveryParticipantRoundTripsApplyRollbackRestart` fails and reports the missing plugin's registered name | Registered plugins with empty `ConfigRoots` (read-only `ConfigReads` or config-free plugins) remain permitted without a reload conformance fixture because they own no config root |
| AC-2 | Every registered plugin declaring `ConfigRoots` runs through: apply A, apply B, rollback to A, restart with A | Observed state after each step equals the state produced by a fresh apply of the expected tree | None; every compiled-in plugin declaring non-empty `ConfigRoots` must pass apply, change, rollback, and restart parity with zero skipped participants |
| AC-3 | A daemon running without a config root (such as `traffic`) receives a SIGHUP/reload adding that root with an invalid backend configuration (`backend vpp` with `qdisc hfsc`) | `Server.ReloadConfig` returns the `InProcessConfigVerifier` refusal (`qdisc hfsc: not supported by backend vpp`), raises `CodeDoctorConfigReloadRefused`, keeps the active running configuration intact, and starts no newly added plugin | Valid newly added config roots that pass `VerifyPluginConfigMapTransition` remain permitted to auto-load and apply on reload; external-plugin declaration changes remain refused by `RefuseChangedDeclarations` |
| AC-4 | `ze start` boots with an invalid service block (`LoadConfigForStart`) | `ze start` boots the daemon with the remaining healthy services, excludes the invalid service block under its doctor code per the 2026-11-26 "Ze always starts" ruling, and runs the new pre-autoload verifier gate only on reload (never on `LoadConfigForStart`) | Healthy service blocks in the startup config remain started and served while the invalid service block stays excluded and reported by `ze doctor`, because `ze start` must bring up networking |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryParticipantRoundTripsApplyRollbackRestart` | `internal/component/plugin/all/reload_conformance_test.go` | AC-1, AC-2 | |
| `TestConformanceTestFailsNamingUnregisteredParticipant` | `internal/component/plugin/all/reload_conformance_test.go` | AC-1 | |
| `TestReloadConfigVerifiesNewlyAddedRootBeforeAutoLoad` | `internal/component/plugin/server/reload_test.go` | AC-3, AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N/A | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `traffic-vpp-reject-hfsc-added-on-reload` | `test/reload/traffic-vpp-reject-hfsc-added-on-reload.ci` | Daemon boots with no `traffic` block, SIGHUP adds `traffic { control { backend vpp; ... qdisc hfsc; } }`, reload is refused with `qdisc hfsc: not supported by backend vpp` and running config is unchanged (AC-3) | |

### Interop Tests (Scope: config)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | - | - | Config reload verification and in-process participant conformance only; no protocol wire change | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add the `ReloadConformance` fixture registration seam in `internal/component/plugin/registry/` and `TestEveryParticipantRoundTripsApplyRollbackRestart` in `internal/component/plugin/all/reload_conformance_test.go` (red for every participant lacking a fixture), plus `TestReloadConfigVerifiesNewlyAddedRootBeforeAutoLoad` (red before pre-autoload check).
2. **Phase: Pre-autoload reload verification** -- call `config.VerifyPluginConfigMapTransition(running, newTree)` in `Server.reloadConfig` before `autoLoadForReload`, joining and returning any refusal; add `test/reload/traffic-vpp-reject-hfsc-added-on-reload.ci`.
3. **Phase: Participant fixtures** -- register a conformance fixture for every compiled-in plugin declaring `ConfigRoots` so `TestEveryParticipantRoundTripsApplyRollbackRestart` turns green across the full registry under all feature tags.

## Files to Modify
- `internal/component/plugin/server/reload.go`
- `internal/component/plugin/server/reload_test.go`
- `internal/component/plugin/registry/registry.go`
- Participant `register.go` (or sibling `conformance_fixture.go` files) for compiled-in plugins declaring `ConfigRoots`

## Files to Create
- `internal/component/plugin/all/reload_conformance_test.go`
- `test/reload/traffic-vpp-reject-hfsc-added-on-reload.ci`

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| gate-excludes-part-of-its-population | 2026-08-25 | `Server.ReloadConfig` verifies no plugin for a root the reload adds |

## Work Not Done

Sibling children of `plan/release/fixes/spec-single-owner-reload-reconcile.md` carry the remaining reload/reconcile work behind their own subsystem doors:
- `plan/release/fixes/spec-single-owner-reload-reconcile-2-iface-reconcile.md` (iface owed-apply guard, VPP learned-address proof, carrier resync epoch, DHCPv6 `IFLA_CARRIER_CHANGES`)
- `plan/release/fixes/spec-single-owner-reload-reconcile-3-firewall-nft-ownership.md` (nft `NFTA_TABLE_USERDATA` ownership marker and restart reclaim)
- `plan/release/fixes/spec-single-owner-reload-reconcile-4-web-form-atomic-save.md` (atomic web form save in `EditorManager.ApplyFormEdits` and `cli.Editor`)
- `plan/release/fixes/spec-single-owner-reload-reconcile-5-isis-identity-purge.md` (IS-IS old-SystemID LSP purge on `SystemID` change)

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A config change is a transaction of stream events (verify, apply, rollback) that each plugin answers for its own roots (`docs/architecture/config/transaction-protocol.md`)." | agrees | `Server.reloadConfig` runs `config.VerifyPluginConfigMapTransition` before `autoLoadForReload` so newly added roots are verified before starting any plugin, and `reload_conformance_test.go` verifies apply/rollback/restart for every registered `ConfigRoots` participant. |
| `docs/architecture/api/process-protocol.md` | Config Reload Protocol | the `// Design:` page of `internal/component/plugin/server/reload.go`; anchored to `reload.go`, `registry.go` | "If any plugin rejects, the reload is aborted." | agrees | Pre-autoload verification aborts the reload before starting any newly added plugin or mutating running state; wire RPC framing is unchanged. |
| `docs/architecture/behavior/signals.md` | SIGUSR1 (Reload) | anchored to `internal/component/plugin/server/reload.go` | — | silent | Describes ExaBGP signal handling; Ze's reload verifier gate is internal to `plugin/server` and changes no signal semantics. |
| `docs/architecture/config/apply-ordering.md` | Registry fixtures | anchored to `internal/component/plugin/server/reload.go` | "Tests that register operation decomposers, constraint rules or settlement rules save `SnapshotOperationRegistry` before registering fixtures and restore it in `t.Cleanup`." | agrees | The conformance suite runs against the sealed `registry.All()` set and isolates any registry-mutating tests with `registry.Snapshot()` / `Restore()`. |
| `docs/architecture/config/transaction-protocol.md` | 5. Plugin Load/Unload During Reload | anchored to `internal/component/plugin/server/reload.go`, `internal/component/plugin/registry/registry.go` | "When a config change adds or removes config roots, plugins must be loaded or stopped. This happens outside the transaction, not inside it." | agrees | Verification of newly added config roots runs before `autoLoadForReload`, preventing invalid newly added roots from starting a plugin outside the transaction. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `internal/component/plugin/server/reload.go` | — | silent | Covers `ze doctor` runtime checks; reload pre-autoload verification does not alter startup service-block behavior or doctor codes. |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `internal/component/plugin/server/reload.go`, `internal/component/plugin/registry/registry.go` | "Registration happens only during package initialization. Go runs every `init()` before `main`, and `main` in `cmd/ze` calls `registry.Seal` before it dispatches any command, so a later `registry.Register` changes nothing and returns an error wrapping `registry.ErrRegistrySealed` that names the plugin." | agrees | Conformance fixtures are registered in `init()` (or on `registry.Registration`) before `registry.Seal`, obeying the seal guard and `TestEveryRegistryWriterIsGuardedOrASeam`. |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `plugin/server`, `plugin/registry`, `plugin/all` | "The engine never enumerates plugins." | agrees | Both `VerifyPluginConfigMapTransition` and `TestEveryParticipantRoundTripsApplyRollbackRestart` iterate registrations dynamically rather than hardcoding plugin names. |
| `docs/architecture/api/architecture.md` | Message Bus and Participant Wiring | the `// Design:` page of `internal/component/plugin/registry/registry.go`; anchored to `registry.go` | — | silent | Describes bus participant filtering and wiring; adding a conformance-fixture registration field leaves bus participant routing and wire behavior unchanged. |
| `docs/architecture/api/commands.md` | Overview | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers CLI/API command syntax and dispatch; this spec adds no commands or pipe operators. |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers BGP UPDATE command and config syntax; unaffected by reload verification or conformance testing. |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers BGP UPDATE forwarding pipeline; unaffected by plugin reload verification. |
| `docs/architecture/config/syntax.md` | Overview | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers Ze configuration file grammar; unaffected by reload verification. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers metrics collection model; unaffected by reload verification. |
| `docs/architecture/dns/geodns.md` | Config model | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers GeoDNS plugin configuration; unaffected except that GeoDNS gets a conformance fixture entry if it declares `ConfigRoots`. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers IKEv2 engine design; unaffected by reload pre-autoload verification. |
| `docs/architecture/wire/nlri.md` | Overview | anchored to `internal/component/plugin/registry/registry.go` | — | silent | Covers BGP NLRI wire encoding; unaffected by reload verification. |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/reload/traffic-vpp-reject-hfsc-added-on-reload.ci` | "A `.ci` that needs a Linux kernel feature carries `option=needs-linux`" | agrees | The new `.ci` functional test under `test/reload/` exercises reload refusal of an invalid newly added root through the standard functional runner. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | No engine operation or plugin profile is added or changed; pre-autoload verification runs in-process before plugin startup. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Event delivery queues and losslessness are untouched by this spec. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | Offline config parsing and query-mode external plugin validation are untouched; reload continues to refuse changed external plugin declarations via `RefuseChangedDeclarations`. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Implied filter display is untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Bus participant naming and message registration are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Bus `send > to` semantics are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Bus `receive > from` semantics are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Bus allow-list enforcement is untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Bus filter compilation and commit conflict checks are untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Bus non-arbitration is untouched by this spec. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Bus filter display is untouched by this spec. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Command grammar and pipe semantics are untouched by this spec. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Pipe overrides are untouched by this spec. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Pipe operation semantics are untouched by this spec. |

## Design presentation

### What changes
- `Server.reloadConfig` runs `config.VerifyPluginConfigMapTransition(running, newTree)` before `autoLoadForReload`, closing the gap where a newly added config root bypassed its `InProcessConfigVerifier` on reload.
- `registry.Registration` gains a reload conformance fixture registration seam, and `internal/component/plugin/all/reload_conformance_test.go` runs apply A, apply B, rollback to A, and restart against every registered plugin with `ConfigRoots`, failing by name if any participant omits a fixture.

### Before and after
- **Before:** Booting without `traffic` and SIGHUP-ing to `traffic { control { backend vpp; interface eth0 { qdisc { type hfsc; } } } }` bypassed `verifyTrafficConfig` before `autoLoadForReload` started `traffic` via Stage 2 `OnConfigure`; new config-owning plugins had no automated apply/rollback/restart conformance test.
- **After:** Reload runs `VerifyPluginConfigMapTransition(running, newTree)` before `autoLoadForReload`, refusing the invalid candidate with the exact `ze config validate` error and starting no plugin; `TestEveryParticipantRoundTripsApplyRollbackRestart` fails by name if any `ConfigRoots` plugin lacks a fixture and verifies apply, change, rollback and restart parity for every participant.

### Alternatives set aside
- **Calling `VerifyPluginConfigMapTransition` inside `config.LoadConfig` for both start and reload:** Rejected because `LoadConfigForStart` must keep the 2026-11-26 "Ze always starts" service-block exclusion behavior; running the gate in `Server.reloadConfig` keeps reload strict without adding any refusal to `ze start`.
- **Putting the conformance test inside `internal/component/config/transaction/`:** Rejected because `plugin/all` imports `plugin/server` which imports `config/transaction`, so importing `plugin/all` from `config/transaction` creates a Go import cycle. Placing the test in `internal/component/plugin/all/` matches the repository's existing all-plugins tests (`all_test.go`, `registrations_held_test.go`).
- **Maintaining a hand-kept list of participants in the conformance test:** Rejected under `ai/doors/system.md` ("Registration over enumeration"): iterating `registry.All()` guarantees every new `ConfigRoots` participant is caught automatically.

### Documents departed from
None.

### Choices for Thomas
none

## Review Gate

- [ ] `/ze-review` passed on diff
- [ ] `/ze-review-spec` confirmed all acceptance criteria and files

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Producer evidence re-read for every row
- [ ] No code snippets

### Goal Gates (MUST pass)
- [ ] AC-1 to AC-4 each proven by a named test
- [ ] Wiring Test table complete
- [ ] Targeted tests pass in the main checkout: `./le test unit pkg path ./internal/component/plugin/server/`, `./le test unit pkg path ./internal/component/plugin/all/`, and `./le test functional reload`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` test for end-to-end reload refusal

### Closure
- [ ] Owned journal row marked `fixed:` with commit and test evidence
