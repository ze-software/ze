# Spec: vpp-slaac-tap-premise

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | none. `spec-fixit-vpp-slaac-no-dataplane-path` landed the gate and closed on 2026-08-29 |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Measure whether a VPP-owned NIC delivers IPv6 Router Advertisements to the Linux
Control Plane tap, and record the answer.

The closed spec `spec-fixit-vpp-slaac-no-dataplane-path` gated the `autoconf`
and `accept-ra` leaves to the netlink backend on 2026-08-24. The annotation it
wrote is in `internal/component/iface/yang/ze-iface-conf.yang`, and the five
`.ci` fixtures that pin the refusal are in `test/parse/`. It landed that gate
WITHOUT this measurement, on an owner ruling, because the measurement needs a
QEMU harness that does not exist and the spec was stalling behind it.

**This spec's outcome can REVERSE that gate.** If an address appears on the tap,
the two leaves work on the vpp backend, the `ze:backend "netlink"` annotation
comes off both of them, and the five `.ci` fixtures that pin the refusal are
rewritten to pin acceptance. That is a legitimate outcome, not a failure of this
spec.

**The evidence available today is suggestive, not conclusive.** The vendored
Linux Control Plane subset (`third_party/vpp-linux-cp/src/lcp_interface.c`)
programs a cross-connect input feature on the host interface, IPv4 and IPv6 punt
redirect on the physical interface, and the ARP nodes. It touches no multicast
arc, and Router Advertisements arrive on the all-nodes multicast group `ff02::1`.
That reads like no delivery. It settles nothing, because the vendored subset is
management-only: it is the part of the Linux Control Plane that Ze vendored, not
the whole of what a running VPP does with a punt path. Only a run answers this.

The measurement: boot an image whose VPP owns a NIC, put an advertisement sender
on the link, enable `autoconf` on the unit, and record whether the kernel learns
an address on the tap. Record the negative result as carefully as the positive
one. A negative result is the evidence the landed gate currently lacks.

