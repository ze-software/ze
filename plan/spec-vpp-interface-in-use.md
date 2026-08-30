# Spec: vpp-interface-in-use

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-08-18 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/vpp/register.go` - `verifyVPPConfig`
4. `internal/plugins/firewall/vpp/verify.go` - NAT/ACL VPP verify
5. `internal/plugins/flowexport/config.go` - sFlow/IPFIX sampling verify

## Task

Ze's VPP features each validate their own config in isolation. An interface can be
referenced by a VPP feature (NAT / ACL / IPFIX / sFlow) and simultaneously assigned
as a VPP bridge member with no conflict detected, and an interface still referenced
by a VPP feature can be deleted with no guard. These produce broken or ambiguous
dataplane state that only surfaces at runtime.

Add referential-integrity validation for VPP interface usage:

- At config verify, detect an interface used in conflicting roles (e.g. referenced
  by a VPP feature AND assigned as a VPP bridge member) and reject with a clear
  message naming both references.
- Block deletion of an interface still referenced by any VPP feature/member.

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - VPP feature/interface model.
  → Constraint: features validate independently today; this adds a cross-feature check.
- [ ] `ai/rules/plugins.md` - each VPP feature owns its config.
  → Constraint: the cross-reference check needs a shared, discoverable registry of "interface used by <feature>", not a hardcoded list embedded in one feature.

**Key insights:**
- The gap is the absence of a shared interface-role/usage registry; each feature only sees its own config.
- ~~Ze has no VPP xconnect and no VPP bond today, so the member side is limited to VPP bridge membership.~~ (Superseded 2026-07-10: still no xconnect/bond, but the followup-vpp-iface wave added mirror destination, LCP pairing, and VPP tunnel interface roles -- see Post-wave corrections below.)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/vpp/register.go` - `verifyVPPConfig` (register.go, wired as `InProcessConfigVerifier` at :40) only parses each vpp section and calls `Validate()`; no interface-reference inspection.
- [ ] `internal/plugins/firewall/vpp/verify.go` - `Verify` (verify.go) checks only unsupported match/action expression types; it does not inspect which interface a rule binds relative to other features.
- [ ] `internal/plugins/flowexport/config.go` - `SamplingConfig.validate` (config.go) checks only that `interface` is non-empty and ranges; `Config.Validate` (config.go) never cross-references the sampling interface.
- [ ] `internal/component/iface/operation.go` - defines apply-ordering ops for bridge membership (operation.go) but those are transaction dependency-ordering, not a referential-integrity verify against VPP features.

**Behavior to preserve:**
- Each feature's own validation is unchanged; this adds a cross-feature layer.
- Valid single-role interface assignments continue to work.

**Behavior to change:**
- Conflicting interface roles and delete-while-referenced are caught at verify.

## Data Flow (MANDATORY)

### Entry Point
- Config verify: the full candidate config tree at commit time, seen by the VPP verify hook and/or a new cross-feature verifier.

### Transformation Path
1. Collect all interface references from VPP-touching features (NAT inside/outside, ACL, IPFIX, sFlow) into a usage map keyed by interface name, each with the referencing feature.
2. Collect VPP member assignments (bridge membership) into the same map.
3. Detect conflicts: an interface appearing both as a feature reference and as a member (bidirectional).
4. Detect dangling deletes: an interface removed from `interfaces` while still present in the usage map.
5. On conflict/dangle, return a `ConfigError` naming both sides; otherwise pass.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Features ↔ usage registry | each feature contributes its interface references | [ ] |
| Verify ↔ usage registry | cross-feature verifier reads the aggregated map | [ ] |
| iface delete ↔ registry | deletion checked against outstanding references | [ ] |

### Integration Points
- `verifyVPPConfig` (`vpp/register.go`) - host of, or coordinator for, the cross-feature check.
- firewall/flowexport verify paths - contribute their interface references (via a shared discovery mechanism, not a hardcoded list in one place).
- `iface/operation.go` ordering - complemented by the new verify-time integrity check.

