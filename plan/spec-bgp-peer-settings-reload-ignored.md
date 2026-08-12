# Spec: Config reload silently discards changed per-peer settings

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 4/9 |
| Updated | 2026-08-12 |

## B1 ANSWERED (Thomas, 2026-08-07)

**B1 is no longer blocking.** Thomas was asked to categorise about fifty
`PeerSettings` fields as hot-swappable or restart-requiring. He answered with a
decision procedure instead, quoted verbatim:

> if capabilities are removed from the peer which were not used or if added when
> the other peer does not use them, ie: if the resulting negotiation would be
> similar and lead to the same encoding and same families, we can accept the
> change and keep the BGP session up, otherwise have to re-start and re-negotiate

The ruling is a RULE, not a per-field classification. It is implemented as
`negotiationOutcomeUnchanged` (`peer_settings_negotiation.go`) and described
under D-6 below. No field was hand-classified to satisfy it.

**What the ruling does NOT answer.** It governs the capability set, which is what
it names. It says nothing about the fields whose effect never reaches the OPEN,
so those keep failing closed to a restart, and that remains correct: restarting
for a field nobody classified is visible and self-healing, while running a
session on settings nobody checked is the silent mis-enforcement this spec exists
to remove. The three fields in `hotSwappableSettings` stay swappable for their
own separate reason (D-7).

Earlier history, kept because it explains the shape of the code. A2 shipped
alone, against this spec's own "A2 MUST NOT SHIP ALONE": `peerSettingsEqual`
(`internal/component/bgp/reactor/reactor_api.go`) compares every field, and
`reconcilePeersJournaled` applied any difference by removing and re-adding the
peer, so an edit to a static route or a prefix limit bounced the session. The
swap-or-restart split (A3/B2/B3) then landed in
`internal/component/bgp/reactor/peer_settings_apply.go`, and `PrefixUpdated` was
classified swappable on 2026-08-07 (B1's "Cosmetic" line below, corrected).

Phase note (was in the Phase cell; moved 2026-07-22): A1+A2 DONE (bug reproduced,
guard fixed via `reflect.DeepEqual` in `reactor_api.go`, landed 38170a13b;
package green). A3/A4 + Phase B NOT started. NOT shippable — see "A2 MUST NOT
SHIP ALONE". The A/B split needs consolidating (next section) before
implementation resumes; the 2/9 count reflects the CURRENT A1-A4+B1-B5
structure, not the unapproved 7-step consolidation.

## ⚠ Phase structure needs consolidating (recorded 2026-07-16)

The A/B split was designed around D-1's mislabel (Phase A = interim refresh-based apply, Phase B =
pre-policy store). D-1b cancels the store, which makes the split partly redundant:
- **A3 (settings swap)** and **B2 (atomic swap for hot-swappable fields)** are now the SAME work.
- **A4 (apply via `SoftClearPeer` refresh)** is now likely unnecessary: BIRD does not re-obtain
  routes on reconfigure, it swaps the pointer and lets the new chain govern subsequent routes
  (`bird-bgp-reference.md`). A4 asks the peer to re-send, which is a strictly larger promise
  than parity and was only there to compensate for the store that no longer exists.
- **B1 (categorise fields)** should arguably run BEFORE the swap, since the category decides
  whether a change swaps or restarts.

