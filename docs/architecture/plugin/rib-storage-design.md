# Plugin RIB Storage Design

**Status:** Design Reference for API Programs
**Location:** Derived from retired summary 059. That design was abandoned for the
edge speaker and is recorded under Abandoned approaches in
`plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding and RIB".

This document describes the pool-based RIB storage design for API programs (plugins).
The Ze engine does NOT implement this - it belongs in API programs like `ze plugin bgp-rs`.

**See also:**
- `docs/architecture/rib-transition.md` - Architecture overview
- `plan/spec-plugin-rs.md` - Route Server plugin spec
- `docs/architecture/pool-architecture.md` - Pool system design

---

## Overview

This is the **implementation reference** for Pool + Wire design in API programs:
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool struct -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle type -->
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate struct -->

| This Spec Implements | Design Goal |
|---------------------|-------------|
| `Update` interface | Unified access to UPDATE parts |
| `WireUpdate` / `PooledUpdate` | Mode-specific storage |
| Pool with `Intern()` / `Get()` | Memory deduplication (RIB mode) |
| RIB keyed by attribute handle | Efficient route grouping |

### JSON received-route reconciliation

Received JSON UPDATEs apply Adj-RIB-In withdrawals before announcements, then
reconcile the final stored candidates after releasing the peer lock. Reconciliation
retains the action: withdrawals use the registered withdrawal splitter and semantic
key, while announcements use the registered announcement framing. RFC 8277
Compatibility bytes never identify a route or select a label-stack boundary.

The JSON receive path stores labeled announcements through the same label/CIDR
insertion operation as structured events. It also restores the family next hop
that JSON carries separately from `raw.attributes`, using the existing MP_REACH
writer before interning attributes. Thus a surviving route retains its own label
binding and usable next hop when another peer withdraws.

Registered CIDR keys normalize labeled NLRIs before prefix elections. VPN keys
retain the Route Distinguisher and prefix; ADD-PATH identifies only the received
path removed from its source peer, not a separate election. Removing the best
path publishes the surviving candidate's decision, or one withdrawal when no
candidate remains. Repeating the withdrawal publishes no duplicate change.

Labeled best-path records retain an owned snapshot of the last published label
values, separate from reusable attribute-pool handles and subscriber payloads.
Unchanged labels participate in normal same-best suppression; a real relabel
still publishes a replacement. The snapshot exists only for a live labeled best
record and is removed on withdrawal or peer purge. Ordinary CIDR records remain
pointer-free and carry no label snapshot.

<!-- source: internal/component/bgp/plugins/rib/rib_validation.go -- receivedFamilyAttrs, reconcileReceived, reconcileReceivedNLRIs -->
A negotiated family with no registered `nlrisplit` splitter (a family an
external plugin registered at runtime) cannot be stored. Its routes are not
stored, selected or re-sent, and the RIB says so at Warn once per peer and
family, naming the fix, on both receive paths (`reportUnstorableFamily`).

<!-- source: internal/component/bgp/plugins/rib/rib.go -- handleReceivedPool, insertPoolNLRIs -->
<!-- source: internal/component/bgp/plugins/rib/rib_unstorable.go -- reportUnstorableFamily -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- bestPrevStore, checkRouteBestChange, purgeBestPrevForPeer -->

### Received attribute flags

The dedicated AGGREGATOR, COMMUNITIES, EXTENDED_COMMUNITIES and
LARGE_COMMUNITIES pools share values, independently of their received flags.
Each `RouteEntry` retains four Partial facts in a private byte beside its stale
state. Equal values can share the same 48-byte bundle while one received path
is partial and another is complete. `Clone` and retained value copies preserve
these facts; attribute equality includes them, so a flag-only replacement does
not keep the previous path's flag.

`ParseAttributes` establishes the facts from each incoming attribute block.
`ToWireBytes` restores the corresponding received Partial bit only for a present
dedicated optional-transitive attribute. `OtherAttrs` retains its existing full
header handling; well-known and non-transitive attributes keep their required
Partial clearing. Selected and recovery snapshots therefore retain the flag
without consulting the original message cache.

Canonical reconstruction and the MRT snapshot bridge read these four facts
through `AggregatorFlags`, `CommunityFlags`, `ExtCommunityFlags` and
`LargeCommunityFlags`. The accessors share one private flag encoder and expose
no mask layout. They report standard Optional/Transitive bits and the received
Partial fact only: they do not report attribute presence, retain a pool
reference, grant source authority or select Extended Length. Each writer keeps
its existing presence and framing rules, including MRT's preservation of legal
short Extended Length headers in `OtherAttrs`.

RFC 4271 Section 5 forbids clearing a previous AS's Partial bit on a recognized
optional-transitive attribute that is accepted and passed along. Appending a
community to an existing stored attribute preserves that received fact; a fresh
received replacement establishes its own flags. Zero metadata means no retained
received Partial bit. It does not define the flags for attaching a new attribute:
Section 5's attachment rule and the existing policy encoders still govern that
operation. Source authorization and explicit attribute replacement or removal
remain separate from this storage metadata.

<!-- source: internal/component/bgp/plugins/rib/storage/attrparse.go -- ParseAttributes -->
<!-- source: internal/component/bgp/plugins/rib/storage/routeentry.go -- RouteEntry, Clone -->
<!-- source: internal/component/bgp/plugins/rib/storage/routeentry.go -- AggregatorFlags, CommunityFlags, ExtCommunityFlags, LargeCommunityFlags -->
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go -- entriesEqual, RouteEntry.ToWireBytes -->
<!-- test: internal/component/bgp/plugins/rib/storage/partial_storage_test.go TestKnownTransitivePartialSurvivesStorage -->
<!-- test: internal/component/bgp/plugins/rib/storage/partial_storage_test.go TestPartialOnlyReplacementUpdatesStoredAttributes -->
<!-- test: internal/component/bgp/plugins/rib/partial_selected_test.go TestReceivedPartialSurvivesSelectedCapture -->
<!-- test: internal/component/bgp/reactor/rfc4271_partial_session_wire_test.go TestReceivedPartialSurvivesOrdinarySessionWire -->

### Received source-policy provenance

Received routes retain `SourcePolicyEpoch`, the policy epoch captured when the
UPDATE was admitted, and `SourceReceipt`, the issuing source Session's identity.
They are independent of the received message ID, dispatch session generation,
validation eligibility and GR staleness. Equal-attribute refreshes replace both
with the incoming event's originals. Labeled and native NLRI storage, ADD-PATH
paths (including identifier zero), retained clones and external event delivery
preserve them. A byte-only insertion has no source receipt and clears an old
receipt even when the attributes are unchanged.

Replay carries the stored epoch in the typed `rpc.StoredRoute` field
`source-policy-epoch`; selecting-RIB FlowSpec and recovery snapshots carry the
same provenance, including the original source receipt. A replay may use the
source's current dispatch session while retaining its original admission facts.
Zero means unrecorded provenance, never a request to stamp old bytes with the
current epoch or a replacement session's receipt. The immutable selected capture
revises when that receipt changes, even if the selected path's attributes and
monitoring identity remain equal. Capturing a receipt transports identity; it
does not itself acquire engine authorization to send the route.

The ordinary RIB acquires a source lease before admitting an authorized received
candidate. Every current candidate, including an unelected alternate, and every
owned `RouteEntry` clone holds a local reference to that lease. Equal-attribute
refreshes replace the reference along with the original receipt; byte-only
insertions clear it. Releasing the last reference only wakes the RIB's one lease
worker. The worker performs the engine release and joins final writes outside
peer, storage and attribute-pool locks. The lease registry retains current leases
and pending releases only, with no withdrawn-route or source-session history.
The same shared reference captures the original stable Peer owner and the
original received event's nonzero physical connection ID after that successful
acquire. Owned clones read them through `SourcePeerOwner` and
`SourceConnectionID`; neither is copied into every route or recovered from
current address metadata. A later connection's UPDATE gets another reference;
its UP cannot rewrite the connection on retained routes from the old session.
A connection-constrained lifecycle command judges those original references,
so it can clean up old retained routes while preserving new routes of the same
Peer. Missing or mismatched connection provenance completes no such command. A nil
reference yields explicitly unrecorded zero. This scalar is provenance, while
the engine's receipt and grant checks remain the authorization to send.
The containing received store is keyed by the nonzero configured Peer owner.
Its metadata holds the observed address and current Session token. Selection
uses that containing owner to read protocol metadata while the candidate keeps
the entry's original owner and receipt for dissemination. A passive byte-only
entry can therefore remain useful to forwarding without acquiring authority.

The internal GR source commands carry both immutable identifiers explicitly.
Received mutations match `SourcePeerOwner` and `SourceConnectionID` under the
storage lock; sent mutations match the corresponding original source fields
beside each advertisement's own receipt. The existing active `peerGRState`
captures that physical pair once, and its safety callback checks the same state
pointer before capturing output and again before mutation. An old callback or
an old release therefore cannot act on a successor's GR cycle. Explicit
peer-wide operator commands keep an unscoped active state.

The pending source-lease population supplies original receipts; no address
lookup, completed-source ledger or replacement metadata supplies those rights.
Cleanup holds the original selected destinations before removing input,
reconciles the affected native keys, joins captured and resulting output, then
settles only that physical source's receipts. A failed join is returned with
output still owed on the existing leases. Empty LLGR intermediate steps keep
the same active GR cycle until release or safety expiry, so deleting the last
NO_LLGR route does not make the following decoration and stale mark unknown.
<!-- source: internal/component/bgp/plugins/rib/rib_source_commands.go -- registerSourceCommands, sourceCommandDecls -->
<!-- source: internal/component/bgp/plugins/rib/rib_source_mutation.go -- removePhysicalEntriesLocked, removePhysicalSource, receiptsForPhysical -->
<!-- test: internal/component/bgp/plugins/rib/rib_source_commands_test.go TestGRSourceCommandsPreserveOtherPhysicalRoutes -->
<!-- test: internal/component/bgp/plugins/rib/rib_source_commands_test.go TestGRSourceEmptyLLGRContinuation -->

Manual injection uses the existing BGP synthetic-source slot in `ribInPool`.
It participates in forwarding and system-RIB election, alongside received
candidates, but has no source lease. Ordinary output separately elects from
admitted received candidates before applying destination export policy. A
manual forwarding winner cannot suppress a change in that learned selection.
Both selections share one retained candidate snapshot; no historical route
store is added.
GR and LLGR retention explicitly retain the matching leases before permitting
the candidates to survive physical DOWN. Passive storage can keep an untrusted
receipt without a lease; that receipt alone never authorizes dissemination.

The sent inventory retains the epoch beside its received-owner receipt.
Attribute grouping keeps different epochs separate. Initial and refresh replay
commands carry it in the typed update-route request, not in user route metadata.
RS and RR retain the admitted epoch in their existing accepted-route inventories,
including work queued before forwarding; no separate permanent store is added.
Their existing route inventories also retain the original received Peer owner
and source receipt. Route keys include that numeric owner, so identical routes
from same-address peers coexist. A DOWN takes only entries with its captured
owner; a repeated old DOWN cannot take another owner's route or transaction.
Withdrawal grouping preserves the retained owner rather than borrowing the
DOWN's identity. Source-policy pins and purge operations use that same original
owner; address buckets remain observations and settlement serialization only.
These fields transport provenance. The source-policy lifecycle owners below
read them to invalidate and restore a source in place.

<!-- source: internal/component/bgp/plugins/rib/storage/routeentry.go -- RouteEntry, Clone -->
<!-- source: internal/component/bgp/plugins/rib/rib_advertise_capture.go -- advertiseRoute.matches -->
<!-- source: internal/component/bgp/plugins/rib/rib_recovery.go -- recoveryCandidateWithLabels -->
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_validation.go -- flowSpecPath -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- RawRoute, replayStoredRoute -->
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- groupKey, replayRoutesWithCursor, resendRoutesWithCursor -->

### Bounded selected output

The ordinary selected dispatcher separates its scope revision from the received
message ID, source-policy epoch and retained capture revision. It publishes
current desired native keys under one mutex. The independent current-selection
callback reads only that publication, without engine calls or peer/election
locks. Removing a never-admitted key removes it completely: the scope cut
answers absence without a global per-key tombstone.

Each explicitly admitted destination Session has one long-lived worker and at
most one final-writer batch in flight. Its pending key set is limited to current
desired keys, possible acknowledged output and that one batch. A completion
projection holds native identity, revision and source receipt, never another
copy of route attributes. Suppression confirms retirement; once its current key
also disappears, no absence remains owed. A withdrawal following an in-flight
announcement remains owed until the final announcement receipt determines
whether a removal is needed. An uncertain failed call never counts as completed
output removal.

Route bytes are projected into the selected RPC value once per changed winner
and shared by destination batches. Batches are split using the larger actual
encoded request and widest complete response, including envelope and deadline,
against the IPC frame limit. `selectedDispatchSizedBatch` consumes cached item
budgets while admitting the bounded batch, after the route's shared projection
has been sized once. A single oversized route stays owed with its own
failure; other keys and other destinations continue. DOWN cancels and joins
the captured destination worker before releasing its output projection. Plugin
stop joins all workers before releasing shared capture/source state.

Initial readiness follows final writer receipts, including the empty initial
table. Transaction callers use the same scheduler settlement barrier, restricted
to affected destinations. A source barrier includes old acknowledged and
in-flight source identities after desired selection has changed. Abort restores
fresh source authority and renews its scope revisions even when route bytes and
the original source receipt are unchanged. Old queued work therefore cannot
regain admission through a restored lease.

Capture failures remain latched. Destination and source-lease operational
failures are scoped to the current affected owner, reported through the SDK and
cleared after successful reconciliation. One runtime doctor callback combines
these findings; an unrelated healthy source or recipient remains usable.

The dispatcher and its current-selection callback are initialized by `bgp-rib`.
Its declared event-settlement callback joins queued and in-flight selected work,
then pending final source releases, before global quiesce can succeed. Neither
wait holds an election, peer or publication lock.

State admission reads the engine's per-recipient `selected-destination` and
captured `destination-session` facts. Eligible UP, including an attached-UP
delivered while that address is already marked up, starts the exact destination
worker and seeds its current selection. It bypasses legacy SENT-history replay.
An empty selection still reports readiness through that worker. A source-only
observer or explicit-role recipient keeps the legacy state behavior and starts
no ordinary worker. DOWN cancels and joins only the worker named by its captured
connection token, even when delivery no longer marks that process eligible.
If the join deadline expires, the destination remains owed until its worker
finishes. The worker releases its own session and any obsolete scoped selection
once the last writer for that scope has joined.
The worker never discovers authority by trying a selected RPC; the engine's
final writer independently checks permission on every operation.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_dispatch.go -- destinationDown, destinationFinished -->
<!-- source: internal/component/bgp/plugins/rib/rib_origins.go -- pruneDisconnectedOriginsLocked -->

A targeted `state` up with reason `export-policy` asks only the existing ordinary
owner to re-evaluate current learned selections on the same captured Session.
The `export-policy-all` reason also rechecks local selections for that destination.
Both handlers consume these events before connection bookkeeping. The worker
waits for the previous answer and then queues the applicable current keys.
Local origins stay admitted through both events. Learned-only reoffer leaves
their output in place. The events arm no readiness fence and send no refresh
markers. The settlement barrier waits for the queued work and its final receipts.
A late old-Session event cannot create a replacement destination.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_policy.go -- reofferExportPolicy, consumeExportPolicy -->
<!-- source: internal/component/bgp/server/export_policy_event.go -- OnPeerExportPolicy -->

The engine also sends `ordinary-replay-owned` to state observers of that
captured UP. Adj-RIB-In records the peer state and reports readiness, but skips
both received-route and FlowSpec automatic replay while ordinary selection owns
the connection. This prevents retained but unresolved input from bypassing the
current selection on reconnect. With no ordinary owner, existing RS/RR claims
and per-peer unheld-role fallback still decide auxiliary replay. Explicit replay
commands are unchanged.
<!-- source: internal/component/bgp/server/selected_state.go -- selectedStatePeer -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- handleStructuredState, handleState -->

Ordinary Route Refresh carries its receiving Peer owner and exact Session
through both public event rails. The existing destination worker coalesces at
most one pending request per registered family beside one active replay. It
awaits Begin, captures that family's current desired keys and possible output,
then awaits each fresh selected-route receipt. Before End, it also joins replay
of current configured announcements from the requested family and the original
destination's SENT inventory. A replay requires the original writer's local
origin receipt as well as the configured marker. The engine checks the original
write receipt against that session's live output before admitting the replay.
A configured replay carries no source policy epoch, so the writer refuses a
learned receipt even if it receives one. The worker releases source and
recipient locks before calling the engine, so other recipients continue independently.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_refresh_configured.go -- replayConfiguredSent, sendConfiguredRefreshRoute -->
<!-- test: internal/component/bgp/plugins/rib/rib_selected_configured_refresh_test.go TestSelectedRefreshJoinsOnlyCurrentConfiguredOutput -->
<!-- test: test/plugin/refresh-config-static.ci -->

The initial cut bounds keys and source receipt identities. Replacement writes
outside that cut retain one conservative uncertainty bit until End rather than
growing source history. Initial readiness and event/source settlement include
the final End receipt. Status exposes pending and active refresh obligations.
Non-selected recipients retain their legacy SENT replay and marker order under
the engine's existing grants. Before that fallback, the RIB compares a captured
refresh with the latest live owner and Session stored in its existing peer
metadata. A stale observation cannot replay SENT to a replacement. A current
ordinary observation without eligibility also cannot borrow legacy replay.

Source-policy phases, explicit stale purge and safety-net expiry capture affected
worker incarnations before removing input. They publish replacements and join
both that captured set and remaining source obligations before releasing source
leases. This includes an old winner that disappeared before any wire receipt.
Command/timer settlement is bounded by the process lifetime and the existing
cancellable-operation budget. Before mutation, a transient operation hold blocks
the existing last-reference release worker. While that hold remains active,
post-mutation capture adds workers admitted after the first capture to the same
coalesced lease obligations, even when the foreground join has been refused or
canceled. After publication and an attempted join, the release worker checks
those obligations without waiting, so a refused foreground join cannot release
the source. Each live lease coalesces
one pointer per unjoined worker; a retry prunes only workers whose done channel
has closed, never a worker merely replaced by a new Session. These obligations
end with the source lease and add no source or destination history. Unrelated
source releases continue independently. Lease and dispatcher mutexes never nest.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_refresh.go -- requestRefresh, refreshStep, replayLegacyRefresh, currentRefreshSession, rememberRefreshSession -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_barrier.go -- sourceDestinations, sourceReleaseReady -->
<!-- source: internal/component/bgp/plugins/rib/rib_source_lease.go -- holdSourceOutput, settleReceipts -->
<!-- source: internal/component/bgp/plugins/rib/rib_source_output.go -- settleSourceOutput, settlePurgedSource -->
<!-- source: internal/component/bgp/plugins/rib/rib_event_settlement.go -- settleEvents -->
<!-- source: internal/component/bgp/plugins/rib/rib.go -- runRIBPlugin -->
<!-- source: internal/component/bgp/plugins/rib/rib_ordinary_destination.go -- updateSelectedDestination -->

A failed barrier names unresolved destination addresses and Session tokens.
Its operator error fits the existing 4096-byte event-settlement failure field;
the IPC limit is unchanged. When complete causes exceed that field, the reply
keeps whole identities in address/Session order, explicitly counts omitted
identities and cause details, and ends with `ze show bgp rib status` as its fix.
The local error chain still retains the original deadline and every destination
cause. `selected-output` in that existing status document lists every current
unsettled destination, its pending and in-flight counts, readiness and any full
failure cause. It is a fresh, neutral snapshot, not another completion ledger
or a doctor failure during normal convergence. Settled or ended destinations
leave that list; retrying quiesce must still join the actual writers.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_failure.go -- selectedBarrierError -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_status.go -- unsettledStatus -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- status -->

<!-- source: internal/component/bgp/plugins/rib/rib_selected_dispatch.go -- selectedDispatch, currentSelection -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_batch.go -- selectedItemSize, selectedEnvelopeSize, selectedDispatchBatch, selectedDispatchSizedBatch -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_barrier.go -- barrier, barrierSource -->
<!-- source: internal/component/bgp/plugins/rib/rib_selected_diagnostic.go -- doctorFailures -->

### Route-server physical actors

RS keys its peer state, capabilities, UPDATE workers, validation workers and
pause bookkeeping by the original nonzero Peer owner and physical connection.
The address stays display and selector data. Two named peers sharing an address
have independent actors; a delayed old DOWN cannot close a successor actor or
consume its retention verdict. The source-policy barrier joins every already
admitted physical lane of its original owner rather than creating a lane from
the source address.
<!-- source: internal/component/bgp/plugins/rs/server.go -- dispatchText, dispatchStructured, wireFlowControl -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleState, handleStateDown -->
<!-- source: internal/component/bgp/plugins/rs/source_policy.go -- settleSource -->

### Source-policy lifecycle owners

`bgp-rib`, `bgp-adj-rib-in` and `bgp-rs` register an `OnSourcePolicy` handler, so each
declares itself a source-policy owner at Stage 1 and answers the four phases of
an RFC 8212 Section 3 import invalidation (`docs/architecture/api/process-protocol.md`).
`bgp-rib` selects its received store and transaction by the captured
`source.peer-owner`. Its `source.peer` address is observed display data.
`bgp-adj-rib-in` keys source-policy ledgers and denial bounds by the captured
original `source.peer-owner`, independently of the diagnostic address. Prepare
requires that owner's captured established physical connection; a same-address
sibling supplies no authority. Typed and full-JSON DOWN retire only the captured
owner and current physical connection, leaving a sibling and a newer connection
untouched. Address-only DOWN clears only unowned observations; it authorizes
no physical lifecycle or source-policy phase. The auxiliary installed, pending and early-decision stores qualify their outer
source key by the original owner and address. The native route key remains
48 bytes. An unowned byte observation has a distinct observational outer key;
it never matches an owned eligibility query or source-policy transaction.
One current value per owner/native path retains its original physical connection,
ingress message and unchanged source receipt. The receipt remains passive data;
this optional store acquires no lease or dissemination grant merely by storing it. Older physical callbacks, withdrawals and late UPDATEs
cannot replace or retire the successor; a same-address owner holds an independent
store. OPEN metadata is keyed by that same original owner and physical connection
and is retained only while current installed or pending paths refer to it.

Broad replay, explicit replay and operator address selectors retain their
existing aggregate behavior, traversing all matching outer stores. Exact
validation replay instead carries the changed path's original owner and physical
connection, so a missing original returns a withdrawal even if a sibling holds
the same prefix. The output caller still owes its existing source and destination
authorization; these storage keys acquire no engine authority. No observational
limit, eviction policy, current-address source fallback or completed-lifetime
history is added. Each phase
changes state under the store lock, releases it, and returns only after its
downstream work is done.

| Phase | `bgp-rib` | `bgp-adj-rib-in` |
|-------|-----------|------------------|
| Prepare | Refuses a source whose session is down, a token older than a path it already holds (a stored epoch above `policy-epoch`), and a source another open transaction holds. Pins the identity of every stored path: family, NLRI, received message ID and attribute fingerprint | Same refusals. Pins the `RawRoute` pointers of the source's installed and validation-pending routes |
| Apply | Records the denial bound on the source's `PeerRIB`, keeping the bound it held before: every path stored at or below the captured epoch leaves candidacy (`candidateAdmitted`) and FlowSpec authorization, in place. Withdraws the forwarded copies those paths own at every destination and holds their sent entries, then re-elects each pinned route and emits the best changes, one batch per family, before the receipt | Records the bound, keeping the previous one; `routeEligible`, replay and `replay-path` exclude the paths, VPN and the other families keyed by NLRI included |
| Abort | With target zero (prepared, never applied) only releases the pins: no restamp, no re-election, no write; a zero target after apply is refused. Otherwise restamps each pinned path still stored with the same identity to `next-policy-epoch`. After an apply it also drops every other denied path, puts back the bound from before the apply, returns the held copies of restored paths to the sent inventory under the new epoch and announces them again. A held copy whose key another owner now holds at that destination (the survivor a recovery round wrote) is announced again too and stays out of the inventory: the destination owes it to its recovery election, which chooses between the restored path and the survivor and writes the winner. It then re-elects and emits. No Route Refresh and no import evaluation | Restamps the pinned pointers still stored; after an apply, drops the rest and puts back the previous bound |
| Finalize | Admitted only after apply: abort closes a transaction that was not applied. Removes the denied paths, so no denied state is retained for a later recovery, and releases the held sent entries. The bound stays | Same, without sent entries |

The ledger both owners keep (stage, captured identity, recorded outcome of
each phase, and the admission of a phase) is one package,
`internal/core/bgp/sourcepolicy`; what a phase does to a store stays in each
owner. For a live original reflector actor, a target-zero abort of a transaction
that never prepared is a successful no-op, including when another transaction
holds the ledger. It changes no pins, epochs or output. An abort carrying a
target still requires its prepared transaction. The reflector asks the shared
ledger before inspecting the held transaction's pins.
<!-- source: internal/component/bgp/plugins/rr/source_policy.go -- abortSourcePolicy -->
<!-- test: internal/component/bgp/plugins/rr/source_policy_abort_unprepared_test.go TestRRUnpreparedAbortPreservesLiveSource -->

`bgp-adj-rib-in` holds no FlowSpec rule, so for FlowSpec it defers to
the FlowSpec provider `ribevents.RouteEligible` asks first, `bgp-rib`, which
applies the denial to FlowSpec authorization. For every other family a query
for a route the store does not hold is answered ineligible.

Best-change publication is the bus emit. An engine subscriber has run when
the emit returns, but a plugin-process subscriber such as
`bgp-redistribute-egress`, which feeds destination writes, has the batch
queued and not yet applied. The event bus offers no emit-and-wait, so the
owner's receipt does not prove those consumers settled; the coordinator owns
that barrier after the owner call, beside the destination writers it already
joins. It drains the plugin-process subscribers of `bgp-rib/validation-change`
as well: `bgp-adj-rib-in` publishes one on apply and abort when its validation
gate is on, and `bgp-rib` answers it with best changes a forwarder relays. That
drain also runs after each owner's call, before the next owner is called.

The coordinator calls the captured owners in their registered dependency order
(`Dependencies`, `OptionalDependencies` and `StartAfter`, as startup orders
them), breaking
ties by name, for every phase. `bgp-rs` depends on `bgp-rib` and
`bgp-adj-rib-in`, so it is called after both, and a relay their phase caused is
already queued on its source worker when its own settle runs.

`bgp-rib` is the only downstream writer of a forwarded copy. Its sent inventory
holds every copy a forwarder (`bgp-rs`, `bgp-rr`) sent, so its apply withdraws
them with sent-owner authority and the destination's recovery re-elects at a
destination without ADD-PATH. The forwarders only settle their own pending
work. Prepare therefore refuses, before anything is published, a source whose
UPDATEs reach a forwarder (an owner declaring `FencesLiveForwards`) while the
process answering the RIB recovery command is not a captured owner: the message
names the peer and the forwarder, and tells the operator to attach the RIB to
the peer with `receive [ update-received ]` or to take the peer down around the
change.

Pins are temporary and belong to one transaction: no second route store and no
copied UPDATE. A path the source withdrew, GR purged or a later UPDATE replaced
no longer matches its pin and is never restored. A duplicate of a completed
phase returns its recorded outcome without writing again. A foreign or stale
token, a phase out of order and an abort after finalize are refused before any
change. When the source's session goes down, its transaction and held
references go with its routes, so a later phase is refused rather than applied
to a successor session. A `PeerRIB` that GR retains keeps its bound, so its
denied paths stay ineligible until GR purges them.

Losing the session inside the window fails closed. The transaction drops with
the session and the bound stays; neither owner restores anything locally. The
coordinator's later abort is refused, and the coordinator reports the
compensation as incomplete, naming the peer and the owner that refused.

`bgp-rs` is an owner with a handler too, but it holds no route and runs no
election: it forwards each UPDATE and keeps, per source, its queued forwarding
work and the withdrawal inventory a departure is built from, each key stamped
with the policy epoch it was admitted under. Its phases act on those two, and
the withdrawal or replacement of every copy it forwarded is `bgp-rib`'s apply
above, because `bgp-rs` depends on `bgp-rib`, whose sent inventory holds those
copies, and the destination's recovery worker elects the replacement from it.

| Phase | `bgp-rs` |
|-------|----------|
| Prepare | Refuses a source whose session is down, a token older than a key it already forwarded, and a source another open transaction holds. Waits for a departure still queued for the source to be sent and for every item on the source's worker, its forward batch included, to reach the engine. Pins the source's withdrawal-inventory keys |
| Apply | Settles the source's worker again, so every forward queued for the source, and each relay of the receive store's (`bgp-adj-rib-in`) eligibility change, has reached the engine before the receipt. The relay is covered because the coordinator calls `bgp-rs` after `bgp-adj-rib-in` and `bgp-rib` and drains the validation-change subscribers in between, not by anything `bgp-rs` does. The keys stay for abort |
| Abort | With target zero only releases the pins. Otherwise restamps each pinned key still held at or below the captured epoch to `next-policy-epoch`, then settles the worker, so the receive store's re-announcement relays, queued before this phase by the same order and drain, have reached the engine before the receipt |
| Finalize | Removes every key held at or below the captured epoch, so a later departure owes none of the denied routes |

A settle is bounded by the request's deadline, and a settle that does not
finish is the phase's error. The session's down drops its transaction with its
inventory. `bgp-rs` answers it on either transport: DirectBridge in process,
the RPC when it runs forked.

`bgp-rr` is an owner that settles its own pending work for the source in each
phase and drops the denied entries it keeps for the source on finalize. Like
`bgp-rs`, it neither withdraws nor replaces a copy it forwarded: that is
`bgp-rib`'s apply.

<!-- source: internal/component/bgp/plugins/rib/source_policy.go -- handleSourcePolicy, applySourcePolicy, abortSourcePolicy -->
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- DenySourcePolicyThrough, RestoreSourcePolicyDenial, SourcePolicyEligible -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/source_policy.go -- handleSourcePolicy -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- routeEligible, validationRouteKey -->
<!-- source: internal/core/bgp/sourcepolicy/ledger.go -- Owner.Admit, Owner.Reprepare, Ledger.Record -->
<!-- source: internal/component/bgp/plugins/rib/source_policy.go -- restoreSentLocked, reelectSourcePolicy, dropSourcePolicyLocked -->
<!-- source: internal/component/bgp/plugins/rs/server.go -- routeServerRegistration -->
<!-- source: internal/component/bgp/reactor/source_policy_reload.go -- requireSentOwner, drainValidation, settleDownstream -->
<!-- source: internal/component/plugin/server/source_policy.go -- orderSourcePolicyOwners -->
<!-- source: internal/component/bgp/plugins/rs/source_policy.go -- handleSourcePolicy, settleSource -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- routeReflectorRegistration -->

### Adj-RIB-Out native inventory

The RIB plugin keeps one sent inventory for every registered NLRI splitter.
`ribOutKey` uses an allocation-free `netip.Prefix` for plain CIDR families;
other families use the registered semantic key and retain the exact advertised
NLRI separately in `ribOutEntry.NativeNLRI`. ADD-PATH presence is retained even
when its identifier is zero. Withdrawals use the same family identity operation,
including withdrawal-specific framing. Each destination entry owns its
`SourcePeer`; there is no second source inventory. Removing one destination
cannot erase another's origin. `SourceMessageID` records the received revision;
the sent ADD-PATH identifier is not an ingress identifier. The existing sent
receipt also retains explicit local origin or the stable source-Peer incarnation
and received ADD-PATH presence/identifier. Address equality or an absent source
field is not proof of origin. The final writer emits those receipts from its
actual surviving announcements, without retaining forwarded attributes.
Replay feedback does not write ownership or attributes; a new advertisement
replaces them.

The sent callback borrows the writer's typed `wireu.SentOrigin` view. The event
producer copies its scalar source/local fields into the existing `RawMessage`
and retains the event-owned path sidecar; it never keeps the borrowed view.
External serializers project those fields only when needed. Ordinary internal
forwarding creates no ownership metadata map and formats no incarnation string.

This sent inventory is an asynchronous projection, not the ordinary writer's
admission authority. The final writer extends the existing per-peer `adjOut`
with source/path ownership and binds it to one destination Session. It sees
buffered writes in order without a RIB RPC. Source-bound withdrawals must match
that live owner; automatic cleanup carries the captured owner and advertisement
revision before deleting a projection entry. Sent-route refresh preserves
ownership instead of becoming unrestricted local origination. Cold source-DOWN
recovery uses the applied-delivery receipts and the per-key owed records below.
Sent events record successful buffered acceptance, not successful flush or TCP
delivery. The writer's pending frontier and AIGP receipts commit on flush, and a
failure invalidates that Session. Neither this projection nor its old callback
alone grants admission on a new session.
<!-- source: internal/component/bgp/reactor/adj_rib_out.go -- adjRIBOut, adjOutPath -->
<!-- source: internal/component/bgp/reactor/session_write.go -- writeRawUpdateBody -->

Peer-up history uses the new Session's captured initial-sync receipt rather than
bypassing ordinary ownership on an empty table. It can populate only empty slots
in that phase and cannot displace newer live output. Forwarded history requires
the same source incarnation and exact retained received path/message revision.
Each forwarded replay group also carries `initial-source-message`: the received
message ID of the source route it restates (the ribOut entry's
`SourceMessageID`). The update parser refuses a forwarded initial replay without
it, or with a zero or non-decimal value, and refuses it on a local one; the
reactor refuses a forwarded initial replay whose batch names message zero. The
Adj-RIB-Out entry the replay installs records that revision, so a later
recovery election of the same received route recognizes the route the
destination already holds and writes nothing.
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- replayRoutesWithCursor -->
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- initialReplayFromMeta -->
Changed or removed source history is ineligible; replay does not elect or
reconstruct a newer route. The RIB joins the existing initial-update fence so
restoration precedes queued live changes and lifecycle cleanup. Both local and
forwarded history retain their logical old sent revision, while the new wire
message has its own unique ID. This does not depend on the optional Adj-RIB-In
plugin. See [replay cursor](../bgp/replay-cursor.md).
An up with reason `attached`, which a reload sends when it attaches a process to
a session already up, carries no initial token: the session's initial routing
update already went out. Every plugin that acts toward the peer on up records
the peer up and skips that action on it: `bgp-adj-rib-in`, `bgp-rr`, `bgp-rs`
and `bgp-rib` start no replay and report no ready, `bgp-watchdog` resends no
announced route, and `redistribute-orchestrator` fires no redistribute replay.
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleUpWithoutReplay -->
<!-- source: internal/component/bgp/plugins/rib/rib.go -- handleStructuredState -->
<!-- source: internal/component/bgp/plugins/watchdog/server.go -- handlePeerUp -->
<!-- source: internal/plugins/redistribute/replay.go -- onStateUp -->
The initial token remains valid while the peer-up replay fence is held, even
after Ze's own EOR. Completion carries the same captured token to the actual
readiness owner, including when no replay group was emitted; old work cannot
release a replacement Session's queued live changes.

Replay and refresh retain every attribute from the immutable `pool.RibOut`
blob through `Route.RawAttrs` and the existing `update hex` command. Native
NLRIs are replayed from their stored bytes, not converted into CIDR prefixes.
The sent-route view exposes opaque bytes as `raw-nlri` with `add-path` when
present; its prefix column is the hex representation rather than an invalid
IP prefix.

GR/LLGR EOR purge, expiry and NO_LLGR deletion select and remove received paths
under the same `PeerRIB` storage lock used by received inserts. `peerMu` alone
does not exclude those inserts. A callback records only successfully removed
owners while that storage lock is held; it does not dispatch or reacquire the
storage lock. Sent cleanup matches only those owners: semantic identity,
received ADD-PATH presence and identifier, and received
generation. The identifier cannot be discarded: one UPDATE can announce several
paths for the same prefix, and one can refresh before another is purged.
The temporary index dies with the operation; it is not a second lifecycle store.
Labeled received CIDR keys and sent native keys are normalized for comparison.

A fresh received route can precede both RS forwarding and the asynchronous sent
projection. An old sent generation missing from the current received table is
therefore not itself a withdrawal instruction. Cleanup leaves that projection
alone unless the operation actually removed its received owner. Community
attachment changes only exact matching stale paths and emits no withdrawals.
Family pruning leaves kept families alone, while explicit release removes the
source's whole sent inventory. Community changes intern an immutable replacement
before releasing the old reference. Removal encodes withdrawals and captures the
actual sent receipt before releasing entries, then dispatches after unlocking.
The unchanged final writer receipt check refuses obsolete cleanup after a newer
advertisement. Lifecycle feedback cannot delete that newer ownership.

GR supplies the explicit `on-down` argument with its initial retention allowlist.
That handoff leaves nonretained sent entries in place until the forwarding owner
(RS) reconciles them: ownership is needed to fence withdrawal or replacement.
Without `on-down`, the public command reconciles and withdraws removed families
itself, because an established source has no pending forwarding DOWN owner.
Retained families remain in this inventory; later NO_LLGR removal, EOR purge,
expiry and release encode their source-specific withdrawals here.

<!-- source: internal/component/bgp/plugins/rib/ribout_entry.go -- ribOutRouteKey, reconstructRoute -->
<!-- source: internal/component/bgp/plugins/rib/rib_structured.go -- storeSentEntries, removeSentNLRIs -->
<!-- source: internal/component/bgp/plugins/rib/rib_sent_lifecycle.go -- withdrawRemovedSentLocked, pruneSentSourceLocked, attachSentSourceCommunityLocked, dispatchSentLifecycle -->
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- RemoveFamilyMatching -->
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- formatCursorCommands -->

### Source-DOWN replacement and sent ordering

RS drains the departed source's workers and submits its existing native/text
withdrawal batches with the source address alone. RR submits its peer-down
withdrawal batches the same way. Each plugin sends its departures on a fixed
set of long-lived departure workers (`departureWorkers`, 8), taking sources in
the order they went down. The engine answers a departure command only once
every owed key is decided at every destination, so one source's full table, or
one destination slow to take its writes, holds a worker that long; the other
workers keep that from holding every other source's departure behind it. One
source's departures never run concurrently: a source that goes down again
waits for its running departure. The event handler only queues a departure,
merging it into one of the same source still waiting, so it never blocks event
delivery. RR has no
worker to drain: it forwards each UPDATE on the event delivery goroutine that
also delivers the DOWN, so every earlier forward of the source was issued
before the DOWN is handled. Both apply the same retention filter first: a
family `bgp-gr` still retains for the source (GR or LLGR) is left out of the
batches, because the RIB owns those routes and their eventual purge. No received-message cut
travels with them. Each departure names instead the session whose end it
answers: the `session-generation` of the down (`docs/architecture/api/json-format.md`,
"State Events") travels as `recovery-generation` meta beside `recovery-source`,
a decimal string, and `bgptypes.NLRIBatch.RecoveryGeneration` carries it into
the reactor. The engine refuses a departure with `recovery-source` and no
`recovery-generation`, or with an unparsable or zero one, rather than reading
it as any session. A departure still waiting when the same source goes down
again merges with the newer one, and each route keeps the generation of the
down that queued it last, so the merged command groups routes by generation
and sends each group under its own bound. The reactor's writer owes a key only
where the departed source owns it through a session that is no longer current
and that is the named session or an older one (the departure bound), so
neither a reconnected session's newer route nor one a newer session wrote and
then lost under graceful restart is taken for the old one: RFC 4724 Section 4.2
says the speaker MUST retain the routes of a peer that restarts. The bounds
compare generations by source address, across `Peer` objects, so every
generation comes from one process-wide clock (`forwardGenerationClock`,
`Peer.advanceForwardGeneration`): a `Peer` rebuilt for the same address, after
a config remove and re-add or when a dynamic peer is created again, starts
above every session its predecessor ended, and a departure the old `Peer`
queued neither owes nor keeps excluded a route the new one wrote. The
reactor asks the RIB about one destination at a time, and each requested key
carries its own excluded-source set: a departure puts the departed source in
it, and an empty set excludes nobody, so the withdrawing source's other
received paths stay electable. An exclusion is bounded too
(`adjOutExclusion`): a source that announced the key again after the
departure owed it is lifted from the set when that announcing session is
still current, or newer than the bound, so a source that re-announced the key
and then restarted under graceful restart stays electable and its retained
route is not skipped (`adjOutOwed.electableExcluded`). There is no mode field; the two triggers differ
only in data. The RIB skips every candidate of an excluded source, the
destination's own candidates and those of down peers, without deleting any
received state from a possibly reconnected session. Election needs no sent
entry for the key.

This is a forwarded-wire replacement query over received state, not an offer
to install a route in Ze's forwarding Loc-RIB. Its private gather admission
therefore does not require the next hop to resolve through that local table.
Import policy, receive validation, self-next-hop rejection and the destination,
excluded-source and down-peer checks still apply. The ordinary forwarding
gathers retain prospective next-hop feasibility. The published callback and
hidden command parser enter the same recovery handler and select this same
private purpose; no public role or mode field changes that contract.
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- candidateAdmission, gatherPrefixCandidatesForLocked, gatherKeyCandidatesForLocked -->
<!-- test: internal/component/bgp/plugins/rib/rib_recovery_admission_test.go -- TestRecoveryAdmissionKeepsWireCandidates, TestRecoveryAdmissionPreservesCandidateGuards -->

For non-CIDR recovery, candidate lookup derives the receive-storage key from the
validated requested NLRI after removing ADD-PATH framing. It does not reuse the
sent-table identity: FlowSpec receive keys retain a canonical native length field,
while the sent identity omits it. Both short and extended encodings therefore
find the same retained candidates and use the existing election.

The answer holds exactly one outcome per requested key, in request order, and
the reactor refuses an answer that misses a key (`ribevents.CheckRecoveryAnswer`)
rather than reading the gap as "no survivor". The outcome is WITHDRAW when no
candidate remains, otherwise SURVIVORS: every remaining candidate in election
order, meaning `SelectBest` over the remainder, then over the rest, because the
decision comparator is not transitive under MED and cannot sort. Each survivor
carries its own NLRI, attributes, next hop, received message ID, received path
identifier and received ADD-PATH flag. A non-ADD-PATH key's outcome leaves
`Sent` empty: the writer's owed record already names the one advertisement the
decision replaces or withdraws, and the reactor writes the first survivor in
its place, or withdraws it on WITHDRAW.
ADD-PATH destinations run no election: the outcome is WITHDRAW and `Sent` lists
every identifier the failed source owns at the destination for the prefix,
including zero, and including one its reconnected session advertised, because
the RIB holds no session generation. The writer withdraws only an identifier
whose owner is no longer current: the session that wrote it has ended, or a
reload has since unbound processes from that still-running session
(`Peer.forwardRevocation`). The reconnected session's path stays, and so does
an identifier written by a session newer than the record's departure bound
that has since ended too: graceful restart retains it, and only that
session's own departure withdraws it (`adjOutWrite.recoverySpares`).

The request's `Limit` changes that answer. It is the destination's PATHS-LIMIT
bound for the family, and zero means unbounded. A nonzero `Limit` on an
ADD-PATH destination asks for the limited election that best-path admission
under a peer's PATHS-LIMIT needs: the RIB runs the same election over every
eligible candidate of the key, the several paths of one ADD-PATH source
included, and answers at most `Limit` survivors in election order, or WITHDRAW
when no candidate remains. `Sent` stays empty, because the caller's own
admission state names what the destination holds. The election stops after
`Limit` winners rather than ranking the whole set, and
`ribevents.CheckRecoveryAnswer` refuses an answer with more survivors than the
bound. On a destination without ADD-PATH a nonzero `Limit` only bounds the
survivor list.

The external command is
`request bgp rib recovery <destination> <family> <add-path> <sent-add-path> [limit <paths>] <key>...`,
one argument per key in `ribevents.FormatRecoveryKey`'s form,
`<nlri-hex>=<excluded>,<excluded>`. The `limit` pair is written only for a
nonzero `Limit`, and a stated limit is 1 to 65535: `limit 0` is refused, never
read as unbounded. `ribevents.AppendRecoveryArgs` writes the form for the
reactor and `ribevents.ParseRecoveryArgs` reads it for the RIB, so both sides
share one declaration. This remains the cold lookup's retained
sent projection; final writer admission still requires its live owner receipt.
bgp-rr reaches this path with one departure command per family and command
form, carrying `recovery-source` and `recovery-generation` meta and naming each route with its own `del`
keyword, at most `withdrawalBatchSize` (1000) routes a command.
<!-- source: internal/core/bgp/ribevents/recovery.go -- RecoveryRequest, RecoveryOutcome, CheckRecoveryAnswer, FormatRecoveryKey, AppendRecoveryArgs, ParseRecoveryArgs -->
<!-- source: internal/component/bgp/plugins/rib/rib_recovery.go -- recoverySurvivorsLocked, recoverySentAddPathLocked -->
<!-- source: internal/component/bgp/reactor/relay_recovery.go -- queueRecoveryOutcome, departedOwner, adjOutOwnerSessionCurrent -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- handleStateDown, enqueueDeparture, sendPeerDown, recoverRouteSel, nlriDelCmd, withdrawalBatchSize -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleStateDown, enqueueDeparture -->
<!-- source: internal/core/syncutil/coalescer.go -- Coalescer -->
<!-- source: internal/component/bgp/reactor/session_ownership.go -- adjOutWrite.allow, recoverySpares -->
<!-- source: internal/component/bgp/reactor/adj_rib_out.go -- adjOutExclusion, electableExcluded, announcerLifts, spares -->
<!-- source: internal/component/bgp/reactor/peer.go -- forwardGenerationClock, forwardGenerationCut, advanceForwardGeneration -->
<!-- source: internal/component/bgp/plugins/cmd/update/recovery.go -- recoveryFromMeta -->

A lifecycle purge can remove the chosen survivor between the lookup and the
write: B's retained stale path is elected for C, then B's End-of-RIB purges it.
The purge emits a removal only where B already owns the sent entry, and at C it
does not yet, so nothing would ever remove B's purged route from C. The sent
event of a recovery write therefore carries the `recovery-write` meta key
(`bgptypes.RecoveryWriteMeta`), and each of the three sent-store sites
(`handleSentStructured`, `handleSent`'s raw NLRI loop and its `FamilyOps` add)
checks the projected entry of such an event against the source's received
inventory. When the source holds neither the candidate's received path nor a
newer revision of it, the projection deletes the entry in the same `peerMu`
section and queues the removal the purge would have emitted. A recovery
write is checked unconditionally: its candidate came out of bgp-rib's own
lookup, so its path was applied before the write and its absence can only
mean removal. An entry without its candidate's provenance (source, owner,
received message) is stored as before and logged.

A forwarded write meets the same window from the other side. Commands and
events reach bgp-rib on different goroutines, so a lifecycle removal of A's
retained path (purge-stale, release-routes, restart-time expiry, a community
purge, a standalone retain-routes pruning a family; the DOWN handoff prunes
nothing and leaves sent ownership to the route server or reflector that sends
the source-DOWN recovery) can run while the sent event of A's
earlier forward to C is still queued. The removal finds no entry of A's at C
and emits nothing, and the late projection would leave C holding a route A no
longer has. Its absence alone proves nothing for a forwarded write, though:
the reactor's fast path forwards from the session read goroutine before the
received event is queued, so a sent projection can precede its own received
message, and a received withdrawal also removes the path while its own forward
reaches C behind the announcement. bgp-rib therefore keeps, per source, the
highest received message it applied and that value at the last lifecycle
removal of the source's sent copies (`sourceApplied`). A forwarded write is
checked only when its received message is at or below that fence, so only a
path applied before a lifecycle removal and absent after it is removed and its
removal queued; a path a received withdrawal took after the fence is judged
gone too, and the late removal is refused by the writer's revision check or
withdraws what the withdrawal would have. A forwarded write no removal
followed pays one address parse and one map lookup per stored NLRI section
of a sent event and no
inventory walk. The queued removals leave through
one long-lived dispatcher, never through an RPC on the event path, which would
stop bgp-rib applying events while the RPC is in flight. The plugin's run
function starts it before events flow and stops it after `Run` drains
delivery: stop cancels the dispatcher's context, which cancels the send in
flight, joins the worker, and drops what is still pending, because no recovery
owner exists while bgp-rib is stopped. A late removal is safe: the writer
admits it only when the entry still holds the same revision. The reactor sets
the key when it stages a recovery section, in a fresh copy of the item's meta,
because the item's meta is the update's shared map and every write hands it to
its sent event; a forwarded write never carries it.
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- forwardUpdateSection (recovery branch) -->
<!-- source: internal/component/bgp/plugins/rib/rib_sent_lifecycle.go -- projectRecoveryWriteLocked, projectForwardedWriteLocked, lifecycleFollowedLocked, fenceSentLifecycleLocked, recoveryRemovals, startRecoveryRemovals, stopRecoveryRemovals -->
<!-- source: internal/component/bgp/plugins/rib/sent_owner.go -- recoveryCandidateGone, forwardedSourceGone, sourcePathGone -->

Sent event callbacks enqueue delivery; they do not synchronously update the RIB.
The destination's recovery worker first snapshots its owed keys under the
adj-out table mutex, then captures each source's accepted receive-message cut. Before the selecting RIB's barrier, it waits for those
exact cuts to be published into plugin queues. Acceptance precedes reactor-native
fast forwarding; completion follows `OnMessageBatchReceived` (or the synchronous
receive callback). Thus a fast-path write cannot outrun its source event and
leave a falsely current RIB snapshot.

The publication frontier contains only scalar IDs and the exact delivery-worker
lifetime. Cold waiters share a broadcast, support cancellation and fail if that
worker stops before publishing their cut. A dropped or panicked batch poisons
that worker's receipt; later success does not hide it. With no cold waiter,
completion takes no publication mutex; only a blocking cold waiter allocates a
notification channel. There is no per-UPDATE channel, polling loop, unjoined
waiter goroutine, or hot RIB lookup. Completion is publication, not application.

Recovery then drains the RIB command owner's existing FIFO event-delivery queue
with `DrainEventsApplied`. This pure application receipt does not invoke the
deferred-work settlement used by global quiesce: recovery itself helps finish
that work, so joining it here would wait on its own completion.
The application receipt checks owner liveness for both
DirectBridge and plugin IPC, including internal runners whose allocated bridge
was never activated. A stopped or failed activated bridge cannot fall back to
its startup IPC connection to authorize recovery. Earlier delivery failures remain sticky
for that Process incarnation; a later successful delivery cannot clear them.
Only then does recovery query the RIB, in-process or through the same hidden
`request bgp rib recovery` producer.

The cold source receipt also captures configured peer identities and source
generations. Reconstruction and final admission must still match its worker,
accepted receive cut, and generation. Reconnect or a later source UPDATE cannot
relabel old selected bytes with a new session's context.

Every selected path and every post-policy output section of one lookup is
staged in one destination admission, each section tagged with the owed key it
decides. The destination worker decides each key under `writeMu` against the
record, session and epoch its round read, the movers noted since the snapshot,
and the source generations and received-publication cuts. A stale record or
epoch, or a chosen candidate that withdrew the key, leaves the key owed without
writing that decision. A forwarded or initial-replay re-announcement is withheld
for election instead: the drained round may decide, but the announcement keeps
the record owed with an advanced epoch for the next round to elect over the
newer revision. A survivor whose live source accepted more UPDATEs
since the round's cut is still written: an UPDATE of the key itself reaches the
destination's writer as a mover or as a later write, so a busy source never
postpones the decision. A survivor whose source session changed owes only its own key again,
with that source excluded. A survivor whose source has no Established session
to send it from (a source graceful restart holds, or one going away) cannot be
written. Where the destination already holds that source's path, whatever its
revision, the key is held and nothing is written: the newer revision waits for
the source's next session to announce it, and the source's own departure or
the RIB's stale purge withdraws the path if the source does not return. An
ADD-PATH key, owed per prefix, never holds this way. Otherwise the key is
owed again with the source excluded over every session the round's source
snapshot knew: the survivor was judged sessionless against that snapshot, so
the exclusion's bound is the newest generation issued when the snapshot was
taken (`reactorAPIAdapter.recoverySources`), never the clock when the round
decided. The snapshot reads the clock and every `Peer`'s generation in one cut
that each advance takes too (`forwardGenerationCut`), so no session straddles
it: every session a receipt names is at or below the bound, and a session it
does not name that started after the snapshot is above it. A session the source starts after the snapshot, even during the RIB
lookup, and even one that announces the key and then ends under graceful
restart before the next round, lifts it (`recoveryAdmission.unsendableSurvivor`). Siblings do not invalidate one another, and
unrelated output to the destination never cancels a decision. Item release
answers one completion after releasing all child resources, including
cancellation and pool shutdown, and a canceled or unrun admission leaves its
keys owed. A read-buffer shortage while staging defers the unstaged keys to a
later round and backs off when nothing could be staged, retiring the
destination only after 32 such rounds in a row (`docs/architecture/update-building.md`).
A hard deadline, write or flush error retires the destination
session, whose table reset completes every owed key, even when a sibling key
was already owed again. Sent callbacks remain asynchronous.
<!-- source: internal/component/bgp/reactor/receive_publication.go -- receivePublicationCut.wait, state -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- runOnce delivery publication -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- acceptedReceive, synchronous publication -->

The publication regression runs actual `Peer.runOnce` sessions over `net.Pipe`.
It blocks the real batch-receiver callback after native forwarding and waits
for a later same-source marker on the recipient wire, fencing the filtered
withdrawal's writer decision. Recovery then captures a cut including that
marker after the departed source closed its transport, and waits for publication before allowing
the registered dispatcher to deliver the withdrawal. The lookup wrapper checks
the selecting RIB's actual candidate removal before calling its real provider;
recipient history must then contain a withdrawal, not a stale replacement.
A panicking delivery callback instead tears its source down with Cease /
Unspecific (a Ze defect has no defined subcode, RFC 4271 Section 4.5); the destination is not retired, the lookup runs with that source
excluded, and the recipient receives the withdrawal, never the source's
withdrawn route. This carrier exercises the applied-event
drain but does not independently discriminate that drain or external IPC.
<!-- test: internal/component/bgp/reactor/forward_publication_ownership_test.go TestRecoveryWaitsForFilteredReceivedPublication -->

There is no timer retry and no second route inventory. RS and RR run each
DOWN operation on their departure workers under the plugin lifecycle context,
and stop and join those workers at shutdown, without an unrelated
60-second command expiry. Unrelated output never cancels a
required repair, and a newer target owner is protected at owe time: a
departure owes nothing where another source, the source's current session,
or a session of the source newer than the one the departure names owns the
key, unless a reload unbound processes from that session after it
wrote the key. A hard projection, lookup or writer failure retires the
affected destination session through the existing AutomaticStop/Cease path
rather than leaving known-invalid advertisements installed. An admission that
reaches a destination with no Established session, or no connection, writes
nothing and retires nothing: that session is already closing, and its close
drops the owed records. Fail-close ownership
follows the destination session the owed records name: the worker retires only
its own session, and a reconnect never inherits a record, so an unrelated
subsequent reconnect is not retired by the old session's failure.
Replacement UPDATEs pass ordinary egress policy and reflection rules; a
policy-denied replacement withdraws the obsolete advertised identity.

<!-- source: internal/component/bgp/plugins/rib/rib_recovery.go -- recoveryRoutes, recoveryCandidate, recoverySentNLRI -->
<!-- source: internal/component/bgp/reactor/relay_recovery.go -- recoverNLRIBatch, oweDeparture, departedOwner, unsendableSurvivor, heldSurvivor, recoverySources, drainRecoverySources, recoveryAdmission.admit, settle -->
<!-- source: internal/component/plugin/process/delivery.go -- DrainEventsApplied -->
<!-- source: internal/component/bgp/reactor/forward_pool.go -- fwdBatchHandler, closeRecoveryWrites, releaseItem -->
<!-- source: internal/component/bgp/reactor/recovery_owed.go -- recoveryWorker.run -->
<!-- source: internal/component/bgp/plugins/rs/server.go -- recoverRouteSel, RunRouteServer -->

### Supersedes

| Spec | Status |
|------|--------|
| `buildRIBRouteUpdate` conversion | **SKIP** - pool forwarding replaces it |

### Enables

| Spec | When |
|------|------|
| `spec-unified-handle-nlri.md` | After Phase 2 |
| Zero-copy forwarding | After Phase 3 |

---

## Memory Management Design

### Mode-Specific Read Strategies

Different buffer strategies optimize each mode:

```
┌─────────────────────────────────────────────────────────────────────────┐
│  RIB MODE - Reusable buffer + Intern                                    │
│                                                                         │
│  conn.Read(reusableBuf) ──► pool.Intern(buf[:n]) ──► reuse buffer      │
│                                    │                                    │
│                              Intern COPIES into pool                    │
│                              1 copy total                               │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  API MODE - Allocate per read, no copy                                  │
│                                                                         │
│  buf := make([]byte, msgLen) ──► conn.Read(buf) ──► WireUpdate owns buf│
│                                                           │             │
│                                                     0 copies            │
│                                                     GC frees when done  │
└─────────────────────────────────────────────────────────────────────────┘
```

### RIB Mode Implementation

```go
type Connection struct {
    conn net.Conn
    buf  []byte  // Reusable buffer (65535 bytes)
}

