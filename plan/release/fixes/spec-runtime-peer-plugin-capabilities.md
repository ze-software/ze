# Spec: runtime-peer-plugin-capabilities

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P1 |
| Release reason | Shipped-behaviour defect: a reload adding a peer that carries a plugin capability (`role`, `software-version`, `hostname`, `graceful-restart`, `link-local-next-hop`, the ExaBGP bridge's codes) is refused, against the owner rule that a peer with any of its settings joins the running config without a restart |
| RFC gaps | - |
| Depends | - |
| Phase | interop proof and SDK documentation repair |
| Handoff | - |
| Updated | 2026-11-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Implementation Status

The runtime implementation is committed. The implementation follow-up adds
reload interoperability proof and repairs the SDK guidance before closure.

| Commit | What landed |
|--------|-------------|
| 22051cb019 | Reactor: a reload judges plugin capabilities on an unchanged peer (`Session.pluginCapabilitiesKept`), `TestReloadPluginCapabilityChange` |
| f3b68fe48f | Test peer: each session writes its own established-file marker |
| 93bd019e2d | Reactor: a capability-block edit restarts only when the plugin capability did, `TestReloadCapabilityBlockEdit` |
| bb50677cb4 | SDK verify answer carries the candidate declaration, refusal deleted, per-plugin tests, docs (AC-7), seven reload `.ci` (five refused files replaced), judge-approved |

The closure review on 2026-11-15 found missing reload interoperability proof
and stale SDK instructions. The implementation phase must supply those before
the independent Review Gate can finish. See Remaining Verification and the closure sections.

## Remaining Verification

| Blocker | Producing surface | What clears it |
|---------|-------------------|----------------|
| Independent closure review remains pending | Both Role reload scenarios have observed candidate-withholding reds followed by unmodified greens. The snapshot regression and affected/direct-builder selected unit runs exited 0. Nine proof records were refreshed natively, scoped freshness found no stale entries, and four audits were rejudged independently | Finish the independent Review Gate using the scoped results and their limits |
| AC-7 focused documentation review remains active | The obsolete refusal and two-argument example in `docs/plugin-development/handlers.md` are repaired against `composeCapabilityVerify` and the one-argument `DeclareCapabilitiesFrom`; the matching SDK godoc is corrected. Required `doc check all` exited 1 with 136 drift findings, including concurrent IRR and baseline changes | Record the focused independent review of this change's documentation; do not claim the all-doc check passed |

## Task

Owner rule (`ai/META-DECISIONS.md`, commit 4e632b786c): an operator adds a
peer, with any of its settings, to the running configuration without
restarting Ze; a reload refusing a new peer for what it configures is a
defect. Today `sdk.Plugin.DeclareCapabilitiesFrom` composes a config-verify
check that refuses any candidate whose capability declaration differs from
the Stage 3 one (`capabilityChanges`, `capabilityRefusal` in
`pkg/plugin/sdk/capability_declaration.go`), because the engine holds the
Stage 3 declaration for the process life and has no message that replaces it.

The fix makes the declaration follow the configuration: every reload carries
each capability plugin's candidate declaration to the engine, which installs
it before any participant applies, so a peer the reload adds sends an OPEN
with its plugin capabilities, and a running session whose plugin capability
changes is restarted or kept by the existing negotiation-outcome ruling.

The closure follow-up also makes the plugin declaration recorded for an OPEN
come from the same store snapshot as its wire capabilities. The reload probe
must compare and build its candidate from one snapshot too, so a transaction
that publishes B and restores A cannot hide an OPEN that actually carried B.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/le-tooling.md`
  → Constraint: `pkg/plugin/rpc` field names are wire; a new field is optional and absent means "nothing new", so a plugin in another language keeps working.
  → Constraint: the SDK and plugin server name no plugin; the engine reaches capabilities only through the declaration.
- [ ] `docs/architecture/api/process-protocol.md` "The Stage 3 declaration is fixed for the plugin process's life"
  → Decision: that paragraph is replaced; the declaration is re-sent in the config-verify answer.
- [ ] `docs/architecture/core-design.md` config reload, and `docs/architecture/api/ipc_protocol.md` answer framing (design pages the touched code declares)
  → Constraint: the verify answer stays one framed JSON answer; the new field is additive.
- [ ] `docs/architecture/config/transaction-protocol.md` Phases 1-3
  → Constraint: verify is non-destructive; nothing the engine installs may be visible before apply starts, and a rollback or abort must restore it.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5492.md`
  → Decision: RFC 5492 Section 1: "A BGP speaker that supports a particular capability may use this capability with its peer after the speaker determines (as described above) that the peer supports this capability." Capabilities exist only in OPEN; there is no in-session change, so a changed capability reaches the peer only through a new OPEN.

### Interoperability evidence (RFC silent on reconfiguration)
- BIRD `proto/bgp/bgp.c` `bgp_reconfigure`: `int same = !memcmp(((byte *) old) + sizeof(struct proto_config), ...)` over the whole `struct bgp_config` (which holds `local_role`), so any capability config change restarts the session.
- FRR `bgpd/bgpd.c` `peer_role_set`: stores `peer->local_role = role` and calls `bgp_session_reset(peer)` only when strict mode turns on and the peer sent no Role; otherwise the new role reaches the peer at the next OPEN.
- The two disagree; Ze's agreed design decides: the owner ruling of 2026-08-07 implemented by `Session.negotiationOutcomeUnchanged` keeps the session only when the negotiation would come out the same, and restarts otherwise (fail-closed). A plugin capability whose negotiation the engine cannot model is judged by its bytes: unchanged, or the peer never advertised that code (RFC 5492: the capability is then not in use), keeps the session; otherwise restart.

## Current Behavior (MANDATORY)

- [ ] `pkg/plugin/sdk/capability_declaration.go` - extractor runs once at Stage 2; `composeCapabilityVerify` refuses a candidate declaration that differs for a live selector or adds one for a new peer.
- [ ] `internal/component/plugin/server/startup.go` `onCapabilities` - Stage 3 sets `proc.SetCapabilities` and `capInjector.AddPluginCapabilities`; never rewritten.
- [ ] `internal/component/plugin/registration.go` `CapabilityInjector` - add with conflict check, remove per plugin; no replace.
- [ ] `internal/component/plugin/server/config_tx_bridge.go` `phaseKind.runRPC` - verify answer read for status/error only.
- [ ] `internal/component/bgp/reactor/peer_settings_negotiation.go` `negotiationOutcomeUnchanged` - compares negotiated outcome; its comment relies on the store never changing during a reload.
- [ ] `internal/component/bgp/reactor/session_negotiate.go` `buildOpen`, `prepareOpen` - plugin capabilities read from the store when the OPEN is built.

**Behavior to preserve:** a reload that changes no plugin capability is accepted and changes no session; an ExaBGP bridge config change stays refused by its own check; a capability conflict between two plugins is still refused.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Capability extractor registry | `pkg/plugin/sdk/capability_declaration.go` `DeclareCapabilitiesFrom`, `CapabilityExtractor`; six registrants (bgp-role, bgp-softver, bgp-gr, bgp-llnh, bgp-hostname, exabgp bridge internal and external) | Pure function config sections to declaration, run at Stage 2 and at verify | Extends: the verify-time result is sent to the engine instead of being compared and refused |
| Stage 3 declaration | `startup.go` `engineStartupSink.onCapabilities`, `capabilitiesFromRPC` | Installs the declaration once | Extends: the same conversion installs a reload's declaration |
| Capability injector | `internal/component/plugin/registration.go` `CapabilityInjector` | Per-selector store read at every OPEN build | Extends: an atomic per-plugin replace with the same conflict checks, plus a dry-run check used at verify |
| Config transaction bridge | `config_tx_bridge.go` `configTxBridge`, `phaseKind.runRPC`; `reload_tx.go` `runTxCoordinator` | Sends config-verify/apply RPCs per participant, maps answers to acks | Extends: verify stages the declaration per participant; the first apply of the transaction installs every staged one; a rolled-back or aborted transaction restores the previous ones |
| Peer settings swap-or-restart | `peer_settings_negotiation.go` `negotiationOutcomeUnchanged` (owner ruling 2026-08-07), `peer_settings_apply.go` | Keeps a running session when the negotiation would land the same | Extends: also compares the plugin capabilities the sent OPEN carried with the ones the store holds now, for codes the peer advertised |
| Reload delivers plugin config | `docs/architecture/config/transaction-protocol.md` Phases 1-3 | Verify then apply, per plugin, with the candidate sections | Extends: the verify answer carries the candidate declaration (new optional `ConfigVerifyOutput.Capabilities`) |
| `CheckCapabilityReload` test helper | `capability_declaration.go` | Lets a plugin test prove the refusal | Replaced: by `sdk.CandidateCapabilities`, which returns the declaration the SDK would send, so a plugin test proves a new peer gets its capability |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator reload (SIGHUP, `ze config reload`, commit) with a new or changed peer.

### Transformation Path
1. Engine sends `config-verify` with the candidate sections to each affected plugin.
2. SDK runs the plugin's verify handler, then its extractor on the candidate sections, and answers OK with `capabilities` (the whole candidate declaration).
3. Bridge checks the declaration converts and does not conflict with another plugin's; a conflict answers verify-failed naming both plugins.
4. On the transaction's first apply RPC, the bridge replaces each staged plugin's injector entries and its process record, once.
5. The BGP reactor applies its diff: a new peer builds its OPEN from the store, which now holds its plugin capabilities; a running peer's swap probe compares the sent plugin capabilities with the store's.
6. On rollback or abort, the previous declarations are re-installed.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin to engine | `ConfigVerifyOutput.Capabilities` (JSON over RPC, or DirectBridge with the same struct) | [ ] |
| Plugin server to reactor | the injector store, read by `GetPluginCapabilitiesForSelectors` | [ ] |

### Integration Points
- `capabilitiesFromRPC` - reused for the reload declaration.
- `Session.negotiationOutcomeUnchanged` - gains the plugin capability comparison.

### Architectural Verification
- [ ] No plugin, feature or config path named in SDK, server or reactor code
- [ ] Absent `capabilities` in a verify answer keeps the plugin's current declaration (non-Go plugins unchanged)
- [ ] No second path: Stage 3 and reload use one conversion and one injector

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|---------------------------------|----------|--------------|--------|
| A-1 | Every apply of a reload goes through `phaseKind.runRPC` phaseApply before the reactor reconciles peers | `config_tx_bridge.go` runRPC comment (orchestrator apply and executor section apply both reach it) | A new peer could build its OPEN before install | AC-1 native QEMU functional PASS observed by main | confirmed |
| A-2 | The reactor builds a new peer's OPEN when the session starts, reading the store then | `peer.go` `getPluginCapabilities`, `session_negotiate.go` `prepareOpen` | Peer would carry the startup caps | AC-1 exact new-peer OPEN and establishment assertions passed | confirmed |

### Risks
| Risk | Mitigation |
|------|------------|
| A session reconnecting between install and rollback sends the candidate capabilities | Restore happens when the transaction leaves apply; the reconnect window is the rollback's own duration, the same window every other applied setting has |
| Two plugins' candidates conflict with each other | Install fails, apply fails, transaction rolls back with the conflict text |

## Blast Radius

The SDK (public), the RPC type (wire, additive), the plugin server reload path, the reactor swap probe, six capability plugins' reload tests, seven reload `.ci` tests that asserted the refusal.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Reload adding a role peer (`ze` reload of config2) | → | `composeCapabilityVerify`, `configTxBridge` install, `CapabilityInjector.ReplacePluginCapabilities`, `buildOpen` | `test/reload/reload-add-peer-role-capability.ci` |

## Acceptance Criteria

| AC | Assertion |
|----|-----------|
| AC-1 | A reload that adds a peer with `role` is accepted, the new peer's OPEN carries capability 9 with the configured value (wire hex in the `.ci`), and the session establishes |
| AC-2 | For every capability plugin (enumerated by `DeclareCapabilitiesFrom` registrants), a candidate that adds a peer carrying its capability yields a verify answer with that peer's declaration (unit, per plugin) |
| AC-3 | The injector replace is atomic and refuses a conflict with another plugin, leaving the store unchanged (unit) |
| AC-4 | A rolled-back or aborted reload restores the previous declaration (unit on the bridge) |
| AC-5 | A running session whose plugin capability changes restarts when the peer advertised that code, and keeps running when it did not (unit on `negotiationOutcomeUnchanged`) |
| AC-6 | A reload that changes no capability changes no session (existing `reload-capability-unchanged-accepted.ci` stays green) |
| AC-7 | The three operator pages and the protocol page no longer call it a restart-only change |
| AC-8 | OPEN construction and the reload probe use coherent raw-declaration/wire snapshots. After publication of B and restoration of A, the keep decision agrees with the actual OPEN bytes. Either complete generation may be chosen at construction; a code the peer never advertised retains the existing permission to keep the session |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestComposedVerifyCarriesCandidateCapabilities | `pkg/plugin/sdk/capability_declaration_test.go` | AC-2 | |
| TestReplacePluginCapabilities | `internal/component/plugin/registration_test.go` | AC-3 | |
| TestConfigTxBridgeInstallsAndRestoresCapabilities | `internal/component/plugin/server/` | AC-4 | |
| TestNegotiationPluginCapabilityChange | `internal/component/bgp/reactor/` | AC-5 | |
| per-plugin `capability_reload_test.go` (role, softver, gr, llnh, hostname, bridge) | each plugin | AC-2 | |
| TestPrepareOpenKeepsPluginSnapshotAcrossRollback | `internal/component/bgp/reactor/session_plugin_snapshot_test.go` | AC-8: deterministic publication between the old two reads, then restoration | Main observed red before the repair; the affected unit selector exited 0 after it |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| reload-add-peer-role-capability | `test/reload/` | operator adds a role peer by reload | |
| the seven refusal `.ci` tests | `test/reload/` | converted to the accepted behaviour (judge-gated) | |

### Interop Tests (Scope: protocol)
Required: the OPEN encoding is reused, but a reload now changes the declaration
that reaches it and the running session's restart decision. The startup-only
`bgp-role-frr` and `frr-software-version` scenarios do not exercise that change.
The two native reload scenarios below supply that proof.

| Scenario | Foreign observation | Status |
|----------|---------------------|--------|
| `bgp-role-reload-frr` | FRR's received Role changes from undefined to provider and back after applied SIGHUP reloads, each on a new established session | Main observed initial PASS, candidate-withholding FAIL at assertion 4, then final unmodified PASS after the snapshot repair |
| `bgp-role-reload-frr-forked` | The same transitions with `bgp-role` over real TLS connect-back | Main observed candidate-withholding FAIL at assertion 4, then final unmodified PASS after the snapshot repair |

## Files to Modify

- `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/capability_declaration.go`
- `internal/component/plugin/registration.go`, `internal/component/plugin/server/config_tx_bridge.go`, `reload_tx.go`
- `internal/component/bgp/reactor/peer_settings_negotiation.go`, `session_negotiate.go`, `session.go`
- Docs: `docs/guide/config-reload.md`, `docs/guide/plugins.md`, `docs/plugin-development/handlers.md`, `docs/architecture/api/process-protocol.md`
- Follow-up: `pkg/plugin/sdk/sdk_callbacks.go` and `internal/le/interoplab/bgp/check_softver.go` (stale comments only), `docs/architecture/testing/interop.md` (the new reload scenarios)
- Snapshot repair: `internal/component/bgp/reactor/peer.go`, `peer_run.go`, `session.go`, `session_negotiate.go`, `peer_settings_negotiation.go`, `peer_gr_flags.go`, `peer_settings_plugin_capability_test.go`; the judge-approved helper comments in `rfc4724_restart_state_test.go`; `docs/architecture/behavior/peer-lifecycle.md` and `docs/architecture/meta/role.md`

## Files to Create

- `internal/le/interoplab/bgp/register_role_reload.go` - internal and forked scenario registration
- `internal/le/interoplab/bgp/check_role_reload.go` - applied reload and foreign Role/session observations
- `internal/le/interoplab/bgp/check_role_reload_test.go` - controlled-lab sequence and missing-evidence failures
- `internal/component/bgp/reactor/session_plugin_capabilities.go` - one immutable raw-declaration/wire snapshot contract
- `internal/component/bgp/reactor/session_plugin_capabilities_fixture_test.go` - wire-only fixtures for unchanged existing tagged test bodies
- `internal/component/bgp/reactor/session_plugin_snapshot_test.go` - deterministic publication/rollback regression
- `test/interop/scenarios/bgp-role-reload-frr/ze.conf`, `ze-reload.conf`, `frr.conf` - internal plugin proof
- `test/interop/scenarios/bgp-role-reload-frr-forked/ze.conf`, `ze-reload.conf`, `frr.conf` - external plugin proof

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| AC-1: a peer added by reload sends its configured Role and establishes | Run `./le --name session test bgp reload --pattern reload-add-peer-role-capability`; its new peer compares the OPEN bytes, including `09 01 03`, then receives End-of-RIB |
| AC-2: each capability extractor returns the candidate declaration through config-verify | Read `pkg/plugin/sdk/capability_declaration.go` `composeCapabilityVerify` and `CandidateCapabilities`; run `TestComposedVerifyCarriesCandidateCapabilities` and the `capability_reload_test.go` tests of every current `DeclareCapabilitiesFrom` registrant |
| AC-3: replacement publishes one complete set and refuses conflicts without mutation | Read `internal/component/plugin/registration.go` `ReplacePluginCapabilities`, `replacementLocked` and `planLocked`; run `TestReplacePluginCapabilities`, `TestReplacePluginCapabilitiesConflict` and `TestCapabilityTransactionAtomicOPENReplacement` |
| AC-4: an unsuccessful transaction restores the previous declarations | Read `internal/component/plugin/server/config_tx_capabilities.go` `installStagedCapabilities` and `restoreCapabilities`, and their callers in `reload_tx.go`; run `TestConfigTxBridgeInstallsAndRestoresCapabilities` |
| AC-5: a changed capability restarts only a session that can use it | Read `internal/component/bgp/reactor/peer_settings_negotiation.go` `pluginCapabilitiesKept` and `pluginDeclarationsKept`; run `TestNegotiationPluginCapabilityChange`, `TestReloadPluginCapabilityChange` and `TestReloadCapabilityBlockEdit`, then the changed-capability reload functional tests |
| AC-6: a reload with unchanged capabilities keeps the session | Run `./le --name session test bgp reload --pattern reload-capability-unchanged-accepted`; its peer permits one TCP connection and the fixture confirms that the reload applied |
| AC-7: operator and plugin-author instructions describe live replacement | Compare the four pages listed in Files to Modify with the current SDK, transaction bridge and reactor producers; record the documentation check result at closure |

### Security Review Checklist

| Check | What to verify |
|-------|----------------|
| Refusal precedence | `composeCapabilityVerify` preserves a plugin's own refusal and refuses an unreadable answer or failed extractor before publishing any candidate |
| Declaration validation and ownership | `stageVerifiedCapabilities` accepts only transaction section verifiers; `capabilitiesFromRPC` and `planLocked` validate the declaration, encoding and code ownership before replacement |
| Complete candidate and atomic publication | `CheckReplacePluginCapabilities` checks all staged owners with unchanged owners retained; `ReplacePluginCapabilities` holds its write lock across publication, so concurrent OPEN readers cannot observe a partial replacement |
| Transaction compensation | `installStagedCapabilities` records the previous per-plugin declarations; every unsuccessful transaction reaches `restoreCapabilities`, including apply failure and abort, and restoration failures are reported |
| Session safety | `pluginCapabilitiesKept` compares the declaration actually sent, refuses to assume an unreadable peer OPEN is unchanged, and does not keep a session whose changed code the peer advertised |
| Process boundary | Candidate declarations are self-contained RPC values; the external config-verify answer and DirectBridge reach the same staging and publication path, with no plugin-specific exception |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update or source-aware check |
|---|----------|----------|-------------------------------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | Check `features/configuration-reload.md` against the new accepted reload cases and its real-path evidence; read the level from `./le feature report` before changing any declaration |
| 2 | Config syntax changed? | No | Existing capability leaves feed `CandidateCapabilities`; the implementation adds an RPC answer field, not YANG syntax |
| 3 | CLI command added/changed? | No | The existing reload entry point reaches the transaction bridge; no command registration changes in this implementation |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`, "A reload replaces the Stage 3 declaration through the config-verify answer": check absent, empty and complete-candidate semantics against `ConfigVerifyOutput.Capabilities` and `stageVerifiedCapabilities` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, "When a Reload Changes a Plugin's Capability": check acceptance, ownership transfer, rollback and session restart behavior against the bridge and `pluginDeclarationsKept` |
| 6 | Has a user guide page? | Yes | `docs/guide/config-reload.md`, "What Can Change Live": check the plugin-capability row and restart exceptions against the SDK, bridge and reactor |
| 7 | Wire format changed? | Yes | The plugin RPC answer adds `capabilities`; document its Stage 3 shape in `docs/architecture/api/process-protocol.md`. BGP OPEN encoding is reused; inspect its reload proof rather than claim that an unchanged encoding removes the interop obligation |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/handlers.md`, "DeclareCapabilitiesFrom": check own-verifier precedence, candidate extraction, the answer field and `CandidateCapabilities`; check the SDK rules against that public contract |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | Check `rfc/short/rfc5492.md` and the affected capability RFCs against the actual tagged tests and records; resolve the interop requirement for changed OPEN/session behavior before closure, without publishing new coverage from this checklist |
| 10 | Test infrastructure changed? | Yes | `f3b68fe48f` changes the test peer's established-file marker per session; compare its producer and current `docs/architecture/testing/ci-format.md` contract, and update that page if its claim is stale |
| 11 | Affects daemon comparison? | No | This repairs the existing live-reload path; no new protocol, command surface or backend is introduced. Recheck `docs/comparison.md` for a restart-only claim before closure |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/process-protocol.md` describes verify staging, complete-candidate validation and atomic install/restore; check `docs/architecture/config/transaction-protocol.md` keeps verify side-effect-free and installs before apply |
| 13 | Route metadata keys added/changed? | No | `CandidateCapabilities` and the injector exchange capability declarations; neither adds route metadata |
| 14 | Prometheus counters added/changed? | No | The SDK composition, capability stage, injector replacement and session comparison add no metric registration |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | The existing extractors and capability codes remain registered; this changes when their values are replaced, not the registered inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Source-anchor search finds the four pages in Files to Modify for `capability_declaration.go`, `config_tx_capabilities.go` and `peer_settings_negotiation.go`; verify every affected claim and run `./le doc check all` at closure |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Check the SDK extractor example in `docs/plugin-development/handlers.md` and the reload examples in the two operator guides against current handlers and the native functional proofs |

## Implementation Steps

1. Red `.ci` (AC-1), observed refused.
2. RPC field, SDK verify answer, injector replace, bridge stage/install/restore.
3. Reactor probe comparison.
4. Docs; per-plugin tests and refusal `.ci` conversions (judge-gated).

### Critical Review Checklist

| Check | What to verify for this implementation |
|-------|-----------------------------------------|
| Real reload | The native interop checker calls `applyZeReload`, which requires one applied configuration generation before reading the foreign peer |
| Foreign observation | FRR's `bgp_show_peer` reports the Role received from Ze and a new established-session generation after the Role is added and removed |
| Both transports | The internal and forked `bgp-role` scenarios execute the same checker and differ only in their plugin transport |
| Discrimination | Suppressing the candidate in `composeCapabilityVerify`, followed by a daemon and image rebuild, prevents FRR from reporting the newly configured Role |
| Fail closed | Missing peer output, a refused reload, an unchanged Role and an unchanged session generation fail the checker tests |
| SDK guidance | `DeclareAppliesNothing` describes engine-owned capability installation and the example calls the current one-argument `DeclareCapabilitiesFrom` API |

## Owned journal rows

None.

## Key Design Decisions

- The declaration rides the config-verify answer, not a new engine RPC: verify already runs the extractor on the candidate, the answer is ordered before every apply, and an abort needs no undo because nothing was installed.

## Known Limitations

- None planned.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Targeted tests pass: `./le job run label unit-pkg command go test -race -tags ze_bgp ./pkg/plugin/sdk/ ./internal/component/plugin/ ./internal/component/plugin/server/ ./internal/component/bgp/reactor/` and each capability plugin package; `./le test reload <ids>` for the reload `.ci` tests

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

## Implementation Summary

### What Was Implemented
- The SDK's `composeCapabilityVerify` carries `CandidateCapabilities` after the plugin's own verifier accepts.
- The server's `stageVerifiedCapabilities`, `installStagedCapabilities` and `restoreCapabilities` stage, publish and compensate complete declarations through `CapabilityInjector.ReplacePluginCapabilities`.
- `Session.pluginCapabilitiesKept` compares the declaration sent in the OPEN with the current store; the reload and swap paths consult it even when other peer settings are unchanged.

### Bugs Found/Fixed
- No product behavior fix was made by the closing agent. The implementation follow-up repairs the snapshot defect and SDK instructions and supplies both observed interop walks.
- Follow-up: the native Role reload scenarios and checker tests are written. The SDK paragraph, example and matching godoc now describe candidate installation. Both transports' red/green results are recorded below; final documentation checks remain owed.
- Follow-up: `Peer.pluginCapabilitySnapshot` now derives the raw declaration index before runtime GR-bit transforms and derives the wire offer from that same injector read. `prepareOpen` records that snapshot beside the OPEN; the negotiation probe compares and builds one captured candidate. Existing tagged test bodies and assertions are unchanged.

### Snapshot Defect Diagnosis

| Part | Evidence and decision |
|------|-----------------------|
| Symptom | Main observed `TestPrepareOpenKeepsPluginSnapshotAcrossRollback` FAIL: record 11, actual OPEN 22, then kept=true after restoration of 11 when false was required |
| Producer | `prepareOpen` called `pluginDeclarations`, then `buildOpen` independently called the wire-capability getter; `negotiationOutcomeUnchanged` paired the same two independent reads |
| Owning layer | Reactor session OPEN construction and reload comparison; each injector read was already atomic |
| Alternatives | [workaround] Change getter order or add a retry around the two reads. [source] Capture one paired raw/wire snapshot, use it to build and record OPEN, and use one snapshot for the candidate comparison and build |
| Why the source fix | Publication B followed by restoration A defeats a two-read ordering. One snapshot makes the retained declaration describe the actual wire offer, while retaining raw GR values separately from runtime flag edits |

Main then ran the native reactor package action with the regression and affected
negotiation, GR, Role/first-AS and OPEN-builder selector: exit 0, package `ok`.
No individual test PASS names were emitted. The retained red log is
`tmp/session/2026-11-15-130223/scratch/job-unit-pkg-b97229a0.log`, and the
post-fix selected run is `tmp/session/2026-11-15-130223/scratch/job-unit-pkg-97afac9e.log`.
They preserve the before/after evidence until the completed closure record
supersedes it, then are removed with the task's disposable proof artifacts.

### Native Interop Evidence

| Build | Native action and outcome | Immutable Ze image |
|-------|---------------------------|--------------------|
| Unmodified internal plugin | Main observed `./le --name session test qemu docker-lab lab 'test integration interop' env 'INTEROP_SCENARIO=bgp-role-reload-frr'` PASS: one scenario passed, zero failed, 45 seconds; no resources left and the VM stopped | `sha256:7c4ea418a42cf77adba6667dcc7bf5672b3314bffc642a7fdbd2e8449bcaf476` |
| Candidate withheld by source-pinned Go overlay | The same native scenario FAILS at assertion 4 after the applied add-Role reload: FRR's established generation stays 1 and the new-session wait times out. Main observed the scenario FAIL in 2 minutes 11 seconds, exit 1; no resources left and the VM stopped | `sha256:53fa88527660d5edb5cbff7a40685cd0cd642c92f1141d0b9681a70eceeba09a` |
| Candidate withheld, forked plugin | Main observed `bgp-role-reload-frr-forked` FAIL at the same assertion 4: the applied add-Role reload leaves FRR's established generation at 1. Scenario FAIL in 2 minutes 13 seconds, exit 1; no resources left and the VM stopped | `sha256:2f93351fd6168f611758c34a62a5041abca02e2b3578f7dad4631ff3c6a7199f` |
| Final unmodified internal plugin, after snapshot repair | Main observed `bgp-role-reload-frr` PASS in 59 seconds, one passed and zero failed; no resources left and the VM stopped | `sha256:071556eddb53ba34b67059a0a67d095ea7912a655a265f3e62586504b605894f` |
| Final unmodified forked plugin, after snapshot repair | Main observed `bgp-role-reload-frr-forked` PASS in 57 seconds, one passed and zero failed; no resources left and the VM stopped | `sha256:146bbf4024f880789855be101da299a071ceb81a2442b37d34ec0c033afa62de` |

The overlay replaces `composeCapabilityVerify`'s candidate assignment with an
unused-value statement and leaves tracked source unchanged. Its source input
is pinned to SHA-256 `1a1a48ad17202805b67db5272ea11c817877571302d4c3a4375ea037b6270fff`.
The task-owned `tmp/session/2026-11-15-130223/scratch/peer-caps-reload-discrimination/`
and preparation script hold this bounded proof setup. Remove them after all
red/green outcomes and image IDs are recorded here. Both candidate-withholding
reds and final unmodified greens are observed for each transport. The setup is
now disposable; retained logs need only support any unresolved check or review.

### Refreshed RFC Proofs

| Requirement | Entries refreshed by the native recorder | Preserved proof |
|-------------|-----------------------------------------|-----------------|
| RFC2918-2-1 | Positive | Actual `buildOpenFromSnapshot` producer-body break |
| RFC6793-4.1-1 | Positive and negative | `DisableASN4` condition forced false and true, respectively |
| RFC6793-4.2.1-1 | Positive | Paired AS_TRANS threshold breaks in `buildOpenFromSnapshot` and `message.writeFixedFields`; both individual breaks observed green before the combined red |
| RFC7705-4.2-3 | Negative | Actual `buildOpenFromSnapshot` producer-body break |
| DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-6-1 | Positive and negative | Actual `buildOpenFromSnapshot` producer-body break |
| RFC4724-4.1-4 and RFC4724-4.2-6 | Negative and positive, respectively | Actual `Peer.pluginCapabilitySnapshot` producer-body break |

Main reported all nine records successfully written after baseline PASS and
the planned observed red. Independent audit rejudgments refreshed
RFC4724-4.1-4, RFC4724-4.2-6, RFC9234-4.1-1 and RFC9234-4.1-2. No proof JSON
was edited by hand. Main then ran all nine scoped native freshness checks:
none reported stale entries. Existing unrelated unproven RFC2918 and RFC9234
obligations are unchanged; no whole-corpus coverage claim is made.

The additional direct-OPEN-builder unit selector also exited 0 with one package
`ok`; its retained output is `tmp/session/2026-11-15-130223/scratch/job-unit-pkg-863a511a.log`.
It is disposable after the final closure record supersedes this evidence.

### Scoped Unit Evidence
- On 2026-11-15 the main thread ran the native package action with the AC-2 to AC-5 selector across the SDK, plugin infrastructure, plugin server, reactor, five BGP capability plugins and ExaBGP bridge plugin: exit 0, ten package `ok` lines, no failures, skips or no-test warnings. The runner emitted no per-test PASS names, so this records the selected package run, not separate observed results for each named test.
- The retained output is `tmp/session/2026-11-15-130223/scratch/job-unit-pkg-be6b0d3d.log`. It preserves the package results for this unresolved closure. Remove it after the completed implementation and closure record supersedes this handback evidence.

### Documentation Updates
- The prerequisite commit `e7c999863a` adds the deliverables, security and documentation checklists to this spec.
- Existing guide and process-protocol claims were checked against the current SDK, capability stage and injector. The `DeclareAppliesNothing` paragraph, example and matching godoc are repaired. Required `./le --name session doc check all` exited 1 with 136 drift findings, including concurrent IRR and baseline changes; focused independent review is active.

### Validation Limits
- The initial scoped lint found one owned ST1005 diagnostic, corrected to lowercase. Its other findings were unrelated package baseline findings. Two final bounded three-package attempts stopped during planning because other sessions were changing `register_watchdog_withdraw_value.go` and then `reactor_api_sealed_session_test.go`; neither produced a new lint verdict. No lint pass is claimed. Independent manual style/source review found no issues.
- The affected reactor and incremental direct-builder native unit actions exited 0, with one package `ok` each and no individual PASS names printed. The latter output is retained above; the former is `tmp/session/2026-11-15-130223/scratch/job-unit-pkg-97afac9e.log`, disposable after closure supersedes it.

### Deviations from Plan
- The original interop exemption relied on unchanged encoding. The implemented reload changes what the peer receives and whether its session restarts, so interoperability proof is required.
- Closure is handed back to implementation. No acceptance criterion is removed or transferred.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Unchanged OPEN encoding was treated as an interop exemption | Reload changes the declaration sent and the session restart decision | Read `installStagedCapabilities`, `pluginCapabilitiesKept` and the startup-only FRR scenario producers | Add a native reload scenario in this spec |
| approach | AC-7 was described as implemented while the SDK apply-handler example retained the old refusal contract | `DeclareCapabilitiesFrom` takes one extractor and config-verify carries its candidate | Compared "DeclareAppliesNothing" with `DeclareCapabilitiesFrom` and `composeCapabilityVerify` | Repair the SDK instructions before closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A reload carries each plugin's candidate declaration | Scoped unit and interop proof passed | `capability_declaration.go` `composeCapabilityVerify` | Both transports have an observed missing-candidate red followed by unmodified green |
| A peer added by reload sends its configured capabilities | Functional proof passed | `config_tx_capabilities.go` `installStagedCapabilities` | Main observed the unchanged AC-1 native QEMU test PASS on Ze's own system |
| A running session is restarted or kept by the negotiated-outcome rule | Scoped unit and interop proof passed | `peer_settings_negotiation.go` `pluginCapabilitiesKept` | Both native FRR scenarios prove Role add/remove on new sessions; existing unit cases retain the unsupported-code survivor |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Pass | `reload-add-peer-role-capability.ci` | Main observed the unchanged native QEMU test PASS on Ze kernel 7.3.0-ze arm64, 3.8 seconds, one test executed and zero skipped |
| AC-2 | Scoped batch passed | `TestComposedVerifyCarriesCandidateCapabilities` and each capability extractor's tests | Selected package run exited 0; no individual test result was printed |
| AC-3 | Scoped batch passed | `TestReplacePluginCapabilities`, `TestReplacePluginCapabilitiesConflict`, `TestCapabilityTransactionAtomicOPENReplacement` | Selected package run exited 0; no individual test result was printed |
| AC-4 | Scoped batch passed | `TestConfigTxBridgeInstallsAndRestoresCapabilities` | Selected package run exited 0; no individual test result was printed |
| AC-5 | Scoped unit and interop proof passed | `TestNegotiationPluginCapabilityChange`, `TestReloadPluginCapabilityChange`, `TestReloadCapabilityBlockEdit`, both Role reload scenarios | Selected unit runs exited 0; both FRR transports completed discriminating red/green walks |
| AC-6 | Done | `reload-capability-unchanged-accepted` | Main observed PASS for named test 99 on 2026-11-15, 2.5 seconds |
| AC-7 | Repair written; check owed | `docs/plugin-development/handlers.md`, "DeclareAppliesNothing" | Candidate installation replaces the obsolete refusal; the example uses one extractor argument |
| AC-8 | Observed red, then selected unit proof passed | `TestPrepareOpenKeepsPluginSnapshotAcrossRollback` | The old two-read record/wire mismatch failed deterministically; the repaired snapshot selector exited 0 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| SDK, injector, bridge and reactor tests | Scoped batch passed | The test files named in Deliverables Checklist | Ten selected package results were `ok`; no per-test PASS names were printed |
| `reload-add-peer-role-capability` | Pass | `test/reload/reload-add-peer-role-capability.ci` | Main observed the named PASS through `./le --name session test qemu ci test test/reload/reload-add-peer-role-capability.ci`; the earlier macOS attempt stopped at loopback setup |
| `reload-capability-unchanged-accepted` | Pass | `test/reload/reload-capability-unchanged-accepted.ci` | Main observed the named result |
| Interop for a reload changing plugin capabilities | Red/green walks passed | `bgp-role-reload-frr` and its forked twin | Each completed applied add/remove reloads and FRR's received Role on new sessions, and failed with the candidate withheld |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| SDK, injector, transaction bridge and reactor producers | Present and inspected | Source locations are named in Deliverables Checklist |
| Four documentation pages | Present; repair written and checks owed | SDK apply-handler guidance now describes engine installation of the candidate and the current API |

### Audit Summary
- The audit is incomplete. AC-1 and AC-6 have named passing runs, and the scoped unit batch exited 0 for ten packages; no final completion count is claimed.
- No item was skipped, removed or transferred to another spec.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer added by reload sends its plugin capabilities and establishes | Functional and interop | Main observed the AC-1 functional PASS through the native QEMU action on Ze's own system, with exact OPEN and End-of-RIB assertions; both transport variants also completed live capability replacement against FRR |
| A changed plugin capability restarts only a session whose negotiated use can change | Unit, functional and interop | Selected AC-5 unit runs exited 0 and retain the peer-without-code survivor; both FRR transports observed the configured Role in a new session after each applied add/remove reload |
| An unchanged declaration does not disturb the session | Functional | Main observed `reload-capability-unchanged-accepted` PASS on 2026-11-15, one TCP connection, accepted reload |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Focused documentation review and independent Review Gate | Scoped proof, freshness and unit results are recorded with lint/doc limits; final independent review is active and no scope is removed | `plan/release/fixes/spec-runtime-peer-plugin-capabilities.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | None; no CLEAN verdict was recorded |
| `./le spec review check` | Not run; independent closure review remains pending |
| Rounds | Round 1 stopped with open findings; not a completed clean review |
| Reviewer lenses used | Wiring and goal evidence; SDK refusal precedence, ownership replacement and compensation; running-session comparison |
| Round 1 scope | Current producers and the implementation commits named above, including the later complete-set replacement fix `942ca06d90`; the generic review remains incomplete |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Missing native interop proof for reload capability replacement and session decisions | `installStagedCapabilities`, `pluginCapabilitiesKept`; startup-only scenario producers | Internal and forked carriers now have observed candidate-withholding reds and final unmodified greens |
| 2 | ISSUE | SDK guidance asserted the removed refusal and obsolete API call | `docs/plugin-development/handlers.md`, "DeclareAppliesNothing" | Paragraph, example and matching SDK godoc repaired; documentation checks owed |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/reload/reload-add-peer-role-capability.ci` | Yes | Read its full body: daemon reload, exact new-peer OPEN bytes and End-of-RIB |
| `test/reload/reload-capability-unchanged-accepted.ci` | Yes | Read its full body: one connection and applied reload assertion |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-6 | Unchanged capabilities keep the session | Main observed the named functional PASS on 2026-11-15 |
| AC-2 to AC-5 | Scoped package verification | The main thread's native selected batch exited 0, with ten package `ok` lines; no individual PASS names were printed |
| AC-1 | The newly added peer sends its configured Role and establishes | Main observed `reload-add-peer-role-capability` PASS through `./le --name session test qemu ci test test/reload/reload-add-peer-role-capability.ci` on Ze kernel 7.3.0-ze arm64, one executed and zero skipped |
| AC-5, AC-8 | Native reload and snapshot proof | Both FRR transport walks completed red then green; the deterministic snapshot regression failed before the source repair and its post-fix selected run exited 0 |
| AC-7 | Final verification remains open | Repaired documentation awaits its final check |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Daemon reload adds a role peer | `test/reload/reload-add-peer-role-capability.ci` | Main observed the named native QEMU PASS; its exact OPEN and End-of-RIB assertions ran |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1, A-2 | Confirmed | Main observed the planned AC-1 functional proof PASS on Ze's own system; the new peer established after reload with the configured Role bytes |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config-verify carries a candidate and the store installs/restores it atomically | `composeCapabilityVerify`, `stageVerifiedCapabilities`, `ReplacePluginCapabilities`, `restoreCapabilities` | Current guide/protocol claims match the inspected producers; checks remain owed |
| SDK apply-handler instructions | `DeclareCapabilitiesFrom`, `composeCapabilityVerify` | The paragraph, one-argument example and matching godoc are repaired against source; documentation checks remain owed |
