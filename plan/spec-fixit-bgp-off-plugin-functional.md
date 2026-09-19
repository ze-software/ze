# Spec: fixit-bgp-off-plugin-functional

| Field | Value |
|-------|-------|
| Status | design |
| Depends | - |
| Phase | - |
| Updated | 2026-07-22 |

## Task

Complete elective runtime coverage for FIB, MRT and flow export in a build
without `ze_bgp`. Preserve the existing compile-out and registration proofs;
add the remaining schema, daemon-health, graceful-degradation and kernel-FIB
readback evidence against a binary that contains the features being exercised.
This is coverage of a non-default composition, as the 2026-08-17 disposition
recorded. Its filename does not make it a first-release product defect.

The original task called all five plugins always-on and planned to exercise
all of them in bare `ze_core` or `ze-stripped`. That composition is obsolete.
`feature-gates.txt` now gates MRT with `ze_mrt`, flow export with
`ze_flowexport`, and the VPP FIB with `ze_vpp`. The generated
`internal/component/plugin/all/all_ze_mrt.go`, `all_ze_flowexport.go` and
`all_ze_vpp.go` own those imports. Kernel and P4 FIB remain in `all.go`.
A positive test cannot require a deliberately compiled-out plugin to exist.

The test composition must omit `ze_bgp` and explicitly include the independent
feature tags needed by each retained AC. Native CLI-driven cases also need the
SSH surface they drive. Determine how that composition reaches the existing
functional and QEMU runners before implementation; do not silently substitute
the full BGP-enabled binary or weaken the MRT/flow-export assertions.

### Provenance and progress

Commit `c4038def0` requalified the FIB, MRT and flow-export dependencies onto
core-leaf packages and enabled BGP compile-out. The August disposition said
half the original work had landed. Current source establishes the following
pieces, without claiming a new passing run:

| Existing carrier | What it covers | What remains |
|------------------|----------------|--------------|
| `cmd/ze/hub/build_tag_bgp_absent_test.go` | BGP registration, reactor factory and decoder absence; rejection of BGP config | Positive operation of the surviving features |
| `cmd/ze/hub/build_tag_protocols_absent_test.go` | Protocol symbol-drop proof | Runtime behaviour |
| `cmd/ze/hub/build_tag_gate11_absent_test.go`, `mrt-only` row | A `ze_core,ze_mrt` composition expects MRT symbols and no BGP engine | MRT boot and nil-source behaviour |
| `cmd/ze/hub/build_tag_mrt_present_test.go` and `build_tag_gate12_group_a_present_test.go` | Independent-feature registration tests | Functional evidence with BGP absent |
| `test/ui/ze-stripped-no-bgp.ci`, driven by `runZEStrippedNoBGP` in `internal/test/fixture/ui_fixture_ze_stripped_no_bgp.go` | Validates a kernel-FIB config with the BGP-absent binary and requires BGP config rejection | MRT/flow-export are explicitly excluded by this fixture; it performs no kernel route-install readback |
| `test/ui/ze-stripped-surface.ci` | Existing stripped-binary command-surface fixture | It is a runner precedent, not evidence that all five ACs below passed |

The landed functional half is the kernel-FIB schema/BGP-rejection pair.
Its `fib-static.conf` currently contains only `fib { kernel { } }`, so the
fixture's static-route wording must not be counted as a static-route install
proof. Keep that existing carrier and complete the remaining feature schema,
boot/degradation and Linux route-readback obligations. The proposed
`TestBuildTag_AlwaysOnPluginsPresent` and three `test/plugin/bgp-off-*.ci`
files were not found in the current tree; proposed names are not delivery
evidence.

**Scope:** test and verification coverage. A reproduced product defect that
blocks an AC must be fixed at its producer, with evidence recorded; an absent
proof alone does not establish one. Existing full-binary coverage remains.

## Required Reading

- [ ] `docs/architecture/plugin/feature-gates.md`: manifest-owned composition
      and present/absent proof.
- [ ] `docs/architecture/testing/runner-architecture.md`: binary selection.
- [ ] `docs/architecture/testing/ci-format.md`: functional assertions and
      Linux capability requirements.
- [ ] `ai/rules/plugins.md`, `ai/rules/testing.md` and
      `ai/rules/interop-and-goal-validation.md`.

## Current Behavior (MANDATORY)

- [ ] `feature-gates.txt` and `internal/component/plugin/all/`: read the
      composition that makes each plugin present.
- [ ] `internal/plugins/fib/kernel/fibkernel.go`: the protocol-neutral sysrib
      consumer and kernel route install path.
- [ ] `internal/plugins/mrt/component.go` and `dump.go`: establish the current
      no-BGP-source behaviour before writing the graceful-degradation fixture.
- [ ] `internal/plugins/flowexport/enrichbgp.go`: enrichment consumes BGP
      best-change events; inspect raw export separately if the fixture uses it.
- [ ] `internal/le/functional/` and `internal/le/integration/`: determine the
      existing route for a BGP-absent binary with the required feature tags.

Preserve compile-out, existing full-binary coverage and graceful source absence.
MRT data production is not required without a BGP source. Flow export must run
without BGP AS enrichment. FIB must install a static route via the Loc-RIB.

## Data Flow (MANDATORY)

### Entry Point

Config and commands on an explicitly BGP-absent test binary, with each tested
feature compiled in.

### Transformation Path

1. Parse the configured plugin roots and start the enabled plugins.
2. Static routes enter the protocol-neutral Loc-RIB; the kernel FIB consumes
   its best-change events and installs the route.
3. MRT and flow export encounter the absent BGP source and retain their
   documented graceful-degradation behaviour.
