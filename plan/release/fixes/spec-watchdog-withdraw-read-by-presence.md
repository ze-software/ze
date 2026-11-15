# Spec: watchdog-withdraw-read-by-presence

| Field | Value |
|-------|-------|
| Status | done |
| Priority | P1 |
| Release reason | Explicit watchdog withdraw false must announce rather than hold the route. |
| Scope | plugin |
| RFC gaps | - |
| Depends | - |
| Phase | complete |
| Handoff | - |
| Updated | 2026-11-15 |

## Task

Read the watchdog `withdraw` boolean by value. Preserve the existing behavior
when the leaf is omitted: announce when the BGP session reaches Established.
Hold a route only when the operator explicitly sets `withdraw true`.

The owner approved the ExaBGP behavior on 2026-11-15: "ok", in response to
the table stating "Omitted or withdraw false: announce" and "withdraw true:
hold until explicitly announced". The alternative, holding an omitted leaf,
was rejected. This is a correction of an implemented setting, not a new
watchdog mechanism.

The owner authorized a one-off WIP exception on 2026-11-15: "Allow a one-off
exception for these three fixes". It covers this watchdog fix, firewall prefix
validation and BMP receiver binding only. Main may admit those named fixes
with a cap of 15; the standing cap remains unchanged for other work. This
authorization does not replace approval of the complete design. The owner
approved the complete design on 2026-11-15 with "go".

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`
  → Constraint: the watchdog owns its route decisions and reaches the engine through its existing SDK path, in process and forked.
- [ ] `ai/doors/core.md`, `ai/doors/le-tooling.md`, `ai/doors/config-yang.md`
  → Decision: reuse the shared boolean reader and existing test runners.
- [ ] `docs/guide/healthcheck.md`, `docs/architecture/rib-transition.md`
  → Constraint: keep the healthcheck's explicit held-route example and plugin ownership.
- [ ] `docs/architecture/plugin/rib-storage-design.md`
  → Constraint: attachment does not trigger a replay; preserve existing peer-state handling.
- [ ] `docs/architecture/testing/ci-format.md`, `docs/architecture/testing/interop.md`
  → Constraint: preserve the existing compiled-fixture and registered-checker contracts; no runner or protocol format changes.
- [ ] `ai/rules/config.md`, `ai/rules/testing.md`, `ai/rules/commands.md`
  → Constraint: delivered booleans are strings; main runs scoped tests; no whole-tree verification is owed here.

## Current Behavior

The 2026-11-15 audit read the producers below. It ran no tests, builds or
functional scenarios. These are source observations, not passing-test claims.

**Source files read:**
- [ ] `internal/component/bgp/plugins/watchdog/config.go` — initial-state parsing.
- [ ] `internal/component/bgp/plugins/watchdog/config_tx.go` — runtime-state reconciliation.
- [ ] `internal/component/bgp/plugins/watchdog/server.go` — actual peer-up sends.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` — declared boolean and default.

| Producer | Observed behavior |
|----------|-------------------|
| `internal/component/bgp/plugins/watchdog/config.go` `collectWatchdogRoutes` | Tests membership of the withdraw key, so explicit false holds routes too. |
| Same file, `parseNLRIEntries` | Stores the inverse of initiallyWithdrawn as initiallyAnnounced. |
| `internal/component/bgp/plugins/watchdog/server.go` `handleStateUp` | Marks initially announced entries and sends the announced routes through the existing send path. |
| `internal/component/bgp/plugins/watchdog/config_tx.go` `carryEntryState` | Carries runtime announcement state for existing entries; a new entry uses its initial state. |
| `internal/core/configvalue/configvalue.go` `Bool` | Reads native booleans and the string form delivered to plugins. |
| `internal/component/bgp/yang/ze-bgp-conf.yang` `update-block` | The leaf has no default. Its description documents the defect; its help incorrectly says default true. |
| `internal/exabgp/migration/migrate_routes.go` | Writes withdraw true for an ExaBGP route carrying the explicit withdraw flag and otherwise omits the leaf. |

The audit found one weak existing assertion: `TestParseConfigWithdrawFlag`
in `config_test.go` reads the empty per-peer announced map before peer-up,
so it cannot distinguish an initially held entry from an initially announced
one. Add independent assertions of the initial flag and actual peer-up sends;
do not remove or weaken the existing test.

## Existing Mechanisms

| Mechanism | Where | Current role | This design |
|-----------|-------|--------------|-------------|
| Shared scalar coercion | `configvalue.Bool` | Reads delivered strings and native booleans | Reuse in collectWatchdogRoutes; omitted means false. |
| Initial route state | `parseNLRIEntries`, pool entry initiallyAnnounced | Holds the initial announcement choice | Preserve; pass the corrected boolean into it. |
| Startup and reload parsing | `parseConfig`, `parseWatchdogSections` | Read peer and group update blocks for configure and verify | Preserve the same entry points. |
| Runtime state reconciliation | `carryEntryState` | Preserves existing route state on reload | Preserve; a leaf edit does not reset runtime announcement state. |
| Peer-up and commands | `handleStateUp`, `handlePoolActionSingle` | Send initial routes and explicit announce/withdraw operations | Preserve. |
| ExaBGP migration | `migrate_routes.go` | Converts its explicit flag to the Ze boolean | Preserve and exercise existing migration regressions. |

## Design

1. Replace the membership read in collectWatchdogRoutes with the shared boolean reader. An omitted leaf is false.
2. Keep the YANG boolean type and declare its default as false. Correct both the leaf description and help, and the container help that currently says every route is held.
3. Keep the route, pool, command, session-ready and SDK wire formats unchanged.
4. Document the approved default in the healthcheck guide and the watchdog ownership page.
5. Add tests before changing the producer. Keep existing reconnect and reload-state behavior.

The existing schema validation remains responsible for accepting boolean
configuration. The delivered-map reader also refuses a present value that the
shared boolean reader cannot parse, instead of treating it as the omitted
default. This is defense of the existing boolean contract, not a new accepted
input form.

## Data Flow

### Entry Point

The existing configure/verify callbacks receive the BGP ConfigSection JSON.
Its withdraw leaf is a delivered string or a native boolean in controlled
tests. Existing peer-up events and watchdog commands drive route emission.

### Transformation Path

| Stage | Producer and result |
|-------|---------------------|
| Configuration | The shared update-block supplies the watchdog name and withdraw boolean to group and peer update blocks. |
| Delivery | Existing configure and transaction callbacks pass the BGP section to parseConfig. |
| Initial state | collectWatchdogRoutes reads the value; parseNLRIEntries sets initiallyAnnounced. |
| Peer up | handleStateUp sends the entries the pool marks announced. |
| Explicit command | Existing announce/withdraw commands change the pool and send through the SDK. |
| Reload | Existing entries carry runtime state; new entries use the corrected initial value. |

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Config provider → watchdog | Existing BGP ConfigSection and shared scalar reader. | Parser, configure and verify controls PASS. |
| Watchdog → engine | Existing SDK UpdateRoute request, internal or forked. | Both promoted daemon wire proofs PASS. |
| Engine → BGP peer | Existing UPDATE messages selected by the pool state. | Scripted peer and final FRR proof PASS. |

