# Spec: bgp-refresh-reapplies-export-policy

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | A route refresh reapplies export policy to its prior output; pre-policy input and reconciliation gaps remain |
| RFC gaps | rfc2918 |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Scope is protocol: the wire change is the attributes of a re-advertised route, and
a withdrawal where a denied route used to stay. No config or CLI surface changes.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`bgp-rib` keeps Adj-RIB-Out as sent, after export policy. A refresh hands those
bytes back, and `sendBatchUpdate` writes them with `writeUpdate`, which runs the
export gate again with no replay exemption. A prepend member therefore applies one
extra time on each refresh. It does not accumulate, because `handleSent` drops a
sent event that carries `metaKeyReplay`, so `ribOut` keeps the first bytes. The
whole-chain run of the stale rail reads the same bytes and inherits the defect.

Skipping the chain on a refresh is wrong. RFC 2918 Section 4: "Otherwise, the BGP
speaker shall re-advertise to that peer the Adj-RIB-Out of the <AFI, SAFI> carried
in the message, based on its outbound route filtering policy." The fix gives the
chain the bytes it should read: the RIB keeps the attributes as they stood before
export policy (alongside the producing rail and ASN4 encoding provenance), and a
refresh or whole-chain stale run evaluates the current chain once over those.

This spec tracks the six original reconciliation defects (rebased against HEAD on
2026-11-26) plus the raw-injection watermark sibling from its owned journal rows:

| # | Defect | Producer | HEAD Status (2026-11-26) |
|---|--------|----------|--------------------------|
| D-1 | Refresh and whole-chain stale pass reapply export policy over post-policy `ribOut` attributes | `writeUpdateGated`, `writeUpdateBody`, `forwardUpdateCore`, `reactorForwardRS`, `onMessageReceived`, `handleSent` / `handleSentStructured`, `ribOutEntry`, `replayLegacyRefresh`, `replayConfiguredSent` | **Open** |
| D-2 | Policy denial on re-advertisement suppresses instead of withdrawing a held route | `decideStaleReadvertise` (stale rail) and `writeUpdateGated` (ordinary refresh gate) | **Partial**: stale-rail whole-chain denial now returns `staleWithdraw` (`reactor_api_batch.go:2747`); ordinary refresh suppression in `session_write.go:633` remains **Open** (AC-7) |
| D-3 | Definition change inside an external filter plugin is invisible to the export policy watermark | `filterapi.Policy.SameDecisions`, `policychain.member.sameDecisions`, in-tree `filter-update` plugins | **Open** (AC-8) |
| D-4 | Rebuilt policy chain with an in-process func member never compares equal under `reflect.DeepEqual` | `applyHotSwappableSettingsWithGraphDiff`, `peerSettingsEqual`, `peerSettingsSwapPlan` | **Partial**: `applyHotSwappableSettingsWithGraphDiff` (`peer_settings_apply.go:590`) already uses `Policy.SameDecisions`; `reactor_api.go::peerSettingsEqual` (including its false no-funcs comment) and `peer_settings_apply.go::peerSettingsSwapPlan` field walk remain **Open** (AC-9) |
| D-5 | Stale decision taken before `writeMu` can write under the old chain after a reload | `sendStaleReadvertiseUnit`, `Session.noteExportWrite` | **Partial**: `Session.noteExportWrite` (`stale_readvertise_chain.go:102`) already raises the watermark past a late old receipt; pre-write generation check under `writeMu` preventing the old-chain write itself remains **Open** (AC-11) |
| D-6 | Reload changing policy during an initial-sync hold stalled at `joinExportWriters` and rested at `MaxUint64` | `policy_reload.go`, `stale_readvertise_chain.go::noteExportPolicyChangeLocked` | **Already resolved on HEAD**: `joinExportWriters` and `MaxUint64` are gone; `noteExportPolicyChangeLocked` uses lock-free atomic `exportGeneration.Add(1)` and `raiseExportPolicyFrom(nextMsgID())`. Original AC-12/AC-13 are retired |
| D-7 | Explicit raw injection (`raw=true`) carries generation zero and falsely raises the export watermark once `exportGeneration > 0` | `session_write.go::writeUpdateBody` (`:988`), `stale_readvertise_chain.go::noteExportWrite` | **Open** (new AC-12) |

Goal: one chain run over pre-policy attributes on every re-advertisement, the peer
left holding what the current policy produces (withheld or withdrawn when denied),
and an export policy watermark that moves on real definition changes (including
external filter plugins) and never on no-op rebuilds or raw diagnostic writes.

## Current investigation (2026-11-26)

This section updates the 2026-11-15 investigation against HEAD. It is a source
investigation, not a fresh test pass or implementation sign-off. The reported CI
failure is `TestOrdinaryRefreshDoesNotDoublePrepend` at
`reactor/refresh_prepend_double_test.go:89`: the first AS_PATH is
`02030000fde80000fde80000fde9`, and refresh produces
`02040000fde80000fde80000fde80000fde9`.

**Diagnosis.** `Session.writeUpdateGated` runs `s.egressRouteFilter` over the
input body; `Session.writeUpdateBody` reports only the final emitted body through
`s.onMessageReceived`. Forward rails (`reactor_api_forward.go::forwardUpdateCore`
and `forward_rs.go::reactorForwardRS`) run the export chain on received wire and
then apply the RFC 4271 Section 9.1.2 local-AS prepend, also reporting only the
final emitted body. `Reactor.onMessageReceived` (`reactor_notify.go:650-683`)
copies that final body and derives `RawMessage.AttrsWire` from it. Both
`RIBManager.handleSent` (`rib.go:1085`) and `handleSentStructured`
(`rib_structured.go:545`) intern only those post-policy attributes into
`ribOutEntry.AttrHandle`. Refresh enters `handleRefresh` / `handleRefreshStructured`
-> `queueSelectedRefresh` (`rib_selected_refresh.go:493`): legacy/non-selected
recipients call `replayLegacyRefresh` -> `collectRibOutRoutes` -> `sendRoutes`,
and ordinary selected recipients replay configured static routes through
`replayConfiguredSent` (`rib_selected_refresh_configured.go:42`), both reading
`reconstructRoute` and feeding post-policy attributes back into
`announceTarget.sendBatchUpdate` -> `Session.writeUpdate`, which runs the export
chain a second time. Both sent consumers discard `metaKeyReplay` feedback, so
successive refreshes repeat one extra prepend rather than growing unboundedly.

The owning layer is the sent-event and RIB refresh-input boundary. **[workaround]**
Skip the chain for a receipt that appears current, or strip one leading ASN. That
bypasses current or stateful policy, or destroys legitimate AS_PATH edits.
**[source]** Carry the pre-policy attributes and their producing rail and ASN4
width provenance on the sent event (only when they differ from the sent attributes),
intern them in `pool.RibOut` beside `AttrHandle`, and select them explicitly for
refresh and whole-chain stale evaluation. Keep the ordinary export gate with no
replay exemption. The isolated red test `TestOrdinaryRefreshDoesNotDoublePrepend`
supplies post-policy bytes directly to `sendBatchUpdate`, bypassing the RIB
boundary; replacing it goes to `ze-test-judge`.

**Pinned wire and event contracts.**
- **Sent event pre-policy fields (D-1):** On `format=full` JSON (`format/text_update.go::appendRawSectionsJSON`, parsed in `event.go::parseRawFields`), three additive kebab-case members inside `"raw"`, emitted only when pre-policy attributes differ byte-for-byte from the sent attributes:
  1. `"pre-policy-attributes"` (string, hex-encoded wire path attributes fed to the export chain, MP_REACH/MP_UNREACH excluded).
  2. `"pre-policy-rail"` (string, `"announce"` or `"forward"`, recording whether the pre-policy bytes precede the announce builder's export gate or the forward rail's chain-then-local-AS-prepend order).
  3. `"pre-policy-asn4"` (boolean, `true` for 4-octet AS_PATH encoding, `false` for 2-octet AS_PATH encoding of those pre-policy bytes).
  On the structured DirectBridge path (`bgptypes.RawMessage`), matching typed fields `PrePolicyAttrs []byte`, `PrePolicyRail bgptypes.PrePolicyRail`, and `PrePolicyASN4 bool` carry the same values without hex conversion.
- **Replay command provenance metadata (D-1):** `bgptypes.PrePolicyRailMeta` (`"pre-policy-rail"`) and `bgptypes.PrePolicyASN4Meta` (`"pre-policy-asn4"`) travel in `CommandContext.Meta` alongside `metaKeyReplay` and `SentOwnerMessageMeta`, parsed in `cmd/update/update_text.go` onto `NLRIBatch.PrePolicyRail` and `NLRIBatch.PrePolicyASN4` so the reactor preserves forward-rail order (chain, then local-AS prepend) and ASN4 decoding on replay.
- **External filter definition-changed event (D-3):** Namespace `"bgp"` (`bgpevents.Namespace`), event type `"filter-definition-changed"` (`bgpevents.EventFilterDefinitionChanged`), message token `"bgp:filter-definition-changed"`, emitted over `sdk.Plugin.EmitEvent` (DirectBridge in-process, `emit-event` RPC when forked) with JSON payload `{"refs":["<plugin>:<filter>", ...]}` (each entry a registered `<plugin>:<filter>` reference whose definition changed). Because `internal/component/plugin/server/bus_op_gate.go` fails closed on `emit-event` unless the emitting participant declared the message, every emitting filter plugin MUST declare `{Message: "bgp:filter-definition-changed"}` in both its compiled `registry.Registration.Sends` (for in-tree plugins) and its Stage 1 `sdk.Registration.Sends` (for both in-tree and third-party external plugins). The reactor subscribes and raises `peer.noteExportPolicyChangeLocked()` for each peer whose `ExportPolicy.NamesExternalFilter(plugin, filter)` matches; unknown or unmatched references raise nothing.

**D-5 decision (AC-11 vs `noteExportWrite`).** `Session.noteExportWrite`
(`stale_readvertise_chain.go:102`) already closes the watermark-assurance hole by
raising `exportPolicyFrom` past a late old receipt (`receipt + 1`), so a future
stale pass will not mistake it for current. It does NOT prevent a stale unit
decided under the old chain from writing old-chain bytes to TCP after a reload
replaced the policy. Running filter RPCs under `writeMu` would stall KEEPALIVEs,
so the clean design keeps decision outside `writeMu` and adds a pre-write
generation check under `writeMu` in `sendBatchUpdatePreFiltered`: if
`owner.exportGeneration != peer.exportGeneration.Load()`, the writer returns a
typed sentinel `errExportGenerationChanged` without writing bytes, and
`sendStaleReadvertiseUnit` re-reads `staleChainFor` and re-decides under the new
chain (bounded by a fixed retry cap of 8 attempts, failing closed with a logged
withhold on a sustained reload storm). `noteExportWrite` remains as the
defense-in-depth receipt watermark guard across all rails.

**Preserve the sent view.** `reconstructRoute` (`ribout_entry.go:113`) is shared
by the sent pipeline (`rib_pipeline.go`), grouped replay (`rib_replay.go`), abort
restoration (`source_policy.go::restoreSentLocked`), and lifecycle withdrawals
(`rib_sent_lifecycle.go::sentRemoval`). Keep `reconstructRoute` returning the sent
view and add an explicit pre-policy selection helper (`reconstructRefreshRoute` /
`collectRibOutRefreshRoutes`) for `replayLegacyRefresh`, `replayConfiguredSent`,
and whole-chain stale/resend callers. `groupKey` in `rib_replay.go` distinguishes
`PrePolicyAttrHandle`, `PrePolicyRail`, and `PrePolicyASN4`.

**Verification of the extra owned journal rows against HEAD (2026-11-26).**
| Journal row (2026-11-06) | HEAD verification | Disposition in this spec |
|--------------------------|-------------------|--------------------------|
| `state-event-emitted-without-comparing-state.md`: `peerSettingsEqual` runs `DeepEqual` over the policies and reads `refused` | Still open at `reactor_api.go:1648-1666` and `peer_settings_apply.go:261-267` | Fixed in this spec (D-4, AC-9) |
| `state-event-emitted-without-comparing-state.md`: raw injection carries no generation and raises the export watermark | Still open at `session_write.go:988` (`s.noteExportWrite(receipt, body)` runs even when `raw == true`) | Fixed in this spec (D-7, AC-12) |
| `policy-change-never-re-evaluates-what-it-admitted.md`: a filter's Withdraw on a no-owner stale batch removes whatever the key holds | `sendOwnRouteWithdrawal` (`session_ownership.go:512-518`) already spares received routes (`entry.owner.source != nil`); residual two-originated-routes case applies only when `SentOwnerMessage == 0`, while all RIB refresh/stale batches carry `SentOwnerMessage` (`adjOutExpected`) | Kept in owned rows; verified residual documented in Known Limitations (a caller with no receipt cannot name one originated revision over another) |
| `guard-added-to-one-half-of-a-pair.md`: import member strips `LLGR_STALE`; forward-rail `LOCAL_PREF` and `NO_EXPORT` undo | `filter_community/handler.go:87-97,108-126,198-202` already preserves `LLGR_STALE` (0xFFFF0006) across `Suppress`, `Set`, and `Remove` on both import and export (`filter_community/rfc9494_llgr_stale_preserve_test.go`), and `runFinalJudges` protects export-rail `LLGR_STALE`/`LOCAL_PREF`/`NO_EXPORT` (`stale_export_stamp_test.go`). Only a raw external import override could still drop `LLGR_STALE` | Kept in owned rows; note residual raw-override import case in Known Limitations |
| `guard-added-to-one-half-of-a-pair.md`: RFC4271-5.1.2-2 no tagged unit reaches the route-server rail AS_PATH guards | **Already resolved on HEAD** by `internal/component/bgp/reactor/rfc4271_rs_aspath_export_test.go::TestRouteServerIBGPASPathExportEdits` and `TestRouteServerIBGPASPathProtocolEdits` (cited in `rfc/requirements/rfc4271.md:39`) and owned by `plan/release/validation/spec-rfc-weak-verdict-proof-gaps.md` | Duplicate row removed from this spec's owned table |
| `green-that-could-not-have-been-red.md`: `TestRFC4271LocalPrefIncludedForInternalPeers` claims more than its body checks | Still open at `internal/component/bgp/reactor/rfc4271_test.go:88-90` (tag claims general `RFC4271-9.1.1-2` stored LOCAL_PREF while body checks only default 100) | Kept in owned rows; narrow its tag claim under `ze-test-judge` in Phase 5 |

