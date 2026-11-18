# The unified subscriber session model

One shared session type, one event namespace and transport-generic handler
registries, used by both L2TP and PPPoE. Before it, three session models existed
side by side, PPPoE emitted no bus events, and authentication, pools, shaping,
accounting and change-of-authorization were wired to L2TP alone. PPPoE sessions
were invisible to downstream plugins and to the show commands.

<!-- source: internal/component/l2tp/subscriber/session.go -- Session, AccessType -->
<!-- source: internal/component/l2tp/subscriber/registry.go -- Registry, Add, Remove, All, ByAccessType, LookupByAcctSessionID -->
<!-- source: internal/component/l2tp/subscriber/handler_registry.go -- RegisterAuthHandler, RegisterPoolHandler, RegisterShaperHandler -->
<!-- source: internal/component/l2tp/subscriber/events/events.go -- the subscriber event namespace -->
<!-- source: internal/component/l2tp/subscriber_bridge.go -- subscriberBridge, onSessionUp, onSessionDown, onSessionIPAssigned -->

## Decisions

**A bridge subscribes to L2TP events and re-emits them as subscriber events.**
Modifying the L2TP reactor directly was rejected: the reactor is heavily tested
and the bridge is a clean seam.

**The internal registry map holds `*Session`, the public API returns copies.**
The session struct is 408 bytes, so range copies were flagged. Callers still get
value copies, so no caller can mutate registry state.

**L2TP handler registration delegates to the subscriber registry.** The L2TP
register functions wrap and forward. Existing L2TP plugins keep working with no
code change, and they do not import the subscriber package.

**Prefix handlers stay L2TP-scoped.** The prefix request and result types carry
a tunnel-id and session-id tuple that is specific to L2TP. Generalization waits
until PPPoE needs DHCPv6 prefix delegation.

**`DefaultRegistry` is a package-level singleton.** L2TP and PPPoE both write to
it and the show commands need one query point.

**A lifecycle consumer subscribes to the subscriber topic alone, never to both
topics.** The address pool was the first to move. It reads
`subevents.SessionDown` and no longer reads `l2tpevents.SessionDown`, because
the bridge re-emits every L2TP teardown onto the subscriber topic. Holding both
subscriptions would deliver each L2TP teardown twice. The two emitters cannot
disagree about whether the bridge exists: `Subsystem.Start` gives the reactor
and the bridge the same event bus, and builds neither when that bus is nil.

**`Session.PPPKey` names a transport-local PPP driver pair.** L2TP uses its
tunnel and session IDs; PPPoE uses its access interface and session IDs.
Those pairs can be equal across transports. They remain display fields.
Shared authorization and pool ownership use the existing canonical
`Session.ID`, carried unchanged through PPP requests and cleanup events.
The shared PPP driver admits by that opaque ID and binds an exact worker to
the transport's validated `SessionAttempt`; it never reconstructs the ID
from numeric coordinates.
<!-- source: internal/component/l2tp/subscriber/session.go -- PPPKey -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- handle, onSessionDown -->
<!-- source: internal/component/l2tp/session_metadata.go -- StoreSessionMetadata -->

When a transport supplies an L2TP birth in `StartSession`, PPP caches it with
the original canonical subscriber ID. Its lifecycle emitters copy both cached
values with the exact admitted `Control`. Admission refusal preserves the
incoming birth, ID and `Attempt`. A zero birth remains absent, including for
PPPoE. PPP copies the caller's metadata without parsing the ID or rebuilding it
from numeric labels.
<!-- source: internal/component/l2tp/ppp/manager.go -- spawnSession, sendRejection -->
<!-- source: internal/component/l2tp/ppp/events.go -- bindEventControl -->
<!-- source: internal/component/l2tp/ppp/ncp.go -- sendIPAssigned -->

Both NAS authentication drains require the ID before provider lookup, realm
selection or pending-authorization changes. A missing ID receives a refusal
without a subscriber result event. Valid opaque IDs pass unchanged to the
provider and its result event. Pool drains apply the same guard before
invoking the allocator. The L2TP pool drain also validates the ID before
resolving its current provider.
<!-- source: internal/component/l2tp/drain.go -- startAuthDrain, startPoolDrain -->
<!-- source: internal/component/l2tp/pppoe/drain.go -- startPPPoEAuthDrain, startPPPoEPoolDrain -->

