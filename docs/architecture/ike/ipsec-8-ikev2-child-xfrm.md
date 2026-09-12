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

**The XFRM interface id comes from config, not from a runtime lookup.** The
engine runs as a plugin subprocess and has no access to the interface backend.

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
worked only if an XFRM interface happened to carry that id. It is now the value
from config, and 0 means unbound.

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
