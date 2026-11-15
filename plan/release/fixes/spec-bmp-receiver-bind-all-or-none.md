# Spec: bmp-receiver-bind-all-or-none

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | A BMP receiver listen address that fails to bind is only logged while the other addresses serve, so start and reload report success with the receiver half-bound |
| RFC gaps | - |
| Depends | - |
| Phase | closure - scoped proof and documentation receipt complete; landing and final pin pending |
| Handoff | - |
| Updated | 2026-11-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`(*BMPPlugin).applyReceiverConfig` (`internal/component/bgp/plugins/bmp/receiver.go`)
closes the listeners no longer wanted, then calls `startListenerLocked` once per
new address. `startListenerLocked` logs `bmp: listener bind failed` and returns,
and `applyReceiverConfig` returns nothing. Its three callers in `bmp.go` (the
Stage-2 configure handler, `OnConfigApply` and `OnConfigRollback`) therefore
cannot refuse: the receiver serves on the addresses that bound and the commit
or start reports success.

Make the receiver bind its wanted set all or none, as SSH now does: bind every
added address before closing any removed one, close what this apply bound on a
failure so the previous set keeps serving, and return an error that names the
address, why it failed and the fix to the config transaction, so the commit or
the start fails visibly (`ai/rules/user-facing-errors.md`).

The external BMP proof exposed one small startup defect that this work also
repairs: `runYANGConfig` ignores an invalid explicit plugin-hub configuration
and can select automatic transport when an external plugin needs one. Validate that configuration once
after loading it, refuse an extraction error before starting components, and
give the same validated value to the process manager and plugin server. This
changes startup refusal only; it adds no hub transport reload feature.

## Required Reading

- [ ] `ai/doors/system.md`, `ai/doors/bgp-plugins.md`, `ai/doors/config-yang.md`, `ai/doors/plugin-infra.md`
  → Constraint: listener state stays in BMP; reuse the SSH design without importing SSH implementation.
- [ ] `docs/guide/bmp.md`, Receiver
  → Decision: retain existing listener addresses, per-listener session limits and accepted-session lifetime.
- [ ] `docs/architecture/config/transaction-protocol.md`, Rollback
  → Constraint: an ordinary failed apply must leave its previous state intact; SDK rollback does not reach that participant.
- [ ] `docs/architecture/plugin/plugin-system.md`, The process boundary
  → Constraint: use the existing SDK callbacks for internal and external BMP processes.

## Current Behavior (MANDATORY)

- [ ] `internal/component/bgp/plugins/bmp/receiver.go` - `applyReceiverConfig` closes removed listeners first, then `startListenerLocked` binds each added address alone; a bind failure is logged at Error and skipped, and nothing is returned
- [ ] `internal/component/bgp/plugins/bmp/bmp.go` - the Stage-2 handler, `OnConfigApply` and `OnConfigRollback` call `applyReceiverConfig` and never see a failure
- [ ] `internal/component/ssh/ssh.go` - `bindListeners` and `(*Server).Reconfigure`, the all-or-none shape home-cpe child 7 gave SSH (7ff7614e5f)
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `configApplyCallback` opens rollback only after success or an error wrapping `ErrRunningConfigLost`; `OnConfigRollback` propagates the handler's error
- [ ] `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` - the server description explicitly describes partial binding today

Behavior to preserve: unchanged listeners retain their sockets and accepted sessions; removed listeners stop accepting without ending accepted sessions; a successful cap-only change takes effect without rebinding. No BMP message or SDK wire format changes.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| SSH all-or-none bind | `bindListeners`, `serveListeners`, `(*Server).Reconfigure`, `internal/component/ssh/ssh.go` (home-cpe child 7, 7ff7614e5f) | Binds every address or none, closes the partial set on failure, binds additions before closing removals so a failed reload keeps the old set | Extends the same shape to the BMP receiver. The code stays in the BMP plugin, because a plugin never imports another component's implementation |
| BMP receiver apply | `(*BMPPlugin).applyReceiverConfig`, `startListenerLocked`, `listenerIsCurrent`, `receiver.go` | Diffs wanted against bound addresses, closes removed ones first, then binds each new one alone and logs a failure | Replaced by an apply that binds additions first, returns an error, and leaves the previous set serving on failure |
| Config transaction callbacks | `OnConfigApply`, `OnConfigRollback` and the Stage-2 handler in `bmp.go`; `docs/architecture/config/transaction-protocol.md` | The receiver apply's result never reaches the transaction | Extends: the bind error is returned through the existing callbacks, so the commit refuses and rolls back |
| SDK rollback gate | `configApplyCallback`, `OnConfigRollback`, `pkg/plugin/sdk/sdk_callbacks.go` | A failed own apply normally gets no rollback callback | Constrains: bind before publishing any BMP root's candidate state; a failed bind leaves no partial work |

## Design

1. Keep receiver reconciliation under `bp.mu`, the lock that already serializes it with stop and protects the listener map. Derive retained, added and removed addresses, keeping the configured server name for an error's repair command. Sort added addresses so partial-bind tests can choose a deterministic failing position.
2. Bind additions into a local staged set. Do not publish them, start accept loops, close old listeners or store the new session cap yet. On any failure close the staged set and return an operator error. The running map, cap and accepted sessions are unchanged.
3. After every bind succeeds, publish the added sockets and the requested cap. Remove obsolete map entries before closing their sockets, as `listenerIsCurrent` requires. Start accept loops only for the added sockets. Existing listeners keep their identity and sessions. Delete `startListenerLocked` when its eager-bind-and-serve role is replaced; do not retain a second apply path.
4. The Stage-2 handler returns receiver installation errors and updates `currentReceiver` only after success. Startup failure follows `runBMPPlugin`'s existing teardown, which closes listeners and waits for receiver tasks.
5. `OnConfigApply` attempts the receiver first, before system identity, sender changes or current-config publication. A failed receiver install returns immediately, so the SDK's no-rollback-on-own-failure rule cannot leave another root partly applied. Publish the receiver's current/replaced config only after success, then perform the remaining existing infallible installs.
6. `OnConfigRollback` attempts the saved receiver configuration before changing the other roots. On restore success publish the restored config and clear its undo target. On failure retain that target, return the error and let the existing transaction rollback-failure handling report the broken transaction and restart the participant. Do not report rollback success or silently keep the candidate as though it were the prior config.
7. A bind error names the server entry, endpoint and cause, explains that the requested listener set could not be installed, and gives the concrete `delete environment bmp server <name>` repair with the actual configured name. Startup and rollback errors use the same operator-error producer; callers add phase context only to its problem line.

A wildcard/specific-address replacement that cannot bind while the old socket exists is refused and leaves the old set serving, as SSH does. Binding an external port is runtime work, so offline validation does not reserve sockets or claim the port is available.

## Owner Decisions

| Date | Decision | Effect |
|------|----------|--------|
| 2026-11-15 | "Allow a one-off exception for these three fixes" | Permits watchdog, firewall-prefix and this BMP fix under the session's one-off WIP exception. The standing cap is unchanged. Complete design approval followed in the next row |
| 2026-11-15 | "go" | Approves the complete design presented by the main session; implementation starts with regression tests. |