The PPPoE drains and realm relay retain the complete original auth or address
request for every answer. Its private wait capability expires when that
decision ends, so a delayed answer cannot satisfy a replacement wait on the
same worker. The realm handoff also retains the exact attempt for transport
teardown after PPP releases the channel to its L2TP bridge.
<!-- source: internal/component/l2tp/pppoe/realm_relay.go -- realmHandoff, endRelayedSession -->

## Packed L2TP identity value

`subscriberid.L2TPBirth` is a comparable 128-bit value in the dependency-free
`internal/core/subscriberid` package. Its private words keep an accepted value
immutable. `NewL2TPBirth` draws sixteen cryptographic bytes and refuses an
all-zero draw; no tunnel number, listener index or shared registry contributes
to that value. Zero is absent, rather than an admitted subscriber.

The external representation is `l2tp-` followed by exactly 32 lowercase
hexadecimal digits. Parsing rejects a zero value or a noncanonical spelling.
Text decoding changes its receiver only after the entire input is accepted.
Internal equality and map keys use the packed value; text conversion belongs
at the wire or human boundary. Possessing the value grants no authority to
admit, mutate or clean up a session.

The optional `Session.L2TPBirth` field uses the JSON key `l2tp-birth`. An absent
value is omitted, so a PPPoE snapshot still serializes with its complete access
index and session ID. A populated value round-trips without changing the
packed identity or the snapshot's existing canonical ID and numeric labels.
<!-- source: internal/core/subscriberid/l2tp_birth.go -- L2TPBirth, NewL2TPBirth, ParseL2TPBirth, IsValid, IsZero, MarshalText, UnmarshalText -->

The three actual call admissions mint this value before inserting an
`L2TPSession` in its tunnel: incoming ICRQ, incoming OCRQ and local call
origination. An invalid entropy result refuses the call before publication
and raises an internal-defect doctor warning. A later successful mint clears
that transient warning. Wire SID allocation remains a separate operation.
<!-- source: internal/component/l2tp/session_birth.go -- newSessionBirth -->
<!-- source: internal/component/l2tp/session_fsm.go -- handleICRQ, handleOCRQ -->
<!-- source: internal/component/l2tp/session_initiator.go -- newInitiatorSession -->

Those constructors also cache the canonical subscriber ID before publication.
`Birth` and `SubscriberID` remain unchanged through PPP and network restarts.
The kernel handoff supplies both to the original PPP attempt. Its lifecycle
consumers check both facts beside the exact admitted Control or refused
Attempt; reusable wire labels cannot supply a missing identity.
<!-- source: internal/component/l2tp/session.go -- Birth, SubscriberID, matchesPPPIdentity -->
<!-- source: internal/component/l2tp/session_fsm.go -- handleICRQ, handleOCRQ -->
<!-- source: internal/component/l2tp/session_initiator.go -- newInitiatorSession -->
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleKernelSuccess, pppSessionLocked, handlePPPEvent -->

The actor captures framed-route metadata under its row lock using that cached
ID. Its ordered assignment notification carries an independent route slice;
the route observer copies it into a record keyed by the complete Birth.
Address changes, withdrawal and replay use that same record. The observer
does not rebuild a metadata key from the Birth or wire labels.
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleSessionIPAssigned -->
<!-- source: internal/component/l2tp/route_observer.go -- OnSessionIPUp, OnSessionDown, reemitAll -->

The subscriber bridge and profile readers preserve the supplied ID when
loading authorization metadata. Session and LCP teardown clear only that ID.
Prefix allocation and release also require the caller's original ID; tunnel
and session numbers remain diagnostic labels on the request.
<!-- source: internal/component/l2tp/subscriber_bridge.go -- onSessionUp, onSessionDown, onSessionIPAssigned -->
<!-- source: internal/component/l2tp/session.go -- removeSession -->
<!-- source: internal/component/l2tp/session_timeout.go -- startSessionTimeouts -->
<!-- source: internal/component/l2tp/handler.go -- PrefixRequest, PrefixReleaser -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- handlePrefix, releasePrefixBySubscriberID -->

