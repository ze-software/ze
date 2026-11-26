# Spec: bgp-graceful-restart-rfc-defects

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Implemented GR and LLGR must not drop RFC 9494 Section 4.3/4.6 communities, depreference non-LLGR communities, or ship without full RFC9494-4.2-6 / 4.3-2 audit verdicts and interop proof. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The strict RFC re-read of `spec-rfc-requirement-quote-hand-backfill` (2026-09-27)
identified six control-plane Graceful Restart (RFC 4724) and Long-Lived Graceful
Restart (RFC 9494) defects (D1 through D6), five `weak` RFC audit verdicts (AC-4),
and one `RFC9494-4.3-2` gap, followed by a Restarting Speaker forwarding-deferral
defect (D7, 2026-11-24) and six control-plane journal rows.

A full HEAD verification on 2026-11-26 confirmed that D1, D2, D3, D4, and D6,
four of the five AC-4 verdicts (`RFC4724-4-2`, `RFC9494-4.2-8`, `RFC9494-4.3-1`,
`RFC9494-4.3-3`), the `RFC9494-4.3-2` `{gap}` removal, and four of the six
control-plane journal rows have ALREADY landed in HEAD (retired below into
`## Existing Mechanisms` with exact symbol and audit proof).

D7 (RFC 4724 Section 4.1 Restarting Speaker forwarding-state retention against a
competing IGP route during `selectionDeferral`, touching the daemon unified
Loc-RIB, `sysrib`, `zefs`, and Linux/QEMU kernel verification) has been split
into its own child spec in this set,
`plan/release/fixes/spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md`
(see `## Work Not Done`).

This spec now owns the remaining open BGP control-plane GR/LLGR work:

| ID | RFC section | Verbatim quote | Producer | What Ze still does wrong at HEAD (2026-11-26) |
|----|-------------|----------------|----------|----------------------------------------------|
| D5a | RFC 9494 Section 4.3 (RFC9494-4.3-2) | "The LLGR_STALE community MUST NOT be removed when the route is further advertised." | `internal/component/bgp/filterapi/filterapi.go`, `internal/component/bgp/plugins/gr/register.go`, `internal/component/bgp/plugins/filter_community/handler.go`, `internal/component/bgp/plugins/filter_community/config.go`, `internal/component/bgp/plugins/filter_community/filter.go`, `internal/component/bgp/reactor/filter_library.go::evaluateLibraryChain` | Three remaining gaps after commit `eeb7041f6a`: (1) `filter_community/handler.go` hardcodes `llgrStaleWire` (`0xFFFF0006`) instead of reading a `filterapi` preserved-community registry that `bgp-gr` populates from `init()`, coupling `filter_community` to an LLGR constant; (2) `filter_community/config.go::parseCommunityDefinitions` and `filter.go::ingressStripCommunities` still include `0xFFFF0006` in code-8 strip sets (emitting a redundant `FilterModify` remove op on ingress); (3) `reactor/filter_library.go::evaluateLibraryChain`'s raw-override branch returns `run.sets.Raw()` without restoring a registered preserved community that the raw override dropped on import (where `runFinalJudges` does not run) or on an export chain where `llgr-egress` is not inserted or the route is below `llgrStaleLevel` (owner decision Q-3: unconditional on presence) |
| D5b (Q-7) | RFC 9494 Section 4.6 (RFC9494-4.6-2) | "The NO_EXPORT community [RFC1997] MUST be attached to the stale routes." | `internal/component/bgp/policyview/register.go`, `internal/component/bgp/plugins/gr/gr_filter.go::grFilterDecls`, `llgrEgressVerdict`, `internal/component/bgp/reactor/reactor_api_forward.go`, `internal/component/bgp/reactor/forward_rs.go`, `internal/component/bgp/filterapi/filterapi.go`, `internal/component/bgp/plugins/filter_community/handler.go::genericCommunityHandler`, `internal/component/bgp/reactor/policy_dryrun.go::wireModVerb` | `gr_filter.go::llgrEgressVerdict` adds `NO_EXPORT` (`65535:65281`) inside `runEgressPolicyChain`, producing `exportWireOverride`; both forward rails (`reactor_api_forward.go` and `forward_rs.go`) then call `applyFactsSendCommunity` on top of `exportWireOverride`, recording `AttrModSuppress` on code 8 for a peer with `send-community none` or a list omitting `standard`; `genericCommunityHandler` preserves only `LLGR_STALE` on `Suppress`, silently dropping Section 4.6 `NO_EXPORT` |
| D8 (journal) | RFC 9494 Section 4.3 (RFC9494-4.3-1) | "A BGP speaker that has advertised the Long-Lived Graceful Restart Capability to a neighbor MUST perform the following upon receiving a route from that neighbor with the LLGR_STALE community or upon attaching the LLGR_STALE community itself per Section 4.2:" | `internal/component/bgp/plugins/rib/rib_commands_community.go::attachCommunityCommand`, `internal/component/bgp/plugins/rib/rib_source_commands.go::attachPhysicalSourceCommunityCommand`, `internal/component/bgp/plugins/rib/rib_sent_lifecycle.go::attachSentSourceCommunityLocked` | Both `attach-community` commands unconditionally raise `entry.StaleLevel` to `storage.DepreferenceThreshold` (2) for ANY attached 4-byte community, not only `LLGR_STALE` (`0xFFFF0006`); owned journal row `escape-hatch-scoped-wider-than-its-justification` (2026-10-18) |
| V1 (AC-4) | RFC 9494 Section 4.2 / 4.3 (RFC9494-4.2-6, RFC9494-4.3-2) | "The helper router MUST perform the procedures listed in Section 4.3." and "The LLGR_STALE community MUST NOT be removed when the route is further advertised." | `rfc/audit/rfc9494/RFC9494-4.2-6.json`, `rfc/audit/rfc9494/RFC9494-4.3-2.json`, `internal/component/bgp/plugins/rib/rfc9494_test.go` | `RFC9494-4.2-6` remains `weak` (tagged tests prove only self-attached depreference, not the remaining Section 4.3 procedures triggered by Section 4.2 attachment); `RFC9494-4.3-2` has discrimination records for `genericCommunityHandler` only and no independent audit verdict yet |
| I1 (interop) | RFC 4724 Section 4, RFC 9494 Section 4.3 | Real-daemon proof of NOTIFICATION flush (D1) and received LLGR_STALE handling (D3/D4/D5) | `test/interop/scenarios/bgp-graceful-restart-frr/`, `test/interop/scenarios/bgp-llgr-stale-received-frr/`, `internal/le/interoplab/bgp/`, `docs/architecture/testing/interop.md` | `bgp-graceful-restart-frr` still excludes NOTIFICATION teardown, and no `bgp-llgr-stale-received-frr` interop scenario exists yet |

All owner decisions recorded on 2026-10-13 and 2026-10-14 are unfolded into the
design below. Because `gr_egress.go::LLGREgressFilter` now runs inside
`gr_filter.go::evaluateLLGREgress` and communicates with the reactor across the
`filter-update` boundary (internal and forked) via `policyview.Treatment`
(`TreatmentAttachNoExport`) rather than modifying `LLGREgressFilter`'s own
`AttrModAdd` op, the two RFC9494-4.6-2 tagged units in
`gr/rfc9494_gr_egress_test.go` are NOT modified at all, retiring former Q-8 with
zero remaining owner questions.

Related: `plan/post-release/features/spec-gr-advanced.md` designs RFC 8538 N-bit
retention on top of the landed D1 `ReasonForbidsRetention` path.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, process boundary, registration over enumeration
  → Constraint: every plugin filter and forwarding treatment works identically internal and forked; no plugin hardcodes another plugin's protocol constant when a registry belongs in `filterapi`
- [ ] `ai/doors/bgp-engine.md` - BGP engine filter pipeline, `filterapi`, `policyview`, `policychain`, reactor forward rails
  → Constraint: plugin filters cross `filter-update` on both transports; per-destination forwarding treatments travel in `egressStepResult.treatments` (`policyview.Treatment`), never via process-local globals
- [ ] `ai/doors/bgp-plugins.md` - `bgp-gr`, `bgp-filter-community`, `bgp-rib`
  → Constraint: Stage 1 declarations (`grFilterDecls`) and compiled `registry.Registration` (`register.go`) must agree; no plugin imports another plugin's package
- [ ] `ai/doors/le-tooling.md` - functional `.ci` and Docker interop tests
  → Constraint: new interop scenario directories are named, never numbered, and must register a checker in `internal/le/interoplab/bgp/`
- [ ] `docs/architecture/core-design.md` - Section 9 (Policy Filter Chain, `AttrOp` actions, `policyview.Treatment`)
  → Constraint: `AttrOp` stays 32 bytes (`Action` is one byte of the closed action vocabulary); `AttrModAttach` uses that existing `Action` byte and adds no field
- [ ] `docs/architecture/api/architecture.md` - BGP route filter pipeline
  → Constraint: filter modification handlers are registered by attribute code in `filterapi` and invoked by `buildModifiedPayload`
- [ ] `docs/architecture/bgp/structural-forwarding.md` - route-server forward rail (`forward_rs.go`)
  → Constraint: `forward_rs.go` and `reactor_api_forward.go` must apply `TreatmentAttachNoExport` identically beside `applyFactsSendCommunity`
- [ ] `docs/architecture/plugin/rib-storage-design.md` - community attach/delete and sent lifecycle inventory
  → Constraint: `attach-community` raises `StaleLevel` to `DepreferenceThreshold` only for `LLGR_STALE` (`0xFFFF0006`)
- [ ] `docs/architecture/testing/interop.md` - `bgp-graceful-restart-frr` and BGP interop suite discovery
  → Constraint: update the `bgp-graceful-restart-frr` description when adding its NOTIFICATION phase, and document `bgp-llgr-stale-received-frr`
