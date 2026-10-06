# SRv6 FIB Programming

Both FIB backends read the `SRv6SID` field of a best-change entry. The kernel
backend builds a SEG6 encapsulation. The VPP backend creates a local
encapsulation policy and steers the destination prefix into it. API-model
coverage is distinct from the native packet and installed-state probes below.

## Where the SID comes from

The SID reaches the FIB through the BGP **Prefix-SID attribute**, not through a
dedicated SRv6 NLRI family. The RIB pool extracts the SID from the attribute and
the RIB manager writes it onto the selected Loc-RIB path. The route-install RPC
and sysrib's live and replay batches preserve it.

<!-- source: internal/component/bgp/plugins/rib/pool/srv6sid.go -- ExtractSRv6SID, ExtractSRv6SIDFull -->

This matters because an SRv6 NLRI family is often assumed to be the prerequisite
for SRv6 forwarding. It is not. Any change that only needs `SRv6SID` populated
can proceed on the Prefix-SID path alone.

## Kernel backend

`buildSEG6Encap` builds the encapsulation and `buildRichRoute` reaches it. The
call is gated on a valid IPv6 SID on a unicast route. A Service SID selects
IPv6 encapsulation ahead of any label field, which can contain transposed SID
bits. Without a SID, the route uses MPLS when labels are present, or plain IP.
For ECMP, each next hop carries its own encapsulation rather than inheriting a
route-level label stack. A Service SID selects SEG6 on every member; without
one, a member uses its own labels or remains plain IP.

An IPv4 destination can have an IPv6 gateway after recursive resolution.
`buildRichRoute` encodes that gateway as `RTA_VIA`; a same-family gateway uses
`RTA_GATEWAY`. Each ECMP member selects its gateway attribute independently and
retains its device, on-link flag, weight and encapsulation. IPv6 destinations
with IPv4 gateways are rejected. An unusable member rejects the route update,
so the kernel never receives a silently reduced group.

<!-- source: internal/plugins/fib/kernel/nexthop_linux.go -- buildSEG6Encap, buildRichRoute -->

## VPP backend

`processEvent` selects the SRv6 path when a change carries a valid SID or its
prefix and table have owned SRv6 steering or a durable ordinary fallback from a
previous transition. `processSRv6Change` dispatches the normalized route verb.
Install and replace call `addSRv6Steer`; remove calls `delSRv6Steer`.
Unspecified and unknown actions normalize to Skip.
IPv4 and IPv6 destinations select their respective VPP steering traffic types.

The received Service SID is the policy's **only segment**, never its local
binding SID. `acquirePolicy` allocates a random IPv6 ULA binding SID and sends
`sr_policy_add` with encapsulation enabled. Only then may steering reference
that policy. Prefixes sharing a SID share the policy within the configured
underlay table. Replacing one prefix installs its new policy first, updates
steering without a preceding delete, and releases the old policy only after
its last reference leaves. Before attaching another prefix, Ze validates the
cached shared policy against a fresh VPP dump; a sequential foreign change is
not accepted as the original policy. Failed API requests return errors;
reconciliation checks the actual outcome where VPP exposes sufficient identity.

VPP's policy `FibTable` selects both the binding-SID FIB and the **outer IPv6
lookup table**. Ze uses the configured backend table for that underlay lookup.
A route's nonzero table override selects only its destination steering table.
The two tables must already exist in VPP. VPP's encapsulation source address
and SID underlay reachability must also be configured; this path does not
provision them. A zero-table withdrawal can recover a uniquely owned prefix's
table. If the prefix exists in multiple tables, the withdrawal must identify
one; Ze refuses to guess.

<!-- source: internal/plugins/fib/vpp/fibvpp.go -- processEvent, flushRoutes -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- processSRv6Change, acquirePolicy, addSRv6Steer, delSRv6Steer -->

### Ownership and restart

The existing daemon-owned state service stores policy, steering and ordinary
fallback records under the feature-registered
`meta/fib-vpp/srv6/{kind}/{identity}` key.
Storage errors block mutation. No extra operator option or loose state file is
introduced. A planned policy record precedes creation; only a successful API
reply followed by durable acknowledgement confirms ownership. Steering records
retain current and proposed binding SIDs before a replacement.

Admission reuses the production state-list contract, `rpc.StateListMax`
(4096 keys), for the **whole ownership namespace**, not per resource kind.
Every temporary policy, steering and fallback record needed by a transition
must fit before its first state or hardware mutation. For example, replacing
the last reference to a policy still needs a spare policy slot; its old record
cannot be released early. A new IP-to-SRv6 transition can need three slots.
At the limit, existing-key replay and withdrawal remain recoverable, but an
operation requiring additional records is refused. Ordinary fallback records
continue to count after a completed SRv6-to-IP transition until that ordinary
ownership is explicitly removed. No larger RPC limit, pagination protocol or
additional capacity option is introduced.

On process reconnect or restart, Ze checks durable records against VPP policy
and steering dumps. Confirmed policies must match encapsulation mode, underlay
table, weight and the single received SID. Foreign steering references or
conflicting policy contents cause an explicit error rather than adoption,
overwrite or deletion. A live **unconfirmed** policy is ambiguous: the API may
have succeeded before its durable confirmation failed. Ze reports its binding
SID and refuses adoption or cleanup. The operator must inspect that BSID and
the durable record before reconciling the uncertain resource; resemblance to a
desired policy is not proof of ownership.