func (c *Connection) readUpdateRIB() (*PooledUpdate, error) {
    n, err := c.conn.Read(c.buf)
    if err != nil {
        return nil, err
    }
    // Intern copies into pool, buf can be reused immediately
    return NewPooledUpdate(c.buf[:n], c.ctxID), nil
}
```

### API Mode Implementation

```go
func (c *Connection) readUpdateAPI() (*WireUpdate, error) {
    // Allocate fresh buffer for each message
    buf := make([]byte, msgLen)
    _, err := io.ReadFull(c.conn, buf)
    if err != nil {
        return nil, err
    }
    // WireUpdate takes ownership, no copy needed
    return &WireUpdate{payload: buf, sourceCtxID: c.ctxID}, nil
}
```

### Key Principles

1. **RIB mode:** Reusable connection buffer
   - `pool.Intern()` copies into pool buffer
   - Connection buffer reused immediately
   - 1 copy per message

2. **API mode:** Allocate per read
   - Fresh buffer each message
   - WireUpdate takes ownership directly
   - 0 copies per message (just allocation)
   - GC frees when WireUpdate unreferenced

3. **Pool owns its data** - `Intern()` copies bytes into pool's internal buffer
   - Handle points to pool's copy, NOT original
   - No dangling references

### Summary

| Mode | Allocation | Copies | Buffer Strategy |
|------|------------|--------|-----------------|
| RIB (PooledUpdate) | 0 per msg | 1 (into pool) | Reusable buffer |
| API (WireUpdate) | 1 per msg | 0 | Allocate per read |

Both modes efficient. RIB optimizes for memory dedup. API optimizes for simplicity.
<!-- source: internal/component/bgp/wireu/wire_update.go -- WireUpdate (API mode) -->

---

## Overview

Migrate from direct `[]byte` storage to single `pool.Handle` reference.
Route stores ONE handle to the UPDATE payload; all parts (withdrawn, attrs,
NLRI) are derived on access. CPU derives offsets; storage is minimal.

## RFC 4271 UPDATE Format

```
UPDATE Message (Section 4.3):
┌────────────────────────────────────────────────────────┐
│  Withdrawn Routes Length      (2 octets)               │
├────────────────────────────────────────────────────────┤
│  Withdrawn Routes             (variable)               │
├────────────────────────────────────────────────────────┤
│  Total Path Attribute Length  (2 octets)               │
├────────────────────────────────────────────────────────┤
│  Path Attributes              (variable)               │
├────────────────────────────────────────────────────────┤
│  Network Layer Reachability Information (variable)     │
└────────────────────────────────────────────────────────┘
```

All positions derivable from the two length fields.
<!-- source: internal/component/bgp/wireu/wire_update.go -- ensureParsed, derived accessors -->

## Current State

### Design Doc
- `POOL_ARCHITECTURE.md` describes the pool system design
- Double-buffer with MSB handles, incremental compaction

### Current Code
```go
// Route stores wire bytes directly (to be changed)
type Route struct {
    wireBytes     []byte           // Direct ownership
    nlriWireBytes []byte           // Direct ownership
    sourceCtxID   ContextID
}