- [ ] `docs/guide/graceful-restart.md` - user guide for GR/LLGR readvertisement and community preservation
  → Constraint: document Section 4.6 IBGP `NO_EXPORT` surviving `send-community none` and unconditional `LLGR_STALE` preservation across raw overrides

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md`, `rfc/short/rfc9494.md`, `rfc/full/rfc9494.txt` Sections 4.2, 4.3, and 4.6
  → Constraint: RFC 9494 Section 4.3 binds preservation of `LLGR_STALE` unconditionally ("MUST NOT be removed when the route is further advertised"); Section 4.6 binds `NO_EXPORT` attachment ("MUST be attached to the stale routes") on internal partial deployment; Section 4.3 depreference on self-attachment applies to `LLGR_STALE` alone
  → Constraint: RFC scope is frozen: no enrolment of new RFCs, and `rfc/short/rfc8538.md` stays blocked

**Key insights:**
- D1, D2, D3, D4, and D6 are already implemented and `enforced` in HEAD; only D5a (registration + strip-set + raw-override restoration), D5b (Q-7 Section 4.6 `NO_EXPORT` surviving `send-community` Suppress), D8 (`attach-community` depreference scoped to `LLGR_STALE`), the `RFC9494-4.2-6` / `RFC9494-4.3-2` audit verdicts, and the two Docker interop proofs remain.
- Because `llgr-egress` runs inside `runEgressPolicyChain` over `filter-update` (internal and forked) and `applyFactsSendCommunity` runs afterward in `reactor_api_forward.go` and `forward_rs.go`, carrying Section 4.6 `NO_EXPORT` protection as a registered `policyview.Treatment` (`TreatmentAttachNoExport`) works across both transports AND leaves `gr_egress.go::LLGREgressFilter` and its RFC9494-4.6-2 tagged tests unchanged.
- `internal/component/bgp/config/register.go::exportSettable` already derives settable export treatments from `policyview.TreatmentFields()`, so registering `TreatmentAttachNoExport` in `policyview/register.go` requires zero central-list edits.

## Current Behavior (MANDATORY)

**Source files read (verified at HEAD on 2026-11-26):**
- [ ] `internal/component/bgp/filterapi/filterapi.go` - declares `AttrModSet`, `AttrModAdd`, `AttrModRemove`, `AttrModPrepend`, `AttrModSuppress`, `AttrModReplaceNLRI`, `LastSetOrSuppress`, `RegisterAttrModHandler`; holds no `AttrModAttach` and no preserved-community registry
- [ ] `internal/component/bgp/plugins/gr/register.go` - registers `bgp-gr`, its `fieldStale` metadata, `grFilterDecls`, `grInsertions`, and `llgr-stale`/`no-llgr` community names; registers no preserved community in `filterapi`
- [ ] `internal/component/bgp/plugins/gr/gr_filter.go` - `grFilterDecls` declares `llgr-ingress` and `llgr-egress` (`Readvertise: true`, `JudgesFinal: true`); `evaluateLLGREgress` calls `LLGREgressFilter` and `llgrEgressVerdict` translates its ops to `rpc.FilterSet` (`community add "65535:65281"`, `local-preference set 0`); emits no forwarding treatment for `NO_EXPORT`
- [ ] `internal/component/bgp/plugins/gr/gr_egress.go` - `LLGREgressFilter` (threshold `llgrStaleLevel = 2`, Q-2) and `advertisedStaleLevel` (`meta["stale"]` + `MetaKeyRebuilt` calling `llgrStaleFromSource`); already landed and `enforced` for RFC9494-4.3-3 and RFC9494-4.6-2
- [ ] `internal/component/bgp/plugins/gr/gr_ingress.go`, `gr.go`, `gr_state.go`, `gr_capability.go` - D1 (`ReasonForbidsRetention`), D2 (`reconnectKeepsFamily`, Q-1 AND), D3 (`LLGRIngressFilter`, `llgrStaleFromSource`), and GR capability tuple emission (`parseGRCapValue`) are all landed and `enforced` in HEAD
- [ ] `internal/component/bgp/policyview/register.go` - declares `FieldPreserveOpaqueAttributes` and `TreatmentPreserveOpaqueAttributes = MustRegisterTreatment(...)`; `internal/component/bgp/config/register.go::exportSettable` automatically exposes every `policyview.TreatmentFields()` entry on `bgp export`
- [ ] `internal/component/bgp/plugins/filter_community/handler.go` - `genericCommunityHandler` hardcodes `llgrStaleWire = [4]byte{0xFF, 0xFF, 0x00, 0x06}` and exempts it on Suppress, Set, and Remove; ignores any `AttrModAttach` action and drops `NO_EXPORT` on `AttrModSuppress`
- [ ] `internal/component/bgp/plugins/filter_community/config.go`, `filter.go`, `community_filter.go` - `parseCommunityDefinitions` builds one `wireValues` slice used for both `tag` and `strip`; `ingressStripCommunities` strips `0xFFFF0006` in `ingressFilter`, causing `rewriteVerdict` to emit a `FilterSetActionRemove` that `genericCommunityHandler` later ignores
- [ ] `internal/component/bgp/reactor/filter_library.go` - `evaluateLibraryChain` returns `run.sets.Raw()` directly when a chain member supplies a raw UPDATE override, without checking whether the input carried a preserved community that the override removed
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go`, `forward_rs.go` - both call `applyFactsSendCommunity(facts, &mods)` after `runEgressPolicyChain` / `runsExportChain` on top of `exportWireOverride`; both read `treatments` for `TreatmentPreserveOpaqueAttributes`, neither checks a `NO_EXPORT` attachment treatment
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `sendStaleReadvertiseUnit` runs `decideStaleReadvertise` on already post-`send-community` Adj-RIB-Out bytes and never calls `applyFactsSendCommunity`, so it is unaffected by `send-community` Suppress
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` - `wireModVerb` maps each `AttrMod*` action to a string; has no case for `AttrModAttach`
- [ ] `internal/component/bgp/plugins/rib/rib_commands_community.go`, `rib_source_commands.go`, `rib_sent_lifecycle.go` - `attachCommunityCommand` (line 101) and `attachPhysicalSourceCommunityCommand` (lines 363, 382) unconditionally set `entry.StaleLevel = storage.DepreferenceThreshold` for any 4-byte community; `attachSentSourceCommunityLocked` (line 193) skips received entries with `level < storage.DepreferenceThreshold` before attaching to sent copies and copying `entry.StaleLevel`

**Behavior to preserve:**
- All landed D1, D2, D3, D4, and D6 behaviors and their `enforced` RFC audit verdicts (`RFC4724-4-2`, `RFC9494-4.2-8`, `RFC9494-4.3-1`, `RFC9494-4.3-3`, `RFC9494-4.6-2`).
- Ordinary `AttrModAdd` on code 8 (`filter_community` `community { tag }`, policy `community-add`) is still dropped by a `send-community` Suppress (`none`, `extended`, `large`) or a Set.
- A route without `LLGR_STALE` sent to a `send-community none` peer still carries no `COMMUNITIES` attribute unless Section 4.6 partial deployment applies.
- `filter_community` `community { tag }` still appends every configured value (including `65535:6` if an operator explicitly configures a tag set naming it).
- `genericCommunityHandler` keeps zero-copy `p.Keep` fragments and zero-allocation fast paths; `AttrOp` stays 32 bytes.
- `gr_egress.go::LLGREgressFilter` keeps its existing `AttrModAdd` op and both RFC9494-4.6-2 tagged units in `gr/rfc9494_gr_egress_test.go` stay untouched.

**Behavior to change:**
- D5a: `bgp-gr` registers `LLGR_STALE` (`0xFFFF0006`) as a preserved standard community via `filterapi`; `genericCommunityHandler`, `filter_community` code-8 strip sets, and `evaluateLibraryChain`'s raw-override branch (both import and export) preserve every registered community unconditionally on presence (owner Q-3).
- D5b (Q-7): `bgp-gr`'s `llgr-egress` sets `TreatmentAttachNoExport` on Section 4.6 IBGP delivery; both forward rails record `AttrModAttach` for `NO_EXPORT` on code 8; `genericCommunityHandler` emits `AttrModAttach` values (deduplicated) after Suppress, Set, and Remove/Add; `wireModVerb` prints `"attach"`.
- D8: `attachCommunityCommand`, `attachPhysicalSourceCommunityCommand`, and `attachSentSourceCommunityLocked` raise `StaleLevel` to `storage.DepreferenceThreshold` only when attaching `attribute.CommunityLLGRStale` (`0xFFFF0006`).
- V1 & I1: full `RFC9494-4.2-6` and `RFC9494-4.3-2` audit verdicts (`enforced`), `llgr-stale-community-preserved.ci` (+ `-forked.ci`), `bgp-graceful-restart-frr` NOTIFICATION phase, and `bgp-llgr-stale-received-frr` interop scenario.

## Existing Mechanisms (MANDATORY)

Every mechanism already in HEAD (including retired D1, D2, D3, D4, D6, and
already-fixed journal items verified on 2026-11-26):

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| D1 (retired): NOTIFICATION close forbids retention | `internal/component/bgp/reactor/peer_close.go::sessionCloseFor`; `pkg/plugin/rpc/enums.go::ReasonNotification`, `ReasonForbidsRetention`; `internal/component/bgp/plugins/gr/gr.go::handleStructuredState`, `handleStateEvent`; `rfc/audit/rfc4724/RFC4724-4-2.json` (`enforced`) | Reactor emits `rpc.ReasonNotification` on sent or received NOTIFICATION; both GR state handlers enforce `ReasonForbidsRetention`, flushing routes instead of retaining. Six tagged units prove both polarities | Does not apply for code (already landed and `enforced`); extends interop proof only (`bgp-graceful-restart-frr` NOTIFICATION phase) |
| D2 (retired): per-family LLGR reconnect F-bit validation | `internal/component/bgp/plugins/gr/gr_state.go::onSessionReestablished`, `reconnectKeepsFamily`; `rfc/audit/rfc9494/RFC9494-4.2-8.json` (`enforced`) | Captures `inLLGRPeriod` before timer updates and enforces Q-1 (AND of GR and LLGR F-bits for LLGR-period families; RFC 4724 alone for GR-period families). Seven tagged units prove all bullets and negative polarities | Does not apply (already landed and `enforced`) |
| D3 (retired): received `LLGR_STALE` depreference | `internal/component/bgp/plugins/gr/gr_ingress.go::LLGRIngressFilter`, `llgrStaleFromSource`; `gr_filter.go::evaluateLLGRIngress`; `internal/component/bgp/plugins/rib/storage/routeentry.go::ReceivedLLGRStale`, `AdvertisedStaleLevel`; `test/plugin/llgr-stale-received-depreferenced.ci` (+ `-forked.ci`, `llgr-stale-kept-after-community-strip.ci`); `rfc/audit/rfc9494/RFC9494-4.3-1.json` (`enforced`) | Inserted `llgr-ingress` (priority 50) marks routes received with `LLGR_STALE` from an LLGR-advertised session at stale level 2 before operator import policy; RIB keeps `ReceivedLLGRStale` distinct from retention `StaleLevel` | Does not apply for code (already landed and `enforced`); extends interop proof (`bgp-llgr-stale-received-frr`) |
| D4 & D6 (retired): `LLGREgressFilter` across all rails at threshold 2 | `internal/component/bgp/plugins/gr/gr_egress.go::LLGREgressFilter`, `advertisedStaleLevel`; `gr_filter.go::evaluateLLGREgress`; `internal/component/bgp/plugins/rib/sent_owner.go::sentSourceState`; `internal/component/bgp/reactor/reactor_api_relay.go::buildRelayUpdate`; `rfc/audit/rfc9494/RFC9494-4.3-3.json` (`enforced`) | Threshold `llgrStaleLevel = 2` (Q-2) passes level-1 GR routes through; live forward, RS forward, RIB peer-up/refresh replay, and relay rebuilt rails all deliver or recover stale level 2 and withhold from EBGP non-LLGR peers | Does not apply for D4/D6 decision (already landed and `enforced`); extends `gr_filter.go::llgrEgressVerdict` for D5b (Q-7) and adds interop scenario `bgp-llgr-stale-received-frr` |
| D5 partial: code-8 community handler & `JudgesFinal` export pass | `internal/component/bgp/plugins/filter_community/handler.go::genericCommunityHandler` (commit `eeb7041f6a`); `internal/component/bgp/plugins/gr/gr_filter.go::lostLLGRStale`; `internal/component/bgp/reactor/filter_library.go::runFinalJudges` | `genericCommunityHandler` preserves hardcoded `llgrStaleWire` across Suppress, Set, and Remove (covering `bgp-filter-community:ingress` via `rewriteVerdict` as well as egress); `llgr-egress` (`JudgesFinal: true`) restores `LLGR_STALE` on LLGR-governed export rewrites | Extends: replaces hardcoded `llgrStaleWire` with `filterapi` preserved-community registry, exempts preserved values from code-8 strip sets at config parse, restores preserved values on raw overrides in `evaluateLibraryChain`, and adds `AttrModAttach` for D5b |
| Forwarding treatments across `filter-update` | `internal/component/bgp/policyview/register.go::TreatmentPreserveOpaqueAttributes`; `internal/component/bgp/policyview/treatment.go::MustRegisterTreatment`; `internal/component/bgp/config/register.go::exportSettable`; `internal/component/bgp/reactor/reactor_api_forward.go`; `internal/component/bgp/reactor/forward_rs.go` | Export chain members set registered `policyview.Treatment` flags across both internal and forked transports; `egressStepResult.treatments` delivers the bitmask to both forward rails | Extends: registers `TreatmentAttachNoExport` so `llgr-egress` signals Section 4.6 `NO_EXPORT` attachment to both forward rails without breaking process isolation or editing `LLGREgressFilter` |
| `AttrMod` vocabulary and dry-run display | `internal/component/bgp/filterapi/filterapi.go::AttrOp`, `LastSetOrSuppress`; `internal/component/bgp/reactor/policy_dryrun.go::wireModVerb` | Six-value `Action` byte drives progressive attribute rebuilds in `buildModifiedPayload` and dry-run rendering | Extends: adds `AttrModAttach` to the existing `Action` byte (keeping `AttrOp` at 32 bytes) and `"attach"` in `wireModVerb` |
| RIB `attach-community` commands | `internal/component/bgp/plugins/rib/rib_commands_community.go::attachCommunityCommand`; `internal/component/bgp/plugins/rib/rib_source_commands.go::attachPhysicalSourceCommunityCommand`; `internal/component/bgp/plugins/rib/rib_sent_lifecycle.go::attachSentSourceCommunityLocked` | Attaches a 4-byte community to stale routes and copies to sent inventory; unconditionally raises `StaleLevel` to `DepreferenceThreshold` | Extends (D8): scopes the `DepreferenceThreshold` raise to `attribute.CommunityLLGRStale` (`0xFFFF0006`) only |
| Retired journal fixes already in HEAD | `gr_state.go::onSessionDownPlan` (LLGR-only family retention; `RFC9494-4.2-1`/`4.2-2` `enforced`); `format/decode.go:177` (`buf[2:]` header strip); `gr_capability.go::parseGRCapValue` (GR family tuples; `RFC4724-4-4` `enforced`); `adj_rib_in/rib_validation_config_test.go::TestLateReflectorClaimsExistingReplayOwner` (`NewBridgedConn`); `rib_source_mutation.go::retainPhysicalStateLocked` (`peerMeta` retention + `dispatchSourceAnswer`) | Each defect recorded in the earlier owned journal rows is already fixed in HEAD source | Does not apply for code; journal rows stay listed in `## Owned journal rows` and are verified and marked `fixed:` during closure Commit A |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | The core knows nothing, and everything registers | the door every change reads | "It never needs an edit to a switch, a factory, a field list, a help string or a completion table elsewhere" | agrees | `bgp-gr` registers `LLGR_STALE` in `filterapi` from `gr/register.go::init()`, `TreatmentAttachNoExport` registers via `policyview.MustRegisterTreatment` and is discovered automatically by `exportSettable`, and `filter_community` carries no LLGR constant |
| `docs/architecture/api/architecture.md` | RIB Ownership | `// Design:` of `filterapi/filterapi.go`; anchored to `reactor_api_forward.go` | "The engine is a minimal BGP speaker." | agrees | Protocol policy (`bgp-gr`, `bgp-filter-community`) stays in plugins; `filterapi` and `policyview` carry only generic registration seams and `AttrModAttach` |
| `docs/architecture/core-design.md` | Policy Filter Chain | `// Design:` of `policyview/register.go`, `filter_community/*`, `reactor/filter_library.go`, `reactor_api_forward.go`, `forward_rs.go`, `policy_dryrun.go`; modified by this spec | "without `bgp-gr` in the chain nothing runs and an operator's removal stands. An import member can still strip the community before any export chain runs." | departs | Per owner decision Q-3 (2026-10-14), RFC 9494 Section 4.3 preservation of `LLGR_STALE` is unconditional on presence rather than limited to `llgr-egress`'s `JudgesFinal` export pass: `bgp-gr` registers `0xFFFF0006` in `filterapi` at `init()`, `filter_community` preserves registered values across Suppress/Set/Remove and omits them from code-8 ingress strip sets even without `bgp-gr` in the chain, and `evaluateLibraryChain` restores dropped registered values on both import and export raw overrides; `bgp-gr:llgr-egress` stays a policy-chain member carrying `TreatmentAttachNoExport` over `filter-update` on both transports, and this spec updates `docs/architecture/core-design.md` (`Policy Filter Chain`) to reflect the departure plus `AttrModAttach`, `TreatmentAttachNoExport`, and preserved-community registration |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `filterapi`, `policyview`, `reactor` | "Every plugin filter is a policy-chain member, reached through `filter-update` on both transports." | agrees | `llgr-egress` signals Section 4.6 `NO_EXPORT` protection through `filter-update` (`FilterSet` on registered treatment `attach-no-export`), never through an engine import of `plugins/gr` |
| `ai/doors/bgp-plugins.md` | Boundaries | door covering `plugins/gr`, `plugins/filter_community`, `plugins/rib` | "A procedure a feature's config implies is a filter-list member its plugin inserts." | agrees | `bgp-gr` keeps its declared `llgr-ingress` and `llgr-egress` filter insertions and adds `attach-no-export` to `grFilterDecls` (shared by compiled and Stage 1 declarations); no plugin imports another plugin |
| `docs/guide/graceful-restart.md` | Peer Restarts | `// Design:` of `gr/gr_filter.go`, `rib/rib_source_commands.go` | "The GR plugin uses the internal `request bgp rib source` commands, always with `owner <original-owner> connection <original-connection>` before the operation's arguments." | agrees | D8 narrows `DepreferenceThreshold` inside both `attachCommunityCommand` and `attachPhysicalSourceCommunityCommand` without changing command grammar or owner/connection scoping; this spec updates the guide for Section 4.6 `NO_EXPORT` and raw-override preservation |
| `docs/architecture/wire/attributes.md` | 8. COMMUNITY (Code 8) | anchored to `filter_community/handler.go`, `reactor/filter_library.go` | "Registered community names retain their display spelling in both `String` and `AppendText`." | agrees | Standard 4-octet RFC 1997 `COMMUNITIES` wire format and registered community names (`llgr-stale`, `no-export`) are unchanged; `filterapi`'s preserved-community registry holds 4-byte wire values alongside existing attribute code-8 handling |
| `docs/architecture/meta/filter-community.md` | Ordering | anchored to `filter_community/filter.go`; modified by this spec | "The config loader places every inserted member ahead of the operator's, so `bgp-gr`'s `llgr-ingress` marks a route carrying LLGR_STALE before an operator strip can remove it, and an operator tag of LLGR_STALE marks nothing (RFC 9494 Section 4.3)." | agrees | `filter_community` keeps its declared member order so an operator `tag` of `LLGR_STALE` still marks nothing, reads `filterapi.IsPreservedCommunity` when compiling code-8 strip sets and handling code-8 mods so `LLGR_STALE` is neither stripped on ingress nor coupled via a hardcoded constant, and updates lines 52–54 wording to note code-8 strip exemption |
| `docs/architecture/api/commands.md` | Operational Report Bus (ze-cmd:show-warnings, ze-cmd:show-errors) | anchored to `reactor/filter_library.go`, `reactor/policy_dryrun.go` | "A plugin member that rewrote the whole UPDATE body (a raw answer) is a modify with no `delta`, and its `text-after` is the body it answered, though the policy's final still decides whether the route keeps it. The result's `text-after` is the rewritten UPDATE." | agrees | Under `Query RPCs` (`ze-bgp:show-policy-test`), `wireModVerb` adds `"attach"` for `AttrModAttach` within the existing `wire-changes` `"<ATTRIBUTE> <verb>"` format, and `evaluateLibraryChain`'s raw-override preserved-community restore runs after `chain.Trace` so each member's trace `text-after` (`libraryRewriteText`) stays the body that member answered while the final result's `text-after` matches the live rewritten UPDATE (with the policy's final still deciding whether the route keeps it); under `Inter-Plugin RIB Commands (GR/LLGR)`, D8 keeps `request bgp rib attach-community` syntax unchanged while scoping stale level 2 (`DepreferenceThreshold`) to `LLGR_STALE` (preserving `level: 1=GR, 2=LLGR`) |
| `docs/architecture/api/process-protocol.md` | Event Delivery | anchored to `reactor/filter_library.go`, `reactor_api_forward.go`, `policy_dryrun.go`, `rib_source_commands.go` | "Every participant registers the messages it sends and receives at compile time (`registry.Registration`) or Stage 1 (`declare-registration`)." | agrees | `bgp-gr` shares `grFilterDecls` between compiled and Stage 1 registrations so `attach-no-export` is declared identically internal and forked; no process-protocol framing or event delivery contract changes |
| `docs/architecture/bgp/egress-attribute-rules.md` | ATOMIC_AGGREGATE policy admission | anchored to `reactor/filter_library.go`, `reactor_api_forward.go`, `forward_rs.go` | "`runLibraryChain` checks every accepting modified result, raw and typed alike." | agrees | Preserved-community restoration in `evaluateLibraryChain` runs on raw-override outputs before `runLibraryChain`'s final admission checks, leaving LOCAL_PREF, MED, AS-zero, and ATOMIC_AGGREGATE provenance rules untouched |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | `// Design:` of `reactor/forward_rs.go`; anchored to `filter_library.go`, `reactor_api_forward.go`, `forward_rs.go` | "Each rail reads the treatment from the export chain's result (`egressStepResult.treatments`) for each destination, and keys body reuse and edit-set deduplication by it." | agrees | Both `reactor_api_forward.go` and `forward_rs.go` read `TreatmentAttachNoExport` from `egressStepResult.treatments` beside `applyFactsSendCommunity` and record `AttrModAttach` into `mods`, so edit-set fingerprinting and dedup cover it automatically |
| `docs/architecture/buffer-architecture.md` | Executive Summary | `// Design:` of `reactor/reactor_api_forward.go` | "Ze uses a **buffer-first** architecture where BGP messages are represented as byte buffers with iterators and partial parsers." | agrees | `genericCommunityHandler` emits `AttrModAttach` and preserved `p.Keep` slices into the caller's pooled buffer through `buildModifiedPayload` with zero extra allocations and keeps `AttrOp` at 32 bytes |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `reactor/reactor_api_forward.go` | - | silent | This page specifies the `update text` and `update wire` route-injection grammar; this spec adds no keywords, attributes, or syntax to route injection commands |
| `docs/architecture/bgp/fanout-dedup.md` | The decisions | anchored to `reactor/reactor_api_forward.go` | "The fingerprint is a hint. Full equality decides." | agrees | `AttrModAttach` is an `AttrOp.Action` value inside `filterapi.ModAccumulator`, so fan-out edit-set fingerprinting and full `ModAccumulator` equality distinguish attached `NO_EXPORT` destinations automatically without bypassing equality |
| `docs/architecture/encoding-context.md` | Overview | anchored to `reactor/reactor_api_forward.go` | - | silent | This page governs per-peer negotiated capability contexts and zero-copy `ContextID` matching; this spec modifies only post-policy `COMMUNITIES` preservation/attachment and does not alter `EncodingContext` or `ContextID` comparison |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `reactor/reactor_api_forward.go` | - | silent | This page governs two-tier forward buffer pool sizing and congestion backpressure; this spec does not change pool capacities, block allocation, or buffer recycling |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `reactor/reactor_api_forward.go`, `reactor/forward_rs.go` | "On the hot path Ze **borrows** bytes (zero-copy slices) rather than copying them." | agrees | `genericCommunityHandler` keeps zero-copy `p.Keep` borrows within `buildModifiedPayload` and `communityNoExportWire` is a fixed 4-byte package array, introducing no borrowed slice that outlives its boundary |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `reactor/reactor_api_forward.go` | - | silent | This page specifies RFC 9234 peer-role metadata and OTC enforcement; this spec touches neither peer roles nor `src-peer-role` metadata |
| `docs/architecture/plugin/rib-storage-design.md` | Received attribute flags | `// Design:` of `rib/rib_commands_community.go`, `rib/rib_sent_lifecycle.go`; anchored to `reactor_api_forward.go`, `rib_source_commands.go` | "Appending a community to an existing stored attribute preserves that received fact; a fresh received replacement establishes its own flags." | agrees | D8 scopes only the `entry.StaleLevel = storage.DepreferenceThreshold` raise in `attachCommunityCommand` and `attachPhysicalSourceCommunityCommand` to `LLGR_STALE`, leaving stored attribute flags, pooling, and sent provenance unchanged |
| `docs/architecture/update-building.md` | Three Paths for UPDATE Messages | anchored to `reactor/reactor_api_forward.go` | - | silent | This page specifies outbound UPDATE construction paths and cross-peer update grouping; this spec modifies only code-8 community handler actions and raw-override post-processing, leaving message framing and builder paths unchanged |
| `docs/architecture/route-selection.md` | Phase 2: Best-Path Selection (RFC 4271 Section 9.1.2) | anchored to `reactor/forward_rs.go` | "GR/LLGR stale-level; threshold = 2. A route received carrying LLGR_STALE from a neighbor Ze sent the LLGR Capability to enters at the threshold (`extractCandidate`)" | agrees | Step 9 (`stale-deprioritized`, RFC 9494, threshold = 2) is preserved for `LLGR_STALE` (both received via `llgr-ingress` and self-attached via `attach-community` with `0xFFFF0006`); D8 scopes the `DepreferenceThreshold` raise in `attachCommunityCommand`, `attachPhysicalSourceCommunityCommand`, and `attachSentSourceCommunityLocked` to `LLGR_STALE` alone so non-LLGR `attach-community` keeps `StaleLevel` at 1 (below threshold 2) |
| `docs/architecture/cli/command-verbs.md` | The verb classes | anchored to `reactor/policy_dryrun.go` | - | silent | This page defines the CLI command verb taxonomy; this spec adds or renames no CLI commands |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `reactor/policy_dryrun.go` | - | silent | This page documents the architectural move of RIB storage out of the engine into API plugins and `msg-id` cache control; this spec does not alter RIB ownership or cache-control verbs |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | `// Design:` of `internal/le/interoplab/bgp/register_llgr.go`; modified by this spec | "A scenario directory carries only declarative inputs its runner reads: `ze.conf` plus the peer configuration and argument files that topology needs. Assertions never live there. They are typed Go checkers under `internal/le/interoplab/`" | agrees | `bgp-llgr-stale-received-frr` carries only declarative peer configs (`ze.conf`, `frr.conf`, `gobgp.toml`) with its typed Go checker registered in `internal/le/interoplab/bgp/register_llgr.go`, and `interop.md` is updated alongside it |
| `ai/doors/le-tooling.md` | Where it sits | door covering `interoplab/bgp/*`, `test/plugin/*`, `test/interop/*` | "Interop scenarios live under `test/interop*/` and run through `internal/le/interoplab`" | agrees | New interop scenario `test/interop/scenarios/bgp-llgr-stale-received-frr/` is named (never numbered), registered in `internal/le/interoplab/bgp/`, and paired with internal and forked `.ci` functional tests under `test/plugin/` |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec adds no new automatic filter insertion and preserves inline `# implied:` display for existing `bgp-gr:llgr-ingress` and `bgp-gr:llgr-egress` insertions |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `bgp-gr`, `bgp-filter-community`, and `bgp-rib` keep their registered bus declarations unchanged; no undocumented bus traffic is added |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | No participant bus routing or `send > to` compilation logic is altered by this spec |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | No participant `receive > from` enforcement is altered by this spec |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | No bus allow-list compilation or enforcement is altered by this spec |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec touches no bus-filter compilation or commit validation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | No exclusive-role rule or bus arbitration is introduced; `bgp-gr` registers `LLGR_STALE` preservation and sets `TreatmentAttachNoExport` within its own filter evaluation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | No bus effective-filter display behavior is altered; `grFilterDecls` remains the single declaration for `bgp-gr:llgr-egress` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec is pure BGP control-plane policy/community handling with no kernel-only data-plane dependency |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec introduces no data-plane backend mismatch or validation path divergence |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec touches no firewall or VPP configuration or verifier |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no data-plane selector or backend naming |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec does not touch egress next-hop rewriting or session next-hop config; `TreatmentAttachNoExport` applies only to code-8 `COMMUNITIES` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | `policychain.Sets` next-hop intent and its authoring forms remain untouched; `TreatmentAttachNoExport` uses the existing treatment bitmask alongside `TreatmentPreserveOpaqueAttributes` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Built-in `next-hop:*` engine steps are untouched; `bgp-gr:llgr-egress` stays a plugin filter step using `filter-update` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Raw-override preserved-community restoration in `evaluateLibraryChain` touches only attribute code 8 (`COMMUNITIES`), leaving next-hop provenance and trailing `next-hop:auto` intact |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Next-hop handling on import and export chains is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Trailing `next-hop:auto` scoping on forwarded vs originated routes is untouched by this spec |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | `bgp-gr`'s `llgr-ingress` and `llgr-egress` remain declared `FilterInsertions` published through the existing `filterapi.ImpliedFilter` seam |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec adds no startup refusal; `community-remove ffff0006` stays accepted at config load and inert at runtime (owner Q-3) |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec introduces no kernel/VPP backend mismatch and no startup refusal |
| `docs/guide/graceful-restart.md` | Design decisions | decision of 2026-11-22 | "Graceful Restart and Long-Lived Graceful Restart use symmetric `short-lived` and `long-lived` containers inside `graceful-restart`, both carrying keyed `list family` (`restart-time` inside `short-lived`; mandatory `stale-time` per family inside `long-lived`), with legacy `long-lived-stale-time`, top-level `restart-time`, and `family { name [...] }` removed" | agrees | All new `.ci` and Docker interop configs in this spec use the symmetric `short-lived` and `long-lived` YANG containers |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds no command path or selector words |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec adds or modifies no pipe overrides |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec adds or modifies no pipe operations |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no new engine operation; `TreatmentAttachNoExport` travels inside the existing authorized `filter-update` reply |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | No event queue or delivery behavior is changed; lossless event delivery guarantees GR/RIB state-event ordering |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | This spec does not touch FlowSpec MP_REACH next-hop emission or `flowspec-redirect-next-hop` |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Both `bgp-graceful-restart-frr` and `bgp-llgr-stale-received-frr` are pure-BGP scenarios requiring no unrelated kernel capabilities (`KernelCapabilities` empty) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- D5a: any BGP UPDATE traversing `filter_community` (`ingressStripCommunities` / `genericCommunityHandler`) or a raw-override chain member (`evaluateLibraryChain` on import or export) whose input `COMMUNITIES` (code 8) contains a registered preserved community (`LLGR_STALE`, `0xFFFF0006`).
- D5b (Q-7): any LLGR-stale BGP UPDATE (`advertisedStaleLevel >= 2`) forwarded toward an IBGP non-LLGR destination whose `send-community` suppresses standard communities (`none`, `extended`, or `large`).
- D8: `request bgp rib attach-community` or `request bgp rib source attach-community` with a 4-byte community hex argument.