### Changed-Wire Census

**Search evidence:** Read the watchdog config, transaction, server and SDK
wiring files for `withdraw`, `initiallyAnnounced`, `FormatAnnounceCommand`,
`FormatWithdrawCommand`, `UpdateRoute` and the configure/state callbacks.
The producer control is `config.go` parseNLRIEntries: it builds the same
route command strings with `internal/component/bgp/format.go`
FormatAnnounceCommand and FormatWithdrawCommand. The consumer control is
`watchdog.go` runWatchdogPlugin: its unchanged send callback calls
`pkg/plugin/sdk/sdk_engine.go` Plugin.UpdateRoute. The existing UpdateRouteInput
reaches `internal/component/plugin/server/dispatch_registry.go` opUpdateRoute
and the normal command dispatcher. Both promoted daemon tests observe the
existing BGP UPDATE encoding, and the FRR scenario reads it independently.

No wire format changes. The boolean changes whether an existing configured
route command is emitted, through its initial pool announcement state.
The BGP message, attribute and NLRI encoders remain unchanged, as do the
plugin JSON/RPC encodings, method names and fields. The config leaf is already
a boolean at the same path. The route-presence proof therefore validates the
correct emission decision, not a format migration. No writer or reader needs
a migration or compatibility path.

### Changed-Surface Census

| Surface | Writer | Reader | Change |
|---------|--------|--------|--------|
| Config leaf | CLI, saved config, ExaBGP migration | collectWatchdogRoutes | Explicit false now announces. Omission remains announced. |
| YANG default/help | ze-bgp-conf.yang | Schema consumers and operator help | False is the declared and documented default. |
| Initial-state flag | parseNLRIEntries | Pool initialization and peer-up | Its meaning and type stay unchanged. |
| SDK route commands and peer-state events | Existing watchdog callbacks | Existing engine dispatch | No format or registration change. |
| Reload state | carryEntryState | New pool entries | No change to existing entries' runtime state. |
| Ordinary static route ownership | Original config tree | bgp/config hasWatchdogContainer and patchRoutes | Preserved: peer defaults operate on copied resolved maps, while route extraction reads the original tree. No presence-container change is needed. |

### Integration Points

collectWatchdogRoutes reuses configvalue.Bool and passes the result to the
existing initial-state flag. Peer-up, SDK dispatch and carryEntryState remain
the consumers. No new command, callback, registration or wire field is added.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Withdraw omitted or false, as a delivered string or native boolean | Entry is initially announced | No omitted/false entry stays held: its configured initial state is announced. |
| AC-2 | Withdraw true, as a delivered string or native boolean | Entry stays held until an explicit announce command | The held route remains in its pool so a later announce can release it. |
| AC-3 | Group update block or peer update block | AC-1 and AC-2 hold through parseConfig | Existing group/peer merge rules remain because only the leaf-value read changes. |
| AC-4 | Real daemon and a BGP peer, with separate omitted, false and true routes | Peer receives omitted and false routes initially; true route arrives only after announce | Independent announcements may interleave because no global UPDATE-order contract changes. |
| AC-5 | Watchdog runs in process or forked | AC-4 holds in both modes | Neither transport keeps the false-as-held defect: both consume the shared reader. |
| AC-6 | Reload keeps a known route, or adds a new watchdog route | Known route keeps runtime state; new route follows AC-1 or AC-2 | Existing reconciliation semantics are intentional. |
| AC-7 | Schema and operator help | Default is false and every text describes the same omitted/false/true behavior | No conflicting default/help remains because the loaded boolean has one meaning. |
| AC-8 | Existing announce, withdraw, reconnect and migration tests | Their existing behavior remains | No unrelated lifecycle behavior is changed. |
| AC-9 | Present malformed withdraw value at the delivered-map boundary | Parser refuses with context and a corrected withdraw statement | Only omission selects the default; valid true and false remain accepted. |
| AC-10 | Ordinary update with no watchdog container, through schema defaults and actual peer construction | Its route remains an engine static route; a watchdog-owned sibling stays excluded from that set | The resolved map may receive nonpresence defaults; the original route-ownership tree remains unchanged. |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Real daemon loads omitted, false and true watchdog routes, then receives an announce command | → | collectWatchdogRoutes through handleStateUp and existing command dispatch | watchdog-withdraw-value.ci in process and forked |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Location | Proves |
|------|----------|--------|
| TestParseConfigWithdrawValues | New `internal/component/bgp/plugins/watchdog/config_withdraw_test.go` | AC-1 to AC-3; assert initiallyAnnounced directly for both delivery shapes and omission. |
| TestParsedWithdrawValuesControlPeerUp | Same new file | Parsed configuration reaches actual peer-up send decisions, then explicit announce releases true routes. |
| TestNewWithdrawValuesOnReload | Same new file | AC-6: new routes use the corrected value and retained routes keep runtime state. |
| TestReloadWithdrawValueRetainsRuntimeState | Same new file | A command-held route keeps its runtime held state when reload changes its configured initial value to false. |
| TestParseConfigWithdrawRejectsInvalid | Same new file | AC-9: string, empty, numeric and null malformed values return no pools and an operator refusal. |
| TestWatchdogDefaultPreservesOrdinaryStaticRoute, TestWatchdogSchemaDefaultIsFalse | New `internal/component/bgp/config/watchdog_defaults_test.go` | AC-10 through PeersFromConfigTree and the loaded schema. |
| TestWatchdogInitialEORFence | New `internal/le/interoplab/bgp/check_watchdog_withdraw_value_test.go` | Protocol control: FRR received IPv4 EOR; sent-only, missing, false and malformed flags do not pass. It is not the watchdog completion fence. |
| TestWatchdogReadyReceipt, TestWatchdogFirstSession | New `internal/le/interoplab/bgp/check_watchdog_ready_receipt_test.go` | One exact credited-ready record names the watchdog and peer, while first-session/no-drop counters exclude a stale record. Missing or mismatched evidence cannot pass. |
| TestWatchdogWithdrawValueReplaysTheLabRecording | New `internal/le/interoplab/bgp/check_watchdog_replay_test.go` and one real PASS recording under that package's testdata/watchdog-withdraw-value-frr | The registered checker consumes the actual call sequence through credited readiness, quiesce, fresh sentinel, initial absence and explicit announce/withdraw, and reaches the recorded lab verdict. |
| TestWatchdogRouteFailureStopsOperations | New `internal/le/interoplab/bgp/check_watchdog_operation_failure_test.go` | A failed first route operation returns its scenario, assertion index, selected networks and cause text, with no later query or pool command. |
| TestConfigureWithdrawRejectsInvalid, TestVerifyWithdrawRejectsInvalid | Same watchdog value test file | Actual configure refuses before installing pools; invalid verify clears an older candidate, preserves running state and leaves apply unverified. |
| Existing watchdog package | Existing files | AC-8 package regressions. |