→ Suggested consolidation (NOT yet user-approved, do not execute silently): a single sequence —
(1) failing tests [DONE], (2) fail-closed guard [DONE], (3) categorise fields, (4) atomic swap for
hot-swappable + restart for FSM-relevant, (5) log the restart category, (6) `.ci` + interop, (7)
fix the false doc claim at `bird-bgp-reference.md`. Present this to the user before resuming.
→ Constraint: A1+A2 are already implemented and green; consolidation must not discard them.
| Updated | 2026-07-16 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/reactor/reactor_api.go` (`peerSettingsEqual` :780, `reconcilePeersJournaled` :477)
4. `internal/component/bgp/reactor/filter_ordered.go`, `internal/component/bgp/reactor/peer.go`
5. `plan/spec-bgp-filtered-route-storage.md` - the spec blocked on this one (its BLOCKER section)

## Task

**A BGP peer's configured policy and behavior settings do not take effect when the operator
edits the config and reloads. The change is silently discarded and the daemon reports success.**

`peerSettingsEqual` (`reactor_api.go`) decides whether a peer changed on reload by
comparing a **hand-maintained list** of fields. Functionally significant fields are missing from
that list, so a config edit touching only those fields is judged "functionally equivalent". The
peer is not reconciled, the freshly parsed `*PeerSettings` carrying the new values is dropped, and
the running peer keeps its original settings until the daemon restarts.

Fields present in `PeerSettings` but ABSENT from `peerSettingsEqual`:
`ImportFilters`, `ExportFilters`, `RouteReflectorClient`, `ASOverride`, `RSClient`, `RSFastPath`,
`AcceptSRv6PrefixSID`, `ClusterID`, `NextHopMode`.

The worst facet is import/export policy: the operator edits policy, reloads, sees success, and the
**datapath keeps enforcing the old chain**. `bgp peer <ip>` then DISPLAYS the new `import-policy`
(it reads the re-parsed config, not the running peer), so the display and the enforcement disagree.
`peerDiffCount` (`reactor_api.go`) uses the same predicate and reports **0 peer changes**. The
failure is silent and in the direction that looks like success: an operator tightening policy to
block a prefix believes it applied.

This is a **pre-existing correctness defect discovered during the audit of
`spec-bgp-filtered-route-storage`** (2026-07-16), not a new feature. That spec is now `blocked` on
this one: its AC-3 ("loosen policy → routes_filtered drops") cannot be built while loosening policy
does nothing.

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x]. Capture insights as → Decision: / → Constraint: annotations. -->
- [ ] `docs/architecture/core-design.md` - reactor lock discipline, peer lifecycle
  → Constraint: TO BE FILLED by the implementing session. Do not copy a constraint from another
  spec's annotation: three of `spec-bgp-filtered-route-storage`'s Required Reading citations were
  fabricated. Read the producer (`ai/rules/evidence.md`).
- [ ] `ai/rules/evidence.md` - this defect IS the canonical shape the rule warns about
  → Constraint: a hand-maintained equality list is a guard whose omission looks like working code.
  Whatever replaces it must fail CLOSED: an unknown/new field must force a change, never silently
  compare equal.

### RFC Summaries
- [ ] `rfc/short/rfc2918.md` (Route Refresh) - needed only if the chosen design uses refresh to
  re-apply policy without bouncing the session.
  → Constraint: TO BE FILLED once the design decision is made.
- [ ] `rfc/full/rfc5492.txt` Section 2 (Capability Advertisement) - read 2026-08-07 for D-6.
  → Decision: the ruling is conformant, and Section 2 is why. "A given capability can be
  used on a peering if that capability has been advertised by both peers. If either peer
  has not advertised it, the capability cannot be used." The swap changes what ze will
  advertise in the NEXT OPEN; it never changes what the running peering uses, because
  every wire decision reads the negotiated state (`EncodingContext`, `Negotiated`) and
  the only runtime readers of `PeerSettings.Capabilities` are `buildOpen` (once per
  connection), `GetPeerCapabilityConfigs` and `validatePeerFamilies`. A capability the
  peer did not advertise stays unusable whether ze's config lists it or not, which is
  exactly the pair of cases the ruling names.
  → Constraint: BGP-4 has no mid-session capability renegotiation. Section 2 sources the
  peer's capabilities from the OPEN alone, so "apply at the next OPEN, or restart" is the
  only conformant pair of options. There is no third.
- [ ] `rfc/full/rfc4271.txt` Sections 4.2 and 6.8 (OPEN fields, collision detection) - read
  2026-08-07 for D-6.
  → Constraint: the negotiated result is not enough on its own. My AS, the BGP Identifier
  and the Hold Time reach the peer unmediated by any negotiation and are what the peer
  keys collision detection and its own hold timer off, and a change to any of them leaves
  `capability.Negotiate` returning the same value on ze's side. `openHeaderEqual`
  therefore compares them and restarts on a difference.

**Key insights:**
- Ze already has the refresh machinery (`SoftClearPeer` `reactor_api.go`; `SendRefresh`/`SendBoRR`/`SendEoRR` `reactor_api_forward.go`), but nothing on the reload path calls it.

## Current Behavior (MANDATORY)

**Source files read (each verified at the producer, 2026-07-16):**
<!-- NEVER tick [ ] to [x]. -->
- [ ] `internal/component/bgp/reactor/reactor_api.go` (`peerSettingsEqual`, lines 780-825) - compares identity, connectivity, a 7-field behavior block (`:803-809`: ReceiveHoldTime, SendHoldTime, KeepaliveTime, ConnectRetry, GroupUpdates, IgnoreFamilyMismatch, DisableASN4), `len(StaticRoutes)`, and capabilities by wire encoding. Nothing else.
  → Constraint: its doc comment claims "Returns true if the settings are functionally equivalent" (`:778-779`). That claim is FALSE for the nine omitted fields. Fixing the comment is not fixing the bug.
- [ ] `internal/component/bgp/reactor/reactor_api.go` (`reconcilePeersJournaled`, line 477) - at `:498-513`, a peer whose settings compare equal lands in neither `toRemove` nor `toAdd`, so the newly parsed `*PeerSettings` goes out of scope and is garbage.
  → Constraint: there is NO in-place update branch. Reconcile is remove-then-re-add only (`:516-546`, `peer.Stop()` at `:524`), i.e. today any applied change costs a session bounce.
- [ ] `internal/component/bgp/reactor/peer.go` (line 318) - `settings` is assigned once in the `Peer` constructor (struct literal `settings: settings`). `Settings()` (`peer.go`) returns the pointer.
  → Constraint: grep for `p.settings =` / `peer.settings =` across `internal/component/bgp/reactor/` returns EMPTY. There is no setter. The only post-construction write to the pointee is `resolveDynamicPeerSettings` (`reactor_dynamic.go`), which fires on Established for DYNAMIC peers only.
- [ ] `internal/component/bgp/reactor/filter_ordered.go` (lines 134-141) - `runIngressPolicyChain` reads `filters := peer.settings.ImportFilters` live per message, off the never-reassigned pointer. "Live" is real but useless: the target never changes.
- [ ] `internal/component/bgp/config/peers.go` (line 155) - the producer of the value: `ps.ImportFilters = concatFilters(bgpImport, groupImport, peerImport)`. Reload re-runs this exact path (`config/loader_create.go` `createReloadFunc` → `PeersFromConfigTree`), so the NEW list is correctly computed — and then discarded downstream.
- [ ] `internal/component/bgp/reactor/policy_dryrun.go` (lines 95-96) - already documents the concurrency hazard: "ImportFilters/ExportFilters can be rewritten by the peer FSM goroutine", and snapshots under `r.mu`.
  → Constraint: `peer.settings` is read from the session read goroutine (`filter_ordered.go`) with no lock. Any fix that mutates settings from the reload goroutine introduces a data race unless it swaps an atomic pointer or takes a lock. This hazard already exists via `resolveDynamicPeerSettings`.

**Reported by the audit but NOT yet verified at the producer (verify before relying on it):**
- Every `filter_*` plugin under `internal/component/bgp/plugins/` registers only `OnConfigure`, which
  is startup-only (`plugin/server/startup.go`, driven solely from `:578`); `internal/plugins/vrrp/register.go`
  reportedly states "OnConfigure does not fire on reload; OnConfigApply is the commit step". If true,
  changing a filter's CONTENTS (e.g. adding a prefix to a prefix-list) also fails to apply on reload
  — a second, independent facet of the same defect class.
  → Constraint: this facet is UNVERIFIED. Read those producers before scoping it in or out.

**Behavior to preserve:**
- A config edit that genuinely changes nothing MUST still be a no-op (no gratuitous peer bounce).
- The journal/rollback semantics of `reconcilePeersJournaled` (`reactor_api.go`) stay intact.
- Capability changes keep forcing a bounce (renegotiation is required); `capabilitiesEqual` stays.

**Behavior to change:**
- An edit to any functionally significant per-peer field MUST take effect on reload.
- `peerDiffCount` (`reactor_api.go`) MUST report such an edit as a change.
- The `bgp peer <ip>` `import-policy` display MUST agree with what the datapath enforces.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator edits the config file and reloads (`Reload()` `reactor_api.go`, or the
  transactional path `plugin/register.go` `OnConfigApply` → `ReconcilePeersWithJournal`
  `reactor.go`). Both funnel into `reconcilePeersJournaled`.
- Format at entry: config file text → `*config.Tree` → `map[string]any` → `[]*PeerSettings`.

### Transformation Path
1. `createReloadFunc` (`config/loader_create.go`) parses and calls `PeersFromConfigTree`.
2. `peers.go` computes the NEW `ImportFilters` (3-layer merge). **Correct at this point.**
3. `reconcilePeersJournaled` (`reactor_api.go`) diffs old vs new via `peerSettingsEqual`.
4. **THE DEFECT** (`reactor_api.go`): the predicate returns true for a policy-only edit, so
   the peer is skipped and the new settings are dropped.
5. The running peer keeps its constructor-assigned `settings` pointer; `filter_ordered.go`
   reads the stale `ImportFilters` forever.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file <-> PeerSettings | reload re-runs the full parse pipeline | [x] verified: `loader_create.go` |
| Reload goroutine <-> session read goroutine | `peer.settings` read unlocked on the hot path | [x] verified: `filter_ordered.go`; hazard documented `policy_dryrun.go` |
| PeerSettings <-> datapath | the ONLY link is the pointer assigned at `peer.go` | [x] verified: no setter exists |

### Integration Points
- `reactor_api.go` `peerSettingsEqual` - the guard to fix.
- `reactor_api.go` `reconcilePeersJournaled` - may need an in-place/refresh branch.
- `reactor_api.go` `SoftClearPeer` / `reactor_api_forward.go` `SendRefresh` - existing
  refresh machinery a non-bouncing design could reuse.

### Architectural Verification
- [ ] No bypassed layers (the fix lives on the reconcile path, not a side channel).
- [ ] No unintended coupling.
- [ ] No duplicated functionality (reuses existing refresh machinery if the design needs refresh).
- [ ] Zero-copy preserved where applicable.
- [ ] Registration over hardcoding - see A-2: the replacement guard must not be another
  hand-maintained field list that the next new leaf silently falls out of.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The omission is an oversight, not a deliberate design | `:778-779` comment claims functional equivalence, which is false for the 9 omitted fields; no comment anywhere explains an intentional exclusion | If deliberate, there is a hidden apply path to find first | grep for a design note / learned summary on `peerSettingsEqual`; check git history | unvalidated |
| A-2 | A field-by-field equality list will rot again | This defect IS the rot; `GroupUpdates` is covered while its neighbours are not | A cheap fix (add 9 fields) leaves the next leaf to fall out silently | design review: prefer a mechanism that fails closed for unknown fields | unvalidated |
| A-3 | Bouncing the session on a policy edit is acceptable to operators | none - this is exactly the open design question | The cheap fix is unshippable; soft reconfiguration is required | USER DECISION (Phase 1) | unvalidated |
| A-4 | Nothing else already re-applies settings post-reload | grep for `p.settings =` returned empty; `reconcilePeersJournaled` has no in-place branch | The bug is narrower than stated | verified 2026-07-16 (grep + read) | **confirmed** |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Data race: mutating `peer.settings` from the reload goroutine while `filter_ordered.go` reads it unlocked on the session goroutine | `go test -race` failures; corrupted filter chain | Atomic pointer swap or copy-under-lock. The hazard is pre-existing via `resolveDynamicPeerSettings` (`reactor_dynamic.go`) and `policy_dryrun.go` already snapshots under `r.mu` |
| R-2 | Fixing the guard makes every reload bounce every peer (over-triggering) | sessions flap on an unrelated config edit | Compare only functionally significant fields; add a test asserting a no-change reload bounces nothing |
| R-3 | A policy edit that now applies changes the RIB, surprising operators used to the buggy no-op | route churn after upgrade | Document in the changelog as a behavior fix; it is the correct behavior |
| R-4 | The filter-CONTENTS facet (unverified) is a separate defect that leaves the fix half-effective | policy contents still stale after reload despite the fix | Verify the `OnConfigure`-on-reload claim early; scope in or split into its own spec |

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| Operator edits a peer's import policy and reloads | -> | `reconcilePeersJournaled` marks the peer changed and the new chain is enforced | `test/reload/reload-import-policy-applies.ci` PASS 2026-08-12 |
| Operator reloads with NO change | -> | no peer bounce, no churn | `test/reload/reload-no-change.ci`; `TestPeerDiffCountIsZeroForAnIdenticalReload` |

→ Constraint: the two file names this table carried until 2026-08-12
(`bgp-import-policy-applies.ci`, `bgp-reload-noop-no-bounce.ci`) were never
created under those names. The rows now name the files that exist.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Peer established; operator changes ONLY the peer's import filter list and reloads | The new chain is enforced on subsequently evaluated routes; a prefix newly rejected by the new policy is no longer accepted |
| AC-2 | Same as AC-1 | `peerDiffCount` reports >= 1 changed peer (not 0) |
| AC-3 | Reload with a byte-identical config | No peer is bounced; `peerDiffCount` reports 0 |
| AC-4 | Operator changes `route-reflector-client` and reloads | The new value governs forwarding (`peer_forward_facts.go` reads the new value) |
| AC-5 | `bgp peer <ip>` after a policy reload | The displayed `import-policy` matches what the datapath enforces |
| AC-6 | A NEW per-peer field is added to `PeerSettings` and changed across a reload | It is not silently ignored (A-2: the guard fails closed for fields it does not know) |
| AC-7 | `go test -race` over a reload-while-receiving-UPDATEs test | No data race on `peer.settings` (R-1) |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Tightens import policy to block a prefix, reloads, and the prefix is actually blocked | edit -> reload -> reconcile -> new chain at `filter_ordered.go` -> reject | `test/reload/bgp-import-policy-applies.ci` |
| 2 | Reloads an unchanged config and sees no session flap | edit-none -> reload -> reconcile no-op | `test/reload/bgp-reload-noop-no-bounce.ci` |
| 3 | Reads `bgp peer <ip>` and trusts that the displayed policy is the enforced policy | display vs datapath agree | `test/reload/bgp-import-policy-applies.ci` (assert both) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPeerSettingsEqualDetectsImportFilterChange` | `internal/component/bgp/reactor/reactor_api_test.go` | the guard returns false when only ImportFilters differs (AC-1/AC-2) | |
| `TestPeerSettingsEqualDetectsEachSignificantField` | `internal/component/bgp/reactor/reactor_api_test.go` | table-driven over ALL 9 omitted fields (AC-4) | |
| `TestPeerSettingsEqualIdenticalIsEqual` | `internal/component/bgp/reactor/reactor_api_test.go` | no over-triggering (AC-3, R-2) | |
| `TestPeerSettingsEqualFailsClosedOnUnknownField` | `internal/component/bgp/reactor/reactor_api_test.go` | AC-6 / A-2; shape depends on the chosen mechanism | |
| `TestReloadWhileReceivingNoRace` | `internal/component/bgp/reactor/peer_settings_reload_race_test.go` | AC-7 / R-1. Reloads that swap the chains run against `runIngressPolicyChain`, the REAL ingress reader, not the accessor alone | PASS 2026-08-12, `make ze-race-reactor` |
| `TestPeerDiffCountIsZeroForAnIdenticalReload` | `internal/component/bgp/reactor/peer_settings_swap_test.go` | AC-3 / R-2: a reload that changes nothing reports no change | PASS 2026-08-12 |
| `TestReloadRouteReflectorClientReachesForwarding` | `internal/component/bgp/reactor/peer_settings_swap_test.go` | AC-4: the restart branch delivers the new value to `peerForwardFacts` | PASS 2026-08-12 |
| `TestReloadImportPolicyDisplayMatchesDatapath` | `internal/component/bgp/reactor/peer_settings_swap_test.go` | AC-5: `Peers()` and `Peer.ImportFilters()` both report the NEW chain | PASS 2026-08-12 |
| `TestOpenHeaderEqualCoversEveryOpenField` | `internal/component/bgp/reactor/peer_settings_negotiation_test.go` | `openHeaderEqual` discriminates on every `message.Open` field except `OptionalParams`, with the field list derived from the struct by reflection | PASS 2026-08-08 |
| `TestReloadDecisionReadsPeerSettingsUnderLock` | `internal/component/bgp/reactor/peer_settings_negotiation_test.go` | the reload reconcile reads the running peer's settings under `p.mu`, via `Peer.SettingsSnapshot`. Race-detector test: evidence only under `make ze-race-reactor` | PASS 2026-08-08 |
| `TestNegotiationProbeFailsClosed/router_id_change_alongside_an_ignorable_capability_change` | `internal/component/bgp/reactor/peer_settings_negotiation_test.go` | the multi-field restart reason names BOTH fields, asserted exactly rather than by `Contains` | PASS 2026-08-08 |

