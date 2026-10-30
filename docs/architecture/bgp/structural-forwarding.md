# Structural Forwarding: what left the critical path

The route-server forwarding gap against BIRD on grouped input was closed by
changing structure, not by tuning constants. `docs/architecture/core-design.md`
Section 9 describes the mechanisms that resulted. This page records what they
replaced and the ordering constraint that keeps them correct.

<!-- source: internal/component/bgp/reactor/forward_body.go -- shared body building -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactor-native RS forwarding -->

## Four costs, and what replaced them

| Old cost | Replacement |
|----------|-------------|
| Forward context stored in a `sync.Map` that every structured and text dispatch path loaded from, one map hop per UPDATE | a value-carrying `workItem` passed through dispatch, holding the source peer, the message and the text payload |
| Peer-down withdrawal inventory built inline in the forward path, allocating strings and writing maps before any byte left the box | NLRI records extracted as `netip.Prefix` before forwarding, applied to the withdrawal map after forwarding |
| One `Retain(id)` per destination peer, so N destinations meant N entry points | one `retainN(id, peerCount)` per update id, fed by a pending dispatch buffer |
| Identical path attributes written as separate TCP writes | `fwdBucketMerge` at the batch-handler level merges NLRIs into fewer outbound bodies, inside the negotiated message size limit |

The forwarding builders issue one cache retain per UPDATE. The final writer
retains one compact native ownership key per advertised recipient path; it does
not retain forwarded attributes or query the selecting RIB on each UPDATE.

Both forwarding rails key body reuse on effective opaque-attribute treatment,
not on a plugin or peer role. Ordinary RFC 4271 Section 5 treatment
drops unknown non-transitive attributes and marks unknown transitive attributes
Partial. A plugin can select preservation for a destination through
`filterapi.Filter.PreserveOpaqueAttributes`; without a selector, ordinary
treatment applies. The route-server plugin selects preservation for its clients.

The selector receives the existing peer metadata at forwarding-facts refresh,
not on each UPDATE, and the resulting boolean lives in the immutable facts
snapshot. Both rails key body reuse and edit-set deduplication by that treatment.
On a materialization miss, ordinary treatment compacts the destination-owned
payload in place before dedup publishes it; a dedup hit copies those effective
bytes into the next destination's own buffer. An unmodified shared input stays
immutable: when treatment changes it, one read-pool buffer is adopted by the
received-update cache entry and returned at eviction, after the workers finish.
Preserved and already-normalized inputs keep their original bytes.
The builder never decides a peer's policy. Body-cache hits need no attribute
scan or copy. There is no second client inventory or per-update plugin lookup.
The fast rail retains its four stack slots; the general rail's cache lasts one
forward call and is bounded by its destination count.

## The ordering constraint

**`extractWireNLRIRecords` MUST run before forwarding.** Cache eviction can free
the pool buffer backing `msg.WireUpdate` once `ForwardCached` has run, so
extraction after forwarding reads freed memory. Application to the withdrawal
map happens after forwarding, when the keys are already materialized. The pooled
`nlriRecord` slice is returned once the map update completes.

Correctness of the withdrawal map depends on this split: extract while the cache
buffer is alive, apply once the bytes are gone.

## Receive validation and cache changes

`forwardUpdateCore` queries the Adj-RIB-In validation gate for each prefix and
received Path Identifier. A pending or ineligible path becomes a withdrawal;
eligible sibling NLRIs retain their attributes. The final writer sends that
withdrawal only to a destination it advertised the path to (see "Withheld
routes are withdrawn"), so a path held ineligible from its first receipt
reaches no destination, however many receipts or ValidationChange relays
follow, and one advertised before it turned ineligible is withdrawn once. A generation that has already
been replaced cannot advertise over its replacement. The cached received bytes
remain unchanged, and derived forwarding buffers belong to the same cache entry.
The forwarding API succeeds when either the eligible section or its synthesized
withdrawal is dispatched. An ineligible announcement that becomes an accepted
withdrawal is not reported as suppressed by every destination.
<!-- source: internal/component/bgp/reactor/forward_validation.go -- forwardUpdateValidated -->

A received withdrawal goes through the same engine. The live forward carries it
only once the gate no longer answers the path eligible; while Adj-RIB-In still
holds the path as eligible, the withdrawal leaves the live forward and is sent
by the ValidationChange relay that Adj-RIB-In's removal wakes (`removePending`
notes every withdrawn key, pending or installed). That relay asks for the
current path after any revalidation relay queued before it, so the withdrawal
is written after a queued announcement and never overtaken by one. A route
withdrawn while still pending is cancelled at its pending record: a verdict
that arrives later for that generation installs nothing, and the relay answers
the explicit withdrawal. A destination never advertised the route is sent
nothing on either rail, by the writer rule in "Withheld routes are withdrawn".
<!-- source: internal/component/bgp/reactor/forward_validation.go -- validationSelection.section -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_validation.go -- removePending, applyDecision -->
<!-- test: internal/component/bgp/reactor/forward_validation_pending_withdrawal_test.go TestPendingRouteWithdrawnBeforeVerdictSendsNothing, TestAnnouncedRouteWithdrawalFollowsQueuedRevalidation -->
<!-- test: internal/component/bgp/plugins/adj_rib_in/rib_validation_pending_withdrawal_test.go TestPendingWithdrawalCancelsTheVerdict, TestAnnouncedWithdrawalWakesTheRelay -->

When receive validation is enabled, or an UPDATE carries FlowSpec or AIGP,
`reactorForwardRS` defers to cached forwarding. That choice remains set on the
source peer across reconnects and configuration changes. Later attribute-free
withdrawals and replacements therefore use the same source FIFO as the earlier
announcement. Peers that never require this deferral keep the direct-write path.
The choice is recorded at receipt even when the fast path is disabled, so
enabling it cannot overtake cached work already queued for that source.