### Transformation Path
1. **D5a (preserved-community registry & raw-override restore):**
   - At startup `init()`, `gr/register.go` calls `filterapi.RegisterPreservedCommunity(llgrStaleWire)` to register `0xFFFF0006` (standard 4-octet community). `filterapi.Snapshot` / `Restore` / `ResetForTest` preserve and restore the set for test isolation (`ai/doors/core.md` Traps).
   - At config parse time, `filter_community/config.go::parseCommunityDefinitions` populates `communityDef.wireValues` (all configured values, used by `tag`) and, for `communityTypeStandard`, populates `communityDef.stripWireValues` with registered preserved values excluded; `filter.go::applyIngressOps` passes `stripWireValues` to `ingressStripCommunities` so stripping a definition naming `65535:6` neither mutates the payload for `65535:6` nor emits a redundant remove op. Codes 16 and 32 are untouched.
   - In `filter_community/handler.go::genericCommunityHandler`, the hardcoded `llgrStaleWire` checks on Suppress, Set, and Remove are replaced by `filterapi.IsPreservedCommunity` over the source `p.Value()` values (zero-copy `p.Keep(off, 4)` fragments, zero allocation on the hot path).
   - In `reactor/filter_library.go::evaluateLibraryChain`, inside the `if raw := run.sets.Raw(); raw != nil` branch (which handles BOTH import and export raw overrides): when `filterapi.HasPreservedCommunities()` is true, scan the input `payload`'s `COMMUNITIES` attribute for any registered preserved value absent from `raw`'s `COMMUNITIES`; if any are missing, accumulate one `AttrModAdd` on code 8 per missing value into `run.mods` and rebuild via `buildModifiedPayload(raw, &run.mods, r.attrModHandlers, nil, nil)` (failing the rewrite if `modFail.failed()`). When `raw` is nil, or the input carries no preserved value, or `raw` already keeps it, zero rebuild runs.