### Architectural Verification
- [ ] No bypassed layers (verify runs in the config-verify path)
- [ ] No unintended coupling (features contribute references through a discoverable mechanism, per plugin-self-containment)
- [ ] No duplicated functionality (one aggregation point, not per-feature pairwise checks)
- [ ] Registration over hardcoding — feature-to-interface references are discovered via registration, not a central hardcoded list of feature paths.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A shared, registration-based way exists (or can be added) for features to declare interface use | small-core/registration model | risk of a hardcoded feature-path list | design the registry during RESEARCH | unvalidated |
| A-2 | ~~Only VPP bridge membership is a "member" role today (no xconnect/bond)~~ superseded, see Post-wave corrections (2026-07-10) | grep found no xconnect/bond in VPP | more roles later | re-grep during audit | unvalidated -> broken (2026-07-10: wave added mirror/LCP/tunnel roles) |
| A-3 | The verify hook sees the whole candidate tree, not just the vpp section | `InProcessConfigVerifier` semantics | need a different hook | confirm verifier scope during audit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Hardcoding the feature-path list violates plugin-self-containment | reviewer flags central list | features register their interface-use contributions |
| R-2 | False positives block legitimate configs | valid config rejected | scope conflicts precisely (feature-ref AND member), with clear messages |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| interface used by NAT and assigned as bridge member | → | cross-feature verify rejects | `test/plugin/vpp-interface-in-use.ci` |
| delete an interface still referenced by sFlow | → | verify blocks deletion | `test/plugin/vpp-interface-in-use.ci` |
| delete an interface still used as a mirror destination (added 2026-07-10) | → | verify blocks deletion, naming the mirroring source | `test/plugin/vpp-interface-in-use.ci` |
| remove a VLAN sub-interface still referenced by a feature (added 2026-08-18) | → | verify blocks the removal, naming the sub-interface | `test/plugin/vpp-interface-in-use.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | interface in NAT (or ACL/IPFIX/sFlow) AND as VPP bridge member | verify rejects, naming both references |
| AC-2 | interface referenced by a VPP feature, then deleted from `interfaces` | verify rejects the deletion |
| AC-3 | interface used by exactly one feature, no member conflict | accepted |
| AC-4 | interface as bridge member only | accepted |
| AC-5 | error message | names the interface and both conflicting references |
| AC-6 (added 2026-07-10) | interface configured as a SPAN mirror destination (mirror ingress/egress of another interface), then deleted from `interfaces` | verify rejects the deletion, naming the mirroring source interface |
| AC-7 (added 2026-07-10) | interface with an LCP pairing (VPP backend), referenced by a feature or deleted while paired | usage aggregation records the LCP-paired role; delete/conflict checks account for it |
| AC-8 (added 2026-07-10) | VPP-created tunnel interface (gre/gretap/ipip/vxlan) referenced by a feature or as a mirror destination, then deleted | verify rejects, same as for physical interfaces (tunnel interfaces participate in the usage map as reference targets) |
| AC-9 (added 2026-08-18) | a VLAN sub-interface (`unit`) of a dummy or tunnel interface is referenced by a feature or as a mirror destination, then removed while its parent stays | verify rejects the removal, naming the sub-interface, not only the parent |
| AC-10 (added 2026-08-18) | an interface with VLAN sub-interfaces is deleted, and one sub-interface is referenced by a feature | verify rejects the parent deletion and names the referenced sub-interface, because deleting the parent removes it |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | assigns a NAT interface as a bridge member | verify aggregates refs → conflict → reject | `test/plugin/vpp-interface-in-use.ci` |
| 2 | deletes an interface still used by IPFIX | verify blocks with a clear message | `test/plugin/vpp-interface-in-use.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVPPInterfaceConflictFeatureAndMember` | `internal/component/vpp/verify_refs_test.go` | feature+member conflict rejected | |
| `TestVPPInterfaceDeleteWhileReferenced` | `internal/component/vpp/verify_refs_test.go` | delete-while-referenced rejected | |
| `TestVPPInterfaceSingleRoleOK` | `internal/component/vpp/verify_refs_test.go` | single-role accepted | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A (referential integrity, no numeric inputs) | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-interface-in-use` | `test/plugin/vpp-interface-in-use.ci` | conflicting/deleted interface rejected at commit | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - config validation, no wire protocol | - | - | validated by functional tests | - |

### Future (if deferring any tests)
- xconnect/bond member roles are out of scope until Ze adds those VPP features.

