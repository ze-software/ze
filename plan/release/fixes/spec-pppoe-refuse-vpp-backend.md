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
| Updated | 2026-11-23 |

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
| `pppoe-vpp-backend-reload-refused` | `test/reload/` | AC-4 | Promoted (`5f8fef2c4f`); 1/1 pass (5.0s) and 80/80 stress pass under load on a no-`ze_vpp` binary, cleanly skipped on a `ze_vpp` binary (`option=needs-build-tag:value=ze_vpp:state=absent`), 2026-11-23 |

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
| Refusal | `TestPPPoEVerifyRefusesVPPBackend` (registration-overlay red + live green), `pppoe-vpp-backend-refused.ci` (410ms), `pppoe-netlink-backend-accepted.ci` (591ms), `pppoe-vpp-no-access-accepted.ci` (403ms), `pppoe-vpp-backend-reload-refused.ci` (1/1 5.0s + 80/80 stress on no-`ze_vpp`, skipped on `ze_vpp`), `./internal/component/l2tp/pppoe/` (13.876s), `TestRepositoryConfigsValidate` (230.535s) — all passed 2026-11-23 |

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

---

## Implementation Summary

### What Was Implemented
- Registered original-tree validator `pppoe-backend` (`validateBackendGate`, `internal/component/l2tp/pppoe/backend_gate.go:20-58`) in `init()` (`internal/component/l2tp/pppoe/register.go:11`).
- Implemented `sessionBackendSelection` and exported `ConfiguredSessionBackendName` (`internal/component/l2tp/pppoe/backend_gate.go:63-93`), failing closed via `backendSchemaError` when the `interface/backend` schema leaf or default is unavailable.
- Updated `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` (`list interface`, lines 161-167) to document that access interfaces require a registered session provider for the selected interface backend.
- Added unit tests `TestPPPoEVerifyRefusesVPPBackend`, `TestPPPoEVerifyAcceptsDefaultBackend`, `TestPPPoEVerifyAllowsNoAccessInterface`, and `TestPPPoEVerifyReadsTheBackendPath` (`internal/component/l2tp/pppoe/backend_gate_test.go`) plus kernel provider test registration (`internal/component/l2tp/pppoe/backend_kernel_registration_test.go`).
- Added offline parse tests `test/parse/pppoe-vpp-backend-refused.ci`, `test/parse/pppoe-netlink-backend-accepted.ci`, and `test/parse/pppoe-vpp-no-access-accepted.ci`.
- Added live commit + SIGHUP reload refusal test `test/reload/pppoe-vpp-backend-reload-refused.ci` and its observer fixture `pppoeBackendRefusal` (`internal/test/fixture/register_pppoe_backend_refusal.go`, landed in `5f8fef2c4f`).
- Flipped the owned 2026-10-28 journal row in `plan/journal/validate-accepts-what-start-refuses.md` from `open:` to `fixed:`, added operator news fragment `website/changes/fragments/pppoe-refuse-vpp-backend.md`, and repointed all 6 path citations in `plan/post-release/features/spec-pppoe-vpp-dataplane.md` to bare stem `spec-pppoe-refuse-vpp-backend`.

### Bugs Found/Fixed
- Multi-finding commit error parsing in `pppoeBackendCommit` (`internal/test/fixture/register_pppoe_backend_refusal.go`): when `interface`'s `ConfigNeed` appended a second validation finding on no-`ze_vpp` builds, a whole-message `HasSuffix` check failed. Fixed in `5f8fef2c4f` by adding `pppoeBackendFinding` to isolate the `/pppoe/interface/access0` finding block and strictly assert its final line equals `fix: set interface backend netlink` (approved via `Test-judge:`).

### Documentation Updates
- `docs/guide/pppoe.md` (lines 118-128): documents `validateBackendGate`, `ze_vpp` build tag, no-access-interface carve-out, and `pppoe-client` separation; anchors `<!-- source: internal/component/l2tp/pppoe/backend_gate.go -- validateBackendGate -->` and `<!-- source: internal/component/l2tp/pppoe/register.go -- original-tree validator registration -->`.
- `features/commit-time-backend-capability-check.md` (lines 11-19): lists `internal/component/l2tp/pppoe/backend_gate.go`, all four `.ci` tests, updated Doc review (2026-11-23), and Description anchored to `validateBackendGate`.
- `docs/architecture/l2tp/bng-5-pppoe.md` (lines 187-196) and `docs/guide/vpp.md` (lines 340-346): document `validateBackendGate` and `ConfiguredSessionBackendName`, both anchored to `internal/component/l2tp/pppoe/backend_gate.go`.
- `./le doc check all`: 0 drift (`Documentation drift`), all 562 YANG commands validated, 0 issues on any PPPoE/config/backend_gate source anchor (the 2 reported source-anchor errors are unrelated pre-existing BGP `rsRelaySendGranted` anchors in `docs/architecture/bgp/structural-forwarding.md:117` and `docs/guide/route-reflection.md:491`), all 3169 digest anchors resolve.

