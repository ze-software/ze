# Spec: bgp-mrai-pacing

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P3 |
| Release reason | Unapproved optional positive-interval capability; preserve the effective zero default and do not infer mandatory default-path failure from the policy candidates |
| RFC gaps | RFC4271-9.2.1.1-2, RFC4271-9.2.1.1-3 |
| Depends | - |
| Phase | Design rework; selected deferral proposed, legacy retention and quota decisions remain open |
| Updated | 2026-11-18 |

## Task

Decide Ze's route-advertisement interval policy and turn the test campaign's
draft pacing probes into tests of that adopted policy. Keep this work visible
for later review while the remaining RFC test batches proceed, as the owner
instructed on 15 November. No timer design or nonzero default is approved here.

No existing MRAI feature spec was found. The current three tests in
`internal/component/bgp/reactor/rfc4271_mrai_candidate_test.go` propose the
suggested 30-second eBGP interval, an immediately eligible first advertisement,
and jitter between 22.5 and 30 seconds. They are explicitly untagged policy
candidates, not proof that current output violates a mandatory RFC obligation.
They compiled and ran in
`tmp/session/2026-11-14-210034/scratch/job-rfc-campaign-new-batch-reds-37b87864.log`.
Each fails at its early-route assertion after the first positive route reaches
the socket. Later coalescing and cancellation assertions are not yet reached.
Retain that log until an adopted-policy run supersedes the candidate result.

## Authority

- RFC 4271 Section 9.2.1.1:
  "Two UPDATE messages sent by a BGP speaker to a peer that advertise
  feasible routes and/or withdrawal of unfeasible routes to some common
  set of destinations MUST be separated by at least
  MinRouteAdvertisementIntervalTimer."
  → Constraint: with a positive interval, announcements and withdrawals for
  common destinations share the spacing requirement. Conservative batching
  across unrelated destinations is permitted; do not require unrelated
  prefixes to transmit immediately.
- RFC 4271 Section 9.2.1.1:
  "If new routes are selected multiple times while awaiting the expiration of
  MinRouteAdvertisementIntervalTimer, the last route selected SHALL be
  advertised at the end of MinRouteAdvertisementIntervalTimer."
  → Constraint: queued work must retain the latest selection and cannot
  resurrect a route after withdrawal.
- RFC 4271 Section 10 suggests 30 seconds for eBGP and five seconds for iBGP.
  It also states: "An implementation of BGP MUST allow the HoldTimer to be
  configurable on a per-peer basis, and MAY allow the other timers to be
  configurable."
  → Constraint: configurable MRAI and the suggested default are distinct
  policy choices; neither may be promoted into an unconditional MUST.

## Existing Mechanisms and Research

| Mechanism | Current evidence | Design constraint |
|-----------|------------------|-------------------|
| Session timers | `internal/component/bgp/fsm/timer.go` documents MRAI as absent | Do not count an unused timer declaration as pacing |
| Actual writer | `internal/component/bgp/reactor/session_write.go`, `sendUpdateCounted` | Tests observe real UPDATE bytes after a real Peer handshake |
| Current candidate tests | Three tests cover spacing, latest replacement and withdrawal cancellation | Bind them to the adopted interval before RFC tags or passing proof claims |
| FRR reference | Upstream master `bgpd/bgp_fsm.c`, `bgp_adjust_routeadv`, inspected 2026-11-15 | Zero bypasses pacing; a positive interval schedules remaining time. This is an unpinned research snapshot, not a versioned interop result |
| BIRD reference | Upstream master `proto/bgp/attrs.c`, `bgp_get_prefix` and `bgp_rt_notify`, inspected 2026-11-15 | Existing bucket replacement illustrates coalescing; no nonzero default was established by this review |

## Questions for Later Design Review

1. Which fixed/default interval should Ze adopt, and should operators be able
   to configure it? A zero interval must be distinguished from an absent or
   ineffective positive interval.
2. Should the first selected route be immediately eligible? The draft assumes
   this, but that assumption is not a requirement extracted from the RFC.
3. Which existing destination ownership and writer lifecycle should hold pending
   work, so peer teardown and withdrawals cancel it without bypassing ownership?
4. Which versioned FRR/BIRD scenario will prove the chosen wire behavior?

## Validation Owed

The three candidate tests ran and failed at their early-route assertion after
the first valid route reached the socket, as recorded above. After policy
approval, cover the chosen nonzero interval, latest selection at expiry,
withdrawal cancellation and teardown/reconnect ownership. Keep positive live
route controls. Record actual passing-baseline/failing-break evidence before
adding requirement tags or upgrading any mandatory row. No full RFC support or
completed mandatory batch is claimed by this proposal.

## Design proposal for owner review, 18 November 2026

This addition supersedes the old master-only research references and the open
default question above. The earlier candidate observations remain historical
evidence. No implementation, positive interval policy, nonzero default, Ready
transition, RFC annotation change or coverage credit is authorized by this
design. The main session presents the recommendations and unresolved choices
to the owner. This research did not create or claim another work item.

The proposed feature lets an operator request a positive route-advertisement
interval on a peer while leaving the existing effective zero interval as the
default. Thirty seconds must work as an ordinary configuration value; the
design must not depend on a millisecond test interval fitting an existing RPC
timeout. A positive value constrains announcements and withdrawals on every
actual UPDATE rail for that peer. Selection, current desired state and source
receipt publication continue immediately.

Fixed-zero spacing has no genuine too-early counterexample: every nonnegative
wire separation meets zero. There is no positive pending-expiry window for the
conditional latest-selection clause either. An artificial delay can break an
immediate-output software contract while still satisfying the RFC minimum.
Consequently zero-default tests alone cannot earn discrimination proof for
positive waiting or latest-at-expiry behavior. This is not a justification to
remove either requirement from the denominator or change its applicability.
Missing a literal FSM timer object is not evidence of a default-path defect.

## Required Reading

| Reading | Actionable constraint |
|---------|-----------------------|
| `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md` | → Constraint: keep engine eligibility distinct from the selecting plugin's current desired state; a forked plugin receives the same behavior through the declared protocol. |
| `ai/doors/plugin-infra.md`, `docs/architecture/api/process-protocol.md`, selected-route operations | → Constraint: a deferred eligibility receipt cannot masquerade as a completed write, suppression or absence; update typed and JSON consumers together. |
| `ai/doors/config-yang.md`, `ai/patterns/config-option.md` | → Constraint: add an ordinary peer/group/template YANG option with units, schema default and native range validation; no hidden environment-only policy. |
| `docs/architecture/update-building.md`, selected final completion | → Constraint: materialized sections and source-write tokens currently remain owned until synchronous final flush; pacing cannot retain them in a detached callback. |
| `docs/architecture/bgp/structural-forwarding.md`, direct and queued rails | → Constraint: the direct RS rail runs on a source read goroutine and must fall back rather than wait; held replay and forward batches cannot sleep while holding the socket path. |
| `docs/architecture/plugin/rib-storage-design.md`, Bounded selected output | → Constraint: extend the existing current/pending maps, exact Session worker and settlement barriers; do not introduce a second route-payload inventory. |
| `plan/learned/DESIGN-HISTORY.md`, existing batching and registration constraints; `plan/journal/unwired-feature.md`, outbound batching entry | → Constraint: the existing batching leaf is not an implemented MRAI setting; do not adopt its name or count it as a working timer. |
| `rfc/full/rfc4271.txt`, Sections 9.2.1.1 and 10; `rfc/short/rfc4271.md` | → Constraint: keep the existing two requirement identities, preserve zero, constrain common destinations and both announcement/withdrawal directions, and do not restart an established deadline on each new selection. |
| `ai/doors/core.md`, `internal/core/clock/clock.go` | → Constraint: reuse injectable clock/timer interfaces; keep transient eligibility state Session-owned rather than persisted or registered globally. |
| `ai/doors/le-tooling.md`, test and interop ownership | → Constraint: new functional and interop scenarios run through existing runners; source research and draft candidate reds are not runtime conformance proof. |

**Key insights:** the hard selected-call bound is thirty seconds; waiting today
also holds a selected-stream lock and an active source-write token. Therefore
moving a sleep outside `writeMu` alone does not solve the ownership problem.
Default zero must remain the allocation-free bypass. Positive pacing needs a
nonblocking final-admission decision and waiting in an already-owned producer
or destination worker.

## Current Behavior

