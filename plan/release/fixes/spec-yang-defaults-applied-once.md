# Spec: yang-defaults-applied-once

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | A YANG `default` takes effect only where a consumer remembers to call `config.ApplyDefaults`, so `show` and the docs state a value the device does not run (`accept-ra`), and the listener conflict check never sees default ports. Both are accepted-but-ineffective configuration in shipped capabilities (`plan/README.md`, "Split fixes from features") |
| RFC gaps | - |
| Depends | Split from `plan/release/fixes/spec-yang-single-source-of-truth.md` (piece C). Siblings: `plan/release/fixes/spec-validation-walks-every-schema-section.md` (piece B), `plan/release/fixes/spec-review-typed-config-decode.md` (piece A). Choice-case defaults (RFC 7950 Section 7.6.1) are already applied by `ApplyDefaults` through `yang.CaseInEffect`; `when` stays with `plan/release/fixes/spec-config-yang-when-unique-choice.md`. Rpc input defaults stay with `plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md` |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| ID | Decision |
|----|----------|
| D-1 (YANG defaults for external plugins) | Option 3 (presented as Option 1 in interview): Ze delivers **both** the operator-configured values and a separate defaults map in Stage 2 / transaction config payloads (documented in `docs/plugin-development/protocol.md`, with an SDK merge helper), so external plugins receive defaults automatically while still distinguishing explicit settings from omitted leaves. |

## Task

**Problem.** A YANG `default` is applied by each consumer that calls
`config.ApplyDefaults` on its own subtree. Four do (iface tunnels, the PPPoE
client timing, sysrib admin distances, the BGP peer schema). Every other
consumer either restates the default in Go or runs with the zero value. Two
recorded results: the interface `ipv6 accept-ra` leaf declares `default 0`,
nothing applies it, and the device runs the kernel's own value; the listener
conflict check judges only endpoints an operator wrote, so two services that
collide on a default port pass `ze config validate` and fail at bind.

**Goal.** A YANG default reaches every consumer from one place, applied once on
the delivery path, and listener defaults come from the same schema facts, so the
conflict check judges the endpoints the daemon will bind.