**Peer implementation evidence, read locally.** FRR `cd39d029` handles refresh in
`bgpd/bgp_packet.c::bgp_route_refresh_receive` -> `bgp_announce_route` ->
`bgpd/bgp_updgrp_adv.c::subgroup_announce_table` ->
`bgpd/bgp_route.c::subgroup_announce_check`, reading `pi->attr` into the outbound
attribute work area and applying `ROUTE_MAP_OUT` once. BIRD `02d082a7` calls
`channel_request_feeding` from `proto/bgp/packets.c::bgp_rx_route_refresh` ->
`nest/rt-table.c::rt_feed_channel` -> `rt_notify_basic`, evaluating `c->out_filter`
over the table route. Both re-evaluate policy from routing input, never from prior
post-policy output bytes.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` "Policy Filter Chain", "18a. BGP Peer Reload: Swap or Restart", and `## Design decisions`
  → Decision: the engine holds no policy and no Loc-RIB; a chain member is reached through `filter-update` on both transports, and `Policy.SameDecisions` compares chain definitions.
  → Constraint: every route-processing step is a filter in the peer's filter list (`ai/META-DECISIONS.md`). No special case may name one member, such as prepend.
- [ ] `docs/architecture/bgp/egress-attribute-rules.md` - the order of egress rules
  → Constraint: the forward rail runs the chain on the received wire and then applies the RFC 4271 Section 9.1.2 local-AS prepend. Replay of a forward-rail route keeps that order.
- [ ] `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/pool-architecture.md`, `ai/digests/rib.md` - `pool.RibOut`, interned attributes
  → Constraint: attribute bytes live in `pool.RibOut`, shared and refcounted; release must cover both handles on every removal/overwrite path.