// AttributesWire references external buffer - UNCHANGED
// (per spec-attributes-wire.md, zero-copy reference)
type AttributesWire struct {
    packed      []byte             // NOT owned, refs message buffer
    sourceCtxID ContextID
    // ...
}
```

## Target State

### Two Modes

| Mode | When | Storage | Intern |
|------|------|---------|--------|
| **API-controlled** | Adj-RIB disabled | Raw wire bytes | Never |
| **RIB-enabled** | Adj-RIB on any peer | Pool handle | On RIB insert |

Both expose the same `Update` interface. API mode uses wire bytes directly;
RIB mode uses pooled handle. On session restart in API mode, API resends routes.

### Single Global Pool (RIB Mode)

```go
// Global pool shared across all peers (only used when RIB enabled)
// Stores: attribute bytes, NLRI bytes (for large NLRIs)
// Deduplication: same bytes = same handle across all peers
var Pool = pool.NewPool(PoolConfig{
    InitialBufferSize: 1 << 20,  // 1MB
    ExpectedEntries:   100000,
})

// Convenience accessors (all use the same pool)
var Attrs = Pool   // Attribute bytes
var NLRIs = Pool   // NLRI bytes (IPv6+, VPN, etc.)
```

One pool. Shared across peers. Deduplication works globally.

### Attributes Interface

```go
// Attributes provides access to path attributes.
// Implemented by AttributesWire.
type Attributes interface {
    GetRaw(code AttributeCode) ([]byte, bool)
    Get(code AttributeCode) (Attribute, error)
    Has(code AttributeCode) bool
    Packed() []byte
    Codes() []AttributeCode
}
```

### MP Interfaces (RFC 4760)

```go
// MPReach wraps MP_REACH_NLRI attribute (code 14)
// Format: [AFI:2][SAFI:1][NH_Len:1][NextHop:var][Reserved:1][NLRI:var]
type MPReach interface {
    AFI() uint16
    SAFI() uint8
    NextHop() []byte
    NLRI() []byte
    Raw() []byte  // Full attribute bytes
}

// MPUnreach wraps MP_UNREACH_NLRI attribute (code 15)
// Format: [AFI:2][SAFI:1][Withdrawn:var]
type MPUnreach interface {
    AFI() uint16
    SAFI() uint8
    Withdrawn() []byte
    Raw() []byte  // Full attribute bytes
}
```

### Update Interface

```go
// Update provides access to UPDATE message parts.
// Same interface for both wire and pooled backing.
type Update interface {
    Attrs() Attributes      // Path attributes (interface, not concrete type)
    NLRI() []byte           // IPv4 unicast NLRI (message body)
    Withdrawn() []byte      // IPv4 unicast withdrawn (message body)
    MPReach() MPReach       // MP_REACH_NLRI attr (nil if not present)
    MPUnreach() MPUnreach   // MP_UNREACH_NLRI attr (nil if not present)
    SourceCtxID() ContextID
    Release()
}
```

### WireUpdate (API Mode)

```go
// WireUpdate holds raw UPDATE bytes. No pool interaction.
// Used when Adj-RIB disabled, API controls route lifecycle.
type WireUpdate struct {
    payload     []byte      // Owned buffer (allocated per read, no copy)
    sourceCtxID ContextID
}

func (u *WireUpdate) Release() {} // No-op, GC handles it
```

### PooledUpdate (RIB Mode)

```go
// PooledUpdate references decomposed UPDATE components.
// Used when Adj-RIB enabled for deduplication.
// Reconstructed from RIB data, not stored directly.
type PooledUpdate struct {
    attrHandle  pool.Handle   // Interned attrs in global pool
    nlriSet     NLRISet       // Contains family internally
    sourceCtxID ContextID
}