2. **D5b (Q-7: Section 4.6 `NO_EXPORT` surviving `send-community` Suppress):**
   - `policyview/register.go` registers `FieldAttachNoExport` (`"attach-no-export"`) and `TreatmentAttachNoExport = MustRegisterTreatment(FieldAttachNoExport)`. `bgp/config/register.go::exportSettable` discovers it automatically via `policyview.TreatmentFields()`.
   - `gr/gr_filter.go::grFilterDecls` adds `policyview.FieldAttachNoExport.String()` to `llgr-egress`'s `Sets` list (shared by both `registry.Registration` and Stage 1 `sdk.Registration`).
   - When `gr/gr_filter.go::llgrEgressVerdict` translates `LLGREgressFilter`'s Section 4.6 IBGP `NO_EXPORT` op, it emits both `FilterSet{Field: "community", Action: Add, Value: "\"65535:65281\""}` (unless `session.holdsNoExport` is already true) and `FilterSet{Field: "attach-no-export", Action: Set, Value: "true"}` (always, even if `session.holdsNoExport` was true, because a later `applyFactsSendCommunity` Suppress would otherwise strip the existing `NO_EXPORT`). `gr_egress.go::LLGREgressFilter` itself is unchanged.
   - `policychain` folds `TreatmentAttachNoExport` into `egressStepResult.treatments` identically for internal and forked `bgp-gr` processes.
   - Both forward rails (`reactor/reactor_api_forward.go` and `reactor/forward_rs.go`), immediately beside `applyFactsSendCommunity(facts, &mods)`, test `treatments&policyview.TreatmentAttachNoExport.Bit() != 0`; when set, they record `mods.Op(uint8(attribute.AttrCommunity), filterapi.AttrModAttach, communityNoExportWire[:])`.
   - `filter_community/handler.go::genericCommunityHandler` honours `filterapi.AttrModAttach`:
     - On `AttrModSuppress`: emits preserved source values (`p.Keep`), then every `AttrModAttach` op value not already emitted, and calls `p.Drop()` only if nothing was emitted.
     - On `AttrModSet`: emits `ops[setIdx].Buf`, then preserved source values absent from `ops[setIdx].Buf`, then `AttrModAttach` op values absent from what was already emitted, and calls `p.Drop()` only if nothing was emitted.
     - On Remove/Add (no Set or Suppress): keeps retained source values, emits `AttrModAdd` ops, then emits `AttrModAttach` ops whose 4-byte values were not already emitted (so an already-present `NO_EXPORT` from `exportWireOverride` is never duplicated when `send-community` allows standard communities).
     - Every `AttrModAdd` producer (`filter_community` `tag`, policy `community-add`, etc.) keeps today's contract and is still dropped by `AttrModSuppress` or `AttrModSet`.
   - `reactor/policy_dryrun.go::wireModVerb` maps `filterapi.AttrModAttach` to `"attach"`.
3. **D8 (`attach-community` depreference scope):**
   - `rib/rib_commands_community.go::attachCommunityCommand` and `rib/rib_source_commands.go::attachPhysicalSourceCommunityCommand` test `binary.BigEndian.Uint32(commBytes) == uint32(attribute.CommunityLLGRStale)` (using a shared helper `isLLGRStaleCommunity(comm []byte) bool`): only when true do they raise `entry.StaleLevel` to `storage.DepreferenceThreshold` (2) on received entries (and on sent entries in `attachPhysicalSourceCommunityCommand`); attaching any other 4-byte community leaves `entry.StaleLevel` at 1.
   - In `rib/rib_sent_lifecycle.go::attachSentSourceCommunityLocked`, the guard changes from `if level < storage.DepreferenceThreshold` to `if level == storage.StaleLevelFresh`: because non-LLGR `attach-community` now keeps `entry.StaleLevel` at 1 on received entries, checking `level == storage.StaleLevelFresh` ensures sent copies still receive the attached non-LLGR community while preserving their `StaleLevel` (1), and still take `DepreferenceThreshold` (2) when `LLGR_STALE` is attached.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `bgp-gr` ↔ `bgp-filter-community` / `reactor` | `filterapi.RegisterPreservedCommunity` / `IsPreservedCommunity` (in-process registry in `filterapi`; `bgp-gr` imports `filterapi`, never `filter_community`, and `filter_community` never imports `gr`) | No |
| `bgp-gr` (internal or forked) ↔ `reactor` export path | `filter-update` reply `FilterSet` on registered treatment field `attach-no-export` (`policyview.TreatmentAttachNoExport`) carried in `egressStepResult.treatments` | No |
| `reactor` forward rails ↔ `filter_community` code-8 handler | `filterapi.ModAccumulator` op with `Action: filterapi.AttrModAttach` | No |

### Changed-Wire Census