**Open decision.** An external plugin receives its config at Stage 2 as JSON
strings and cannot call `ApplyDefaults`, which lives under `internal/`. Who
applies its defaults is a contract question about `pkg/plugin`, recorded in the
decisions table below for Thomas.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md`, "How a Config Value Reaches a Plugin", and section 4 on choice-case defaults
  → Constraint: every delivered value is a JSON string; `Tree.ToPluginMap` is the lowering a plugin's config goes through
  → Decision: `ApplyDefaults` fills a case's default only while `yang.CaseInEffect` says the case is in effect
- [ ] `docs/architecture/config/yang-config-design.md`, section 11 "Network Endpoints"
  → Constraint: `refine` sets per-service listener defaults for `ip` and `port`
- [ ] `docs/plugin-development/protocol.md`, "Stage 2"
  → Constraint: a removed root arrives as `{}`; the body is a published contract for third-party plugins
- [ ] `ai/doors/config-yang.md` gate row: `./le config ports check` judges the listener default table against the YANG refine defaults

**Key insights:**
- `RegisterBuiltinListenerDefaults` exists because, by its own comment, the schema build does not propagate YANG `refine` defaults; the table is a second copy that a gate compares with the first.
- RFC 4862 Section 5.5: "However, the processing described below MUST be enabled by default." The declared `accept-ra` default of 0 contradicts it, so the fix is not to apply that value.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/schema_defaults.go` `ApplyDefaults` - fills missing leaves from the schema node, creates non-presence containers, skips a case not in effect; called by `iface/tunnel.go` `tunnelSchema.applyDefaults`, `iface/pppoe_client_timing.go` `containerWithDefaults`, `sysrib/sysrib.go` (admin distances) and `bgp/config/peers.go` `applyPeerSchemaDefaults`, and by nothing on the delivery path
- [ ] `internal/component/bgp/config/peers.go` `applyPeerSchemaDefaults` - returns with no defaults and no error when the schema fails to load or `bgp/peer` is not found (a silent zero, `ai/rules/principles.md`); the iface and sysrib callers return an error instead
- [ ] `internal/component/config/tree.go` `ToPluginMap` - lowers the tree with `toMap`, no defaults
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` `leaf accept-ra` - `uint8` range 0..2, `default 0`; `internal/component/iface/config_sysctl.go` writes `accept_ra` only when the parsed `AcceptRA` pointer is set, so the default never reaches the kernel
- [ ] `internal/component/config/listener_defaults.go` `RegisterBuiltinListenerDefaults` - a hand table of eight service endpoints plus the l2tp entry default; its only non-test caller is `registerListenerDefaultsOnce` in `doctor/checks_listener.go` `collectSchemaListeners`
- [ ] `internal/component/config/listener.go` `CollectListenersWithDefaults` - adds the registered defaults; called only by the doctor. `CollectListeners` (no defaults) is what `config/cli/cmd_validate.go` (`config-listener-conflict`), `config/graph.go` `addListenerNodes` and `bgp/config/loader_create.go` judge
- [ ] `internal/component/config/listener.go` `listenerProtocols` - the transport table is seeded at package init for the same reason: the defaults table never runs for `ze config validate`

**Behavior to preserve:**
- Every config that loads today loads with the same effect, except where the effect was a defect named below.
- A plugin that reads a leaf absent from the map to mean "inherit" keeps that meaning (BGP group and peer inheritance).
- Stage 2 bytes to a third-party plugin change only through the open decision, on purpose and with its page.

**Behavior to change:**
- Every compiled-in consumer receives the YANG defaults of its subtree without calling `ApplyDefaults`.
- The listener conflict check at `ze config validate`, `ze start` and reload judges default endpoints.
- `accept-ra` declares what the device runs.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Default application | `config/schema_defaults.go` `ApplyDefaults`, `choiceMembers.skipsDefaults` | Fills defaults into one map from one schema node, case-aware | Extends: called once on the delivery path; the per-consumer calls are deleted |
| Plugin lowering | `config/tree.go` `ToPluginMap`; `yang-config-design.md`, section 14 | Lowers a plugin's subtree to strings with no defaults | Extends: the delivery path applies defaults before or during the lowering, subject to the open decision for external plugins |
| Declared lookup | `config` `Schema.LookupDeclared` (`yang-config-design.md`, `ze:os` row) | Answers what a module declares about a node, defaults included, on every host | Extends: the source of listener defaults |
| Listener default table | `config/listener_defaults.go` `RegisterBuiltinListenerDefaults`, `RegisterListenerDefault`, `RegisterListenerEntryDefault` | Hand table of service endpoints, run only for the doctor | Replaces: defaults derived from the YANG `refine` defaults; the empty-list behavior each service's extraction decides (`WhenListEmpty`) is registered by that service from its own `register.go` |
| Listener collection | `config/listener.go` `CollectListeners`, `CollectListenersWithDefaults`, `DiscoverListenerServices` | Two collectors, only the doctor's adds defaults | Replaces: one collector that adds defaults, used by every caller (`ai/rules/no-layering.md`) |
| Port table gate | `internal/le/config/ports` (`./le config ports check`) | Compares the hand table with the YANG refine defaults | Replaces: with no second table there is nothing to compare; the gate is deleted or becomes a check that every `ze:listener` service resolves a default |
| Doctor listener probe | `doctor/checks_listener.go` `collectSchemaListeners` | Probes default endpoints at run time | Extends: uses the one collector; no `sync.Once` registration |
| Choice-case defaults | `config/yang/validator_default.go` `CaseInEffect` | Decides whether a case's defaults are in use | Does not apply: already correct; kept as is |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config file, editor commit, reload: the parsed `Tree`.

### Transformation Path
1. Parse and validate (`LoadConfig`).
2. Per plugin root, lower with `ToPluginMap` and apply the subtree's schema defaults once.
3. Deliver at Stage 2 or in a transaction (`rpc.ConfigSection`), or hand the map to an in-process consumer.
4. Listener collection reads configured and default endpoints from the same schema facts, then `FindListenerConflict` judges them.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | Stage 2 and transaction JSON | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `test/`; terms `ToPluginMap`, `ConfigSection`, `ApplyDefaults`, `CollectListeners`; positive control: `applyPeerSchemaDefaults` found as a known consumer of defaults. Whether the Stage 2 body changes is the open decision D-1; the rows record both ends either way.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Stage 2 and transaction config body (`rpc.ConfigSection`) | writer | `config/tree.go` `ToPluginMap`, lowered for delivery by the plugin server | `ToPluginMap` calls `toMap` with no defaults | Under D-1 option 1 the body gains default leaves; under option 2 or 3 the configured values are preserved |
| same | reader | `pkg/plugin/sdk` config callbacks (`OnConfigVerify`, `OnConfigApply`); every internal plugin's config parser | `docs/plugin-development/protocol.md`, "Stage 2" | Internal parsers drop their own defaults under any option; a third-party reader is outside the repository and is told by the page |

### Integration Points
- The plugin server's config delivery and the in-process consumers named above.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Defaults applied on the delivery path |
| No unintended coupling (components stay isolated) | No | Services register their own listener behavior |
| No duplicated functionality (extends existing, does not recreate) | No | One collector, one default source |
| Zero-copy preserved where applicable (refs, not copies) | No | Config delivery is not a hot path |
| Registration over hardcoding, outbound | No | A service's empty-list behavior registers from its `register.go` |
| Registration over hardcoding, inbound | No | Searched: `RegisterBuiltinListenerDefaults` (deleted), `listenerProtocols` seed (moves to each service's registration), per-consumer `ApplyDefaults` calls (deleted) |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No internal consumer relies on a leaf being absent where a YANG default exists | inheritance reads absence only in the BGP group and peer layers | A consumer reads a default as an explicit setting | review of every consumer of a defaulted leaf; BGP inheritance tests | unvalidated |
| A-2 | The schema build can carry `refine` defaults onto the refined node | `listener_defaults.go` comment says they are not propagated today | A second source stays | unit test over `zt:listener` refines | unvalidated |
| A-3 | Applying defaults at delivery changes no reload diff | defaults are applied to both sides of the diff | spurious changes at reload | reload `.ci` tests | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A default newly applied changes device behavior | a shipped config's behavior differs in a functional test | Each such leaf is checked against its RFC or design, as `accept-ra` is |
| R-2 | A default port conflict newly refused in shipped configs | parse suite reds | Each is a real bind conflict; fix the config |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin runs with a value it did not run with before; a config is refused for a default port clash |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-review-typed-config-decode` (BGP decode from the resolved tree), `spec-config-yang-rpc-mandatory-defaults`, `spec-plugin-external-yang-at-startup` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on two services that both default to one port | → | the one listener collector with schema defaults, `FindListenerConflict` | `test/parse/listener-default-conflict-refused.ci` |
| `ze start` with a plugin leaf that has a YANG default the config omits | → | delivery-path default application | `test/plugin/yang-default-delivered.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A leaf with a YANG `default` the operator omits, in a compiled-in plugin's subtree (old AC-5) | The plugin receives the default without calling `ApplyDefaults`; the four per-consumer calls are deleted | A leaf whose absence means "inherit" (BGP group and peer layers) keeps arriving absent where the design says so; each one is named in the design page |
| AC-2 | Two endpoints colliding on a default port (old AC-5) | `ze config validate`, `ze start` and reload refuse with `config-listener-conflict`, naming both services and the port | None |
| AC-3 | Listener defaults | Derived from the YANG `refine` defaults; `RegisterBuiltinListenerDefaults` and the second collector no longer exist; `ze doctor` probes the same endpoints the validator judges | A service whose empty server list starts nothing registers that fact from its own package |
| AC-4 | A unit with no `ipv6 accept-ra` | Ze writes no `accept_ra` sysctl, so the kernel's RA processing stays enabled (RFC 4862 Section 5.5), and the leaf declares no default that contradicts that | An operator who sets the leaf gets the value written |
| AC-5 | The schema fails to load in a consumer that applied defaults | The consumer refuses with the cause, and never runs with no defaults | None |
| AC-6 | An external plugin with a defaulted leaf | Behaves as decision D-1 rules, stated in `docs/plugin-development/protocol.md` | Until `spec-plugin-external-yang-at-startup` compiles external YANG, Ze holds no schema for an external root |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Enables two listeners with no ports where both default to one port | file → validate → listener collector → conflict refusal | `test/parse/listener-default-conflict-refused.ci` |
| 2 | Omits a defaulted plugin leaf | file → delivery → plugin sees the default | `test/plugin/yang-default-delivered.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDeliveredConfigCarriesSchemaDefaults` | `internal/component/config/schema_defaults_test.go` | AC-1 | |
| `TestListenerDefaultsComeFromRefine` | `internal/component/config/listener_test.go` | AC-3 | |
| `TestUnsetAcceptRAWritesNoSysctl` | `internal/component/iface/config_sysctl_test.go` | AC-4 | |
| `TestPeerDefaultsRefuseWithoutSchema` | `internal/component/bgp/config/peers_test.go` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `accept-ra` | 0..2 | 2 | N/A | 3 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `listener-default-conflict-refused` | `test/parse/listener-default-conflict-refused.ci` | a default port clash is refused before bind | |
| `yang-default-delivered` | `test/plugin/yang-default-delivered.ci` | an omitted leaf arrives with its default | |

### Interop Tests (Scope: protocol)
N-A: config delivery and listener validation; no protocol wire change.

## Files to Modify
- `internal/component/config/schema_defaults.go`, `internal/component/config/tree.go` - defaults on the delivery path
- `internal/component/config/listener.go`, `internal/component/config/listener_defaults.go` - one collector, schema-derived defaults
- `internal/component/config/cli/cmd_validate.go`, `internal/component/config/graph.go`, `internal/component/bgp/config/loader_create.go` - the one collector
- `internal/component/doctor/checks_listener.go` - the one collector
- `internal/component/iface/tunnel.go`, `internal/component/iface/pppoe_client_timing.go`, `internal/component/sysrib/sysrib.go`, `internal/component/bgp/config/peers.go` - drop per-consumer `ApplyDefaults`
- `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/iface/config_sysctl.go` - `accept-ra`
- `internal/le/config/ports` - delete or repurpose
- `docs/architecture/config/yang-config-design.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/plugin-development/protocol.md`

## Files to Create
- `test/parse/listener-default-conflict-refused.ci`
- `test/plugin/yang-default-delivered.ci`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `accept-ra` default |
| YANG validation constraints | No | Unchanged |
| YANG custom validators | No | None |
| CLI commands/flags | No | None |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | No | Unchanged |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output |
| Env var registration | No | None |
| Doctor check for runtime dependencies | Yes | `checks_listener.go` uses the one collector |
| Doctor code per problem class | No | `config-listener-conflict` and `doctor-listen-unavailable` exist |
| Prometheus counters/metrics | No | None |
| BGP family surface | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` for `accept-ra` |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | the interface guide page that names `accept-ra` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | Depends on D-1 | `docs/plugin-development/protocol.md`, "Stage 2"; `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior changed? | Yes | `rfc/short/rfc4862.md` row for Section 5.5, if the summary carries one |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` section 14; `docs/architecture/doctor-and-health-checks.md` listener defaults |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | Yes | derive with `./le spec citation anchors spec plan/release/fixes/spec-yang-defaults-applied-once.md`; `docs/architecture/config/syntax.md` is declared by `schema_defaults.go` and is updated where it states who applies defaults; `docs/features/interfaces.md` is updated if it names the `accept-ra` default; `docs/features/ai-first.md` is declared by a changed file and is unaffected (no agent-facing surface changes) |
| 17 | Existing docs show examples for this area? | Yes | `ai/doors/config-yang.md` gate row for `./le config ports check` |

