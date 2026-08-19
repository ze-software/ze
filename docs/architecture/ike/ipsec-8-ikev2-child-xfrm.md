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

**The XFRM interface id comes from config, not from a runtime lookup.** The
engine runs as a plugin subprocess and has no access to the interface backend.

<!-- source: internal/component/ike/engine/established.go -- resolveIfID -->

**Local VPP API message types, not the vendored govpp binapi.** The IPsec binapi
module is not in the vendored dependency, so the backend compiles against local
types. It has the right structure and cannot run until that module is vendored.

## Vendored netlink patch

The pinned `github.com/vishvananda/netlink` release lacks three XFRM
corrections. Ze records them in
`scripts/dev/patches/netlink-xfrm-fixes.patch`:

- The state reader decodes `XFRMA_REPLAY_ESN_VAL`, restores the replay window,
  and reports the `XFRM_STATE_ESN` flag.
- The state writer uses `XFRMA_REPLAY_ESN_VAL` for replay windows of more than 32. It
  sets `XFRM_STATE_ESN` only when the SA uses extended sequence numbers.
- The policy writer copies the selector family into any template that carries no
  destination, which in ze is every transport-mode template. An explicit
  destination still determines the template family.

Keep the patch and its drift test outside `vendor/` because `go mod vendor`
replaces that directory. Run this command from the repository root after every
vendor refresh:

```sh
git apply scripts/dev/patches/netlink-xfrm-fixes.patch
```

`TestNetlinkXFRMPatchApplied` uses `git apply --reverse --check` to require the
exact corrections, with their context lines. It runs in `./scripts/dev`, which
`ZE_PACKAGES` covers, so `make ze-unit-test` runs it as a `ze-precommit-verify` stage.
`make ze-unit-pkg-test PKG=./scripts/dev` is the narrow way to run it alone.

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

**A new peer field must join the comparison.** `peerConfigChanged` decides
whether to restart a session. A field added to the peer struct and left out of
that comparison means config changes to it never take effect.

<!-- source: internal/component/ike/engine/reconcile.go -- reconcilePeers, peerConfigChanged, Stop -->

**One field cannot carry both the KEYMAT role and the selector orientation.**
`ChildSA.Selectors` is stored in TSi/TSr order. `ChildSA.SelectorsLocalIsTSi`
says which half of each pair is this node's side, and the policy install reads
it. `ChildSA.LocalIsInitiator` answers a different question: which KEYMAT half
keys this pair (RFC 7296 Section 2.17). That field flips each time the other end
starts the rekey, and the stored selectors do not move.

Reading it to orient those selectors installed a port-swapped policy at the first
peer-initiated rekey. The kernel then protected the peer's port as this node's.
`samePolicySelector` stopped recognizing the pair the replacement shares its
policy with, so retiring the superseded pair removed the live pair's policy. The
orientation is written once, at creation, from the IKE_AUTH role, and
`newRekeyedChild` inherits it unchanged.

<!-- source: internal/component/ike/engine/child.go -- ChildSA.SelectorsLocalIsTSi, selectorPort -->
<!-- source: internal/component/ike/engine/rekey.go -- newRekeyedChild -->

**Key material flows through four hops and each one must clear.** Derivation,
the child key struct, the SA parameters, then the install call. Any new path
inherits the clear chain.

<!-- source: internal/component/ike/crypto/keys.go -- DeriveChildSAKeys, ChildSAKeys.Clear -->
<!-- source: internal/component/ike/engine/child.go -- ChildSA.Clear -->

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

<!-- source: internal/component/ike/dataplane/policy_owner.go -- policyOwners.claim, policyOwners.release, PolicyOwnedError -->
<!-- source: internal/component/ike/engine/child.go -- childPolicyParams, firstSharingSelector, samePolicySelector -->

## Inbound classification

`inbound.go` classifies inbound INFORMATIONAL and CREATE_CHILD_SA messages for
an established SA. The negotiation itself is driven by the owner loop, described
in `docs/architecture/ike/ipsec-13-rekey-wire.md`.

<!-- source: internal/component/ike/engine/inbound.go -- inbound message classification -->
<!-- source: internal/component/ike/engine/delete.go -- Child SA teardown over INFORMATIONAL -->
<!-- source: internal/component/ike/engine/bypass.go -- IKE control-plane bypass policies -->
