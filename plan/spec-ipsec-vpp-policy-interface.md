# Spec: ipsec-vpp-policy-interface

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | fixit-vpp-ipsec-inoperable |
| Phase | - |
| Updated | 2026-08-10 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Nothing tells the VPP IPsec backend which VPP interface a Child SA's policy
applies to, so no IKE-produced policy can be installed on that backend. Give the
interface a source.**

VPP has no node-wide security policy database. A policy lives in an SPD, and an
SPD takes effect only on the interfaces it is bound to (`ipsec_interface_add_del_spd`).
`vppPolicyInterface` (`internal/component/ike/dataplane/vpp_policy.go`) therefore
refuses `SPParams.IfIndex` 0, which is correct: sw_if_index 0 is a real VPP
interface, so reading an unset field as "any" would program the wrong one.

`childPolicyParams` (`internal/component/ike/engine/child.go`) leaves the field at
its zero value. It sets `IfID` (the XFRM if_id) and never `IfIndex`. So every IKE
Child SA policy is refused by that backend today.

The refusal is right. The gap is that nothing supplies the interface.

## Why the answer is not one assignment

Three producers were read on 2026-08-10, and each rules out the cheap fix.

| Producer | What it says |
|----------|--------------|
| `SPParams.IfIndex` (`internal/component/ike/dataplane/dataplane.go`) | On Linux the field is an XFRM `sel.ifindex`, and IKE leaves it 0 deliberately, meaning node-wide. A non-zero value SCOPES the Linux policy to one interface, so filling it in the engine changes behavior on the production XFRM backend |
| `buildIPsecPolicies` (`internal/plugins/ospf/ipsec_install.go`) | The one producer of a non-zero `IfIndex` in the tree. It passes a KERNEL ifindex, from netlink. A kernel index is not a VPP sw_if_index, so the same number names a different interface on each backend |
| `childPolicyParams` (`internal/component/ike/engine/child.go`) | The IKE engine holds addresses, never interfaces. `ChildSA` carries `LocalAddr` and `RemoteAddr` and no interface at all |

So the backend cannot be handed a number that means the same thing on both
dataplanes, and the engine has no interface to hand it.

## The two candidate designs

Neither is a defect fix, and choosing between them is the work this spec exists for.

| Design | What it needs | Open question |
|--------|---------------|---------------|
| An operator names the interface, in config | A YANG leaf under the IPsec configuration, and a name-to-`sw_if_index` lookup at install time | Ze has no operator-facing IPsec dataplane selection at all yet (`plan/spec-ipsec-dataplane-selector.md`), so this leaf would arrive before the leaf that selects the backend it configures |
| The backend resolves the interface from the tunnel's local address | An interface dump plus its addresses, inside the backend | Which interface, when the local address is on several, or is a loopback with the traffic leaving elsewhere. VPP also offers a different model for this shape, `ipsec_tunnel_protect` over an ipip interface, which binds no SPD at all |

## Required Reading

| Document | Why |
|----------|-----|
| `ai/rules/config.md` | Whether the interface is a YANG leaf or a derived value |
| `ai/rules/protocol.md` | A backend that cannot apply the config exactly must reject rather than approximate, which is what the refusal does today |
| `plan/spec-ipsec-dataplane-selector.md` | The sibling gap: no config surface selects the IPsec dataplane either |

## Acceptance Criteria

| Id | Criterion |
|----|-----------|
| AC-1 | An IKE Child SA policy reaches the VPP backend naming a VPP interface, and the SPD is bound to it |
| AC-2 | The value the XFRM backend reads as an interface-scoping `sel.ifindex` is unchanged, so the production backend behaves as it does today |
| AC-3 | An interface that cannot be resolved is REFUSED, and the error names what would have been programmed instead |
| AC-4 | The real-VPP evidence run drives the path from an IKE-shaped policy rather than from a hand-set index |

## Owed on the first touch of `dataplane.go`: the `SPParams.IfIndex` comment under-counts

The comment on `SPParams.IfIndex` (`internal/component/ike/dataplane/dataplane.go`) says
"Three refusals keep that inert today" and names them: the `SADirFwd` direction refusal
in `spdEntry`, the transport-mode refusal in `vppProtectMode` (both `vpp_policy.go`), and
the unset-`Dir` refusal in `vppUnsupportedSA` (`vpp.go`).