The fast path forwards in place of the route server's cached forward, so it
reaches only the destinations that forward may reach. The cached forward is
checked against the destination's `attach process <route server> { send
[ update ] }` grant (`filterPermittedPeers`). The fast path reads the same
grant: a peer's forwarding snapshot records whether it grants `send [ update ]`
to a process whose registration declares `RelaysOnRSFastPath`, resolved through
the process's `use` spelling, and both passes of `reactorForwardRSSection`
select no peer without it. bgp-rs is the plugin that declares it. The source
side reads the same declaration: the cached forward relays only what the route
server receives, and the delivery graph feeds it a source's UPDATEs only through
the source's `attach process <route server> { receive [ update-received ] }`
grant (or `update`, or `*`). The snapshot records that grant too, and the gate
in `notifyMessageReceiver` declines the fast path for a source without it, so
neither rail relays that source.
<!-- source: internal/component/bgp/reactor/peer_forward_facts.go -- rsRelaySendGranted, rsRelayReceiveGranted -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiver -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRSSection -->
<!-- source: internal/component/plugin/registry/registry.go -- RelaysOnRSFastPath -->
<!-- test: internal/component/bgp/reactor/forward_rs_permission_test.go TestReactorForwardRSHonorsRelaySendPermission, TestReactorForwardRSRetryRechecksRelaySend, TestRSFastPathGateHonorsSourceRelayReceive -->

Live cache entries and reconstructed stored-route replays retain their dispatch
peer and session generation. Source removal or re-establishment invalidates
queued dispatch, and destination workers check that generation before writing.
Stored replay resolves the **current** dispatch generation; it deliberately does
not reuse the session generation of GR-retained route bytes.

Source policy admission has a separate, globally monotonic `SourcePolicyEpoch`.
Each Peer starts with a nonzero epoch unique across Peer lifetimes. Ordinary
reconnects leave it unchanged. Receive admission captures it before running
ingress filters; delayed filtering, cache publication, forwarding workers and
reconstruction retain that original value. Stored routes, AIGP retained input,
initial sent replay and recovery candidates do not acquire the current policy
epoch merely because they are dispatched again.

The final ownership writer checks received announcements against that original
epoch under the destination's `writeMu`. Zero is unrecorded and grants no received
announcement authority. This check leaves local origins, exact-receipt removals
and current-session explicit withdrawals distinct: ordinary withdrawal ownership
still ignores policy epoch, message revision and the old advertisement's session
generation. Policy invalidation must close receive admission and settle affected
writers under the same write locks; changing an atomic epoch alone is not a
downstream completion receipt.

One predicate, `sourcePolicyCurrent`, answers the question everywhere: a local
origin (no source) is current, and received work is current only when its
nonzero original epoch equals its owner Peer's live epoch. There is no fallback
from zero to the live epoch. A plugin's sent-owner re-send carries an epoch in
the request, but the request never decides alone, since any plugin can learn
the live epoch: `sentOwnerCurrent` admits it only when the reactor's own record
of the exact receipt agrees (below), the request's epoch is the live one, and
the source Peer still exists. Where each writer applies it:

| Writer | Stale announcement | Still admitted |
|--------|--------------------|----------------|
| Forwarded (direct, cached, pool, stored replay) | Dropped before identifier binding, owing or election | Every withdrawal, stale or not |
| Exact owner (`adjOutExpected`: a plugin's sent-owner replay, resend or abort restore through `update-route`) | Dropped, judged after the revision check (`sentOwnerCurrent`): a live entry re-announces only while its own admission epoch is the live one and the request carries that epoch; a released receipt only as the abort restore of the invalidation that released it (below). An unrecorded or superseded request epoch, an entry admitted under a superseded policy, a record of a finalized or another invalidation, or a source Peer that a configuration change removed (stopping; its replacement is another Peer with its own epoch), is refused. An admitted write re-owns the entry under the request's epoch | The exact-owner removal; a local owner |
| Initial replay | `checkInitialWrite` and the writer refuse it with an error, never a policy no-op | A local replay |
| Recovery | Skipped at candidate selection like an export refusal: the next eligible survivor is written, else the owed withdrawal. A PATHS-LIMIT election runs without it and still stages its departure and demotion withdrawals. A stale survivor is never held as retained. A section that turns stale after staging is held back alone (`recoveryKeyPolicyStale`): it is never written or reported written, its key's other sections are written, and the key is re-owed with its epoch moved and the worker woken, never left owed with no round to decide it | Withdrawal decisions and sections; GR expiry |

An aborted invalidation restores a receipt the destination no longer holds.
At apply the owner's exact removal (or, where the RIB elects owner
withdrawals, the recovery decision that settles it) deletes the entry, so the
abort's re-announcement meets no entry. The coordinator opens a window on the
source Peer when apply issues E1 (`Peer.sourcePolicyWindow`), and records in it
the E2 its abort issues. A deletion by `adjOutExpected` or `adjOutRecovery`, of
a source-owned entry whose session is current, made while the source's live
epoch is the window's E1 and the entry's is older, keeps the receipt in the
table's `released` set with its owner, revision, slot (identifier reservation
and ingress binding) and the stamp E1. A sent-owner announcement that meets no
entry is admitted only against that record: the same revision, the owner's
session still current, the window recording that its abort issued the live
epoch for that stamp, and the request carrying that live epoch
(`sentOwnerCurrent`). A record of transaction T thus matches only T's abort
restore. It re-installs the entry under the request's epoch and revision.

Without ADD-PATH a recovery round can answer the owner's removal with a
survivor from another source, which takes the key while the receipt is
released. That write keeps the record of the receipt it replaced, under the
same conditions as a deletion, and an install by another owner of a key whose
receipt is released leaves the record too: neither holds the receipt's slot.
The abort restore then meets the survivor's entry. When the record matches it
(same revision, the same `sentOwnerCurrent` fence) the restore is owed to the
recovery worker rather than written (`oweReleasedRestore`), so the RIB
re-elects between the restored path and the survivor and the round writes the
winner (RFC 4271 Section 9.1). A restore the fence refuses owes nothing.

One reload can invalidate two sources that held the same key in turn: A's
receipt is removed and the round elects B, then B's receipt is removed too. The
`released` set therefore keeps one record per owner of a key, never one per
key, and a record is found by its revision, so B's record never overwrites A's.
Each abort restore matches its own record: the first one to arrive meets no
entry and is written, and the next meets that entry and is owed to the
election, which chooses among the restored paths. A key holds at most one
record for each source whose open invalidation removed its receipt there, and
each record lives only until that invalidation's release, the bound of the
window itself.

Under ADD-PATH a released record holds its slot: another source's announcement
whose identifier would take it gets a reserved identifier instead, as one
meeting a live entry does (`bindSourcePath`), so the restore and the other
path both reach the recipient (RFC 7911 Section 2).

The source's own withdrawal of the path, the owner's exact removal of it, an
install of the key by its own owner (or by any writer under ADD-PATH), and every
reset of the table drop the record. So does every moment no restore may name it any more
(`Reactor.dropReleasedSource`, under each destination table's `mu`): the
invalidation's release (finalize, the rejection receipt or the hold's expiry,
after the window closes), the source session's departure, a targeted down's
revocation, and the source Peer's stop. A deletion for any other reason keeps
none, so a replay in flight after a purge or a source withdrawal still meets no
entry and is refused.
<!-- source: internal/component/bgp/reactor/adj_rib_out_released.go -- keepReleased, releasedReceipt, oweReleasedRestore, takeReleased, dropReleasedOwner, dropReleasedAt, dropReleasedSource -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- bindSourcePath -->
<!-- source: internal/component/bgp/reactor/source_policy_reload.go -- openSourcePolicyWindow, abortSourcePolicyWindow, release, dropReleasedSource -->
<!-- test: internal/component/bgp/reactor/rfc8212_abort_restore_test.go TestRFC8212AbortRestore -->
<!-- test: internal/component/bgp/reactor/rfc8212_abort_restore_test.go TestRFC8212AbortRestoreAfterSurvivor -->
<!-- test: internal/component/bgp/reactor/rfc8212_released_receipt_test.go TestRFC8212ReleasedReceiptAuthority -->
<!-- test: internal/component/bgp/reactor/rfc8212_released_receipt_test.go TestRFC8212ReleasedReceiptDropped -->
<!-- test: internal/component/bgp/reactor/rfc8212_released_receipt_test.go TestRFC8212ReleasedSlotAddPath -->
<!-- test: internal/component/bgp/reactor/rfc8212_released_two_source_test.go TestRFC8212AbortRestoresBothReleasedSources, TestRFC8212ReleasedReceiptDepartureDropped -->

A policy change keeps the source's session, so it never becomes a
session-generation exclusion or a receive-cut end. Departure ownership
(`departedOwner`), the ADD-PATH departure withdrawal guard in `allow`, and
PATHS-LIMIT retention read the session tests alone
(`adjOutOwnerSessionCurrent`): a stale path of a live session is neither
departed nor withdrawn for an old DOWN, and stays rankable rather than pinned,
so the election can demote it.

Sent events carry the actual advertised owner's epoch, including exact-owner
replays, rather than loading the source's epoch when the callback runs. JSON
writes `source-policy-epoch` beside `message` and `peer`; parsed text writes
`source-policy-epoch <uint64>` after the UPDATE message ID. It is dedicated
transport provenance, not a user route `meta/*` key.
<!-- source: internal/component/bgp/reactor/peer.go -- sourcePolicyEpoch, advanceSourcePolicyEpoch -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiver -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- allow, recordSection, checkInitialWrite -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- sourcePolicyCurrent, sentOwnerCurrent -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- sendBatchUpdate, queueBehindForwards -->
<!-- test: internal/component/bgp/reactor/rfc8212_sent_owner_epoch_test.go TestRFC8212SentOwnerEpochFence -->
<!-- source: internal/component/bgp/reactor/relay_recovery.go -- queueRecoveryOutcome, recoverySurvivorPolicyStale, heldSurvivor, admit, writes, settle, adjOutHolds, departedOwner, adjOutOwnerSessionCurrent -->
<!-- source: internal/component/bgp/reactor/session_paths_limit_elect.go -- queuePathsLimitElection -->
<!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateBody -->
<!-- source: internal/component/bgp/format/text_update.go -- appendSourcePolicyEpochJSON, appendSourcePolicyEpochText -->

The common path serializes eligibility lookup and destination enqueue. Overflow
superseding removes an older equal body and appends the new item at the tail.
Recovery therefore stays after an intervening validation withdrawal, including
when the destination is congested. The removed item releases its cache reference
and pool handles once.

`validationChanged` enqueues route keys on the route server's existing source
workers. Each worker flushes its buffered live UPDATEs before asking Adj-RIB-In
for `replay-path`, which answers the current retained path or an explicit
withdrawal for a removed path; the worker relays that answer itself, under the
route server's own send grant. This callback performs no RPC on the emitting
Adj-RIB-In command handler. Recovery therefore uses the stored bytes without
requiring another UPDATE, and an event overtaken by another change still queries
the current route. Every rebuilt update carries the `rebuilt` route-metadata key
(`docs/architecture/meta/README.md`), because no ingress step ran on it: the GR
egress filter recovers a received LLGR_STALE route's stale level from the source
session's OPEN and the stored COMMUNITIES rather than read the missing level as
fresh.

The flush orders a relay only after UPDATEs the worker already holds. The RIB
and the route server take the source's UPDATEs on independent deliveries, so a
FlowSpec ValidationChange can reach the source worker before the UPDATE the
judged rule arrived in, and before the unicast routes the source sent ahead of
it. The worker therefore parks a FlowSpec relay whose path MessageID is above
the highest MessageID it has handled, and releases it right after it handles
that UPDATE: flush, then relay. Per-source delivery is in order, so handling any
later MessageID also releases it. A source that sends nothing more releases its
parked rules at the worker's idle timeout, with a warning, because the UPDATE is
not coming (a route server started after it, or an UPDATE not delivered to it).
A source that goes down drops them, with a warning, and its peer-down withdraws
its routes. One parked entry is kept per rule. A rule is parked even when it
has no target yet, because a target coming up meanwhile takes its cut before
the rule's UPDATE reaches the route server, and the release selects targets
again.

A target's peer-up replay and the live rail split FlowSpec at the target's cut
the way they split unicast: `replayFlowSpecs` carries the rules whose UPDATE is
at or below the cut, and leaves a later generation to the source worker's
validation relay. The replay passes the target's replay fence and a live
forward waits behind it, so a replayed post-cut rule reached the target ahead
of the live unicast UPDATEs its source sent first. The fence itself is raised
for every process whose registration declares `FencesLiveForwards`, resolved
through the process's `use` spelling, so a route server attached as
`internal rs { use bgp-rs }` holds live forwards until its replay reports.
<!-- source: internal/component/bgp/plugins/rs/server_validation.go -- validationChanged, processValidation, parkValidation, markProcessed, releaseParkedValidations, dropParkedValidations, relayFlowSpecValidation, replayFlowSpecs -->
<!-- source: internal/component/plugin/registry/registry.go -- FencesLiveForwards -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- forwardUpdateCore, forwardValidationWire -->
<!-- source: internal/component/bgp/reactor/reactor_api_relay.go -- buildRelayUpdate, buildRelayWithdrawal -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- dispatchOverflow, forwardSourceCurrent -->
<!-- source: internal/component/bgp/reactor/received_update.go -- receivedPeer, receivedGeneration -->
<!-- source: internal/component/bgp/reactor/peer.go -- forwardCached, forwardGeneration, setState, Stop -->

Both rails converge on the session writer's existing `adjOut`, now the single
ordinary-send authority as well as the local duplicate cache. Destination
identity uses the registered native splitter and semantic key, including VPN RD
and outgoing ADD-PATH presence/identifier, but excluding labels and Compatibility.
The owner retains stable source-peer identity and received ADD-PATH presence/ID.
Ordinary withdrawals do not compare the advertisement's message or session
generation, so a current session can withdraw a GR-retained path. Queued work
still checks its captured source generation and destination Session under
`writeMu`.

Producers preserve path provenance before stripping or regenerating identifiers.
An ordinary unframed path reuses `fwdItem`'s source fields; transformations that
lose identity use bounded pooled source-section owner slices and output-body
spans. A synthesized withdrawal keeps its per-item mark (`fwdProvenance`
`synthesizedWithdrawal`, read by recovery staging) through splitting,
transcoding and overflow. Item release returns the manifest. Superseding cannot
collapse different source/session generations, policy epochs or manifested ingress paths.
Unknown ordinary withdrawals look up outgoing identifiers without minting new
mappings. A retained replay can assign a path's first outgoing identifier after
a live withdrawal was materialized but before the replay fence releases it.
Final admission resolves that withdrawal through a derived ingress-identity
index of the recipient's actual advertisements, not through the cache allocator.
It patches only the session-private wire buffer and still checks the source and
received path that own the resolved slot. Reannouncements use that same index,
so cache eviction cannot give an already-advertised ingress path a second
outgoing identifier. A new source-owned announcement whose proposed ID collides
with another recipient owner gets a distinct reserved ID; direct local/API
writes keep their explicit identifiers and ownership semantics.

The index changes only with recorded slot installation, ownership replacement,
deletion and session clear. Resolving an unknown withdrawal in this index uses
bounded map lookups; it neither scans the route table nor creates an allocator
mapping. Every new-path proposal holds a temporary ID
reservation through both filtering passes, even if it does not collide with an
existing recipient slot. This prevents a later path in the same UPDATE from
allocating an earlier proposal's ID after source-cache eviction and allocator
wrap. The bindings are bounded scratch for one UPDATE; recording or refusing
the write releases their temporary reservations. This applies to peer-up
replay and export-final restoration; a retained path need not have been exported
before either replay. Ownership filtering precedes PATHS-LIMIT admission; only
the final normalized surviving body changes the recipient inventory.
The table is an ordered buffered frontier under `writeMu`, not a separately
published successfully-sent cache. Flush success commits it; any write/flush
failure clears it and seals that exact Session before another send can enter.
Sent callbacks preserve their existing timing after buffered acceptance, not
after successful flush or TCP delivery. They remain asynchronous projection
events, not committed ownership proof. Receipt-fenced cold replay and recovery
must not treat an old callback alone as authority on a replacement session.
<!-- source: internal/component/bgp/reactor/adj_rib_out.go -- adjRIBOut, adjOutPath, setOwner, releasePath, clearRoutes -->
<!-- source: internal/component/bgp/reactor/session_write.go -- writeRawUpdateBody, flushWrites -->

Raw UPDATE injection shares this final export admission: an absent EBGP export
policy cannot create an advertisement, even with a raw send grant. Rejection
projects only explicit withdrawals and EOR, never synthesizes withdrawals from
denied raw announcements. The existing raw ownership preflight refuses an
opaque projection before emission; it does not add a second parsing pass.
Complete non-UPDATE control packets remain independent of export policy, but
uncertain framing (including a trailing UPDATE) requires an accepting final.
<!-- source: internal/component/bgp/reactor/session_write.go -- writeUpdateBody, SendRawMessage, rawControlPacket -->
<!-- source: internal/component/bgp/reactor/forward_provenance.go -- prepareFwdProvenance, writePath -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- beginAdjOut, bindSourcePath, filter, record, release -->
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- reserve, retain, release -->
<!-- test: internal/component/bgp/reactor/policy_reconcile_test.go TestPolicyFinalReplayCannotReviveWithdrawnAddPath, TestPolicyFinalReplayWithdrawalSurvivesGapEviction -->
<!-- test: internal/component/bgp/reactor/policy_replay_identity_test.go TestRecipientAddPathIdentitySurvivesCacheEviction, TestRecipientAddPathCollisionPreservesLocalOwner, TestRecipientAddPathCombinedCollisionReservesStagedIDs, TestRecipientAddPathLocalReplacementRemovesBinding, TestRecipientAddPathReservationsEndWithSession, TestRecipientAddPathCollisionReservationReleasedOnRefusal -->

Withdrawal and recovery fixtures MUST first advertise the affected paths through
the real Session writer with source identity and negotiated framing. A synthetic
RIB lookup response alone does not establish a recipient owner. The regression
fixtures retain that setup on the same Session, then check only the subsequent
wire output; they do not populate `adjOut` directly or relax its admission guards.
<!-- source: internal/component/bgp/reactor/session_ownership_writer_test.go -- ownershipWriterForward -->
<!-- source: internal/component/bgp/reactor/relay_recovery_test.go -- recoveryAdvertiseFailed -->

The unknown-ADD-PATH withdrawal regression also sends one 200-identifier UPDATE
through the real cached-forwarding writer. It retains that UPDATE until after
the writer fence and zero-mapping assertion, so allocation followed by cache
eviction cannot satisfy the bound. The same wire history rejects every unknown
withdrawal. This is an implementation bound, not a separate RFC uniqueness claim.
<!-- test: internal/component/bgp/reactor/forward_provenance_ownership_test.go TestRSWithdrawalOwnershipUnknownPathIDsDoNotAllocate -->

Delayed-worker fixtures bind their gate to the observed destination's worker
key. Native RS forwarding fans out to other workers too; pausing whichever one
runs first does not establish that the recipient's operation is still queued.
The source-generation regression checks the held operation's source, received
generation and message receipt before restarting its source, then keeps the
exact recipient withdrawal history as its behavioral assertion.
<!-- test: internal/component/bgp/reactor/forward_withdrawal_ownership_test.go TestRSWithdrawalOwnershipSourceGeneration -->

The unknown-identifier regression isolates its allocator as well as its source
Peer: registered IDs can overlap literal IDs used by other fixtures, so a
dedicated address alone does not isolate state. Both the measured source's
framed and unframed mapping sets must start and finish empty, with no withdrawal
on the recipient wire. Another source supplies the writer-fence announcement.
Cleanup stops forwarding, clears recipient slots and cached history, releases
source mappings, and checks for leaked reservations before restoring the saved
allocator. Repeated package runs therefore preserve the zero-state assertion
without deleting another fixture's evidence.
<!-- test: internal/component/bgp/reactor/forward_provenance_ownership_test.go TestRSWithdrawalOwnershipUnknownPathIDsDoNotAllocate -->

A source-bound operation discarded without output still advances the existing
causal write sequence. Otherwise a recovery that selected source C could survive
C's filtered withdrawal merely because another source currently owned the
destination. Cold recovery must re-snapshot and drain received-event application
before selecting again; this counter is not a sent-message or delivery metric.

Source-DOWN recovery is a cold selecting-RIB operation, not a second steady-state
forwarding model. Its replacement uses the existing retained candidate and
ordinary egress rail. A departure owes each key the departed source owns
through its ended session to the destination's recovery worker. Before querying
the RIB, the worker snapshots the destination's owed keys, then captures each
source's accepted receive-message cut. It waits for publication into plugin queues before
requiring successful application from the RIB owner. Scalar accepted/completed
IDs bridge the fast-forward-before-delivery gap; cold waits broadcast to every
waiter and are tied to the exact delivery worker, without a hot dispatch lock.
Source identity, generation, worker, and accepted receive cut fence the lookup,
including external IPC.

All selected paths and post-policy sections share one final writer admission
and completion. The writer decides each owed key under `writeMu` against its
record, epoch and movers and those causal receipts; a key whose question
changed is left owed or owed again with the lost source excluded, never
dropped, and unrelated output cancels no decision. RS's joined lifecycle owns that work without a command-expiry timer.
Hard ownership or writer failures retire the affected destination session
instead of leaving stale advertisements installed. Sent callbacks stay
asynchronous. See [sent recovery ordering](../plugin/rib-storage-design.md#source-down-replacement-and-sent-ordering).
<!-- source: internal/component/bgp/reactor/relay_recovery.go -- recoverNLRIBatch, oweDeparture, recoveryAdmission.admit -->

FlowSpec uses the same gate with a full native NLRI key rather than a CIDR
prefix. Its selecting RIB publishes per-path generation and authorization after
checking the associated unicast table; a missing provider is ineligible. The
partitioner splits SAFI 133/134 NLRIs, preserves permitted siblings and turns
infeasible paths into MP_UNREACH.

The live forward of a received UPDATE carries no FlowSpec rule, announced or
withdrawn: the partitioner keeps the rules only for a stored-route relay. The
RIB judges each received generation from its own copy of the UPDATE event, so
the live forward may already have asked before it judged, and it publishes a ValidationChange for every new
generation, every removal and every later verdict change. That event is what
exports the rule, once. Forwarding it live as well sent each rule twice, and
when the live forward asked before the RIB had judged the generation its copy
was a withdrawal, so a rule the RIB then accepted flapped on the wire. A
forwarder that relays FlowSpec therefore replays on ValidationChange, as the
route server and the reflector do. Unicast siblings in the same UPDATE still
leave on the live forward.
<!-- source: internal/component/bgp/reactor/forward_validation.go -- flowSpecSection -->

When optional receive validation is disabled,
non-FlowSpec sections pass through unchanged, including explicit unicast
withdrawals in a mixed UPDATE. Mandatory FlowSpec authorization supplies no
unicast eligibility verdict.

The key retains native framing with the shortest length encoding, so a rule
advertised with either RFC 8955 length form has one eligibility and withdrawal
identity. ADD-PATH remains a separate field and a VPN Route Distinguisher remains
part of the key.

Because the authorization lookup is synchronous, FlowSpec configuration requires
an internal `bgp-rib` process. An explicitly external RIB is rejected even when
it receives all UPDATE and state events; event delivery alone cannot provide
that lookup.

For FlowSpec changes, route-server workers fetch the current path attributes
directly from the selecting RIB instead of using the CIDR-only `replay-path`
command. The reflector queues the same recovery work on its validation worker.
Both use `RelayStoredRoute`, including its normal reflection and egress policy
checks. The reconstruction retains FlowSpec's required empty next-hop field.
If the route server's optional receive-store replay is unavailable on peer-up,
it replays the authoritative RIB's feasible FlowSpec paths before sending EOR.
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_validation.go -- flowSpecEligible, flowSpecPath -->
<!-- source: internal/component/bgp/plugins/rr/validation.go -- startValidation, replayValidation -->

## How long a Path Identifier lives

Both forwarding rails use one source-path allocator (`forward_path_id.go`).
The recipient's recorded source-path index is authoritative once an identifier
has reached that session, including after a source mapping was evicted.

The key mirrors the key the SOURCE uses to name a path. A source that negotiated
no ADD-PATH names a path by its prefix and frames no identifier, so ze holds one
identifier for that source's whole session and gives every prefix of it the same
one. A source that negotiated ADD-PATH names a path by (prefix, identifier), so
ze holds one entry per pair.

A locally assigned number may equal the received number: RFC 7911 Section 2
makes assignment a local matter, not a requirement for numerical inequality.
The split-forwarding ownership test sends identical received identifiers for
the same prefixes from two sources and checks that each corresponding pair
leaves under distinct identifiers. This catches relaying received identifiers
without depending on the allocator's counter or earlier tests.
<!-- test: internal/component/bgp/reactor/rfc7911_forward_body_test.go TestForwardSplitSameContextKeepsRawSplit -->

Native families are walked with their registered NLRI splitters on both raw and
cross-context forwarding paths. The framing is preserved while the leading
ADD-PATH identifier is replaced. Identity comes from each family's registered
route-key function, so changes to non-key label fields or FlowSpec length
encoding keep the identifier. Long native keys are retained in full.

Only the second kind of source mapping is freed before the peer is removed.
The recent-update cache releases it when evicting the UPDATE that withdrew the
path (`recent_cache.go` `evictLocked`), not at the end of either forwarding rail:
one UPDATE reaches both the reactor fast path and the plugin's cached fallback.

Source mappings, recorded recipient slots and temporary writer reservations
hold counted references in the allocator's existing used-ID table. Evicting a
source mapping does not free an ID still advertised by a recipient, even when
the safety valve evicts an entry with outstanding forwarding retains. Recipient
deletion and session clear release the advertisement's reference. This also
reserves explicit local/API identifiers against allocator wrap. Every staged
proposal's reservation is released after the write records its surviving slots,
or after refusal; only recorded slots keep an additional reference.

The free has two exceptions. A path retained by the validation provider keeps
its identifier while ineligible, so recovery can replace its earlier
advertisement. An UPDATE that withdraws and announces the same pair also keeps
the identifier because the destination still holds the path. RFC 7606 Section 5.1
forbids a conforming sender to write both fields,
so the announced section is empty for every ordinary withdraw and the exception
costs nothing. When it is not empty, the section is keyed once
(`fwdAnnouncedPaths`) rather than walked once per withdrawn NLRI: both counts
come from one peer's message, and the product of the two is what the peer would
otherwise choose.
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- fwdReleaseSection, fwdAnnouncedPaths -->
<!-- source: internal/component/bgp/reactor/recent_cache.go -- evictLocked, Delete -->

The walk runs BEFORE the entry's buffer goes back to the pool, because it reads
slices into that buffer (`docs/architecture/memory/lifetime-contracts.md`).

With receive validation enabled, eviction also checks whether the path remains
pending or stored in Adj-RIB-In. A retained ineligible path keeps its identifier
for recovery, and an obsolete withdrawal cannot free its replacement's identity.
The presence check and identifier removal share the identifier-table lock.
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- releasePath -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- validationRetainsPath -->

## Request-owned batch writes

`AnnounceNLRIBatch` and `WithdrawNLRIBatch` carry their caller's context through
grouped, partial-deduplication, stale-route and split-message sends. Waiting for
the session writer is cancellable; a cancelled waiter cannot emit its UPDATE
after another writer releases the session.

After acquiring the writer, the request owns the socket deadline through the
write and flush. Cancellation interrupts a blocked flush. If it interrupts a
write, the socket is closed because a partially emitted BGP frame cannot safely
be resumed by a later request. Cancelling only a waiter leaves the active
writer's socket untouched. Cleanup joins the cancellation callback before
releasing write ownership, so an old request cannot expire a later writer.
<!-- source: internal/component/bgp/reactor/session_write.go -- sendUpdateCounted, requestWriteDeadline -->
<!-- source: internal/component/bgp/reactor/session_write_mutex.go -- LockContext -->

## Configured native next-hop self

A native `PluginRoute` retains explicit `NextHopSelf` intent until initial sync.
Both grouped and ungrouped builders resolve it from the connected session's
actual local endpoint, independently of the peer's default forwarding next-hop
mode. An unavailable endpoint prevents that route from being sent; it does not
substitute a configured address or another next hop. Self intent is part of the
grouping key, so a self route cannot borrow an explicit route's attributes.
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- sendPluginRoutesVia, pluginRouteGroupKey -->
<!-- source: internal/component/bgp/reactor/peer_static_routes.go -- toPluginParams -->

## Forwarded routes and next-hop self

RFC 4271 Section 5.1.3: "A BGP speaker MUST be able to support the disabling
advertisement of third party NEXT_HOP attributes in order to handle imperfectly
bridged media." A peer's `next-hop self` is that disabling for the routes Ze
forwards. Both forward rails, the general one and the route-server fast path,
rewrite the next hop to the address the peer reaches Ze on: the local endpoint
of the established session's TCP connection. That is the address the announce
rail sends for a `next-hop self` route, so the rails agree. It is also the only
answer under `connection > local > ip auto`, which configures no local address.
The configured local address is read only when the session holds no endpoint,
and the socket is bound to it, so the two never differ.

When no address of Ze exists for the destination, the announcement is withheld
from it and a warning names the peer: "withholding route: next-hop self is
configured and the session has no local address". The received third-party
next hop is never sent in its place. The destination is sent a withdrawal
instead, as under every withhold gate (below).

On a session that runs over an IPv6 link-local address, the connected endpoint
is link-local, and `next-hop self` writes it alone: the 16-octet Link-Local-only
Next Hop of draft-ietf-idr-linklocal-capability Section 3. The forward rails
send that form only where the announce rail would: the session negotiated the
Link-Local Next Hop capability (code 77), and for IPv4 NLRI RFC 8950 Extended
Next Hop Encoding as well. Both rails ask the one predicate the announce rail
asks. On any other session the announcement is withheld, the destination is
sent a withdrawal instead, and a warning names the peer: "withholding route:
its next hop is link-local-only and this peer did not negotiate the Link-Local
Next Hop capability".

The speaker's own Link-Local address is appended behind a next-hop-self Global
address (RFC 2545 Section 3, draft-ietf-idr-linklocal-capability Section 4)
against the Global address the rewrite chose, so under `local ip auto` the
connected endpoint qualifies exactly as a configured local address does.

The second address of the 32-octet form is the Link-Local of the next hop
(RFC 2545 Section 3), and the only Link-Local Ze holds is its own. So it is
appended only after a Global that is this speaker's own address: the session's
connected endpoint, the configured local address, or an address on any local
interface. The draft's Section 4 also names the internal peer's own address,
but no rail sends that route: RFC 4271 Section 5.1.3 withholds an originated
route whose NEXT_HOP is the peer's address, and the relayed rail withholds it
too. So the predicate classifies the peer's address as a third party, and the
draft's second condition is a `{not-applicable}` row,
`DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-15`. An
explicit next hop naming another router, even one on the shared link, is sent
as its Global alone, length 16. RFC 2545 would want that router's own
Link-Local there, which Ze never learns: a recorded gap in the whole
`RFC2545-3-3` condition as well as `RFC2545-3-6`.

A relayed route under `next-hop unchanged` or `auto` keeps its effective next
hop only if its wire form is usable. For a 32-octet pair (or a 48-octet VPN-IPv6
pair), the first address must be IPv6 global unicast and the second must be
IPv6 link-local unicast, apart from the canonical VPN exception below.
Link-local, IPv4-mapped, loopback, unspecified and multicast first addresses
cannot supply the ordinary pair's global slot. A global, unspecified,
multicast or IPv4-mapped second address is also invalid. Either invalid slot
causes a native withdrawal. Capability 77 does not legalize such a pair.
The trimming boundary leaves an invalid pair intact for the shared withholding
gate; it cannot silently turn it into a valid single-address form.

For a valid plain pair, one locally connected prefix must contain both the
recipient and the effective global address (RFC 2545 Section 3). Separate
memberships in S's A and B prefixes do not establish a common subnet for all
three entities. If no such prefix exists, the link-local half is dropped and the
global goes alone, length 16. A VPN-IPv6 pair answers RFC 4659 Section
3.2.1.1's condition instead, which names only the peer: the pair is kept when
the speaker shares a connected subnet with the recipient, wherever the global
is, and is otherwise cut to its 24-octet RD+global half
(`linkLocalConditionFor`). Next-hop-self construction uses the same
per-family predicate. A filter-written field is judged after the rewrite,
including `auto` and `unchanged`: a valid speaker-owned global, 16 octets or
24 with the RD, receives the configured speaker Link-Local when its family's
predicate permits it. The new pair is
copied into the existing operation accumulator; trimming an existing pair
aliases its global half. Both forward rails preserve cached input and do not
rerun policy. The source peer's prefix cannot substitute for the effective
next-hop address.

The immutable session snapshot retains the recipient address and connected
prefixes. The per-UPDATE check scans that bounded slice without allocating or
reading the kernel. RFC 2545 Section 2 defines directly connected routes by a
common subnet prefix; this check adds no ND, probing or liveness requirement.
Real-topology fixtures independently establish who owns the advertised pair.
A 16-octet global naming another router on that prefix receives that
router's own Link-Local from the neighbor table the snapshot read
(`linkScope.thirdPartyLinkLocal`), never the speaker's; with no usable entry
the field stays 16 octets and a rate-limited warning names the next hop.
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- applyEgressNextHopScope -->
<!-- source: internal/component/bgp/reactor/link_scope.go -- linkLocalConditionFor, linkScope.holdsCommonSubnet -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedPairSubnetConditions -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedPairSecondAddressValidated -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedPairFirstAddressValidated -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545EffectivePairPolicyAndMixedSibling -->
<!-- test: internal/component/bgp/reactor/rfc2545_joint_subnet_test.go TestRFC2545JointSubnetWriter -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_owned_policy_test.go TestRFC2545ForwardPolicyOwnGlobalJointSubnet -->
<!-- test: internal/component/bgp/reactor/rfc2545_third_party_link_local_test.go TestRFC2545ThirdPartyLinkLocalForward -->

These checks judge the effective pair after policy. A valid replacement of
obsolete malformed input is announced; a malformed replacement of valid input
is withdrawn. When legacy and MP announcements share the received UPDATE, a
refused MP pair does not withdraw an independently valid legacy sibling.

The global-unicast requirement also applies to a single IPv6 global next hop.
The effective next-hop encoding, not the NLRI AFI, decides its address family:
4/12-octet fields carry IPv4, while 16/24/32/48-octet fields carry IPv6.
Loopback, unspecified and multicast addresses in an IPv6 encoding cause a
native withdrawal even toward an on-link destination. A single sixteen-octet
IPv4-mapped field remains valid for IPv6 unicast, multicast and labeled NLRI
(RFC 8950 Section 1); a mapped-plus-link-local pair remains invalid. Native IPv4
next hops remain outside that IPv6 check, including an IPv4 next hop for
IPv6 SR Policy NLRI (RFC 9830 Section 2.1). Unmapping an address for identity
comparison does not change its recorded wire family; an effective MP policy
replacement updates both facts before admission.
An MP rewrite is effective only when the source carries MP_REACH: the writer
cannot create its NLRI from a next-hop operation alone. Configured IPv4 modes
record both legacy and MP operations, but the unused MP operation does not
impose IPv6 admission rules on a legacy-only route.

For a configured IPv4 self or explicit rewrite, `applyFactsNextHop` selects the
native four-octet MP form only when the existing next-hop length contract for
that AFI/SAFI permits it. Admission and the delta writer consume that same
operation, rather than judging a mapped IPv6 address and emitting something
else. This includes IPv6 SR Policy, whose next-hop family is independent of its
NLRI AFI; it does not flatten VPN framing or make a four-octet field valid for
IPv6-unicast NLRI. Received or policy-written mapped fields are not normalized
by this producer choice; their existing family profile decides admission.
The mixed AIGP socket regression uses a genuine IPv6 self address for MP and an
export rewrite to a local IPv4 interface for legacy NLRI. Both remain local
next-hop changes, so initial forwarding and metric replay retain their distinct
received-distance accumulation.
`attribute.ValidNextHopLens` returns shared read-only length tables, so this
per-destination family choice does not allocate or depend on compiler inlining.

RFC 4659 Section 3.2.1.1 explicitly permits one VPN-IPv6 exception: the exact
48-octet zero-RD plus unspecified
Global, followed by zero-RD plus a valid Link-Local, when both established
session endpoints use link-local IPv6 and the destination is on-link. Both
forward rails retain that complete pair rather than trimming or withdrawing it.
The session's captured local endpoint decides this, not a configured local
address that disagrees with the connection. A single unspecified address,
multicast, invalid second address or nonzero next-hop RD does not qualify.
Capability 77's Link-Local-only form is a separate case.
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedUnusableGlobalRefused -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestRFC2545ReceivedSingleGlobalAddressValidated -->
<!-- test: internal/component/bgp/reactor/rfc2545_forward_subnet_test.go TestSRPolicyNextHopWireFamily -->
<!-- test: internal/component/bgp/reactor/rfc4659_link_local_peering_test.go TestRFC4659LinkLocalPeeringPreservesUnspecifiedGlobalPair -->

When the received next hop is Link-Local-only (16 octets, or 24 with the RD),
there is no Global to keep. Towards a destination more than one hop away the
route is withheld, under any next-hop mode that leaves the received next
hop in place, and a warning names the peer: "withholding route: its next hop is
link-local-only and this peer is more than one IP hop away". Section 4: "If,
after completing these procedures, there are no IPv6 next hop addresses included
in the next hop, the BGP route MUST not be advertised to its peer. Instead,
treat-as-withdraw (Section 2 of [RFC7606]) is used." A directly attached
destination that negotiated the Link-Local Next Hop Capability (capability 77)
receives the Link-Local-only next hop unchanged. One that did not is withheld
the route and sent its withdrawal, whether this speaker wrote the next hop or
relayed the received one: RFC 2545 Section 3 puts the Global address in the
field, and the draft's Section 3 calls a Link-Local-only Next Hop "received
without the Link-Local Next Hop Capability having been negotiated" "not
conformant with [RFC2545]". A next-hop rewrite
(`self`, a filter) replaces the address before the question is asked, so it
passes. Rewriting to self under `auto` towards an external peer is not done yet.

## Withheld routes are withdrawn

Every egress gate that refuses a destination an announcement sends it the
withdrawal of every route the UPDATE names instead: RFC 1997 well-known
communities, RFC 7947 control communities, a genuine egress policy reject, and
the next-hop gates (next-hop self with no local address, a next hop that is the
peer's own address, a reflected Link-Local-only next hop off the advertiser's
segment, a Link-Local-only next hop towards a multihop peer, RFC 8950 without
Extended Next Hop, a Link-Local-only next hop without capability 77, or an
unusable unspecified or multicast IPv6 Global, excluding the RFC 4659
link-local-peering exception above). The
destination may hold the previous generation of the route. The final writer
therefore admits each synthesized withdrawal only while its route is still
owned by that received source path in the destination's `adjOut`. It refuses
it when another source or a local advertisement replaced that owner, and when
the destination holds no record of the route: RFC 4271 Section 3.1 withdraws "a
previously advertised route", so a route refused before it was ever advertised
to a destination sends that destination nothing, as FRR and BIRD do. FRR's
`bgp_adj_out_unset_subgroup` (`bgpd/bgp_updgrp_adv.c`, commit `cd39d029`) sends a
withdrawal only for an adj-out entry that carries an advertisement:
`adj = adj_lookup(dest, subgrp, addpath_tx_id); if (adj != NULL) { ... if (adj->attr && withdraw) {`.
BIRD's `rt_notify_basic` (`nest/rt-table.c`) drops an old route the channel never
exported before deciding: on the 2.x line (`v2.19.0-4-g02d082a7`)
`if (old && !bmap_test(&c->export_map, old->id)) old = NULL; if (!new && !old) return;`,
and on the BIRD 3 line (`v3.2`) an old route in `export_rejected_map` is
"Treat old rejected as never seen." `old = NULL;`, then `if (!np && !old)` returns
as an "idempotent withdraw (filtered on export)". Original withdrawn siblings remain
received withdrawals and require a matching owner. A filter step that could not
run is a drop, not a reject, and sends nothing.

The withdrawal is the RFC 9494 LLGR conversion (`buildWithdrawalPayload`). It
carries the source UPDATE's own Withdrawn Routes and MP_UNREACH_NLRI beside the
converted announcement, in one Withdrawn Routes field and one MP_UNREACH_NLRI,
and it is built once per UPDATE and shared by every refused destination
(`fwdWithdrawal`), so the body is built once too. The body builder sends each
NLRI-bearing field of it in its own message (RFC 7606 Section 5.1).

A mixed UPDATE is partitioned one field at a time for a refused destination in
two cases (`withdrawalBySection`). A next-hop gate judged one field's next hop,
so the other fields' routes are still owed: an
UPDATE carrying IPv4 routes with a NEXT_HOP and IPv6 routes with a
Link-Local-only next hop withdraws the IPv6 routes from a multihop peer and
announces it the IPv4 ones. And a source whose MP_UNREACH_NLRI and MP_REACH_NLRI
name different families cannot merge into one withdrawal (RFC 7606 Section
3(g) refuses a repeated MP_UNREACH_NLRI), so each family is withdrawn in its own
message. Each section meets the gates on its own. The two rails ask the
next-hop gates through one function, so they cannot answer differently.

On the general rail, that partition consumes the actual output judged for the
destination, including a raw export filter's replacement and preceding
in-process edits. The export chain runs once for the original decision, even
when it turns a single-field source into a mixed UPDATE. The bounded
continuation asks the next-hop gates of each output section; it never reruns
policy to recreate that output. Announcement-only AS-path resolution follows
those gates, so a malformed path cannot consume a sibling's withdrawal. Source
identity and the received AIGP baseline remain those of the original UPDATE.
AIGP on a policy-produced mixed output is computed per section, before dispatch.
Intermediate pooled bytes are returned after the split owns its section bytes.
If an edit leaves only one field, the unsplit buffer stays with its item until
rebuild or dispatch releases it. All sections use the same pending-item retain
and dispatch path as an unsplit output.

The route-server retry scans current peers once for source reflection facts and
policy skips, then visits selected destinations directly. Both passes skip an
absent EBGP export policy under RFC 8212 Section 3, using the policy and session
type in the same forwarding snapshot. These destinations return to the cached
path, which applies the same final. Selection accepts only current peer identities
with live facts and costs O(current peers + selected destinations), without a
membership map allocation.
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRSSection -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- egressNextHopWithheld -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- egressNextHopLinkLocalOnlyOffLink -->
<!-- source: internal/component/bgp/reactor/forward_build.go -- buildWithdrawalPayload -->

`buildWithdrawalPayload` answers 0 octets when the payload's layout cannot be
read. That zero is a guard: no UPDATE payload is shorter than 4 octets, so
`fwdWithdrawal.of` refuses on it and the rail suppresses the route rather than
forwarding an empty withdrawal.
<!-- source: internal/component/bgp/reactor/forward_build.go -- fwdWithdrawal -->
<!-- source: internal/component/bgp/reactor/forward_build.go -- withdrawalBySection -->
<!-- source: internal/component/bgp/reactor/forward_build.go -- fwdWithdrawal -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- applyEgressNextHopScope -->
<!-- source: internal/component/bgp/reactor/peer_forward_facts.go -- precomputeNextHop -->
<!-- source: internal/component/bgp/reactor/link_scope.go -- applyLinkLocalNextHop -->
<!-- source: internal/component/bgp/reactor/link_scope.go -- nextHopOwners.classify -->
<!-- source: internal/component/bgp/reactor/peer.go -- linkLocalOnlyNextHopRefused -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- egressNextHopLinkLocalOnlyRefused -->
<!-- source: internal/component/bgp/reactor/session_connection.go -- connectedLocalAddress -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateSection -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRS -->

## What bucketing excludes, and why

Bucket merge handles an item with exactly one `rawBodies` entry, no parsed
`updates`, and `peerBufIdx == 0`. Those three conditions exclude every
parsed-update path, and every copy-on-modify path whose copy differs per
destination: such bytes must not be merged.

One copy is not excluded, and must not be. The RFC 7911 Path Identifier rewrite
(`fwdRegenerateRawPathIDs`) writes into a pooled copy whose handle travels on
`fwdBodyResult.transcodeBuf` rather than on `peerBufIdx`, so the item still meets
all three conditions. It is correct to merge it, because ze's identifiers are
chosen per ingress path and not per destination: the copy carries the same bytes
for every destination that reads Path Identifiers.

Merging uses pooled scratch buffers and FNV-64a for attribute grouping. A hash
collision is caught by a `bytes.Equal` check against the actual attribute bytes
before any merge happens.

A recovery admission (a route-server or reflector election written by the
destination's recovery worker) stages one UPDATE per owed key. Bucket merge
never packs those sections: it takes only unowned forwards (writer authority
unspecified, no source message ID, no recovery), because the item it emits for
a run is a fresh item carrying only the run head's peer, metadata and source
fields, and an owned item's write depends on more than that (its authority, its
destination session, its AIGP send state). After `admit` decides which keys are
written, `recoveryBucketMerge` packs them by its own rule: adjacent
announcements of one candidate source's same received message, with
byte-identical attributes and the same authority, session and AIGP send state,
join into one raw body within the message size.

Recovery may pack what bucket merge must not, for three reasons. Each packed
section is a full copy of its first member, and a member joins only when every
field its write consults is equal, so nothing the write reads is lost. Each
key's ownership is settled per staged key by the admission after the flush,
never read from the written section. And the packed body is a fresh copy of the
staged pieces, so a re-encoded section (parsed `updates`, read through its wire
fields) or a section whose bytes sit in a pooled per-destination copy gives up
only its NLRI field: the admission's own items keep their buffers until they are
released. The admission belongs to one destination, so every section it packs
was already built for that destination's encoding context.

A withdrawal, an MP-framed body, an ADD-PATH source's section (it carries
provenance) and an AIGP replay stay alone. Each packed section names the staged
items it carries, so a source that ends between `admit` and the write re-owes
every key of that section. Without the pack, an election over the 900 prefixes
of one received UPDATE reached the client as 900 UPDATEs.
<!-- source: internal/component/bgp/reactor/forward_bucket.go -- recoveryBucketMerge -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler -->