4. The fixture observes daemon health and the actual kernel route.

### Boundaries Crossed

| Boundary | Proof |
|----------|-------|
| Build composition -> plugin and schema registration | AC-1, AC-2 |
| Config -> running daemon | AC-3 |
| Loc-RIB -> Linux kernel FIB | AC-4 |
| Absent BGP source -> MRT/flow export | AC-5 |

### Integration Points

Reuse the existing build-tag matrix and functional runners. Design must name
the exact non-default binary and its runner exposure before adding fixtures.

### Architectural Verification

No import may reintroduce BGP into the BGP-absent composition. No new test-file
build selector is presumed necessary; first use the runners' existing binary
selection mechanism.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validation | Status |
|----|------------|-------|----------|------------|--------|
| A-1 | The selected BGP-absent composition can contain all retained test subjects | Independent feature gates and generated imports | Correct the test composition before claiming a product failure | Read the build producer and inspect its composed registry | unvalidated |
| A-2 | The existing runner can expose that binary natively and in QEMU | It already supports alternate binaries | Design scoped runner wiring without substituting the full binary | Trace launch and PATH setup | unvalidated |
| A-3 | Static routes install through kernel FIB without BGP | The task's original protocol-neutral sysrib design | A product coupling defect blocks AC-4 | Linux kernel readback | unvalidated |
| A-4 | MRT and flow export boot and handle the missing BGP source gracefully | Existing producer seams | A reproduced runtime defect blocks AC-3/AC-5 | Functional boot and command assertions | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | The test uses a full BGP-enabled binary | It accepts BGP config | Pair positive feature assertions with BGP rejection and record build identity |
| R-2 | A fixture asks for a feature its binary deliberately omits | Unknown config root before runtime | Pin the required independent tags in the test composition |
| R-3 | A healthy boot is mistaken for route-install proof | No kernel readback | AC-4 requires the installed prefix |

## Wiring Test (MANDATORY)

| Entry Point | Feature Code | Required carrier |
|-------------|--------------|------------------|
| BGP-absent composition with the tested features enabled | Generated imports and plugin registry | Existing build-tag tests, extended only for a missing composition proof |
| Config validation and daemon boot | Plugin schemas and startup | Planned `bgp-off-schema-stripped.ci` and `bgp-off-boot-stripped.ci` |
| Static route on Linux | sysrib -> fib-kernel -> netlink | Planned `bgp-off-fib-install.ci` |

The historical fixture names above remain proposed names. They do not imply
that today's stock `ze-stripped` includes MRT, flow export or VPP.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A BGP-absent composition with MRT, flow export and the tested FIB backends explicitly enabled | `fib-kernel`, `fib-p4`, `fib-vpp`, `mrt` and `flow-export` are registered; BGP remains absent. Reuse existing registration proofs where they cover the same composition |
| AC-2 | Validate each FIB/MRT/flow-export config on that composition, then a BGP config | All enabled-subsystem configs validate; BGP config is rejected. No bare-core claim is made for gated features |
| AC-3 | Boot with kernel FIB, MRT and flow export configured | Daemon reaches readiness, health and warnings show no error for those plugins, and configuration/start is observable |
| AC-4 | Static route plus kernel FIB on the BGP-absent Linux binary | The exact prefix and next-hop appear in kernel readback; the case runs under the Linux/QEMU gate |
| AC-5 | Trigger MRT RIB dump with routes configured and let flow export run without BGP | MRT handles the missing source cleanly; flow export runs without BGP enrichment or an enrichment-subscription error; the fixture observes the relevant behaviour rather than only lack of a crash |

## End-to-End User Stories

A user of a non-default BGP-absent build can validate and run the independent
features they enabled, install a static route, and receive a clear rejection
for BGP config. MRT and flow export tolerate the missing BGP source.

## TDD Test Plan

Preserve the existing build-tag tests. Add only the missing composition and
runtime cases from AC-1 through AC-5 after mapping existing evidence. The
schema test must distinguish an inert validator, and the FIB case must fail
when the route-install producer is broken. Graceful-degradation assertions
must reach the absent-source path.

## Files to Modify

- Existing `cmd/ze/hub` build-tag tests where composition evidence is missing.
- Existing runner wiring only if it cannot expose the selected test binary.
- `docs/functional-tests.md` for the resulting non-default build test pattern.

## Files to Create

The remaining functional carriers are planned under `test/plugin/` with the
`bgp-off-` names above. Choose their exact binary invocation during design.

## Implementation Steps

1. Record the BGP-absent tag composition and map existing evidence to every AC.
2. Supply any missing registration/schema proof without duplicating current tests.
3. Add boot and source-absence assertions for AC-3/AC-5.
4. Add kernel readback for AC-4 and execute it under the Linux/QEMU gate.
5. Prove each behavioural assertion discriminates, complete review and run the worktree gate.

## Known Limitations

P4 and VPP FIB installation remain outside the original kernel-install AC;
their retained requirements are presence and schema reachability. This spec
does not claim a new owner-approved deferral or new dataplane coverage for them.
The kernel proof uses static routes; it does not become direct evidence of
IGP-sourced installation in this non-default build.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-5 demonstrated against the recorded BGP-absent composition.
- [ ] Existing evidence and newly executed evidence distinguished.
- [ ] AC-4 exercised under `./le qemu run command "./le qemu all-tests"`.
- [ ] Every assumption resolved; no weakened assertion hides a product defect.
- [ ] `./le verify worktree` passes.

### Closure
- [ ] Complete `plan/TEMPLATE-CLOSURE.md` and independent review.
- [ ] Preserve implementation and evidence in commit A before removing this spec in commit B.