## Files to Modify
- `internal/component/vpp/register.go` - host/coordinate the cross-feature check in verify
- `internal/component/vpp/` - the usage-aggregation + conflict/dangle logic (new file)
- firewall/flowexport features - contribute their interface references via the shared mechanism

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Functional test for new behaviour | [ ] yes | `test/plugin/vpp-interface-in-use.ci` |
| Registration over hardcoding | [ ] yes | features register interface-use contributions; `ai/rules/plugins.md` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | [ ] no | validation only |
| 12 | Internal architecture changed? | [ ] yes | `docs/research/vpp-deployment-reference.md` |

## Files to Create
- `internal/component/vpp/verify_refs.go` - interface-usage aggregation + conflict/dangle checks
- `internal/component/vpp/verify_refs_test.go` - unit tests
- `test/plugin/vpp-interface-in-use.ci` - functional test

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add the cross-feature verify hook host with a no-op aggregator; failing `test/plugin/vpp-interface-in-use.ci`.
2. **Phase: Usage aggregation** — collect feature refs + member assignments via a registration-based mechanism (no hardcoded feature-path list).
3. **Phase: Conflict + dangle detection** — reject conflicts and delete-while-referenced with clear messages.
   - Tests: `TestVPPInterfaceConflictFeatureAndMember`, `TestVPPInterfaceDeleteWhileReferenced`, `TestVPPInterfaceSingleRoleOK`
4. **Functional test**
5. **Full verification** → `./le verify current mode full`
6. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | bidirectional detection; precise scope (no false positives) |
| Data flow | verifier sees the whole candidate tree |
| Registration over hardcoding | feature references discovered via registration, not a central list |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| cross-feature verify | `go test ./internal/component/vpp -run InterfaceInUse` (or verify_refs) |
| functional | `test/plugin/vpp-interface-in-use.ci` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Error messages | name references without leaking secrets |
| Denial of service | aggregation is linear in interface count |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->

## Known Limitations
- xconnect/bond member conflict checks are out of scope (Ze has no such VPP features today).

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N/A)
- [ ] Functional tests for end-to-end behavior

### Post-wave corrections (2026-07-10)

Decision (2026-07-10, user): spec fixed to cover mirror/LCP/tunnel references.

The followup-vpp-iface implementation wave (closed at commit fe6aa242f and earlier) invalidated assumption A-2 and the second Key Insight: VPP bridge membership is no longer the only interface-reference role. Three NEW roles now exist, each verified against the producing function in current code:

| New role | Producer (verified 2026-07-10) | Reference shape |
|----------|-------------------------------|-----------------|
| SPAN mirror destination | `SetupMirror(srcIface, dstIface string, ingress, egress bool)` at `internal/plugins/iface/vpp/mirror.go` (teardown `RemoveMirror` :51); backend surface `internal/component/iface/backend.go`; config fields `MirrorIngress`/`MirrorEgress` at `internal/component/iface/config.go`, parsed from the unit `mirror` map at `config.go` | interface-to-interface: the source unit names a destination interface; deleting the destination while a source mirrors to it dangles the SPAN session |
| LCP pairing | `SetupLCPPair(vppIface, hostName string)` at `internal/plugins/iface/vpp/lcp.go` (no-op when LCP disabled; validates host name, reserves collision-checked TAP name); backend surface `internal/component/iface/backend.go` | interface-to-host-TAP: the VPP interface carries an LCP-paired role; conflict/delete checks must treat a paired interface as referenced |
| VPP tunnel interfaces | `createGRETunnel` at `internal/plugins/iface/vpp/tunnel.go`, `createIPIPTunnel` at `tunnel.go`, `createVxlanTunnel` at `internal/plugins/iface/vpp/vxlan.go` | tunnels are keyed by ADDRESS endpoints (`tunnelEndpoints` parses LocalAddress/RemoteAddress), not by interface references; the new fact for this spec is that they create additional VPP-backed interface KINDS that participate in the usage map as reference TARGETS (a tunnel interface can be a mirror destination, a feature-referenced interface, or be deleted while referenced) |

Consequences for the design (extends, does not replace, the existing Transformation Path):
- Usage-aggregation step 2 ("Collect VPP member assignments") widens to: bridge membership AND mirror-destination references AND LCP-paired status.
- Conflict/dangle detection (steps 3-4) must treat mirror destinations and LCP-paired interfaces as outstanding references, and must include VPP tunnel interfaces in the set of deletable-while-referenced targets.
- New ACs AC-6, AC-7, AC-8 and the added Wiring Test row (above) capture these roles.
- The Known Limitations entry on xconnect/bond remains valid: still no VPP xconnect or bond in the tree.
