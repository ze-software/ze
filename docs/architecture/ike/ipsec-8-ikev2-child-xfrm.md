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

**The outbound SPI of a Child SA is always the peer's number, never Ze's.**
It comes from SAr2, SAi2, or the SA payload of a CREATE_CHILD_SA. RFC 4303
Section 2.1 reserves SPI 0, so each of those four parsers refuses a peer SPI of
0, and `createFirstChildSA` refuses an SA that holds no peer SPI. A responder
refusing an SAi2 SPI of 0 refuses only the Child SA: its IKE_AUTH response
carries NO_PROPOSAL_CHOSEN in place of SAr2, and the IKE SA stays up childless
(RFC 7296 Section 2.21.2). RFC 7296 Section 3.10.1 lets either NO_PROPOSAL_CHOSEN
("SA payload values ... are not acceptable") or INVALID_SYNTAX ("some type,
length, or value was out of range") report it, and Section 2.21.2 makes
INVALID_SYNTAX delete the IKE SA, so IKE_AUTH answers the first. An earlier
version drew a random SPI in its place, which keyed an outbound SA the peer
never allocated.

<!-- source: internal/component/ike/engine/fsm.go -- handleAuthResponse -->
<!-- source: internal/component/ike/engine/responder.go -- selectAuthChildSA -->
<!-- source: internal/component/ike/engine/responder.go -- buildChildlessAuthResponse -->
<!-- source: internal/component/ike/engine/rekey.go -- applyChildRekeyResponse, respondChildRekey -->

**One 1-second ticker drives dead peer detection and lifetimes, not a timer per
SA.** Detection intervals start at 10 seconds, so losing up to one second of
precision costs nothing and removes a goroutine per SA.

<!-- source: internal/component/ike/engine/dpd.go -- dead peer detection state and probe -->

**Transport-mode NAT substitution changes the selectors, not the outer addresses.**
`createFirstChildSA` normally takes `LocalAddr` and `RemoteAddr` from the operator's
`local-address` and `remote-address`. A negotiated MOBIKE tunnel instead uses its
authenticated current endpoints. The constructor takes `Selectors`, `TSLocal`
and `TSRemote` from the negotiated
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

**A Child SA's UDP encapsulation ports come from its IKE SA, never from a constant.**
RFC 3948 Section 2.1 says "the Source Port and Destination Port MUST be the same as
that used by IKE traffic," and RFC 4555 Section 3.3 adds that the tunnel header
"(and port, if doing UDP encapsulation)" is "taken from the IKE_SA". Every Child SA
that needs encapsulation passes through one derivation, `childEncapPorts`, called by
`takeEncapPorts` from `newChildSA` (IKE_AUTH, CREATE_CHILD_SA in either direction)
and after `newRekeyedChild` (a rekey in either direction, which does not copy the
retired child's ports). The peer's port is the port of the IKE SA's authenticated
endpoint (`sa.peerEndpoint`), so a peer behind a NAT that rewrote its IKE source
port to Y gets ESP on Y, and a NAT-mapped peer port of 500 is followed like any
other. Ze's own port is the port of the NAT-T socket it sends IKE from (the MOBIKE
local path where MOBIKE recorded one), which the test-port hook binds off 4500, and
it is never 500 (RFC 7296 Section 2.23, "UDP encapsulation MUST NOT be done on port
500."). `installChildSA` writes those two ports and defaults nothing.

With no authenticated endpoint, or no NAT-T socket, there is no port to answer, so
the Child SA is refused with `errChildEncapPort` and nothing is installed. Where Ze
answers the exchange it refuses before sending (a childless IKE_AUTH response, or an
error notify). Where Ze initiated it, the peer has already installed its half, so Ze
sends a Delete listing its own inbound SPI of the pair, the SPI "as they would be
expected in the headers of inbound packets" (RFC 7296 Section 1.4.1), and keeps the
IKE SA; a refused rekey leaves the old Child SA in use. Both responders adopt the
authenticated endpoint before they build the first Child SA, so no production path
is expected to reach the refusal.

<!-- source: internal/component/ike/engine/create_child.go -- childEncapPorts -->

**Which kernel object holds the inbound SA's selectors depends on the mode.** RFC
4301 Section 4.4.2 makes the inbound SAD entry carry the negotiated selectors, and
Linux checks a decrypted packet against them in two places. In tunnel mode
`installChildSA` leaves `SAParams.Sel` nil and the inbound require-policy does the
work: `__xfrm_policy_check` drops a decapsulated packet that matches no inbound
policy while its secpath holds a tunnel-mode state (`XfrmInNoPols`). A
transport-only secpath passes that check, so in transport mode the inbound state
carries the selector itself (`SAParams.Sel`, the XFRM `x->sel`), built from the
inbound policies' own fields so the two cannot disagree, and a packet outside it is
dropped as `XfrmInStateMismatch`. strongSwan makes the same split.

**Every negotiated selector pair gets its own policy, in each direction.** A peer
that answers several TSi/TSr pairs negotiated several values, and `childPolicies`
builds one inbound and one outbound policy per pair, all protecting with the one SA:
XFRM resolves each to the state through its template, VPP through the SAD id. A pair's
two halves each carry a protocol, and a packet matches the pair only when it matches
both, so the policy carries their intersection: TSi "any" with TSr TCP installs TCP.

**TSi and TSr are read as every combination of their entries.** RFC 7296 Section
3.13: "a packet matches a given TSi/TSr if it matches at least one of the individual
selectors in TSi, and at least one of the individual selectors in TSr." So an answer
TSi {A}, TSr {C, D} negotiates A-C and A-D, and Ze installs both (`answerPairs`,
`crossPairs`); entry i is never paired with entry i alone. A packet carries one protocol
and one address family, so `crossPairs` leaves out a combination whose protocols are
disjoint or that pairs an IPv4 selector with an IPv6 one: it matches no packet. A
dual-stack list of one IPv4 entry and one IPv6 entry already holds every combination of
its entries (`sameFamily`). A payload pair can therefore
state only a selector set that is every combination of its own entries, and
`pairsToWire` refuses any other set rather than offer or answer combinations the set
does not hold. Three places follow from it. The responder walks every combination of
the proposal against policy, and `acceptablePairs` refuses a combination of disjoint
protocols or of an IPv4 and an IPv6 selector, even from a peer with no configured
`traffic-selector`, whose policy otherwise accepts the proposal whole. A proposal that
holds only such combinations, such as TSi {10.0.0.0/8} with TSr {2001:db8::/32}, is
answered TS_UNACCEPTABLE (RFC 7296 Section 2.9). `closedSubset` then keeps, first choice first, the
largest set of acceptable pairs whose every combination is acceptable and that installs
at most `answerPairsMax` policies: a policy of (A, C) and (B, D) answers the proposal
TSi {A, B}, TSr {C, D} with A-C alone, because answering both would hand the peer A-D
and B-C. The initiator refuses an answer whose combinations leave its proposal. And a
configured `traffic-selector` list that is not every combination of its local and
remote selectors, such as (A, C) with (B, D), "cannot be negotiated as a single Child
SA pair" (Section 3.13), so `ze config verify` refuses it
(`validateSelectorCombinations`, `engine/config.go`) and names the missing combination.

**The initiator installs the answer exactly, or refuses it.** The responder's TS
payloads are the negotiated values (RFC 4301 Section 4.4.2), so
`recordInitiatorSelectors` reads them through `answerSelectors`, never through the
responder's narrowing `wireToSelectors`. An answered address range that is not one
prefix becomes the exact set of prefixes tiling it (`rangePrefixes`), one pair each,
and an answer expanding past `answerPairsMax` (64) policies is refused. A port range
other than any port, one port or OPAQUE is refused: XFRM (`xfrmSelectorPort`) and
VPP (`vppPortRange`) program every port or exactly one, so `tcp/1024-65535`, which
strongSwan answers for such a policy, has no exact form short of 64512 policies. A pair
neither dataplane can program (an OPAQUE port) refuses the whole answer rather than
being dropped. A refusal is TS_UNACCEPTABLE on IKE_AUTH, a deleted pair on a creation,
and a Delete for the new pair on a rekey. The responder installs what it answers
(`narrowChildSelectors` sets `NegotiatedPairs`, which both `pairsToWire` and the
policies read), so its narrowing is never a gap between the ends.

**An IKE_AUTH response that accepts a Child SA without naming its traffic is
refused.** RFC 7296 Section 2.9: "Two TS payloads appear in each of the messages in
the exchange that creates a Child SA pair." A response carrying SAr2 and no TSi or no
TSr negotiated no selector at all.
`adoptAuthResponseNegotiation` deletes the IKE SA with INVALID_SYNTAX and logs
`childResponseFixNoTS`, rather than install the Child SA with host-to-host selectors
for every protocol and port, which is wider than the pairs Ze proposed. A response
with no SAr2 (an EAP round, a childless answer) carries no selector and is not
refused for it.

**A transport-mode Child SA carries exactly one pair.** An XFRM state holds one
selector, and a selector that covers several pairs admits a decrypted packet in no
pair (Linux limitation `LL-XFRM-STATE-ONE-SELECTOR`,
`docs/architecture/linux-limitations.md`): RFC 4301 Section 5.2 requires such a
packet to be discarded. Both negotiation ends therefore keep one pair. The
responder narrows a transport-mode answer to its first pair
(`oneTransportPair`, `narrowChildSelectors`), which RFC 7296 Section 2.9 permits
("the responder arbitrarily chooses one of them") and which keeps the initiator's
first choice the same section requires. The initiator refuses a transport-mode
answer of several pairs (`recordInitiatorSelectors`, `errTSTransportPairs`): the
IKE_AUTH path deletes the SA with TS_UNACCEPTABLE, a CREATE_CHILD_SA creation
deletes the pair and names the fix (`childResponseFixTransportPairs`), and a refused
rekey keeps the Child SA in use. `inboundStateSelector` then copies
the one inbound policy into the state, and refuses a transport-mode Child SA of
several pairs as a Ze defect rather than widen it. Tunnel mode keeps every pair.
The interop scenario
`responder-two-subnets` has strongSwan propose two subnets in one Child SA and
reads both kernels: each must hold exactly the two ESP policy pairs.
`initiator-answer-one-local-two-remote` turns the roles round: Ze proposes one local
and two remote subnets, strongSwan answers one TSi and two TSr selectors, and both
kernels must again hold both pairs (RFC 7296 Section 3.13). MOBIKE migration moves tunnel-mode SAs only (the
migration refuses a non-tunnel template), and it carries the old state selector
unchanged. The kernel probes are in
`internal/component/ike/engine/rfc4301_sad_selector_linux_test.go`.

**Two SAs are refused before they reach the kernel.** `xfrmStateFromParams` builds no
state for an SPI of zero, which RFC 4303 Section 2.1 reserves and forbids on the wire,
so a peer that proposes SPI 0 fails the Child SA install rather than getting an
outbound SA that sends it. It also refuses a tunnel-mode SA whose tunnel header
addresses mix IPv4 and IPv6 (RFC 4301 Section 4.4.2.1). Each RFC 4301 Section 4.4.2.1
SAD data item Ze installs is asserted one item at a time in
`internal/component/ike/dataplane/rfc4301_sad_items_linux_test.go`.

<!-- source: internal/component/ike/engine/child.go -- installChildSA, childPolicies, inboundStateSelector -->
<!-- source: internal/component/ike/engine/ts_narrow.go -- narrowChildSelectors, oneTransportPair, recordInitiatorSelectors, answerSelectors, answerPairs, rangePrefixes, crossPairs, closedSubset, pairsToWire -->
<!-- source: internal/component/ike/engine/config.go -- validateSelectorCombinations -->
<!-- source: internal/component/ike/engine/transport_mode.go -- adoptAuthResponseNegotiation -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmStateFromParams -->

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
corrections (Linux limitation `LL-NETLINK-XFRM-DECODE`,
`docs/architecture/linux-limitations.md`). Ze records them in
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
| PROTECT | `childPolicies`, from a negotiated Child SA | `allow` with a template | `IPSEC_API_SPD_ACTION_PROTECT` |
| BYPASS | `ikeBypassPolicies`, the operator `vpn ipsec policy` list, and `unmatchedPolicies` under `vpn ipsec unmatched bypass` | `allow` with no template | `IPSEC_API_SPD_ACTION_BYPASS` |
| DISCARD | the operator `vpn ipsec policy` list, and `unmatchedPolicies` under `vpn ipsec unmatched discard` | `block` | `IPSEC_API_SPD_ACTION_DISCARD` |

The template-free XFRM action mapper is called only after `isTemplateFree`
selects BYPASS or DISCARD. An unnamed action at that internal mapper raises a
`BUG` panic; an explicit PROTECT input still returns its existing unsupported
operation error because it belongs to the template path.
The VPP mapper receives actions from the same local policy builders or checked
operator policy parsing. It also asserts `BUG` for an unnamed internal action
rather than choosing a VPP disposition.

MOBIKE migration policies likewise come from the local Child SA builder with
inbound/outbound directions. A named forward direction retains its refusal;
an unnamed internal direction raises a `BUG` panic before migration proceeds.

`unmatchedPolicies` (`engine/unmatched.go`) is the catch-all of RFC 4301 Section
5: a wildcard selector in, out and fwd for both families at
`PriorityUnmatched`, the largest u32 the kernel holds, so it is searched last.
The kernel passes an unmatched packet with no policy at all, so the discard the
section mandates is an entry Ze installs rather than a default it inherits. VPP
holds no entry bound to no interface (`vppPolicyInterface`), so it installs no
catch-all at all. A DISCARD catch-all that is not installed is never a warning:
config verify refuses `unmatched discard` when the loaded backend's
`CatchAllSupported` (`dataplane.CatchAllInstaller`) says no, or when no
dataplane is loaded, and `installUnmatched` returns an error for a discard
install that fails, which fails the apply before the operator's entries, the
cookie threshold or any peer change. A BYPASS catch-all that is not installed
fails the apply the same way (no dataplane, no XFRM, a failed install), because
without the entry an inbound packet meets no SPD-I entry at all (RFC 4301
Section 5.2). The one bypass neither installed nor refused is on a backend whose
`CatchAllSupported` answers `dataplane.ErrNoNodeWideSPD` (VPP), one of the two open gaps on
RFC4301-5.2-1 and RFC4301-5.2-9.

Ze's SPD is one unit with a lifetime of [first apply, exit]. `applyConfig`
installs the catch-all first and, on the first apply only, the IKE bypass
(`installIKEBypass`) right after it, before any IKE socket opens; engine start
installs nothing. Every exit of `runEngine` stops every peer, removing its Child
SA (`stopAllPeers`), then `releaseSPD` removes the IKE bypass, the operator's
entries, and the catch-all LAST. So no Ze entry is in force without the final
entry beneath it, and outside that window Ze holds no SPD entry and no SA: it is
not acting as an IPsec implementation then (owner decision 2026-10-20). A
removal that fails (anything but ENOENT, another peer's ownership refusal, or
`dataplane.ErrNotSupported`, the backend's answer for an entry it cannot hold, such
as the IKE bypass on VPP) leaves its entry in the SPD ledger (`removeZePolicy`,
`spd_left.go`), which every Child SA policy, Child SA install rollback, bypass and
operator removal reports to; `releaseSPD` then keeps
the catch-all beneath the entries left and logs them at Warn, each written as
the command the engine's backend names for removing it (`warnSPDLeft`,
`spdLeftover.describe`, `policyDeleteCommand`): on XFRM the exact
`ip xfrm policy delete` command, every selector field included, and on VPP the
`vppctl ipsec policy del` line for the SPD entry the backend recorded. The whole shutdown runs in `runEngine`'s
deferred cleanup for a clean and an error return alike: `reEstablishFn` is
cleared and the IKE sockets closed after the peers stop and before the release,
so no session can install a policy behind it. This lifetime is the IKE engine's
SPD only: OSPFv3 RFC 4552 manual keying (`internal/plugins/ospf/ipsec_install.go`)
installs its own SAs and transport policies with no catch-all, and they are not
released by the IKE engine. The
engine records whether configuration reached the SPD writers (`spdApplied`);
an SDK startup failure before it removes nothing, so a pre-existing policy it
never managed survives.

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
<!-- source: internal/component/ike/engine/unmatched.go -- unmatchedPolicies, installUnmatched -->
<!-- source: internal/component/ike/engine/apply.go -- applyConfig, releaseSPD -->

## Policy ownership

Two peers whose selectors overlap would otherwise take each other's kernel
policy. `policyOwners.claim` refuses a claim when the policy is already held by
another owner.

Transport mode is compared for overlap as well as identity. Two clients behind
one NAT reach the server from one address, so their transport-mode selectors
differ only in protocol and ports (RFC 3948 Section 5.2). When a different owner
holds a transport-mode selector that overlaps the claimed one, `claim` refuses
with `TransportSelectorConflictError`, which disallows the conflicting
connection. Without this, the kernel's ordered search would send the overlap to
whichever client's SA ranks first. Here, tunnel-mode selectors are compared for
identity only, because an overlap between operator prefixes that priority orders
is a deliberate configuration.

A tunnel-mode overlap a PEER chose is refused earlier, by the IKE engine and for
every backend: the engine's cross-peer inner-selector registry
(`innerSelectorRegistry`, RFC 3948 Section 5.1) drops such a pair at narrowing
and refuses it at `installChildSA` before any dataplane call. It judges overlap
with `dataplane.PolicySelectorsOverlap`, the same predicate over `policyKey`
that `claim` uses for transport mode, so the two cannot disagree. The VPP and
noop backends run no `policyOwners`, which is why that guard does not live here.

The registry records transport-mode Child SAs as well. A tunnel one peer
negotiates around another peer's transport-mode address (an unbound peer
proposing TSi 198.18.0.9/32 and TSr 0.0.0.0/0 while the transport-mode peer at
198.18.0.9 holds 198.18.0.9/32 to 192.0.2.1/32) matches the same outbound packet
at one priority, which is the Section 5.1 confusion, so it is refused, and so is
a transport-mode connection inside another peer's tunnel, in either arrival
order, at narrowing and at install. Narrowing judges the answer in the mode the
Child SA it creates installs: the mode this node decided for the IKE SA
(`decidedChildMode`, `sa.UseTransportMode`) for a new Child SA, and the retired
Child SA's mode for a Child SA rekey (`respondChildRekey` calls
`narrowChildSelectorsIn` with `old.Mode`), because an IKE SA rekey leaves
`UseTransportMode` unset on the replacement SA. The refusal names both peers and
the selector or pool fix on the tunnel peer. Only the overlap of two
transport-mode pairs is left to `claim` above (`conflictLocked`,
`bothTransport`), so it is refused once, with `TransportSelectorConflictError`.

