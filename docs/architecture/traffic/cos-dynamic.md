# Dynamic Class of Service via RADIUS

BNG subscribers need a per-session 802.1p QoS profile assigned by RADIUS. The
static plugin ([cos-plugin](cos-plugin.md)) defines the named profiles and
`cos.Lookup()`. This adds the mechanism that applies one at session up.

## Decisions

### The `cos:` prefix in Filter-Id is the primary carrier

<!-- source: internal/core/cos/cos.go -- ParseFilterID, filterPrefix -->
<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- the Filter-Id walk -->
<!-- source: internal/component/l2tp/plugins/authradius/extract_vsa.go -- extractVSACoSProfile -->

Filter-Id (RFC 2865 attribute 11) already carries the shaper rate and is already
extracted. It is multi-valued, so a `cos:` prefix lets both values coexist, and
that avoided registering a vendor ID for a new VSA.

`extract.go` falls back to a vendor-specific attribute when no Filter-Id value
carries the prefix. The prefix is the primary path, and the VSA is the
compatibility path for a server that cannot send it.

### Subscribe to the L2TP session event, not the subscriber event

<!-- source: internal/plugins/cos/handler.go -- cosHandler, SessionUp handling -->
<!-- source: internal/component/l2tp/session_metadata.go -- AuthMetadata.CoSProfile -->

The lifecycle producer supplies its cached canonical subscriber ID for the
`AuthMetadata.CoSProfile` lookup and its packed Birth for CoS ownership. Event
readers never reconstruct ownership from reusable tunnel and session labels.
The original labels remain independent diagnostic values.
<!-- source: internal/plugins/cos/handler.go -- onSessionUp, onSessionDown, onCoSChange -->

### Session identity and shared access ownership are separate

The package-level `sessionStore` is a `sync.Map` keyed by `sessionKey`, the
original packed session Birth. Each admitted session refers to its shared map owner.
The handler's separate `accesses` map is keyed by access-interface name. These
are separate indexes; the access name is not part of `sessionKey`. Original
tunnel and session labels remain separate diagnostic data and never select
membership, compensation or cleanup.
<!-- source: internal/plugins/cos/session_state.go -- sessionStore, sessionKey, sessionCoSState -->
<!-- source: internal/plugins/cos/shared_access.go -- sharedCoSAccess, captureSharedAccess -->

The first use captures the interface backend's actual maps before a write.
Compatible subscribers share that baseline. A different subscriber's request
must preserve every surviving user's effective maps, including a completed
transaction's retained compensation maps. A nonfinal departure removes only
that user's membership. The final departure restores the captured baseline.
An unconfirmed write or restore keeps its original owner and cause for retry.
<!-- source: internal/plugins/cos/shared_access.go -- applyLegacySharedProfile, compatibleCompensation, departSharedSession -->

For a VLAN created by the selected kernel backend, the first admission retains
that creator's map lease before reading its baseline. Compensation, final
departure and cleanup use the original resource and transport; they never
resolve the currently selected device again. The creator verifies its birth
on native replies. A successful map write also reads back through that same
original lease and compares all eight effective ingress and egress mappings;
an ACK alone cannot confirm restoration. Failed or different readback retains
the original cleanup owner. Deletion and backend replacement are refused while
that lease remains live. A matching name, index or alias grants no lease. This
finite kernel mechanism supplies no configured-access generation, authenticated
SDK grant, external receiver or VPP acceptance.
<!-- source: internal/component/iface/native_access.go -- ResolvePreparedAccess, NativeVLANLease -->
<!-- source: internal/plugins/iface/netlink/native_access_linux.go -- captureNativeVLANDirect, readLocked -->
<!-- source: internal/plugins/cos/original_target.go -- captureCreatorCoSTarget, creatorCoSTarget -->

