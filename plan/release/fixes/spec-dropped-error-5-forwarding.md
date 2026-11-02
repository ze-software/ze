# Spec: dropped-error-5-forwarding

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Priority | P1 |
| Release reason | 35 errors in interface config, the DHCP client, router advertisements, the netlink and VPP monitors and sysctl are caught and logged only below WARN: a DHCP lease address or route that fails to be removed stays in the kernel, a sysctl apply event is lost, each with no trace at the default level |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/fixes/spec-dropped-error-0-umbrella.md`, which holds the
resolution policy and the tree-wide acceptance. Door: `ai/doors/forwarding.md`.

Population: every `error-dropped-below-warn` finding of
`./le arch refuse-by-default report` under the door's base paths
(`internal/component/{iface,sysrib,vpp,firewall,traffic,trafficfeature,sysctl,mtu}`,
`internal/plugins/{iface,fib,firewall,traffic,policyroute}`). At HEAD on
2026-11-02: **35 instances in 13 files** (DHCP client 15, sysctl 9, iface
component 6, netlink and VPP monitors 4, RA sender 1).

Each instance is resolved by exactly one answer, judged in this order
(umbrella, Key Design Decisions):

| Order | When | Answer |
|-------|------|--------|
| 1 | Going on is wrong: the caller would act on the failure | Return the error, or fix the behavior the drop hid. A kernel address or route Ze installed and failed to remove is state Ze still owns; the failure reaches the code that tracks that ownership |
| 2 | Going on is right and an operator can act | WARN with what, why and the fix (`ai/rules/user-facing-errors.md`) naming the interface and the kernel object |
| 3 | Going on is right and nobody can act (closing a socket at shutdown) | `// refuse-by-default: guards <Symbol> -- <why>` on the line above |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/forwarding.md`
  → Constraint: Ze owns only what it marked; a failed removal of a DHCP-installed address or route leaves Ze-owned state in the kernel, so the failure must reach whatever reconciles ownership
  → Constraint: the report bus (`internal/core/report`) is process-local; a forked plugin's warning never reaches `show warnings`, so a forkable plugin WARNs in its log
  → Constraint: kernel tunables go through `sysctl`; its event emits are how plugins learn a value was applied
- [ ] `docs/architecture/core-design.md`, sections 14 and 15 - backend split and the report bus
  → Constraint: a backend that cannot apply exactly refuses at verify; a runtime error is defence in depth, not the gate
- [ ] `ai/rules/user-facing-errors.md`
  → Constraint: what, why, fix
- [ ] `docs/contributing/ze-go-style.md` - "Error dropped below WARN"
  → Constraint: return, count, or WARN; the annotation names a guard

**Key insights:**
- `runSysctlPlugin` drops the EventBus emit error after a successful apply (seven sites): the value is applied but its subscribers never hear it, which is answer 2 at least.
- The DHCPv6 `shutdown` and `drop` paths remove kernel state; a failure there leaves state behind.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/refusebydefault/dropped.go` - the shape
- [ ] `internal/component/sysctl/register.go` - an `applied` emit that fails is logged at Debug; the default write failure beside it is already at WARN

**Behavior to preserve:**
- What Ze installs in the kernel or VPP, and ownership rules.

**Behavior to change:**
- Each listed instance resolved by one answer.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| refuse-by-default dropped-error shape | `internal/le/arch/refusebydefault/dropped.go` `droppedError` | Lists these 35 | Applies: the acceptance check |
| Guard annotation | `internal/le/arch/refusebydefault/annotation.go` | Accepts a named guard | Applies: answer 3 only |
| Operator warnings | `internal/core/report` (`show warnings`, core-design section 15) | Lasting conditions, in-process producers only | Applies for in-process components (`iface`, `sysctl`) where a failure is a state |
| Interface plugin metrics | `docs/features/interfaces.md`, the iface counters | Interface and DHCP counters | Extends: answer 2 counts where a path repeats |
| Ownership reconcile | `internal/plugins/fib/kernel/monitor.go` `handleExternalChange`, `iface` reconcile | Reasserts what Ze owns | Applies: the consumer of a failed removal under answer 1 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A failed netlink, VPP or `/proc` call, or a failed EventBus emit, inside the forwarding components and plugins.

### Transformation Path
1. The branch catches the error.
2. It returns it to the owner of the kernel state, counts it and WARNs once, WARNs, or carries the annotation.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze ↔ kernel or VPP | netlink, GoVPP | No |
| Component ↔ subscribers | EventBus emit | No |
| Ze ↔ operator | Log at WARN, `show warnings` | No |

### Integration Points
- The loggers, the report bus for in-process components, and the ownership reconcile above.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Existing paths |
| No unintended coupling (components stay isolated) | Yes | No forked plugin imports `internal/core/report` |
| No duplicated functionality (extends existing, does not recreate) | Yes | No new reporting path |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Control-plane paths |
| Registration over hardcoding, outbound | N-A | Nothing registers |
| Registration over hardcoding, inbound | N-A | No list learns a name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 35 are still the population | Report of 2026-11-02 | Work from the fresh report | Report filtered to this child's paths before the first edit | unvalidated |
| A-2 | A failed DHCP removal leaves the object in the kernel | Netlink delete error | An ENOENT (already gone) is answer 3 | Each removal branch distinguishes "already gone" from a real failure | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A WARN per netlink monitor message under a link flap storm | `test/iface` log grows per event | WARN once per interface and condition |
| R-2 | ENOENT on removal at shutdown reported as a failure | WARN at every clean shutdown | Treat "already gone" as success |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Log noise, or a reconcile that now acts on a reported failure |
| How is it reverted? | Single commit revert |
| Who else touches this path? | Specs on iface, DHCP and VPP |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le arch refuse-by-default report` | → | `droppedError` over this child's paths | Report filtered to this child's paths: zero findings of the shape |
| A sysctl default applied whose `applied` emit fails | → | `runSysctlPlugin` | `TestSysctlAppliedEmitFailureIsWarned` |
| A DHCPv6 address that fails to be removed at shutdown | → | `v6Worker.shutdown` | `TestDHCPv6ShutdownReportsFailedRemoval` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le arch refuse-by-default report '|' json`, filtered to this child's paths | No `findings` entry with rule `error-dropped-below-warn` |
| AC-2 | A removal of Ze-owned kernel state that fails for a reason other than "already gone" | The failure reaches its owner (answer 1) or a WARN names the interface, the object and the fix |
| AC-3 | Each answer-1 instance | A unit test where the caller observes the error |
| AC-4 | Each file with an answer-2 WARN | A unit test asserts one WARN line names what, why and the fix |
| AC-5 | Each annotation | Names a guard the file can see |
| AC-6 | Package tests, `./le test integration iface`, and the `.ci` the change reaches | Green; the Linux-only ones in `./le test qemu all` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSysctlAppliedEmitFailureIsWarned` | `internal/component/sysctl/register_test.go` | AC-4 | |
| `TestDHCPv6ShutdownReportsFailedRemoval` | `internal/plugins/iface/dhcp/dhcp_v6_linux_test.go` | AC-2 | |
| One test per other answer-1 instance | The package of the instance | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| unannotated findings under this child's paths | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `.ci` the change reaches under `test/` | `test/` | Unchanged kernel state; WARN where a `.ci` drives the failure | |