The operator-ordered exemption applies to a tunnel against a transport-mode pair
as to two tunnels: both pairs operator-fixed, at different `policy-priority`
values (`operatorOrdered`). A pair is judged whole, as the policy it installs:
both prefixes, the protocol (the intersection of the two halves' protocols,
`childPolicyPair`) and both ports (`samePolicyPair`). A tunnel pair is
operator-fixed when it is exactly a traffic selector its operator configured. A
transport-mode pair is operator-fixed when it is exactly such a selector, or when
it is exactly the pair between the peer's configured `local-address` and
`remote-address` as host prefixes, /32 or /128, for any protocol and any port on
both halves (`classifyInnerPair`, `configuredHostPair`, `isConfiguredHost`),
because a transport-mode connection's selectors are its endpoint addresses and an
ordinary host-to-host peer configures no traffic selector. The operator wrote
those addresses and nothing else, so a pair the peer narrowed to one protocol or
one port on either half (TSi 198.18.0.9/32 TCP port 80, or TSi any with TSr
192.0.2.1/32 TCP port 80), or a host other than the configured address, is the
peer's choice, as inside a configured selector, and is refused against an
overlapping pair at any priority (`TestRFC3948OrderedTransportNarrowedToPortRefused`,
`TestRFC3948OrderedTransportLocalHalfNarrowedRefused`,
`TestRFC3948TransportPeerChosenHostBesideConfiguredTunnelRefused`). The refusal
(`innerSelectorTransportConflict`) names that traffic selector with both halves at
the connection's own host prefixes. So a transport-mode peer at 198.18.0.9 beside
a full-tunnel peer configured with 0.0.0.0/0 on both sides is served when the two
priorities differ, and the second to establish is refused when they are equal. A
peer whose `remote-address` is a DNS hostname rather than an address literal has
no configured host, so its transport-mode pair is ordered only when it is exactly
one of its configured traffic selectors.