No unresolved product choice remains in this design. PPP work is excluded and its specs remain unchanged.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

- `environment bmp server` entries in the config, delivered to the BMP plugin at Stage 2 or as a reload's verify and apply.

### Transformation Path

1. `parseReceiverConfig` reads the `environment` section.
2. `wantedListeners` derives the address set.
3. `applyReceiverConfig` stages every added bind, then publishes the successful set and cap before retiring removed sockets.
4. Configure/apply/rollback return its result through the existing SDK callback; callback-local current and undo values change only after installation succeeds.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | Existing config callback response carries the error; no new fields or methods | SDK callback units and four real internal/external daemon cases passed |

### Changed-Wire Census

No wire format changes. Existing SDK configure/apply/rollback methods, request
and response fields, framing and transports are preserved. TCP/BMP encoding
and decoding are unchanged. This repair changes listener admission, truthful
success/refusal and error/log content through existing fields. No new method,
field or message version needs migration in an external plugin or collector.

**Search evidence:** Searched `pkg/plugin/sdk`, `pkg/plugin/rpc` and `internal/component/plugin/ipc` for `callbackConfigApply`, `callbackConfigRollback`, `ConfigApplyInput`, `SendConfigApply`, `SendConfigRollback` and `marshalStatusError`. Positive controls found `PluginConn.SendConfigApply`/`SendConfigRollback` writing the existing method/input shape, `Plugin.configApplyCallback`/`OnConfigRollback` consuming it, and `marshalStatusError` writing unchanged `status`/`error` fields. Searched the BMP package for `decodeCommonHeader`, `DecodeMsg` and `writeInitiation`, then read `handleSession`, the header codec and SDK callback carriers. Those codecs and payload declarations are outside this spec's edit set. The changed producer calls the same carriers and returns an error rather than adding a second protocol. No fictitious changed-wire rows are needed.

### Integration Points

- The SDK config callbacks in `bmp.go`; no new registration.

### Changed-Surface Census

| Writer or reader | Required change |
|------------------|-----------------|
| `applyReceiverConfig`, `startListenerLocked`, `stopListeners`, `listenerIsCurrent`, `acceptLoop` | One staged install; preserve map/close ordering, task cleanup and per-listener caps |
| `registerCallbacks`: Stage 2, verify, apply, rollback | Return install errors; keep current/replaced values truthful; verify remains side-effect-free |
| `receiverConfig.Servers`, `wantedListeners` | Preserve the address set and associate a failed address with its configured name |
| YANG server description and BMP guide | Replace the partial-success description and qualify the reload contract |
| SDK config callback envelope | Unchanged; test the real callback path rather than only calling the installer |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Startup with an externally held BMP port | → | Stage-2 configure and `applyReceiverConfig` | `test/plugin/bmp-receiver-bind-conflict.ci` |
| Reload adding an externally held BMP port | → | SDK config-apply and `applyReceiverConfig` | `test/reload/bmp-receiver-bind-conflict.ci`; fixture checks the prior listener still accepts and staged additions are unbound |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Start with two BMP servers, one address already held | The plugin's configure fails with an error naming the address and the fix; no receiver listener stays bound | none |
| AC-2 | Reload adds a server whose bind fails | The commit is refused with that error; the previously bound set still accepts | none |
| AC-3 | A failed apply also requests a different cap, identity and sender config | Previous listener identities, cap, identity and sender configuration remain in force; current config is not advanced | none |
| AC-4 | Reload adds/removes listeners successfully or changes only the cap | Wanted set and cap take effect; unchanged listener sockets and their open sessions survive; removed listeners stop accepting | Existing accepted sessions on a removed listener continue, matching current lifetime semantics |
| AC-5 | A later participant rejects after BMP successfully applied | Rollback restores the previous set and cap and returns success only after restoration | Existing accepted sessions continue |
| AC-6 | A previous listener's address is taken externally before rollback | Rollback returns the endpoint-specific error, retains its undo target, and reaches existing broken-rollback handling | Candidate sockets remain until coordinated recovery; no success is claimed |
| AC-7 | Disable receiver or remove its root/server entries | Removed sockets close and no replacement sockets open; shutdown completes | Existing accepted sessions keep their existing lifetime |
| AC-8 | Failed reload is followed by a valid reload, including an equal previous configuration | The valid reload is accepted and uses the actual running set, never the failed candidate or a stale undo set | none |
| AC-9 | A daemon without a BGP configuration starts with an explicit plugin-hub block that `ExtractHubConfig` rejects | Startup exits 1, names the configured block and reason on stderr and in the startup log, and starts no replacement transport. Valid explicit and omitted hub settings retain their existing behavior | none |

## Risks and Assumptions

| Risk | Mitigation and proof |
|------|----------------------|
| A staged socket remains after failure | Force the second sorted bind to fail, then bind the earlier address again in the test |
| An apply failure escapes after changing another root | Receiver is the only fallible runtime install and runs first; SDK callback test changes all three roots |
| Closing a removed listener produces a false accept warning | Remove its map entry before close; exercise accept-loop shutdown and existing panic containment tests |
| The old address cannot coexist with its replacement | Return refusal without closing it; test a wildcard/specific overlap |

### Test Corrections During Implementation

| Failure | Producing cause | Correction and independent judgment |
|---------|-----------------|-------------------------------------|
| Successful-reconcile session timed out waiting for its identity | The new `receiverBindSendName` fixture sent only sysName; `decodeInitiation` requires both sysName and sysDescr and correctly closed the malformed session | Add sysDescr to the fixture input while retaining every parsed-session assertion. Independent `plugin_review` approved the correction before editing; native approval recorded in `tmp/commit-weakening-approved-810c8fb1.md` |
| Wildcard replacement was accepted on Darwin while the test required refusal | The new oracle assumed every kernel forbids wildcard/specific coexistence; the installer can bind that pair on this host, which the design permits | Independent kernel probe determines the wildcard case's exact expected result; unexpected probe errors fail. A separate exact-endpoint collision deterministically proves refusal without retiring the old socket or cap. `plugin_review` approved the exact proposed patch before editing; the same native approval record carries the verdict |

These corrections repair test input and its platform oracle. They do not change the producer or count as product fixes. Main's scoped rerun remains the acceptance evidence.

### Final Corpus Finding: External Reload Hub Token

1. Symptom: `TestRepositoryConfigsValidate` rejected the initial, refused and repair snapshots in the external reload case. Each supplied 31 token characters.
2. Producer: `config/loader_extract.go` `extractHubServerConfig` rejects a secret shorter than `minTokenLength` (32). `cli/cmd_validate.go` `runValidation` calls `ExtractHubConfig` and reports `config-hub-invalid`.
3. Owning layer of those corpus findings: the new test's synthetic input. The separate startup error below explains why the daemon had not refused it.
4. `[workaround]` Lower the token minimum or declare the configurations expected refusals. That would hide invalid setup and stop proving the intended BMP rejection/repair path.
5. `[source]` Append one ASCII `0` to the same literal in all three snapshots, first in the draft. Keep every command and assertion. The independent closure reviewer approved the exact three-line patch under `ze-test-judge`, APPROVE(b), against the documented 32-character contract.

