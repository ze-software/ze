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

**A tunnel-mode state with no selector carries `XFRM_STATE_AF_UNSPEC`.** A zero
selector is not "any family" to the kernel. `copy_from_user_state`
(`net/xfrm/xfrm_user.c`) runs `if (!x->sel.family && !(p->flags &
XFRM_STATE_AF_UNSPEC)) x->sel.family = p->family;`, so without the flag the state
takes the outer family. `xfrm_state_look_at` then skips it for a flow of the other
family (`x->sel.family != family`), and an IPv6 packet routed into an IPv4-outer
Child SA is dropped as `XfrmOutNoStates` while the SA reports up. RFC 4301 Section
5.1.2 says "IPsec allows the IP version of the encapsulating header to be different
from that of the inner header." With the flag the selector family stays
`AF_UNSPEC`, `__xfrm_init_state` fills the tunnel inner mode of both families, and
`xfrm_ip2inner_mode` picks one for each decapsulated packet. `xfrmStateFromParams`
sets the flag (`netlink.XfrmState.AFUnspec`, a field Ze's vendored-module patch
`internal/le/vendorpatch/patches/netlink-xfrm-07-state-af-unspec.patch` adds) only for tunnel
mode with a nil `SAParams.Sel`. Transport mode, the OSPF RFC 4552 states and any
state given a selector keep that selector and its family. What enters the state is
still decided by the policies: one per negotiated selector pair and direction,
bound to the Child SA's `if_id`. The outer pair stays one family, and
`xfrmStateFromParams` refuses a mixed pair (RFC 4301 Section 4.4.2.1). Readback
decodes the endpoints with the message family, never the selector's, so a flagged
state is still reported with its outer family.

<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmStateFromParams -->

**A dual-family proposal narrowed to one family on a VTI installs that family alone.**
A peer whose `traffic-selector` list holds an IPv4 entry and an IPv6 entry proposes
both, in list order (`proposeChildTSPayloads`), and records them as the ceiling in
`SA.ProposedChildPairs`. RFC 7296 Section 2.9: "When the responder chooses a subset
of the traffic proposed by the initiator, it narrows the Traffic Selectors to some
subset of the initiator's proposal (provided the set does not become the null set)."
A responder that keeps one family is such a subset, so `recordInitiatorSelectorsIn`
accepts it when each answered pair sits inside one proposed entry, and records only
the answered pairs in `SA.NegotiatedPairs`. `createFirstChildSA` builds the Child SA's
selectors from `NegotiatedPairs`, never from the proposal, so `childInstalledPolicies`
emits the in, forward and out policies of the answered family, each with the `if_id`
`resolveIfID` read from the bound xfrm interface. Nothing of the dropped family
reaches the kernel: a policy for it on the `if_id` would pull that family's traffic
into an interface whose SA does not carry it. The state pair keeps the outer family
and the flag above, so IPv6 inside an IPv4 outer pair finds its state.
`TestDualFamilyProposalNarrowedToIPv6InstallsVTIPolicy` drives this from the parsed
configuration to a fake dataplane, and the `vti-ipv6-inner-ipv4-outer` interop
scenario runs it against strongSwan.

<!-- source: internal/component/ike/engine/rekey.go -- proposeChildTSPayloads -->
<!-- source: internal/component/ike/engine/ts_narrow.go -- recordInitiatorSelectorsIn -->
<!-- source: internal/component/ike/engine/child.go -- createFirstChildSA, childInstalledPolicies -->

**Every negotiated selector pair gets its own policy, in each direction.** A peer
that answers several TSi/TSr pairs negotiated several values. `childInstalledPolicies`
builds inbound and outbound policies for each pair, plus a forward policy when the
backend declares `RequiresForwardPolicy`. Linux declares it for tunnel reception:
local delivery checks `IN`, while delivery to a protected host checks `FWD`.
The forward policy has the inbound selectors, ports, template, SA id, priority,
owner and `if_id`. VPP declares no separate forward lookup because its inbound SPD
applies before routing; it still refuses a caller's explicit `SADirFwd`.
Transport mode does not add a forward policy.

RFC 4301 Section 5.2 says: "Then match the packet against the inbound selectors
identified by the SAD entry to verify that the received packet is appropriate for
the SA via which it was received." Linux's separate transit lookup is part of
that check, not permission to bypass it. Missing it drops a decrypted tunnel
packet as `XfrmInNoPols`, even when both SAs and the protected routes exist.

`installChildSA` records the backend's requirement before claiming selectors.
The same complete policy set reaches installation, rollback, cross-peer ownership
and MOBIKE; incoming teardown removes both receive directions. A rekey keeps
shared policies until the last Child SA using their selectors retires.
`TestChildTunnelForwardsProtectedHostTraffic` exercises kernel forwarding after a
refused rekey, a deliberately removed forward policy and restoration; the
lifecycle and migration tests beside it cover ownership and cleanup.

RFC 7296 Section 2.8 says: "To rekey a Child SA within an existing IKE SA, create a
new, equivalent SA (see Section 2.17 below), and when the new one is established,
delete the old one." A failed replacement must not delete its predecessor's
shared policies. `newRekeyedChild` carries the predecessor only until the install
attempt consumes it, including selector refusal, unsupported and no-dataplane
exits. On policy failure, `rollbackChildPolicies` restores shared entries to the
old selectors, templates and owner, and removes only genuinely new entries.
An unsuccessful restoration is an explicit error and marks the predecessor
degraded without releasing its selector claim. Its initial-install ownership
stays immutable: a separate atomic restoration-failure fact feeds the concurrent
metrics reader. A subsequent refused rekey still restores that predecessor's
policies rather than treating them as unowned. The health failure stays latched
until a successful replacement publishes a new Child.

A new entry that rollback cannot remove remains in both the SPD ledger and the
stranded-selector registry for the existing removal retry. Re-establishment
adopts same-owner stranded entries only after installation succeeds.
`beginInstall` through `finishInstall` holds the registry's retry exclusion, but
not its map lock, across dataplane writes. A failed, unsupported or
control-plane-only attempt leaves the old stranded reservation retryable.
<!-- source: internal/component/ike/engine/child.go -- rollbackChildPolicies, installChildSA, installChildTolerant -->
<!-- source: internal/component/ike/engine/rekey.go -- newRekeyedChild -->
<!-- source: internal/component/ike/engine/inner_selectors.go -- beginInstall, finishInstall -->
<!-- source: internal/component/ike/engine/metrics.go -- espInstalled -->

XFRM resolves each policy to the state through its template, VPP through the SAD
id. A pair's two halves each carry a protocol, and a packet matches the pair only
when it matches both, so the policy carries their intersection: TSi "any" with
TSr TCP installs TCP.
<!-- source: internal/component/ike/engine/child.go -- childInstalledPolicies, childReceivePolicies -->
<!-- source: internal/component/ike/engine/child_forward_policy_linux_test.go -- TestChildTunnelForwardsProtectedHostTraffic -->

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
`recordInitiatorSelectorsIn` reads them through `answerSelectors`, never through the
responder's narrowing `wireToSelectors`. An answered address range that is not one
prefix becomes the exact set of prefixes tiling it (`rangePrefixes`), one pair each,
and an answer expanding past `answerPairsMax` (64) policies is refused. A port range
other than any port, one port or OPAQUE is refused: a negotiated pair installs one
policy per direction, whose selector the engine builds as every port or exactly one
(`selectorPort`), and VPP (`vppPortRange`) programs nothing else. XFRM
(`xfrmSelectorPort`) could carry a block aligned on a power of two, but
`tcp/1024-65535`, which strongSwan answers for such a policy, is six such blocks and
no single selector. A pair
neither dataplane can program (an OPAQUE port) refuses the whole answer rather than
being dropped. A refusal is TS_UNACCEPTABLE on IKE_AUTH, a deleted pair on a creation,
and a Delete for the new pair on a rekey. The responder installs what it answers
(`narrowChildSelectorsIn` sets `NegotiatedPairs`, which both `pairsToWire` and the
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
(`oneTransportPair`, `narrowChildSelectorsIn`), which RFC 7296 Section 2.9 permits
("the responder arbitrarily chooses one of them") and which keeps the initiator's
first choice the same section requires. The initiator refuses a transport-mode
answer of several pairs (`recordInitiatorSelectorsIn`, `errTSTransportPairs`), on
IKE_AUTH through its `recordInitiatorSelectors` wrapper and on every CREATE_CHILD_SA
and rekey directly: the IKE_AUTH path deletes the SA with TS_UNACCEPTABLE, a CREATE_CHILD_SA creation
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
<!-- source: internal/component/ike/engine/ts_narrow.go -- narrowChildSelectors, narrowChildSelectorsIn, oneTransportPair, recordInitiatorSelectors, recordInitiatorSelectorsIn, answerSelectors, answerPairs, rangePrefixes, crossPairs, closedSubset, pairsToWire -->
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
`docs/architecture/linux-limitations.md`). Ze records them as an ordered
series, one file per change, in `internal/le/vendorpatch/patches/netlink-xfrm-*.patch`,
applied in file-name order. Each file's leading text names the patches it needs.
`netlink-xfrm-90-ze-state-scope.patch` carries Ze's private kernel ABI, is never
proposed upstream, and sorts last:

- The state reader decodes `XFRMA_REPLAY_ESN_VAL`, restores the replay window,
  and reports the `XFRM_STATE_ESN` flag.
  Patch: `netlink-xfrm-01-state-readback-esn-replay.patch`.
- The state reader refuses a message shorter than `struct xfrm_usersa_info`
  before it reads the header. Patch: `netlink-xfrm-05-state-truncated-header.patch`.
- The state writer uses `XFRMA_REPLAY_ESN_VAL` for replay windows of more than 32. It
  sets `XFRM_STATE_ESN` only when the SA uses extended sequence numbers.
  Patch: `netlink-xfrm-02-state-replay-window-above-32.patch`.
- The policy writer copies the selector family into any template that carries no
  destination, which in ze is every transport-mode template. An explicit
  destination still determines the template family.
  Patch: `netlink-xfrm-03-template-family-no-destination.patch`.
- The selector writer takes its family from a valid destination prefix's mask
  width, or from the source prefix's when the selector has no destination, and
  preserves all sixteen address bytes for IPv6, including mapped IPv6
  prefixes. Policies and a state's legacy selector use the same writer,
  so a transport selector can match its canonical state-scope record exactly.
  Patch: `netlink-xfrm-10-selector-prefix-family.patch`.
- The policy reader preserves source and destination port masks, including partial
  masks and an exact port zero. The writer sends a non-zero mask as given and
  derives the mask from the port value only when the mask is zero, so Ze can
  install an exact ICMP type or code of 0 and a mask-aligned ICMP code block
  (RFC 4301 Section 4.4.1.1, `xfrmSelectorPort`).
  Patch: `netlink-xfrm-06-selector-port-masks.patch`.
- The policy reader decodes selectors using each message's selector family.
  An all-family dump keeps `0.0.0.0/0` and `::/0` distinct, and IPv6 prefixes
  whose trailing bytes are zero retain their IPv6 identity. Tunnel endpoints use
  their template's family, which can differ from the selector family.
  Patch: `netlink-xfrm-04-readback-message-family.patch`.
- The state reader uses the SAD message family for both outer addresses and the
  nested selector's family for its prefixes. The same address conversion serves
  the policy reader, so `2001:db8::` remains distinct from `32.1.13.184` even when
  SPI, protocol and interface id agree. Patch: `netlink-xfrm-04-readback-message-family.patch`.
  Nested selectors retain raw port masks. Patch: `netlink-xfrm-06-selector-port-masks.patch`.
