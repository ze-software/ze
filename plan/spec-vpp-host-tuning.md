# Spec: vpp-host-tuning

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-vpp-isolated-cpus |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-isolated-cpus.md` - the prerequisite (isolcpus sourcing + CPU validation, `ready`)
4. `internal/component/vpp/startupconf.go` - startup.conf generation
5. `internal/component/vpp/config.go` - VPP settings model

## Task

**Skeleton created from the osvbng comparison refresh (2026-07-10). Full design not started.**

Ze generates VPP's startup.conf (cores, hugepage page-size, buffers-per-numa,
rx-queues) but leaves the HOST side of a performant VPP deployment to the
operator, and omits the idle-behaviour knob entirely. `plan/spec-vpp-isolated-cpus.md`
(ready) already owns isolated-core sourcing, CPU validation, and requesting
isolcpus on the appliance boot cmdline. This spec covers what remains on top of
it:

1. **`poll-sleep-usec` exposure.** VPP's idle-worker sleep is not configurable
   from Ze (verified: no `poll-sleep` match under `internal/component/vpp/`).
   Default VPP behaviour busy-polls workers at 100% CPU; a sleep value trades
   latency for idle CPU. Expose it in the YANG cpu/dataplane settings with a
   documented recommendation (0 or unset for production latency, non-zero for
   dev/shared hosts). Reference: osvbng 5fc360e documents exactly this trade-off.
2. **Hugepage reservation at boot.** startup.conf consumes a hugepage page-size,
   but nothing reserves hugepages on the host. On the gokrazy appliance Ze owns
   the kernel cmdline, so it can emit `default_hugepagesz`/`hugepagesz`/
   `hugepages` (and per-NUMA-node reservation where topology calls for it)
   alongside the isolcpus request that spec-vpp-isolated-cpus already plans.
3. **NUMA/SMT awareness (design to scope).** Candidates: derive or validate
   worker placement against the NIC's NUMA node; avoid splitting a physical core
   between VPP and the host (SMT sibling awareness); disable automatic NUMA
   balancing where it fights pinning. Reference: osvbng 88064b7 (KVM deploy
   rewrite) does per-node hugepages, SMT-aware pinning, and NUMA-node-derived
   core selection on the HOST; Ze's equivalent surface is the appliance image
   and doctor checks, not a deploy script.

Ze-shape note: osvbng tunes a KVM host that runs the VM. Ze's unit of deployment
is the gokrazy appliance image itself (see memory: Ze owns full process
lifecycle, no systemd), so items land in the image build/boot config, the VPP
component's startup.conf generation, validation, and `ze doctor`, not in a
shell script.

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - startup.conf syntax + production values.
  → Constraint: keep generated startup.conf valid for the pinned VPP version.
- [ ] `plan/spec-vpp-isolated-cpus.md` - prerequisite scope boundary.
  → Constraint: isolcpus request + CPU validation belong THERE; this spec must not duplicate them.
- [ ] `ai/rules/config-surface.md` - YANG vs env var for the new knobs.
- [ ] `ai/rules/qemu-testing.md` - boot-cmdline behaviour needs QEMU evidence.

**Key insights:**
- The dependency ordering matters: isolated-CPU sourcing (prerequisite spec)
  decides WHICH cores VPP gets; this spec decides how the host is prepared
  around that choice (hugepages, idle behaviour, NUMA fit).

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-07-10 at survey depth; re-read fully at design time)
- [ ] `internal/component/vpp/startupconf.go` - emits `main-core`, `corelist-workers`, `page-size` from hugepage-size, `buffers-per-numa`, per-interface `num-rx-queues`. No `poll-sleep-usec` (grep verified 2026-07-10: zero matches in `internal/component/vpp/`).
- [ ] `internal/component/vpp/config.go` - `CPUSettings` (main-core/workers), `MemorySettings` (hugepage-size/buffers); no idle/poll knob, no host hugepage reservation.
- [ ] `internal/appliance/` image build - confirm where kernel cmdline is assembled (gokrazy) and whether any hugepage parameters are set today (expected: none; validate A-1).
- [ ] `internal/core/sysctl/profiles.go` - whether a performance profile exists that should carry `kernel.numa_balancing` (validate at design).

**Behavior to preserve:**
- Generated startup.conf remains valid; absent new config, output is byte-identical to today.
- spec-vpp-isolated-cpus semantics untouched.

**Behavior to change:**
- New knobs (poll-sleep-usec; hugepage reservation; NUMA checks per design outcome), all additive.

## Data Flow (MANDATORY)

### Entry Point
- Config: new leaves in the VPP YANG (idle/poll behaviour, hugepage reservation) and/or appliance image settings.
- Host facts: NUMA topology, SMT siblings, NIC `numa_node` (read paths decided at design).

### Transformation Path
1. New leaves parsed into VPP settings / appliance image config.
2. startup.conf generation emits `poll-sleep-usec` when configured.
3. Image build emits hugepage (and existing isolcpus) kernel cmdline parameters.
4. Doctor/verify checks compare requested resources against host topology.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ VPP settings | YANG leaves → settings structs | [ ] |
| Settings ↔ startup.conf | poll-sleep-usec emission | [ ] |
| Appliance config ↔ kernel cmdline | hugepage parameters at image build | [ ] |
| Doctor ↔ host topology | NUMA/SMT/hugepage sanity checks | [ ] |

### Integration Points
- `internal/component/vpp/startupconf.go` - new emission.
- `internal/component/vpp/config.go` + YANG - new leaves.
- `internal/appliance/` image build - cmdline parameters (coordinate with spec-vpp-isolated-cpus Phase "Boot isolation").
- `ze doctor` - host-topology checks (`ai/rules/doctor-checks.md`).

### Architectural Verification
- [ ] No bypassed layers (config through parse/validate; no side-channel host writes)
- [ ] No unintended coupling (topology reads behind testable helpers)
- [ ] No duplicated functionality (isolcpus stays in the prerequisite spec)
- [ ] Registration over hardcoding - doctor checks register in the owning package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No hugepage reservation exists anywhere today | grep `nr_hugepages`/`hugepagesz` over `internal/` and `scripts/` (2026-07-10 survey) found nothing | scope shrinks | re-grep + read appliance cmdline assembly | unvalidated |
| A-2 | gokrazy image build lets Ze append arbitrary kernel cmdline parameters | gokrazy owns cmdline; spec-vpp-isolated-cpus A-2 makes the same bet | hugepages via sysctl `vm.nr_hugepages` fallback (no 1G pages then) | read `internal/appliance/` cmd_build cmdline path | unvalidated |
| A-3 | The pinned VPP version accepts `poll-sleep-usec` in the cpu section | osvbng doc + VPP upstream docs | emit under the correct section per version | check VPP version docs during design | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Non-zero poll-sleep default silently costs latency | throughput/latency regression in ze-perf | default unset (VPP default); document the trade-off; no silent default |
| R-2 | Hugepage reservation starves general memory on small appliances | OOM at boot in QEMU test | validate reservation against total memory at verify |
| R-3 | NUMA scope creep (this spec grows a deploy-tool) | design keeps expanding | keep host prep = image build + doctor checks; split anything else |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| poll-sleep configured in VPP settings | → | startup.conf contains `poll-sleep-usec` | `test/plugin/vpp-poll-sleep.ci` |
| hugepage reservation configured | → | image kernel cmdline carries hugepage parameters | QEMU appliance test (name at design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | poll-sleep leaf set | `poll-sleep-usec` emitted in startup.conf with the value |
| AC-2 | poll-sleep leaf unset | startup.conf byte-identical to today (VPP default behaviour) |
| AC-3 | hugepage reservation configured | boot cmdline reserves the pages; VPP starts with them available (QEMU evidence) |
| AC-4 | reservation exceeds appliance memory | config verify rejects |
| AC-5 | NUMA checks (per design scope) | mismatched NIC/worker NUMA placement surfaces as doctor warning |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | sets poll-sleep for a shared dev host | config → startupconf → idle workers sleep | `test/plugin/vpp-poll-sleep.ci` |
| 2 | builds an appliance image with hugepages + isolated cores | image build → cmdline → boot → VPP consumes pages | QEMU appliance test (name at design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStartupConfPollSleep` | `internal/component/vpp/startupconf_test.go` | emission + absence when unset | |
| `TestHugepageReservationValidate` | `internal/component/vpp/config_test.go` | reservation vs memory bounds | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| poll-sleep-usec | design (candidate 0-100000) | design | N/A | design |
| hugepage count | 1-(memory/page-size) | design | 0 | over-memory |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-poll-sleep` | `test/plugin/vpp-poll-sleep.ci` | operator tunes idle behaviour | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - dataplane/appliance config, no wire protocol | - | - | QEMU appliance test covers boot behaviour | - |

### Future (if deferring any tests)
- None planned (skeleton; refine at design).

## Files to Modify
- `internal/component/vpp/startupconf.go` - poll-sleep-usec emission
- `internal/component/vpp/config.go` + `internal/component/vpp/yang/ze-vpp-conf.yang` - new leaves
- `internal/appliance/` image build - hugepage kernel cmdline (coordinate with spec-vpp-isolated-cpus)

## Files to Create
- `test/plugin/vpp-poll-sleep.ci` - functional test
- doctor check file(s) in the owning package (names at design)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first; implement AFTER spec-vpp-isolated-cpus) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - run the `/ze-spec` workflow: confirm A-1..A-3, scope the NUMA/SMT item (may split out), then fill ACs/tests above. Coordinate the boot-cmdline work with spec-vpp-isolated-cpus so both specs touch the appliance cmdline assembly once, not twice.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.
- isolcpus sourcing/validation is owned by `plan/spec-vpp-isolated-cpus.md`, not here.

## Implementation Summary
### What Was Implemented
- Nothing yet (skeleton).

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] Full `/ze-spec` DESIGN completed and approved before implementation
- [ ] `make ze-test` passes (after implementation)
- [ ] Feature code integrated (`internal/*`)
- [ ] QEMU evidence for boot-cmdline behaviour

### Quality Gates (SHOULD pass)
- [ ] `docs/research/vpp-deployment-reference.md` updated with the new knobs

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