#### Round-1 review fixes: discrimination evidence (2026-08-08)

Each fix was reverted, its test run, and the fix restored. `make ze-test-pkg PKG=./internal/component/bgp/reactor`.

| Fix | Mutation applied | Test | Observed red |
|-----|------------------|------|--------------|
| `openHeaderEqual` compares a derived field list rather than a hand-picked one (`peer_settings_negotiation.go`) | the hand-picked list restored with one field escaping it (`ASN4` dropped). A plain revert is green by construction: the pre-fix list was TOTAL over `message.Open` today, so the escaped field IS the failure mode the fix removes | `TestOpenHeaderEqualCoversEveryOpenField` | `--- FAIL: .../ASN4` -- `Should be false`, `a difference in ASN4 reaches the peer unmediated by any negotiation` |
| The reload reconcile takes `Peer.SettingsSnapshot` rather than `Peer.Settings` (`reactor_api.go`) | `peer.SettingsSnapshot()` reverted to `peer.Settings()` in `reconcilePeersJournaled` | `TestReloadDecisionReadsPeerSettingsUnderLock` | `WARNING: DATA RACE` -- read in `peerSettingsEqual` from `reconcilePeersJournaled` against a write in `hotSwappableSettings` from `applyHotSwappableSettings`. Note the racing read is `peerSettingsEqual`, NOT `peerSettingsSwapPlan`: a lock taken inside the plan would not have covered it |
| The multi-field restart reason is asserted exactly (`peer_settings_negotiation_test.go`) | the `changed = append(changed, "Capabilities")` branch removed from `peerSettingsSwapPlan`, so only one of the two names survives | `TestNegotiationProbeFailsClosed` | `expected: "Capabilities,RouterID"`, `actual: "RouterID"`. The previous `assert.Contains(reason, "RouterID")` passes against that same mutation, which is why it was not evidence |

