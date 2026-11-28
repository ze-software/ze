# Spec: BGP one-off defects from the journal

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Umbrella routing the remaining shipped BGP one-off defects (wire, RIB, FSM, plugin relay, and protocol test/interop proofs) by subsystem door while retiring 33 already-fixed journal rows and 22 already-fixed acceptance criteria with HEAD evidence |
| RFC gaps | - |
| Depends | - |
| Phase | split-by-door umbrella |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Set: this file is the umbrella of `spec-bgp-one-off-defects-<N>-<name>.md`, children 1 to 3, all in `plan/release/fixes/`. Per `ai/META-DECISIONS.md` ("a spec whose edits need many doors is split into narrower child specs"), the 99 open journal rows that survived the 2026-11-26 HEAD audit are partitioned with zero overlap across three single-door child specs (`bgp-engine.md`, `bgp-plugins.md`, `le-tooling.md`). The umbrella edits no code itself, holds the retirement ledger for every defect already fixed at HEAD, holds zero `## Owned journal rows` of its own, and closes once all three children close.

## Task

The 2026-10-23 journal sweep gathered 67 BGP one-off defects and later amendments added rows up to 132 owned journal entries spanning BGP engine packages, BGP plugins, MRT, and functional/interop test suites. On 2026-11-26 every acceptance criterion and owned journal row was audited against HEAD code and git history:

- **33 owned journal rows** (covering 22 whole or partial acceptance criteria, plus AC-62 and the MRT callback type half of AC-24) are already fixed at HEAD and are retired below into `## Existing Mechanisms` and `## Rows found already fixed`, with exact file, symbol, and commit SHA evidence. All MRT one-off rows (`AC-50`, `AC-58` MRT abbreviated `MP_REACH_NLRI`, `AC-62` ADD-PATH Path ID, and the `AC-24` callback type) landed in `238bdb62e9`, `381e927417`, `4db24861ee`, and `d07e4dc944`, leaving zero open `system-plugins.md` MRT work in this set.
- **99 open journal rows** remain and are split cleanly by subsystem door with zero duplicate AC or journal ownership across three release-fix child specs:
  1. `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md` (`ai/doors/bgp-engine.md`, 60 open rows)
  2. `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md` (`ai/doors/bgp-plugins.md`, 24 open rows)
  3. `plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md` (`ai/doors/le-tooling.md`, 15 open rows)