### Functional Tests

| Test | Location | Proves |
|------|----------|--------|
| watchdog-withdraw-value, watchdog-withdraw-value-forked | Promoted unchanged to `test/plugin/` after separate named 80-iteration proofs | AC-4 and AC-5 through the daemon and peer, including an ordinary static route control. |
| Existing named watchdog functional tests | Existing plugin and reload files | AC-8 command, reconnect and reload regressions. |

### Interop Tests

| Scenario | Location | Proves |
|----------|----------|--------|
| watchdog-withdraw-value-frr | `test/interop/scenarios/watchdog-withdraw-value-frr/` | FRR observes the same initial route distinction and the later explicit announce. |

The new false case must fail with the presence read and pass with the fix.
The functional and interop proof must also be shown to fail when the producer
fix is absent, with the artifact rebuilt. Main performs these runs. Existing
tests are not edited without the independent test-judge route where required.

The interop checker reads the FRR source at the lab version before relying on
its route output. This spec changes an operator setting rather than claiming
a new RFC requirement, so it adds no RFC conformance tag solely for this fix.
Numeric boundary tests do not apply to a boolean.

## Files to Modify

- `internal/component/bgp/plugins/watchdog/config.go`: read the boolean value.
- `internal/component/bgp/yang/ze-bgp-conf.yang`: false default and accurate help/description.
- `docs/guide/healthcheck.md`: explain the default beside the held-route example.
- `docs/architecture/rib-transition.md`: record the initial-state behavior of watchdog-owned routes.
- `internal/test/fixture/constants.go`: shared watchdog announce command beside the existing withdraw command; preserve unrelated dirty hunks.
- `internal/test/fixture/misc_fixture_observers.go`, `internal/test/fixture/plugin_fixture_11.go`: replace only the identical announce-command literals with the shared constant.
- `test/reload/bgp-watchdog-pool-reload.ci`: correct its stale presence-read comment without changing assertions.

## Files to Create

- `internal/component/bgp/plugins/watchdog/config_withdraw_test.go`.
- `internal/component/bgp/config/watchdog_defaults_test.go`.
- `internal/test/fixture/watchdog_withdraw_value.go`: state-based driver; the scripted peer judges wire messages.
- `test/plugin/watchdog-withdraw-value.ci`, after the draft proof.
- `test/plugin/watchdog-withdraw-value-forked.ci`, after the draft proof.
- `test/interop/scenarios/watchdog-withdraw-value-frr/ze.conf`.
- `test/interop/scenarios/watchdog-withdraw-value-frr/frr.conf`.
- `internal/le/interoplab/bgp/register_watchdog_withdraw_value.go`: one bespoke checker registration from its own file; no generic fallback.
- `internal/le/interoplab/bgp/check_watchdog_withdraw_value.go`: credited-completion and fresh-sentinel causal fence, plus initial-EOR protocol control, before the existing scenario operations.
- `internal/le/interoplab/bgp/check_watchdog_withdraw_value_test.go`: fail-closed received-EOR predicate controls.
- `internal/le/interoplab/bgp/check_watchdog_ready_receipt_test.go`: exact producer/peer receipt and current-session scope controls.
- `internal/le/interoplab/bgp/check_watchdog_replay_test.go`: whole-checker replay of the real passing FRR recording.
- `internal/le/interoplab/bgp/check_watchdog_operation_failure_test.go`: first-operation failure keeps its diagnostic context and stops later operations.
- `internal/le/interoplab/bgp/testdata/watchdog-withdraw-value-frr/watchdog-withdraw-value-frr-YQ2OFFHKRDG5YT26PAVQVNAIHB.jsonl`: unmodified passing lab recording, reviewed for secrets before retention.
- `website/changes/fragments/watchdog-withdraw-read-by-presence.md`: operator release note.

## Risks & Assumptions

| Risk or constraint | Control |
|--------------------|---------|
| Explicit false begins announcing where it was wrongly held | State the change in help and docs; prove its precise route set. |
| A test inspects an uninitialized map and passes vacuously | Assert the initial flag and actual peer-up sends. |
| A fresh pool replaces an existing runtime decision | Preserve carryEntryState and cover retained entries on reload. |
| Only the internal plugin path works | Functional proof includes a forked watchdog. |
| A new default affects ordinary route ownership | Actual peer-construction test and an ordinary static control in both functional modes and FRR. The source pipeline reads routes from the original tree, not its default-filled copy. |
| Test runners or unrelated sessions change the checkout | Main serializes runs, uses a named build, preserves unrelated changes and owns cleanup. |

No new command, event, metric, listener, runtime dependency or persistence
surface is introduced. Existing registration and compile-out behavior remain.
PPP work is excluded and its specs remain unchanged.

## Critical Review Checklist

| Check | Required evidence |
|-------|-------------------|
| Boolean value | Omitted and false announce; true holds, through both delivery forms. |
| Runtime state | Retained entries preserve their runtime state; new entries use the configured initial state. |
| Wiring | Real daemon tests cover internal and forked watchdogs, with observed route announcements. |
| Documentation | YANG help/default and both owning pages agree with the approved behavior. |
| Scope | No unrelated lifecycle, route command, SDK wire or PPP behavior changes. |

## Implementation Steps

1. After owner approval of this design, author the new tests and functional draft. Main observes the explicit false failure against the existing producer.
2. Correct the boolean read and YANG text/default; update the two owning pages in the same work.
3. Main runs the scoped unit and functional tests, records discrimination and runs the named FRR scenario.
4. Obtain independent review of the whole change and spec requirements; fix findings, then close through the two-commit workflow.

## Checklist

- [x] Tests written: value, callback, default, ownership and checker controls.
- [x] Tests FAIL: initial false-value units, both daemon presence cuts and the actual FRR true-leak cut.
- [x] Tests PASS: restored units, promoted daemon cases, named stress selections and final FRR proof.
- [x] Main completed the named-build form of `./le test bgp plugin --pattern watchdog-withdraw-value` and the scoped selections recorded below.
- [x] Wiring 6c7093ce15 landed and the independent native review pin/check are CLEAN. Main executes the prepared two-commit closure script.

## Deliverables Checklist