## Files to Modify
- `internal/component/iface/config_apply.go` - 1: `addDesiredAddresses` 2218
- `internal/component/iface/config_sysctl.go` - 3: `applySysctl.func` 30; `applySysctlProfiles` 107,124
- `internal/component/iface/register.go` - 2: `runEngine.func.func` 767; `cleanupStaleIPv6DefaultRoutes` 1706
- `internal/component/sysctl/register.go` - 8: `runSysctlPlugin.func` 166,187,203,218,256,275,410; `runSysctlPlugin.func.func` 340
- `internal/component/sysctl/sysctl.go` - 1: `store.saveOriginal` 148
- `internal/plugins/iface/dhcp/dhcp_linux.go` - 2: `DHCPClient.publishDHCP` 263,267
- `internal/plugins/iface/dhcp/dhcp_v4_exchange_linux.go` - 2: `closeV4Socket` 215; `closeV4Client` 230
- `internal/plugins/iface/dhcp/dhcp_v4_linux.go` - 2: `DHCPClient.removeV4Route` 639; `DHCPClient.removeV4Address` 698
- `internal/plugins/iface/dhcp/dhcp_v6_attach_linux.go` - 2: `v6Worker.dropAddresses` 338; `v6Worker.decline` 581
- `internal/plugins/iface/dhcp/dhcp_v6_linux.go` - 7: `closeV6Link` 202; `v6Worker.drop` 567; `v6Worker.expireDue` 584; `v6Worker.shutdown` 602,609,617,622
- `internal/plugins/iface/netlink/monitor_linux.go` - 2: `monitor.emit` 410,414
- `internal/plugins/iface/ra/sender_linux.go` - 1: `Sender.run.func` 236
- `internal/plugins/iface/vpp/monitor.go` - 2: `monitor.emit` 301,306

Lines are those of HEAD on 2026-11-02; the report is the current list.

## Files to Create
- Test additions in the packages above; no new source file.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No config |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No command |
| Functional test for new RPC/API | N-A | No RPC |
| Pipe completeness | N-A | No output change |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | No dependency |
| Prometheus counters/metrics | Yes, if answer 2 counts | Existing interface families; name each counter used at implementation |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | If answer 2 adds a counter | `docs/features/interfaces.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Declared `// Design:` docs: `docs/architecture/core-design.md`, `docs/features/interfaces.md`, `docs/research/vpp-deployment-reference.md`. A log level changes none of them unless answer 1 changes described ownership behavior; confirm with `./le spec citation anchors spec plan/release/fixes/spec-dropped-error-5-forwarding.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- run the report filtered to this child's paths; write the sysctl and DHCPv6 tests red.
2. **Phase: Classify and resolve** -- every instance gets its answer and reason in Design Insights; tests first for answers 1 and 2; annotations last.
3. **Phase: Prove** -- AC-1 to AC-6, Linux-only tests in QEMU.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every instance has its answer at file:line |
| Correctness | "Already gone" is never reported as a failure; a real failed removal is never silent |
| Rule: user-facing-errors | Each WARN names the interface, the kernel object, why and the fix |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Zero findings under this child's paths | `./le arch refuse-by-default report '|' json` filtered by path |
| Package tests | `./le test unit plugins` limited to the changed packages, and `./le job run label forwarding command go test ./internal/component/iface/ ./internal/component/sysctl/` |
| Linux integration | `./le test integration iface`, and `./le test qemu all` for the Linux-only packages |
| Change set judged | `./le verify changed` after the closing commit |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | N-A: no new input |

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

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|

The class row for the whole population is owned by the umbrella.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A failed removal of Ze-owned kernel state reaches its owner | WARN only | Ownership is what keeps Ze from leaking or deleting state another owner installed |
| The umbrella's three answers, in order, elsewhere | Raise every line to WARN | A WARN does not make a default correct |

## Known Limitations
- None.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason): N-A, no wire change

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-dropped-error-5-forwarding.md` only, in the same `./le commit create` script