Unowned or preexisting access retains the historical immediate callback path.
It cannot authorize the new delayed recovery command, and stopping its original
runner fences those callbacks permanently. A retry or transferred departure
refuses without touching the replacement and retains the original warning,
baseline and custody. Follow that warning's original-owner recovery advice;
repeating a name-based write cannot confirm the original outcome. The diagnostic
owner key identifies only its warning lifetime.
<!-- source: internal/plugins/cos/original_target.go -- unresolvedCoSTarget, originalCoSTargetFix -->
<!-- source: internal/plugins/cos/cleanup.go -- retrySharedCleanup -->
<!-- source: internal/plugins/cos/handler.go -- takeStoppedOwners -->

### `ConfigureEventBus` for delivery

`ConfigureEventBus` constructs the dynamic handler with its required report
publisher before subscribing to typed session events. A stopped handler
unsubscribes and refuses event and command callbacks. Its exact completed
rollback token can still be acknowledged by the active replacement handler
without writing or clearing a new owner that later uses the old access name. The registration transfers the exact
retained map records from its stopped previous handler to the replacement;
it does not reconstruct a baseline from a new session or a name lookup.
<!-- source: internal/plugins/cos/register.go -- ConfigureEventBus -->
<!-- source: internal/plugins/cos/handler.go -- newCosHandlerFromStopped, stop, takeStoppedOwners -->

That hook runs inside the daemon. An external CoS process has no dynamic event
owner or interface backend wiring. Static profile configuration and
`ze show class-of-service` remain available. The daemon-bound report publisher
is explicit; it is not a fallback for an unavailable external transport.
<!-- source: internal/plugins/cos/register.go -- warnIfExternal, ConfigureEventBus -->
<!-- source: internal/plugins/cos/diagnostic_publisher.go -- publishDaemonCoSIssue -->

### CoA waits for the owner to apply its maps

<!-- source: internal/plugins/cos/session_transaction.go -- executeSessionTransaction, prepareSessionChange -->
<!-- source: internal/plugins/cos/register.go -- commandDecls, executeCommand -->

RADIUS dispatches the hidden `request class-of-service session` command through
the existing command transport. Its arguments are `transaction` and one JSON
value. Prepare carries `operation`, canonical `subscriber-id`, diagnostic
`tunnel-id` and `session-id`, `interface` and `profile`; its answer carries an
opaque `token`. The decoder parses the canonical ID once and retains its Birth
as the ownership key. Apply, rollback and commit
carry only `operation` and that token. The authenticated request's peer context
must remain the same for every step.

`sessionCommandDecls` registers this hidden command, and
`executeSessionTransaction` validates its keyword, bounded JSON value and
authenticated source. The configuration module holds the persistent profiles
and interface bindings.
<!-- source: internal/plugins/cos/session_transaction.go -- sessionCommandDecls, executeSessionTransaction -->
<!-- source: internal/plugins/cos/yang/ze-cos-conf.yang -- container class-of-service, container interface -->

Prepare changes no native maps. It captures the actual immediate beforeimage,
copies the proposed profile and reserves a compatible shared owner. It refuses
a beforeimage that could not later be restored without changing a surviving
subscriber's policy. Apply installs the maps before publishing membership. A
failed apply attempts compensation and retains the transaction for rollback;
an unconfirmed compensation does not become a successful result. RADIUS commits
only after every owner accepted the request, then acknowledges the CoA.
<!-- source: internal/plugins/cos/session_transaction.go -- prepareSessionChange, applySessionChange, rollbackSessionChange -->

The post-commit session event consumes one transaction receipt when its token
matches the retained committed snapshot and both current profile maps equal
the maps that command installed. That notification needs no second backend
write. Later legacy events still apply their profile. If the first notification
was lost and the profile's maps changed, the old receipt cannot suppress the
updated maps.

<!-- source: internal/plugins/cos/handler.go -- cosHandler.consumeCommandReceipt -->