`CaptureSession` resolves a readable selector while all selected reactor row
locks are held in subsystem order. A second match is an ambiguity refusal.
The returned private capture retains the original reactor, tunnel, row, bound
listener and Birth. Its snapshot and history methods refuse once that exact
row is gone, including when a replacement has the same numeric IDs. A counter
read also retains and rechecks its original PPP attempt, control and network
lifetime around the native read.
<!-- source: internal/component/l2tp/session_capture.go -- CaptureSession, currentLocked, Snapshot, FSMHistory, Counters -->

The clear-session command parses an optional canonical `expected-id` once into
the typed Birth filter, then uses the captured original for audit and teardown.
The CLI list and detail renderer exposes that reference as `birth` only when
the original snapshot carries a valid Birth; a zero value omits the field.
Audit retains the observer and hidden native generation selected from that
original reactor and rechecks its row
under the reactor lock. A retry of an already removed original records no new
audit event and can still finish its retained cleanup. It never records against
a replacement by looking its SID up again.
<!-- source: internal/component/l2tp/session_capture_audit.go -- RecordDisconnect -->

Observer history uses the complete Birth. A captured history read checks its
original row and reads only that original observer; it does not look the SID
up again. Aggregate CQM queries remain on the subsystem, while single-session
history reads stay bound to the captured original. Closed observers preserve
their readonly snapshots while rejecting
copied callbacks. The bounded CQM disconnect entry is separate from command
audit and is not a durable action receipt.
<!-- source: internal/component/l2tp/session_capture.go -- CaptureSession, SessionEvents -->
<!-- source: internal/component/l2tp/observer.go -- SessionEvents, Stop -->
<!-- source: internal/component/l2tp/cmd/session_teardown.go -- parseSessionTeardownRequest, resolveSelector -->
<!-- source: internal/component/l2tp/cmd/l2tp.go -- sessionJSON -->

The capture's teardown contract requires the original physical owner to
confirm release before success. Logical removal, session-down publication and
a queued numeric kernel deletion do not establish that result. A pending
physical release remains an explicit refusal.

Session removal revokes queued native setup before releasing the row. Both
queued cleanup and the worker's resource object retain that original row;
reusable tunnel/session numbers locate resources but cannot authorize their
deletion. The worker reserves an occupied pair until its original delete and
descriptor outcomes are confirmed. Each distinct descriptor, including zero,
is consumed once before Close. A failed raw Close retains uncertainty and is
never retried by its recyclable number. An unconfirmed native delete retains
its original owner for an exact retry.
<!-- source: internal/component/l2tp/session.go -- removeSession -->
<!-- source: internal/component/l2tp/kernel_linux.go -- setupSession, teardownSession, teardownSessionFDsLocked, deleteTunnelLocked -->

The captured teardown removes the original row once, then checks that row's
native receipt and original PPP attempt. It attempts native cleanup before
queueing any still-needed copy of that original teardown. When its admitted
PPP worker is already closing the owner, it waits for that exact worker's
retirement within the command context, then requires confirmed physical release
and no recorded cleanup error. Cancellation returns promptly without certifying
release. A constructor or native operation whose completion remains unconfirmed
returns an explicit pending result. A failed cleanup retains its original row outside
the active SID map, allowing a Birth-qualified retry without selecting a
replacement. Stopped reactors with unresolved owners remain attached to the
subsystem; a new Start refuses until their exact cleanup is confirmed. Their
kernel backend stays owned until those retained consumers release it.
<!-- source: internal/component/l2tp/session_capture.go -- Teardown -->
<!-- source: internal/component/l2tp/session_retirement.go -- cleanupCapturedOriginal, retainStoppedReactorsLocked, cleanupRetiredReactorsLocked -->