**Search evidence:** Searched `internal/component/bgp/policyview/`, `internal/component/bgp/policychain/`, `internal/component/bgp/plugins/gr/`, `internal/component/bgp/reactor/`, and `pkg/plugin/rpc/` for `FilterSet`, `Treatment`, `FieldPreserveOpaqueAttributes`, and `FilterUpdateOutput` (positive control: `TreatmentPreserveOpaqueAttributes` in `policyview/register.go` written by `bgp-rs` over `filter-update` and read by `policychain/sets_treatment.go::applyExternalTreatment`). The `filter-update` JSON schema (`pkg/plugin/rpc/filter.go`) is UNCHANGED (no new JSON keys or RPC verbs); one new treatment field name (`"attach-no-export"`) travels as an existing `FilterSet{Field, Action: "set", Value: "true"}` entry on `filter-update` replies from `bgp-gr:llgr-egress`. No BGP wire message format, CLI command grammar, or YANG schema changes; the BGP `COMMUNITIES` attribute bytes emitted to peers now conform to RFC 9494 Sections 4.3 and 4.6.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `filter-update` `FilterSet` field `"attach-no-export"` | writer | `internal/component/bgp/plugins/gr/gr_filter.go::llgrEgressVerdict` (declared in `grFilterDecls`) | Emits `rpc.FilterSet{Field: policyview.FieldAttachNoExport.String(), Action: rpc.FilterSetActionSet, Value: []byte("true")}` on Section 4.6 IBGP delivery | Change: additive treatment field declared in `grFilterDecls` and `policyview/register.go`; ignored by peers where Section 4.6 does not fire; proven in both internal and forked `bgp-gr` modes |
| `filter-update` `FilterSet` field `"attach-no-export"` | reader | `internal/component/bgp/policychain/sets_treatment.go::applyExternalTreatment` -> `internal/component/bgp/reactor/reactor_api_forward.go::forwardUpdateCore` & `internal/component/bgp/reactor/forward_rs.go::reactorForwardRS` | `policychain` resolves registered treatments via `policyview.TreatmentNamed`, sets `TreatmentAttachNoExport.Bit()` in `egressStepResult.treatments`; both forward rails read the bit and record `filterapi.AttrModAttach` | Change: reuses the existing generic treatment dispatch path (identical to `TreatmentPreserveOpaqueAttributes`), requiring zero `policychain` or `rpc` wire schema changes |
| BGP UPDATE `COMMUNITIES` attribute (code 8) | writer | `internal/component/bgp/plugins/filter_community/handler.go::genericCommunityHandler` and `internal/component/bgp/reactor/filter_library.go::evaluateLibraryChain` | Emits preserved `0xFFFF0006` (`LLGR_STALE`) across Suppress/Set/Remove/raw-override and `0xFFFFFF01` (`NO_EXPORT`) on `AttrModAttach` toward Section 4.6 IBGP peers | Change: RFC 9494 Section 4.3 / 4.6 compliant attribute emission; standard 4-octet RFC 1997 format unchanged |
| BGP UPDATE `COMMUNITIES` attribute (code 8) | reader | External BGP protocol peers (FRR, GoBGP, BIRD, `le test peer` in `.ci` tests) | Standard RFC 1997 / RFC 9494 Section 3.2 well-known 4-octet communities parsed by any RFC 4271/1997/9494 peer | Preserve: wire encoding is unchanged RFC 1997 4-octet communities; proven by `.ci` and Docker FRR/GoBGP interop scenarios |

### Integration Points
- `filterapi.RegisterPreservedCommunity` / `IsPreservedCommunity` / `HasPreservedCommunities` / `AttrModAttach` (`internal/component/bgp/filterapi/filterapi.go`)
- `policyview.FieldAttachNoExport` / `TreatmentAttachNoExport` (`internal/component/bgp/policyview/register.go`), automatically discovered by `bgp/config/register.go::exportSettable` and `policychain/sets_treatment.go`
- `gr/gr_filter.go::grFilterDecls` (both compiled `registry.Registration` and Stage 1 `sdk.Registration`)
- `internal/le/interoplab/bgp/` checker registration for `bgp-graceful-restart-frr` and `bgp-llgr-stale-received-frr`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | `llgr-egress` signals `TreatmentAttachNoExport` through the standard `filter-update` / `policychain` treatment mechanism; raw overrides rebuild via `buildModifiedPayload` with `r.attrModHandlers` |
| No unintended coupling (components stay isolated) | Yes (design) | `filter_community` no longer hardcodes `LLGR_STALE`; it reads `filterapi` preserved-community registration populated by `bgp-gr` at `init()` |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | Reuses `policyview.MustRegisterTreatment`, `genericCommunityHandler`, and `buildModifiedPayload` |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | `genericCommunityHandler` retains zero-copy `p.Keep` fragments and deduplicates `AttrModAttach` inline; `AttrOp` stays 32 bytes |
| Registration over hardcoding, outbound | Yes (design) | `bgp-gr` registers `LLGR_STALE` via `filterapi.RegisterPreservedCommunity`; `TreatmentAttachNoExport` registers via `policyview.MustRegisterTreatment` |
| Registration over hardcoding, inbound | Yes (design) | Searched `bgp/config/register.go::exportSettable` and `policychain/sets_treatment.go`: both derive from `policyview.TreatmentFields()` with zero switch or seed-map edits; `policy_dryrun.go::wireModVerb` is the one closed-enum display switch on `AttrOp.Action` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `bgp-gr` is linked whenever LLGR runs, so its `init()` registers `0xFFFF0006` in `filterapi` before any route is parsed or forwarded | `internal/component/plugin/all/all_ze_bgp.go` blank-imports `plugins/gr` whenever `ze_bgp` is built | A test of `filter_community` that does not import `gr` sees an empty preserved set unless it registers the value itself | `filter_community` unit tests explicitly register/snapshot `filterapi` preserved communities, and `gr` tests assert `filterapi.IsPreservedCommunity` holds `0xFFFF0006` | unvalidated |
| A-2 | `filter_community`'s code-8 handler is linked in every build that forwards | `filter_community/register.go::init()` registers code 8 via `filterapi.RegisterAttrModHandler` | A build without it would bypass code-8 preservation | Existing `plugin/all/all_ze_bgp.go` blank import | unvalidated |
| A-3 | `policyview.Treatment` bitmask (`uint8`) has free slots beyond `TreatmentPreserveOpaqueAttributes` | `internal/component/bgp/policyview/register.go` registers only 1 treatment today (slot 0 of 8) | Slot overflow panics at `init()` | `policyview` unit tests + build | unvalidated |
| A-4 | FRR can originate a route carrying `llgr-stale` via route-map toward a session where LLGR is negotiated | FRR community keyword list includes `llgr-stale` | Interop scenario uses a native `le test peer` injector beside FRR/GoBGP | First interop run of `bgp-llgr-stale-received-frr` | unvalidated |
| A-5 | `policychain` `Sets.Treatments()` merges treatments across `runFinalJudges` | `internal/component/bgp/reactor/filter_library.go:303` (`last.treatments \|= res.treatments`) | Final-judge pass could lose the treatment bit | Verified in source (`filter_library.go:303`) and covered by `TestLLGREgressTreatmentSurvivesFinalJudge` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving `llgrStaleWire` from a hardcoded constant in `filter_community` to a `gr`-registered `filterapi` entry causes existing `filter_community/rfc9494_llgr_stale_preserve_test.go` to fail when `gr` is not imported by that package's test binary | `go test ./internal/component/bgp/plugins/filter_community/` fails | Register `0xFFFF0006` via `filterapi.RegisterPreservedCommunity` in the test setup (with `Snapshot`/`Restore` cleanup) or blank-import `gr` in the RFC9494-4.3-2 test, and test both "registered preserves" and "unregistered removes" |
| R-2 | Adding `AttrModAttach` on both forward rails duplicates `NO_EXPORT` when `send-community` allows standard communities (because `exportWireOverride` already carries `NO_EXPORT`) | `TestCommunityHandler_AttachNoDuplicateWhenPresent` or `.ci` hex assertion fails | `genericCommunityHandler` checks whether the attached 4-byte value is already present in emitted source/Set/Add bytes before emitting it |
| R-3 | Raw-override restore in `evaluateLibraryChain` adds hot-path cost on non-override routes | Benchmark regression on normal library chain | Guarded strictly inside `if raw := run.sets.Raw(); raw != nil` and skips rebuild when input has no preserved community or `raw` already retains it |
| R-4 | A raw-override import or export filter deliberately removed `LLGR_STALE` | AC-3 / AC-4 units | Intended per owner decision Q-3 (2026-10-14): RFC 9494 Section 4.3 "MUST NOT be removed when the route is further advertised" is unconditional; documented in `docs/guide/graceful-restart.md` |
| R-5 | Editing `genericCommunityHandler`, `evaluateLibraryChain`, `attachCommunityCommand`, or `attachPhysicalSourceCommunityCommand` stales existing discrimination records | `./le rfc check` reports `producer-sha` mismatch | Re-record every affected discrimination record in the same phase (`RFC9012-11-10`, `RFC9494-4.3-2`, `RFC9494-4.3-1`, and any `evaluateLibraryChain` records) and verify `./le rfc check` clean |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `LLGR_STALE` stripped by raw overrides or `NO_EXPORT` stripped under `send-community none`, leaking stale routes beyond IBGP; or `NO_EXPORT` duplicated on IBGP updates; or non-LLGR `attach-community` wrongly depreferencing routes |
| How is it reverted? | Single commit revert per phase; no config syntax or persistent state migration |
| Who else touches this path? | `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md` (split child owning D7 — disjoint files in `locrib`/`sysrib`/`grmarker`); `plan/post-release/features/spec-gr-advanced.md` (RFC 8538 N bit on top of landed D1) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `gr/register.go::init()` registers `0xFFFF0006` + `send-community none` / `community-remove` / export policy Suppress | → | `filterapi` preserved set -> `genericCommunityHandler` | `test/plugin/llgr-stale-community-preserved.ci` and `test/plugin/llgr-stale-community-preserved-forked.ci` |
| IBGP non-LLGR peer with `send-community none` receives LLGR-stale route | → | `gr_filter.go::llgrEgressVerdict` (`TreatmentAttachNoExport`) -> `reactor_api_forward.go` / `forward_rs.go` (`AttrModAttach`) -> `genericCommunityHandler` | `test/plugin/llgr-stale-community-preserved.ci` and `test/plugin/llgr-stale-community-preserved-forked.ci` (IBGP destination + `community { tag }` step) |
| Import or export raw-override plugin returns UPDATE omitting `LLGR_STALE` | → | `reactor/filter_library.go::evaluateLibraryChain` raw-override restore | `TestIngressRawOverrideRestoresPreservedCommunity` and `TestEgressRawOverrideRestoresPreservedCommunity` |
| `request bgp rib attach-community` / `source attach-community` with non-LLGR vs `LLGR_STALE` community | → | `rib_commands_community.go::attachCommunityCommand` & `rib_source_commands.go::attachPhysicalSourceCommunityCommand` | `TestAttachCommunityDepreferencesOnlyLLGRStale` |
| Real peer daemons (FRR & GoBGP) | → | NOTIFICATION teardown (D1) and received `LLGR_STALE` selection/egress/preservation (D3/D4/D5) | `test/interop/scenarios/bgp-graceful-restart-frr/` and `test/interop/scenarios/bgp-llgr-stale-received-frr/` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `bgp-gr` starts and registers `0xFFFF0006` via `filterapi.RegisterPreservedCommunity`; a route carrying `LLGR_STALE` and another standard community is forwarded to peers with `send-community none`, `community-remove ffff0006`, or a policy Set/Suppress | Each destination receives `COMMUNITIES` containing `LLGR_STALE` (`0xFFFF0006`) while the other standard community is removed or replaced as configured, and `filter_community` contains zero hardcoded `LLGR_STALE` constants | Unregistered communities remain removable/suppressible as configured; when `bgp-gr` is not registered, `filterapi` holds no preserved value and `genericCommunityHandler` removes every named community |
| AC-2 | A route WITHOUT `LLGR_STALE` is forwarded to the same `send-community none`, `community-remove`, and policy Set/Suppress destinations (negative polarity) | Destination receives byte-for-byte unchanged output (no `COMMUNITIES` for `send-community none`, configured removals/sets otherwise) | Existing `test/plugin/send-community-suppress.ci` and non-LLGR policy behavior remain unchanged |
| AC-3 | A `filter_community` `community { strip }` definition includes `65535:6` alongside another standard community and runs on ingress | The other standard community is stripped while `LLGR_STALE` stays in the route without emitting a remove op for `65535:6`; a `community { tag }` definition naming `65535:6` still appends `65535:6` | Codes 16 (extended) and 32 (large) community definitions and non-preserved standard values are stripped as configured |
| AC-4 | A raw-override filter on import (`runIngressPolicyChainMeta`) or export (`runEgressPolicyChainASN4`) returns a rewritten UPDATE body that dropped `LLGR_STALE` from an input carrying it | `evaluateLibraryChain` restores `LLGR_STALE` (`0xFFFF0006`) into the returned UPDATE's `COMMUNITIES` attribute while keeping all other raw-override edits intact; an input without `LLGR_STALE` or an override that kept it is returned untouched | All non-preserved attributes and communities modified by the raw override remain as the override wrote them |
| AC-5 | An LLGR-stale route is forwarded (both standard forward rail and RS rail, with `bgp-gr` internal AND forked) to an IBGP non-LLGR peer configured with `send-community none` (or a list omitting `standard`) | `llgr-egress` sets `TreatmentAttachNoExport`, the forward rail records `AttrModAttach` for `NO_EXPORT`, and the peer receives `COMMUNITIES` holding `LLGR_STALE` (`FFFF0006`) and `NO_EXPORT` (`FFFFFF01`) with `LOCAL_PREF 0`, with no duplicate `NO_EXPORT` when `send-community` allows standard communities; an operator `community { tag }` or policy `community-add` toward that `send-community none` peer is still suppressed | Operator-added standard communities (`AttrModAdd`) remain suppressed by `send-community none`; non-stale routes to that peer remain without `COMMUNITIES`; `gr_egress.go::LLGREgressFilter` and its RFC9494-4.6-2 tagged units remain unchanged |
| AC-6 | `request bgp rib attach-community` and `request bgp rib source attach-community` run on GR-stale routes with a non-LLGR community (for example `65000:100`) vs `LLGR_STALE` (`ffff0006`) | Attaching `ffff0006` attaches the community and raises `StaleLevel` to `storage.DepreferenceThreshold` (2) on received and matching sent entries; attaching any other 4-byte community attaches the community to both received and matching sent entries (`attachSentSourceCommunityLocked` checks `level == storage.StaleLevelFresh` instead of `< DepreferenceThreshold`) while keeping `StaleLevel` at 1 (normal selection) | Other community values remain attachable to received and sent copies without falsely triggering RFC 9494 Section 4.3 depreference |
| AC-7 | `RFC9494-4.2-6` and `RFC9494-4.3-2` compliance proof and `./le rfc check` | Tagged tests prove all RFC 9494 Section 4.3 procedures triggered by Section 4.2 self-attachment (depreference, EBGP non-LLGR withdrawal, IBGP Section 4.6 partial deployment, `LLGR_STALE` preservation) and unconditional `LLGR_STALE` preservation; every new or modified tagged unit and edited producer carries a fresh discrimination record; independent re-judgment stamps `enforced` in `rfc/audit/rfc9494/RFC9494-4.2-6.json` and `rfc/audit/rfc9494/RFC9494-4.3-2.json`; `./le rfc check` exits 0 with zero violations for RFC 4724 and RFC 9494 | Existing `enforced` verdicts (`RFC4724-4-2`, `RFC9494-4.2-8`, `RFC9494-4.3-1`, `RFC9494-4.3-3`, `RFC9494-4.6-2`) remain `enforced`; `rfc/short/rfc8538.md` stays `blocked` |
| AC-8 | Interop scenarios `bgp-graceful-restart-frr` (extended with NOTIFICATION phase) and `bgp-llgr-stale-received-frr` (new) | `bgp-graceful-restart-frr` proves FRR `clear bgp` Cease NOTIFICATION flushes FRR's routes on Ze within a bound well below Restart Time and withdraws onward; `bgp-llgr-stale-received-frr` proves Ze depreferences FRR's received `LLGR_STALE` route against GoBGP's lower-LOCAL_PREF copy, withholds it from GoBGP (EBGP non-LLGR), and advertises it with `LLGR_STALE` intact to a second FRR LLGR peer; both scenarios are confirmed red when their respective producer fix is reverted and green with it in place | Existing GR negotiation, route retention, and EOR phases of `bgp-graceful-restart-frr` remain preserved and green |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `send-community none` on an IBGP non-LLGR peer while an upstream LLGR peer restarts or sends `LLGR_STALE` routes | `llgr-egress` (`TreatmentAttachNoExport`) -> `reactor_api_forward` / `forward_rs` (`AttrModAttach`) -> `genericCommunityHandler` -> wire UPDATE carrying `LLGR_STALE` + `NO_EXPORT` | `test/plugin/llgr-stale-community-preserved.ci` and `-forked.ci` |
| 2 | Runs `clear bgp` on an FRR GR neighbour (sending Cease NOTIFICATION) | peer NOTIFICATION -> `peer_close.go` `ReasonNotification` -> `gr.go` `ReasonForbidsRetention` -> RIB flush & onward withdraw | `test/interop/scenarios/bgp-graceful-restart-frr/` |
| 3 | Upstream FRR helper readvertises `LLGR_STALE` routes to Ze beside a GoBGP peer and a second FRR LLGR peer | `llgr-ingress` -> RIB `ReceivedLLGRStale` least preferred -> `llgr-egress` withholds from GoBGP and preserves `LLGR_STALE` to second FRR | `test/interop/scenarios/bgp-llgr-stale-received-frr/` |
| 4 | Runs `request bgp rib attach-community` with a non-LLGR community on GR-stale routes | `attachCommunityCommand` -> attaches community, preserves GR stale level 1 without depreferencing | `TestAttachCommunityDepreferencesOnlyLLGRStale` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPreservedCommunityRegistrySnapshotRestore` | `internal/component/bgp/filterapi/filterapi_test.go` | AC-1: register, query, `HasPreservedCommunities`, `Snapshot`/`Restore` isolation | new |
| `TestGRInitRegistersLLGRStalePreserved` | `internal/component/bgp/plugins/gr/rfc9494_gr_ingress_test.go` | AC-1: `gr` `init()` registers `0xFFFF0006` in `filterapi` | new |
| `TestCommunityHandler_PreservedFromRegistry` | `internal/component/bgp/plugins/filter_community/rfc9494_llgr_stale_preserve_test.go` | AC-1, AC-2: Suppress, Set, Remove preserve registered values and remove unregistered values (tagged RFC9494-4.3-2 positive and negative) | new |
| `TestCommunityHandler_AttachSurvivesSuppress`, `_AttachSurvivesSet`, `_AttachNoDuplicateWhenPresent`, `_AddDroppedBySuppress` | `internal/component/bgp/plugins/filter_community/handler_test.go` | AC-5: `AttrModAttach` survives Suppress and Set, deduplicates against already-emitted `NO_EXPORT`, and `AttrModAdd` stays dropped by Suppress (tagged RFC9494-4.6-2 positive/negative; `_AttachSurvivesSet` untagged handler contract) | new |
| `TestIngressStripExemptsPreservedCommunity` | `internal/component/bgp/plugins/filter_community/filter_test.go` | AC-3: `parseCommunityDefinitions` + `applyIngressFilter` keeps registered `0xFFFF0006` on strip while stripping other values and keeping `tag` intact (tagged RFC9494-4.3-2 positive/negative) | new |
| `TestIngressRawOverrideRestoresPreservedCommunity`, `TestEgressRawOverrideRestoresPreservedCommunity` | `internal/component/bgp/reactor/rfc9494_raw_override_test.go` | AC-4: `evaluateLibraryChain` restores dropped `LLGR_STALE` on both import and export raw overrides; negative polarity leaves non-LLGR inputs untouched (tagged RFC9494-4.3-2) | new |
| `TestLLGREgressSetsAttachNoExportTreatment` | `internal/component/bgp/plugins/gr/gr_filter_test.go` | AC-5: `evaluateLLGREgress` sets `attach-no-export` on Section 4.6 IBGP delivery (both view and forked JSON paths), and omits it on LLGR-capable or EBGP destinations | new |
| `TestForwardRailsPreserveSection46NoExportUnderSendCommunityNone` | `internal/component/bgp/reactor/rfc9494_section46_suppress_test.go` | AC-5: both `forwardUpdateCore` and `reactorForwardRS` preserve `NO_EXPORT` and `LLGR_STALE` under `send-community none` while suppressing operator `AttrModAdd` tags (tagged RFC9494-4.6-2) | new |
| `TestWireModVerbAttach` | `internal/component/bgp/reactor/policy_dryrun_test.go` | AC-5: `wireModVerb(filterapi.AttrModAttach)` returns `"attach"` | new |
| `TestAttachCommunityDepreferencesOnlyLLGRStale` | `internal/component/bgp/plugins/rib/rfc9494_test.go` | AC-6: `attachCommunityCommand`, `attachPhysicalSourceCommunityCommand`, and `attachSentSourceCommunityLocked` attach the community to both received and sent copies, raising `StaleLevel` to 2 for `ffff0006` and keeping level 1 for non-LLGR communities (tagged RFC9494-4.3-1 negative) | new |
| `TestRFC9494Section42SelfAttachFullSection43Procedures` | `internal/component/bgp/plugins/rib/rfc9494_test.go` | AC-7: proves Section 4.2 self-attachment triggers all Section 4.3 procedures (depreference, EBGP non-LLGR withdrawal, IBGP 4.6 partial deployment, `LLGR_STALE` preservation; tagged RFC9494-4.2-6 positive and negative) | new |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `AttrOp.Action` | `0..6` (`AttrModSet`..`AttrModAttach`) | `AttrModAttach` (6) | N/A (`uint8`) | Unknown values map to `"modified"` in `wireModVerb` |
| `policyview.Treatment` slot | `0..7` (bits in `uint8`) | slot 1 (`TreatmentAttachNoExport`) | N/A | >7 panics in `MustRegisterTreatment` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `llgr-stale-community-preserved` | `test/plugin/llgr-stale-community-preserved.ci` | Route carrying `LLGR_STALE` and another standard community forwarded to (1) LLGR peer with `send-community none`, (2) LLGR peer with `community-remove ffff0006`, (3) LLGR peer with export policy suppressing communities, (4) IBGP non-LLGR peer with `send-community none` and `community { tag }`: destinations (1)-(3) receive `FFFF0006`, destination (4) receives exact `FFFF0006 FFFFFF01` with `LOCAL_PREF 0` and no operator tag; fresh control route carries no `COMMUNITIES` | new |
| `llgr-stale-community-preserved-forked` | `test/plugin/llgr-stale-community-preserved-forked.ci` | Same scenario with `bgp-gr` and `bgp-filter-community` running as forked subprocesses, proving `TreatmentAttachNoExport` over the `filter-update` RPC wire | new |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-graceful-restart-frr` (extended) | `test/interop/scenarios/bgp-graceful-restart-frr/` | FRR | Adds a NOTIFICATION phase: FRR `clear bgp <ze>` sends Cease; Ze holds none of FRR's routes within a bound far below Restart Time and withdraws onward (D1) | extend |
| `bgp-llgr-stale-received-frr` | `test/interop/scenarios/bgp-llgr-stale-received-frr/` | FRR (sender with LLGR + `llgr-stale` route-map, higher LOCAL_PREF), GoBGP (EBGP non-LLGR peer + lower-LOCAL_PREF competing route), second FRR (LLGR receiver) | Ze depreferences FRR's received `LLGR_STALE` route against GoBGP, withholds it from GoBGP, and advertises it to the second FRR with `LLGR_STALE` intact (D3/D4/D5) | new |

