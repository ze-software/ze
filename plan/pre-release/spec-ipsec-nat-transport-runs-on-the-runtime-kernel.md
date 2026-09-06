# Spec: ipsec-nat-transport-runs-on-the-runtime-kernel

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Where this came from.** `spec-ipsec-transport-nat-selector-substitution` implemented
RFC 7296 Section 2.23.1's transport-mode selector substitution and proved it against
strongSwan across a netfilter NAT in Docker. It closed on 2026-09-06 with two of its
acceptance criteria unbuilt, and this spec owns them. It reduced nothing: AC-11 and AC-12
of that spec are restated below as AC-1 and AC-2 here, and its `.ci` row is AC-3.

**The gap.** The Docker proof runs on the DEVELOPMENT host's kernel. Ze ships an
appliance with a kernel of its own, and nothing measures that the substitution works
there. `gokrazy/kernel/runtime.config` carries `CONFIG_IP_NF_NAT=y` and
`CONFIG_IP_NF_TARGET_MASQUERADE=y`, and `gokrazy/kernel/runtime.require` names neither, so
a demotion to `=m` would pass the build and break the feature with nothing red.
`ai/rules/platform-linux.md` requires the QEMU proof for Linux-only behavior, and the
IPsec lab has no row in the QEMU lab table at all.

**The second gap.** There is no `.ci` covering an operator who brings up a transport-mode
peer behind a NAT and reads the substituted selectors back. `test/ipsec/ipsec-sa-show.ci`
reaches the SA payload, including `original-tsi` and `original-tsr`, but its two instances
sit on loopback with no translation between them, so every substitution there is the
identity.

**Goals.**

| ID | Goal |
|----|------|
| G-1 | Ze establishes a transport-mode Child SA with strongSwan across a masquerading network namespace, on Ze's own runtime kernel, and carries traffic over it |
| G-2 | A demotion of the netfilter NAT kernel symbols fails the appliance build rather than the feature |
| G-3 | An operator path through the daemon asserts the substituted selectors and the pre-substitution originals |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/testing/qemu-integration.md` - the lab table that pairs each Docker lab with a QEMU action, and the IPsec row that does not exist
- [ ] `docs/architecture/testing/interop.md` - "The IPsec NAT box", which states the topology this runner has to reproduce with namespaces instead of containers
- [ ] `docs/guide/ipsec.md` - "Transport mode behind a NAT", the operator contract the `.ci` asserts
- [ ] `ai/rules/platform-linux.md` - the QEMU obligation and the requirement that the action have a real caller in the same change

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7296.md` - rows RFC7296-2.23.1-1, -2 and -3, already proven by unit tests and by the Docker scenarios
- [ ] `rfc/short/rfc3948.md` - UDP encapsulation of ESP, whose transport-mode checksum handling the runtime kernel has to take as the Docker host's does