- `XfrmState` gains `AFUnspec`. The state writer sets `XFRM_STATE_AF_UNSPEC` in
  `xfrm_usersa_info.flags` when it is true, and the state reader reports the flag.
  Upstream sets no flag but `XFRM_STATE_ESN`, so a tunnel-mode state with no
  selector could not be stored as serving both inner families (see "A tunnel-mode
  state with no selector carries `XFRM_STATE_AF_UNSPEC`" above).
  Patch: `netlink-xfrm-07-state-af-unspec.patch`.
- `XfrmState` gains `NoPMTUDisc`. The state writer sets `XFRM_STATE_NOPMTUDISC`
  when it is true, and the state reader reports the flag. It is the only way to
  ask the kernel for a clear DF bit in the tunnel header (see "The outer header
  DF bit" below).
  Patch: `netlink-xfrm-08-state-no-pmtu-disc.patch`.
- `XfrmState` gains `ICMP` and `XfrmPolicy` gains `ICMP`. The writers set
  `XFRM_STATE_ICMP` and `XFRM_POLICY_ICMP` when they are true, and the readers
  report them (see "ICMP error messages carried by a Child SA" below).
  Patch: `netlink-xfrm-09-icmp-flag.patch`.
- `XfrmState` gains `StateScope`, an ordered list of 1..64 canonical IPv4 or
  IPv6 selectors and a negotiated noninitial-fragment flag. Its writer and
  reader validate and preserve every field. `XfrmZECapabilitiesGet` reads the
  live namespace's GETDEFAULT receipt and refuses missing, duplicate,
  malformed or foreign receipts. Individual feature bits let callers require
  the kernel behavior their request needs before installation.
  Patch: `netlink-xfrm-90-ze-state-scope.patch`.
<!-- source: internal/le/vendorpatch/patches/ -- netlink-xfrm-*.patch, XFRM state and policy decoding corrections -->

`TestXFRMReadbackPolicyTunnelAddressFamily` installs an IPv4 policy with IPv6
tunnel endpoints whose trailing bytes are zero, including `2001:db8::`.
It checks both endpoints through the backend and checks that removal clears the
policy from the kernel dump.
<!-- source: internal/component/ike/dataplane/xfrm_readback_integration_linux_test.go -- TestXFRMReadbackPolicyTunnelAddressFamily -->

Keep the patches and their drift test outside `vendor/` because `go mod vendor`
replaces that directory. After every vendor refresh, run this command from the
repository root. It vendors the modules again and applies every patch under
`internal/le/vendorpatch/patches/` in file-name order:

```sh
./le setup install
```

`TestNetlinkXFRMPatchApplied` uses `git apply --reverse --check` to require the
exact corrections with their context lines, in a subtest named after each file
of the XFRM series under `internal/le/vendorpatch/patches/`. It lives in
`internal/le/vendorpatch`; run it alone with
`go test ./internal/le/vendorpatch -run TestNetlinkXFRMPatchApplied`.

## The outer header DF bit

RFC 4301 Section 8.1: "This means that it MUST be possible to configure the
implementation's treatment of the DF bit (set, clear, copy from inner header)
for each SA.  This applies to SAs where both inner and outer headers are IPv4."
The peer's `df-bit` leaf (`ipsec.SiteToSitePeer.DFBit`) is that setting. The
engine copies it onto the Child SA (`ChildSA.DFBit`, inherited by a rekey) and
`installChildDataplane` sets `SAParams.DF` on the outbound tunnel-mode state
only: the inbound state decapsulates and emits no header, and a transport-mode
SA builds none.

`dataplane.DFTreatment` has three values, and its zero is `DFCopy`, the option
Section 8.1 makes mandatory and the kernel's default. A caller that states
nothing, the RFC 4552 OSPFv3 installer included, builds the state it built
before. `outerHeaderTreatmentError` refuses a treatment other than copy on a
transport-mode SA, an inbound SA or an SA whose outer header is IPv6, on every
backend.

| Treatment | Linux XFRM (`xfrmNoPMTUDisc`) | VPP (`vppTunnelFlags`) |
|-----------|-------------------------------|-------------------------|
| copy | no flag: `xfrm4_tunnel_encap_add` copies `XFRM_MODE_SKB_CB(skb)->frag_off & htons(IP_DF)` | `ENCAP_COPY_DF`, read by `tunnel_encap_fixup_4o4_w_chksum` |
| clear | `XFRM_STATE_NOPMTUDISC`: the same expression writes 0 | no flag: `tunnel_build_v4_hdr` never writes DF |
| set | refused: no XFRM flag or attribute sets it (`XFRMA_IPTFS_DONT_FRAG` is IP-TFS only) | refused: `ENCAP_SET_DF` is read by the ipip plugin alone, never by `esp_encrypt` |

Before this, no VPP SA carried `ENCAP_COPY_DF`, so every VPP tunnel cleared
DF. The zero value now copies on both backends.

Commit refuses `df-bit set` and a non-copy treatment on a transport-mode or
IPv6-addressed peer (`ValidateOuterHeader`, `internal/component/ike/ipsec/outer_header.go`),
so the backend refusals are defence in depth. Setting DF on Linux needs a
mechanism the kernel lacks; the choice between rewriting the ESP packet after
the transform and a kernel patch is open in
`plan/release/fixes/spec-rfc4301-architecture-gaps.md`.
<!-- source: internal/component/ike/dataplane/outer_header.go -- DFTreatment, outerHeaderTreatmentError -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmNoPMTUDisc -->
<!-- source: internal/component/ike/dataplane/vpp.go -- vppTunnelFlags -->
<!-- source: internal/component/ike/engine/child.go -- installChildDataplane -->

### The outer header DSCP

RFC 4301 Section 5.1.2.1 note 5: "If the packet will immediately enter a domain
for which the DSCP value in the outer header is not appropriate, that value MUST
be mapped to an appropriate value for the domain". The peer's `dscp-map` leaf
(`SiteToSitePeer.OuterDSCP`) reaches `SAParams.OuterDSCP` on the outbound
tunnel-mode state the same way as `df-bit`. `dataplane.OuterDSCP` is copy (zero)
or `0x80 | dscp` built by `FixedOuterDSCP`.

| Treatment | Linux XFRM (`xfrmDontEncapDSCP`) | VPP (`vppTunnelFlags`, `vppTunnelDSCP`) |
|-----------|----------------------------------|------------------------------------------|
| copy | no flag: `xfrm4_tunnel_encap_add` writes `XFRM_MODE_SKB_CB(skb)->tos` | `ENCAP_COPY_DSCP` |
| 0 | `XFRM_SA_XFLAG_DONT_ENCAP_DSCP`: the outer DS field is written 0, ECN still encapsulated | `Dscp` 0, no copy flag |
| 1 to 63 | refused: no flag or attribute writes another value | `Dscp` set: `tunnel_build_v4_hdr` writes `t_dscp << 2` |

Before this, no VPP SA carried `ENCAP_COPY_DSCP`, so every VPP tunnel wrote
DSCP 0 into its outer header, against the table's "copied from inner hdr".
<!-- source: internal/component/ike/dataplane/outer_header.go -- OuterDSCP, outerDSCPError -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmDontEncapDSCP -->
<!-- source: internal/component/ike/dataplane/vpp.go -- vppTunnelFlags, vppTunnelDSCP -->

## ICMP error messages carried by a Child SA

RFC 4301 Section 6.2 asks two things of an ICMP error carried by an SA: an
inbound one whose own headers miss the SA's selectors is checked against the
header it embeds, reversed, and dropped if that header does not match; an
outbound one no SA or SPD entry carries goes through the SA of the return
traffic of the packet it reports. The peer leaf `icmp-error-check` (default
true, `ipsec.SiteToSitePeer.SkipICMPErrorCheck`) turns both on. The engine sets
`SAParams.ICMPErrorCheck` on the inbound state and `SPParams.ICMPErrorReturn` on
each outbound policy (`installChildSA`, `childPairPolicy`).

The Linux backend lowers them to `XFRM_STATE_ICMP` and `XFRM_POLICY_ICMP`.
With the state flag, `icmp_rcv` and `icmpv6_rcv` answer a failed inbound policy
check by checking again on the reversed embedded header, which then meets the
Child SA's inbound protect entry or the discard catch-all. With the policy
flag, `icmp_route_lookup` and `icmpv6_route_lookup` may send an error the node
generates through the outbound policy its reversed header matches; without it
`xfrm_lookup_with_ifid` answers `-ENOENT` to that lookup. Linux refuses the
state flag on an SA flagged outbound, and the backend refuses either flag where
the kernel never reads it.

A FORWARDED ICMP error that arrives on a Child SA is not covered: the kernel
reads its payload only when no fwd policy matches its own headers, and Ze's
catch-all always does (LL-XFRM-ICMP-TRANSIT in
`docs/architecture/linux-limitations.md`). Such an error is dropped, so nothing
inconsistent is forwarded, but a consistent one is lost too. A forwarded error
leaving towards a Child SA is mapped to the return SA (`xfrm_out_fwd_icmp`,
called by `__xfrm_route_forward`) only after the out lookup passed it
untransformed: under `unmatched bypass` the catch-all does; under
`unmatched discard` it blocks the error first. The VPP backend reads no ICMP payload on either path and refuses
both requests (`vppUnsupportedSA`, `vppICMPErrorReturnError`).

Both flags override an operator SPD entry, so config verify refuses the pair
rather than install an entry the kernel does not honour (`verifyICMPOverlap`,
`icmp_overlap.go`, the last step of `validateIPsecSections`, the offline
verifier that `ze config validate`, every commit door and `OnConfigVerify` run).
`icmp_route_lookup` maps an error whose normal lookup answered the plain route,
and a template-free allow policy (a BYPASS entry) answers that route as no
policy does, so `refuseICMPErrorBypass` refuses a peer with the check on beside
`unmatched bypass`, or beside a bypass entry that is out or both, protocol 0 or
the family's ICMP protocol, ICMP type any or an error type the kernel sends
(`icmpErrorTypesV4`, `icmpErrorTypesV6`, which name their kernel callers),
and whose remote prefix overlaps the peer's remote selectors. `icmp_rcv`
delivers what an inbound discard entry refused whenever the reversed embedded
header matches the SA, whatever the type, so `refuseICMPCheckDiscard` refuses
an in or both discard entry matching ICMP in a family the peer's selectors
carry. Both are conservative: neither narrows on the entry's local prefix or
order, because the Ze address of the ICMP message is unknown at commit. An
entry below the peer's protect policy is overridden for every triggering
packet whose Ze address is outside the local selectors; a protocol-0 bypass
above it may not be overridden for every triggering packet, but it is for
some.

A `vti bind` peer is skipped by both. Its policies carry the xfrm interface's
if_id, and an operator entry if_id 0. The reverse lookup of
`icmp_route_lookup` and `icmpv6_route_lookup` is `xfrm_lookup`, which is
`xfrm_lookup_with_ifid` with if_id 0, and `xfrm_policy_match` refuses a policy
whose if_id differs, so that peer's out policy is never the mapping target;
ICMP decrypted on the interface is policy-checked at its if_id
(`xfrmi_decode_session`), where no operator entry is consulted.

The start is never verified, so `warnICMPOverlapAtStart` logs the same refusal
at Error level under `config-plugin-verify`, the code under which the
`config-semantics` doctor check reports it: that check runs every plugin's
offline verifier over the running configuration, so the IPsec plugin carries
no doctor check of its own for it.

<!-- source: internal/component/ike/engine/icmp_overlap.go -- refuseICMPErrorBypass, refuseICMPCheckDiscard, verifyICMPOverlap, warnICMPOverlapAtStart -->
<!-- source: internal/component/ike/engine/config.go -- validateIPsecSections -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmStateFromParams, xfrmPolicyFromParams -->
<!-- source: internal/component/ike/engine/child.go -- childPairPolicy -->
<!-- source: internal/component/ike/dataplane/vpp_policy.go -- vppICMPErrorReturnError -->

## Path MTU of a Child SA

RFC 4301 Section 8.2.1 maps a PMTU message to its SA and updates "the SAD PMTU
field, taking into account the size of the AH or ESP header that will be
applied, any crypto synchronization data, and the overhead imposed by an
additional IP header, in the case of a tunnel mode SA." Ze holds no PMTU field
of its own: Linux keeps it on the SA's outer route and derives the inner one
per packet.

| Step | Linux function |
|------|----------------|
| Map an ICMP Fragmentation Needed or Packet Too Big about an ESP packet to the SA by SPI, and lower the outer route's PMTU | `esp4_err`, `esp6_err`, `xfrmi4_err`, `xfrmi6_err`, then `ipv4_update_pmtu` / `ip6_update_pmtu` |
| Derive the inner MTU: outer PMTU less header_len (ESP header, IV, outer IP header), ICV, block alignment and the 2-octet trailer | `xfrm_bundle_ok`, `xfrm_state_mtu` |
| Case 1 and 3: discard and send a PMTU ICMP carrying the inner MTU | `xfrmi_xmit2` (route-based), `ip_forward` / `ip6_forward` against the bundle MTU (policy-based), `xfrm4_tunnel_check_size` / `xfrm6_tunnel_check_size` for local traffic |
| Case 2: DF clear, encrypt, then fragment the ESP packet | `ip_finish_output` on the outer route |
| Section 8.2.2 aging: the exception expires after `mtu_expires` and the route's MTU is the device MTU again | `__ip_rt_update_pmtu`, `ipv4_mtu`; `rt6_do_update_pmtu`, `rt6_check_expired` |

The state Ze installs decides two inputs: the overhead (`xfrmStateFromParams`
writes the ESP transform and ICV length) and whether a PMTU is learned at all
(`xfrmNoPMTUDisc`: with `df-bit clear` an IPv4 tunnel header carries no DF, so
routers fragment it and return nothing). The aging period is
`net.ipv4.route.mtu_expires` and `net.ipv6.route.mtu_expires`, registered as
known keys with the range 1 to 2147483 in `internal/core/sysctl/known_linux.go`
and set through `sysctl { setting ... }`.

`rfc4301_pmtu_integration_linux_test.go` proves the three cases, the overhead
and the IPv4 aging between namespaces, for a route-based and a policy-based
SA and for both outer families. The IPv6 sysctl table is not exported to a
network namespace another user namespace owns (`ipv6_route_sysctl_table_size`),
so the IPv6 aging leg is `rfc4301_pmtu_ipv6_integration_linux_test.go`, tagged
`integration`, which runs as root in the QEMU guest.

VPP has no equivalent. `esp_encrypt.c` holds no path MTU, `icmp4.c` acts on no
Fragmentation Needed, and `ip_path_mtu.c` is a configured per-destination value
with no learning and no aging, so over VPP the SA's PMTU is the egress
interface MTU.
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- xfrmNoPMTUDisc, xfrmStateFromParams -->
<!-- source: internal/core/sysctl/known_linux.go -- init -->

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
| BYPASS | `ikeBypassPolicies`, the operator `vpn ipsec policy` list, `unmatchedPolicies` under `vpn ipsec unmatched bypass`, and `xfrmInterfaceExemptions` under `vpn ipsec unmatched discard` | `allow` with no template | `IPSEC_API_SPD_ACTION_BYPASS` |
| DISCARD | the operator `vpn ipsec policy` list, `unmatchedPolicies` under `vpn ipsec unmatched discard`, and `interfaceSPDCatchAll` for every bound XFRM interface under either value | `block` | `IPSEC_API_SPD_ACTION_DISCARD` |

The template-free XFRM action mapper is called only after `isTemplateFree`
selects BYPASS or DISCARD. An unnamed action at that internal mapper raises a
`BUG` panic; an explicit PROTECT input still returns its existing unsupported
operation error because it belongs to the template path.
The VPP mapper receives actions from the same local policy builders or checked
operator policy parsing. It also asserts `BUG` for an unnamed internal action
rather than choosing a VPP disposition.

MOBIKE migration policies come from the same complete Child SA policy set.
Inbound and forward templates move from remote to local; outbound templates move
from local to remote. The transaction retains each direction's owner and restores
all three on rollback. An unnamed internal direction raises a `BUG` panic before
migration proceeds.

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

The catch-all sits at if_id 0, and a `vti bind` peer's Child SA policies sit at
its XFRM interface's if_id (`resolveIfID`). Linux keeps one database per if_id
(RFC 4301 Section 4.4.1's multiple SPDs) and looks a packet up at if_id 0 before
it reaches the interface: IPv6 host output in `ip6_dst_lookup_flow`, with the
flow's oif still 0; IPv4 host output with `flowi4_oif` already the route's
device; forwarded traffic in fwd and out with the oif from the route
(`decode_session4`, `decode_session6`). Only `xfrmi_xmit` looks it up again at
the interface's if_id. So under a DISCARD catch-all, `xfrmInterfaceExemptions`
(`engine/unmatched_xfrm_interface.go`) is the SPD selection function the section
requires of a multi-SPD implementation: for each bound interface, BYPASS entries
at if_id 0 and `PriorityUnmatchedXFRMInterface` (one above the catch-all, below
every operator order and Child SA policy), out and fwd for both families with
the selector ifindex set to the interface, and out from each IPv6 address it
holds.

Each bound interface's SPD also gets its own final entry (RFC 4301 Sections
4.4.1 and 5.2): `interfaceSPDCatchAll` builds the same wildcard in, out and fwd
for both families as `unmatchedPolicies`, through the shared builder
`spdCatchAllEntries`, at the interface's if_id and `PriorityUnmatched`, below
the interface's Child SA policies at `PriorityChildSA`. It is DISCARD under both
values of `unmatched`: an XFRM interface has no cleartext path, since
`xfrmi_xmit` drops a packet whose lookup returns no state and
`__xfrm_policy_check` rejects a tunnel-mode packet a template-free entry matched,
so a BYPASS there would be listed as bypass while the kernel drops. The if_id
comes from `boundInterfaceIfID`, the resolution `resolveIfID` uses for the Child
SA policies, which refuses 0 (the node-wide SPD, and what a metadata-mode
interface reports). A backend whose `CatchAllSupported` answers
`dataplane.ErrNoNodeWideSPD` (VPP) keeps no if_id SPD and gets no interface
catch-all.

`planXFRMInterfaceEntries` reads the interface through the iface resolver
(`iface.ResolveIndex`, `iface.Addresses`) and its if_id, and answers each entry
with its `dataplane.PolicyRole`: `interface-catch-all` always,
`interface-exemption` only under DISCARD. It keeps the held entries of an
interface whose read fails, the exemptions only while the catch-all still
discards. `installXFRMInterfaceEntries` installs by role, the interface
catch-all before the exemptions, and `pruneXFRMInterfaceEntries` removes in the
reverse order. An interface catch-all the backend refuses is an error
`installXFRMInterfaceEntries` returns, and `applyConfig` returns it as it does
`installUnmatched`'s: the interface's SPD would have no final entry, and
`__xfrm_policy_check` accepts an unmatched inbound packet whose secpath holds
only transport-mode or offloaded states. No exemption of that interface is
installed in the same pass, so nothing is handed to the SPD that lacks its
final entry. The address worker has no commit to refuse, so it logs the error,
and the next apply retries. `applyConfig` installs new entries before re-asserting the node
catch-all on a reload and prunes stale ones after it; the address worker
(`followAddress`, on `addr-added`, `addr-removed` and `created`) rebuilds them,
and `releaseSPD` removes them through the SPD ledger after the Child SA policies
(`stopAllPeers` ran first) and before the node catch-all.

The IPv6 source entry would also pass a packet from that address routed out of
another interface, so under DISCARD each source address gets a tunnel-source
guard (`source_guard.go`), and `installXFRMInterfaceEntries` installs a source
entry only for an address `sourceGuards.install` answers as guarded. The guard
is three kernel objects: the nft table `ze_ipsec_source_guard` (registered with
`firewall.RegisterTables`, one postrouting chain, per address a pass term for
its own device, a pass term for a packet entering an IPsec transform
(`firewall.MatchIPsecTransform`, nft `rt ipsec exists`) and a drop term for any
other device), one `iprule.Source` rule per
address at `iprule.IPsecSourceGuardPriority`, and one table per interface from
`iprule.IPsecSourceGuardTables` holding `unreachable ::/0` and a device route
for every IPv6 remote selector of a Child SA at the interface's if_id, from
every negotiated pair and not only the first that `TSRemote` holds
(`childRemotePrefixes`; `guardTunnelRoute`, called beside `emitRouteAdd` and `emitRouteRemove`; routes
carry `rtproto.IPsecSourceGuard`). Install runs the nft table, then routes,
then rules, between the interface catch-alls and the exemptions; prune runs
after the exemptions and before the interface catch-alls, so no source BYPASS
is ever live without its guard. The rule steers a packet whose socket chose the
tunnel source (`ip6_dst_lookup_tail` looks the route up with the source set);
when the kernel picks the source it keeps the route found with none, and the
nft drop stops it. A packet routed into the interface meets postrouting a second
time in `xfrm6_output`, on its way into the Child SA, with the bundle's outer
device (`eth0`) as its output device; the transform term passes it, because it
leaves encrypted. Without that term the drop took every IPv6 packet from the
tunnel address into the tunnel: `vti0` counted a transmit error with no carrier
error and the outbound SA encrypted nothing. A guard address that is also a peer's IKE local address is
skipped with a warning, and the same configuration is refused at verify
(`refuseTunnelAddressAsIKEEndpoint`).

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
<!-- source: internal/component/ike/engine/unmatched_xfrm_interface.go -- xfrmInterfaceExemptions, interfaceSPDCatchAll, planXFRMInterfaceEntries -->

## Both ESP forms on one inbound state

RFC 7296 Section 2.23 says "all devices MUST be able to receive and process
both UDP-encapsulated ESP and non-UDP-encapsulated ESP packets at any time."
One XFRM state receives one form (`LL-XFRM-STATE-ONE-ESP-FORM` in
[Linux limitations](../linux-limitations.md#ll-xfrm-state-one-esp-form)).
`installChildDataplane` sets `SAParams.AcceptBothESPForms` on every inbound
Child SA state. For one that also carries the ESP-in-UDP template
(`UDPEncap`), `InstallSA` watches its SPI (`espFormReceiver.Watch`) with the
peer and local address and the template's ports in one call, so the first
datagram the receiver re-presents already carries the ports the state matches;
`RemoveSA` forgets it, and `MigrateTunnel` retargets it after a MOBIKE move. A
template-free state receives bare ESP itself and is never watched.

On a distribution kernel, a loaded `esp4_offload` module can consume bare ESP
in `esp4_gro_receive` before tc ingress and raw socket delivery. It calls
`xfrm_input` with the bare form, which an ESP-in-UDP state refuses, so neither
of Ze's receive paths sees that packet. The Linux-only runtime check
`ipsec-esp-form-offload` reads the receiver's state and, while it watches an
SPI, the shared `kernelcap.LoadedModules` snapshot. A loaded module is a
`doctor-ipsec-esp-form-offload` warning; an unreadable receiver or module list
is `doctor-ipsec-esp-form-offload-unknown`, never module absence. An idle
receiver reports neither. The check only reads and never unloads a module.

The loaded-module fix stops Ze, unloads the module, then starts Ze. The stop
removes Ze's Child SAs: `xfrm_set_type_offload` takes a module reference for an
ESP state even when that state requested no hardware offload, so unloading a
module while those SAs live can fail. Recreating Ze's SAs does not autoload the
module: `xfrm_state_construct` asks `xfrm_set_type_offload` to load it only for
`XFRMA_OFFLOAD_DEV`, which Ze's state serializer does not send. Another IPsec
owner can still hold or load it; the finding names that condition and the
existing per-interface `offload gro false` setting as the alternative when
that owner needs the module. A readable module list cannot detect ESP GRO
built into the kernel. That packaging is outside this loaded-module check's
proof; Ze's runtime kernel does not enable `CONFIG_INET_ESP_OFFLOAD`.
<!-- source: internal/component/ike/engine/register_esp_form_offload_linux.go -- init -->
<!-- source: internal/component/ike/engine/doctor_esp_form_offload_linux.go -- answerESPFormOffload, espFormOffloadModuleFindings -->
<!-- source: internal/component/kernelcap/modules_linux.go -- LoadedModules -->
<!-- source: internal/component/ike/engine/apply.go -- shutdown, stopAllPeers -->
<!-- source: internal/component/ike/engine/established.go -- cleanupChild -->
<!-- source: vendor/github.com/vishvananda/netlink/xfrm_state_linux.go -- xfrmStateAddOrUpdate -->
<!-- source: internal/component/iface/offload_linux.go -- applyOffloads -->
<!-- test: internal/component/ike/engine/doctor_esp_form_offload_linux_test.go TestDoctorESPFormOffloadLoadedModuleWarns -->
<!-- test: internal/component/ike/engine/doctor_esp_form_offload_linux_test.go TestDoctorESPFormOffloadUnreadableModulesWarns -->

The receiver names a watched SA by its SPI and its local address, the pair XFRM
keys an inbound state on (`__xfrm_state_lookup`). Ze draws each inbound SPI at
random and checks it against no SPI in use (`generateESPSPI`), so two Child SAs
on two local addresses can carry one SPI, and the kernel installs both. The
registry holds both, and the complete path looks a datagram up by its SPI and
its destination address, so each SA's bare ESP reaches its own target and a
datagram sent to an address no watched SA holds is dropped. `RemoveSA` forgets
only the SA on the state's destination, so removing an outbound SA whose
peer-chosen SPI equals a watched inbound SPI leaves the inbound SA served. The
fast path's map holds one target per SPI: it keeps the first SA watched, and the
program rewrites only a datagram from that SA's peer to its local address, so
the second SA's bare ESP takes the complete path, rate-bounded. While the fast
path is attached, Watch logs both local addresses at Warn under
`doctor-ipsec-esp-form-fast-path`, with `ze show vpn ipsec sa` as its fix and the
message saying that clearing either peer whose Child SA carries the SPI ends the
collision. A fast path that is unavailable or detached serves no SA, so the
collision costs nothing more there and Watch does not warn of it. When the
first SA is forgotten, the second takes the fast path over.

A MOBIKE move onto a local address another SA on the same SPI holds is refused
before the SAD is touched. `espFormRegistry.retarget` holds the one check of
that collision and answers why it refused (`errESPFormLocalTaken`, or
`errESPFormNotWatched` for an SA it does not watch). `MigrateTunnel` asks the
same check first, through `admitsMove`, and answers an operator error whose fix
is `ze show vpn ipsec sa`. A watch replaces an SA already on the same SPI and
local address rather than refusing it, so for an SA the receiver does not watch
yet this check is the only thing that keeps the move from taking another SA's
watch.

`MigrateTunnel` always templates the moved inbound state, so after the move the
kernel refuses its bare ESP and the receiver is the only server of that form.
For an SA the receiver does not watch yet, `MigrateTunnel` therefore opens the
receiver's paths, the raw ESP socket and the `ze-espform` device, and holds
them (`espFormReceiver.acquire`) before it touches the SAD. When they cannot
open, the move is refused with nothing changed: no state moves, the registry is
untouched, and the tunnel stays on its old address. The refusal is an operator
error (`xfrmMigrationUnserved`) that names the SPI and both local addresses and
wraps the cause, and its fix is the cause's own. The cause renders as its
problem and its own cause, never its fix line (`espFormRefusalCause`), so the
refusal shows one fix. Every open failure the host can produce is an operator
error with its fix (`openESPFormPaths`): the raw socket's open and its policy
exemption answer the capability install for EPERM (`espFormSocketRefusal`,
`espFormBypassRefusal`), the journal's SELinux and AppArmor denials for EACCES,
which a security module gives and no capability changes
(`espFormSecurityModuleRefusal`), the unit install that sets `LimitNOFILE` for
EMFILE, the `fs.file-max` write for ENFILE, `free -m` for ENOMEM and ENOBUFS, and
the receiver's warnings for any other answer, which Ze retries by itself
(`espFormOpenFailure`). The TUN open names its fix, and each later step of the
device's configuration answers the capability install for EPERM and the
receiver's warnings otherwise, because the name is Ze's after TUNSETIFF and the
device is gone again (`espFormTUNStepRefusal`). A sysctl write that fails for a
reason other than a read-only `/proc/sys` answers the `sysctl -w` of
`net.ipv4.conf.default` (`espFormTUNSysctlWriteRefusal`). A cause that is no operator
error is a Ze defect, such as a receiver the backend was built without, and
passes through with the move's context and no fix line. This is the same
refusal class `InstallSA` gives an SA whose bare form nothing would serve: it
wraps the same operator error, whose fix the engine reads
(`childInstallRefusedFix`).

The receiver records its last failure to open its paths, the error and its
time, under its lock, and clears the record on the next open that succeeds.
`ESPFormFastPathState.CompletePathFailure` carries it to the doctor, which reads
it and opens nothing: one `doctor-ipsec-esp-form-tun` error naming the cause
and saying that no Child SA that must receive both forms can be installed, or
moved onto the ESP-in-UDP form, until the path opens
(`espFormCompletePathFindings`). Only such a move acquires the paths: a watched
SA's move retargets the registry and opens nothing. It is the TUN code and not
the fast path's because its subject is the complete path, the socket and the
device the TUN finding names, and its consequence is that finding's, a refused
SA, which no fast path finding has. When the TUN record of the last apply has
the same cause, the check reports the receiver's finding alone
(`espFormDeviceFindings`, `espFormSameCause`). Every doctor finding calls the
complete path Ze's own receiver for bare ESP, including the fast-path findings
that explain the rate bound and the input interface the firewall sees.

A reader whose socket fails ends while the SAs it served stay in the SAD, so
its bare form is owed and unserved. Each reader has one revive worker
(`espFormReceiver.revive`), started with it, which wakes on that failure, logs
it at Warn under `doctor-ipsec-esp-form-reader-stopped`, hands it to the sink
the engine registered (`RegisterESPFormReaderStopped`, `reportReaderStopped`),
and opens the paths again until an open succeeds or the paths are stopped. The
stop is transient, and the first reopen often succeeds, leaving no record for a
check to read, so the engine raises it on the report bus as an error event under
that code, a registered event code, through its own `report-issue` request
(`espFormReaderStoppedSink`): the dataplane runs in the engine's process, in-process
or forked, and holds no route to the daemon's bus. The wait before each attempt
is the receiver's, not one reader's (`reopenWait`): it doubles from one second
up to `ESPFormCompletePathReopenMax` (30 seconds) at each attempt and returns to
zero only at the stop of a reader that served `espFormReopenResetAfter` (a
minute) (`reopenWaitLocked`, `reviveLocked`), so a socket that fails right after
each open is not reopened in a tight loop. After `Close`, a Watch or acquire
refuses with `errESPFormReceiverClosed` and opens nothing. A Watch or acquire that meets the
stopped reader first (`servingLocked`) opens the paths itself
(`reopenReaderLocked`), which closes the failed socket and the device before it
opens new ones, because two `ze-espform` devices cannot exist; when that open
fails, it refuses with the open's error and the revive worker keeps trying. The
reader takes no lock, so its socket's close joins it under the receiver's lock
(`closeReaderLocked`); a revive worker takes the lock for each attempt, so
`Close` joins the workers after it released the lock.

The revive worker can raise its event while the plugin sends another RPC.
`sdk.Plugin.RaiseIssue` builds a request local to the call; the pipe path
multiplexes each request separately and serializes its framed write, and the
direct bridge admits concurrent dispatches. The daemon serializes the report
rate check and the error ring's append.
<!-- source: pkg/plugin/sdk/report.go -- RaiseIssue, reportCall -->
<!-- source: pkg/plugin/sdk/sdk.go -- callEngineRaw -->
<!-- source: pkg/plugin/rpc/mux.go -- CallRPC -->
<!-- source: pkg/plugin/rpc/conn.go -- writeFrame -->
<!-- source: pkg/plugin/rpc/bridge.go -- DispatchRPC, beginDispatch -->
<!-- source: internal/component/plugin/server/dispatch_report.go -- opReportIssue, admit -->
<!-- source: internal/core/report/report.go -- raiseError -->

The receiver's registry follows the SAD and never leads it: `MigrateTunnel`
moves the states first, while the receiver keeps re-presenting bare ESP on the
old address where the kernel still holds the SA, and only then retargets a
watched SA, or registers an SA that was not watched on its new address on the
paths it holds (`espFormReceiver.watchHeld`), which cannot fail. A failed move
that rolls back has left the registry unchanged and ends the hold
(`espFormReceiver.release`), which closes the paths when nothing else is
watched. A rollback that is lost (`ErrTunnelMigrationLost`) also forgets a
watched SA on its old address, because the caller retires that Child SA. A
retarget that fails after a move that succeeded costs serving only: the SA
still receives the ESP-in-UDP form, so Ze keeps the move, never rolls the SAD
back for it, and logs one Warn, from `followMove` alone, whose fix is
`ze show vpn ipsec sa`, the message naming `ze clear vpn ipsec sa peer <name>`
to install a new Child SA.

<!-- source: internal/component/ike/dataplane/espform.go -- espFormRegistry, watch, forget, target, watching, fastTarget -->
<!-- source: internal/component/ike/dataplane/espform_linux.go -- espFormReceiver.Watch, espFormReceiver.acquire, espFormReceiver.watchHeld, espFormReceiver.release, espFormReceiver.Forget, espFormReceiver.retarget, espFormReceiver.admitsMove, syncFastPath, servingLocked, reopenReaderLocked, closeReaderLocked, espFormRegistry.empty, espFormRegistry.retarget, espFormRegistry.admitsMove, moveSlotLocked, run -->
<!-- source: internal/component/ike/dataplane/espform_open_linux.go -- openESPFormPaths, espFormSocketRefusal, espFormBypassRefusal, espFormSecurityModuleRefusal, espFormOpenFailure, espFormReceiver.read, espFormReceiver.revive, reportReaderStopped, reopenWaitLocked, reviveLocked -->
<!-- source: internal/component/ike/dataplane/espform_tun_linux.go -- espFormTUNStepRefusal, espFormTUNSysctlWriteRefusal -->
<!-- source: internal/component/ike/dataplane/espform_state.go -- ESPFormReaderStopped, RegisterESPFormReaderStopped -->
<!-- source: internal/component/ike/engine/register_esp_form_reader.go -- espFormReaderStoppedSink -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormDeviceFindings, espFormSameCause -->
<!-- source: internal/component/ike/dataplane/xfrm_migrate_linux.go -- MigrateTunnel, xfrmMigrationCollision, xfrmMigrationUnserved, espFormRefusalCause, followMove -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormCompletePathFindings -->
<!-- test: internal/component/ike/dataplane/xfrm_migrate_linux_test.go TestXFRMMigrationUnwatchedSAOpensThePathsBeforeTheMove -->
<!-- test: internal/component/ike/dataplane/espform_refusal_linux_test.go TestXFRMMigrationUnservedRendersTheFixOnce -->
<!-- test: internal/component/ike/dataplane/espform_reopen_linux_test.go TestESPFormReceiverWatchReopensAStoppedReader -->
<!-- source: internal/component/ike/engine/child.go -- generateESPSPI -->
<!-- test: internal/component/ike/dataplane/espform_test.go TestESPFormRegistryHoldsTwoSAsOnOneSPI -->
<!-- test: internal/component/ike/dataplane/espform_run_linux_test.go TestESPFormRunServesTwoSAsOnOneSPI -->
<!-- test: internal/component/ike/dataplane/espform_linux_test.go TestESPFormRegistryRetargetRefusesATakenAddress -->
<!-- test: internal/component/ike/dataplane/espform_linux_test.go TestESPFormReceiverWarnsOfNoSharedSPIWhileUnavailable -->
<!-- test: internal/component/ike/dataplane/espform_fastpath_integration_linux_test.go TestESPFormFastPathSecondSATakesOver -->
<!-- test: internal/component/ike/dataplane/espform_fastpath_integration_linux_test.go TestESPFormFastPathWarnsOfASharedSPIWhileAttached -->
<!-- test: internal/component/ike/dataplane/xfrm_migrate_linux_test.go TestXFRMMigrationReceiverFollowsTheMove -->
<!-- test: internal/component/ike/dataplane/xfrm_migrate_linux_test.go TestXFRMMigrationRetargetRefusesAnotherSAsAddress -->

The receiver's state records each such SA for the doctor check
(`ESPFormFastPathState.SharedSPIs`, read from the registry by
`espFormRegistry.shared`): one entry per SA the fast path does not serve,
naming the SPI, the local address of the SA the fast path serves and the local
address of the one it does not. The entry appears with the second Watch and
goes with the Forget of either SA. While the fast path is attached, the check
reports each entry as a `doctor-ipsec-esp-form-fast-path` warning. It names the
SPI and both addresses, and says that the SA on the second address takes the
complete path at its rate bound because two Child SAs on different local
addresses drew the same random SPI. Its fix is
`ze clear vpn ipsec sa peer <name>`, for the peer whose current or pending
Child SA carries that SPI to the unserved address. The lookup compares the SPI
and the address while it holds the session's lock, the one a MOBIKE move
rewrites the address under. It does not read the Child SA a rekey replaced
(`supersededChild`), which only the session's own loop may read. Ze has no
command that rekeys one Child SA, so clearing that peer is how the SA draws a
new SPI, and that peer's tunnel carries no traffic until its new IKE SA and
Child SA are up. When no current or pending Child SA matches, as while a rekey
replaces it, or when the peer's name cannot be typed as one command word, the
fix is `ze show vpn ipsec sa`. That listing shows each peer's inbound SPI but
no local address, so the message says that clearing either peer whose Child SA
carries the SPI ends the collision: the cleared peer's new Child SA draws a new
SPI, and the other SA takes the fast path over. While the fast path is unavailable or
detached, every SA's bare ESP takes the complete path, so that status's own
warning is reported alone.

<!-- source: internal/component/ike/dataplane/espform.go -- espFormRegistry.shared -->
<!-- source: internal/component/ike/dataplane/espform_linux.go -- espFormReceiver.fastPathState -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormFastPathAttached, espFormSharedSPI, espFormSharedSPIPeer, espFormChildCarries -->
<!-- source: internal/component/ike/cmd/ipsec.go -- handleClearIPsecSAPeer -->
<!-- test: internal/component/ike/dataplane/espform_test.go TestESPFormRegistryListsSharedSPIs -->
<!-- test: internal/component/ike/dataplane/espform_linux_test.go TestESPFormReceiverReportsSharedSPI -->
<!-- test: internal/component/ike/engine/doctor_esp_form_fast_path_test.go TestDoctorESPFormSharedSPI -->
<!-- test: internal/component/ike/engine/doctor_esp_form_fast_path_test.go TestDoctorESPFormSharedSPIPeerRacesNoMOBIKEMove -->

Bare ESP for a watched SPI reaches XFRM in the template's form through one of
two paths, and both are policy-checked. Both serve an IPv4 outer header, and
for IPv4 the receiving device decides the check: the decapsulated packet keeps
that device as `skb->dev`, the input path and the input route set
`IPSKB_NOPOLICY` only when its `disable_policy` (ORed with `conf/all`) is 1,
and both devices carry 0 (the real link unless the operator set it, and
`ze-espform` always), so the inbound policy check matches the packet against
the SA's selectors (RFC 4301 Section 5.2). IPv6 decides the check differently,
from the `DST_NOPOLICY` flag of the local route rather than the device, and no
IPv6 SPI is watched yet (see the end of this section).

<!-- source: internal/component/ike/engine/child.go -- installChildDataplane, inbound.AcceptBothESPForms -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- InstallSA, RemoveSA -->
<!-- source: internal/component/ike/dataplane/xfrm_migrate_linux.go -- MigrateTunnel -->
<!-- source: internal/component/ike/dataplane/espform_linux.go -- espFormReceiver.Watch, espFormReceiver.Forget, espFormReceiver.retarget -->
<!-- source: internal/component/ike/dataplane/espform.go -- espFormRegistry -->

**The fast path** is a tc ingress program built from `asm.Instructions`
(`espFormFastPathSpec`), one variant for an Ethernet link (14 octets of L2
header) and one for an L3 device. Before it reads the packet it reads a
one-element gate map (`ze_esp_gate`), and while the gate holds 0, or cannot be
read, it leaves every packet untouched with `TCX_NEXT`. A new collection's gate
reads 0. It looks the SPI up in a hash map
(`ze_esp_watch4`, `ze_esp_watch6`), checks the outer source and destination
against the target, inserts the eight-octet UDP header with
`bpf_skb_adjust_room(BPF_ADJ_ROOM_NET)`, writes the template's ports, and fixes
the lengths, the protocol and the IPv4 header checksum. It verifies the
received IPv4 header checksum first, so it never gives a corrupt header a valid
one. An IPv6 datagram gets checksum `0xffff` and is marked
`CHECKSUM_UNNECESSARY` with `bpf_csum_level`; that UDP receive then skips the
checksum is unproven, and nothing reaches the IPv6 half until
`plan/post-release/features/spec-ike-ipv6-transport.md`, which owes that proof.
The IPv6 datagrams it skips are served by no other path either: the complete
path reads IPv4 only. Every datagram it does not
rewrite leaves byte-identical with `TCX_NEXT`: a fragment, an IPv4 header with
options, an IPv6 extension header, a `CHECKSUM_COMPLETE` IPv6 skb, an IPv4
header whose checksum is wrong, and a datagram the helper refuses (a GSO skb).
A rewritten datagram also answers `TCX_NEXT`, so a program attached behind it
(the trafficusage counter) still runs. Only a datagram whose room was opened
and whose header write then failed is dropped. A per-CPU counter
(`ze_esp_count`) counts the rewrites per family.

<!-- source: internal/component/ike/dataplane/espform_program_linux.go -- espFormFastPathSpec, espFormFastPathInstructions, espFormFastPathIPv4, espFormFastPathIPv6 -->

Ze attaches the program at the head of every non-loopback link while any SPI
is watched, follows links created later through a netlink link subscription,
and logs a link type the program has no variant for by name. `lo` and
`ze-espform` stay unattached. Every watch, forget and retarget also writes the
map. The last forget and `Close` detach every link and release the map.
The receiver starts the fast path after the complete path, and the fast path
never refuses an SA: when the program cannot load, or a map write fails, the
failure is logged at Warn with its doctor code and the complete path serves
the SPI. Each watch, forget and retarget updates the registry and decides the
start or stop under one lock, so a watch that races the last forget never finds
the receiver stopping under it. `ESPFormFastPathState` on the XFRM backend
reports the fast path's state for the doctor check.

<!-- source: internal/component/ike/dataplane/espform_fastpath_linux.go -- the fast path's attach lifecycle -->
<!-- source: internal/component/ike/dataplane/espform_linux.go -- espFormReceiver.Watch, espFormReceiver.Forget, espFormReceiver.watchFastPath, openLocked, stopLocked -->
<!-- source: internal/component/ike/dataplane/espform_backend_linux.go -- ESPFormFastPathState -->

**The complete path** is the raw `IPPROTO_ESP` reader (`espFormReceiver.run`).
Linux delivers a copy of every bare ESP datagram to a raw socket before XFRM
refuses it, reassembled when it was fragmented. A datagram the fast path
rewrote is UDP by then, so the reader never sees it and no datagram is served
twice. For a watched SPI whose source is the SA's peer, the reader builds the
ESP-in-UDP datagram (`writeESPForm`, IPv4 header checksum computed) and writes
it into the TUN `ze-espform` (`espFormTUN.inject`). Ze writes
`disable_policy` 0 and IPv4 `rp_filter` 2 on the TUN: the datagram keeps the
peer's source, whose route leaves through the real link, so strict reverse-path
filtering (1) would drop it. Where `/proc/sys` is read-only and Ze cannot
write, it requires the TUN's own `disable_policy` to be 0 already, and it
accepts the `rp_filter` the kernel applies (the larger of `conf/all` and the
device's value) when that value is not 1, because only strict filtering drops
the datagram (`settleESPFormTUNUnwritable`). An operator who sets
`conf/all/disable_policy` 1 disables the check on every device, this one
included. The path runs in userspace, so `espFormLimiter` bounds it to 5000
datagrams per second with a burst of 10000. The reader socket and the TUN open
on the first watch and close on the last forget, and the TUN is not persistent,
so a daemon that dies leaves no device behind.

<!-- source: internal/component/ike/dataplane/espform_linux.go -- espFormReceiver.run, openLocked, stopLocked -->
<!-- source: internal/component/ike/dataplane/espform_open_linux.go -- openESPFormPaths -->
<!-- source: internal/component/ike/dataplane/espform_tun_linux.go -- espFormTUN, openESPFormTUN, espFormTUNSysctls, settleESPFormTUNUnwritable -->
<!-- source: internal/component/ike/dataplane/espform.go -- writeESPForm, espFormRate, espFormBurst -->

A firewall rule keyed on the input interface sees `ze-espform` for the traffic
the complete path carries. The fast path runs at tc ingress, which
`__netif_receive_skb_core` (`net/core/dev.c`, v7.0) runs in
`sch_handle_ingress` before `nf_ingress`, the netdev ingress hook, and both run
before `ip_rcv` reaches `NF_INET_PRE_ROUTING` (`net/ipv4/ip_input.c`). A
datagram the fast path rewrites therefore reaches every netfilter hook as UDP
to port 4500, and a rule on `protocol esp` would never see it. A datagram the
complete path carries passes `NF_INET_LOCAL_IN` as ESP first, because
`raw_local_deliver` hands the reader its copy after that hook
(`ip_local_deliver`), so a rule there still applies to it; the datagram Ze
writes into `ze-espform` then passes the hooks again as UDP to port 4500.

So the fast path rewrites only while no nftables rule on the input path can
drop or change ESP. The fast path's worker reads the kernel's ruleset, not Ze's own
copy, because a forked IKE engine holds no copy, and judges it
(`espFormFirewallBlocks`): the base chains of the ingress, prerouting and input
hooks of every table in the inet, ip, netdev and bridge families, each walked
rule by rule. It opens the gate only when every such chain is safe. It judges
the ruleset when the fast path starts and again on each commit an nftables
watch signals. Each judgment dumps the ruleset holding no lock, and
publishes only when no hold, release or commit came while it read. One
judgment is several dumps (the tables, the chains, then one rule dump per base
chain), and `NLM_F_DUMP_INTR` flags a commit inside one dump only, while the
watch's `NFT_MSG_NEWGEN` can arrive after the last dump ended. So the
judgment reads the ruleset's generation (`NFT_MSG_GETGEN`) before its first
dump and after its last. Every commit bumps that counter. When the two reads
differ, a commit landed between two dumps, and the judgment publishes nothing
and runs again (`errESPFormRulesetChanged`).

The bracket catches a commit that starts and ends inside one judgment without
waiting for the watch, but it does not refuse every splice. `nf_tables_commit`
(v7.0) stores the bumped counter (`smp_store_release(&net->nft.base_seq,
base_seq)`) before it flips the generation cursor that decides which rules are
active, and before its commit loop applies table and chain updates, chain
policy included; `NFT_MSG_GETGEN` is answered under RCU, not under the commit
mutex. A first read can therefore answer the new number while the dumps still
read the old state, and the two reads agree. The kernel sends `NFT_MSG_NEWGEN`
after that loop, so the watch signals every such commit once it applied, and
the signal closes the gap.

Every change of the ruleset the worker sees, a commit the watch signals or a
bracket that moved, shuts the gate before the judgment that follows, as a hold
does (`firewallCommitted`). The gate is therefore open only over a ruleset some
judgment published, never across a change the watch delivered, and a judgment
that a commit spliced is open for at most the watch's latency, the same window
as a commit that lands while the gate is open. When two passes in a row each
see the ruleset move, the commits are a stream (fail2ban adding a set element
every few milliseconds is one, each element a commit): the worker keeps the
gate shut and judges again after a wait that starts at
`espFormNFTRejudgeFirst` (50 ms) and doubles to `espFormNFTRejudgeMax` (1 s),
so it never dumps back to back. The wait runs in the worker's loop, which
still applies link updates and ends at stop; a commit during it shuts the gate
again and leaves the judgment to the wait. A dump message the kernel flagged
`NLM_F_DUMP_INTR` (`errESPFormDumpInterrupted`) counts as a move of the ruleset
exactly as a moved generation does (`espFormRulesetMoved`): it shuts the gate
and steps the epoch, and never publishes `dump-failed`. From the first judgment
of a stream that ends Moving, the state reports the `moving` reading with
`FirewallMovingSince`, the worker clock's time when that judgment ended
(`firewallMoving`): the commits it met came before it, so the ruleset has kept
changing since at least then. The reading lasts until a judgment publishes or a shut records its own reading.

<!-- source: internal/component/ike/dataplane/espform_fastpath_gate_linux.go -- judgeFirewall, espFormRulesetMoved, firewallCommitted, judgeWatched, firewallMoving, endMovingLocked, espFormNFTRejudgeFirst, espFormNFTRejudgeMax -->
<!-- source: internal/component/ike/dataplane/espform_fastpath_linux.go -- serve -->

A ruleset
that cannot be read, a watch that ended, and a rule that can drop ESP each
shut the gate. `ESPFormFastPathState` then reports `FirewallBlocked` with the
first such rule in `FirewallSite`, or the failure in `FirewallCause`. With the
gate shut, the complete path serves the SA's bare ESP after the firewall
decided on it, rate-bounded.

`FirewallReading` (`ESPFormFirewallReading`) says how the ruleset is read, so a
reader never matches the cause's text: `unheld` while no program is loaded,
`pending` until the first judgment publishes, `read` once one read the ruleset
(`FirewallBlocked` then says whether a rule can drop ESP), `unreadable` when the
reader or the watch could not open at start, `rewatching` while the worker
opens a failed watch again, `watch-closed` when `Close` ended the watch,
`dump-failed` when the reader and the watch are open but the last dump failed,
and `moving` while a stream of commits keeps every judgment from reading the
ruleset (`FirewallMovingSince` is when the first judgment that met the stream
ended, not when the stream began, so the ruleset has kept changing since at
least then; `FirewallBlocked` and the fields beside it are clear). While the
fast path is attached, the doctor check chooses its finding and its fix from
the reading alone, under `doctor-ipsec-esp-form-firewall`. While it is
unavailable (the program never loaded) or detached (it loaded, then a write
that shuts the gate failed, `detachLocked`), no program reads the gate, so the
doctor reports that status alone and no firewall finding. The two have their
own findings under `doctor-ipsec-esp-form-fast-path`: unavailable names
`CAP_BPF`, the unit and the kernel, with the unit and capability install as its
fix; detached says the gate could not be shut, so the program was detached from
every link and bare ESP takes the complete path, with `sudo systemctl restart ze`
as its fix, the command the dataplane's warning names. A link subscription that
ends after the detach leaves the detach as the cause (`followEnded`). Where a
firewall finding below says no restart is needed, it says so for the firewall
reading alone, because under `attached` it can sit beside the finding for a
link subscription that ended, whose fix is a restart:

<!-- source: internal/component/ike/dataplane/espform_fastpath_gate_linux.go -- writeGateLocked, detachLocked -->
<!-- source: internal/component/ike/dataplane/espform_fastpath_linux.go -- followEnded -->
<!-- source: internal/component/ike/dataplane/espform_state.go -- ESPFormFastPathDetached -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormFastPathFindings, espFormFastPathDetached, espFormFastPathUnavailable -->
<!-- test: internal/component/ike/engine/doctor_esp_form_fast_path_test.go TestDoctorESPFormFastPathDetached -->
<!-- test: internal/component/ike/dataplane/espform_fastpath_gate_linux_test.go TestESPFormFastPathFailedShutDetaches -->
<!-- test: internal/component/ike/dataplane/espform_fastpath_gate_linux_test.go TestESPFormFastPathSubscriptionEndKeepsTheDetachCause -->


| Reading | Finding | Fix line |
|---------|---------|----------|
| `unheld`, `pending` | none | |
| `read` | the blocking site, when `FirewallBlocked`; none otherwise | the command or editor lines chosen for the site's table, or the warning log when none was chosen (a Ze defect) |
| `unreadable`, `watch-closed` | nothing reads the ruleset again until ze restarts | `sudo systemctl restart ze` |
| `dump-failed` | no restart is needed: the watch still runs, so the next commit reads the ruleset again, and nothing reads it before then | `sudo systemctl restart ze`, which reads it at once |
| `rewatching` | no restart is needed: Ze watches again by itself, at most 30 seconds apart | `ze show log recent level warn component ike.dataplane`, which names why the watch keeps failing |
| `moving`, `FirewallMovingSince` ten seconds ago or longer | the ruleset has kept changing since at least the time it names, so bare ESP takes the complete path at its rate bound, at most 5000 datagrams per second; no restart is needed: the fast path comes back on once the commits pause long enough for one read, and that read finds no rule that can drop ESP; run the fix to see which tool keeps committing, then make that tool batch its changes, or stop it (fail2ban, for example) | `sudo nft monitor` |
| `moving`, `FirewallMovingSince` shorter than ten seconds ago | none: the fast path outlasts a burst of commits | |

<!-- source: internal/component/ike/dataplane/espform_state.go -- ESPFormFirewallReading, ESPFormFirewallMovingReportAfter, ESPFormFixMoving, ESPFormFixRestart, ESPFormFixLog -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormFastPathFindings, espFormFirewallFindings, espFormFirewallDumpFailed, espFormFirewallRewatching, espFormFirewallUnread, espFormFirewallMoving -->
<!-- source: internal/component/ike/dataplane/espform.go -- espFormRate -->

Only a watch that `Close` ended keeps the gate shut until the fast path stops.
A watch that fails, on a failure the library reports or on a monitor that
ended with no `Close` asking for it, is opened again by the worker in the
namespace the fast path started in. The wait before each attempt doubles from
one second (`espFormNFTRewatchFirst`) to a cap of thirty seconds
(`espFormNFTRewatchMax`), and a watch that opens resets it. Once the new watch
is up, a fresh dump, which begins after it and so covers every commit made
while nothing watched, judges the ruleset again and reopens the gate when it
finds no rule that can drop ESP and no hold is open.

The watch's socket asks for a 16 MiB receive buffer
(`espFormNFTWatchBufferOctets`) through `SO_RCVBUFFORCE`, past
`net.core.rmem_max`. The kernel queues one notification for each object a
transaction changes before the `NFT_MSG_NEWGEN` that ends it, and Ze's firewall
applies its whole ruleset on every apply, so a large commit could overflow the
default buffer, and the next receive then fails with `ENOBUFS` and ends the
watch. The larger buffer makes that overflow less likely; it does not rule it
out, and the reopening above recovers it. The option needs `CAP_NET_ADMIN`, as
joining the nftables group does: when the kernel refuses it, Ze logs a warning
and the watch keeps the kernel's default buffer, because the option never fails
the dial.

The watch has one known limit: a removal it is never told of. When the process
that owns a table created with the owner flag (`NFT_TABLE_F_OWNER`) exits, the
kernel removes the table outside any commit (`nft_rcv_nl_event`, then
`__nft_release_table`, `net/netfilter/nf_tables_api.c`, v7.0), with no
generation bump and no `NFT_MSG_NEWGEN`. A gate that table shut stays shut
until the next commit, by any program, has the worker judge again. That limits
the fast path's liveness, never the firewall's decision: an accept in one base
chain does not stop another, so removing a table can make no safe ruleset
unsafe.

<!-- source: internal/component/ike/dataplane/espform_fastpath_gate_linux.go -- espFormNFTRewatchFirst, espFormRewatchIn, nextWait, watchLost, watchAgain, firewallWatchEnded, firewallWatched, firewallRewatchFailed, shutGateLocked, clearFirewallLocked -->
<!-- source: internal/component/ike/dataplane/espform_state.go -- espFormNFTRewatchMax, ESPFormFirewallReading -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormFirewallFindings -->
<!-- source: internal/component/ike/dataplane/espform_fastpath_linux.go -- serve -->
<!-- source: internal/component/ike/dataplane/espform_firewall_kernel_linux.go -- espFormNFTWatchBufferOctets, startESPFormNFTWatch, espFormNFTWatchBuffer, forceESPFormNFTWatchBuffer, errESPFormNFTWatchEnded -->
<!-- test: internal/component/ike/dataplane/espform_firewall_linux_test.go TestESPFormNFTWatchReceiveBuffer -->

The walk is conservative, and it judges each rule from that rule's own
expressions, with no model of the registers (`espFormJudgeRule`). A rule is
excluded only when its first two expressions are a load and an equality compare
of exactly the register that load wrote, against one octet that is not ESP's
value (`espFormExcludesESP`). Three loads count, each a value the kernel computes
from the IPv4 header of the packet the program rewrites (`espFormFirstLoad`):
`meta l4proto` in every family walked, the IPv4 protocol octet (network header
offset 9, one octet) in an `inet` or `ip` table, and `meta nfproto` in an `inet`
or `ip` table, where it reads `ipv4` for every IPv4 packet. ESP's value is 50
for the first two and `NFPROTO_IPV4` for the third. So `meta l4proto udp`, `ip
protocol tcp` and the `meta nfproto ipv6` guard Ze's firewall lowering puts first
in an IPv6 address match each exclude a rule. Nothing else does: not `meta
l4proto != esp`, not the same pair later in the rule, and not a match on
anything else (an address, a port, an interface, `meta protocol`, `nfproto` in a
`netdev` or `bridge` table). `nft_do_chain` runs a rule's expressions in order
from the first, so a compare of the register the first expression has just
written reads that load and nothing an earlier rule or expression left behind.

The stock producers whose drops sit on the walked hooks put their protocol match
first for this reason. GTSM's listen-port term (`listenPortTerm`) opens with
`meta l4proto tcp` ahead of the source address, which in its `inet` table would
lower to `meta nfproto ipv4` first and exclude nothing for an IPv4 peer; the
source match keeps its own `nfproto` guard directly ahead of its address load
(`lowerAddrMatch`). The on-host DDoS drop (`buildDropTerm`) opens with the
vector's protocol ahead of the victim address, and keeps the fast path off only
for a vector whose protocol is ESP or has no canonical name, where the drop can
match ESP to the victim. The proofs judge what the nft backend committed to a
kernel from the producers' own builders, `gtsm.FilterTables` and
`local.MitigationTable` (the table `applyMitigation` installs), never rules
written by hand.

<!-- source: internal/component/gtsm/gtsm.go -- listenPortTerm, FilterTables -->
<!-- source: internal/plugins/ddos/local/match.go -- buildDropTerm -->
<!-- source: internal/plugins/ddos/local/responder.go -- MitigationTable, applyMitigation -->
<!-- source: internal/plugins/firewall/nft/lower_linux.go -- lowerAddrMatch, lowerProtoMatch -->
<!-- test: internal/component/ike/dataplane/espform_firewall_stock_linux_test.go TestESPFormFirewallStockGTSMKeepsTheFastPathOn -->
<!-- test: internal/component/ike/dataplane/espform_firewall_stock_linux_test.go TestESPFormFirewallStockDDoSNamedProtocolKeepsTheFastPathOn -->
<!-- test: internal/component/ike/dataplane/espform_firewall_stock_linux_test.go TestESPFormFirewallStockDDoSUnnamedProtocolClosesTheFastPath -->

The definite accept (`espFormAcceptsESP`) has the same opening: a protocol
load, a compare of that register for equality with 50, then at most a counter
and an accept, and nothing else. It accepts every ESP packet that reaches it and
ends the chain's walk as safe. Ze's lowering of a term that matches `protocol
esp` and accepts takes that shape, and so does nft's `meta l4proto esp accept`.

Every other rule can match ESP. It lets the walk go on to the next rule only
when every expression in it is on a short closed list (`espFormHarmless`) and
its verdict is accept, break or continue (`espFormJudgeVerdict`). Each entry
writes nothing but a data register, and either selects or records the packet
somewhere outside it: a compare or a range, a counter, a log, a limit, a load
from `meta`, `ct`, the packet or an extension header (a destination register
and no source register), a bitwise, byteorder, `fib`, `rt`, `socket`, `hash`
or `numgen`, and constant data into a data register. For an IPv4 packet `fib`
and `socket` set no verdict but `NFT_BREAK`: the `NF_DROP` at the end of
`nft_fib_inet_eval` is reached only for a result `nft_fib_init` refuses. `hash`
and `numgen` write their register and nothing else, and have no map form.
Every other expression closes the fast path, and the site names its class:

| Expression | Reason |
|------------|--------|
| A `meta` or `ct` write (`meta mark set`, `ct mark set`), `notrack` | `meta or ct set` |
| A payload or extension header write, a DSCP set included, and an extension header with no register (the TCP option strip, which can drop) | `packet rewrite` |
| A set lookup for membership (`ip saddr @s`) | `set lookup` |
| A lookup with a data output (a map) | `map lookup` |
| A lookup whose output is the verdict (a verdict map) | `verdict map` |
| A set add or update (`dynset`) | `set update` |
| `dup` | `dup` |
| A reference to a stateful object, other than a synproxy (`espFormObjref`) | `object reference` and the object type |
| `quota`, `connlimit` | `expression off the harmless list` and the type |
| `reject`, `queue`, NAT, masquerade or redirect, `tproxy`, `synproxy` | `reject`, `queue`, `nat`, `tproxy`, `synproxy` |
| A netdev `fwd` | `forward` |
| An iptables compat expression (`xt`, which iptables-nft writes for an extension) | `xt compat` |
| An expression the walk does not name | `unknown expression` and the type |

A set lookup is not harmless, membership included: on a hit `nft_lookup_eval`
runs the matched element's own stateful expressions
(`nft_set_elem_update_expr`), which the rule does not carry and the walk never
reads, and a `connlimit` there can set `NF_DROP`. No stock Ze rule needs one on
the path: copp's set match sits behind its TCP guard, which excludes the rule
first, and the irr whitelist ends in a drop of its own. A mark write is not
harmless, because a mark can choose a route through an `fwmark` rule. A DSCP set is not either: `nft_payload_set_eval` writes the
checksum delta at the offset the rule chose (`csum_offset`, any value up to
`U8_MAX`), so a one-octet DSCP write can rewrite the TTL and the protocol too.
Only base chains are read. A rule that can match ESP and sends it anywhere but
an accept or a drop (a jump, a goto, a return, a verdict map) counts as unsafe,
and its target is never read. In
`nft_do_chain` (`net/netfilter/nf_tables_core.c`) a return in a base chain skips
every later rule and meets the policy, and a goto pushes nothing, so the end of
its target is a return from the chain that went to it. A walk that read on past
either would count a later ESP accept that the ESP which left never reaches. A
ruleset that sends ESP through a jump therefore keeps the fast path off until an
ESP accept comes ahead of the jump. Every other verdict (drop, queue, stolen,
repeat, stop, or a code the walk cannot read) counts as able to drop. A chain
whose policy is drop, reached with no accept of ESP, is unsafe too.

The reader (`espFormKernelReader`) dumps the tables, the chains and the rules
itself, on one netfilter socket, rather than through the vendored library's
readers: `Conn.GetRules` drops every expression the library cannot decode, so a
netdev `fwd` would read as a rule with no verdict, and `Conn.ListTables` and
`Conn.ListChains` pass on a message the kernel flagged `NLM_F_DUMP_INTR`. A
table, chain or rule message flagged `NLM_F_DUMP_INTR` (the ruleset changed
while it was read, so what was read is two rulesets spliced) fails the dump,
and the gate stays shut until the judgment the commit's `NFT_MSG_NEWGEN` asks
for. That flag misses a commit between two dumps, which the generation
bracket above refuses. The reader dials the kernel's nftables netlink from the namespace the fast
path started in, and the watch joins the nftables multicast group there.

<!-- source: internal/component/ike/dataplane/espform_firewall_linux.go -- espFormFirewallBlocks, errESPFormRulesetChanged, espFormFirewallWalk, espFormBaseChains, espFormBaseBlocks, espFormJudgeRule, espFormJudgeVerdict, espFormFirstLoad, espFormESPValue, espFormExcludesESP, espFormAcceptsESP, espFormHarmless, espFormLookup, espFormObjref, espFormUnsafe, espFormFirewallSite -->
<!-- source: internal/component/ike/dataplane/espform_firewall_kernel_linux.go -- espFormKernelReader, openESPFormNFTReader, Generation, espFormGenerationFromMessage, espFormDumpMessage, errESPFormDumpInterrupted, espFormRuleFromMessage, startESPFormNFTWatch, espFormNFTWatch -->

The program passes every skb that carries a VLAN tag, out of band
(`vlan_present`) or in the frame, untouched to the complete path
(`espFormFastPathInstructions`). `skb_vlan_untag` runs before TCX in
`__netif_receive_skb_core` (`net/core/dev.c`), so on a VLAN's parent link TCX and
the netdev hooks see one skb whose protocol is already the inner IPv4, and a rule
there can still match the tag (`nft_payload_copy_vlan`,
`net/netfilter/nft_payload.c`). A rewrite would hand that rule UDP where it
matches ESP. The VLAN device's own TCX and hooks run on the next round of the
receive loop, where the datagram arrives untagged.

<!-- source: internal/component/ike/dataplane/espform_program_linux.go -- espFormFastPathInstructions, espFormFastPathSkbVLANPresent -->
<!-- test: internal/component/ike/engine/rfc4301_espform_policy_integration_linux_test.go TestRFC4301BareESPFirewallDropKeepsTheFastPathOff -->

What the walk cannot see, it cannot gate on. The cases below are the ones known
today. They are not a proof that nothing else on a host decides on ESP after tc
ingress: anything outside the nftables ruleset that matches the IP protocol
meets the same limit.

- Rules loaded with iptables-legacy sit in x_tables, which no nftables dump
  returns, so the fast path rewrites past them; iptables-nft rules are nftables
  rules and are judged.
- Rules loaded with ebtables-legacy sit in the bridge's own ebtables tables
  (`net/bridge/netfilter/ebtables.c`), which no nftables dump returns either;
  an nftables `bridge` table is judged, and ebtables-nft writes one.
- A `BPF_PROG_TYPE_NETFILTER` program attached through a
  `BPF_LINK_TYPE_NETFILTER` link (`include/uapi/linux/bpf.h`, the
  `netfilter` attach fields `pf`, `hooknum` and `priority`) runs on an IPv4 or
  IPv6 netfilter hook, `NF_INET_PRE_ROUTING` and `NF_INET_LOCAL_IN` among
  them. It is a netfilter hook entry, not an nftables chain, so no nftables
  dump returns it, and it sees the rewritten datagram as UDP to port 4500.
- A tc ingress program or filter another owner attached behind the fast path
  sees the rewritten datagram as UDP to port 4500, because the fast path
  attaches at `link.Head()` and answers `TCX_NEXT` (`espFormFastPathNext`), and
  classic tc filters run after every TCX program (`sch_handle_ingress`,
  `net/core/dev.c`).
- Policy routing keyed on the protocol sees the rewritten datagram as UDP.
  `ip_route_input_slow` (`net/ipv4/route.c`) and `__fib_validate_source`
  (`net/ipv4/fib_frontend.c`) fill `flowi4_proto` and the ports from
  `fib4_rules_early_flow_dissect` (`include/net/ip_fib.h`), which dissects the
  skb as it is after tc ingress, and only once a rule that needs it exists
  (`fib_rules_require_fldissect`, counted from `fib_rule_requires_fldissect`
  in `include/net/fib_rules.h`). `fib4_rule_match` (`net/ipv4/fib_rules.c`)
  compares `rule->ip_proto` with that field. The reverse-path check is reached
  first: `fib_validate_source` runs `__fib_validate_source` whenever
  `rp_filter` is on (the secpath exemption does not apply, because XFRM has
  not seen the datagram yet), its lookup goes to the peer's address, so the
  `local` rule at priority 0 finds nothing and the walk reaches later rules,
  and a lookup that fails under `rp_filter` 1 or 2 ends at `e_rpf`, a drop. An
  `ipproto esp` rule answering `prohibit`, `blackhole` or `unreachable` then
  drops bare ESP and passes the rewritten UDP. The forward lookup reaches such
  a rule only when the `local` rule no longer sits at priority 0, as the usual
  VRF setup arranges. An `fwmark` rule never meets the difference through
  nftables: a rule that can match ESP and sets a mark is a `meta` write, which
  keeps the fast path off (`espFormHarmless`). So only a routing rule written by
  hand that selects on `ipproto esp` reaches it, unread by the walk in the same
  way as an iptables-legacy rule. Ze installs no `ipproto` rule
  and keeps the `local` rule ahead of its own: the IPsec source guard at
  `iprule.IPsecSourceGuardPriority` (30000), the policy routing plugin's fwmark
  rules from 100, and the MPLS context rules at 1 and 2, whose installer leaves
  the kernel's priority-0 `local` rule in place (`ensureMPLSSelector`).

The guide's "Both ESP wire forms" gives the operator's fix for each.

<!-- source: internal/component/ike/dataplane/espform_fastpath_linux.go -- linkAdded, the link.Head() anchor -->
<!-- source: internal/component/ike/dataplane/espform_program_linux.go -- espFormFastPathNext -->

Ze's own firewall commits take a hold (`ESPFormFirewallHolder`, implemented by
the XFRM backend): the hold writes 0 into the gate before the commit, and its
release asks for a judgment that begins after the commit. A hold taken while
the fast path is idle still counts, so a start during the commit stays shut.
A commit by another program is seen only after it landed: the window is the
watch's latency, plus the end of the one dump the worker may be running when
the signal arrives, after which the signal shuts the gate. A stream of commits
keeps the gate shut until a judgment reads the ruleset with nothing moving. A
forked IKE engine takes no hold, so it has
that window for Ze's commits too. A datagram in that window is still
decrypted and checked against the SA's selectors; what it misses is the
firewall's decision.

<!-- source: internal/component/ike/dataplane/espform_fastpath_gate_linux.go -- judgeFirewall, publishFirewall, hold, releaseHold -->
<!-- source: internal/component/ike/dataplane/espform_firewall_hold.go -- ESPFormFirewallHolder -->
<!-- source: internal/component/ike/dataplane/espform_program_linux.go -- espFormFastPathInstructions, the gate lookup -->

The IKE engine takes that hold through a firewall commit observer
(`holdESPFormFastPathForCommit`, registered from `init()` with
`firewall.RegisterCommitObserver`). `firewall.ApplyAll` runs it inside its
reconcile lock just before the backend applies, and calls its release once the
apply returns, on success and on failure. The observer runs in every process
that links the engine, so with the engine forked the daemon's copy finds no
IPsec dataplane in the daemon (`dataplane.Get` answers nil, or an XFRM backend
OSPF loaded with no dual-form receiver) and holds nothing. The doctor check
`ipsec-esp-form-fast-path` reports a shut gate of an attached fast path as
`doctor-ipsec-esp-form-firewall`: the first blocking site, or the cause when
the ruleset was not read or followed, the complete path's rate bound, the
drops above it, and the fix.

The fix depends on who writes the blocking site's table
(`espFormFirewallAccept`). A table whose name does not start with `ze_` is not
Ze's, and the fix is the `nft insert` of an ESP accept at the head of that base
chain (`espFormForeignTableAccept`), or the ruleset listing when the table or
chain name holds a quote or a control character. Ze rewrites a `ze_` table, so
an nft rule there does not last, and who writes it is the firewall registry's
to say, which the dataplane does not import: the engine registers
`espFormOwnedTableAccept` with `dataplane.RegisterESPFormOwnedTableAccept`
(`register_esp_form_table.go`). A table a producer reserved
(`firewall.TableControlOf`) is answered with the control the producer declared
when it reserved it: the plugin that writes it, what decides its rules, and the
command that shows that state. Any other `ze_` table is the firewall config's,
and the fix is the editor lines that add the `accept-esp` term to the chain and
commit it, with the sentence that the editor appends a new term and has no
command that moves it ahead of another. A `ze_` table or chain name the config
cannot hold gets the ruleset listing.

<!-- source: internal/component/ike/dataplane/espform_fastpath_gate_linux.go -- espFormFirewallAccept, espFormForeignTableAccept, espFormNFTQuotable -->
<!-- source: internal/component/ike/dataplane/espform_state.go -- ESPFormOwnedTableAccept, RegisterESPFormOwnedTableAccept, FirewallAcceptFix, FirewallAcceptHow -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormOwnedTableAccept, espFormConfigSite, espFormConfigName -->
<!-- source: internal/component/firewall/reservation.go -- TableControl, MustReserveTableNames, TableControlOf -->
<!-- test: internal/component/ike/engine/doctor_esp_form_fast_path_test.go TestESPFormOwnedTableAccept -->

<!-- source: internal/component/ike/engine/register_firewall_observer.go -- holdESPFormFastPathForCommit -->
<!-- source: internal/component/firewall/commit_observer.go -- CommitObserver, applyObserved -->
<!-- source: internal/component/ike/engine/doctor_esp_form_fast_path.go -- espFormFirewallFindings, espFormFirewallBlocked, espFormFirewallUnread -->

The `lo` re-presentation that `ze-espform` replaced arrived with
`disable_policy` 1, which sets `IPSKB_NOPOLICY`, so `__xfrm_policy_check2`
returned before any policy was read.

Dual-form serving over an IPv6 outer header is a gap owed after the first
release. Ze has no IPv6 IKE transport yet: the IKE listener opens `udp4` only
(`transport/udp.go` `newTransport`) and the probe refuses an IPv6 peer
(`engine/probe.go`), so no Child SA with IPv6 outer addresses is negotiated.
`InstallSA` refuses one that asks for both forms, and `MigrateTunnel` refuses a
MOBIKE move to IPv6 endpoints. The IPv6 variant of the fast path program is
built and tested, and nothing writes its map. The work is owned by
`plan/post-release/features/spec-ike-ipv6-transport.md`.

<!-- source: internal/component/ike/dataplane/espform_tun_linux.go -- espFormTUNSysctls -->
<!-- source: internal/component/ike/dataplane/xfrm_linux.go -- InstallSA -->
<!-- source: internal/component/ike/dataplane/xfrm_migrate_linux.go -- validateXFRMMigration -->
<!-- source: internal/component/ike/transport/udp.go -- newTransport, the udp4 listener -->

## The application override guard

RFC 4301 Section 4.4.1 owes the administrator a choice over whether a user or
application may override the SPD. On Linux the override is a per-socket policy:
`xfrm_lookup_with_ifid` and `__xfrm_policy_check` consult `sk->sk_policy` before
any SPD entry, and `do_ip_setsockopt` and `do_ipv6_setsockopt` ask only for
`CAP_NET_ADMIN`. The kernel's own enforcement point is cgroup BPF on setsockopt:
`do_sock_setsockopt` runs `BPF_CGROUP_RUN_PROG_SETSOCKOPT` before the protocol
handler, and any attached program that answers 0 turns the call into `EPERM`.

`vpn ipsec application-override` (`deny` by default) selects it.
`applyApplicationOverride` (`engine/application_override.go`) attaches the guard
after the if_id 0 catch-all is first installed and before the IKE bypass, moves
it on a reload, and `releaseApplicationOverride` detaches it after the catch-all
leaves. A guard that cannot attach under `deny` fails the apply. The seam is
`dataplane.OverrideGuard` (`override_guard.go`); the XFRM backend implements it
in `override_guard_linux.go`, VPP answers `OverrideGuardNotApplicable` because
its SPD holds no socket policies, and the noop backend answers `ErrNotSupported`.

The program (`overrideGuardInstructions`) is assembled in Go with
`cilium/ebpf/asm`, attached by link to the cgroup2 root found in
`/proc/self/mountinfo`, and lives as long as the link Ze holds: no bpffs pin. It
answers 1 for every option but `IP_XFRM_POLICY`, `IP_IPSEC_POLICY` (level
`SOL_IP`), `IPV6_XFRM_POLICY` and `IPV6_IPSEC_POLICY` (level `SOL_IPV6`), so
every other setsockopt on the host costs two loads and two compares. For those
four it answers 1 when `bpf_get_netns_cookie` says the socket is outside Ze's
network namespace, or when the caller is the process that loaded the guard, and
otherwise counts the refusal in a per-CPU array and answers 0.

The caller is named with `bpf_get_ns_current_pid_tgid`, handed the device and
inode of the loader's `/proc/self/ns/pid`, and compared with `os.Getpid`. Both
are then numbers in the same PID namespace. `bpf_get_current_pid_tgid` answers
in the initial namespace, which in a container (the interop lab, a Docker
deployment) never equals `os.Getpid`, so a guard built on it refused Ze's own
ESP form policy. A caller in any other PID namespace gets a non-zero answer from
the helper and is refused. The device is converted to the kernel's `dev_t`
encoding (`kernelDev`): `ns_match` compares against the nsfs superblock, and the
stat(2) encoding differs from it once the minor reaches 256. Both helpers are in
`bpf_base_func_proto` ahead of its `CAP_BPF` gate, and `cg_sockopt_func_proto`
falls through to it (`kernel/bpf/helpers.c`, `kernel/bpf/cgroup.c`, v7.3).

Ze's one socket policy, the ESP form reader's inbound ALLOW
(`espFormBypassInboundPolicy`), lets the complete path read the bare ESP copy
that Ze's own inbound Child SA policy would refuse at `raw_rcv`. It covers that
read and nothing else: the datagram the reader writes into `ze-espform` gets the
inbound policy check (see "Both ESP forms on one inbound state"). The policy
carries `PriorityESPFormSocket` as a mark. The
kernel copies a socket policy's priority (`copy_from_user_policy`, reached from
`xfrm_compile_policy`) and never matches on it, so the read-back names that row
`esp-form-socket` and any other `socket-override` (`socketPolicyRole`). A
process that set its policy before the guard attached can copy the mark; doctor
therefore counts every socket policy, not only the unmarked ones.

<!-- source: internal/component/ike/dataplane/override_guard_linux.go -- attach, overrideGuardInstructions, currentPIDNamespace, kernelDev, cgroup2RootFrom -->
<!-- source: internal/component/ike/dataplane/override_guard.go -- OverrideGuard, OverrideGuardStatus -->
<!-- source: internal/component/ike/engine/application_override.go -- applyApplicationOverride, releaseApplicationOverride -->
<!-- source: internal/component/ike/dataplane/policy_role.go -- socketPolicyRole -->

Both eBPF loads, the guard and the ESP form fast path, run in a daemon that is
not root and holds only the capabilities the systemd unit grants
(`privilege.DaemonCapabilities`). That set carries no `CAP_PERFMON`, so the
verifier judges both hand-assembled programs by its unprivileged rules, which
every other test here hides by loading as root. Two guest-root tests prove each
load passes them: each starts this test binary again under uid 65534, with the
unit's set as its only effective capabilities, in a network namespace of its
own. `TestESPFormFastPathLoadsUnderTheUnitCapabilities` loads the fast path's
collection through `loadESPFormFastPathCollection` and attaches both programs at
the head of a dummy link's TCX ingress. `TestOverrideGuardLoadsUnderTheUnitCapabilities`
attaches the guard through `AttachOverrideGuard`, reads it attached, and
detaches it. The helper is the loader, so the guard exempts it, and the guard
governs only the helper's namespace.

<!-- source: internal/component/ike/dataplane/unit_capabilities_integration_linux_test.go -- TestESPFormFastPathLoadsUnderTheUnitCapabilities, TestOverrideGuardLoadsUnderTheUnitCapabilities -->

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
Child SA it creates installs, which each exchange computes once and passes to
`narrowChildSelectorsIn`: the IKE_AUTH decision (`decidedChildMode`,
`sa.UseTransportMode`, read by no later exchange), the new-child decision
(`respondNewChild`), and for a Child SA rekey the mode the rekey negotiates
(`respondChildRekey`), so an IKE SA rekey, whose successor carries no IKE_AUTH
decision, changes nothing. The refusal names both peers and
the selector or pool fix on the tunnel peer. Only the overlap of two
transport-mode pairs is left to `claim` above (`conflictLocked`,
`bothTransport`), so it is refused once, with `TransportSelectorConflictError`.

The operator-ordered exemption applies to a tunnel against a transport-mode pair
as to two tunnels: both pairs operator-fixed, at different `policy-priority`
values (`operatorOrdered`). A pair is judged whole, as the policy it installs:
both prefixes, the protocol (the intersection of the two halves' protocols,
`childPolicyPair`) and both ports (`samePolicyPair`), so a pair that differs from
the configured selector only by its local port or its local prefix is the peer's
choice (`TestRFC3948ConfiguredSelectorNarrowedOnLocalPortRefused`,
`TestRFC3948ConfiguredSelectorNarrowedOnLocalPrefixRefused`). A tunnel pair is
operator-fixed only when it is exactly a traffic selector its operator configured:
a tunnel peer with no selector that proposes the pair between its own configured
addresses beside a full-tunnel peer is refused, whichever arrives second
(`TestRFC3948TunnelPeerOwnHostPairBesideFullTunnelRefused`). A
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
peer whose `remote-address` is a DNS hostname, or whose local address is chosen
by the route or the interface, takes its configured host from the pair its connect
cycle measured: the address the name resolved to and the address it sends from
(`cycleRemoteAddress`, `cycleLocalAddress`, `engine/cycle_addr.go`). An SA no cycle
measured, whose leaf is not an address literal, has no configured host, so its
transport-mode pair is ordered only when it is exactly one of its configured
traffic selectors.

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
to the configuration's original endpoints. Its replacement inbound state's watch
uses those mapped ports when either path turns bare ESP into ESP-in-UDP, just as
its kernel state does.

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
the initiator childless. It starts it too when this node could not install or bind
the IKE_AUTH Child SA (`errChildInstall` or `errChildBinding`: a refused
`InstallSA`, or a `vti` bind that `resolveIfID` cannot resolve), and when it could
not build that Child SA (`errChildProposal`, `errChildKeying`, `errChildAddress`,
or no peer SPI recorded), every cause `createFirstChildSA` answers but
`errChildNoIKEKeys`, the IKE SA's own failure, which still ends it. Only the
initiator resolves the binding there: a responder resolved it in IKE_AUTH
(`selectAuthChildSA`). In that case it
first sends a Delete for the ESP pair (`sendDeleteESP`), so the peer holds no
Child SA this node does not, and keeps the IKE SA rather than end it: ending it
re-ran the whole handshake about once a second while the cause lasted. Its warning
carries `childInstallRefusedFix`, the fix of the operator error the refusing
producer wrote, such as the ESP form receiver's TUN refusal. The owner tick calls
`serviceChildCreate`: the first
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

<!-- source: internal/component/ike/engine/established.go -- runEstablished, the errChildInstall and errChildBinding arm, the errChildNoIKEKeys arm and the build-failure arm -->
<!-- source: internal/component/ike/engine/create_child.go -- childCreateSchedule, childCreateFirst, childCreateMax, childInstallRefusedFix, errChildProposal, errChildKeying, errChildNoIKEKeys -->
<!-- source: internal/component/ike/engine/child.go -- createFirstChildSA -->

A Child SA rekey answer behaves the same way. An answer carrying an error
notification is the peer's refusal and is routed by the notification. An answer
carrying none means the peer installed the new pair, so when this node refuses it
(no Nr, a selector answer it will not adopt, an offer it never made), the rekey
response handler sends a Delete listing its own inbound SPI of the new pair (RFC
7296 Section 1.4.1), logs `childRekeyFixAnswer`, and keeps the Child SA in use.

A rekey installs the mode its own exchange negotiates, and a Child SA rekey is a
Child SA creation for that purpose. `initiateChildRekey` puts
`N(USE_TRANSPORT_MODE)` after `N(REKEY_SA)` when the retired SA runs transport mode
(RFC 7296 Section 1.3.3: "when rekeying a transport mode SA, the
USE_TRANSPORT_MODE notification will be used"). `respondChildRekey` computes the
mode once: transport only when the request carries the notification and the retired
SA runs transport, tunnel otherwise (Section 1.3.1: "Except when using this option
to negotiate transport mode, all Child SAs will use tunnel mode."). That one value
gates the Section 2.23.1 substitution and single-address rule, builds the
replacement (`newRekeyedChild` takes it), and decides the echo, so a request without
the notification for a transport SA is answered without it and installs tunnel at
both ends, and a transport request on a tunnel SA is declined the same way. Behind a
NAT the same request installs nothing: judged in tunnel mode, the proposal gets no
Section 2.23.1 substitution, so it names the NAT's public address for this node and
falls outside the retired SA's host pair, and the Section 2.9.2 floor (the rekeyed
SA's selectors are the scope in use) refuses it with TS_UNACCEPTABLE, which keeps the
transport Child SA running (`TestChildRekeyWithoutTransportNotifyBehindNATIsRefused`). A
request without it asks for tunnel mode, and a peer behind a NAT
then skips its own Section 2.23.1 substitution: strongSwan 5.9.14 answered such a
rekey with TS_UNACCEPTABLE after a NAT rebind. An answer to a transport-mode rekey
that leaves the notification out established a tunnel-mode SA at the peer (Section
1.3.1), so `applyChildRekeyResponse` refuses it with
`errChildRekeyTransportDeclined`, and the handler deletes the new pair, holds the
next attempt for `rekeyRefusedWait` (the peer's configuration decides the answer,
so an immediate retry draws the same one), and logs `childRekeyFixTransport`. An
answer that keeps transport mode is read in transport mode whatever the IKE SA
went through: `applyChildRekeyResponse` passes the retired SA's mode to
`recordInitiatorSelectorsIn`, which substitutes a NATed answer back into this
node's address space (Section 2.23.1) and refuses more than one pair
(`errTSTransportPairs`).

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

The exception is a reauthentication the peer started (RFC 7296 Section 2.8.3:
new IKE SA, new Child SAs, then the Delete of the old IKE SA). Once the
successor's Child SA is installed, the peer's Delete of the old IKE SA, or the
supersede signal, whichever the owner loop reads first, ends the old SA's owner
loop and the run loop adopts the successor instead of reconnecting:
`runResponder` for a `respond` peer, `adoptPeerStartedSuccessor` in
`runInitiator` for an `initiate` peer. A Delete that arrives after the adoption
names an SA no longer in the SATable. A successor still without a Child SA is
deleted and the session reconnects.

<!-- source: internal/component/ike/engine/reauth_receive.go -- adoptPeerStartedSuccessor -->

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