func (u *PooledUpdate) Release() {
    // Note: RIB owns the attrHandle ref, not PooledUpdate
    // This is called when done iterating, not for refcount
}
```

**Storage per attr set in RIB:** 4 bytes (handle) + NLRISet overhead

### Derived Accessors (shared implementation)

Both `WireUpdate` and `PooledUpdate` use these helpers on their payload.

**Note:** Use `uint32` for offset calculations to avoid overflow with extended messages (RFC 8654).

```go
// deriveWithdrawn extracts Withdrawn Routes from UPDATE payload
func deriveWithdrawn(buf []byte) []byte {
    if len(buf) < 2 {
        return nil  // Malformed: too short for withdrawn length
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    if wdLen == 0 {
        return nil
    }
    if uint32(len(buf)) < 2+wdLen {
        return nil  // Malformed: withdrawn length exceeds buffer
    }
    return buf[2 : 2+wdLen]
}

// deriveAttrs extracts Path Attributes as Attributes interface
func deriveAttrs(buf []byte, ctxID ContextID) Attributes {
    if len(buf) < 2 {
        return nil  // Malformed: too short for withdrawn length
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    attrLenOffset := 2 + wdLen
    if uint32(len(buf)) < attrLenOffset+2 {
        return nil  // Malformed: too short for attr length
    }
    attrLen := uint32(binary.BigEndian.Uint16(buf[attrLenOffset:]))
    if attrLen == 0 {
        return nil
    }
    attrStart := attrLenOffset + 2
    if uint32(len(buf)) < attrStart+attrLen {
        return nil  // Malformed: attr length exceeds buffer
    }
    return NewAttributesWire(buf[attrStart:attrStart+attrLen], ctxID)
}

// deriveNLRI extracts NLRI from UPDATE payload
func deriveNLRI(buf []byte) []byte {
    if len(buf) < 2 {
        return nil  // Malformed: too short for withdrawn length
    }
    wdLen := uint32(binary.BigEndian.Uint16(buf[0:2]))
    attrLenOffset := 2 + wdLen
    if uint32(len(buf)) < attrLenOffset+2 {
        return nil  // Malformed: too short for attr length
    }
    attrLen := uint32(binary.BigEndian.Uint16(buf[attrLenOffset:]))
    nlriStart := attrLenOffset + 2 + attrLen
    if nlriStart >= uint32(len(buf)) {
        return nil  // No NLRI present
    }
    return buf[nlriStart:]
}

// MP-BGP types (RFC 4760)

type mpReachWire struct {
    raw []byte
}

func (m *mpReachWire) AFI() uint16      { return binary.BigEndian.Uint16(m.raw[0:2]) }
func (m *mpReachWire) SAFI() uint8      { return m.raw[2] }
func (m *mpReachWire) NextHop() []byte  {
    nhLen := m.raw[3]
    return m.raw[4 : 4+nhLen]
}
func (m *mpReachWire) NLRI() []byte {
    nhLen := m.raw[3]
    // Skip: AFI(2) + SAFI(1) + NH_Len(1) + NextHop(nhLen) + Reserved(1)
    return m.raw[4+nhLen+1:]
}
func (m *mpReachWire) Raw() []byte { return m.raw }

type mpUnreachWire struct {
    raw []byte
}

func (m *mpUnreachWire) AFI() uint16       { return binary.BigEndian.Uint16(m.raw[0:2]) }
func (m *mpUnreachWire) SAFI() uint8       { return m.raw[2] }
func (m *mpUnreachWire) Withdrawn() []byte { return m.raw[3:] }
func (m *mpUnreachWire) Raw() []byte       { return m.raw }

func deriveMPReach(buf []byte, ctxID ContextID) MPReach {
    attrs := deriveAttrs(buf, ctxID)
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(14)  // MP_REACH_NLRI
    if !ok || len(raw) < 5 {
        return nil
    }
    return &mpReachWire{raw: raw}
}

func deriveMPUnreach(buf []byte, ctxID ContextID) MPUnreach {
    attrs := deriveAttrs(buf, ctxID)
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(15)  // MP_UNREACH_NLRI
    if !ok || len(raw) < 3 {
        return nil
    }
    return &mpUnreachWire{raw: raw}
}
```

### WireUpdate Methods

```go
func (u *WireUpdate) Withdrawn() []byte      { return deriveWithdrawn(u.payload) }
func (u *WireUpdate) Attrs() Attributes      { return deriveAttrs(u.payload, u.sourceCtxID) }
func (u *WireUpdate) NLRI() []byte           { return deriveNLRI(u.payload) }
func (u *WireUpdate) MPReach() MPReach       { return deriveMPReach(u.payload, u.sourceCtxID) }
func (u *WireUpdate) MPUnreach() MPUnreach   { return deriveMPUnreach(u.payload, u.sourceCtxID) }
func (u *WireUpdate) SourceCtxID() ContextID { return u.sourceCtxID }
func (u *WireUpdate) Release()               {} // No-op
```

### PooledUpdate Methods

```go
// PooledUpdate is reconstructed from RIB data, not derived from wire bytes.
// It provides access to decomposed components stored in RIB.

func (u *PooledUpdate) Attrs() Attributes {
    attrBytes := pool.Attrs.Get(u.attrHandle)
    return NewAttributesWire(attrBytes, u.sourceCtxID)
}

func (u *PooledUpdate) NLRI() []byte {
    // For IPv4 unicast, return concatenated NLRIs from DirectNLRISet
    if direct, ok := u.nlriSet.(*DirectNLRISet); ok {
        return direct.data
    }
    return nil  // Non-IPv4 uses MPReach, or caller iterates nlriSet
}

func (u *PooledUpdate) Withdrawn() []byte {
    return nil  // RIB doesn't store withdrawals - they're processed immediately
}

func (u *PooledUpdate) MPReach() MPReach {
    // MP_REACH_NLRI is stored in attrs, extract it
    attrs := u.Attrs()
    if attrs == nil {
        return nil
    }
    raw, ok := attrs.GetRaw(14)
    if !ok || len(raw) < 5 {
        return nil
    }
    return &mpReachWire{raw: raw}
}

func (u *PooledUpdate) MPUnreach() MPUnreach {
    return nil  // Unreachable routes not stored in RIB
}

func (u *PooledUpdate) SourceCtxID() ContextID { return u.sourceCtxID }

func (u *PooledUpdate) Release() {
    // No-op: RIB owns the attrHandle reference
}
```

### AttributesWire UNCHANGED

```go
// Wraps path attribute bytes - NO POOL interaction
// Implements Attributes interface
// References either:
//   - GC-managed copy (during processing)
//   - Pooled buffer slice (from PooledUpdate.Attrs())
type AttributesWire struct {
    packed      []byte             // Refs external buffer
    sourceCtxID ContextID
    index       []attrIndex
    parsed      map[AttributeCode]Attribute
    mu          sync.RWMutex
}
```

---

## RIB Storage Model

An UPDATE contains multiple NLRIs sharing the same attributes. RIB stores routes keyed by attributes, with NLRI lists as values.

### Address Families

BGP supports many AFI/SAFI combinations:

| AFI | SAFI | Family | NLRI Size |
|-----|------|--------|-----------|
| 1 | 1 | IPv4 Unicast | 1-5 bytes |
| 1 | 2 | IPv4 Multicast | 1-5 bytes |
| 1 | 128 | VPNv4 | 12+ bytes |
| 2 | 1 | IPv6 Unicast | 2-18 bytes |
| 2 | 2 | IPv6 Multicast | 2-18 bytes |
| 2 | 128 | VPNv6 | 24+ bytes |
| 1 | 133 | FlowSpec IPv4 | Variable |
| 2 | 133 | FlowSpec IPv6 | Variable |
| 25 | 65 | L2VPN/EVPN | Variable |

### Storage Strategy

| NLRI Size | Storage | Reason |
|-----------|---------|--------|
| ≤ 4 bytes | Wire format `[]byte` | Smaller than handle overhead |
| > 4 bytes | `pool.Handle` | Handle (4 bytes) saves space |

```go
// Family-specific threshold
func shouldPoolNLRI(afi uint16, safi uint8) bool {
    switch {
    case afi == 1 && safi == 1:   // IPv4 Unicast
        return false              // 1-5 bytes, inline is fine
    case afi == 1 && safi == 2:   // IPv4 Multicast
        return false              // 1-5 bytes
    default:
        return true               // All others: pool
    }
}
```

### RIB Structure

```go
// FamilyKey identifies an address family
type FamilyKey struct {
    AFI  uint16
    SAFI uint8
}

// RIBManager.ribInPool is a two-level map:
//   map[redistevents.ProtocolID]map[string]*PeerRIB
// The outer key is the source protocol (BGP, BMP, etc.); the inner key
// is the protocol's source label. The BGP slot holds manual injection.
// Authenticated received stores are separate: bgpPeers is keyed by
// nonzero uint64 Peer owner. Forwarding gathers both populations;
// ordinary learned output gathers only admitted received candidates.
// Show commands and metrics iterate both plus every other protocol slot.

// PeerRIB is the Adj-RIB-In for one peer
// Each peer has its own RIB, but all share the global pool
type PeerRIB struct {
    peerID   PeerID
    mu       sync.RWMutex
    families map[FamilyKey]*FamilyRIB
}

// FamilyRIB stores routes for one AFI/SAFI
// NOTE: Current implementation uses per-attribute dedup (not whole-blob).
// FamilyRIB wraps a generic *Store[RouteEntry] which dispatches between a
// BART trie (non-ADD-PATH) and a map keyed by NLRIKey (ADD-PATH).
// FamilyRIB adds pool-handle lifecycle methods (Release, MarkStale,
// PurgeStale, StaleCount) on top of the pure storage primitives.
// See internal/component/bgp/plugins/rib/storage/familyrib_bart.go.
type FamilyRIB struct {
    store *Store[RouteEntry]  // generic storage with BART+map dispatch
}
```

The generic `storage.NewPeerRIB(name)` keeps the supplied source label opaque,
even when it looks like an IP address. `storage.NewAddressPeerRIB(address)`
accepts a validated typed address, caches its canonical spelling once and
returns an error without allocating a store for an invalid address.
`PeerAddress()` exposes that optional immutable display value. Received owner
maps and engine leases supply identity and authorization independently of it.

### Adj-RIB-In route keys

On the bridge ingest path (`handleReceivedStructured`, the in-process plugin),
`bgp-adj-rib-in` stores one entry per received route, keyed by family, path
identifier and the route the NLRI names. The plain unicast and multicast
families key on their prefix. Every other family is split by its registered
`nlrisplit` splitter and keyed by the family's own decoder, never by reading its
octets as an address: labeled unicast keys on the prefix behind its label stack,
and VPN keys on the Route Distinguisher and prefix together, so one prefix under
two Route Distinguishers is two routes. A withdrawal is framed by the family's
withdrawal splitter and ignores the RFC 8277 Compatibility field, so it removes
the one route it names. Each stored route keeps its own NLRI bytes, label stack
and Route Distinguisher included, with any ADD-PATH identifier moved to its
`PathID`.

The key is 48 bytes for every family. A family keyed by its decoder holds the
decoded identity as an interned `unique.Handle[string]` rather than a string, so
the key of the plain prefix families does not grow, and a refreshed or withdrawn
route whose identity is already held costs no allocation.

The full-JSON ingest path (`handleReceived`, taken by a forked plugin) reads
the event's raw NLRI and withdrawal bytes through those same registered splitters.
VPN routes retain their RD, per-route native bytes and ADD-PATH identifier zero;
withdrawals use the family's actual withdrawal identity. It therefore stores
each decoded native path independently on both event transports.

The peer-up replay (`walkReplayRoutes`) covers every stored route of every
source peer other than the target, within the requested from-index and
max-msg-id cut, except FlowSpec routes, whose snapshots come
only from the selecting RIB, and routes a validation decision marked
`Ineligible`. A replay owner (`bgp-rs`, `bgp-rr`) reads it page by page through
the hidden `request bgp adj-rib-in replay-routes` (`buildReplayPage`) and relays
each page under its own send grant; the store relays the whole walk itself
(`buildReplayRoutes`) only for its own self-replay and the operator's
`request bgp adj-rib-in replay`. A page is the lowest store sequences across all
sources, cut at a byte budget. One bound holds across every source: the page
keeps the lowest sequences seen so far that fit the budget together, drops its
highest whenever a route pushes it over, and, once it has dropped one, ends a
source's walk at the first route above its highest sequence. A page therefore
holds and sorts at most one budget of routes, however many sources share the
table. Its walk still visits each source's first route past the cursor, and the
routes of a source that sit below the page boundary before that boundary is
known, so its cost depends on the source order as well as on the page; it does
not grow with the routes that sit above the boundary. A page's `more` is true
when it left a route out, and can be true with nothing left when the routes it
left out are deleted before the next read, which costs the owner one empty page.

<!-- source: internal/component/bgp/plugins/adj_rib_in/compact_key.go -- compactRouteKey, routeKeyFromNLRI -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_complex.go -- installComplexNLRIs, removeComplexNLRIs -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- handleReceived, buildReplayRoutes, walkReplayRoutes -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/replay_page.go -- buildReplayPage -->

### Selecting a session by its name

`bgp-rib` keys received stores, protocol metadata, liveness, configured names,
source-policy transactions and GR state by the configured Peer owner. A state
event records its observed address and current Session. A physical DOWN must
match that Session before it can remove received state, so a late predecessor
DOWN cannot purge a replacement or another configured Peer at the same address.

Operator address selectors deliberately match every owner with that observed
address. Name selectors match the configured name recorded by the state event.
An internal lifecycle operation carries its captured owner directly. Route
counts remain an address-based display and sum owners sharing an address.
`gr-state` uses decimal owner keys and includes `peer-address` in each value.
The SENT destination index also uses the original destination Peer owner.
Recovery requests carry that owner separately from the display address;
excluded sources carry their original numeric owners, and survivors preserve
both their original source owner and issuing Session receipt. Same-address
announcers cannot lift one another's exclusions. Replay and lifecycle commands
can restrict route writes to the captured destination owner through typed batch
metadata; that field narrows the existing send grants and grants nothing itself.
A physical destination DOWN retains its SENT inventory for that configured
owner's reconnect. Permanent Peer removal instead releases that exact
destination's inventory and its undo copies held by any open source-policy
transaction before forgetting metadata. Abort restores only surviving
destinations, so an old transaction cannot recreate removed-owner history or
send to an invalid address. Same-address siblings keep their own inventory.
<!-- source: internal/component/bgp/plugins/rib/rib_sent_destination.go -- retireSentDestinationLocked -->
<!-- source: internal/component/bgp/plugins/rib/rib_peer_name.go -- recordPeerName, selectsPeer -->

### Validation eligibility

`bgp-adj-rib-in` retains the received wire attributes, next hop and NLRI when a
validation decision marks a path `Ineligible`. This is separate from origin
validation's ordinary rejection, which removes the stored route. A retained path
does not enter the timeout queue and cannot become eligible through fail-open.
It becomes usable when a later decision accepts the same received generation.

The engine routes each complete RPKI validation batch to the actual process
registered for `request bgp adj-rib-in batch-validate`, on socket and direct SDK
rails. It preserves all nine fields per decision and fails the whole request
if that registered owner is missing or refuses it; a process-local manager
cannot substitute for a forked receive store.

An empty validation batch still enters the receive store's normal mutation
admission. It refuses a removed, stopped or terminally failed owner, and must
settle older owed validation publication before it can acknowledge zero new
decisions. Empty input does not bypass that lifecycle boundary.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_commands.go -- batchValidateCommand -->


RPKI's origin and ASPA trackers retain the original numeric Peer owner and
physical connection with every route key. Both UPDATE delivery rails, queued
decisions, VRP/ASPA callbacks and configuration snapshots preserve that pair.
`rpc.ValidationDecision` carries it as lossless string-valued `peer-owner` and
`connection-id` JSON fields; the fields are explicit zero for an unowned
observation, never a wildcard for an owned route. The captured configured name
also survives the queue so named peers sharing an address keep their own
policy. Tracker DOWN cleanup matches one original physical connection;
terminal Peer removal names one original owner without inventing a Session.

For an actual GR-retained family, the received store and both RPKI trackers
preserve their existing original rows through physical DOWN. A private retained
bit lives on those rows; it is not a completed-connection ledger. The existing
configuration snapshot carries `gr-retained` with its original owner,
connection, message ID and raw attributes so validator disable, re-enable and
rollback preserve that lifetime. New received bytes replace the mark with their
own actual lifetime. GR supplies no validation verdict.

The `bgp-gr/validation-cleanup` event narrows removal by original owner,
physical connection and native family, with NO_LLGR as an optional additional
predicate. Its codec validates the entire scope before mutation. Current
received events carry the accepted effective attributes after import policy:
`notifyOneMessage` replaces RawMessage's bytes, WireUpdate and AttrsWire before
publishing to every received consumer. Thus an imported NO_LLGR community is
visible to the selecting RIB, Adj store and RPKI trackers alike; this is not a
claim of pre-policy retention.

Cleanup closes Adj replay admission and joins a direct replay's snapshot through
its actual relay without holding the received-store mutation lock across I/O.
Applied emission joins the captured consumers' event application and declared
deferred work before the existing final RIB source purge/output barrier. Escaped
replay pages belong to their role's deferred-work join. A session-wide receipt
still held by a sibling is not evidence that a removed path remains admitted.
<!-- source: internal/component/bgp/plugins/adj_rib_in/retained_validation.go -- retireValidationConnectionLocked, cleanupValidationConnectionLocked -->
<!-- source: internal/component/bgp/plugins/rpki/retained_validation.go -- removeUnretainedValidation, handleRetainedValidationEvent -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyOneMessage -->
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- validationRequest, handlePeerDown, buildDecisions -->
<!-- source: internal/component/bgp/plugins/rpki/aspa_tracker.go -- rpkiSource, routeKey -->
<!-- source: internal/component/bgp/plugins/rpki/rpki_reload.go -- configRouteSnapshot, revalidateConfigSnapshot -->

`rpc.ValidationDecision.MsgID` identifies that generation. A stale decision is
ignored; a decision for a newer UPDATE waits for its route and removes the
previous path from selection immediately. Zero explicitly means an unversioned
decision for the current route, or the next arrival if no route is stored. The
text batch command uses nine fields per decision:
`action peer family prefix pathID state msgID peerOwner connectionID`. Its actions are `a` (accept),
`r` (remove), and `i` (retain ineligible); accepted and retained routes carry the
origin state, 1 (Valid), 2 (NotFound), or 3 (Invalid).

`enable-validation [refresh] [timeout seconds]` applies the configured timeout
before taking a revalidation snapshot. The timeout accepts 1 through 65535 seconds
and defaults to 30. A timeout change applies to the original receive deadline of
a still-pending path; retained rejected paths remain outside fail-open.

The selecting RIB consults `ribevents.RouteEligible` against this receive store.
After changing it, Adj-RIB-In releases its lock and emits `ValidationChange`;
`RIBManager.validationChanged` reruns best-path selection and updates the shared
Loc-RIB, including withdrawal or selection of another peer's path. Recovery
therefore needs no new UPDATE. Each notified `ValidationRoute` includes the
original stable Peer owner as well as its display address, family and path ID.
The receive store retains that owner in `RawRoute`, including pending routes;
eligibility and presence queries refuse an unrecorded or different owner.
The selecting RIB reads the original owner from its retained source reference,
and the forwarding path carries the received UPDATE's captured source owner.
These values do not come from a replacement peer's current metadata.
Each key names its path the way the selection query looks it up: by prefix for the families keyed on
it, and by the route's native NLRI, without ADD-PATH, for VPN and every other
family keyed by its decoder (see "Adj-RIB-In route keys"). The store takes
those bytes from the stored or pending route, so a removal notes its change
before it deletes, and a change for a route the store does not hold is not
emitted, because its eligibility was false before and after. No
`ValidationChange` is emitted before
`enable-validation` enables the eligibility gate. Without it every
path is eligible, so a store change alters no verdict. An event would make the
route server and the reflector replay a route the live forward already sent.
The lookup stays registered after `disable-validation`, and so do the events. Stored replay excludes ineligible paths, and
recovery advances their replay sequence so an existing cursor can discover them.
Ordinary withdrawals and session removal delete retained routes.

One whole-table change may need several events. The receive store checks the
encoded `emit-event` and enveloped subscriber frame sizes before sending each
ordered chunk; a route count alone is not a size bound. Emission runs outside
the store lock and is serialized across chunks. A failed management command
retains only the unacknowledged suffix, coalesced by route key with newer changes.
A validation command, including `disable-validation`
when already disabled, explicitly flushes those obligations before its next
mutation and before reporting success. Source-policy apply, abort and finalize receipts do the same, including
duplicate calls at the recorded epoch. No autonomous retry worker exists.
Consumers read current state; an explicit retry after a timeout can repeat a key
whose acknowledgement was lost, but never retries an acknowledged prefix
automatically.

An ordinary receive or timeout delivery encountering paused publication debt
stops the receive-store owner before changing any route. A new publication
failure also stops the owner. Mutation admission and removal settlement
share one lock; queries remain available during publication. The owner keeps the
failure cause, closes its callback transport, and leaves a static refusing lookup
through teardown. The plugin declares `fatal`, so any unexpected current owner
exit stops the daemon and its BGP peers, not only a publication failure. An exit
leaves the refusing lookup only when the owner held a gate: validation was enabled
at some point, or a publication failed. An owner that never enabled validation
registered no lookup and leaves none. Successful
live removal first settles all owed publications and then refuses further mutations.
On runtime exit, the owner stops and joins its timeout scanner before sampling
the final publication error or releasing the lookup. A scanner publication still
in flight therefore contributes its failure to the owner's result.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_publication_owner.go -- admitMutation, failValidationOwner, removeValidationOwner, releaseValidationOwner -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- runAdjRIBInPlugin -->

Validation keys include the stored original physical connection beside the
stable Peer owner. The receive store captures both from the same original
UPDATE event, and changes and queries retain them through the typed and JSON
paths. The selecting RIB reads the connection from its held source reference;
sent replay reads its immutable original-source connection. FlowSpec path
snapshots and comparable eligibility keys require that same connection.
A replacement connection cannot supply the missing identity of older bytes.
<!-- source: internal/core/bgp/ribevents/validation.go -- ValidationRoute -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- routeEligible, routePresent, noteValidationChange -->
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_validation.go -- flowSpecKey, flowSpecPath, reconcileFlowSpecs -->

A first pending arrival also emits `ValidationChange` after storing the path,
with its original owner and physical connection. Subscribers can then observe
present but ineligible membership without waiting for a validation decision
or another UPDATE. Before a gate has ever enabled, normal installed arrivals
still emit no validation change.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- installStructuredNLRIs, handleReceived -->
<!-- test: internal/component/bgp/plugins/adj_rib_in/rib_pending_admission_test.go -- TestPendingAdmissionPublishesOriginalMembershipBothRails -->

`ribevents.RoutePresent` separately reports pending or installed ownership,
including ineligible paths. Cache eviction uses it to preserve the regenerated
ADD-PATH identifier across a temporary validation withdrawal; an actual route
removal releases that ownership. Eligibility and presence callbacks are registered
together. In-process providers read the receive-store lock without invoking the
cache; remote providers dispatch the registered gate command and can wait. The
forward identifier allocator therefore snapshots a compare-release claim, drops
its lock before RoutePresent, and compares the claim under its lock afterward.
The query names the original connection retained on the current mapping. Any
intervening allocation/reuse invalidates the claim, including same-generation
replay, so a remote answer cannot release a reused or reminted mapping.
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- releasePath -->
<!-- source: internal/core/bgp/ribevents/gate_remote.go -- remoteValidation -->
<!-- source: internal/core/bgp/ribevents/flowspec_remote.go -- remoteFlowSpec -->

The internal `replay-path` command takes destination, source address, family,
prefix, path identifier, original peer owner and original physical connection. It answers the current path, including pending or ineligible
state, as a one-route replay page, and relays nothing: the caller (`bgp-rs`,
`bgp-rr`) relays it under its own send grant to the reactor's validation gate for
reconciliation, because the operator gives the store no `send [ update ]`
(`ribevents.RelayReplayPath`). An absent path uses `StoredRoute.Withdraw`; every
retained path carries its original `MsgID`.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_validation.go -- applyDecision, applyToInstalled, promoteToInstalled -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- routeEligible, unlockValidation, noteValidationChange, validationNLRILocked -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_replay_path.go -- replayPathCommand -->
<!-- source: internal/core/bgp/ribevents/replay.go -- ReplayPathCommand, RelayReplayPath -->
<!-- source: internal/component/bgp/plugins/rib/rib_validation.go -- validationChanged, validationEligible -->
<!-- source: internal/core/bgp/ribevents/validation.go -- RouteEligible, ValidationChange -->

The eligibility lookup is a Go function, registered only in the process the
receive store runs in, and only when that is the daemon. Every
`enable-validation` and `disable-validation` therefore also announces the
gate's state as `bgp-rib/validation-gate`
(`{"enabled": true, "command": "request bgp adj-rib-in eligibility"}`), through
`emit-event` on both transports. The announcement names the command the store
serves for eligibility queries, so no reader names the plugin that owns the
gate. A process that receives it and holds no lookup of its own (the daemon
while `bgp-adj-rib-in` runs forked, or a forked `bgp-rib`) asks the store
through that command: one argument, the JSON `ribevents.EligibilityRequest`
(`{"queries": [{"route": <route>, "msg-id": <id>, "present": <bool>}]}`),
answered by `{"answers": [<bool>, ...]}` in query order, at most
`ribevents.EligibilityBatchMax` (1024) queries per command and
`ribevents.GateLookupTimeout` (5 s) per answer. The reactor's forward path
batches its questions per NLRI section, at most 256 prefixes a command
(`eligibilityBatch`), rather than asking per prefix. `bgp-rib` asks the same
way: one command for each selection's candidate paths
(`gatherPrefixCandidatesLocked` through `candidateGate`), one per family for a
best-path replay (`collectBestPaths`) and for an Adj-RIB-Out refresh or replay
(`replaySourceRefused`), and one for the unicast paths a FlowSpec
reconciliation reads (`dropIneligible`), never one per path. Each process reaches the
command through the dispatch it registers (`ribevents.RegisterGateDispatch`):
the reactor through the daemon's command dispatcher, `bgp-rib` through
`dispatch-command-args`. A query the store cannot answer (no dispatch, an
error, a timeout, a short answer) is refused, fail closed, with one error
logged per minute. An announcement that names no command makes every route
refused. The daemon applies the announcement from the reactor's subscription,
taken before the plugins that depend on BGP start, and a forked `bgp-rib` from
its SDK subscription. A registered lookup is never replaced or removed by an
announcement. A failed announcement fails the command, so `bgp-rpki` reports
that validation is not in force. A forked store stores each route's message ID
from the JSON event, so the msgID match holds there too, and it emits
`ValidationChange` once validation is enabled, as the daemon's store does.

RTR responses retain their Cache Response's protocol version and Session ID
until End of Data. The transport rejects valid payloads and End of Data
received before Cache Response with Corrupt Data. An empty ASPA announcement
retains its specific Provider List Error diagnostic without staging data. A
Serial Query must also retain the identity of its cached base; a Reset Query
can establish a new Session ID. An unknown initial version receives Unsupported
Protocol Version, while a version change within a response receives Unexpected
Protocol Version. A received Error Report never receives another Error Report.
For v2, Corrupt Data, Unexpected Protocol Version and Provider List Error revoke
that cache's current VRPs and ASPAs even if the report write fails. No Data
Available, Unsupported Protocol Version and Cache Restart retain the unexpired
base. V1 retains its base after other
protocol errors, except that a Session ID mismatch flushes it in both versions.
A completed replacement from another cache survives stale-session cleanup.
Store invalidation precedes the bounded report write, and the transport closes
before the joined revalidation callbacks run. Expiry uses the same store-clear
and callback path.
<!-- source: internal/component/bgp/plugins/rpki/rtr_response_identity.go -- RTRSession.checkStreamSequence, RTRSession.checkResponseIdentity, RTRSession.revokeResponseData, rtrResponseErrorCode -->
<!-- source: internal/component/bgp/plugins/rpki/rtr_expire.go -- rtrDataLease.invalidate, rtrDataLease.expire -->

FlowSpec authorization crosses the boundary the same way. `bgp-rib` registers
its FlowSpec lookups (`ribevents.RegisterFlowSpecLookup`,
`RegisterFlowSpecRedirectLookup`) in its own process and announces
`bgp-rib/flowspec-gate` (`{"command": "request bgp rib flowspec-gate"}`) once
its subscriptions are live. A process holding no lookup of its own (the
daemon while `bgp-rib` runs forked, or a forked `bgp-rr`, `bgp-rs` or
`bgp-adj-rib-in`) installs a provider that asks through that command
(`ribevents.NoteFlowSpecGate`): one argument, the JSON
`ribevents.FlowSpecRequest`
(`{"queries": [{"operation": "eligible"|"present"|"path"|"redirect", "route": <route>, "msg-id": <id>}], "routes": <bool>}`),
answered by `{"results": [{"verdict": <bool>, "path": <path>}], "routes": [<route>, ...]}`
under the same batch bound, timeout and dispatch as the receive gate, with at
most `ribevents.FlowSpecRoutesMax` (16384) routes per answer. The forward
path's batch (`RouteEligibleBatch`) asks every FlowSpec rule of a batch in one
command, and a stored FlowSpec relay asks a section's rules in two, at most
256 rules each: one for their eligibility and one for the redirect verdict of
the rules it keeps (`flowSpecRuleBatch`, `ribevents.FlowSpecRedirectRefusedBatch`). An answer that cannot be obtained refuses: not eligible, no path (so
replay withdraws), redirect refused, no routes replayed. Every reader
registers a dispatch: `bgp-rr`, `bgp-rs` and `bgp-adj-rib-in` through
`dispatch-command-args`, the daemon through its command dispatcher.

A subscription delivers future events only, so a reader whose subscription
went live after an owner announced would never learn the gate. A reader
therefore emits `bgp-rib/gate-query` once its subscriptions are live (`bgp-rib`
and `bgp-adj-rib-in` at start, `bgp-rr` and `bgp-rs` once every plugin is
ready), and each owner announces again on it: `bgp-adj-rib-in` its
validation gate, `bgp-rib` its FlowSpec gate. An owner announces only after
its own gate-query subscription is live, so one of the two always reaches
the other.

What `bgp-rib` publishes leaves it the same way in either process. In the
daemon it emits `bgp-rib/best-change`, `bgp-rib/flowspec-change`,
`bgp-rib/validation-change` and the redistribution batches on the daemon's
bus; forked, where `ConfigureEventBus` never runs, `runRIBPlugin` installs an
`sdkEventBus` whose `Emit` hands the JSON to the engine as `emit-event`, and
the engine decodes it into the event's registered type for its own
subscribers. A replay request on either hop (`bgp-rib/replay-request`,
`redistribute/replay-request`) reaches `bgp-rib` through its startup
subscriptions on both transports (`dispatchReplayRequest`), never through a
bus subscription.
The adapter splits oversized best-change, redistribution and validation batches
by the same encoded-frame bound, preserving the original batch metadata and
entry order. One emission's chunks remain together, including during replay,
and one timeout covers the hand-off. Ordinary small payloads keep their single
marshal. Larger validation arrays encode each route once into bounded owning
strings. The SDK adapter encodes its whole typed batch once, then cuts only
between complete entries in the existing canonical array, retaining its
original metadata. Sizing never re-encodes or re-quotes whole payloads.
<!-- source: internal/component/bgp/plugins/rib/rib_event_sdk.go -- sdkEventBus, dispatchReplayRequest -->
<!-- source: internal/core/bgp/ribevents/validation.go -- NoteValidationGate, DecodeValidationGate, ValidationGate, RouteEligibleBatch -->
<!-- source: internal/core/bgp/ribevents/gate_remote.go -- EligibilityRequest, AnswerEligibility, RegisterGateDispatch, askGateOwner -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- eligibilityBatch, flowSpecRuleBatch -->
<!-- source: internal/component/bgp/plugins/rib/rib_validation.go -- candidateGate, replaySourceRefused, dropIneligible -->
<!-- source: internal/core/bgp/ribevents/flowspec.go -- FlowSpecRedirectRefusedBatch -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- announcedGateCommand, emitValidationGateEvent -->
<!-- source: internal/component/bgp/reactor/reactor_validation_gate.go -- subscribeValidationGate, dispatchGateCommand -->

### Mandatory FlowSpec authorization

FlowSpec feasibility is independent of optional RPKI validation. Enabling
SAFI 133 or 134 makes configuration derivation feed received UPDATE and session
events from every peer into `bgp-rib`, including the unicast routes needed to
authorize rules. The feature-only binding is `receive [ update-received state ]`:
it grants neither sent-UPDATE nor refresh delivery, and no send permission.
Explicit peer bindings and BGP-source redistribution retain their separate
grants; the authorization feed does not take ownership of a family's exporter
replay. A missing FlowSpec provider fails closed; disabling an RPKI validator
does not bypass this check.
<!-- source: internal/component/bgp/config/redistribute_binding.go -- wireRedistributeDelivery -->

The received rule remains in its peer's RIB while infeasible. Its destination
component must have a best covering unicast route in the same AFI. VPN FlowSpec
uses VPN unicast with the same Route Distinguisher, not global unicast or another
VPN. Authorization implements RFC 8955 Section 6 as replaced by RFC 9117
Section 4:

- The originators must match, using ORIGINATOR_ID when present and the peer
  transport address otherwise. A peer's BGP Identifier is not the fallback.
- An empty or confederation-only AS_PATH identifies a locally originated
  controller rule and permits the same-domain originator alternative by default.
  RFC 9117 Section 1 includes AS_CONFED_SET as well as AS_CONFED_SEQUENCE.
- For an external rule, the leftmost AS_SEQUENCE ASN must match the best covering
  unicast route's leftmost ASN. It need not match the route-server peer's ASN.
- A more-specific unicast route from another neighboring AS makes the rule
  infeasible.

A received rule that no packet can match (an ICMP type or code with a port, RFC
8955 Section 4.2, `flowspec.CheckMatchable`) is never feasible either. It stays
in its peer's RIB, as an unauthorized rule does, so it is neither installed nor
forwarded nor replayed, which is the "SHOULD NOT be propagated" of that section.
<!-- test: internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go TestFlowSpecUnmatchableReceivedRuleIsNotPropagated -->

A feasible rule received from an EBGP peer whose redirect-to-IP community
(0x0800) names next hop X also gets the draft-simpson-idr-flowspec-redirect-02
Section 3 check. A session whose local AS is unknown counts as EBGP here, as it
does for authorization, so the check fails closed. Ze takes the longest unicast match for X in the rule's own
domain and X's address family. When the last AS of its AS_PATH, the origin AS,
is not the peer's AS, the redirect action is refused. No match, an AS_SET origin
and an empty path refuse too. "Last AS" is read as the origin because the
draft's authors restate the check in draft-ietf-idr-flowspec-redirect-ip-02 as
"the best matching route to X is not a BGP route with origin AS matching the
peer AS". FRR and BIRD do not implement this check. A refused rule stays
feasible. The verdict is per rule, because two rules of one flow-vpn UPDATE can
carry two Route Distinguishers and so two domains. The selected install loses
its 0x0800 values, and the snapshot records the refusal on the rule's own
verdict (`ribevents.FlowSpecRedirectRefused`, keyed by the rule and its received
generation). The forward rail's validation split (`forwardValidationWire`) asks
it for every announced rule and moves the refused rules into a copy of the
UPDATE with the received attributes. Every destination receives that copy
without any 0x0800 community and with the zero next-hop length
(`discardRedirectAction`), so the wire and the install agree rule by rule. The
split asks by the received generation, never by the forward cache id, so a
validation replay of the stored route carries the verdict too. FlowSpec never
takes the route-server fast rail (`sourceUsesCachedForward`). A change of the
verdict alone is a validation change, so the path is sent again. `flowspec-redirect-check { enabled false; }` at the
bgp, group or peer level turns the check off for a session. The peer's value
beats the group's, and the group's beats the bgp level's. When no level states
the leaf, the check runs.
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_redirect.go -- flowSpecRedirectRefused, parseFlowSpecRedirectConfig -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- forwardValidationWire, flowSpecSection -->
<!-- source: internal/component/bgp/reactor/forward_next_hop.go -- discardRedirectAction -->
<!-- test: internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go TestFlowSpecRedirectToIPEBGPCheck -->
<!-- test: internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go TestFlowSpecRedirectVPNVerdictPerRule -->
<!-- test: internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go TestFlowSpecRedirectUnknownLocalASIsChecked -->
<!-- test: internal/component/bgp/reactor/forward_validation_test.go TestForwardValidationSplitsRedirectRefusedRules -->
<!-- test: internal/component/bgp/reactor/flowspec_redirect_refused_wire_test.go TestFlowSpecRedirectRefusedRouteServer -->
<!-- test: internal/component/bgp/reactor/flowspec_redirect_refused_wire_test.go TestFlowSpecRedirectRefusedReplay -->
<!-- test: test/plugin/flowspec-redirect-check-refused.ci -->
<!-- test: test/plugin/flowspec-redirect-check-disabled.ci -->

`reconcileFlowSpecs` recomputes authorization after received-route changes,
unicast best-path changes, validation changes, withdrawals and session removal.
It selects one eligible path per native FlowSpec NLRI with the ordinary BGP
comparison. ADD-PATH identifies received candidates but is not part of the rule
identity. Equivalent one- and two-octet length fields share a canonical native
key with the shortest length encoding. An immutable snapshot records each path's generation and feasibility,
and the selected path's action communities. Pool handles are released before
publication.
The advertised MP_REACH next-hop bytes are ignored for FlowSpec, including
self-address and non-address values; they do not drive next-hop eligibility or
interior-distance selection.

The selected state emits both the ordinary best-change notification and the
typed `ribevents.FlowSpecChanged` event. The latter owns the full encoded native
NLRI, without an ADD-PATH identifier, and the winning path's concatenated
eight-octet extended communities and twenty-octet IPv6 extended communities in
separate fields. A replacement changing either action attribute emits a new
install; loss of the winner emits a withdrawal unless another feasible path
wins. `ReplayRequest` also emits the selected actions. Changes and replay are
serialized without holding RIB locks during event delivery.

`RouteEligible` combines this mandatory gate with any optional validation gate.
`RoutePresent` preserves ADD-PATH ownership while an infeasible path is retained.
Route-server and reflector recovery fetch current attributes from
`LookupFlowSpecPath` and send them through the ordinary generation-sensitive
reactor gate. On every peer-up, route servers and reflectors enumerate these
authorized paths before End-of-RIB, whether optional Adj-RIB-In replay succeeds
or fails. Adj-RIB-In excludes FlowSpec copies from its ordinary store replay so
there is only one snapshot source. When Adj-RIB-In itself owns replay for a
peer, it fetches the mandatory RIB's authorized paths before reporting ready;
it stands down for a delegated owner, including the per-peer unheld-role
exception. Its bounded relay preserves source identity and generation and uses
the reactor's destination session, family and export checks. Route-server
snapshots carry every current path, including a generation received after the
destination's peer-up cut: the live forward carries no FlowSpec rule, and the
RIB's ValidationChange for that generation is not ordered against the route
server's peer-up, so it can be processed before the destination is a target.
A rule judged in that window can reach the destination twice as the same
announcement, never not at all. RS and RR also reject stale destination
replay generations.
<!-- test: internal/component/bgp/plugins/rs/server_validation_test.go TestPeerUpReplaysFlowSpecJudgedBeforeTheTargetWasUp -->
Retained attribute reconstruction includes the separately pooled Extended
Communities field. Authorization recovery therefore keeps a rule's traffic-rate
action rather than advertising the same NLRI as a no-action rule when the first
live forward preceded authorization.
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go -- RouteEntry.ToWireBytes -->
<!-- test: internal/component/bgp/reactor/flowspec_rs_wire_test.go TestFlowSpecRouteServerOmitsNextHop -->
<!-- test: internal/component/bgp/reactor/flowspec_rs_replay_wire_test.go TestFlowSpecRouteServerReplayKeepsActions -->
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_validation.go -- reconcileFlowSpecs, flowSpecAuthorized, drainFlowSpecEvents -->
<!-- source: internal/core/bgp/ribevents/flowspec.go -- FlowSpecChanged, LookupFlowSpecPath, FlowSpecRoutes -->
<!-- source: internal/component/bgp/plugins/rs/server_validation.go -- processValidation, replayFlowSpecs -->
<!-- source: internal/component/bgp/plugins/rr/validation.go -- startValidation, replayValidation, replayFlowSpecs -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- handleStructuredState, handleState -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_replay_path.go -- replayFlowSpecs -->

### Who Reads a Best-Path Change

`publishBestChanges` (`rib_bestchange.go`) turns a best-path change into an
event, and it has two readers. It emits the batch on `(bgp-rib, best-change)`,
which `system-rib` takes for FIB arbitration. It also calls
`redistribute.EmitBestChange`, which converts the same batch into a
`redistevents.RouteChangeBatch` under the `bgp` protocol id.

It is not the only caller of `EmitBestChange`. `replayRedistribute`
(`rib_bestchange.go`) answers a consumer that registered late: it walks the
whole best-path table with `collectBestPaths` and emits directly, carrying the
request's `ReplayID` so the orchestrator routes the answer to that one consumer.
A reader tracing where a redistributed route comes from has two entry points to
account for, not one.

That second call is the whole of BGP's redistribution source.
`redistribute { destination ospf { import bgp } }` moves a route because this
function ran.

So a Loc-RIB holding nothing is redistribution producing nothing. The Loc-RIB
holds a route only where a peer's config feeds it the UPDATE. That delivery is
derived from the rule rather than written by the operator. See
[API architecture](../api/architecture.md), "The Two Bindings Ze Derives".
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- publishBestChanges -->
<!-- source: internal/component/bgp/redistribute/producer.go -- EmitBestChange -->

### Retained selection for ordinary advertisement

The RIB also captures its current learned winners as immutable native route
values. `checkRouteBestChange` prepares each value while its winning candidate
still retains the received entry and labels. It commits the value under the
same selection shard that records the winner. The FlowSpec authorization pass
captures its own winner before releasing its retained entry. Neither path runs
another election or reads the route again after releasing that entry.

The snapshot owns its attributes, next hop and NLRI independently of received
pool storage. Its native advertisement NLRI without the source ADD-PATH prefix
is kept separately from the received path framing. Path identifier zero remains
an ADD-PATH identifier when the framing says so. CIDR routes use their prefix as
the internal key; other families use their registered native identity, so VPN
and EVPN routes sharing an IP portion remain distinct.

Each changed selection gets a positive revision from one manager-wide clock.
An identical receipt or a losing candidate does not advance it. A change to the
stored source-policy epoch or stale treatment does: those facts belong to the
captured revision even when the monitoring best-path record stays equal. The
mark-stale command re-runs the existing election after releasing the peer lock,
so a GR or LLGR transition updates the selected value without another UPDATE.
An old snapshot is never restamped or mutated.

Label comparison uses owned label values rather than a released pool handle:
the production pool can reuse a numeric slot for a different label stack.
Capture borrows the winner's already resolved values for comparison and owns
a copy only when the selection changes. The same resolved values reach
`recoveryCandidateWithLabels`, so wire reconstruction does not decode the
label pool again. Legacy recovery callers retain `recoveryCandidate`, which
resolves their labels and delegates to that shared reconstruction.

The capture retains current selections only. Removing an unsent route releases
its current record; reappearance uses a newer revision without a historical
per-key counter or tombstone. A failed extraction or exhausted revision clock
refuses the snapshot explicitly instead of presenting an old table as current.
The selection shard or FlowSpec lock precedes the capture mutex; code holding
that mutex takes no selection or peer lock and calls no engine operation.

An extraction failure or exhausted revision clock latches the original cause.
The RIB declares `bgp-rib-selected-output` at Stage 1 and answers that runtime
doctor check in its own process, so a forked RIB reads the same state as an
internal one. The finding and report-bus event use
`doctor-bgp-selected-output`, which the RIB registers as an event code from
its `init()` so the daemon accepts its `report-issue` raises and clears. The
Stage 1 doctor declaration does not replace that event registration. The
capture failure gives the restart command for the configured instance.
Before an instance name is acknowledged, the repair is
`ze request restart`. Capture cannot resume from an incomplete snapshot, so
the failure remains visible until the deciding process restarts.

Source-lease failures are scoped to the affected source. They appear beside
capture failures in the same runtime doctor check and under the same declared
code, with a separate report-bus subject for each source. They do not invalidate
healthy captured selections or other sources. Failed releases remain pending;
the RIB's one lease worker retries them and clears the source warning only after
settlement. A later successful acquisition or retention clears its admission
failure. Permanent source removal discards its admission failure after purge;
an unsettled release still remains owed. Only acknowledged warnings awaiting
their clear survive after the underlying source state has gone.

A synchronous source-policy phase and stale purge settle only their source's
explicit receipt list, including its pending final releases. They do not retry
or join another source's failed release. The background lease worker retains
the global retry scan. These source selectors, failures and reports are keyed
by the captured Peer owner, so one configured source does not settle another
source's receipts at the same address. Release authority remains the
authenticated process incarnation and exact receipt ID.

A withdrawal election keeps the departed peer slot until its synchronous
Loc-RIB removal finishes. A concurrent DOWN can therefore join that original
writer after its final best record has left the table.
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- checkRouteBestChange, purgeBestPrevForOwner -->

The deciding process owns its received stores and their pooled references even
when it runs inside the daemon. SDK event delivery drains before shutdown;
shutdown then closes new timer and intake admission, cancels and joins its
AIGP, metric and compaction workers and every scheduled GR callback, and joins
selected and recovery writers. It releases received stores, sent inventory and
source-policy undo references only after those readers finish. A replaced GR
timer remains counted until its callback finishes, even though the current GR
map no longer names it. The source-lease worker stops before local references
are released: a closed plugin connection cannot issue a new release RPC.
Engine generation cleanup revokes that ended owner's authority and clears its
report-bus warnings; local shutdown releases only the plugin's own references.
Refused late intake and mark-stale commands return before registering any
reconciliation, so they cannot repopulate the selected capture. Source warning
admission also closes before teardown, and shutdown joins reports already
admitted before releasing local state. Another RIB manager's pooled route and
source references survive this owner's shutdown.
<!-- source: internal/component/bgp/plugins/rib/rib_source_shutdown.go -- shutdownReceivedSources, startRIBWorkers, stopGRExpiryLocked -->
<!-- source: internal/component/plugin/server/dispatch.go -- cleanupProcess -->

The selection, purge and FlowSpec return paths report only after releasing
their locks. A healthy election reads one atomic reporting flag. A failed
capture has at most one report request in flight, bounded to five seconds;
an unacknowledged request stays owed and retries at most once per report-bus
window. An acknowledged raise is sent once. Doctor reads the failure without
mutating or re-reporting it.

The private ordinary-output dispatcher holds the current desired selections
and each admitted destination Session's acknowledged output and single bounded
batch. A chosen batch owns arrays sized to that batch, rather than views into
the remaining pending population. Request and widest-response sizes come from
the RPC envelope definitions; immutable capture values cache the steady,
initial and absence item budgets before publication. Choosing a batch does not
marshal the remaining table while holding the publication mutex.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_batch.go -- newSelectedScopedRoute, selectedDispatchSizedBatch -->
<!-- source: pkg/plugin/rpc/selected.go -- SelectRoutesExchangeSizes, SelectRoutesItemSizes -->

Capture failure is published into the dispatcher's current-selection answer
and completion barriers in the same transition. A previously acknowledged
route cannot remain an authorized answer after its newer capture failed.
An empty source-receipt set has no affected destination, but does not hide a
failed selection publication.

Destination output and diagnostics have separate lifetimes. Ending a Session
joins its worker and releases its route projections even when a warning clear
fails. One dispatcher-owned reporter reconciles one aggregate warning under a
fixed subject; the runtime doctor check reads current failed destinations and
their causes. Normal queued or in-flight output is visible through the neutral
`selected-output` status projection rather than a fabricated doctor failure.
The reporter retains only the current aggregate, its acknowledged value and
one uncertain outcome, never an ended-Session ledger. A failed report may have
applied without an answer, so a later healthy aggregate still clears that
possible warning. Process generation cleanup owns the final report clear.
Every attempted raise starts a `rpc.ReportRaiseWindow` interval, including a
successful refresh of the aggregate warning. Changes during that interval
coalesce to the latest current cause; an unchanged acknowledged cause needs no
refresh. Clearing a recovered warning is immediate and does not renew the raise
allowance. A failed clear retries after one second. Thus this one finding cannot
consume the plugin's shared raise quota as destinations change, while doctor
continues to show their current causes.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_diagnostic.go -- run -->

The dispatcher admits ordinary destinations from authenticated public state;
its event-settlement callback joins their queued and in-flight output. The
ordinary routing workflow still requires the release spec's remaining refresh
and origin/source lifecycle integration. Current peer metadata cannot authorize
an older retained path. Existing route-server and reflector recovery continues
to use its own candidate population.

<!-- source: internal/component/bgp/plugins/rib/rib_advertise_capture.go -- advertiseState, advertiseSnapshot, prepareAdvertise, commitAdvertise, removeAdvertise, reelectAdvertiseStale -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- checkRouteBestChange, withdrawIfUnheld -->
<!-- source: internal/component/bgp/plugins/rib/rib_flowspec_validation.go -- reconcileFlowSpecs -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- markStaleCommand -->
<!-- source: internal/component/bgp/plugins/rib/rib_advertise_diagnostic.go -- advertiseDoctorDeclaration, advertiseDoctorCheck, reportAdvertiseAfterElection, reportAdvertiseIssue -->
<!-- source: internal/component/bgp/plugins/rib/rib_recovery.go -- recoveryCandidate, recoveryCandidateWithLabels -->

### Generic `Store[T]` and bestPrev Consolidation

The BART-vs-map dispatch is factored into `storage.Store[T]` (a generic
NLRI-keyed store). `FamilyRIB` is one user; the best-path change tracker
(`rib_bestchange.go`) is the other. `FamilyRIB` is single-mode (mode fixed at
peer-OPEN time from negotiated ADD-PATH capability). Best-path tracking is
cross-peer and per PREFIX: a CIDR family's `bestPrevStore` holds one
BART-backed `*Store[bestPrevRecord]` with one record per prefix, whatever the
ADD-PATH mode of the sessions that carry it. The record packs the winner and
names the path that won by its received path id and ADD-PATH flag, which is
what the published best-change carries.

Candidates for a CIDR prefix are gathered by prefix:
`PeerRIB.AppendPrefixPathsRetained` appends every stored path, one per path id
under ADD-PATH, into a caller-owned slice backed by a stack array. It retains
the entry and label handles under the storage lock. Gathering transfers those
references into candidates; rejected paths call `PrefixPath.Release`.
A peer is never asked with another session's wire key, because a key framed for
ADD-PATH reads as a different prefix in a peer stored without it. Winner metadata
comes from the retained candidate revision, not a second lookup. RFC 8277
Section 3.1 makes two paths of one ADD-PATH session comparable, so the path id
names a path and never
partitions the election. Two paths that tie on every RFC 4271 step are ordered
by the lowest path id, a Ze tie-break rather than RFC text.
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- AppendPrefixPathsRetained, AppendKeyPathsRetained -->
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go -- PrefixPath.Release -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- gatherPrefixCandidatesLocked, releaseCandidates -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- checkRouteBestChange -->

A changed winner reserves a peer-address interner reference before releasing
`peerMu`. Successful insertion transfers it to the stored `bestPrev` record;
failed next-hop or metric admission releases it. This makes DOWN fence an
admitted first publication even when the peer has no prior winning record.
Replacement or removal releases the old record's reference; a same-best
decision acquires none. Peer purge pins the slot across its shard scan and
releases each removed record separately. A same-address replacement in an
already-scanned shard therefore retains its identity; only the final record,
publication, or scan release returns the slot to the free list.
A replacement in a shard still to be scanned is
re-elected before Loc-RIB removal, without an intermediate withdrawal.
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- checkRouteBestChange, internPeer, retainPeer, releasePeer, purgeBestPrevForPeer, emitPurgedWithdraws -->

The Loc-RIB mirror writes one BGP path per prefix under Instance 0
(`bgpLocRIBInstance`): the RFC 4271 decision is already made here, and a second
BGP path in the Loc-RIB would be ranked by distance and metric alone. A peer
going down drops the records its paths won (`purgeBestPrevForPeer`), and
`emitPurgedWithdraws` then re-elects each of those routes from the paths that
remain before it publishes anything: a route with a survivor goes to it in one
Update and never leaves the Loc-RIB, and only a route left with no candidate is
withdrawn. Each route is published as soon as its election answers, one batch
per route. The check that no best is recorded for a withdrawn route and its
removal from the Loc-RIB run under one hold of the bestPrev shard lock
(`withdrawIfUnheld`), the lock an UPDATE's election records and mirrors under,
so a best a concurrent UPDATE elects for the route is never removed behind it.

Other non-CIDR families take the same opaque-map backend `FamilyRIB` does, for the
same reason: its NLRI leads with a label stack and a Route Distinguisher, or
with a route type, so `store.NLRIToPrefix` names no `netip.Prefix` for it. The
key is the route key: the wire bytes without the ADD-PATH path identifier, less
every field the family's registered key operation (`nlrisplit.GetPrefixKey`)
says does not identify the route (`storage.RouteKey`). A VPN route loses its
label stack, an EVPN route its labels, ESI and gateway, so two PEs announcing
one RD and prefix under two labels are two paths of one route (RFC 8277
Section 3.1), a relabel replaces the route (Section 2.5), and a withdrawal whose
label field is the Compatibility value is read with the withdrawal framing
(`FamilyRIB.Withdraw`, Section 2.4). The election that follows reads it the
same way: `checkRouteBestChange` computes the route key once, with the framing
the NLRI arrived in, and that one key gathers the candidates
(`gatherKeyCandidates`) and finds the stored best, so a withdrawal from one PE
promotes the other PE's path whatever its Compatibility field holds. The wire route each path was received with,
labels included, is kept beside the key (`FamilyRIB.wire`, only for a route
whose key differs from it), and every walk, `PrefixPath.Route`,
`Candidate.Route` and the stored best (`opaqueBestPrev`) read the labels from
there. Under
ADD-PATH `FamilyRIB` keeps a `pathSet` per route key (`opaqueMulti`), as it does
per CIDR prefix, so the path identifier names a path of the route and never a
second route (RFC 7911 Section 2). `PeerRIB.AppendKeyPathsRetained` hands every
retained path of the key to `gatherKeyCandidatesLocked`, which asks every peer
by the route key, so the paths of one ADD-PATH session and the same route from
a session without ADD-PATH meet in one election. Callers MUST release each
retained path after its last read or transfer its handles to another owner.
A walk hands each path back framed as its session sent it. In `FamilyRIB`, a FlowSpec key uses the shortest length field,
so both framings of one rule share one stored route (`storage.RouteKey`), and
`storage.IsCIDRFamily` is the one predicate both stores partition by. Such a
route is published on `(bgp-rib, best-change)` through
`ribevents.BestChangeEntry.NLRI`, the winner's own wire NLRI with `AddPath` and
`PathID` saying how it is framed, with a zero `Prefix`, and it is NOT mirrored
into the unified Loc-RIB: that store is `netip.Prefix`-keyed down to sysrib's
FIB arbitration, two Route Distinguishers share one IP prefix, and ze has no
VRF to install them into.

The `maprib` build tag degrades `Store[T]` to map-only storage for both
backends; `FamilyRIB` retains its two build-tagged files
(`familyrib_bart.go`, `familyrib_map.go`) wrapping the corresponding
`Store[T]` variant.
<!-- source: internal/core/rib/store/store_bart.go -- Store[T] default BART+map dispatch -->
<!-- source: internal/core/rib/store/store_map.go -- Store[T] map-only under maprib -->
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- bestPrevStore picks BART or opaque map per family -->
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib_opaque.go -- opaqueRouteKey, RouteKey, routeNLRI, appendKeyPathsRetained -->
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- AppendKeyPathsRetained -->
<!-- source: internal/core/bgp/nlri/nlrisplit/prefix_key.go -- GetPrefixKey -->

`bestPathRecord` is a named `uint64` -- four 16-bit fields (MetricIdx,
PeerIdx, NextHopIdx, Flags bit 0 = isEBGP) packed into one scalar. The
three index fields resolve through a shared `bestPrevInterner` on the
RIBManager (one interner across every family's `bestPrevStore`), which
dedupes captured `(peer owner, peer address)` pairs, next-hops, and received MED/AIGP tuples to dense uint16
indices. The hot-path same-best check compares that `uint64`, the path
identifier, and the winner's attribute fingerprint
(`storage.RouteEntry.AttrFingerprint`, an FNV-1a hash of the received
attribute bytes) that `bestPrevRecord` keeps beside it. Without the
fingerprint, a re-announcement that changed only an attribute (a COMMUNITY,
a LOCAL_PREF) under the same winner published no best change, and BMP's
Loc-RIB kept the old set. A pool handle cannot stand in for the fingerprint:
handles are reused once released, and the record holds no reference. A zero
fingerprint means "not known" (an entry rewritten in place clears it) and
never compares equal, so the change is published. The record is 24 bytes per
prefix. Emission resolves indices back to their original values via reverse
tables. BART fringe nodes are opaque to GC because the stored value
carries no pointers -- the motivation for the pack is cutting GC scan
time at 1M-prefix scale.

Two configured peers may share an address. Their winning records retain distinct
owner slots, and the same-best check compares the captured owner as well as the
address and route attributes. After all protocol criteria and the path identifier
tie, the lower stable owner resolves the remaining tie. This final step is Ze's
deterministic choice. An exact-owner purge removes only that owner's records;
the deliberate address-wide purge used by a peer selector visits every matching
owner. Each scan pins its slots until its last shard, so a removed winner's slot
cannot be reused beneath another scan or a surviving sibling record.
<!-- source: internal/component/bgp/plugins/rib/rib_bestchange.go -- bestPathRecord uint64 + bestPrevInterner, bestPrevRecord, sameAttributeSet -->


### ADD-PATH Support (RFC 7911)

When ADD-PATH is negotiated, path ID is part of prefix identity:

```
Without ADD-PATH:
  [prefix-len:1][prefix-bytes:0-4]

With ADD-PATH:
  [path-id:4][prefix-len:1][prefix-bytes:0-4]

The path-id + prefix together form the unique key of a stored path.
Same IP prefix with different path-ids = different paths of one prefix,
compared in one best-path election (RFC 8277 Section 3.1).
```

Each path of a labeled unicast (SAFI 4) prefix holds its own MPLS label handle,
in its `pathEntry` beside the `RouteEntry`. RFC 8277 Section 2.5 binds a label
per path: an UPDATE for the same path id replaces that path's binding.
`pathSet.upsert` keeps the old handle until `setLabels` replaces and releases
it, so a concurrent election cannot see an unbound path between insert and
rebind. An UPDATE or withdrawal for another path id leaves it in place.
Without ADD-PATH the family keeps one handle per prefix in
a parallel trie. Removing the route releases its binding with it, so no caller
removes a label alone.

A withdrawal frames its NLRI as `[Length][Compatibility(3)][Prefix]` (RFC 8277
Section 2.4), never as a label stack: the 0x800000 the RFC recommends has its S
bit clear, and the announcement's walk to the bottom of the stack would read past
the NLRI. Every RIB withdrawal path splits with `nlrisplit.SplitWithdrawn`, and
`LabeledWithdrawnPrefix` skips the three octets whatever they hold, so the
withdrawal reaches the prefix the route was stored under.
<!-- source: internal/component/bgp/plugins/rib/storage/pathset.go -- pathEntry, upsert, setLabels, remove -->
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go -- SetLabels, LookupLabels, Withdraw, LabeledWithdrawnPrefix -->
<!-- source: internal/core/bgp/nlri/nlrisplit/nlrisplit.go -- SplitWithdrawn -->

See `DirectNLRISet.nlriLen()` for parsing implementation.

### Reverse Index

For efficient prefix lookup and withdrawal:

```go
// Insert adds NLRI to RIB, managing pool refcount internally.
// Caller passes raw attr bytes, RIB manages all pool references.
// Key invariant: each unique attrHandle in r.entries has exactly ONE pool ref.
func (r *FamilyRIB) Insert(attrBytes []byte, nlri []byte) {
    h := pool.Attrs.Intern(attrBytes)  // refcount++ (1 for new, +1 for existing)

    // Check reverse index for implicit withdraw
    nlriKey := string(nlri)  // NLRI bytes as key (includes path-id if ADD-PATH)
    if oldHandle, exists := r.prefixIndex[nlriKey]; exists {
        if oldHandle != h {
            // Implicit withdraw: remove from old attr set
            r.removeFromSet(oldHandle, nlri)
        } else {
            // Same prefix, same attrs → no-op (route refresh)
            pool.Attrs.Release(h)  // Undo Intern's ref
            return
        }
    }

    // Add to forward index
    set, exists := r.entries[h]
    if exists {
        pool.Attrs.Release(h)  // Already own a ref for this attr set
    } else {
        // New attr set in RIB - keep the ref from Intern()
        set = NewNLRISet(r.family, r.addPath)
        r.entries[h] = set
    }
    set.Add(nlri)

    // Update reverse index
    r.prefixIndex[nlriKey] = h
}

// Lookup by prefix
func (r *FamilyRIB) Lookup(nlri []byte) (pool.Handle, bool) {
    handle, exists := r.prefixIndex[string(nlri)]
    return handle, exists
}

// Remove removes NLRI from RIB, releasing pool ref when attr set empty.
func (r *FamilyRIB) Remove(nlri []byte) bool {
    nlriKey := string(nlri)
    h, exists := r.prefixIndex[nlriKey]
    if !exists {
        return false
    }

    set := r.entries[h]
    set.Remove(nlri)
    delete(r.prefixIndex, nlriKey)

    // Last NLRI removed → release RIB's ref to attrs
    if set.Len() == 0 {
        set.Release()              // Release NLRI handles if pooled
        delete(r.entries, h)
        pool.Attrs.Release(h)      // Release RIB's single ref
    }
    return true
}
```

### NLRISet Interface

```go
// NLRISet stores NLRIs for one attribute set
type NLRISet interface {
    // Add appends NLRI to the set
    Add(nlri []byte)

    // Remove removes NLRI from the set, returns true if found
    Remove(nlri []byte) bool

    // Contains checks if NLRI exists
    Contains(nlri []byte) bool

    // Iterate calls fn for each NLRI (wire bytes)
    Iterate(fn func(nlri []byte) bool)

    // Len returns number of NLRIs
    Len() int

    // Release frees any pool handles (no-op for direct)
    Release()
}
```

### DirectNLRISet (IPv4 Unicast/Multicast)

```go
// DirectNLRISet stores small NLRIs as concatenated wire bytes
// Used for IPv4 where NLRI (1-5 bytes) < handle overhead
type DirectNLRISet struct {
    data    []byte     // Concatenated wire-format NLRIs
    count   int        // Number of NLRIs (avoid re-parsing for Len())
    family  FamilyKey  // AFI + SAFI
    addPath bool       // If true, NLRIs have 4-byte path-id prefix
}

// nlriLen returns the wire length of an NLRI at offset
func (s *DirectNLRISet) nlriLen(offset int) int {
    if offset >= len(s.data) {
        return 0
    }
    if s.addPath {
        // ADD-PATH: [path-id:4][prefix-len:1][prefix-bytes]
        if offset+4 >= len(s.data) {
            return 0
        }
        prefixLen := s.data[offset+4]
        return 4 + 1 + (int(prefixLen)+7)/8
    }
    // Standard: [prefix-len:1][prefix-bytes]
    prefixLen := s.data[offset]
    return 1 + (int(prefixLen)+7)/8
}

func (s *DirectNLRISet) Add(nlri []byte) {
    s.data = append(s.data, nlri...)
    s.count++
}

func (s *DirectNLRISet) Remove(nlri []byte) bool {
    offset := 0
    for offset < len(s.data) {
        length := s.nlriLen(offset)
        if length == 0 {
            break
        }
        if bytes.Equal(s.data[offset:offset+length], nlri) {
            // Found - remove by shifting remaining data
            copy(s.data[offset:], s.data[offset+length:])
            s.data = s.data[:len(s.data)-length]
            s.count--
            return true
        }
        offset += length
    }
    return false
}

func (s *DirectNLRISet) Contains(nlri []byte) bool {
    offset := 0
    for offset < len(s.data) {
        length := s.nlriLen(offset)
        if length == 0 {
            break
        }
        if bytes.Equal(s.data[offset:offset+length], nlri) {
            return true
        }
        offset += length
    }
    return false
}

func (s *DirectNLRISet) Iterate(fn func(nlri []byte) bool) {
    offset := 0
    for offset < len(s.data) {
        length := s.nlriLen(offset)
        if length == 0 {
            break
        }
        if !fn(s.data[offset : offset+length]) {
            return
        }
        offset += length
    }
}

func (s *DirectNLRISet) Len() int {
    return s.count
}

func (s *DirectNLRISet) Release() {} // No-op, GC handles data
```

### PooledNLRISet (IPv6, VPN, EVPN, FlowSpec)

```go
// PooledNLRISet stores large NLRIs as pool handles
// Used for IPv6+ where NLRI > handle size (4 bytes)
type PooledNLRISet struct {
    handles []pool.Handle
    family  FamilyKey  // AFI + SAFI
    addPath bool       // If true, NLRIs have 4-byte path-id prefix
}

func (s *PooledNLRISet) Add(nlri []byte) {
    h := pool.NLRIs.Intern(nlri)
    s.handles = append(s.handles, h)
}

func (s *PooledNLRISet) Remove(nlri []byte) bool {
    // Lookup without modifying refcount
    targetHandle, exists := pool.NLRIs.Lookup(nlri)
    if !exists {
        return false
    }

    for i, h := range s.handles {
        if h == targetHandle {
            // Found - remove by swap with last
            pool.NLRIs.Release(h)  // Release our ref
            s.handles[i] = s.handles[len(s.handles)-1]
            s.handles = s.handles[:len(s.handles)-1]
            return true
        }
    }
    return false
}

func (s *PooledNLRISet) Contains(nlri []byte) bool {
    // Lookup without modifying refcount
    targetHandle, exists := pool.NLRIs.Lookup(nlri)
    if !exists {
        return false
    }

    for _, h := range s.handles {
        if h == targetHandle {
            return true
        }
    }
    return false
}

func (s *PooledNLRISet) Iterate(fn func(nlri []byte) bool) {
    for _, h := range s.handles {
        if !fn(pool.NLRIs.Get(h)) {
            return
        }
    }
}

func (s *PooledNLRISet) Len() int {
    return len(s.handles)
}

func (s *PooledNLRISet) Release() {
    for _, h := range s.handles {
        pool.NLRIs.Release(h)
    }
    s.handles = nil
}
```

### Factory

```go
// NewNLRISet creates appropriate implementation for family
func NewNLRISet(family FamilyKey, addPath bool) NLRISet {
    if shouldPoolNLRI(family.AFI, family.SAFI) {
        return &PooledNLRISet{fam: family, addPath: addPath}
    }
    return &DirectNLRISet{fam: family, addPath: addPath}
}
```

### Why Key by Attributes?

1. **Deduplication:** Many routes share same AS_PATH, communities, etc.
2. **Efficient lookup:** Find all routes with same attributes
3. **UPDATE building:** Group routes by attrs for efficient encoding

### Route Change Time

Each stored route holds the time its stored attributes last changed, so a
reader can ask how long a route has been stable. `show bgp rib` answers it as
the `age` member of a received row, and the looking glass shows that age.

**Design:** `ChangedAt storage.RouteTime` on `RouteEntry`: per-route metadata
like `StaleLevel`, never pooled. A `RouteTime` is one reading of the RIB's route
clock: the whole seconds elapsed since the epoch the RIB took when it started,
plus one, in four bytes.

| Choice | Reason |
|--------|--------|
| Seconds since the RIB's own start, not since the Unix epoch | An age is a difference of two readings. Readings of one process's monotonic clock do not move when the wall clock is stepped, so an NTP correction never makes a route older or younger |
| Plus one | Zero stays free. The zero value is `RouteTimeUnrecorded`: the route was stored without a reading. `RouteTime.AgeAt` answers no age for it, and the row then carries no `age` member, where a zero would read as "changed this second" |
| One-second resolution, four bytes | A looking glass shows ages in seconds, and four bytes hold 136 years of uptime. A reading the type cannot hold is `RouteTimeUnrecorded`, never a wrapped small number |
| Nothing is persisted | A reading means nothing outside the process that took it. After a restart every route is relearned and is new |

**When it is written.** Only when a route's stored attributes change: on its
first insert and on an announcement whose attributes differ. A re-announcement
with equal attributes keeps the stored reading, on every insert path, because
the equal branches write the stored entry back and copy only the received
identity onto it. The test is the one that already decides a no-op: the
attribute fingerprint and length, then `entriesEqual` on the parsed handles. A
route refresh therefore leaves every stable route's age growing. A new label
stack under equal attributes does not move the time either, and neither does an
in-place change through `ModifyFamilyEntry` (a stale mark, the received
identity).

| Insert path | Where the reading comes from |
|-------------|------------------------------|
| Received UPDATE, attributes parsed once (`handleReceivedStructured`) | One reading for the UPDATE, set on the parsed entry and stored by `InsertEntry` for each NLRI, ADD-PATH and non-CIDR families included, and by `insertLabeledEntry` for labeled unicast |
| Received UPDATE, attributes not parsed once | The same reading, handed to `PeerRIB.InsertAt`. `insertLabeled`, the labeled form of this fallback, reads the clock itself for each route: it already parses the attributes for each route |
| JSON event path (`insertPoolNLRIs`) | One reading for each NLRI section of the event |
| `request bgp rib inject` (`injectRoute`) | One reading for the command, handed to `PeerRIB.InsertAt` |
| An injected wire UPDATE (`handleInjectWireRoute`) | One reading for the UPDATE |
| `PeerRIB.Insert` and `FamilyRIB.Insert` | None: the route is stored with `RouteTimeUnrecorded`. No shipped caller uses them |

**The clock.** `RIBManager.routeClock` wraps an `internal/core/clock` clock and
the epoch it read when `newRIBManager` built it, so a test hands the RIB a clock
it moves. The handlers read it once per received message, never per route,
apart from the fallback the table names. A `RIBManager` built without
`newRIBManager` holds no clock: every reading is `RouteTimeUnrecorded` and its
routes have no age.

**Best-path selection never reads it.** `extractCandidate` does not copy the
time into a `Candidate`: an age is information about a route, never a reason to
prefer it.

**Cost.** The four bytes fill padding the record already had, between its three
one-byte fields and the message ID. The record is expected to stay 64 bytes, to
be confirmed by `TestRouteEntrySize`, which also pins the field's offset.
`RouteEntry.Clone` copies the time, so a retained or cloned route keeps its age.

<!-- source: internal/component/bgp/plugins/rib/storage/routetime.go -- RouteTime, RouteTimeUnrecorded, RouteTimeSince, RouteTime.AgeAt -->
<!-- source: internal/component/bgp/plugins/rib/storage/routeentry.go -- RouteEntry, RouteEntry.Clone -->
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go -- FamilyRIB.InsertAt, FamilyRIB.InsertEntry, entriesEqual -->
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- PeerRIB.InsertAt, PeerRIB.InsertEntry -->
<!-- source: internal/component/bgp/plugins/rib/rib_route_age.go -- routeClock, newRouteClock -->
<!-- source: internal/component/bgp/plugins/rib/rib_structured.go -- handleReceivedStructured, insertLabeled -->

### Stale Route Tracking (Graceful Restart)

RFC 4724 requires the Receiving Speaker to mark routes as stale when a GR-capable peer
drops, retain them during the restart window, and selectively purge only stale routes
when EOR arrives — preserving fresh routes received during the GR window.

**Design:** `StaleLevel uint8` on `RouteEntry` — per-route metadata, NOT a pooled attribute.
Stale state is not shared via dedup because each route instance has independent stale status.
Level 0 means fresh (not stale). Plugins define non-zero levels and pass them via RIB commands.
`DepreferenceThreshold` (default 2) is the level at which best-path deprioritizes routes.
`ReceivedLLGRStale bool` sits beside it: the GR plugin's ingress step writes
`meta["stale"] = 2` for a route received carrying LLGR_STALE from a neighbor Ze sent
the LLGR Capability to (RFC 9494 Section 4.3), the reactor puts that metadata on the
received event, and `handleReceivedStructured` writes the flag on every insert of the
route, the no-op branches included, so it follows the latest UPDATE. No stale
operation below reads or writes it. `storage.AdvertisedStaleLevel` combines the two
into the higher one, and both readers call it: `extractCandidate` for best path, and
the Adj-RIB-Out rails (peer-up replay, resend, route refresh) for the `meta["stale"]`
they send. An Adj-RIB-Out copy holds no received flag of its own. `sentSourceState`
reads the flag from the source's current path by add-path mode and path identifier,
whatever its MsgID, in the same lookup that tells the peer-up replay whether the
copy is still current.

<!-- source: internal/component/bgp/plugins/rib/storage/routeentry.go — StaleLevelFresh, DepreferenceThreshold, AdvertisedStaleLevel -->
<!-- source: internal/component/bgp/plugins/rib/sent_owner.go — sentSourceState, replayStaleLevel -->

| Operation | Method | Scope |
|-----------|--------|-------|
| Mark stale | `FamilyRIB.MarkStale(level)` | All routes in one family at given level |
| Purge stale | `FamilyRIB.PurgeStale()` | Only stale routes (level > 0) in one family |
| Stale count | `FamilyRIB.StaleCount()` | Count stale routes (level > 0) |
| Implicit unstale | `FamilyRIB.Insert()` | New entry always has `StaleLevel = 0` |

<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go — FamilyRIB stale methods -->

`PeerRIB` provides aggregate methods: `MarkAllStale`, `PurgeFamilyStale`, `PurgeAllStale`, `StaleCount`.

**RIB commands:** `rib mark-stale <peer> <restart-time> [level]` and `rib purge-stale <peer> [family]`.
Called by bgp-gr plugin via `DispatchCommand`. The optional level parameter defaults to 1
(GR-stale); level 2 triggers best-path depreference (LLGR-stale). Generic composable commands
`rib attach-community` and `rib delete-with-community` allow plugins to manipulate route
communities without RIB code changes. The RIB stores per-peer GR state
(`StaleAt`, `RestartTime`, `ExpiresAt`) and starts a safety-net expiry timer
(restart-time + 5s margin) that auto-purges stale routes if bgp-gr never sends
`purge-stale` or `release-routes`.

<!-- source: internal/component/bgp/plugins/rib/rib_commands.go — markStaleCommand, registerCommunityCommands -->

**Show output:** `show bgp rib received` includes `"stale": true` and `"stale-level": N` on stale routes (omitted when level is 0).
`show bgp rib status` includes aggregate `stale-routes` count and per-peer GR state with
`stale-at` and `expires-at` absolute times.

---

## Update → RIB Flow

The RIB decomposes incoming Updates, interns components, and can reconstruct pooled Updates.

### Decomposition and Interning

```
┌─────────────────────────────────────────────────────────────────────────┐
│  RECEIVE                                                                 │
│                                                                         │
│  conn.Read(buf) ──► Parse UPDATE ──► Temporary access to parts         │
│                          │                                              │
│                    [withdrawn][attrs][nlri][mp_reach][mp_unreach]       │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  INTERN INTO GLOBAL POOL                                                 │
│                                                                         │
│  attrs bytes ──► pool.Attrs.Intern() ──► attrHandle                    │
│                                              │                          │
│  (Same attrs from any peer = same handle)    │                          │
│                                              ▼                          │
│  nlri bytes ──► RIB.Insert(attrHandle, nlri)                           │
│                       │                                                 │
│                 ┌─────┴─────┐                                          │
│                 │           │                                          │
│           IPv4: direct   IPv6+: pool.NLRIs.Intern()                    │
└─────────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────────────┐
│  STORE IN PER-PEER RIB                                                  │
│                                                                         │
│  PeerRIB[family].entries[attrHandle] ──► NLRISet                       │
│  PeerRIB[family].prefixIndex[nlri] ──► attrHandle                      │
│                                                                         │
│  (Each peer has own RIB, but handles point to shared pool)              │
└─────────────────────────────────────────────────────────────────────────┘
```

### Reconstructing Update from Pool

When we need an Update interface from stored data, PooledUpdate is constructed
from RIB entries. See "PooledUpdate (RIB Mode)" section for struct definition
and "PooledUpdate Methods" section for accessor implementations.

**Note:** Family information is stored in NLRISet, not in PooledUpdate directly.

### Processing Flow Summary

```go
func (peer *Peer) processUpdate(buf []byte) error {
    // 1. Parse into temporary accessors
    wdLen := binary.BigEndian.Uint16(buf[0:2])
    // ... derive all parts
    attrBytes := buf[attrStart:attrEnd]

    // 2. Process withdrawals (no pool interaction)
    for each withdrawn nlri {
        peer.rib.Remove(family, nlri)
    }

    // 3. Store announcements - RIB manages pool refs internally
    for each announced nlri {
        peer.rib.Insert(family, attrBytes, nlri)  // RIB calls Intern()
    }

    // 4. Original buffer can be reused immediately
    return nil
}
```

**Key:** Caller never touches pool directly for RIB operations. RIB.Insert() takes raw bytes, handles all refcounting.

### Lifecycle Management

RIB manages pool handle lifecycle. See "Reverse Index" section above for full Insert/Remove implementations.

**Key invariant:** Each unique `attrHandle` in `r.entries` has exactly ONE pool reference, regardless of how many NLRIs share those attributes.

### Reference Counting Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│  INSERT (first NLRI for new attr set)                                    │
│                                                                         │
│  RIB.Insert(attrBytes, nlri)                                            │
│        │                                                                │
│        ├──► pool.Intern(attrBytes) ──► handle (refcount = 1)           │
│        │                                                                │
│        ├──► entries[handle] not found                                   │
│        │        └──► keep the ref, create new NLRISet                  │
│        │                                                                │
│        └──► set.Add(nlri), prefixIndex[nlri] = handle                  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  INSERT (additional NLRI for existing attr set)                          │
│                                                                         │
│  RIB.Insert(attrBytes, nlri)                                            │
│        │                                                                │
│        ├──► pool.Intern(attrBytes) ──► same handle (refcount = 2)      │
│        │                                                                │
│        ├──► entries[handle] found                                       │
│        │        └──► pool.Release(handle) ──► refcount = 1             │
│        │                                                                │
│        └──► set.Add(nlri), prefixIndex[nlri] = handle                  │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────┐
│  REMOVE (last NLRI for attr set)                                         │
│                                                                         │
│  RIB.Remove(nlri)                                                       │
│        │                                                                │
│        ├──► handle = prefixIndex[nlri]                                  │
│        ├──► set.Remove(nlri)                                            │
│        │                                                                │
│        └──► set.Len() == 0                                              │
│                  ├──► set.Release() (NLRI handles if pooled)            │
│                  ├──► delete(entries, handle)                           │
│                  └──► pool.Release(handle) ──► refcount = 0 ──► freed  │
└─────────────────────────────────────────────────────────────────────────┘
```

**RIB owns the lifecycle.** Callers pass bytes, never handles. Pool refcount matches RIB entries exactly.
<!-- source: internal/component/bgp/attrpool/pool.go -- Intern, Release refcounting -->

### Reading the RIB for an Answer

A show command walks the RIB and writes what it finds to a socket. Two locks are
in play and neither one covers what a reader needs for the whole walk.

| Lock | What it protects |
|------|------------------|
| `RIBManager.peerMu` | the peer-keyed maps, and nothing inside a `PeerRIB` |
| `PeerRIB.mu` | that peer's routes, including the pool handles each entry holds |

`PeerRIB.Lookup` returns a COPY of the entry and gives its lock back, so the
handles in that copy are safe to HOLD and unsafe to READ once the lock is gone:
`FamilyRIB.Remove` releases them under `PeerRIB.mu` alone, a released slot goes
on its shard free list, and a release build re-interns that slot with other
bytes. A reader that dereferences one afterwards reads another route's
attributes rather than an error.

**`PeerRIB.LookupRetained` is the read for a walk that must not hold a lock.** It
takes a reference to the entry's handles inside the read lock, so the caller
dereferences them with no lock held, and the caller MUST call `Release` on the
entry exactly once.

The best-path walk is built on it. `bestPipeline` reads the peer maps under
`peerMu` to build its item list, gives that lock back, and then hands its rows
over one at a time; `bestSource.Next` retains each item's entry when the item is
pulled and gives the reference back on the next pull. So no socket write happens
while `peerMu` is held, which matters because `handleReceivedStructured` takes
`peerMu.Lock` for every UPDATE it processes: a walk that held the read side
across its writes would stall UPDATE processing for as long as an operator's
terminal took to drain a full table.

**The reference covers the item last handed over and nothing else.** A pipeline
stage that HOLDS items past the pull that produced them is outside that cover
and takes a reference of its own. `last N` is that stage: it drains the whole
walk into a ring of N and emits from the ring afterwards, so without one, every
row `show bgp rib best last N` produced but the final one would be encoded from
a released slot. `applyBestStages` is where the ring is given its reference and
where the release it owes is returned to the caller.

`show bgp rib` holds `peerMu.RLock` only to build its sources
(`newShowSource`). Each source then materializes one peer at a time: the
adj-rib-in half retains every entry it buffers inside the PeerRIB walk, and the
adj-rib-out half takes `peerMu` again for each peer. No lock is held while a row
is serialized or yielded.

**A received row says whether its path won.** After a peer's walk has given the
PeerRIB lock back, `markBestFacts` looks each buffered CIDR row up in the
`bestPrev` store under that prefix's shard read lock, and the row carries
`best`: `true` when the recorded winner names this peer and, under ADD-PATH,
this path identifier, `false` otherwise or when no winner is recorded. The
lookup MUST run with no other lock held, because an election takes the shard
lock before `peerMu` and the PeerRIB lock; a lookup made inside the walk, or
under `peerMu`, would wait behind a queued writer that is itself waiting on the
election. A protocol RIB's rows answer `false`, because its routes never enter
the election. A non-CIDR row carries no `best`: its winner is keyed by wire
NLRI, which the row no longer holds, and an absent member says the RIB could
not tell, where `false` would claim it lost. Sent rows carry none either.

**A received row says how long ago its route changed.** After the same walk,
`markReadAt` stamps the buffered rows with one reading of the route clock, and
`writeRouteAge` writes `age`: a JSON number, the whole seconds from the route's
`ChangedAt` to that reading. The reading is taken after the walk, so it is
never earlier than a time one of the walked entries holds. A row whose
route holds no reading carries no `age`, and sent rows carry none: an
Adj-RIB-Out copy keeps no time. The member is additive, so a reader that does
not know it is unaffected.

**A best row carries no `age`**, in the walk `show bgp rib best` answers with
and under its `json` terminal alike. The RIB records when the winning route's
attributes last changed. It does not record when that route became the best
one: a route unchanged for a week becomes best the second a better one is
withdrawn. One `age` on a best row would be read as the time the path has been
best, which the RIB cannot state, so `bestSource.Next` takes no reading of the
route clock and the row has no member. The age of the winning route is on its
received row.
<!-- source: internal/component/bgp/plugins/rib/rib_route_age.go -- markReadAt, writeRouteAge, rowKeyAge -->
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- LookupRetained -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline_best.go -- bestPipeline, bestPathRows, bestSource, applyBestStages -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline.go -- showPipeline, newShowSource, inboundSource, lastFilter -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline_bestfact.go -- markBestFacts, bestFactOf, writeBestFact -->

Invalid arguments to `show bgp rib` and `show bgp rib best` produce an error
verdict before the route walk starts. The refusal preserves the parser's reason
and names a valid count command to start from. A JSON object containing an
`error` member is not a successful route answer. Terminal failures, such as an
oversized graph, use the same refusal path.
<!-- source: internal/component/bgp/plugins/rib/rib_topology.go -- refusePipelineArgs, commandAnswer -->

### The covering read

`show bgp rib received covering <address>` answers every received route whose
prefix contains one IPv4 or IPv6 address: within each peer the most specific
prefix first, across the peer's families (two families holding a prefix of one
length answer it in family order), with every ADD-PATH path of each prefix. It is
the read a looking glass needs when a user types a host address, and it must
not cost a full-table scan, because that endpoint is public.

The read follows the trie, never the table. `store.Store.Covering` walks the
BART nodes on the path from the root to the address (`bart.Table.Supernets`),
so its cost follows the address length, at most one entry per prefix length,
whatever the table holds. `FamilyRIB.iterateCovering` runs it over `direct`, or
over `multi` with every path of each `pathSet`. `PeerRIB.IterateCovering`
runs it, under the PeerRIB read lock, when the peer holds one CIDR family of
the address's IP version. When it holds several (unicast with multicast or
labeled unicast), each family's walk would order only its own prefixes, so
`PeerRIB.IterateCovering` asks each prefix length from the longest down of
every family in family order (`FamilyRIB.iterateExact`): a length names the
one prefix of it that contains the address, so the read stays bounded by the
address length. A non-CIDR family holds no prefix to
contain an address and contributes nothing. The `maprib` build has no trie, so
its `Store.Covering` scans the map and sorts the matches; the cost bound belongs
to the default backend.