The corrected external reload draft passed 1/1 in 1.7 seconds, then passed
80 stress invocations with eight parallel workers and eight burners. All
240 retained named assertions passed. It was promoted unchanged at SHA-256
`76abfd1815d0faf6fe02e6e9f6112ef749c1c14ecb9e35d9c2c653d22540698a`.
The final promoted case passed 1/1 in 1.7 seconds. The repository-config corpus
accepted all three corrected bodies. Its 31 aggregate findings concern other
interface, DDOS and Darwin XFRM inputs. No whole-corpus pass is claimed.

### Companion Startup Diagnosis

1. Symptom: the old external reload test booted successfully with a hub token
   that the repository configuration validator refused. A valid fixture fixes
   the setup, but does not explain the accepted invalid operator input.
2. Producer: `cmd/ze/hub/main.go` `runYANGConfig` calls `ExtractHubConfig` twice
   and uses each result only when its error is nil. `LoadConfig` does not run
   that semantic extractor. With no BGP configuration, no later BGP reader
   returns the error, so the manager sees no explicit hub and may generate one.
3. Owning layer: daemon startup, before process-manager and plugin-server
   construction. The existing extractor already owns token and certificate
   agreement rules; no validation policy needs to change.
4. `[workaround]` Correct only the synthetic token, or log the failed extraction
   and continue. Either leaves a declared transport replaced by default state.
5. `[source]` Extract the hub configuration once after successful config load
   and recovery. Return its error through the existing fatal-startup reporting
   path before component setup; reuse the validated result for both consumers.
   Keep the existing behavior for an omitted block and valid explicit blocks.