The owner holds one pending change and one completed token with its snapshot.
The last committed change remains compensatable until the next accepted prepare
or its original session's departure invalidates it, so a
later owner's failed commit can restore every part of the request. This bounds
retained snapshots and makes retries of the last apply, rollback or commit idempotent.
Consuming the post-commit notification receipt does not release this beforeimage.
<!-- source: internal/plugins/cos/session_transaction.go -- prepareSessionChange, forgetSessionChange, rollbackSessionToken -->
<!-- source: internal/plugins/cos/handler.go -- consumeCommandReceipt -->
An outstanding transaction refuses another prepare, so another request cannot
overwrite the state needed to compensate it. A pending change expires after one
minute: the next request restores its maps before admitting new work. Session
departure invalidates its tokens, so a delayed apply cannot restore a departed
subscriber. An external CoS process has no
interface backend wiring and refuses the operation; it cannot acknowledge maps
it did not install.

The netlink backend reports both VLAN maps in `GetLink`, including an empty map
when none is installed. VPP's `detailsToInfo` reports link identity and VLAN
attributes but does not report the maps. Its `UpdateVLANQoSMap` can program
identity ingress and egress marking, but the atomic CoA path refuses prepare
when it cannot capture the previous maps to restore. Empty unreported fields
are never treated as evidence that VPP has no installed mapping. This limitation
does not change the existing configuration or session event paths.

<!-- source: internal/plugins/iface/netlink/show_linux.go -- linkToInfo -->
<!-- source: internal/plugins/iface/vpp/query.go -- detailsToInfo -->
<!-- source: internal/plugins/iface/vpp/ifacevpp.go -- UpdateVLANQoSMap -->

### Diagnose and retry failed cleanup

An unconfirmed native compensation or final restore raises the active warning
`doctor-cos-cleanup-unconfirmed`, with its original access, native cause and
stable bounded owner subject. Each failed retry refreshes that warning.
`ze show doctor` reads it through the registered recent-events route. A policy
refusal instead raises `doctor-cos-policy-refused` as an error-history event;
it does not label a healthy or pending policy as failed cleanup. The stored
policy event names the supplied profile and the RADIUS or profile correction;
the cleanup warning explains why its original owner remains retained. Both
producers log once per failed attempt at WARN with the same code and fix as
the report. Rollback retains raw native causes; its command, Down or cleanup
entry point owns that attempt's single warning publication. A healthy event raises
neither problem.

Warning state belongs to that exact captured access owner. It starts with no
Raise attempt, becomes pending before delivery is attempted, and becomes
clear only after the daemon acknowledges its original CLEAR. Unknown Raise
delivery still needs that clearance. A healthy owner with no Raise attempt
sends no CLEAR, so an unused report transport cannot make healthy rollback or
departure fail. A stopped-runner transfer preserves the original warning
state and subject with the map records. A failed CLEAR retains only its
publication cause after native confirmation; it is not another native failure.
<!-- source: internal/plugins/cos/shared_access.go -- sharedCoSWarning, captureSharedAccess -->
<!-- source: internal/plugins/cos/diagnostic_publisher.go -- diagnostic registrations, publishDaemonCoSIssue -->
<!-- source: internal/plugins/cos/diagnostic_shared.go -- policyFailure, cleanupFailure, cleanupClear -->

Before its first baseline read, an access owner reserves one bounded
`report.ResourceWarning` slot. A full store refuses admission before native
work. Each actual cleanup failure retains its original typed episode; retries
keep its first and latest causes. Ordinary source cleanup, a matching subject
or another runner cannot overwrite or clear it. The stopped-handler transfer
retains that same reservation and episode with its original target.