With no dataplane (`Get()` nil) nothing is installed, and the record is still
taken, so the control plane never holds two peers' Child SAs for one inner
address; `removeChildSAExcept` releases it at once. With a dataplane, the record
is released only after the removal (`releaseInnerRecord`), and only when every
policy the Child SA installed is gone by `policyGone`: removed, ENOENT, owned by
another peer, or refused as a dataplane with no XFRM (`isXFRMUnsupported`, the
same refusal on which `installChildSA` kept a degraded Child SA and its record).
The SPD ledger (`spdLeftover.record`) reads the same predicate, so the two never
disagree. Any other failure strands the record with the policies left. An
overlapping peer is then refused with `errInnerSelectorStranded`, whose text
names, for each policy, the removal command of the dataplane that holds it and
no configuration of the holder, which may have been removed. The command is the
backend's own (`dataplane.PolicyDeleteCommander`), named once when the record is
stranded (`policyDeleteCommand`): `ip xfrm policy delete` with every selector field
for XFRM, and `vppctl ipsec policy del` for VPP, written from the SPD entry the
VPP backend recorded, because VPP matches a delete on the whole entry, SA id
included, and the SA is already removed by then. The VPP backend's own retry
deletes that same recorded entry (`vppBackend.recordedEntry`): while the policy's
SA is held, the entry naming its SAD id; once the SA is removed, the entry with
the selector whose SA the backend no longer holds, since SAD ids are never
reused. A rebuilt entry would name no SA, and the removal used to answer
`ErrNotSupported`, which `policyGone` reads as gone, so the record was released
with the entry still in VPP's SPD. The engine keeps removing the SA after a
failed policy removal: on XFRM the stranded policy's template then resolves no
state and its traffic is not tunnelled, and on VPP the entry holds its own
reference on the SA, which VPP frees once the retried delete lands (`spdEntries`,
`vpp.go`). A backend that names no command
(the noop one, whose removal never fails) is said to name none, with the
selector, rather than handed another backend's command. `retryStranded` removes
them again before each narrowing, before each claim and on each
`reconcilePeers`, one removal per policy each time with no goroutine of its own,
and releases the record once all are gone; a count of stranded records lets it
return at once when there is none. It retries through the dataplane that
installed the policies, which is the engine's one backend for its life
(`dataplane.Load` in `runEngine`). A claim by the same owner adopts each stranded
policy its own install re-creates (`adoptStrandedLocked`), because the upsert
makes the kernel entry the live Child SA's, and a later retry must not remove it.