Main observed the unchanged producer continue to `Ze running` and the native
draft return TIME after 20 seconds. This is an actual startup RED, not a setup
failure or an observed generated listener in this minimal case, which declares
no external process. The source now extracts once and refuses through a typed
operator error before component setup. The real `Run` entry-point unit passed,
including invalid explicit, valid explicit and omitted hub cases. A separate
verbose count run named all three PASS cases. The guard entered three times,
refused once and continued twice. This is focused outcome proof, not whole-package
coverage. The final CLI draft passed, then completed 80 stress invocations
with 240 named passing assertion receipts. It was promoted unchanged at
SHA-256 `278d395c106471a644852e56bfc1c7804c053571a8f85ad553f4c60239c6b0e6`.
The promoted case passed in 5.0 seconds. Valid configured external BMP startup
and reload each passed in 2.6 seconds. Omitted-hub forked watchdog passed in
2.0 seconds. Offline validation and doctor already report this
extractor's errors. The SIGHUP loader and plain editor validation do not call
it; this startup repair makes no claim of changing hub configuration at reload.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReceiverBindFailureClosesStagedListeners` | `receiver_test.go` | AC-1 and deterministic partial-bind cleanup | passed |
| `TestReceiverFailedApplyPreservesListenersAndCap` | `receiver_test.go` | AC-2, AC-3 | passed |
| `TestReceiverSuccessfulReconcileKeepsSessions` | `receiver_test.go` | AC-4, AC-7; pointer identity and live connections | passed |
| `TestReceiverOverlappingReplacementPreservesOldListener` | `receiver_test.go` | Kernel-controlled overlap outcome without break-before-make | passed |
| `TestReceiverOccupiedReplacementPreservesOldListener` | `receiver_test.go` | Deterministic exact-endpoint refusal preserves the old socket and cap | passed |
| `TestReceiverRefusalQuotesServerName` | `receiver_test.go` | Repair preserves a bare, spaced or pipe list key as one command word | passed |
| `TestBMPReceiverConfigureReturnsBindFailure` | `receiver_config_tx_test.go` | AC-1 through SDK configure | passed |
| `TestBMPReceiverApplyFailurePreservesAllRoots` | `receiver_config_tx_test.go` | AC-2, AC-3, AC-8 through SDK verify/apply | passed |
| `TestBMPReceiverRollbackRestoresPriorConfig` | `receiver_config_tx_test.go` | AC-5 through SDK rollback | passed |
| `TestBMPReceiverRollbackFailureIsReturned` | `receiver_config_tx_test.go` | AC-6 and retained undo target | passed |
| `TestRunHubConfigurationOutcomes` | `cmd/ze/hub/hub_config_startup_test.go` | AC-9 through public Run; short explicit token refuses and logs, valid explicit and omitted hub configurations start and stop through NUL/EOF | passed; all three named cases and added guard outcomes measured |

### Functional Tests

| Test | Entry point and assertion |
|------|---------------------------|
| `plugin/bmp-receiver-bind-conflict` | An external holder reserves one requested port before startup; the daemon exits 1 with the exact operator error. Deterministic staged cleanup is measured by the unit and external startup counterpart |
| `reload/bmp-receiver-bind-conflict` | A daemon starts normally, fixture holds a candidate port and requests reload; refusal leaves prior listener accepting, old config in force and other staged ports available; a subsequent valid reload succeeds |
| `plugin/bmp-receiver-bind-conflict-external` | An explicit external BMP process refuses its held port, reports the bind error and releases its sibling socket; authenticated subsystem listing proves the daemon remains available and the failed BMP process is absent, preserving default ignore-on-failure policy |
| `reload/bmp-receiver-bind-conflict-external` | The same reload observer drives an explicit external BMP process over its TLS callback path and proves refusal, preserved state and valid repair |
| Existing `plugin/bmp-receiver-session` | Receiver continues to accept a real TCP connection |
| `plugin/hub-invalid-config-refused` | A real daemon without BGP refuses an invalid explicit hub block instead of ignoring the error and continuing startup; existing external BMP and forked watchdog cases are valid configured/default transport controls |

Draft the new `.ci` files under `test/draft/plugin/` and `test/draft/reload/` and promote only after their red/green and stress proof under `ze-test`. No protocol interoperability scenario is added: listener admission changes no BMP message behavior.

### Lifecycle Test Selection

| Lifecycle or transport | Selected proof | Scope decision |
|------------------------|----------------|----------------|
| Internal startup, blocked endpoint | Internal startup conflict `.ci` | Must fail daemon startup, release the complete partial set and report the exact refusal |
| Explicit external startup, blocked endpoint | External startup conflict `.ci` | Daemon remains available under the existing default ignore policy; failed BMP process is absent and its sibling endpoint is free |
| Internal and external rejected reload followed by repair | Both reload conflict `.ci` files, same observer | Actual running config, old session traffic, session cap and added listener are read through each transport |
| Successful receiver addition/removal/disable and accepted-session continuity | `TestReceiverSuccessfulReconcileKeepsSessions` and reload repair halves | Exercises retained socket identity, cap changes and traffic on accepted sessions after listener removal |
| BMP applied successfully before another participant fails | `TestBMPReceiverRollbackRestoresPriorConfig` | Real SDK rollback restores exact previous endpoint and cap; the coordinator's generic fan-out protocol is unchanged |
| Rollback cannot restore an externally occupied prior endpoint | `TestBMPReceiverRollbackFailureIsReturned` | Real SDK response reports failure while the candidate set survives; generic coordinator recovery owns restart |
| Whole-daemon stop, crash restart and reboot | Existing lifecycle cleanup; no new standalone scenario | No worker, timer, stop protocol, process policy or persistent state is added. Staged sockets have no accept task and are closed before refusal; normal receiver-task teardown is unchanged |
| Protocol interoperability or Linux-only host behavior | Not selected | TCP listener admission changes no BMP wire encoding or kernel-specific facility; host conflict tests run directly on the current host |

| Required lifecycle scenario | Selection or explicit exclusion | Unchanged source boundary |
|-----------------------------|---------------------------------|---------------------------|
| Process ends after declaration under restart, ignore or fatal policy | Startup refusal is selected in both modes; arbitrary later exits and policy permutations are excluded | This patch returns a Stage-2 error through the existing SDK path and changes no failure-policy declaration, process supervisor or post-start exit path |
| Startup stage fails after declaration | Selected: internal and external startup conflict drafts | Both assert the reported refusal; external also asserts failed-process absence and daemon availability under its existing policy |
| Restart bound within its window or daemon lifetime | Excluded | No restart request, restart counter, admission bound or supervision timer changes |
| Operator respawn setting disagrees with declaration | Excluded | No respawn leaf, declaration or policy-verification path changes |
| Plugin exits during stop drain or teardown | Excluded | No drain callback, teardown exit path or restart-on-stop behavior changes; staged sockets have no worker and close before a refusal returns |
| Signal arrives during config fence, drain or teardown | Excluded | Signal routing and the stop state machine are unchanged; the receiver still uses its existing serialized callback and listener lock |
| Reload or commit arrives after shutdown began | Excluded | Admission to the config transaction and shutdown gate is unchanged; this patch acts only after the SDK delivers a callback |
| Crash in verify/apply, missed callback deadline or broken result | Crash/deadline paths excluded; failed receiver restore uses the selected controlled SDK test | No panic boundary, callback deadline, timeout classification or broken-result transport changes. The controlled occupied-endpoint seam deterministically tests the returned restore error without relying on a timing race |
| Reload context ends during startup handshake or sibling fails at tier barrier | Excluded | Startup context ownership, tier barriers and cancellation are unchanged; the installer performs local synchronous binds and starts no staged accept worker |
| Cross-plugin command dispatch during startup | Excluded | No command dispatch is added to configure or binding; existing sender behavior and startup sequencing are unchanged |

### New-Code Coverage

Main ran the complete BMP package with count coverage before the mechanical
`maps.Copy` modernization. The profile measures installer and callback outcomes,
not only the package percentage. It covers retained/new endpoints, bind success
and failure, staged cleanup, publication, failed apply and both rollback outcomes.

The new duplicate-endpoint name tie-break in `wantedListeners` was removed after review: validated config already refuses duplicate endpoints, and this change needs no canonical-name policy for invalid schema input. The existing bind, cap, cleanup and error guards remain. The sorted addition order is retained for deterministic staged-bind proof.

Main used native job admission, count mode and the toolchain's complete declared
feature tags for the recorded profile. The later modernization replaces only
the staged publication loop with `maps.Copy` under the unchanged lock and
initialized destination. It adds no outcome. Independent source review plus
the final native race package PASS (17.093s), four daemon cases and their stress
runs cover that revision. No duplicate coverage run is owed. Retain the
historical profile only while its changed-outcome comparison is needed.

Main's recorded run passed the complete package in 15.654 seconds, with 88.0% statement coverage. The count profile is `tmp/session/2026-11-15-125434/scratch/bmp-bind-cover.out`; its owning output is `scratch/job-bmp-bind-cover-595efb8b.log`. These are execution counts for the changed outcomes, not a claim that every pre-existing package branch is covered:

| Added outcome | Profile executions |
|---------------|--------------------|
| Installer returns bind failure without publication | 9 |
| Successful installer path and cap publication | 19 |
| Retained listener branch | 17 |
| Removed-listener close path | 8 |
| New-listener publication and accept-task start | 15 |
| New endpoint selected for staging | 26 |
| Bind succeeds and enters staged map | 16 |
| Bind failure returns operator error | 9 |
| Failed stage closes an already-bound sibling | 1 |
| Stage-2 receiver failure reaches configure response | 1 |
| Stage-2 successful receiver config becomes current | 4 |
| Apply receiver failure returns before other roots | 1 |
| Apply receiver success advances current/undo values | 6 |
| Rollback receiver failure returns without publication | 1 |
| Rollback receiver success advances current/undo values | 2 |

## Files to Modify

- `internal/component/bgp/plugins/bmp/receiver.go`
- `internal/component/bgp/plugins/bmp/bmp.go`
- `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` - server description states all-or-none installation and failed-reload preservation
- `docs/guide/bmp.md`
- `cmd/ze/hub/main.go` - one startup extraction, truthful refusal and shared validated hub value
- `docs/guide/plugins.md` - hub startup refusal, current listener syntax and source anchor

## Files to Create

- `internal/component/bgp/plugins/bmp/receiver_test.go`
- `internal/component/bgp/plugins/bmp/receiver_config_tx_test.go`
- `internal/test/fixture/register_bmp_receiver_bind.go` - conflict holder and reload assertions, registered locally by `init`
- `test/plugin/bmp-receiver-bind-conflict.ci`
- `test/reload/bmp-receiver-bind-conflict.ci`
- `test/plugin/bmp-receiver-bind-conflict-external.ci`
- `test/reload/bmp-receiver-bind-conflict-external.ci`
- `cmd/ze/hub/hub_config_startup_test.go` - real Run entry-point refusal and startup-log coverage
- `test/plugin/hub-invalid-config-refused.ci` - drafted first under `test/draft/plugin/`

### Integration and Documentation Checklist

| Surface | Applies | Action |
|---------|---------|--------|
| YANG values, command grammar, SDK protocol, registrations | No | Existing grammar, fields, callbacks and plugin registration remain unchanged |
| Listener and config transaction state | Yes | Stage bind before publication; preserve all roots on failed apply; return failed rollback |
| Functional entry points | Yes | Startup and reload conflict cases, hub startup refusal and valid configured/default transport controls |
| Runtime dependencies and metrics | No | Existing TCP listener dependency and counters; no new dependency or metric |
| BMP guide | Yes | Opening reload claim and Receiver section describe refusal, old-set preservation and overlap limit |
| YANG help/description | Yes | Remove partial-bind-success claim from server description |
| Plugin guide | Yes | Hub Configuration documents early refusal and uses current `ip` syntax |
| API/wire/SDK, comparison, architecture, RFC and route metadata | No | No new surface, format, protocol behavior, structural contract or metadata |
| Test infrastructure and agent discovery | No | Use existing fixture registry, SDK callbacks and functional harness |

## Documentation Update Checklist

| Category | Update | Evidence |
|----------|--------|----------|
| Feature declaration | No | `features/bmp-delivery.md` keeps the same BMP delivery scope and level. Listener admission fixes an existing capability |
| User guide | Yes | `dc6b579fcf` updates `docs/guide/bmp.md`, opening reload contract and Receiver. Anchors name `applyReceiverConfig`, `bindReceiverListeners` and `registerCallbacks` |
| Config syntax | No | No node, leaf, type, default or grammar changes. The server-list description in `ze-bmp-conf.yang` now states all-or-none installation |
| CLI reference | No | No command registration or grammar changes. Repairs use existing `delete environment bmp server` and `delete plugin hub` paths |
| API and RPC | No | Existing configure/apply/rollback error responses carry the refusal. No method or payload changes |
| Plugin guide and SDK | Yes, guide only | Document early invalid-hub startup refusal and use current `ip` listener syntax in `docs/guide/plugins.md`. No SDK or failure-policy change |
| Wire format and RFC compliance | No | No BMP message, codec, protocol state machine or RFC-tagged unit changes |
| Comparison | No | No new parity or support claim |
| Test infrastructure | No | Five new cases use the existing fixture registry and runner. No runner or test format changes |
| Architecture | No | BMP keeps its existing SDK callbacks and mutex-owned listener map. No new component or cross-plugin coupling |
| Route metadata and metrics | No | No key, counter or gauge changes |
| Runtime inventory and agent discovery | No | No plugin, command, config root, feature gate or development action is added |

Source-anchor search over the changed receiver/callback files found their guide
claims in `docs/guide/bmp.md`. Each changed claim now names its actual producer.
The new fixture also points at that durable guide rather than this spec.

## Implementation Steps

1. Wiring: draft startup/reload functional assertions and SDK callback tests; main records failures against current behavior.
2. Add installer tests, implement staged binding and truthful cap publication, and preserve listener/session cleanup.
3. Return errors from callbacks and order state publication after receiver success. Add rollback and multi-root failure tests.
4. Update YANG and guide with the behavior change; main executes scoped red/green, stress and package regression commands, then obtains independent review.
5. Repair the small hub startup error-discard found by the external fixture: draft the real CLI regression, have main record RED, then extract once and refuse before component setup. Add real Run entry-point coverage, run valid configured/default transport controls, measure the new guard outcomes and promote after the required stress proof.

## Deliverables Checklist

| Deliverable | Verification |
|-------------|--------------|
| One staged receiver install and no eager bind helper | Source review of `applyReceiverConfig`, `bindReceiverListeners` and all callers |
| Errors cross configure/apply/rollback truthfully | SDK callback tests plus internal and external daemon conflict tests |
| Old cap, roots and sessions survive failed apply | Multi-root SDK test and both reload observers |
| Operator documentation matches new behavior | YANG description and Receiver guide review with source anchors |
| New control-flow outcomes are measured | Main's full BMP count-coverage profile, inspected against the changed source |
| Live functional regression coverage | Four conflict drafts pass, discriminate and pass the required stress run before promotion |
| Invalid explicit hub refuses before startup | Real no-BGP CLI draft RED then GREEN, real Run unit with invalid/valid/omitted controls, measured guard outcomes and unchanged promotion after stress |

## Security Review Checklist

| Check | Required finding |
|-------|------------------|
| Resource release | Every failed stage closes only this attempt's sockets; previously serving endpoints and sessions remain |
| Error injection | Server keys reach the exact repair command through `configtext.QuoteCommandWord`; bare, spaced and pipe names are tested |
| Transaction integrity | Failed apply publishes no other root or candidate cap; rollback failure is returned, not translated into success |
| Process boundary and failure policy | Same refusal crosses external SDK transport; default external failure policy is preserved, not replaced with daemon-fatal behavior |
| Secret or protocol exposure | No credential, wire message or network authorization mechanism is added; existing hub validation failures now refuse startup without exposing a token |
| Hub error and logging path | Typed operator refusal has the existing `config-hub-invalid` doctor code, a concrete repair, stderr and startup-log evidence, without the token value |

## Implementation Review History

| Round | Scope | Independent result and action |
|-------|-------|-------------------------------|
| 1 | Complete BMP producer, tests, fixture and design | ISSUE: lifecycle entry-point proof was internal-only. Added explicit external startup/reload counterparts, preserving existing process policy; subsequent main runs and stress passed |
| 2 | Changed producer outcomes and coverage profile | No additional product defect. Every added outcome executed. Explicit lifecycle exclusions added. Subsequent external runs, stress and discrimination completed |

### Current Implementation and Verification State

| Item | Recorded state |
|------|----------------|
| Product repair | Committed as `dc6b579fcf`, `fix(bmp): install receiver listeners atomically`: receiver/callback source, both unit-test files, guide and YANG description |
| Unit and source evidence | Complete BMP package passed with 88.0% count coverage; every added outcome above executed; independent source review found no product defect |
| Functional baseline | Main observed all four startup and rejected-reload/repair drafts pass, including both external modes after the independently judged fixture corrections |
| Initial discrimination attempt | Zero test evidence: the named action panicked during unrelated developer-help initialization before running a test. The help blocker was subsequently fixed before the actual runs below |
| Actual functional discrimination | Main verified the false-success source cut and ran all four drafts: external startup FAIL (2.7s), incorrectly retaining `bgp-bmp` as Running/count 1 and omitting refusal; internal startup TIME (60.1s), incorrectly reaching Ze Running instead of exit 1; internal/external reload FAIL (1.5s/1.7s), with the observer reporting outcome `applied`, wanted `failed` for the occupied endpoint |
| Mutation restoration and GREEN | Main restored `receiver.go` to SHA-256 `d92e3d699287d99ef145bd99e6a17f2eb735246a007af586674b5590fb20be80` and confirmed the three scoped producer paths are marker-clear. Restored startup PASS 2/2 (external 2.3s, internal 15.1s); restored reload PASS 2/2 (both 1.7s). All four actual RED/restored GREEN requirements are met |
| Scoped-lint modernization | Root approved replacing the staged publication loop with `maps.Copy` after the unchanged destination initialization under the unchanged lock. Current receiver SHA-256 is `d3eb9733f1ff6d018567c09bb6c7c5e7bc719e602b1957929db1dbe1395427f1`. Full package and all four promoted daemon cases passed on this revision |
| Final readiness review | Follow-up CLEAN after bounded waits for both `fileDaemonReady` and `fileDaemonPID` before unchanged PID validation/SIGHUP. Review assumption corrected: current `runYANGConfig` registers signals before plugin startup, so a default-fatal SIGHUP defect was not established. The guard proves complete daemon readiness plus atomic identity. Final promoted and stress runs passed with this guard |
| Post-modernization package and compatibility | Main reports full BMP package PASS (`job-unit-pkg-55a7662d.log`) and five existing compatibility cases PASS, including `bmp-receiver-session` |
| Qualified stress complete | All four selections PASS 80 invocations each, with 240 named PASS receipts and zero failures per capture (parallel 8, burners 8). Exact evidence paths and current hashes are in the owning session state |
| Promotion | All four drafts moved unchanged to `test/plugin/` and `test/reload/`; before/after SHA-256 matches for every file. Own draft copies are gone |
| Final promoted entry points | Independent main thread ran plugin startup PASS 2/2 (external 1.9s, internal 15.1s) and reload PASS 2/2 (internal 1.6s, external 1.8s). Each named PASS line and exit 0 was read |
| Final scoped lint | Native host and Linux runs reported no finding in BMP or `register_bmp_receiver_bind.go`. Global exit 1 contains three foreign IRR fixture findings, listed below |
| Corrected external reload input | Draft GREEN, fresh 80-run stress, unchanged promotion and final promoted PASS 1.7s. Corpus accepted all three corrected snapshots; its global result stays red for 31 other findings |
| Companion hub startup unit | Real Run and verbose count proof passed on final guard source. Named invalid/valid/omitted cases all passed. Guard outcomes: entry 3, refusal 1, continuation 2 |
| Companion hub final proof | CLI draft GREEN, 80 successful stress invocations and 240 named passing receipts, unchanged promotion and final promoted PASS 5.0s. Configured external BMP startup/reload PASS 2.6s each, omitted-hub forked watchdog PASS 2.0s |
| Companion hub scoped lint | Host, Linux, capability, appliance and compile-out runs have no finding in changed `main.go` or the new unit. Four foreign locations remain, recorded below |
| Final documentation receipt | Named check completed exit 1: no drift, 454 commands validated and complete help shape. Four foreign IRR source-anchor findings remain; none concerns the changed hub paragraph or BMP guide |
| Still required | Separate guard and wiring commits, hash-pinned Review Gate and two closure commits. Status stays `in-progress` |

The commit preparer owns reserved approval trailers. The product commit records
independent unit judgment in ordinary prose. Fixture/draft `Test-judge` fallback
verdicts ride with the wiring change. The four actual cut runs discriminate
false success. The internal startup timeout is the native harness verdict,
with its exit-1 and refusal assertions retained. No test was skipped or weakened.
Restored runs, all four stress selections and all four promoted cases passed.

## Checklist

- [x] Tests written
- [x] Tests FAIL before the fix
- [x] Tests PASS after the fix
- [x] Main runs the full BMP package with native feature tags and race detection, and records count coverage separately
- [x] Main runs named-build functional drafts, records actual red/restored green, stress-tests each 80 times and promotes unchanged
- [x] Main runs existing `bmp-receiver-session`, final promoted cases and scoped BMP package regression
- [x] Main runs documentation checks for the edited guides and independent review over the final change set. Global foreign findings are disclosed below

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `listener-set-partly-bound-reported-as-applied.md` | 2026-11-09 | BMP receiver `startListenerLocked` logs a failed bind and serves on the rest |

## Implementation Summary

### What Was Implemented

`applyReceiverConfig` stages added endpoints under the existing listener lock.
It publishes sockets and the cap only after every bind succeeds. A failed
stage closes its own sockets. Retained sockets and accepted sessions survive.
`registerCallbacks` returns installation errors through configure, apply and
rollback. Apply and rollback attempt the fallible receiver before other roots.
The companion startup repair extracts explicit hub settings once before
component setup. Any extractor error now refuses startup instead of becoming
absent configuration. The same validated value reaches all three consumers.

### Bugs Found/Fixed

- Partial receiver installs no longer report success. `TestReceiverBindFailureClosesStagedListeners` and all four daemon cases cover refusal.
- Failed apply preserves other BMP roots. `TestBMPReceiverApplyFailurePreservesAllRoots` covers identity, sender state, cap and subsequent repair.
- Failed rollback reports the occupied endpoint without destroying the candidate. `TestBMPReceiverRollbackFailureIsReturned` covers that path.
- The reload observer waits for complete readiness and atomic PID publication. The same external reload assertions cover the original missing-PID failure.
- The external reload fixture supplied a 31-character hub token. The independent judgment and corrected-input proof are recorded above.
- Startup ignored an invalid explicit hub when no later BGP reader rejected it. `TestRunHubConfigurationOutcomes` proves refusal and valid/omitted continuation. The promoted CLI case passed after its actual RED and 80-run qualification.

### Documentation Updates

`dc6b579fcf` updates the BMP guide and server-list description. Guide anchors
name `applyReceiverConfig`, `bindReceiverListeners` and `registerCallbacks`.
The Hub Configuration paragraph in `docs/guide/plugins.md` now states early
refusal, uses current `ip` syntax and anchors `runYANGConfig` and the YANG module.
The Documentation Update Checklist records the unchanged surfaces. Main's final
named documentation check found no drift, validated 454 commands and confirmed
complete help shape. Its four foreign IRR anchor findings are listed below.

### Deviations from Plan

The wildcard test uses an independent kernel probe rather than assuming every
host rejects overlap. A separate exact-address collision proves refusal.
The fixture corrections preserve the intended BMP assertions. Internal startup
proves daemon refusal. Deterministic units and external startup prove cleanup,
because the runner's asynchronous foreground step was not a post-exit fence.
No product scope was removed. No BMP wire behavior changed.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Initiation fixture sent only sysName | RFC 7854 Section 4.3 says: "The sysDescr and sysName Information TLVs MUST be sent, any others are optional." | Decoder refused the fixture | Add sysDescr under independent judgment |
| assumption | Wildcard replacement was assumed unavailable on every host | The kernel can permit coexistence | Independent bind probe | Assert the measured kernel result and retain deterministic exact-address refusal |
| approach | A foreground startup step was treated as complete before the next command | Foreground daemon orchestration is asynchronous | Runner source review | Keep startup daemon final and prove cleanup through units/external startup |
| assumption | Seeing daemon.pid was treated as complete readiness | Runner publishes PID after a ready-wait timeout too | Original ENOENT and source review | Wait for both ready and PID markers. No default-fatal SIGHUP defect was established |
| approach | A synthetic hub token had 31 characters | `extractHubServerConfig` requires 32 | Actual repository-config corpus | Independently judged three-literal draft correction and fresh selected proof |
| approach | Correcting the fixture alone left accepted invalid operator input unexplained | Two startup extractor errors were ignored | Real no-BGP CLI RED reached `Ze running`, not refusal | Extract once, return typed refusal before components and prove valid/omitted controls |

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Bind every requested addition or refuse | Done | `receiver.go` `bindReceiverListeners` | Stable staging, close-on-failure and typed operator error |
| Failed reload preserves the serving set and cap | Done | `receiver.go` `applyReceiverConfig` | No map/cap/removal publication before successful staging |
| Return truthful configure/apply/rollback outcomes | Done | `bmp.go` `registerCallbacks` | Error returned before current/replaced and other roots advance |
| Refuse invalid explicit plugin-hub settings before components start | Done | `cmd/ze/hub/main.go` `runYANGConfig` | Existing extractor policy reused, one validated value for all consumers, final real CLI refusal and valid/default controls passed |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | Staged cleanup and configure-refusal units, internal/external startup cases | Exact endpoint and repair command |
| AC-2 | Done | Failed-apply unit and both daemon reload modes | Old listener remains live after refusal |
| AC-3 | Done | `TestBMPReceiverApplyFailurePreservesAllRoots` | Listener, identity, mirroring and cap preserved |
| AC-4 | Done | `TestReceiverSuccessfulReconcileKeepsSessions` | Socket identity, new endpoint, new cap and parsed traffic on retained/removed sessions |
| AC-5 | Done | `TestBMPReceiverRollbackRestoresPriorConfig` | Prior endpoint/cap restored, candidate endpoint free |
| AC-6 | Done | `TestBMPReceiverRollbackFailureIsReturned` | Error names old endpoint, candidate still serves, later repair succeeds |
| AC-7 | Done | `TestReceiverSuccessfulReconcileKeepsSessions` | Disable closes listeners while accepted session still carries parsed identity |
| AC-8 | Done | Multi-root failed-apply unit and both daemon repair halves | Rejected candidate does not poison later accepted config |
| AC-9 | Done | `TestRunHubConfigurationOutcomes` and promoted `hub-invalid-config-refused` | Invalid input exits 1 and logs code/reason/fix without its token. Valid explicit and omitted input start and stop, with final configured/default transport cases passed |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Six installer/control/quoting units | Done | `receiver_test.go` | All names are listed in the TDD table |
| Four configure/apply/rollback SDK units | Done | `receiver_config_tx_test.go` | Real callback transport and rollback gate |
| Internal startup and reload cases | Done | Promoted `test/plugin/` and `test/reload/` cases | Actual cut RED/restored GREEN, 80 each, final promoted PASS |
| External startup case | Done | `test/plugin/bmp-receiver-bind-conflict-external.ci` | Actual cut RED/restored GREEN, 80, final promoted PASS |
| External reload token correction | Done | `test/reload/bmp-receiver-bind-conflict-external.ci` | Corrected draft GREEN+80, unchanged promotion, final PASS 1.7s and three corpus bodies accepted |
| Existing receiver connection regression | Done | `test/plugin/bmp-receiver-session.ci` | Main observed PASS in the selected compatibility batch |
| Hub startup refusal and valid/omitted controls | Done | `hub_config_startup_test.go` and `test/plugin/hub-invalid-config-refused.ci` | Real Run/count cases passed, CLI RED/GREEN+80 and final promoted case passed. Configured and automatic transport controls also passed |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `bmp.go`, `receiver.go` | Done | Core commit plus reviewed `maps.Copy` modernization |
| `receiver_test.go`, `receiver_config_tx_test.go` | Done | Ten new top-level units |
| `yang/ze-bmp-conf.yang`, `docs/guide/bmp.md` | Done | Refusal, session lifetime and external failure policy documented |
| `internal/test/fixture/register_bmp_receiver_bind.go` | Done | Dedicated registrations and real live-state assertions |
| Four promoted BMP `.ci` paths | Done | Corrected external reload promoted unchanged after its renewed proof |
| `cmd/ze/hub/main.go`, `hub_config_startup_test.go`, `docs/guide/plugins.md` | Done | Early refusal, public Run controls, measured outcomes, scoped lint and current guide claim reviewed |
| `test/plugin/hub-invalid-config-refused.ci` | Done | Promoted byte-for-byte after qualification, final case PASS, draft gone |

### Audit Summary

All product requirements and nine acceptance criteria are proved. Scoped tests,
stress, promoted entry points and valid/default controls are complete. Only
landing and closure bookkeeping remain. No product scope
was skipped or reduced.

## Goal Validation

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Refuse startup instead of serving a partial receiver set | Socket/SDK units and real daemon | Internal startup exits 1. External startup reports refusal, excludes failed BMP and proves sibling free while daemon remains available |
| Preserve the old set after failed reload | Real daemon reload and SDK | Observer checks exact running config, newly parsed identity on the old session, cap-1 refusal and staged port freedom |
| Apply a valid repair after refusal | Real daemon reload and SDK | Old session parses a new identity, session 2 is admitted and the added endpoint parses its identity |
| Report failed restoration truthfully | Real SDK callback | Occupied prior endpoint makes rollback return its endpoint-specific error, preserving candidate socket/cap until recovery |
| Prove the assertions detect false success | Actual source discrimination | Returning nil on a staged bind failure made all four cases red. Restored source passed. Corrected external reload input completed fresh GREEN+80/final proof |
| Invalid explicit hub does not become default transport | Real CLI RED/GREEN and public Run positive/negative controls | Old binary reached `Ze running`. Repaired promoted CLI exits 1 with its refusal, PASS 5.0s. Valid configured external BMP startup/reload PASS 2.6s each, omitted-hub forked watchdog PASS 2.0s |

## Work Not Done

No product scope was omitted or moved. Pending checks are work in this spec,
not a destination or a deferral.

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All approved product scope implemented and proved. This spec stays open until the remaining closure bookkeeping finishes | None |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | Pending root guard/wiring handoff and claim release |
| `./le spec review check` | Not run yet |
| Rounds | 4: two implementation reviews, independent full-population closure review, then the added hub startup guard and its consumers |
| Reviewer lenses used | Logic/wiring, security/edge cases, style/resource lifetime. Complete historical core and wiring population read |
| Current result | Complete receiver, fixture and added hub startup source review CLEAN. All scoped proof and final entry-point controls complete. Final hash artifact follows root guard/wiring handoff |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | External process entry-point proof missing | Implementation wiring | Added external startup/reload counterparts and exercised them |
| 2 | ISSUE | External reload snapshots carry an invalid 31-character token | External reload `.ci` | Independent approval and fresh corrected draft GREEN+80, unchanged promotion, final promoted PASS and corpus acceptance |
| 3 | ISSUE | Startup discards explicit hub extraction errors | `runYANGConfig` | Single early extraction, typed coded refusal and shared validated value. Final Run/count, actual CLI RED/GREEN+80, promoted case and valid/default controls passed |

The generic review found no removed guard, fail-open bind outcome, unwired
producer, hot-path allocation or peer-reachable panic. The style pass covered
all changed Go files. `repo check` reported three foreign IRR number-parse
findings. `test weakened check base origin/main` reported five foreign
RFC-tagged test changes and no BMP finding. These are not whole-tree green
results and do not waive any BMP proof.

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/bmp/receiver_test.go` | Yes | `ls -l`: 10,962 bytes, read all six units |
| `internal/component/bgp/plugins/bmp/receiver_config_tx_test.go` | Yes | `ls -l`: 7,691 bytes, read all four units |
| `internal/test/fixture/register_bmp_receiver_bind.go` | Yes | `ls -l`: 8,577 bytes, read registrations and observer |
| `test/plugin/bmp-receiver-bind-conflict.ci` | Yes | `ls -l`: 961 bytes, read daemon exit/refusal assertions |
| `test/plugin/bmp-receiver-bind-conflict-external.ci` | Yes | `ls -l`: 2,220 bytes, read failure-policy and cleanup assertions |
| `test/reload/bmp-receiver-bind-conflict.ci` | Yes | `ls -l`: 2,127 bytes, read rejected/repair snapshots |
| `test/reload/bmp-receiver-bind-conflict-external.ci` | Yes | Corrected live SHA-256 `76abfd1815d0faf6fe02e6e9f6112ef749c1c14ecb9e35d9c2c653d22540698a`. Draft removed after unchanged promotion |
| `docs/guide/bmp.md` | Yes | `ls -l`: 33,561 bytes, changed guide claims and anchors reviewed |
| `internal/component/bgp/plugins/bmp/yang/ze-bmp-conf.yang` | Yes | `ls -l`: 10,938 bytes, server-list description reviewed |
| `website/changes/fragments/bmp-receiver-bind-all-or-none.md` | Yes | Metadata and six headings read. Before/After cover both receiver binding and companion hub startup refusal |
| `cmd/ze/hub/hub_config_startup_test.go` | Yes | Final SHA-256 `acb1298d5cef966e6fa16699069c040e143b5b135b4bea313b3cd4ee5122d95b`, real Run test read |
| `test/plugin/hub-invalid-config-refused.ci` | Yes | Live SHA-256 `278d395c106471a644852e56bfc1c7804c053571a8f85ad553f4c60239c6b0e6`, unchanged promotion, draft gone |

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1/2/3 | Refusal preserves old state and closes only staged additions | Full BMP native package PASS after modernization, 17.093s. Added failure/success outcomes have nonzero count coverage |
| AC-4/7 | Retained/removed accepted sessions survive successful changes and disable | Successful-reconcile test sends fresh parsed identities over those exact sockets |
| AC-5/6 | Rollback restores or returns a truthful error | Both SDK rollback tests in the passing package. Failure path keeps candidate cap and socket |
| AC-8 | Later accepted config is not poisoned by refusal | Actual refused/repair observer checks and SDK test. Corrected external reload passed renewed qualification and all three corpus bodies |
| AC-9 | Invalid explicit hub refuses while valid/omitted settings continue | Run/count cases PASS, entry 3/refusal 1/continuation 2. Final promoted CLI PASS 5.0s, external configured BMP startup/reload PASS 2.6s each and omitted-hub watchdog PASS 2.0s |

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Internal startup, held endpoint | `test/plugin/bmp-receiver-bind-conflict.ci` | Holder ready before final daemon step. Endpoint/repair text and exit 1 asserted |
| External startup, held endpoint | `test/plugin/bmp-receiver-bind-conflict-external.ci` | TLS plugin callback, daemon availability, failed BMP absence and live sibling release asserted |
| Internal rejected reload and repair | `test/reload/bmp-receiver-bind-conflict.ci` | Real SIGHUP transaction, exact config, session traffic, cap and listener assertions |
| External rejected reload and repair | `test/reload/bmp-receiver-bind-conflict-external.ci` | Same observer over TLS callback path. Corrected input passed fresh draft/stress/final promoted proof |
| Invalid explicit hub without BGP | `test/plugin/hub-invalid-config-refused.ci` | Real CLI RED then final GREEN, 80-run qualification, unchanged promotion and final promoted PASS |

