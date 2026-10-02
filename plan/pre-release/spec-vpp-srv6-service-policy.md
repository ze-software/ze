# Spec: VPP SRv6 service policies and local binding SIDs

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | design required |
| Handoff | - |
| Updated | 2026-10-02 |

## Task

Make a received SRv6 Service SID produce a usable VPP ingress encapsulation path. The current backend sends steering with the remote Service SID as its binding SID but creates no SR policy. Recording the steering request is not proof of forwarding.

Thomas chose a separate pre-release spec on 2026-10-02, with a local binding SID allocated from a local locator. This records the approved scope; it does not authorize implementation during `spec-rfc-verdict-test-fix-pass`. The release bucket follows that explicit owner choice. RFC9252-5-3 moves here under the parent's P-3 procedure and remains weak until independently proved.

## Required Reading

- [ ] `docs/architecture/fib/fib-depth-4-srv6.md` and `docs/features/srv6.md`.
  → Constraint: retain the Prefix-SID-to-FIB input path; no new SRv6 NLRI family is required for this defect.
- [ ] `rfc/full/rfc9252.txt`, Section 5, and `rfc/short/rfc9252.md`.
  → Constraint: the ingress outer IPv6 destination is the received Service SID, not the local binding SID.
- [ ] `plan/spec-srv6-static-segments.md`.
  → Constraint: that spec owns configured multi-segment routes and an encapsulation source setting. This one owns the missing single-Service-SID policy and its lifetime; neither implements the other's extra scope.
- [ ] `ai/rules/platform-linux.md`, `ai/rules/protocol.md`, `ai/rules/config.md`, and the existing VPP API bindings.
  → Constraint: design the locator configuration and allocator against existing registries; do not add a second central feature list or depend on an external command in shipped code.

## Current Behavior

| Source read | Behavior |
|-------------|----------|
| `internal/plugins/fib/vpp/srv6.go::processSRv6Change` | Install/replace calls addSRv6Steer and records the prefix only after success; remove calls delSRv6Steer. |
| `internal/plugins/fib/vpp/srv6.go::addSRv6Steer` | Issues only SrSteeringAddDel, placing the received Service SID in BsidAddr; there is no policy creation in this producer. |
| `internal/plugins/fib/vpp/srv6.go::delSRv6Steer` | Removes prefix steering but holds no policy ownership or reference state. |
| `internal/plugins/fib/vpp/rfc9252_srv6_encap_red_test.go::TestRFC9252VPPServiceRouteEncapsulatesTowardTheSID` | Preserved untracked failing probe requires an encapsulating SR policy with the Service SID as its sole segment, referenced by the prefix steering entry. |

RFC 9252 Section 5 states: "This TLV serves two purposes -- first, it indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required; second, it indicates the value of the Service SID to be used in the encapsulation."

Preserve ordinary IP/MPLS routes, the Linux SEG6 backend, table selection, and error reporting. A policy-creation failure must not be reported as an installed route. Do not mark RFC9252-5-3 enforced merely because a model accepts a request.

## Data Flow

| Stage | Input | Required output |
|-------|-------|-----------------|
| Existing FIB event | Prefix, table and received Service SID | VPP SRv6 route operation |
| Local policy ownership | Service SID and local locator | Locally owned binding SID and encapsulating SR policy |
| VPP steering | Prefix and policy binding SID | Prefix references an installed policy |
| Withdrawal/replacement | Existing prefix ownership | Obsolete steering removed; shared policy retained until its last user leaves |
| Error boundary | VPP failure | Operation reports failure and retains enough ownership to reconcile or clean up |

## Acceptance Criteria

| AC ID | Input / condition | Expected behavior |
|-------|-------------------|-------------------|
| AC-1 | First IPv4 or IPv6 service route to a received SID | An encapsulating VPP SR policy is installed before steering; its segment list contains the received SID and steering references the policy's local binding SID. |
| AC-2 | Binding-SID allocation | Every binding SID belongs to the configured local locator, does not collide with another local allocation, and is not obtained by treating the remote Service SID as a local allocation. Exhaustion or unavailable locator is a visible failure. |
| AC-3 | Two prefixes sharing one policy, then one withdrawal | Remaining prefix still forwards through its policy; withdrawal does not remove a policy still in use. |
| AC-4 | Last withdrawal, SID replacement, or table-specific removal | No dangling steering or leaked unused policy; another table or prefix is not removed. |
| AC-5 | Policy creation, steering update, or cleanup failure | No false installed state, lost live policy ownership, or success answer; recovery follows the actual VPP result. |
| AC-6 | Real BGP-to-FIB route and a packet destined to its prefix | Packet reaches the egress peer with an outer IPv6 destination equal to the received Service SID, through VPP, not a mock dataplane. |
| AC-7 | RFC9252-5-3 after the fix | Tagged proof covers each implemented backend and the full ingress behavior; native discrimination records and an independent rejudgment establish enforced. Existing kernel evidence is retained. This owns the BGP child's transferred weak verdict. |

## Wiring and Test Plan

| Proof | Entry point and assertion |
|-------|---------------------------|
| Existing failing probe | Preserve TestRFC9252VPPServiceRouteEncapsulatesTowardTheSID and make its policy/steering assertions pass; extend through the established VPP operation seam for lifetime and error cases. |
| Functional | Received UPDATE reaches the selected route and VPP policy/steering state. Name and design the scenario before implementation. |
| Interop | Named scenario `srv6-service-policy-vpp` exercises VPP ingress against an independent SRv6 egress implementation; observe the outer destination and delivery, not merely session establishment. The design must identify the peer implementation and executable runner before implementation. |
| Discrimination | Native records for every added or changed RFC-tagged carrier; no handwritten fingerprints. |

## Risks and Design Decisions Still Owed

| Risk / decision | Required design resolution |
|-----------------|----------------------------|
| Locator configuration ownership | Reuse the existing SRv6 configuration/registration boundary if one owns the fact; explicitly choose the scope and validation if it does not. |
| Policy sharing key | Include every VPP attribute that changes forwarding; do not assume a SID alone identifies equivalent policy across all tables. |
| Restart and external VPP state | Specify discovery/reconciliation and collision handling before choosing allocator persistence. |
| Partial failure | Specify creation, replacement and removal ordering with retained ownership; no optimistic installed marker. |
| Proof availability | Resolve real VPP and independent egress infrastructure during design. A scripted API model is unit evidence, not AC-6. |

## Files and Documentation

Implementation owns `internal/plugins/fib/vpp/srv6.go`, its existing backend/config integration as established during design, tests beside it, the named interop scenario, and the RFC9252 ledger proof. Locator schema, migration and diagnostic surfaces must be enumerated before the status leaves design.

Update `docs/architecture/fib/fib-depth-4-srv6.md`, `docs/features/srv6.md` and the RFC9252 Support metadata only to behavior the evidence supports. The current pages disclose the policy gap. Coordinate the overlapping backend with `spec-srv6-static-segments`; do not claim its multi-segment feature here.

## Review Gate

No implementation review or verification has run for this skeleton. Design, implementation, independent review, native proof and full worktree verification remain required before closure.

| Run | Scope | Reviewer | Artifact | Outcome |
|-----|-------|----------|----------|---------|
