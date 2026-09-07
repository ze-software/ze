# Spec: vpp-isolated-cpus

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 5/5 |
| Updated | 2026-09-05 |

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
| A-1 | `/sys/devices/system/cpu/isolated` is readable and reflects boot isolcpus | standard Linux l3mdev/isolcpus | need a different source | read the file on a running appliance | confirmed 2026-09-05: `cat /sys/devices/system/cpu/isolated` returns an empty line on a host with no isolcpus, `online` returns `0-31`. Both files exist and are readable. |
| A-2 | Ze can influence the boot cmdline to set isolcpus for chosen cores | gokrazy owns kernel cmdline | isolation must be operator-provisioned | check gokrazy cmdline handling during audit | confirmed 2026-09-05: `internal/appliance/kernelargs.go` already assembles kernel arguments (`hugepageKernelArgs`, `resolveBuildParentDir`) and its header names this spec as the second consumer of that seam. |
| A-3 | uint8 core ids are sufficient (≤255 cores) | current `CPUSettings` uses uint8 | large hosts need wider type | confirm target hardware core counts | confirmed 2026-09-05: kept at uint8. `parseCoreList` refuses an id above 255 with a named error rather than truncating. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Requesting isolation but boot not updated → workers on non-isolated cores | perf regression, jitter | verify-time warning when chosen cores are not in the isolated set |
| R-2 | Over-isolating starves the control plane | host sluggish, few cores for Linux | validate a minimum of non-isolated cores remain |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `set vpp cpu workers 2` on a host with isolated cores | → | `resolveWorkerCores` (`internal/component/vpp/cpuset.go`) draws the cores from the isolated set into `corelist-workers` | `test/plugin/vpp-isolated-cpus.ci` |
| worker cores outside the isolated set | → | `evaluateVPPCPUIsolation` (`internal/component/vpp/doctor_cpu_linux.go`) reports `doctor-vpp-cpu-isolation` | `test/plugin/vpp-cpu-not-isolated.ci` |
| `main-core` also named in `worker-cores` | → | `CPUSettings.validateAgainst` (`internal/component/vpp/cpuset.go`) rejects | `test/plugin/vpp-cpu-validation.ci` |

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
| `TestWorkerCoresFromIsolatedSet` | `internal/component/vpp/startupconf_test.go` | corelist drawn from isolated set | pass |
| `TestWorkerCoreListNoIsolation` | `internal/component/vpp/startupconf_test.go` | a host with no isolation keeps the contiguous placement | pass |
| `TestCPUValidateOverlap` | `internal/component/vpp/config_test.go` | main-core/worker overlap rejected | pass |
| `TestCPUValidateInsufficientCores` | `internal/component/vpp/config_test.go` | too-many-workers rejected | pass |
| `TestCPUValidateUnknownCore` | `internal/component/vpp/config_test.go` | non-existent core id rejected | pass |
| `TestParseCPUWorkerCores` | `internal/component/vpp/config_test.go` | the worker-cores leaf parses into an ascending core list | pass |
| `TestHostCPUInventoryIsolationUnknown` | `internal/component/vpp/cpuset_test.go` | an unreadable isolated file is not an empty isolated set | pass |
| `TestHostCPUInventoryUnreadableOnlineIsAnError` | `internal/component/vpp/cpuset_test.go` | an unreadable online file is an error, never an empty inventory | pass |
| `TestCPUValidateCannotReadHost` | `internal/component/vpp/cpuset_test.go` | validation refuses a placement it could not check | pass |
| `TestCPUValidateNoPlacementReadsNoHost` | `internal/component/vpp/cpuset_test.go` | no cpu placement leaf reads no host (AC-6) | pass |
| `TestCPUValidateWorkersAndCoresConflict` | `internal/component/vpp/cpuset_test.go` | workers and worker-cores together are refused | pass |
| `TestEvaluateVPPCPUIsolation` | `internal/component/vpp/doctor_cpu_linux_test.go` | the doctor check reports each host shape | pass |
| `TestEvaluateVPPCPUIsolationUnknownBeatsEmpty` | `internal/component/vpp/doctor_cpu_linux_test.go` | "isolated nothing" and "could not be asked" produce different messages | pass |
| `TestParse`, `TestFormat` | `internal/core/cpulist/cpulist_test.go` | the CPU list grammar, with its 0/255/256 boundaries | pass |
| `TestKernelArgsIsolatedCPUs` | `internal/appliance/kernelargs_test.go` | isolcpus, nohz_full and rcu_nocbs tokens | pass |
| `TestValidateIsolatedCPUs` | `internal/appliance/kernelargs_test.go` | CPU 0 must stay with Linux (R-2) | pass |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| main-core | 0..NumCPU-1 | NumCPU-1 | N/A | NumCPU |
| workers | 0..available-isolated | available | N/A | available+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vpp-isolated-cpus` | `test/plugin/vpp-isolated-cpus.ci` | workers pinned to isolated cores | written; RED observed under a probe that ignores the isolated set |
| `vpp-cpu-not-isolated` | `test/plugin/vpp-cpu-not-isolated.ci` | workers on non-isolated cores are reported by `ze doctor` | written |
| `vpp-cpu-validation` | `test/plugin/vpp-cpu-validation.ci` | bad CPU config rejected at verify | written; RED observed under the same probe |

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
- `internal/core/cpulist/cpulist.go` - the CPU list grammar Linux uses for isolcpus, for `/sys/devices/system/cpu/{online,isolated}` and for VPP's `corelist-workers`. In `internal/core/` rather than the planned `internal/component/vpp/isolated_linux.go` because the appliance builder validates `image.isolated-cpus` with the same grammar, and a second copy would be a future disagreement with nothing to arbitrate it.
- `internal/core/cpulist/cpulist_test.go` - the grammar, with its boundaries
- `internal/component/vpp/cpuset.go` - the host CPU inventory, worker core resolution and CPU validation. Not `_linux`-tagged: the sysfs read fails honestly on any platform that has no sysfs, and `Validate` then refuses rather than silently skipping the check.
- `internal/component/vpp/cpuset_test.go` - unit tests (sysfs root override)
- `internal/component/vpp/doctor_cpu_linux.go`, `internal/component/vpp/doctor_cpu_linux_test.go` - the `vpp-cpu-isolation` doctor check
- `test/plugin/vpp-isolated-cpus.ci`, `test/plugin/vpp-cpu-not-isolated.ci`, `test/plugin/vpp-cpu-validation.ci`

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
6. **Full verification** → `./le verify current mode full`
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

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The two worker-placement paths were written to different standards. `contiguousWorkerCores` checked every core it produced against the online inventory; `CPUInventory.workerPool` drew from `Isolated` and checked nothing, so the path this spec ADDED was the weaker of the two. | The kernel writes `/sys/devices/system/cpu/isolated` from the boot cmdline and never revises it, while a CPU hotplugged out afterwards leaves `online`. An isolated CPU can therefore be offline, and the count path would have written it into `corelist-workers`. | Closure review, applying the symmetry technique of `/ze-review` to the two branches of `resolveWorkerCores`. | Fixed in `workerPool`; `TestWorkerCoresSkipAnOfflineIsolatedCPU` and `test/plugin/vpp-cpu-offline-isolated.ci` cover it, and both were observed RED with the filter removed. |
| approach | `image.isolated-cpus` shipped with no functional test, while `image.hugepages` and `image.crash-dump`, the two reservations validated beside it in the same `Validate`, each have one under `test/appliance/`. | A new config option owes a `.ci` at the entry point the operator reaches (`ai/rules/testing.md`). A unit test on `validateIsolatedCPUs` proves the function, not that `ze appliance init` calls it. | Closure review, comparing the new option against its two siblings in `applianceConfig.Validate`. | `test/appliance/appliance-isolated-cpus-validate.ci` added. |

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
- `internal/core/cpulist` holds the CPU list grammar once. `Parse` refuses a
  reversed range, a repeated core, a core above 255 and a list longer than the
  256 ids a uint8 holds; `Format` renders ascending ids back with runs
  collapsed, so `corelist-workers 1-3` is unchanged from before this spec.
- `internal/component/vpp/cpuset.go` reads the host inventory behind the
  `ze.test.vpp.cpu.root` override. `CPUInventory.IsolationKnown` separates "the
  kernel isolated no CPU" from "this host could not tell us", which is the
  defect class `ai/rules/principles.md` names. An unreadable `online` file is an
  error, because Ze cannot then say whether a requested core exists.
- `resolveWorkerCores` is the single producer of the worker core list.
  `GenerateStartupConf` and `CPUSettings.validateAgainst` both call it, so a
  config that validates is a config Ze can write a core list for.
- `vpp cpu worker-cores` names the cores explicitly, in the kernel's own
  syntax, and MUST NOT be set beside `workers`.
- The `vpp-cpu-isolation` doctor check reports workers on non-isolated CPUs, a
  host that will not say which CPUs are isolated, and an isolated set that
  leaves Linux no CPU of its own (R-2).
- `image.isolated-cpus` writes `isolcpus`, `nohz_full` and `rcu_nocbs` through
  the existing `internal/appliance/kernelargs.go` seam, and refuses CPU 0.

### Not in this commit
The boot-isolation half (`image.isolated-cpus`, `isolatedCPUKernelArgs`,
`validateIsolatedCPUs` and their two tests) is written and green in the working
tree, and it is NOT in this commit. `internal/appliance/config.go` and
`internal/appliance/kernelargs.go` each carry another session's uncommitted
crash-dump hunks interleaved with those changes, and git stages whole files, so
committing them would carry work that is not this spec's
(`ai/rules/principles.md`). `docs/architecture/vpp-host-tuning.md` and
`docs/guide/vpp.md` describe `image.isolated-cpus` in this commit and become
true when those three files land.

### Deviations from the plan
- One YANG leaf was added rather than the leaf-list the Data Flow section
  allowed. A string leaf carrying the kernel's own list syntax lets ONE parser
  serve sysfs and the operator, which a leaf-list of uint8 would not.
- The AC-5 warning lives in `ze doctor`, not in `Validate`. Validation has one
  channel and it refuses; a warning needs a severity, and the doctor registry
  already has one.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Source worker cores from the kernel-isolated set | Done | `resolveWorkerCores`, `CPUInventory.workerPool` (`internal/component/vpp/cpuset.go`) | `GenerateStartupConf` (`startupconf.go`) writes what it returns as `corelist-workers`. |
| Ensure the cores handed to VPP are actually isolated | Done | `isolatedCPUKernelArgs` (`internal/appliance/kernelargs.go`), reached from `resolveBuildParentDir` in the same file | `image.isolated-cpus` writes `isolcpus`, `nohz_full` and `rcu_nocbs` into the built image's cmdline. Landed at `0e72b398f`. |
| Validate at verify: cores exist, `main-core` disjoint, enough isolated CPUs | Done | `CPUSettings.validate` and `validateAgainst` (`internal/component/vpp/cpuset.go`), called from `VPPSettings.Validate` (`config.go`), reached by `verifyVPPConfig` (`register.go`) | The `InProcessConfigVerifier` is the path `ze config validate` takes. |
| Report a placement the host holds but should not | Done | `evaluateVPPCPUIsolation` (`internal/component/vpp/doctor_cpu_linux.go`), registered by `register_linux.go` | Code `doctor-vpp-cpu-isolation`, declared in `internal/core/diagnostic/codes.go`. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestWorkerCoresFromIsolatedSet` (`startupconf_test.go`); `test/plugin/vpp-isolated-cpus.ci` | Isolated 2-4, `main-core 0`, `workers 2` gives `corelist-workers 2-3`, where the old arithmetic gave 1-2. |
| AC-2 | Done | `TestCPUValidateInsufficientCores` (`config_test.go`); `vpp-cpu-validation.ci` seq=1 exit 1 | |
| AC-3 | Done | `TestCPUValidateOverlap` (`config_test.go`); `vpp-cpu-validation.ci` seq=2 exit 1 | `workerPool` also removes `main-core` on the count path, and `contiguousWorkerCores` starts one after it. |
| AC-4 | Done | `TestCPUValidateUnknownCore` (`config_test.go`); `vpp-cpu-validation.ci` seq=3; and on the derived-core path `TestWorkerCoresSkipAnOfflineIsolatedCPU` (`cpuset_test.go`) plus `test/plugin/vpp-cpu-offline-isolated.ci` | The derived half was the closure review's first finding. |
| AC-5 | Done | `TestEvaluateVPPCPUIsolation` (`doctor_cpu_linux_test.go`); `test/plugin/vpp-cpu-not-isolated.ci` | Changed per Deviations: the warning lives in `ze doctor`, which has severities, rather than in `Validate`, which only refuses. |
| AC-6 | Done | `TestCPUValidateNoPlacementReadsNoHost` (`cpuset_test.go`); `vpp-cpu-validation.ci` seq=6 exit 0 | With every placement leaf absent, `validate` reads no host at all. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The 17 unit tests listed in the TDD plan | Done | `internal/component/vpp`, `internal/core/cpulist`, `internal/appliance` | All pass; the named-test run is in Pre-Commit Verification. |
| `vpp-isolated-cpus`, `vpp-cpu-not-isolated`, `vpp-cpu-validation` | Done | `test/plugin/` | All three PASS in `./le functional plugin`. |
| `TestWorkerCoresSkipAnOfflineIsolatedCPU`, `vpp-cpu-offline-isolated`, `appliance-isolated-cpus-validate` | Changed | `internal/component/vpp/cpuset_test.go`, `test/plugin/`, `test/appliance/` | Added at closure, beyond the plan, for the two review findings. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/core/cpulist/cpulist.go`, `cpulist_test.go` | Done | |
| `internal/component/vpp/cpuset.go`, `cpuset_test.go` | Done | |
| `internal/component/vpp/doctor_cpu_linux.go`, `doctor_cpu_linux_test.go` | Done | |
| `internal/component/vpp/startupconf.go`, `config.go`, `yang/ze-vpp-conf.yang` | Done | |
| gokrazy/appliance boot config | Changed | `internal/appliance/config.go` and `kernelargs.go` rather than a new file: the kernel-argument seam already existed, as the Design Insights coordination note required. |
| `test/plugin/vpp-isolated-cpus.ci`, `vpp-cpu-not-isolated.ci`, `vpp-cpu-validation.ci` | Done | |

### Audit Summary
- **Total items:** 20
- **Done:** 17
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (AC-5's warning channel, the boot-config file placement, and the three tests added at closure; each recorded in Deviations or the Mistake Log)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A VPP worker does not share a CPU with the Linux scheduler: the worker cores come from the kernel-isolated set, not from arithmetic | functional | `test/plugin/vpp-isolated-cpus.ci` PASS (`./le functional plugin`, case 758 of 772). It is a real discrimination, not a tautology: the fixture isolates 2-4 and asks for `main-core 0, workers 2`, so the replaced arithmetic answered 1-2, core 1 is outside the isolated set, and `ze doctor --json` would report `doctor-vpp-cpu-isolation` under the old placement. The file asserts that code is ABSENT. |
| A bad CPU placement is refused at verify instead of failing or underperforming at runtime | functional | `test/plugin/vpp-cpu-validation.ci` PASS: six `ze config validate` commands with per-command exit codes, covering too many workers (1), main-core inside the worker set (2), a core the host does not hold (3), both leaves set at once (4), and the two accepted shapes (5, 6). `test/plugin/vpp-cpu-offline-isolated.ci` PASS covers the derived-core half of the same goal. |
| Ze REQUESTS the isolation it later consumes, so the isolated set has something in it | functional + unit | `test/appliance/appliance-isolated-cpus-validate.ci` drives `ze appliance init` through `validateIsolatedCPUs`; `TestKernelArgsIsolatedCPUs` (`internal/appliance/kernelargs_test.go`) PASS asserts the three tokens `isolcpus=`, `nohz_full=` and `rcu_nocbs=`, and `resolveBuildParentDir` (`kernelargs.go`) appends them to the instance's `ExtraKernelArgs`. |
| The control plane keeps CPUs of its own (R-2) | functional + unit | `TestValidateIsolatedCPUs` PASS refuses CPU 0; `appliance-isolated-cpus-validate.ci` seq=2 refuses it at the CLI; `evaluateVPPCPUIsolation` warns when every online CPU is isolated, asserted by `TestEvaluateVPPCPUIsolation`. |
| Interop | not applicable | No wire protocol: this is dataplane thread placement and appliance boot configuration. The spec's Interop table records the same. |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| none | Every AC, every Wiring Test row and both user stories are implemented and proven. | - |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/vpp-isolated-cpus-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md` (23 files, verdict=clean) |
| `./le spec session review check` | clean |
| Rounds | 3 |
| Reviewer lenses used | wiring + guard audit + symmetry (round 1); functional-test coverage against the sibling config options (round 2); the round-2 fix's own scope (round 3) |

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `CPUInventory.workerPool` drew the worker pool from `Isolated` without intersecting it with `Online`, while the fallback `contiguousWorkerCores` in the same file checked every core it produced against `hasCore`. The kernel writes `/sys/devices/system/cpu/isolated` from the boot cmdline and never revises it, so a CPU hotplugged out after boot stays in that file and leaves the online set. A worker could therefore be pinned to a CPU VPP cannot run on, after a commit that passed verify. | `internal/component/vpp/cpuset.go`, `CPUInventory.workerPool` | fixed |
| 2 | NOTE | `evaluateVPPCPUIsolation` reports worker cores outside the isolated set and says nothing about `main-core`. | `internal/component/vpp/doctor_cpu_linux.go` | acknowledged: the main thread does not busy-poll, so a shared CPU costs it far less, and AC-5 names the worker placement. |
| 3 | NOTE | `textbuf.Buffer` is reused after `String()` in `evaluateVPPCPUIsolation` and in `isolatedCPUKernelArgs`. | `doctor_cpu_linux.go`, `kernelargs.go` | acknowledged: `Buffer.String` (`internal/core/textbuf/textbuf.go`) copies inline-backed data and hands heap-backed data to the string, resetting the slice either way, so the reuse is safe. Verified at the producer rather than assumed. |