The retry holds the registry's `retryMu` from its copy of the stranded records,
through every removal, to its last settle, and `claim` takes the same lock before
the registry's map lock (order `retryMu`, then `mu`; nothing takes them the other
way). Without it, a claim between a retry's copy and its removal adopted a policy
the retry then removed, deleting the new Child SA's live policy (the XFRM owner
check passes for the same owner, and the VPP backend checks none), and two
retries could both remove one policy, the second taking the policy another peer
had just installed on the freed selector. What the lock spans is bounded: one
netlink or VPP API request per stranded policy.
<!-- source: internal/component/ike/dataplane/policy_owner.go -- policyOwners.claim, selectorsOverlap, PolicySelectorsOverlap, TransportSelectorConflictError -->
<!-- source: internal/component/ike/engine/inner_selectors.go -- innerSelectorRegistry, retryStranded, claim, policyDeleteCommand, adoptStrandedLocked, innerSelectorStrandedConflict, conflictLocked, bothTransport, innerSelectorTransportConflict -->
<!-- source: internal/component/ike/dataplane/policy_command.go -- PolicyDeleteCommander, XFRMPolicyDeleteCommand -->
<!-- source: internal/component/ike/dataplane/vpp_policy.go -- vppBackend.PolicyDeleteCommand, vppBackend.RemovePolicyParams, vppBackend.recordedEntry, vppPolicyDelCommand -->
<!-- source: internal/component/ike/engine/child.go -- removeChildSAExcept, releaseInnerRecord, policyGone, removeStrandedPolicies -->

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
<!-- source: internal/component/ike/engine/child.go -- childPolicies, firstSharingSelector, samePolicySelector -->

