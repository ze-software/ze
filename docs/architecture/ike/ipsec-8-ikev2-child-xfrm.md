# Child SA and the dataplane abstraction

After IKE_AUTH completes, the engine creates ESP Child SAs, installs them in the
kernel or in VPP, monitors liveness with dead peer detection, and rekeys before
expiry. The dataplane is an interface so the same engine drives Linux XFRM and
VPP.

<!-- source: internal/component/ike/dataplane/dataplane.go -- Dataplane, SAParams, SPParams, SASelector -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- XFRM netlink backend -->
<!-- source: internal/component/ike/dataplane/vpp.go -- VPP backend -->
<!-- source: internal/component/ike/engine/child.go -- createFirstChildSA, installChildSA, removeChildSA -->
<!-- source: internal/component/ike/engine/established.go -- maintainSA, cleanupChild -->

## Decisions

**A `Dataplane` interface with Register, Load and Get, following the interface
backend pattern.** Embedding XFRM calls in the engine was rejected: the same
engine must support Linux XFRM and VPP.

**The dataplane package lives inside the IKE component, not under
`internal/plugins/`.** It is coupled to the SA lifecycle, not a standalone
plugin.

**The backend is chosen by `Load("xfrm")` at engine startup, not by
auto-detection.** Linux against VPP is a deployment decision.

**One 1-second ticker drives dead peer detection and lifetimes, not a timer per
SA.** Detection intervals start at 10 seconds, so losing up to one second of
precision costs nothing and removes a goroutine per SA.

<!-- source: internal/component/ike/engine/dpd.go -- dead peer detection state and probe -->

**Behind a NAT the SELECTORS move and the outer addresses do not.**
`createFirstChildSA` takes `LocalAddr` and `RemoteAddr` from the operator's
`local-address` and `remote-address`, which are the addresses each end's own
stack uses, and takes `Selectors`, `TSLocal` and `TSRemote` from the negotiated
set. On a transport-mode SA across a NAT that negotiated set carries the RFC 7296
Section 2.23.1 substituted addresses, so the policy Ze installs names the
addresses its own kernel will see on the wire. Nothing here reads the NAT verdict:
the substitution happened during negotiation and this constructor installs its
result.

Transport mode is why that is enough. There is exactly one IP header, the NAT
translates it, and ESP authenticates the ESP header and payload rather than the
IP header, so the translated header reaches the peer and matches the substituted
selectors. `XfrmStateEncap.OriginalAddress` stays unset: the kernel takes RFC 3948
Section 3.1.2's third alternative for transport mode, which
`natt-transport-inner-checksum` measures.

<!-- source: internal/component/ike/engine/child.go -- createFirstChildSA -->

**The XFRM interface id comes from the interface the peer's `vti { bind }` names.**
`resolveIfID` reads the if_id of that xfrm interface through the iface component
(`GetXFRMInfo`) when the first Child SA is created, on both roles. A peer with no
binding installs its SA with if_id 0, policy-based. A binding that names no xfrm
interface, or one whose if_id is 0, fails the Child SA with an error naming the
interface, so a misspelled binding is a refused tunnel rather than a tunnel that
silently installs unbound (that was the failure until 2026-09-16: `SiteToSitePeer.IfID`
was never assigned, so every bound peer installed with if_id 0). `show mtu` sizes a
tunnel by that if_id, which is how it finds the interface whose MTU it reports.

<!-- source: internal/component/ike/engine/established.go -- resolveIfID -->

The VPP backend uses the vendored, generated GoVPP IPsec API types. Its initial
SAD-ID allocation uses `IpsecSaV3Dump`. Public SAD and SPD inspection still return
`ErrNotSupported`; the allocation dump does not implement the readback contract.
<!-- source: internal/component/ike/dataplane/vpp.go -- firstFreeSadID, ListSAs -->
<!-- source: internal/component/ike/dataplane/vpp_policy.go -- ListPolicies -->

## Vendored netlink patch

The pinned `github.com/vishvananda/netlink` release lacks these XFRM
corrections. Ze records them in
`internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch`:

- The state reader decodes `XFRMA_REPLAY_ESN_VAL`, restores the replay window,
  and reports the `XFRM_STATE_ESN` flag.
- The state writer uses `XFRMA_REPLAY_ESN_VAL` for replay windows of more than 32. It
  sets `XFRM_STATE_ESN` only when the SA uses extended sequence numbers.
