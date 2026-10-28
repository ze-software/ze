# Spec: pppoe-refuse-vpp-backend -- refuse PPPoE under the VPP interface backend at verify

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped defect: `ze config validate` and a commit accept PPPoE under `interface backend vpp`, then every subscriber session fails at IPCP Opened. Moved here from `spec-pppoe-vpp-dataplane` Phase 1 by owner decision OD-5 (2026-10-28) |
| RFC gaps | - |
| Depends | - |
| Phase | 0/1 |
| Handoff | - |
| Updated | 2026-10-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Doors: `ai/doors/system.md`, `ai/doors/forwarding.md` (vpp, iface), `ai/doors/access-security.md` (PPPoE, PPP).

## Task

A config with `interface backend vpp` and a `pppoe` block commits. PPPoE then
runs every session on the kernel PPP path, and the VPP interface backend
answers the session's address install with "AddAddressP2P (PPP NCP not
supported on VPP backend yet)" (`vppBackendImpl.AddAddressP2P`,
`internal/plugins/iface/vpp/ifacevpp.go`), so every session fails at IPCP
Opened. The operator learns this from failing subscribers, not from the commit.

This spec makes the combination a verify-time refusal. The VPP session data
plane that later makes it work is `plan/post-release/features/spec-pppoe-vpp-dataplane.md`;
when that lands, its VPP backend registration lifts this refusal (its AC-2
replaces it with the LCP-pair check).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` "ze:backend" - `ze:backend "<names>"` on a YANG node declares which backends implement it; the walker refuses at commit and `ze config validate`
  → Constraint: the gate is per component: `iface` calls it for root `interface` with its own `backend` leaf (`validateBackendGate`, `internal/component/iface/register.go`); `vrrp` is caught only because it augments `interface`. `pppoe` is its own root, so an annotation alone is walked by nobody
- [ ] `docs/features.md` "Commit-Time Backend Capability Check" - the operator-facing statement of the gate; gains `pppoe`
- [ ] `docs/guide/pppoe.md` - gains the refusal and the reason
- [ ] `plan/post-release/features/spec-pppoe-vpp-dataplane.md` - OD-3 decided: PPPoE's data plane follows `interface backend`, no new leaf
  → Decision: the refusal keys on `interface backend`, not on any PPPoE leaf

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - `pppoe` container with no `ze:backend`
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfigMapTransition` runs `InProcessConfigVerifier` of every registration that declares `ConfigRoots`, handing it one section per root; no registration declares root `pppoe`, so no verifier sees it
- [ ] `cmd/ze/hub/register_l2tp.go` - `registerBNGSubsystems` builds the PPPoE subsystem from `pppoe.ExtractParameters` at startup; nothing consults the interface backend
- [ ] `internal/component/iface/register.go` - `validateBackendGate`, `backendLeafPath` `/interface/backend`, `verifyIfaceConfig`
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - `AddAddressP2P` answers `errNotSupported`

**Behavior to preserve:**
- PPPoE under the default netlink backend: every existing PPPoE `.ci`, unit and interop test unchanged.
- `iface` `pppoe-client` units, which `test/parse/iface-netlink-accepts-pppoe-client.ci` covers, are a different feature and are not touched.