Before channel/unit descriptors leave the kernel worker, the original row
retains its PPP construction receipt. Removal during construction therefore
remains pending even though no PPP attempt exists yet. A constructor refusal
distinguishes its primary error from an unconfirmed rollback Close; the latter
keeps the original cleanup root. An owner returned after removal remains with
that same receipt until its unused Close is confirmed. A failed raw Close is
retained as uncertainty rather than retried through a recycled descriptor.
<!-- source: internal/component/l2tp/ppp_construction.go -- pppConstruction -->
<!-- source: internal/component/l2tp/ppp/kernel/session.go -- newSession -->

When a constructed owner never reaches the PPP driver's input channel, the
transport records that exact unused owner's actual Close result on its
canceled attempt. A confirmed Close finishes the unused handoff; a failure
finishes its observation while preserving the error and original owner.
Recording this result cannot rewrite an attempt the driver already claimed.
Cleanup reads the row's immutable attempt under the reactor lock before
calling the owner outside that lock, then rechecks the original row's receipts.
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleKernelSuccess, closeUntransferredPPPAttempt -->
<!-- source: internal/component/l2tp/ppp/session_control.go -- RecordUntransferredClose -->

PPPoE session admission belongs to the prepared access. Its required
`AccessOwner.Open` supplies the original provider's session factory. Kernel
preparation owns one independent netlink backend shared by that access's
sessions. VPP admission borrows the exact original preparation and its captured
API connection generation. The caller MUST stop admission and join all dependent
sessions. It MUST then close the access.
<!-- source: internal/component/l2tp/pppoe/dataplane/access.go -- AccessOwner, preparedAccess.Open -->
<!-- source: internal/component/l2tp/pppoe/dataplane/kernel/prepare.go -- Prepare, kernelAccessOwner -->
<!-- source: internal/component/l2tp/pppoe/dataplane/vpp/prepare.go -- accessPreparationLease.Open, accessPreparation.borrow -->

## Session rate policy contract

The rate policy boundary distinguishes two sealed variants. A local policy
borrows the created session's rate controller; a remote policy carries only
its declared command owner. The remote variant does not pretend to provide a
local controller. Both declare an operational request command using the same
command grammar. A missing declaration is an error, never a request to choose
the kernel implementation.
<!-- source: internal/component/l2tp/subscriber/rate_policy.go -- RatePolicy, LocalRatePolicy, RemoteRatePolicy, NewRateOwnerCommand -->

The local controller receives a complete, immutable pair of positive rates.
The policy changes neither native units nor burst sizes. It preserves a
prepared transaction returned with an error, so the caller can compensate it.
That caller must finish installation before it acknowledges assignment and
allows forwarding. The session handle remains the sole cleanup owner: the
borrowed policy has no independent close operation.
<!-- source: internal/component/l2tp/subscriber/rate_policy.go -- Rates, LocalRatePolicy, RateController, RateTransaction -->

## Subscriber transmit limit

### Owner requirement, 2026-11-04

> The auth policy holds, but the MRU control doesn't do its job, so I wouldn't treat this spec as closed. With a 1492 limit, Ze can still transmit above it. All of this still needs a fix; I changed nothing and no rows are filed.

> Fix for 1 and 2: limit the transmit MTU to the session's mru

The second excerpt is the intact opening of the owner's final paragraph;
the remainder arrived with truncated words and is not reconstructed here.

### Design derived from the requirement

Each admitted L2TP or PPPoE session retains its transmit cap through normal
LCP, accepted proxy LCP and later renegotiation. The effective transmit limit
is the smaller of the peer's negotiated receive MRU and the session cap. An
omitted peer MRU means the PPP default of 1500; a larger valid proxy MRU
remains admissible without raising the installed interface MTU above the cap.
The peer's raw negotiated MRU remains distinct from this effective limit.
<!-- source: internal/component/l2tp/ppp/session_run.go -- transmitMRU, afterLCPOpen -->
<!-- source: internal/component/l2tp/ppp/auth.go -- acceptProxyPolicy -->

The same limit bounds userspace PPP replies. Complete configuration options
and mandatory authentication fields are preserved; a reply that cannot fit
fails the attempt before an unsent response can advance negotiation. The
receive buffer and kernel receive MRU retain the full 1500-octet capacity.
<!-- source: internal/component/l2tp/ppp/session_run.go -- writeFrame, sendConfigureAck, sendConfigureNakOrReject, afterLCPOpen -->
<!-- source: internal/component/l2tp/ppp/ncp.go -- sendNCPConfigureAck, sendNCPConfigureNakOrReject -->
<!-- source: internal/component/l2tp/ppp/mschapv2.go -- fitMSCHAPv2FailureMessage -->