**Key insights:** (minimal context to resume after compaction)
- The Docker lab puts the NAT in its own CONTAINER with a secondary address per peer, because `interoplab.ScenarioPlan` carries one `NetworkSpec`. A QEMU runner has namespaces instead, so the middle namespace can hold two segments and the design question is whether to keep the one-segment shape for parity or use two.
- `real-nat-tunnel-control` is the control that makes a red transport verdict readable. The QEMU runner owes the same control or it cannot tell a broken topology from a broken substitution.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/qemu/actions.go` - where a QEMU action registers, and the shape every existing action takes
- [ ] `internal/le/qemu/alltests.go` - the integration package a runner adds itself to
- [ ] `internal/le/interoplab/ipsec/nat.go` - `readNATConfig` and `natSetupScript`, the iptables rules the namespace runner has to reproduce
- [ ] `internal/le/interoplab/ipsec/checkers.go` - `checkRealNATTransport` and `checkRealNATTunnelControl`, the assertions the QEMU proof owes
- [ ] `gokrazy/kernel/runtime.require` - the symbol list a demotion has to fail against
- [ ] `test/ipsec/ipsec-sa-show.ci` - the `.ci` shape, its `option=` lines, and the SA payload keys it already asserts
- [ ] `internal/component/ike/cmd/show_ipsec.go` - `saToMap`, whose `original-tsi` and `original-tsr` the `.ci` asserts

**Behavior to preserve:** (unless the user explicitly said to change it)
- The three Docker scenarios and their checkers. This spec adds a second proof, never a replacement.
- Every existing `./le qemu` action and its caller.

**Behavior to change:** (only what the user asked for)
- A QEMU action exists, is listed, is documented in the lab table, and is called by a workflow job.
- `runtime.require` names the two netfilter NAT symbols.
- A `.ci` asserts the operator path.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [Where data enters - to be written at design time. The action is `./le qemu <name>`, and the operator entry point of the `.ci` is `show vpn ipsec sa`.]

### Transformation Path
1. [To be written at design time.]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Runner ↔ kernel | the middle namespace's iptables NAT rules translate the IKE and ESP datagrams | No |
| Engine ↔ dataplane | the substituted selectors reach XFRM on the runtime kernel | No |

### Integration Points
- `internal/le/qemu/actions.go` - the action registry.
- `gokrazy/kernel/runtime.require` - the kernel symbol floor.
- `test/ipsec/` - the functional suite.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le qemu ipsec-nat-transport-test` | → | the three-namespace runner | the action's own assertions |
| a workflow job | → | the same action | the job named in the same commit |
| `mode transport` peer behind a translation, `show vpn ipsec sa` | → | `saToMap` and the substitution | `test/ipsec/ipsec-transport-nat-selectors.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le qemu ipsec-nat-transport-test` on Ze's runtime kernel | Ze and strongSwan establish transport mode across a masquerading namespace, traffic crosses it, and a tunnel-mode control over the same topology stays green |
| AC-2 | `./le qemu` with no arguments, and the lab table in `docs/architecture/testing/qemu-integration.md` | The action is listed in both, a workflow job names it, and `gokrazy/kernel/runtime.require` names `CONFIG_IP_NF_NAT` and `CONFIG_IP_NF_TARGET_MASQUERADE` so a demotion to `=m` fails the build |
| AC-3 | `test/ipsec/ipsec-transport-nat-selectors.ci` | An operator brings up a transport-mode peer across a translation and reads the substituted `ts-local` and `ts-remote` plus the pre-substitution `original-tsi` and `original-tsr` from `show vpn ipsec sa` |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs Ze on the appliance with a transport-mode peer behind a NAT | config → IKE_AUTH → substitution → XFRM on the runtime kernel | `ipsec-nat-transport-test` |
| 2 | Reads the selectors the tunnel really carries | `show vpn ipsec sa` → `saToMap` | `ipsec-transport-nat-selectors.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [the runner's namespace and rule construction, driven without a VM] | `internal/le/qemu/` | the rules match `natSetupScript`'s, so the two proofs cannot diverge | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| namespaces per run | 3 | 3 | 2, which has no middlebox | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ipsec-transport-nat-selectors` | `test/ipsec/ipsec-transport-nat-selectors.ci` | AC-3, with `option=needs-linux` and the capability the XFRM install needs | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `real-nat-transport-ze-initiator` (existing) | `test/interop-ipsec/scenarios/` | strongSwan | the Docker half, already green | done |
| `ipsec-nat-transport-test` | `internal/le/qemu/` | strongSwan | the same path on Ze's runtime kernel | |

## Files to Modify
- `internal/le/qemu/actions.go` - register `ipsec-nat-transport-test`
- `internal/le/qemu/alltests.go` - the integration package, if the runner adds one
- `gokrazy/kernel/runtime.require` - require the two netfilter NAT symbols
- `docs/architecture/testing/qemu-integration.md` - the IPsec row in the lab table
- `docs/guide/status.md` - the `./le qemu` action inventory
- the workflow file that gains the job