- [ ] `docs/architecture/api/process-protocol.md`, `docs/architecture/api/json-format.md` - the sent event and `emit-event`
  → Constraint: a sent event member and an emitted event type are published contracts for third-party plugins. New members are additive, kebab-case, and documented on their pages.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2918.md` - re-advertisement
  → Constraint: RFC 2918 Section 4: the BGP speaker re-advertises Adj-RIB-Out based on its outbound route filtering policy. The chain runs once over pre-policy input on every refresh.
- [ ] `rfc/short/rfc7313.md` - BoRR and EoRR
  → Constraint: RFC 7313 Section 4: a speaker removes routes still marked stale at EoRR, so enhanced refresh already removes an omitted route. Without RFC 7313, nothing removes a denied route unless an explicit withdrawal is sent (AC-7).
- [ ] `rfc/short/rfc9494.md` - LLGR
  → Constraint: `llgr-egress` keeps its own suppress and withdraw verdicts on the opted-member run over the sent body.

**Key insights:**
- The bytes the chain reads are the defect; the writer is doing what it should.
- There are two rails with two orders: the chain before local-AS prepend (forward), and after the builder (announce). The stored pre-policy form and its rail/ASN4 provenance travel together.
- D-6 (`joinExportWriters` / `MaxUint64`) is already gone on HEAD; the lock-free `exportGeneration` + `raiseExportPolicyFrom` watermark model is the foundation D-3, D-4, D-5, and D-7 complete.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` - `sendBatchUpdate` passes `(*Session).writeUpdate` whether or not `batch.Replay`; `decideStaleReadvertise` now returns `staleWithdraw` for a whole-chain (`chain.judgeAll`) rejection (`:2747`) while `sendStaleReadvertiseUnit` decides outside `writeMu` and does not re-check `exportGeneration` under `writeMu` before writing
- [ ] `internal/component/bgp/reactor/session_write.go` - `writeUpdateGated` runs `egressRouteFilter` when `gate` is true and returns `nil` on `suppress` (`:633`) even for a replay of a held route; `writeUpdateBody` reports only the final emitted body to `onMessageReceived` (`:1051`) and calls `s.noteExportWrite(receipt, body)` unconditionally at `:988` even when `raw == true`
- [ ] `internal/component/bgp/reactor/stale_readvertise_chain.go` - `staleChainFor` reads policy, `exportGeneration`, and `exportPolicyNewerThan(message)` together; `noteExportPolicyChangeLocked` atomically increments `exportGeneration` and raises `exportPolicyFrom` to `nextMsgID()` (lock-free, no barrier); `Session.noteExportWrite` raises the watermark past a late old receipt (`:102`); `runStaleChain` runs the whole chain over the rebuilt sent body when `judgeAll` is set
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go`, `internal/component/bgp/reactor/forward_rs.go` - forward rails run `runEgressPolicyChain` on received wire before applying EBGP local-AS prepend, without recording pre-policy attributes or rail/ASN4 provenance for the sent event
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `onMessageReceived` (`:650-683`) constructs sent `bgptypes.RawMessage` with only final `RawBytes` / `WireUpdate` / `AttrsWire`
- [ ] `internal/component/bgp/reactor/peer_settings_apply.go` - `applyHotSwappableSettingsWithGraphDiff` (`:590`) already compares export policy via `!exportBefore.SameDecisions(p.settings.ExportPolicy)`; `peerSettingsSwapPlan` (`:261-267`) still runs raw `reflect.DeepEqual` across every struct field including `ImportPolicy` and `ExportPolicy`
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `peerSettingsEqual` (`:1623`) still runs `reflect.DeepEqual` over `ImportPolicy` and `ExportPolicy` (reading evaluator funcs and mutable `refused` counters) and carries a false comment claiming `PeerSettings` holds no funcs (`:1645`)
- [ ] `internal/component/bgp/reactor/policy_reload.go` - verified `joinExportWriters` and `MaxUint64` are absent on HEAD; only export-final reconciliation remains here
- [ ] `internal/component/bgp/filterapi/policy.go` - `Policy.SameDecisions` (`:191`) compares final, refs, bindings, template, and `LibraryChain.SameDecisions`; lacks a predicate to test whether an export policy references a given external `(plugin, filter)`
- [ ] `internal/component/bgp/policychain/chain.go` - `Chain.SameDecisions` (`:333`) ignores evaluator funcs and `refused` counters, while `member.sameDecisions` (`:311`) notes external member definition changes are invisible to it
- [ ] `internal/component/bgp/types/rawmessage.go`, `internal/component/bgp/types/types.go`, `internal/component/bgp/event.go`, `internal/component/bgp/format/text_update.go` - sent event (`format=full` `"raw"` object and structured `RawMessage`) carries only post-policy `raw.attributes` / `AttrsWire`; no pre-policy attributes or rail/ASN4 provenance
- [ ] `internal/component/bgp/plugins/rib/ribout_entry.go`, `rib.go`, `rib_structured.go`, `rib_replay.go`, `rib_commands.go`, `rib_selected_refresh.go`, `rib_selected_refresh_configured.go` - `ribOutEntry` holds one `AttrHandle` (sent bytes); `handleSent` and `handleSentStructured` intern only sent attributes; `replayLegacyRefresh` and `replayConfiguredSent` replay `reconstructRoute`'s sent attributes
- [ ] `internal/component/bgp/reactor/refresh_prepend_double_test.go` - isolated red test feeding post-policy bytes directly to `sendBatchUpdate`, bypassing the RIB boundary
- [ ] `internal/component/bgp/reactor/rfc4271_test.go` - `TestRFC4271LocalPrefIncludedForInternalPeers` (`:88-90`) overclaims general `RFC4271-9.1.1-2` stored LOCAL_PREF while checking only default 100

**Behavior to preserve:**
- The export chain runs on every refresh, for every member, under the current policy.
- `ribOut` and `reconstructRoute` continue to present what the peer was sent for `show`, lifecycle withdrawals, abort restoration, and Section 9.2 suppression.
- The opted-member stale run (`!judgeAll`) continues to read the sent body, so `llgr-egress` keeps its verdicts over what the peer holds.
- Forward-rail order: export chain on received wire, then local-AS prepend.
- Lock-free `noteExportPolicyChangeLocked` and `Session.noteExportWrite` receipt watermark protection remain intact.

**Behavior to change:**
- Sent events carry optional pre-policy attributes plus producing rail and ASN4 width when pre-policy differs from sent; `ribOutEntry` interns the second handle in `pool.RibOut` and releases both handles on every removal/overwrite.
- Refresh (`replayLegacyRefresh`, `replayConfiguredSent`) and whole-chain stale runs (`judgeAll`) read pre-policy attributes and preserve the producing rail order and ASN4 width.
- An ordinary refresh denial (`writeUpdateGated` `suppress == true` on a replay of a held route) synthesizes and writes a withdrawal under the same ownership fence.
- External filter plugins (in-tree `filter-update` plugins and third-party plugins) emit `bgp/filter-definition-changed` when a definition changes, and the reactor raises the export watermark of matching peers.
- `peerSettingsEqual` and `peerSettingsSwapPlan` compare `ImportPolicy` and `ExportPolicy` via `Policy.SameDecisions` (and `Implied()`), removing the remaining `reflect.DeepEqual` over policy funcs/counters.
- `sendBatchUpdatePreFiltered` / `sendStaleReadvertiseUnit` re-checks `exportGeneration` under `writeMu` before writing stale-rail bytes, re-deciding outside `writeMu` if the generation moved (bounded retry, fail-closed).
- Raw diagnostic writes (`raw == true`) skip `Session.noteExportWrite` so they never falsely raise `exportPolicyFrom`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Adj-RIB-Out store | `internal/component/bgp/plugins/rib/ribout_entry.go` `ribOutEntry`, `pool.RibOut` | One interned attribute handle (`AttrHandle`) per route, as sent | Extends: adds `PrePolicyAttrHandle`, `PrePolicyRail`, `PrePolicyASN4` (handle invalid/zero when byte-identical to sent bytes); both handles released in `release()`; `reconstructRoute` stays the sent view |
| Sent event | `format/text_update.go` `appendRawSectionsJSON`, `event.go` `parseRawFields`, `types/rawmessage.go` `RawMessage`, `rib.go` `handleSent`, `rib_structured.go` `handleSentStructured` | Delivers final emitted attributes in `raw.attributes` / `AttrsWire` | Extends: adds optional `raw.pre-policy-attributes`, `raw.pre-policy-rail`, `raw.pre-policy-asn4` (and structured `RawMessage` fields) when pre-policy differs from sent |
| Export gate | `internal/component/bgp/reactor/session_write.go` `writeUpdateGated`, `egressRouteFilter` | Runs the export chain on an originated/replayed body and returns `nil` on `suppress` | Extends: captures chain input before `egressRouteFilter` for the sent event, and withdraws a held route when `suppress` fires on a replay (AC-7); no replay exemption is added |
| Forward rails | `reactor_api_forward.go` `forwardUpdateCore`, `forward_rs.go` `reactorForwardRS` | Runs export chain on received wire, then applies EBGP local-AS prepend | Extends: records pre-policy attributes and forward rail + ASN4 provenance on the dispatched `fwdItem` for the sent event; replay preserves chain-then-local-AS-prepend order |
| Receive store replay | `policy_reload.go` `reconcileExportFinalKinds`, `RelayReplay`, `RelayStoredRoute` | Restores retained received routes from `bgp-adj-rib-in` on export-final change | Does not apply to refresh: holds received routes only and is optional; originated/configured routes have no receive-store entry. Unchanged |
| Stale rail | `stale_readvertise_chain.go` `staleChainFor`, `runStaleChain`, `reactor_api_batch.go` `decideStaleReadvertise`, `sendStaleReadvertiseUnit` | Opted-member or whole-chain run over a rebuilt body; whole-chain rejection already returns `staleWithdraw` | Extends: `judgeAll` reads pre-policy attributes with rail/ASN4 provenance (opted-only run keeps sent body); `sendStaleReadvertiseUnit` re-checks `exportGeneration` under `writeMu` and re-decides outside `writeMu` if generation changed |
| Policy watermark & generation | `stale_readvertise_chain.go` `noteExportPolicyChangeLocked`, `raiseExportPolicyFrom`, `Session.noteExportWrite` | Lock-free atomic `exportGeneration` + `exportPolicyFrom` watermark; `noteExportWrite` raises watermark past late old receipts | Extends: skips `noteExportWrite` for `raw == true` diagnostic writes (AC-12); raises watermark on `bgp/filter-definition-changed` events for peers naming the changed external filter (AC-8) |
| Withdraw path & ownership fence | `reactor_api_batch.go` `staleWithdraw`, `buildBatchWithdrawUpdate`, `session_ownership.go` `adjOutWrite.allow` | Sends bare withdrawal gated by `adjOutExpected` / `ownRouteWithdrawal` | Extends: ordinary refresh denial (AC-7) takes the same bare-withdrawal synthesis and ownership fence so unheld or replaced keys are never falsely withdrawn |
| Config transaction & `EmitEvent` | `docs/architecture/config/transaction-protocol.md`, `pkg/plugin/sdk/sdk_engine.go` `EmitEvent`, `internal/component/plugin/server/bus_op_gate.go` | Each filter plugin applies its own config via `ConfigTx` / `OnConfigApply`; `EmitEvent` works on DirectBridge and external RPC and requires a declared `Sends` entry | Extends: in-tree `filter-update` plugins declare `{Message: "bgp:filter-definition-changed"}` in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends` and emit `bgp:filter-definition-changed` after a real definition change; third-party filter plugins use the same Stage 1 `Sends` + `EmitEvent` contract |
| Reload settings comparison | `peer_settings_apply.go` `applyHotSwappableSettingsWithGraphDiff`, `peerSettingsSwapPlan`, `reactor_api.go` `peerSettingsEqual`, `filterapi/policy.go` `Policy.SameDecisions` | `applyHotSwappableSettingsWithGraphDiff` already uses `Policy.SameDecisions`; `peerSettingsEqual` and `peerSettingsSwapPlan` still use raw `reflect.DeepEqual` | Extends: `peerSettingsEqual` and `peerSettingsSwapPlan` compare `ImportPolicy` and `ExportPolicy` via `SameDecisions` (and `Implied()`), and `filterapi.Policy` gains `NamesExternalFilter` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Reach another unit through the boundary that survives a process split:" | agrees | Sent pre-policy fields cross both DirectBridge and `format=full` JSON, replay provenance crosses `update-route` metadata, and external filter changes cross `EmitEvent` (`bgp/filter-definition-changed`) identically for internal and forked plugins |
| `docs/architecture/core-design.md` | Policy Filter Chain | `// Design:` and anchors of `reactor/*`, `types/*`, `filterapi/policy.go`, `filter_*`; modified by this spec | "A write no export chain judged carries no stamp, which reads as generation zero: it raises the watermark once the peer's generation is above zero, and at generation zero, before any change, it matches and raises nothing. Message ids are one global counter, so this needs no new field on the plugin protocol and a forked RIB changes nothing. A change inside an external plugin's own data is not a policy change here and stays invisible to the comparison." | departs | Departs on four points: (1) D-7/AC-12 skips `noteExportWrite` on explicit `raw == true` diagnostic writes instead of letting generation zero raise the watermark once `exportGeneration > 0`; (2) D-3/AC-8 adds `bgp:filter-definition-changed` (declared in `Sends` and emitted via `EmitEvent`) so external filter plugins notify the reactor when definitions change instead of staying invisible; (3) D-1 adds optional `raw.pre-policy-attributes`, `raw.pre-policy-rail`, and `raw.pre-policy-asn4` on `format=full` sent events (plus replay provenance metadata) across the plugin protocol; (4) D-5/AC-11 refuses an old-generation stale write under `writeMu` and re-decides outside `writeMu` rather than writing old-chain bytes and relying solely on `noteExportWrite` afterwards. `docs/architecture/core-design.md` is updated in `Files to Modify` |
| `docs/architecture/api/process-protocol.md` | Cached-write errors and joined output | `// Design:` of `internal/core/bgp/events/events.go` and anchors of `reactor/*`, `rib.go`, `event.go`; modified by this spec | "Actual SENT UPDATE events carry the original producing session's `destination-session` as descriptive provenance on both the structured and full JSON transports." | agrees | Follows the exact same dual-transport additive provenance pattern for `raw.pre-policy-attributes`, `raw.pre-policy-rail`, `raw.pre-policy-asn4` and documents `bgp/filter-definition-changed` on the existing `emit-event` contract |
| `docs/architecture/memory/lifetime-contracts.md` | TL;DR | anchored to `types/rawmessage.go`, `reactor_api_forward.go`, `forward_rs.go`, `reactor_notify.go` | "To use bytes past the boundary a consumer must **retain** them (take a reference) or **own** a copy — always *before* the boundary." | agrees | `onMessageReceived` copies `PrePolicyAttrs` into owned bytes before async delivery, and `bgp-rib` interns them in `pool.RibOut` with refcounted `AddRef`/`Release` |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `internal/component/bgp/*`, `internal/core/bgp/events/events.go` | "The engine reaches plugin features only through what they register:" | agrees | Reactor names no filter plugin package; it matches `<plugin>:<filter>` refs declared in `ExportPolicy` and subscribes to the registered `bgp/filter-definition-changed` event |
| `docs/architecture/api/architecture.md` | RIB Ownership | anchored to `types/types.go`, `reactor/*`, `ribout_entry.go`, `update_text.go` | "**API programs own all RIB data and logic.** The engine is a minimal BGP speaker." | agrees | The engine holds no pre-policy route store; `bgp-rib` owns `ribOutEntry.PrePolicyAttrHandle` and replays it on refresh |
| `docs/architecture/api/commands.md` | Overview | `// Design:` of `cmd/update/update_text.go` and anchors of `reactor/*`, `rib_commands.go` | "Ze uses verb-first command paths with JSON or text encoding." | agrees | Replay commands keep their existing `update-route` verb-first syntax and carry `pre-policy-rail` / `pre-policy-asn4` in `CommandContext.Meta` alongside `sent-owner-message` |
| `docs/architecture/api/update-syntax.md` | Overview | anchored to `types/types.go`, `reactor/*`, `cmd/update/update_text.go` | "Unified syntax for API commands and config files to send/receive BGP UPDATE messages." | agrees | No change to `update text`/`hex`/`b64`/`cursor` command grammar; replay provenance travels in `CommandContext.Meta` |
| `docs/architecture/route-types.md` | Overview | anchored to `types/types.go` | | silent | Covers general route type wrappers; does not speak to export policy evaluation, pre-policy Adj-RIB-Out storage, or watermark comparison |
| `docs/architecture/plugin/rib-storage-design.md` | Received source-policy provenance | `// Design:` of `event.go`, `rib.go`, `rib_structured.go`, `rib_commands.go`, `rib_selected_refresh*.go`; modified by this spec | "The sent inventory retains the epoch beside its received-owner receipt. Attribute grouping keeps different epochs separate." | agrees | Extends `ribOutEntry` and `groupKey` in the same way to retain `PrePolicyAttrHandle`, `PrePolicyRail`, and `PrePolicyASN4` beside `AttrHandle` without altering the sent view of `reconstructRoute` |
| `docs/architecture/api/json-format.md` | Structure | `// Design:` of `format/text_update.go` and anchored to `event.go`; modified by this spec | "Full-format raw bytes are inside `bgp.raw`, alongside `bgp.update`." | agrees | Adds optional kebab-case `pre-policy-attributes`, `pre-policy-rail`, and `pre-policy-asn4` inside `bgp.raw` on `format=full` sent events only when pre-policy differs from sent attributes |
| `docs/architecture/api/text-coverage.md` | Message Type Coverage | anchored to `format/text_update.go` | | silent | Tracks text-encoding field coverage; pre-policy attributes are added only to `format=full` JSON and structured `RawMessage` |
| `docs/architecture/api/text-parser.md` | Current Parser | anchored to `format/text_update.go` | | silent | Covers text-format line parsing; does not govern `format=full` JSON `bgp.raw` fields or export policy reconciliation |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | `// Design:` of `forward_rs.go` and anchored to `reactor/*`, `format/text_update.go` | "Both forwarding rails key body reuse on effective opaque-attribute treatment, not on a plugin or peer role." | agrees | Recording pre-policy attributes and forward rail provenance on dispatched `fwdItem` preserves body-reuse keying and zero-copy caching on both forward rails |
| `docs/architecture/buffer-architecture.md` | When a copy is deliberate | `// Design:` of `reactor_api_forward.go` and anchored to `format/text_update.go`, `session_write.go`; modified by this spec | "A copy on the wire path MUST match one of the triggers below. A copy that fits none of them is a defect until this table names the new trigger." | agrees | Pre-policy attributes remain raw wire bytes (no parsed structs), unchanged routes copy nothing, and `docs/architecture/buffer-architecture.md` is updated in `Files to Modify` to add the deliberate copy-trigger row for `onMessageReceived` copying `PrePolicyAttrs` into owned bytes when pre-policy differs from sent attributes |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `session_write.go`, `rib.go` | | silent | Covers Established-state FSM event transitions; this spec does not alter FSM states or transitions |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to `session_write.go`, `reactor_notify.go`, `reactor_api.go` | | silent | Covers BGP FSM state machine transitions; no FSM transition changes |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to `session_write.go`, `reactor_notify.go`, `reactor_api_batch.go`, `reactor_api.go`, `peer_settings_apply.go` | "A peer outlives its sessions, so the `Peer` also carries the two counters that fence its forwarded routes across them." | agrees | Export generation and watermark live on `Peer` across sessions, and `peerSettingsEqual`/`peerSettingsSwapPlan` preserve hot-swap vs restart lifecycle semantics |
| `docs/architecture/congestion-industry.md` | Sources | anchored to `session_write.go` | | silent | Surveys industry congestion handling; does not speak to export policy refresh or watermark comparison |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `session_write.go` | | silent | Covers RFC 8654 65535-octet message limits; this spec preserves existing message sizing and splitting |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `session_write.go`, `reactor_api_forward.go`, `reactor_api_batch.go` | | silent | Covers per-peer overflow queuing and congestion backpressure; no queue admission or backpressure rule changes |
| `docs/architecture/route-selection.md` | Overview | anchored to `session_write.go`, `forward_rs.go`, `rib.go`, `rib_commands.go` | | silent | Covers Loc-RIB best-path selection; this spec changes Adj-RIB-Out refresh input and export policy reconciliation, not best-path selection |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | `// Design:` of `internal/le/interoplab/bgp/checkers.go` and anchored to `session_write.go` | "A scenario directory carries only declarative inputs its runner reads: `ze.conf` plus the peer configuration and argument files that topology needs. Assertions never live there. They are typed Go checkers under `internal/le/interoplab/`: BGP builds its registry from `scenarioOperations` in `internal/le/interoplab/bgp/checkers.go`" | agrees | The new `test/interop/scenarios/bgp-route-refresh-prepend-frr/` directory carries only declarative Ze and FRR config files, and its assertions live in a typed Go checker registered in `internal/le/interoplab/bgp/checkers.go` so `Discover` joins scenario and checker cleanly |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `session_write.go`, `reactor_api_forward.go`, `reactor_notify.go`, `reactor_api_batch.go`, `rib_replay.go`, `rib_commands.go` | | silent | Covers low-level UPDATE wire section construction; this spec reuses existing builders and splitters unchanged |
| `docs/architecture/wire/messages.md` | TL;DR (Read This First) | anchored to `session_write.go` | | silent | Documents RFC 4271 message header and type byte layouts; wire framing is unchanged |
| `docs/architecture/wire/mp-nlri-ordering.md` | Key Insight | anchored to `session_write.go` | | silent | Covers RFC 7606 Section 5.1 MP attribute ordering; preserved by `writeOrdinaryUpdateBody` |
| `docs/architecture/wire/nlri-bgpls.md` | MP_REACH next hop | anchored to `session_write.go`, `reactor_api.go` | | silent | Covers BGP-LS NLRI encoding; this spec is family-agnostic |
| `docs/architecture/bgp/egress-attribute-rules.md` | How provenance is established | anchored to `reactor_api_forward.go`, `forward_rs.go`, `reactor_notify.go`, `stale_readvertise_chain.go` | "Both forward rails relay an UPDATE another speaker sent, after import policy has had a chance to rewrite it." | agrees | Recording the forward rail's pre-policy input and rail provenance preserves chain-then-local-AS-prepend order and egress attribute rules on replay |
| `docs/architecture/bgp/fanout-dedup.md` | The decisions | anchored to `reactor_api_forward.go` | | silent | Covers per-policy-group forward materialization deduplication; unaffected by carrying pre-policy input on sent events |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `reactor_api_forward.go` | | silent | Covers `ContextID` zero-copy equivalence; pre-policy ASN4 provenance complements encoding context without altering `ContextID` |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `reactor_api_forward.go`, `reactor_notify.go` | | silent | Documents RFC 9234 OTC `src-role` route metadata; unchanged |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | anchored to `reactor_notify.go` | | silent | Covers `ze exabgp plugin` bridge and `base64` wire stream; additive `format=full` JSON sent fields do not alter the ExaBGP bridge |
| `docs/architecture/mrt.md` | Packages | anchored to `reactor_notify.go` | | silent | Covers MRT table/message dumping; unaffected by Adj-RIB-Out pre-policy storage |
| `docs/architecture/bgp/replay-cursor.md` | What it buys | `// Design:` of `rib_replay.go` and anchored to `reactor_api_batch.go`, `ribout_entry.go`, `rib_structured.go`, `update_text.go` | "Config-static advertisements are retained in Adj-RIB-Out with their origin flag. A ROUTE-REFRESH includes them, subject to the current outbound policy, just like other routes of the requested family (RFC 2918 Section 4)." | agrees | Extends `groupKey` with `(PrePolicyAttrHandle, PrePolicyRail, PrePolicyASN4)` and selects pre-policy attributes for refresh (including configured static routes) while preserving sent-owner receipts and cursor batching |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `reactor_api_batch.go` | | silent | Documents BGP OPEN capability wire layouts; no capability encoding changes |
| `docs/architecture/wire/rfc7606-relay-shape.md` | The requirement | anchored to `reactor_api_batch.go` | | silent | Covers RFC 7606 Section 5.1 single-NLRI-field relay splitting; preserved unchanged |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor_api.go` | | silent | Documents human-readable text event format; pre-policy fields apply to `format=full` JSON and structured events |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `reactor_api.go` | "Config transactions use the same `(namespace, event-type)` pub/sub backbone as all other cross-component coordination (interface events, RIB changes, BGP events)." | agrees | Filter plugins apply config through the existing transaction protocol (`ConfigTx`) and emit `bgp/filter-definition-changed` over the shared event bus when a definition actually changes |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `peer_settings_apply.go` | | silent | Covers multi-component config apply dependency tiers; reload apply order is unchanged |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to `filterapi/policy.go` | | silent | Covers YANG configuration block syntax; no config syntax changes |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `filterapi/policy.go`, `rib.go`, `rib_selected_refresh.go` | | silent | Historical overview of moving RIB storage out of the engine into `bgp-rib`; this spec stays within that split by storing pre-policy attributes in `bgp-rib` |
| `docs/architecture/pool-architecture.md` | Intern (Deduplicate and Store) | `// Design:` of `ribout_entry.go`; modified by this spec | "Intern stores data with deduplication." | agrees | Pre-policy attributes intern in `pool.RibOut` only when distinct from sent attributes and are released via `Pool.Release` in `ribOutEntry.release()` |
| `ai/doors/bgp-plugins.md` | Boundaries | door covering `plugins/rib/*`, `plugins/cmd/update/*`, `plugins/filter_*` | "Reach the engine through the bus, DirectBridge or a text command, never a plain call into `reactor` or another component." | agrees | `bgp-rib` receives pre-policy bytes over sent events and returns provenance over `update-route` metadata; filter plugins notify the reactor via `EmitEvent` |
| `docs/architecture/decisions/001-pull-model-metrics.md` | The Question | anchored to `rib.go` | | silent | Covers Prometheus pull-model metrics architecture; no new metrics architecture introduced |
| `docs/architecture/doctor-and-health-checks.md` | BGP policy absences in the supplied configuration | anchored to `rib.go` | | silent | Covers `ze doctor` check registration; no doctor checks added or changed |
| `docs/architecture/rib/forward-handle.md` | The decisions | anchored to `rib.go`, `rib_structured.go` | | silent | Covers lazy-copy `ribForwardHandle` for received state trackers; Adj-RIB-Out uses `pool.RibOut` handles |
| `docs/architecture/bgp/as112-coordination.md` | The layering rule | anchored to `cmd/update/update_text.go` | | silent | Covers AS112 service coordination; unrelated to BGP export-policy refresh |
| `docs/architecture/wire/nlri-evpn.md` | Common Header | anchored to `cmd/update/update_text.go` | | silent | Covers EVPN NLRI wire layout; this spec is family-agnostic |
| `docs/architecture/meta/filter-community.md` | Keys Set | anchored to `filter_community/filter_community.go` | | silent | Documents community filter route metadata keys; emitting `bgp/filter-definition-changed` adds no route metadata key to `filter_community` |
| `docs/architecture/bgp/filter-irr.md` | The decisions | `// Design:` and anchor of `filter_irr/config_tx.go` | "A commit reaches the filter through the reload pair, not through `OnConfigure`." | agrees | `filter_irr` emits `bgp:filter-definition-changed` after `ConfigTx` apply or stored-list refresh changes a definition, preserving its reload and store-reader contract |
| `docs/architecture/irr-owner.md` | Where it sits | anchored to `internal/component/bgp/plugins/filter_irr/register.go` | | silent | Covers the `irr` owner plugin, stored datasets, and `irr/dataset-stored` / `irr/interest-changed` notices; adding `{Message: "bgp:filter-definition-changed"}` to `filter_irr`'s `Sends` does not alter IRR owner storage or notices |
| `docs/architecture/ospf/ospf-ext-16-ipsec-auth.md` | Decisions | anchored to `internal/le/interoplab/bgp/checkers.go` | | silent | Covers RFC 4552 OSPFv3 IPsec AH/ESP authentication; anchored only because `checkers.go` sits in the interop harness, unrelated to BGP export-policy refresh or FRR route-refresh interop |
| `ai/doors/le-tooling.md` | Where it sits | door covering `test/plugin/*.ci`, `test/reload/*.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | New `.ci` tests live under `test/plugin/` and `test/reload/` and follow the functional runner contract |
| `docs/plugin-development/handlers.md` | EmitEvent | modified by this spec | "Pushes an event into the engine's delivery pipeline for plugin-to-plugin communication." | agrees | Documents that a filter plugin whose internal definitions change at runtime emits `bgp/filter-definition-changed` via `EmitEvent` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | No kernel/VPP data-plane configuration or commit validation rule is altered |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | No backend mismatch verdict is altered |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | No VPP firewall verifier or YANG backend annotation is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | No data-plane selector or backend registration name is touched |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Refresh and whole-chain stale runs evaluate the export filter chain (including built-in `next-hop:*` steps) over pre-policy attributes without adding any session-level next-hop knob |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Both next-hop authoring forms remain untouched and evaluate once per refresh over pre-policy attributes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Built-in `next-hop:*` steps remain compiled in `policychain.Chain`, compared by `Policy.SameDecisions` in `peerSettingsEqual` and `peerSettingsSwapPlan` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Replaying pre-policy attributes preserves the original pre-override next-hop baseline so raw-override provenance checks stay accurate on refresh |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Preserves IPv4 and IPv6/MP_REACH next-hop handling and import-chain refusals unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Carrying `PrePolicyRail` (`announce` vs `forward`) preserves the forwarded-vs-originated distinction on refresh so trailing `next-hop:auto` continues to apply only to forwarded routes |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | `peerSettingsEqual` and `peerSettingsSwapPlan` compare `Policy.Implied()` beside `SameDecisions` without altering the implied-filter seam |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | No daemon startup refusal or service-disable change is introduced; reload transaction refusal semantics remain intact |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | No kernel/VPP startup or commit gate behavior is altered |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `bgp:filter-definition-changed` uses the existing dynamic `emit-event` operation (`bus_op_gate.go`), requires `{Message: "bgp:filter-definition-changed"}` in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends` for every emitting plugin so the compiled bus profile authorizes it, and validates declared `<plugin>:<filter>` refs against each peer's `ExportPolicy` |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Pre-policy attributes travel on existing lossless sent event delivery with no drop-on-full behavior |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | No change to `| implied` config rendering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `bgp:filter-definition-changed` registers through `events.RegisterEventType`, and every emitting filter plugin (the 5 in-tree `filter-update` plugins in both `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`, plus third-party filter plugins in Stage 1 `sdk.Registration.Sends`) declares `{Message: "bgp:filter-definition-changed"}` so `bus_op_gate.go` authorizes `emit-event` by default |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | No change to `send > to` bus routing semantics |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | No change to `receive > from` bus filtering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | No change to bus allow-list enforcement |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | No change to bus filter compilation or disagreement refusal |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | No exclusive-role arbitration added on the bus |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | No change to bus defaults display |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | No CLI command path or pipe grammar is added or changed |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | No command pipe overrides are added or changed |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | No pipe operations are altered |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | New `bgp-route-refresh-prepend-frr` scenario declares no unrelated kernel capabilities |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Preserves `announceFacts.flowSpecRedirectNextHop` and Simpson opt-in gating unchanged |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | No change to ExaBGP wrapper endpoint architecture |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | Additive `raw.pre-policy-*` fields apply only to `format=full` JSON and structured `RawMessage`, leaving `base64` UPDATE wire grammar untouched |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer sends ROUTE-REFRESH (or a stale replay / manual resend runs): `handleRefresh` / `handleRefreshStructured` -> `queueSelectedRefresh` -> `replayLegacyRefresh` or `replayConfiguredSent` -> `update-route` with `metaKeyReplay`, `SentOwnerMessageMeta`, `PrePolicyRailMeta`, `PrePolicyASN4Meta`.
- A config reload reaching `peerSettingsEqual`, `peerSettingsSwapPlan`, and `applyHotSwappableSettingsWithGraphDiff`.
- A filter plugin's `OnConfigApply` (or external data refresh) emitting `bgp/filter-definition-changed` via `EmitEvent`.
- An explicit raw diagnostic write (`Session.SendRawMessage` -> `writeUpdateBody(body, true)`).