## Consequences worth knowing

- A new access type such as IPoE follows the same three steps: populate the
  default registry, emit subscriber events, register the authentication and pool
  handlers.
- Change-of-authorization can match a session by accounting session id across
  access types.
- `show subscriber summary` is the single view over every session type. The
  access-type commands remain for transport-level detail.

<!-- source: internal/component/l2tp/subscriber/cmd/subscriber.go -- subscriber CLI handlers -->
<!-- source: internal/component/l2tp/subscriber/metrics.go -- session telemetry -->
<!-- source: internal/component/l2tp/subscriber/service.go -- service locator -->

## The order of one L2TP session's notifications

`tunnelsMu` orders every change of an L2TP session's state. A path that changes
a session and has a notification to deliver takes the tunnel's next order
ticket while it still holds `tunnelsMu`. A notification is a route observer
call, or an `(l2tp, session-ip-assigned)`, `session-up` or `session-down`
event. After the path releases the lock, it waits until every earlier ticket
of that tunnel has finished, delivers, and then finishes its own ticket. Every
reader and the route observer therefore receive one session's notifications in
the order its state changed, whichever goroutines changed it: the reactor, an
operator command, a timeout or a CoA Disconnect. The sequence belongs to the
tunnel, because a tunnel reuses its session IDs. Notifications of different
tunnels do not wait for each other.

No lock is held while a reader runs. A slow reader delays only the later
notifications of its own tunnel, and the goroutine that carries them. RADIUS
accounting is the slowest: it sends Accounting-Stop synchronously, bounded at
10 seconds. When the waiting goroutine is the reactor's own, packet processing
waits with it, as it does whenever the reactor publishes to a slow reader
(`plan/journal/slow-reader-stalls-a-shared-goroutine.md`).

<!-- source: internal/component/l2tp/session.go -- sessionNoticeOrder, sessionNoticeTicket, issueNoticeTicket, awaitTurn, deliver -->

| Notification | Ticket taken by | Delivered by | What runs after the delivery returned |
|--------------|-----------------|--------------|----------------------------------------|
| Session-down of a removed session | `drainPendingTeardowns`, once for the batch it returns, stamped on each down | `deliverSessionDowns`, one batch per turn | Outbound CDN or StopCCN, kernel teardown |
| Session-down that ends a network lifetime (LCP down in the network phase) | `handleLCPDown` | `notifyNetworkLifetimeDown` | `ClearSessionMetadata`, then PPP's acknowledgement |
| IP assigned | `handleSessionIPAssigned` | the same, with the route observer's IP-up | PPP's acknowledgement, which enables forwarding |
| Session up | `handleSessionUp` | the same | The RADIUS session and idle timeouts |

<!-- source: internal/component/l2tp/session.go -- removeSession, drainPendingTeardowns -->
<!-- source: internal/component/l2tp/teardown.go -- deliverSessionDowns, notifySessionDowns -->
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleLCPDown, notifyNetworkLifetimeDown, handleSessionIPAssigned, handleSessionUp -->

Inside a batch's turn, `notifySessionDowns` tells the route observer to
withdraw the route of each down whose session was established, then publishes
one session-down per down. A session removed before it was established gets
its session-down and no withdrawal. Only an established session ran PPP and
can hold a subscriber route. The observer now selects the complete original
Birth; wire labels do not select another subscriber's record. The rule that a
pre-PPP removal owes no route withdrawal remains the same on
every removal path: operator, CoA, timeout, PPP, kernel error, a peer StopCCN
or CDN, a dead peer, retention reaping and a lost tie breaker.

<!-- source: internal/component/l2tp/session.go -- sessionDown, removeSession -->
<!-- source: internal/component/l2tp/teardown.go -- notifySessionDowns -->