| Deliverable | Required evidence | Current state |
|-------------|-------------------|---------------|
| Boolean value and omitted default | Focused watchdog package tests, including false/native values and callbacks | Watchdog package PASS in job-unit-pkg-721fa93f; configure/verify additions PASS in job-unit-pkg-cfef2b72. The earlier combined job had unrelated BMP failures, so this is a watchdog-package result only. |
| Loaded schema and ordinary route ownership | Default/static-route controls through actual peer construction | PASS in job-unit-pkg-acbbcc88. |
| Real daemon, internal and forked | Both watchdog value CI tests and their wire expectations | Baseline and restored 2/2 PASS; presence cut made both RED with explicit false absent. Each exact-name stress run has 80 exit-0 invocations and 160 named PASS receipts, with zero failures. Both tests promoted unchanged. Final promoted run PASS 2/2 in 1.6 and 1.7 seconds. |
| Foreign receiver | Named FRR scenario with credited completion, queue drain, fresh received sentinel and route controls | Baseline 1/1 PASS in 52 seconds; true-leak cut failed held-route absence after the sentinel. Final restored run PASS 1/1 in 51 seconds, with zero failures and no resources left behind. |
| Checker outcomes | Single registered owner, complete real-recording replay and first-operation failure | Focused native race selection PASS. Selected count profiles execute all 14 route operations and the local failure return. The failure unit has an explicit named PASS receipt. |
| Documentation | YANG help/default, healthcheck guide and route-ownership page agree | Committed with the producer in eb084a4b99; independent source review CLEAN. The 16:14 doc-check run validates all 454 command nodes and help shape. Its only four source-anchor failures name foreign IRR pages. Watchdog source and schema have no later edit. |
| Closure | Independent clean review artifact and two closure commits | Native artifact CLEAN at round 5 over 25 paths. Immutable-tree check CLEAN over all 17 code-bearing paths. Main owns the final A+B publication and claim release. |

## Security Review Checklist

| Check | Required evidence |
|-------|-------------------|
| Invalid delivered value cannot announce by default | PASS: parser, configure and verify controls preserve the running pool and leave apply unverified after refusal. |
| True route is held while false and ordinary routes work | PASS: actual true-leak cut failed after credited completion, queue drain and fresh sentinel receipt. Restored FRR run PASS 1/1, including all positive routes and explicit release/withdrawal. |
| Cross-process behavior | PASS: final promoted internal and forked daemon tests see the same wire routes. |
| Error disclosure | Refusal names the peer, watchdog and malformed boolean value; no credential or private key is read. |
| Existing trust boundaries | No new operator command, SDK wire field, privilege, production listener or persistence surface. |

## Documentation Update Checklist

| Surface | Applies | Owning change or reason |
|---------|---------|-------------------------|
| Config default and help | Yes | ze-bgp-conf.yang default false and matching leaf/container text. |
| Operator guidance | Yes | docs/guide/healthcheck.md explains explicit hold and omitted/false announcement. |
| Architecture and owner decision | Yes | docs/architecture/rib-transition.md records watchdog ownership, default and retained runtime state. |
| Operator release note | Yes | website/changes/fragments/watchdog-withdraw-read-by-presence.md describes the false-value correction and preserved reload state. |
| Existing reload explanation | Yes | test/reload/bgp-watchdog-pool-reload.ci now explains the false default; only the stale comment changed. |
| Command grammar and RPC format | No | Existing commands and SDK route/state formats are unchanged. |
| Functional test conventions | No | Existing CI/compiled observer mechanism; both new tests follow ze-test drafting and promotion. |
| RFC conformance claims | No | No new RFC tag or published support claim. Test-only GR enables FRR's received-EOR observation. |
| Runtime dependencies and doctor | No | No new production dependency; the existing GR plugin is used only in the lab config. |
| Feature declaration and comparison | No | Existing watchdog plugin, registration, feature scope and maturity remain. `features/plugins.md` describes the existing plugin surface; no watchdog-default claim in `docs/comparison.md` needs correction. |
| Config syntax | No | Existing boolean leaf and paths remain. `docs/architecture/config/syntax.md` uses explicit `withdraw true` in its held-route example, which stays correct. |
| SDK, wire format, route metadata and counters | No | The diff changes none of these contracts. Source reads confirm the same configure/state/route-command path and existing pool state. |
| Agent discovery and test infrastructure | No | The scenario and fixture use the existing registries, runner and replay mechanism documented by `docs/architecture/testing/ci-format.md` and `docs/architecture/testing/interop.md`. No tool, action, gate or workflow is added. |

## Review Gate

### Round 1 — independent review

Scope: the watchdog boolean producer, schema, unit and callback coverage,
ordinary-route control, both daemon drafts and fixture, FRR configs/checker,
and owning documentation. Main commissioned this review in a context that did
not author the changes.

| Finding | Severity | Author response | Verification state |
|---------|----------|-----------------|--------------------|
| Initial FRR absence was sampled before a received-initial-EOR fence | ISSUE | Added a strict typed FRR neighbor fence and explicit test-only GR capability/attachment, with missing/false/malformed controls. Round 2 replaced the insufficient completion assumption below. | Predicate tests PASS. The repaired causal fence detected the actual true-leak cut. Final restored FRR run PASS 1/1 in 51 seconds. |
| Invalid values were tested only through the parser, not configure/verify | ISSUE | Added separate configure and verify tests, including rejection after an older candidate and preservation of running state/apply-unverified. | Focused callback selection PASS in job-unit-pkg-cfef2b72. |

Round 2 scope: only the two R1 fixes and their affected sibling paths, plus
always-in-scope correctness findings. A fresh reviewer must judge this round;
the author does not claim a clean Review Gate.

### Round 2 — independent review and causal-fence repair

The independent R2 reviewer found that a received EOR cannot fence watchdog
startup: SignalsSessionReady deliberately does not hold Ze's marker. Positive
route reads and the marker therefore leave a later held-pool leak unobserved.
The repaired scenario, actual leak discrimination and final restored run now
resolve this finding.

The author repair changes only the checker, its own registration, scenario
logging and new predicate controls. It waits for the exact bgp.routes Debug
record from Peer.creditAPIReady naming the FRR peer and bgp-watchdog. That
producer validates the receipt against the current live session, and watchdog
handleStateUp reports only after submitting every pool's sends. First-session
and zero-drop checks reject a receipt from an earlier connection.

After receipt, request quiesce drains forward workers and initial-sync queued
writes. The checker then injects a fresh, unconfigured 10.77.4.0/24 route and
waits until FRR receives it on that connection before reading held absence.
The sentinel is not in a watchdog pool or the startup config. Initial EOR stays
as a protocol control only. New controls reject missing, wrong-peer,
wrong-process, wrong-logger, noncredited and fragmented records, and stale or
unmeasured session counters. No production API or readiness behavior changes.

