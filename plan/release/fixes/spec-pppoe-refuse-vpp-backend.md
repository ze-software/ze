# Spec: pppoe-refuse-vpp-backend -- refuse PPPoE under the VPP interface backend at verify

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped defect: `ze config validate` and a commit accept PPPoE under `interface backend vpp`, then every subscriber session fails at IPCP Opened. Moved here from `spec-pppoe-vpp-dataplane` Phase 1 by owner decision OD-5 (2026-10-28) |
| RFC gaps | - |
| Depends | - |
| Phase | 1/1 |
| Handoff | `tmp/session/2026-11-15-134815/state/session-state-pppoe-refuse-vpp-backend-caa03485-2cbe-4a47-96e9-b290a7c0a4d1.md` |
| Updated | 2026-11-15 |

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
- [ ] `features/commit-time-backend-capability-check.md` - the operator-facing statement of the gate; generates the `docs/features.md` row, which gains `pppoe`
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

## Existing Mechanisms

| Mechanism | Location | Current role | Use in this spec |
|-----------|----------|--------------|------------------|
| Original-tree validator registry | `internal/component/config/tree_validators.go`, `RegisterTreeValidator` and `validateOriginalTree` | Runs component-owned validators on the original full tree before defaults; PPPoE already registers its authentication policy here | Extend with a PPPoE backend validator, without a dummy plugin engine |
| Schema backend capability declaration and walker | `internal/component/config/backend_gate.go`, `ValidateBackendFeaturesJSON`; `ze:backend` extension | Rejects unsupported nodes using the owning schema's declarations | Declare netlink support on the PPPoE access-interface list and walk it using the interface backend |
| Interface backend schema default | `internal/component/iface/yang/ze-iface-conf.yang` | Declares the default interface backend | Read the default from the schema when the original tree omits it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `ze config validate`, an editor commit, or a reload: original-tree validation before the candidate is published.

### Transformation Path
1. PPPoE's registered original-tree validator reads the active interface backend, using its schema default when the leaf is absent.
2. The generic backend walker reads the support declaration on the PPPoE access-interface list and refuses an unsupported named interface.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config candidate -> verifier | Registered original-tree validation | `test/parse/pppoe-vpp-backend-refused.ci` |

### Integration Points
- The `ze:backend` walker (`config.ValidateBackendFeaturesJSON`) reads the access-interface declaration.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| Registration over hardcoding | Yes (design) | PPPoE registers its own original-tree validator; the supported-backend set comes from the PPPoE schema declaration |
| No layering | Yes | the refusal is deleted, not bypassed, when the VPP session data plane registers |

## Resolved design question

| # | Decision | Alternatives and evidence |
|---|----------|---------------------------|
| Q-1 | Use option A with the existing original-tree validator registry and put the backend declaration on the access-interface list | `registry.Register` refuses nil `RunEngine` and `CLIHandler`, so a verifier-only plugin would require dummy handlers. `config.RegisterTreeValidator` already gives PPPoE the full original tree and is used by its authentication validator. A container-wide unconditional annotation would incorrectly refuse a block with no access interface (AC-3). The real runtime session registry remains in the parent VPP spec; this refusal is replaced when that seam exists. Parent implementation approved this refinement on 2026-11-15. |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | `interface backend` absent means `netlink` | `iface` YANG default | the refusal fires on a default config | `TestPPPoEVerifyAcceptsDefaultBackend` passed in the main-thread focused unit run, 2026-11-15 | confirmed |