- The policy writer copies the selector family into any template that carries no
  destination, which in ze is every transport-mode template. An explicit
  destination still determines the template family.
- The policy reader preserves source and destination port masks, including partial
  masks and an exact port zero installed by another writer. These read-only fields
  leave the writer's existing any-port and non-zero exact-port semantics unchanged.
- The policy reader decodes selectors using each message's selector family.
  An all-family dump keeps `0.0.0.0/0` and `::/0` distinct, and IPv6 prefixes
  whose trailing bytes are zero retain their IPv6 identity. Tunnel endpoints use
  their template's family, which can differ from the selector family.
- The state reader uses the SAD message family for both outer addresses and the
  nested selector's family for its prefixes. The same address conversion serves
  the policy reader, so `2001:db8::` remains distinct from `32.1.13.184` even when
  SPI, protocol and interface id agree. Nested selectors retain raw port masks.
<!-- source: internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch -- XFRM state and policy decoding corrections -->

`TestXFRMReadbackPolicyTunnelAddressFamily` installs an IPv4 policy with IPv6
tunnel endpoints whose trailing bytes are zero, including `2001:db8::`.
It checks both endpoints through the backend and checks that removal clears the
policy from the kernel dump.
<!-- source: internal/component/ike/dataplane/xfrm_readback_integration_linux_test.go -- TestXFRMReadbackPolicyTunnelAddressFamily -->

Keep the patch and its drift test outside `vendor/` because `go mod vendor`
replaces that directory. Run this command from the repository root after every
vendor refresh:

```sh
git apply internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch
```

`TestNetlinkXFRMPatchApplied` uses `git apply --reverse --check` to require the
exact corrections with their context lines. It lives in
`internal/le/vendorpatch`; run it alone with
`go test ./internal/le/vendorpatch -run TestNetlinkXFRMPatchApplied`.

## Traps this code exists to avoid

**A nil dataplane looks like working code.** `Get()` returns nil when `Load()`
was never called, and `createFirstChildSA` then skips installation in silence.
Tests passed and the kernel held zero SAs. `Load("xfrm")` in `runEngine` is what
closes it. A new call path that installs SAs must not reintroduce the silent
skip.

**A hardcoded interface id only works by accident.** `resolveIfID` was 1, which
worked only if an XFRM interface happened to carry that id. It then read a config
field nothing assigned, so every bound peer installed unbound. It now resolves the
bound interface's if_id, 0 means no binding, and a dangling binding is an error.

**Dead peer detection needs its last-sent time initialized.** A zero value fires
a probe immediately on creation.

**Stopping a session can outrun child creation.** `reconcilePeers` calls `Stop`,
which triggers the child cleanup inside `maintainSA`, but the child may not
exist yet if the session is still in IKE_SA_INIT. The explicit child removal
after `Stop` in reconcile covers that race.

**A new peer field needs no edit to the comparison.** `peerConfigChanged`
decides whether to restart a session, and it asks one question:
`ipsec.SiteToSitePeer.Equal`, which compares the whole peer value. A member
added to the struct is compared on the day it is added, so a config change to it
restarts the session and reaches the wire. Subtracting a member from that
comparison is allowed, and it has to be done by name with the reason recorded on
`Equal`. It used to be the other way round: two hand-written field lists, one
here and one in `Changed`, that did not even name the same eight members, and a
member left out of both was ignored in silence.

<!-- source: internal/component/ike/engine/reconcile.go -- reconcilePeers, peerConfigChanged, Stop -->
<!-- source: internal/component/ike/ipsec/types.go -- SiteToSitePeer.Equal -->

**One field cannot carry both the KEYMAT role and the selector orientation.**
`ChildSA.Selectors` is stored in TSi/TSr order. `ChildSA.SelectorsLocalIsTSi`
says which half of each pair is this node's side, and the policy install reads
it. `ChildSA.LocalIsInitiator` answers a different question: which KEYMAT half
keys this pair (RFC 7296 Section 2.17). The two agree for a set the exchange in
hand negotiated, and they part company for a set the replacement inherited, so
one field cannot serve both.

