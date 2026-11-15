# Spec: runtime-peer-plugin-capabilities

| Field | Value |
|-------|-------|
| Status | blocked |
| Scope | plugin |
| Priority | P1 |
| Release reason | Shipped-behaviour defect: a reload adding a peer that carries a plugin capability (`role`, `software-version`, `hostname`, `graceful-restart`, `link-local-next-hop`, the ExaBGP bridge's codes) is refused, against the owner rule that a peer with any of its settings joins the running config without a restart |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Implementation Status

Implementation complete; next step `/ze-close` (Review Gate, then the two
closure commits). Not closed.

| Commit | What landed |
|--------|-------------|
| 22051cb019 | Reactor: a reload judges plugin capabilities on an unchanged peer (`Session.pluginCapabilitiesKept`), `TestReloadPluginCapabilityChange` |
| f3b68fe48f | Test peer: each session writes its own established-file marker |
| 93bd019e2d | Reactor: a capability-block edit restarts only when the plugin capability did, `TestReloadCapabilityBlockEdit` |
| bb50677cb4 | SDK verify answer carries the candidate declaration, refusal deleted, per-plugin tests, docs (AC-7), seven reload `.ci` (five refused files replaced), judge-approved |

The closure review on 2026-11-15 found missing reload interoperability proof
and stale SDK instructions. The implementation phase must supply those before
the independent Review Gate can finish. See Blocker and the closure sections.

## Blocker

| Blocker | Producing surface | What clears it |
|---------|-------------------|----------------|
| A live plugin-capability replacement has no qualifying interoperability proof | `internal/component/bgp/reactor/peer_settings_negotiation.go` `pluginCapabilitiesKept` changes the restart decision, and `internal/component/plugin/server/config_tx_capabilities.go` `installStagedCapabilities` replaces what the next OPEN sends. `internal/le/interoplab/bgp/checkers.go` `bgpRoleFRROperations` observes startup only; `check_softver.go` `checkSoftwareVersionFRR` also observes startup only | Add and run a native scenario against another BGP implementation that exercises a capability declaration changed or added by reload; prove that its assertion fails when that runtime replacement is broken |
| AC-7 still has stale SDK guidance | `docs/plugin-development/handlers.md`, "DeclareAppliesNothing", says the SDK refuses capability edits and shows the removed two-argument `DeclareCapabilitiesFrom` call; `pkg/plugin/sdk/capability_declaration.go` `composeCapabilityVerify` now answers with the candidate and `DeclareCapabilitiesFrom` takes only the extractor | Correct the paragraph and example against the current SDK, then check the affected documentation |
| The current AC-1 run stopped during host setup | `internal/test/runner` loopback setup reported `loopback_address_missing` and could not add `127.0.0.2` to `lo0`; no product assertion ran | Make that address available through the approved host setup, or run the unchanged test on a host that supplies it, then observe the named test result |

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
| A-1 | Every apply of a reload goes through `phaseKind.runRPC` phaseApply before the reactor reconciles peers | `config_tx_bridge.go` runRPC comment (orchestrator apply and executor section apply both reach it) | A new peer could build its OPEN before install | the new `.ci` | unvalidated |
| A-2 | The reactor builds a new peer's OPEN when the session starts, reading the store then | `peer.go` `getPluginCapabilities`, `session_negotiate.go` `prepareOpen` | Peer would carry the startup caps | the new `.ci` | unvalidated |

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

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| TestComposedVerifyCarriesCandidateCapabilities | `pkg/plugin/sdk/capability_declaration_test.go` | AC-2 | |
| TestReplacePluginCapabilities | `internal/component/plugin/registration_test.go` | AC-3 | |
| TestConfigTxBridgeInstallsAndRestoresCapabilities | `internal/component/plugin/server/` | AC-4 | |
| TestNegotiationPluginCapabilityChange | `internal/component/bgp/reactor/` | AC-5 | |
| per-plugin `capability_reload_test.go` (role, softver, gr, llnh, hostname, bridge) | each plugin | AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| reload-add-peer-role-capability | `test/reload/` | operator adds a role peer by reload | |
| the seven refusal `.ci` tests | `test/reload/` | converted to the accepted behaviour (judge-gated) | |

### Interop Tests (Scope: protocol)
Required: the OPEN encoding is reused, but a reload now changes the declaration
that reaches it and the running session's restart decision. The startup-only
`bgp-role-frr` and `frr-software-version` scenarios do not exercise that change.
The missing native reload scenario remains work of this spec.

## Files to Modify

- `pkg/plugin/rpc/types.go`, `pkg/plugin/sdk/capability_declaration.go`
- `internal/component/plugin/registration.go`, `internal/component/plugin/server/config_tx_bridge.go`, `reload_tx.go`
- `internal/component/bgp/reactor/peer_settings_negotiation.go`, `session_negotiate.go`, `session.go`
- Docs: `docs/guide/config-reload.md`, `docs/guide/plugins.md`, `docs/plugin-development/handlers.md`, `docs/architecture/api/process-protocol.md`

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
- No product fix was made by the closing agent. The missing interop proof and stale SDK instructions are open in Blocker.

### Scoped Unit Evidence
- On 2026-11-15 the main thread ran the native package action with the AC-2 to AC-5 selector across the SDK, plugin infrastructure, plugin server, reactor, five BGP capability plugins and ExaBGP bridge plugin: exit 0, ten package `ok` lines, no failures, skips or no-test warnings. The runner emitted no per-test PASS names, so this records the selected package run, not separate observed results for each named test.
- The retained output is `tmp/session/2026-11-15-130223/scratch/job-unit-pkg-be6b0d3d.log`. It preserves the package results for this unresolved closure. Remove it after the completed implementation and closure record supersedes this handback evidence.

### Documentation Updates
- The prerequisite commit `e7c999863a` adds the deliverables, security and documentation checklists to this spec.
- Existing guide and process-protocol claims were checked against the current SDK, capability stage and injector. The `DeclareAppliesNothing` instructions still need the repair named in Blocker; no documentation check result is claimed.

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
| A reload carries each plugin's candidate declaration | Implemented; final proof pending | `capability_declaration.go` `composeCapabilityVerify` | The scoped unit batch exited 0; native reload interop remains owed |
| A peer added by reload sends its configured capabilities | Implemented; proof pending | `config_tx_capabilities.go` `installStagedCapabilities` | The current AC-1 run stopped at host setup |
| A running session is restarted or kept by the negotiated-outcome rule | Implemented; proof pending | `peer_settings_negotiation.go` `pluginCapabilitiesKept` | Native interop proof is missing |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Open proof | `reload-add-peer-role-capability.ci` | Host setup prevented the current run |
| AC-2 | Scoped batch passed | `TestComposedVerifyCarriesCandidateCapabilities` and each capability extractor's tests | Selected package run exited 0; no individual test result was printed |
| AC-3 | Scoped batch passed | `TestReplacePluginCapabilities`, `TestReplacePluginCapabilitiesConflict`, `TestCapabilityTransactionAtomicOPENReplacement` | Selected package run exited 0; no individual test result was printed |
| AC-4 | Scoped batch passed | `TestConfigTxBridgeInstallsAndRestoresCapabilities` | Selected package run exited 0; no individual test result was printed |
| AC-5 | Open interop proof | `TestNegotiationPluginCapabilityChange`, `TestReloadPluginCapabilityChange`, `TestReloadCapabilityBlockEdit` | Scoped batch exited 0; native interop proof remains owed |
| AC-6 | Done | `reload-capability-unchanged-accepted` | Main observed PASS for named test 99 on 2026-11-15, 2.5 seconds |
| AC-7 | Open repair | `docs/plugin-development/handlers.md`, "DeclareAppliesNothing" | Stale refusal and obsolete call remain |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| SDK, injector, bridge and reactor tests | Scoped batch passed | The test files named in Deliverables Checklist | Ten selected package results were `ok`; no per-test PASS names were printed |
| `reload-add-peer-role-capability` | Present; run blocked | `test/reload/reload-add-peer-role-capability.ci` | Loopback setup, before product execution |
| `reload-capability-unchanged-accepted` | Pass | `test/reload/reload-capability-unchanged-accepted.ci` | Main observed the named result |
| Interop for a reload changing plugin capabilities | Missing | Current startup scenarios do not exercise reload | Remains within this spec |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| SDK, injector, transaction bridge and reactor producers | Present and inspected | Source locations are named in Deliverables Checklist |
| Four documentation pages | Present; one repair owed | The obsolete SDK apply-handler guidance is named in Blocker |

### Audit Summary
- The audit is incomplete. AC-6 has a named passing run, and the scoped unit batch exited 0 for ten packages; no final completion count is claimed.
- No item was skipped, removed or transferred to another spec.

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A peer added by reload sends its plugin capabilities and establishes | Functional and interop | The AC-1 functional file directly asserts the OPEN bytes and End-of-RIB; its current run stopped during host setup. Native interop proof remains missing |
| A changed plugin capability restarts only a session whose negotiated use can change | Unit, functional and interop | The reactor package's selected AC-5 unit run exited 0; qualifying native interop proof remains owed |
| An unchanged declaration does not disturb the session | Functional | Main observed `reload-capability-unchanged-accepted` PASS on 2026-11-15, one TCP connection, accepted reload |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Native reload interop proof, AC-1 execution, SDK documentation repair and final independent Review Gate | Closure is handed back to implementation; no scope is removed | `plan/release/fixes/spec-runtime-peer-plugin-capabilities.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | None; no CLEAN verdict was recorded |
| `./le spec review check` | Not run; blocking proof and documentation findings remain |
| Rounds | Round 1 stopped with open findings; not a completed clean review |
| Reviewer lenses used | Wiring and goal evidence; SDK refusal precedence, ownership replacement and compensation; running-session comparison |
| Round 1 scope | Current producers and the implementation commits named above, including the later complete-set replacement fix `942ca06d90`; the generic review remains incomplete |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Missing native interop proof for reload capability replacement and session decisions | `installStagedCapabilities`, `pluginCapabilitiesKept`; startup-only scenario producers | Open, retained in this spec |
| 2 | ISSUE | SDK guidance still asserts the removed refusal and obsolete API call | `docs/plugin-development/handlers.md`, "DeclareAppliesNothing" | Open, retained in this spec |

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
| AC-1, AC-5 interop, AC-7 | Final verification remains open | AC-1 stopped during host setup, the native interop proof is absent, and the SDK documentation needs repair |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Daemon reload adds a role peer | `test/reload/reload-add-peer-role-capability.ci` | The file exercises the intended path; the current run did not reach it because loopback setup failed |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1, A-2 | Unresolved at handback | `installStagedCapabilities` and the reload path were inspected; the planned AC-1 validation run has no product verdict yet |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config-verify carries a candidate and the store installs/restores it atomically | `composeCapabilityVerify`, `stageVerifiedCapabilities`, `ReplacePluginCapabilities`, `restoreCapabilities` | Current guide/protocol claims match the inspected producers; checks remain owed |
| SDK apply-handler instructions | `DeclareCapabilitiesFrom`, `composeCapabilityVerify` | Failed: stale refusal claim and obsolete API example, retained in Blocker |