**Blocking prerequisite: `test/qemu/` <!-- doc-links: ignore (this spec's blocking prerequisite is that the tree does not exist) --> does not exist.** The scenario has no home
yet, and choosing one is the first design question. The repository does carry a
QEMU harness family: driver scripts under `internal/le/` (for example
`effective-vpp-hugepages-qemu.py`, which builds a host `ze`, initializes an
appliance, builds the gokrazy image, boots it under QEMU and asserts through the
Ze CLI over SSH), driven from `.ci` files in `test/appliance/` and
`test/install/`, with `internal/le/qemu/netns_linux.go` for namespace isolation.
R-1 of the parent spec says to reuse that harness rather than build a new one.
Whether this scenario lands beside it or in a new `test/qemu/` <!-- doc-links: ignore (this spec's blocking prerequisite is that the tree does not exist) --> tree is for the
design phase to answer.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/testing/qemu-integration.md` - how a QEMU test is built,
  named, registered, and how it degrades when QEMU or KVM is absent
  → Constraint: [what the doc requires of a new QEMU scenario]
- [ ] `ai/digests/vpp-dataplane.md` - which surface the VPP backend owns, and how
  the Linux Control Plane tap is created
  → Constraint: [how a NIC reaches VPP ownership in a booted image]
- [ ] `docs/research/vpp-deployment-reference.md` - Linux Control Plane pairing
  → Decision: the tap shadows a VPP interface so kernel networking can bind on a
  VPP-owned NIC

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4862.md` - what a host must receive before it configures an
  address without a stateful server
  → Constraint: [the receive obligation the kernel cannot meet without the packets]
- [ ] RFC 4861 has no summary under `rfc/short/` yet. The advertisement the
  sender puts on `ff02::1` is defined there, and the research phase decides
  whether the measurement needs one written
  → Constraint: [the multicast group and hop limit the measurement depends on]

**Key insights:** (minimal context to resume after compaction)
- The answer is one bit: does an address appear on the tap. Everything else in
  this spec exists to produce that bit reliably and to record it.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `third_party/vpp-linux-cp/src/lcp_interface.c` - which arcs and punt paths
  the vendored subset programs
- [ ] `internal/plugins/iface/vpp/ifacevpp.go` - what the VPP backend sends when
  it takes a NIC, and what it sends for a VLAN sub-interface
- [ ] `internal/component/vpp/startupconf.go` - `GenerateStartupConf` writes
  `lcp-auto-subint`
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - the
  `ze:backend "netlink"` annotation this spec can remove
- [ ] `internal/le/qemu/hugepages.go` - the closest existing
  QEMU driver: host build, appliance init, image build, boot, CLI over SSH
- [ ] `test/appliance/vpp-hugepages-qemu.ci` - how such a driver is wrapped as a
  functional test, including the explicit timeout and the PASS/SKIP contract

**Behavior to preserve:**
- Every other verdict of the backend gate. This spec touches at most two leaves.
- The QEMU harness's SKIP contract: a host without QEMU, KVM, or the toolchain
  must skip with a stated reason and exit 0.

**Behavior to change:**
- Nothing, unless the measurement says the tap DOES receive the advertisements.
  In that case: remove `ze:backend "netlink"` from `autoconf` and `accept-ra` in
  `internal/component/iface/yang/ze-iface-conf.yang`, and rewrite the five `.ci`
  fixtures the parent spec created.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- An IPv6 Router Advertisement on `ff02::1`, put on the link by a sender that is
  not Ze, arriving at a NIC that VPP owns.

### Transformation Path
1. The NIC hands the frame to VPP.
2. VPP's graph decides whether the frame reaches the Linux Control Plane tap.
   **This step is the whole question.**
3. If it reaches the tap, the kernel processes it under
   `net.ipv6.conf.<tap>.accept_ra` and `net.ipv6.conf.<tap>.autoconf`, which
   `applySysctl` (`internal/component/iface/config_sysctl.go`) already emits.
4. An address appears on the tap, or it does not.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Link ↔ VPP | the advertisement sender is a separate endpoint on the same link | No |
| VPP ↔ kernel tap | Linux Control Plane pair, punt paths programmed by the plugin | No. This is the boundary the spec measures |
| iface component ↔ sysctl plugin | EventBus key/value emission | Read by the parent spec. Unchanged |

### Integration Points
- `ValidateBackendFeatures` (`internal/component/config/backend_gate.go`) - the
  gate whose verdict this measurement can reverse.
- The QEMU driver family under `internal/le/` - where the run lives.

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
| A-1 | A QEMU guest can be given a NIC that VPP takes over, with a second endpoint on the same link to send advertisements | the existing appliance QEMU drivers boot a gokrazy image with VPP present, and `internal/le/qemu/netns_linux.go` isolates namespaces | the measurement needs different infrastructure, and the spec must say what | build the topology and confirm VPP owns the NIC and the tap exists | unvalidated |
| A-2 | A negative result is trustworthy: no address appeared because the packets never arrived, not because the sender was silent or the sysctl never landed | the run can assert both independently | a negative result proves nothing, and the landed gate stays unevidenced | assert the sender transmitted and the two sysctl keys read back on the tap, before reading the address | unvalidated |
| A-3 | The tap's kernel behavior under `accept-ra` is the same as any other device's | RFC 4861 and the kernel treat the tap as an ordinary interface | the measurement answers a question about taps rather than about VPP | run the same assertions on the netlink backend as a control | unvalidated |
| A-4 | The result generalizes from the parent NIC to a VLAN sub-interface | `lcp-auto-subint` creates the sub-interface tap, and the sub-interface is a `unit` entry reaching the same leaf | the two disagree and each needs its own verdict | run the scenario a second time on a VLAN sub-interface | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The run is silent for the wrong reason, and a false negative confirms the gate that is already landed | no advertisement counter moves anywhere in the topology | A-2's independent assertions: the sender transmitted, the sysctl keys read back, and only then the address is read |
| R-2 | The scenario needs KVM and skips on every host that runs it, so the answer is never produced | the `.ci` reports SKIP on the developer machine and in CI | state where the run must happen, and treat a SKIP-only result as no answer rather than as a pass |
| R-3 | The measurement takes long enough that the spec stalls the way its parent did | the harness question is still open after the design phase | the parent spec's gate is already landed and needs nothing from this spec, so a stall here costs no user-visible behavior |
| R-4 | The result reverses the gate, and the reversal is forgotten | this spec closes with a positive result and no follow-up edit | the reversal is AC-5 of this spec, not a note |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A false negative leaves a gate refusing a configuration that works, so an operator loses IPv6 autoconfiguration on the vpp backend for no reason. A false positive removes a gate and restores the silent accept the parent spec deleted |
| How is it reverted? | Single commit revert. The gate is one YANG annotation and five `.ci` fixtures |
| Who else touches this path? | the closed spec `spec-fixit-vpp-slaac-no-dataplane-path` landed the gate; `plan/immediate/spec-fixit-vpp-vlan-promiscuous.md` works the same VLAN surface; `plan/spec-dataplane-seams-4-control-packet-rx.md` owns the shared receive-path question |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| an advertisement on `ff02::1` at a VPP-owned NIC | → | the VPP graph and the Linux Control Plane punt path | `vpp-slaac-tap.ci` |
| `ipv6 autoconf true` on the unit in the booted image | → | `applySysctl` writes the two keys on the tap | `vpp-slaac-tap.ci`, sysctl read-back assertion |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A run with `autoconf true` on a VPP-owned NIC and an advertisement sender on the link | The run records whether an address appears on the tap, and that record is the evidence the parent spec's gate lacks |
| AC-2 | The same run, before the address is read | The run asserts independently that the sender transmitted and that both sysctl keys read back on the tap, so a negative result is not a false negative |
| AC-3 | The same configuration under `backend netlink` in the same topology | An address appears. This is the control: without it a negative result says nothing about VPP |
| AC-4 | The same configuration on a VLAN sub-interface of the VPP-owned NIC | The run records the sub-interface verdict beside the parent's |
| AC-5 | The run shows an address DOES appear on the tap | `ze:backend "netlink"` is removed from both leaves, the five `.ci` fixtures the parent spec created are rewritten to pin acceptance, and `docs/features/interfaces.md` loses the section that says otherwise |
| AC-6 | The run shows no address appears | The gate stays, and this spec's recorded output is cited from the parent spec's Known Limitations, which currently says the premise is unmeasured |
| AC-7 | A host with no QEMU, no KVM, or no toolchain | The run skips with a stated reason and exits 0, matching the existing QEMU drivers. A SKIP is not an answer to AC-1 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | enables IPv6 autoconfiguration on a VPP-owned NIC and expects an address | link → VPP graph → Linux Control Plane tap → kernel SLAAC | `vpp-slaac-tap.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBackendGateIPv6Autoconf` | `internal/component/config/backend_gate_test.go` | the existing gate verdicts, which AC-5 inverts if the run is positive | exists, from the parent spec |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `accept-ra` | 0-2 | 2 | N/A | 3 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-slaac-tap.ci` | the design phase chooses between a new `test/qemu/` <!-- doc-links: ignore (this spec's blocking prerequisite is that the tree does not exist) --> tree and the existing `test/appliance/` route, which is where `vpp-hugepages-qemu.ci` sits | the operator learns whether autoconfiguration can work on a VPP-owned NIC | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `vpp-slaac-tap` | decided by the design phase; `test/qemu/` <!-- doc-links: ignore (this spec's blocking prerequisite is that the tree does not exist) --> does not exist today | an advertisement sender on the link, running on a separate endpoint | whether the VPP dataplane delivers Router Advertisements to the Linux Control Plane tap | |

The scenario is named, never numbered (`ai/rules/interop-and-goal-validation.md`,
and the de-numbering that landed in `9aaa03e3d`).

## Files to Modify
- `internal/component/iface/yang/ze-iface-conf.yang` - only under AC-5, to remove
  the annotation on `autoconf` and `accept-ra`
- `test/parse/iface-vpp-rejects-ipv6-autoconf.ci`,
  `test/parse/iface-vpp-rejects-ipv6-accept-ra.ci`,
  `test/parse/iface-vpp-rejects-ipv6-autoconf-vlan.ci` - only under AC-5
- `docs/features/interfaces.md` - the section and the two feature rows the parent
  spec wrote, only under AC-5

## Files to Create
- the `vpp-slaac-tap` driver, beside the existing QEMU drivers under
  `internal/le/` unless the design phase places it elsewhere
- the `.ci` that wraps it, with an explicit timeout and a PASS/SKIP contract

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | | only under AC-5: the annotation comes off two existing leaves |
| YANG validation constraints | No | the leaves already carry their types and ranges |
| YANG custom validators | No | the backend gate is the validator |
| CLI commands/flags | No | no new command |
| CLI grammar (keyword before value) | N-A | no new grammar |
| Editor autocomplete | No | no new leaf |
| Functional test for new RPC/API | No | no new RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | No | no environment leaf |
| Doctor check for runtime dependencies | | decide during design: under AC-6 the gate refuses at commit and no running state needs reporting; under AC-5 a check could report a tap that receives nothing |
| Prometheus counters/metrics | No | no new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | | under AC-5 two leaves gain a backend |
| 2 | Config syntax changed? | No | the syntax is unchanged either way |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | | decide during design |
| 6 | Has a user guide page? | Yes | `docs/features/interfaces.md`, the section "Receiving advertisements needs the netlink backend" |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | | RFC 4862 host behavior is newly PROVEN or newly disproven on the vpp backend; decide during design whether a `rfc/short/` row moves |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and `docs/architecture/testing/qemu-integration.md`, for the new scenario and wherever it lands |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | | `ai/digests/vpp-dataplane.md`, which today says nothing about SLAAC on a tap |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `internal/component/iface/yang/ze-iface-conf.yang` is anchored from `docs/features/interfaces.md`, named above. Re-derive with `./le spec citation anchors spec plan/spec-vpp-slaac-tap-premise.md` once the file list is final |
| 17 | Existing docs show config/CLI/API examples for this area? | | re-check every `autoconf` and `accept-ra` mention in `docs/` against whichever verdict this spec produces |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- decide where the scenario lives and
   prove the topology exists
   - Tests: `vpp-slaac-tap`, failing because it asserts nothing yet
   - Files: the driver and its `.ci`
   - Verify: the guest boots, VPP owns the NIC, the Linux Control Plane tap
     exists, and a second endpoint is on the same link. The test fails on the
     address assertion, not on the topology
2. **Phase: Trustworthy negative** -- make a silent result mean something
   - Tests: the AC-2 assertions and the AC-3 control
   - Files: the driver
   - Verify: the sender transmitted, both sysctl keys read back on the tap, and
     the netlink control learns an address in the same topology
3. **Phase: Record the answer** -- and act on it
   - Tests: `vpp-slaac-tap` on the parent NIC and on a VLAN sub-interface
   - Files: under AC-5, the YANG annotation, three `.ci` fixtures and
     `docs/features/interfaces.md`; under AC-6, nothing but the record
   - Verify: the recorded output is quoted in this spec, and the parent spec's
     Known Limitations entry is answered

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol, and AC-5 and AC-6 are both written before the run happens |
| Correctness | A negative result is backed by the AC-2 assertions and the AC-3 control, so it is evidence rather than silence |
| Naming | The scenario is `vpp-slaac-tap`, with no numeric prefix, everywhere it is cited |
| Data flow | The measurement reads the tap's address, not a VPP counter that could move for another reason |
| Rule: `ai/rules/evidence.md` | The verdict comes from a run. Reading `lcp_interface.c` is what made this spec necessary, and it is not an answer to it |
| Rule: `ai/rules/interop-and-goal-validation.md` | The test discriminates: it fails when the delivery path is broken, and it is not a test that would pass with the mechanism removed |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The premise answer | the run's output quoted in this spec, with the sender and sysctl assertions beside it |
| A reversal, or a record | under AC-5 the annotation is gone and the fixtures pin acceptance; under AC-6 the parent spec's Known Limitations entry cites this spec's output |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | `accept-ra` stays bounded to 0..2 whichever verdict lands |
| Fail closed | If the run cannot tell whether delivery happened, the gate stays. An unmeasured path must not lose its restriction |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| The run only ever SKIPs | Not an answer. Say where it must run, and keep the spec open |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Measure after the gate landed, not before | hold the gate until the measurement existed | Owner ruling, 2026-08-24. The parent spec's R-1 predicted the stall, and the gate removes a silent accept today on the reasoning that already governs three sibling containers |
| The spec may reverse its parent | treat the gate as settled | An annotation that refuses a working configuration is its own defect. The reversal is AC-5, written before the run, so a positive result cannot be quietly filed |

## Known Limitations
- The spec measures one dataplane on one topology. A different VPP version, a
  different driver, or a different Linux Control Plane build can answer
  differently, and the record must name what it ran.

## RFC Documentation (Scope: protocol)

RFC 4862 governs the host that builds an address from a Router Advertisement, and
RFC 4861 governs the advertisement itself. Ze implements neither in this spec: it
measures whether the packets a conforming host needs reach the interface the
kernel owns. Under AC-5, the code that arranges delivery carries a comment naming
the RFC 4862 requirement the host cannot meet without the packets.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