#### Round-2 discrimination evidence (2026-08-12), AC-1/AC-3/AC-4/AC-5/AC-7

Each mutation was applied, the named test run, and the mutation removed. The unit
mutations ran in the working tree; the `.ci` mutations ran in a `git archive HEAD`
copy, because another session held the tree red in `wireu` and its two call sites.

| Mutation applied | Test | Observed red |
|------------------|------|--------------|
| `ac.RouteReflectorClient, bc.RouteReflectorClient = false, false` in `peerSettingsEqual` (the original hand-maintained omission, for one field) | `TestReloadRouteReflectorClientReachesForwarding` | `Should be true` -- `the egress datapath must see the new route-reflector-client value after reload`, plus `"1" is not greater than "1"`: the peer was never reconciled at all |
| `ac.ImportFilters, bc.ImportFilters = nil, nil` in `peerSettingsEqual` | `TestReloadImportPolicyDisplayMatchesDatapath` | `expected: ["policy:new-import"]`, `actual: ["policy:old-import"]` on BOTH the display and the datapath read |
| `if a != b { return false }` in `peerSettingsEqual` (identity instead of value) | `TestPeerDiffCountIsZeroForAnIdenticalReload` | `expected: 0`, `actual: 2` |
| `peer.ImportFilters()` replaced by `peer.settings.ImportFilters` in `runIngressPolicyChain` (`filter_ordered.go`) | `TestReloadWhileReceivingNoRace` | `WARNING: DATA RACE` -- read in `runIngressPolicyChain` against a write in `hotSwappableSettings` from `applyHotSwappableSettings` |
| `ac.ImportFilters, bc.ImportFilters = nil, nil` in `peerSettingsEqual` | `reload-import-policy-applies.ci` | `ZE-OBSERVER-FAIL: the reloaded import chain never reached the running peer: ["bgp-filter-prefix:ALLOW"]` |
| the reloaded config's DENY list changed to `action accept` (fixture) | `reload-import-policy-applies.ci` | `ZE-OBSERVER-FAIL: the DENY chain must reject the route after the reload, got accept`. This one separates the two assertions: the chain was still DELIVERED (the poll passed) and only the evaluation changed |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - this spec adds no numeric input | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-import-policy-applies` | `test/reload/reload-import-policy-applies.ci` | policy edit + reload is actually ENFORCED: after the reload the same route is rejected by the edited peer and accepted by a control peer whose chain no edit touched | PASS 2026-08-12 |
| `reload-import-policy-no-bounce` | `test/reload/reload-import-policy-no-bounce.ci` | the edit is delivered by a swap, not a bounce | PASS (2026-08-08) |
| `reload-no-change` | `test/reload/reload-no-change.ci` | unchanged config does not flap sessions | PASS 2026-08-12 |
| `reload-capability-restart-names-both-fields` | `test/reload/reload-capability-restart-names-both-fields.ci` | a capability-block edit restarts the peer and the log names EVERY field that forced it, not just the first | PASS 2026-08-07 |

The swap branch of D-6 has no functional test, and D-8 says why: no config file can
express a capability-only edit today. It is proved instead through a real `Reload()`
over a peer with a live negotiated session, in `peer_settings_negotiation_test.go`.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `NN-policy-reload-bird` | `test/interop/scenarios/` | BIRD | if the design uses ROUTE_REFRESH: the peer honors the refresh and re-sends, and the new policy applies without a session reset | |
<!-- Required only if the chosen design sends ROUTE_REFRESH (RFC 2918). If the design is
     bounce-only, justify skipping: no new wire behavior is introduced. -->

### Future (if deferring any tests)
- The filter-CONTENTS facet (R-4) may split into its own spec once verified. Requires user approval.

## Files to Modify
- `internal/component/bgp/reactor/reactor_api.go` - `peerSettingsEqual` and, depending on the design, an apply branch in `reconcilePeersJournaled`.
- `internal/component/bgp/reactor/peer.go` - only if the design needs an atomic settings swap.
- `internal/component/bgp/reactor/reactor_api_test.go` - unit tests above.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | [ ] No - this spec adds no config surface; it makes existing surface work |
| CLI commands/flags | [ ] No |
| Functional test for new RPC/API | [x] | `test/reload/*.ci` |
| Prometheus counters/metrics | [ ] | consider a counter for "peers reconciled on reload" if the design warrants |
| Doctor check for runtime dependencies | [ ] No - no new runtime dependency |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | [ ] No - syntax unchanged; only whether it takes effect |
| 12 | Internal architecture changed? | [x] | `docs/architecture/core-design.md` - reload/reconcile semantics |
| 16 | Changed files referenced by doc source anchors? | [ ] | grep `docs/` for `source: .*reactor_api.go` and update stale claims |
| 11 | Affects daemon comparison? | [ ] | only if soft reconfiguration is chosen (BIRD parity) |