Standing architectural constraints that every child spec enforces:
1. **Ze always starts (2026-11-26 ruling, `docs/architecture/core-design.md`)**: no new BGP configuration refusal may exit `ze start`; invalid per-peer or per-service configuration refuses at commit, reload, and `ze config validate`, while daemon start leaves only the affected peer or endpoint unstarted and reports it under a declared doctor code in logs and `ze doctor`.
2. **Process-boundary parity (`docs/architecture/plugin/plugin-system.md`, `docs/architecture/api/process-protocol.md`)**: BGP plugin fixes work both internal and forked; transient report-bus issues from forkable plugins use `sdk.Plugin.RaiseIssue` through `issuequeue` rather than direct `internal/core/report` calls, register every code with `diagnostic.Register` and `diagnostic.RegisterEventCode`, and declare emitted EventBus events in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`.
3. **Wire-visible protocol proofs and test governance**: every wire-visible fix names its interop scenario under `test/interop/scenarios/` (or records an explicit departure); every test expectation or RFC-tag change is judged by `ze-test-judge`; functional plugin/LG tests live under `test/plugin/*.ci`.

## Children

| # | Spec | Subsystem Door | Open Rows | Scope | Status |
|---|------|----------------|-----------|-------|--------|
| 1 | `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md` | `ai/doors/bgp-engine.md` | 60 | Reactor, FSM, wire/wireu, BGP config, BGP CLI decode, BGP server/format, core BGP attributes | design |
| 2 | `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md` | `ai/doors/bgp-plugins.md` | 24 | BGP plugins (`cmd/peer`, `cmd/announce`, `cmd/update`, `filter_community`, `filter_prefix`, `rib`, `adj_rib_in`, `rs`, `rr`, `bmp`, `rpki`, `nlri/*`) | design |
| 3 | `plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md` | `ai/doors/le-tooling.md` | 15 | `.ci` functional tests, `test/interop/scenarios/`, `rfc/short/` enrollment, `docs/architecture/testing/interop.md` | design |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, public `pkg/` API, compile-out boundaries, and operator message rules
  → Constraint: every child fix must survive running plugins forked, keep compile-out isolation, and produce operator messages naming what failed, why, and the exact fix.
- [ ] `docs/architecture/core-design.md` - 2026-11-26 "Ze always starts" design decision and component boundaries
  → Constraint: no new configuration refusal may exit `ze start`; bad service or peer config stays down with a declared doctor code while `ze` starts with its working services, whereas commit, reload, and `ze config validate` still refuse.
- [ ] `plan/journal/README.md` - owned journal row rules and `fixed:` marker format
  → Constraint: every open row belongs to exactly one child spec's `## Owned journal rows` table; retiring a row requires HEAD commit evidence and flipping the row's Fix marker to `fixed: `.

**Key insights:**
- Auditing HEAD before designing retired 33 of 132 journal rows and all MRT defects (`system-plugins.md`), allowing a clean three-door split with zero cross-door sprawl.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/message/rfc7606.go` (`validateAttributeFlags`) - delegates to `attribute.AttributeCode.FlagsConflict` (`c6f6672895`), covering all registered attributes including `0xC0` AIGP discard (`rfc7311_aigp_test.go`).
- [ ] `internal/component/bgp/fsm/fsm.go` (`handleOpenSent`, `handleOpenConfirm`, `handleEstablished`) - ignores Start events with no state or counter change (`42bdb05df3`), and recovers panicking FSM callbacks (`898caba3f6`).
- [ ] `internal/component/bgp/reactor/session_hold_expiry.go`, `session_write.go`, `peer_bfd.go`, `session_open_as.go`, `session_handlers.go`, `reactor_notify.go`, `forward_pool.go`, `peer_initial_sync_test.go`, `rfc8654_directional_session_test.go` - verified HEAD fixes for AC-2, AC-14, AC-19, AC-24 (callback type), AC-35, AC-40, AC-57, `validateOpenPeerAS`, `takeOverflowReleased`, `delay_open_test`, and `reactor_connection` clock.
- [ ] `internal/component/bgp/config/routeattr.go` (`parseRawAttributeInto`) - refuses duplicate generic and typed attributes per RFC 4271 Section 6.3 (`6399061087`).
- [ ] `internal/component/bgp/plugins/rib/rib_replay.go` (`resendRoutesWithCursor`), `storage/attrparse.go`, `plugins/adj_rib_in/rib.go` (`handleReceived`), `plugins/rpki/rtr_cache_restart_test.go`, `server/events.go` (`procRequiresPeerUpBarrier`) - verified HEAD fixes for AC-6, AC-7 storage/rebuilds, AC-59, AC-60, and the RPKI restart test race.
- [ ] `internal/plugins/mrt/dump.go`, `component.go` - verified HEAD fixes for AC-50, AC-58 (MRT abbreviated `MP_REACH_NLRI`), AC-62 (ADD-PATH Path ID), and AC-24 (callback type).
- [ ] `internal/core/network/md5_freebsd.go` - verified `TCPMD5Supported()` returns `false` on FreeBSD (`2cfa63332e`).

**Behavior to preserve:**
- Every already-fixed mechanism retired in `## Existing Mechanisms` stays green and is never regressed by child-spec work.

**Behavior to change:**
- None in the umbrella itself; all remaining behavior repairs are owned by Children 1 to 3.

## Existing Mechanisms (MANDATORY)

Every defect retired at HEAD is recorded here with exact symbol and commit evidence so no child spec duplicates solved work:

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Registry-derived attribute flag validation & AIGP discard (AC-8, AC-17) | `internal/component/bgp/message/rfc7606.go` `validateAttributeFlags`; `internal/core/bgp/attribute/flags_spec.go` `FlagsConflict`; `internal/component/bgp/message/rfc7311_aigp_test.go` `TestRFC7606AIGPTransitiveIsDiscarded` | Validates every registered attribute code from its registration metadata and discards `0xC0` AIGP (`c6f6672895`) | Does not apply: AC-8 and AC-17 retired as already fixed at HEAD |
| FSM Start-event ignore & callback panic recovery (AC-9, AC-23 part 2) | `internal/component/bgp/fsm/fsm.go` `handleOpenSent`, `handleOpenConfirm`, `handleEstablished`, `FSM.Event` | Ignores Start events in OpenSent/OpenConfirm/Established (`42bdb05df3`) and recovers panicking callbacks without crashing the daemon (`898caba3f6`) | Does not apply: AC-9 and FSM panic half of AC-23 retired as already fixed at HEAD |
| Hold and Send-Hold timer lock-free expiry (AC-2) | `internal/component/bgp/reactor/session_hold_expiry.go` `holdTimerExpiredLocked`; `session_write.go` `sendHoldTimerExpired`; `rfc9687_rib_release_test.go` `TestRFC9687Event29ReleasesTheLivePeersRIB` | Fires `FSM.Event` without `Session.mu` held (`1c6795e9e4`, `51aa251107`, `86b381c3b4`) | Does not apply: AC-2 retired as already fixed at HEAD |
| Non-waiting BFD client release (AC-14) | `internal/component/bgp/reactor/peer_bfd.go` `stopBFDClient`, `releaseBFDClient`, `awaitBFDRelease` | Separates non-waiting release from subscriber join so a BFD Down callback cannot wait on itself (`9886507d51`) | Does not apply: AC-14 retired as already fixed at HEAD |
| Refused resend accounting (AC-6) | `internal/component/bgp/plugins/rib/rib_replay.go` `RIBManager.resendRoutesWithCursor` | Returns `(resent, refused int)` and counts refused commands into `refused`, never `resent` (`b561f1c5e1`) | Does not apply: AC-6 retired as already fixed at HEAD |
| Partial bit preservation in RIB storage, AS4 conversion, community rebuilds, and Prefix-SID cleanup (AC-7 phases 1-4) | `internal/component/bgp/plugins/rib/storage/attrparse.go` (`c0fad815db`); `wireu/aspath_collapse.go` & `aspath_transcode.go` (`30d31d0b49`); `plugins/filter_community/handler.go` (`313de90fee`); `reactor/filter_delta_handlers.go` & `forward_prefix_sid.go` (`6c22530c23`) | Preserves received Partial bits across storage, AS4 width conversion, community rebuilds, and Prefix-SID cleanup | Extends in Child 1: remaining typed `Flags()` / peer-visible end-to-end wire proof moves to Child 1 |
| Labeled withdrawal 0x800000 framing across walks (AC-19) | `session_prefix.go` `forEachPrefixEntry`, `server/codec.go` `decodeMPNLRIs`, `nlritype.go` `Retain`, `plugins/nlri/{vpn,labeled}` `DecodeNLRIHex`, `rib_validation.go` `reconcileReceivedNLRIs`, `rib_recovery.go` `recoverySentNLRI` | Every withdrawn walk takes explicit withdrawal framing (`445c5e3f97`, `691bd048e9`, `af69577952`) | Does not apply: AC-19 retired as already fixed at HEAD |
| MRT TABLE_DUMP_V2 SAFI, abbreviated MP_REACH_NLRI, ADD-PATH Path ID, and peer callback type (AC-50, AC-58 MRT, AC-62, AC-24 callback) | `internal/plugins/mrt/dump.go` `writeTableDumpV2` (`381e927417`, `4db24861ee`, `d07e4dc944`); `internal/component/bgp/reactor/reactor_notify.go` `callbackAdapter.OnPeerEstablished` (`238bdb62e9`) | Emits `RIB_GENERIC` / `RIB_GENERIC_ADDPATH`, abbreviates `MP_REACH_NLRI` per RFC 6396 §4.3.4, carries 4-octet ADD-PATH Path ID via `OnRouteAddPath`, and adapts `*Peer` to `*plugin.PeerInfo` | Does not apply: all MRT defects retired as already fixed at HEAD |
| Adj-RIB-In complex-family JSON ingest & peer-up barrier registration identity (AC-59, AC-60) | `internal/component/bgp/plugins/adj_rib_in/rib.go` `handleReceived` (`b60d8b92a5`, `44411d90c3`); `internal/component/bgp/server/events.go` `procRequiresPeerUpBarrier` (`9779726169`) | Routes non-simple families through `installComplexNLRIsWithFilter` preserving RD per NLRI; resolves peer-up barrier via `proc.RegistrationName()` | Does not apply: AC-59 and AC-60 retired as already fixed at HEAD |
| Duplicate raw attribute refusal, RFC 8654 asymmetric receive limit, RFC 7313 unknown subtype ignore, no-family EOR, initial-sync hold removal, ordinary selected-route egress, FreeBSD MD5 check, superseded route store deletion, legacy FlowSpec reader deletion, and reactor hygiene fixes (AC-1a, AC-23c, AC-34 FreeBSD, AC-35, AC-39, AC-40, AC-43b/c, AC-55, plus 6 unnumbered rows) | `config/routeattr.go` `parseRawAttributeInto` (`6399061087`); `rfc8654_directional_session_test.go` (`a936e853c7`, `737e5494f6`); `session_handlers.go` `screenRouteRefresh` (`d7bccb81ed`); `peer_initial_sync_test.go` `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily` (`df44d8d279`); `peer_initial_sync.go` (`9c93f82bb4`); `a27a951239`; `5682b2c17c`; `md5_freebsd.go` (`2cfa63332e`); `1ba6055caf`; `1b1b36cb5f`; `ca7f979d74`; `c8a7d38c3f`; `7825b5ec49`; `d4737dfa1b`; `86b381c3b4`; `3917f8277d` | Each defect is repaired and covered at HEAD | Does not apply: all retired with commit evidence in `## Rows found already fixed` |
| Journal ownership and door-scoped spec sets | `internal/le/spec/journal/owner.go` `UnownedOpenRows`; `internal/le/spec/status/specset.go` `GroupSets` | Tracks open journal rows across all specs and groups bare umbrella stems with numbered children | Extends: splits the remaining 99 open rows cleanly across Children 1 to 3 with zero duplicate ownership |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | Every child spec preserves process-boundary parity; no child adds direct cross-process calls into daemon state. |
| `docs/architecture/core-design.md` | Design decisions | 2026-11-26 owner ruling ("Ze always starts") | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code." | agrees | Children 1 to 3 enforce this ruling: new BGP config refusals reject at commit, reload, and `ze config validate`, while `ze start` keeps unaffected peers running and reports any unstarted peer or endpoint under a declared doctor code. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- No runtime code is edited by this umbrella; each child spec traces its own entry points (BGP wire/session/config in Child 1, BGP plugin RPC/bus/command rails in Child 2, functional/interop test harness in Child 3).

### Transformation Path
1. Audit every owned journal row and acceptance criterion against HEAD.
2. Retire already-fixed rows into `## Existing Mechanisms` and `## Rows found already fixed` with exact file, symbol, and commit SHA evidence, flipping their Fix marker in `plan/journal/` to `fixed: `.
3. Route each remaining open defect and journal row to the single child spec matching its subsystem door.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Umbrella ↔ Child specs 1–3 | Disjoint `## Owned journal rows` tables and `## Work Not Done` cross-links | No |

### Changed-Wire Census

**Search evidence:** No runtime or wire code is modified by this umbrella (`## Files to Modify` and `## Files to Create` are empty). Every wire-visible change and its paired writer/reader census is owned by Child 1 (`plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md`) and Child 2 (`plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md`).

### Integration Points
- `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md` - BGP engine defects (`ai/doors/bgp-engine.md`).
- `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md` - BGP plugin defects (`ai/doors/bgp-plugins.md`).
- `plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md` - Functional tests, interop scenarios, and RFC short enrollment (`ai/doors/le-tooling.md`).
- `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` - Sibling release fix owning RFC 2545 condition-flip re-advertisement (the re-advertisement half of historical AC-10).

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Umbrella changes no code; each child stays within its single subsystem door |
| No unintended coupling (components stay isolated) | Yes | Three child specs split cleanly on `bgp-engine.md`, `bgp-plugins.md`, and `le-tooling.md` |
| No duplicated functionality (extends existing, does not recreate) | Yes | 33 already-fixed rows retired into `## Existing Mechanisms` rather than re-implemented |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Child 1 preserves pooled buffer-first encoding on every wire fix |
| Registration over hardcoding, outbound | Yes | Child 1 and Child 2 preserve registry-driven dispatch and doctor code registration |
| Registration over hardcoding, inbound | Yes | No central switch or hardcoded list added by any child spec |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All 33 retired rows are genuinely repaired at HEAD and need no further product edit | Verified against HEAD source files and git commits on 2026-11-26 | Re-open the specific row in its door-scoped child spec | Source inspection and commit log review | validated |
| A-2 | The 99 open journal rows partition cleanly with zero overlap across Children 1 to 3 | Verified across all 132 original rows (`33 + 60 + 24 + 15 = 132`) | Any misrouted row moves cleanly to its true door child | `UnownedOpenRows` check at commit time | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An open row in Child 1 or Child 2 turns out to be already fixed when its red test is written | Red-first test passes immediately at HEAD | Move the row to that child's `## Existing Mechanisms` / `## Rows found already fixed` with HEAD commit evidence and flip its journal Fix marker to `fixed: ` |
| R-2 | Child 1 `peerOnLink` work overlaps with `spec-rfc2545-readvertise-on-condition-flip.md` | Both touch `internal/component/bgp/reactor/link_scope.go` | Child 1 restricts its scope to the receiving-interface scope guard (`peerOnLink`) and leaves condition-flip re-advertisement to `spec-rfc2545-readvertise-on-condition-flip.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Only spec routing or journal ownership accounting; umbrella modifies no runtime code |
| How is it reverted? | Single commit revert of the spec and journal-marker updates |
| Who else touches this path? | `spec-rfc2545-readvertise-on-condition-flip.md`, `spec-bgp-ls-receiver-accept-and-limit-controls.md`, `spec-doctor-coverage-6-bgp-engine.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Spec set discovery and journal ownership | → | `specset.GroupSets` and `specjournal.UnownedOpenRows` | `./le spec design check spec plan/release/fixes/spec-bgp-one-off-defects.md` and per-child wiring tests in Children 1 to 3 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-U1 | All three child specs (`1-bgp-engine`, `2-bgp-plugins`, `3-tests-and-interop`) complete and close | Every one of the 99 remaining open BGP one-off journal rows is repaired with red-first evidence or retired with HEAD proof, and all three child specs are closed | None: every owned row must reach a verified `fixed:` state before this umbrella closes |
| AC-U2 | Cross-spec journal ownership check runs across the umbrella and all three children | Every open journal row from the original 132-row table is owned by exactly one child spec with zero duplicate rows and zero unowned open rows | The 33 retired rows are marked `fixed:` in `plan/journal/` and recorded in `## Rows found already fixed`, so they need no open owner |
| AC-U3 | Sibling overlap with `spec-rfc2545-readvertise-on-condition-flip.md` | Condition-flip re-advertisement stays owned by `spec-rfc2545-readvertise-on-condition-flip.md` while Child 1 owns `peerOnLink` receiving-interface scope without duplicating work | Sibling spec owns re-advertisement so neither spec duplicates `reactor_readvertise.go` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| Child 1 unit tests | `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md` | All BGP engine defects | |
| Child 2 unit tests | `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md` | All BGP plugin defects | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Child 1–3 `.ci` functional tests | `test/plugin/*.ci`, `test/parse/*.ci`, `test/encode/*.ci` | End-to-end daemon, plugin, and config behavior | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| Child 1 and Child 3 interop scenarios | `test/interop/scenarios/` | FRR / GoBGP / BIRD | Wire-visible BGP compliance across real peers | |

## Files to Modify
- None (umbrella routes work to Children 1 to 3 and holds the retirement ledger).

## Files to Create
- None (Children 1 to 3 are sibling specs listed under `## Children`).

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | Umbrella changes no code; tracked per child |
| YANG validation constraints | N-A | Umbrella changes no code; tracked per child |
| YANG custom validators | N-A | Umbrella changes no code; tracked per child |
| CLI commands/flags | N-A | Umbrella changes no code; tracked per child |
| CLI grammar (keyword before value) | N-A | Umbrella changes no code; tracked per child |
| Editor autocomplete | N-A | Umbrella changes no code; tracked per child |
| Functional test for new RPC/API | N-A | Umbrella changes no code; tracked per child |
| Pipe completeness | N-A | Umbrella changes no code; tracked per child |
| Env var registration | N-A | Umbrella changes no code; tracked per child |
| Doctor check for runtime dependencies | N-A | Umbrella changes no code; tracked per child |
| Doctor code per problem class | N-A | Umbrella changes no code; tracked per child |
| Prometheus counters/metrics | N-A | Umbrella changes no code; tracked per child |
| BGP family surface (new SAFI / capability / attribute) | N-A | Umbrella changes no code; tracked per child |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Bug-fix umbrella only |
| 2 | Config syntax changed? | No | Umbrella edits no config |
| 3 | CLI command added/changed? | No | Umbrella edits no CLI |
| 4 | API/RPC added/changed? | No | Umbrella edits no RPCs |
| 5 | Plugin added/changed? | No | Umbrella edits no plugins |
| 6 | Has a user guide page? | No | Tracked per child |
| 7 | Wire format changed? | No | Tracked per child |
| 8 | Plugin SDK/protocol changed? | No | Tracked per child |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | Tracked in Child 1 and Child 3 (`rfc/short/`) |
| 10 | Test infrastructure changed? | No | Tracked in Child 3 |
| 11 | Affects daemon comparison? | No | No daemon comparison change |
| 12 | Internal architecture changed? | No | Tracked per child |
| 13 | Route metadata keys added/changed? | No | No new metadata keys |
| 14 | Prometheus counters added/changed? | No | Tracked per child |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Tracked in Child 2 |
| 16 | Any changed source file referenced by existing doc source anchors? | No | Umbrella modifies no source files (`./le spec citation anchors spec plan/release/fixes/spec-bgp-one-off-defects.md` exits 0) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Tracked per child |

## Design presentation

### What changes

This umbrella retires 33 already-fixed BGP journal rows (including all MRT one-off defects and 22 whole or partial acceptance criteria verified at HEAD) into `## Existing Mechanisms` and `## Rows found already fixed`, and splits the remaining 99 open rows cleanly by subsystem door into three focused release-fix child specs: BGP engine (`1-bgp-engine`), BGP plugins (`2-bgp-plugins`), and BGP tests/interop/RFC enrollment (`3-tests-and-interop`). Operators see no direct change from the umbrella itself; each child delivers its door's fixes independently.

### Before and after

1. **Multi-door spec gating before:** editing `plan/release/fixes/spec-bgp-one-off-defects.md` required loading five unrelated doors (`bgp-engine.md`, `bgp-plugins.md`, `system-plugins.md`, `commands-cli.md`, `le-tooling.md`) and re-auditing 33 already-fixed defects alongside 99 open ones. **After:** the umbrella edits no code and holds the verified retirement ledger; each child edits files under a single subsystem door (`bgp-engine.md`, `bgp-plugins.md`, or `le-tooling.md`), with zero duplicate journal or AC ownership.
2. **MRT and retired RFC/engine defects before:** `AC-2`, `AC-6`, `AC-8`, `AC-9`, `AC-14`, `AC-17`, `AC-19`, `AC-35`, `AC-39`, `AC-40`, `AC-50`, `AC-55`, `AC-57`, `AC-58` (MRT), `AC-59`, `AC-60`, `AC-62`, and 6 unnumbered journal rows appeared open in the spec despite already landing on `main`. **After:** every retired defect is documented with exact file, symbol, and commit SHA evidence, and its `plan/journal/` Fix marker is `fixed: `.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep all 99 open rows in one monolithic spec | Violates `ai/META-DECISIONS.md` ("a spec whose edits need many doors is split into narrower child specs") and forces every implementation session to load three unrelated doors |
| Split into one spec per individual defect (60+ micro-specs) | Violates `ai/META-DECISIONS.md` ("The number of specs is kept as low as the work allows"); one child per subsystem door matches `spec-dropped-error-0-umbrella` |
| Leave the 33 already-fixed rows in `## Owned journal rows` | Keeps solved defects in release-fix scope and wastes implementation time rediscovering landed commits |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. Implement and close `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md` (`ai/doors/bgp-engine.md`).
2. Implement and close `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md` (`ai/doors/bgp-plugins.md`).
3. Implement and close `plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md` (`ai/doors/le-tooling.md`).
4. Close this umbrella once all three child specs have closed.

## Work Not Done

All remaining open work from the original 132-row table is owned by these sibling/child specs (zero work left unowned):

- `plan/release/fixes/spec-bgp-one-off-defects-1-bgp-engine.md` - 60 open BGP engine journal rows (AC-4, AC-5, AC-7 wire proof, AC-10 `peerOnLink`, AC-12 engine fixtures, AC-13, AC-15, AC-18, AC-21, AC-23 RUnlock, AC-24 EOR/capture timestamp, AC-26, AC-29, AC-30, AC-34 macOS note, AC-36 human decode, AC-37, AC-38, AC-41 engine tags, AC-44 `SendRoutes`, AC-45, AC-46, AC-47, AC-49, AC-51, AC-52, AC-53 engine gaps, AC-61, and open reactor journal rows).
- `plan/release/fixes/spec-bgp-one-off-defects-2-bgp-plugins.md` - 24 open BGP plugin journal rows (AC-1 `destination` keyword & `cmd/update`, AC-3, AC-16, AC-22, AC-25, AC-27 `rr` log level, AC-31, AC-32, AC-33, AC-36 VPN best, AC-41 plugin tags, AC-43 `LARGE_COMMUNITY`, AC-44 `announceAndTrack`/`DecodeNLRIHex`/`lsNodeFlagBits`, AC-53 plugin gaps, AC-56, AC-58 MVPN Leaf A-D, AC-63, and open plugin journal rows).
- `plan/release/fixes/spec-bgp-one-off-defects-3-tests-and-interop.md` - 15 open test/fixture/interop/RFC-short journal rows (AC-11, AC-12 `test/parse`, AC-20, AC-28, AC-42, AC-48, AC-54, `new-v6.ci` RFC2545-3-2 tag, and open FRR interop rows).
- `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` - RFC 2545 condition-flip re-advertisement (`refreshPeerLinkScopes` re-advertise half of historical AC-10).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Rows found already fixed

| Class file | Date | Surface | Evidence |
|------------|------|---------|----------|
| `test-asserts-superseded-behaviour` | 2026-10-30 | reactor RS tests lack the fast-path receive grant | `c8a7d38c3f` (already `fixed:` in journal) |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `reconcileReceivedNLRIs` | HEAD `internal/component/bgp/plugins/rib/rib_validation.go` uses `nlrisplit.SplitWithdrawn` (already `fixed:` in journal) |
| `grace-bound-hides-a-missing-signal` | 2026-08-15 | initial-sync hold waits for a ready only bgp-rib sends (AC-55) | `9c93f82bb4` removed `apiSyncTimeout` wait (already `fixed:` in journal) |
| `unwired-feature` | 2026-08-17 | normal selected-route egress (AC-43 part 2) | `a27a951239` ordinary `bgp-rib` selected-route dissemination (already `fixed:` in journal) |
| `test-reads-producer-state-unlocked` | 2026-10-30 | rpki cache-restart test races `cacheGroup.load` | `ca7f979d74` (`internal/component/bgp/plugins/rpki/rtr_cache_restart_test.go`) |
| `assertion-reads-state-the-scheduler-owns` | 2026-10-27 | `TestRFC4271DelayOpenBothSidesHold` closed-pipe deadline | `7825b5ec49` (`internal/component/bgp/reactor/delay_open_test.go`) |
| `clock-bypass` | 2026-10-27 | `reactor_connection.go` forward write deadline | `d4737dfa1b` (`internal/component/bgp/reactor/reactor_connection.go`) |
| `guard-keyed-on-a-field-its-consumer-ignores` | 2026-10-26 | `validateOpenPeerAS` reads ASN4 capability with `asn4 disable` | `86b381c3b4` (`internal/component/bgp/reactor/session_open_as.go` passes `!settings.DisableASN4`) |
| `bulk-rename-corruption` | 2026-09-13 | `flowspec_legacy_test.go` (AC-1 part 1) | `1b1b36cb5f` deleted the legacy flow route reader and test |
| `callback-reenters-held-lock` | 2026-10-22 | BGP hold/send-hold timer callbacks (AC-2) | `1c6795e9e4`, `51aa251107`, `86b381c3b4` (`session_hold_expiry.go`, `session_write.go`, `rfc9687_rib_release_test.go`) |
| `counter-counts-the-wrong-packets` | 2026-10-13 | `resendRoutesWithCursor` (AC-6) | `b561f1c5e1` (`internal/component/bgp/plugins/rib/rib_replay.go` returns `(resent, refused int)`) |
| `derived-flags-overwrite-what-the-wire-carried` | 2026-09-20 | `validateAttributeFlags` (AC-8) | `c6f6672895` (`internal/component/bgp/message/rfc7606.go` delegates to `FlagsConflict` in `internal/core/bgp/attribute/flags_spec.go`) |
| `error-answered-with-another-errors-code` | 2026-10-05 | `handleOpenSent` (AC-9) | `42bdb05df3` (`internal/component/bgp/fsm/fsm.go` ignores Start events in OpenSent, OpenConfirm, Established) |
| `goroutine-waits-on-its-own-exit` | 2026-10-13 | `Peer.stopBFDClient` (AC-14) | `9886507d51` (`internal/component/bgp/reactor/peer_bfd.go` splits `releaseBFDClient` from `awaitBFDRelease`) |
| `guard-added-to-one-half-of-a-pair` | 2026-09-05 | AIGP receive, `rfc7606.go` (AC-17) | `c6f6672895` (`internal/component/bgp/message/rfc7311_aigp_test.go` `TestRFC7606AIGPTransitiveIsDiscarded`) |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `forEachPrefixEntry` (AC-19) | `445c5e3f97`, `af69577952` (`session_prefix.go` passes `!sec.announce`) |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `decodeMPNLRIs` (AC-19) | `445c5e3f97` (`server/codec.go` passes `true` for `WithdrawnBytes`) |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `nlritype.Retain` (AC-19) | `691bd048e9` (`internal/core/bgp/nlri/nlritype/nlritype.go` takes `announce bool`) |
| `guard-added-to-one-half-of-a-pair` | 2026-10-03 | `DecodeNLRIHex` (AC-19) | `445c5e3f97` (`plugins/nlri/{vpn,labeled}` `DecodeNLRIHex` takes `withdraw bool`) |
| `paraphrase-becomes-the-authority` | 2026-10-13 | `recoverySentNLRI` (AC-19) | `445c5e3f97` (`internal/component/bgp/plugins/rib/rib_recovery.go`) |
| `refcount-released-outside-the-lock` | 2026-08-18 | rib route store `releaseRoute` (AC-23 part 3) | `aae53cb1b8` fixed under lock; `1ba6055caf` deleted the superseded store |
| `declared-format-contradicts-payload` | 2026-10-07 | MRT `writeTableDumpV2` ignores SAFI (AC-50) | `381e927417`, `4db24861ee` (`internal/plugins/mrt/dump.go` uses `TDV2RIBGeneric`/`TDV2RIBGenericAP`) |
| `declared-format-contradicts-payload` | 2026-10-30 | MRT TABLE_DUMP_V2 RIB entry carries MP_REACH_NLRI unabbreviated (AC-58 MRT) | `4db24861ee` (`internal/plugins/mrt/dump.go` abbreviates code 14 per RFC 6396 §4.3.4) |
| `green-that-could-not-have-been-red` | 2026-08-31 | `handleRouteRefresh` unknown subtype not ignored (AC-57) | `d7bccb81ed` (`session_handlers.go` `screenRouteRefresh` ignores unknown subtypes via `routeRefreshSubtypeUnknown`) |
| `silent-fall-through` | 2026-09-01 | `setTCPMD5Sig` FreeBSD (AC-34 FreeBSD) | `2cfa63332e` (`internal/core/network/md5_freebsd.go` `TCPMD5Supported()` returns `false`) |
| `silent-fall-through` | 2026-08-16 | peer with no `family` block (AC-35) | `df44d8d279` (`peer_initial_sync_test.go` `TestInitialSyncEORSentWhenNeitherSideDeclaredAFamily`) |
| `unguarded-goroutine-kills-the-process` | 2026-10-01 | bgp fsm callback panic (AC-23 part 2) | `898caba3f6` (`internal/component/bgp/fsm/fsm.go`) |
| `wire-carries-both-arms-of-an-either-or` | 2026-10-18 | `parseRawAttribute` (AC-39) | `6399061087` (`internal/component/bgp/config/routeattr.go` `parseRawAttributeInto`) |
| `green-that-could-not-have-been-red` | 2026-10-05 | RFC8654-6-1 asymmetric receive limit (AC-40) | `a936e853c7`, `737e5494f6` (`internal/component/bgp/reactor/rfc8654_directional_session_test.go`) |
| `unwired-feature` | 2026-09-07 | `interface-event-reactions.md` cease subcode 6 paragraph (AC-43 part 3) | `5682b2c17c` removed the stale paragraph from `docs/architecture/bgp/interface-event-reactions.md` |
| `parked-work-returns-out-of-order` | 2026-11-13 | forward pool `takeOverflowReleased` | `3917f8277d` (`internal/component/bgp/reactor/forward_pool.go`) |
| `declared-format-contradicts-payload` | 2026-10-20 | AC-59: adj-rib-in `handleReceived` VPN NLRI RD | `b60d8b92a5`, `44411d90c3` (`internal/component/bgp/plugins/adj_rib_in/rib.go` `installComplexNLRIsWithFilter`) |
| `gate-excludes-part-of-its-population` | 2026-09-02 | AC-60: `countPeerUpBarrier` by registration name | `9779726169` (`internal/component/bgp/server/events.go` `procRequiresPeerUpBarrier`) |
| (no journal row; AC-62) | 2026-11-14 | MRT TABLE_DUMP_V2 ADD-PATH Path Identifier | `d07e4dc944`, `4db24861ee` (`internal/plugins/mrt/dump.go` `OnRouteAddPath`) |
| (no journal row; AC-24 part 2) | 2026-10-13 | MRT `onPeerEstablished` / `onPeerClosed` callback type | `238bdb62e9` (`internal/component/bgp/reactor/reactor_notify.go` `callbackAdapter`) |

## Review Gate

### AC-7 historical storage review (landed in `c0fad815db`, `30d31d0b49`, `313de90fee`, `6c22530c23`)

| Run | Scope | Independent reviewer | Result |
|-----|-------|----------------------|--------|
| Storage 1 | RIB storage Partial bit preservation (`storage/routeentry.go`, `attrparse.go`, `familyrib.go`), AS4 conversion, community rebuilds, Prefix-SID cleanup | Independent `classic_daemon_publication_review` context | Source clean; storage, AS4, community, and Prefix-SID repairs landed on `main`. Remaining peer-visible AC-7 wire proof routed to Child 1 (`spec-bgp-one-off-defects-1-bgp-engine.md`). |