## Files to Modify
- `internal/component/bgp/filterapi/filterapi.go` - add preserved-community registry (`RegisterPreservedCommunity`, `IsPreservedCommunity`, `HasPreservedCommunities`, `PreservedCommunities`, snapshot/restore support) and `AttrModAttach` action constant
- `internal/component/bgp/policyview/register.go` - register `FieldAttachNoExport` and `TreatmentAttachNoExport`
- `internal/component/bgp/plugins/gr/register.go` - register `attribute.CommunityLLGRStale` in `filterapi` preserved-community registry at `init()`
- `internal/component/bgp/plugins/gr/gr_filter.go` - declare `policyview.FieldAttachNoExport` in `grFilterDecls` and emit it in `llgrEgressVerdict` for the Section 4.6 IBGP branch
- `internal/component/bgp/plugins/filter_community/handler.go` - replace hardcoded `llgrStaleWire` with `filterapi` preserved-community checks and implement `AttrModAttach` (deduplicated) across Suppress, Set, and Remove/Add
- `internal/component/bgp/plugins/filter_community/config.go` - build `stripWireValues` on code-8 `communityDef` omitting registered preserved values while keeping `wireValues` intact for `tag`
- `internal/component/bgp/plugins/filter_community/filter.go` - pass `def.stripWireValues` to `ingressStripCommunities` for standard communities
- `internal/component/bgp/reactor/filter_library.go` - restore registered preserved communities dropped by a raw-override reply in `evaluateLibraryChain` (covering both import and export)
- `internal/component/bgp/reactor/reactor_api_forward.go` - record `AttrModAttach` for `NO_EXPORT` beside `applyFactsSendCommunity` when `TreatmentAttachNoExport` is set
- `internal/component/bgp/reactor/forward_rs.go` - record `AttrModAttach` for `NO_EXPORT` beside `applyFactsSendCommunity` when `TreatmentAttachNoExport` is set
- `internal/component/bgp/reactor/policy_dryrun.go` - add `AttrModAttach` (`"attach"`) case to `wireModVerb`
- `internal/component/bgp/plugins/rib/rib_commands_community.go` - raise `StaleLevel` to `DepreferenceThreshold` in `attachCommunityCommand` only when attaching `attribute.CommunityLLGRStale`
- `internal/component/bgp/plugins/rib/rib_source_commands.go` - raise `StaleLevel` to `DepreferenceThreshold` in `attachPhysicalSourceCommunityCommand` only when attaching `attribute.CommunityLLGRStale`
- `internal/component/bgp/plugins/rib/rib_sent_lifecycle.go` - change `attachSentSourceCommunityLocked` stale guard from `level < storage.DepreferenceThreshold` to `level == storage.StaleLevelFresh` so sent copies receive attached non-LLGR communities while preserving their `StaleLevel` (1) and taking level 2 for `LLGR_STALE`
- `internal/component/bgp/plugins/rib/rfc9494_test.go` - add `TestAttachCommunityDepreferencesOnlyLLGRStale` and `TestRFC9494Section42SelfAttachFullSection43Procedures`
- `internal/le/interoplab/bgp/register_llgr.go` - register checker for `bgp-llgr-stale-received-frr`
- `test/interop/scenarios/bgp-graceful-restart-frr/ze.conf` - support NOTIFICATION teardown phase in `bgp-graceful-restart-frr`
- `docs/architecture/core-design.md` - update `Policy Filter Chain` to reflect the Q-3 departure (replacing the lines 1954–1956 statement that without `bgp-gr` in the chain an operator's removal stands and an import member can still strip `LLGR_STALE`) and document `AttrModSuppress`, `AttrModReplaceNLRI`, `AttrModAttach`, `TreatmentAttachNoExport`, and preserved-community registration
- `docs/architecture/meta/filter-community.md` - update lines 52–54 wording ("before an operator strip can remove it") to note that registered preserved communities (`LLGR_STALE`) are exempt from code-8 strips while inserted-member ordering still prevents an operator `tag` of `LLGR_STALE` from marking the route
- `docs/architecture/testing/interop.md` - update `bgp-graceful-restart-frr` paragraph and add `bgp-llgr-stale-received-frr`
- `docs/guide/graceful-restart.md` - document Section 4.6 IBGP `NO_EXPORT` surviving `send-community none` and unconditional `LLGR_STALE` preservation on raw overrides

## Files to Create
- `internal/component/bgp/reactor/rfc9494_raw_override_test.go` - unit tests for import and export raw-override preserved-community restoration (AC-4)
- `internal/component/bgp/reactor/rfc9494_section46_suppress_test.go` - unit tests for both forward rails preserving Section 4.6 `NO_EXPORT` under `send-community none` while suppressing operator `AttrModAdd` (AC-5)
- `test/plugin/llgr-stale-community-preserved.ci` - internal functional test for AC-1, AC-2, AC-5
- `test/plugin/llgr-stale-community-preserved-forked.ci` - forked-plugin functional test for AC-1, AC-2, AC-5
- `test/interop/scenarios/bgp-llgr-stale-received-frr/ze.conf` - Ze config for received-`LLGR_STALE` interop scenario
- `test/interop/scenarios/bgp-llgr-stale-received-frr/frr.conf` - FRR sender config originating `llgr-stale` route
- `test/interop/scenarios/bgp-llgr-stale-received-frr/gobgp.toml` - GoBGP EBGP non-LLGR peer config
- `internal/le/interoplab/bgp/check_llgr_stale_received.go` - interop checker for `bgp-llgr-stale-received-frr`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config leaf or RPC added |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf; owner Q-3 keeps `community-remove ffff0006` accepted and inert at runtime |
| CLI commands/flags | No | None added |
| CLI grammar (keyword before value) | N-A | None added |
| Editor autocomplete | N-A | None added |
| Functional test for new RPC/API | Yes | `test/plugin/llgr-stale-community-preserved.ci` and `-forked.ci` |
| Pipe completeness | N-A | No output command added |
| Env var registration | No | None added |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | No new failure class or warning code |
| Prometheus counters/metrics | No | None added (owner review accepted no received-stale counter) |
| BGP family surface (new SAFI / capability / attribute) | No | No family, capability, or attribute added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | GR/LLGR feature level unchanged; fixes existing RFC compliance defects |
| 2 | Config syntax changed? | No | No config syntax changes |
| 3 | CLI command added/changed? | No | No CLI commands added or changed |
| 4 | API/RPC added/changed? | No | `filter-update` schema unchanged; `attach-no-export` uses existing `FilterSet` mechanism |
| 5 | Plugin added/changed? | Yes | `docs/guide/graceful-restart.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/graceful-restart.md` |
| 7 | Wire format changed? | No | Standard RFC 1997 4-octet `COMMUNITIES` format unchanged |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin/sdk` and `pkg/plugin/rpc` unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/audit/rfc9494/RFC9494-4.2-6.json`, `rfc/audit/rfc9494/RFC9494-4.3-2.json`, and discrimination records (`rfc/short/rfc9494.md` already dropped the 4.3-2 `{gap}`) |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` |
| 11 | Affects daemon comparison? | No | No daemon comparison table rows change |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (`Policy Filter Chain` Q-3 departure, `AttrModAttach`, `TreatmentAttachNoExport`, preserved-community registration) and `docs/architecture/meta/filter-community.md` (lines 52–54 strip-exemption wording) |
| 13 | Route metadata keys added/changed? | No | `stale` and `rebuilt` metadata keys are already documented at HEAD |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No new plugin, event, send, command, or capability; `bgp-gr:llgr-egress` adds one treatment field to its existing `Sets` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked with `./le spec citation anchors`: modified docs listed in `## Files to Modify` (`docs/architecture/core-design.md`, `docs/architecture/meta/filter-community.md`, `docs/architecture/testing/interop.md`, `docs/guide/graceful-restart.md`); declared `// Design:` page `docs/architecture/buffer-architecture.md` (declared by `reactor_api_forward.go`) is unaffected because `AttrModAttach` reuses the existing buffer-first `ModAccumulator` and `buildModifiedPayload` path into the caller's pooled buffer without changing buffer ownership or encoding contracts |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/graceful-restart.md` readvertisement and community preservation examples |

## Design presentation

### What changes

This spec finishes the remaining BGP control-plane Graceful Restart and Long-Lived
Graceful Restart compliance fixes after D1, D2, D3, D4, and D6 landed in HEAD,
and splits D7 (Restarting Speaker forwarding-state deferral against IGP routes)
into child spec
`plan/release/fixes/spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md`.
Specifically: `bgp-gr` registers `LLGR_STALE` (`0xFFFF0006`) in a `filterapi`
preserved-community registry so `bgp-filter-community` removes its hardcoded
constant and exempts `LLGR_STALE` from code-8 strip sets at config parse;
`reactor/filter_library.go::evaluateLibraryChain` restores dropped preserved
communities on both import and export raw overrides (owner Q-3); `bgp-gr`'s
`llgr-egress` signals RFC 9494 Section 4.6 `NO_EXPORT` protection across both
internal and forked transports via a registered `policyview.Treatment`
(`TreatmentAttachNoExport`), which both forward rails translate into a new
`filterapi.AttrModAttach` op that survives `send-community` Suppress without
letting operator `AttrModAdd` tags leak past `send-community none` (owner Q-7);
`bgp-rib`'s `attach-community` commands raise `StaleLevel` to
`DepreferenceThreshold` only for `LLGR_STALE` (D8); `RFC9494-4.2-6` and
`RFC9494-4.3-2` reach `enforced` audit verdicts; and the FRR/GoBGP Docker interop
scenarios prove NOTIFICATION teardown and received `LLGR_STALE` end to end.

### Before and after

1. **Section 4.6 IBGP peer with `send-community none`:**
   - Before: `llgr-egress` adds `NO_EXPORT` in `exportWireOverride`, then `applyFactsSendCommunity` suppresses code 8 and `genericCommunityHandler` keeps only `LLGR_STALE`, so the IBGP non-LLGR peer receives `COMMUNITIES: 65535:6` without `NO_EXPORT` (violating RFC 9494 Section 4.6).
   - After: `llgr-egress` sets `attach-no-export`, both forward rails emit `AttrModAttach` for `NO_EXPORT`, and the IBGP non-LLGR peer receives `COMMUNITIES: 65535:6 65535:65281` with `LOCAL_PREF 0`, while operator `community { tag }` values stay suppressed.
2. **Import or export raw-override filter dropping `LLGR_STALE`:**
   - Before: `evaluateLibraryChain` returns the raw-override bytes verbatim, dropping `LLGR_STALE` from the stored or advertised route.
   - After: `evaluateLibraryChain` detects any registered preserved community present in the input and missing from the raw override, and restores it into `COMMUNITIES` via `buildModifiedPayload`.
3. **Hardcoded `LLGR_STALE` in `bgp-filter-community` and ingress strip:**
   - Before: `filter_community/handler.go` hardcodes `llgrStaleWire`, and `ingressStripCommunities` strips `65535:6` before `rewriteVerdict` emits a remove op that `genericCommunityHandler` later discards.
   - After: `bgp-gr` registers `0xFFFF0006` via `filterapi.RegisterPreservedCommunity`; `filter_community` reads the registry in both `genericCommunityHandler` and `parseCommunityDefinitions`.
4. **`request bgp rib attach-community` with a non-LLGR community:**
   - Before: attaching `65000:100` to GR-stale routes unconditionally raises `StaleLevel` to 2, depreferencing routes that are not `LLGR_STALE`.
   - After: only attaching `ffff0006` raises `StaleLevel` to 2; any other community keeps `StaleLevel` unchanged.
5. **Interop and RFC verdicts:**
   - Before: `RFC9494-4.2-6` is `weak`, `RFC9494-4.3-2` has no audit verdict, `bgp-graceful-restart-frr` skips NOTIFICATION teardown, and no received-`LLGR_STALE` interop scenario exists.
   - After: both verdicts are independently judged `enforced`, and both Docker interop scenarios pass against FRR and GoBGP (after failing with each fix reverted).

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Change `gr_egress.go::LLGREgressFilter` to emit `AttrModAttach` directly (2026-10-14 round-6 design) | After `spec-filter-wire-6-stage-filters`, `LLGREgressFilter` runs inside `gr_filter.go::evaluateLLGREgress` (possibly in a forked process) and translates ops into `rpc.FilterSet`s baked into `exportWireOverride`; `applyFactsSendCommunity` runs later in the reactor on a fresh `ModAccumulator`, so `AttrModAttach` inside `gr_egress.go` would never reach `applyFactsSendCommunity` and would needlessly require changing two RFC9494-4.6-2 tagged tests (retiring Q-8) |
| Exempt every `AttrModAdd` from `AttrModSuppress` in `genericCommunityHandler` | Would leak operator `community { tag }` and policy `community-add` values past `send-community none` |
| Add `NO_EXPORT` (`0xFFFFFF01`) to `filterapi`'s preserved-community registry | Would prevent operators from stripping received `NO_EXPORT` on ordinary non-LLGR routes, which no RFC requires |
| Keep D7 (Restarting Speaker forwarding deferral vs IGP) inside this spec | D7 touches `locrib`, `sysrib`, `zefs`, and Linux/QEMU kernel verification, completely disjoint from BGP filter/community/interop work; splitting into child spec `spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md` keeps both designs focused |

### Documents departed from

| Document | Section | Before | After | Rationale and doc update |
|----------|---------|--------|-------|--------------------------|
| `docs/architecture/core-design.md` | `Policy Filter Chain` (lines 1954–1956) | `llgr-egress` restores `LLGR_STALE` only via `runFinalJudges` on export; *"without `bgp-gr` in the chain nothing runs and an operator's removal stands. An import member can still strip the community before any export chain runs."* | `bgp-gr` registers `LLGR_STALE` (`0xFFFF0006`) in `filterapi`'s preserved-community registry at `init()`, `filter_community` preserves registered values across Suppress, Set, Remove, and code-8 ingress strip sets even without `bgp-gr` in the chain, and `evaluateLibraryChain` restores any dropped registered preserved community on both import and export raw overrides unconditionally on presence. | Owner decision Q-3 (2026-10-14): RFC 9494 Section 4.3 (*"The LLGR_STALE community MUST NOT be removed when the route is further advertised"*) binds unconditionally on presence and must not be defeated by an import strip, raw override, or chain omitting `bgp-gr`, while keeping `bgp-gr` as the sole owner of the `LLGR_STALE` constant via registration. `docs/architecture/core-design.md` (`Policy Filter Chain`) is updated in this spec (`## Files to Modify` and Documentation Update Checklist rows 12 and 16). |

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add preserved-community registration and `AttrModAttach` in `filterapi/filterapi.go`; register `0xFFFF0006` in `gr/register.go`; register `FieldAttachNoExport` / `TreatmentAttachNoExport` in `policyview/register.go`; declare `attach-no-export` in `gr/gr_filter.go::grFilterDecls`; add failing wiring tests (`TestPreservedCommunityRegistrySnapshotRestore`, `TestGRInitRegistersLLGRStalePreserved`, `TestLLGREgressSetsAttachNoExportTreatment`, and `test/plugin/llgr-stale-community-preserved.ci` red).
2. **Phase: D5a (Preserved-community registration, strip exemption, and raw-override restore)** -- update `filter_community/handler.go`, `config.go`, `filter.go`, and `reactor/filter_library.go::evaluateLibraryChain`. Tests: `TestCommunityHandler_PreservedFromRegistry`, `TestIngressStripExemptsPreservedCommunity`, `TestIngressRawOverrideRestoresPreservedCommunity`, `TestEgressRawOverrideRestoresPreservedCommunity`. Re-record affected discrimination records (`RFC9012-11-10`, `RFC9494-4.3-2`, and any `evaluateLibraryChain` records). Update `docs/architecture/meta/filter-community.md` lines 52–54 wording.
3. **Phase: D5b (Q-7 Section 4.6 `NO_EXPORT` under `send-community` Suppress)** -- emit `attach-no-export` in `gr/gr_filter.go::llgrEgressVerdict`; record `AttrModAttach` in `reactor_api_forward.go` and `forward_rs.go`; honour `AttrModAttach` (deduplicated) in `filter_community/handler.go::genericCommunityHandler`; add `"attach"` in `reactor/policy_dryrun.go::wireModVerb`. Tests: `TestCommunityHandler_AttachSurvivesSuppress`, `_AttachSurvivesSet`, `_AttachNoDuplicateWhenPresent`, `_AddDroppedBySuppress`, `TestForwardRailsPreserveSection46NoExportUnderSendCommunityNone`, `TestWireModVerbAttach`, `test/plugin/llgr-stale-community-preserved.ci` and `-forked.ci`. Update `docs/architecture/core-design.md` (`Policy Filter Chain`) and `docs/guide/graceful-restart.md`.
4. **Phase: D8 (`attach-community` depreference scope)** -- restrict `DepreferenceThreshold` raise to `attribute.CommunityLLGRStale` in `rib_commands_community.go` and `rib_source_commands.go`, and change `attachSentSourceCommunityLocked` in `rib_sent_lifecycle.go` from `level < storage.DepreferenceThreshold` to `level == storage.StaleLevelFresh` so sent copies receive attached non-LLGR communities while preserving their `StaleLevel` (1) and taking level 2 for `LLGR_STALE`. Tests: `TestAttachCommunityDepreferencesOnlyLLGRStale`. Re-record affected `RFC9494-4.3-1` discrimination records.
5. **Phase: Interop (I1)** -- extend `test/interop/scenarios/bgp-graceful-restart-frr/` with the NOTIFICATION teardown phase, add `test/interop/scenarios/bgp-llgr-stale-received-frr/` and its checker in `internal/le/interoplab/bgp/`, confirm red with each fix reverted and green with it in place, and update `docs/architecture/testing/interop.md`.
6. **Phase: Verdicts & Journal Closure (V1)** -- add `TestRFC9494Section42SelfAttachFullSection43Procedures`, record all new and refreshed discrimination records, verify `./le rfc check` clean, obtain independent `enforced` audit verdicts for `RFC9494-4.2-6` and `RFC9494-4.3-2`, and mark all six control-plane owned journal rows `fixed:` in Commit A.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has an implementation at file:line and a passing test |
| Feature completeness | Both internal and forked `bgp-gr` / `bgp-filter-community` pass `llgr-stale-community-preserved` |
| Correctness | `AttrModAttach` never duplicates `NO_EXPORT` when `send-community` allows standard communities; `AttrModAdd` stays dropped by Suppress and Set; raw-override restore preserves all other override edits; `attach-community` raises level only for `0xFFFF0006` |
| Data flow | `filter_community` holds zero LLGR constants; `llgr-egress` communicates `NO_EXPORT` protection only via `policyview.TreatmentAttachNoExport` (surviving process boundary) |
| Rule: performance | `AttrOp` stays 32 bytes; zero allocations on `genericCommunityHandler` retained-run path; raw-override restore does zero work when `raw == nil` or input carries no preserved community |
| Rule: rfc-compliance | Every new enforcing function quotes its RFC 9494 Section 4.3 / 4.6 sentence; `./le rfc check` passes with zero violations |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero hardcoded `LLGR_STALE` constant in `filter_community` | `git grep -n "llgrStaleWire\|FFFF.*0006\|65535:6" internal/component/bgp/plugins/filter_community/` returns no non-test match |
| Internal and forked `.ci` proofs pass | `./le test plugin llgr-stale-community-preserved` and `llgr-stale-community-preserved-forked` |
| Interop scenarios pass (after red confirmation) | `./le test integration interop` for `bgp-graceful-restart-frr` and `bgp-llgr-stale-received-frr` |
| Audit verdicts `enforced` and records clean | `./le rfc check` exits 0; `rfc/audit/rfc9494/RFC9494-4.2-6.json` and `RFC9494-4.3-2.json` report `"verdict": "enforced"` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Malformed `COMMUNITIES` lengths are already treat-as-withdraw at receipt (RFC 7606); raw-override restore ignores partial trailing bytes and fails closed if `buildModifiedPayload` refuses |
| Policy bypass | Preserved-community registry holds `LLGR_STALE` (`0xFFFF0006`) only; `TreatmentAttachNoExport` is set only by `bgp-gr:llgr-egress` on Section 4.6 IBGP delivery, so operator `community { tag }` / `community-add` (`AttrModAdd`) never leak past `send-community none` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read source in Current Behavior; if misunderstood → RESEARCH |
| Functional or interop test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `negotiated-family-lost-at-retention-boundary` | 2026-10-04 | `onSessionDownDeferred` (`gr/gr_state.go`) -- fixed in HEAD (`onSessionDownPlan`), close row in Commit A |
| `escape-hatch-scoped-wider-than-its-justification` | 2026-10-18 | `attachCommunityCommand` (`rib/rib_commands_community.go`) -- fixed by D8 |
| `declared-format-contradicts-payload` | 2026-10-02 | `formatCapability` (`format/decode.go`) -- fixed in HEAD (`buf[2:]`), close row in Commit A |
| `unwired-feature` | 2026-09-08 | `parseGRCapValue` (`gr/gr_capability.go`) -- fixed in HEAD (GR family tuples), close row in Commit A |
| `test-against-broken-path` | 2026-10-23 | `TestLateReflectorClaimsExistingReplayOwner` (`adj_rib_in/rib_validation_config_test.go`) -- fixed in HEAD (`NewBridgedConn`), close row in Commit A |
| `false-synchronization-claim` | 2026-10-31 | `retainRoutes` (`rib/rib_commands.go`) -- `gr.go` now uses `source retain-routes` (`peerMeta` + `dispatchSourceAnswer`); verify and close row in Commit A |

## Work Not Done

| Item | Destination spec |
|------|------------------|
| D7: Restarting Speaker forwarding-state retention against competing IGP routes during `selectionDeferral` (`RFC4724-4.1-1`, journal row `reconciler-claims-state-another-owner-installed` 2026-11-24) | `plan/release/fixes/spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| D5a: `filterapi` preserved-community registry populated by `bgp-gr` at `init()`, consulted by `genericCommunityHandler`, `parseCommunityDefinitions`, and `evaluateLibraryChain` raw-override branch | Hardcode `llgrStaleWire` in `filter_community`; handle raw overrides only in `llgr-egress` `JudgesFinal` | Removing the hardcoded constant keeps `bgp-gr` owning its protocol surface (`ai/doors/system.md`); restoring in `evaluateLibraryChain` covers both import (no `runFinalJudges`) and export raw overrides unconditionally (owner Q-3) |
| D5b (Q-7): `policyview.TreatmentAttachNoExport` set by `llgr-egress` over `filter-update`, turned into `filterapi.AttrModAttach` on both forward rails, honoured by `genericCommunityHandler` with deduplication | Change `gr_egress.go::LLGREgressFilter` to emit `AttrModAttach` directly; exempt all `AttrModAdd` from Suppress; put `NO_EXPORT` in the preserved-community registry | `llgr-egress` runs in `runEgressPolicyChain` (internal or forked) before `applyFactsSendCommunity`; a `policyview.Treatment` crosses the process boundary using existing machinery, preserves operator `send-community none` isolation, and keeps `LLGREgressFilter` and its RFC9494-4.6-2 tagged units unchanged |
| D8: scope `DepreferenceThreshold` raise in `attach-community` to `attribute.CommunityLLGRStale` | Keep raising level for any attached community | RFC 9494 Section 4.3 justifies depreference for `LLGR_STALE` alone |
| Split D7 into child spec `spec-bgp-graceful-restart-rfc-defects-1-deferral-forwarding.md` | Keep D7 inside this control-plane spec | D7 spans `locrib`, `sysrib`, `zefs`, and Linux/QEMU kernel forwarding verification, completely independent of BGP filter/community/interop phases |

## Known Limitations

- RFC 8538 N-bit retention stays in `plan/post-release/features/spec-gr-advanced.md`.
- RFC9494-4.2-7, 4.2-9, 4.2-10, and 5-2 are already implemented in HEAD (`18b491ae23`, `26d70dfa0d`, `21587224da`, `3c2a055fd9`).

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code: RFC
9494 Section 4.3 at `gr/register.go` preserved-community registration,
`filter_community/config.go` strip exemption, `filter_community/handler.go`
preserved-community lookups, `reactor/filter_library.go` raw-override restore,
and `rib` `attach-community` `LLGR_STALE` guard; RFC 9494 Section 4.6 at
`gr/gr_filter.go::llgrEgressVerdict` `TreatmentAttachNoExport` emission,
`reactor_api_forward.go` and `forward_rs.go` `AttrModAttach` ops, and
`filter_community/handler.go` `AttrModAttach` emission.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 9494 Section 4.3 (RFC9494-4.3-2) | `LLGR_STALE` (`0xFFFF0006`) is preserved on further advertisement across `send-community`, `community-remove`, `community { strip }`, and raw-override import/export filters | Non-LLGR communities obey configured `send-community`, `strip`, `remove`, and raw overrides unchanged; `community { tag }` still adds all configured values | `TestCommunityHandler_PreservedFromRegistry`, `TestIngressStripExemptsPreservedCommunity`, `TestIngressRawOverrideRestoresPreservedCommunity`, `TestEgressRawOverrideRestoresPreservedCommunity`, `test/plugin/llgr-stale-community-preserved.ci` | `test/plugin/send-community-suppress.ci`, negative polarity arms of each unit |
| RFC 9494 Section 4.6 (RFC9494-4.6-2) | Internal non-LLGR peer receiving an LLGR-stale route receives `NO_EXPORT` (`0xFFFFFF01`) and `LOCAL_PREF 0` even when `send-community` suppresses standard communities | Operator `community { tag }` and policy `community-add` (`AttrModAdd`) remain suppressed under `send-community none`; LLGR-capable peers receive routes unchanged | `TestCommunityHandler_AttachSurvivesSuppress`, `TestForwardRailsPreserveSection46NoExportUnderSendCommunityNone`, `test/plugin/llgr-stale-community-preserved.ci`, `-forked.ci` | `TestCommunityHandler_AddDroppedBySuppress`, `test/plugin/llgr-stale-community-preserved.ci` tag step |
| RFC 9494 Section 4.2 / 4.3 (RFC9494-4.2-6, RFC9494-4.3-1) | Self-attached `LLGR_STALE` triggers all Section 4.3 procedures; only `LLGR_STALE` raises `StaleLevel` to `DepreferenceThreshold` | Operator/plugin `attach-community` of non-LLGR communities attaches the community without altering `StaleLevel` | `TestRFC9494Section42SelfAttachFullSection43Procedures`, `TestAttachCommunityDepreferencesOnlyLLGRStale` | Negative polarity arm of `TestAttachCommunityDepreferencesOnlyLLGRStale` |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, checkboxes kept unticked
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks every row Yes / No / N-A with reason

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted package, functional `.ci`, and interop tests pass in the main checkout (run by the main thread; GitHub CI runs whole-tree verification on push)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (before the fix)
- [ ] Tests PASS (after the fix)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior (internal and forked)
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal row closures
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-graceful-restart-rfc-defects.md` only, in the same `./le commit create` script
