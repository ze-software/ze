# Spec: qemu-targets-boot-the-shipped-kernel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | `plan/deferrals/qemu-targets-boot-the-shipped-kernel.md` <!-- doc-links: ignore (nothing was deferred, so this shard was never created) --> |
| Handoff | - |
| Updated | 2026-08-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Seven of the thirteen `qemu-run.py` invocations in `internal/le/integration/gates.go` boot
the stock Alpine kernel while `internal/appliance/kernel.version` reads 7.2 and
`validate_version` (`tools/kernel-builder/build.py` (retired; now `internal/le/deployment/hostkernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) refuses anything below 7.0.
Their verdicts are true of Alpine 6.12.13-0-virt and silently untrue of the
product. Six of the seven state no reason for it.

Make every QEMU target boot the kernel ze ships, and remove the friction that
made stock the quick path.

The friction is one omission. The `docker run` argument list in
`tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> passes no `--user`, so the kernel build runs as
root and leaves `tmp/kernel/build/lib/modules/<version>/` owned `root:root` in a
scratch tree every other target owns as the invoking user. The next
`ze appliance kernel` then cannot clean it and needs a `sudo rm` no
automated caller can issue. Two instances are in the tree now.

**Corrected 2026-08-24, while implementing.** Two claims in this paragraph did
not survive contact.

- "Two instances are in the tree now" was true when this spec was written and
  false when the work started: `find tmp \! -user thomas` over 742,856 paths
  returned nothing, so Thomas had cleared both. The MECHANISM was reproduced
  instead, by writing through the same mount from the same image and meeting the
  same `rm: cannot remove ...: Permission denied`.
- "passes no `--user`" is right, and adding one is the wrong fix. See A-1.

One more count was wrong. SIX invocations were said to pass
`--kernel $(ZE_QEMU_KERNEL)`; three of those six passed the literal
`tmp/kernel/vmlinuz` instead, so only three used the variable. AC-2 asks for
thirteen using the variable, and all three literals were converted.

The cost that appears to justify stock does not exist. `ze-kernel-vmlinuz-stage`
routes through a durable arch-and-config-keyed cache under `~/.cache/ze` and
prints `Runtime kernel cache HIT: materializing from <dir> (no ~30-min
rebuild)`. The ~30 minutes is a cold-cache cost, paid once per kernel config
change and shared by every target, not a per-target or per-run cost. CI already
stages the kernel this way (`.github/workflows/qemu-nightly.yml`).

## Required Reading

- [ ] `internal/le/integration/gates.go` - the thirteen invocations, `ze-qemu-kernel-guard`, `ZE_QEMU_KERNEL`
- [ ] `internal/appliance/cmd_build.go` - `ze-kernel-vmlinuz-stage` and the durable-cache materialize path
- [ ] `tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - the `docker run` argument list, and where a `--user` belongs
- [ ] `tools/kernel-builder/build.py` (retired; now `internal/le/deployment/hostkernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - `validate_version`, the 7.0 floor
- [ ] `internal/appliance/cache.go` - `copyTree`, the atomicity the the native action tables under `internal/le/` staging mirrors
- [ ] `.github/workflows/qemu-nightly.yml` - how CI stages a kernel without an appliance build

### Architecture Docs

- [ ] `docs/architecture/testing/interop.md` - scenario structure and the naming rule
- [ ] `ai/rules/platform-linux.md` - QEMU integration tests are mandatory; it states that the kernel-consuming targets share one guard

## Current Behavior (MANDATORY)

Source read for this section:

- [ ] `internal/le/integration/gates.go`
- [ ] `internal/appliance/cmd_build.go`
- [ ] `tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->
- [ ] `internal/le/deployment/`

Thirteen real `qemu-run.py` invocations. Six pass `--kernel $(ZE_QEMU_KERNEL)`
AND take `$(ze-qemu-kernel-guard)` AND depend on `./le build-artifacts host`. The guard has
exactly six call sites and each sits in one of those six recipes, so the three
properties travel together today. Seven invocations have none of them:

| Target | Group |
|--------|-------|
| `ze-qemu-debug` | developer reproduction |
| `ze-qemu-shell` | developer reproduction |
| `_ze-qemu-integration-test-impl` | general suite |
| `_ze-qemu-netns-test-impl` | general suite |
| `_ze-qemu-ldp-frr-test-impl` | protocol interop |
| `_ze-qemu-isis-frr-test-impl` | protocol interop |
| `_ze-qemu-vrrp-keepalived-test-impl` | protocol interop, and the only one with a written reason |

`ze-qemu-debug` is the sharpest case: it exists to reproduce a failure, and it
reproduces it on a different kernel than the one that failed. A firewall and
ddos concurrency measurement was taken through it on 2026-08-24 and reported as
evidence about the runtime environment. It was Alpine's.

`_ze-qemu-vrrp-keepalived-test-impl` states its reason: VRRP needs only macvlan,
bridge and veth, all present on stock and probed 2026-07-15, and staying on
stock avoids a ~30-minute build. That reason cites the isis-frr and ldp-frr labs
as its precedent, and neither of those states a reason of its own.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

`./le qemu run command "<native guest action>" kernel tmp/kernel/vmlinuz` ->
`internal/le/qemu/run.go` -> the QEMU guest.

### Transformation Path

`ze appliance kernel` dispatches through `internal/appliance/cmd_kernel.go`.
A cache hit materializes `~/.cache/ze/<key>/` into `tmp/kernel/build/` and
stages `tmp/kernel/vmlinuz`; a miss runs the native appliance kernel builder and
then populates the cache.

### Boundaries Crossed

| From | To | Where | What crosses |
|------|----|-------|--------------|
| host filesystem | container filesystem | `-v {out_dir}:/out` in the `docker run` argv | the built kernel and its modules, written by uid 0 |
| durable cache | working tree | `internal/appliance/cmd_kernel.go` materialization | `vmlinuz` and `lib/modules` |
| native QEMU action | QEMU VM | `internal/le/qemu/run.go`, `kernel <path>` | the kernel the VM boots |

### Integration Points

`ze-qemu-kernel-guard` is the one place that asserts the staged kernel is this
tree's runtime kernel: it compares `tmp/kernel/vmlinuz` against the durable
cache entry with `cmp -s` and fails on a mismatch, on an absent stage, and on an
absent cache entry.

### Architectural Verification

| Claim | Holds? | Evidence |
|-------|--------|----------|
| The guard had exactly six call sites, one per `--kernel` invocation | yes, before the change | `grep -c` over `internal/le/integration/gates.go` returned 6 guard call sites, 6 `--kernel` flags and 13 `qemu-run.py` invocations. All three now read 13 |
| No target passes `--kernel` without also taking the guard | yes | derived rather than counted: `TestQemuTargetsGuardTheStagedKernel` iterates `qemuRunTargets`, which fails when it attributes fewer invocations than the file holds. That vacuity check fired for real at 12 of 13 and exposed `recipeOf` reading only `ze-qemu-debug`'s first rule line |
| A cache HIT costs a copy and no rebuild | yes | in `ze-kernel-vmlinuz-stage` (`internal/appliance/cmd_build.go`) the HIT branch runs `mktemp -d`, `cp -R`, `mv`, and only the MISS branch reaches `$(MAKE) -C gokrazy/kernel`. Measured: MISS 22 min, the following HIT 1.4 s |
| The `docker run` argv passes no `--user` | yes, and it must not | `run_docker` (`tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) builds the argv with no `--user`, and A-1 below shows that adding one denies the build its source tree. `repair_out_dir_ownership` is what replaces it |
| Nothing outside the container needs root-owned output | yes | `ze-kernel-vmlinuz-stage` `rm -rf`s and `cp -R`s the tree as the invoking user, and `write_provenance` (`run.py`) writes `kernel.version` into it from the host afterwards. Both need it uid-owned and neither needs it root-owned. A real build through the patched driver exited 0 with all 30 output paths `thomas:thomas` |
| A test may drive the real kernel stage without isolating the durable cache | NO -- broken | `test/install/ze-kernel-overlay.ci` did. One `./le functional install` wrote a 518-byte fake `vmlinuz` into `~/.cache/ze/runtime-kernel/`, after which `ze-qemu-kernel-guard` refused every QEMU target. Fixed here; row in `plan/journal/suite-shares-one-persistent-store.md` |

## Risks & Assumptions

### Assumptions

- A-1: **BROKEN**, and it decided the fix shape as expected -- though not for the
  reason the spec guessed. `modules_install` into `/out` is not the blocker: the
  build writes two NAMED VOLUMES, `/build` and `/tmp/kbuild`, and docker creates
  a named volume owned `root:root 0755`. Probed 2026-08-24 with `--user
  1000:1000` against the live volumes and again against volumes created FRESH for
  the probe: `/out` accepted a write, both named volumes refused one, on the
  fresh pair as on the old. `--user` therefore denies the build its source tree
  on a clean machine too, and no ordering or migration makes it work without a
  privileged recursive chown of those volumes. The fix is the spec's own
  fallback: `repair_out_dir_ownership` (`tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) -->) chowns
  `/out` from a `finally`. AC-1's second clause is met and its first is void.
- A-2: **CONFIRMED, with a caveat worth stating.** `kernelCacheVariantFor`
  (`internal/appliance/cache.go`) hashes the resolved fragments, their manifests
  AND every module under `tools/kernel-builder/`. So the key does cover what
  makes two kernels differ -- and it means this spec's own `run.py` edit moved
  the key and invalidated every existing runtime-kernel cache entry. One ~22-min
  rebuild per machine and per CI cache key, paid once. The guard's `cmp -s` did
  its job during that transition: it refused a stale staged kernel rather than
  booting it.
- A-3: **CONFIRMED.** Nothing in `internal/le/integration/gates.go` stated a reason for any
  of the four, and none was found. All four now boot the runtime kernel and
  `make -n` shows the flag and the guard on each. `ze-qemu-debug` is the case
  that most needed it: it exists to reproduce a failure.
- A-4 (new): **BROKEN.** A functional test may not drive the real kernel stage
  without isolating `XDG_CACHE_HOME`. See the Architectural Verification row.

### Risks

- R-1: Moving the interop labs makes them depend on a staged kernel, so a
  contributor with a cold cache pays the build once. Mitigated by the cache
  being shared with every other QEMU target, and by CI already staging.
- R-2: `_ze-qemu-vrrp-keepalived-test-impl`'s reason cites the two interop labs.
  Moving those two without revisiting it leaves a stale citation, which is the
  class this repository keeps meeting.
- R-3: A target that gains the guard fails loudly where it used to run. That is
  the point, and it still changes what a contributor sees on a cold cache.

## Blast Radius

Every QEMU target, `.github/workflows/qemu-nightly.yml`, and any developer
running `ze-qemu-debug` or `ze-qemu-shell`. No production code path.
`internal/le/deployment/` derives its own list of `--kernel`
users, so it follows the change rather than needing a hand edit.

## Wiring Test (MANDATORY -- NOT deferrable)

`internal/le/deployment/` already asserts which targets boot
the runtime kernel. Extend it to assert the THREE properties together for every
`qemu-run.py` invocation: `--kernel`, the guard, and the `./le build-artifacts host`
prerequisite. A target added later carrying one but not the others must go red.

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le qemu run` | → | `internal/le/integration/gates.go` recipe, `--kernel $(ZE_QEMU_KERNEL)` | `TestQemuFunctionalTargetsBootTheRuntimeKernel` |
| `./le qemu run command "./le functional isis"` | → | same recipe, `$(ze-qemu-kernel-guard)` | `TestQemuTargetsGuardTheStagedKernel` |
| `./le qemu netns-test` | → | same recipe, `: ./le build-artifacts host` prerequisite | `TestQemuTargetsDependOnHostBuild` |
| the retired `ze-kernel-vmlinuz-stage` (current: `ze appliance kernel`) | → | `docker run` argv in `tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> | `test/install/kernel-build-output-ownership.ci` (A-1 broke `--user`, so the test asserts the ownership REPAIR argv and its ordering, and asserts it after a FAILED build too) |
| any booted VM | → | the kernel the VM booted | `assert_runtime_kernel_booted` (`internal/le/qemu/run.go`), run at every boot that passes `--kernel` |
| a workflow JOB running a guarded target | → | a kernel staged in that same job | `TestQemuKernelPreconditionIsMetInTheSameJob` (`internal/le/`), revived from a dead file path |

## Acceptance Criteria

- AC-1: `tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> runs the container as the invoking user,
  and a kernel build leaves no root-owned path under `tmp/`.
- AC-2: All thirteen `qemu-run.py` invocations pass `--kernel $(ZE_QEMU_KERNEL)`.
- AC-3: All thirteen recipes take `$(ze-qemu-kernel-guard)`.
- AC-4: All thirteen depend on `./le build-artifacts host`.
- AC-5: The wiring test asserts AC-2, AC-3 and AC-4 for every invocation and
  goes red when any one is removed.
- AC-6: `_ze-qemu-vrrp-keepalived-test-impl`'s comment no longer cites the
  interop labs as a stock-kernel precedent.
- AC-7: One QEMU target that previously booted stock is run on the runtime
  kernel and its result recorded.

## End-to-End User Stories

An operator reproduces a QEMU failure with `./le qemu run` and the VM boots
the kernel their appliance runs, so what they see is what shipped.

A contributor runs `./le qemu run command "./le functional isis"` on a warm cache and pays a file
copy, not a kernel build.

A developer deletes `tmp/` and re-runs any QEMU target without `sudo`.

## 🧪 TDD Test Plan

### Unit Tests

| Test | Asserts |
|------|---------|
| `TestQemuFunctionalTargetsBootTheRuntimeKernel` (extend) | every invocation passes `--kernel` |
| `TestQemuTargetsGuardTheStagedKernel` (extend) | every recipe takes the guard |
| `TestQemuTargetsDependOnHostBuild` (new) | every recipe has `./le build-artifacts host` |
| `TestKernelBuilderRunsAsInvokingUser` (new) | the `docker run` argv carries `--user <uid>:<gid>` |

### Functional Tests

| Test | Location | Scenario |
|------|----------|----------|
| the guest is running ze's kernel | `assert_runtime_kernel_booted` (`internal/le/qemu/run.go`) | reads `uname -r` over SSH right after the VM comes up and refuses to go on unless it matches `internal/appliance/kernel.version`. This is the check that would have caught the whole defect. It replaces the planned `test/qemu/runtime-kernel-booted.ci` <!-- doc-links: ignore (a suite this spec considered and did not create) -->, and is STRONGER than it: a `.ci` runs only in the four targets that run `.ci` suites, while this runs at all thirteen boots, needs no new suite, no new runner option and no registration. A `.ci` would also have needed a QEMU-only marker, which the runner has no equivalent of -- `option=needs-linux` still runs on a Linux HOST, where `uname -r` is the host's kernel and the assertion is meaningless |
| a kernel build leaves no root-owned scratch | `test/install/kernel-build-output-ownership.ci`, plus a real build | the `.ci` asserts the repair argv, its ordering after the build, and its presence after a FAILED build, with only docker faked. The real build is the end-to-end evidence: `run.py` drove an amd64 7.2 build to exit 0 and left 30 output paths, every one owned by the invoking user |

## Files to Modify

- `tools/kernel-builder/run.py` (retired; now `internal/le/deployment/gokrazykernel.go`) <!-- doc-links: ignore (retired 2026-08-28 by eae282592) --> - `repair_out_dir_ownership`, called from a `finally` in `run_docker`. Not `--user`: A-1 broke
- `internal/le/integration/gates.go` - seven recipes gain `--kernel`, the guard and the prerequisite; three literal `--kernel tmp/kernel/vmlinuz` become `$(ZE_QEMU_KERNEL)`; the vrrp comment corrected, and three more comments that went false with it
- `internal/le/deployment/` - the three properties, each its own test over a derived population; two parser defects fixed (a recipe ends at a make conditional, a target's prerequisites come from one rule line)
- `internal/le/qemu/run.go` - `assert_runtime_kernel_booted`, the guest-side check
- `.github/workflows/qemu-nightly.yml` - `protocol-labs` gains the kernel cache restore, the stage and the save; its budget follows `runtime-kernel-labs`
- `internal/le/` - revived: it read `internal/appliance/cmd_build.go`, renamed away by 72d2f0d59
- `test/install/ze-kernel-overlay.ci` - isolates `XDG_CACHE_HOME` and asserts the real cache is untouched
- `test/install/{appliance-kernel-docker,appliance-kernel-auto-docker,appliance-kernel-registry,appliance-kernel-runtime,kernel-compose,kernel-version-provenance}.ci` - each fake docker now treats only a run carrying `build.py` as a build
- `ai/rules/points/platform-linux/.../both-functional-targets-boot-zes-runtime-kernel.md` - six targets becomes thirteen, plus the two new obligations
- `docs/architecture/testing/qemu-integration.md` - the three properties and what staging costs

## Files to Create

- `test/install/kernel-build-output-ownership.ci` - the ownership repair's argv, ordering, and its presence after a failed build
- `plan/deferrals/qemu-targets-boot-the-shipped-kernel.md` <!-- doc-links: ignore (nothing was deferred, so this shard was never created) --> - nothing deferred, so not created

### Integration Checklist

- [ ] `make -n` on each of the seven shows the guard and `--kernel`
- [ ] `ze appliance kernel` succeeds twice in a row without `sudo`
- [ ] `.github/workflows/qemu-nightly.yml` still satisfies the guard

Answers, with evidence (checkboxes stay unticked per `.claude/rules/post-compaction.md`):

| Item | Answer | Evidence |
|------|--------|----------|
| `make -n` on each of the seven | Yes | all seven print exactly one `--kernel tmp/kernel/vmlinuz` and one `ze-host ... --print-cache-dir` |
| stage twice in a row without `sudo` | Yes | first run MISS, built and populated the cache; second run `Runtime kernel cache HIT`, 4 s, exit 0. `find tmp/kernel \! -user thomas` empty after both |
| the workflow still satisfies the guard | Yes | `protocol-labs` gained the cache restore, the stage and the save. `TestQemuKernelPreconditionIsMetInTheSameJob` passes with it and fails naming the job and its three targets without it |

### Documentation Update Checklist (BLOCKING)

- [ ] `docs/architecture/testing/interop.md` - every QEMU target boots the shipped kernel
- [ ] `ai/rules/platform-linux.md` - its "all six kernel-consuming targets" sentence becomes thirteen

Answers, with evidence:

| Item | Answer | Evidence |
|------|--------|----------|
| `docs/architecture/testing/interop.md` | N-A, and the spec was wrong to name it | `grep -n 'qemu\|QEMU\|kernel'` over that file returns NOTHING. It documents the DOCKER interop labs and says nothing about QEMU. The QEMU doc is `docs/architecture/testing/qemu-integration.md`, which gained the three properties, the guest check and what staging costs |
| `ai/rules/platform-linux.md` | Yes | edited at its POINT file, `ai/rules/points/platform-linux/.../both-functional-targets-boot-zes-runtime-kernel.md`, and rendered with `./le rules render-update`; the generated file refuses a direct edit. Six becomes thirteen, and two obligations are added: a target MUST NOT be written to boot stock, and a test driving the real kernel stage MUST isolate `XDG_CACHE_HOME`. All five rule gates green |

## Implementation Steps

1. Fix the root-owned scratch first. Until it is fixed, every target moved
   inherits a landmine, and the second instance is already in the tree
   unremovable by any automated caller.
2. Validate A-1 before choosing the fix shape: run the build with `--user` and
   see whether it completes. If the container needs root internally, chown on
   exit instead.
3. Move the four non-interop targets.
4. Move the two interop labs, and revisit the vrrp comment in the same change.
5. Extend the wiring test and prove it goes red on each of the three properties.

### Critical Review Checklist

- [ ] Does the wiring test fail when a target carries `--kernel` but not the guard?
- [ ] Does a cold cache produce a clear error rather than a stock-kernel boot?
- [ ] Is any target's stated reason for stock left standing but now false?

Answers, with evidence:

| Check | Answer | Evidence |
|-------|--------|----------|
| red when a target keeps `--kernel` but loses the guard | Yes | mutation M2 of six, run against a scratch copy of the makefile: `_ze-qemu-isis-frr-test-impl` keeps the flag and loses the guard. `TestQemuTargetsGuardTheStagedKernel` FAILs and the other two stay PASS. The matrix is a clean diagonal over all six mutations, so no check is standing in for another |
| a cold cache errors rather than booting stock | Yes, measured on the target that used to boot stock | `XDG_CACHE_HOME=<empty dir> the retired make ze-qemu-debug (current: ./le qemu run) RUN=...` exits 1 before any VM with `error: no amd64 runtime kernel in the durable cache (run: the retired make ze-kernel-vmlinuz-stage (current: ze appliance kernel) KERNEL_ARCH=amd64)`. The message names the cause and the command that fixes it |
| a stated reason for stock left standing but now false | No, and four comments were corrected, not one | `_ze-qemu-vrrp-keepalived-test-impl`'s reason was the only STATED one and it is rewritten (AC-6). Three more went false with the change and are corrected: the file header (`both functional QEMU targets`), the guard's `hand-written list of two`, and `ze-qemu-pppoe-test`'s `ze-qemu-netns-test gives the first on the stock kernel` |

### Deliverables Checklist

- [ ] Every AC has working code and a test that can fail
- [ ] No target boots a kernel it does not name

### Security Review Checklist

- [ ] `--user` does not widen what the container can write on the host
- [ ] No path gains world-writable permissions as a workaround

### Failure Routing

A red that is the guard firing on a cold cache is not a defect: stage the kernel.
A red that is a target failing ON the runtime kernel is a real finding and gets
its own row, because it is a bug the stock kernel was hiding.

**This routing ran, and it needs one more step it did not state: A/B the red
against stock before calling it a bug stock was hiding.** AC-7 ran
`ze-qemu-ldp-frr-test` and it went red. Two distinct causes came out, and only
the first was what this routing predicts:

| Finding | Hidden by stock? | Disposition |
|---------|------------------|-------------|
| FRR zebra logged `Disabling MPLS support (no kernel support)` and rejected `mpls ldp`. Ze's runtime kernel carried NO MPLS symbol, while `dumpMPLSRoutes` and `addMPLSSwap` program `AF_MPLS` | Yes. Stock Alpine has MPLS, so no run had asked this kernel the question | FIXED. Five symbols in `runtime.config` and `runtime.require`. `af_mpls.o` is in the image and the zebra line is gone. Row in `plan/journal/unwired-feature.md` |
| `TestLDPInteropFRR` still fails: `zeldp0` never becomes available on ze's side, 15 retries, `ze did not reach an operational LDP session with FRR` | NO. Measured on both: 76.6s on ze's 7.2 kernel, 77.1s on stock, identical symptom | Not fixed and not this spec's. Pre-existing, kernel-independent, and the nightly has been carrying it. Row in `plan/journal/test-against-broken-path.md` |

So AC-7's result is recorded and the move is cleared of causing the red. The
lab was already red.

## Design Insights

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Repair `/out`'s ownership rather than run the container with `--user` | A-1 broke. Docker creates a named volume owned `root:root 0755`, measured on a volume created FRESH for the probe, so `--user` denies the build `/build` and `/tmp/kbuild` on a clean machine as well as on an old one. Making `--user` work needs a privileged recursive chown of two pre-existing volumes on every machine, which is more machinery than the one container the repair costs, and reaches the same host-visible outcome |
| The guest-side check lives in `qemu-run.py`, not in a new `test/qemu/` <!-- doc-links: ignore (a suite this spec considered and did not create) --> suite | It covers all 13 boots instead of the 4 targets that run `.ci` suites, and needs no new suite, no runner option and no registration. A `.ci` would also have needed a QEMU-only marker the runner has no equivalent of: `option=needs-linux` still runs on a Linux HOST, where `uname -r` is the host's kernel and the assertion says nothing |
| Three separate tests over one derived population, not one compound check | Removing any one property must name which one went missing. Proven: six mutations, each reddening exactly one test and leaving the other two green |
| The vacuity guard compares attributed invocations against a raw line count | It is self-calibrating, so a parser that silently loses a target takes the file red instead of reporting a shrunken population as fully wired. It fired for real at 12 of 13 and found `recipeOf` reading only `ze-qemu-debug`'s first rule line |
| MPLS added to the shipped kernel rather than the LDP lab left red | The lab failing on ze's kernel is the defect this spec exists to surface, not a cost of surfacing it. `dumpMPLSRoutes` and `addMPLSSwap` program `AF_MPLS` on a kernel that carried no MPLS symbol at all |

## Known Limitations

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