`covering` is a source selector, like `peer`, and not a filter stage:
`parsePipelineArgs` reads the address once into a `receivedWalk`, and the
received sources (`inboundSource`, and `protocolInboundSource` for `show bgp rib
protocol`) read each peer through that walk. A stage would see every row the
source yields, which is the scan the read exists to avoid. The walk changes
which entries a source reads and nothing else, so the locking and the pool
references of the section above apply unchanged, and the filters and
terminals after it compose as they do on the whole table.

The value is a bare address. A value with a mask, a zone, or anything that is
not an address is refused at parse time with the value and a form that works;
`prefix` keeps its text meaning, partial form included. The scopes that hold
sent routes refuse `covering`, naming the received form, because the
Adj-RIB-Out is held in maps with no trie to walk. `show bgp rib best` refuses it
too: its source collects prefixes with a whole-table walk of every peer.
<!-- source: internal/core/rib/store/store_bart.go -- Store.Covering -->
<!-- source: internal/core/rib/store/store_map.go -- Store.Covering -->
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go -- iterateCovering, familyHoldsAddress -->
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- IterateCovering, iterateExact over several families -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline_covering.go -- receivedWalk, coveringRead, parseCoveringAddress, refuseCoveringScope, coveringOnBestRefusal -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline.go -- parsePipelineArgs, showPipeline, inboundSource -->