## MOBIKE migration

`TunnelMigrator` is a backend capability, separate from installing a new SA.
Migration must preserve the live outbound sequence, replay window, keys and
lifetime. Deleting a state and reinstalling its original keys would reset the
sequence under the same key; it is not an implementation of migration.

The engine waits for a valid COOKIE2 response, then migrates both current Child
states and their policies. It also moves a superseded pair still awaiting the
peer's Delete, without moving shared policies twice; distinct selectors retain
their own migrated policies. Inner selectors and policy ownership do not change.
Child rekey inherits the migrated addresses and UDP mapping rather than returning
to the configuration's original endpoints. Its replacement inbound receiver uses
those mapped ports when re-presenting bare ESP, just as its kernel state does.

A parallel responder handshake and the old owner's migration are serialized from
Child installation through publication. Once that handshake publishes
`pendingChild`, the retiring owner's COOKIE2 answer no longer moves Child policies:
their templates now belong to the Child that cleanup and promotion will retain.

The Linux capability uses the atomic `XFRM_MSG_MIGRATE_STATE` API. The factory
probes it before advertising `TunnelMigrator`; legacy `XFRM_MSG_MIGRATE` does not
provide the required locked replay-state transfer. An ordinary backend migration
error restores the old endpoints. A failed rollback removes both endpoint
candidates and reports `ErrTunnelMigrationLost`; failed deletions remain tracked
for teardown. The engine closes a MOBIKE IKE SA after either migration failure,
retiring the Child pair instead of reinstalling its keys. A NAT rebind without
MOBIKE (`rebindChildSAs`, ipsec-9 "The rebind") closes the IKE SA on
`ErrTunnelMigrationLost`, and for a transport-mode Child SA whose peer changed
address, which no migration or rekey can follow; after any other migration failure
the old endpoints stand, and it rekeys the Child SA onto the new endpoint instead.

