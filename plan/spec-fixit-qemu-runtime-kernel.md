# spec-fixit-qemu-runtime-kernel

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-fixit-qemu-artifact-cache (land that first: durable kernel cache) (superseded, removed without landing; absorbed by spec-relocate-scratch-and-cache, learned 1173 -- durable cache now live at mk/gokrazy.mk:194-232) |
| Phase | 0/N (research) |
| Updated | 2026-07-17 |

> **DESIGN, 2026-07-16.** Research done, in sequence after the cache spec. Two
> findings reshape this spec:
> 1. **R-5 is BROKEN.** The two targets do not "disagree about firewall". Neither runs
>    it. `ze-qemu-needs-linux-test` skips all 23 firewall tests via a mechanism the
>    skeleton did not know about. This ANSWERS the open question about why nobody has
>    seen crashes there, and it ADDS scope: the firewall suite is marked with the wrong
>    directive.
> 2. **Half this spec's ACs belong to the cache spec** and are delivered there. See its
>    "Handoff Contract". AC-6..AC-9 are dropped here.
>
> AC-1 (does 7.1.1 actually fix the nft crash) remains genuinely UNVALIDATED and
> BLOCKING. It needs a real ~30-minute build plus a QEMU run; it cannot be settled by
> reading code, and everything else is downstream of it.

## Task

The QEMU functional targets boot the stock Alpine virt kernel (6.12.13-0-virt), which
is older than the kernel ze actually targets and cannot survive ze's own firewall
tests: `make ze-qemu-all-test` SKIPS the `firewall` suite by default because it
"crashes the Alpine QEMU kernel on nft set-element-timeout operations". Run the QEMU
targets on the in-tree runtime kernel (7.1.1) instead, then stop skipping firewall.

The headline finding is that almost none of this needs inventing. The 7.x kernel, the
builder, the staging path and the QEMU `--kernel` boot path all already exist and are
already used by three other targets. What is missing is wiring plus a story for the
build cost, which is the documented reason the main targets stayed on stock. Whether
7.1.1 actually fixes the nft crash is UNVERIFIED and is AC-1.

The cache work this implies (the kernel must not live in `tmp/`, and a version bump
must reclaim what it supersedes) is NOT owned here. It is
`plan/spec-fixit-qemu-artifact-cache.md`, which covers the kernel, the Alpine ISO and
its extracted initramfs together, because they are one artifact class behind one
mechanism. That spec should land FIRST: it is independently useful (it fixes a
confirmed stale-initramfs bug), and it gives this spec a kernel that survives a
`tmp/` wipe.

## Origin

Found during `spec-fixit-migrate-sleeps-infra` work, 2026-07-16, while recording why
the firewall suite never runs under QEMU (`plan/spec-fixit-firewall-concurrency-deadlock.md`
R-5/R-6). User direction the same day: "Alpine kernel is too old for our features, we
need to find a way to run a 7.+ kernel."

## Required Reading

### Source (read before designing)
- [ ] `mk/test-integration.mk` lines 211-213 - the reason for the default skip:
      "ZE_QEMU_SKIP_SUITES (default: web,firewall) lets you drop suites: web needs
      agent-browser; firewall crashes the Alpine QEMU kernel on nft
      set-element-timeout operations."
- [ ] `mk/test-integration.mk` lines 436-441 - the explicit cost/benefit already made
      for a sibling target, and the source of the stock kernel version: "the stock
      Alpine 6.12.13-0-virt kernel"; staying on stock "keeps this target runnable
      without a ~30-minute `make ze-kernel` build first".
      -> Constraint: the ~30-minute build is the real blocker, not capability.
- [ ] `mk/gokrazy.mk` lines 194-200 - `ze-kernel`: builds via `make -C gokrazy/kernel`
      and stages to `tmp/kernel/vmlinuz`. Its own echo scopes the staging to the l2tp
      and pppoe labs.
- [ ] `mk/gokrazy.mk` lines 202-205 - `ze-kernel` also needs `KERNEL_MODULE_VERSION`
      and `KERNEL_MODCACHE_DIR`, i.e. a prior `make ze-gokrazy-deps`. A cost the QEMU
      targets do not pay today.
- [ ] `scripts/evidence/qemu-run.py` lines 505-509 - the `--kernel` flag already
      exists: "Path to custom kernel (e.g. tmp/kernel/vmlinuz for gokrazy kernel with
      PPPoL2TP)".
- [ ] `scripts/evidence/qemu-run.py` lines 118-119 - `_extract_alpine_initramfs`,
      "Extract initramfs-virt from Alpine ISO (needed for custom kernel boot)". The
      custom-kernel boot reuses Alpine's initramfs, so kernel and initramfs come from
      different sources. This seam is where a module/initramfs mismatch would appear.
- [ ] `tools/kernel-builder/build.py` line 38 - the repo already refuses older
      kernels: "kernel >= 7.0 required (L2TP_NETLINK removed, serial 8250 deps
      changed)".
- [ ] `tools/kernel-builder/build.py` lines 91-92 - `resolve_profile_fragments`: the
      profile is `kernel.config` PLUS the profile fragment, which is why nftables is
      present even though `runtime.config` never mentions it.