| Source read | Current producing behavior and preservation constraint |
|-------------|-------------------------------------------------------|
| `reactor/session_write.go`, `sendUpdateCounted`, `writeMessageHeld` | The comment requires the writer to consume the borrowed UPDATE synchronously under `writeMu`. Acquisition arms a socket budget, followed by write and flush. The generic message writer also recognizes UPDATE and routes it through originated admission. Preserve borrowed ownership and truthful final counts. |
| `reactor/session_write_mutex.go`, `acquireWriteContext`, `releaseWrites` | The actual socket budget starts after mutex acquisition; release clears that holder's deadline. Pacing time must not consume this write budget or extend it silently. |
| `reactor/reactor_api_selected.go`, `SelectRoutes`, `selectRoute` | The batch holds `session.selectedMu`; received selection admits a source-write token before materialization and completes it after final writing. A thirty-second timer inside this call would retain revocation blockers. |
| `reactor/source_receipt.go`, `sourceReceiptWrite`, `complete` | The producer says the token completes after write/flush or cancellation and must not acquire a newer lease for an old queued operation. Release a deferred attempt before waiting and re-admit only a newly checked current operation. |
| `pkg/plugin/rpc/cancel.go`, `SelectedCallContext`; `pkg/plugin/sdk/sdk_selected.go`; `plugin/server/dispatch_selected.go` | The SDK, server and reactor reapply a thirty-second cap, even with a longer caller context. A normal thirty-second interval cannot safely be spent inside this RPC, and increasing a caller timeout alone does not remove the cap. |
| `reactor/selected_flush.go`, `writeSelected` | Current owner and current selection are checked under the final socket lock. Several transformed bodies and old-slot retirements may belong to one key. Preserve that authority and partial-output accounting. |
| `reactor/session_write.go`, `writeUpdateBody` | Final policy, normalization, ownership filtering and PATHS-LIMIT decisions precede ownership recording and buffer write. Eligibility must see the actual outgoing destinations after transformations, before ownership/counters/bytes for that frame commit. |
| `reactor/forward_rs_direct.go`, `tryDirectWriteNoFlush` | Its source-read-goroutine fast path uses TryLock and otherwise falls back to the pool. A positive interval must never block this goroutine; zero keeps the existing path. |
| `reactor/forward_pool.go`, `fwdBatchHandler`; `reactor/peer_initial_sync.go`, initial replay holds | The worker and replay currently hold `writeMu` across multiple messages. Positive pacing must release the hold and finish per-frame accounting before any eligibility wait, retaining existing initial/EOR ordering fences. |
| `reactor/peer_send.go`, `sendUpdateOwnedWithSplit` | The splitter lends each chunk only through its synchronous callback. Waiting may retain that callback's borrowed chunk, but no asynchronous queue may outlive it; retry must not resend completed chunks. |
| `bgp/plugins/rib/rib_selected_dispatch.go` | Each destination Session has one worker, a bounded pending-key set, current immutable desired selections, acknowledgment uncertainty and final receipt settlement. Its one-second retry timer is failure retry, not MRAI. |
| `reactor/session_write.go`, `SendRawMessage`, `finishRawWrite` | Accountable UPDATEs use the ordinary body admission; explicit opaque diagnostics remain allowed under their existing policy and session-reset rules. Preserve those operator features and distinguish unknown destination sets conservatively. |
| `bgp/plugins/cmd/raw/raw.go`, `handleRaw`; `bgp/types/reactor.go`; `reactor/reactor_api.go`, `peer_send.go`, `session_write.go`, `SendRawMessage` | The command has `CommandContext.Context()` but passes only peer, type, payload and sender. Every raw-send layer is context-free. A cancellable positive wait requires actual caller plumbing through this chain; a timer inside the current Session method cannot observe request cancellation. These first two paths are relative to `internal/component/`. |
| `reactor/forward_pool.go`, `safeBatchHandle`, `dispatchOverflow`, `fwdSupersedeKey`; `reactor/forward_body.go`, `ownOverflowBodies` | Returning from the handler runs every item's done callback and releases its buffers/recovery. Overflow supersedes equal bytes, not destinations, and copies to the heap if its body-pool handle is absent or too small. The overflow slice is explicitly unbounded. Neither safe deferred retention nor bounded latest-destination state exists here today. |
| `reactor/reactor_api_batch.go`, `sendBatchUpdate`, `sendOwnRouteWithdrawal`, `sendBatchUpdatePreFiltered` | Direct sends borrow builders through return; alternate branches transfer owned bytes into the forward queue. Callers increment sent counts on nil error or inspect `ownRouteWithdrawn`; prefiltered groups consume policy removals before the survivor. Deferral cannot be returned as nil, nor retried as an undifferentiated whole group. |
| `reactor/bufmux.go`, `MixedBufMux`, `combinedBudget`; `reactor/forward_path_id.go`, `fwdPathKeyFor` | MixedBufMux budgets body blocks, not arbitrary heap metadata. The separate combinedBudget supports atomic block reservation for its participating BufMux instances; it is not currently a Session cooldown quota. Native keys longer than the inline array allocate an overflow string. A scalar map is not charged by obtaining an unrelated body handle. |
| `reactor/config.go`, timer parsing; `reactor/peer_settings_apply.go` | Existing timer settings arrive through parsed peer configuration and explicit reload classification. The proposed leaf needs group inheritance, dynamic-peer copying, equality and application coverage rather than an unused declaration. |

Source paths in the table are relative to `internal/component/bgp/` unless
they begin `pkg/` or `plugin/`; the latter names
`internal/component/plugin/server/dispatch_selected.go`.

**Behavior to preserve:** effective zero pacing, all export kinds and explicit
raw diagnostics, typed/raw/cached/native/RS/RR writers, policy removals,
ADD-PATH, MP families, splitting, refresh/EOR order, source-generation and
destination-Session fencing, existing configuration defaults and bounded
socket failure. No policy is removed to make a conformance figure improve.

**Behavior proposed only when explicitly configured positive:** repeated output
for common destinations becomes ineligible until its effective deadline;
ordinary selected changes coalesce to the latest current state while waiting.

## Existing Mechanisms

| Mechanism | Where | What it does today | Proposed use |
|-----------|-------|--------------------|--------------|
| Current selected state and one destination worker | `internal/component/bgp/plugins/rib/rib_selected_dispatch.go`, `selectedDispatch` | Coalesces desired keys, owns one request, cancels/joins exact Sessions | Extend with earliest eligible wake and fair ready-key selection; retain no second payload queue. |
| Forward worker and overflow ownership | `internal/component/bgp/reactor/forward_pool.go`, `fwdPool`, `fwdItem` | Owns queued bodies/handles and completion callbacks; overflow is unbounded and byte deduplication is not latest-destination coalescing | Reuse the worker and explicit ownership-move pattern; bounded admission and current-intent replacement still need the design below. Do not describe current storage as bounded. |
| Exact final ownership | `selected_write.go`, `selected_flush.go`, `source_receipt.go` | Checks current owner/source/selection, retires actual old slots | Revalidate every attempt after waiting; deferred means not complete, including after partial output. |
| Native destination keys | `forward_path_id.go`, `fwdPathKeyFor`; registered NLRI splitters | Identifies actual families/native destinations and path IDs | Derive pacing identity from actual outgoing destination with Path Identifier excluded; paths to the same destination share spacing. Preserve RD/opaque family identity. |
| Session write serialization and observed frame completion | `session_write_mutex.go`; `selected_refresh_eor.go`, `observedBGPWriter.complete` | Serializes and observes complete real frames | Extend Session admission; positive mode flushes an admitted UPDATE frame before committing its cooldown. Zero retains current buffering. |
| Peer operation/replay ordering | `peer_send.go`, refresh markers; `peer_initial_sync.go` | Keeps requested markers behind preceding route output | Preserve fences while releasing socket holds for pacing. |
| Injectable clocks and retirement | `internal/core/clock/clock.go`; `reactor/session.go`, `SetClock`; `fsm/timer.go`, `Retire` | Deterministic waits and generation-safe timer retirement | Reuse a cancellable Session-clock timer per existing waiting worker; no per-prefix goroutines or required FSM timer object. |
| Selected protocol registration | `pkg/plugin/rpc/selected.go`; `internal/component/plugin/server/dispatch_registry.go` | One typed/JSON operation and complete result validation | Extend result vocabulary for deferred eligibility through the same registration and both transports. |
| Request cancellation | `internal/component/plugin/server/command.go`, `CommandContext.Context` | Supplies the trusted request context, falling back to server lifetime | Plumb it through the raw command's currently context-free send chain before allowing a positive wait. Preserve sender authority as a separate argument. |
| Policy replay ownership move | `reactor/forward_pool.go`, `requeuePolicyReplay` | Owns channel bodies before parking, transfers the whole item, and zeroes the original so safeBatchHandle cannot complete it | Reuse this transfer discipline for a deferred item, not the policy replay fence or its unbounded slice as a pacing quota. |
| Buffer reservation primitives | `reactor/bufmux.go`, `combinedBudget.tryReserve`, `releaseBytes`; `MixedBufMux.setByteBudget` | Bounds participating buffer allocations only | A cooldown allocator needs an explicit admission/release contract. Do not treat these two independent budgets as interchangeable or silently alter their operator meaning. |