**Behavior to change:**
- `interface backend vpp` together with a `pppoe` block that names an access interface is refused by `ze config validate`, at commit and at reload verify, naming the `pppoe` path, the active backend, the backend that supports it, and the fix.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze config validate`, an editor commit, or a reload: `VerifyPluginConfigMapTransition` over the candidate tree.

### Transformation Path
1. A verifier that receives both the `interface` and the `pppoe` sections reads the active interface backend (default `netlink`).
2. When it is not one PPPoE supports and `pppoe` names an access interface, it refuses.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config candidate -> verifier | `InProcessConfigVerifier` sections | `test/parse/pppoe-vpp-backend-refused.ci` |

### Integration Points
- The `ze:backend` walker (`config.ValidateBackendFeaturesJSON`), if the design question below picks it.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| Registration over hardcoding | to settle in design | the refusal must not be a `pppoe` case added to `iface`'s verifier; the supported-backend set comes from a declaration PPPoE owns |
| No layering | Yes | the refusal is deleted, not bypassed, when the VPP session data plane registers |

## Open design question (research, then `ready`)

| # | Question | Candidates | Evidence to gather |
|---|----------|-----------|--------------------|
| Q-1 | Where does the refusal live? | (a) annotate the `pppoe` container `ze:backend "netlink"` and give PPPoE a config verifier registration whose `ConfigRoots` are `pppoe` and `interface`, which walks the gate with `interface backend` as the active backend; (b) a PPPoE session-data-plane registry (the one `spec-pppoe-vpp-dataplane` D-1 introduces, kernel entry only) that the verifier asks whether the active backend has a session data plane | whether a registry `Registration` can carry only a verifier with no `RunEngine` (`internal/component/plugin/registry/registry.go`); (b) is what the VPP spec's AC-1 and Phase 2 already assume, so it would land the registry early with one entry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | `interface backend` absent means `netlink` | `iface` YANG default | the refusal fires on a default config | parse test on a tree with no backend leaf | unvalidated |

### Risks
| ID | Risk | Detection | Mitigation |
|----|------|-----------|------------|
| R-1 | The refusal fires for a `pppoe` block with no access interface, which runs nothing | parse test | refuse only when an access interface is named, matching `registerBNGSubsystems`'s own condition |

## Blast Radius
| Question | Answer |
|----------|--------|
| What else is affected? | `ze config validate`, commit and reload verify for any tree holding `pppoe` |
| How is it reverted? | one commit |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` on `interface backend vpp` plus `pppoe interface X` | → | PPPoE backend refusal | `test/parse/pppoe-vpp-backend-refused.ci` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `interface backend vpp` and a `pppoe` block naming an access interface | `ze config validate` exits non-zero; the message names the `pppoe` path, says the VPP backend has no PPPoE session data plane, names the backend that supports it, and gives the command that fixes it |
| AC-2 | The same tree with the default backend | validate accepts |
| AC-3 | `interface backend vpp` and a `pppoe` block naming no access interface | validate accepts |
| AC-4 | A reload that switches a running PPPoE config to `interface backend vpp` | the reload is refused at verify with the AC-1 message; the running config is unchanged |

## End-to-End User Stories
| # | Story | Path | Test |
|---|-------|------|------|
| 1 | An operator moving a BNG to VPP is told at commit that PPPoE cannot run there yet | `ze config validate` | `test/parse/pppoe-vpp-backend-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPPPoEVerifyRefusesVPPBackend` | `internal/component/l2tp/pppoe/` (file decided by Q-1) | AC-1, AC-3 | |
| `TestPPPoEVerifyAcceptsDefaultBackend` | same | AC-2, A-1 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-vpp-backend-refused` | `test/parse/` | AC-1 | |
| `pppoe-netlink-backend-accepted` | `test/parse/` | AC-2 | |
| `pppoe-vpp-backend-reload-refused` | `test/reload/` | AC-4 | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | config-only refusal, no protocol behavior changes | - |

## Files to Modify
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - per Q-1
- `internal/component/l2tp/pppoe/` - the verifier registration, per Q-1
- `docs/guide/pppoe.md`, `docs/features.md` - the refusal

## Files to Create
- `test/parse/pppoe-vpp-backend-refused.ci`, `test/parse/pppoe-netlink-backend-accepted.ci`, `test/reload/pppoe-vpp-backend-reload-refused.ci`

### Integration Checklist
| Item | Needed | Notes |
|------|--------|-------|
| YANG schema | per Q-1 | `ze:backend` annotation under (a) |
| CLI grammar | No | |
| Doctor check | No | |

### Documentation Update Checklist (BLOCKING)
| # | Trigger | Needed | Where |
|---|---------|--------|-------|
| 1 | Config refused | Yes | `docs/guide/pppoe.md`, `docs/features.md` backend gate row |

## Implementation Steps
1. **Phase 1:** settle Q-1 against the registry source, then the verifier, tests AC-1 to AC-4, docs, journal row marked `fixed:`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Message | `ai/rules/user-facing-errors.md`: what, why, the exact fix command |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Refusal | `pppoe-vpp-backend-refused.ci` red against today's binary, green after |

### Security Review Checklist
| Check | What to look for |
|-------|------------------|
| N-A | a refusal adds no input path |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Functional test fails | check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- `ze:backend` is walked per component root, so a feature in a root of its own is gated only when its owner runs the walk.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `validate-accepts-what-start-refuses.md` | 2026-10-28 | PPPoE under `interface backend vpp` validates, every session fails at IPCP Opened |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Refuse at verify | refuse at PPPoE start | the class this fixes is a check that lives only on the start path |
| Key on `interface backend` | a PPPoE data-plane leaf | owner decision OD-3 (`spec-pppoe-vpp-dataplane`): one fact declared once |

## Known Limitations
- PPPoE on VPP itself: `plan/post-release/features/spec-pppoe-vpp-dataplane.md`.

## RFC Documentation (Scope: protocol)
- N-A: config refusal.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches, and `./le verify changed` over the closing commits (`/ze-close` step 6f). The full verification is Thomas's release check: no agent runs it, and one that is owed is handed to him as its exact command. A red there gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-pppoe-refuse-vpp-backend.md` only, in the same `./le commit create` script