### Fixes applied
- `CPUInventory.workerPool` (`internal/component/vpp/cpuset.go`) skips an isolated CPU that is not online. `resolveWorkerCores`'s insufficiency error now names the available pool beside the isolated set, because the two differ exactly when this case fires and the isolated set alone explains the refusal with the wrong number.
- `TestWorkerCoresSkipAnOfflineIsolatedCPU` (`cpuset_test.go`) and `test/plugin/vpp-cpu-offline-isolated.ci` added. Both were observed RED with the filter removed and GREEN with it restored.
- `docs/architecture/vpp-host-tuning.md` and `docs/guide/vpp.md` state the online condition, which the pages did not carry before.

### Run 2
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | ISSUE | `image.isolated-cpus` shipped with no functional test, while `image.hugepages` and `image.crash-dump` -- the two other reservations bounded by the same `applianceConfig.Validate` -- each have one under `test/appliance/`. `validateIsolatedCPUs` refusing CPU 0 is a guard proven only at the helper, with nothing driving it from `ze appliance init`. | `internal/appliance/config.go`, `validateIsolatedCPUs` | fixed by `test/appliance/appliance-isolated-cpus-validate.ci` |
| 2 | NOTE | The round-1 fix's own scope: `workerPool` on the no-isolation path draws from `Online`, so the added `hasCore` test is a tautology there and changes nothing. | `internal/component/vpp/cpuset.go` | acknowledged: the filter is correct on both paths and costs one comparison over at most 256 ids, on a cold path. |

