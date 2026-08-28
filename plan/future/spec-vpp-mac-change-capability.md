# Spec: vpp-mac-change-capability

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A MAC address the dataplane cannot apply is discovered late, and the operator is
told about it in a number.

The configuration reaches the dataplane, which is correct and stays that way.
The `mac` container carries no backend annotation
(`internal/component/iface/yang/ze-iface-conf.yang`), so
`ValidateBackendFeatures` (`internal/component/config/backend_gate.go`) permits
it under `backend vpp`, and `test/parse/iface-vpp-accepts-ethernet.ci` pins that.
`applyConfig` (`internal/component/iface/config_apply.go`) then calls
`SetMACAddress`, which on this backend is `vppBackendImpl.SetMACAddress`
(`internal/plugins/iface/vpp/query.go`) and sends `SwInterfaceSetMacAddress`.

Two problems follow, and neither is a wrong result. Both are the cost of finding
out too late.

First, some poll-mode drivers do not implement the secondary unicast MAC filter,
so the dataplane refuses the change and returns a negative errno.
`vppBackendImpl.SetMACAddress` turns that into a message whose whole content is
the number: it reports the request name and `retval` and nothing an operator can
act on. Nothing anywhere maps a negative reply to a cause.

Second, verification never asks the question. `parseAndVerifyIfaceSections`
(`internal/component/iface/register.go`) checks the backend gate, the uniqueness
of a match MAC, and the VPP QoS maps. It never contacts the dataplane, so the
refusal can only arrive during apply. `applyConfig` treats it as a fatal step:
it records the error and rolls back the journal, so one interface whose driver
refuses the address stops every later interface in the same commit from being
configured.

Ze already holds what is needed to predict this. `DPDKBinder.bindPCI`
(`internal/component/vpp/dpdk.go`) reads the current kernel driver before binding
and keeps it in `savedDrivers`, whose only reader today is `UnbindAll`. No driver
name is consulted anywhere else, and no capability table exists.

This spec is filed here rather than in `plan/` because the current behavior fails
closed. The address is never silently dropped, the interface is not left down,
and the commit does not report success. What is missing is a verdict at verify
time and an error a person can read.

Goal: refuse the configuration while the operator can still change it, naming the
interface and the driver, and make a late refusal from the dataplane say which
interface and which address it concerns.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `ai/digests/vpp-dataplane.md` - who owns NIC binding, and when it happens
  relative to a config apply
  → Constraint: binding runs at VPP start, so a verify-time check reads state the
  component already holds rather than probing hardware
- [ ] `docs/architecture/iface/management.md` - the split between verify and apply

### RFC Summaries (Scope: protocol)
- [ ] Not applicable. No wire format changes.