The dataplane package is registered in the native QEMU integration pass.
`TestXFRMMigrationPreservesLiveKernelState` covers legacy, bitmap and ESN replay
state and lifetime counters; the rollback tests cover policy-write failure,
failed restoration and teardown retries. `TestXFRMMigrationKeepsOutboundSequenceOnPeerWire`
reads real ESP packets in a second namespace before and after movement.
`TestXFRMMigrationRekeyReceiverUsesMappedPorts` observes the production receiver's
UDP datagram after installing a Child with a translated peer port. A capability
skip is not migration evidence: these checks need the migration-capable kernel.

<!-- source: internal/component/ike/dataplane/dataplane.go -- TunnelMigration, TunnelMigrator, ErrTunnelMigrationLost -->
<!-- source: internal/component/ike/engine/mobike.go -- migrateMobikeChild, migrateMobikePair -->

## Inbound classification

`inbound.go` classifies inbound INFORMATIONAL and CREATE_CHILD_SA messages for
an established SA. The negotiation itself is driven by the owner loop, described
in `docs/architecture/ike/ipsec-13-rekey-wire.md`.

A CREATE_CHILD_SA request from the peer is classified by its payloads, in this
order (`handleCreateChildSAOwned`):

| The request carries | It is | Ze answers |
|---------------------|-------|------------|
| a REKEY_SA notify | a Child SA rekey (RFC 7296 Section 1.3.3) | `respondChildRekey` |
| TSi or TSr, no REKEY_SA | a new Child SA (Section 1.3.1), with or without KEi | `handleNewChildRequest`: NO_ADDITIONAL_SAS while the session holds its one Child SA (Section 1.3); on a childless IKE SA, `respondNewChild` creates it |
| KEi and no TS | an IKE SA rekey (Section 1.3.2, `{SA, Ni, KEi}`) | `respondIKERekey` |
| none of these | neither | NO_PROPOSAL_CHOSEN (Section 2.21.3) |

The TS payloads, never the KE payload, separate a new Child SA from an IKE SA
rekey, because a new Child SA with PFS carries KEi too.

