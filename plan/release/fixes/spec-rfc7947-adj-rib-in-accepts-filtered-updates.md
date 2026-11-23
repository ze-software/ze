# Spec: rfc7947-adj-rib-in-accepts-filtered-updates

| Field | Value |
|-------|-------|
| Status | verification |
| Priority | P1 |
| Release reason | A route server must retain the required pre-policy received population independently of policy rejection. |
| Scope | protocol |
| Depends | - |
| Phase | Closure preparation: functional tests, interop proof, metrics, docs, and review gate |
| Handoff | - |
| Updated | 2026-11-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

RFC 7947 Section 2.1 opens with an obligation Ze does not meet:

> "A route server MUST accept all UPDATE messages received from each of its
> clients for inclusion in its Adj-RIB-In. These UPDATE messages MAY be omitted
> from the route server's Loc-RIB or Loc-RIBs, due to filters configured for the
> purpose of implementing routing policy."

`notifyOneMessage` (`internal/component/bgp/reactor/reactor_notify.go`) runs
the ordered ingress step loop and returns as soon as a step denies the route. That
return sits AHEAD of the plugin dispatch that feeds `handleReceivedStructured`
(`internal/component/bgp/plugins/adj_rib_in/rib.go`) and ahead of
`reactorForwardRS`. Furthermore, for mixed UPDATEs (carrying withdrawals plus
denied announcements), `filter_ordered.go::ingressPolicyWithdrawals` returns
`accept=true` with a withdrawal-only `modifiedPayload`, discarding the denied
announcements before dispatch, and a denied replacement of a previously accepted
route leaves the stale accepted route selected in Loc-RIB and Adj-RIB-Out.

The path is reachable by DEFAULT, not only by operator configuration. `LoopIngress`
(`internal/component/bgp/reactor/filter/loop.go`) denies on the local AS appearing
in AS_PATH, on the local Router ID in ORIGINATOR_ID, and on the local Router ID in
CLUSTER_LIST. The per-peer import chain (`runIngressPolicyChain`) denies on
operator policy (including absent EBGP import policy under RFC 8212).

Thomas ruled on 2026-08-30 that Ze inserts into the Adj-RIB-In FIRST, before the
filter verdict is honored, for route-server client peers, marking the stored entry
filtered. On 2026-11-22 he approved revising this spec to match the live codebase:
1. Keep `RFC7947-x-4` (`[SHOULD]`) and `RFC7947-x-5` (`[SHOULD]`, already proven in
   both polarities) untouched in `rfc/short/rfc7947.md`; `RFC7947-2.1-1` and
   `rfc/extraction/rfc7947.json` already exist in the tree.
2. Capture original pre-policy UPDATE bytes and policy disposition before
   `ingressPolicyWithdrawals` projection, and withdraw previously accepted routes
   from Loc-RIB / Adj-RIB-Out when replaced by a denied UPDATE.
3. Store an independent `ImportFiltered` mark (separate from RPKI `Ineligible`)
   on `RawRoute` and gate `walkReplayRoutes`, `replayPathCommand`, and
   `routeEligible`.
4. Enforce an explicit unique-retained-population bound (replacing the disproven
   offered-count assumption).
5. Split the orthogonal 2026-11-16 selected-path replay ordering proposal into
   `plan/release/fixes/spec-bgp-selected-path-replay-ordering.md`.

### Goals

| # | Goal |
|---|------|
| G-1 | A route-server client's denied UPDATE (full denial, mixed-UPDATE denied announcement, or denied replacement) is held in that peer's Adj-RIB-In, marked import-filtered, so Ze satisfies RFC 7947 Section 2.1 |
| G-2 | An import-filtered entry is never selected or re-advertised (live rail, bulk/paged replay, single-path replay, or after RPKI validation acceptance), and a denied replacement withdraws any previously accepted version from Loc-RIB and recipients |
| G-3 | An operator can tell a filtered entry from an accepted one at the CLI, on every pipe rendering |
| G-4 | Retained entries (accepted + import-filtered) are bounded by unique retained keys against the configured prefix maximum, unaffected by unmatched withdrawals |
| G-5 | `RFC7947-2.1-1` leaves `{gap}` and passes `./le rfc check` with positive and negative discrimination proofs |

### Non-goals

| Item | Why |
|------|-----|
| A pre-policy Adj-RIB-In for a non-route-server peer | RFC 7947 Section 2.1 binds a route server. Key Design Decisions states what the wider scope would cost |
| BMP Adj-RIB-In Pre-Policy monitoring (RFC 7854 Section 5) | A separate obligation under a separate RFC, and bgp-bmp is a separate subscriber |
| Path-hiding mitigation (multiple Loc-RIBs, add-path) | RFC 7947 Section 2.3 is non-normative; `RFC7947-x-5` is already `[SHOULD]` and untouched |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/plugin/rib-storage-design.md` - the Adj-RIB-In raw hex store, declared by `rib.go` and `rib_commands.go`
  → Constraint: the store is a per-source-peer map of sequence maps holding raw wire hex (`AttrHex`, `NHopHex`, `NLRIHex`), never a decoded route. A filtered mark is a field on `RawRoute`, never a second map.
  → Decision: the sequence number lives in the seqmap, not in `RawRoute`, so a filtered entry consumes a sequence number like any other and the `Since` walk still visits it. The skip therefore has to be in the WALK, not in the numbering.
- [ ] `docs/architecture/api/process-protocol.md` - the event namespace and type registry, declared by `subscribe.go`, `events.go`, `bridge.go` and `replay_cut.go`
  → Constraint: `parseOneReceiveFlag` (`internal/component/bgp/reactor/config.go`) and `ReceiveEventValidator` (`internal/component/config/validators.go`) both resolve a `receive` token against the bgp event registry at call time. A new event type registered in that namespace therefore reaches the subscription grammar, the YANG validator and completion with no edit to any of the three.
  → Decision: delivery is per-subscription (`SubscriptionManager.getMatching`), so an event type nobody subscribes to reaches nobody. That is what makes a distinct event type fail-closed and a boolean on the existing `update` event fail-open.
- [ ] `docs/architecture/api/ipc_protocol.md` - the typed RPC enums, declared by `pkg/plugin/rpc/enums.go`
  → Constraint: `EventKind` is a small unsigned integer with a fixed-size string table and a `MarshalText`/`UnmarshalText` pair. A new kind takes the next ordinal, a wire string, a table row, an `UnmarshalText` case, and `EventKindCount` moves.
- [ ] `docs/architecture/core-design.md` - the message receiver dispatch, declared by `reactor_notify.go`, `session_read.go`, `config.go` and `internal/component/bgp/plugin/register.go`
  → Constraint: the ingress step loop is the ONE stage-ordered pipeline; step order is a declared Stage, not code position. Each `orderedIngressStep` carries a `name` (`filter_ordered.go`), which is the operator-visible reason a route was denied and needs no new plumbing.
  → Decision: `checkPrefixLimits` (`session_prefix.go`) runs BEFORE `notifyMessageReceiver`, but default offered-mode subtraction decrements on unmatched withdrawals without checking store membership. For `rs-client true` peers in offered mode, `prefixCounts.retained` tracks unique retained keys alongside the offered tally so unmatched withdrawals cannot grow retained entries past `maximum` (AC-15).
- [ ] `docs/architecture/testing/interop.md` - BGP interop scenario discovery, native checker registration, and forced-RED validation, declared by `internal/le/interoplab/bgp/check_rfc7947_filtered.go`
  → Constraint: scenario directory names carry no numeric prefix, assertions live in registered Go checkers under `internal/le/interoplab/bgp/`, and protocol scenarios require a forced-RED proof against a rebuilt artifact.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7947.md` - the route server obligations; `RFC7947-x-4` and `RFC7947-x-5` are already `[SHOULD]` and proven, and `RFC7947-2.1-1` exists at `[MUST]`
  → Constraint: `RFC7947-2.1-1` is gated (`MUST`), so removing `{gap}` requires both positive and negative tagged tests with verified `./le rfc record` discrimination proofs and an independent `./le rfc audit` verdict (`enforced`).
- [ ] `rfc/full/rfc7947.txt` Section 2.1 - the authority for client UPDATE retention in Adj-RIB-In and policy omission from Loc-RIB
- [ ] `rfc/short/rfc4271.md` - the Adj-RIB-In definition the store implements
  → Constraint: RFC 4271 Section 3.2 makes the Adj-RIB-In the unprocessed information a peer advertised. RFC 7947 Section 2.1 is not in tension with it; it makes explicit for a route server what the base spec leaves to the implementation.