### Assumptions Resolved

| ID | Final Status | Evidence |
|----|--------------|----------|
| Failed-own-apply rollback | Confirmed | SDK `configApplyCallback` opens the gate only for accepted apply or `ErrRunningConfigLost`. Receiver runs first |
| Wildcard/specific coexistence | Corrected | Independent kernel probe chooses the exact valid expected result. Separate exact-address collision always proves refusal |
| PID implies ready | Broken and corrected | Bounded waits now require both daemon-owned ready and atomically published PID. Original ENOENT retained as the observed failure |
| Fixture authentication setup | Broken and corrected | Existing hub contract requires 32. Exact correction independently approved, qualified and accepted by the corpus |
| Startup propagates hub extractor errors | Broken and repaired | Actual old-binary RED, final Run/count and promoted CLI proof, configured/default transport controls passed |

### Documentation Verified

| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| All-or-none startup/reload and old-set preservation | `bindReceiverListeners`, `applyReceiverConfig`, all three callback callers | Yes |
| Accepted-session lifetime | Listener close stops accepts. `handleSession` owns each accepted socket | Yes, with parsed-identity tests |
| External failure policy | External startup case keeps daemon available and excludes failed BMP | Yes |
| Invalid explicit hub stops component startup | Guard is before component setup and returns 1. `opererr` supplies reason/fix and coded startup log attributes | Yes, source and real Run proof |
| Config grammar, commands, SDK/wire, metrics and inventory | No registration, type, default, event, command or metric changes in the reviewed diff | No page change required |
| Documentation gate | Final named `doc check all` exited 1. No drift, 454 commands validated and complete help shape. Four foreign IRR anchors remain, with no changed-hub-paragraph or BMP-guide finding | Changed claims verified. Global result disclosed |