## Files to Create
- `internal/le/qemu/ipsec_nat_linux.go` - the three-namespace runner
- `test/ipsec/ipsec-transport-nat-selectors.ci` - the functional test above

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new config; the protocol behavior already exists |
| CLI commands/flags | N-A | No new command |
| Functional test for new RPC/API | Yes | `test/ipsec/ipsec-transport-nat-selectors.ci` |
| Doctor check for runtime dependencies | Yes | the netfilter NAT kernel symbols are a runtime dependency of the LAB, and `runtime.require` is the mechanism `ai/rules/platform-linux.md` names |
| Prometheus counters/metrics | No | None added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The feature shipped with the substitution spec |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/qemu-integration.md`, `docs/functional-tests.md` |
| 15 | Registered inventory changed? | Yes | `docs/guide/status.md`, the `./le qemu` action inventory |

## Implementation Steps

1. **Phase: the `.ci`** -- the cheapest of the three, and it needs no VM
   - Files: `test/ipsec/ipsec-transport-nat-selectors.ci`
   - Verify: it goes RED against a tree with the substitution reverted
2. **Phase: the kernel floor**
   - Files: `gokrazy/kernel/runtime.require`
   - Verify: a demotion of either symbol to `=m` fails the build
3. **Phase: the runner**
   - Files: `internal/le/qemu/ipsec_nat_linux.go`, `actions.go`, `alltests.go`
   - Verify: AC-1, with the tunnel control green in the same run
4. **Phase: the caller and the pages**
   - Files: the workflow job, `qemu-integration.md`, `status.md`
   - Verify: AC-2

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1, AC-2 and AC-3 each have a run whose output is pasted |
| Correctness | The namespace rules and the container rules translate the same way, so the two proofs cannot disagree |
| Rule: `ai/rules/platform-linux.md` | The action has a real caller in the same change |
| Rule: `ai/rules/interop-and-goal-validation.md` | The RED was forced with the artifact rebuilt, not with a source edit the VM never saw |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The QEMU action | `./le qemu` lists `ipsec-nat-transport-test`, and a workflow job names it |
| The kernel floor | a demotion fails the build |
| The functional test | `./le functional ipsec` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Privilege | The runner installs namespace and netfilter state; it must do so inside the VM and never on the host |
| Error leakage | The runner's failures name addresses already on the lab wire |

### Failure Routing
| Failure | Route To |
|---------|----------|
| The runner cannot masquerade | Read `runtime.config` for the symbol; a missing one is AC-2's own subject |
| The scenario stays RED after the fix | Compare the namespace rules with `natSetupScript` before touching the engine |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Docker lab's one-segment shape was forced by `interoplab.ScenarioPlan` carrying a single `NetworkSpec`. Namespaces carry no such constraint, so the runner CAN use two segments. Whether it should is the design question: parity with the container topology is worth more than fidelity to the RFC's figure, because a disagreement between the two proofs is what this spec exists to prevent.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| [to be written at design time] | | |

## Known Limitations

- IPv6 transport mode behind a NAT stays out of scope: the IKE transport is `udp4`.

**One owner question is inherited and is NOT this spec's to answer.** RFC 7296
Section 2.23.1 lets a responder that finds no transport-mode policy for the substituted
selectors undo the substitution and repeat the lookup for a tunnel-mode entry: "If an
entry is found but it does not allow transport mode, then the server MAY undo the address
substitution and redo the SPD lookup using the original Traffic Selectors." Ze does not,
and answers TS_UNACCEPTABLE, which is conformant because the clause is a MAY. It was
raised as OQ-1 of `spec-ipsec-transport-nat-selector-substitution` and never answered, so
it is recorded here rather than lost when that spec closed. The three answers are:
implement the fallback, keep the refusal, or put it behind a config leaf.
`ai/rules/rfc-compliance.md` reserves that choice for Thomas. `OriginalTSiAddr` and
`OriginalTSrAddr` (`internal/component/ike/engine/sa.go`) already hold what the fallback
would need.

## RFC Documentation (Scope: protocol)

No new enforcing code is expected. The RFC 7296 Section 2.23.1 quotes already sit above
`substituteResponderSelectors` and `substituteInitiatorSelectors`
(`internal/component/ike/engine/ts_nat_substitute.go`). This spec adds proof, not
enforcement, so any new `// RFC` comment here would be a second declaration of a
requirement the engine already carries.

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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-3 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated, not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec session review record`
- [ ] **Commit A:** code + tests + docs + spec
- [ ] **Commit B:** remove the spec