Confirmed live ownership survives process restart; replay does not create
duplicates. A **Ze-managed VPP restart** emits the reconnect event: reconciliation
removes absent ownership records and replay recreates policies before steering. One
plugin-lifetime best-change subscription precedes initial ownership restore and
survives backend replacement. Its callback shares the lifecycle lock with restore
and resolves the current writer only after that lock is acquired. An event
arriving during restore therefore reaches the replacement, never a retired
writer. This retains withdrawals that partial sysrib replay cannot recover.
VPP offers no conditional steering replacement, so external writers must not
race Ze's ownership checks with CLI changes.

External-VPP mode does not currently emit that event when its separately
supervised VPP process restarts. Automatic recovery from that external restart
is not provided by this change; a Ze process restart reconnects and reconciles
durable ownership normally. Runtime restart proof must use Ze-managed VPP,
not a fabricated reconnect event.

<!-- source: internal/component/vpp/vpp.go -- runOnce -->

Sysrib replay has no complete-snapshot boundary. Ze therefore retains
durable-owned live references until explicit withdrawal; absence from a
partial replay is not permission to sweep them. Withdrawal removes steering
before last-reference policy cleanup. Unrelated policies are never swept.

Changing a service route back to ordinary IP persists an unconfirmed fallback
intent, installs the replacement, confirms ownership durably, then removes
steering. Changing ordinary IP to SRv6 checkpoints previously successful
ordinary ownership **before** creating a policy or steering; it removes that
older route only after steering succeeds. Restart can therefore restore the
exact prefix-and-table cleanup obligation in either direction. Ordinary
ownership, withdrawal, flush and installed-route output preserve table identity;
the same prefix in another table is not overwritten. Retained table zero is an
actual table identity, even if the configured default later changes.

A known VPP rejection rolls back the IP intent without removing previous SR
forwarding. Rollback restores the previous durable fallback record when one
existed; otherwise it removes the new intent. Absence of a previous record is
a valid first transition, not an error. A lost API reply or failed durable
confirmation leaves unconfirmed IP ownership and blocks further mutation
pending operator reconciliation.
VPP's IP dumps encode the best FIB source, not an ordinary API-source route
hidden beneath SR steering. Ze therefore does **not** infer ordinary ownership
from a matching live route. Confirmed ownership is durable installation history:
other writers must not replace those owned API-source routes, even
sequentially. The API does not provide a hidden-source contents check.

Withdrawal and flush remove an owned ordinary fallback before removing SR
steering; failed API or durable-record removal retains cleanup ownership and
SR forwarding for retry. A completed SRv6-to-IP transition retains its fallback record so a
later process restart still recognizes an explicit ordinary withdrawal.
The existing MPLS backend supports only its configured table; a transition or
restored MPLS fallback requiring another configured table is refused.

<!-- source: internal/plugins/fib/vpp/srv6_state.go -- restore, loadState, sameSRv6Policy -->
<!-- source: internal/plugins/fib/vpp/register.go -- srv6OwnershipKey, runFibVPPPlugin -->
<!-- source: internal/plugins/fib/vpp/srv6_fallback.go -- reserveState, checkpointSRv6Fallback, beginSRv6Fallback, finishSRv6Fallback -->
<!-- source: internal/plugins/fib/vpp/backend.go -- addRichRouteInTable, delRouteInTable, errVPPMutationUncertain -->

The owner included policy installation and local binding-SID ownership in this
pass on 2026-10-04, superseding the 2026-10-02 separate-scope decision.
`TestRFC9252VPPServiceRouteEncapsulatesTowardTheSID` and lifecycle controls model
the API dependency. `TestVPPSRv6ServiceRouteProbe` exercises BGP input, installed
policy identity and captured packets through the production best-change consumer,
including Ze restart, shared policy references, replacement and withdrawal.
The native managed-VPP probe separately exercises VPP restart and tenant tables.
The subscription fixtures join their delivery callbacks before closing the
durable store, including on assertion failure.
The API model accepts the outgoing steering types constructed by Ze; an unknown
internal type is a `BUG` assertion rather than a simulated VPP reply. Actual API
dump values remain external input and keep their rejecting paths.
<!-- source: internal/plugins/fib/vpp/rfc9252_srv6_encap_red_test.go -- srModelRequest.ReceiveReply -->
<!-- source: internal/plugins/fib/vpp/srv6.go -- steerTypeForPrefix -->

<!-- source: internal/le/test/deployment/vpp_srv6_probe_integration_linux_test.go -- TestVPPSRv6ServiceRouteProbe -->
<!-- source: internal/plugins/fib/vpp/register_test.go -- TestSRv6SubscriptionResolvesCurrentWriter, TestSRv6SubscriptionWaitsForPublication -->

Primary semantics:
[RFC 9252 Section 1](https://www.rfc-editor.org/rfc/rfc9252#section-1),
[VPP steering](https://github.com/FDio/vpp/blob/stable/2502/src/vnet/srv6/sr_steering.c),
[VPP policy/encapsulation](https://github.com/FDio/vpp/blob/stable/2502/src/vnet/srv6/sr_policy_rewrite.c),
and [VPP IP API dumps and route source](https://github.com/FDio/vpp/blob/stable/2502/src/vnet/ip/ip_api.c).

## Trap

A spec's "current behavior" section ages. The SRv6 backend was recorded as
blocked on an SRv6 NLRI family that it never needed, and as missing an
encapsulation path that already existed and was already tested. Re-derive the
input-to-FIB chain from the code before you trust a written status.
