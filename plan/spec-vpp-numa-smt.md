# Spec: vpp-numa-smt

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | spec-vpp-isolated-cpus, spec-vpp-host-tuning (closed, learned 1105) |
| Phase | - |
| Updated | 2026-07-10 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `plan/spec-vpp-isolated-cpus.md` - owns worker-core selection (isolated-set sourcing)
4. `plan/spec-vpp-host-tuning.md` - owns global hugepage reservation + the kernel-args assembly seam
5. `internal/component/host/inventory.go` - `CoreInfo`/`NICInfo` (facts this spec must extend)

## Task

**Skeleton split out of `plan/spec-vpp-host-tuning.md` during its 2026-07-10
design session. Full design not started.**

NUMA/SMT awareness for VPP appliances: make worker placement and memory
reservation topology-aware, and surface topology mismatches through `ze doctor`.
Split from vpp-host-tuning because every candidate item binds to surfaces that
do not exist yet (verified 2026-07-10):

1. **Host facts.** The inventory has no NUMA or SMT data: `CoreInfo` carries
   only CPU/CoreID/PhysicalPackage (internal/component/host/inventory.go,
   SMT siblings derivable but not exposed), `readNICSysfs` does not read
   `device/numa_node` (internal/component/host/nic_linux.go), and nothing
   under `internal/` reads `/sys/devices/system/node` (grep `numa`, 2026-07-10).
   New facts needed: per-CPU NUMA node, NIC NUMA node, SMT sibling map.
2. **NIC/worker NUMA alignment.** Doctor check (moved AC-5 of vpp-host-tuning):
   warn when a DPDK NIC's NUMA node has no VPP worker on it, or when workers sit
   on a remote node. Depends on the worker-core selection helper that
   `spec-vpp-isolated-cpus` designs (`ready`, unimplemented) -- placement cannot
   be validated before it exists.
3. **SMT sibling awareness.** Avoid splitting a physical core between VPP and
   the host: warn (or refuse, per design) when a chosen worker core's SMT
   sibling is not also dedicated (isolated or another VPP core).
4. **Per-NUMA-node hugepage reservation.** vpp-host-tuning reserves globally
   (kernel splits `hugepages=N` evenly across nodes); multi-socket hosts may
   need per-node counts (`hugepages=<node>:<count>` cmdline syntax on recent
   kernels, or per-node sysfs at early boot). Extends the kernel-args assembly
   seam (`internal/appliance/kernelargs.go`) that vpp-host-tuning creates.
5. **Automatic NUMA balancing.** `kernel.numa_balancing` fights explicit
   pinning. The sysctl profile surface is interface-scoped
   (internal/core/sysctl/profiles.go, :91-100), so disabling a global key
   needs its own surface decision (global profile, boot cmdline `numa_balancing=disable`,
   or doctor-only warning).