**Key insights:** (minimal context to resume after compaction)
- The reject return is in `reactor_notify.go`, inside the ordered ingress step loop, and it precedes BOTH the Adj-RIB-In dispatch and `reactorForwardRS`; mixed-UPDATE policy denial runs through `filter_ordered.go::ingressPolicyWithdrawals`.
- Seven plugins subscribe to `update direction received`: bgp-rib, bgp-rs, bgp-rr, bgp-rpki, bgp-rpki-decorator, bgp-bmp, bgp-adj-rib-in. A distinct event type (`update-filtered`) reaches only `bgp-adj-rib-in` and fails closed.
- The receive-token vocabulary, the subscription grammar and the YANG completion are all DERIVED from the bgp event registry, so a new event type costs one constant plus one registration argument.
- `ImportFiltered` is stored independently of RPKI `Ineligible` and gates `walkReplayRoutes`, `replayPathCommand`, `routeEligible`, and `applyToInstalled`.
- `RFC7947-x-4` and `RFC7947-x-5` are already `[SHOULD]` in `rfc/short/rfc7947.md` and remain untouched (2026-11-22 revision).

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` builds the `RawMessage`, runs the ordered ingress pipeline, caches the `ReceivedUpdate`, runs the RS fast path, then dispatches. The step loop's non-accept branch is where a denied UPDATE stops, with the comment "Route rejected by filter; don't cache or dispatch."
- [ ] `internal/component/bgp/reactor/filter_ordered.go` - `orderedIngressStep` carries `name`, `stage`, `priority`, and exactly one executor (an in-process filter func, or the policy chain flag). `ingressStepResult` carries `accept`, `modifiedPayload`, `teardown`, `notifyCode`, `notifySubcode`.
- [ ] `internal/component/bgp/reactor/filter/loop.go` - `LoopIngress` denies on the local ASN in AS_PATH, and on iBGP sessions on ORIGINATOR_ID or CLUSTER_LIST naming the local Router ID. Inert only when the peer sets `LoopDisabled`.
- [ ] `internal/component/bgp/reactor/session_read.go` - the read loop calls `checkPrefixLimits` before `notifyMessageReceiver`, so the configured prefix maximum counts offered NLRIs.
- [ ] `internal/component/bgp/reactor/session_prefix.go` - `checkPrefixLimits` collects every prefix section of a message before counting any of it, and settles the installed-versus-offered counting modes.
- [ ] `internal/component/bgp/reactor/config.go` - `ps.RSClient` and `ps.RSFastPath` come from the plugin-owned `rs-client` and `rs-fast-path` YANG leaves. `parseOneReceiveFlag` resolves an attach receive token against the bgp event registry.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - `runAdjRIBInPlugin` subscribes to `update direction received` and `state` at format `full`; `handleReceivedStructured` walks withdrawals before announces and installs through `installStructuredNLRIs` and `installComplexNLRIs`; `buildReplayRoutes` walks every source peer's seqmap and emits `rpc.StoredRoute`; `handleStructuredState` deletes a peer's whole map on down.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `status` answers `running`, `total-routes` and `peers`; `show` answers an `adj-rib-in` envelope mapping a peer to a list of route objects carrying `family`, `key`, `nhop-hex`, `attr-hex`, `nlri-hex`, `seq-index` and `validation-state`; `replayCommand` calls `buildReplayRoutes` then `relayRoutes`. `commandDecls` declares both show commands with a document shape.
- [ ] `internal/component/bgp/plugins/adj_rib_in/replay_cut.go` - `replayCut.excludes` bounds a replay by reactor MessageID; presence and value are carried separately because a cut of zero is a real cut.
- [ ] `internal/core/bgp/events/events.go` - the bgp event type constants, a leaf package with no dependencies.
- [ ] `internal/component/bgp/plugin/register.go` - registers the bgp namespace with its event types.
- [ ] `pkg/plugin/rpc/enums.go` - `EventKind` and its wire strings, twelve ordinals including the unspecified zero and the count sentinel.
- [ ] `internal/component/config/validators.go` - `ReceiveEventValidator` resolves a token against the bgp event registry and completes from the registry's own names.
- [ ] `internal/component/plugin/server/subscribe.go` - `ParseSubscription` and `SubscriptionManager.getMatching`: delivery is to the processes whose subscription matches namespace, type, direction and peer.
- [ ] `internal/le/rfc/check_ratchets.go` - `checkLevelRatchet` and `correctionAuthorizes`.
- [ ] `internal/le/rfc/check_core.go` - `evaluate` demands both polarities for every gated requirement of an enrolled RFC.
- [ ] `internal/le/rfc/summary.go` - `parseChecklistLine`, `extractSection` and `validateID`: the id is anchored to the trailing section citation.
- [ ] `internal/le/rfc/rfc.go` - the five gated levels, the six advisory levels, and the no-section anchor.
- [ ] `rfc/full/rfc7947.txt` - Sections 2.1, 2.2, 2.3 and 2.3.3.
- [ ] `rfc/short/rfc7947.md` - the eight checklist rows and the three existing Correction paragraphs.
- [ ] `test/interop/scenarios/bgp-route-server-frr/ze.conf` - the working route-server scenario: two `rs-client true` peers, each attaching the rs process with an explicit receive and send grant.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A denied UPDATE is still NOT forwarded, NOT installed in the RIB, NOT reflected, NOT validated by RPKI, and NOT reported to BMP. The filter verdict keeps every consequence it has today except the Adj-RIB-In one.
- A denied UPDATE from a peer that is not a route-server client stays absent from the Adj-RIB-In.
- `show bgp adj-rib-in` keeps its document shape, its per-peer envelope, and every existing route key.
- `status` keeps `running`, `total-routes` and `peers`, with `total-routes` counting every entry the store holds.
- `buildReplayRoutes` keeps its cursor semantics, its replay cut bound, and its rule that a peer's own routes are never replayed to it.
- The reactor still returns false from `notifyMessageReceiver` on a denial, so the cache is not populated and the pool buffer's lifetime is unchanged.
- A teardown verdict still queues the NOTIFICATION and drops the route with no Adj-RIB-In insertion: a session being torn down has no Adj-RIB-In to hold anything.

**Behavior to change:** (only what the user asked for)
- On a non-teardown denial (or mixed-UPDATE denied announcements via `ingressPolicyWithdrawals`) when the source peer has `rs-client true`, the reactor queues withdrawals for affected prefixes on the ordinary `update` rail first (clearing Loc-RIB and recipients) and then emits one `update-filtered` event carrying the original pre-policy bytes and the denying step's name, before returning false.
- `bgp-adj-rib-in` subscribes to `update-filtered direction received`, stores the routes with `ImportFiltered: true` and `FilterStep` independently of RPKI `Ineligible`, and updates `ze_adjribin_routes_filtered{peer=...}`.
- `walkReplayRoutes`, `replayPathCommand`, `routeEligible`, and `applyToInstalled` exclude `ImportFiltered` entries unconditionally.
- `session_prefix.go` tracks unique retained keys for `rs-client true` families in offered mode so unmatched withdrawals cannot exceed configured prefix maximums.
- `show bgp adj-rib-in` and `show bgp adj-rib-in status` report `filtered`, `filter-step`, and `filtered-routes`.
- `RFC7947-2.1-1` leaves `{gap}` with positive/negative tagged conformance tests, discrimination records, and an `enforced` audit verdict; `rfc/extraction/rfc7947.json` prose reflects enforcement (`RFC7947-x-4` and `RFC7947-x-5` stay untouched at `[SHOULD]`).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where | What it does today | This spec |
|-----------|-------|--------------------|-----------|
| Ordered ingress disposition | `reactor_notify.go`, `notifyOneMessage`; `filter_ordered.go`, `ingressPolicyWithdrawals` | Applies policy before event publication and can project mixed denied input into withdrawals | Extends publication at the original-input/disposition boundary; preserves ordinary policy denial |
| Received route storage and replay | `adj_rib_in/rib.go`, `walkReplayRoutes`; `rib_replay_path.go`, `replayPathCommand` | Retains accepted paths and supplies both bulk and individual replay | Extends entries with independent import-filtered state and applies it to every replay path |
| Validation eligibility | `adj_rib_in/rib_validation.go`, `applyToInstalled`; `rib_eligibility.go`, `routeEligible` | Applies validation decisions and determines selectable paths | Preserves policy denial independently of validation acceptance |
| Ordered receive completion | `receive_publication.go`, `receivePublicationCut.wait`; `process/delivery.go`, `DrainEventsApplied` | Joins publication and store application after socket ingress | Reuses both barriers in live tests; does not substitute a KEEPALIVE counter for store completion |
| Prefix limits | `session_prefix.go`, `checkPrefixLimits` | Counts offered withdrawals and announcements, including unmatched withdrawals | Preserves offered semantics; adds a separate proven bound for retained unique routes |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire bytes: a BGP UPDATE arriving on an established session from a peer configured `rs-client true`.
- Format at entry: the raw UPDATE body, already header-validated and prefix-limit-counted by the session read loop, wrapped as a lazily parsed `WireUpdate` over a pool buffer.
- Second entry point, operator-facing: `show bgp adj-rib-in` and `show bgp adj-rib-in status`, typed at the CLI or sent as a command RPC.

### Transformation Path
1. `session_read.go` reads the message, runs `checkPrefixLimits`, and calls `notifyMessageReceiver`.
2. `notifyMessageReceiver` builds the `RawMessage` and walks the ordered ingress steps.
3. A step answers accept false. Today: return false. New: when the peer's `RSClient` setting is true, emit one filtered-update received event carrying the `RawMessage` and the denying step's name, then return false.
4. The event dispatcher delivers the event only to processes whose subscription matches the bgp namespace, the filtered-update type and the received direction, and whose peer binding grants that token.
5. bgp-adj-rib-in's structured handler routes the new event kind to a filtered-ingest path that reuses the existing withdrawal-before-announce walk and installs each `RawRoute` with the filtered mark set.
6. `buildReplayRoutes` walks the seqmap and skips a marked entry, so no `rpc.StoredRoute` ever carries one.
7. `show` and `status` read the same entries and report the mark.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session read goroutine → reactor ingress pipeline | direct call, `notifyMessageReceiver` | Yes |
| Reactor → plugin process | `rpc.StructuredEvent` over DirectBridge, or the JSON event rail for a forked plugin | Yes |
| Reactor config → peer settings | `ps.RSClient` from the plugin-owned `rs-client` YANG leaf | Yes |
| Config attach grant → delivery graph | `update-filtered` / `update-filtered-received` token resolved against the bgp event registry | Yes |
| Plugin → engine (replay) | `rpc.StoredRoute` via `walkReplayRoutes` and `replayPathCommand`; `ImportFiltered` entries never emitted | Yes |
| Plugin → operator | `show bgp adj-rib-in` and `show bgp adj-rib-in status` through `ApplyPipes` (`json`, `yaml`, `table`) and `ze_adjribin_routes_filtered` gauge | Yes |

### Integration Points
- `internal/core/bgp/events/events.go` - the one declaration of the type name (`EventUpdateFiltered`); the subscription grammar, the YANG validator and completion all derive from it.
- `internal/component/bgp/plugin/register.go` - adds `EventUpdateFiltered` to the namespace registration argument list.
- `pkg/plugin/rpc/enums.go` - `EventKindUpdateFiltered` (ordinal 12; `EventKindCount` is now 14 after `EventKindUpdatePrePolicyOut = 13`) carried on `StructuredEvent`.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` `SetStartupSubscriptions` - subscribes to `update-filtered direction received`.
- `internal/component/bgp/plugins/adj_rib_in/rib.go` `walkReplayRoutes`, `rib_replay_path.go` `replayPathCommand`, `rib_eligibility.go` `routeEligible`, `rib_validation.go` `applyToInstalled` - gate selection, replay, and validation independently of `Ineligible`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Denied `rs-client true` UPDATEs flow through `notifyOneMessage` -> `enqueueFilteredIngress` -> `EventKindUpdateFiltered` -> `bgp-adj-rib-in` (`handleReceivedStructured`/`handleReceived`) without touching `recentUpdates`, `reactorForwardRS`, `EventBus`, or `bgp-rib` |
| No unintended coupling (components stay isolated) | Yes | `./le arch tier check` and `./le plugin boundary check` both pass; `bgp-rs` takes no subscription to `update-filtered` (`TestRouteServerTakesNoDeliveryOfFilteredUpdates`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Single `ribIn` seqmap with `ImportFiltered`/`FilterStep` fields; reuses existing withdrawal walk, `putInstalledRouteLocked`, and `session_prefix.go` set machinery (`prefixCounts.retained`) |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Accepted UPDATEs keep existing zero-copy cache/pool lifecycle; `bytes.Clone` occurs only on the denied `rs-client` cold path in `enqueueFilteredIngress` so the borrowed read buffer is released |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `EventUpdateFiltered` registers once in `internal/component/bgp/plugin/register.go`; `ReceiveEventValidator`, `parseOneReceiveFlag`, and completion all derive from the registry (`TestReceiveGrantAcceptsUpdateFilteredToken`); `ConfigureMetrics` wires `ze_adjribin_routes_filtered` |

## Test-campaign research: 2026-11-15

The test campaign adds defect probes only. The implementation and RFC ledger
changes remain open. RFC 7947 Section 2.1 states: "A route server MUST accept all
UPDATE messages received from each of its clients for inclusion in its
Adj-RIB-In." The five tests in
`internal/component/bgp/reactor/rfc7947_filtered_received_test.go` carry
`VALIDATES` comments, not conformance tags. They use live sessions and the real
receive-store command handlers. The main thread formatted, compiled and ran
their baselines; the observed failures and passing controls are recorded below.

| Probe | Required observation |
|-------|----------------------|
| `TestRFC7947ReceivedLoopDenialRetained` | The client store retains a loop-denied UPDATE's original bytes; an ordinary peer's denied route stays absent. Accepted routes remain replayable |
| `TestRFC7947ReceivedImportFinalDenialRetained` | Explicit final rejection and absent EBGP import policy each retain the client's original announcement |
| `TestRFC7947ReceivedMixedDenialRetainsAnnouncement` | A mixed UPDATE removes the old key and retains its denied announcement, even when ordinary delivery accepts a withdrawal-only projection. A later explicit withdrawal removes it |
| `TestRFC7947ReceivedDeniedReplacementSupersedesOldBytes` | A denied replacement updates the received bytes under the existing key and prevents replay of the previously accepted version |
| `TestRFC7947ReceivedValidationCannotAcceptPolicyDenial` | An origin-validation acceptance cannot make an import-denied path eligible or replayable; the accepted control remains eligible |

The additional probes in
`internal/component/bgp/reactor/rfc7947_filtered_selection_test.go` exercise
AC-3, AC-4 and AC-6 with a recipient socket and the daemon's actual selected
Loc-RIB, on both cached and fast route-server forwarding. The fixture starts
the registered received store, selecting RIB and route-server plugins. Its
only directly installed route is the terminal interior host route resolving
the announced NEXT_HOP. An accepted control must be received, selected and
advertised before a live-export absence is judged; the late-client probe must
receive that accepted control through peer-up replay. Main-thread execution
of the final capture-fenced tests is recorded under Observed test-campaign
results.

| Probe | Required observation |
|-------|----------------------|
| `TestRFC7947FilteredReceivedExcludedFromSelection` | Loop rejection, explicit import rejection and absent import policy each retain the original client entry marked filtered, while selection, live export and both replay queries withhold it; accepted controls remain useful |
| `TestRFC7947FilteredReplacementClearsSelectedRoute` | A previously received, selected and advertised path is replaced by current denied bytes marked filtered; its earlier version leaves the selected table, recipient and both replay rails. A subsequent explicit withdrawal removes the proven filtered entry |
| `TestRFC7947FilteredReceivedWithheldOnClientJoin` | A client joining after the denial receives accepted replay and no denied route; a subsequent explicit withdrawal removes the proven filtered entry without removing the accepted control |

Retention and downstream assertions in these probes are nonfatal so a stale
accepted replacement can expose selection, wire and replay defects in the same
run. The final removal checks require a successfully observed retained and
marked entry first. A run that stops at that prerequisite proves no removal
of a filtered entry. These are untagged defect probes, not new conformance
credit or an implementation of this spec.

The current source changes several earlier assumptions in this design:

| Subject | Producer evidence and consequence |
|---------|-----------------------------------|
| Denial provenance | `reactor_notify.go` `notifyOneMessage` returns before dispatch on `!res.accept`. However, `filter_ordered.go` `ingressPolicyWithdrawals` returns `accept=true` and a withdrawal-only `modifiedPayload` for a denied mixed UPDATE. A hook only at the false branch misses its announcements. Retention needs the original bytes and the policy disposition before that projection |
| Replay consumers | `adj_rib_in/rib.go` `walkReplayRoutes` is shared by full and paged replay. `rib_replay_path.go` `replayPathCommand` separately returns a stored announcement. The filtered mark must govern both, plus `rib_eligibility.go` `routeEligible` |
| Validation independence | `rib_validation.go` `applyToInstalled` replaces `RawRoute.Ineligible` with the new validation decision. The import-filtered mark must be separate, or accepting a later validation decision releases the policy denial |
| FIFO barrier | `receive_publication.go` `receivePublicationCut.wait` joins the per-peer delivery worker's publication. `process/delivery.go` `DrainEventsApplied` then joins store application. A KEEPALIVE counter alone does not join the asynchronous UPDATE delivery worker |
| Retained memory bound | `session_prefix.go` `checkPrefixLimits` subtracts every offered withdrawal without checking whether the store holds that key. Therefore A-2 and AC-15 cannot use the default offered tally as proof of a bound on the retained population |

The population-bound counterexample is: configure maximum one in offered mode;
receive denied A; receive an unmatched withdrawal B; receive denied C. The
offered count moves one, zero, one, while retaining both A and C would need two
entries. The existing offered semantics must remain intact. The implementation
needs an independent unique-retained-population bound, or another explicit
admission design that proves the same bound. Its test must run this exact
sequence, require A's retention before the unmatched withdrawal, and prove that
C cannot leave two retained entries. It must also prove that replacing A does
not consume a second slot, withdrawing a held key releases its slot, and an
accepted control can occupy a released slot. This is an additional design and
test obligation, not an assertion that the current counter already provides it.

The existing SHOULD levels and gap annotations for `RFC7947-x-4` and
`RFC7947-x-5` are unchanged by this test batch. No extraction sign-off or
discrimination record is created for these defect probes.

## Selected-path replay ordering investigation: 2026-11-16 (split out 2026-11-22)

The 2026-11-16 delayed selected-path replay ticket/cold-worker proposal is
orthogonal to RFC 7947 Section 2.1 pre-policy retention and has been split into
its own spec (`plan/release/fixes/spec-bgp-selected-path-replay-ordering.md`,
owner approval 2026-11-22).

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `walkReplayRoutes`, `replayPathCommand`, and `routeEligible` are the ONLY paths by which an Adj-RIB-In entry becomes an advertisement or eligible path | Source inspection of `adj_rib_in/{rib.go,rib_replay_path.go,rib_eligibility.go,rib_validation.go}` | a filtered route leaks on an unguarded rail or is un-filtered by RPKI validation | `TestAdjRIBInFilteredLifecycleAndViews`, `TestRFC7947ReceivedValidationCannotAcceptPolicyDenial`, and `TestRFC7947FilteredReceivedExcludedFromSelection` | confirmed |
| A-2 | Unique retained keys per source peer (accepted + import-filtered) must be bounded independently of offered-withdrawal subtraction | 2026-11-15 counterexample: unmatched withdrawal B decrements offered count while retained denied A stays in memory | a hostile rs-client can exceed its configured prefix maximum using unmatched withdrawals | `TestRFC7947FilteredRetainedPopulationBound` (`session_prefix.go::prefixCounts.retained`) | confirmed |
| A-3 | A denied replacement of a previously accepted route must emit a withdrawal for that replaced prefix on the accepted rail (Loc-RIB / forwarding) while retaining the new denied bytes marked filtered in Adj-RIB-In | `TestRFC7947FilteredReplacementClearsSelectedRoute` and `TestRFC7947ReceivedDeniedReplacementSupersedesOldBytes` | stale accepted path remains selected and advertised to clients | Both probes passing via `enqueueFilteredIngress` withdrawal-first FIFO ordering | confirmed |
| A-4 | No existing consumer reads `show bgp adj-rib-in` route objects by exact key set | Grep of `test/` and `internal/test/fixture/` confirms every consumer reads named fields (`adj-rib-in`, `total-routes`, `attr-hex`, etc.) | a functional fixture goes red on an added key | Grep verified across `internal/test/fixture/` + promoted `.ci` runs | confirmed |
| A-5 | `RFC7947-x-4` and `RFC7947-x-5` are already at `[SHOULD]` in `rfc/short/rfc7947.md`, and `RFC7947-2.1-1` + `rfc/extraction/rfc7947.json` already exist | Verified 2026-11-22 against `rfc/short/rfc7947.md` and `rfc/extraction/rfc7947.json` | unnecessary or harmful level downgrade | `./le rfc update` + `./le rfc extraction report` | confirmed |
| A-6 | `bgp-rs` takes no delivery of `update-filtered`, so its forward-target selection and its replay cut are unchanged | Read all `SetStartupSubscriptions` sites under `internal/component/bgp/plugins/`; only `bgp-adj-rib-in` subscribes to `update-filtered` | the route server forwards a filtered route, which is the leak G-2 exists to prevent | `TestRouteServerTakesNoDeliveryOfFilteredUpdates` plus `TestRFC7947FilteredReceivedExcludedFromSelection` | confirmed |
| A-7 | The ingest-position contract is not disturbed by a second event kind carrying a MessageID | `noteIngested` publishes the position `bgp-rs`'s cut is compared against, and a filtered UPDATE also has a MessageID | the peer-up cut advances past routes the live rail will not deliver, and the replay omits them: both rails silent | `TestAdjRIBInFilteredLifecycleAndViews` (`ingestPosition` advances to msgID 12 across filtered msgID 11) | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The filtered entry is replayed on peer-up and the policy denial becomes a route leak | the interop scenario's third party learns a prefix its policy denies | `buildReplayRoutes` skips the mark, and the interop scenario asserts the ABSENCE, proven by reverting the skip and watching it go red |
| R-2 | A future subscriber to the new type treats it as an ordinary UPDATE | a new plugin's `SetStartupSubscriptions` names the type | the event kind is distinct and its name says what it is; the handler that routes it is the only one that stores |
| R-3 | The withdrawal path does not remove a filtered entry, so a withdrawn prefix stays in the store forever | `show bgp adj-rib-in` reports a prefix the client withdrew | the filtered-ingest path reuses the SAME withdrawal walk; a withdrawal is not filtered separately, it removes whatever key it names |
| R-4 | An rs-client peer subject to a teardown verdict inserts a route into a store about to be deleted | none, it is harmless | teardown returns before the emit, and the state handler deletes the whole map on down |
| R-5 | The filtered population grows without bound for an rs-client peer with no configured prefix maximum | `total-routes` climbs while `show bgp rib` stays flat | A-2 and A-3 settle whether the existing bound covers it; if not, this spec owes an explicit cap and the AC table gains a row |
| R-6 | Adding a new event kind moves the enum count, breaking a consumer that indexes the table by ordinal | compile failure, or a wrong wire string | the new kind takes the next ordinal and the count moves with it; run `gopls references` on the count constant before the edit |
| R-7 | The correction paragraph's quote does not appear verbatim in `rfc/full/rfc7947.txt` after whitespace squashing | `./le rfc check` reports the level ratchet with the id named | `correctionAuthorizes` squashes whitespace on both sides; the quote is copied from the file, not retyped |
| R-8 | The new `RFC7947-2.1-1` is gated and one polarity is missing at closure | `./le rfc check` names the missing polarity | both polarities are in the AC table and the TDD plan from the start |
| R-9 | Another session is mid-edit in `internal/component/bgp/reactor` (`api_sync_test.go`, `SignalPeerAPIReady`), so a red there is not this spec's | a red in a test file this spec does not name | judge the change by the evidence the change produced; report what could not be verified (`ai/rules/principles.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A route the operator's policy denies is advertised to other route-server clients at an IXP. That is a route leak with a third party's traffic behind it, and it is visible outside this repository |
| How is it reverted? | Single commit revert. Nothing persists: the Adj-RIB-In is in-memory and rebuilt on peer-up, and the new event type is inert with no subscriber |
| Who else touches this path? | A concurrent session is editing `internal/component/bgp/reactor` (`api_sync_test.go`, `SignalPeerAPIReady`). The initial-sync barrier is built and documented at `docs/architecture/api/architecture.md` (`spec-bgp-session-ready-contract`, closed 2026-09-05). `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` owns the rfc7947 sign-off this spec unblocks |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE denied by `LoopIngress` on an `rs-client true` peer | → | the filtered-update emit in `notifyMessageReceiver` | `TestNotifyEmitsFilteredUpdateForRouteServerClient` |
| A filtered-update received event | → | the `AdjRIBInManager` filtered-ingest handler | `TestAdjRIBInStoresFilteredUpdate` |
| Peer establishes, bgp-rs drives `request bgp adj-rib-in replay` | → | `buildReplayRoutes` | `TestBuildReplayRoutesOmitsFilteredEntries` |
| Operator types `show bgp adj-rib-in` | → | the `show` handler in `rib_commands.go` | `TestShowReportsFilteredMark` |
| Operator types `show bgp adj-rib-in status` | → | the `status` handler in `rib_commands.go` | `TestStatusReportsFilteredCount` |
| Config attach block granting the filtered receive token | → | `parseOneReceiveFlag` | `TestReceiveGrantAcceptsUpdateFilteredToken` |
| A route-server client announces a prefix its policy denies | → | the whole chain, over the wire | `bgp-route-server-filtered-adj-rib-in-frr` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | An `rs-client true` peer sends an UPDATE that `LoopIngress` denies | `show bgp adj-rib-in` reports the prefix under that peer, marked filtered, naming the loop step as the denying step |
| AC-2 | An `rs-client true` peer sends an UPDATE that its import policy chain denies | `show bgp adj-rib-in` reports the prefix under that peer, marked filtered, naming the peer policy chain as the denying step |
| AC-3 | The condition of AC-1 or AC-2 holds | `show bgp rib` reports no route for that prefix from that peer, and no other route-server client is sent an UPDATE for it |
| AC-4 | The condition of AC-1 holds, then a second route-server client establishes | The second client receives no advertisement for the filtered prefix, and receives every accepted prefix |
| AC-5 | A peer with `rs-client` unset (the default false) sends an UPDATE that `LoopIngress` denies | `show bgp adj-rib-in` reports no entry for that prefix: today's behavior is unchanged for every non-route-server peer |
| AC-6 | An `rs-client true` peer announces a prefix that is denied, then withdraws it | The filtered entry is gone from `show bgp adj-rib-in` |
| AC-7 | An `rs-client true` peer holding filtered entries goes down | Its filtered entries are gone from `show bgp adj-rib-in`, with the rest of its store |
| AC-8 | `show bgp adj-rib-in` is rendered through the json, yaml and table pipe operators | Each rendering distinguishes a filtered entry from an accepted one, from the same payload |
| AC-9 | `show bgp adj-rib-in status` on a store holding both populations | The answer reports the filtered count separately, and `total-routes` still counts every entry |
| AC-10 | A config names the filtered receive token in an attach process block | The config loads, and completion offers the token |
| AC-11 | A config names a misspelling of that token | The config is refused, naming the token and listing the valid names |
| AC-12 | `./le rfc check` runs on the tree | `RFC7947-2.1-1` leaves `{gap}`, has positive and negative tagged tests with verified discrimination records, and `rfc7947` passes `./le rfc check` cleanly |
| AC-13 | `rfc/short/rfc7947.md` existing advisory rows | `RFC7947-x-4` and `RFC7947-x-5` remain untouched at `[SHOULD]` (already corrected and proven; superseded 2026-11-22) |
| AC-14 | `./le rfc extraction report` runs | `rfc/extraction/rfc7947.json` remains signed with Section 2.1 mapped to `RFC7947-2.1-1` |
| AC-15 | An `rs-client true` peer with a configured prefix maximum of N sends denied A, unmatched withdrawal B, then denied C (with N=1) | Unique retained entries (accepted + import-filtered) never exceed N; replacing an existing key does not consume a second slot, withdrawing a held key releases its slot, and an accepted control can occupy the released slot |
| AC-16 | Mixed UPDATEs and denied replacements on an `rs-client true` peer | A mixed UPDATE removes withdrawn keys and retains denied announcements marked import-filtered; a denied replacement of a previously accepted/selected route stores the new bytes marked import-filtered AND withdraws the previously accepted path from Loc-RIB, live recipients, and both replay rails; later RPKI valid verdicts cannot un-filter an import-filtered entry |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs an IXP route server, an import policy denies a client's prefix, and asks what that client actually sent | wire → session read → ingress deny → filtered event → Adj-RIB-In marked → `show bgp adj-rib-in` | `adj-rib-in-filtered-visible` |
| 2 | Confirms the denied prefix reaches no other client, on the live rail and after a reconnect | wire → ingress deny → no forward; peer-up → replay → `buildReplayRoutes` skip | `bgp-route-server-filtered-adj-rib-in-frr` |
| 3 | Counts how much of a client's table policy is rejecting | `show bgp adj-rib-in status` → filtered count beside total | `adj-rib-in-filtered-status` |
| 4 | Grants a process delivery of filtered UPDATEs in the config editor, using completion | YANG leaf-list → the receive validator's completion func → the bgp event registry | `TestReceiveGrantAcceptsUpdateFilteredToken` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNotifyEmitsFilteredUpdateForRouteServerClient` | `internal/component/bgp/reactor/reactor_notify_test.go` | a denial on an rs-client peer emits one filtered-update received event carrying the denying step's name (RFC7947-2.1-1 positive) | |
| `TestNotifyEmitsNoFilteredUpdateForOrdinaryPeer` | `internal/component/bgp/reactor/reactor_notify_test.go` | a denial on a peer with rs-client false emits nothing (AC-5) | |
| `TestNotifyEmitsNoFilteredUpdateOnTeardown` | `internal/component/bgp/reactor/reactor_notify_test.go` | a teardown verdict returns before the emit | |
| `TestFilteredUpdateIsNotForwarded` | `internal/component/bgp/reactor/forward_rs_test.go` | a denied UPDATE reaches neither the RS fast path nor the delivery channel (RFC7947-2.1-1 negative) | |
| `TestAdjRIBInStoresFilteredUpdate` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | the filtered-ingest path stores the route with the mark and the denying step name | |
| `TestAdjRIBInFilteredWithdrawalRemovesEntry` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-6 | |
| `TestAdjRIBInPeerDownClearsFilteredEntries` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-7 | |
| `TestBuildReplayRoutesOmitsFilteredEntries` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-4, the security constraint: no stored route carries a filtered entry | |
| `TestBuildReplayRoutesCursorSkipsFilteredWithoutStalling` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | a filtered entry consumes a sequence number, so the returned cursor still advances past it and a later incremental replay does not re-walk it | |
| `TestIngestPositionAdvancesOnFilteredUpdates` | `internal/component/bgp/plugins/adj_rib_in/ingest_position_test.go` | A-7: the ingest position and the replay cut agree when a filtered UPDATE sits between two accepted ones | |
| `TestShowReportsFilteredMark` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-8, at the payload level | |
| `TestStatusReportsFilteredCount` | `internal/component/bgp/plugins/adj_rib_in/rib_test.go` | AC-9 | |
| `TestRouteServerTakesNoDeliveryOfFilteredUpdates` | `internal/component/bgp/plugins/rs/server_test.go` | A-6: the route server's subscription set does not match the new type | |
| `TestReceiveGrantAcceptsUpdateFilteredToken` | `internal/component/bgp/reactor/config_test.go` | AC-10 and AC-11 | |
| `TestEventKindUpdateFilteredRoundTrips` | `pkg/plugin/rpc/enums_test.go` | the wire string marshals and unmarshals, and the enum count covers it | |
| `TestFilteredRoutesCountAgainstThePrefixMaximum` | `internal/component/bgp/reactor/session_prefix_test.go` | AC-15 and A-2 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| the new event kind's ordinal | 1 to one below the enum count | the next free ordinal | 0, the unspecified kind, which refuses to marshal | the count sentinel and above, which the marshaller rejects |
| prefix maximum counting a denied UPDATE | 0 to 4294967295 | N, the session survives | N/A | N+1, Cease with the max-prefix subcode |
| filtered entries per source peer | 0 to the family's configured maximum | the maximum | N/A | one above the maximum is unreachable: the session is torn down first |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `adj-rib-in-filtered-visible` | `test/plugin/` | an rs-client peer announces a prefix a loop check denies; the operator sees it in `show bgp adj-rib-in` marked filtered, and absent from `show bgp rib` | |
| `adj-rib-in-filtered-status` | `test/plugin/` | `show bgp adj-rib-in status` reports the filtered count beside the total | |
| `adj-rib-in-filtered-not-replayed` | `test/plugin/` | a second rs-client peer establishes after the filtered UPDATE and is sent every accepted prefix and none of the filtered one | |
| `adj-rib-in-filtered-absent-for-plain-peer` | `test/plugin/` | the same denied UPDATE on a peer without rs-client leaves the Adj-RIB-In empty | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-route-server-filtered-adj-rib-in-frr` | `test/interop/scenarios/` | FRR and BIRD, modelled on `bgp-route-server-frr`: the same two-client shape, `rs-client true` on both, the rs attach block on each, plus bgp-adj-rib-in attached and an import policy on one client that denies one prefix | An FRR client's denied prefix is held in Ze's Adj-RIB-In, is never advertised to the BIRD client on the live rail, and is still never advertised after the BIRD client reconnects and is replayed | |

## Files to Modify

- `internal/core/bgp/events/events.go` - the filtered-update event type constant beside the existing type constants. Design doc: `docs/architecture/api/process-protocol.md`
- `internal/component/bgp/plugin/register.go` - name the new type in the namespace registration. Design doc: `docs/architecture/core-design.md`
- `pkg/plugin/rpc/enums.go` - the new event kind, its wire string, its table row, its unmarshal case, and the count. Design doc: `docs/architecture/api/ipc_protocol.md`
- `internal/component/bgp/reactor/reactor_notify.go` - emit the filtered event before the deny return, gated on the peer's `RSClient` setting, carrying the denying step's name; retain partially dropped announcement subsets on `update-filtered`. Design doc: `docs/architecture/core-design.md`
- `internal/component/bgp/reactor/filter_ordered.go`, `filter_library.go`, `filter_nlri_delta.go`, `internal/component/bgp/wireu/nlri_cidr.go` - extract policy-dropped announcement subsets (`KeepCIDREntries`, `buildDeniedAnnouncementPayload`) with unmodified pre-policy attributes when a multi-NLRI UPDATE is partially accepted on an `rs-client true` peer. Design doc: `docs/architecture/core-design.md`
- `internal/component/bgp/reactor/session_prefix.go` - track unique retained keys (`prefixCounts.retained`) for `rs-client true` peers in offered mode so unmatched withdrawals cannot exceed configured prefix maximums. Design doc: `docs/architecture/core-design.md`
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - `RawRoute` gains the filtered mark and the denying step name; the startup subscriptions gain the new type; the structured dispatch routes the new kind; `buildReplayRoutes` skips a marked entry. Design doc: `docs/architecture/plugin/rib-storage-design.md`
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `show` and `status` report the mark and the count. Design doc: `docs/architecture/plugin/rib-storage-design.md`
- `rfc/short/rfc7947.md` - add `RFC7947-2.1-1`; correct `RFC7947-x-4` and `RFC7947-x-5`; add two `Correction 2026-08-30:` paragraphs
- `docs/features/rfc-status.md` - the RFC 7947 row: the Adj-RIB-In acceptance, and the two corrected levels
- `docs/architecture/plugin/rib-storage-design.md` - the filtered population and the replay skip
- `docs/architecture/api/process-protocol.md` - the filtered-update event type and its receive-grant token
- `docs/architecture/api/ipc_protocol.md` - the new event kind
- `docs/architecture/api/commands.md` - the `show bgp adj-rib-in` and status answer keys
- `docs/architecture/core-design.md` - the ingress deny path now has a consumer
- `docs/guide/plugins.md`, `docs/features/plugins.md` - the bgp-adj-rib-in subscription set
- `docs/guide/route-reflection.md`, `docs/guide/rpki.md` - checked for staleness against the changed `show bgp adj-rib-in` shape

## Files to Create

- `internal/component/bgp/plugins/adj_rib_in/metrics.go` - `ze_adjribin_routes_filtered` Prometheus gauge and per-peer filtered counter helpers
- `internal/component/bgp/plugins/adj_rib_in/rfc7947_filtered_test.go` - unit tests for filtered lifecycle, views, replay skip, and Prometheus gauge
- `internal/component/bgp/plugins/rs/rfc7947_filtered_subscription_test.go` - unit test proving `bgp-rs` takes no `update-filtered` delivery
- `internal/component/bgp/reactor/rfc7947_filtered_conformance_test.go` - `RFC7947-2.1-1` positive/negative conformance tests, receive-token validation, AC-15 bound, and teardown guard
- `internal/component/bgp/reactor/rfc7947_filtered_partial_test.go` and `internal/component/bgp/wireu/nlri_cidr_test.go` - unit tests for multi-NLRI partial-policy-rejection retention and `KeepCIDREntries`
- `pkg/plugin/rpc/event_kind_filtered_test.go` - `EventKindUpdateFiltered` marshal/unmarshal and count test
- `internal/test/fixture/register_adj_rib_in_filtered.go` and `internal/test/fixture/plugin_adj_rib_in_filtered.go` - compiled functional test observers
- `test/plugin/adj-rib-in-filtered-visible.ci`
- `test/plugin/adj-rib-in-filtered-status.ci`
- `test/plugin/adj-rib-in-filtered-not-replayed.ci`
- `test/plugin/adj-rib-in-filtered-absent-for-plain-peer.ci`
- `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr/` (`ze.conf`, `frr.conf`, `bird.conf`) and `internal/le/interoplab/bgp/check_rfc7947_filtered.go`
- `website/changes/fragments/rfc7947-adj-rib-in-accepts-filtered-updates.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new leaf. The filtered receive token becomes a valid VALUE of the existing receive leaf-list because its validator resolves the bgp event registry at call time (`ReceiveEventValidator`, `internal/component/config/validators.go`) |
| YANG validation constraints | N-A | The receive leaf-list is already constrained by `ReceiveEventValidator`; the new value is admitted by the registry, not by a widened constraint |
| YANG custom validators | Yes | `ReceiveEventValidator` (`internal/component/config/validators.go`) needs no edit; AC-10 and AC-11 prove it admits the new token and refuses a misspelling |
| CLI commands/flags | No | No new command. `show bgp adj-rib-in` and `show bgp adj-rib-in status` gain answer keys, declared in `commandDecls` (`internal/component/bgp/plugins/adj_rib_in/rib.go`) |
| CLI grammar (keyword before value) | N-A | No new verb, noun or selector; the grammar is untouched |
| Editor autocomplete | Yes | Automatic: the receive validator's completion func reads the registry the new type registers into |
| Functional test for new RPC/API | Yes | `test/plugin/adj-rib-in-filtered-visible.ci` and its three siblings |
| Pipe completeness | Yes | Both commands keep their document shape; the mark is a FIELD on the route object, never a character glued to the key, so json, yaml and table each render it (AC-8) |
| Env var registration | N-A | No leaf under the environment subtree |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, kernel module, procfs, netlink, binary or certificate. The change is in-memory and on an existing event rail |
| Prometheus counters/metrics | Yes | A filtered-route gauge per peer, named and labelled in the implementation phase beside the existing prefix-count metric (`setPrefixCountMetric`, `internal/component/bgp/reactor/session_prefix.go`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new address family, NLRI type, capability code or attribute code. The change is in the ingress pipeline and a plugin's store |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - the route-server Adj-RIB-In holds the pre-policy population |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` - the filtered receive token |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` - the new `show bgp adj-rib-in` and status keys |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` - the answer shapes |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` - the bgp-adj-rib-in subscription set |
| 6 | Has a user guide page? | Yes | `docs/guide/route-reflection.md` and `docs/guide/rpki.md` both show `show bgp adj-rib-in` output; verified against the new shape |
| 7 | Wire format changed? | No | No BGP message, attribute or NLRI encoding changes. Only which in-process consumer sees a message already parsed |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` and `docs/architecture/api/ipc_protocol.md` - the event type and the event kind |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7947.md` and the RFC 7947 row of `docs/features/rfc-status.md` |
| 10 | Test infrastructure changed? | No | No runner, harness or fixture-format change. Four new functional fixtures and one new interop scenario use the existing shapes |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - a pre-policy Adj-RIB-In for route-server clients is a comparison row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (the ingress deny path has a consumer) and `docs/architecture/plugin/rib-storage-design.md` (the filtered population and the replay skip) |
| 13 | Route metadata keys added/changed? | No | The mark lives on `RawRoute` inside the plugin, not on the route metadata map the reactor threads through the ingress pass |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` - the filtered-route gauge |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` - a new registered event type |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, do not answer from memory: run `./le spec citation anchors spec plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` at the start of the implementation phase and name every document it lists. The four Design documents the changed files DECLARE are named above in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/route-reflection.md` and `docs/guide/rpki.md` carry `show bgp adj-rib-in` samples; each is checked against the handler and updated |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the event type exists end to end and nobody stores anything yet
   - Tests: `TestEventKindUpdateFilteredRoundTrips`, `TestReceiveGrantAcceptsUpdateFilteredToken`, `TestNotifyEmitsFilteredUpdateForRouteServerClient`
   - Files: `internal/core/bgp/events/events.go`, `internal/component/bgp/plugin/register.go`, `pkg/plugin/rpc/enums.go`, `internal/component/bgp/reactor/reactor_notify.go`
   - Verify: the emit fires, the token validates, and `TestAdjRIBInStoresFilteredUpdate` fails because nothing subscribes
2. **Phase: Store the filtered entry** -- the mark and the ingest path
   - Tests: `TestAdjRIBInStoresFilteredUpdate`, `TestAdjRIBInFilteredWithdrawalRemovesEntry`, `TestAdjRIBInPeerDownClearsFilteredEntries`, `TestIngestPositionAdvancesOnFilteredUpdates`
   - Files: `internal/component/bgp/plugins/adj_rib_in/rib.go`
   - Verify: `show bgp adj-rib-in` holds the entry; `show bgp rib` still does not
3. **Phase: Close the replay rail (the security half)** -- write `TestBuildReplayRoutesOmitsFilteredEntries` FIRST, watch it fail against phase 2's code, then add the skip
   - Tests: `TestBuildReplayRoutesOmitsFilteredEntries`, `TestBuildReplayRoutesCursorSkipsFilteredWithoutStalling`, `TestRouteServerTakesNoDeliveryOfFilteredUpdates`, `TestFilteredUpdateIsNotForwarded`
   - Files: `internal/component/bgp/plugins/adj_rib_in/rib.go`
   - Verify: no stored route carries a filtered entry, on any path
4. **Phase: Operator surface** -- the mark reaches the CLI
   - Tests: `TestShowReportsFilteredMark`, `TestStatusReportsFilteredCount`, the four functional fixtures
   - Files: `internal/component/bgp/plugins/adj_rib_in/rib_commands.go`, the four new fixtures under `test/plugin/`
   - Verify: json, yaml and table each distinguish the two populations
5. **Phase: Bound the population** -- settle A-2 and A-3 at the producer
   - Tests: `TestFilteredRoutesCountAgainstThePrefixMaximum`
   - Files: `internal/component/bgp/reactor/session_prefix.go`, only if the reading shows the existing count does not cover the denied UPDATE
   - Verify: AC-15; if A-3 is broken, the AC table gains an explicit cap row before any code is written for it
6. **Phase: Interop** -- the scenario, with its red phase forced
   - Tests: `bgp-route-server-filtered-adj-rib-in-frr`
   - Files: the scenario directory, modelled on `bgp-route-server-frr`
   - Verify: revert phase 3's skip, rebuild the daemon binary the scenario drives, confirm the third party learns the filtered prefix (RED), restore, confirm GREEN, and record the RED output
7. **Phase: RFC ledger** -- the requirement row, the two corrections, the sign-off
   - Tests: `./le rfc check`, `./le rfc extraction report`
   - Files: `rfc/short/rfc7947.md`, `rfc/extraction/rfc7947.json`, `docs/features/rfc-status.md`
   - Verify: AC-12, AC-13, AC-14; the level ratchet is silent because each correction quotes the RFC verbatim
8. **Phase: Documentation** -- every row of the checklist above that reads Yes
   - Files: as listed in Files to Modify
   - Verify: `./le spec citation anchors spec plan/release/fixes/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` names no unlisted owner document

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and AC-4 and AC-5 each have a test that FAILS when the guard is removed |
| Feature completeness | The chain from a denied wire UPDATE to `show bgp adj-rib-in` runs with no stub, on the DirectBridge rail and on the forked JSON rail |
| Correctness | `buildReplayRoutes` skips the mark on every branch of its walk, including the replay-cut early return and the same-peer skip; the cursor still advances |
| Correctness | The emit is gated on the peer's `RSClient` setting, not on `RSFastPath`: the fast path is a performance rail and the RFC obligation is about the peer's ROLE |
| Correctness | The teardown branch returns before the emit |
| Naming | The two new JSON keys are lowercase kebab-case; the event type name matches its event kind wire string exactly |
| Data flow | No consumer other than bgp-adj-rib-in subscribes to the new type; verified by reading each of the seven startup-subscription call sites, not by grep |
| Rule: `ai/rules/principles.md` | The guard fails CLOSED: an entry with no mark is treated as accepted only because a filtered entry cannot arrive on the accepted rail. Confirm no path stores from the filtered handler without setting the mark |
| Rule: `ai/rules/rfc-compliance.md` | The RFC 7947 Section 2.1 comment with the quoted sentence sits directly above the emit, and above the replay skip |
| Rule: `ai/rules/interop-and-goal-validation.md` | The interop scenario's RED phase was forced by reverting the change and REBUILDING the artifact, and the RED output is recorded |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The new event type reaches only its subscriber | grep the plugin tree for startup subscriptions and read all seven call sites |
| No filtered entry ever becomes a stored route | `TestBuildReplayRoutesOmitsFilteredEntries` plus `gopls references` on the store field, re-run after the change |
| Both polarities for `RFC7947-2.1-1` | `./le rfc check` |
| rfc7947 signed off | `./le rfc extraction report` reports rfc7947 as signed |
| Level corrections accepted | `./le rfc check` reports no level-ratchet violation |
| Interop scenario exists and is named, not numbered | list `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr` |
| Spec valid | `./le ai hooks validate-spec` exits 0 |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The filtered UPDATE is a message a filter already REFUSED. Its bytes are stored, not trusted: the store keeps hex, and the only consumer that could re-emit it is `buildReplayRoutes`, which must skip it |
| Resource exhaustion | A hostile route-server client can now cost memory with routes that are all denied. A-2, A-3, R-5 and AC-15 settle whether the configured prefix maximum bounds it, and the answer must be proven, not assumed |
| Authorization that could fail open | The receive grant is what admits the new event to a process. A process granted the wildcard gains it implicitly, which is the documented meaning of the wildcard; confirm no fixture grants the wildcard where the filtered population would surprise its assertions |
| Information disclosure | `show bgp adj-rib-in` now shows routes an operator's policy rejected. That is the point of the feature, and the command sits behind the same authz as the accepted population; confirm no new command path bypasses it |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A red in `internal/component/bgp/reactor` naming `api_sync_test.go` or `SignalPeerAPIReady` | Not this spec's: another session owns it. Report it, do not repair it (`ai/rules/principles.md`) |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The receive-grant vocabulary, the subscription grammar and the config completion are all DERIVED from one event registry, so adding a new event type costs a constant and one registration argument, and reaches four surfaces. That is what makes the fail-closed design cheaper than it looks.
- Offered-mode prefix counting decrements on unmatched withdrawals without tracking store membership, so retaining denied routes on `rs-client true` peers requires an independent unique-retained set (`prefixCounts.retained` in `session_prefix.go`) to enforce the configured maximum without altering offered-mode semantics.
- Capturing original pre-policy UPDATE bytes before `ingressPolicyWithdrawals` projection and queueing withdrawals on the ordinary `update` rail ahead of the `update-filtered` event keeps Loc-RIB, downstream recipients, and Adj-RIB-In consistent for both mixed UPDATEs and denied replacements.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Deliver the filtered UPDATE on a DISTINCT registered event type (`update-filtered`) | A filtered boolean on the existing `RawMessage`, dispatched on the ordinary `update` event | Seven plugins subscribe to `update direction received`. A boolean makes ingress-filter correctness depend on six plugins each checking it (fail-open). A distinct type reaches only `bgp-adj-rib-in` and fails closed |
| Scope the change to peers with `rs-client true` | Every peer | RFC 7947 Section 2.1 binds a route server, and `rs-client` is the sole leaf defining that role (`ps.RSClient`, `internal/component/bgp/reactor/config.go`) |
| Carry the denying step's name (`FilterStep`) on `RawRoute` alongside `ImportFiltered` | A boolean alone; reusing RPKI `Ineligible` | `orderedIngressStep.name` tells operators why a route was denied, and keeping `ImportFiltered` separate from `Ineligible` prevents a later RPKI validation decision from releasing a policy denial |
| Skip `ImportFiltered` in `walkReplayRoutes`, `replayPathCommand`, `routeEligible`, and `applyToInstalled` | A separate store for filtered routes | One seqmap with one field avoids duplicating peer keying, withdrawals, and peer-down cleanup while guarding every selection and replay consumer |
| Track unique retained keys in `prefixCounts.retained` for `rs-client true` offered-mode families | Relying on the offered-mode counter | Unmatched withdrawals decrement the offered counter while denied routes remain stored; `prefixCounts.retained` enforces `maximum` on unique retained keys while keeping offered semantics intact |
| Keep `RFC7947-x-4` and `RFC7947-x-5` untouched at `[SHOULD]` | Downgrading `x-5` to `MAY` | Approved 2026-11-22: both rows are already `[SHOULD]` and proven in `rfc/short/rfc7947.md` |

## Known Limitations

- BMP Adj-RIB-In Pre-Policy monitoring (RFC 7854 Section 5) still reports the post-policy population. `bgp-bmp` does not subscribe to `update-filtered`, deliberately: adding it changes what a monitoring station receives and is a separate RFC's obligation.
- A peer that is not a route-server client keeps its post-policy Adj-RIB-In.
- Path hiding is still mitigated only by ADD-PATH; multiple Loc-RIBs per client (Section 2.3.2.1) are not implemented.
- The filtered population shares the per-family configured prefix maximum (`prefixCounts.retained`); a peer with no configured maximum has the same unbounded exposure it already has for accepted routes.

## RFC Documentation (Scope: protocol)

Add an `// RFC NNNN Section X.Y:` comment with the quoted requirement above enforcing code.

| Site | Comment content |
|------|-----------------|
| `enqueueFilteredIngress` in `internal/component/bgp/reactor/reactor_notify.go` | RFC 7947 Section 2.1, quoting `"A route server MUST accept all UPDATE messages received from each of its clients for inclusion in its Adj-RIB-In."` |
| `walkReplayRoutes` (`rib.go`), `replayPathCommand` (`rib_replay_path.go`), `routeEligible` (`rib_eligibility.go`), `applyToInstalled` (`rib_validation.go`) | RFC 7947 Section 2.1 exclusion of import-filtered entries from selection, replay, and validation promotion |
| `retainDeniedIngress` gate in `reactor_notify.go` | Confines pre-policy retention to `rs-client true` peers |

Requirement rows in `rfc/short/rfc7947.md`:
- `RFC7947-2.1-1` (`[MUST]`, §2.1): `{gap}` removed; proven in positive (`TestRFC7947ClientDeniedUpdateRetainedInAdjRIBIn`) and negative (`TestRFC7947OrdinaryPeerDeniedUpdateAbsentFromAdjRIBIn`) polarities with verified discrimination records (`rfc/discrimination/rfc7947/RFC7947-2.1-1.json`) and independent audit verdict `enforced` (`rfc/audit/rfc7947/RFC7947-2.1-1.json`).
- `RFC7947-x-4` and `RFC7947-x-5`: remain untouched at `[SHOULD]` (2026-11-22 owner approval).
- `rfc/extraction/rfc7947.json`: remains signed with site `2.1:1` mapped to `RFC7947-2.1-1` and prose updated to reflect enforcement.

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
- [ ] AC-1 through AC-16 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Scoped verification passes: `./le test unit pkg path ./internal/component/bgp/plugins/adj_rib_in/`, `./le test unit pkg path ./internal/component/bgp/reactor/ run TestRFC7947`, `./le test unit pkg path ./internal/component/bgp/plugins/rs/`, `./le test unit pkg path ./pkg/plugin/rpc/ run TestEventKindUpdateFiltered`, `./le test bgp plugin --pattern adj-rib-in-filtered`, `./le test integration interop` (`bgp-route-server-filtered-adj-rib-in-frr`), and `./le rfc check`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Any lesson is recorded in its governing surface; no separate lesson artifact is required merely for closure
- [ ] **Commit A:** code, tests, docs, spec and learned summary
- [ ] **Commit B:** remove the spec only (commit A preserves it in history)

## Observed test-campaign results: 2026-11-15

All five new `TestRFC7947Received...` probes compiled and ran in
`tmp/session/2026-11-14-210034/scratch/job-rfc-campaign-new-batch-reds-37b87864.log`.
They fail because a denied route is absent from received storage, or because a
denied replacement leaves the previously accepted attribute bytes there. The
ordinary-peer comparison passes, and the accepted-route setup assertions pass.

The validation-independence test fails at its initial retained-route assertion;
its later validation/replay assertions are not yet reached. Do not report those
later behaviors as runtime-proven defects from this run. Source investigation
above identifies their required safeguards for the eventual implementation.
No product behavior, RFC level or support figure was changed by these probes.

The owner authorized retaining complex fixes for later review while the test
campaign proceeds. This spec owns that repair. Retain the shared log until all
specs citing it have superseding evidence; no extra report copy is needed.

The main thread compiled and ran the final three
`rfc7947_filtered_selection_test.go` probes with the race detector in
`tmp/session/2026-11-14-210034/scratch/job-rfc-boundaries-final-race-cfc4586c.log`.
All ten subcases passed their accepted controls: real receipt, next-hop
resolution, Loc-RIB selection and useful recipient output on both cached and
fast rails. The two late-client cases also received their accepted peer-up
replay. All ten subcases then reported the following product failures. The
final scoped run reported no DATA RACE; it remained red on the named behavior
assertions and is not a whole-package or whole-tree pass.

| Probe | Observed result | Still unproven |
|-------|-----------------|----------------|
| `TestRFC7947FilteredReceivedExcludedFromSelection` | All six loop, explicit-import-reject and absent-import-policy cases lack the denied entry in received storage. The joined downstream observations find no selected route, recipient announcement or replay announcement for that absent entry | Correct retention and its filtered mark; withholding a route that is actually retained as filtered |
| `TestRFC7947FilteredReplacementClearsSelectedRoute` | Both rails retain the old accepted attributes without a filtered mark. The actual Loc-RIB still selects the path. The final captured recipient history still holds that earlier route and has zero withdrawals where one is required. Bulk replay returns it and single-path replay answers an announcement | Correct replacement and suppression; explicit deletion of a retained filtered entry, because its retained-and-marked prerequisite failed before the withdrawal was sent |
| `TestRFC7947FilteredReceivedWithheldOnClientJoin` | Both rails receive the accepted peer-up control, but the denied source entry is absent from storage. Their final joined recipient histories contain no denied announcement | Correct retained-but-withheld state and explicit deletion from a proven filtered entry; the withdrawal steps are not reached |

The source cause is before each consumer: `reactor_notify.go`
`notifyOneMessage` returns on `!res.accept` before caching, live RS forwarding
or received-event dispatch. A denied replacement therefore never reaches
`adj_rib_in/rib.go` `handleReceivedStructured` or the selecting RIB as new
received truth. The old entry is still eligible for `walkReplayRoutes` and
`rib_replay_path.go` `replayPathCommand`; neither can infer that an UPDATE it
never received replaced that entry. These direct store, selected-table and
command observations corroborate the captured socket defect.

Independent review found that `filteredSelectedLive.settle` took its last
history snapshot before waiting for final recovery completion. The test now
retains both accepted markers and all original assertions, then joins the
forward writer, sends an actual recipient KEEPALIVE, waits for its capture,
and rebuilds the final history. The final run named above executed that
corrected fence. Both replacement subcases still held the old accepted route
after it completed, so this result establishes the settled recipient defect
as well as the store and selection defects. No product fix or conformance
credit is claimed.

Retain `job-rfc-boundaries-final-race-cfc4586c.log` at the path above until the
eventual repair provides superseding evidence for retention, selection,
replay, settled recipient state and filtered-entry deletion, and every other
spec that cites the shared log has superseding evidence. The earlier
`job-unit-pkg-334ed25d.log` is superseded for these three probes; retain it
only while another owning record still needs its other findings. No extra
report copy is needed.

---

## Implementation Summary

### What Was Implemented
- **Core filtered-update rail (landed in `44411d90c3`, completed in Commit A)**:
  - Registered `bgpevents.EventUpdateFiltered` (`"update-filtered"`) in `internal/core/bgp/events/events.go` and `internal/component/bgp/plugin/register.go`, and added `rpc.EventKindUpdateFiltered` (ordinal 12; `EventKindCount` 13 at `44411d90c3`, now 14 after `EventKindUpdatePrePolicyOut`) in `pkg/plugin/rpc/enums.go`.
  - Wired `reactor_notify.go` (`retainDeniedIngress`, `enqueueFilteredIngress`, `notifyOneMessage`, `partialDenied`) to queue withdrawals for affected prefixes on the ordinary `update` rail and then deliver original pre-policy UPDATE bytes on `update-filtered` for `rs-client true` peers on full denial, mixed-UPDATE `ingressPolicyWithdrawals` denial, and multi-NLRI partial-accept denial.
  - Added `wireu.KeepCIDREntries` (`internal/component/bgp/wireu/nlri_cidr.go`), `keepNLRI` (`filter_nlri_delta.go`), and `buildDeniedAnnouncementPayload` (`filter_library.go`) so multi-NLRI UPDATEs where a prefix-list or family strip drops only a subset of prefixes retain the denied subset with unmodified pre-policy attributes while the accepted subset proceeds on the ordinary rail.
  - Added `prefixCounts.retained` in `internal/component/bgp/reactor/session_prefix.go` to bound unique retained keys (accepted + import-filtered) against configured `maximum` for `rs-client true` offered-mode families independently of unmatched withdrawals.
  - Updated `internal/component/bgp/plugins/adj_rib_in/` (`rib.go`, `rib_complex.go`, `rib_commands.go`, `rib_eligibility.go`, `rib_replay_path.go`, `rib_validation.go`, `rib_validation_config.go`, `metrics.go`, `register.go`) to subscribe to `update-filtered direction received`, store `ImportFiltered` and `FilterStep` independently of RPKI `Ineligible`, exclude `ImportFiltered` from `walkReplayRoutes`, `replayPathCommand`, `routeEligible`, `applyToInstalled`, `suspendForRevalidation`, and `disableValidationCommand`, expose `filtered`, `filter-step`, and `filtered-routes` in CLI commands, and maintain the per-peer `ze_adjribin_routes_filtered{peer=...}` Prometheus gauge.
  - Extracted `routeServerStartupSubscriptions()` and `(*routeServer).handleStructuredEvent` in `internal/component/bgp/plugins/rs/server.go` so `TestRouteServerTakesNoDeliveryOfFilteredUpdates` directly proves subscription and dispatch isolation.
  - Added compiled functional test observers (`internal/test/fixture/register_adj_rib_in_filtered.go`, `plugin_adj_rib_in_filtered.go`) and four promoted `.ci` functional tests (`test/plugin/adj-rib-in-filtered-{visible,status,not-replayed,absent-for-plain-peer}.ci`), verified 80/80 under 64 CPU burners.
  - Added protocol interop scenario `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr/` with Go checker `internal/le/interoplab/bgp/check_rfc7947_filtered.go` + unit test `check_rfc7947_filtered_test.go`.
  - Closed `RFC7947-2.1-1` (`{gap}` removed in `rfc/short/rfc7947.md`), recorded discrimination proofs (`rfc/discrimination/rfc7947/RFC7947-2.1-1.json`), independently re-audited `RFC7947-2.1-1` (`enforced`), resealed shifted `RFC7947-x-{1,2,3,5}` audits, updated `rfc/extraction/rfc7947.json`, updated `features/ixp-route-server-dynamic-peers.md`, and created news fragment `website/changes/fragments/rfc7947-adj-rib-in-accepts-filtered-updates.md`.

### Bugs Found/Fixed
- **Multi-NLRI partial-policy-rejection silent drop (found by FRR 10.7.1 interop `task-388`)**: FRR packs equal-attribute prefixes (`10.99.0.0/24` and `10.99.1.0/24`) into a single BGP UPDATE. When `filter_prefix` rejected only `10.99.1.0/24`, `policychain` returned `OutcomeAccept` with a rewritten `modifiedPayload` containing only `10.99.0.0/24`, so `notifyOneMessage` never emitted `update-filtered` for the stripped `10.99.1.0/24`. Fixed by extracting the stripped announcement subset with unmodified pre-policy attributes (`wireu.KeepCIDREntries`, `filter_library.go::buildDeniedAnnouncementPayload`, `reactor_notify.go::partialDenied`). Covered by `TestKeepCIDREntriesMatchesComplementOfDrop`, `TestKeepCIDREntriesAddPathAndErrors`, `TestRFC7947PartialMultiNLRIDenialRetained`, `TestRFC7947PartialMultiNLRIDenialSupersedesAccepted`, and `bgp-route-server-filtered-adj-rib-in-frr`.
- **Pre-existing stale source anchor in `docs/guide/route-reflection.md:494`**: `rsRelaySendGranted` had been renamed to `rsRelaySendProcessName`; fixed during `./le doc check all`.

### Documentation Updates
- `docs/architecture/plugin/rib-storage-design.md` (`RawRoute`, `walkReplayRoutes`, `replayPathCommand`, `routeEligible`, `applyToInstalled`) — updated in `44411d90c3`.
- `docs/architecture/api/process-protocol.md` (`EventUpdateFiltered`), `docs/architecture/api/ipc_protocol.md` (`EventKindUpdateFiltered`), `docs/architecture/api/commands.md` (`filtered`, `filter-step`, `filtered-routes`) — updated in `44411d90c3`.
- `docs/plugin-development/metrics.md` (`ze_adjribin_routes_filtered`, `internal/component/bgp/plugins/adj_rib_in/metrics.go`).
- `docs/architecture/config/syntax.md`, `docs/guide/configuration.md` (`update-filtered` / `update-filtered-received` receive token).
- `docs/guide/command-reference.md` (`show bgp adj-rib-in` and `show bgp adj-rib-in status` fields).
- `docs/guide/plugins.md`, `docs/features/plugins.md`, `docs/plugin-overview.md`, `docs/guide/status.md` (`bgp-adj-rib-in` `update-filtered` subscription and filtered count).
- `docs/architecture/core-design.md` (`reactor_notify.go::enqueueFilteredIngress`, `filter_library.go::buildDeniedAnnouncementPayload`, `session_prefix.go::prefixCounts.retained`).
- `docs/comparison.md` (pre-policy Adj-RIB-In for route-server clients).
- `docs/architecture/testing/interop.md` (`bgp-route-server-filtered-adj-rib-in-frr`).
- `docs/guide/route-reflection.md` (updated route-server Adj-RIB-In description and fixed stale anchor `rsRelaySendProcessName`); `docs/guide/rpki.md` verified via grep to contain zero `show bgp adj-rib-in` output blocks.
- `features/ixp-route-server-dynamic-peers.md` and `rfc/extraction/rfc7947.json` (removed stale Section 2.1 `{gap}` prose and open spec path citation).
- `plan/release/fixes/spec-bgp-selected-path-replay-ordering.md`, `plan/release/validation/spec-feature-maturity-declared.md`, `plan/release/validation/spec-rfcgate-6-supported-extraction-signoff.md` (switched citations to bare stem `spec-rfc7947-adj-rib-in-accepts-filtered-updates`).
- `./le doc check all`: 0 errors on all files touched by this spec (3170 digest anchors verified).

### Deviations from Plan
- **Partial multi-NLRI policy rewrite retention added**: The original plan assumed ingress policy rejection always took `!res.accept` or `ingressPolicyWithdrawals`. Real FRR 10.7.1 interop revealed that prefix-list rejection on a multi-NLRI UPDATE returns `OutcomeAccept` with a subset `modifiedPayload`; adding `KeepCIDREntries` and `buildDeniedAnnouncementPayload` fulfills RFC 7947 §2.1 for real-world BGP speakers.
- **RFC 7947 `x-4` and `x-5` kept untouched**: Owner ruled on 2026-11-22 that both rows are already `[SHOULD]` and proven in `rfc/short/rfc7947.md`, so no downgrade or correction paragraphs were needed.
- **Selected-path replay ordering split out**: Owner approved splitting the 2026-11-16 delayed selected-path replay proposal into `plan/release/fixes/spec-bgp-selected-path-replay-ordering.md`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | Assumed ingress policy denial of a prefix always enters `!res.accept` or `ingressPolicyWithdrawals` | When FRR packs accepted and rejected prefixes into one multi-NLRI UPDATE, `filter_prefix` + `policychain` returns `OutcomeAccept` with a rewritten subset `modifiedPayload` | Real FRR 10.7.1 interop `bgp-route-server-filtered-adj-rib-in-frr` failed at Assertion 5 (`has 1 Adj-RIB-In entries, want 2`) | Added `wireu.KeepCIDREntries` and `filter_library.go::buildDeniedAnnouncementPayload` + reactor unit tests; re-ran interop to GREEN |
| assumption | Original 2026-08-30 spec assumed default offered-mode prefix counting bounded retained filtered routes | Unmatched withdrawals decrement offered count while retained denied entries stay in memory | 2026-11-15 test-campaign counterexample | Added `prefixCounts.retained` unique-key bound in `session_prefix.go` + `TestRFC7947FilteredRetainedPopulationBound` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Pre-policy Adj-RIB-In retention for `rs-client true` denied UPDATEs (full, mixed, replacement, partial multi-NLRI) | Done | `internal/component/bgp/reactor/reactor_notify.go:339-370`, `reactor_notify.go:787-858`, `filter_library.go:404-508`, `plugins/adj_rib_in/rib.go:464-480` | Original pre-policy attrs preserved on `update-filtered` rail |
| Omission from Loc-RIB, live forwarding, bulk/paged replay, single-path replay, and RPKI validation promotion | Done | `plugins/adj_rib_in/rib.go:1276`, `rib_replay_path.go:99`, `rib_eligibility.go:101`, `rib_validation.go:81` | `ImportFiltered` independent of `Ineligible`; `bgp-rs` does not subscribe to `update-filtered` |
| Operator CLI visibility (`filtered`, `filter-step`, `filtered-routes`) across json/yaml/table + Prometheus gauge | Done | `plugins/adj_rib_in/rib_commands.go:236-285`, `plugins/adj_rib_in/metrics.go:1-85` | `ze_adjribin_routes_filtered{peer=...}` maintained O(1) |
| Unique retained-population prefix bound unaffected by unmatched withdrawals | Done | `internal/component/bgp/reactor/session_prefix.go` (`prefixCounts.retained`) | Verified by `TestRFC7947FilteredRetainedPopulationBound` |
| `RFC7947-2.1-1` leaves `{gap}`, proven both polarities with discrimination + audit | Done | `rfc/short/rfc7947.md`, `rfc/discrimination/rfc7947/RFC7947-2.1-1.json`, `rfc/audit/rfc7947/RFC7947-2.1-1.json` | 0 `rfc7947` violations in `./le rfc check`; 206/206 signed |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestRFC7947ClientDeniedUpdateRetainedInAdjRIBIn`, `TestRFC7947ReceivedLoopDenialRetained`, `test/plugin/adj-rib-in-filtered-visible.ci` | Loop-denied UPDATE retained with `filtered: true, filter-step: "loop"` |
| AC-2 | Done | `TestRFC7947ReceivedImportFinalDenialRetained`, `TestRFC7947PartialMultiNLRIDenialRetained`, `test/plugin/adj-rib-in-filtered-status.ci`, `bgp-route-server-filtered-adj-rib-in-frr` | Policy-chain denial (full + partial multi-NLRI) retained with `filter-step: "peer-policy-chain"` |
| AC-3 | Done | `TestRFC7947FilteredReceivedExcludedFromSelection`, `test/plugin/adj-rib-in-filtered-visible.ci`, `bgp-route-server-filtered-adj-rib-in-frr` | Absent from `show bgp rib` and never sent live to other RS clients |
| AC-4 | Done | `TestRFC7947FilteredReceivedWithheldOnClientJoin`, `TestAdjRIBInFilteredLifecycleAndViews`, `test/plugin/adj-rib-in-filtered-not-replayed.ci`, `bgp-route-server-filtered-adj-rib-in-frr` | Reconnecting/late RS client receives accepted prefix only |
| AC-5 | Done | `TestRFC7947OrdinaryPeerDeniedUpdateAbsentFromAdjRIBIn`, `TestRFC7947PartialMultiNLRIDenialRetained/ordinary-peer`, `test/plugin/adj-rib-in-filtered-absent-for-plain-peer.ci` | Non-`rs-client` denied UPDATE stays absent |
| AC-6 | Done | `TestAdjRIBInFilteredLifecycleAndViews`, `TestRFC7947ReceivedMixedDenialRetainsAnnouncement`, `TestRFC7947PartialMultiNLRIDenialRetained` | Explicit withdrawal removes filtered entry |
| AC-7 | Done | `TestAdjRIBInFilteredLifecycleAndViews`, `TestAdjRIBInFilteredRoutesMetric` | Peer-down clears filtered entries and zeroes gauge |
| AC-8 | Done | `test/plugin/adj-rib-in-filtered-visible.ci`, `test/plugin/adj-rib-in-filtered-status.ci` | Verified across `json`, `yaml`, and `table` pipes |
| AC-9 | Done | `TestAdjRIBInFilteredLifecycleAndViews`, `test/plugin/adj-rib-in-filtered-status.ci`, `bgp-route-server-filtered-adj-rib-in-frr` | `total-routes: 2, filtered-routes: 1` |
| AC-10 | Done | `TestReceiveGrantAcceptsUpdateFilteredToken`, all 4 `test/plugin/adj-rib-in-filtered-*.ci` | `update-filtered` / `update-filtered-received` loads and completes |
| AC-11 | Done | `TestReceiveGrantAcceptsUpdateFilteredToken` | Misspelled token rejected naming valid tokens |
| AC-12 | Done | `./le rfc check` (0 violations on `rfc7947`) | `RFC7947-2.1-1` proven both polarities, discriminated, audited `enforced` |
| AC-13 | Done | `rfc/short/rfc7947.md` | `RFC7947-x-4` and `RFC7947-x-5` remain untouched at `[SHOULD]` |
| AC-14 | Done | `./le rfc extraction report` | `backlog: 0, enrolled: 206, signed: 206, unsigned: []` |
| AC-15 | Done | `TestRFC7947FilteredRetainedPopulationBound` | Denied A + unmatched withdrawal B + denied C bounded at max=1; replace/withdraw/accept verified |
| AC-16 | Done | `TestRFC7947ReceivedMixedDenialRetainsAnnouncement`, `TestRFC7947FilteredReplacementClearsSelectedRoute`, `TestRFC7947ReceivedValidationCannotAcceptPolicyDenial`, `TestRFC7947PartialMultiNLRIDenialSupersedesAccepted` | Mixed UPDATEs, denied replacements, and RPKI validation independence all pass |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| Conformance & reactor lifecycle tests | Done | `internal/component/bgp/reactor/rfc7947_filtered_conformance_test.go`, `rfc7947_filtered_received_test.go`, `rfc7947_filtered_selection_test.go`, `rfc7947_filtered_partial_test.go` | Pass via `./le test unit pkg path ./internal/component/bgp/reactor/ run TestRFC7947` |
| Wire subset complement tests | Done | `internal/component/bgp/wireu/nlri_cidr_test.go` | Pass via `./le test unit pkg path ./internal/component/bgp/wireu/ run TestKeepCIDREntries` |
| Adj-RIB-In store, replay, CLI & metric tests | Done | `internal/component/bgp/plugins/adj_rib_in/rfc7947_filtered_test.go` | Pass via `./le test unit pkg path ./internal/component/bgp/plugins/adj_rib_in/` |
| RS subscription isolation test | Done | `internal/component/bgp/plugins/rs/rfc7947_filtered_subscription_test.go` | Pass via `./le test unit pkg path ./internal/component/bgp/plugins/rs/` |
| RPC EventKind round-trip & boundary test | Done | `pkg/plugin/rpc/event_kind_filtered_test.go` | Pass via `./le test unit pkg path ./pkg/plugin/rpc/ run TestEventKindUpdateFiltered` |
| Interop checker binding & predicate unit tests | Done | `internal/le/interoplab/bgp/check_rfc7947_filtered_test.go` | Pass via `./le test unit pkg path ./internal/le/interoplab/bgp/ run "TestRFCInteropCheckerBindings\|TestRouteServerFilteredAdjRIBInPredicates"` |
| Four functional `.ci` tests | Done | `test/plugin/adj-rib-in-filtered-{visible,status,not-replayed,absent-for-plain-peer}.ci` | 80/80 stress-repro under 64 CPU burners + 4/4 promoted PASS (7.3s) |
| FRR + BIRD protocol interop scenario | Done | `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr/` | 1 passed, 0 failed against FRR 10.7.1 + BIRD 3.4.0 |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 42 files from `44411d90c3` | Done | Landed in HEAD (`44411d90c3`) |
| `internal/component/bgp/plugins/adj_rib_in/{metrics.go,register.go,rib.go,rib_complex.go,rib_eligibility.go,rib_validation.go,rfc7947_filtered_test.go}` | Done | Prometheus gauge + lint/RFC-naming cleanup |
| `internal/component/bgp/wireu/{nlri_cidr.go,nlri_cidr_test.go}` & `reactor/{filter_library.go,filter_nlri_delta.go,filter_ordered.go,reactor_notify.go,rfc7947_filtered_partial_test.go}` | Done | Multi-NLRI partial-denial retention fix + tests |
| `internal/component/bgp/plugins/rs/rfc7947_filtered_subscription_test.go` | Done | Added RFC untagged header |
| `internal/test/fixture/{register_adj_rib_in_filtered.go,plugin_adj_rib_in_filtered.go}` & `test/plugin/adj-rib-in-filtered-*.ci` | Done | 4 functional tests + compiled observers |
| `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr/*` & `internal/le/interoplab/bgp/{check_rfc7947_filtered.go,check_rfc7947_filtered_test.go,check_special.go,bgp_test.go}` | Done | Interop scenario + registered checker |
| Docs, RFC audit/extraction, feature declaration, citing specs, news fragment | Done | All Checklist Yes rows updated; `./le doc check all` clean on touched files |

### Audit Summary
- **Total items:** 29 (5 task requirements, 16 ACs, 8 test groups)
- **Done:** 29
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations: multi-NLRI partial-reject retention added; `x-4`/`x-5` kept untouched per 2026-11-22 ruling; selected-path replay ordering split to `spec-bgp-selected-path-replay-ordering.md`)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| G-1: Route-server client denied UPDATEs (full, mixed, replacement, partial multi-NLRI) retained in Adj-RIB-In marked import-filtered | Interop + functional + unit | **RED -> GREEN interop**: `bgp-route-server-filtered-adj-rib-in-frr` failed at Assertion 5 (`peer 172.30.0.3 has 1 Adj-RIB-In entries, want 2`, `task-388`) before `buildDeniedAnnouncementPayload`, then **PASSED** (`1 passed, 0 failed`, recording `bgp-route-server-filtered-adj-rib-in-frr-NZY7RKHQFOWXH3D4NMBK2CSFLJ.jsonl`, `task-535`) against FRR 10.7.1 + BIRD 3.4.0; `adj-rib-in-filtered-visible.ci` PASS (1.0s); `TestRFC7947PartialMultiNLRIDenialRetained` PASS |
| G-2: Import-filtered entry never selected or re-advertised (live, bulk/paged replay, single-path replay, post-RPKI valid) and denied replacement withdraws prior accepted path | Interop + functional + unit | `bgp-route-server-filtered-adj-rib-in-frr` Assertions 7, 8, 9–12 (BIRD omits `10.99.1.0/24` live AND after `birdc disable/enable ze_peer` reconnect replay); `adj-rib-in-filtered-not-replayed.ci` PASS (7.3s, 80/80 under 64-burner load); `TestRFC7947FilteredReceivedExcludedFromSelection`, `TestRFC7947FilteredReplacementClearsSelectedRoute`, `TestRFC7947ReceivedValidationCannotAcceptPolicyDenial` PASS |
| G-3: Operator distinguishes filtered from accepted entries at CLI across all pipes + metric | Functional + unit | `adj-rib-in-filtered-visible.ci` (`json`, `yaml`, `table` pipes + `ze_adjribin_routes_filtered{peer="127.0.0.1"} == 1`) & `adj-rib-in-filtered-status.ci` (`total-routes: 2, filtered-routes: 1`) PASS (4/4 promoted, 80/80 stress-repro); `TestAdjRIBInFilteredRoutesMetric` PASS |
| G-4: Retained entries bounded by unique keys against prefix maximum, unaffected by unmatched withdrawals | Unit | `TestRFC7947FilteredRetainedPopulationBound` PASS (`session_prefix.go::prefixCounts.retained`) |
| G-5: `RFC7947-2.1-1` leaves `{gap}` and passes `./le rfc check` with discrimination and audit | RFC gate | `./le rfc check` reports 0 violations for `rfc7947`; `rfc/discrimination/rfc7947/RFC7947-2.1-1.json` verified; `rfc/audit/rfc7947/RFC7947-2.1-1.json` independently judged `enforced`; `./le rfc extraction report` `backlog: 0, enrolled: 206, signed: 206` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| Delayed selected-path replay ticket/cold-worker ordering | Orthogonal 2026-11-16 investigation split out by owner ruling on 2026-11-22 | `plan/release/fixes/spec-bgp-selected-path-replay-ordering.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/rfc7947-adj-rib-in-accepts-filtered-updates-f1a36472-8747-465b-8316-df184028508b.md` |
| `./le spec review check` | clean (53 files verified) |
| Rounds | 2 (Round 1: `e8bce97a`, 1 BLOCKER, 2 ISSUE, 2 NOTE; Round 2: `dbcc88c5`, CLEAN: 0 BLOCKER, 0 ISSUE, 1 NOTE) |
| Reviewer lenses used | logic+wiring, security+guard+edge-cases, spec-compliance+tests+docs |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | `suspendForRevalidation` (`enable-validation refresh`) moved `ImportFiltered` routes into `r.pending`, zeroing the gauge and allowing RPKI reject to delete them | `internal/component/bgp/plugins/adj_rib_in/rib_validation_config.go:72` | Skipped `route.ImportFiltered` in `suspendForRevalidation` and `disableValidationCommand`; added `TestAdjRIBInSuspendForRevalidationKeepsImportFiltered` |
| 2 | ISSUE | `TestRouteServerTakesNoDeliveryOfFilteredUpdates` asserted a hardcoded local slice and did not drive `bgp-rs`'s structured event handler | `internal/component/bgp/plugins/rs/rfc7947_filtered_subscription_test.go:22` | Extracted `routeServerStartupSubscriptions()` and `handleStructuredEvent()` in `rs/server.go` and tested both directly |
| 3 | ISSUE | `docs/features.md` (derived, gitignored) still showed pre-fix Section 2.1 text locally | `features/ixp-route-server-dynamic-peers.md` -> `docs/features.md:32` | Verified `docs/features.md` is gitignored/derived (`.gitignore:219`) and `features/ixp-route-server-dynamic-peers.md` carries no stale gap prose or open spec path |
| 4 | NOTE | `buildDeniedAnnouncementPayload` skipped silently on `!ok` from `keepNLRI` and `rfc7947_filtered_partial_test.go` covered IPv4 only | `internal/component/bgp/reactor/filter_library.go:455` | Returned `modifyFailureMalformed` on `!ok` from `keepNLRI`; added IPv6 `MP_REACH_NLRI` test `TestRFC7947PartialMPReachDenialRetained` |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/plugins/adj_rib_in/metrics.go` | Yes | `-rw-r--r-- 2335 Nov 23 15:45` |
| `internal/component/bgp/plugins/adj_rib_in/rfc7947_filtered_test.go` | Yes | `-rw-r--r-- 11703 Nov 23 19:19` |
| `internal/component/bgp/plugins/rs/rfc7947_filtered_subscription_test.go` | Yes | `-rw-r--r-- 1378 Nov 23 19:20` |
| `internal/component/bgp/reactor/rfc7947_filtered_conformance_test.go` | Yes | `-rw-r--r-- 13286 Nov 23 02:40` |
| `internal/component/bgp/reactor/rfc7947_filtered_partial_test.go` | Yes | `-rw-r--r-- 7229 Nov 23 16:52` |
| `internal/component/bgp/wireu/nlri_cidr_test.go` | Yes | `-rw-r--r-- 1541 Nov 23 16:44` |
| `pkg/plugin/rpc/event_kind_filtered_test.go` | Yes | `-rw-r--r-- 1669 Nov 22 15:36` |
| `internal/test/fixture/register_adj_rib_in_filtered.go` & `plugin_adj_rib_in_filtered.go` | Yes | `-rw-r--r-- 563` & `9744 Nov 23 15:51` |
| `test/plugin/adj-rib-in-filtered-{visible,status,not-replayed,absent-for-plain-peer}.ci` | Yes | `2381`, `2407`, `3539`, `2067` bytes |
| `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr/{ze,frr,bird}.conf` | Yes | `1417`, `351`, `185` bytes |
| `internal/le/interoplab/bgp/check_rfc7947_filtered.go` & `_test.go` | Yes | `10007` & `2323` bytes |
| `website/changes/fragments/rfc7947-adj-rib-in-accepts-filtered-updates.md` | Yes | `-rw-r--r-- 1500 Nov 23 15:53` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3, AC-4, AC-5 | Retention on `rs-client true`, omission from Loc-RIB/live/replay, plain peer absent | `./le test unit pkg path ./internal/component/bgp/reactor/ run TestRFC7947` exit 0 (`task-531`); `./le --name rfc7947-draft test bgp plugin --pattern adj-rib-in-filtered` 4/4 PASS 7.3s (`task-594`); `bgp-route-server-filtered-adj-rib-in-frr` 1 passed, 0 failed (`task-535`) |
| AC-6, AC-7, AC-8, AC-9 | Withdrawal/peer-down cleanup, json/yaml/table pipe rendering, status filtered-routes & gauge | `./le test unit pkg path ./internal/component/bgp/plugins/adj_rib_in/` exit 0 (`task-571`) + `TestAdjRIBInFiltered` exit 0 (`task-585`); `adj-rib-in-filtered-{visible,status}.ci` PASS (`task-594`) |
| AC-10, AC-11 | `update-filtered` receive grant loads/completes, misspelling refused | `TestReceiveGrantAcceptsUpdateFilteredToken` PASS (`task-531`); `EventKindUpdateFiltered` PASS (`task-571`) |
| AC-12, AC-13, AC-14 | `RFC7947-2.1-1` proven/discriminated/audited; `x-4`/`x-5` `[SHOULD]`; extraction signed | `./le rfc check` 0 `rfc7947` violations (`task-600`); `./le rfc extraction report` 206/206 signed |
| AC-15, AC-16 | Unique retained prefix bound; mixed UPDATEs, denied replacements, RPKI independence | `TestRFC7947FilteredRetainedPopulationBound`, `TestRFC7947ReceivedMixedDenialRetainsAnnouncement`, `TestRFC7947FilteredReplacementClearsSelectedRoute`, `TestRFC7947ReceivedValidationCannotAcceptPolicyDenial`, `TestRFC7947PartialMultiNLRIDenialSupersedesAccepted` PASS (`task-531`) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| Loop-denied UPDATE on `rs-client true` -> `show bgp adj-rib-in` + `ze_adjribin_routes_filtered` | `test/plugin/adj-rib-in-filtered-visible.ci` | Yes (PASS 1.0s; 80/80 under load) |
| Policy-chain denied UPDATE on `rs-client true` -> `show bgp adj-rib-in status` | `test/plugin/adj-rib-in-filtered-status.ci` | Yes (PASS 3.0s; 80/80 under load) |
| Peer-up replay to second `rs-client true` omits filtered route | `test/plugin/adj-rib-in-filtered-not-replayed.ci` | Yes (PASS 7.3s; 80/80 under load) |
| Loop-denied UPDATE on non-`rs-client` peer stays absent | `test/plugin/adj-rib-in-filtered-absent-for-plain-peer.ci` | Yes (PASS 3.0s; 80/80 under load) |
| FRR multi-NLRI partial policy denial + BIRD live & reconnect replay | `test/interop/scenarios/bgp-route-server-filtered-adj-rib-in-frr/` | Yes (1 passed, 0 failed; RED->GREEN verified) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `grep ImportFiltered internal/component/bgp/plugins/adj_rib_in/` confirms guards in `walkReplayRoutes`, `replayPathCommand`, `routeEligible`, `applyToInstalled`; verified by `TestAdjRIBInFilteredLifecycleAndViews` and `TestRFC7947ReceivedValidationCannotAcceptPolicyDenial` |
| A-2 | confirmed | `session_prefix.go::prefixCounts.retained` verified by `TestRFC7947FilteredRetainedPopulationBound` |
| A-3 | confirmed | `reactor_notify.go::enqueueFilteredIngress` withdrawal-first FIFO ordering verified by `TestRFC7947FilteredReplacementClearsSelectedRoute` and `TestRFC7947PartialMultiNLRIDenialSupersedesAccepted` |
| A-4 | confirmed | Promoted functional suite (4/4) and full `adj_rib_in` / `rs` unit suites pass with added `filtered`, `filter-step`, and `filtered-routes` keys |
| A-5 | confirmed | `rfc/short/rfc7947.md` keeps `x-4` and `x-5` at `[SHOULD]`; `./le rfc check` has 0 `rfc7947` violations; `./le rfc extraction report` 206/206 signed |
| A-6 | confirmed | All `SetStartupSubscriptions` sites inspected; only `bgp-adj-rib-in` subscribes to `update-filtered`; `TestRouteServerTakesNoDeliveryOfFilteredUpdates` PASS (`task-571`) |
| A-7 | confirmed | `TestAdjRIBInFilteredLifecycleAndViews` proves `ingestPosition` advances cleanly across filtered UPDATEs (`task-571`) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Wire format / metadata / test infra (No) | No BGP wire encoding, route metadata map, or harness changes; verified by `git diff --stat` | Yes |
| Architecture & API docs (`rib-storage-design.md`, `process-protocol.md`, `ipc_protocol.md`, `commands.md`, `core-design.md`) | Verified against `rib.go`, `rib_commands.go`, `events.go`, `enums.go`, `reactor_notify.go`, `filter_library.go`, `session_prefix.go`; `./le doc check all` 0 errors on touched files | Yes |
| Operator & plugin docs (`metrics.md`, `syntax.md`, `configuration.md`, `command-reference.md`, `plugins.md`, `features/plugins.md`, `plugin-overview.md`, `status.md`, `comparison.md`, `route-reflection.md`, `interop.md`, `features/ixp-route-server-dynamic-peers.md`) | Verified against `metrics.go`, `register.go`, `rib_commands.go`, `check_rfc7947_filtered.go`; grep confirmed `docs/guide/rpki.md` has no `show bgp adj-rib-in` sample output; `./le doc check all` clean | Yes |
| RFC ledger & news (`rfc/short/rfc7947.md`, `docs/features/rfc-status.md`, `rfc/extraction/rfc7947.json`, `website/changes/fragments/rfc7947-adj-rib-in-accepts-filtered-updates.md`) | Verified by `./le rfc check` (0 `rfc7947` violations) and `./le rfc extraction report` (206/206 signed) | Yes |

## Core Insight

Real BGP speakers pack equal-attribute prefixes into multi-NLRI UPDATEs, where per-prefix import policies return `OutcomeAccept` with a rewritten subset payload rather than `OutcomeReject`. Satisfying RFC 7947 Section 2.1 therefore requires retaining not only whole-UPDATE and mixed-UPDATE denials, but also the exact dropped-NLRI complement (`wireu.KeepCIDREntries`) with unmodified pre-policy attributes alongside the accepted subset.