On a childless IKE SA, in either IKE role, `respondNewChild` (`create_child.go`)
answers the request with `{SA, Nr, [KEr,] [N(USE_TRANSPORT_MODE),] TSi, TSr}`.
It reads the session's configured esp-group, never `sa.ESPGroup`, which holds
whatever IKE_AUTH left when its Child SA was refused. The esp-group's `pfs` leaf
decides whether KEi is required, as on the rekey path: a KEi in another group
draws INVALID_KE_PAYLOAD naming the IKE SA's group. The selectors are narrowed
with no floor, because the creation replaces no SA. The Child SA is keyed from
this exchange's Ni and Nr, plus the D-H secret when KEi was sent (Section 2.17),
with Ze as the responder of the exchange, whatever its IKE SA role. It is built
by `newChildSA`, the builder `createFirstChildSA` uses for the IKE_AUTH Child SA,
and installed before the answer is sent. The owner loop then starts its lifetime
and emits `child-up` and the route add, not a rekey event. A refusal answers the
error notify `notifyForRefusal` names and leaves the IKE SA up and childless
(Section 1.3.1, "A failed attempt to create a Child SA SHOULD NOT tear down the
IKE SA").

Only the IKE SA initiator asks for the Child SA it lacks, so two Ze peers never
collide on it. `runEstablished` starts `childCreateSchedule` when IKE_AUTH left
the initiator childless, and the owner tick calls `serviceChildCreate`: the first
request goes 30 s after establishment, and each refusal doubles the wait up to
300 s. The request is `{[N(USE_TRANSPORT_MODE),] SA, Ni, [KEi,] TSi, TSr}` from
the configured esp-group, with KEi in the IKE SA's group under `pfs`. No request
goes out while our IKE SA rekey is pending, while a peer IKE SA rekey awaits its
swap (Section 2.8), or while the request window is held. `finishChildCreate`
routes the answer: TEMPORARY_FAILURE waits 60 s, NO_ADDITIONAL_SAS re-establishes
the IKE SA, INVALID_KE_PAYLOAD naming a configured group retries at once in it,
and any other refusal keeps the IKE SA and backs off. A refused creation sets no
Child or IKE rekey hold. An answer this node refuses after the peer accepted it
(a missing KEr, for example) is followed by a Delete for the pair, and so is a
success that arrives after the peer created the session's Child SA meanwhile. The
schedule lives on the session, so an IKE SA rekey keeps it.

A Child SA rekey answer behaves the same way. An answer carrying an error
notification is the peer's refusal and is routed by the notification. An answer
carrying none means the peer installed the new pair, so when this node refuses it
(no Nr, a selector answer it will not adopt, an offer it never made), the rekey
response handler sends a Delete listing its own inbound SPI of the new pair (RFC
7296 Section 1.4.1), logs `childRekeyFixAnswer`, and keeps the Child SA in use.

A rekey keeps the mode of the Child SA it replaces, so it says which mode that is.
`initiateChildRekey` puts `N(USE_TRANSPORT_MODE)` after `N(REKEY_SA)` when the
retired SA runs transport mode (RFC 7296 Section 1.3.3: "when rekeying a transport
mode SA, the USE_TRANSPORT_MODE notification will be used"), and
`respondChildRekey` echoes it when the request carried it and the replacement runs
transport mode. A request without it asks for tunnel mode, and a peer behind a NAT
then skips its own Section 2.23.1 substitution: strongSwan 5.9.14 answered such a
rekey with TS_UNACCEPTABLE after a NAT rebind. An answer to a transport-mode rekey
that leaves the notification out established a tunnel-mode SA at the peer (Section
1.3.1), so `applyChildRekeyResponse` refuses it with
`errChildRekeyTransportDeclined`, and the handler deletes the new pair, holds the
next attempt for `rekeyRefusedWait` (the peer's configuration decides the answer,
so an immediate retry draws the same one), and logs `childRekeyFixTransport`.

<!-- source: internal/component/ike/engine/rekey.go -- initiateChildRekey, applyChildRekeyResponse, respondChildRekey, errChildRekeyTransportDeclined -->

<!-- source: internal/component/ike/engine/inbound.go -- inbound message classification, handleNewChildRequest, hasTSPayload, handleCreateChildSAOwned -->
<!-- source: internal/component/ike/engine/create_child.go -- respondNewChild, newChildSA, parseNewChildRequest, serviceChildCreate, finishChildCreate, childCreateSchedule -->
<!-- source: internal/component/ike/engine/delete.go -- Child SA teardown over INFORMATIONAL -->
<!-- source: internal/component/ike/engine/bypass.go -- IKE control-plane bypass policies -->

## INFORMATIONAL requests, the window and retransmission

`maintainSA` (`established.go`) is the owner loop of an established SA: one
goroutine, a one-second ticker, and sole ownership of `sa.NextMsgID`, the SK keys
and the request window. Every request Ze raises after IKE_AUTH is built there:
the DPD probe (`sendDPD`, `dpd.go`), the rekey (`startChildRekey`,
`startIKERekey`), the Delete (`delete.go`), the INVALID_MESSAGE_ID notify, MOBIKE
address updates and return-routability checks, and the padded path probe below.

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
| Delete, INVALID_MESSAGE_ID, MOBIKE, path probe | the window's slot, `armRequestRetransmit` then `serviceRequestRetransmit`: 3 repeats, 500 ms doubling to 60 s | `serviceRequestWindow`: `StateDead` once `requestWindowTimeout` (30 s) passes |

`serviceRequestWindow` has two exits and no third: a response, or `StateDead`.
It releases no window without a response and rewinds no message id. The loop's
`StateDead` arm then runs `cleanupChild` and returns `errSADeletedByPeer`, so
`PeerSession.run` reconnects.

## A rekey the peer refuses

A CREATE_CHILD_SA response that answers a rekey with an error notify carries the
notify alone. `rekeyRefusal` (`rekey.go`) reads it before either rekey path walks
the payloads, so the refusal is reported as itself and never as a missing Nr
(RFC 7296 Section 3.10.1). The soft lifetime is a level trigger: each answer
below that does not end the SA either changes what the next attempt sends or arms
a hold, or the one-second ticker would resend the refused rekey on every tick until
the hard lifetime.

| Answer | What ze does | Source |
|--------|--------------|--------|
| TEMPORARY_FAILURE | keeps the SA, holds the rekey for `temporaryFailureBackoff` (60 s); the path probe also waits | RFC 7296 Section 2.25 |
| NO_ADDITIONAL_SAS | re-establishes: the delete-and-create fallback | RFC 7296 Section 4 |
| CHILD_SA_NOT_FOUND, on a Child SA rekey | re-establishes, which deletes the Child SA and builds a new one | RFC 7296 Section 2.25 |
| INVALID_KE_PAYLOAD naming a group the operator configured for this peer (a DH group in its ike-group proposals) that the peer has not refused for this SA, on a request that carried KEi | keeps the SA, retries on the next tick with KEi in that group | RFC 7296 Section 1.3 |
| any other error notify, recognized or not (NO_PROPOSAL_CHOSEN, TS_UNACCEPTABLE, an unusable INVALID_KE_PAYLOAD, ...) | keeps the SA; every configured proposal is now refused, so the rekey waits `rekeyRefusedWait` (15 s less up to 10% jitter), then the record is cleared and the cycle restarts | RFC 7296 Sections 1.3.1 and 3.10.1 |
| a response with no notify that lacks a payload | retries on the next tick | |

Re-establishing is the owner loop's `reestablish` exit (`maintainSA`,
`established.go`). Besides the two refusals above (and NO_ADDITIONAL_SAS on a
Child SA creation), the same exit serves a peer's Delete of the live Child SA (the
session reconnects rather than staying up childless, owner decision Q-5 of the
childless IKE SA spec), a MOBIKE update that fails, and a rebind that cannot
follow the peer. Each path reaches it on an authenticated message the peer just
sent, so the peer is responding. Before the exit drops the IKE SA it sends a
Delete for it through `sayGoodbye`, because the peer still holds that IKE SA: RFC
7296 Section 2.4 says an endpoint closing an IKE SA "SHOULD send a Delete payload
indicating that it has closed the IKE SA unless the other endpoint is no longer
responding". When the message that raised the exit is itself the answer to our
outstanding request (a rekey response that arrived from a new endpoint and drew
the rebind exit), that answer frees the request window before the rebind runs,
so the Delete leaves at once and the closing SA applies nothing else from the
response. Otherwise `sayGoodbye` first waits up to `goodbyeWindowWait` (2 s) for
an outstanding request of ours, and sends nothing if no authenticated answer
arrives in that time. Without the Delete, strongSwan 6.x, which re-creates a
reloaded Child SA in place with CREATE_CHILD_SA, drew an unprotected
INVALID_IKE_SPI on an IKE SA it still believed in.