### Run 3
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | none | Round 3 read the round-2 fix, which is one new `.ci` file adding no product code. No finding within its scope. | - | - |

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/core/cpulist/cpulist.go`, `cpulist_test.go` | yes | `ls internal/core/cpulist/` lists both. |
| `internal/component/vpp/cpuset.go`, `cpuset_test.go`, `doctor_cpu_linux.go`, `doctor_cpu_linux_test.go` | yes | `ls internal/component/vpp/` lists all four. |
| `test/plugin/vpp-isolated-cpus.ci`, `vpp-cpu-not-isolated.ci`, `vpp-cpu-validation.ci`, `vpp-cpu-offline-isolated.ci` | yes | `./le functional plugin` discovered and ran all four; the case count rose from 771 to 772 when the fourth was added. |
| `test/appliance/appliance-isolated-cpus-validate.ci` | yes | `ls test/appliance/` lists it beside its two siblings. |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Worker cores come from the isolated set | `--- PASS: TestWorkerCoresFromIsolatedSet`; `PASS 757 vpp-isolated-cpus` |
| AC-2 | A worker count larger than the isolated set is refused | `--- PASS: TestCPUValidateInsufficientCores`; `vpp-cpu-validation` PASS with `seq=1 ... :exit=1` |
| AC-3 | `main-core` inside the worker set is refused | `--- PASS: TestCPUValidateOverlap`; `vpp-cpu-validation` PASS with `seq=2 ... :exit=1` |
| AC-4 | A core the host does not hold is refused, named or derived | `--- PASS: TestCPUValidateUnknownCore`, `--- PASS: TestWorkerCoresSkipAnOfflineIsolatedCPU`; `vpp-cpu-offline-isolated` PASS |
| AC-5 | A worker core outside the isolated set is reported | `--- PASS: TestEvaluateVPPCPUIsolation`; `PASS 753 vpp-cpu-not-isolated` |
| AC-6 | No cpu leaf leaves behaviour unchanged and reads no host | `--- PASS: TestCPUValidateNoPlacementReadsNoHost`; `vpp-cpu-validation` PASS with `seq=6 ... :exit=0` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `set vpp cpu workers 2` on a host with isolated cores | `test/plugin/vpp-isolated-cpus.ci` | yes: the file writes `cpufix/online` and `cpufix/isolated`, points `ze.test.vpp.cpu.root` at them, and runs `ze doctor --json` over a config carrying `workers 2`, asserting `doctor-vpp-cpu-isolation` is ABSENT. |
| Worker cores outside the isolated set | `test/plugin/vpp-cpu-not-isolated.ci` | yes: `worker-cores 6-7` against isolated 2-4, asserting `doctor-vpp-cpu-isolation` and `worker core 6-7 is not isolated`. |
| `main-core` also named in `worker-cores` | `test/plugin/vpp-cpu-validation.ci` | yes: seq=2 runs `ze config validate -` over `main-core 3; worker-cores 2-4;` with `:exit=1`, and the file asserts `is also main-core`. |
| `ze appliance init` over `image.isolated-cpus` | `test/appliance/appliance-isolated-cpus-validate.ci` | yes: four configs through `ze appliance --dir appliances init --config`, one accepted and three refused with the message each refusal owns. |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `/sys/devices/system/cpu/isolated` returns an empty line on a host with no `isolcpus` and `online` returns `0-31`; both files exist and are readable (recorded 2026-09-05). `hostCPUInventory` reads exactly these two paths. |
| A-2 | confirmed | `resolveBuildParentDir` (`internal/appliance/kernelargs.go`) appends `isolatedCPUKernelArgs` to the instance's `ExtraKernelArgs`, which the packer writes to `/cmdline.txt`. |
| A-3 | confirmed | Core ids stay `uint8`. `cpulist.ParseID` refuses an id above `IDMax` with a named error instead of truncating, asserted by `TestParse`. |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 1. New user-facing feature -> `docs/features.md` | No. `grep -n "guide/vpp" docs/features.md` returns nothing: the VPP dataplane has no row on that page, only its firewall and traffic-control backends. Adding one would be a status claim about the whole component, which this spec does not make. The user-facing surface is documented in `docs/guide/vpp.md`. | yes |
| 2. Config syntax changed -> `docs/guide/configuration.md` | No. `grep -in vpp docs/guide/configuration.md` returns 15 hits, none of which document the `vpp {}` block's leaves; the vpp config table lives in `docs/guide/vpp.md`, where rows for `vpp.cpu.workers` and `vpp.cpu.worker-cores` were added, each with a `<!-- source: -->` anchor. | yes |
| 12. Internal architecture changed -> `docs/research/vpp-deployment-reference.md` | No change owed. That page describes VPP's own `startup.conf` syntax and the reference deployment; `corelist-workers` and `isolcpus=<worker-cores>` still read true. Ze's own design moved to `docs/architecture/vpp-host-tuning.md`, which carries three `<!-- source: -->` anchors (`cpuset.go`, `doctor_cpu_linux.go`, `cpulist.go`) and was extended at closure with the online condition. | yes |
| `docs/guide/vpp.md` worker-placement rows | Re-read against `resolveWorkerCores` and `CPUInventory.workerPool` after the round-1 fix and corrected: the isolated set is now qualified with "and any CPU no longer online excluded". | yes |
| Doctor check for a runtime dependency | `doctor-vpp-cpu-isolation` is registered by `vppCPUIsolationDoctorCheck` (`doctor_cpu_linux.go`) through `register_linux.go`, and the code is declared in `internal/core/diagnostic/codes.go`. | yes |
| `./le doc check verify` | Not run to a verdict for this spec: the gate is red across the BGP command surface and `../gh-pages/` from other sessions' uncommitted work, so it cannot answer about these pages alone. Recorded as verification debt. | no |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] `./le verify worktree` passes
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

## Progress, 2026-09-06

`6114b57a1f` carries the VPP half. The boot half was swept into another
session's commit `0e72b398f2`: the content is at HEAD, but the attribution is
wrong, and `git log -S isolcpus` answers with a kernel-panic subject.
