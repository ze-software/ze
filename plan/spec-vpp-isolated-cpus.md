# Spec: vpp-isolated-cpus

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-07-10 |

**Notes:** Promoted to ready per user instruction 2026-07-10 (followup-wave impact review session) authorizing conversion to ready.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/vpp/startupconf.go` - `cpu {}` section generation
4. `internal/component/vpp/config.go` - `CPUSettings`, `parseCPU`, `Validate`
5. `internal/component/vpp/yang/ze-vpp-conf.yang` - cpu leaves

## Task

VPP's dataplane workers are latency-sensitive: they busy-poll and must not share a
CPU with the Linux scheduler's other tasks. Today Ze assigns VPP worker cores by
simple arithmetic (the N cores immediately after `main-core`) and performs no
validation, so:

- Worker cores are not pinned to CPUs the kernel has **isolated** (`isolcpus` /
  `/sys/devices/system/cpu/isolated`); the scheduler can still place other work on
  them, defeating the point of dedicating cores to VPP.
- Nothing checks that the requested cores actually exist on the host, that
  `main-core` does not overlap the worker range, or that enough cores are available
  or isolated. Bad CPU config passes verify and only fails (or silently
  underperforms) at runtime.

Add isolated-CPU-aware worker placement plus CPU validation:

- Source worker cores from the kernel-isolated set (and, since Ze owns its kernel
  config, ensure the cores handed to VPP are actually isolated).
- Validate at config verify: requested cores exist, `main-core` is disjoint from the
  worker set, and enough (isolated) CPUs are available for the requested worker count.

## Required Reading

### Architecture Docs
- [ ] `docs/research/vpp-deployment-reference.md` - startup.conf `cpu {}` syntax and production values (already referenced by `startupconf.go`).
  → Constraint: VPP accepts `main-core` + either `workers` (count) or `corelist-workers` (explicit ids); Ze emits `corelist-workers`.
- [ ] `ai/rules/config.md` - CPU pinning is operator config.
  → Constraint: keep operator ergonomics (a worker count) while sourcing the actual ids from the isolated set.

**Key insights:**
- Ze owns the appliance kernel config (gokrazy), so it can both request isolation for a core set at boot and consume it, closing the loop end-to-end rather than relying on externally provisioned isolation.
- The feature is two halves: (1) choose worker cores from the isolated set; (2) validate the choice at verify.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/vpp/startupconf.go` - the `cpu {}` section writes `main-core` directly and computes `corelist-workers` via `workerCoreList(mainCore, count)` as a contiguous range `mainCore+1 .. mainCore+count` (startupconf.go, `workerCoreList` at :124-131). No isolated-CPU sourcing.
- [ ] `internal/component/vpp/config.go` - `CPUSettings{MainCore *uint8; Workers *uint8}` (config.go); `parseCPU` only range-checks uint8 and rejects unknown keys (config.go); `VPPSettings.Validate()` never references CPU/core (config.go).
- [ ] `internal/component/vpp/yang/ze-vpp-conf.yang` - exposes only `main-core` and `workers` (count) under `cpu` (cpu container :46, main-core :49, workers :56).

### Post-wave corrections (2026-07-10)

All refs re-verified against current code after the followup-spec wave (the
wireguard startup.conf toggle landed in the SAME files):

- Line drift corrected in place above (old -> new): `CPUSettings` config.go
  -> :51-53; `parseCPU` :296-319 -> :323-347; `Validate` :255-294 -> :282-321
  (re-read in full: still NO CPU/core validation -- the gap this spec closes is
  confirmed open); `workerCoreList` startupconf.go -> :124-131 (re-read:
  still the naive contiguous range mainCore+1..mainCore+count). The `cpu {}`
  section is unchanged at startupconf.go; yang cpu container :46,
  main-core :49, workers :56.
- Rebase requirement: the wave added a VPP plugin-enablement surface in these
  same files -- `PluginSettings` on `VPPSettings` (config.go), `parsePlugins`
  (config.go), a `plugins {}` startup.conf section (startupconf.go)
  with the wireguard toggle (`s.Plugins.Wireguard`, :84-86), and a yang
  `plugins` container with the `wireguard` leaf (ze-vpp-conf.yang). The
  cpu work must rebase onto this layout: the generated `cpu {}` section sits
  above the new `plugins {}` section, and new cpu leaves join a schema that
  now also carries the plugins container. No design change needed, only
  merge awareness.