A request that deletes the IKE SA itself never takes this exit, even when it
also names the live Child SA: the peer has closed the IKE SA at both ends, so no
Delete is owed. `handleInformationalOwned` (`inbound.go`) clears the reestablish
outcome, and the `StateDead` arm ends the session with `errSADeletedByPeer`,
which reconnects without a goodbye. The SA remembers that the peer deleted it
(`peerDeletedIKE`), and `sayGoodbye` sends nothing for such an SA, so a datagram
queued behind the peer's Delete, or read while `sayGoodbye` waits, cannot raise
a goodbye for it either.

The one IKE SA Delete that does take the exit is the peer's Delete of the OLD
IKE SA after it rekeyed the IKE SA, when the same request also deletes the live
Child SA. The response is empty, as for every request that deletes an IKE SA
(RFC 7296 Section 1.4.1). The peer now holds only the new IKE SA (Section 2.8:
the old SA's Delete "MUST be the last request sent over the old IKE SA"), so the
goodbye goes out under the new SA, which then forgets its keys, because nothing
adopts it.

<!-- source: internal/component/ike/engine/established.go -- maintainSA, sayGoodbye -->
<!-- source: internal/component/ike/engine/inbound.go -- handleOwnedInbound, handleInformationalOwned -->

`rekeyRefusalRecord` (`rekey.go`), one per rekey kind on the SA, records what the
peer refused, and no refused group is offered again on that SA, so no answer can
loop. Ze sends every configured proposal in one SA payload, so a refusal that names
no group refuses all of them at once; the group of the KEi is the only part of an
offer ze can change and send again. Ze never picks a group the operator did not
configure, and a request that carried no KEi (PFS disabled) is not given one. A
successful rekey clears the record.

The waits are separate per kind (`childRekeyRefusedUntil`, `ikeRekeyRefusedUntil`
in `reconcile.go`), so a refused IKE SA rekey does not stop a Child SA rekey. A
refusal does not hold the path probe, because a peer that refused a proposal is not
mid-rekey.

<!-- source: internal/component/ike/engine/rekey.go -- rekeyRefusal, rekeyRefusedError, rekeyRefusalRecord, initiateChildRekey, initiateIKERekey -->
<!-- source: internal/component/ike/engine/inbound.go -- handleCreateChildSAOwned, refuseRekey -->
<!-- source: internal/component/ike/engine/established.go -- rekeyRefusedWait, startChildRekey, startIKERekey -->

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