Reading it to orient those selectors installed a port-swapped policy at the first
peer-initiated rekey. The kernel then protected the peer's port as this node's.
`samePolicySelector` stopped recognizing the pair the replacement shares its
policy with, so retiring the superseded pair removed the live pair's policy.

The orientation travels with the selectors it describes, so it names the exchange
that NEGOTIATED them. `newRekeyedChild` takes the set this rekey agreed, which
both roles hand it as `sa.NegotiatedPairs`, and stores it with that exchange's
orientation: the end that sent Ni is TSi (RFC 7296 Section 2.9). A rekey that
negotiates no set at all keeps the retired pair's selectors AND its orientation,
so the next rekey still has an RFC 7296 Section 2.9.2 floor. The KEYMAT role is
read for neither case.

<!-- source: internal/component/ike/engine/child.go -- ChildSA.SelectorsLocalIsTSi, selectorPort -->
<!-- source: internal/component/ike/engine/rekey.go -- newRekeyedChild -->

**Key material flows through four hops and each one must clear.** Derivation,
the child key struct, the SA parameters, then the install call. Any new path
inherits the clear chain.

<!-- source: internal/component/ike/crypto/keys.go -- DeriveChildSAKeys, ChildSAKeys.Clear -->
<!-- source: internal/component/ike/engine/child.go -- ChildSA.Clear -->

## SPD dispositions

RFC 4301 Section 4.4.1 gives the Security Policy Database three dispositions,
and `SPAction` carries all three. `SPActionProtect` is 0 on purpose, so a caller
who forgets the field gets a policy that protects rather than one that passes
traffic in the clear.

| Disposition | Producer | Linux XFRM | VPP |
|---|---|---|---|
| PROTECT | `childPolicyParams`, from a negotiated Child SA | `allow` with a template | `IPSEC_API_SPD_ACTION_PROTECT` |
| BYPASS | `ikeBypassPolicies`, and the operator `vpn ipsec policy` list | `allow` with no template | `IPSEC_API_SPD_ACTION_BYPASS` |
| DISCARD | the operator `vpn ipsec policy` list | `block` | `IPSEC_API_SPD_ACTION_DISCARD` |

BYPASS and DISCARD are TEMPLATE-FREE: neither hands traffic to a transform, so
neither names a mode, a tunnel endpoint pair or a reqid, and both are built
before every template check in `xfrmPolicyFromParams`. `SPAction.isTemplateFree`
is what asks the question, rather than each site testing one constant: a check
written as `Action == SPActionBypass` reads a discard as a protect policy and
then demands tunnel endpoints it must not carry.

`xfrmPolicyAction` and `vppSPDAction` never default. The two kernel actions are
opposites, so a default is a coin toss between passing and dropping the
operator's traffic, and a backend that cannot express a disposition refuses the
install rather than substituting another.

The readback reads DISCARD from the kernel action rather than from an empty
template list. The kernel accepts a template beside a `block` policy and ignores
it, so a discard another daemon installed with one would otherwise be reported
as a protect entry: the operator would be told their traffic is encrypted while
it is being dropped.

<!-- source: internal/component/ike/dataplane/dataplane.go -- SPAction, isTemplateFree -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmPolicyAction, xfrmPolicyFromParams, policyInfoFromKernel -->
<!-- source: internal/component/ike/dataplane/vpp_policy.go -- vppSPDAction -->
<!-- source: internal/component/ike/engine/spd_policy.go -- spdPolicyParams, installSPDPolicies -->

## Policy ownership

Two peers whose selectors overlap would otherwise take each other's kernel
policy. `policyOwners.claim` refuses a claim when the policy is already held by
another owner.

**A guard keyed on inequality is inert when both sides are empty.** The claim
refuses on `held != p.Owner`. Delete the owner at either producer and every
claim compares an empty string against an empty string, so the two-peer takeover
the guard exists to refuse is admitted in silence. The test that covered it
compared two empties, because its own fixture never set the field. Drive a guard
from the entry point that PRODUCES its input, never from the guard's own helper.

Ownership applies to PROTECT entries alone. `policyOwners` exempts every
template-free policy, because ownership exists to stop one PEER's selector
taking another peer's live tunnel over, and a bypass or a discard belongs to the
node rather than to a peer.