The actual episode publishes before the report callback acknowledges it. The
callback can report a lost or failed acknowledgement, but supplies no resource
custody. Its ordinary warning write is a protected-subject no-op. After original
native recovery and readback, CLEAR requires both that acknowledgement and the
original episode's clearance. Only released, cleared owners close their
reservation; healthy surviving users retain it for a later failure. A completed
old episode cannot clear a later one on that resource. Policy history continues
through the ordinary report publisher. External dynamic CoS has no daemon-local
reservation binding and remains unavailable.
<!-- source: internal/plugins/cos/shared_access.go -- captureSharedAccess, sharedCoSAccess -->
<!-- source: internal/plugins/cos/diagnostic_shared.go -- publishCleanupWarning, cleanupClear, closeUnusedCleanupReservation -->
<!-- source: internal/plugins/cos/session_transaction.go -- acknowledgeCompletedRollback -->
<!-- source: internal/plugins/cos/original_target.go -- creatorCoSTarget.write, releaseUnusedSharedAccess -->

Run the warning's command to retry the retained failed owners:

```text
ze request class-of-service cleanup
```

An unknown first baseline read retains the captured lease without admitting
a user or inventing maps. Its retry only joins and releases that original
operation. After a confirmed final restore, an unfinished join or release
retains a separate release phase; retry never repeats the restore. Warning
clearance follows confirmed original resource release. A failed CLEAR remains
report-only and cannot relabel that completed native phase as a new failure.
<!-- source: internal/plugins/cos/original_target.go -- sharedCoSCapturePending, sharedCoSReleasePending, finishRetainedSharedAccess -->

The command makes one bounded pass and reports `attempted` and `recovered`,
including both counts in a failed answer when other owners could not recover.
It compensates failed command transactions and finishes retained departures;
an owner counts as recovered only after all its selected cleanup completes.
It preserves healthy users, healthy pending transactions and completed
compensation snapshots. It returns every current failure while allowing other
failed owners to progress. Only confirmed original cleanup can clear its
warning. When that cleanup succeeded but clearing the warning failed, the
command retains only the failed publication and retries it without repeating
confirmed native work. The original completed rollback token can also retry
its failed warning clearance without another native compensation. To investigate a policy refusal, run
`ze show class-of-service` and correct the conflicting RADIUS assignment or
profile; cleanup does not remove another healthy user's policy.
<!-- source: internal/plugins/cos/register.go -- cleanupCommandDecls, sharedCoSCleanupResult -->
<!-- source: internal/plugins/cos/cleanup.go -- executeCoSCleanup, retrySharedCleanup -->
<!-- source: internal/plugins/cos/diagnostic_shared.go -- cleanupClear -->

## Consequences

- `UpdateVLANQoSMap` is part of the `Backend` interface. Every new backend
  implementation must provide it.
- The CoA handler accepts a rate and a CoS Filter-Id in one request.
  `readCoAChange` walks every Filter-Id and Vendor-Specific attribute once and
  records the rate and the profile through `setRate` and `setCoSProfile`, which
  refuse a second value of either with NAK 407.
- `AccessInterface` was added to `SessionUpPayload`. It is empty for a pure L2TP
  LNS, where the access VLAN is at the remote LAC, and the handler skips that
  case instead of failing.

<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- readCoAChange, setRate, setCoSProfile -->

## Traps

<!-- source: internal/plugins/iface/vpp/ifacevpp.go -- enableVLANQoS, UpdateVLANQoSMap -->
<!-- source: internal/component/l2tp/plugins/authradius/extract.go -- FindAllAttr for Filter-Id -->
<!-- source: internal/component/l2tp/plugins/authradius/coa.go -- readCoAChange walks pkt.Attrs -->

- **govpp binapi generates `[]byte` for a fixed-size `u8[256]` array.**
  `QosEgressMapRow.Outputs` must be explicitly allocated with
  `make([]byte, 256)`. `enableVLANQoS` carried a latent nil-slice panic that
  never fired, because no unit test exercised the VPP QoS path with a mock
  channel.
- **A multi-valued attribute needs `FindAllAttr`.** `extractAuthMetadata` used
  `FindAttr`, which returns the first match, so a Filter-Id carrying both a rate
  and a CoS profile lost one of them. The CoA path reads every attribute for
  the same reason: `readCoAChange` walks the whole attribute list.