Ze-shape note: all of it lands in host inventory, config validation, the
appliance image build, and `ze doctor` -- not in a deploy script. Reference
hardware today is single-socket (docs/research/vpp-deployment-reference.md),
so this spec is about correctness on bigger iron, not the common path.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-vpp-isolated-cpus.md` - worker-core selection helper this spec validates against.
- [ ] `plan/spec-vpp-host-tuning.md` (or its learned summary after closure) - kernel-args seam, hugepage doctor check to extend.
- [ ] `ai/rules/repo-maintenance.md` - ownership + diagnostic codes for the new checks.
- [ ] `ai/rules/platform-linux.md` - QEMU can emulate NUMA topologies (`-numa` options) for evidence.
- [ ] `docs/research/vpp-deployment-reference.md` - production placement guidance.

## Current Behavior (MANDATORY)

**Source files read:** (survey 2026-07-10; re-read fully at design time)
- [ ] `internal/component/host/inventory.go` - `CoreInfo` (:203-212), `NICInfo` (:215+): no NUMA fields.
- [ ] `internal/component/host/nic_linux.go` - `readNICSysfs` (:75-95): no `numa_node` read.
- [ ] `internal/component/host/cpu_linux.go` - cpuinfo/cpufreq/hybrid detection; extension point for topology facts.
- [ ] `internal/core/sysctl/profiles.go` - interface-scoped profiles only.
- [ ] `internal/component/vpp/startupconf.go` - `buffers-per-numa` already emitted (:55-59); placement inputs arrive via spec-vpp-isolated-cpus.

**Behavior to preserve:**
- Single-socket appliances see no new warnings and no behaviour change.
- vpp-host-tuning's global reservation remains the default; per-node is additive.

**Behavior to change:**
- New host facts, new doctor diagnostics, optional per-node reservation (all additive; exact scope at design).

## Data Flow (MANDATORY)

### Entry Point
- Host facts: `/sys/devices/system/node/`, per-CPU `topology/`, NIC `device/numa_node` (read paths decided at design).
- Config: worker-core selection (from spec-vpp-isolated-cpus) and per-node hugepage counts (appliance config, extends spec-vpp-host-tuning's `image.hugepages`).

### Transformation Path
1. Host inventory gains topology facts (CPU node, NIC node, SMT sibling map).
2. Validation/doctor compares VPP worker placement + DPDK NICs against those facts.
3. Image build emits per-node hugepage arguments through the kernel-args seam (`internal/appliance/kernelargs.go`).
4. Doctor surfaces misalignment as warnings.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host sysfs ↔ inventory | new topology readers (testable roots) | [ ] |
| Inventory ↔ doctor | alignment checks read facts, never sysfs directly | [ ] |
| Appliance config ↔ kernel cmdline | per-node hugepage args via the shared seam | [ ] |

### Integration Points
- `internal/component/host/` detectors - new facts.
- `internal/appliance/kernelargs.go` - per-node arguments (seam from spec-vpp-host-tuning).
- `ze doctor` - new checks per `ai/rules/repo-maintenance.md`.

### Architectural Verification
- [ ] No bypassed layers (doctor reads inventory facts, not raw sysfs)
- [ ] No unintended coupling (topology readers behind testable helpers)
- [ ] No duplicated functionality (placement selection stays in spec-vpp-isolated-cpus scope)
- [ ] Registration over hardcoding - doctor checks register in the owning package

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | QEMU can present multi-node NUMA + SMT topologies good enough for evidence | QEMU `-numa node` / `-smp threads=` options | evidence needs real hardware; scope shrinks to unit tests over fixture sysfs | prototype a `-numa` boot in the evidence harness | unvalidated |
| A-2 | `hugepages=<node>:<count>` boot syntax is supported by the runtime kernel version | kernel 7.1.1 pinned (internal/appliance/kernel.version) is far above the syntax's introduction | fall back to early-boot per-node sysfs writes | kernel docs for the pinned version + QEMU evidence | unvalidated |
| A-3 | SMT sibling map is derivable from existing cpuinfo fields (core id + package id) without new sysfs reads | `CoreInfo` already carries both (inventory.go) | read `topology/thread_siblings_list` from sysfs instead | unit test against a hyperthreaded fixture | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Scope creep back into a deploy tool | design adds host mutation beyond image build | keep host prep = image build + doctor, same boundary as vpp-host-tuning |
| R-2 | Warnings too noisy on single-socket/no-SMT hosts | doctor output on the reference hardware | checks no-op unless >1 node / SMT present |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| host inventory on a NUMA/SMT host | → | new topology facts | (name at design) |
| vpp.enabled with misaligned NIC/worker nodes | → | doctor warning | (name at design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | DPDK NIC on node 1, all workers on node 0 | doctor warning naming NIC, node, and worker cores (provisional; refine at design) |
| AC-2 | worker core whose SMT sibling is not dedicated | doctor warning (provisional) |
| AC-3 | per-node hugepage reservation configured | per-node counts visible in /sys/devices/system/node/*/hugepages (provisional) |
| AC-4 | single-node host | no new diagnostics, unchanged behaviour (provisional) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (design) | `internal/component/host/...` | topology facts from fixture sysfs | |
| (design) | `internal/component/vpp/...` | alignment checks | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| per-node hugepage count | design | design | design | design |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-numa-doctor` | `test/plugin/vpp-numa-doctor.ci` (provisional name; confirm at design) | doctor surfaces NUMA misalignment | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - host topology, no wire protocol | - | - | QEMU `-numa` evidence covers boot behaviour | - |

## Files to Modify
- `internal/component/host/` - topology facts (CPU node, NIC numa_node, SMT siblings)
- `internal/appliance/kernelargs.go` - per-node hugepage arguments (seam created by spec-vpp-host-tuning)
- `internal/core/diagnostic/codes.go` - new doctor codes

## Files to Create
- doctor check files in the owning packages (names at design)
- QEMU `-numa` evidence script (name at design)

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file (skeleton - run `/ze-spec` RESEARCH/DESIGN first; implement AFTER spec-vpp-isolated-cpus and spec-vpp-host-tuning) |

### Implementation Phases
1. **RESEARCH/DESIGN (not started)** - run the `/ze-spec` workflow: extend host inventory first (facts), then validation/doctor, then per-node reservation; confirm A-1..A-3.

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Known Limitations
- Skeleton only: acceptance criteria and tests above are provisional placeholders to be refined during DESIGN.
- Worker-core selection is owned by `plan/spec-vpp-isolated-cpus.md`; global hugepage reservation and the kernel-args seam by `plan/spec-vpp-host-tuning.md`.

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
- [ ] QEMU evidence for topology-dependent behaviour

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