## Files to Create
- `test/reload/reload-import-policy-applies.ci` - CREATED 2026-08-12.
- `internal/component/bgp/reactor/peer_settings_reload_race_test.go` - CREATED 2026-08-12.
- `plan/learned/NNN-bgp-peer-settings-reload-ignored.md` - learned summary at closure.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Current Behavior (already verified at producers - do not re-derive) |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |
| 13. /ze-review gate | Review Gate section |

### Implementation Phases

**Phase 1 (Decision) is COMPLETE**: user chose soft reconfiguration (D-1) on 2026-07-16. Per D-2 it
is a superset of the refresh option, so the work phases as A (defect fix) then B (BIRD parity).

### PHASE A - stop the silent mis-enforcement (shippable on its own; does NOT close this spec)

A1. **Failing tests** - unit tests proving the guard misses each field today.
   - Tests: `TestPeerSettingsEqualDetectsImportFilterChange`, `TestPeerSettingsEqualDetectsEachSignificantField`
   - Files: `reactor/reactor_api_test.go`
   - Verify: they FAIL against current code (the bug, reproduced).
A2. **Fix the guard** - `peerSettingsEqual` covers every functionally significant field, preferring
   a mechanism that fails CLOSED for fields it does not know (A-2, AC-6).
   - Tests: as above, plus `TestPeerSettingsEqualIdenticalIsEqual` (no over-trigger, R-2)
   - Files: `reactor/reactor_api.go`
   - Verify: unit tests pass; a no-change reload bounces nothing.
A3. **Settings swap** - get the new `*PeerSettings` onto the running peer without a data race (R-1).
   - Tests: `TestReloadWhileReceivingNoRace` under `-race`
   - Files: `reactor/peer.go`, `reactor/reactor_api.go` (apply branch)
   - Verify: AC-4, AC-7. `filter_ordered.go` observes the new chain.
   - → Constraint: `peer.settings` is read unlocked from the session goroutine; the swap must be an
     atomic pointer or copy-under-lock. Cross-check `policy_dryrun.go`, which already snapshots
     under `r.mu` for this exact hazard.
A4. **Apply via refresh** - issue `SoftClearPeer` (`reactor_api.go`) after the swap so the peer
   re-sends and the new chain runs. Existing machinery; this phase only calls it.
   - Tests: `test/reload/bgp-import-policy-applies.ci`, `test/reload/bgp-reload-noop-no-bounce.ci`
   - Verify: AC-1, AC-2, AC-3, AC-5 end-to-end.
   - → Constraint: Phase B replaces this call with a local replay. Keep the call site isolated so B
     swaps the mechanism, not the surrounding logic.

### PHASE B - real BIRD parity: no restart unless unavoidable (delivers D-1b)

> **REWRITTEN 2026-07-16.** The previous Phase B built a pre-policy store. That was D-1's
> mislabel (Cisco's mechanism under BIRD's name) and is **cancelled** — no store is needed, and
> A3's atomic swap already does the load-bearing work. See D-1b / D-4 / D-5.

B1. **DONE 2026-08-07. Thomas answered with a procedure, not a categorisation.**
   The ruling is quoted verbatim in "B1 ANSWERED" at the top of this file, and the
   implementation is D-6. Read those two before reading the candidate lists below,
   which were written when a per-field answer was still expected and are kept only
   as a record of the question. The procedure supersedes them: it runs the
   negotiation rather than consulting a list, so a capability the peer does not use
   swaps whatever list it appears on, and one the peer does use restarts.
   The original wording follows.
   ~~Categorise every `PeerSettings` field (BLOCKING - present to user before coding)~~ into:
   - **FSM-relevant → restart** (BIRD: "If any FSM-relevant field changes (ASN, authentication
     type, hold time, etc.) ... the protocol framework tears down and restarts the session",
     `bird-bgp-reference.md`). Candidates: `LocalAS`, `PeerAS`, `RouterID`, `Address`,
     `Port`, `Connection`, `MD5Key`/`MD5IP`, hold/keepalive timers, `Capabilities`,
     `RequiredFamilies`.
   - **Hot-swappable → atomic swap, session survives**. Candidates: `ImportFilters`,
     `ExportFilters`, `RouteReflectorClient`, `ClusterID`, `ASOverride`, `NextHopMode`,
     the prefix-limit maps, `DefaultOriginate`, `SendCommunity`, the loop-detection fields.
   - ~~**Cosmetic → no action**. `PrefixUpdated` (display-only staleness marker,
     `peersettings.go`).~~ **CORRECTED 2026-08-07: `PrefixUpdated` is hot-swappable,
     and "no action" was the defect rather than the classification.**
     → Decision: the field is not display-only. It drives two operator surfaces,
     both published from the dates alone: the prefix-stale report bus warning
     (`ze show warnings`, the login banner) and the `ze_bgp_prefix_stale` gauge
     (`RaisePrefixStale` and `setPrefixStaleMetric`, `session_prefix.go`). Both were
     published only from `Reactor.AddPeer` (`reactor_peers.go`), so a PeeringDB
     refresh that bumped only the dates left the alarm raised until the daemon
     restarted. "Cosmetic → no action" would have kept that defect and called it
     intended. It is now classified swappable in `hotSwappableSettings`
     (`peer_settings_apply.go`) and republished on the swap by
     `Peer.refreshPrefixStale`, so the session never restarts for it.
     → Constraint: this closes ONE field, on the authority of a later review-gate
     row (`plan/deferrals/fixit-bgp-per-family-prefix-enforcement.md`, 2026-08-04)
     that named the three edits. It does NOT answer B1. The remaining ~45 fields are
     still uncategorised and this spec stays `blocked` on Thomas.
   → Constraint: the categorisation MUST fail closed (A-2/AC-6) — an unclassified field is
   treated as FSM-relevant (restart) rather than silently ignored. Restarting on an unknown
   field is conservative and visible; ignoring it is the bug this spec exists to fix.
   → Constraint: A2's `reflect.DeepEqual` guard answers "did anything change?" but NOT "which
   category?". It stays as the fail-closed backstop and gains a category split on top.
B2. **Atomic swap for hot-swappable fields** - extend A3's swap so a hot-swappable-only change
   applies WITHOUT a restart (`bird-bgp-reference.md`: "filters are swapped atomically").
   Drop A4's `SoftClearPeer` from this path; the new chain applies to subsequently received routes.
B3. **Log the category that forced a restart** - the repo's own explicit recommendation
   (`bird-bgp-reference.md`): "log which category of change caused a restart when one
   happens. BIRD produces no log at all for a successful soft reconfigure, which is excellent for
   ops teams watching dashboards." Mirror that: silent on hot swap, one clear line on restart.