**Behavior to preserve:**
- Operator ergonomics: a worker *count* remains a valid way to ask for N workers.
- The `cpu {}` startup.conf output stays valid VPP syntax (`main-core` + `corelist-workers`).
- When CPU config is absent, behaviour is unchanged.

**Behavior to change:**
- Worker cores are sourced from the isolated set (not naive offset), and CPU config is validated at verify.

## Data Flow (MANDATORY)

### Entry Point
- Config: `cpu` leaves in `ze-vpp-conf.yang` (`main-core`, `workers`, and design may add an explicit `worker-cores` id-list and/or an `isolate` toggle).
- Host facts: the kernel-isolated CPU set (`/sys/devices/system/cpu/isolated`) and the online CPU inventory.

### Transformation Path
1. `parseCPU` reads the cpu leaves into `CPUSettings` (config.go).
2. `Validate()` gains CPU checks: requested cores exist in the online inventory; `main-core` disjoint from worker cores; enough isolated CPUs for the worker count.
3. Worker core selection draws from the isolated set (via a new helper that reads `/sys/devices/system/cpu/isolated`), replacing the naive `mainCore+1..` arithmetic.
4. `startupconf.go` emits `main-core` + `corelist-workers` from the validated, isolated-sourced core list.
5. Kernel config side (Ze-owned): ensure the chosen cores are marked isolated at boot.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ VPP settings | cpu leaves → `CPUSettings` via `parseCPU` | [ ] |
| Verify ↔ host inventory | `Validate()` reads online + isolated CPU sets | [ ] |
| Settings ↔ startup.conf | isolated-sourced core list → `corelist-workers` | [ ] |
| VPP config ↔ kernel config | chosen cores requested as isolated at boot | [ ] |

### Integration Points
- `workerCoreList` (`startupconf.go`) - replace/augment with isolated-set sourcing.
- `VPPSettings.Validate()` (`config.go`) - add CPU validation.
- kernel/boot config (gokrazy cmdline) - request isolation for the VPP core set.