### The exact read

`show bgp rib received exact <prefix>` answers the received routes whose prefix
is that prefix: within each peer, family by family, every ADD-PATH path of it.
It is the read a looking glass needs when a user types a prefix with its mask.
The `prefix` filter cannot answer that question. It is a text filter
(`prefixFilter`), so it keeps every row whose prefix text starts with its value:
`prefix 192.0.0.0/2` also lists `192.0.0.0/24`, `prefix 2000::/3` also lists
`2000::/32`, and it reads the whole table to find them. `prefix` keeps that
meaning, because operators use its partial form (`prefix 10.0`).

The read is one trie lookup per family, never a scan. `FamilyRIB.iterateExact`
reads the entry keyed by the prefix (`store.Store.Lookup`) from `direct`, or
from `multi` with every path of its `pathSet`, and `PeerRIB.IterateExact` runs
it over the peer's CIDR families of the prefix's IP version, in family order,
under the PeerRIB read lock; `PeerRIB.appendAddressFamilies` picks and orders
those families for both the covering and the exact read. A non-CIDR family, and
a family of the other IP version, contribute nothing.

`receivedWalk` is a sealed interface of three variants, each holding only its
own payload, so no walk can name both an address and a prefix:
`wholeTableRead` reads the whole table, `coveringRead` the routes containing an
address, `exactRead` the routes of one prefix. `parsePipelineArgs` answers
`wholeTableRead` when neither keyword is given, and no walk at all beside a
refusal. It reads the value once. It refuses a bare
address (naming the `covering` form), a value that is not a prefix, a zone, and
a prefix with host bits set (naming the masked form, `10.0.0.1/8` gives
`10.0.0.0/8`), because the store keys its entries by the masked prefix and
masking silently would answer a prefix the operator did not write. One command
takes `covering` or `exact`, never both (`walkConflictRefusal`). The scopes
follow `covering`: `received` and `show bgp rib protocol` answer, while the
scopes holding sent routes and `show bgp rib best` refuse with the received
form, since neither the sent sources nor the best-path source take a walk.
<!-- source: internal/component/bgp/plugins/rib/storage/familyrib.go -- iterateExact -->
<!-- source: internal/component/bgp/plugins/rib/storage/peerrib.go -- IterateExact, appendAddressFamilies -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline_covering.go -- receivedWalk, wholeTableRead, coveringRead, exactRead, parseExactPrefix, exactValueRefusal, walkConflictRefusal, refuseExactScope, exactOnBestRefusal -->
<!-- source: internal/component/bgp/plugins/rib/rib_pipeline.go -- prefixFilter, parsePipelineArgs, showPipeline -->