B4. **Fix the false doc claim** - `docs/research/bird-bgp-reference.md` asserts "ze's filter
   reload path already handles the common case without bouncing sessions." That is FALSE today and
   is plausibly why this defect went unnoticed. It becomes true only when B2 lands; correct the
   line either way (checklist #16).
B5. **Interop** - `NN-policy-reload-bird`: change an import filter on both Ze and BIRD, reload
   both, and assert neither session resets and both enforce the new policy.

**Scope note:** re-applying a changed policy to ALREADY-RECEIVED routes is NOT in this spec.
BIRD does not do it either on reconfigure (`nest/proto.c:843` carries an open
`/* FIXME: better handle these changes, also handle in_keep_filtered */`). If that is wanted, it
needs a route refresh (ask the peer to re-send) or Cisco-style retention — a separate spec with
its own justification. Do not smuggle it in here.

### CLOSING

- **Full verification** -> `make ze-verify-changed` (other sessions have uncommitted work).
- **Complete spec** -> audit tables, learned summary, two-commit closure. Then revisit
  `spec-bgp-filtered-route-storage` per D-4: with a pre-policy store, `routes_filtered` is a query
  over it, so that spec is superseded rather than merely unblocked. Do not leave it `blocked`
  pointing at a spec that no longer describes the design.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has file:line implementation |
| Correctness | The guard covers every functionally significant field; capability changes still bounce |
| Fail-closed | A newly added `PeerSettings` field is not silently ignored (`ai/rules/evidence.md`, AC-6) |
| Data flow | The apply path runs through reconcile, not a side channel |
| Concurrency | No unlocked cross-goroutine write to `peer.settings` (R-1); check against `policy_dryrun.go` |
| Rule: no-workarounds | Do not "fix" this by editing the misleading doc comment at `:778-779`; fix the predicate (`ai/rules/completion.md`) |
| Over-triggering | A no-change reload bounces nothing (R-2, AC-3) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Guard covers every significant field | `go test -run TestPeerSettingsEqualDetectsEachSignificantField` |
| Policy actually applies | run `test/reload/bgp-import-policy-applies.ci` |
| No gratuitous bounce | run `test/reload/bgp-reload-noop-no-bounce.ci` |
| No race | `go test -race -run TestReloadWhileReceivingNoRace` |
| Blocked spec unblocked | `grep -n "Status\|Depends" plan/spec-bgp-filtered-route-storage.md` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Policy enforcement integrity | This defect IS a security-relevant failure: an operator tightening policy to block a prefix believes it applied while the old permissive chain runs. Verify the fix closes it for BOTH tightening and loosening |
| Race conditions / TOCTOU | R-1: settings swap vs unlocked hot-path read (`filter_ordered.go`) |
| Resource exhaustion | A fix that bounces peers on every reload is a self-inflicted DoS on a large fleet (R-2) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Reload bounces peers that did not change | over-triggering in the guard; narrow the comparison |
| Policy still stale after reload | the apply mechanism is not wired; re-check `peer.settings` is actually swapped |
| Race detector fires | R-1; use an atomic pointer swap |
| 3 fix attempts fail | STOP, report, ask user |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| (`spec-bgp-filtered-route-storage` A-5) A new per-peer leaf takes effect on reload | `peerSettingsEqual` omits it, so it silently no-ops | Audit read of `reactor_api.go` | Spawned this spec; blocked the other |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| A hand-maintained equality/field list silently drops new fields | Unknown - audit other reconcile/diff predicates | Possibly extend `ai/rules/evidence.md` with a "no hand-maintained field lists in guards" clause | Decide at closure |

## Design Insights
<!-- LIVE — write IMMEDIATELY when you learn something -->
- The defect is invisible because the DISPLAY path and the ENFORCEMENT path read different sources:
  `bgp peer <ip>` shows `import-policy` from the re-parsed config (`plugins/cmd/peer/peer.go`)
  while the datapath reads the stale `peer.settings` (`filter_ordered.go`). Any fix must make
  these two agree (AC-5); a fix that only corrects enforcement still leaves the two paths
  independently derived and free to diverge again.
- ~~"Option 3 and `spec-bgp-filtered-route-storage` want the SAME thing: a pre-policy Adj-RIB-In.
  BIRD's `import keep filtered` and its soft reconfiguration are two views of one store."~~
  **RETRACTED 2026-07-16 — fabricated, and the most expensive error in this spec's history.** It
  was written here with no citation, then found by a later search and reported back as though it
  were repo evidence, then used to supersede a sibling spec and to justify a user decision. BIRD
  v2.19.0 disproves every clause: `import keep filtered` retains ONLY the rejected copy behind a
  `REF_FILTERED` flag in the main table (`nest/route.h:274`, `nest/rt-table.c:1687-1697`);
  `import table` is a SEPARATE knob (`nest/config.Y:718-720`); BIRD's "soft reconfig" is an atomic
  filter-pointer swap that stores nothing (`bird-bgp-reference.md`). Three mechanisms,
  not one store.
  → Lesson: an uncited claim written into a spec becomes indistinguishable from evidence within a
  single session. `ai/rules/evidence.md` says cite the producer; this is WHY. The cost is not
  merely being wrong — it is that being wrong gets laundered into a citation and then decided upon.
- **The real insight the fabrication displaced: this defect needs no store, because it is not a
  data problem.** The new chain is computed correctly on every reload — `config/peers.go`,
  `ps.ImportFilters = concatFilters(bgpImport, groupImport, peerImport)` (re-read 2026-07-16). It
  is simply never delivered: `peerSettingsEqual` reports "no change", and `settings` is assigned
  once in the `Peer` literal (`peer.go`, re-read 2026-07-16) with no setter. The fix is to
  deliver a pointer, not to archive routes. BIRD reaches the same conclusion — swap the pointer
  atomically, keep the session.
- **BIRD parity gives less than the name suggests.** Even at full parity a policy change applies
  only to routes received AFTER the swap; BIRD does not re-run policy over already-imported routes
  on reconfigure (`nest/proto.c:843`: "FIXME: better handle these changes, also handle
  in_keep_filtered"). Re-applying to existing routes is a separate capability needing a refresh or
  retention. Do not promise it under the "parity" label.

## Key Design Decisions
| ID | Decision | Alternatives Considered | Rationale |
|----|----------|------------------------|-----------|
| D-1 | ~~**Soft reconfiguration (BIRD parity)** is the target design~~ **CORRECTED 2026-07-16 — see D-1b.** The option was presented under the wrong daemon's name: its description ("retain a pre-policy Adj-RIB-In, re-run policy over stored routes") is CISCO's `soft-reconfiguration inbound`, not BIRD's. The user's decision was taken on that mislabel. | Bounce; In-place + ROUTE_REFRESH | Superseded by D-1b. |
| D-1b | **Real BIRD parity: compare-then-act with field CATEGORIES.** Compare old vs new per-peer config; if only non-session-affecting fields changed, **swap them atomically on the live session** and the session survives; if an FSM-relevant field changed (ASN, auth, hold time, capabilities), restart. **Log the category that forced a restart.** No new store anywhere. | Cisco-style pre-policy store (D-1's mislabel); bounce-on-any-change | User sign-off 2026-07-16 (third gate, after the mislabel was found). This is BIRD's actual `bgp_reconfigure` (`docs/research/bird-bgp-reference.md`: "the session continues, filters are swapped atomically, and running FSM state survives"; "no restart unless unavoidable"). It is ALSO what the repo already told ze to do: `:1615-1618` — "ze should explicitly compare old and new per-peer config for the fields that matter and log which category of change caused a restart when one happens." Far smaller than a pre-policy store, and it needs no retention. |
| D-2 | Soft reconfiguration is a **strict superset** of the in-place+refresh option, NOT an alternative to it | - | Verified reasoning: re-running policy over stored routes uses `peer.settings.ImportFilters` (`filter_ordered.go`). If the guard still discards the new settings, soft reconfig re-runs the OLD policy and the defect survives. The guard fix AND the settings swap are unavoidable in every option. |
| D-3 | Phase the work: **Phase A** (guard + swap + refresh-based apply) ships the defect fix; **Phase B** (pre-policy store + local re-run) delivers BIRD parity and replaces the refresh | Build Phase B only, shipping nothing until it lands | The defect is LIVE and silent today: operators are mis-enforcing policy right now. Phase A is small, complete on its own, and its work is not throwaway — the guard fix and settings swap are Phase B prerequisites (D-2), and `SoftClearPeer` already exists (`reactor_api.go`) so Phase A only calls it. Phase B then swaps the re-obtain mechanism from "ask the peer" to "replay the local store". |
| D-4 | ~~`spec-bgp-filtered-route-storage` is **superseded**~~ **WITHDRAWN 2026-07-16. It is NOT superseded.** It is an independent spec that merely DEPENDS on this one for its AC-3. | - | D-4 rested on the fabricated "pre-policy store subsumes keep-filtered" claim. **Disproven at BIRD v2.19.0 source:** `import keep filtered` retains only the rejected copy behind a `REF_FILTERED` flag in the main table (`nest/route.h:274`, `nest/rt-table.c:1687-1697`), and `import table` is a SEPARATE knob (`nest/config.Y:718-720`). Neither is a pre-policy store, so nothing here subsumes that spec. It has been rewritten around BIRD's real model; do not re-supersede it. |
| D-6 | **Swap-or-restart for the capability set is decided by RUNNING the negotiation, not by classifying fields.** `negotiationOutcomeUnchanged` (`peer_settings_negotiation.go`) takes the OPEN ze really sent (`Session.localOpen`) as the baseline, builds the candidate OPEN from the new settings through `buildOpen` (the same producer `sendOpen` uses), and negotiates both against the capabilities the peer really advertised (`Session.peerOpen`). The two results are compared on the derived sub-components of `capability.Negotiated`, which cover the families, the encoding and the session capabilities. Equal means the running session is already what the new config asks for, so `Capabilities` is delivered and the session is left alone; unequal means restart. | a per-field allow-list; comparing the OPEN bytes only | Thomas's ruling of 2026-08-07, quoted verbatim above. An allow-list cannot express it: the SAME capability edit swaps against one peer and restarts against another, because the verdict depends on what the peer advertised. Comparing OPEN bytes alone is the special case where nothing changed at all, and it would restart for exactly the two examples the ruling exists to allow. `Mismatches` is excluded from the comparison because removing a capability the peer never had removes its mismatch entry, which would make the ruling's first example always compare unequal. |
| D-7 | **The two swap categories stay separate, and the copier travels with the decision.** `hotSwappableSettings` holds the fields a running session re-reads or republishes (`ImportFilters`, `ExportFilters`, `PrefixUpdated`). `negotiatedCapabilitySettings` holds `Capabilities`, and applies only when D-6 proved the outcome unchanged. `peerSettingsSwapPlan` returns the copier it neutralized with, and `applyHotSwappableSettings` applies that same function. | one merged list; a boolean flag at the apply site | The two categories qualify for different reasons, and merging them would make `Capabilities` unconditionally swappable, which is the wrong direction of the ruling. Returning the copier keeps the invariant the original defect lacked: the set neutralized when deciding IS the set delivered, so no field can be judged swappable and then discarded. |
| D-8 | **The safety direction is restart, and `CapabilityConfigJSON` and `RawCapabilityConfig` are on that side.** They carry the peer's capability block to plugins, and a plugin's capabilities enter the OPEN through `pluginCapGetter`, which reads the injector store the plugin has already written. The probe builds its candidate OPEN from that same store, so it sees what the plugin holds now and cannot see what the plugin would inject after receiving the new config. | neutralizing them with `Capabilities`, since one config edit writes all three | An edit whose wire effect arrives later, by a path the probe does not run, is exactly what the procedure cannot determine (`ai/rules/evidence.md`). A wrong restart costs one reconverge and announces itself. **Consequence, and it is a finding rather than a design goal: no `capability { }` block edit can reach the swap branch today, because `parseCapabilitiesFromTree` rewrites `CapabilityConfigJSON` from the same input. A `family { }` edit cannot either, because every non-disabled family carries a mandatory prefix maximum (`config_prefix.go`) and so moves `PrefixMaximum` too. The procedure is therefore correct and currently unreachable from a config file. Closing that needs a decision about those fields, which is a new question for Thomas and not one this ruling answers.** |
| D-5 | This spec builds **no store of any kind** | pre-policy retention (D-1's mislabel) | Applying a changed policy needs the config to reach the peer, not a route archive. BIRD achieves it with an atomic pointer swap and no retention. A store would be a large, memory-expensive answer to a question that is actually about a stale pointer (`peer.go`, no setter). |

**Phase A is NOT "done" on its own** (`ai/rules/completion.md`): it closes AC-1..AC-7 of
this spec (the defect), but D-1's BIRD parity is delivered only by Phase B. Do not close this spec
at the end of Phase A.

## Known Limitations
- The filter-CONTENTS facet (R-4) is unverified and may be a separate defect.
- This spec fixes whether settings APPLY; it does not add soft reconfiguration.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete - every row has a concrete test name
- [ ] `/ze-review` gate clean
- [ ] `make ze-test` passes (or `make ze-verify-changed` when scoped, with rationale: other sessions hold uncommitted work in this tree)
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] `spec-bgp-filtered-route-storage` unblocked at closure

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed (the hand-maintained-list rule candidate)

### TDD
<!-- NEVER tick [ ] — evidence goes below the box, not in it. -->
- [ ] Tests written — DONE (A1): `internal/component/bgp/reactor/peer_settings_reload_test.go`
- [ ] Tests FAIL (paste output) — DONE (A1), the bug reproduced across 11 fields
- [ ] Tests PASS (paste output) — DONE (A2)
- [ ] Functional tests for end-to-end behavior — DONE 2026-08-12: `test/reload/reload-import-policy-applies.ci` PASS in 3.6s
- [ ] `-race` test for R-1 — DONE 2026-08-12: `TestReloadWhileReceivingNoRace`, `make ze-race-reactor` green (408s)

**AC coverage as of 2026-08-12.** AC-1 through AC-7 each have code and a test, and
each test was proved to discriminate (Round-2 table above).

| AC | Evidence |
|----|----------|
| AC-1 | `test/reload/reload-import-policy-applies.ci`; `TestReloadImportPolicyKeepsTheSamePeer` |
| AC-2 | `TestPeerDiffCountCountsSwapAsOneChange` |
| AC-3 | `TestPeerDiffCountIsZeroForAnIdenticalReload`; `test/reload/reload-no-change.ci` |
| AC-4 | `TestReloadRouteReflectorClientReachesForwarding` |
| AC-5 | `TestReloadImportPolicyDisplayMatchesDatapath`; the `.ci` polls the DISPLAY and asserts on the DATAPATH evaluation |
| AC-6 | `TestPeerSettingsRestartRequiredIsFailClosed` |
| AC-7 | `TestReloadWhileReceivingNoRace` under `make ze-race-reactor` |

**B4 needs no work: the false claim was already corrected.**
`docs/research/bird-bgp-reference.md` (Section "For ze") now states that the reload
path did NOT handle the common case, names `peerSettingsSwapPlan` as the
compare-then-act answer, and records what ze still lacks against BIRD. Read
2026-08-12; the correction is dated 2026-08-08 in the text itself.

**Still open on this spec, and NOT part of the 2026-08-12 package:** B5, the
`NN-policy-reload-bird` interop scenario.

**Found while doing this, recorded and not fixed** (`ai/rules/completion.md`):
- `reconcilePeersJournaled` removes every established DYNAMIC peer on reload,
  because a runtime-created peer is not in the config-derived list it diffs
  against. Measured with a probe, then recorded in
  `plan/journal/gate-excludes-part-of-its-population.md`. It also gates the race in
  `plan/deferrals/fixit-dynamic-peer-settings-unlocked-read.md`: that race needs
  `applyHotSwappableSettings` to run on a dynamic peer, which cannot happen while
  the peer is removed instead of swapped. One decision answers both.
- `test/reload/mgmt-guard-reload-refuses-nonloopback.ci` is red at HEAD, unrelated
  to this spec. Recorded in `plan/journal/silent-fall-through.md`.

**Fixed on the spot, because it blocked this spec's own test**
(`ai/rules/completion.md`): `option=linger` did nothing when a peer script ended on
a `sighup` or `sigterm` action. Both sites in `runMessageLoop`
(`internal/test/peer/peer.go`) returned success directly instead of calling
`Peer.completed`, so the peer closed its connection 500 ms after the signal and ze
logged `session closed ... reason="connection lost"`. That made "the session
survived the reload" unassertable. `reload-import-policy-applies.ci` is the only
`.ci` in the tree that pairs `linger` with a signal action, so the change is inert
for every other test.

**A1 evidence — the defect reproduced against unmodified code (2026-07-16):**
```
$ go test ./internal/component/bgp/reactor/ -run 'TestPeerSettingsEqual' -count=1
--- FAIL: TestPeerSettingsEqualDetectsImportFilterChange (0.00s)
        Error: Should be false
        Messages: an import filter chain change must mark the peer changed
--- FAIL: TestPeerSettingsEqualDetectsEachSignificantField (0.00s)
    --- FAIL: .../ImportFilters          (governs inbound policy at filter_ordered.go:138)
    --- FAIL: .../ExportFilters          (governs outbound policy)
    --- FAIL: .../RouteReflectorClient   (RFC 4456; read at peer_forward_facts.go:110)
    --- FAIL: .../ClusterID              (RFC 4456 Section 7)
    --- FAIL: .../ASOverride             (rewrites outbound AS_PATH)
    --- FAIL: .../RSClient               (RFC 7947 Section 2.2.2)
    --- FAIL: .../RSFastPath             (selects reactor-native forwarding)
    --- FAIL: .../AcceptSRv6PrefixSID    (admits PrefixSID code 40 from EBGP)
    --- FAIL: .../NextHopMode            (next-hop rewriting)
    --- FAIL: .../MD5Key                 (RFC 2385 TCP-MD5 key rotation — security)
    --- FAIL: .../LinkLocal              (RFC 2545 Section 3 MP_REACH next-hop)
FAIL	github.com/ze-software/ze/internal/component/bgp/reactor	0.433s
```
`TestPeerSettingsEqualIdenticalIsEqual` PASSED here, establishing the no-over-trigger baseline
BEFORE the fix (R-2).

**A2 evidence — after the fail-closed rewrite of `peerSettingsEqual`:**
```
$ go test ./internal/component/bgp/reactor/ -run 'TestPeerSettingsEqual' -count=1 -v
--- PASS: TestPeerSettingsEqualIdenticalIsEqual (0.00s)
--- PASS: TestPeerSettingsEqualDetectsImportFilterChange (0.00s)
--- PASS: TestPeerSettingsEqualDetectsEachSignificantField (0.00s)   [11/11 subtests]
--- PASS: TestPeerSettingsEqual (0.00s)                 <- PRE-EXISTING, still green
--- PASS: TestPeerSettingsEqualCapabilityChange (0.00s) <- PRE-EXISTING, still green
ok  	github.com/ze-software/ze/internal/component/bgp/reactor	0.489s

$ go test ./internal/component/bgp/reactor/ -count=1
ok  	github.com/ze-software/ze/internal/component/bgp/reactor	3.889s   <- whole package, no regressions
```

### A2 finding: the omission is far larger than the audit first reported
`peerSettingsEqual` compared ~15 of PeerSettings' ~50 fields. Also silently ignored, beyond the 11
under test: `PrefixMaximum`/`PrefixWarning` (prefix limits), `PrefixTeardown`, `PrefixIdleTimeout`,
`DefaultOriginate`, `DefaultOriginateFilter`, `SendCommunity`, `LoopAllowOwnAS`, `LoopClusterID`,
`LoopDisabled`, `LocalASNoPrepend`, `LocalASReplaceAS`, `NextHopAddress`, `MD5IP`, `GlobalLocalAS`,
`BFD`, `RequiredFamilies`, `IgnoreFamilies`, `RequiredCapabilities`, `RefusedCapabilities`,
`ProcessBindings`, `PluginRoutes`. `StaticRoutes` was compared by LENGTH ONLY, so editing a
static route's contents was also silently dropped.
→ Decision: A-2 is **confirmed** — a hand-maintained list is unmaintainable here. The fix compares
the whole struct with `reflect.DeepEqual`, excluding only `Capabilities` (semantic wire compare,
preserved) and `PrefixUpdated` (display-only PeeringDB staleness marker, `peersettings.go`).
Verified prerequisite: PeerSettings holds no locks/funcs/channels, so the struct copy and DeepEqual
are well-defined.

### ⚠ A2 MUST NOT SHIP ALONE (read before committing)
The fix is strictly BROADER than the old predicate, so many edits that previously (incorrectly) did
nothing now mark the peer changed — and `reconcilePeersJournaled` applies a change by remove+re-add
(`reactor_api.go`), i.e. **a session bounce**. Editing a static route or a prefix limit would
now flap the session. That is correct-but-disruptive and is NOT the mechanism the user chose (D-1).
→ Constraint: A3 (settings swap) + A4 (apply without reset) are REQUIRED before this is shippable.
Do not commit A2 as a standalone fix without re-consulting the user (`ai/rules/completion.md`).

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Minimal coupling