## Implementation Steps

1. **Phase: Wiring** - write both `.ci` files; both fail today.
2. **Phase: listener defaults** - carry `refine` defaults in the schema, one collector, delete the table and the second collector (AC-2, AC-3).
3. **Phase: delivery defaults** - apply once on the delivery path for compiled-in plugins; delete the four per-consumer calls; refuse on a schema failure (AC-1, AC-5).
4. **Phase: accept-ra** - correct the declaration (AC-4).
5. **Phase: external plugins** - per D-1 once Thomas decides (AC-6).
6. **Phase: docs** - the pages above.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Correctness | Inheritance by absence preserved; no reload diff from defaults |
| Data flow | One application site; no consumer calls `ApplyDefaults` |
| Rule: no-layering | No second collector, no second default table |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One application site | `grep -rn "ApplyDefaults(" internal` answers only the delivery path and `schema_defaults.go` |
| Table gone | `grep -rn RegisterBuiltinListenerDefaults internal` answers nothing |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Default exposure | A default bind address applied at delivery never widens a listener beyond what the YANG declares |

### Failure Routing

| Failure | Route To |
|---------|----------|
| A consumer changes behavior | Check the leaf's RFC or design; fix the declaration or the consumer |
| Reload diff appears | Apply defaults to both sides before the diff |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `declared-default-applied-by-each-consumer.md` | 2026-09-04 | YANG `default` applied only where a consumer calls `ApplyDefaults` |
| `declared-default-applied-by-each-consumer.md` | 2026-10-30 | interface `ipv6 accept-ra` `default 0` never applied; the device runs the kernel's 1 |
| `gate-excludes-part-of-its-population.md` | 2026-08-23 | listener conflict check never sees listener defaults |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Apply defaults on the delivery path | Keep per-consumer calls and add a gate that checks each consumer calls it | A gate over call sites is a second declaration; one site needs no gate |
| Derive listener defaults from `refine` | Keep the table and `./le config ports check` | The table exists only because the build drops `refine` defaults; fixing the build removes the copy |
| `accept-ra` declares no default that disables RA processing | Apply `default 0` | RFC 4862 Section 5.5: "However, the processing described below MUST be enabled by default." |