### Example: IPv4 Unicast

```
Received UPDATE:
  Attrs: AS_PATH=[65001], NEXT_HOP=10.0.0.1
  NLRI: [192.168.1.0/24, 192.168.2.0/24, 192.168.3.0/24]

RIB stores (IPv4 unicast, inline wire bytes):
  families[{1,1}].entries[attrHandle] = &DirectNLRISet{
      data:  [0x18 0xC0 0xA8 0x01][0x18 0xC0 0xA8 0x02][0x18 0xC0 0xA8 0x03]
      count: 3
  }
```

### Example: VPNv4

```
Received UPDATE:
  Attrs: AS_PATH=[65001], NEXT_HOP=10.0.0.1, RT=65001:100
  MP_REACH_NLRI: VPNv4 [RD:label:prefix, RD:label:prefix]

RIB stores (VPNv4, pooled handles):
  families[{1,128}].entries[attrHandle] = &PooledNLRISet{
      handles: [handle1, handle2]
  }
```

---

## Mode Selection

### Global Mode Decision

```go
// Mode is determined globally at startup and when peers change
type UpdateMode int

const (
    ModeWire   UpdateMode = iota  // WireUpdate - no pooling
    ModePooled                     // PooledUpdate - with pooling
)

func DetermineMode(peers []PeerConfig) UpdateMode {
    for _, p := range peers {
        if p.AdjRIBEnabled() {
            slog.Info("RIB enabled on peer, using pooled mode", "peer", p.Name)
            return ModePooled
        }
    }
    slog.Info("No peer has RIB enabled, using wire mode (API-controlled)")
    return ModeWire
}
```

### Behavior

| Condition | Mode | Log Message |
|-----------|------|-------------|
| No peer has Adj-RIB | `ModeWire` | "No peer has RIB enabled, using wire mode (API-controlled)" |
| Any peer has Adj-RIB | `ModePooled` | "RIB enabled on peer, using pooled mode" |

### Runtime Considerations

- Mode is set at startup
- If peer config changes (add peer with RIB), mode switches to `ModePooled`
- Once in `ModePooled`, stays there (no downgrade to `ModeWire`)

## Migration Phases

### Phase 1: Pool Core ✅

**File:** `internal/component/bgp/attrpool/pool.go`
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool, Intern, Get, Release -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle layout -->

> **Note:** The implementation uses a hybrid handle layout. See `docs/architecture/pool-architecture.md` for current design.

```go
package pool

type Handle uint32

// Handle layout: bufferBit(1) | poolIdx(5) | flags(2) | slot(24)
const InvalidHandle Handle = 0xFFFFFFFF  // poolIdx=31 (reserved)

// Core operations (all return errors for validation)
func (p *Pool) Intern(data []byte) Handle
func (p *Pool) InternWithError(data []byte) (Handle, error)
func (p *Pool) Get(h Handle) ([]byte, error)
func (p *Pool) AddRef(h Handle) error
func (p *Pool) Release(h Handle) error
func (p *Pool) Length(h Handle) (int, error)

// Normalized access (by slot index)
func (p *Pool) GetBySlot(slotIdx uint32) ([]byte, error)
func (p *Pool) ReleaseBySlot(slotIdx uint32) error
```

**Tests:**
- Intern returns same handle for identical data
- Get returns correct bytes
- AddRef/Release reference counting
- Compaction preserves data integrity
- Fuzz tests for handle encoding

### Phase 2: Global Pool

**File:** `internal/component/bgp/attrpool/global.go`

```go
// Global pool shared across all peers
// Stores attribute bytes and large NLRIs
// Deduplication: same bytes = same handle
var Pool = NewPool(PoolConfig{
    InitialBufferSize: 1 << 20,  // 1MB
    ExpectedEntries:   100000,
})

// Convenience aliases (same pool, for code clarity)
var Attrs = Pool
var NLRIs = Pool
```

Single pool. Shared globally. Deduplicates identical bytes across all peers.

### Phase 3: Update Types

**File:** `internal/component/bgp/message/update.go`

```go
// Update interface - same API for both modes
type Update interface {
    Attrs() Attributes
    NLRI() []byte
    Withdrawn() []byte
    MPReach() MPReach
    MPUnreach() MPUnreach
    SourceCtxID() ContextID
    Release()
}

// WireUpdate - API mode (no RIB)
// Takes ownership of payload buffer (allocated per read)
type WireUpdate struct {
    payload     []byte
    sourceCtxID ContextID
}

func NewWireUpdate(payload []byte, ctxID ContextID) *WireUpdate {
    // Takes ownership - caller allocated buffer, we own it now
    // No copy needed - buffer was allocated for this message
    return &WireUpdate{payload: payload, sourceCtxID: ctxID}
}

// PooledUpdate - RIB mode (reconstructed from RIB entries)
// Family info stored in nlriSet, not here
type PooledUpdate struct {
    attrHandle  pool.Handle   // Interned attrs
    nlriSet     NLRISet       // Contains family internally
    sourceCtxID ContextID
}

// Note: No NewPooledUpdate constructor - RIB constructs these
// from stored entries when iterating routes
```

### Phase 4: RIB Storage Model

**File:** `internal/component/bgp/rib/rib.go`

```go
// NLRISet interface with two implementations
type NLRISet interface {
    Add(nlri []byte)
    Remove(nlri []byte) bool
    Contains(nlri []byte) bool
    Iterate(fn func(nlri []byte) bool)
    Len() int
    Release()
}

// DirectNLRISet - IPv4 (wire bytes, no pool)
type DirectNLRISet struct {
    data    []byte
    count   int
    family  FamilyKey  // AFI + SAFI
    addPath bool
}

// PooledNLRISet - IPv6+, VPN, EVPN (pool handles)
type PooledNLRISet struct {
    handles []pool.Handle
    family  FamilyKey  // AFI + SAFI
    addPath bool
}

// FamilyRIB stores routes for one AFI/SAFI
type FamilyRIB struct {
    afi      uint16
    safi     uint8
    entries  map[pool.Handle]NLRISet  // attrHandle → NLRIs
}
```

### Phase 5: Compaction (Future/Optional)

**Status:** Not needed for MVP. Pool grows without reclaiming. Add when memory pressure matters.

**File:** `internal/component/bgp/attrpool/scheduler.go`
<!-- source: internal/component/bgp/attrpool/scheduler.go -- Scheduler, Run -->

#### The Problem: Fragmentation

```
Pool buffer over time with Release() calls:

Initial:  [entry1][entry2][entry3][entry4][entry5][...........]
                                                    ↑ free space

After releases:
          [entry1][freed][entry3][freed][entry5][...........]
                    ↑            ↑
                  holes - wasted space, cannot be reused
                  (entries are variable size)
```

Without compaction, pool grows indefinitely even if most entries are released.

#### The Solution: Double-Buffer Compaction

```
Buffer A (current):  [e1][--][e3][--][e5][--][e7]  ← fragmented
Buffer B (target):   [e1][e3][e5][e7][............]  ← compact

1. Copy live entries from A → B (incrementally)
2. Update slot offsets to point to B
3. Flip: B becomes current, A becomes target
4. A is now empty, ready for next compaction cycle
```

#### Handle Design Supports This

> **Note:** Current implementation uses hybrid layout. See `docs/architecture/pool-architecture.md`.

```
Handle layout: bufferBit(1) | poolIdx(5) | flags(2) | slot(24)

Handle 0x80000005 → bufferBit=1, poolIdx=0, flags=0, slot=5
Handle 0x00000005 → bufferBit=0, poolIdx=0, flags=0, slot=5
```

- Handle encodes buffer bit + pool index + flags + slot
- Slot stores offset in BOTH buffers
- During compaction, slot is valid in both buffers
- After compaction, flip the buffer bit for new allocations

#### Incremental Compaction

```go
type CompactionScheduler struct {
    pool          *Pool
    state         CompactionState
    cursor        uint32          // Current slot being migrated
    batchSize     int             // Slots per iteration
    idleThreshold time.Duration   // Wait for idle before compacting
}

type CompactionState int

const (
    Idle       CompactionState = iota
    Compacting
    Flipping
)

func (s *CompactionScheduler) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-time.After(s.idleThreshold):
            if s.pool.ShouldCompact() {
                s.compactBatch()
            }
        }
    }
}

func (s *CompactionScheduler) compactBatch() {
    // Move batchSize slots from old buffer to new
    // Pause if BGP activity detected
    // Resume on next idle period
}
```

#### When to Compact

| Trigger | Threshold |
|---------|-----------|
| Fragmentation ratio | > 30% of buffer is freed slots |
| Memory pressure | Pool size > configured limit |
| Idle time | No BGP activity for N seconds |

#### Activity-Based Pausing

```go
func (s *CompactionScheduler) compactBatch() {
    for i := 0; i < s.batchSize; i++ {
        if s.pool.HasRecentActivity() {
            // BGP traffic - pause compaction
            return
        }
        s.migrateSlot(s.cursor)
        s.cursor++
    }
}
```

Compaction yields to BGP processing - never blocks message handling.

#### MVP: Skip Compaction

For initial implementation:
- Pool grows without bound
- No compaction scheduler
- Acceptable for short-lived sessions or small route tables
- Add compaction when memory profiling shows need

```go
type CompactionScheduler struct {
    pool *Pool
    // ... per POOL_ARCHITECTURE.md
}

func (s *CompactionScheduler) Run(ctx context.Context)
```

**Integration:** Start scheduler in reactor initialization (when implemented).

## API Changes

### New: Update Interface

| Type | Mode | Storage |
|------|------|---------|
| `WireUpdate` | API-controlled (no RIB) | Owned `[]byte` copy |
| `PooledUpdate` | RIB-enabled | `pool.Handle` |

Both implement `Update` interface with same methods.

### Breaking Changes

| Before | After |
|--------|-------|
| Direct `[]byte` storage | `Update` interface |
| N/A | `Update.Release()` MUST be called (no-op for WireUpdate) |

### Unchanged (Zero-Copy Preserved)

| Component | Why Unchanged |
|-----------|---------------|
| `AttributesWire` | Keeps zero-copy reference (to wire or pooled buffer) |
| `NewAttributesWire()` | No pool interaction |
| `AttributesWire.Packed()` | Returns reference, not pool data |

### New Requirements

1. **Mode selection:** Use `WireUpdate` when Adj-RIB disabled, `PooledUpdate` when enabled
2. **Lifecycle management:** Always call `Release()` (safe no-op for WireUpdate)
3. **No modification:** Bytes from `Update.NLRI()` etc. MUST NOT be modified
4. **Thread safety:** Pool operations are thread-safe

## Memory Analysis

### Without Pool (Current)

| Routes | Memory per Route | Total |
|--------|------------------|-------|
| 1M | ~200 bytes (attrs+nlri copy) | ~200MB |

### With Pool (Target)

| Routes | Memory per Route | Total |
|--------|------------------|-------|
| 1M | 8 bytes (handle+ctxID) | 8MB + pooled data |

**Deduplication:** Identical UPDATE payloads share storage.
For route reflectors forwarding same UPDATE to many peers → significant savings.

## Checklist

### Phase 1: Pool Core ✅
- [x] Implement `internal/component/bgp/attrpool/pool.go`
- [x] Implement `internal/component/bgp/attrpool/handle.go`
- [x] Implement `pool.Lookup()` for read-only handle lookup
- [x] Tests for Intern/Get/Lookup/AddRef/Release
- [x] Tests for deduplication
- [x] Tests for concurrent access

### Phase 2: Global Pool
- [ ] Create `internal/component/bgp/attrpool/global.go` with single global pool
- [ ] Add `Attrs` and `NLRIs` aliases pointing to same pool
- [ ] Remove old separate pools if they exist

### Phase 3: Update Types
- [ ] Define `Attributes` interface
- [ ] Make `AttributesWire` implement `Attributes`
- [ ] Define `MPReach` and `MPUnreach` interfaces
- [ ] Implement `mpReachWire` and `mpUnreachWire` structs
- [ ] Define `Update` interface
- [ ] Implement `WireUpdate` (API mode)
- [ ] Implement `PooledUpdate` (RIB mode)
- [ ] Implement derived accessors: `Withdrawn()`, `Attrs()`, `NLRI()`
- [ ] Implement MP-BGP accessors: `MPReach()`, `MPUnreach()`
- [ ] Implement mode selection (`DetermineMode()`)
- [ ] Add logging for mode selection
- [ ] Tests for all types
- [ ] Update message processing to use Update interface

### Phase 4: RIB Storage Model
- [ ] Define `NLRISet` interface (with `family` field)
- [ ] Implement `DirectNLRISet` (IPv4: wire bytes)
- [ ] Implement `PooledNLRISet` (IPv6+: pool handles, uses `Lookup()`)
- [ ] Implement `NewNLRISet()` factory with ADD-PATH support
- [ ] Implement `FamilyKey`, `PeerRIB`, `FamilyRIB` types
- [ ] Implement reverse index (prefix → attrHandle)
- [ ] Implement RIB-bound refcounting (RIB.Insert takes bytes, not handles)
- [ ] Implement `shouldPoolNLRI()` for family-specific storage
- [ ] Implement implicit withdraw (same prefix, new attrs)
- [ ] Implement ADD-PATH aware NLRI parsing
- [ ] Implement Update → RIB decomposition flow
- [ ] Implement PooledUpdate reconstruction from RIB data
- [ ] Tests for all address families
- [ ] Tests for ADD-PATH scenarios
- [ ] Tests for refcount invariant (1 ref per entries key)

### Phase 5: Compaction (Future)
- [ ] Implement CompactionScheduler
- [ ] Integrate with reactor
- [ ] Test incremental compaction
- [ ] Test activity-based pausing