### Known Shared-Tree Reds

| Check | Scope and result |
|-------|------------------|
| BMP/fixture scoped lint | Host and Linux each report `plugin_fixture_07_irr.go` errorlint on a possibly nil diagnostic error, and repeated `AS-TEST` in that file and `clear_caches_fixture.go`. No finding in this spec's files |
| Hub scoped lint | Five variants completed. Other-file findings: `local_bridge_match_test.go` hugeParam, `bootstrap_credentials.go` G101 command placeholder, `bootstrap_certificate_login_test.go` embedlit and `config_upload_ssh_test.go` noctx. No changed-source/unit finding |
| Repository structure | Three IRR number-parse allowlist findings in the component IRR plugin and system IRR config/legacy files. No BMP finding |
| Test weakening audit | Five foreign RFC-tagged test changes in link-local multihop, labeled propagation and IKE ordered-SPD tests. No BMP finding |
| Documentation check | Global exit 1: `docs/architecture/resolve.md` names missing store `Open`; `docs/guide/irr-filtering.md` names missing `clearResolverCache` and `readmitRetryInterval`; an IRR paragraph in `docs/guide/plugins.md` names missing `ConfigReads`. Changed hub/BMP claims have no finding |
| Repository config corpus | 3308 Ze and 242 migrated configs inspected. The corrected BMP snapshots accepted. Aggregate 31 findings are other interface, DDOS and Darwin XFRM inputs |