### Risks
| ID | Risk | Detection | Mitigation |
|----|------|-----------|------------|
| R-1 | The refusal fires for a `pppoe` block with no access interface, which runs nothing | parse test | refuse only when an access interface is named, matching `registerBNGSubsystems`'s own condition |
| R-2 | The reload fixture is treated as portable because it configures no access interface, but its explicit interface root starts the Linux interface monitor | Native Darwin startup failure before the observer, observed 2026-11-15 | Classify the reload draft with `needs-linux` after independent test judgment and run it through the existing one-test QEMU route; keep its configuration and assertions |

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
| `TestPPPoEVerifyRefusesVPPBackend` | `internal/component/l2tp/pppoe/backend_gate_test.go` | AC-1 through the registered shared validator, including the repair as the last line | Main-thread registration-overlay runtime red (`20fe0032`), live-registration green (`739d3fec`), 2026-11-15; full log paths in session state |
| `TestPPPoEVerifyAcceptsDefaultBackend` | same | AC-2, A-1 | Main-thread focused run passed, 2026-11-15 |
| `TestPPPoEVerifyAllowsNoAccessInterface` | same | AC-3 | Main-thread focused run passed, 2026-11-15 |
| `TestPPPoEVerifyReadsTheBackendPath` | same | Parsed backend selection cannot be confused by a comment or another leaf | Main-thread corrected-fixture rerun passed; independent judge APPROVE for this correction only, 2026-11-15 |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | no numeric input | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pppoe-vpp-backend-refused` | `test/parse/` | AC-1 | Draft, 80 stress and promoted live suite passes; registration-omission counterfactual failed as expected, 2026-11-15 |
| `pppoe-netlink-backend-accepted` | `test/parse/` | AC-2 | Draft, 80 stress and promoted live suite passes, 2026-11-15 |
| `pppoe-vpp-no-access-accepted` | `test/parse/` | AC-3 through the offline command | Draft, 80 stress and promoted live suite passes, 2026-11-15 |
| `pppoe-vpp-backend-reload-refused` | `test/reload/` | AC-4 | Draft passed all assertions on Ze's Linux kernel after independently approved Linux/quote fixture corrections; 80-iteration stress and promotion owed |

The three parse tests were promoted after their named draft runs, 80 stress
passes each and the cost report; none was listed as a cost-review candidate.
They prove the compiled command links the gate, which helper unit tests cannot
establish. The reload test remains under `test/draft/reload/`. Its observer
also drives an authenticated editor commit, checks the same refusal, compares
the parsed running configuration after each refusal, and proves a subsequent
valid reload applies. The main thread observed a runtime red for the public
unit refusal by using a single-file Go overlay that removes only the new
validator registration. The live registration then passed the same test.
The same one-file compiler overlay also made the actual refusal functional
test fail: expected exit 1, got 0 and `configuration valid`. This proves both
tests discriminate the registration. All three passed in the promoted live
parse suite; that Darwin run completed 474 tests with five unrelated failures
routed to the existing CI-fix session. The Linux reload test also passed the
commit, completed-reload, retained-running-tree and subsequent-valid-reload
assertions. Reload stress and promotion remain owed. No live source mutation
was applied.

The native Mac reload run did not reach those assertions: the explicit
`interface` root auto-loads the interface plugin and VRRP. The interface
plugin's startup calls `StartMonitor`, which the non-Linux backend refuses.
The independently approved fixture correction adds `option=needs-linux`,
preserving every configuration and assertion. The later actual QEMU run passed
all assertions after correcting a separately judged plain-text quote needle.
No capability marker is justified by this fixture's empty interface/group
sets: Linux monitor subscriptions use unforced receive buffers, and the
existing address-subscription test proves the dump works without net-admin.
The empty IPv6 reverse-path cleanup explicitly tolerates permission refusal.
The existing `test qemu ci test <path>` route resolves Ze's runtime kernel
and runs the draft through its real suite. The failed Mac attempt is not AC-4
evidence; the passing Linux run is recorded in the owning state.

The initial parsed-path fixture failure occurred before validation because it
used `//` comments. The corrected `#` fixture passed, and an independent judge
approved only that fixture correction after the edit. The eventual commit
body must include this exact judgment:

> Test-judge: internal/component/l2tp/pppoe/backend_gate_test.go: APPROVE TestPPPoEVerifyReadsTheBackendPath fixture correction: replace invalid // with the documented # comment syntax; all backend inputs and validation assertions remain unchanged.

No ordinary weakening record was created: this whole test file is new and
absent from HEAD. The spec remains in progress; scoped regression/lint,
functional/stress, promotion and configuration-corpus validation remain owed.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | config-only refusal, no protocol behavior changes | - |

## Files to Modify
- `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` - access-interface backend support declaration
- `internal/component/l2tp/pppoe/register.go` - register the original-tree backend validator
- `docs/guide/pppoe.md`, `features/commit-time-backend-capability-check.md` - the refusal; the latter generates `docs/features.md`

## Files to Create
- `test/parse/pppoe-vpp-backend-refused.ci`, `test/parse/pppoe-netlink-backend-accepted.ci`, `test/reload/pppoe-vpp-backend-reload-refused.ci`
- `test/parse/pppoe-vpp-no-access-accepted.ci`
- `internal/component/l2tp/pppoe/backend_gate.go`, `internal/component/l2tp/pppoe/backend_gate_test.go`
- `internal/test/fixture/register_pppoe_backend_refusal.go` - dedicated editor/reload observer

### Integration Checklist
| Item | Needed | Notes |
|------|--------|-------|
| YANG schema | Yes | `ze:backend` on the access-interface list, walked by the registered original-tree validator |
| CLI grammar | No | |
| Doctor check | No | |

### Documentation Update Checklist (BLOCKING)
| # | Trigger | Needed | Where |
|---|---------|--------|-------|
| 1 | Config refused | Yes | `docs/guide/pppoe.md`, `features/commit-time-backend-capability-check.md` (generates the `docs/features.md` backend gate row) |

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
| Refusal | Public unit registration-overlay runtime red and live-registration green observed; named offline and daemon functional proofs remain owed |

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
- [ ] The targeted package and named functional tests for this change pass; GitHub CI verifies the pushed commit. No agent runs a `./le verify` command
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