**Note:** AttributesWire is NOT modified - it remains a zero-copy view.

## Dependencies

- `POOL_ARCHITECTURE.md` - Design specification
- `spec-attributes-wire.md` - AttributesWire current design

## Risks

| Risk | Mitigation |
|------|------------|
| Forgetting Release() | Static analysis, runtime leak detection |
| Pool contention | Sharding if needed |
| Compaction latency | Incremental, pause on activity |
| Lower dedup ratio | Full UPDATE less likely to match than attrs alone |

## Deduplication Granularity

### Three Levels of Deduplication

| Level | What's Deduplicated | Storage/Route | Dedup Ratio | Status |
|-------|---------------------|---------------|-------------|--------|
| **1. UPDATE blob** | Entire UPDATE message | 8 bytes | Low (exact match only) | ❌ Not used |
| **2. Attribute blob** | All attrs as one blob | handle + NLRI | Medium (same attr set shared) | ✅ **Current** |
| **3. Per-attribute** | Each attr type separately | ~10 handles | High (partial sharing) | 📋 Planned |

### Current Implementation (Level 2)

```
Route A: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=0
Route B: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=50
         ↓
pool.Attributes.Intern(attrBytes)  → TWO different blobs (MED differs)
```

**Limitation:** Routes sharing ORIGIN, AS_PATH, LOCAL_PREF but differing in MED get NO sharing.

### Target Implementation (Level 3 - Phase 6)

```
Route A: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=0
Route B: ORIGIN=IGP, AS_PATH=[65001], LP=100, MED=50
         ↓
RouteEntry {
    StaleLevel: 0
    Bundle:     BundlePool.Intern(Bundle{Origin, NextHop, LP, MED, ...}) → SHARED if identical
    ASPath:     pool.ASPath.Intern([65001])                              → separate handle
}
```

`RouteEntry` is 64 bytes on the current 64-bit layout: it holds stale state,
received Partial facts, the route's change time, message and source-policy
provenance, an attribute fingerprint and length, the bundle and AS_PATH handles,
and a local source-lease reference. The Partial byte and the four-byte change
time occupy existing padding before the message ID. With the change time the
64-byte figure is expected, to be confirmed by `TestRouteEntrySize`.
The 12 non-AS_PATH attribute handles live in a `Bundle` struct (48 bytes) stored
in `BundlePool`. Routes with identical attribute values share that bundle even
when their received Partial facts differ.

`Bundle` is a comparable Go struct (12 Handle fields), used directly as a
`map[Bundle]uint32` key for zero-serialization dedup. BundlePool manages
refcounting; when a bundle's refcount reaches zero, all 12 inner attribute
handles are cascade-released.

<!-- source: internal/component/bgp/plugins/rib/storage/bundle.go — Bundle, BundlePool -->

**Benefit:** 1M routes with same ORIGIN/LP but different MED → ~3 ORIGIN refs + ~100 LP refs vs 1M blobs.

### Memory Impact

| Scenario | Level 2 (blob) | Level 3 (per-attr) |
|----------|----------------|-------------------|
| 1M routes, same ORIGIN/LP/AS_PATH | Good (if identical) | Same |
| 1M routes, same except MED | 1M × ~50B = 50MB | ~1MB (shared + MED pool) |
| Route reflector (identical attrs) | Excellent | Same |

### Phase 6 Implementation Plan

See `plan/spec-plugin-rib-pool-storage.md` § "Phase 6: Per-Attribute Deduplication" for:
- Per-attribute pools (Origin, ASPath, LocalPref, MED, etc.)
- RouteEntry struct with per-attr handles
- Attribute parser using existing `AttrIterator`
- Wire reconstruction for route resend

**Dependencies:**
- `internal/core/bgp/attribute/iterator.go` - `AttrIterator` (exists, reuse)
- `internal/component/bgp/attrpool/pool.go` - Pool infrastructure (exists, extend)
<!-- source: internal/core/bgp/attribute/ -- AttrIterator -->
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool with Intern/Get/Release -->

---

## Current Decision

**Level 2 (blob dedup)** is implemented. Good dedup when attribute sets are identical.

**Level 3 (per-attr dedup)** planned for Phase 6. Better dedup when routes share
most attributes but differ in one (e.g., MED varies, rest identical).

---

**Created:** 2026-01-01
**Updated:** 2026-01-03 - Fixed design: decomposed attrs+NLRIs, RIB-bound refcounting, pool.Lookup()

### Admitted scoped origins

The real RIB registers the generic `OnOriginIntake` callback on both SDK
transports. Its shared contribution table retains engine-authorized source,
instance, native key, original path-identifier presence, configured Peer owner
and origin receipt. Origin build attributes and engine encoding context remain
opaque. A contribution competes only at its admitted Peer scope, alongside the
same common learned BGP winner. No recipient can nominate a second learned path.

Cross-source comparison uses the Loc-RIB's shared distance/metric preference
function and stable admission order. Explicit BGP origins use the configured
static class; admitted redistribution keeps its original registered protocol and
accepted override. A zero override is present and meaningful. The existing
`rib` read root supplies the declaration to internal and forked RIB config
transactions. Apply and rollback publish new scoped revisions and join output.
System-RIB visibility alone never admits an origin or applies FIB import policy.

Dispatch, refresh, completion and CurrentSelection use the same immutable scoped
winner. Recipient export acts after election: exclusion retires prior output
without trying a loser. Removing a losing origin leaves another winner intact;
a retained loser becomes useful only after genuine reselection. Receipts
explicitly distinguish retained selection/export suppression with Announced
zero. Real failures retain their cause. Cancellation removes only its own issued
generation and cancels its accepted sender calls. Completion requires their join
and current output confirmation; a bounded failure reports unresolved obligations
while retaining the uncertain frontier. Unrelated destinations are not canceled. Storage keeps current memberships and
selected state plus actual acknowledged/in-flight obligations, without withdrawn
history. Physical reconnect replays the configured owner's current selection.
<!-- source: internal/component/bgp/plugins/rib/rib_origins.go -- originIntake, reelectOriginLocked -->
<!-- source: internal/component/bgp/plugins/rib/rib_origin_preferences.go -- installOriginPreferences -->
<!-- source: internal/core/rib/locrib/preference.go -- PreferenceFor, Preference.Before -->

Origin settlement separates a captured accepted sender call from current desired
replacement. The existing worker closes that call's completion only after its
send/receipt operation returns. Failed current revisions retain their exact
failure and uncertain frontier, so intake reports an unresolved cause instead
of waiting indefinitely for retry. Candidate withdrawal preserves the passed
cleanup deadline and does not acknowledge completion while output remains owed.
Failed announce compensation has at most the existing cancellation-join budget;
preference restoration uses the original apply context and reports broken config
when the desired old declaration cannot be confirmed on output. SDK shutdown
therefore does not depend on a replacement retry completing before Run returns.
The engine issuer still owns final physical write revocation and joining.
<!-- source: internal/component/bgp/plugins/rib/rib_origins.go -- joinOriginWrites, originBarrier -->

<!-- source: internal/component/bgp/plugins/adj_rib_in/received_peer.go -- receivedPeer, receivedPeerOf, receivedBefore -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_validation_config.go -- rememberValidationPeer, validationPeerPresentLocked -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/received_operator.go -- applyOperatorDecisionLocked -->


### Reflector original sessions and retained membership

The reflector keeps peer actors under the original nonzero Peer owner and
physical connection. The address is an observation and an operator field.
Capabilities, the replay lifecycle and the destination owner/session snapshot
belong to that actor; an old DOWN or OPEN cannot mutate its successor.
The existing withdrawal inventory keeps the original connection, ingress,
receipt and effective NO_LLGR flag with each route. A retained Session receipt
does not make every route from that Session replayable: the exact route must
still belong to this inventory and its retained family.

The reflector acquires its own source lease while handling live received
input, before ordered DOWN. Retention keeps that same lease. It never borrows
the receive store's lease or acquires a new lease for a source already down.
Stored-page relays, validation relays and cached forwarding use captured
destination owner/session values on the existing SDK requests.

The cleanup path validates the complete original removal predicate and
seals matching inventory rows without waiting for I/O. It removes them only
after already admitted cached, native and deferred output has joined. The registered
event-settlement callback joins admitted and superseded peer-UP replays,
validation batches, live forwards and departures under its finite application
deadline, then releases leases whose last retained route was removed. Failed
settlement keeps its obligations pending. Final source and SENT purge follows
successful applied cleanup; a replay-only join cannot confirm that cleanup.
Native TEXT OPEN and state use the original physical metadata, and malformed
or duplicate reserved fields refuse the whole event. Departures keep the
destination owner/session captured at original DOWN, including same-address
siblings. Live Adj-RIB-In page adoption first checks current receive eligibility
and acquires this role process's own live lease under the actor lock. A DOWN
actor cannot adopt an escaped row, even while another retained route keeps
its Session-wide receipt valid. Async output failure remains an owed error
through settlement; an exited local waiter never proves remote cancellation.
The normal-DOWN worker releases an ended source's last lease and prunes its
actor after actual departure completion, without waiting for another barrier.

For an Adj-RIB-In row, a cold read also names the original source
UPDATE frontier this actor has applied. The receive store answers pending,
not rejected, while that exact physical projection is behind. Before admitting
output, the reflector rechecks its local inventory revision under the same
lock used by received withdrawals. A racing withdrawal repeats only the current
eligibility read within the replay deadline; it never replays an admitted write.
Unseen current routes remain admissible after catch-up, while a withdrawn route
cannot borrow another route's Session receipt.
FlowSpec asks its selecting RIB through the same context-bound lookup and
requires the copied path's original message, receipt and policy epoch to match.
A pending selecting projection does not mean that the rule is absent.
<!-- source: internal/component/bgp/plugins/rr/stored_admission.go -- admitStoredRoute, waitStoredEligibility -->
<!-- source: internal/component/bgp/plugins/rr/stored_flow_query.go -- currentStoredFlowSpec -->
<!-- source: internal/component/bgp/plugins/rr/actor.go -- peerIdentity, relayToPeer -->
<!-- source: internal/component/bgp/plugins/rr/source_cleanup.go -- cleanupSource, settleEvents -->

### Reflector optional replay reads and current replay obligations

The reflector distinguishes a refused optional Adj-RIB-In page read from a
failure admitting or writing that page. Successfully relayed earlier pages
have already joined their output. A page-read refusal is warned about and
mandatory selecting-RIB FlowSpec replay, End-of-RIB and readiness continue.
A failed current-eligibility admission withholds readiness and remains on the
original destination actor. A later finite applied settlement rereads the
current snapshot and clears that obligation only after all required phases
succeed. Normal projection churn therefore needs no process restart.

An actual relay or final marker failure keeps the existing unresolved output
obligation. Settlement refuses before retrying current reads while that
writer outcome is unknown; a read retry never certifies writer completion.
<!-- source: internal/component/bgp/plugins/rr/replay_retry.go -- retryPeerReplays -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- replayForPeer -->

Unknown cached and stored output is kept on its original physical source actor.
After physical delivery has ended, all original membership is closing, and the
local enrollment gate has joined, an exact successful source-lease Release
can discharge that actor's debt: Release invalidates its lease and waits its
admitted carriers. A live source or a useful retained row cannot use that
operation as a reconciliation shortcut. A different owner/receipt never
clears it. Final destination markers have their own target-actor obligation;
without positive exact completion evidence that obligation remains named.
<!-- source: internal/component/bgp/plugins/rr/output_obligation.go -- reconcileEndedOutput -->

A revalidation read failure remains in the reflector's existing coalescing
validation queue. The next input or finite settlement retries the current
path; the worker does not wake itself repeatedly after a failure. A successful
batch clears that queue's current error. An unknown actual output blocks its
retry until the corresponding original output obligation is settled.
<!-- source: internal/component/bgp/plugins/rr/validation.go -- runValidation, settleValidation -->

An admitted reflector replay also pins its original destination actor through
its final marker/readiness return. Ordinary source-side DOWN cleanup cannot
erase that actor while a final request is still held. On healthy completion,
the existing departure worker prunes the ended actor; an unknown final result
keeps only that captured actor's target obligation named.
<!-- source: internal/component/bgp/plugins/rr/replay_retry.go -- pinPeerReplay, finishPeerReplay -->


### Route-server current retained membership

RS acquires its own source lease before it queues an UPDATE. Physical DOWN
seals new source admission and queues its existing departure worker. That
worker joins the captured UPDATE lane, including overflow and its final cached
flush, before it changes the current inventory. Only the actual GR family
verdict and a successful retain action keep that source's rows. The rows keep
the original owner, physical connection, message, policy epoch and NO_LLGR bit.
<!-- source: internal/component/bgp/plugins/rs/source_lifetime.go -- dispatchSourceUpdate, finishSourceDown -->

A source receipt identifies a session; it does not establish current route
membership. Every stored-route relay checks the exact current path and original
provenance at output admission. Cold replay can adopt an authenticated received
row only while its original actor is live and its own lease was acquired.
Cleanup closes that actor's cold admission. Removing one path cannot be undone
by a captured old page while another retained path keeps the same receipt.
<!-- source: internal/component/bgp/plugins/rs/retained_membership.go -- currentStoredRouteLocked, adoptStoredRouteLocked -->

Native departure work keeps its original destination owner and output session.
A failed required departure stays current and is retried with those same
identities. Pending departure keys outlive the final empty actor until the
actual queue handler returns; completion removes them. The last current path
releases only its original existing source lease, after admitted output joins.
<!-- source: internal/component/bgp/plugins/rs/source_lifetime.go -- sourceDeparture, settleDeparture, releaseEmptySource, sendDepartureWork -->


### Route-server cold replay admission

A copied receive-store page is not current route authority. An RS actor records
its applied UPDATE frontier and inventory revision in its existing current
state, after applying the whole UPDATE, withdrawals included. Before adopting
an unseen live row, RS asks the family's current owner for that exact original
owner, physical connection and route generation. The receive store's announced
eligibility command answers other families; the mandatory selecting RIB's
announced FlowSpec command answers rules even when the optional store is absent.
The request carries the actor's applied frontier. A store still behind it
answers Pending without a route verdict; another source's progress cannot
satisfy it. A FlowSpec path also matches the original message, receipt and
policy epoch. RS rechecks the same
inventory revision under the membership lock before adoption and output
enrollment. A changed revision retries the current query within the existing
finite replay deadline, before any output is admitted.
An old removed row is excluded while an unseen current sibling remains useful.
<!-- source: internal/component/bgp/plugins/rs/retained_membership.go -- checkColdRoutes, admitStoredRoutesLocked -->
<!-- source: internal/component/bgp/plugins/rs/server_withdrawal.go -- processForward -->

RS marks its process-owned source lease revoked before sending Release. A
failed or unacknowledged Release leaves one current join obligation on the
original actor, never an active lease. Live admission first retries that exact
release, then acquires the original live source again. DOWN cannot acquire.
<!-- source: internal/component/bgp/plugins/rs/source_lifetime.go -- acquireSourceLocked, releaseEmptySource -->


### Route-server optional replay read failure

A failed optional receive-store page read does not suppress the selecting RIB's
mandatory FlowSpec phase or the original destination's End-of-RIB/readiness
report. Already successful page outputs have joined. RS keeps the refused
read as one obligation on that current original UP; successful current
reconciliation clears it. A relay or final-writer failure remains required
output work and withholds readiness. Unknown remote completion prevents a
write retry until its actual original output joins. Neither condition is a
process-lifetime history or permission to write a destination successor.
<!-- source: internal/component/bgp/plugins/rs/replay_read.go -- relayReplayRead, finishOptionalReplayRead -->

The registered `plugin/bgp-best-route-rs-lifetime` functional driver uses real
BGP sockets with both internal and forked RS. A and B first reach the original
destination; B's ordinary withdrawal reaches it before it closes. The successor
receives A, then End-of-RIB, and useful live C. The original source's real DOWN
must withdraw A and C on that same successor socket before local API output
passes a final useful-output fence. It checks exact AS_PATH and native NLRIs,
not mock evaluator acknowledgements. Packet grouping is immaterial; duplicate
or unrelated withdrawals and any obsolete announcement are refused.
<!-- source: internal/test/fixture/bgp_best_route_rs_lifetime.go -- bgpBestRouteRSLifetime, bgpBestRouteRSWithdrawals -->


RS subscribes to the retained-cleanup namespace and registers one combined
applied-event settlement callback. The ordinary event loop only decodes and
seals exact current admission; the off-loop callback joins workers, superseded
replays, native departures and enrolled outputs before purging membership or
releasing a lease. An unresolved original remote completion refuses the barrier
with the captured destination owner/session and the current source cleanup obligation.
A live source cannot use a failed release/reacquire to make that uncertainty
disappear. This is a failed completion result, not permission to retry the old
write or a successful acknowledgement. A healthy original scope still joins,
releases and permits useful fresh live work.
<!-- source: internal/component/bgp/plugins/rs/server.go -- RunRouteServer, dispatchText -->
<!-- source: internal/component/bgp/plugins/rs/source_lifetime.go -- releaseEmptySource, acquireSourceLocked -->
<!-- source: internal/component/bgp/plugins/rs/retained_settlement.go -- settleRetainedEvents -->


### Original source catch-up before cold membership

The existing eligibility request may carry `after-msg-id` on a query naming
the original peer owner and physical connection. The receive store checks its
current `sourceLive.processedMsgID`, advanced only after the whole typed or
JSON UPDATE has committed its effects. Withdraw-only and empty UPDATEs advance
that position too. An unrelated source's larger message ID cannot satisfy it.
Duplicate UP on the same physical connection preserves the position; a new
physical connection starts at zero. There is no retained route-removal history.
<!-- source: internal/component/bgp/plugins/adj_rib_in/source_state.go -- sourceLive, noteSourceProcessed -->

If an original source is live but its processing has not reached the requested
minimum, the whole existing eligibility answer is `pending: true` with no
answers. This is neither eligible nor absent. A caller retries only its current
query within its existing request context before adopting a cold stored route.
Missing or mismatched original source identity is an error. A provider without
an authoritative progress reader refuses a nonzero minimum.
The mandatory selecting FlowSpec provider has the same whole-request contract:
`FlowSpecQuery.AfterMsgID` and `FlowSpecAnswer.Pending`. It reads one scalar
`processedMsgID` on the existing RIB `peerMeta` physical actor, after the whole
original typed or full JSON UPDATE's mutation and selection effects. The command
answers from its actual manager instance. Every query tuple is checked before
Pending, which carries neither results nor route snapshots. Both local lookup
and framed command paths retain Pending rather than infer absence.
<!-- source: internal/component/bgp/plugins/rib/rib_source_progress.go -- sourceProgress, noteSourceProcessed -->
<!-- source: internal/core/bgp/ribevents/flowspec_remote.go -- AnswerFlowSpecFor, LookupFlowSpecRequest -->

Bound cold stored relay captures only its original live physical source's
receive publication cut, waits before dispatch or writer locks, then checks
the actual provider's applied floor under the original finite context. Live
cached forwarding and ordinary selection never wait on their own callbacks.
An ended original needs its existing process-owned retained lease; it never
waits on or reads processing authority from the same Peer's successor.
<!-- source: internal/component/bgp/reactor/relay_cold_source.go -- coldBoundRelaySource, retainedColdRelaySource -->

An unappliable original UPDATE records a failed processing gap on that same
physical RIB actor. The scalar position does not advance across it, and a later
higher valid message cannot certify current absence through the missing input.
Duplicate UP preserves the failure; only a new physical actor resets it. This
is explicit unknown application state, separate from retryable Pending lag.
 Once caught up,
the ordinary exact route/message verdicts retain their existing meaning.
<!-- source: internal/core/bgp/ribevents/gate_remote.go -- EligibilityQuery, EligibilityAnswer, AnswerEligibility -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_eligibility.go -- eligibilityProgress, eligibilityAnswerCommand -->


### Original source terminal removal and late validation

`removed` (`rpc.ReasonPeerRemoved`) is an owner-terminal state event: its original nonzero
`peer-owner` ends every current and retained connection of that logical peer,
even when no physical session is active. Adj-RIB-In joins its direct replay
before removing that owner's installed routes, pending routes, early decisions,
source-policy transaction and denial bound, current state and validation
metadata. Another owner at the same address and manual observations remain.
Connection zero never selects all physical connections on an ordinary DOWN.
<!-- source: pkg/plugin/rpc/enums.go -- ReasonPeerRemoved -->

A validation decision can precede an UPDATE while its original connection is
UP. After source cleanup, a decision is admitted only for the exact original
generation of bytes still held in the receive store, including retained GR
input. An absent source cannot recreate early decisions or policy metadata.
The store keeps no completed-owner history to make this refusal.
<!-- source: internal/component/bgp/plugins/adj_rib_in/source_state.go -- endSourceOwner, sourceDecisionAdmittedLocked -->

An address-wide stale purge captures original retained lease addresses before
taking the mutation lock, including independently owned sources at one address.
The current stable-owner population determines which candidates it removes;
the address lookup grants no send or source authority. Post-mutation capture
and final writer joins retain their existing source-operation holds. Lease
and dispatch calls run outside the peer lock.
<!-- source: internal/component/bgp/plugins/rib/rib_source_lease.go -- receiptsAtAddress, receiptsForPeer -->
<!-- source: internal/component/bgp/plugins/rib/rib_commands.go -- purgeStaleCommand -->

Learned-only export-policy reoffers read each destination's elected scoped
winner. A common learned key displaced by a local origin in that scope does
not cause the local origin to be sent again. Other learned keys remain
eligible for the existing reoffer, and unrelated scopes keep their output.
<!-- source: internal/component/bgp/plugins/rib/rib_selected_dispatch.go -- selectedDispatch.run -->


An original physical GR cleanup also keeps auxiliary SENT withdrawal debt on
its existing live source leases. Before input removal, every such row must
carry its original destination session; current peer metadata cannot fill a
missing session. One shared batch owns its encoded native commands, original
destination owner/session and sent-owner-message. The command and existing
lease worker join that same batch outside RIB/storage locks. Lease release is
refused until every original auxiliary withdrawal and selected worker has
joined. A positively completed batch immediately clears its owned commands and detaches
from every attached live lease, including untouched references that settlement
skips. Ready, held and UNKNOWN batches remain. There is no completed-source
ledger or separate retry worker; pruning takes only the existing lease mutex,
after releasing the batch mutex, and performs no engine I/O.

A positive original constrained SDK answer follows the engine's synchronous
bound-maintenance path. Zero withdrawals or a missing answer is not proof.
A known local NotDispatched or validated joined failure can retry the original
target. UNKNOWN retains the original request and deadline, refuses cleanup and
blocks blind retry: another target's DOWN, current absence, cancellation ACK or
a later call's NotDispatched cannot settle an earlier original writer.

<!-- source: internal/component/bgp/plugins/rib/rib_source_sent_output.go -- ribSourceSentOutput -->
<!-- test: internal/component/bgp/plugins/rib/rib_source_auxiliary_test.go TestGRSourceAuxiliaryOutputOwnsOriginalLease -->
<!-- test: internal/component/bgp/plugins/rib/rib_source_auxiliary_pruning_test.go TestGRSourceAuxiliaryDebtPrunesOnlyCompletedBatches -->

### Owner-excluded operational replay

`clear bgp rib out-excluding-owner owner <id> [family <afi/safi>]` omits one
configured destination owner, including when another owner shares its address.
It captures each remaining live owner's current UP receipt before output.
Ordinary destinations use their existing selected refresh worker, including
its configured-announcement replay and final End receipt. Omitting the family
also includes families carrying only configured announcements. Auxiliary destinations
replay immutable SENT attributes through bound markers and route requests; a
replaced destination cannot lend its successor to the rest of that bracket.
No same-session replay emits an initial readiness report. The command reports
refreshed and refused destination counts and returns an error when any captured
operation is unresolved; another destination's success does not clear it.
<!-- source: internal/component/bgp/plugins/rib/rib_resend_excluding_owner.go -- parseOwnerExcludedResend, selected, resend, resendExcludedSent -->