### Transformation Path
1. First write (announce or forward rail): the writer holds the chain's input attributes, rail (`announce` or `forward`), and ASN4 width before export policy runs. When the emitted post-policy attributes differ byte-for-byte from that input, `writeUpdateBody` / `onMessageReceived` carries `PrePolicyAttrs`, `PrePolicyRail`, and `PrePolicyASN4` on the sent event (`RawMessage` for structured consumers, `raw.pre-policy-attributes` / `raw.pre-policy-rail` / `raw.pre-policy-asn4` for `format=full` JSON).
2. `RIBManager.handleSent` and `handleSentStructured` intern both sent and (when present) pre-policy attributes in `pool.RibOut`, storing `PrePolicyAttrHandle`, `PrePolicyRail`, and `PrePolicyASN4` on `ribOutEntry`. Unchanged routes store no second handle. Every overwrite, withdrawal, or peer purge calls `ribOutEntry.release()`, releasing both handles.
3. Refresh (`replayLegacyRefresh`, `replayConfiguredSent`) and grouped resend select pre-policy attributes explicitly when `PrePolicyAttrHandle.IsValid()` (falling back to `AttrHandle` when identical) while leaving `reconstructRoute` as the sent view for show, lifecycle, abort, and opted-only stale callers. `groupKey` partitions by `(PrePolicyAttrHandle, PrePolicyRail, PrePolicyASN4)`.
4. Replay commands pass `pre-policy-rail` and `pre-policy-asn4` in `CommandContext.Meta`; `cmd/update/update_text.go` populates `NLRIBatch.PrePolicyRail` and `NLRIBatch.PrePolicyASN4`.
5. `announceTarget.sendBatchUpdate` runs the ordinary export gate once over the pre-policy attributes; when `batch.PrePolicyRail == PrePolicyRailForward`, it preserves forward-rail order (export chain first, then RFC 4271 Section 9.1.2 local-AS prepend). If the ordinary gate suppresses a replay of a held route, `writeUpdateGated` synthesizes and writes a bare withdrawal under the existing `adjOut` ownership fence (AC-7).
6. On the stale readvertise rail, `judgeAll` runs the whole chain over pre-policy attributes (opted-only runs keep the sent body). Before writing to TCP under `writeMu`, `sendBatchUpdatePreFiltered` checks `owner.exportGeneration == peer.exportGeneration.Load()`; on mismatch it returns `errExportGenerationChanged` without writing, and `sendStaleReadvertiseUnit` re-decides outside `writeMu` under the new chain up to a fixed retry cap (fail-closed on exhaustion).
7. On config reload, `peerSettingsEqual` and `peerSettingsSwapPlan` compare `ImportPolicy` and `ExportPolicy` via `Policy.SameDecisions` (and `Implied()`), so a no-op rebuild never flags a change or moves the watermark. Each emitting filter plugin declares `{Message: "bgp:filter-definition-changed"}` in both compiled `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends` (satisfying `bus_op_gate.go`), and when it changes a definition and emits `bgp:filter-definition-changed` with `{"refs":["<plugin>:<filter>", ...]}`, the reactor raises `peer.noteExportPolicyChangeLocked()` for each peer whose `ExportPolicy.NamesExternalFilter(plugin, filter)` holds.
8. Explicit raw writes (`raw == true`) skip `Session.noteExportWrite` in `writeUpdateBody`, leaving `exportPolicyFrom` untouched.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to RIB (sent event) | Structured `bgptypes.RawMessage` fields and `format=full` JSON `raw.pre-policy-attributes`, `raw.pre-policy-rail`, `raw.pre-policy-asn4` | No: proven by `TestSentEventCarriesPrePolicyAttributes` |
| RIB to engine (`update-route`) | Existing `CommandContext.Meta` carrying additive `pre-policy-rail` and `pre-policy-asn4` beside `replay` and `sent-owner-message` | No: proven by `TestRefreshReplaysPrePolicyAttributes` and `TestRefreshForwardedRouteKeepsRailOrder` |
| Filter plugin to engine | `EmitEvent` (`bgp:filter-definition-changed`) declared in `Sends` (`registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`) with JSON payload `{"refs":["<plugin>:<filter>", ...]}` over DirectBridge and external RPC | No: proven by `TestFilterDefinitionEventRaisesWatermark` |
| Reactor to filter member | Existing `filter-update` RPC / DirectBridge slot, unchanged | Yes: `runLibraryChain` |

### Changed-Wire Census

