# Spec: ipsec-opaque-selector-port-mask

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the "own spec" in the quoted ruling IS this spec. Create `plan/deferrals/ipsec-opaque-selector-port-mask.md` on the first deferral) |
| Updated | 2026-08-01 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make an exact match on port 0 expressible in the Linux XFRM policy selector. Ze can then
program the OPAQUE traffic selector of RFC 7296 Section 3.13.1 instead of refusing it.

**Ze is conformant today.** RFC 7296 Section 3.13.1 states, at
`rfc/full/rfc7296.txt:6074-6079`:

> Systems that are complying with [IPSECARCH] that wish to indicate "ANY" ports MUST set the
> start port to 0 and the end port to 65535; note that according to [IPSECARCH], "ANY"
> includes "OPAQUE". Systems working with [IPSECARCH] that wish to indicate "OPAQUE" ports,
> but not "ANY" ports, MUST set the start port to 65535 and the end port to 0.

The MUST binds a system that WISHES to indicate OPAQUE. Ze never wishes to, and it says so
in two places rather than in silence. The encoder implements the 65535/0 form, and a
tagged pair proves it. `ai/rules/protocol.md` makes the refusal the correct answer.
An OPAQUE selector installed as an any-port policy protects more traffic than the peers
negotiated. RFC 7296 Section 2.9 forbids that widening.

**This spec removes the reason for the refusal.** After it lands, `RFC7296-3.13.1-3` has a
production sender rather than an encoder-only proof, and both refusals relax.

### The defect, stated exactly

The kernel selector `xfrm_selector` carries a port and a SEPARATE port mask, in each
direction. The vendored library models only the port. It drops both mask fields from its
public type and re-derives them from the port value:

| Where | What it does |
|-------|--------------|
| `vendor/github.com/vishvananda/netlink/nl/xfrm_linux.go` | `XfrmSelector` declares `Dport`, `DportMask`, `Sport` and `SportMask`, all big endian |
| `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` | the public `XfrmPolicy` declares `DstPort` and `SrcPort`, and NO mask field of any kind |
| `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` | `selFromPolicy` writes the ports, then sets `DportMask` to `^uint16(0)` only when `Dport` is non-zero, and `SportMask` to `^uint16(0)` only when `Sport` is non-zero |
| `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` | `parseXfrmPolicy` reads `Dport`, `Sport`, `Proto` and `Ifindex` back into the public struct, and reads NEITHER mask |

So exactly two port matches are expressible through this API. Any port is port 0 with mask
0. One port N above zero is port N with mask 0xffff. A request for exactly port 0 yields
mask 0, which the kernel reads as any port.

The read path drops the masks as well. A listed policy therefore reports `DstPort` 0 for
"any port" AND for "exactly port 0". The two are indistinguishable through the public API,
so a caller cannot even OBSERVE which one the kernel holds.

### Provenance (do not delete)

Thomas ruled on 2026-08-01 in two steps.

Step one. `RFC7296-3.13.1-3` lands in the rfcgate-1b RFC 7296 pilot spec as
encoder-proven. The tag states that Ze refuses OPAQUE at config commit and at negotiation,
because the dataplane cannot express it exactly.

Step two, in his words:

> Fixing it properly means changing how that layer derives the mask so an exact port-zero
> match is representable, which is vendored territory and would need its own spec and a
> QEMU proof on a real kernel. We should do it, if a patch must be sent upstream we will
> also do that.

This spec is step two. The intent to send an upstream patch is part of the ruling. This
spec must keep it, and no phase can drop it.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/protocol.md` - why the refusal is correct today
  → Constraint: a backend that cannot deliver the operator's config EXACTLY must fail at
    `ze config verify` or `ze config commit` with a clear error. It must never approximate.
  → Decision: the refusals are removed only after the exact form is expressible. They are
    never removed to reach a green row.
- [ ] `ai/rules/platform-linux.md` - Linux-only code needs a real kernel
  → Constraint: the whole change is `//go:build linux`, so a QEMU integration test is
    mandatory. "Needs hardware" is never a reason to skip one.
  → Constraint: a package carrying `//go:build integration && linux` is auto-enrolled in
    `ZE_QEMU_INTEGRATION_PKGS` (`mk/test-integration.mk`). No new make target is needed
    for a test placed in `internal/component/ike/dataplane/`.
- [ ] `ai/rules/evidence.md` - a foreign system's semantics come from its source
  → Constraint: every claim about the kernel comes from the kernel, and every claim about
    the library comes from the library. A binding stub documents a field's existence, never
    what the system does with it.
- [ ] `ai/rules/go-standards.md` - the Dependencies directive
  → Constraint: a new third-party import needs the user's answer first. This spec adds no
    dependency. It changes how an existing one is called, or what it contains.
- [ ] `ai/rules/interop-and-goal-validation.md` - the vacuity traps
  → Constraint: revert the change and confirm the test reddens. A test that asserts an
    absence, or that runs at an extreme, passes for the wrong reason.
- [ ] `ai/rules/evidence.md` - a zero value must never be a valid-looking answer
  → Decision: this defect IS the zero-value trap, in a vendored library. A derived zero
    mask reads as a legitimate "any port" and nothing downstream can tell.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - IKEv2. Section 3.13.1 holds `RFC7296-3.13.1-1` through `-3`
  (`:479-481`)
  → Constraint: OPAQUE is start 65535 and end 0 (`rfc/full/rfc7296.txt:6074-6079`).
  → Constraint: ANY includes OPAQUE, so an any-port policy is a SUPERSET of an opaque-port
    policy. Answering OPAQUE with ANY widens the protected set.
  → Constraint: Section 2.9 forbids a responder from widening beyond what it received.
- [ ] `rfc/short/rfc4301.md` - Security Architecture, the [IPSECARCH] reference. MUST CREATE
  if absent
  → Constraint: Section 4.4.1.1 defines the OPAQUE selector value and says when a port is
    not available. This is the definition of what Ze must match, and it decides the
    behavioral half of the QEMU proof.

**Key insights:** (minimal context to resume after compaction)
- Ze is conformant. This spec removes a platform limit, and it does not fix a violation.
- The fault is a modelling fault in the library, not a kernel limit. The kernel carries the
  mask. The library declines to.
- The fix has two halves. The write path must carry the mask. The read path must return it,
  or the QEMU proof has no oracle.

## Current Behavior (MANDATORY)