**Key insights:** (minimal context to resume after compaction)
- The driver name is captured once at bind time and lives in process memory only.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/iface/vpp/query.go` - `SetMACAddress` parses the address,
  sends `SwInterfaceSetMacAddress`, and reports a non-zero reply as a number
- [ ] `internal/component/iface/config_apply.go` - the MAC step, its rollback to
  the previous address, and the abort of the whole apply on failure
- [ ] `internal/component/iface/register.go` - `parseAndVerifyIfaceSections`, the
  verify-time checks that exist today
- [ ] `internal/component/vpp/dpdk.go` - `bindPCI` reads the current driver and
  stores it; `UnbindAll` is its only reader
- [ ] `internal/component/iface/backend.go` - the `iface.Backend` contract both
  backends implement

**Behavior to preserve:**
- The MAC reaching the dataplane on backends that accept it.
- The rollback to the previous address when the change fails.
- `test/parse/iface-vpp-accepts-ethernet.ci`, which pins acceptance of the `mac`
  container under `backend vpp`.

**Behavior to change:**
- A MAC the dataplane cannot apply is refused at verify time.
- A refusal that still arrives at apply time names the interface and the address.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config `interface ethernet <name> mac address <value>` with `interface backend
  vpp`, read at commit time.

### Transformation Path
1. `ValidateBackendFeatures` (`internal/component/config/backend_gate.go`) permits
   the container, which carries no backend annotation.
2. `parseAndVerifyIfaceSections` (`internal/component/iface/register.go`) runs the
   verify-time checks, none of which concerns the dataplane's ability to apply an
   address.
3. `applyConfig` (`internal/component/iface/config_apply.go`) reads the previous
   address, then calls the backend's `SetMACAddress` inside a journal step.
4. `vppBackendImpl.SetMACAddress` (`internal/plugins/iface/vpp/query.go`) resolves
   the index, parses the address, and sends the request. A non-zero reply becomes
   an error carrying the number.
5. The apply records the error and rolls back, so the commit fails.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config verify ↔ VPP component state | would read the driver captured at bind time | Unverified |
| iface component ↔ VPP backend | `iface.Backend.SetMACAddress` | Read |
| VPP backend ↔ dataplane | govpp binary API request and reply | Read |

### Integration Points
- `DPDKBinder` (`internal/component/vpp/dpdk.go`) - already holds the original
  driver per interface.
- `parseAndVerifyIfaceSections` (`internal/component/iface/register.go`) - the
  place a verdict would be reached while the operator is still in the commit.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The set of drivers that cannot change a MAC is knowable ahead of the attempt | the dataplane refuses with a fixed errno for a missing filter callback | the verdict must come from a probe rather than a table | a run against a driver known to refuse | unvalidated |
| A-2 | The driver captured at bind time is the right key | `bindPCI` records the kernel driver before the interface becomes a dataplane port | the check reads the tap driver and always passes | read `savedDrivers` on a running system | unvalidated |
| A-3 | A refusal never leaves the interface down, because the address step runs before the admin-state step | the ordering of the phases in `applyConfig` | the fix must also restore the admin state | the functional test asserts the state after a refused commit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A hardcoded driver list ages badly and refuses a driver that gained the capability | an operator reports a refusal for a working NIC | keep the list in one place, name it in the error, and say how to override |
| R-2 | The verify step reaches into another component's memory and couples the two | a new import from the config path into the VPP component | expose the driver through an existing accessor, not a new dependency |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A refusal that is too wide rejects a valid configuration at commit. A refusal that is too narrow leaves today's late failure in place. |
| How is it reverted? | Single commit revert. No config migration. |
| Who else touches this path? | `plan/spec-vpp-loopback-mac.md` sends a MAC on the create call; `plan/spec-vpp-numa-smt.md` reads the same DPDK settings. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `mac address` on an interface whose driver cannot apply it | → | the verify-time check | `iface-vpp-rejects-mac-on-unsupported-driver.ci` |
| a dataplane refusal during apply | → | `vppBackendImpl.SetMACAddress` | `TestSetMACAddressErrorNamesInterface` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `mac address` set on a VPP interface whose driver cannot apply it | The commit is refused, and the message names the interface, the address, and the driver |
| AC-2 | The same configuration on a driver that can apply it | Accepted and applied, exactly as today |
| AC-3 | The dataplane refuses the request despite the verify-time verdict | The error names the interface and the address, and no bare number is the whole message |
| AC-4 | A refused commit | The interface set is left as it was before the commit, and the operator can commit a corrected config immediately |
| AC-5 | Any interface on the netlink backend | Unchanged behavior, and no new verify-time refusal |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets a MAC on a VPP-owned NIC and commits | config → verify → apply → dataplane | `iface-vpp-rejects-mac-on-unsupported-driver.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyMACChangeSupported` | `internal/component/iface/register_test.go` | the verdict per driver | | <!-- doc-links: ignore (artifact this spec will create; it is future work and nothing has built it) -->
| `TestSetMACAddressErrorNamesInterface` | `internal/plugins/iface/vpp/apply_test.go` | the error text of a refused request | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| MAC length | 6 bytes | 6 | 5 | 7 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `iface-vpp-rejects-mac-on-unsupported-driver.ci` | `test/parse/` | the operator is told at commit time that this NIC cannot take a MAC address | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | no wire-visible protocol behavior changes | |

## Files to Modify
- `internal/component/iface/register.go` - the verify-time verdict
- `internal/plugins/iface/vpp/query.go` - the error text of a refused request
- `internal/component/vpp/dpdk.go` - expose the captured driver to a reader other
  than the unbind path

## Files to Create
- `test/parse/iface-vpp-rejects-mac-on-unsupported-driver.ci` - the commit-time <!-- doc-links: ignore (artifact this spec will create; it is future work and nothing has built it) -->
  refusal

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | no new leaf |
| YANG validation constraints | No | the address pattern is unchanged |
| YANG custom validators | | decide during design: the verdict needs the running driver, which a schema validator cannot see |
| CLI commands/flags | No | no new command |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | No | no new leaf |
| Functional test for new RPC/API | No | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | No | no environment leaf |
| Doctor check for runtime dependencies | | a check could report an interface whose configured MAC the driver cannot apply |
| Prometheus counters/metrics | No | no new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | Yes | the `iface-vpp` refusal behavior |
| 6 | Has a user guide page? | | `docs/guide/interfaces.md` | <!-- doc-links: ignore (artifact this spec will create; it is future work and nothing has built it) -->
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | | `ai/digests/vpp-dataplane.md` if the driver record gains a second reader |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | | grep `docs/` for the changed files |
| 17 | Existing docs show config/CLI/API examples for this area? | | check the MAC examples under the VPP backend |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - make the late failure visible from the
   entry point
   - Tests: `TestSetMACAddressErrorNamesInterface`
   - Files: `internal/plugins/iface/vpp/apply_test.go`
   - Verify: the test fails against today's number-only message
2. **Phase: A readable refusal from the dataplane**
   - Tests: the same test
   - Files: `internal/plugins/iface/vpp/query.go`
   - Verify: the error names the interface and the address
3. **Phase: The verdict moves to verify time**
   - Tests: `TestVerifyMACChangeSupported`,
     `iface-vpp-rejects-mac-on-unsupported-driver.ci`
   - Files: `internal/component/iface/register.go`,
     `internal/component/vpp/dpdk.go`
   - Verify: the commit is refused before any interface is touched

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Correctness | The verdict keys on the driver captured before binding, not on the tap |
| Naming | The error names the interface, the address and the driver, in that order |
| Data flow | The config path reads the driver through an accessor, and does not import the binder |
| Rule: `ai/rules/evidence.md` | The claim that a driver cannot apply a MAC is settled by a run or by the driver's own source, not by repetition |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| A verify-time refusal | the `.ci` test passes and fails when the check is reverted |
| A readable dataplane error | the unit test asserts the interface name in the message |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Error leakage | The message names an interface and a driver, never a path or a socket |
| Fail closed | An unknown driver must not be treated as capable without saying so |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- The spec does not make the driver record survive a restart. It is read while
  the daemon that captured it is running, which is when a commit happens.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-5 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify current mode full` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/speclifecycle/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