Round 3 covered these R2 fixes and their reached sibling paths. The actual
announce-true mutation failed after the sentinel, then the restored run
passed. No elapsed-time wait substitutes for that evidence.

### Round 3 — independent source review

Scope: the R2 credited-ready receipt parser, first-session controls, queue
drain and fresh-sentinel ordering, plus the reached checker and scenario
paths. Main reported independent source review CLEAN. Focused receipt/session
controls PASS in job-unit-pkg-2bb8d21f; received-EOR controls PASS in
job-unit-pkg-cfef2b72. The repaired FRR baseline passed, but source review and
a baseline pass alone do not establish discrimination. Main observed both
required RED results with rebuilt artifacts and restored the producer hash
after each cut. Restored internal and forked daemon tests PASS 2/2, and their
final promoted run also PASS 2/2. Final restored FRR PASS 1/1 in 51 seconds
completes the execution proof. Native closure landing remains separate.

### Round 4 scope — single checker ownership repair

Main's full checker-package run found one failure: the existing population
test refuses a scenario registered in both specialCheckers and
scenarioOperations. The watchdog registration did both, and the bespoke
checker called the generic checkScenario path for its route operations. The
owning defect is the scenario's registration and orchestration. Exempting it
from the population test would leave the dual ownership and weaken the guard.

The source repair keeps the bespoke registration and moves the unchanged
14-operation sequence into checkWatchdogWithdrawValueRoutes. It runs the
same runOperation calls with the same assertion numbering, selected-network
context and checkerFailure wrapping. The causal fence, recorded calls,
population test and real recording remain unchanged. The fresh independent
review covers only this ownership repair and its reached sibling paths.
Main reported the independent repair review CLEAN: one bespoke owner,
identical operations and error indices/context, and an unchanged fixture.
Focused population and watchdog predicate/replay tests PASS in
job-unit-pkg-78e93431 under the native race package action. The selected
count/profile run also PASS in job-watchdog-checker-coverage-a893ef81 and
executes every local route operation. The later
TestWatchdogRouteFailureStopsOperations count run PASS in
job-watchdog-route-failure-count-a30c737c, with one execution of the local
failure return. Independent source review of that new unit is CLEAN. Its
assertion preserves the cancellation cause text, not typed error identity:
the existing Wait helper constructs a new diagnostic error before this
checker wraps it. These scoped results do not relabel the earlier
full-package RED as a passing run.

### Round 5 scope — final closure review