**There is a fourth on the same path.** `buildIPsecPolicies`
(`internal/plugins/ospf/ipsec_install.go`) sets no `SAID`, and `SPActionProtect` is the
zero value of `SPParams.Action`, so every OSPF policy reaches `spdEntry` as a PROTECT
policy with `SAID` 0, which `spdEntry` refuses. It is reached only after
`vppProtectMode`, so widening the mode refusal alone still leaves it closed.

The comment's point survives: the count is an under-count, not a false claim. Correct the
count on the first edit of that file. It was not corrected in `spec-fixit-vpp-ipsec-inoperable`
because the file was hash-pinned by a clean review artifact, and re-opening a clean gate
for a count is the wrong trade (`ai/rules/planning.md`, a finding in the record is not a
finding in the product).

## What this spec makes reachable: the rekey policy leak, and the SA under it

**Building this turns a latent defect live, so it is in scope here and it is not a
separate find.** No IKE policy reaches the VPP backend today, so no IKE rekey does
either. Supplying the interface removes exactly that block.

`xfrmBackend.InstallPolicy` (`internal/component/ike/dataplane/xfrm_linux.go`) UPSERTS
with `XfrmPolicyUpdate`, and `removeChildSAExcept`
(`internal/component/ike/engine/child.go`) is built on that behaviour: a make-before-break
rekey holds ONE shared policy per direction, so it passes `dropPolicy=false` and never
sends the retired child's policy back to the dataplane.

The VPP backend does not upsert. `InstallPolicy`
(`internal/component/ike/dataplane/vpp_policy.go`) sends `ipsec_spd_entry_add_del_v2` with
`IsAdd` true and appends the entry to `b.spdEntries`. VPP matches an SPD entry by its
whole content, and a replacement child's entry differs in `SaID`: a rekey carries a new
SPI (RFC 7296 Section 2.8), which is a new `saIdentity` and a new SAD id (`allocSadID`,
`vpp.go`). So one rekey would leave VPP holding two entries per direction, and the retired
one would stay in `b.spdEntries` and in VPP until `Close`, naming a retired SA whose
reference VPP is still counting.

**The retired SA is leaked past `Close`, which is worse than the entry.** Found on
2026-08-10 by reading the whole chain. `removeChildSAOutgoing` and
`removeChildSAIncoming` (`internal/component/ike/engine/child.go`) call
`RemovePolicyParams` only when `dropPolicy` is true, and call `RemoveSA` unconditionally.
On a rekey the retired SA's PROTECT entry therefore still holds VPP's reference when
`RemoveSA` runs, `deleteSAD` returns retval 0 with the SA still installed (the behavior
`removeInstalled` measured on VPP v26.06), and `RemoveSA` drops the identity from
`b.sadIDs` on that success. `removeInstalled` iterates `b.sadIDs` for its SA deletes, so
the retired identity is no longer in the population it walks and no delete is ever sent.
The SA lives until VPP restarts. Recorded in
`plan/journal/false-synchronization-claim.md`.

| Id | Criterion |
|----|-----------|
| AC-5 | A Child SA rekey over the VPP backend leaves ONE SPD entry per direction naming the live SA, and the retired entry AND the retired SA are gone: from VPP, from `b.spdEntries` and from `b.sadIDs`. A delete VPP reports retval 0 for while keeping the object is not read as success. Proven against a real VPP |

Two shapes answer it, and choosing between them is part of this spec's design work: give
the VPP backend a replace step keyed on the selector, or give the engine a way to say
"this policy now names a different SA" that both backends honor.

## Known Limitations

Until this is built, **the VPP IPsec backend cannot be driven by IKE.** It
installs SAs, and every policy IKE produces for them is refused. This is recorded
as the headline Known Limitation of `spec-fixit-vpp-ipsec-inoperable`.

**This spec is one of two preconditions on the dataplane selector.**
`plan/spec-ipsec-dataplane-selector.md` must not land until this spec AND an
ESP-on-the-wire harness exist. See "Release judgment" in
`spec-fixit-vpp-ipsec-inoperable`.