Every ticket is finished on every exit of the path that took it, a contained
panic included. A path takes its ticket as its last step under `tunnelsMu` and
delivers it as its first step after the unlock, so nothing that can panic runs
between the two. `handle`, the control task's panic boundary, drains before
other locked work, so it defers the delivery before it takes `tunnelsMu`. The
deferred call runs after the deferred unlock and before
`clearTunnelAfterPanic` tears the tunnel down, so the teardown's own ticket
never waits behind one the panic left open. A reader that panics still
finishes its ticket, because `sessionNoticeTicket.deliver` defers the finish.

<!-- source: internal/component/l2tp/reactor.go -- handle -->
<!-- source: internal/component/l2tp/panic_guard.go -- clearTunnelAfterPanic -->

**The reader contract.** An in-process reader runs on the publishing
goroutine. It must not cause a publication for the same tunnel from inside its
handler, by publishing an L2TP session event or by calling the reactor:
that publication would wait behind the ticket its own caller holds, and the
tunnel's notifications would stop. The contract is written on the events
package.

<!-- source: internal/component/l2tp/events/events.go -- the reader contract in the package doc -->

## Traps this code exists to avoid

**Registry insertion must precede the event emit.** The PPPoE event consumer
first handled only session-down. Expanding it to session-up and IP-assigned
required that ordering, or a subscriber to the event does not see the session.

PPP reports each negotiated address before the final session-up event. The
registry keeps that state as `configuring`; session-up preserves the addresses
and marks the subscriber `active`.

Both transports MUST acknowledge each IP-assigned event after synchronous
address publication completes, including its lifecycle callbacks. PPP waits for
that acknowledgment before NCP succeeds and before it enables forwarding.
Rejected assignments and missing sessions are also acknowledged, so the producer
does not remain blocked after the consumer returns.

PPP records the request username only when the authentication handler accepts
the request. Each IP-assigned event carries that accepted username to the
transport, which publishes it in the subscriber registry and lifecycle events.
L2TP also uses it for subscriber routes and accounting. A rejected request
cannot replace the accepted username. A PPPoE session a realm relay takes
never reaches the handler: the drain hands it to the L2TP call, PPP records
no username, and the session is released to the LNS without an
authentication reply (`docs/guide/l2tp.md`, "Relay by realm").

While a decision is pending, PPP keeps serving the link: it answers LCP
Echo-Request, drops a retransmitted CHAP Response or PAP
Authenticate-Request (the decision answers it), and ends the wait on an LCP
Terminate-Request or a renegotiation. A realm relay's decision can take a
dial and an ICRQ, ICRP and ICCN round trip, so the keepalive matters there.

Explicit no-auth admission has an empty username. L2TP retains a proxy identity
supplied for the current transport until PPP supplies an accepted username.
LCP-down clears the accepted username and the transport's network-lifetime
identity, so replacement admission cannot reuse the previous subscriber.

<!-- source: internal/component/l2tp/ppp/auth.go -- awaitAuthDecision, serveFrameAwaitingDecision -->
<!-- source: internal/component/l2tp/pppoe/realm_relay.go -- realmRelay.decide -->
<!-- source: internal/component/l2tp/ppp/ncp.go -- onNCPOpened, resetNCP -->
<!-- source: internal/component/l2tp/reactor_kernel.go -- handleSessionIPAssigned, handleLCPDown -->

LCP-down ends the network lifetime without ending the transport. L2TP withdraws
its subscriber routes. Both transports release the old allocation and
authorization state before they acknowledge LCP-down. PPP waits for that
acknowledgment before it can start replacement authentication. Session-down
remains responsible for the L2TP CDN or PPPoE PADT.

Per-session cancellation still publishes the final session-down event. A stop
while address publication is pending cannot leave the subscriber registered.

<!-- source: internal/component/l2tp/pppoe/subsystem.go -- handlePPPEvent, onSessionUp, onSessionDown -->
<!-- source: internal/component/l2tp/pppoe/drain.go -- startPPPoEAuthDrain, startPPPoEPoolDrain -->

**A teardown is published even when the registry holds no session.** An address
can be allocated before its NCP completes. A publication gated on a registry
hit leaks that address. Both transports use the session identifiers when no
registry entry exists.