**Source files read:** (verified in the working tree on 2026-08-01)
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` - `selFromPolicy`,
  the derivation. Three call sites: `:156` (`xfrmPolicyAddOrUpdate`, used by
  `XfrmPolicyAdd` and `XfrmPolicyUpdate`), `:286` (`xfrmPolicyGetOrDelete`, used by
  `XfrmPolicyDel` and `XfrmPolicyGet`), and
  `vendor/github.com/vishvananda/netlink/xfrm_state_linux.go` (the STATE selector,
  `x->sel`).
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` -
  `xfrmPolicyAddOrUpdate` builds the whole message: the policy info, the template array,
  the mark attribute and the if_id attribute. A Ze-side replacement must reproduce all of
  it.
- [ ] `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` -
  `xfrmPolicyGetOrDelete` calls the SAME derivation for delete. The delete selector must
  equal the installed selector byte for byte.
- [ ] `vendor/github.com/vishvananda/netlink/nl/xfrm_linux.go` - the kernel
  selector. `XfrmUserpolicyInfo` (`nl/xfrm_policy_linux.go`) embeds it.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmSelectorPort`. It
  accepts only mask 0 and mask 0xffff, and it REFUSES port 0 with mask 0xffff. Its comment
  names `selFromPolicy` as the reason.
- [ ] `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmPolicyFromParams`, the
  single builder shared by install and delete. `InstallPolicy`,
  `RemovePolicyParams` and `RemovePolicy` are its callers.
- [ ] `internal/component/ike/dataplane/dataplane.go` - `PortMatch`, a value plus a
  mask. `AnyPortMatch` is mask 0. `ExactPortMatch` is mask 0xffff. The two-field shape is
  the kernel's, and it already carries what the library drops.
- [ ] `internal/component/ike/dataplane/dataplane.go` - `SPParams.SrcPort` and
  `.DstPort`, the seam between the IKE engine and both dataplane backends.
- [ ] `internal/component/ike/engine/child.go` - `selectorPort`. It maps
  `ipsec.PortOpaque` to `ExactPortMatch(0)`, which is the value the backend then refuses.
  The mapping is already correct. Only the backend is not.
- [ ] `internal/component/ike/ipsec/traffic_selector.go` -
  `checkPortProgrammable`, the CONFIG-COMMIT refusal. `PortOpaque` returns an error naming
  the derived mask as the reason.
- [ ] `internal/component/ike/engine/ts_narrow.go` - `programmableSelector`, the
  NEGOTIATION refusal. A peer that proposes OPAQUE only finds no permitted subset and is
  answered with TS_UNACCEPTABLE.
- [ ] `internal/component/ike/engine/ts_narrow_test.go` -
  `TestPortEncodingFollowsSection3131`, the tagged pair for all three Section 3.13.1 rows.
  The `RFC7296-3.13.1-3` tags sit at `:336` and `:361`, and they carry a
  `rfc-test-change-approved` marker dated 2026-08-01.
- [ ] `internal/component/ike/dataplane/xfrm_transport_integration_linux_test.go` - the
  worked example of a kernel-backed test. `TestXFRMSinglePortSelectorReachesTheKernel`
  (`:157`) installs a port-restricted policy and reads it back.
  `TestXFRMOpaquePortIsRefused` pins the current refusal.
- [ ] `scripts/dev/reapply-updater-fixes.py` - the repository's precedent for a local patch
  on a vendored dependency. It names the three artifacts a patched vendor tree needs.
- [ ] `go.mod:22` and `vendor/modules.txt` - `github.com/vishvananda/netlink v1.3.1`, a
  direct dependency, a released semantic version, with no `replace` directive anywhere in
  `go.mod`.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A policy with no port constraint programs the same bytes it programs today. Every IKE
  peer configured before ports existed must be unaffected.
- A policy with one port above zero keeps mask 0xffff.
  `TestXFRMSinglePortSelectorReachesTheKernel` stays green.
- The delete selector keeps matching the installed selector. The kernel identifies a policy
  by its whole selector (`internal/component/ike/dataplane/xfrm_linux.go`).
- Every caller of `netlink.XfrmPolicyAdd`, `XfrmPolicyDel` and `XfrmPolicyList` outside the
  IKE dataplane keeps working. They are read-only or port-free:
  `internal/plugins/iface/netlink/xfrm_linux.go`,
  `internal/plugins/ospf/doctor_ipsec_linux.go`, and the OSPF integration test at
  `internal/plugins/ospf/ipsec_integration_linux_test.go`.
- The OSPFv3 state selector of RFC 4552 keeps its current bytes
  (`internal/component/ike/dataplane/xfrm_linux.go`). It sets no port.
- Every `test/ipsec/*.ci` stays green.

**Behavior to change:** (only what the user asked for)
- The XFRM backend expresses an exact match on port 0, in both directions.
- `checkPortProgrammable` accepts `PortOpaque` instead of refusing it.
- `programmableSelector` keeps an OPAQUE selector instead of dropping it.
- `RFC7296-3.13.1-3` gains a production sender.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: an operator sets `port opaque` on a traffic selector under an IPsec peer.
  `ipsec.ParsePortSelector` (`internal/component/ike/ipsec/traffic_selector.go`)
  already accepts the word.
- Wire: a peer proposes a traffic selector whose start port is 65535 and whose end port is
  0. `ipsec.PortSelector` parsing (`traffic_selector.go`) already recognizes the form.

### Transformation Path
1. Config parse or wire parse yields `ipsec.PortSelector{Form: PortOpaque}`.
2. Commit validation. `checkPortProgrammable`
   (`internal/component/ike/ipsec/traffic_selector.go`) refuses the form TODAY. After
   this spec it accepts it.
3. Negotiation. `programmableSelector` (`internal/component/ike/engine/ts_narrow.go`)
   drops the form TODAY. After this spec it keeps it.
4. Selector mapping. `selectorPort` (`internal/component/ike/engine/child.go`) returns
   `dataplane.ExactPortMatch(0)`. Unchanged.
5. Backend translation. `xfrmSelectorPort`
   (`internal/component/ike/dataplane/xfrm_linux.go`) refuses port 0 with a full mask
   TODAY. After this spec it carries the mask forward.
6. Netlink message build. The port and the mask reach `nl.XfrmSelector`.
7. Kernel. The policy matches only a packet that presents port 0.
8. Read back. `netlink.XfrmPolicyList` returns the port AND the mask, so a test and
   `show vpn ipsec sa` can tell an opaque-port policy from an any-port policy.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| IKE engine ↔ dataplane | `dataplane.PortMatch` already carries a mask | No |
| Ze ↔ vendored netlink | the public `XfrmPolicy` type, or a Ze-built netlink message | No |
| Vendored netlink ↔ Linux kernel | `XFRM_MSG_NEWPOLICY` carrying `xfrm_selector` | No |
| Ze ↔ upstream project | a patch offered to `github.com/vishvananda/netlink` | No |

### Integration Points
- `dataplane.PortMatch` (`internal/component/ike/dataplane/dataplane.go`) needs no
  change. It already models the kernel's two-field shape.
- `nl.NewNetlinkRequest`, `NetlinkRequest.AddData`, `NetlinkRequest.Execute`,
  `nl.XfrmUserpolicyInfo` and `nl.XfrmSelector` are ALL exported
  (`vendor/github.com/vishvananda/netlink/nl/nl_linux.go`, `:505`, `:519`,
  `nl/xfrm_policy_linux.go`, `nl/xfrm_linux.go`). A Ze-side wrapper is therefore
  possible without a fork.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## The three routes

The design phase picks one. All three are technically available. The trade-off is
maintenance, not feasibility.

| Route | What it is | Cost | Risk |
|-------|-----------|------|------|
| 1. Patch the vendored copy | Add the mask to `XfrmPolicy` and to `selFromPolicy` and `parseXfrmPolicy` inside `vendor/` | A re-apply script, an upstream patch file, and a marker test. The repository already runs this exact pattern | `go mod vendor` reverts it. A silent revert programs an any-port policy where an opaque-port policy was negotiated, which is the widening this whole spec exists to stop |
| 2. Ze-side wrapper | Build `XFRM_MSG_NEWPOLICY` and `XFRM_MSG_DELPOLICY` in Ze, through the exported `nl` package | Reproduce `xfrmPolicyAddOrUpdate` and `xfrmPolicyGetOrDelete`: message header, policy info, template array, mark, if_id | Ze owns a second netlink encoder that must stay byte-compatible with the library one, forever. The install and the delete path must agree exactly, or a delete silently matches nothing |
| 3. Upstream only | Send the patch, wait for a release, bump the dependency | Lowest long-term cost, and it helps every other user | MEASURED latency, not merely unbounded. Upstream `main` runs about fourteen months ahead of its newest tag. See the upstream section below |

**These routes are not exclusive, and the owner's ruling already implies the combination.**
Route 3 is mandatory whatever else is chosen, because he said "if a patch must be sent
upstream we will also do that". Route 1 or route 2 is the bridge that lets the feature land
before an upstream release exists.

**A vendored fork that silently diverges from upstream is the liability.** The repository
already knows this and has an answer. `scripts/dev/reapply-updater-fixes.py` carries local
fixes to `vendor/github.com/gokrazy/updater/updater.go`, alongside
`scripts/dev/gokrazy-updater-upstream.patch` (the same fixes as an upstream patch) and
`TestUpdaterHardeningMarkersPresent`
(`internal/appliance/updater_hardening_markers_test.go`), which fails when the re-apply
is forgotten. Its own docstring states the exit condition: "Once merged upstream, bump the
dependency and delete both this script and the patch file." Route 1 must copy all three
artifacts, or it must not be chosen.

### The minimal upstream patch

Expressed as a table, because `ai/rules/spec-no-code.md` forbids code in a spec.

| Element | Shape | Why |
|---------|-------|-----|
| Two new fields on `XfrmPolicy` | `SrcPortMask` and `DstPortMask`, each an optional 16-bit value | The kernel has them. The public type must not hide them |
| Optionality | A pointer, or a separate "explicit" flag. A bare `uint16` is WRONG | A bare zero would be indistinguishable from "not set", which is the same zero-value trap the patch exists to remove |
| Write path | `selFromPolicy` uses the explicit mask when it is set, and keeps the current derivation when it is not | Every existing caller keeps its exact current bytes. The patch is additive |
| Read path | `parseXfrmPolicy` sets both fields from `msg.Sel` | Without this, a caller still cannot observe what the kernel holds, and no test can assert it |
| State path | `xfrm_state_linux.go` inherits the change through `selFromPolicy` | The state selector shares the derivation, so it shares the fix |

The patch is small, additive, and byte-identical for every current caller. Those three
properties are what upstream needs to see.

### The upstream project, and what it already knows

Upstream is `github.com/vishvananda/netlink`, under the Apache License 2.0
(`vendor/github.com/vishvananda/netlink/LICENSE:1-3`).

| Fact | Value | Evidence |
|------|-------|----------|
| Vendored version | `v1.3.1`, a direct dependency, marked `## explicit` | `go.mod:22`, `vendor/modules.txt` |
| Local divergence | none. `go.mod` holds zero `replace` directives, and `git status --porcelain vendor/github.com/vishvananda/` is empty | measured 2026-08-01 |
| Latest upstream release | `v1.3.1`, published 2025-05-09. It is the version already vendored | releases API, fetched 2026-08-01 |
| The defect on upstream `main` | LIVE. `XfrmPolicy` carries no mask field. `selFromPolicy` still sets each mask only when its port is non-zero. `parseXfrmPolicy` still copies six selector fields and neither mask | `raw.githubusercontent.com/vishvananda/netlink/main/xfrm_policy_linux.go`, fetched 2026-08-01 |
| An existing issue or pull request | NONE. A repository search for `DportMask OR SportMask OR selFromPolicy OR "port mask"` returns five items, all pull requests. The only `selFromPolicy` hit is #120, closed in 2016, and it fixes a different bug | GitHub search API, 2026-08-01 |
| Contribution process | undocumented. There is no `CONTRIBUTING.md`, no pull-request template, and no contribution section in the README. The process is a pull request against `main` | GitHub contents API, 2026-08-01 |
| Merge activity | active. The most recent commit on `main` is `4e35dc940`, dated 2026-06-29 | commits API, 2026-08-01 |

**Upstream does not know. This is a new report, and the patch would be the first.**

**Merges are active. Releases are not, and that decides the routing.** The newest tag is
dated 2025-05-09 and the newest commit is dated 2026-06-29, so `main` runs about fourteen
months ahead of the last release. A merged patch therefore reaches a tagged release on no
predictable timeline, and Ze consumes tagged releases through `go.mod`.

**The conclusion follows from those two dates, and it is not a preference.** Route 3 alone
cannot deliver the feature. It is necessary, because the owner ruled it so and because it is
the only route that ends the divergence. It is not sufficient, because Ze cannot wait on a
release cadence it does not control. So the design phase chooses route 1 or route 2 as the
bridge, and sends the patch either way.


## Does any other selector field share the fault?

The verdict decides whether this is a bug or a design fault, so it is derived field by
field rather than asserted. Sources: `nl/xfrm_linux.go` for the kernel struct,
`xfrm_policy_linux.go` for the public type, `:103-126` for the write derivation, and
`:318-333` for the read path.

| Kernel field | Public representative | How it is set | Lossless? |
|--------------|-----------------------|---------------|-----------|
| `Daddr` | `Dst.IP` | direct | yes |
| `Saddr` | `Src.IP` | direct | yes |
| `Dport` | `DstPort` | direct | yes |
| `DportMask` | **none** | DERIVED from `Dport` | **no** |
| `Sport` | `SrcPort` | direct | yes |
| `SportMask` | **none** | DERIVED from `Sport` | **no** |
| `Family` | none | DERIVED from `Dst.IP`, and it defaults to IPv4 when `Dst` is nil | only when `Dst` is set |
| `PrefixlenD` | `Dst.Mask` | DERIVED through `Mask.Size()` | only when the mask is well formed |
| `PrefixlenS` | `Src.Mask` | DERIVED through `Mask.Size()` | only when the mask is well formed |
| `Proto` | `Proto` | direct | yes |
| `Ifindex` | `Ifindex` | direct | yes |
| `User` | none | never set | not exposed |

**The answer: one modelling fault, with two field instances and three call sites.** It is
the only field pair the library drops outright. The port mask is the only case where three
things hold together. The kernel carries a field. The caller holds information for it. The
public type has no way to say it.

The SHAPE of the fault, a zero value that silently reads as "any", recurs in `Family` and in
the two prefix lengths. Those two are different in kind. A well-formed `net.IPNet` carries
the prefix length exactly, so the derivation loses nothing. Ze already refuses the
ill-formed case. `programmableSelector`
(`internal/component/ike/engine/ts_narrow.go`) rejects a selector whose mask reports
zero ones and zero bits. `Family` defaults to IPv4 when `Dst` is nil, and Ze always sets
both `Src` and `Dst` (`internal/component/ike/dataplane/xfrm_linux.go`).

So the correct verdict is this. The fault is not a pattern across the selector, and it is
also not a one-line bug. It is one wrong decision about how to model a port selector. That
decision costs two fields, both directions, and both the write path and the read path. A
reader usually loses the read half, and the read half is what blocks the proof.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The Linux kernel honors a port mask of 0xffff with a port value of 0, and matches only a packet presenting port 0 | Not yet read. The kernel is the authority, and no Ze file states this | The whole spec is void: the exact form is unrepresentable in the kernel too, and the refusal is permanent | Read the kernel XFRM selector match path, then prove it in QEMU | unvalidated |
| A-2 | A packet that presents port 0 to the selector is what RFC 4301 Section 4.4.1.1 calls OPAQUE, and a non-initial IP fragment is one | Not yet read. `rfc/short/rfc4301.md` can be absent | The behavioral half of the QEMU proof has no stimulus, and only the read-back assertion survives | Read RFC 4301 Section 4.4.1.1, then generate the stimulus in a namespace | unvalidated |
| A-3 | `vendor/` is regenerated only by an explicit `go mod vendor`, so a patched file survives a normal build | `scripts/dev/reapply-updater-fixes.py` states exactly this hazard and this cadence | Route 1 is unusable and route 2 becomes mandatory | Read the makefile targets that touch `vendor/`, and confirm the marker test catches a revert | unvalidated |
| A-4 | No caller outside the IKE dataplane sets a port on an `XfrmPolicy` | Measured 2026-08-01. The only other callers are `internal/plugins/iface/netlink/xfrm_linux.go`, `internal/plugins/ospf/doctor_ipsec_linux.go` and the OSPF integration test, and all three list rather than install | An additive patch changes bytes for a caller this spec did not consider | Re-grep at implementation time | unvalidated |
| A-5 | The VPP dataplane needs no change, because it REFUSES the selector forms it cannot express rather than approximating them | `spec-fixit-vpp-ipsec-inoperable` closed 2026-08-10: the backend installs SAs on a real VPP, and `vppUnsupportedSA` (`internal/component/ike/dataplane/vpp.go`) refuses any SA carrying an explicit state selector. The earlier basis, that it programs nothing at all, is spent | The VPP backend must express or refuse the opaque form in the same work (`ai/rules/architecture.md`) | Read `vpp.go` and the VPP IPsec binary API | unvalidated |
| A-6 | `TestXFRMOpaquePortIsRefused` (`xfrm_transport_integration_linux_test.go`) is the only test pinning the refusal | Measured 2026-08-01 by grep for `Opaque` under `internal/` | A second test pins the old behavior and reddens unexpectedly | Re-grep at implementation time | unvalidated |
| A-7 | Upstream accepts an additive, backward-compatible patch | Partly established. Upstream merges actively and has no documented process, so the patch is a pull request against `main`. Acceptance itself is unmeasured | Route 3 stalls, and route 1 or route 2 becomes permanent rather than a bridge | Send the patch and record the outcome | unvalidated |
| A-8 | Ze cannot wait for an upstream release, because upstream releases lag `main` by about fourteen months | Measured 2026-08-01. The newest tag is dated 2025-05-09 and the newest commit is dated 2026-06-29 | Route 3 alone would be enough, and no bridge is needed | Re-measure the tag date and the commit date before the design phase closes | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A future `go mod vendor` silently reverts a route 1 patch. Ze then programs an ANY-port policy where an OPAQUE-port policy was negotiated. That is the exact widening RFC 7296 Section 2.9 forbids, and it is SILENT | None, unless a marker test exists | Copy all three precedent artifacts: the re-apply script, the upstream patch file, and a marker test that fails when the re-apply is forgotten. Treat the marker test as part of the feature, not as tidiness |
| R-2 | The install path carries the explicit mask and the delete path does not. The delete then matches no policy, and a stale policy stays in the kernel forever | An IPsec teardown leaves a policy behind, visible in `ip xfrm policy show` | `xfrmPolicyFromParams` is already the single shared builder for install and delete (`xfrm_linux.go`). Keep it single, and assert the delete in the QEMU test rather than only the install |
| R-3 | The QEMU test asserts a read-back that the read path cannot return, so it passes vacuously by comparing zero with zero | The test is green before the write-path fix lands | Land the READ path first. Then write a test that FAILS, then fix the write path. `ai/rules/testing.md` order, and it is the only order that proves the oracle works |
| R-4 | The test proves the bytes and not the matching. A kernel that accepts the mask but ignores it would still pass | None. The test is green | Add the behavioral assertion beside the read-back. A packet with a real port must NOT match, and a packet presenting port 0 must match |
| R-5 | The two refusals are relaxed before the dataplane can honor the form. Ze then accepts a config it cannot program | `ze config commit` accepts `port opaque` while the install still errors | Relax `checkPortProgrammable` and `programmableSelector` in the LAST phase, after the QEMU proof is green. Never earlier |
| R-6 | Only the destination mask is fixed. An opaque SOURCE port still installs as any-port | An asymmetric selector protects more traffic in one direction | Both directions in one change. `xfrmSelectorPort` is already called for both sides (`xfrm_linux.go`) |
| R-7 | The state selector path (`xfrm_state_linux.go`) is forgotten. RFC 4552 OSPFv3 selectors then keep the old derivation | Nothing today, because Ze sets no port on a state selector | Note it in the patch and assert the shared derivation covers it. It is a latent hazard rather than a live defect |
| R-8 | An OPAQUE policy matches far more or far less traffic than the operator expects. "Port 0" is not what an operator pictures | An operator reports traffic that is protected, or unprotected, against expectation | Document the meaning in the operator guide, sourced from RFC 4301 Section 4.4.1.1, before the config leaf is accepted |
| R-9 | Concurrent sessions edit `internal/component/ike/`. Line numbers move | A citation names a line holding different code | Every citation in this spec names its function. Relocate by function name before you quote a line |
| R-10 | An open upstream pull request rewrites the same file, and the patch conflicts. Pull request #1181, "Migrate to net/netip", is open against `xfrm_policy_linux.go` | The patch does not apply to `main` | Rebase the patch on `main` before you send it, and state in the pull request that the change is additive. Re-check the open pull request list at the moment you send it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An IPsec policy protects the wrong set of traffic. Too wide leaks nothing but violates the negotiated scope and RFC 7296 Section 2.9. Too narrow drops traffic the peers agreed to protect. A silently reverted vendor patch produces the wide case with no signal at all |
| How is it reverted? | A single commit revert, while the two refusals remain in place. Once the refusals are relaxed and an operator configures `port opaque`, a revert makes that configuration fail at commit, so the config must change too |
| Who else touches this path? | the rfcgate-1b RFC 7296 pilot spec (owns the `RFC7296-3.13.1-3` row and its tags), `plan/spec-ipsec-ipcomp.md` and `plan/spec-ipsec-auth-piggyback.md` (same engine), `spec-fixit-vpp-ipsec-inoperable` (the other dataplane backend), and `internal/plugins/ospf/` (an RFC 4552 state selector) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A config file sets `port opaque` on a traffic selector and is committed | → | `checkPortProgrammable` (`internal/component/ike/ipsec/traffic_selector.go`) accepts the form | `test/ipsec/ipsec-ts-port-opaque.ci` |
| A peer proposes a traffic selector with start port 65535 and end port 0 | → | `programmableSelector` (`internal/component/ike/engine/ts_narrow.go`) keeps it | `TestOpaquePortSelectorIsNarrowedRatherThanDropped` |
| An established Child SA with an opaque selector installs its policy | → | `xfrmSelectorPort` and `xfrmPolicyFromParams` (`internal/component/ike/dataplane/xfrm_linux.go`, `:149`) | `TestXFRMOpaquePortReachesTheKernelExactly` (QEMU) |
| The same Child SA is torn down | → | `RemovePolicyParams` (`internal/component/ike/dataplane/xfrm_linux.go`) | `TestXFRMOpaquePortPolicyIsRemoved` (QEMU) |
| A developer runs `go mod vendor` and forgets the re-apply | → | the vendor marker test | `TestNetlinkPortMaskPatchPresent` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A policy whose destination port match is port 0 with mask 0xffff | The kernel holds `Dport` 0 and `DportMask` 0xffff, read back through the public API |
| AC-2 | A policy with no port constraint | The kernel holds `Dport` 0 and `DportMask` 0, and the installed bytes are identical to those installed before this spec |
| AC-3 | A policy whose destination port match is port 179 with mask 0xffff | The kernel holds `Dport` 179 and `DportMask` 0xffff. `TestXFRMSinglePortSelectorReachesTheKernel` stays green |
| AC-4 | The same three cases applied to the SOURCE port | `Sport` and `SportMask` behave exactly as their destination counterparts |
| AC-5 | A policy with an exact port-0 match is installed, and a packet carrying a real transport port arrives | The packet does not match the policy |
| AC-6 | The same policy, and a packet presenting no transport port arrives | The packet matches the policy |
| AC-7 | A policy with an exact port-0 match is removed through `RemovePolicyParams` | The kernel holds no such policy afterward |
| AC-8 | A config sets `port opaque` on a traffic selector and is committed | The commit succeeds, and no error names the port form |
| AC-9 | A peer proposes an OPAQUE-only traffic selector against a Ze whose policy allows it | Ze answers with the OPAQUE selector, and it does not answer TS_UNACCEPTABLE |
| AC-10 | A peer proposes an OPAQUE selector against a Ze whose policy does NOT allow it | Ze still refuses, and the refusal names the policy rather than the dataplane |
| AC-11 | `go mod vendor` runs and the local patch is not re-applied | The marker test fails and names the re-apply command |
| AC-12 | The VPP backend receives an opaque port match | It expresses the match, or it refuses at `ze config verify` with an error naming the backend |
| AC-13 | The full change is reverted and the QEMU test runs | The QEMU test fails. A green test over a reverted change proves nothing |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `port opaque` on a traffic selector and commits | config parse → `checkPortProgrammable` → commit | `test/ipsec/ipsec-ts-port-opaque.ci` |
| 2 | Brings up a tunnel whose peer proposes OPAQUE ports | wire → `narrowTS` → `programmableSelector` → `selectorPort` → XFRM | `TestXFRMOpaquePortReachesTheKernelExactly` (QEMU) |
| 3 | Reads the installed selector | kernel → `XfrmPolicyList` → `show vpn ipsec sa` | `test/ipsec/ipsec-show-sa-port-opaque.ci` |
| 4 | Tears the tunnel down | `RemovePolicyParams` → kernel | `TestXFRMOpaquePortPolicyIsRemoved` (QEMU) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestXfrmSelectorPortCarriesAnExactZero` | `internal/component/ike/dataplane/xfrm_linux_test.go` | The translation accepts port 0 with mask 0xffff and produces a full mask | |
| `TestXfrmSelectorPortRejectsAPartialMask` | `internal/component/ike/dataplane/xfrm_linux_test.go` | A mask that is neither 0 nor 0xffff is still refused. The relaxation is narrow | |
| `TestOpaquePortSelectorIsNarrowedRatherThanDropped` | `internal/component/ike/engine/ts_narrow_test.go` | AC-9. `programmableSelector` keeps the form | |
| `TestCheckPortProgrammableAcceptsOpaque` | `internal/component/ike/ipsec/traffic_selector_test.go` | AC-8 | |
| `TestNetlinkPortMaskPatchPresent` | `internal/component/ike/dataplane/vendor_patch_test.go` | AC-11. Route 1 only. Modelled on `TestUpdaterHardeningMarkersPresent` | |
| `TestVPPBackendHandlesOpaquePort` | `internal/component/ike/dataplane/vpp_test.go` | AC-12 | |

`TestPortEncodingFollowsSection3131`
(`internal/component/ike/engine/ts_narrow_test.go`) carries the `RFC7296-3.13.1-3`
tags. This spec does NOT weaken it. It adds a production sender beside it, so the tag
comment must be corrected to say the form is now programmed rather than refused. That edit
needs the owner's approval under the `rfc-tagged-test` hook, and a fresh
`rfc-test-change-approved` marker.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Port value with a full mask | 0-65535 | 65535 | N/A | N/A |
| Port mask | 0 or 0xffff only | 0xffff | N/A | N/A |
| Port mask, refused middle values | 0x0001 to 0xfffe | N/A | N/A | N/A |
| RFC 7296 start port for OPAQUE | 65535 exactly | 65535 | 65534 | N/A |
| RFC 7296 end port for OPAQUE | 0 exactly | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-ts-port-opaque` | `test/ipsec/ipsec-ts-port-opaque.ci` | The operator configures an opaque port and the commit succeeds | |
| `ipsec-ts-port-opaque-reject` | `test/ipsec/ipsec-ts-port-opaque-reject.ci` | A partial port mask is still refused, and the error names the value | |
| `ipsec-show-sa-port-opaque` | `test/ipsec/ipsec-show-sa-port-opaque.ci` | The operator reads the installed selector and sees the opaque form | |

The `ipsec` suite runs inside `ze-precommit-verify` (`mk/test-functional.mk`), so a `.ci` there earns
a verify tier.

### QEMU Integration Tests (MANDATORY - `ai/rules/platform-linux.md`)

The change is `//go:build linux` and it programs the kernel, so a real kernel must prove it.
A unit test with a fake backend cannot tell a full mask from a derived one, which is the
same trap `kernelXFRMMode` records at
`internal/component/ike/dataplane/xfrm_transport_integration_linux_test.go`. The kernel
accepted a wrong mode silently, and only a read-back caught it.

| Test | Package | Asserts | Status |
|------|---------|---------|--------|
| `TestXFRMOpaquePortReachesTheKernelExactly` | `internal/component/ike/dataplane` | AC-1 and AC-4. The kernel holds port 0 with mask 0xffff, in both directions | |
| `TestXFRMOpaquePortIsDistinctFromAnyPort` | `internal/component/ike/dataplane` | AC-2. A control policy with no port constraint holds mask 0 in the SAME test, so the two are proven distinguishable | |
| `TestXFRMOpaquePortMatchesOnlyPortZero` | `internal/component/ike/dataplane` | AC-5 and AC-6. The behavioral half. A packet with a real port does not match, and a packet presenting no port does | |
| `TestXFRMOpaquePortPolicyIsRemoved` | `internal/component/ike/dataplane` | AC-7. The delete selector matches the installed selector | |

The package already carries `//go:build integration && linux` files, so these are
auto-enrolled in `ZE_QEMU_INTEGRATION_PKGS` (`mk/test-integration.mk`) and run under
`make ze-qemu-integration-test`. **No new make target is needed.** The VM installs
`iproute2` (`mk/test-integration.mk`), so `ip xfrm policy show` is available as an
independent oracle beside the netlink read-back.

**The read-back is necessary and not sufficient.** It proves the bytes reached the kernel.
It does not prove the kernel USES them. `TestXFRMOpaquePortMatchesOnlyPortZero` carries the
sufficient half. A-2 must be validated before anyone writes that test, because the stimulus
that presents port 0 comes from RFC 4301 Section 4.4.1.1 rather than from a guess.

**The discriminator, stated so a later reader cannot lose it.** Revert the write-path change
and `TestXFRMOpaquePortReachesTheKernelExactly` must fail, because the mask returns to 0 and
the opaque policy becomes byte-identical to the control policy in
`TestXFRMOpaquePortIsDistinctFromAnyPort`. That is AC-13, and it is run as a mutation rather
than assumed.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `22-ts-port-opaque` | `test/ipsec-interop/scenarios/` | strongSwan | A strongSwan peer that proposes OPAQUE ports establishes a Child SA with Ze, and Ze answers the OPAQUE form rather than TS_UNACCEPTABLE | |

**This scenario cannot carry an RFC tag.** `test/ipsec-interop/` is declared `TIER_UNRUN`
(`scripts/dev/rfc_requirements.py`), and a tag there is refused, because nothing runs the
suite automatically. Write that reason into the scenario header. Compliance evidence stays
on the existing unit-tier tags.

## Files to Modify
- `internal/component/ike/dataplane/xfrm_linux.go` - `xfrmSelectorPort` stops
  refusing port 0, and `xfrmPolicyFromParams` carries the mask
- `internal/component/ike/dataplane/xfrm_linux_test.go` - the translation unit tests
- `internal/component/ike/dataplane/xfrm_transport_integration_linux_test.go` -
  `TestXFRMOpaquePortIsRefused` is replaced by the new proof. It pins behavior this
  spec deliberately changes, so it is a legitimate removal under `ai/rules/testing.md`
- `internal/component/ike/dataplane/vpp.go` - express or refuse the opaque form
- `internal/component/ike/ipsec/traffic_selector.go` - `checkPortProgrammable`
- `internal/component/ike/engine/ts_narrow.go` - `programmableSelector`
- `internal/component/ike/engine/ts_narrow_test.go` - the `RFC7296-3.13.1-3` tag comments
  (`:336`, `:361`) say the form is refused. They must say it is programmed
- `vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go` - route 1 only
- `rfc/short/rfc7296.md` - no new row. Section 3.13.1 rows stay as they are
- `docs/features/rfc-status.md` - the Remaining cell for RFC 7296 names the OPAQUE
  platform limit. That disclosure must go when the limit goes
- `ai/RFC-REQUIREMENTS.md` - regenerated with `make ze-rfc-index-update`

## Files to Create
- `internal/component/ike/dataplane/xfrm_opaque_integration_linux_test.go` - the four QEMU
  assertions
- `internal/component/ike/dataplane/vendor_patch_test.go` - route 1 only, the marker test
- `scripts/dev/reapply-netlink-portmask.py` - route 1 only, the re-apply script
- `scripts/dev/netlink-portmask-upstream.patch` - the patch offered upstream
- `test/ipsec/ipsec-ts-port-opaque.ci`
- `test/ipsec/ipsec-ts-port-opaque-reject.ci`
- `test/ipsec/ipsec-show-sa-port-opaque.ci`
- `test/ipsec-interop/scenarios/22-ts-port-opaque/`
- `rfc/short/rfc4301.md` - the Security Architecture summary, if absent. Section 4.4.1.1
  defines OPAQUE

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | `port opaque` already parses (`internal/component/ike/ipsec/traffic_selector.go`). No leaf is added |
| YANG validation constraints | No | The accepted set is unchanged. Only the programmability check moves |
| YANG custom validators | No | `checkPortProgrammable` is Go-side commit validation, not `ze:validate` |
| CLI commands/flags | No | No new command |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | No | `opaque` is already an accepted value |
| Functional test for new RPC/API | Yes | The three `.ci` files above |
| Pipe completeness | Yes | `show vpn ipsec sa` gains the opaque form, and it must survive `\| json` |
| Env var registration | No | No leaf under `environment/` |
| Doctor check for runtime dependencies | | Open. Answer at design time whether a kernel that ignores the mask is a readiness condition worth a check |
| Prometheus counters/metrics | No | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | This is IKEv2 and IPsec, not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`. An opaque port selector becomes usable |
| 2 | Config syntax changed? | No | The syntax already accepts `opaque`. Only the outcome changes |
| 3 | CLI command added/changed? | No | No new command |
| 4 | API/RPC added/changed? | No | No new RPC |
| 5 | Plugin added/changed? | No | No registration shape changes |
| 6 | Has a user guide page? | Yes | The IPsec traffic-selector guide page. Confirm the path at design time |
| 7 | Wire format changed? | No | The encoder is unchanged. Ze now sends what it already knew how to encode |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md`. Section 3.13.1 stops being a disclosed platform limit |
| 10 | Test infrastructure changed? | | Answer at design time. Route 1 adds a re-apply script that every `go mod vendor` must run, and `docs/contributing/` must say so |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`. strongSwan and libreswan both handle opaque selectors |
| 12 | Internal architecture changed? | | Answer at design time, depending on the route chosen |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing new registers |
| 16 | Any changed source file referenced by existing doc source anchors? | | Grep `docs/` for anchors naming the files in Files to Modify |
| 17 | Existing docs show config/CLI/API examples for this area? | | Verify every traffic-selector example against the changed validation |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the oracle before the fix
   - Tests: `TestXFRMOpaquePortIsDistinctFromAnyPort`
   - Files: `internal/component/ike/dataplane/xfrm_opaque_integration_linux_test.go`, and
     the READ half of the chosen route
   - Verify: the read path returns a mask. The new test FAILS, because the write path still
     derives one. A test written before the read path exists compares zero with zero and
     proves nothing (R-3)
2. **Phase: Validate the two blocking assumptions**
   - Tests: none. This phase produces evidence, not code
   - Files: `rfc/short/rfc4301.md`, and the design notes in this spec
   - Verify: A-1 is confirmed against the kernel source, and A-2 names the exact stimulus
     the behavioral test will use. If A-1 is broken, STOP and report. The spec is void
3. **Phase: Choose the route and record it**
   - Tests: none
   - Files: the Key Design Decisions table below
   - Verify: the choice names its maintenance cost and its exit condition. Route 1 without
     all three precedent artifacts is not a choice, it is R-1
4. **Phase: The write path**
   - Tests: `TestXFRMOpaquePortReachesTheKernelExactly`,
     `TestXfrmSelectorPortCarriesAnExactZero`, `TestXfrmSelectorPortRejectsAPartialMask`
   - Files: `internal/component/ike/dataplane/xfrm_linux.go`, and the route's own files
   - Verify: both directions carry the mask (R-6). AC-2 and AC-3 prove the unchanged cases
     are byte-identical
5. **Phase: The behavioral proof and teardown**
   - Tests: `TestXFRMOpaquePortMatchesOnlyPortZero`, `TestXFRMOpaquePortPolicyIsRemoved`
   - Files: the QEMU test file
   - Verify: a packet with a real port does not match, and the delete removes the policy
     (R-2). Run the AC-13 mutation here
6. **Phase: Relax the two refusals (LAST)**
   - Tests: `TestCheckPortProgrammableAcceptsOpaque`,
     `TestOpaquePortSelectorIsNarrowedRatherThanDropped`, the three `.ci` files
   - Files: `internal/component/ike/ipsec/traffic_selector.go`,
     `internal/component/ike/engine/ts_narrow.go`
   - Verify: the commit accepts `port opaque`, and a peer proposing OPAQUE is answered. This
     phase is last on purpose (R-5)
7. **Phase: The other backend, the tags, and the disclosure**
   - Tests: `TestVPPBackendHandlesOpaquePort`, `make ze-rfc-check`, `make ze-doc-verify`
   - Files: `internal/component/ike/dataplane/vpp.go`,
     `internal/component/ike/engine/ts_narrow_test.go`,
     `docs/features/rfc-status.md`, `ai/RFC-REQUIREMENTS.md`
   - Verify: the tag comments describe the new reality, the owner approved that edit, and
     the platform-limit disclosure is removed from the public page
8. **Phase: Upstream**
   - Tests: none in this repository
   - Files: `scripts/dev/netlink-portmask-upstream.patch`
   - Verify: the patch is sent, its URL is recorded in the learned summary, and the exit
     condition is written down: when it merges and releases, bump the dependency and delete
     the local bridge

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at `file:line` |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness: both directions | The source mask and the destination mask are both explicit. An asymmetric fix is R-6 |
| Correctness: install equals delete | `xfrmPolicyFromParams` stays the single builder, and the QEMU test proves the delete |
| Correctness: unchanged cases | An any-port policy and a single-port policy install byte-identical bytes to today |
| Correctness: the state selector | `xfrm_state_linux.go` shares the derivation, and the change reaches it |
| Mutation: revert the write path | `TestXFRMOpaquePortReachesTheKernelExactly` reddens. AC-13 |
| Mutation: fix only the destination mask | `TestXFRMOpaquePortReachesTheKernelExactly` reddens on its source assertion |
| Mutation: skip the re-apply script | `TestNetlinkPortMaskPatchPresent` reddens. Route 1 only |
| Mutation: relax `checkPortProgrammable` alone | The `.ci` still passes and the QEMU install fails. That ordering IS R-5, and the review must confirm the phases prevent it |
| Naming | The mask fields follow the kernel's own names. No Ze invention where a kernel name exists (`ai/rules/go-standards.md`) |
| Data flow | The mask travels from `PortMatch` to `nl.XfrmSelector` without a second representation |
| Rule: `ai/rules/protocol.md` | Every port form Ze accepts at commit is programmable exactly. A mask that is neither 0 nor 0xffff is still refused |
| Rule: `ai/rules/platform-linux.md` | The kernel proof runs, and it is not replaced by a unit test with a fake backend |
| Rule: `ai/rules/evidence.md` | A-1 is confirmed against the kernel source and not against a comment |
| Rule: `ai/rules/rfc-compliance.md` | No answer narrower than full implementation with full proof was chosen. Thomas answered every such question |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The kernel holds an exact port-0 mask | `make ze-qemu-integration-test` passes |
| The exact form is distinguishable from any-port | `TestXFRMOpaquePortIsDistinctFromAnyPort` passes |
| The matching behavior is proven | `TestXFRMOpaquePortMatchesOnlyPortZero` passes |
| The refusals are gone | `grep -n 'cannot be programmed' internal/component/ike/ipsec/traffic_selector.go` returns nothing |
| The public disclosure is gone | `grep -c 'OPAQUE ports' docs/features/rfc-status.md` returns 0 |
| The vendor patch cannot be silently lost | `TestNetlinkPortMaskPatchPresent` passes, and it fails on a fresh `go mod vendor`. Route 1 only |
| The upstream patch exists and was sent | The patch file exists, and the learned summary records the URL and the outcome |
| The RFC tags describe reality | `make ze-rfc-index-update` produces no diff, and the tag comments say the form is programmed |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | A peer-proposed port pair of 65535/0 is attacker-controlled. It must reach the kernel only through the same validation an operator's config takes |
| Fail closed | A mask the backend cannot express must still be REFUSED, never widened. The relaxation is narrow, and a partial mask stays refused |
| Widening | The whole risk of this change is protecting MORE traffic than negotiated. Every review pass asks whether any path can produce mask 0 where 0xffff was asked for |
| Silent revert | A reverted vendor patch widens every opaque policy with no signal. The marker test is a security control, not tidiness |
| Error leakage | A refusal names the port and the mask. Neither is secret |
| Authorization | None. The change carries no authorization decision |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| A-1 is broken (the kernel ignores the mask) | STOP. The spec is void. Report to Thomas with the kernel evidence |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Ze already models the kernel correctly. `dataplane.PortMatch`
  (`internal/component/ike/dataplane/dataplane.go`) carries a value AND a mask, and its
  own comment says the two-field shape is the kernel's rather than a Ze invention. The
  information survives every Ze layer and is discarded at the last step, inside a vendored
  library. That is why the fix is small in Ze and awkward in placement.
- The read path is the half a reader loses. `parseXfrmPolicy`
  (`vendor/github.com/vishvananda/netlink/xfrm_policy_linux.go`) restores four
  selector fields and neither mask, so an any-port policy and an exact-port-0 policy are
  indistinguishable after a list. If you do not fix that, the QEMU proof has no oracle, and
  a test written against it passes because it compares zero with zero.
- The refusal is not a bug to delete. It is the correct behavior for as long as the exact
  form is unrepresentable. Removing the refusal before the representation exists converts a
  conformant daemon into one that widens a negotiated policy.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Fix the mask rather than accept the refusal permanently | Keep refusing, and record the limit forever | Owner ruling, 2026-08-01. The verbatim words are in the Provenance block above |
| Send an upstream patch whatever bridge is chosen | Carry a private fork indefinitely | Owner ruling, 2026-08-01: "if a patch must be sent upstream we will also do that". A silent divergence from upstream is the liability this avoids |
| The route is chosen in the design phase, not here | Pick route 1 now | The choice depends on the upstream state, which is evidence this skeleton does not yet hold |
| Relax the two refusals LAST | Relax them first so the config surface works while the backend catches up | Accepting a config Ze cannot program is exactly the failure `ai/rules/protocol.md` names |

## Known Limitations

- The spec is void if A-1 is broken, which is to say if the kernel does not honor a full
  port mask over a zero port value. Nothing in Ze can fix that, and the refusal would become
  permanent and correct.
- The VPP backend installs SAs, and refuses one that carries an explicit state selector
  (`vppUnsupportedSA`, `internal/component/ike/dataplane/vpp.go`; `spec-fixit-vpp-ipsec-inoperable`,
  closed 2026-08-10). AC-12 can therefore only be satisfied by an explicit refusal until
  that backend can express the opaque form.
- The interop scenario cannot carry an RFC tag, because `test/ipsec-interop/` is
  `TIER_UNRUN`. Compliance evidence stays on the existing unit-tier tags.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

Two sites need it. The first is the port translation in the XFRM backend. It must quote
RFC 7296 Section 3.13.1 on the OPAQUE encoding, and RFC 4301 Section 4.4.1.1 on what an
opaque port means. The second is `checkPortProgrammable`, whose current comment states the
refusal and its reason. Rewrite that comment rather than delete it, because it records why
the refusal existed.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