### Architectural Verification
- [ ] No bypassed layers (cpu config via `parseCPU`/`Validate`)
- [ ] No unintended coupling (host-CPU read isolated behind a helper, testable)
- [ ] No duplicated functionality (single core-selection helper feeds startup.conf)
- [ ] Registration over hardcoding — CPU validation lives in VPP's own `Validate`, not a central switch.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | `/sys/devices/system/cpu/isolated` is readable and reflects boot isolcpus | standard Linux l3mdev/isolcpus | need a different source | read the file on a running appliance | unvalidated |
| A-2 | Ze can influence the boot cmdline to set isolcpus for chosen cores | gokrazy owns kernel cmdline | isolation must be operator-provisioned | check gokrazy cmdline handling during audit | unvalidated |
| A-3 | uint8 core ids are sufficient (≤255 cores) | current `CPUSettings` uses uint8 | large hosts need wider type | confirm target hardware core counts | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Requesting isolation but boot not updated → workers on non-isolated cores | perf regression, jitter | verify-time warning when chosen cores are not in the isolated set |
| R-2 | Over-isolating starves the control plane | host sluggish, few cores for Linux | validate a minimum of non-isolated cores remain |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set vpp cpu workers 2` on a host with isolated cores | → | worker cores sourced from isolated set into `corelist-workers` | `test/plugin/vpp-isolated-cpus.ci` |
| `main-core` overlaps worker range | → | `Validate()` rejects | `test/plugin/vpp-cpu-validation.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | isolated set = {2,3,4}, `workers 2` | `corelist-workers` uses isolated cores (e.g. 3,4), not naive `main-core+1..` |
| AC-2 | `workers` count exceeds available isolated cores | config verify rejects with a clear error |
| AC-3 | `main-core` inside the worker range | config verify rejects (overlap) |
| AC-4 | requested core id not present on host | config verify rejects |
| AC-5 | chosen cores not actually isolated | verify warns (or errors, per design) |
| AC-6 | no cpu config | unchanged behaviour |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | requests 2 VPP workers on an appliance with isolated cores | config → validate → isolated-sourced corelist → startup.conf | `test/plugin/vpp-isolated-cpus.ci` |
| 2 | mis-configures overlapping/oversized cores | config verify rejects with actionable message | `test/plugin/vpp-cpu-validation.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestWorkerCoresFromIsolatedSet` | `internal/component/vpp/startupconf_test.go` | corelist drawn from isolated set | |
| `TestCPUValidateOverlap` | `internal/component/vpp/config_test.go` | main-core/worker overlap rejected | |
| `TestCPUValidateInsufficientCores` | `internal/component/vpp/config_test.go` | too-many-workers rejected | |
| `TestCPUValidateUnknownCore` | `internal/component/vpp/config_test.go` | non-existent core id rejected | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| main-core | 0..NumCPU-1 | NumCPU-1 | N/A | NumCPU |
| workers | 0..available-isolated | available | N/A | available+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-isolated-cpus` | `test/plugin/vpp-isolated-cpus.ci` | workers pinned to isolated cores | |
| `vpp-cpu-validation` | `test/plugin/vpp-cpu-validation.ci` | bad CPU config rejected at verify | |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - dataplane/appliance config, no wire protocol | - | - | validated by functional + QEMU tests | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/vpp/startupconf.go` - source worker cores from the isolated set
- `internal/component/vpp/config.go` - add CPU validation to `Validate()`; possibly widen/extend `CPUSettings`
- `internal/component/vpp/yang/ze-vpp-conf.yang` - cpu leaves (optional explicit core-id list / isolate toggle)
- gokrazy/appliance boot config - request isolcpus for the VPP core set

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new/changed config) | [ ] maybe | `ze-vpp-conf.yang` cpu leaves; `ai/rules/config.md`, `ai/rules/config.md` |
| YANG validation constraints | [ ] yes | ranges; custom validator if cross-field |
| Doctor check for runtime dependencies | [ ] yes | reads `/sys/devices/system/cpu/isolated`; `ai/rules/repo-maintenance.md` |
| Functional test for new behaviour | [ ] yes | `test/plugin/vpp-isolated-cpus.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] yes | `docs/features.md` |
| 2 | Config syntax changed? | [ ] maybe | `docs/guide/configuration.md` |
| 12 | Internal architecture changed? | [ ] yes | `docs/research/vpp-deployment-reference.md` |

## Files to Create
- `internal/component/vpp/isolated_linux.go` - read `/sys/devices/system/cpu/isolated`
- `internal/component/vpp/isolated_linux_test.go` - unit tests (sysfs root override)
- `test/plugin/vpp-isolated-cpus.ci`, `test/plugin/vpp-cpu-validation.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Phase: Wiring (MANDATORY FIRST)** — add a testable isolated-set reader (sysfs root override) and route `workerCoreList` through it (still returns the same set initially); failing `test/plugin/vpp-isolated-cpus.ci`.
2. **Phase: Isolated sourcing** — select worker cores from the isolated set.
   - Tests: `TestWorkerCoresFromIsolatedSet`
3. **Phase: CPU validation** — add overlap/inventory/sufficiency checks to `Validate()`.
   - Tests: `TestCPUValidateOverlap`, `TestCPUValidateInsufficientCores`, `TestCPUValidateUnknownCore`
4. **Phase: Boot isolation** — request isolcpus for the chosen cores (appliance config).
5. **Functional tests**
6. **Full verification** → `make ze-precommit-verify`
7. **Complete spec** → audit, learned summary, two-commit closure.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | corelist only contains isolated, existing, non-overlapping cores |
| Data flow | host CPU reads behind a testable helper |
| Doctor checks | isolated sysfs path checked |
| Registration over hardcoding | CPU validation in VPP `Validate`, not central |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| isolated sourcing | `go test ./internal/component/vpp -run Isolated` |
| CPU validation | `test/plugin/vpp-cpu-validation.ci` passes |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | core ids bounded to host inventory |
| Resource exhaustion | control plane retains enough non-isolated cores |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights
<!-- LIVE -->
- Coordination (added 2026-07-10 by the spec-vpp-host-tuning design session):
  Phase "Boot isolation" shares ONE kernel-argument assembly seam with
  `plan/spec-vpp-host-tuning.md` -- a function in `internal/appliance/kernelargs.go`
  that computes per-appliance extra kernel arguments and hands them to gok via a
  derived instance config (temp parent dir patching `KernelExtraArgs` of
  `gokrazy/ze/config.json`; gok resolves `<parent_dir>/<instance>/config.json`,
  gokrazy/internal instanceflag.go; packer appends the args to /cmdline.txt,
  tools packer/write.go). Whichever spec is implemented first creates the
  seam and the derived-config writer; the second only adds its arguments there.
  Do not build a second cmdline path.

## Implementation Summary
### What Was Implemented
- (fill during implementation)

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in <commit/line> / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `make ze-standard-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