Installation and readback use the same ownership key. A nil selector becomes the
wildcard for the family the netlink writer selects: the destination prefix's
family, or IPv4 when the destination is nil. Explicit full-tunnel prefixes compare
equal to that wildcard, while IPv4 and IPv6 remain separate policies. Readback
retains the kernel prefixes and port masks; a foreign partial-mask policy cannot
borrow the owner of an exact-port policy.
<!-- source: internal/component/ike/dataplane/policy_owner.go -- policyKey, cidrKey -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- policyInfoFromKernel -->

The SAD reader translates the kernel's `XFRM_INF` hard byte and packet limits to
the public zero value for unlimited. Every finite `uint64` limit is retained.
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- hardLimitFromKernel, saInfoFromState -->

<!-- source: internal/component/ike/dataplane/policy_owner.go -- policyOwners.claim, policyOwners.release, PolicyOwnedError -->
<!-- source: internal/component/ike/engine/child.go -- childPolicyParams, firstSharingSelector, samePolicySelector -->

## Inbound classification

`inbound.go` classifies inbound INFORMATIONAL and CREATE_CHILD_SA messages for
an established SA. The negotiation itself is driven by the owner loop, described
in `docs/architecture/ike/ipsec-13-rekey-wire.md`.

<!-- source: internal/component/ike/engine/inbound.go -- inbound message classification -->
<!-- source: internal/component/ike/engine/delete.go -- Child SA teardown over INFORMATIONAL -->
<!-- source: internal/component/ike/engine/bypass.go -- IKE control-plane bypass policies -->

## INFORMATIONAL requests, the window and retransmission

`maintainSA` (`established.go`) is the owner loop of an established SA: one
goroutine, a one-second ticker, and sole ownership of `sa.NextMsgID`, the SK keys
and the request window. Every request Ze raises after IKE_AUTH is built there:
the DPD probe (`sendDPD`, `dpd.go`), the rekey (`startChildRekey`,
`startIKERekey`), the Delete (`delete.go`), the INVALID_MESSAGE_ID notify and the
padded path probe below.

Ze declares a window of one (RFC 7296 Section 2.3), so `reserveRequestWindow`
(`msgid.go`) claims the one slot before a request is built and binds it to the
current `NextMsgID`; `advanceMsgID` moves the counter after the send. Only an
AUTHENTICATED response frees the slot (`answerAuthenticatedResponse`, called from
the two post-decrypt sites in `handleOwnedInbound`), and a request that holds it
is repeated until it is answered or the SA is deemed failed (Section 2.1):

| Holder | Repeats through | Fails the SA through |
|--------|-----------------|----------------------|
| DPD probe | its own copy, on the liveness backoff (`retransmitDPD`) | the liveness budget (`dpdState.timedOut`) |
| rekey | its own copy (`serviceRekeyRetransmit`) | its retransmit budget |
| Delete, INVALID_MESSAGE_ID, path probe | the window's slot, `armRequestRetransmit` then `serviceRequestRetransmit`: 3 repeats, 500 ms doubling to 60 s | `serviceRequestWindow`: `StateDead` once `requestWindowTimeout` (30 s) passes |

`serviceRequestWindow` has two exits and no third: a response, or `StateDead`.
It releases no window without a response and rewinds no message id. The loop's
`StateDead` arm then runs `cleanupChild` and returns `errSADeletedByPeer`, so
`PeerSession.run` reconnects.

## The padded path probe

The MTU diagnostic can ask a live SA to measure its path with a padded
INFORMATIONAL exchange. The engine registers `probePeer` into the
`internal/core/ikeprobe` leaf at `init()`, beside the inventory snapshot, so
`internal/component/mtu/cmd` reaches it without importing the engine.

`probePeer` runs on the caller's goroutine and touches no SA state. It finds
the session by peer name (`lookupPeerSession`), refuses `sa-down` at once when
the session owns no established SA (`ownedSA` is nil), and otherwise sends the
request on `PeerSession.probeRequests`, an unbuffered channel `maintainSA`
receives on its select beside `stopCh` and `supersede`. Each request carries its
own reply channel, buffered 1, so the loop answers without blocking. The owner
loop is the only goroutine that builds under `sa.SKKeys` and advances
`sa.NextMsgID`, which is why the request crosses to it rather than being built
where it was asked.

`answerProbeRequest` (`probe.go`) answers on the loop. It refuses by name, before
anything is built, spending no message id and taking no window:

| Refusal | Condition |
|---------|-----------|
| `sa-down` | the SA is below `StateEstablished`, or has no send path or peer address |
| `rekey-pending` | `pendingRekey` holds the window |
| `rekey-held` | a TEMPORARY_FAILURE hold is in force (`ikeRekeyHoldUntil`, `childRekeyHoldUntil`) |
| `window-held` | a Delete, a DPD probe or an earlier path probe holds the window |
| `family` | the peer address is not IPv4 (the transport is `udp4`) |
| `size` | the size is above 3000, or below the smallest datagram the suite produces (a size off a CBC suite's 16-octet grid is sent at the grid point below it, and the answer names that size) |
| `send-failed` | the build or the write failed; nothing left the host and the window was handed back |
| `rekeyed` | the request left, then the peer rekeyed the IKE SA it left on (`maintainSA`, the `out.newSA` swap): the retired SA is forgotten with its window, so `retirePendingProbe` answers the probe with the size it sent and the MTU diagnostic asks that size again on the new SA (AC-10) |

Otherwise it runs the exchange in the order `sendDPD` uses: reserve the window,
build ONE INFORMATIONAL request whose one payload is a status Notify of the
private-use type `ZE_PATH_PROBE_PADDING` with its Notification Data sized so the
datagram is exactly the requested size, or the largest size at or below it a
CBC suite's grid reaches (the arithmetic is on
`docs/architecture/ike/ipsec-7-ikev2-engine.md`), send it through
`UDPTransport.SendDF` with the request's DF mode, advance the id, and arm the
window's retransmit slot with the same bytes. The probe's own state
(`PeerSession.pendingProbe`: id, size, whether a DF-clear copy went out, the
reported MTU, the reply channel) never touches `dpdState`.

Three things end it:

| Event | Mechanism | Outcome |
|-------|-----------|---------|
| an authenticated response at the probe's id, no DF-clear copy sent | the `inboundInvalid` arm frees the window; `settleProbe` matches the id | `fits` |
| an authenticated response after a DF-clear copy | as above; the copy went out on the retransmit schedule (`serviceRequestRetransmit` sends a probe holder through `SendDF` with DF clear) or at once when the socket's `Refusals()` channel reported a router's Fragmentation Needed for this SA's peer (`handleSizeRefusal`, matched on the peer's address and port, never on the router). Every established SA's owner loop on that socket reads the one channel (`probeRefusals`), so a refusal is read by one of them: read by another SA's loop it is dropped there, and the DF-clear copy then leaves at the retransmit timer (500 ms) rather than at once | `too-big`, with the reported MTU |
| no response through the full budget | `serviceRequestWindow`, unchanged: `StateDead`, then the loop's teardown; `failPendingProbe` answers on the way out of `maintainSA`, whatever the exit | `sa-failed` |

A local EMSGSIZE from the first `SendDF` (the kernel's path cache refused the
DF copy) is not a failure: the DF-clear copy leaves at once and the answer
reads `too-big` with the cache's figure. A probe response never moves the peer
endpoint (`adoptAuthenticatedEndpoint` is not on that arm) and never credits
liveness. An unanswered probe fails the SA exactly as an unanswered Delete
does: a probe is an ordinary request under RFC 7296 Section 2.1, and the owner
rejected a probe-aware exit that would release the window and rewind the id
(owner ruling (a), 2026-09-16, spec-ike-padded-path-probe). What
keeps a diagnostic from taking a tunnel down is the MTU module: it probes only
at or below the ICMP figure and stops at the first size that stays silent.

Ze's own responder answers the padded request as it answers any INFORMATIONAL
it does not act on: `handleInformationalOwned` sends an empty response, and the
private-use type is in the wire registry, so `logIgnoredNotifies` writes no
line for it.

<!-- source: internal/component/ike/engine/established.go -- maintainSA, serviceRequestRetransmit, serviceRequestWindow -->
<!-- source: internal/component/ike/engine/msgid.go -- reserveRequestWindow, armRequestRetransmit, answerAuthenticatedResponse -->
<!-- source: internal/component/ike/engine/dpd.go -- sendDPD, dpdState -->
<!-- source: internal/component/ike/engine/probe.go -- probePeer, answerProbeRequest, settleProbe, handleSizeRefusal -->
<!-- source: internal/component/ike/engine/reconcile.go -- PeerSession.probeRequests -->