### Architecture Docs
- [ ] `ai/rules/qemu-testing.md` - the rule that makes QEMU coverage mandatory for
      linux-only code; it currently assumes the Alpine VM.

## Current Behavior (MANDATORY)

**Source files (cite file:line). Producers read, not inferred from callers.**

> -> CITATION DRIFT NOTE (2026-07-17, readiness pass): since the 2026-07-16 research
> two files grew and their cited line numbers moved. The cited BEHAVIOR is unchanged
> and was re-verified at the producer; only the anchors drifted. Corrected anchors:
> `internal/test/runner/record_parse.go` -- the `ZE_QEMU_LINUX_ONLY` skip producer is
> now at :235-236 (cited :227-228); the `skip-os` producer at :373-389 (cited
> :375-381); the `needs-linux` producer at :391-405 (cited :383-397).
> `scripts/evidence/qemu-run.py` -- `_extract_alpine_initramfs` is now at :130 (cited
> :118-119); `qemu_args(...)` at :153 (cited :141); the `-kernel`/`-initrd` append
> block at :199-209 (cited :187-192, :199-210); the `--kernel` CLI flag at :579 (cited
> :505-509). The commit that moved qemu-run.py is `d86d3fd29` ("key the Alpine extract
> dir to its ISO"), which also LANDED part of the `spec-fixit-qemu-artifact-cache`
> dependency (the confirmed stale-initramfs fix, now `_extract_dir_for` at :118-127);
> the durable-kernel-cache half of that spec is still OPEN. All other citations
> re-verified unchanged: `internal/appliance/cache.go:110-120`
> (`kernelCacheVariantFor`), `internal/appliance/kernel.version` (7.1.1), firewall
> `.ci` counts (0/23 needs-linux, 21/23 skip-os:value=darwin,
> `test/firewall/flush-crash.ci:11`), `mk/test-integration.mk`, `mk/gokrazy.mk`,
> `gokrazy/kernel/kernel.config:40,55,57`, `gokrazy/kernel/runtime.config:88,89`,
> `gokrazy/kernel/Makefile:28`, `tools/kernel-builder/build.py:38,91`,
> `scripts/evidence/qemu-all-tests.sh:40,89-97`, `ai/rules/qemu-testing.md`.

**How the firewall suite is actually skipped (researched 2026-07-16):**
- [ ] `internal/test/runner/record_parse.go` lines 227-228 - the producer of the
      `ZE_QEMU_LINUX_ONLY` skip: `if r.SkipReason == "" && !r.NeedsLinux &&
      os.Getenv("ZE_QEMU_LINUX_ONLY") == "1"` sets `SkipReason = "ZE_QEMU_LINUX_ONLY
      (not option=needs-linux)"`. Every test NOT marked `needs-linux` is skipped in
      that mode.
- [ ] `internal/test/runner/record_parse.go` lines 383-397 - the `needs-linux` option
      producer: sets `r.NeedsLinux = true`, and on a non-linux host sets a SkipReason.
      Inside the VM (GOOS=linux) it is inert.
- [ ] `internal/test/runner/record_parse.go` lines 375-381 - the `skip-os` producer:
      skips only when `runtime.GOOS` matches the listed value. It does NOT set
      `NeedsLinux`.
- [ ] `test/firewall/*.ci` - measured 2026-07-16: **0 of 23** carry
      `option=needs-linux`; **21 of 23** carry `option=skip-os:value=darwin` (e.g.
      `test/firewall/flush-crash.ci:11`).
      -> Decision: therefore `ze-qemu-needs-linux-test` (`ZE_QEMU_LINUX_ONLY=1`,
      `mk/test-integration.mk:261`) skips EVERY firewall test, even though its
      `ZE_QEMU_SKIP_SUITES="web"` does not skip the firewall SUITE. The suite is
      entered and every test inside it reports SKIP. R-5's "the two targets disagree"
      is FALSE: neither target has ever run a firewall test in the VM. That is why
      nobody has seen the crash from that target.
      -> Constraint: dropping `firewall` from the skip lists therefore changes
      `ze-qemu-all-test` ONLY. Getting firewall into the needs-linux loop is a
      SEPARATE change: 21 `.ci` files must move from `skip-os:value=darwin` to
      `option=needs-linux`, which `ai/rules/qemu-testing.md:61-64` already mandates
      ("Do NOT use `skip-os:value=darwin` as a substitute for `needs-linux`"). The
      firewall suite violates that rule today.
- [ ] `scripts/evidence/qemu-all-tests.sh` lines 89-97 - `fsuite()` is the producer of
      the suite-level skip: a `case ",$SKIP_SUITES," in *",$name,"*` match prints
      "SKIPPED (ZE_QEMU_SKIP_SUITES)" and returns 0. Suite granularity, not test
      granularity.

**The kernel and the arch:**
- [ ] `internal/appliance/kernel.version` - contains `7.1.1`. The 7.x kernel the user
      asked for already exists in-tree.
- [ ] `mk/gokrazy.mk` line 37 and line 177 - `GOKRAZY_ARCH ?= amd64`, `KERNEL_ARCH ?=
      $(GOKRAZY_ARCH)`, while `mk/test-integration.mk` line 216 derives `QEMU_GOARCH`
      from `uname -m` (arm64 on Apple Silicon).
      -> Constraint: a bare `make ze-kernel` on Apple Silicon stages an **amd64**
      vmlinuz to `tmp/kernel/vmlinuz` (`mk/gokrazy.mk:200`, arch-unkeyed), the `test
      -f` guard accepts it (`mk/test-integration.mk:410`), and the VM fails to boot.
      R-6 is a live bug, not a hypothetical. Any target this spec wires MUST derive
      `KERNEL_ARCH` from `QEMU_GOARCH`, never rely on the operator passing
      `GOKRAZY_ARCH=arm64` by hand as today's error messages instruct.
- [ ] `gokrazy/kernel/kernel.config` lines 40, 55, 57 - `CONFIG_NF_TABLES=y`,
      `CONFIG_NF_TABLES_IPV4=y`, `CONFIG_NF_TABLES_IPV6=y`. The runtime kernel does
      support nftables. (`runtime.config` alone does not mention NF_TABLES; the base
      fragment carries it.)
- [ ] `scripts/evidence/qemu-run.py` lines 29-31 - `ALPINE_VERSION = "3.21"`,
      `ALPINE_MINOR = "3"`; the ISO is downloaded per arch.
- [ ] `scripts/evidence/qemu-run.py` lines 141 and 187-192 - `qemu_args(iso, root,
      kernel=None)` appends `-kernel <path>` only when a kernel is supplied.
- [ ] `mk/test-integration.mk` lines 220 and 239 - `ZE_QEMU_SKIP_SUITES ?=
      web,firewall`, passed into `qemu-all-tests.sh`. No `--kernel`, so
      `ze-qemu-all-test` boots stock Alpine.
- [ ] `mk/test-integration.mk` line 261 - `ze-qemu-needs-linux-test` hardcodes
      `ZE_QEMU_SKIP_SUITES="web"` so it DOES run firewall, also with no `--kernel`.
      The two targets therefore disagree about whether firewall is safe on stock.
- [ ] `mk/test-integration.mk` lines 413, 427, 459 - three targets already pass
      `--kernel tmp/kernel/vmlinuz` (l2tp-ppp, pppoe-accel, traffic-usage), each
      guarded by a `test -f tmp/kernel/vmlinuz` precondition.
- [ ] `scripts/evidence/qemu-all-tests.sh` line 40 - the script's own default
      `SKIP_SUITES="${ZE_QEMU_SKIP_SUITES:-web,firewall}"` agrees with the make
      default.

**Behavior to preserve:**
- `ze-qemu-all-test` must stay runnable without a manual multi-step setup, or people
  will stop running it.
- The three existing `--kernel` labs keep working.
- `test/parse/cli-show-version.ci` still passes: the QEMU ze build deliberately omits
  version ldflags (`mk/test-integration.mk:223-227`).
- Suites that pass on stock today must not regress on the runtime kernel.

**Behavior to change:**
- `ze-qemu-all-test` and `ze-qemu-needs-linux-test` boot the runtime kernel.
- `firewall` leaves the default skip list.

## Problem / Evidence

**CONFIRMED.** The stock Alpine kernel cannot run ze's firewall suite. The default
skip exists precisely because "firewall crashes the Alpine QEMU kernel on nft
set-element-timeout operations" (`mk/test-integration.mk:211-213`). The suite is
therefore unproven under QEMU with the default target, while
`ze-qemu-needs-linux-test` runs it anyway (`:261`) against the same crashing kernel.

**CONFIRMED: the kernel gap is real.** Stock is 6.12.13-0-virt
(`mk/test-integration.mk:438`); ze targets 7.1.1
(`internal/appliance/kernel.version`), and the builder refuses anything below 7.0
(`tools/kernel-builder/build.py:38`) because L2TP_NETLINK was removed and the serial
8250 dependencies changed. So the VM runs a kernel ze itself declares unsupported.

**CONFIRMED: the mechanism already exists end to end.** `qemu-run.py` takes
`--kernel` (`:507`), passes `-kernel` (`:187-192`) and extracts Alpine's
initramfs-virt for custom-kernel boot (`:118-119`). `make ze-kernel` builds and stages
`tmp/kernel/vmlinuz` (`mk/gokrazy.mk:194-200`). Three targets already use it
(`mk/test-integration.mk:413, 427, 459`). Nothing here needs inventing.

**CONFIRMED: cost is the blocker, and it is already written down.** The VRRP target's
comment states the tradeoff explicitly: staying on stock "keeps this target runnable
without a ~30-minute `make ze-kernel` build first, matching the isis-frr/ldp-frr
labs" (`:441`). `ze-kernel` additionally requires a prior `make ze-gokrazy-deps`
(`mk/gokrazy.mk:202-205`). A naive `--kernel` addition to `ze-qemu-all-test` imposes
that ~30 minutes plus a dependency step on every full QEMU run.

**UNVERIFIED, and this is AC-1.** That 7.1.1 actually fixes the nft
set-element-timeout crash. Nobody has run the firewall suite on the runtime kernel.
It is plausible (a 6.12-era nft set-element-timeout bug fixed by 7.x) but unproven,
and it is equally possible the crash is a QEMU/Alpine-userland interaction that
survives the kernel swap.

**UNVERIFIED.** Whether the runtime kernel boots the full suite at all. It is built
for the appliance, not for an Alpine live system: `_extract_alpine_initramfs` pairs a
ze kernel with an Alpine initramfs, and the suite needs virtio, filesystems and the
modules the Alpine userland expects. The three labs that use it run narrow workloads,
not the full suite.

## Data Flow

### Entry Point
An agent or operator runs `make ze-qemu-all-test` or `make ze-qemu-needs-linux-test`.

### Transformation Path
Today: make cross-compiles linux binaries -> `qemu-run.py` downloads the Alpine virt
ISO -> boots it as a live system with its stock 6.12.13 kernel -> `qemu-all-tests.sh`
runs the suites minus `ZE_QEMU_SKIP_SUITES`.
Candidate: make ensures `tmp/kernel/vmlinuz` (built or cached) -> `qemu-run.py
--kernel tmp/kernel/vmlinuz` -> QEMU boots the 7.1.1 kernel with Alpine's extracted
initramfs -> the same suites run, with `firewall` no longer skipped.

### Boundaries Crossed
| Boundary | Crossing | Consequence of divergence |
|----------|----------|---------------------------|
| ze kernel -> Alpine initramfs/userland | Custom-kernel boot pairs `tmp/kernel/vmlinuz` with Alpine's extracted initramfs-virt (`qemu-run.py:118-119`) | Missing modules or a version mismatch: the VM fails to boot, or boots without the drivers the suite needs |
| Kernel build -> QEMU run | `make ze-kernel` stages `tmp/kernel/vmlinuz`; the targets consume it | An absent or stale kernel: today's labs hard-fail with a `test -f` guard, which is the pattern to reuse |
| `make ze-gokrazy-deps` -> `ze-kernel` | Kernel packaging needs the pinned modcache (`mk/gokrazy.mk:202-205`) | A full QEMU run inherits a dependency step it does not have today |
| Host arch -> VM arch | ISO arch and `KERNEL_ARCH` are chosen independently (`qemu-run.py:31`, `mk/gokrazy.mk:195`) | A kernel/ISO arch mismatch fails to boot; the labs pass `GOKRAZY_ARCH=arm64` by hand |

### Integration Points
`mk/test-integration.mk` (the QEMU targets), `mk/gokrazy.mk` (`ze-kernel`),
`scripts/evidence/qemu-run.py` (`--kernel`), `scripts/evidence/qemu-all-tests.sh`
(suite list and skips), `ai/rules/qemu-testing.md` (the rule that sends agents here).

## Wiring Test

Note: `.ci` functional tests are N/A as the CHANGE surface here. This is test
infrastructure: make targets and a QEMU harness, which have no `.ci` of their own.
The proof is that the EXISTING `.ci` suites (firewall above all) run and pass inside
the VM on the new kernel, which the table below asserts.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-qemu-all-test` | -> | `--kernel` wiring in `mk/test-integration.mk:229-239` (CANDIDATE) | `test_qemu_all_test_passes_kernel` |
| `make ze-qemu-all-test` | -> | `qemu-all-tests.sh` firewall suite, no longer skipped | the `firewall` `.ci` suite runs green in the VM (AC-2) |
| `make ze-qemu-needs-linux-test` | -> | `--kernel` wiring at `mk/test-integration.mk:251-261` (CANDIDATE) | `test_needs_linux_passes_kernel` |
| A missing `tmp/kernel/vmlinuz` | -> | the `test -f` guard pattern from `:410`/`:424`/`:456` (CANDIDATE) | `test_missing_kernel_fails_loudly` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `test_qemu_all_test_passes_kernel` (CANDIDATE) | new, alongside the make-target checks | AC-3: the target invokes `qemu-run.py` with `--kernel`, so a silent regression to stock is caught |
| `test_firewall_not_in_default_skips` (CANDIDATE) | new | AC-2: `firewall` is gone from `ZE_QEMU_SKIP_SUITES` defaults in BOTH `mk/test-integration.mk:220` and `qemu-all-tests.sh:40`. Two defaults, one assertion each: fixing only the make one leaves the script default `web,firewall` in force for any direct invocation |
| `test_firewall_ci_are_needs_linux` (CANDIDATE) | new | AC-10: every `test/firewall/*.ci` carries `option=needs-linux`, none carries `skip-os:value=darwin` as its kernel guard. This is the test that would have caught today's silent gap, and it is cheap: a grep-level assertion over 23 files |
| `test_missing_kernel_fails_loudly` (CANDIDATE) | new | AC-4: an absent `tmp/kernel/vmlinuz` produces the actionable error the labs already emit, never a silent stock-kernel fallback |
| `test_kernel_arch_matches_vm_arch` (CANDIDATE) | new | AC-11: the staged kernel's arch matches `QEMU_GOARCH`. `test -f` cannot see this (R-6); the assertion needs the arch, e.g. from the cache key or a `file`-style probe |

### Functional Tests
The existing suites ARE the functional test: the `firewall` `.ci` suite must run and
pass inside the VM on the runtime kernel (AC-1/AC-2). No new `.ci` is written by this
work; the change is the harness those `.ci` files run on.

## Files to Modify

- [ ] `mk/test-integration.mk` - pass `--kernel` at the two targets (`:229-239`,
      `:251-261`), drop `firewall` from `ZE_QEMU_SKIP_SUITES` (`:220`), add the kernel
      precondition, derive `KERNEL_ARCH` from `QEMU_GOARCH` (`:216`) for AC-11
- [ ] `scripts/evidence/qemu-all-tests.sh` - the script-side default skip at line 40
      (`SKIP_SUITES="${ZE_QEMU_SKIP_SUITES:-web,firewall}"`). Both defaults must move
      together or the script's default silently re-adds the skip
- [ ] `test/firewall/*.ci` - **NEW SCOPE from research**: 21 files move
      `option=skip-os:value=darwin` -> `option=needs-linux` (AC-10). Per-file, not a
      blind sed (R-7). Without this, `ze-qemu-needs-linux-test` keeps skipping all 23
- [ ] `ai/rules/qemu-testing.md` - it assumes the Alpine VM (`:184-195`), documents the
      stock-vs-custom kernel decision (`:176-181`) and names the ~30-minute build as
      the reason to avoid `--kernel`. That calculus changes once the kernel is cached;
      update it with the cache spec's outcome
- [ ] ~~`mk/gokrazy.mk` - `ze-kernel` staging/caching if the cost story needs it~~
      OWNED BY `spec-fixit-qemu-artifact-cache` (handoff contract row 2). Do not edit
      it here: two specs editing `ze-kernel` is exactly the rework the dependency
      exists to prevent
- [ ] ~~`plan/spec-fixit-firewall-concurrency-deadlock.md` - R-5/R-6 are resolved or
      re-scoped by this work~~ **DO NOT EDIT. A concurrent agent owns that spec**
      (2026-07-16). The interaction is real but must be handled by agreement, not by
      two agents writing the same file. See "Interaction with the firewall spec" below

## Implementation Steps

0. BLOCKING PREREQUISITE: `spec-fixit-qemu-artifact-cache` lands. Its handoff contract
   delivers rows 1-6 (durable arch- and config-keyed kernel, an ensure-kernel target,
   the loud guard). Starting here first means building the cost story twice.
1. BLOCKING: build the runtime kernel for the HOST's arch (`GOKRAZY_ARCH=arm64` on
   Apple Silicon, per R-6) and run the firewall suite on it by hand. Answer AC-1 before
   designing anything further. If 7.1.1 does not fix the crash, the premise collapses
   and steps 3-6 are wrong. Capture the built `.config`'s nft set/timeout symbols while
   here: that also closes A-3.
   -> Constraint: this step cannot be short-cut by reading code. It is a real ~30-minute
   build plus a VM boot. Every later step is downstream of its result.
2. Prove the FULL suite boots and passes on the runtime kernel, not just firewall.
   Watch the kernel/initramfs seam: `qemu_args` pairs a ze kernel with Alpine's
   extracted initramfs (`qemu-run.py:199-210`), so a module mismatch shows up here.
3. ~~Decide the cost story.~~ Consumed from the cache spec (contract row 2).
4. Wire `--kernel` into both targets, with the guard from contract row 6 and
   `KERNEL_ARCH` derived from `QEMU_GOARCH` (AC-11).
5. Remove `firewall` from both default skip lists (`mk/test-integration.mk:220`,
   `qemu-all-tests.sh:40`). This fixes `ze-qemu-all-test` only.
6. NEW: re-mark the 21 firewall `.ci` from `skip-os:value=darwin` to
   `option=needs-linux` (AC-10), per file, so `ze-qemu-needs-linux-test` stops
   silently skipping the suite. Verify natively first: both directives skip on darwin
   (`record_parse.go:375-397`), so `make ze-verify` must stay green.
7. Update `ai/rules/qemu-testing.md`. Report the R-5/R-6 outcome to the firewall spec's
   owner; do not edit that file.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The firewall `.ci` suite runs on the 7.1.1 runtime kernel under QEMU | No kernel crash on nft set-element-timeout operations |
| AC-2 | `make ze-qemu-all-test` with no overrides | The firewall suite runs and passes; `firewall` is not in the default skips |
| AC-3 | `make ze-qemu-all-test` / `ze-qemu-needs-linux-test` | Both boot the runtime kernel, not stock Alpine |
| AC-4 | `tmp/kernel/vmlinuz` is absent | The target fails with an actionable message; it NEVER silently falls back to stock, which would restore the crash quietly. Reuses the cache spec's guard (contract row 6) |
| AC-5 | The suites that pass on stock today | Still pass on the runtime kernel; no suite regresses |
| ~~AC-6~~ | ~~The kernel cost is paid once and cached; survives a `tmp/` wipe~~ | **MOVED to `spec-fixit-qemu-artifact-cache` AC-1** (handoff contract rows 2 and 5). Consumed here, not built here |
| ~~AC-7~~ | ~~A kernel version/config bump reclaims the superseded kernel~~ | **MOVED to the cache spec, AC-4** (contract row 5) |
| ~~AC-8~~ | ~~An Alpine version bump reclaims the superseded ISO~~ | **MOVED to the cache spec, AC-4** (contract row 5) |
| ~~AC-9~~ | ~~A config-fragment change MISSES the kernel cache~~ | **MOVED to the cache spec, contract row 4.** Already implemented there: `kernelCacheVariantFor` (`internal/appliance/cache.go:110-120`) hashes every resolved fragment + manifest + builder script. The cache spec's job is to route the make path through it |
| AC-10 | `make ze-qemu-needs-linux-test` after this work | The firewall tests actually RUN, not SKIP. Requires the 21 `.ci` to carry `option=needs-linux` (`record_parse.go:227-228`, `:383-397`), not `skip-os:value=darwin`. Without this, the target reports green while running zero firewall tests, which is the status quo and looks identical to success |
| AC-11 | `make ze-kernel` with no `GOKRAZY_ARCH` on an arm64 host, then a QEMU target | Either the correct arm64 kernel is used, or the target fails loudly. NEVER an amd64 vmlinuz silently accepted by `test -f` and then failing at boot (R-6, `mk/gokrazy.mk:37,177,200`) |
| AC-12 | `test/parse/cli-show-version.ci` after the change | Still passes. The QEMU ze build deliberately omits version ldflags (`mk/test-integration.mk:223-227`); the kernel swap must not tempt anyone to restore them |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | 7.1.1 fixes the nft set-element-timeout crash | The skip names the Alpine kernel as the crashing component (`mk/test-integration.mk:211-213`), and ze targets 7.1.1 | The premise collapses: the crash may be QEMU or Alpine userland, not the kernel version. Re-scope to diagnosing the crash itself | Step 1: run the firewall suite on the runtime kernel | unvalidated |
| A-2 | The runtime kernel boots an Alpine live system well enough for the full suite | Three labs already boot it under QEMU with Alpine's initramfs | The kernel needs extra config (virtio, fs, modules) or a matching initramfs, which is a much bigger job | Step 2: run the full suite | unvalidated |
| A-3 | The runtime kernel has the nftables surface the firewall suite needs | `kernel.config:40,55,57` enable NF_TABLES + IPV4 + IPV6 | Config fragments must be extended; note `runtime.config` alone lacks NF_TABLES, so fragment resolution matters (`build.py:91-92`) | Diff the built `.config` against what the firewall `.ci` files exercise | **partially confirmed 2026-07-16**: re-read at the producer. `gokrazy/kernel/kernel.config:40,55,57` = `CONFIG_NF_TABLES=y`, `CONFIG_NF_TABLES_IPV4=y`, `CONFIG_NF_TABLES_IPV6=y`, and `kernel.config` IS included for the runtime profile (`resolve_profile_fragments`, `build.py:91-99`, requires `kernel.config` + `<profile>.config`). `runtime.config` itself carries only `CONFIG_IP_NF_NAT`/`CONFIG_IP_NF_TARGET_MASQUERADE` (`:88-89`). Still unvalidated: whether the SET/TIMEOUT surface the crash involves (`nft set element timeout`) is present. That needs the built `.config`, which needs the build. Fold into step 1 |
| A-5 | The firewall suite currently runs somewhere under QEMU | The skeleton's R-5 assumed `ze-qemu-needs-linux-test` runs it (`:261` skips only `web`) | If it runs nowhere, "stop skipping firewall" is a bigger change than removing a word from two lists, and the suite's `.ci` markers are wrong | Grep the `.ci` for `option=needs-linux`; read the runner's skip producer | **BROKEN 2026-07-16**: 0 of 23 firewall `.ci` carry `needs-linux`; 21 carry `skip-os:value=darwin`. `record_parse.go:227-228` skips every non-`needs-linux` test under `ZE_QEMU_LINUX_ONLY=1`. The firewall suite runs in NEITHER QEMU target today. Adds AC-10 and the 21-file re-marking to scope |
| A-6 | Removing `firewall` from the two default skip lists is sufficient to run it by default | The skeleton's AC-2/step 5 | Insufficient: it fixes `ze-qemu-all-test` only | Trace both targets to the producer | **broken 2026-07-16**, per A-5. Sufficient for `ze-qemu-all-test` (suite-level `fsuite`, `qemu-all-tests.sh:89-97`); a no-op for `ze-qemu-needs-linux-test`, which needs the `.ci` re-marking |
| A-4 | The ~30-minute build can be amortised | CONFIRMED for a single checkout: `gokrazy/kernel/Makefile` already declares `$(OUT)/vmlinuz` with the config fragments, patches and builder scripts as prerequisites, so make skips an unchanged rebuild; `mk/test-integration.mk:421-422` relies on exactly that. The ~30 minutes is a first build, not a per-run cost | Only the SCOPE is unsolved: `.gitignore:12` ignores `tmp/*`, so a fresh clone and every `.claude/worktrees/` worktree rebuilds | Confirmed by reading the Makefile prerequisites; the remaining work is choosing a cache scope | confirmed (scope open) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A-1 is wrong and 7.1.1 still crashes | Step 1 crashes the same way | STOP. Re-scope to diagnosing the crash. Do NOT keep the suite skipped silently: if it cannot run, say so in `plan/known-failures/` with the evidence |
| R-2 | The kernel build makes the full QEMU target so slow nobody runs it | The target grows a ~30-minute prelude | This is the reason stock was chosen (`:441`). Cache/prebuild the artifact; consider a checked-in or CI-published kernel; keep a documented stock-kernel escape hatch for suites that do not need 7.x |
| R-3 | A silent fallback to stock when the kernel is missing | The suite passes suspiciously fast, or the crash returns | AC-4: hard-fail, mirroring the `test -f` guard the labs already use |
| R-4 | The kernel/initramfs seam breaks obscurely (VM boots, subtle driver differences) | Unrelated suites fail only under QEMU | Bisect per suite; AC-5 requires no regressions |
| R-5 | ~~The two QEMU targets keep disagreeing about firewall~~ **BROKEN 2026-07-16: they do not disagree, neither runs it** | ~~One skips it, the other runs it~~ | Superseded. `ze-qemu-needs-linux-test` sets `ZE_QEMU_LINUX_ONLY=1` (`:261`) and `record_parse.go:227-228` skips every non-`needs-linux` test; 0 of 23 firewall `.ci` are marked `needs-linux`. The real risk is the INVERSE: removing `firewall` from the skip lists looks like it fixes both targets but only fixes `ze-qemu-all-test`, leaving a silent gap that looks closed. Mitigation: AC-10 |
| R-6 | Kernel/ISO arch mismatch on the two host arches | Boot fails on one arch only | **CONFIRMED live, not hypothetical**: `GOKRAZY_ARCH ?= amd64` (`mk/gokrazy.mk:37`) + arch-unkeyed staging (`:200`) + existence-only guard (`mk/test-integration.mk:410`) means a bare `make ze-kernel` on Apple Silicon stages an unbootable amd64 kernel and the guard passes. Derive `KERNEL_ARCH` from `QEMU_GOARCH` (`:216`); the cache spec's arch-keyed variant (`cache.go:110-120`) makes the mismatch a cache MISS instead of a boot failure |
| R-7 | The 21 `.ci` re-marking (`skip-os` -> `needs-linux`) changes native behavior | Suites that were silent on darwin start reporting SKIP with a different reason, or a `.ci` that genuinely needs `skip-os` for a non-kernel reason gets mis-marked | Both directives skip on darwin, so native results should be unchanged in substance (`record_parse.go:375-397`); verify per file rather than sed-ing all 21. Any file whose darwin skip is NOT about the kernel keeps `skip-os` |

## Open Questions (research before design)

- Does 7.1.1 actually fix the crash? Everything else is downstream of this. Answer it
  first, by hand, before any design.
  -> RESOLUTION (2026-07-17, readiness pass): this is an EMPIRICAL gate, not a design
  question, and a readiness pass cannot settle it -- it needs the real ~30-minute build
  plus QEMU run that Implementation Step 1 mandates. No answer is recorded anywhere
  in-tree, so AC-1 stays UNVALIDATED and the wiring stays CANDIDATE. Conservative
  default already baked in: the spec does NOT assume 7.1.1 fixes the crash -- it makes
  the real run Step 1, R-1 is the fail-closed STOP path if it does not, and AC-4/R-3
  forbid any silent fallback to stock. Readiness meaning: `ready` = "specified enough
  for a fresh implementer to start with ZERO questions," whose first two mandatory
  actions are Step 0 (the `spec-fixit-qemu-artifact-cache` dependency lands) and Step 1
  (this run). It does NOT assert AC-1 is proven. Thomas: this is the one item a
  readiness pass structurally cannot close for you.
- Build cost, caching and eviction are NOT open questions here: they are owned by
  `plan/spec-fixit-qemu-artifact-cache.md`, which covers the kernel, the Alpine ISO
  and its extracted initramfs as one artifact class. It records the user constraints
  (2026-07-16): `tmp/` must stay deletable so the kernel cannot live there; a version
  bump must reclaim what it supersedes; the Go caches deliberately STAY in `tmp/`
  because they grow and are cleared often. Kernel updates land every few WEEKS (user
  correction, 2026-07-16), which makes both the caching and the eviction recurring
  concerns rather than one-offs.

  Only one caching point is kernel-specific and it belongs here: the cache KEY must
  hash the config inputs (`kernel.config` + `runtime.config` + `*.require` +
  `patches/`), not just `internal/appliance/kernel.version`. Keying on the version
  alone means a `runtime.config` edit silently reuses a stale kernel, which is exactly
  the failure the current make prerequisites already prevent
  (`gokrazy/kernel/Makefile`). A cache that loses that property is worse than no
  cache: a config change would appear to work while testing the old kernel. AC-9
  holds it.
- Should `ze-qemu-all-test` require `make ze-gokrazy-deps` (`mk/gokrazy.mk:202-205`)?
  That is a new dependency for a target that currently needs only QEMU.
  -> AUTONOMOUS DEFAULT (2026-07-17): NO new manual step. The cache spec's ensure
  target (Handoff Contract row 2, `ze-kernel-ensure`) encapsulates the modcache
  prerequisite: `ze-qemu-all-test` gains a single make prerequisite on that target,
  which internally satisfies `make ze-gokrazy-deps` on the build path (cache miss) and
  is a no-op on a hit. The operator still runs one command. Rationale: preserves
  "`ze-qemu-all-test` must stay runnable without a manual multi-step setup"
  (Current Behavior / Behavior to preserve); the deps step is a cache-build concern
  owned by row 2, not re-exposed here. Thomas: override if wrong.
- Does the runtime kernel need config additions to host the full suite (virtio, fs,
  modules the Alpine userland expects), and does adding them bloat the appliance
  kernel? The runtime profile is shared with the shipped appliance, so test-only
  additions may warrant a separate profile rather than growing `runtime.config`.
  -> AUTONOMOUS DEFAULT (2026-07-17): EMPIRICAL-FIRST, then FAIL SAFE. Add nothing
  speculatively: Step 2 must first prove whether the runtime kernel boots the full
  suite as-is. The three existing labs already boot it under QEMU with Alpine's
  extracted initramfs (A-2 basis), so the base config may already suffice. IF Step 2
  finds a gap, the default is a SEPARATE test-only profile fragment
  (`resolve_profile_fragments`, `tools/kernel-builder/build.py:91-99` resolves
  `kernel.config` + `<profile>.config`; today only `runtime.config` exists and the
  Makefile hardcodes `PROFILE := runtime`), selected by the QEMU target via a `PROFILE`
  override -- NOT growing `runtime.config`. Rationale: `runtime.config` ships in the
  appliance, so growing it for a test need mutates the shipped artifact; a test-only
  fragment leaves the shipped kernel byte-for-byte unchanged (fail-safe: never change
  the shipped image for a test). The nftables/firewall surface under test lives in the
  SHARED `kernel.config`, so a test profile still exercises the real 7.x firewall
  behavior and does not weaken AC-3 ("7.x, not stock"). Provisional -- no fragment is
  added unless Step 2 proves it necessary, and any addition that is genuinely
  appliance-appropriate (e.g. virtio) may instead go in `runtime.config` at Thomas's
  call. Thomas: override if wrong.
- Should the stock-kernel path be retired entirely, or kept for suites that do not
  need 7.x (the isis-frr/ldp-frr/vrrp labs deliberately use it for speed)?
  -> AUTONOMOUS DEFAULT (2026-07-17): KEEP the stock-kernel path; do NOT retire it.
  The isis-frr/ldp-frr/vrrp labs deliberately boot stock for speed (no ~30-minute
  build) and probed clean on it (`mk/test-integration.mk:436-441`,
  `ai/rules/qemu-testing.md:175-181`); retiring it would impose the kernel build on
  targets that do not need 7.x. Only the two targets this spec names
  (`ze-qemu-all-test`, `ze-qemu-needs-linux-test`) move to `--kernel`. Rationale:
  smaller, reversible, non-regressing; matches R-2's "keep a documented stock-kernel
  escape hatch." Thomas: override if wrong.
- ~~Is `ze-qemu-needs-linux-test` running the firewall suite against the crashing stock
  kernel today, and if so why has nobody seen the failures?~~ **ANSWERED 2026-07-16:
  it is not running them at all.** `ZE_QEMU_LINUX_ONLY=1` (`mk/test-integration.mk:261`)
  makes `record_parse.go:227-228` skip every test without `option=needs-linux`, and 0
  of 23 firewall `.ci` carry it (21 use `option=skip-os:value=darwin`). The suite is
  entered; every test inside reports SKIP. Nobody saw failures because nothing ran.
  Neither the third possibility ("crashes unreported") nor the second ("needs a trigger
  only some tests hit") was the answer. Consequences: R-5 broken, A-5/A-6 broken,
  AC-10 added, 21 `.ci` files added to scope.

## Interaction with `plan/spec-fixit-firewall-concurrency-deadlock.md`

**That spec is owned by a concurrent agent (2026-07-16). This spec does NOT edit it.**
Recorded here so the two can be reconciled by their owners rather than by a race.

| What | Detail |
|------|--------|
| The link | That spec's R-5/R-6 record WHY the firewall suite never runs under QEMU. This spec proposes to make it run |
| What this research changes for it | Its premise, if it says the suite runs under `ze-qemu-needs-linux-test`, is wrong: `record_parse.go:227-228` + 0/23 `needs-linux` markers mean it runs in NEITHER target. Any conclusion drawn from "the needs-linux target exercises firewall" needs re-checking |
| The ordering hazard | This spec re-marks 21 `test/firewall/*.ci` (AC-10). If the firewall spec also edits those files, the two collide. Whoever lands second rebases |
| Who decides | Thomas, or the two owners by agreement. Not resolvable inside this spec |
| Not blocking | This spec's AC-1 (the ~30-minute build + real run) is independent and can proceed regardless |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] `make ze-test` green
- [ ] AC-1 answered by a real run BEFORE any wiring is designed
- [ ] No silent fallback to the stock kernel (AC-4)
- [ ] `firewall` removed from BOTH default skip lists (`mk/test-integration.mk:220`, `qemu-all-tests.sh:40`)
- [ ] `spec-fixit-qemu-artifact-cache` has landed; its handoff contract rows 1-6 are
      consumed, not reimplemented here
- [ ] The 21 firewall `.ci` carry `option=needs-linux`, so `ze-qemu-needs-linux-test`
      actually runs them (AC-10). Verified by a real run reporting PASS, not SKIP
- [ ] The kernel arch matches the VM arch without a hand-passed `GOKRAZY_ARCH` (AC-11)
- [ ] The firewall spec's owner has been told what R-5/R-6 actually are; that file was
      NOT edited by this spec