## Versioned primary-source comparison

| Version and deciding source | Verified behavior | Design implication |
|-----------------------------|-------------------|--------------------|
| [FRR frr-10.7.1, bgpd.h](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgpd.h), default route-advertisement macros | Both eBGP and iBGP values are zero. | Zero is an interoperable baseline choice, not an invented thirty-second MUST. |
| [FRR frr-10.7.1, bgpd.c](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgpd.c), `peer_create`, `peer_advertise_interval_set` | Initialization installs those defaults; the setter accepts zero through six hundred seconds. | The proposed option supports ordinary thirty-second policy and a bounded seconds range without changing the default. FRR's range is a recommendation, not an RFC requirement. |
| [FRR frr-10.7.1, bgp_fsm.c](https://github.com/FRRouting/frr/blob/frr-10.7.1/bgpd/bgp_fsm.c), `bgp_adjust_routeadv` | Its zero branch cancels the timer and schedules generation. Positive pacing derives remaining time from the last update and does not extend an earlier scheduled deadline. | Preserve zero bypass and a non-sliding pending deadline. |
| [BIRD v3.4.0, attrs.c](https://github.com/CZ-NIC/bird/blob/v3.4.0/proto/bgp/attrs.c), `bgp_update_prefix` | Pending membership is removed from the old bucket before replacement. | Reuse latest desired state rather than enqueue every historical selection. |
| [BIRD v3.4.0, packets.c](https://github.com/CZ-NIC/bird/blob/v3.4.0/proto/bgp/packets.c), `bgp_schedule_packet` | It schedules output as soon as possible, respecting existing output and the event loop. | No numerical BIRD MRAI default or positive timer guarantee is claimed from this path. |

These pinned sources were fetched and read during this design research. They
are deciding implementation comparisons, not an executed interoperability run.

## Alternatives and recommendation

| Approach | Benefit | Failure or cost | Recommendation status |
|----------|---------|-----------------|-----------------------|
| Synchronous eligibility wait before socket acquisition | No new selected result variant; naturally retains direct caller input until return | Not sufficient inside today's selected RPC: thirty-second cap, selected-stream lock and active source-write token. Moving before lease admission still occupies the one RPC and prevents its worker from submitting newer desired state. Supporting long intervals would require changed call bounds and wake/requery semantics. | Reject as the uniform selected design. Retain as an adapter for synchronous direct callers only when no active write lease/lock is held. |
| Explicit deferred eligibility plus existing owner workers | Selected calls remain bounded; pending keys stay current; no borrowed payload survives the attempt | Changes the public receipt contract and requires every reader to distinguish deferral from final completion. Legacy workers additionally need a bounded current-intent/admission and parent-completion design; a retry sentinel alone is insufficient. | Recommended selected direction; all-writer completeness unresolved and not owner-approved. |
| One conservative peer-wide interval | Scalar deadline and simple unknown-identity handling | Unrelated prefixes and split UPDATE frames contend for the same slot; a thirty-second setting could serialize a full table one frame per interval. Continuous arrivals need fairness to avoid unbounded postponement. | Valid alternative, not recommended as the normal keyed policy. |
| Per-destination interval, common value per peer | Unrelated destinations remain eligible and grouping remains useful | Needs expiring scalar cooldown keys even after a withdrawal; aliasing and unknown raw data require conservative handling | Recommended, not owner-approved. |

## Proposed configuration and decisions awaiting approval

| Decision | Concrete recommendation | Approval state |
|----------|-------------------------|----------------|
| Enable optional positive pacing | Add peer/group/template `timer route-advertisement-interval` | Not adopted; default remains zero. |
| Type and bounds | Unsigned integer seconds, zero through six hundred, default zero; native YANG range, units, help and description | Proposed bounded policy; thirty seconds is a supported value, not a required default. |
| First advertisement | Immediately eligible for a destination without prior output in that Session | Recommended for convergence; RFC does not force this choice. |
| Granularity | Actual emitted family/native destination, independent of source and Path Identifier | Recommended; explicit peer-wide alternative above remains reviewable. |
| Jitter | Keep configured base distinct from the effective interval; reuse `fsm.Jitter`'s existing uniform factor from 0.75 through 1.0. Sample once after a successful UPDATE frame, share that effective interval across its destinations, and expose the fixed eligibility deadline. Deterministic tests use the same local sampler seam pattern as existing timer tests. | Recommended positive-mode policy, not adopted; no jitter policy changes at zero. |
| Reload | Treat interval changes as negotiated-independent hot settings, recompute eligibility from last successful output and the new interval; zero wakes waiting work immediately. Rollback applies the same rule. | Recommended; avoid destroying current routes or inheriting an obsolete Session deadline. |
| Retirement deadline shorter than eligibility | Cancel new source writes immediately. If required retirement cannot finish within its existing deadline, close only the exact destination Session holding that output before reporting completed retirement. Do not send an early withdrawal or claim a pending one is final. | Material operational trade-off requiring explicit approval: closing the recipient also withdraws unrelated routes. Alternative is an explicitly longer retirement budget for opted-in positive pacing, not a hidden generic timeout increase. |

The strongest design challenge is the last row: normal thirty-second pacing
and short source/owner retirement budgets cannot both promise an UPDATE
withdrawal has completed in five seconds. The owner must choose the bounded
close fallback or approve a longer positive-mode retirement completion budget.
This proposal does not silently select either operational policy.

## Data Flow

### Entry and transformation

1. The existing YANG tree supplies the peer/group/template interval and normal
   settings resolution supplies its Session snapshot; zero takes today's path.
2. Selection and ownership publication continue immediately. Existing selected
   workers choose current ready keys rather than retaining intermediate route
   versions. Legacy queued producers currently retain FIFO operations, not a
   bounded current-intent set; their positive-mode admission remains an open
   design gate described below.
3. A positive-mode attempt materializes existing export/normalization/ownership
   decisions. Before a frame's ownership recording, counters or bytes commit,
   the Session derives its actual outgoing destinations and checks cooldowns.
   Suppressed output consumes no pacing interval. Mixed withdraw/announce keys
   in one UPDATE are one frame, not two advertisements.
4. Ineligibility is a distinguished no-new-frame result, not a socket error.
   Release staged write guards, active source/origin write tokens, pool
   references belonging only to that attempt and `selectedMu` before returning
   a deferred selected receipt. That receipt carries a bounded relative wait,
   the exact requested identity/revision and any existing partial-output
   uncertainty; it confirms no completed replacement or retirement.
5. The existing selected worker keeps the key pending, waits on its own clock
   or a publication/cancellation wake, then reconstructs the latest current
   value. Synchronous adapters wait outside all locks and active write leases,
   retain input only until the original call returns, and retry against the
   same captured Session. Raw adapters first need the context plumbing below.
   Legacy pool workers must transfer ownership before returning from a handler;
   today's overflow is not an accepted bounded deferred store.
6. Eligibility is rechecked under the final socket lock after source, owner,
   destination Session, policy and current-selection checks. Another output
   rail may have consumed the same destination's slot while this caller waited.
7. In positive mode each admitted UPDATE frame is flushed before committing
   its next eligibility time. Socket failure retires the Session as today.
   Never start cooldown from a buffered acceptance that may remain unflushed;
   never delay KEEPALIVE, NOTIFICATION, OPEN or ROUTE-REFRESH control traffic.
8. A deferred attempt following earlier completed frames preserves actual
   Adj-RIB-Out ownership and the existing possible/uncertain acknowledgment
   state. Reconciliation determines the unsent current suffix from actual
   output; it cannot replay a completed splitter callback blindly.

### Eligibility state and boundedness

| Field | Lifetime and meaning |
|-------|----------------------|
| Configured base interval | Peer setting; zero is the no-pacing bypass. |
| Effective deadline per outgoing destination | Captured Session only; derived from successful output and its effective interval. Later selections do not slide that deadline. |
| Withdrawn cooldown key | Scalar identity/deadline retained until eligibility, even when no route remains; expiry removes it. No withdrawn attribute history. |
| Unknown raw destination frontier | Conservative Session-wide deadline: an opaque diagnostic waits behind all current cooldowns and temporarily constrains every destination afterward. Preserve bytes and existing opaque-reset behavior. |
| Existing pending/current route state | Existing selecting owner's current/pending maps; legacy operation retention needs the bounded admission design below. Do not duplicate route payloads in the eligibility map. |
| Timer/wakeup | One earliest-deadline timer per existing waiting worker, not per destination key; canceled/joined through captured lifetime. |

Cooldown state needs its own explicit admission, reservation and release
contract. The previous suggestion to charge entries to the existing forward
budget is rejected: MixedBufMux owns body blocks, while fwdPathKey can own a
variable overflow string. Neither a body handle nor the current Stats answer
accounts for map capacity, expiry indexes, key bytes or parent descriptors.

| Reservation operation | Required proposed behavior |
|-----------------------|----------------------------|
| Preflight | Derive canonical keys into attempt-local bounded scratch. Determine the incremental retained key bytes and container/expiry capacity before allocating or writing; Path Identifier is excluded from pacing identity. |
| Reserve | Before frame admission, reserve the whole required capacity atomically against a declared Session limit and a reactor aggregate limit. Failure chooses the conservative frontier before any bytes or ownership commit; it never writes first and hopes to record cooldown later. |
| Commit | After the frame's successful flush, publish its sampled deadlines under the same Session serialization and transfer reservation ownership to the retained entries. A deferred/no-output attempt releases its provisional reservation. A socket failure retires the captured Session and releases its state. |
| Release | Expiry, collapse, positive-to-zero apply and Session destruction release the exact retained capacity once. Shrinking a limit cannot erase live deadlines or reuse still-retained bytes. |
| Representation | A bounded slot/index store plus explicitly sized key backing is one reviewable option; include allocated capacity and allocator/container overhead, not only logical key length. Reusing fwdPathKey without accounting its string allocation is insufficient. The representation and quota values remain a design gate. |

The scalar fallback is preallocated Session state. Collapse live keys to their
latest existing deadline, release their accounted storage, and freeze that
frontier until expiry. While it is live, all accountable UPDATE frames wait;
new selections and newly arriving destinations cannot extend it. At expiry,
resume a fair eligible-work snapshot before admitting later arrivals. This
preserves spacing conservatively but may delay unrelated output. It supplies
no bound on retained route payloads; that is a separate legacy-admission gate.

Two concrete quota directions remain for review: reserve a distinct scalar
allowance with explicit Session/reactor caps, or extend an aggregate allocator
so both scalar storage and body blocks reserve actual capacity under a named
total. The latter changes the meaning and readers of existing pool limits and
requires an explicit migration disposition in reactor.New and weightTracker;
there is no such shared interface today. Recommend the distinct allowance to
keep the present pool semantics intact. Its numeric caps, distribution among
Sessions and limit-reduction behavior must be fixed before Ready. A permanently
peer-wide scalar deadline avoids keyed allocation but retains the full-table
serialization cost recorded above; it does not solve pending-payload bounds.

Explicit opaque diagnostics are a distinct evidence boundary: the existing
writer can deliberately send bytes whose destinations or framing cannot be
accounted, then resets that Session. A conservative pre-write frontier cannot
prove per-destination spacing inside an arbitrary multi-message diagnostic
payload. Preserve that operator contract and test its bytes/reset independently;
do not count it as proof of valid UPDATE pacing. Every accountable UPDATE rail,
including valid raw body and full-packet input, still shares the same actual
destination gate. Independent review must preserve this distinction rather
than quietly refuse all diagnostics or publish unknown bytes as conformance.

### Deferred result and caller completion

| Field / outcome | Proposed contract |
|-----------------|-------------------|
| Deferred eligibility outcome | Known selected-result variant; never equivalent to Written, Suppressed, Absent or a transport failure. |
| Requested identity and revision | Echo the existing exact family/key/revision. A revision already superseded uses the existing stale-revision outcome instead of a pacing receipt. |
| Remaining wait | Positive relative milliseconds, rounded upward, bounded by the configured effective interval and recomputed from the final Session clock. No absolute timestamp crosses machine clock domains. Zero on completed outcomes; invalid or out-of-bound values fail result validation. |
| Earlier possible output | Preserve the dispatcher's existing possible/uncertain ownership. Deferral after an earlier completed frame is not a claim that the key emitted nothing, nor that replacement/retirement finished. |
| Worker response | Leave key pending, release attempt-local resources and wait outside the call. At wake, choose the latest desired value and requery/revalidate current ownership. A publication wake can trigger reconsideration but cannot reset the Session deadline. |
| Barrier response | Remain unsettled until actual final receipt or captured Session closure. An expiring caller deadline reports outstanding work; it does not delete pending state or return success. |

The deferred variant evolves the existing final-only selected contract
explicitly. Update its SDK/bridge comments, schema, result validator,
frame-budget sizing and selecting consumers in the same implementation. Do not
catch a pacing sentinel as an ordinary error and retire a healthy Session or
fail a recovery key: that sentinel means the current frame wrote nothing.
Completed earlier frames remain accounted independently. Both typed and JSON
dispatch must release their admitted request before a deferred wait begins.

Synchronous adapters retain the supplied input only while their original call
remains live, never beyond its return. They wait before starting an active
source/origin write token, or unwind an attempted token and staging completely
before waiting. Each attempt retains the captured destination Session; it
cannot look up and write through a replacement Session. A caller whose real
context ends returns cancellation/error without subsequently waking into a
write. A source-generation/owner stop must wake a legacy deferred worker at
once so its existing reference/completion release is not held until a long
timer. Preserve cumulative source acknowledgment order when releasing a
superseded queued item.

### Raw request plumbing and synchronous batch completion

The raw command does not yet satisfy the preceding cancellation contract.
The proposed repair carries CommandContext.Context through BGPReactor's
SendRawMessage, reactorAPIAdapter, Peer and Session, including every interface
implementation and test double. The handler continues to pass Sender
separately; cancellation grants no authority. Capture the destination Session
once at Peer entry. Each later eligibility attempt rechecks that incarnation,
the attached sender's permission and final export admission before writing;
never select a replacement Session as a side effect of retry. Request or
Session cancellation wakes the wait, returns an error and leaves no timer
callback capable of writing after return.

Internal route-refresh callers in reactor_api and reactor_api_forward and the
queued refresh marker in peer_send must use their existing operation/Session
lifetime when migrated to a context-aware primitive. They remain control
messages and never acquire a route cooldown. A compatibility context-free
wrapper must not be used by an UPDATE-producing command; retaining one for
internal control callers would need a documented lifetime, not Background as
an accidental substitute for a request. Raw StatusDone still requires that
the synchronous operation finished under its existing suppression/diagnostic
contract. It cannot mean merely parked for a later eligibility callback.

| Batch producer and consumer | Borrowed/completion contract that the adapter must preserve |
|-----------------------------|------------------------------------------------------------|
| announceBatchToPeers to sendBatchUpdate | attrHandle/nlriHandle and the per-unit UPDATE are reused after return. Keep a synchronous borrow through a cancellable no-lock wait, or transfer fully owned bytes by the existing explicit queue branch. The caller's sent count includes accepted targets, including the existing already-held-route case; preserve that contract, without relabeling accepted work as a completed socket frame. A no-write pacing sentinel cannot enter queueRetiredAnnounce as an ordinary failed send. |
| Batch withdrawals to sendBatchUpdate or queueBehindForwards | Keep the request's captured sent-owner, source policy epoch and replay authority, and the initial-replay ordering exception. Do not turn temporary ineligibility into nil success on the direct branch; accepted queued ownership keeps its existing meaning and remains owed until settled. |
| Stale unowned withdrawal to sendOwnRouteWithdrawal | ownRouteWithdrawal restricts the final ownership decision; ownRouteWithdrawn answers whether a matching local path passed that decision. Carry that accumulated result across completed split chunks and return it only with final success. A deferred attempt cannot report a matching path withdrawn. |
| Stale withdrawal or modified/kept survivor to sendBatchUpdatePreFiltered | UPDATE sections, encoded body and removals remain borrowed through return or explicit overflow transfer. Preserve the decided prefilter/export epoch; do not rerun only half a policy group under a newer decision. The two removal children precede the effective survivor and share the parent's fences and completion. |
| sendStalePolicyRemovals recursion and build-failure cleanup | The group is bounded by policyRemovalBodiesMax. Track which child frames actually completed; a pacing refusal before a write cannot clear owner.policyRemovals as though it was a failed partial write. Once children have completed, retry only the unsent suffix. Do not run cleanup again for already consumed children or replay an already completed split callback. |

For these direct branches, recommend a synchronous adapter rather than a
second public deferred batch protocol: it returns final success, existing
suppression or genuine cancellation/failure while retaining the caller's
borrow. Its eligibility wait happens before active source/origin admission or
after completely unwinding the attempted admission and socket hold. The
original request's finite lifetime remains authoritative even when shorter
than thirty seconds. A timed-out direct operation reports unfinished work;
it does not silently become an asynchronous operation. This choice may make
an opted-in positive operation fail its existing request deadline and needs
owner-facing documentation; it is not a reason to enlarge the selected RPC
cap. The live context and source authority of each batch caller must be
confirmed before adopting this adapter for that caller.

### Legacy forward retention: unresolved design gate

The existing pool is a worker to extend, not a bounded latest-route scheduler.
safeBatchHandle's unconditional defer completes and releases every nonzero
item. Merely returning on ineligibility loses work. Merely sleeping inside
fwdBatchHandler retains the socket hold and prevents this worker from
incorporating later intent. Moving items to today's overflow preserves bytes
but neither bounds them nor coalesces changes to one destination.

The following ownership transition is the minimum acceptable proposed pool
extension. It has not been accepted as a complete legacy design:

| Transition | Ownership and completion requirement |
|------------|--------------------------------------|
| Attempt becomes ineligible before any frame | Release active write admissions and writeMu. Transfer the entire item, including provenance, recovery, policy children and done, to the same captured worker's pending ownership. Own borrowed channel bodies before transfer and zero the original item, following requeuePolicyReplay's move discipline. No done/release/recovery completion occurs at that point. |
| Earlier frames completed | Record a committed-frame cursor plus the unsent policy/body/chunk suffix. The existing writeBody ordinal is not proof of completion and cannot stand in for this cursor. Keep actual output ownership and partial uncertainty; never re-offer the completed prefix. |
| New source intent arrives | Continue draining/admitting new intent while the worker's one earliest-eligibility timer is pending. Replace only the superseded unsent portion, with source/generation/authority, family/native key, ingress path where applicable and replay/policy fence identity checked. Different sources or ADD-PATH paths cannot be merged merely because pacing shares a destination. |
| Mixed-destination item partly superseded | Keep the unaffected destinations and indivisible policy-removal ordering. A parent completion needs all its surviving pieces terminal or explicitly subsumed; one newer destination does not authorize releasing the whole old parent. Any descriptor/index points into existing owned item storage rather than copying another route payload. |
| Terminal completion or captured Session close | Run each parent's completion and release exactly once after all surviving pieces settle, or the captured Session definitively closes. Preserve recovery errors and refresh/EOR/barrier ordering; cache Release alone is not a claim that bytes were written. Source cancellation wakes the worker promptly and resolves stale ownership before releasing references. |

Implementing the third and fourth transitions requires a destination-intent
index and parent-piece completion accounting that do not exist today.
fwdSupersedeKey hashes whole raw bodies, re-encoded items have no such key,
and dispatchOverflow guards identical-byte supersession by authority,
Session, source generation, policy epoch, provenance and initial-replay fence.
Dropping those guards is not a solution. A mixed UPDATE can mention several
destinations while one parent owns a single done and recovery admission;
the bounded two-child policy group adds another ordering dependency.

Before Ready, choose and finish one of these designs in this proposal:

| Direction | What still has to be designed | Trade-off |
|-----------|------------------------------|-----------|
| Extend the existing destination worker with bounded current-intent descriptors | Specify the exact source/path/fence replacement relation; ownership of split mixed frames and parent completions; hard reservation for item metadata and retained bytes including heap fallback; admission/cancellation at saturation; and fair progress with live arrivals. The index references owned fwdItems and replaces only unsent pieces. | Reuses the worker and payload owner, but adds substantial semantic state. This is the candidate direction, not an assertion that a small requeue patch suffices. |
| Reconcile desired forwarding from an existing authoritative producer | Identify the actual current owner for every RS/RR/native/raw-forward path and a wake/requery protocol that holds no borrowed UPDATE, locks or source lease across the wait. Imperative operator raw commands keep synchronous byte semantics rather than silently becoming selected state. | Can avoid historical payload accumulation only where current authoritative state really exists; the selected RIB solution cannot be assumed to cover all other producers. |

A peer-wide cooldown simplifies eligibility but fixes neither of these
ownership problems. FIFO delay alone also does not establish last-selection
at expiry for a producer that selected newer state while its older operation
waited. Conversely, silently replacing an imperative raw command with later
bytes changes its operator contract. Those boundaries must be decided from
the actual producer rather than one blanket queue rule. No new unbounded
payload queue or extra worker is authorized; if bounded admission would stop
a source read loop from processing withdrawals or control traffic, that
admission design must be reworked rather than called backpressure and accepted.

### Changed-Wire Census

**Search evidence:** inspected production Go under the BGP reactor, RIB plugin,
plugin server, SDK/RPC, and their documented carriers. Terms included
`writeRawUpdateBody`, `writeUpdateBody`, `bufWriter.Write`, `holdWrites`,
`sendUpdateHeld`, `tryDirectWriteNoFlush`, `SelectRoutes`, `SelectedWritten`,
`SelectedCallContext` and `admitWrite`. Known controls were the ordinary
selected final writer and `SendRawMessage`'s independent buffer-write branch.
The BGP packet format remains unchanged; timing and the selected RPC result
vocabulary change. A second literal search for `select-routes` and
`SelectRoutes` included shell, Python, CI, JavaScript, TypeScript and JSON
under the repository, including interop and website consumers, excluding only
vendor dependencies, task scratch, installed binaries and node modules. It
found no such non-Go consumer. The same terms produced the known SDK and
server controls in the production Go census. This is a bounded source census,
not a statement that third-party clients do not exist.

| Wire | Role (writer / reader) | Carrier (file + symbol, or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------|-----------------|----------------------------------------|
| UPDATE timing | Writer | `internal/component/bgp/reactor/session_write.go`: `writeMessageHeld`, `writeUpdateGated`, `writeRawUpdateBody`, `SendAnnounce`, `sendWithdraw`, `SendRawMessage` | Typed, generic UPDATE, direct announce/withdraw, raw-body and full-packet methods reach actual output. | Common actual-output eligibility; synchronous no-lock waits where appropriate. Non-UPDATE and empty EOR frames preserve control/ordering rules. |
| UPDATE timing | Writer | `internal/component/bgp/reactor/selected_flush.go`: `writeSelectedItem`, `retireSelected`, `writeSelectedWithdrawal` | Selected announcements and retirements reach the Session writer here. | Explicit deferral; actual output ownership remains authoritative. |
| UPDATE timing | Writer | `internal/component/bgp/reactor/forward_pool.go`: `fwdBatchHandler`; `forward_policy_removal.go`: `writePolicyRemovals`; `forward_body.go`: `ownOverflowBodies` (same reactor directory) | The worker writes forwarded items and removals; overflow takes body ownership. | Preserve whole-item ownership through an explicit transfer, release the held socket path before waiting and distinguish deferral from failure. Current overflow is unbounded; bounded current intent, parent completion and saturated admission remain unresolved. |
| UPDATE timing | Writer | `internal/component/bgp/reactor/forward_rs_direct.go`: `tryDirectWriteNoFlush`; `forward_rs.go`: `reactorForwardRSSectionResult` (same reactor directory) | The route-server fast rail can write directly from the source goroutine. | Positive mode falls back before direct write effects; zero retains source-goroutine fast path. |
| UPDATE timing | Writer | `internal/component/bgp/reactor/peer_initial_sync.go`: `sendInitialRoutes`, `sendQueuedWithdrawal`; `peer_send.go`: `sendUpdateOwnedWithSplit`, `sendBodyWithSplit`, `sendRefreshMarker`, `sendQueuedRefreshMarker` (same reactor directory) | Initial/replay routes and refresh markers share split and ordered callback paths. | Release held path around eligibility, retain borrowed chunk only within synchronous callback, preserve initial and refresh markers after owed route completion. |
| UPDATE timing and batch completion | Writer and success reader | `internal/component/bgp/reactor/reactor_api_batch.go`: `announceBatchToPeers`, `withdrawBatchFromPeers`, `sendBatchUpdate`, `initBatchOwner`, `sendOwnRouteWithdrawal`, `sendBatchUpdatePreFiltered`, `sendStalePolicyRemovals`, `queueBatchBodyPreFiltered`, `sendStaleReadvertiseUnit` | These batch producers consume write results and project sent/withheld and own-route withdrawal completion. | Preserve direct borrowed lifetime versus explicit queue transfer, sent/withheld and ownRouteWithdrawn meanings, partial split cursors and removal-before-survivor ordering. No-write deferral is neither nil success nor a route-scoped build failure. |
| Raw request lifetime | Writer and result reader | `internal/component/bgp/plugins/cmd/raw/raw.go`: `handleRaw`; `internal/component/bgp/types/reactor.go`: `BGPReactor.SendRawMessage`; `internal/component/bgp/reactor/reactor_api.go`: `reactorAPIAdapter.SendRawMessage`; `peer_send.go`: `Peer.SendRawMessage`; `session_write.go`: `Session.SendRawMessage` (last two in the reactor directory); control callers in `reactor_api_forward.go` and `peer_derived_bindings.go` | `handleRaw` has the command context, while the current interface, adapter, Peer and Session raw sends do not take it; refresh/control sends use the same methods. | Carry the real request context through the UPDATE command chain, keep Sender permission separate, capture one Session and preserve control bypass. The current API has no context to cancel a pacing wait. |
| UPDATE timing | Reader | External BGP peer UPDATE receivers, including FRR/BIRD interop peers | The existing negotiated BGP session receives the final announcement, withdrawal and EOR frames. | Unchanged bytes; configured positive delays only. All existing families/export roles remain reachable. |
| Selected result | Writer | `internal/component/bgp/reactor/reactor_api_selected.go`: `SelectRoutes`; `selected_flush.go`: `writeSelected` (same reactor directory); `internal/component/plugin/server/dispatch_selected.go`: `opSelectRoutes`, `selectRoutes` | Reactor returns ordered final results; server dispatch validates them on the shared typed/JSON operation. | Add deferred eligibility without falsely final outcomes or unbounded RPC waits. |
| Selected result | Reader | `pkg/plugin/rpc/selected.go`: `SelectRoutesOutput.Validate`, `SelectRoutesExchangeSizes`; `pkg/plugin/sdk/sdk_selected.go`: `Plugin.SelectRoutes`; `pkg/plugin/rpc/bridge_selected.go`: `DirectBridge.SelectRoutes`; `internal/component/bgp/plugins/rib/rib_selected_dispatch.go`: `completeLocked` | RPC validates and budgets the answer, SDK/bridge return it, and the RIB projects completion into pending destination state. | Keep deferred keys pending, preserve actual/uncertain ownership, reserve widest new response size and release bridge dispatch before waiting. |
| Selected request/result | Writer and reader | `internal/component/plugin/selected.go`: `NewSelectedBatch`; `coordinator_selected.go`: `Coordinator.SelectRoutes`; `server/request_cancel.go`: `selectedCancellationScope`; `server/dispatch_registry.go`: `engineOpTable` (last three in the plugin directory); `pkg/plugin/rpc/bridge.go`: `DirectBridge`; `cancel.go`: `SelectedCallContext`; `selected_refresh.go`: `SelectRefreshCallContext` (last two in the RPC directory); `internal/component/bgp/plugins/rib/rib_selected_dispatch.go`: `batchLocked` | The batch snapshots authenticated input and sizes both frames; coordinator, registration, cancellation and refresh envelopes reuse the selected contract. | Preserve authenticated input snapshot and cancellation routing. Request guards and refresh sizing that share selected envelopes must account for the new result shape, even when they do not interpret its outcome. |
| Selected result | Reader | External selecting plugins using the published selected-route RPC; no repository non-Go/embedded client found by the literal census above | Third-party clients consume the same outcome vocabulary even though their source is outside this checkout. | Document the changed contract; unsupported/new unknown outcomes fail closed. No backward-compatibility layer is required before release, but all in-tree readers and envelope reusers must be tested. |

### Architectural Verification

| Check | Design answer | Evidence or remaining review |
|-------|---------------|------------------------------|
| Layers | Yes, proposed | Engine owns actual wire eligibility; plugin owns latest selection. |
| No new payload inventory | Selected direction supported; legacy design unresolved | Existing selected maps retain desired state. Legacy overflow is unbounded; its pending descriptors, hard admission and parent completion relation need design before Ready. |
| No long lock/lease hold | Yes, required | Deferral exits existing selected admission; synchronous waits happen after releasing active tokens and socket/publication locks. |
| Registration | Yes, proposed | Existing BGP YANG module and selected operation registration; no infrastructure switch names an MRAI feature. |
| Zero-copy default | Yes, required | Zero branch allocates no cooldown/timer; positive attempts use existing pools and scoped handles. |
| Full carrier census | Expanded after independent REWORK; completeness not accepted | Batch borrowed/success readers and the context-free raw chain are now explicit alongside generic UPDATE, typed/JSON and envelope consumers. A source list is not an accepted completion/retention design. |

## Acceptance Criteria

All criteria below are proposed and require owner adoption before implementation.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Leaf omitted or explicitly zero, all existing writers | The peer receives the existing useful UPDATE frames on the current writer paths; no MRAI clock advances or pacing state is allocated | Existing policy suppression, raw diagnostics, split/replay and role-specific output remain because zero preserves today's behavior. |
| AC-2 | Positive thirty-second policy; repeated common destination, every announcement/withdrawal direction | First eligible useful UPDATE reaches the peer; subsequent common-destination frames meet the known effective interval | Unrelated destinations may be grouped or delayed conservatively, but remain live and bounded; control messages bypass pacing. |
| AC-3 | Several ordinary selections and a withdrawal while waiting | At the original expiry the latest current permitted value or withdrawal reaches the socket; no intermediate resurrection | Selection and unrelated eligible output continue because the RFC constrains advertisements, not selection. |
| AC-4 | Competing typed/raw/cached/native/selected/RS/RR writes to one peer | Shared actual-destination eligibility prevents cross-rail bypass and accounts for every completed frame | Zero retains fast paths; positive direct RS falls back without blocking its source read goroutine. |
| AC-5 | Source revocation, owner stop, policy reload, destination reconnect during wait | No stale write or lease resurrection; replacement Session starts its own empty eligibility lifetime; completed retirement is truthful | Existing valid replacement source and sibling destinations continue; a forced destination close is allowed only under the retirement policy explicitly adopted above. |
| AC-6 | Deferred selected result, short RPC deadline or partial earlier output | No false written/absent receipt; old output retirement remains owed until actual completion, suppression or explicit Session closure | A newer current selection supersedes a waiting old revision; completed earlier frames retain their actual ownership. |
| AC-7 | Group inheritance, dynamic peers, reload/rollback and positive-to-zero change | The real Session consumes the configured setting; rollback restores policy and zero releases waits promptly | Existing timer settings and peer features remain unchanged; no accepted-but-ineffective leaf. |
| AC-8 | Large churn, cooldown saturation, split frames, ADD-PATH/MP and opaque diagnostics | Memory stays within declared bounds, latest useful output progresses and no key is evicted into early output | Eligible unrelated output remains permitted; unknown diagnostics use the conservative frontier without rewriting bytes. The undeclared legacy-retention and scalar quota bounds are design blockers, not implementation details presumed satisfied. |
| AC-9 | Real raw command request canceled during positive eligibility wait | The handler returns cancellation, releases its borrow and no later write occurs on the captured or replacement Session | Non-UPDATE raw controls remain unpaced; explicit diagnostic bytes/reset preserve their current contract. |
| AC-10 | Direct and queued batch callers with a paced policy-removal group and partially completed splits | Borrowed storage is not reused early, completed chunks are not repeated, surviving removals precede the survivor and all completion/count results preserve their declared meaning | Existing already-held, suppressed and accepted-queue results remain permitted; none is mislabeled as an actual new frame. |

## Wiring Test

| Entry Point | Feature code | Concrete proposed test |
|-------------|--------------|------------------------|
| Parsed peer/group/template timer leaf | `config.go`, PeerSettings and captured Session eligibility | `TestMRAIConfiguredIntervalReachesSession` |
| Real ordinary received updates and selected RPC | selected worker, deferred result, final Session writer | `TestMRAIThirtySecondSelectedLifecycle` |
| Actual direct/raw/pooled/native send callers | Shared Session actual-output gate and existing owner adapters | `TestMRAICrossRailCommonDestinationSpacing` |
| Config reload/rollback | Explicit settings application and wakeup | `TestMRAIReloadRollbackWakesCurrentWork` |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | Owner wants optional positive pacing, not only honest zero-default documentation | Existing proposal and current research request do not adopt a product policy | Unnecessary feature/complexity | Main presents this design before Ready | Awaiting owner |
| A-2 | Native destination identity can be derived for every accountable outgoing family after policy | Existing registered NLRI splitters and `fwdPathKeyFor` | Cross-family bypass or excessive conservative pacing | Independent changed-wire census plus MP/ADD-PATH tests | Source-supported; test owed |
| A-3 | Existing worker/operation budgets already bound deferred legacy work | Source review disproved this: dispatchOverflow is unbounded and ownOverflowBodies can allocate after pool exhaustion | Memory growth and premature parent/recovery release | Complete the bounded current-intent/admission design and parent-piece accounting before Ready, then bound/stress and source-cancellation tests | Rejected premise; replacement design remains open |
| A-4 | Deferred result is acceptable public selected-protocol evolution | Pre-release direct/JSON parity, existing final result validator | Third-party consumer misinterprets receipt | Full consumer census, schema/SDK tests and owner review | Awaiting design review |
| A-5 | A current authoritative source can replace every legacy waiting forward | Only the ordinary RIB selected worker has been established to provide this contract; whole-body queue dedup does not | Old selected value emitted at expiry or an imperative command silently overwritten | Per-producer intent/ownership analysis and the legacy choice table before Ready | Unproven; not accepted |
| A-6 | Existing forward-memory counters can charge arbitrary scalar cooldown keys | MixedBufMux charges body blocks; variable native key strings and containers are independent | Apparent bound excludes real heap retention | Explicit capacity reservation, allocator ownership, caps and reduction semantics before Ready | Rejected premise; distinct quota recommended |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Thirty-second wait exhausts selected RPC and holds revocation tokens | Timeouts or source release waits | Return deferred before waiting; do not enlarge the generic RPC cap. |
| R-2 | Waiting under `writeMu` blocks KEEPALIVE/NOTIFICATION | Hold expiry or teardown stall | Release per-frame holds; control-liveness tests under a thirty-second interval. |
| R-3 | New selections repeatedly slide expiry | No output under churn | Retain deadline anchored to completed output; replace desired value only. |
| R-4 | Cooldown lost after withdrawal or path-ID change | Early reannouncement | Retain scalar destination deadline until expiry, independent of source/path identifier. |
| R-5 | Partial selected/split output is treated as wholly deferred or wholly written | Duplicate prefix, false retirement or stale suffix | Reconcile actual Adj-RIB-Out and completion/uncertainty; independent per-frame cuts. |
| R-6 | A twenty-nine-second RPC/budget workaround passes tiny tests but fails real policy | Normal thirty-second fixture fails | Mandatory thirty-second fake-clock/native boundary test and actual configured interop scenario. |
| R-7 | Positive retirement exceeds existing shorter deadline | Revocation returns with old output still live | Adopt explicit bounded-close fallback or explicitly longer completion budget; never silently bypass spacing. |
| R-8 | Expiry map becomes withdrawn-route history | Memory rises with old churn | Scalar-only timed expiry, explicit budget and conservative saturation frontier. |
| R-9 | Policy changes native destinations after an early eligibility check | Cross-rail or alias early UPDATE | Final post-transform actual-output gate; reserve no eligibility from unmaterialized input names. |
| R-10 | Direct fast path or opaque diagnostics bypass the common gate | Existing writer control stays unpaced | Positive fast path falls back; opaque data uses all-destination conservative frontier; preserve zero behavior. |
| R-11 | Conservative handling is overclaimed as proof about unknown diagnostic framing | An opaque multi-frame payload is reported as fully paced | Keep actual valid UPDATE proof separate from explicit diagnostic bytes/reset; do not invent a destination set for unknown data. |
| R-12 | Raw wait outlives the command because RequestContext was dropped | Canceled peer-raw command later writes | Plumb CommandContext.Context through the real interface/adapter/Peer/Session chain; cancellation tests enter through handleRaw, not a direct Session-only helper. |
| R-13 | Returning from a pool handler completes an item that is only ineligible | safeBatchHandle calls done or closes recovery before output | Explicit whole-owner transfer and zeroing; parent-piece terminal accounting for mixed items; test all completion and buffer-return counts. |
| R-14 | Thirty-second pacing feeds unbounded overflow or heap fallback | Retained item/body/descriptor bytes grow despite MixedBufMux limit | Hard positive-mode admission plus current-intent replacement; finish source-control-safe saturation semantics before Ready. No pool token as a proxy for heap allocation. |
| R-15 | API batch nil-error or ownRouteWithdrawn result hides unfinished work | sent increments or stale cleanup completes while a suffix remains | Distinct internal ineligibility, final synchronous direct result or explicit existing queue transfer, and preserved split/removal cursor. |
| R-16 | Reservation follows flush or ignores variable keys/container capacity | A completed frame cannot install its deadline; used bytes exceed cap | Reserve all retained capacity before write, commit after flush, release once; test long native keys and concurrent rails under quota exhaustion. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if wrong? | Opted-in peers may converge slowly, send stale/early output, stall control traffic, exhaust buffers or reconnect during retirement. Zero-path regressions would affect every peer and must block implementation acceptance. |
| How is the feature disabled? | Set the interval to zero through ordinary config apply/rollback; wake pending work against current ownership. No destructive git recovery is proposed. |
| Who else owns these paths? | Ordinary advertiser work in `plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md`, refresh policy work and ongoing shared Session writer changes. Coordinate source-write windows and preserve their current ownership contracts. |

## 🧪 TDD Test Plan

| Test / scenario | Planned location | One behavior it decides | Status |
|-----------------|------------------|-------------------------|--------|
| `TestMRAIConfiguredIntervalReachesSession` | `reactor/mrai_config_test.go` | Parsed/inherited interval reaches actual captured Session | Proposed; not run |
| `TestMRAIZeroPreservesAllWriterRails` | `reactor/mrai_zero_test.go` | Existing useful output with zero and no MRAI clock advance | Proposed; not run |
| `TestMRAICommonDestinationSpacing` | `reactor/rfc4271_mrai_spacing_test.go` | Known positive effective interval across announcement/withdrawal directions | Proposed; own early-eligibility cut owed |
| `TestMRAILatestSelectionAtOriginalExpiry` | `reactor/rfc4271_mrai_latest_test.go` | Last current value at non-sliding expiry | Proposed; own stale-value and postponed-expiry cuts owed |
| `TestMRAIThirtySecondSelectedLifecycle` | `reactor/mrai_selected_test.go` | Normal thirty-second wait occurs outside RPC/write leases | Proposed; not run |
| `TestMRAICrossRailCommonDestinationSpacing` | `reactor/mrai_writer_census_test.go` | Every census rail consults the same real output deadline | Proposed; each rail needs its own reached discriminator |
| `TestMRAIControlTrafficAndRetirement` | `reactor/mrai_lifecycle_test.go` | Control traffic and adopted bounded retirement remain live during pacing | Proposed; not run |
| `TestMRAIReloadRollbackWakesCurrentWork` | `reactor/mrai_config_test.go` | Reload/rollback updates current eligibility without stale ownership | Proposed; not run |
| `TestMRAICooldownBudgetAndExpiry` | `reactor/mrai_budget_test.go` | Churn stays bounded and withdrawn keys expire without early output | Proposed; not run |
| `TestMRAIRawRequestCancellation` | Existing raw-command boundary tests plus `reactor/mrai_raw_test.go` | Canceling the actual handler request forbids any later raw UPDATE write | Proposed; context plumbing required; not run |
| `TestMRAIBatchBorrowAndPartialCompletion` | `reactor/mrai_batch_test.go` | A deferred batch preserves borrow, completed suffix position and truthful caller results | Proposed; not run |
| `TestMRAIForwardParentRetention` | `reactor/mrai_forward_test.go` | Moving a deferred mixed item preserves every parent/child/recovery owner until terminal | Proposed; blocked on the explicit legacy design; not run |
| `TestMRAIScalarReservationBeforeWrite` | `reactor/mrai_budget_test.go` | Variable key/container capacity is reserved before a successful frame can need a deadline | Proposed; quota representation/caps need design; not run |
| `mrai-configured-peer` | `test/plugin/mrai-configured-peer.ci`, registered fixture under `internal/test/fixture/` | Operator config reaches a real daemon and useful paced socket output | Proposed; requires ze-test before fixture promotion |
| `MRAI-PACING-FRR`, `MRAI-PACING-BIRD` | Existing `internal/le/interoplab/bgp/` catalogs | Configured Ze positive pacing/withdrawal/latest behavior accepted by pinned stable peers | Proposed; peer numeric default is not the oracle |

Numeric boundary plan: zero, one, thirty and six hundred seconds valid;
negative and six hundred and one rejected by schema; zero-to-positive and
positive-to-zero reloads covered. These bounds are proposed policy. Timing
tests use injectable clocks and explicit admission/capture fences, with a known
jitter sample, not an assumed wall-clock sleep. Existing three thirty-second
candidate tests remain untagged until independently reviewed rebinding to an
adopted configuration. No red candidate is weakened merely to clear a gate.

## Files to Modify or Create after approval

| Area | Files and purpose |
|------|-------------------|
| Schema/config/settings | `internal/component/bgp/yang/ze-bgp-conf.yang`; `reactor/config.go`, `peer_settings.go`, `peer_settings_apply.go`, `peer_settings_negotiation.go`, `reactor_api.go`: leaf, inheritance/copy/equality, reload and status. |
| Common output and lifecycle | `reactor/session_write.go`, `session_write_mutex.go`, `session.go`, `peer_send.go`, `peer_initial_sync.go`, `forward_pool.go`, `forward_body.go`, `forward_rs_direct.go`, `forward_policy_removal.go`, `selected_flush.go`, `reactor_api_selected.go`, `reactor_api_batch.go`: actual eligibility, borrowed/counting adapters and explicitly owned suffixes; proposed new `reactor/mrai.go` owns only scalar Session eligibility. Scalar allocation touches `bufmux.go` or a BGP-local quota implementation only after the quota design is accepted; `reactor.go` and weightTracker need review if an existing aggregate changes. Proposed `reactor/register_mrai.go` and `internal/core/diagnostic/codes.go` declare the distinct retirement fallback diagnostic and owning runtime check. |
| Raw request/context chain | `internal/component/bgp/plugins/cmd/raw/raw.go`, `internal/component/bgp/types/reactor.go`; `reactor/reactor_api.go`, `reactor_api_forward.go`, `peer_send.go`, `peer_derived_bindings.go`, `session_write.go`, all interface implementations/test doubles: request cancellation reaches the actual UPDATE wait; existing refresh/control lifetimes remain explicit. Update `docs/architecture/api/commands.md` with the same phase. |
| Selected protocol and worker | `pkg/plugin/rpc/selected.go`, `bridge_selected.go`, `selected_refresh.go`, validator/frame-size tests; `pkg/plugin/sdk/sdk_selected.go`; `internal/component/plugin/selected.go`, `coordinator_selected.go`, `server/dispatch_selected.go`, `server/request_cancel.go`, `server/dispatch_registry.go`; `internal/core/ipc/yang/ze-plugin-engine.yang`; `rib/rib_selected_dispatch.go`, `rib_selected_batch.go`, `rib_selected_barrier.go`, `rib_selected_status.go`: explicit deferral, waits, truthful settlement/status and preserved routing/envelope contracts. |
| Tests | New named files in the TDD table; existing candidate file only after independent review; fixture and interop additions through existing registrations. |
| Documentation | `docs/guide/bgp-peering.md`, `docs/guide/configuration.md`, `docs/architecture/update-building.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md`, `docs/architecture/behavior/fsm-established.md`, `features/bgp-protocol.md`; current `rfc/short/rfc4271.md` only with independent evidence judgment. |

Paths abbreviated as `reactor/` and `rib/` mean
`internal/component/bgp/reactor/` and `internal/component/bgp/plugins/rib/`.
The table is a proposed edit surface, not an assertion that all source anchors
or consumer sites have already been inventoried.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | Yes | Existing BGP module, timer leaf and selected result schema. |
| Native constraints | Yes | Seconds, default zero and proposed zero-through-six-hundred range. |
| Custom validators | N-A | No dynamic reference set; native range is sufficient. |
| CLI commands/flags | N-A | Existing configuration/status surfaces; no new command grammar. |
| CLI grammar | Yes | Existing keyword-before-value configuration path. |
| Editor autocomplete | Yes | Existing YANG-derived completion/help. |
| Functional API/config test | Yes | `mrai-configured-peer` and selected typed/JSON result tests. |
| Pipe completeness | N-A | Existing status renderer/pipes; no new output surface. |
| Environment registration | N-A | Operator policy is ordinary peer configuration, not an environment variable. |
| Runtime dependency doctor | N-A | No new host dependency, socket, binary, kernel feature or filesystem state. |
| Doctor problem classes | Yes | Actual failed selected progress uses existing `CodeDoctorBGPSelectedOutput` runtime reporting; existing forward/operation congestion reports retain their codes and owners. Proposed distinct retirement-budget fallback uses a runtime BGP-owned diagnostic, `CodeDoctorBGPMRAIRetirementBudget`, declared with its owning check in proposed `reactor/register_mrai.go`; its message names the affected captured peer and adopted completion policy. Intentional eligible-in-future work is neutral status, never an error. Invalid config is a native validation refusal, not a runtime dependency failure. |
| Metrics | Yes | Existing output/error counters keep actual completion semantics; expose configured interval, pending count and next eligibility through existing selected/peer status. No new Prometheus family proposed without a concrete reader need. |
| New BGP family/capability/attribute | N-A | Timing-only behavior; use existing registered family splitters. |

### Documentation Update Checklist

| # | Surface | Applies? | Disposition |
|---|---------|----------|-------------|
| 1 | Feature level/evidence | Yes | `features/bgp-protocol.md`; preserve zero and qualify positive support. No feature-level claim before proof. |
| 2 | Config syntax | Yes | Configuration and BGP peering guides; schema-derived syntax remains. |
| 3 | Command reference | N-A | No new operator command. |
| 4 | API/RPC | Yes | Selected deferred result contract in process-protocol and SDK documentation. |
| 5 | Plugin behavior | Yes | RIB selected worker documentation; no new plugin. |
| 6 | User guide | Yes | BGP peering guide explains seconds, zero and retirement trade-off. |
| 7 | BGP wire format | No | Bytes unchanged; timing guide and existing UPDATE writers document eligibility. |
| 8 | SDK/protocol | Yes | Typed/JSON result parity, validators, widest frame accounting and consumer census. |
| 9 | RFC behavior/proof | Yes | Existing requirement identities, evidence and independent audit; derived status files remain untracked. |
| 10 | Test infrastructure | No | Existing clock/fixture/interop runners extended, no replacement framework. |
| 11 | Daemon comparison | Yes | State pinned FRR zero/default and BIRD observed scheduling accurately; no invented numeric BIRD default. |
| 12 | Architecture | Yes | Update-building, structural-forwarding and RIB storage pages in their implementation phases. |
| 13 | Route metadata | No | No route metadata permission or feature name; deferred fields belong to typed result contract. |
| 14 | Prometheus documentation | No | No new family currently proposed; revise if implementation adds one. |
| 15 | Registration/inventory | Yes | Existing selected-operation schema/registration and fixture/interop catalogs; no generated composition-root edit by hand. |
| 16 | Source anchors | Yes | Current anchors to `writeUpdateBody`, `sendUpdateCounted`, `SelectRoutes` and direct RS were inspected. Native spec citation census and independent full carrier review remain Ready prerequisites. |
| 17 | Examples | Yes | Add omitted/zero and thirty-second peer/group examples through parsed configuration; preserve existing examples. |

## Proposed implementation order, only after owner approval

1. Wiring and failing config/transport tests: proposed leaf reaches real
   Session state; deferred result is shape-validated on both transports and
   cannot acknowledge completion. Before implementation, complete the open
   legacy admission/replacement and scalar quota designs, plus the expanded
   consumer/caller census; the later steps do not authorize guessing them.
2. Shared Session positive eligibility and expiry budget, zero bypass, actual
   frame completion, transformed-key and opaque frontier handling.
3. Selected worker earliest wake/latest-state reconciliation, partial output,
   truthful barriers, and the adopted retirement policy. No timed wait inside
   a selected RPC or active write lease.
4. Every legacy/direct/held/raw/replay adapter from the census, each with useful
   survivor, control-liveness, ownership and actual early-output mutation tests.
5. Reload/rollback, churn/bounds, real configured functional and interop proof;
   update owning docs during each phase. Record positive and negative claim
   breaks and independent RFC judgments before changing support annotations.

## Design admission and unresolved approval gates

| Challenge | Assessment |
|-----------|------------|
| Simplicity | Selected deferral reuses an actual latest-state owner. The earlier assertion that this was a complete minimal all-writer design is withdrawn: legacy bounded current-intent/parent accounting is still missing. Uniform synchronous waits remain rejected for selected RPCs by actual thirty-second and lease contracts. |
| Uniformity | Captured Sessions, source tokens, current revisions, typed/JSON operation and pooled item ownership remain authoritative. Raw context plumbing and an explicit legacy ownership move are necessary extensions; neither exists merely because selected deferral is described. |
| Performance | Zero retains direct/buffered paths and no pacing allocations. Positive scalar capacity must be reserved explicitly; long native keys, descriptor storage, current overflow heap fallback and per-frame flush cost need separate accounting. No bound or performance figure is inferred from current body-pool counters. |
| Independent completeness judgment | REWORK received. Raw cancellation chain and API-batch borrowed/count readers are now explicit; legacy bounded last-wins/parent completion and scalar allocator/quota choices remain open design blockers. No accepted all-writer or memory-bound judgment is claimed. |
| Owner adoption | Pending: optional positive feature, granularity/first output/jitter, and bounded-close versus longer retirement completion budget. Broad RFC work authorization does not adopt those product choices. |

The next design pass must finish the legacy direction and scalar reservation
choices in their tables, then obtain a fresh independent comparison against
the actual carriers. Those are engineering design gaps, not questions asking
the owner to waive coverage or choose an arbitrary implementation detail. The
main session owns any eventual product-policy dialogue. No positive-mode
implementation is authorized by this repaired proposal.

## Owned journal rows

| Class file | Date | Surface |
|------------|------|---------|

This proposal adopts no additional journal row. The existing outbound batching
leaf's ineffective configuration is recorded under its current owner; this
design does not silently claim or close it.

## Review Gate

| Run | Scope | Findings | Result |
|-----|-------|----------|--------|

No implementation Review Gate run has occurred. The immediate next step is an
independent design/census review followed by the main session's owner-facing
decision walkthrough. Keep Status design; no code, tests, generator, build,
native claim, record, stamp or configuration adoption follows from this edit.