The closure context is independent of the author. Its final pass covers the
R4 ownership repair, the later mechanical lint edits, the new first-operation
failure unit and their reached sibling paths. The closure gate also checks
the complete seven-file core commit and the daemon, interop, recording and
documentation population. Its lenses are correctness and wiring, security
and failure paths, and proof completeness and Go style. The final restored
FRR run passed without a source change. This pass found zero BLOCKER and zero
ISSUE. Wiring and the claim handoff are complete. Main recorded the native
hash pin and checked all 17 code-bearing paths against the committed tree.
A record correction is a NOTE and does not open another product review round.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/watchdog-withdraw-read-by-presence-01a1df36-384e-7873-a487-85e51b996435.md`, CLEAN, 25 paths. |
| Native review check | `./le commit check` with that artifact and immutable tree 61a5885e1497d49672a7f23231a102d017fd2598: clean true, verdict clean, 17 code files. The historical YANG is pinned to its committed blob and mode, preserving the foreign working-tree hunk. |
| Rounds | 5. No sixth product pass is authorized or needed. |
| Reviewer lenses used | Correctness and wiring; security and failure paths; proof completeness and Go style. |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | Initial absence could precede the watchdog's completed contribution. | `checkWatchdogWithdrawValueFRR` | Exact credited receipt, current-session controls, both drains and a fresh received sentinel; actual true-leak RED followed by restored FRR PASS. |
| 2 | ISSUE | Invalid delivery had parser-only proof. | `watchdogConfigTx.configure`, `watchdogConfigTx.verify` | Separate callback controls preserve the running state and clear an earlier candidate; selected tests PASS. |
| 3 | ISSUE | One scenario had both bespoke and generic owners. | `register_watchdog_withdraw_value.go` | One bespoke registration and identical local route operations; population, replay and error-return controls PASS. |

## Scoped Verification Results

| Command or selected proof | Purpose and observed result |
|---------------------------|-----------------------------|
| `./le --name session test unit pkg path ./internal/component/bgp/plugins/watchdog/` | Boolean and existing watchdog regressions PASS. Whole-producer count coverage is 82.1%, with every added producer outcome executed. |
| Actual schema/default and ordinary-route builder selection | PASS in job-unit-pkg-acbbcc88 through PeersFromConfigTree and the loaded YANG schema. |
| Internal and forked daemon drafts | Baseline and restored PASS 2/2. The presence-read cut made both RED with explicit false absent. |
| `./le --name session test bgp plugin --pattern watchdog-withdraw-value` | Final promoted daemon/peer tests PASS 2/2 in 1.6 and 1.7 seconds. |
| Exact-name draft stress selections | Internal and forked each PASS 80 invocations, parallel 8 and burners 8, with 160 named PASS receipts and zero failures per capture. Promotion kept both hashes unchanged. |
| `./le --name session test qemu docker-lab lab 'test integration interop' env 'INTEROP_SCENARIO=watchdog-withdraw-value-frr'` | Baseline PASS and true-leak cut RED after credited readiness, queue drain and sentinel receipt. Final restored run PASS 1/1 in 51 seconds, with check/command/lab exit 0 and no left-behind resources. |
| Focused population, predicates and recording replay | Native race selection PASS in job-unit-pkg-78e93431. The complete earlier checker-package run had only the now-repaired population failure. |
| `./le --name session test unit pkg path ./internal/le/interoplab/bgp/ run '^TestWatchdogRouteFailureStopsOperations$'` | Native race selection PASS in job-unit-pkg-a33fee18. Separate verbose count selection names the test as PASS and executes the local failure return once. |
| Existing watchdog compatibility, pool reload and migration selections | Four legacy watchdog plugin cases PASS, existing bgp-watchdog-pool-reload PASS 1/1, and TestMigrateWatchdogWithdrawStartsWithdrawn PASS. |
| `./le --name session config coercion check` | PASS, exit 0: `config-string-coercion: OK`. Actual output is job-watchdog-config-coercion-check-f604e763.log in session 2026-11-15-130223. |
| `./le --name session doc yang-contract help-shape` | The completed doc-check-all run includes this action. Every command and config node declares its summary and long help. No watchdog finding. |
| `./le --name session doc check all` | Actual retained output is job-firewall-closure-doc-check-f119ecea.log in session 2026-11-15-130223. Overall RED on four IRR source anchors, with no watchdog failure. No later watchdog code, schema or guide edit stales that result. |

Main owns all execution. Completed selections are not rerun for freshness.
No further test or contract check is owed. Wiring and the independent native
pin/check are complete. Main publishes the prepared A+B closure script and
releases the claim. No whole-tree verification is part of this closure.

## Current Audit and Review State

| Item | State |
|------|-------|
| Source audit | Completed 2026-11-15; observations above. |
| Owner default decision | Approved: match ExaBGP. |
| Complete design approval | Approved by the owner on 2026-11-15: "go". |
| Product, test and documentation edits | Core committed in eb084a4b99 and wiring in 6c7093ce15. Both producer cuts were restored to SHA-256 a36965235495ba086a437f4b59e64bcbcfdfadab8e539376aa5325cf94bc309c. All scoped proof is complete and source stays frozen. |
| Unit execution | Main observed explicit-false RED before the fix in job-unit-pkg-46a814a3. Watchdog package PASS in 721fa93f; schema/static controls PASS in acbbcc88; callback/EOR controls PASS in cfef2b72; receipt/session controls PASS in 2bb8d21f. |
| Real daemon execution | Baseline and restored 2/2 PASS. Presence cut made both RED: ordinary and omitted routes plus EOR arrived; explicit false was missing. Separate internal and forked stress runs each have 80 exit-0 invocations and 160 named PASS receipts, zero failures. Tests promoted unchanged, then final promoted selection PASS 2/2. The earlier combined capture is not counted as proof. |
| Retained checker replay | PASS in the focused native race run job-unit-pkg-78e93431 over the 34-call real passing FRR recording. Selected count/profile PASS in job-watchdog-checker-coverage-a893ef81. The canonical fixture is byte-identical to the original, SHA-256 65f0564dbb1cdbd6fd5fc01219bf73e52a1135043ae959b120bb47598dca6096. |
| Checker ownership repair | Native race focused population, predicates and full recording replay PASS in job-unit-pkg-78e93431. Independent focused repair review CLEAN. Count profiles execute all 14 operations and the local first-operation failure return. The new failure unit also passed independent review. No further Go edit is planned. |
| FRR execution | Baseline 1/1 PASS in 52 seconds. True-leak cut RED at assertion 5: held 10.77.3.0/24 was present after credited readiness, quiesce and fresh 10.77.4.0/24 receipt; positive routes remained present. Final restored run PASS 1/1 in 51 seconds. Check, command and lab exit 0, interrupted false, VM stopped and left-behind none. |
| Independent implementation review | R3 source CLEAN and focused ownership/lint/failure-unit reviews CLEAN. Final closure review found zero BLOCKER and zero ISSUE across correctness, proof, security and style. Native artifact CLEAN at round 5, with the full 17-code-file immutable-tree check CLEAN. |
| Light closure checks | repo check ran and reported four findings outside this spec: RTRSession cross-package use and three IRR number-parse counts. The later weakened-check attempt stopped before its action because the named launcher could not read two shared Go-cache entries. The earlier actual BMP-closure weakened-check over 3dead228c222 through the worktree reports zero deleted and five weakened foreign RFC tests, with no watchdog finding. Watchdog has only new units and its committed comment correction since that comparison. |

The interop registry registers only the bespoke checker. Its own local sequence
reuses committed operation kinds, runOperation, zeCommand and the existing
FRR presence/absence readers. FRR 10.7.1
bgp_show_route and bgp_show_table were read at the pinned upstream source;
the local FRR checkout is 10.5.3 and was not used as evidence for the lab pin.
Both tests are promoted unchanged to test/plugin after separate named stress
proofs; no draft copies remain. No task-owned local server was started.

## Implementation Summary

### What Was Implemented

The watchdog reads the shared boolean value, with false only for omission.
Malformed present values return an operator refusal. The schema declares the
same false default. Existing route ownership and runtime reload state remain.

The daemon proofs cover internal and forked plugins. The FRR checker waits
for credited watchdog completion, drains queued writes and observes a fresh
sentinel before it judges held-route absence. Its real recording replay and
first-operation error control cover the complete checker path.

### Bugs Found/Fixed

Explicit false no longer selects the held state. The configure/verify tests
also prove malformed delivery cannot install pools or reuse an old candidate.
The checker has one owner, and its absence proof detects a deliberate leak.

### Documentation Updates

Commit eb084a4b99 updates the YANG leaf, `docs/guide/healthcheck.md` and
`docs/architecture/rib-transition.md`, with source anchors to
collectWatchdogRoutes, parseNLRIEntries, handleStateUp and carryEntryState.
The existing reload test has only its stale explanatory comment corrected.
The operator fragment records the false-value correction and preserved
reload state. The completed doc check validates command/help contracts and
reports four foreign IRR source-anchor defects, with none in these pages.

### Deviations from Plan

The FRR negative proof needed credited completion and a fresh remote sentinel,
because initial EOR does not wait for SignalsSessionReady. The checker uses
one bespoke registration and a local operation sequence, preserving the
shared population guard. No production readiness or API contract changed.
The plan gained explicit callback and checker-failure controls. No acceptance
criterion was removed or reduced.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Initial EOR was treated as watchdog completion. | Session-ready reports deliberately do not hold EOR. | Independent R2 source review. | Use credited completion, both queue drains and a fresh received sentinel. The actual true-leak cut now fails. |
| approach | The bespoke scenario also had a generic fallback. | The checker population requires one owner. | TestCheckerPopulationMatchesProducer failed the full package run. | Keep only the bespoke registration and preserve all 14 operations. Focused population/replay/error controls pass. |
| approach | The first combined stress capture kept only discovery warnings. | Exit 0 without named receipts is insufficient evidence. | Main read the retained capture. | Reject that capture as proof and use separate exact-name 80-invocation runs with retained named PASS receipts. |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Read the boolean value and preserve the announced omitted default. | Done | `config.go` collectWatchdogRoutes, `ze-bgp-conf.yang` update-block | False/omitted announce, true holds, malformed present values refuse. |
| Preserve runtime state, ownership and existing commands. | Done | carryEntryState, actual peer builder, existing command handlers | New routes use initial state; retained routes keep runtime state. |
| Prove the behavior through both daemon transports and FRR. | Done | Both promoted CI files and checkWatchdogWithdrawValueFRR | Final daemon PASS 2/2, FRR PASS 1/1, with actual discriminating RED runs. |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | TestParseConfigWithdrawValues, both daemon tests and final FRR | String/native false and omission announce. |
| AC-2 | Done | TestParsedWithdrawValuesControlPeerUp, final FRR | True remains held until announce; the actual true-leak cut fails. |
| AC-3 | Done | TestParseConfigWithdrawValues | Both peer and group deliveries exercise every initial value. |
| AC-4 | Done | watchdog-withdraw-value and final FRR | Actual peer reads the exact ordinary, omitted, false and held route sets. |
| AC-5 | Done | watchdog-withdraw-value-forked | Same wire proof through the external SDK path. |
| AC-6 | Done | TestNewWithdrawValuesOnReload, TestReloadWithdrawValueRetainsRuntimeState | New false route announces; retained command-held route stays held. |
| AC-7 | Done | TestWatchdogSchemaDefaultIsFalse, completed help/doc check | Loaded default and documented meaning agree. |
| AC-8 | Done | Existing package, four plugin regressions, pool reload and migration selections | Actual selected runs pass, with no assertion change. |
| AC-9 | Done | Parser/configure/verify malformed-value controls | Refusal returns no candidate/pools and preserves running state. |
| AC-10 | Done | TestWatchdogDefaultPreservesOrdinaryStaticRoute, both CIs and final FRR | The ordinary route remains present beside watchdog-owned routes. |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestParseConfigWithdrawValues | Done | watchdog/config_withdraw_test.go | Initial false cases failed before the fix, then passed. |
| TestParseConfigWithdrawRejectsInvalid | Done | Same file | Malformed string, empty, number and null refuse. |
| TestConfigureWithdrawRejectsInvalid | Done | Same file | Actual configure leaves pools empty. |
| TestVerifyWithdrawRejectsInvalid | Done | Same file | Invalid verify clears the old candidate and preserves running state. |
| TestParsedWithdrawValuesControlPeerUp | Done | Same file | Actual sends and later explicit release pass. |
| TestNewWithdrawValuesOnReload | Done | Same file | New false route announces without replay of the retained route. |
| TestReloadWithdrawValueRetainsRuntimeState | Done | Same file | Runtime-held state survives an initial-value edit. |
| TestWatchdogDefaultPreservesOrdinaryStaticRoute | Done | bgp/config/watchdog_defaults_test.go | Actual peer builder preserves ownership. |
| TestWatchdogSchemaDefaultIsFalse | Done | Same file | Loaded schema default is false. |
| TestWatchdogInitialEORFence | Done | check_watchdog_withdraw_value_test.go | Received flag required; sent-only, missing and malformed controls refuse. |
| TestWatchdogReadyReceipt | Done | check_watchdog_ready_receipt_test.go | Exact complete credited record required. |
| TestWatchdogFirstSession | Done | Same file | Missing/stale session counters refuse. |
| TestWatchdogWithdrawValueReplaysTheLabRecording | Done | check_watchdog_replay_test.go | Complete registered checker replays the unchanged real recording. |
| TestWatchdogRouteFailureStopsOperations | Done | check_watchdog_operation_failure_test.go | Named PASS, with one count-profile hit on the local failure return. |
| watchdog-withdraw-value | Done | test/plugin/watchdog-withdraw-value.ci | Actual presence-cut RED, restored/promoted PASS and 80 named stress passes. |
| watchdog-withdraw-value-forked | Done | test/plugin/watchdog-withdraw-value-forked.ci | Same proof through the external plugin transport. |
| watchdog-withdraw-value-frr | Done | test/interop/scenarios/watchdog-withdraw-value-frr | Actual true-leak RED and final restored PASS 1/1 in 51 seconds. |
| Existing watchdog package | Done | internal/component/bgp/plugins/watchdog | Complete package PASS and 82.1% count coverage. |
| Existing compatibility and migration selections | Done | Four watchdog plugin tests, pool reload and TestMigrateWatchdogWithdrawStartsWithdrawn | All selected cases PASS. |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| internal/component/bgp/plugins/watchdog/config.go | Done | Shared Bool reader and malformed-value refusal. |
| internal/component/bgp/yang/ze-bgp-conf.yang | Done | False default and matching description/help in eb084a4b99. |
| docs/guide/healthcheck.md | Done | Explicit hold and false/omitted behavior. |
| docs/architecture/rib-transition.md | Done | Ownership, approved default and retained reload state. |
| internal/test/fixture/constants.go | Done | Shared announce command; benign foreign literal constants retained. |
| internal/test/fixture/misc_fixture_observers.go | Done | Same literal command through the shared constant. |
| internal/test/fixture/plugin_fixture_11.go | Done | Same literal command through the shared constant. |
| test/reload/bgp-watchdog-pool-reload.ci | Done | Stale comment corrected; assertions unchanged. |
| internal/component/bgp/plugins/watchdog/config_withdraw_test.go | Done | Initial values, callbacks, peer-up and reload controls. |
| internal/component/bgp/config/watchdog_defaults_test.go | Done | Actual ownership/default controls. |
| internal/test/fixture/watchdog_withdraw_value.go | Done | State-based daemon driver. |
| test/plugin/watchdog-withdraw-value.ci | Done | Internal wire proof, promoted unchanged. |
| test/plugin/watchdog-withdraw-value-forked.ci | Done | Forked wire proof, promoted unchanged. |
| test/interop/scenarios/watchdog-withdraw-value-frr/ze.conf | Done | Four controls and test-only GR/log evidence. |
| test/interop/scenarios/watchdog-withdraw-value-frr/frr.conf | Done | Independent FRR receiver. |
| internal/le/interoplab/bgp/register_watchdog_withdraw_value.go | Done | One bespoke checker registration. |
| internal/le/interoplab/bgp/check_watchdog_withdraw_value.go | Done | Causal fence and local operations with diagnostic wrapping. |
| internal/le/interoplab/bgp/check_watchdog_withdraw_value_test.go | Done | Received-EOR predicate controls. |
| internal/le/interoplab/bgp/check_watchdog_ready_receipt_test.go | Done | Credited receipt and session-scope controls. |
| internal/le/interoplab/bgp/check_watchdog_replay_test.go | Done | Complete real-recording replay. |
| internal/le/interoplab/bgp/check_watchdog_operation_failure_test.go | Done | First operation fails closed and stops later work. |
| internal/le/interoplab/bgp/testdata/watchdog-withdraw-value-frr/watchdog-withdraw-value-frr-YQ2OFFHKRDG5YT26PAVQVNAIHB.jsonl | Done | Unmodified canonical regression fixture, reviewed for secrets. |
| website/changes/fragments/watchdog-withdraw-read-by-presence.md | Done | Operator-visible before/after and preserved reload state. |

### Audit Summary

55 items are Done: three Task requirements, ten acceptance criteria,
19 planned proof rows and 23 planned files. No Partial or Skipped item remains.
The proof refinements are recorded in Deviations from Plan and the Mistake Log.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Omitted/false announce and true holds until explicitly released. | Functional and interoperability | Final promoted daemon tests PASS 2/2. Final restored FRR PASS 1/1 in 51 seconds. Presence cut removes explicit false and makes both daemon tests RED; true-leak cut makes FRR assertion 5 RED after the sentinel. |
| Preserve runtime state and ordinary route ownership. | Actual builder, reload and wire controls | TestWatchdogDefaultPreservesOrdinaryStaticRoute, TestNewWithdrawValuesOnReload and TestReloadWithdrawValueRetainsRuntimeState PASS. Existing pool reload PASS 1/1. Ordinary route is observed in both CIs and FRR. |
| Reject malformed delivery and document the accepted default. | Callback, schema and documentation contracts | Parser/configure/verify controls PASS. Loaded-schema default test PASS. Coercion check emits config-string-coercion: OK. Help contracts pass; only four foreign IRR anchors fail the doc check. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every in-scope behavior and scoped proof is complete. | — |

PPP is excluded by the owner's instruction, not an unfinished item of this
spec. No PPP file or spec was edited by this work. Shared lint, repo and
documentation findings outside this population remain explicitly recorded
above and are not reported as green checks.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| internal/component/bgp/plugins/watchdog/config_withdraw_test.go | Yes | Current ls output names the file. |
| internal/component/bgp/config/watchdog_defaults_test.go | Yes | Current ls output names the file. |
| internal/test/fixture/watchdog_withdraw_value.go | Yes | Current ls output names the file. |
| test/plugin/watchdog-withdraw-value.ci | Yes | Current ls output names the promoted file. |
| test/plugin/watchdog-withdraw-value-forked.ci | Yes | Current ls output names the promoted file. |
| test/interop/scenarios/watchdog-withdraw-value-frr/ze.conf | Yes | Current ls output names the file. |
| test/interop/scenarios/watchdog-withdraw-value-frr/frr.conf | Yes | Current ls output names the file. |
| internal/le/interoplab/bgp/register_watchdog_withdraw_value.go | Yes | Current ls output names the file. |
| internal/le/interoplab/bgp/check_watchdog_withdraw_value.go | Yes | Current ls output names the file. |
| internal/le/interoplab/bgp/check_watchdog_withdraw_value_test.go | Yes | Current ls output names the file. |
| internal/le/interoplab/bgp/check_watchdog_ready_receipt_test.go | Yes | Current ls output names the file. |
| internal/le/interoplab/bgp/check_watchdog_replay_test.go | Yes | Current ls output names the file. |
| internal/le/interoplab/bgp/check_watchdog_operation_failure_test.go | Yes | Current ls output names the file. |
| internal/le/interoplab/bgp/testdata/watchdog-withdraw-value-frr/watchdog-withdraw-value-frr-YQ2OFFHKRDG5YT26PAVQVNAIHB.jsonl | Yes | Current ls output names the canonical fixture; SHA-256 stays 65f0564dbb1cdbd6fd5fc01219bf73e52a1135043ae959b120bb47598dca6096. |
| website/changes/fragments/watchdog-withdraw-read-by-presence.md | Yes | Current ls output names the file, with all six required headings. |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | False and omission announce. | Actual main-thread package, final promoted daemon and final FRR PASS receipts. Producer hash remains a36965235495ba086a437f4b59e64bcbcfdfadab8e539376aa5325cf94bc309c. |
| AC-2 | True stays held until announce. | Actual true-leak FRR RED at assertion 5; final restored FRR PASS 1/1. |
| AC-3 | Both delivered scopes work. | TestParseConfigWithdrawValues covers group and peer values under the passing package run. |
| AC-4 | Actual peer sees the required route set. | Both CI bodies have exact scoped NLRI assertions; final selected run PASS 2/2. |
| AC-5 | Forked plugin behaves the same. | Forked CI explicitly launches ze plugin bgp-watchdog; final run PASS. |
| AC-6 | Reload keeps runtime state and initializes new routes. | Both dedicated reload units and the existing live pool reload PASS. |
| AC-7 | Schema/help agree. | Current leaf has default false; loaded-schema test and completed help contracts PASS. |
| AC-8 | Existing behavior remains. | Complete watchdog package, four legacy daemon cases, pool reload and migration selections PASS. |
| AC-9 | Invalid present value refuses. | Current producer branches on configvalue.Bool validity; parser and actual configure/verify controls PASS. |
| AC-10 | Ordinary route ownership survives defaults. | Actual peer-builder control PASS; both CIs and final FRR observe the ordinary prefix. |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Real daemon loads omitted, false and true routes and receives announce/withdraw commands. | test/plugin/watchdog-withdraw-value.ci | Read the full config, fixture registration and scoped wire assertions. Final promoted PASS. |
| The same path runs through a forked watchdog. | test/plugin/watchdog-withdraw-value-forked.ci | Read the explicit external plugin declaration and identical wire assertions. Final promoted PASS. |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| Delivered boolean shape | Confirmed | Shared Bool reads strings/native booleans; both forms and omission pass the group/peer controls. |
| Ordinary route ownership under schema defaults | Confirmed | Current builder defaults a copied resolved map and extracts routes from the original tree. Actual builder and wire controls pass. |
| Initial EOR proves watchdog completion | Broken and repaired | SignalsSessionReady does not hold EOR. The exact credited receipt, both drains and fresh remote sentinel replace that assumption; actual leak discrimination fails. |
| Retained route state survives an initial-value edit | Confirmed | carryEntryState remains unchanged; the command-held reload control and existing live reload pass. |
| The final lab used the named working-tree runner | Confirmed | Main traced ZE_LE_BUILD_NAME through the launcher and runHostLe environment. Final checker hash remains e0e29fd20f4afd6d27544d05fa6c3d320f74c5c167dbe70adc8ce03a934dd052. |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| False/omitted announce and true holds. | collectWatchdogRoutes and parseNLRIEntries set the initial flag; handleStateUp sends announced entries. | Yes, healthcheck guide and route-ownership page agree with the actual source and final wire proof. |
| Existing runtime state survives reload. | carryEntryState copies the old announcement state; new entries use initiallyAnnounced. | Yes, both pages agree and dedicated reload controls pass. |
| Schema default and help | Current update-block leaf has default false and the matching description/help. | Yes, loaded-schema test and help contracts pass. |
| Release note and legacy reload explanation | The fragment describes the same behavior; the legacy CI diff changes only its explanatory comment. | Yes, read against the producer and unchanged assertions. |
| Unchanged categories | No command, SDK, wire, metadata, counter, production dependency, feature-scope or test-infrastructure change appears in the diff. | No additional page is required; the existing syntax example explicitly sets withdraw true. |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `declared-format-contradicts-payload.md` | 2026-09-03 | Watchdog withdraw false and the already-fixed BGP local-port sibling. Both halves are now marked fixed with their own producer and proof provenance. |