### Deviations from Plan
- **Dynamic provider registry instead of static `ze:backend "netlink"` YANG annotation:** Q-1 originally planned a static `ze:backend "netlink"` annotation on `list interface` walked via `config.ValidateBackendFeaturesJSON`. When the shared PPPoE session dataplane registry landed (`59f67661db` / `b7748c4aa6`), `validateBackendGate` (`internal/component/l2tp/pppoe/backend_gate.go`) evolved to query `dataplane.LookupSessionDataplane(backend)` directly instead of a static YANG tag. As a result, builds compiled with `ze_vpp` admit the registered VPP provider while builds without `ze_vpp` refuse `interface backend vpp` with the exact AC-1 message (naming `/pppoe/interface/<name>`, `backend "vpp"`, `supported: netlink`, `no PPPoE session data plane`, `ze_vpp`, and `fix: set interface backend netlink`). Accordingly, `test/parse/pppoe-vpp-backend-refused.ci` and `test/reload/pppoe-vpp-backend-reload-refused.ci` declare `option=needs-build-tag:value=ze_vpp:state=absent`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | `TestPPPoEVerifyReadsTheBackendPath` initially used `//` comments in its inline config fixture | Ze config syntax uses `#` comments | Unit test parse failure before validation (2026-11-15) | Replaced `//` with `#` after independent test-judge APPROVE |
| approach | `pppoe-vpp-backend-reload-refused.ci` initially lacked `option=needs-linux` | Explicit `interface` root auto-loads Linux interface monitor on startup | Native Darwin failure before observer (2026-11-15) | Added `option=needs-linux` and ran in Ze Linux QEMU guest |
| approach | `pppoeBackendCommit` checked `strings.HasSuffix` on the entire gRPC error string | `ValidateContent` returns all findings as separate blocks each ending with its own `fix:` line; `interface` `ConfigNeed` adds a second finding on no-`ze_vpp` builds | Live no-`ze_vpp` reload run (2026-11-23) | Scoped assertions via `pppoeBackendFinding` in `5f8fef2c4f` after `Test-judge:` approval |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refuse `interface backend vpp` + `pppoe` access interface at validate, commit, and reload verify | Done | `internal/component/l2tp/pppoe/backend_gate.go:20-58` (`validateBackendGate`), `internal/component/l2tp/pppoe/register.go:11` (`init`) | Registered original-tree validator; refuses whenever no session provider is registered for `interface backend` |
| Preserve PPPoE under default `netlink` backend | Done | `internal/component/l2tp/pppoe/backend_gate.go:68-93` (`sessionBackendSelection`) | Reads `interface/backend` YANG default (`netlink`); kernel provider admitted |
| Preserve `pppoe` block with no access interface under VPP | Done | `internal/component/l2tp/pppoe/backend_gate.go:28-30` (`validateBackendGate`) | Early `return nil` when `len(access.GetList("interface")) == 0` |
| Preserve `iface` `pppoe-client` units | Done | `internal/component/l2tp/pppoe/backend_gate.go:24` | Only inspects `pppoe` root container, never `interface/pppoe-client` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestPPPoEVerifyRefusesVPPBackend` (`backend_gate_test.go:15-33`), `test/parse/pppoe-vpp-backend-refused.ci` | Exits 1; names `/pppoe/interface/access0`, `backend "vpp"`, `supported: netlink`, `no PPPoE session data plane`, `ze_vpp`, `fix: set interface backend netlink` |
| AC-2 | Done | `TestPPPoEVerifyAcceptsDefaultBackend` (`backend_gate_test.go:37-57`), `test/parse/pppoe-netlink-backend-accepted.ci` | Schema default is `netlink`; validate exits 0 (`configuration valid`) |
| AC-3 | Done | `TestPPPoEVerifyAllowsNoAccessInterface` (`backend_gate_test.go:61-66`), `test/parse/pppoe-vpp-no-access-accepted.ci` | `interface backend vpp` with `pppoe { ac-name proof }` exits 0 (`configuration valid`) |
| AC-4 | Done | `test/reload/pppoe-vpp-backend-reload-refused.ci` + `pppoeBackendRefusal` (`internal/test/fixture/register_pppoe_backend_refusal.go:29-223`) | Proves gRPC editor commit refusal, SIGHUP reload refusal, unchanged running config after both, and subsequent valid reload apply (1/1 5.0s + 80/80 stress on no-`ze_vpp` binary) |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestPPPoEVerifyRefusesVPPBackend` | Done | `internal/component/l2tp/pppoe/backend_gate_test.go:15` | Passed in `./le test unit pkg path ./internal/component/l2tp/pppoe/` (13.876s, 2026-11-23) |
| `TestPPPoEVerifyAcceptsDefaultBackend` | Done | `internal/component/l2tp/pppoe/backend_gate_test.go:37` | Passed (2026-11-23) |
| `TestPPPoEVerifyAllowsNoAccessInterface` | Done | `internal/component/l2tp/pppoe/backend_gate_test.go:61` | Passed (2026-11-23) |
| `TestPPPoEVerifyReadsTheBackendPath` | Done | `internal/component/l2tp/pppoe/backend_gate_test.go:70` | Passed (2026-11-23) |
| `pppoe-vpp-backend-refused` | Done | `test/parse/pppoe-vpp-backend-refused.ci` | Passed (410ms, 2026-11-23) |
| `pppoe-netlink-backend-accepted` | Done | `test/parse/pppoe-netlink-backend-accepted.ci` | Passed (591ms, 2026-11-23) |
| `pppoe-vpp-no-access-accepted` | Done | `test/parse/pppoe-vpp-no-access-accepted.ci` | Passed (403ms, 2026-11-23) |
| `pppoe-vpp-backend-reload-refused` | Done | `test/reload/pppoe-vpp-backend-reload-refused.ci` | Passed 1/1 (5.0s) and 80/80 stress on no-`ze_vpp` binary; skipped cleanly on `ze_vpp` binary (2026-11-23) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` | Changed | Documents runtime session-provider requirement on `list interface` instead of static `ze:backend` tag (see Deviations) |
| `internal/component/l2tp/pppoe/register.go` | Done | Registers `"pppoe-backend"` -> `validateBackendGate` in `init()` |
| `internal/component/l2tp/pppoe/backend_gate.go` | Done | Implements `validateBackendGate`, `ConfiguredSessionBackendName`, `sessionBackendSelection`, `backendSchemaError` |
| `internal/component/l2tp/pppoe/backend_gate_test.go` | Done | Unit tests for AC-1..AC-3 and parsed-path isolation |
| `internal/component/l2tp/pppoe/backend_kernel_registration_test.go` | Done | Links `pppoe-kernel` provider for unit tests |
| `internal/test/fixture/register_pppoe_backend_refusal.go` | Done | Live editor commit + SIGHUP reload refusal observer (`5f8fef2c4f`) |
| `test/parse/pppoe-vpp-backend-refused.ci` | Done | Offline refusal test (AC-1) |
| `test/parse/pppoe-netlink-backend-accepted.ci` | Done | Offline default-backend acceptance test (AC-2) |
| `test/parse/pppoe-vpp-no-access-accepted.ci` | Done | Offline no-access-interface acceptance test (AC-3) |
| `test/reload/pppoe-vpp-backend-reload-refused.ci` | Done | Live commit + SIGHUP reload refusal test (AC-4, `5f8fef2c4f`) |
| `docs/guide/pppoe.md` | Done | Updated with backend refusal and `ze_vpp` requirement + source anchors |
| `features/commit-time-backend-capability-check.md` | Done | Updated with `pppoe/backend_gate.go` and all 4 `.ci` tests |

### Audit Summary
- **Total items:** 28 (4 task requirements, 4 ACs, 8 tests, 12 files)
- **Done:** 27
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (`ze-pppoe-conf.yang`: dynamic provider registry lookup replaced static `ze:backend` tag; recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Refuse `interface backend vpp` + `pppoe` access interface at offline validate before any session can fail at IPCP Opened | Functional + Unit test (with registration-omission red counterfactual) | `test/parse/pppoe-vpp-backend-refused.ci` PASS (410ms, 2026-11-23); `TestPPPoEVerifyRefusesVPPBackend` PASS (`./internal/component/l2tp/pppoe/`, 13.876s, 2026-11-23); registration-omission counterfactual failed with exit 0 (`configuration valid`) |
| Refuse switching a running PPPoE config to unsupported `interface backend vpp` at editor commit and SIGHUP reload while keeping running config unchanged | Functional daemon test + 80x stress | `test/reload/pppoe-vpp-backend-reload-refused.ci` PASS 1/1 (5.0s) and 80/80 stress under load on no-`ze_vpp` binary, skipped cleanly on `ze_vpp` binary (2026-11-23); verifies `OK: PPPoE backend commit refused`, `OK: PPPoE backend reload refused`, unchanged `show config running`, and recovery via `config3.conf` |
| Preserve valid PPPoE configs (default `netlink` backend, VPP with no access interface, and repository config corpus) | Functional + Corpus validation | `test/parse/pppoe-netlink-backend-accepted.ci` PASS (591ms), `test/parse/pppoe-vpp-no-access-accepted.ci` PASS (403ms), `TestRepositoryConfigsValidate` PASS (230.535s, 2026-11-23) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None — all in-scope verify-time refusal deliverables (AC-1..AC-4) are complete; the VPP PPPoE session data plane itself is out of scope per OD-5 | Separate post-release feature per OD-5 (2026-10-28) | `plan/post-release/features/spec-pppoe-vpp-dataplane.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/pppoe-refuse-vpp-backend-410e80c8-b20a-447d-a6b8-c87b6b1ac94b.md` |
| `./le spec review check` | `review_gate: OK (10 code files, clean, hashes match tmp/review/pppoe-refuse-vpp-backend-410e80c8-b20a-447d-a6b8-c87b6b1ac94b.md)` |
| Rounds | 1 |
| Reviewer lenses used | logic+wiring, security+edge-cases, style+rules (`/ze-review` 3-lens guard audit + 7-question Go style pass) and `/ze-review-spec` |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | NOTE | Spec predates `## Design documents` table; verified all 4 derived decision rows in `docs/architecture/l2tp/bng-5-pppoe.md` are VPP API-loss/adoption lifecycle decisions unaffected by `backend_gate.go` | `plan/release/fixes/spec-pppoe-refuse-vpp-backend.md` | NOTE-only; 0 BLOCKER, 0 ISSUE |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/l2tp/pppoe/backend_gate.go` | Yes | `-rw-rw-r-- 1 thomas thomas 3550 Nov 21 02:19 internal/component/l2tp/pppoe/backend_gate.go` |
| `internal/component/l2tp/pppoe/backend_gate_test.go` | Yes | `-rw-rw-r-- 1 thomas thomas 3430 Nov 16 20:01 internal/component/l2tp/pppoe/backend_gate_test.go` |
| `internal/component/l2tp/pppoe/backend_kernel_registration_test.go` | Yes | `-rw-rw-r-- 1 thomas thomas 400 Nov 16 12:17 internal/component/l2tp/pppoe/backend_kernel_registration_test.go` |
| `internal/component/l2tp/pppoe/register.go` | Yes | `-rw-rw-r-- 1 thomas thomas 841 Nov 16 12:17 internal/component/l2tp/pppoe/register.go` |
| `internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` | Yes | `-rw-rw-r-- 1 thomas thomas 10326 Nov 16 20:01 internal/component/l2tp/pppoe/yang/ze-pppoe-conf.yang` |
| `internal/test/fixture/register_pppoe_backend_refusal.go` | Yes | `-rw-rw-r-- 1 thomas thomas 8004 Nov 23 10:15 internal/test/fixture/register_pppoe_backend_refusal.go` |
| `test/parse/pppoe-vpp-backend-refused.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 773 Nov 18 00:54 test/parse/pppoe-vpp-backend-refused.ci` |
| `test/parse/pppoe-netlink-backend-accepted.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 575 Nov 18 00:54 test/parse/pppoe-netlink-backend-accepted.ci` |
| `test/parse/pppoe-vpp-no-access-accepted.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 471 Nov 20 18:00 test/parse/pppoe-vpp-no-access-accepted.ci` |
| `test/reload/pppoe-vpp-backend-reload-refused.ci` | Yes | `-rw-rw-r-- 1 thomas thomas 3369 Nov 23 09:59 test/reload/pppoe-vpp-backend-reload-refused.ci` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `ze config validate` refuses `interface backend vpp` + `pppoe interface access0` with path, backend, supported provider, reason, and fix | `internal/component/l2tp/pppoe/backend_gate.go:20-58` (`validateBackendGate`), `register.go:11`; `TestPPPoEVerifyRefusesVPPBackend` PASS (2026-11-23) + `test/parse/pppoe-vpp-backend-refused.ci` PASS (410ms, 2026-11-23) |
| AC-2 | Same tree with default backend accepted | `internal/component/l2tp/pppoe/backend_gate.go:68-93` (`sessionBackendSelection`); `TestPPPoEVerifyAcceptsDefaultBackend` PASS + `test/parse/pppoe-netlink-backend-accepted.ci` PASS (591ms, 2026-11-23) |
| AC-3 | `interface backend vpp` with `pppoe` naming no access interface accepted | `internal/component/l2tp/pppoe/backend_gate.go:28-30` (`len(access.GetList("interface")) == 0`); `TestPPPoEVerifyAllowsNoAccessInterface` PASS + `test/parse/pppoe-vpp-no-access-accepted.ci` PASS (403ms, 2026-11-23) |
| AC-4 | Live editor commit and SIGHUP reload switching to `interface backend vpp` refused with AC-1 message, running config unchanged | `internal/test/fixture/register_pppoe_backend_refusal.go:29-223` (`pppoeBackendRefusal`, `pppoeBackendCommit`, `pppoeBackendFinding`); `test/reload/pppoe-vpp-backend-reload-refused.ci` PASS 1/1 (5.0s) and 80/80 stress on no-`ze_vpp` binary, skipped cleanly on `ze_vpp` binary (2026-11-23) |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate -` (`interface { backend vpp } pppoe { interface access0 {} }`) -> `config.ValidateCustomSections` -> `pppoe.validateBackendGate` | `test/parse/pppoe-vpp-backend-refused.ci` | Yes — read lines 1-20: pipes stdin config to `ze config validate -`, asserts exit 1 and all 6 needles (`/pppoe/interface/access0`, `backend "vpp"`, `supported: netlink`, `no PPPoE session data plane`, `fix: set interface backend netlink`, `ze_vpp`); passed 410ms (2026-11-23) |
| Live gRPC `CommitSession` + SIGHUP reload -> `pppoe.validateBackendGate` -> unchanged `show config running` -> subsequent valid reload | `test/reload/pppoe-vpp-backend-reload-refused.ci` | Yes — read lines 1-110 and `register_pppoe_backend_refusal.go`: drives `CommitSession` and `liveStagesReload("config2.conf")`, verifies refusal needles + `reflect.DeepEqual` on running tree + `config3.conf` apply + disk store contains no `backend vpp`; passed 1/1 (5.0s) and 80/80 stress (2026-11-23) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestPPPoEVerifyAcceptsDefaultBackend` (`internal/component/l2tp/pppoe/backend_gate_test.go:37-57`) verifies `schema.Lookup("interface/backend")` has `leaf.Default == "netlink"` and accepts an omitted backend leaf; passed in `./le test unit pkg path ./internal/component/l2tp/pppoe/` (13.876s, 2026-11-23) and `test/parse/pppoe-netlink-backend-accepted.ci` (591ms, 2026-11-23) |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| User guide: `docs/guide/pppoe.md` (lines 118-128) | Anchors `internal/component/l2tp/pppoe/backend_gate.go -- validateBackendGate` and `register.go`; matches `validateBackendGate` (`backend_gate.go:20-58`) | Yes |
| Feature declaration: `features/commit-time-backend-capability-check.md` (lines 11-19) | Lists `internal/component/l2tp/pppoe/backend_gate.go` and all 4 `.ci` tests; Description + Doc review updated 2026-11-23 against `validateBackendGate` | Yes |
| Architecture & VPP guide: `docs/architecture/l2tp/bng-5-pppoe.md` (lines 187-196), `docs/guide/vpp.md` (lines 340-346) | Both anchor `internal/component/l2tp/pppoe/backend_gate.go -- validateBackendGate` / `ConfiguredSessionBackendName`; `./le doc check all` confirmed 0 PPPoE/config drift or anchor errors | Yes |
| Problem journal: `plan/journal/validate-accepts-what-start-refuses.md` (line 18) | Flipped 2026-10-28 row from `open:` to `fixed:`; `./le spec journal check file plan/journal/validate-accepts-what-start-refuses.md` returned `is valid (18 row(s))` | Yes |
| Operator news fragment: `website/changes/fragments/pppoe-refuse-vpp-backend.md` | Written with required front matter (`spec`, `topic: PPPoE`, `feature: commit-time-backend-capability-check`) and all 6 headings | Yes |
| Other categories (CLI grammar, API/RPC, plugin SDK, wire format, RFC compliance) | `git grep -n "backend_gate.go" docs/ features/ ai/` shows all anchored claims are covered above; config-only refusal adds no CLI command, RPC, wire format, or RFC obligation | Yes |