**Search evidence:** scope `internal`, `pkg`, `test`. Terms: `raw-attributes`, `appendRawSectionsJSON`, `parseRawFields`, `GetRawAttributesHex`, `RawAttributes`, `handleSent`, `handleSentStructured`, `SentOwnerMessageMeta`, `EmitEvent`, `EventUpdate`, `busMsgDynamicEmit`. Controls: `format/text_update.go::appendRawSectionsJSON` (JSON writer), `event.go::parseRawFields` (JSON reader), `server/events.go::onMessageSent` (dispatch), `plugins/rib/rib.go::handleSent` and `rib_structured.go::handleSentStructured` (RIB consumers), `plugins/cmd/update/update_text.go` (`SentOwnerMessageMeta` parser), `internal/component/plugin/server/bus_op_gate.go` (`emit-event` `Sends` gate).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Sent event `raw.pre-policy-attributes`, `raw.pre-policy-rail`, `raw.pre-policy-asn4` (JSON `format=full` and `RawMessage`) | writer | `internal/component/bgp/reactor/session_write.go`: `writeUpdateGated`, `writeUpdateBody`; `reactor_api_forward.go`: `forwardUpdateCore`; `forward_rs.go`: `reactorForwardRS`; `reactor_notify.go`: `onMessageReceived`; `format/text_update.go`: `appendRawSectionsJSON`; `server/events.go`: `onMessageSent`, `getStructuredEvent` | Existing sent paths publish only final emitted attributes (`AttrsWire` / `raw.attributes`) | New additive fields: emitted only when pre-policy differs from sent attributes; older readers ignore unknown JSON keys |
| Sent event `raw.pre-policy-attributes`, `raw.pre-policy-rail`, `raw.pre-policy-asn4` (JSON `format=full` and `RawMessage`) | reader | `internal/component/bgp/event.go`: `parseRawFields`; `internal/component/bgp/plugins/rib/rib.go`: `handleSent`; `rib_structured.go`: `handleSentStructured`, `storeSentEntries`; third-party `format=full` plugins | Existing readers decode only `raw.attributes` / `AttrsWire` | New: `event.go` parses into `Event`, RIB interns second handle when present; third-party readers that do not read them remain unaffected |
| Sent event `raw.attributes` | writer | `internal/component/bgp/format/text_update.go`: `appendRawSectionsJSON` (`"attributes"`) | Writes packed sent attributes with MP_REACH/MP_UNREACH excluded | Preserve exact sent-attribute bytes and meaning |
| Sent event `raw.attributes` | reader | `internal/component/bgp/event.go`: `parseRawFields`; `plugins/rib/rib.go`: `handleSent`; `plugins/adj_rib_in/`; third-party plugins | Reads sent attributes for `ribOut` show/withdraw/suppression | Preserve unchanged |
| Replay `update-route` metadata `pre-policy-rail`, `pre-policy-asn4` | writer | `internal/component/bgp/plugins/rib/rib_commands.go`: `sendRoutes`; `rib_replay.go`: `resendRoutesWithCursor`, `replayRoutesWithCursorContext`; `rib_selected_refresh_configured.go`: `sendConfiguredRefreshRoute` | Existing replay callers pass `metaKeyReplay` and `SentOwnerMessageMeta` in `Meta` | New additive metadata keys; omitted when absent/default (`announce`) |
| Replay `update-route` metadata `pre-policy-rail`, `pre-policy-asn4` | reader | `internal/component/bgp/plugins/cmd/update/update_text.go`: replay metadata decoder into `bgptypes.NLRIBatch` | Existing parser reads `SentOwnerMessageMeta` and `stale` from `CommandContext.Meta` | New: populates `NLRIBatch.PrePolicyRail` and `NLRIBatch.PrePolicyASN4`; missing keys default safely to announce rail |
| Filter definition-changed event (`bgp:filter-definition-changed`) | writer | `internal/component/bgp/plugins/filter_community/{register.go,filter_community.go}`, `filter_family/{register.go,filter_family.go,config_tx.go}`, `filter_irr/{register.go,filter_irr.go,config_tx.go}`, `filter_modify/{register.go,filter_modify.go,config_tx.go}`, `filter_remove_private_as/{register.go,filter_remove_private_as.go,config_tx.go}`; any third-party external filter plugin via Stage 1 `Sends` + `sdk.Plugin.EmitEvent` | Existing filter plugins install new definitions in `ConfigTx` without declaring `Sends` or notifying the reactor | New: declared in `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends`, emitted after a real definition change with `{"refs":["<plugin>:<filter>", ...]}` |
| Filter definition-changed event (`bgp:filter-definition-changed`) | reader | `internal/component/bgp/reactor/reactor_filter_events.go`: reactor subscriber | New subscriber on `bgpevents.Namespace` / `bgpevents.EventFilterDefinitionChanged` | New: validates `<plugin>:<filter>` against each peer's `ExportPolicy.NamesExternalFilter` and calls `noteExportPolicyChangeLocked` |
| BGP UPDATE to a peer | writer | `internal/component/bgp/reactor/session_write.go`: `writeUpdateGated`, `writeUpdateBody`; `reactor_api_batch.go`: `sendBatchUpdate`, `sendStaleReadvertiseUnit` | Existing refresh path writes double-prepended AS_PATH and suppresses denied held routes | Change: writes single-pass policy output from pre-policy attributes and withdraws held routes denied on refresh |
| BGP UPDATE to a peer | reader | External BGP peer UPDATE receiver, including FRR in `bgp-route-refresh-prepend-frr` | Receives BGP announcements and withdrawals over TCP | Preserve RFC 4271/2918/7313 framing; peer observes single-pass attributes and withdrawals of denied routes |