<!-- source: internal/component/l2tp/subscriber/session.go -- PPPKey, AccessIfIndex -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- setEventBus, onSessionDown -->

L2TP accounting starts once per network lifetime, not once per address family.
A later IPv4 assignment updates subsequent records without replacing the
accounting session. LCP-down stops that session before replacement authentication.

Accounting subtracts each network lifetime's initial raw interface counters.
Traffic from an earlier lifetime is not billed again when the PPP unit remains.
The original network lifetime is established at the first valid address
assignment, before its synchronous publication and acknowledgement. The second
address family and SessionUp retain that same lifetime. Exact captured counter
reads check its original PPP attempt, Control and network pointer around the
read; a pre-network capture has no baseline. The L2TP accounting callback captures
its baseline during address publication, before PPP enables forwarding.
Accounting-Start runs asynchronously, so PPP does not wait for a RADIUS server
response.
Clearing display counters does not change accounting. Unavailable counter values
are omitted and logged, not reported as zero.

PPPoE accounting is not connected to subscriber events.

<!-- source: internal/component/l2tp/plugins/authradius/acct.go -- subscribeEventBus, onSessionIPAssigned -->

**The bridge must not be built on a nil event bus.** L2TP subsystem tests start
with a nil bus, and the bridge panics on one. Guard before constructing it.

**Two structurally identical types are still two types.** The auth respond
function and the auth result exist in both the l2tp and the subscriber package
with the same shape and different names. The delegation layer converts field by
field; assignment does not compile and a type conversion would hide a future
divergence.

## Pool ownership across reload

The pool handlers and their configured allocator share the daemon's memory.
Automatic loading and explicit internal placement use that same allocator.
An external pool process is refused before configuration callbacks or SDK
declaration, because its separate allocator cannot serve the daemon's handlers.
<!-- source: internal/component/l2tp/plugins/pool/register.go -- init, runPlugin -->

Verification distinguishes removal from malformed input before it constructs
pool descriptions. An absent `l2tp` root or `pool` block removes the configured
pools. Empty root and pool objects also remove the pools; empty named lists
have no entries. A present block or named-list entry must be an object.
Scalars, arrays and null values are refused, so a malformed
candidate cannot silently clear the running pools. The engine sends an empty
object section when it removes a root.
<!-- source: internal/component/l2tp/plugins/pool/config.go -- parsePoolDescription -->
<!-- source: internal/component/l2tp/plugins/pool/config_shape.go -- poolConfigObject, poolConfigList -->
<!-- source: internal/component/plugin/server/reload_compensation.go -- reloadConfigSections -->

Session allocation records are keyed only by canonical subscriber ID and are
the source of truth. A pool reload rebuilds its
bitmaps from those records while allocation and release are excluded by the
same locks. Each IPv4 record names the replacement pool that contains its
address. A prefix reserves every overlapping new IPv6 slot, including slots in
different replacement pools. A shared slot remains reserved until its last
live owner releases it.
<!-- source: internal/component/l2tp/plugins/pool/reload.go -- installPoolConfiguration -->
<!-- source: internal/component/l2tp/plugins/pool/pool.go -- carry, release -->
<!-- source: internal/component/l2tp/plugins/pool/pool_v6.go -- carry, releaseOverlap -->

An assignment outside all replacement ranges stays with its session. Teardown
removes that record without changing another pool. Rollback rebuilds the old
ranges from the records held then, so it also preserves sessions admitted
after apply.
<!-- source: internal/component/l2tp/plugins/pool/reload.go -- carryIPv4, carryIPv6 -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- onSessionDown, releasePrefix -->

RADIUS admission checks the same live records before reserving an assignment.
An IPv4 address stays exclusive even outside every current range. The pool
that contains it owns its bitmap reservation; the `Framed-Pool` profile still
supplies the gateway and DNS. IPv6 admission checks actual prefix overlap,
then reserves every overlapping configured slot. Disjoint smaller prefixes
may therefore share a coarse slot without sharing subscriber address space.
Each family holds its allocation lock through the check, reservation and
record store, so a reload cannot pass between those steps.
<!-- source: internal/component/l2tp/plugins/pool/radius.go -- reserveRADIUSAddressLocked, reserveRADIUSPrefixLocked -->
<!-- source: internal/component/l2tp/plugins/pool/register.go -- handle, handlePrefix -->