The retained complete scoped lint receipt is
`tmp/session/2026-11-15-130223/scratch/job-bmp-fixture-final-lint-9bc28ce5.log`.
Its purpose is distinguishing the shared package findings from this spec's
files through closure. Remove it after the compact committed record supersedes
that comparison. These findings change no BMP acceptance criterion.

Final documentation receipt:
`tmp/session/2026-11-15-125434/scratch/job-fixes-final-docs-f119ecea.log`.
It bounds the four shared IRR anchor findings and the unchanged own guide
claims through closure. Remove it when the owning committed record supersedes
this comparison or the IRR author provides a superseding check.

### Security Review

| Check | Result |
|-------|--------|
| Resource release and transaction integrity | Staged map is local until complete, failure closes every stage socket, old map/cap/roots stay unchanged |
| Injection and secrets | Server keys are quoted by `configtext.QuoteCommandWord`. Tests cover bare/spaced/pipe names. No credentials are added to production |
| Bounds and concurrency | Setup loops are bounded by config sets. Existing mutex serializes reconciliation with stop. New accepted-session work retains existing ownership |
| External transport and authorization | Existing SDK/TLS path and failure policy unchanged, with explicit external startup/reload proof |
| Companion startup refusal | Existing extractor policy enforced before components. `config-semantics` doctor already declares `CodeConfigHubInvalid`. Refusal/stderr/log contain name, length and repair, never the supplied token |

### RFC Scope

This changes local listener installation, callback outcomes and startup refusal. It changes no
BMP wire behavior, RFC requirement row or tagged test. No new conformance claim
or whole-tree RFC check is owed by this closure.