### Integration Points
- `session_write.go::writeUpdateGated`, `reactor_api_forward.go::forwardUpdateCore`, `forward_rs.go::reactorForwardRS` - record pre-policy attributes, rail, and ASN4 width for the sent event.
- `rib_selected_refresh.go::replayLegacyRefresh`, `rib_selected_refresh_configured.go::replayConfiguredSent`, `rib_replay.go::collectGroupedRibOutRoutesFiltered` - select pre-policy input explicitly; `reconstructRoute` and sent-view callers keep the advertised form.
- `reactor_api_batch.go::sendBatchUpdate`, `sendStaleReadvertiseUnit`, `sendBatchUpdatePreFiltered` - preserve forward-rail order on replay and re-check `exportGeneration` under `writeMu` before stale-rail writes.
- `reactor_api.go::peerSettingsEqual`, `peer_settings_apply.go::peerSettingsSwapPlan` - compare policies semantically via `Policy.SameDecisions` (and `Implied()`).
- `reactor_filter_events.go` + `internal/core/bgp/events/events.go` - register and subscribe to `bgp/filter-definition-changed`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Refresh continues through the ordinary export gate with no replay exemption |
| No unintended coupling (components stay isolated) | Yes | RIB stores opaque wire attribute bytes and rail enum; reactor never names a plugin package |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `pool.RibOut`, `appendRawSectionsJSON`, `CommandContext.Meta`, `EmitEvent`, `Policy.SameDecisions`, and `noteExportPolicyChangeLocked` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Routes whose pre-policy and sent attributes match store no second handle; distinct pre-policy bytes intern and deduplicate in `pool.RibOut` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `bgp/filter-definition-changed` registers through `events.RegisterEventType`, and any internal or external filter plugin opts in by emitting it over `EmitEvent` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched `internal/component/bgp/reactor/`, `policychain/`, `filterapi/` for hardcoded filter names (such as `prepend` or `filter-modify`): zero added; reactor matches only `<plugin>:<filter>` refs declared in each peer's `ExportPolicy` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The input to the export chain is available where the sent event is constructed | `writeUpdateGated` holds `body` before `egressRouteFilter`; `forwardUpdateCore` and `reactorForwardRS` hold the received wire before `runEgressPolicyChain` | Carry pre-policy bytes on `fwdItem` / `Session` across `writeUpdateBody` | `TestSentEventCarriesPrePolicyAttributes` | unvalidated |
| A-2 | A stateless export chain run once over pre-policy bytes reproduces the first send when policy is unchanged | `policychain` compiles immutable chains | A stateful external filter may legitimately answer differently per RFC 2918 | `test/plugin/route-refresh-export-prepend.ci` | unvalidated |
| A-3 | Extra `pool.RibOut` memory is bounded: only routes whose pre-policy and sent attributes differ intern a second handle, and `pool.RibOut` deduplicates identical attribute blocks across prefixes and peers | `ribout_entry.go` (`pool.RibOut` shared across peers) | High churn with distinct per-route policy edits increases `rib-out` pool slots | `./le perf` comparison on one toolchain plus pool-leak assertion in `TestRefreshReplaysPrePolicyAttributes` | unvalidated |
| A-4 | A live `SentOwnerMessage` receipt from the current session proves the peer holds the route | `adjOutExpected` in `session_ownership.go` | Without the fence a withdrawal would fire for an unheld or replaced route; `adjOutWrite.allow` drops unheld or foreign-owned keys | `TestOrdinaryRefreshDenialWithdrawsHeldRoute`, `TestStaleDenialOnNewSessionSendsNothing` | unvalidated |
| A-5 | `Policy.SameDecisions` already compares compiled in-process `match` conditions and their resolved policy instances | `policychain/chain.go:333` (`Chain.SameDecisions`) and `filterapi/policy.go:191` | If any in-process evaluator hides mutable state outside `config`/`resolved`, `SameDecisions` misses it | `TestNoOpReloadKeepsExportWatermark`, `TestDefinitionChangeRaisesWatermark` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A forward-rail route replayed through the announce rail applies local-AS prepend in the wrong order or loses ASN4 width | `TestRefreshForwardedRouteKeepsRailOrder` red | Record `PrePolicyRail` and `PrePolicyASN4` alongside `PrePolicyAttrHandle` and preserve chain-then-local-AS-prepend order on replay |
| R-2 | A third-party external filter plugin does not emit `bgp/filter-definition-changed` | Its stale routes keep prior judgments until next config reload or refresh | Documented contract in `docs/plugin-development/handlers.md` and `docs/architecture/api/process-protocol.md`; failure mode is stale judgment on that plugin's own edit, never a policy bypass |
| R-3 | Sustained reload storm causes unbounded re-decision loop in `sendStaleReadvertiseUnit` | Repeated `errExportGenerationChanged` retries on one unit | Cap retries at 8 attempts; on exhaustion log a warning and withhold the unit fail-closed |
| R-4 | Second handle (`PrePolicyAttrHandle`) leaks in `pool.RibOut` on route replacement, withdrawal, or peer purge | `pool.RibOut` live-slots / refcount check fails after churn | Every overwrite, removal, and purge path calls `ribOutEntry.release()`, which releases both `AttrHandle` and `PrePolicyAttrHandle` |
| R-5 | Ordinary refresh withdrawal (AC-7) accidentally removes a replacement route installed concurrently | Ownership unit test withdraws wrong route | Synthesized withdrawal carries the same `s.sentForward` (`adjOutExpected` / `ownRouteWithdrawal`) fence so `adjOutWrite.allow` spares any newer or foreign owner |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A peer sees wrong attributes on refresh, or a withdrawal of a route it should keep, or `pool.RibOut` leaks slots on route churn. Live non-refresh forwarding paths keep their existing wire output |
| How is it reverted? | Single commit revert per phase; no persistent state or YANG schema change |
| Who else touches this path? | `plan/release/fixes/spec-bgp-export-policy-reload-readvertise.md` (skeleton) re-runs the chain over advertised routes and needs these pre-policy attributes, so it depends on this spec. `plan/release/fixes/spec-rfc2545-readvertise-on-condition-flip.md` and `plan/release/fixes/spec-bgp-withdraw-only-exported-routes.md` touch `ribOut` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A peer sends ROUTE-REFRESH to a peer with a prepend export member | → | `handleRefresh`, `replayLegacyRefresh` / `replayConfiguredSent`, `sendBatchUpdate`, `writeUpdateGated` | `test/plugin/route-refresh-export-prepend.ci` |
| A peer sends ROUTE-REFRESH after policy denies a previously advertised route | → | `handleRefresh`, `writeUpdateGated` withdrawal synthesis under ownership fence | `test/plugin/route-refresh-export-denied.ci` |
| A reload that changes nothing on a peer with an in-process policy chain | → | `peerSettingsEqual`, `peerSettingsSwapPlan`, `applyHotSwappableSettingsWithGraphDiff` | `TestNoOpReloadKeepsExportWatermark` |
| An external filter plugin emits `bgp/filter-definition-changed` | → | `reactor_filter_events.go`, `Policy.NamesExternalFilter`, `noteExportPolicyChangeLocked` | `TestFilterDefinitionEventRaisesWatermark` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A route sent through an export prepend member, followed by any number of ROUTE-REFRESH messages with policy unchanged | Each refresh emits an UPDATE carrying the exact AS_PATH of the first send (single prepend, never doubled) | A stateful external member may answer differently: RFC 2918 Section 4 makes the current outbound policy authoritative |
| AC-2 | A ROUTE-REFRESH after the export policy changed (e.g., prepend count or community edit changed) | The re-advertised route carries the attributes the new chain produces from the stored pre-policy attributes | None |
| AC-3 | A route whose export chain changed no attributes, followed by ROUTE-REFRESH | The refresh emits the sent UPDATE attributes and `ribOutEntry` holds only one valid handle in `pool.RibOut` (`PrePolicyAttrHandle` invalid/zero) | None |
| AC-4 | A route written by a forward rail (`forwardUpdateCore` or `reactorForwardRS`), followed by ROUTE-REFRESH | Replay evaluates the export chain over the received pre-policy attributes and then applies RFC 4271 Section 9.1.2 local-AS prepend in the same order and ASN4 width as the first send | None |
| AC-5 | A whole-chain stale readvertise pass (`judgeAll`) after an export policy change | `runStaleChain` evaluates the whole chain once over the pre-policy attributes (with rail/ASN4 provenance) and opted-in members last; opted-only runs (`!judgeAll`) continue reading the sent body | None |
| AC-6 | A whole-chain stale readvertise pass denies a route with a live receipt | The peer receives a withdrawal for that NLRI (building on HEAD's `decideStaleReadvertise` `staleWithdraw` branch, now evaluated over pre-policy input) | On a new session with no receipt, nothing is sent because the peer holds nothing; an opted-member `llgr-egress` suppress verdict keeps its meaning |
| AC-7 | An ordinary ROUTE-REFRESH (without RFC 7313) denies a route the peer currently holds | `writeUpdateGated` synthesizes and transmits a bare withdrawal for that NLRI under the existing `adjOut` ownership fence | With RFC 7313 negotiated, EoRR also removes un-refreshed routes and the explicit withdrawal remains RFC-compliant; a concurrently replaced route from another owner is spared by the ownership fence |
| AC-8 | An external filter plugin (in-tree `filter-update` plugin or third-party process) changes a filter definition and emits `bgp/filter-definition-changed` | Each peer whose export policy names that `<plugin>:<filter>` raises its export generation and watermark so stale routes re-judge whole-chain on their next pass; peers not naming it and unknown references leave watermarks unchanged | None |
| AC-9 | A config reload rebuilds an unchanged policy chain containing an in-process evaluator func (including after live traffic increments `refused`), followed by route output | `peerSettingsEqual` reports equal, `peerSettingsSwapPlan` reports no restart reason for policies, the export watermark stays unchanged, and subsequent routes continue to be advertised under the retained chain | Existing advertisements remain valid because policy did not change |
| AC-10 | A config reload adds, removes, reorders, or re-parameterizes a policy member, or changes the final | `peerSettingsEqual` detects the change and `applyHotSwappableSettingsWithGraphDiff` raises the export generation and watermark | None |
| AC-11 | A stale readvertise unit decided under generation G reaches `writeMu` after a reload advanced `exportGeneration` to G+1 | The pre-write generation check under `writeMu` refuses the old-chain write without emitting bytes, and `sendStaleReadvertiseUnit` re-decides outside `writeMu` under G+1 so TCP receives only what the new chain produces (or a withdrawal if G+1 denies it) | On a sustained reload storm exceeding the 8-attempt retry cap, the unit is withheld fail-closed and logged rather than spinning indefinitely |
| AC-12 | Explicit raw diagnostic injection (`Session.SendRawMessage` -> `writeUpdateBody(..., raw=true)`) writes a reachable UPDATE on a peer whose `exportGeneration > 0` | `writeUpdateBody` transmits the raw UPDATE while skipping `Session.noteExportWrite`, leaving `exportPolicyFrom` unchanged so existing stale routes do not spuriously re-judge | None |

*(Original AC-12 and AC-13 for D-6 `joinExportWriters` / `MaxUint64` are retired because HEAD already replaced that mechanism with lock-free atomic `noteExportPolicyChangeLocked`.)*

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A peer operator runs a soft inbound reset against Ze with a prepend export policy | ROUTE-REFRESH -> `handleRefresh` -> pre-policy input -> export gate once -> UPDATE | `test/plugin/route-refresh-export-prepend.ci`, interop `bgp-route-refresh-prepend-frr` |
| 2 | An operator reloads an unchanged config | reload -> `peerSettingsEqual` (`Policy.SameDecisions`) -> no swap, no watermark move | `test/reload/export-policy-noop.ci` |
| 3 | An operator tightens an export filter so a route is denied, then the peer sends ROUTE-REFRESH | reload -> refresh -> `writeUpdateGated` withdrawal -> peer removes route | `test/plugin/route-refresh-export-denied.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSentEventCarriesPrePolicyAttributes` | `internal/component/bgp/reactor/session_write_prepolicy_test.go` | A-1, AC-3: emitted on both structured and `format=full` JSON only when pre-policy differs from sent, with rail and ASN4 width | |
| `TestRefreshForwardedRouteKeepsRailOrder` | `internal/component/bgp/reactor/session_write_prepolicy_test.go` | AC-4: forward-rail replay runs export chain before local-AS prepend with preserved ASN4 width | |
| `TestOrdinaryRefreshDenialWithdrawsHeldRoute` | `internal/component/bgp/reactor/session_write_prepolicy_test.go` | AC-7: ordinary refresh denial withdraws held route and spares unheld or foreign-owned keys | |
| `TestRawWriteDoesNotRaiseExportWatermark` | `internal/component/bgp/reactor/session_write_prepolicy_test.go` | AC-12: `raw=true` reachable UPDATE leaves `exportPolicyFrom` unchanged when `exportGeneration > 0` | |
| `TestRefreshReplaysPrePolicyAttributes` | `internal/component/bgp/plugins/rib/rib_refresh_prepolicy_test.go` | AC-1, AC-2, AC-3, R-4: `ribOutEntry` stores/releases second handle, keeps `reconstructRoute` as sent view, replays pre-policy on legacy and configured refresh | |
| `TestStaleRailReadsPrePolicyOnWholeRun` | `internal/component/bgp/reactor/stale_export_change_test.go` | AC-5: `judgeAll` reads pre-policy attributes; opted-only run keeps sent body | |
| `TestStaleDenialWithdrawsHeldRoute` | `internal/component/bgp/reactor/stale_export_change_test.go` | AC-6: whole-chain denial over pre-policy input withdraws held route | |
| `TestStaleDenialOnNewSessionSendsNothing` | `internal/component/bgp/reactor/stale_export_change_test.go` | AC-6 survivor: new session with no receipt sends nothing | |
| `TestStaleWriteRecheckedUnderWriteMu` | `internal/component/bgp/reactor/stale_export_change_test.go` | AC-11: generation advance before `writeMu` suppresses old-chain write and re-decides under new chain (plus retry-cap fail-closed test) | |
| `TestNoOpReloadKeepsExportWatermark` | `internal/component/bgp/reactor/policy_reload_test.go` | AC-9: `peerSettingsEqual`, `peerSettingsSwapPlan`, and `applyHotSwappableSettingsWithGraphDiff` treat rebuilt unchanged chain (even with nonzero `refused`) as equal | |
| `TestDefinitionChangeRaisesWatermark` | `internal/component/bgp/reactor/policy_reload_test.go` | AC-10: add, remove, reorder, re-parameterize, or final change moves generation and watermark | |
| `TestFilterDefinitionEventRaisesWatermark` | `internal/component/bgp/reactor/policy_reload_test.go` | AC-8: `bgp/filter-definition-changed` raises watermark only for peers whose export chain names `<plugin>:<filter>`, ignoring unknown refs | |
| `TestPolicyNamesExternalFilter` | `internal/component/bgp/filterapi/policy_test.go` | AC-8: `NamesExternalFilter` matches active external refs and ignores inactive/unrelated refs | |

`TestOrdinaryRefreshDoesNotDoublePrepend` (`reactor/refresh_prepend_double_test.go`)
feeds post-policy bytes directly into `sendBatchUpdate` with no RIB involved, so a
RIB-level pre-policy fix cannot turn it green and the only change that could is the
replay exemption RFC 2918 Section 4 rules out. Its obligation moves to
`TestRefreshReplaysPrePolicyAttributes`, `TestRefreshForwardedRouteKeepsRailOrder`,
and `test/plugin/route-refresh-export-prepend.ci`. Replacing it (and narrowing
`TestRFC4271LocalPrefIncludedForInternalPeers`'s `RFC4271-9.1.1-2` comment in
`reactor/rfc4271_test.go`) goes through `ze-test-judge`.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Sent receipt against watermark | uint64 message ids | `message == exportPolicyFrom` is current | `message == exportPolicyFrom-1` (and `0`) is stale (`judgeAll`) | N/A |
| Stale pre-write re-decision retry count | 1..8 | attempt 8 succeeds if generation matches | 0 (never skips initial decision) | attempt 9 withholds fail-closed and logs |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `route-refresh-export-prepend` | `test/plugin/route-refresh-export-prepend.ci` | Two consecutive ROUTE-REFRESH requests each return the exact single-prepend AS_PATH (wire hex asserted) | |
| `route-refresh-export-denied` | `test/plugin/route-refresh-export-denied.ci` | Export filter tightened to deny a held route; ROUTE-REFRESH without RFC 7313 emits a withdrawal | |
| `export-policy-noop` | `test/reload/export-policy-noop.ci` | No-op reload on a peer with an in-process filter leaves session, watermark, and wire advertisements unchanged | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-route-refresh-prepend-frr` | `test/interop/scenarios/` | FRR | FRR's `clear bgp ... soft in` ends with the single-prepend AS_PATH in FRR's BGP table, verified against FRR `bgpd` soft-inbound handling | |

## Files to Modify
- `internal/component/bgp/types/rawmessage.go` - `PrePolicyAttrs`, `PrePolicyRail`, `PrePolicyASN4` on `RawMessage`
- `internal/component/bgp/types/types.go` - `PrePolicyRail` enum, `PrePolicyRailMeta`, `PrePolicyASN4Meta`, and `NLRIBatch` provenance fields
- `internal/component/bgp/event.go` - `parseRawFields` and `Event` fields/accessors for `raw.pre-policy-attributes`, `raw.pre-policy-rail`, `raw.pre-policy-asn4`
- `internal/component/bgp/format/text_update.go` - `appendRawSectionsJSON` emits `pre-policy-attributes`, `pre-policy-rail`, `pre-policy-asn4` when pre-policy differs from sent
- `internal/component/bgp/reactor/session_write.go` - record chain input before `egressRouteFilter`; AC-7 refresh denial withdrawal in `writeUpdateGated`; skip `noteExportWrite` when `raw == true` (AC-12)
- `internal/component/bgp/reactor/reactor_api_forward.go` - record pre-policy attributes and forward rail + ASN4 provenance for sent events
- `internal/component/bgp/reactor/forward_rs.go` - record pre-policy attributes and forward rail + ASN4 provenance for route-server fast-path sent events
- `internal/component/bgp/reactor/reactor_notify.go` - copy pre-policy attributes and rail/ASN4 provenance onto sent `RawMessage`
- `internal/component/bgp/reactor/reactor_api_batch.go` - preserve forward-rail order on replay; AC-11 pre-write `exportGeneration` check under `writeMu` with bounded re-decision loop in `sendStaleReadvertiseUnit`
- `internal/component/bgp/reactor/stale_readvertise_chain.go` - `runStaleChain` uses pre-policy attributes and rail/ASN4 provenance on `judgeAll`
- `internal/component/bgp/reactor/reactor_api.go` - `peerSettingsEqual` compares `ImportPolicy` and `ExportPolicy` via `Policy.SameDecisions` (and `Implied()`), correcting its false no-funcs comment
- `internal/component/bgp/reactor/peer_settings_apply.go` - `peerSettingsSwapPlan` field walk compares `ImportPolicy` and `ExportPolicy` via `Policy.SameDecisions` (and `Implied()`)
- `internal/component/bgp/filterapi/policy.go` - `Policy.NamesExternalFilter(plugin, filter string) bool`
- `internal/core/bgp/events/events.go` - `EventFilterDefinitionChanged = "filter-definition-changed"`
- `internal/component/bgp/plugins/rib/ribout_entry.go` - `PrePolicyAttrHandle`, `PrePolicyRail`, `PrePolicyASN4`, release of both handles, explicit pre-policy refresh selector while preserving `reconstructRoute` as sent view
- `internal/component/bgp/plugins/rib/rib.go` - `handleSent` interns pre-policy attributes and stores rail/ASN4 provenance
- `internal/component/bgp/plugins/rib/rib_structured.go` - `handleSentStructured` and `storeSentEntries` intern pre-policy attributes and store rail/ASN4 provenance
- `internal/component/bgp/plugins/rib/rib_replay.go` - `groupKey` includes pre-policy handle/rail/ASN4; resend/refresh passes provenance metadata
- `internal/component/bgp/plugins/rib/rib_commands.go` - `sendRoutes` passes `PrePolicyRailMeta` and `PrePolicyASN4Meta`
- `internal/component/bgp/plugins/rib/rib_selected_refresh.go` - `replayLegacyRefresh` selects pre-policy refresh input
- `internal/component/bgp/plugins/rib/rib_selected_refresh_configured.go` - `replayConfiguredSent` selects pre-policy refresh input and passes provenance metadata
- `internal/component/bgp/plugins/cmd/update/update_text.go` - parse `PrePolicyRailMeta` and `PrePolicyASN4Meta` onto `NLRIBatch`
- `internal/component/bgp/plugins/filter_community/register.go` - declare `{Message: "bgp:filter-definition-changed"}` in `registry.Registration.Sends`
- `internal/component/bgp/plugins/filter_community/filter_community.go` - declare `{Message: "bgp:filter-definition-changed"}` in Stage 1 `sdk.Registration.Sends` and emit `bgp:filter-definition-changed` when `ConfigTx` changes definitions
- `internal/component/bgp/plugins/filter_family/register.go` - declare `{Message: "bgp:filter-definition-changed"}` in `registry.Registration.Sends`
- `internal/component/bgp/plugins/filter_family/filter_family.go` - declare `{Message: "bgp:filter-definition-changed"}` in Stage 1 `sdk.Registration.Sends`
- `internal/component/bgp/plugins/filter_family/config_tx.go` - emit `bgp:filter-definition-changed` when `ConfigTx` changes definitions
- `internal/component/bgp/plugins/filter_irr/register.go` - add `{Message: "bgp:filter-definition-changed"}` to `filterIRRBusSends()` (`registry.Registration.Sends`)
- `internal/component/bgp/plugins/filter_irr/filter_irr.go` - include `{Message: "bgp:filter-definition-changed"}` in Stage 1 `sdk.Registration.Sends`
- `internal/component/bgp/plugins/filter_irr/config_tx.go` - emit `bgp:filter-definition-changed` when `ConfigTx` or stored list refresh changes definitions
- `internal/component/bgp/plugins/filter_modify/register.go` - declare `{Message: "bgp:filter-definition-changed"}` in `registry.Registration.Sends`
- `internal/component/bgp/plugins/filter_modify/filter_modify.go` - declare `{Message: "bgp:filter-definition-changed"}` in Stage 1 `sdk.Registration.Sends`
- `internal/component/bgp/plugins/filter_modify/config_tx.go` - emit `bgp:filter-definition-changed` when `ConfigTx` changes definitions
- `internal/component/bgp/plugins/filter_remove_private_as/register.go` - declare `{Message: "bgp:filter-definition-changed"}` in `registry.Registration.Sends`
- `internal/component/bgp/plugins/filter_remove_private_as/filter_remove_private_as.go` - declare `{Message: "bgp:filter-definition-changed"}` in Stage 1 `sdk.Registration.Sends`
- `internal/component/bgp/plugins/filter_remove_private_as/config_tx.go` - emit `bgp:filter-definition-changed` when `ConfigTx` changes definitions
- `internal/component/bgp/reactor/refresh_prepend_double_test.go` - replaced under `ze-test-judge`
- `internal/component/bgp/reactor/rfc4271_test.go` - narrow `TestRFC4271LocalPrefIncludedForInternalPeers` `RFC4271-9.1.1-2` claim under `ze-test-judge`
- `internal/le/interoplab/bgp/checkers.go` - register typed Go checker for `bgp-route-refresh-prepend-frr` in `scenarioOperations` / `specialCheckers`
- `docs/architecture/core-design.md` - update stale-rail, pre-policy refresh, external-filter watermark, and raw-write watermark sections
- `docs/architecture/buffer-architecture.md` - add deliberate copy-trigger row in "When a copy is deliberate" for `onMessageReceived` copying `PrePolicyAttrs` into owned bytes when pre-policy differs from sent
- `docs/architecture/api/process-protocol.md` - document `raw.pre-policy-*` sent event fields and `bgp:filter-definition-changed` (including Stage 1 `Sends` declaration requirement)
- `docs/architecture/api/json-format.md` - document `raw.pre-policy-attributes`, `raw.pre-policy-rail`, `raw.pre-policy-asn4` in `format=full`
- `docs/plugin-development/handlers.md` - document filter plugin obligation to declare `{Message: "bgp:filter-definition-changed"}` in Stage 1 `sdk.Registration.Sends` and emit it via `EmitEvent` after changing definitions
- `docs/architecture/plugin/rib-storage-design.md` - document `ribOutEntry` pre-policy handle and explicit refresh selection
- `docs/architecture/pool-architecture.md` - note `pool.RibOut` second handle per policy-modified route
- `docs/guide/config-reload.md` - note reload watermark truth (definition change vs no-op rebuild, external filter notification)
- `rfc/short/rfc2918.md` - note pre-policy refresh evaluation and denial withdrawal

## Files to Create
- `internal/component/bgp/reactor/reactor_filter_events.go` - subscribe to `bgp:filter-definition-changed` and raise matching peers' export generation/watermark
- `internal/component/bgp/plugins/rib/rib_refresh_prepolicy_test.go` - RIB pre-policy storage, refcounting, sent-view preservation, and refresh replay tests
- `internal/component/bgp/reactor/session_write_prepolicy_test.go` - sent-event pre-policy emission, forward-rail replay order, AC-7 refresh denial withdrawal, and AC-12 raw-write watermark tests
- `test/plugin/route-refresh-export-prepend.ci` - end-to-end single-prepend refresh test
- `test/plugin/route-refresh-export-denied.ci` - end-to-end refresh denial withdrawal test
- `test/reload/export-policy-noop.ci` - end-to-end no-op reload test
- `test/interop/scenarios/bgp-route-refresh-prepend-frr/` - declarative Ze (`ze.conf`) and FRR configuration files for the `bgp-route-refresh-prepend-frr` interop scenario

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config or RPC schema changes |
| YANG validation constraints | No | No new config leaves |
| YANG custom validators | No | No new config leaves |
| CLI commands/flags | No | No CLI surface changes |
| CLI grammar (keyword before value) | No | No CLI grammar changes |
| Editor autocomplete | No | No config or CLI surface changes |
| Functional test for new RPC/API | Yes | `test/plugin/route-refresh-export-prepend.ci`, `test/plugin/route-refresh-export-denied.ci`, `test/reload/export-policy-noop.ci` |
| Pipe completeness | No | No command output added or changed |
| Env var registration | No | No environment variables added |
| Doctor check for runtime dependencies | No | No new external runtime dependency |
| Doctor code per problem class | No | No new operator problem class or doctor code; stale retry-cap exhaustion logs with existing logger |
| Prometheus counters/metrics | No | Existing `refused` and `pool-stats` (`rib-out`) metrics cover this path |
| BGP family surface (new SAFI / capability / attribute) | No | Family-agnostic attribute bytes and refresh flow |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Protocol fix; no feature catalog change |
| 2 | Config syntax changed? | No | None |
| 3 | CLI command added/changed? | No | None |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/architecture/api/json-format.md` |
| 5 | Plugin added/changed? | No | Existing filter plugins emit a new event; no new plugin |
| 6 | Has a user guide page? | Yes | `docs/guide/config-reload.md` |
| 7 | Wire format changed? | No | Standard RFC 4271/2918/7313 BGP UPDATE wire framing preserved |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/plugin-development/handlers.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc2918.md` |
| 10 | Test infrastructure changed? | No | None |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, `docs/architecture/buffer-architecture.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/pool-architecture.md` |
| 13 | Route metadata keys added/changed? | Yes | `docs/architecture/meta/README.md` (if `pre-policy-rail` / `pre-policy-asn4` are listed alongside `sent-owner-message`; verify at implementation) |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/architecture/api/process-protocol.md` (`bgp:filter-definition-changed` event and `Sends` declaration) |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/pool-architecture.md`, `docs/architecture/core-design.md`, `docs/architecture/buffer-architecture.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/json-format.md`; verified via `./le spec citation anchors spec plan/release/fixes/spec-bgp-refresh-reapplies-export-policy.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/config-reload.md`, `docs/architecture/api/json-format.md` |

## Design presentation

### What changes

Route refresh and whole-chain stale re-advertisement stop re-running export policy
over post-policy Adj-RIB-Out bytes. Whenever export policy or a forward rail's
post-chain local-AS prepend changes a route's attributes, the sent event carries
the pre-policy attributes along with their producing rail (`announce` or `forward`)
and ASN4 width (`raw.pre-policy-attributes`, `raw.pre-policy-rail`,
`raw.pre-policy-asn4` in `format=full` JSON, and typed fields on structured
`RawMessage`). `bgp-rib` interns that second attribute block in `pool.RibOut` only
when it differs from the sent block, preserves `reconstructRoute` as the sent view
for `show`, lifecycle withdrawals, and abort restoration, and explicitly hands the
pre-policy input (with rail/ASN4 provenance) to ROUTE-REFRESH and whole-chain
stale re-advertisement so export policy evaluates once per refresh in the original
rail order. An ordinary refresh that denies a held route synthesizes a withdrawal
under the existing `adjOut` ownership fence (so peers without RFC 7313 do not keep
denied routes). External filter plugins notify the reactor of definition changes
through `bgp/filter-definition-changed` over `EmitEvent`, `peerSettingsEqual`
and `peerSettingsSwapPlan` switch from raw `reflect.DeepEqual` to
`Policy.SameDecisions`, stale-rail writes re-check `exportGeneration` under
`writeMu` (re-deciding outside `writeMu` on mismatch), and explicit raw writes
(`raw=true`) stop raising the export watermark.

### Before and after

1. **ROUTE-REFRESH on a peer with an export `as-path prepend 1` filter**
   - Before: first UPDATE carries AS_PATH `[65000, 65000, 65001]`; ROUTE-REFRESH replays those post-policy bytes into the export gate and sends `[65000, 65000, 65000, 65001]`.
   - After: `ribOutEntry` retains the pre-policy attributes; every ROUTE-REFRESH runs the export chain once over pre-policy input and sends `[65000, 65000, 65001]`.
2. **ROUTE-REFRESH after tightening an export filter to deny a held route (no RFC 7313)**
   - Before: `writeUpdateGated` returns `nil` on `suppress`, emitting nothing, so the peer retains the denied route indefinitely.
   - After: `writeUpdateGated` synthesizes and sends a bare withdrawal under the existing `adjOut` ownership fence, and the peer removes the denied route.
3. **Config reload that changes nothing on a peer with an in-process filter chain**
   - Before: `peerSettingsEqual` runs `reflect.DeepEqual` over evaluator funcs and mutable `refused` counters, reporting a false change on every reload.
   - After: `peerSettingsEqual` and `peerSettingsSwapPlan` compare policies via `Policy.SameDecisions` (and `Implied()`), reporting no change and leaving the watermark untouched.
4. **Definition change inside an external or in-tree `filter-update` plugin**
   - Before: `SameDecisions` sees only `<plugin>:<filter>` names, so the peer's export watermark never rises and stale routes are not re-judged.
   - After: the filter plugin emits `bgp/filter-definition-changed` naming `{"refs":["<plugin>:<filter>"]}`, raising `exportGeneration` and `exportPolicyFrom` on peers whose export chain references it.
5. **Stale unit decided just before a policy reload, or explicit raw UPDATE injection**
   - Before: a stale unit decided under generation G could write old-chain bytes after generation G+1 published; a reachable `raw=true` diagnostic write raised `exportPolicyFrom` once `exportGeneration > 0`.
   - After: stale writes verify `exportGeneration` under `writeMu` before emitting and re-decide outside `writeMu` on mismatch; `raw=true` writes skip `noteExportWrite`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Add a replay exemption in `Session.writeUpdateGated` to skip the export chain on `metaKeyReplay` | Violates RFC 2918 Section 4 ("based on its outbound route filtering policy"): policy edits and stateful filters would be bypassed on refresh |
| Replay from `bgp-adj-rib-in` receive store on ROUTE-REFRESH | Holds received routes only and is optional; locally originated and config-static routes have no receive-store entry |
| Store two attribute handles unconditionally for every route in `ribOutEntry` | Most routes are not modified by export policy; omitting the second handle when byte-identical avoids doubling `pool.RibOut` references |
| Run stale-rail `decideStaleReadvertise` while holding `Session.writeMu` | An external filter's `filter-update` RPC under `writeMu` stalls BGP KEEPALIVEs and risks hold-timer expiry |
| Rebuild D-6's `joinExportWriters` with a deferred-join barrier | Obsolete on HEAD: `noteExportPolicyChangeLocked` already replaced the barrier with lock-free atomic `exportGeneration` + `raiseExportPolicyFrom` |

### Documents departed from

| Document | Section | Before (current statement) | After (what the page says after this work) |
|----------|---------|----------------------------|--------------------------------------------|
| `docs/architecture/core-design.md` | Policy Filter Chain | "The writer therefore compares the decision's stamp with the current generation when it takes the receipt, and a write of an older decision raises the watermark past its own receipt." | Stale readvertisement checks `owner.exportGeneration == peer.exportGeneration.Load()` under `writeMu` before emitting to TCP and re-decides outside `writeMu` on mismatch (capped at 8 retries, fail-closed); `Session.noteExportWrite` stays as the defense-in-depth receipt watermark guard across all rails |
| `docs/architecture/core-design.md` | Policy Filter Chain | "A write no export chain judged carries no stamp, which reads as generation zero: it raises the watermark once the peer's generation is above zero, and at generation zero, before any change, it matches and raises nothing." | Explicit raw diagnostic writes (`raw == true`) run no export chain and skip `Session.noteExportWrite`, leaving `exportPolicyFrom` untouched so raw injections do not trigger spurious whole-chain re-judging |
| `docs/architecture/core-design.md` | Policy Filter Chain | "Message ids are one global counter, so this needs no new field on the plugin protocol and a forked RIB changes nothing." | Message IDs remain the watermark without a new watermark protocol field, while `format=full` sent events add optional `raw.pre-policy-attributes`, `raw.pre-policy-rail`, and `raw.pre-policy-asn4` (and replay commands carry `pre-policy-rail` / `pre-policy-asn4` in `CommandContext.Meta`) so internal and forked RIBs alike evaluate refreshes and whole-chain stale passes over pre-policy input |
| `docs/architecture/core-design.md` | Policy Filter Chain | "A change inside an external plugin's own data is not a policy change here and stays invisible to the comparison." | External and in-tree `filter-update` plugins declare `{Message: "bgp:filter-definition-changed"}` in `Sends` and emit `bgp:filter-definition-changed` with `{"refs":["<plugin>:<filter>", ...]}` when a definition changes; the reactor raises `exportGeneration` and `exportPolicyFrom` on peers whose `ExportPolicy` references any changed member |

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- sent event pre-policy fields (`RawMessage`, `format=full` JSON, `Event`) and `ribOutEntry` second handle with refcount/release
   - Tests: `TestSentEventCarriesPrePolicyAttributes`, `TestRefreshReplaysPrePolicyAttributes`
   - Files: `types/rawmessage.go`, `types/types.go`, `event.go`, `format/text_update.go`, `session_write.go`, `reactor_api_forward.go`, `forward_rs.go`, `reactor_notify.go`, `ribout_entry.go`, `rib.go`, `rib_structured.go`
   - Verify: RIB stores and releases both handles without leaking `pool.RibOut` slots, unchanged routes store one handle, and `reconstructRoute` preserves the sent view
2. **Phase: Refresh reads pre-policy & ordinary refresh withdrawal** -- D-1, D-2 (AC-7), D-7 (AC-12)
   - Tests: `TestRefreshForwardedRouteKeepsRailOrder`, `TestOrdinaryRefreshDenialWithdrawsHeldRoute`, `TestRawWriteDoesNotRaiseExportWatermark`, `test/plugin/route-refresh-export-prepend.ci`, `test/plugin/route-refresh-export-denied.ci`
   - Files: `ribout_entry.go`, `rib_replay.go`, `rib_commands.go`, `rib_selected_refresh.go`, `rib_selected_refresh_configured.go`, `cmd/update/update_text.go`, `reactor_api_batch.go`, `session_write.go`, `refresh_prepend_double_test.go` (under `ze-test-judge`)
   - Verify: tests fail before fix, pass after fix; revert-check confirms red without fix
3. **Phase: Stale rail pre-policy & pre-write generation check** -- AC-5, AC-6, D-5 (AC-11)
   - Tests: `TestStaleRailReadsPrePolicyOnWholeRun`, `TestStaleDenialWithdrawsHeldRoute`, `TestStaleDenialOnNewSessionSendsNothing`, `TestStaleWriteRecheckedUnderWriteMu`
   - Files: `stale_readvertise_chain.go`, `reactor_api_batch.go`
   - Verify: tests fail, implement, tests pass
4. **Phase: Watermark truth & external filter notification** -- D-3 (AC-8), D-4 (AC-9, AC-10)
   - Tests: `TestPolicyNamesExternalFilter`, `TestNoOpReloadKeepsExportWatermark`, `TestDefinitionChangeRaisesWatermark`, `TestFilterDefinitionEventRaisesWatermark`, `test/reload/export-policy-noop.ci`
   - Files: `filterapi/policy.go`, `internal/core/bgp/events/events.go`, `reactor/reactor_filter_events.go`, `reactor/reactor_api.go`, `reactor/peer_settings_apply.go`, `plugins/filter_community/{register.go,filter_community.go}`, `plugins/filter_family/{register.go,filter_family.go,config_tx.go}`, `plugins/filter_irr/{register.go,filter_irr.go,config_tx.go}`, `plugins/filter_modify/{register.go,filter_modify.go,config_tx.go}`, `plugins/filter_remove_private_as/{register.go,filter_remove_private_as.go,config_tx.go}`
   - Verify: tests fail, implement, tests pass; `./le plugin declarations check` confirms `registry.Registration.Sends` and Stage 1 `sdk.Registration.Sends` agree
5. **Phase: Interop, RFC test hygiene, and docs** -- `test/interop/scenarios/bgp-route-refresh-prepend-frr/`, `internal/le/interoplab/bgp/checkers.go`, `rfc4271_test.go` (under `ze-test-judge`), and all updated architecture/guide/RFC pages (including `docs/architecture/buffer-architecture.md` "When a copy is deliberate")

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-12 has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No replay exemption in `writeUpdateGated`: the chain runs on every refresh |
| Correctness | `ribOutEntry.release()` releases both `AttrHandle` and `PrePolicyAttrHandle` on every overwrite, withdrawal, and peer purge |
| Naming | All JSON keys (`pre-policy-attributes`, `pre-policy-rail`, `pre-policy-asn4`), metadata keys, and event type (`filter-definition-changed`) are kebab-case |
| Data flow | The reactor keeps no pre-policy route store; `bgp-rib` is the only store |
| Rule: one chain | Nothing names prepend or any specific filter member (`ai/META-DECISIONS.md`) |
| Rule: stale comments | Comments on `peerSettingsEqual`, `peerSettingsSwapPlan`, `writeUpdateBody` / `noteExportWrite`, and `core-design.md` reflect the new behavior |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Refresh prepend & denial paths | `./le test bgp plugin route-refresh-export-prepend`, `./le test bgp plugin route-refresh-export-denied` |
| Reload path | `./le test functional reload` |
| Scoped unit tests | `./le job run label refresh-unit command go test -race -tags ze_bgp ./internal/component/bgp/reactor/ ./internal/component/bgp/plugins/rib/ ./internal/component/bgp/filterapi/ ./internal/component/bgp/format/ ./internal/component/bgp/plugins/cmd/update/` |
| Peer proof | `./le test integration interop` with scenario `bgp-route-refresh-prepend-frr` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Fail-closed | A failed comparison, invalid pre-policy rail metadata, or exhausted stale retry cap fails closed (re-judges or withholds, never bypasses policy) |
| Resource exhaustion | `PrePolicyAttrHandle` is interned and refcounted in `pool.RibOut` and absent when unchanged; leaks show in `pool.RibOut` metrics |
| Input validation | `bgp/filter-definition-changed` validates `<plugin>:<filter>` against each peer's registered export chain; unknown or malformed refs raise nothing |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- `sendBatchUpdate` passes `writeUpdate`, `handleSent` drops replays so prepends do not accumulate across refreshes, and RFC 2918 Section 4 rules out skipping the chain.
- The isolated red test `TestOrdinaryRefreshDoesNotDoublePrepend` cannot go green under the RIB fix because it supplies post-policy bytes directly to `sendBatchUpdate`, bypassing the RIB boundary; its replacement requires `ze-test-judge`.
- D-2 is narrower under RFC 7313 where EoRR removes un-refreshed routes, but remains a live defect on plain RFC 2918 ROUTE-REFRESH without RFC 7313 (AC-7).
- D-6 (`joinExportWriters` / `MaxUint64`) was already replaced on HEAD by lock-free atomic `noteExportPolicyChangeLocked`, eliminating the barrier stall without needing a deferred-join mechanism.
- `Session.noteExportWrite` already closes the watermark-assurance hole for late old receipts, while AC-11 closes the stronger wire behavior by preventing an old-chain stale decision from writing to TCP after a reload.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `non-idempotent-append-duplicates-the-record.md` | 2026-11-06 | refresh re-applies export policy over post-policy `ribOut` bytes |
| `policy-change-never-re-evaluates-what-it-admitted.md` | 2026-11-06 | stale rail denial suppresses and leaves the peer's route |
| `memo-suppresses-a-change-it-cannot-see.md` | 2026-11-06 | external plugin definition change invisible to `exportChanged` |
| `state-event-emitted-without-comparing-state.md` | 2026-11-06 | rebuilt chain with a func member never compares equal |
| `late-write-lands-on-the-successor.md` | 2026-11-06 | stale decision before `writeMu` writes under the old chain |
| `grace-bound-hides-a-missing-signal.md` | 2026-11-06 | export policy reload stalls at the barrier, then `MaxUint64` (already resolved on HEAD by lock-free `noteExportPolicyChangeLocked`; mark closed at Commit A) |
| `state-event-emitted-without-comparing-state.md` | 2026-11-06 | `peerSettingsEqual` runs `DeepEqual` over the policies and reads `refused` |
| `state-event-emitted-without-comparing-state.md` | 2026-11-06 | raw injection carries no generation and raises the export watermark |
| `policy-change-never-re-evaluates-what-it-admitted.md` | 2026-11-06 | a filter's Withdraw on a no-owner stale batch removes whatever the key holds (received routes already spared by `sendOwnRouteWithdrawal`; see Known Limitations for unowned originated residual) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-06 | import member strips LLGR_STALE; forward-rail LOCAL_PREF and NO_EXPORT undo (`filter_community` AttrMod and export `runFinalJudges` already landed on HEAD; see Known Limitations for raw import override residual) |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-06 | RFC4271-5.1.2-2: no tagged unit reaches the route-server rail AS_PATH guards (already resolved on HEAD by `rfc4271_rs_aspath_export_test.go`; mark closed in `plan/journal/guard-added-to-one-half-of-a-pair.md` at Commit A) |
| `green-that-could-not-have-been-red.md` | 2026-11-06 | `TestRFC4271LocalPrefIncludedForInternalPeers` claims more than its body checks |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The RIB stores pre-policy attributes with rail and ASN4 provenance | A replay exemption in the writer; receive-store replay | Exemption violates RFC 2918 Section 4; receive store holds no originated routes and is optional |
| Second handle in `pool.RibOut` only when pre-policy differs from sent | Always two handles | Most chains leave attributes unchanged; omitting the second handle when identical costs zero extra slots or wire bytes |
| `Policy.SameDecisions` in `peerSettingsEqual` and `peerSettingsSwapPlan` | Removing funcs from `policychain.Chain`; hashing compiled chains | Evaluator funcs exist by design; `SameDecisions` already compares compiled config/resolved definitions cleanly |
| `bgp/filter-definition-changed` event via `EmitEvent` | Polling external plugins on reload; reading plugin config in reactor | Reactor never names plugins; `EmitEvent` works identically over DirectBridge and external plugin RPC |
| Pre-write `exportGeneration` re-check under `writeMu` with bounded re-decision outside `writeMu`, keeping `noteExportWrite` | Running filter RPCs under `writeMu`; relying solely on `noteExportWrite` | RPCs under `writeMu` stall KEEPALIVEs; `noteExportWrite` alone still permits one old-chain TCP write after reload |
| Skip `noteExportWrite` when `raw == true` | Stamping raw writes with current generation | Explicit raw diagnostic writes run no export policy chain and hold no policy decision for a reload to supersede |

## Known Limitations
- A stateful external filter can answer a refresh differently from the first send; RFC 2918 Section 4 permits this.
- An unowned stale batch carrying `SentOwnerMessage == 0` cannot distinguish two locally originated routes under one prefix key because no receipt identifies the older revision; all RIB-generated refresh and stale batches carry `SentOwnerMessage` and use `adjOutExpected`.
- While `filter_community` preserves `LLGR_STALE` (0xFFFF0006) across all typed `AttrMod` operations on both import and export and `runFinalJudges` protects export chains, a raw whole-UPDATE external import rewrite that omits `LLGR_STALE` is not separately restored at ingress in this spec.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code. RFC 2918 Section 4 stays above the refresh path; the ordinary refresh denial withdrawal branch quotes RFC 2918 Section 4, RFC 4271 Section 9.1.3, and RFC 7313 Section 4 where relevant.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 2918 Section 4 | On receiving ROUTE-REFRESH for a negotiated `<AFI, SAFI>`, re-advertise Adj-RIB-Out evaluated once under the current outbound filtering policy over pre-policy attributes | Stateful external filters may return updated verdicts on refresh | `TestRefreshReplaysPrePolicyAttributes`, `TestRefreshForwardedRouteKeepsRailOrder`, `test/plugin/route-refresh-export-prepend.ci`, `bgp-route-refresh-prepend-frr` | `TestOrdinaryRefreshDenialWithdrawsHeldRoute`, `test/plugin/route-refresh-export-denied.ci` |
| RFC 7313 Section 4 | Enhanced Route Refresh brackets refresh with BoRR/EoRR; without RFC 7313, a route denied on refresh is explicitly withdrawn | Enhanced refresh BoRR/EoRR markers remain intact | `TestOrdinaryRefreshDenialWithdrawsHeldRoute`, `test/plugin/route-refresh-export-denied.ci` | Existing RFC 7313 BoRR/EoRR tests in `reactor/` and `plugins/rib/` |
| RFC 9494 Sections 4.3, 4.6 | Opted-only stale passes keep reading the sent body; whole-chain stale passes after policy change evaluate pre-policy attributes and run opted-in members last | `llgr-egress` suppress and withdraw semantics preserved | `TestStaleRailReadsPrePolicyOnWholeRun`, `TestStaleDenialWithdrawsHeldRoute`, `TestStaleDenialOnNewSessionSendsNothing` | Existing `stale_export_stamp_test.go` LLGR tests |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | pending implementation and independent `/ze-review` |
| Verdict | pending |

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
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-12 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run in the main checkout by the main thread: `./le test bgp plugin route-refresh-export-prepend`, `./le test bgp plugin route-refresh-export-denied`, `./le test functional reload`, and the scoped package tests above. GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script