Pool status takes both family read locks in installation order to snapshot the
default pools, named pools and IPv4 allocation records together. It releases
those locks before looking up usernames by canonical ID in the subscriber
registry. Existing
top-level default statistics remain unchanged; optional `named-pools` and
`named-ipv6-pools` maps expose the same statistics for named pools.
<!-- source: internal/component/l2tp/plugins/pool/register.go -- showPool, showPoolSnapshot, poolShowResult -->

## Undelivered pool answers

A pool answer owns its reservation before PPP receives it. If the session ends
while the request waits in a drain, its session-down can run before that
reservation exists. Both transport drains therefore return an accepted
answer's reservation when `Driver.IPResponse` cannot deliver it. The release
travels as `(subscriber, pool-answer-undelivered)`, carrying the canonical
subscriber ID and the pool's opaque `AllocationID`. Numeric PPP coordinates
are included only for diagnostics.

The pool releases only the record whose subscriber ID and allocation ID match
the answer.
A delayed failure therefore cannot release a successor's reservation, even
when the transport reuses its identifiers. This event concerns one address
answer; it does not publish another session-down or clear authentication,
shaping or accounting state. Refusals and answers without an owned allocation
produce no release. A missing bus or a failed publication raises
`doctor-internal-defect`, visible through `ze show errors`.

The caller stops the PPP driver and joins its drain before discarding the
subscriber bus. A small driver interface in each drain lets unit tests order
session removal before delivery without needing a kernel PPP session.

<!-- source: internal/component/l2tp/ppp/manager.go -- IPResponseArgs, IPResponse -->
<!-- source: internal/component/l2tp/drain.go -- startPoolDrain -->
<!-- source: internal/component/l2tp/pppoe/drain.go -- startPPPoEPoolDrain -->
<!-- source: internal/component/l2tp/subscriber/events/pool_answer.go -- PoolAnswerUndeliveredPayload, ReleaseUndeliveredPoolAnswer -->

PPPoE reload pauses discovery before draining removed accesses. Their old
servers remain reachable by the PPP event consumer while it completes the
captured admissions. Reload waits without holding subsystem/table/discovery
locks, checks the old server pointer before detaching it, then installs the new
map and resumes discovery. An observed refused-transfer cleanup failure returns
an error and retains the old server/admission for retry. Realm release is an
explicit no-Down handback after that exact protocol worker finishes; it never
hands the transferred data-plane lease back for a raw transport-side Close.
<!-- source: internal/component/l2tp/pppoe/subsystem_reload.go -- quiesceRemovedServers, closeInterface, detachDrainedServers -->
<!-- source: internal/component/l2tp/pppoe/admission_control.go -- finishRejectedAdmission, waitAdmission -->
<!-- source: internal/component/l2tp/pppoe/realm_relay.go -- releaseRelayedSession -->

If a joined PPP runtime retains a failed refused-transfer owner, the next Stop
or Reload first invokes the existing Driver retry sweep with that caller's
context, then re-observes the same Attempt. A successful retry can complete
the old closeDone before normal idle startup; an error keeps that same owner.

Before PPP transfer, failed or panicking local lease cleanup retains the exact
Opened borrower, unused lease, relay and table row. Only successful same-owner
cleanup completes closeDone and frees the SID. SessionTable.CleanupStatus
copies the current producer failure/progress under the table lock; nil error
alone is never a deletion receipt. A provider's cached or unknown Close failure
remains retained; no raw descriptor or carrier-only recovery API is used.

Concurrent unused-lease borrowers share the first Close outcome, including a
retained panic cause. Ordinary Close never retries that first unknown result.
A later table-claimed producer cleanup attempt explicitly calls the same
lease's private retryClose; confirmed successful cleanup stays idempotent.
<!-- source: internal/component/l2tp/pppoe/session.go -- sessionLease.Close, sessionLease.retryClose -->
<!-- source: internal/component/l2tp/pppoe/service_relay.go -- beginAdmissionClose -->