### Decisions that are Thomas's

| Decision | Options | What each changes |
|----------|---------|-------------------|
| D-1: who applies a YANG default for an external plugin | 1. Ze fills defaults into the Stage 2 and transaction JSON for every plugin. 2. Ze delivers as today, and an external plugin applies its own defaults from its own module. 3. Ze delivers the configured values plus a separate set of defaults the plugin can tell apart | 1: one behavior for internal and external plugins; the Stage 2 body changes for third-party plugins, and a plugin can no longer tell an omitted leaf from one set to its default, which breaks inheritance-by-absence for any third-party design that uses it. 2: no contract change; internal and external plugins behave differently, and an external author restates every default, the defect this spec removes for internal ones. 3: both facts reach the plugin; a new field in the published contract, documented in `protocol.md`, and the SDK carries the merge |

## Known Limitations
- Rpc input defaults: `plan/release/fixes/spec-config-yang-rpc-mandatory-defaults.md`.
- Defaults under a false `when`: `plan/release/fixes/spec-config-yang-when-unique-choice.md`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run by the main thread: `./le test unit pkg path ./internal/component/config/`, `./le test unit pkg path ./internal/component/iface/`, `./le test unit pkg path ./internal/component/doctor/`, `./le test functional parse`, `./le test functional plugin`, `./le test functional reload`, and the suites `./le repo changed scope print both` reaches. No `./le verify` command
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A: no protocol wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-yang-defaults-applied-once.md` only
