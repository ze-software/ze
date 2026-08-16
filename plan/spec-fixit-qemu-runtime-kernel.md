# spec-fixit-qemu-runtime-kernel

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-fixit-qemu-artifact-cache (land that first: durable kernel cache) (superseded, removed without landing; absorbed by spec-relocate-scratch-and-cache, learned 1173 -- durable cache now live at mk/gokrazy.mk) |
| Phase | implementation |
| Updated | 2026-08-07 |

> **AC-1 IS ANSWERED, 2026-08-07. The premise holds, and a second defect sat behind it.**
>
> **The runtime kernel does not crash.** Built for arm64 and booted under QEMU
> (`Linux localhost 7.1.4 #1 SMP PREEMPT aarch64`), the whole firewall suite ran
> to completion with no panic, including `009-set-element-timeout`, the operation
> the stock Alpine kernel is documented to die on. A direct probe settles the
> other half of A-3:
> `nft add set ip probe4 s { type ipv4_addr; flags timeout; timeout 10s; }` and
> `nft add element ip probe4 s { 1.2.3.4 timeout 5s }` both succeed, the element
> reads back as `1.2.3.4 timeout 5s expires 5s`, and the VM survives a following
> `nft flush ruleset`. R-1 did not fire, so steps 4-7 stand.
>
> **7.1.4 carries no separate Kconfig symbol for the set or timeout surface.**
> `CONFIG_NFT_RBTREE`, `CONFIG_NFT_PAYLOAD`, `CONFIG_NFT_META`,
> `CONFIG_NFT_COUNTER` and `CONFIG_NFT_OBJREF` are absent from the resolved
> `.config` entirely, not merely unset, because upstream folded them into
> `nf_tables`. Set element timeouts arrive with `CONFIG_NF_TABLES=y` and nothing
> else needs asking for.
>
> **The first run was still 3/23, and not for the reason this spec expected.**
> 20 tests failed on
> `firewallnft: flush: conn.Receive: netlink receive: operation not supported`.
> The runtime kernel builds no `CONFIG_NF_TABLES_INET`, so `nft add table inet`
> answers `Not supported` -- while `coppTable`
> (`internal/plugins/copp/translate.go`) makes an `inet` table unconditionally and
> `ze-firewall-conf.yang` offers `inet` as a family an operator can select.
> **That is a defect in the SHIPPED appliance kernel, not in the test**: an
> appliance that runs control-plane policing cannot start its firewall, and the
> daemon exits. Fixed at the source, see "The kernel config defect" below. After
> the fix: **23/23 PASS, 100%, 28.4s.**

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
      without a ~30-minute `make ze-kernel-build` build first".
      -> Constraint: the ~30-minute build is the real blocker, not capability.
- [ ] `mk/gokrazy.mk` lines 194-200 - `ze-kernel-build`: builds via `make -C gokrazy/kernel`
      and stages to `tmp/kernel/vmlinuz`. Its own echo scopes the staging to the l2tp
      and pppoe labs.
- [ ] `mk/gokrazy.mk` lines 202-205 - `ze-kernel-build` also needs `KERNEL_MODULE_VERSION`
      and `KERNEL_MODCACHE_DIR`, i.e. a prior `make ze-gokrazy-deps-download`. A cost the QEMU
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
- [ ] `ai/rules/platform-linux.md` - the rule that makes QEMU coverage mandatory for
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
> re-verified unchanged: `internal/appliance/cache.go`
> (`kernelCacheVariantFor`), `internal/appliance/kernel.version` (7.1.1), firewall
> `.ci` counts (0/23 needs-linux, 21/23 skip-os:value=darwin,
> `test/firewall/flush-crash.ci`), `mk/test-integration.mk`, `mk/gokrazy.mk`,
> `gokrazy/kernel/kernel.config:40,55,57`, `gokrazy/kernel/runtime.config:88,89`,
> `gokrazy/kernel/Makefile:28`, `tools/kernel-builder/build.py,91`,
> `scripts/evidence/qemu-all-tests.sh,89-97`, `ai/rules/platform-linux.md`.

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
      `test/firewall/flush-crash.ci`).
      -> Decision: therefore `ze-qemu-needs-linux-test` (`ZE_QEMU_LINUX_ONLY=1`,
      `mk/test-integration.mk`) skips EVERY firewall test, even though its
      `ZE_QEMU_SKIP_SUITES="web"` does not skip the firewall SUITE. The suite is
      entered and every test inside it reports SKIP. R-5's "the two targets disagree"
      is FALSE: neither target has ever run a firewall test in the VM. That is why
      nobody has seen the crash from that target.
      -> Constraint: dropping `firewall` from the skip lists therefore changes
      `ze-qemu-all-test` ONLY. Getting firewall into the needs-linux loop is a
      SEPARATE change: 21 `.ci` files must move from `skip-os:value=darwin` to
      `option=needs-linux`, which `ai/rules/platform-linux.md` already mandates
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
      -> Constraint: a bare `make ze-kernel-build` on Apple Silicon stages an **amd64**
      vmlinuz to `tmp/kernel/vmlinuz` (`mk/gokrazy.mk`, arch-unkeyed), the `test
      -f` guard accepts it (`mk/test-integration.mk`), and the VM fails to boot.
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
  version ldflags (`mk/test-integration.mk`).
- Suites that pass on stock today must not regress on the runtime kernel.

**Behavior to change:**
- `ze-qemu-all-test` and `ze-qemu-needs-linux-test` boot the runtime kernel.
- `firewall` leaves the default skip list.

## Problem / Evidence

**CONFIRMED.** The stock Alpine kernel cannot run ze's firewall suite. The default
skip exists precisely because "firewall crashes the Alpine QEMU kernel on nft
set-element-timeout operations" (`mk/test-integration.mk`). The suite is
therefore unproven under QEMU with the default target, while
`ze-qemu-needs-linux-test` runs it anyway against the same crashing kernel.

**CONFIRMED: the kernel gap is real.** Stock is 6.12.13-0-virt
(`mk/test-integration.mk`); ze targets 7.1.1
(`internal/appliance/kernel.version`), and the builder refuses anything below 7.0
(`tools/kernel-builder/build.py`) because L2TP_NETLINK was removed and the serial
8250 dependencies changed. So the VM runs a kernel ze itself declares unsupported.

**CONFIRMED: the mechanism already exists end to end.** `qemu-run.py` takes
`--kernel`, passes `-kernel` and extracts Alpine's
initramfs-virt for custom-kernel boot. `make ze-kernel-build` builds and stages
`tmp/kernel/vmlinuz` (`mk/gokrazy.mk`). Three targets already use it
(`mk/test-integration.mk, 427, 459`). Nothing here needs inventing.

**CONFIRMED: cost is the blocker, and it is already written down.** The VRRP target's
comment states the tradeoff explicitly: staying on stock "keeps this target runnable
without a ~30-minute `make ze-kernel-build` build first, matching the isis-frr/ldp-frr
labs" (`:441`). `ze-kernel-build` additionally requires a prior `make ze-gokrazy-deps-download`
(`mk/gokrazy.mk`). A naive `--kernel` addition to `ze-qemu-all-test` imposes
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
| ze kernel -> Alpine initramfs/userland | Custom-kernel boot pairs `tmp/kernel/vmlinuz` with Alpine's extracted initramfs-virt (`qemu-run.py`) | Missing modules or a version mismatch: the VM fails to boot, or boots without the drivers the suite needs |
| Kernel build -> QEMU run | `make ze-kernel-build` stages `tmp/kernel/vmlinuz`; the targets consume it | An absent or stale kernel: today's labs hard-fail with a `test -f` guard, which is the pattern to reuse |
| `make ze-gokrazy-deps-download` -> `ze-kernel-build` | Kernel packaging needs the pinned modcache (`mk/gokrazy.mk`) | A full QEMU run inherits a dependency step it does not have today |
| Host arch -> VM arch | ISO arch and `KERNEL_ARCH` are chosen independently (`qemu-run.py`, `mk/gokrazy.mk`) | A kernel/ISO arch mismatch fails to boot; the labs pass `GOKRAZY_ARCH=arm64` by hand |

### Integration Points
`mk/test-integration.mk` (the QEMU targets), `mk/gokrazy.mk` (`ze-kernel-build`),
`scripts/evidence/qemu-run.py` (`--kernel`), `scripts/evidence/qemu-all-tests.sh`
(suite list and skips), `ai/rules/platform-linux.md` (the rule that sends agents here).

## Wiring Test

Note: `.ci` functional tests are N/A as the CHANGE surface here. This is test
infrastructure: make targets and a QEMU harness, which have no `.ci` of their own.
The proof is that the EXISTING `.ci` suites (firewall above all) run and pass inside
the VM on the new kernel, which the table below asserts.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-qemu-all-test` | -> | `--kernel` wiring in `mk/test-integration.mk` (CANDIDATE) | `test_qemu_all_test_passes_kernel` |
| `make ze-qemu-all-test` | -> | `qemu-all-tests.sh` firewall suite, no longer skipped | the `firewall` `.ci` suite runs green in the VM (AC-2) |
| `make ze-qemu-needs-linux-test` | -> | `--kernel` wiring at `mk/test-integration.mk` (CANDIDATE) | `test_needs_linux_passes_kernel` |
| A missing `tmp/kernel/vmlinuz` | -> | the `test -f` guard pattern from `:410`/`:424`/`:456` (CANDIDATE) | `test_missing_kernel_fails_loudly` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `test_qemu_all_test_passes_kernel` (CANDIDATE) | new, alongside the make-target checks | AC-3: the target invokes `qemu-run.py` with `--kernel`, so a silent regression to stock is caught |
| `test_firewall_not_in_default_skips` (CANDIDATE) | new | AC-2: `firewall` is gone from `ZE_QEMU_SKIP_SUITES` defaults in BOTH `mk/test-integration.mk` and `qemu-all-tests.sh`. Two defaults, one assertion each: fixing only the make one leaves the script default `web,firewall` in force for any direct invocation |
| `test_firewall_ci_are_needs_linux` (CANDIDATE) | new | AC-10: every `test/firewall/*.ci` carries `option=needs-linux`, none carries `skip-os:value=darwin` as its kernel guard. This is the test that would have caught today's silent gap, and it is cheap: a grep-level assertion over 23 files |
| `test_missing_kernel_fails_loudly` (CANDIDATE) | new | AC-4: an absent `tmp/kernel/vmlinuz` produces the actionable error the labs already emit, never a silent stock-kernel fallback |
| `test_kernel_arch_matches_vm_arch` (CANDIDATE) | new | AC-11: the staged kernel's arch matches `QEMU_GOARCH`. `test -f` cannot see this (R-6); the assertion needs the arch, e.g. from the cache key or a `file`-style probe |

### Functional Tests
The existing suites ARE the functional test: the `firewall` `.ci` suite must run and
pass inside the VM on the runtime kernel (AC-1/AC-2). No new `.ci` is written by this
work; the change is the harness those `.ci` files run on.

## Files to Modify

- [ ] `mk/test-integration.mk` - pass `--kernel` at the two targets (`:229-239`,
      `:251-261`), drop `firewall` from `ZE_QEMU_SKIP_SUITES`, add the kernel
      precondition, derive `KERNEL_ARCH` from `QEMU_GOARCH` for AC-11
- [ ] `scripts/evidence/qemu-all-tests.sh` - the script-side default skip at line 40
      (`SKIP_SUITES="${ZE_QEMU_SKIP_SUITES:-web,firewall}"`). Both defaults must move
      together or the script's default silently re-adds the skip
- [x] `test/firewall/*.ci` - **NEW SCOPE from research**: 21 files move
      `option=skip-os:value=darwin` -> `option=needs-linux` (AC-10). Per-file, not a
      blind sed (R-7). Without this, `ze-qemu-needs-linux-test` keeps skipping all 23
      -> **ALREADY LANDED, no edit needed here.** Commit `6c27ebd37` ("fix(test):
      mark net-admin tests, build QEMU ze with its features") did it between the
      research and the implementation. Measured 2026-08-07: 0 of 23 carry
      `skip-os:value=darwin`, and 21 of 23 carry `option=needs-linux:caps=net-admin`.
      The other 2 correctly carry no marker at all: `006-dscp-ipv6-rejected.ci` runs
      `ze config validate` offline and `command-owner-firewall-root.ci` runs
      `ze firewall help`. Neither starts a daemon, so neither touches the kernel, and
      R-7's "any file whose darwin skip is NOT about the kernel keeps skip-os" is
      satisfied by them needing no skip in the first place. The split is EXACT in both
      directions and is now a test: every `.ci` containing `cmd=background:` carries
      `needs-linux`, and no `.ci` without one does
- [x] `gokrazy/kernel/kernel.config`, `gokrazy/kernel/kernel.require` -
      **NEW SCOPE from step 1**, not anticipated by the research. The runtime kernel
      could not host ze's own firewall. See "The kernel config defect"
- [x] `scripts/evidence/qemu_kernel_wiring_test.go` - NEW. The TDD plan's five
      CANDIDATE unit tests, landed as four (the fifth, `test_kernel_arch_matches_vm_arch`,
      merged into the guard test because one guard answers both questions)
- [ ] `ai/rules/platform-linux.md` - it assumes the Alpine VM, documents the
      stock-vs-custom kernel decision and names the ~30-minute build as
      the reason to avoid `--kernel`. That calculus changes once the kernel is cached;
      update it with the cache spec's outcome
- [ ] ~~`mk/gokrazy.mk` - `ze-kernel-build` staging/caching if the cost story needs it~~
      OWNED BY `spec-fixit-qemu-artifact-cache` (handoff contract row 2). Do not edit
      it here: two specs editing `ze-kernel-build` is exactly the rework the dependency
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
   extracted initramfs (`qemu-run.py`), so a module mismatch shows up here.
3. ~~Decide the cost story.~~ Consumed from the cache spec (contract row 2).
4. Wire `--kernel` into both targets, with the guard from contract row 6 and
   `KERNEL_ARCH` derived from `QEMU_GOARCH` (AC-11).
5. Remove `firewall` from both default skip lists (`mk/test-integration.mk`,
   `qemu-all-tests.sh`). This fixes `ze-qemu-all-test` only.
6. NEW: re-mark the 21 firewall `.ci` from `skip-os:value=darwin` to
   `option=needs-linux` (AC-10), per file, so `ze-qemu-needs-linux-test` stops
   silently skipping the suite. Verify natively first: both directives skip on darwin
   (`record_parse.go`), so `make ze-precommit-verify` must stay green.
7. Update `ai/rules/platform-linux.md`. Report the R-5/R-6 outcome to the firewall spec's
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
| ~~AC-9~~ | ~~A config-fragment change MISSES the kernel cache~~ | **MOVED to the cache spec, contract row 4.** Already implemented there: `kernelCacheVariantFor` (`internal/appliance/cache.go`) hashes every resolved fragment + manifest + builder script. The cache spec's job is to route the make path through it |
| AC-10 | `make ze-qemu-needs-linux-test` after this work | The firewall tests actually RUN, not SKIP. Requires the 21 `.ci` to carry `option=needs-linux` (`record_parse.go`, `:383-397`), not `skip-os:value=darwin`. Without this, the target reports green while running zero firewall tests, which is the status quo and looks identical to success |
| AC-11 | `make ze-kernel-build` with no `GOKRAZY_ARCH` on an arm64 host, then a QEMU target | Either the correct arm64 kernel is used, or the target fails loudly. NEVER an amd64 vmlinuz silently accepted by `test -f` and then failing at boot (R-6, `mk/gokrazy.mk,177,200`) |
| AC-12 | `test/parse/cli-show-version.ci` after the change | Still passes. The QEMU ze build deliberately omits version ldflags (`mk/test-integration.mk`); the kernel swap must not tempt anyone to restore them |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | 7.1.1 fixes the nft set-element-timeout crash | The skip names the Alpine kernel as the crashing component (`mk/test-integration.mk`), and ze targets 7.1.1 | The premise collapses: the crash may be QEMU or Alpine userland, not the kernel version. Re-scope to diagnosing the crash itself | Step 1: run the firewall suite on the runtime kernel | **confirmed 2026-08-07**, on 7.1.4 (`internal/appliance/kernel.version` moved on since the research). No panic, and the set-element-timeout operations succeed under direct probe. R-1 did not fire |
| A-2 | The runtime kernel boots an Alpine live system well enough for the full suite | Three labs already boot it under QEMU with Alpine's initramfs | The kernel needs extra config (virtio, fs, modules) or a matching initramfs, which is a much bigger job | Step 2: run the full suite | **confirmed 2026-08-07**. The VM boots (`Linux localhost 7.1.4 ... aarch64`), mounts the 9p share, installs packages and runs the suites. The seam has one real consequence, now documented in `ai/rules/platform-linux.md`: Alpine's `/lib/modules` is built for 6.12.13-0-virt, so NO module of the ze kernel can load and every symbol a QEMU run needs must be `=y` |
| A-3 | The runtime kernel has the nftables surface the firewall suite needs | `kernel.config:40,55,57` enable NF_TABLES + IPV4 + IPV6 | Config fragments must be extended; note `runtime.config` alone lacks NF_TABLES, so fragment resolution matters (`build.py`) | Diff the built `.config` against what the firewall `.ci` files exercise | **partially confirmed 2026-07-16**: re-read at the producer. `gokrazy/kernel/kernel.config:40,55,57` = `CONFIG_NF_TABLES=y`, `CONFIG_NF_TABLES_IPV4=y`, `CONFIG_NF_TABLES_IPV6=y`, and `kernel.config` IS included for the runtime profile (`resolve_profile_fragments`, `build.py`, requires `kernel.config` + `<profile>.config`). `runtime.config` itself carries only `CONFIG_IP_NF_NAT`/`CONFIG_IP_NF_TARGET_MASQUERADE`. Still unvalidated: whether the SET/TIMEOUT surface the crash involves (`nft set element timeout`) is present. That needs the built `.config`, which needs the build. Fold into step 1. **CONFIRMED 2026-08-07 by the build plus a direct probe**: the surface has no Kconfig symbol of its own in 7.1.4 (the old `NFT_SET_*` and `NFT_RBTREE` symbols are absent from the resolved `.config`, folded into `nf_tables`), it arrives with `CONFIG_NF_TABLES=y`, and in the VM a set with `flags timeout` plus an element with `timeout 5s` are both accepted and read back with an expiry. What the fragment DID lack was `CONFIG_NF_TABLES_INET`, which the assumption never asked about, and which broke 20 of 23 tests. See "The kernel config defect" |
| A-5 | The firewall suite currently runs somewhere under QEMU | The skeleton's R-5 assumed `ze-qemu-needs-linux-test` runs it (`:261` skips only `web`) | If it runs nowhere, "stop skipping firewall" is a bigger change than removing a word from two lists, and the suite's `.ci` markers are wrong | Grep the `.ci` for `option=needs-linux`; read the runner's skip producer | **BROKEN 2026-07-16**: 0 of 23 firewall `.ci` carry `needs-linux`; 21 carry `skip-os:value=darwin`. `record_parse.go` skips every non-`needs-linux` test under `ZE_QEMU_LINUX_ONLY=1`. The firewall suite runs in NEITHER QEMU target today. Adds AC-10 and the 21-file re-marking to scope |
| A-6 | Removing `firewall` from the two default skip lists is sufficient to run it by default | The skeleton's AC-2/step 5 | Insufficient: it fixes `ze-qemu-all-test` only | Trace both targets to the producer | **broken 2026-07-16**, per A-5. Sufficient for `ze-qemu-all-test` (suite-level `fsuite`, `qemu-all-tests.sh`); a no-op for `ze-qemu-needs-linux-test`, which needs the `.ci` re-marking |
| A-4 | The ~30-minute build can be amortised | CONFIRMED for a single checkout: `gokrazy/kernel/Makefile` already declares `$(OUT)/vmlinuz` with the config fragments, patches and builder scripts as prerequisites, so make skips an unchanged rebuild; `mk/test-integration.mk` relies on exactly that. The ~30 minutes is a first build, not a per-run cost | Only the SCOPE is unsolved: `.gitignore:12` ignores `tmp/*`, so a fresh clone and every `.claude/worktrees/` worktree rebuilds | Confirmed by reading the Makefile prerequisites; the remaining work is choosing a cache scope | confirmed (scope open) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A-1 is wrong and 7.1.1 still crashes | Step 1 crashes the same way | STOP. Re-scope to diagnosing the crash. Do NOT keep the suite skipped silently: if it cannot run, say so in `plan/known-failures/` with the evidence |
| R-2 | The kernel build makes the full QEMU target so slow nobody runs it | The target grows a ~30-minute prelude | This is the reason stock was chosen. Cache/prebuild the artifact; consider a checked-in or CI-published kernel; keep a documented stock-kernel escape hatch for suites that do not need 7.x |
| R-3 | A silent fallback to stock when the kernel is missing | The suite passes suspiciously fast, or the crash returns | AC-4: hard-fail, mirroring the `test -f` guard the labs already use |
| R-4 | The kernel/initramfs seam breaks obscurely (VM boots, subtle driver differences) | Unrelated suites fail only under QEMU | Bisect per suite; AC-5 requires no regressions |
| R-5 | ~~The two QEMU targets keep disagreeing about firewall~~ **BROKEN 2026-07-16: they do not disagree, neither runs it** | ~~One skips it, the other runs it~~ | Superseded. `ze-qemu-needs-linux-test` sets `ZE_QEMU_LINUX_ONLY=1` and `record_parse.go` skips every non-`needs-linux` test; 0 of 23 firewall `.ci` are marked `needs-linux`. The real risk is the INVERSE: removing `firewall` from the skip lists looks like it fixes both targets but only fixes `ze-qemu-all-test`, leaving a silent gap that looks closed. Mitigation: AC-10 |
| R-6 | Kernel/ISO arch mismatch on the two host arches | Boot fails on one arch only | **FIXED 2026-08-07** by `ze-qemu-kernel-guard` (`mk/test-integration.mk`), which keys on `QEMU_GOARCH` and compares the staged kernel against the architecture-keyed cache entry. Proven with a real Alpine x86_64 `vmlinuz-virt` staged on this arm64 host: the target exits 2 naming the architecture. Original note kept below. **CONFIRMED live, not hypothetical**: `GOKRAZY_ARCH ?= amd64` (`mk/gokrazy.mk`) + arch-unkeyed staging + existence-only guard (`mk/test-integration.mk`) means a bare `make ze-kernel-build` on Apple Silicon stages an unbootable amd64 kernel and the guard passes. Derive `KERNEL_ARCH` from `QEMU_GOARCH`; the cache spec's arch-keyed variant (`cache.go`) makes the mismatch a cache MISS instead of a boot failure |
| R-7 | The 21 `.ci` re-marking (`skip-os` -> `needs-linux`) changes native behavior | Suites that were silent on darwin start reporting SKIP with a different reason, or a `.ci` that genuinely needs `skip-os` for a non-kernel reason gets mis-marked | Both directives skip on darwin, so native results should be unchanged in substance (`record_parse.go`); verify per file rather than sed-ing all 21. Any file whose darwin skip is NOT about the kernel keeps `skip-os` |

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
- Should `ze-qemu-all-test` require `make ze-gokrazy-deps-download` (`mk/gokrazy.mk`)?
  That is a new dependency for a target that currently needs only QEMU.
  -> AUTONOMOUS DEFAULT (2026-07-17): NO new manual step. The cache spec's ensure
  target (Handoff Contract row 2, `ze-kernel-ensure`) encapsulates the modcache
  prerequisite: `ze-qemu-all-test` gains a single make prerequisite on that target,
  which internally satisfies `make ze-gokrazy-deps-download` on the build path (cache miss) and
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
  (`resolve_profile_fragments`, `tools/kernel-builder/build.py` resolves
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
  build) and probed clean on it (`mk/test-integration.mk`,
  `ai/rules/platform-linux.md`); retiring it would impose the kernel build on
  targets that do not need 7.x. Only the two targets this spec names
  (`ze-qemu-all-test`, `ze-qemu-needs-linux-test`) move to `--kernel`. Rationale:
  smaller, reversible, non-regressing; matches R-2's "keep a documented stock-kernel
  escape hatch." Thomas: override if wrong.
- ~~Is `ze-qemu-needs-linux-test` running the firewall suite against the crashing stock
  kernel today, and if so why has nobody seen the failures?~~ **ANSWERED 2026-07-16:
  it is not running them at all.** `ZE_QEMU_LINUX_ONLY=1` (`mk/test-integration.mk`)
  makes `record_parse.go` skip every test without `option=needs-linux`, and 0
  of 23 firewall `.ci` carry it (21 use `option=skip-os:value=darwin`). The suite is
  entered; every test inside reports SKIP. Nobody saw failures because nothing ran.
  Neither the third possibility ("crashes unreported") nor the second ("needs a trigger
  only some tests hit") was the answer. Consequences: R-5 broken, A-5/A-6 broken,
  AC-10 added, 21 `.ci` files added to scope.

## The kernel config defect (found and fixed 2026-08-07)

Step 1 was meant to answer one question and it exposed a second, larger one. The
crash is gone; ze's own kernel could not run ze's own firewall.

> **How the "before" state is evidenced, stated plainly.** The pre-fix kernel no
> longer exists on any host: the `.config` fragments are hashed into the cache key
> (`kernelCacheVariantFor`, `internal/appliance/cache.go`), so editing them
> retired that key and its build. **The claim rests on the in-VM MEASUREMENT
> recorded below, not on a `.config` a later reader can regenerate**: `nft add
> table inet probe` answered `Not supported`, and 20 of 23 firewall tests failed
> on `netlink receive: operation not supported`. Anyone re-deriving this must
> rebuild at the old fragment content; recomputing today's hash will not show it.

| Fact | Evidence |
|------|----------|
| The runtime kernel refuses the `inet` family | Measured in the VM before the fix: `nft add table inet probe` -> `Error: Could not process rule: Not supported`, and that build's resolved `.config` carried `# CONFIG_NF_TABLES_INET is not set` |
| `gokrazy/kernel/kernel.config` had an enumeration hole | It asked for `CONFIG_NF_TABLES`, `CONFIG_NF_TABLES_IPV4` and `CONFIG_NF_TABLES_IPV6`, and never for `CONFIG_NF_TABLES_INET`. In 7.1.4 that symbol is a plain `bool` with no default, so nothing selected it |
| ze needs it in production, not only in tests | `coppTable` (`internal/plugins/copp/translate.go`) sets `Family: firewall.FamilyInet` unconditionally, `lowerFamily` (`internal/plugins/firewall/nft/lower_linux.go`) maps it to `nftables.TableFamilyINet`, and `Apply` fails at `Flush` (`internal/plugins/firewall/nft/backend_linux.go`). `ze-firewall-conf.yang` also offers `inet` as an operator-selectable family. 15 of the 23 firewall `.ci` declare `family inet`, 2 declare `family ip`, and the rest configure no table |
| Four nft expressions were silently modular | The fragment asked for `CONFIG_NFT_CT/NAT/MASQ/REDIR=y` and kconfig answered `=m`, because `NF_CONNTRACK` and `NF_NAT` were themselves `=m`. A module cannot load in the QEMU VM at all: `--kernel` pairs ze's kernel with Alpine's initramfs and Alpine's `/lib/modules`, built for 6.12.13-0-virt |
| Nothing could report either | `enforce_required_symbols` (`tools/kernel-builder/build.py`) checks only the symbols listed in a `.require` manifest, and `gokrazy/kernel/kernel.require` listed no netfilter symbol at all |

**The fix is at the owning layer, the shared fragment, not a test-only profile.**
The spec's earlier autonomous default preferred a separate test profile so a test
need would never change the shipped image. That default does not apply here: the
shipped image is what is broken. A test-only profile would have proved a kernel
nobody ships.

- `gokrazy/kernel/kernel.config` gains `CONFIG_NF_TABLES_INET=y`, plus
  `CONFIG_NF_CONNTRACK=y` and `CONFIG_NF_NAT=y` so the four expressions above stop
  being demoted.
- `gokrazy/kernel/kernel.require` gains the nine symbols ze's firewall cannot work
  without, so a future demotion fails the BUILD, naming the symbol, instead of
  failing a daemon at startup on an appliance.

Every dependency was read in the 7.1.4 source before the edit, not assumed:
`NF_TABLES_INET` is `bool`, `depends on IPV6` (`CONFIG_IPV6=y` here) and selects
the two IP tables; `NF_NAT` and the four expressions are tristate and depend on
`NF_CONNTRACK`. The rebuild then passed the `.require` gate, which is the
machine confirming all nine resolved to `=y`.

**Left, and deliberately not fixed here** (separable: no firewall `.ci` reaches
it, so the goal holds without it). `CONFIG_NF_TABLES_NETDEV` is still unset while
`ze-firewall-conf.yang` offers a `netdev` family and `ingress`/`egress` hooks. The
same class of hole, one family over. It needs its own change with its own test,
and it belongs to whoever owns the firewall model.

### Every guard converted, and the pins reconciled (review follow-up)

Two more instances of this spec's own defect were found inside this spec's own
work, by the method that found the first one. Both are fixed:

| Hole | Fix |
|------|-----|
| `runtime.require` pinned 8 of the 13 qdisc and classifier symbols `runtime.config` requests. `NET_SCH_HFSC`, `_FQ`, `_SFQ`, `_NETEM` and `_PRIO` are all TRISTATE, so a bump could demote any of them to `=m` and no enforcer would read it | All five pinned |
| `kernel.require` pinned 3 of the 4 demoted expressions its own neighbouring comment enumerates: `NFT_REDIR` was named in the comment and absent from the list | Pinned, and with it `NFT_LIMIT`, `NFT_LOG` and `NFT_REJECT` -- derived from the expressions the backend actually emits (`expr.Limit`, `expr.Log`, `expr.Reject`, ... in `internal/plugins/firewall/nft`), each verified tristate in the 7.1.4 Kconfig |

**Full reconciliation, both directions.** Symbols added to either `.config` that
are not pinned: `CONFIG_IP_ADVANCED_ROUTER` only, and deliberately -- it is a
bool menu gate that `IP_MULTIPLE_TABLES` and `IP_ROUTE_MULTIPATH` both
`depends on`, and both are pinned, so pinning the dependents is strictly
stronger. Symbols pinned but no longer requested: **none**. Every deliberate
omission is now written into the `.require` file itself with its reason, so the
next reader does not read a gap as an oversight. All 52 pins were checked against
the resolved `.config` and all 52 are satisfied; because a `.require` manifest is
a checker and never an input to kconfig, that check is exact rather than a
prediction.

**All six kernel-consuming targets now share one guard.** The three older labs
(`ze-qemu-l2tp-ppp-test`, `ze-qemu-pppoe-accel-test`, `ze-qemu-traffic-usage-test`)
kept the existence-only `test -f` that this spec's own comment calls out as the
R-6 hole, and their error hints still said `GOKRAZY_ARCH=arm64`, which is the
variable that causes the bug rather than the one that fixes it. They were one
`define` away, so they were converted rather than homed. `ze-qemu-pppoe-test` had
adopted the guard without the `ze-host-build` prerequisite the guard's first command
needs, so on a clean checkout it denied while reporting the wrong cause; it now
declares it, and the test derives the list of guard users from the file so the
next adopter is covered without editing the test.

## Interaction with `plan/spec-fixit-firewall-concurrency-deadlock.md`

**That spec is owned by a concurrent agent (2026-07-16). This spec does NOT edit it.**
Recorded here so the two can be reconciled by their owners rather than by a race.

| What | Detail |
|------|--------|
| The link | That spec's R-5/R-6 record WHY the firewall suite never runs under QEMU. This spec proposes to make it run |
| What this research changes for it | Its premise, if it says the suite runs under `ze-qemu-needs-linux-test`, is wrong: `record_parse.go` + 0/23 `needs-linux` markers mean it runs in NEITHER target. Any conclusion drawn from "the needs-linux target exercises firewall" needs re-checking |
| The ordering hazard | This spec re-marks 21 `test/firewall/*.ci` (AC-10). If the firewall spec also edits those files, the two collide. Whoever lands second rebases |
| Who decides | Thomas, or the two owners by agreement. Not resolvable inside this spec |
| Not blocking | This spec's AC-1 (the ~30-minute build + real run) is independent and can proceed regardless |

### What to tell that spec's owner (2026-08-07, this file NOT edited)

| Their item | The answer this work produced |
|------------|-------------------------------|
| R-5 "the two QEMU targets disagree about firewall" | Was already recorded BROKEN, and is now closed rather than broken. Both targets run the firewall suite: `ze-qemu-all-test` because `firewall` left both default skip lists, and `ze-qemu-needs-linux-test` because the 21 `.ci` carry `option=needs-linux` (landed by `6c27ebd37`). The "runs nowhere" state is over |
| R-6 "the deadlock may be entangled with a kernel crash" | **The kernel crash is not the explanation.** On 7.1.4 the nft set-element-timeout operations do not crash: the suite is 23/23 in 28.4s. Their Finding 2 story, that a wedged nft subsystem makes `Apply` block forever and holds `r.mu`, is no longer supported by the kernel. If the ~255s dispatch stall still reproduces, it reproduces against a healthy kernel and is a Go-level defect |
| Their repro environment | Now trustworthy. `make ze-qemu-all-test` boots ze's own kernel and refuses to start on anything else, so a run cannot silently be on the crashing stock kernel. Re-run the repro before drawing any conclusion from the older observation |
| The ordering hazard they were warned about | Gone. This spec edited no `test/firewall/*.ci`: the re-marking had already landed, and it was verified rather than redone |

## Implementation Audit (2026-08-07)

| AC | Status | Demonstrated by |
|----|--------|-----------------|
| AC-1 | Done | The firewall suite ran to completion on 7.1.4 under QEMU with no panic, `009-set-element-timeout` included. Direct probe: a set with `flags timeout` and an element with `timeout 5s` are accepted, read back as `expires 5s`, and the VM survives `nft flush ruleset` |
| AC-2 | Done | `firewall` removed from the default skip list in `mk/test-integration.mk` AND `scripts/evidence/qemu-all-tests.sh`. Suite result on the fixed kernel: `pass 23/23 100.0% 28.4s`. Guarded by `TestFirewallNotInDefaultQemuSkips` |
| AC-3 | Done | Both targets pass `--kernel $(ZE_QEMU_KERNEL)`. The VM reports `Linux localhost 7.1.4 ... aarch64`, not 6.12.13-0-virt. Guarded by `TestQemuFunctionalTargetsBootTheRuntimeKernel` |
| AC-4 | Done | `tmp/kernel/vmlinuz` moved aside, `make ze-qemu-all-test` exits 2 with `error: tmp/kernel/vmlinuz not found -- this target boots ze's runtime kernel and never stock Alpine (run: make ze-kernel-build KERNEL_ARCH=arm64)`. Restored after. Guarded by `TestQemuTargetsGuardTheStagedKernel` |
| AC-5 | **Partly** | Every regression the kernel switch caused is found, fixed and re-measured green: `policy` PASS, and the `cos-*`, `iface-*` and `forked-route-install-kernel` tests gone from the failure list. What AC-5 still lacks is a STOCK-kernel baseline for the reds that were red in both runs, so "unrelated" is argued rather than measured for `ospf` and `ddos-*`. See "Full-suite result" |
| AC-10 | Done | Landed by `6c27ebd37` before this work; verified rather than redone. 21 of 23 carry `option=needs-linux:caps=net-admin`, 0 carry `skip-os:value=darwin`, and the 2 unmarked start no daemon. Guarded by `TestFirewallCiTestsAreNeedsLinux` |
| AC-11 | Done | A REAL amd64 kernel (Alpine's own `vmlinuz-virt` for x86_64) staged at `tmp/kernel/vmlinuz` on this arm64 host: `make ze-qemu-needs-linux-test` exits 2 with `error: ... is not this tree's arm64 runtime kernel -- wrong architecture, or a kernel config fragment changed after it was staged`. The guard derives its architecture from `QEMU_GOARCH`, never `GOKRAZY_ARCH` |
| AC-12 | Done | `test/parse/cli-show-version.ci` runs in the `parse` suite of the full pass. No ldflags were added to the QEMU ze build |
| AC-6..AC-9 | Consumed | Delivered by the absorbed cache work (learned 1173). `mk/gokrazy.mk` routes `ze-kernel-build` through `~/.cache/ze`, and this spec did not edit it |

Beyond the ACs, and the reason the suite could not have gone green without it:
the appliance kernel config defect above. R-3 also has a proof of its own -- the
stock Alpine kernel staged at `tmp/kernel/vmlinuz` is REFUSED, so there is no
silent fallback path even when the stock kernel is the thing sitting there.

## Full-suite result (`make ze-qemu-all-test`, 2026-08-07, runtime kernel 7.1.4 arm64)

23 functional suites ran. **`firewall` was one of the 23 and it passed, 23/23** --
the deliverable of this spec, through the real target rather than by hand.

Scope of this record: the FUNCTIONAL phase only. The run was stopped after it, so
the in-VM unit and integration phases of `qemu-all-tests.sh` are not covered here
and must be part of the re-run.

| Verdict | Suites |
|---------|--------|
| PASS (14) | encode, decode, editor, managed, l2tp, **firewall**, appliance, ldp, rsvpte, isis, ospfv3, l2tp-wire, isis-wire, ospf-wire |
| FAIL (9) | plugin, parse, reload, ui, policy, install, ospf, vrrp, traffic |

**The reds split into two classes and only one of them is mine.**

Class 1, CAUSED BY THE KERNEL SWITCH, and therefore in scope for AC-5. The
runtime kernel is missing more symbols ze's own code programs, in exactly the
shape of the `CONFIG_NF_TABLES_INET` hole:

| Symptom | Missing symbol | Producer that needs it |
|---------|----------------|------------------------|
| `policy` suite times out | `CONFIG_IP_MULTIPLE_TABLES`, `CONFIG_IPV6_MULTIPLE_TABLES` | `applyIPRules`, `applyAutoRoutes` (`internal/plugins/policyroute/rules_linux.go`); the table id range is published in `ze-policyroute-conf.yang` |
| `traffic` suite fails | `CONFIG_NET_SCH_HTB`, `_FQ_CODEL`, `_HFSC`, `_FQ`, `_SFQ`, `_NETEM`, `_PRIO`, `CONFIG_NET_CLS_U32`, `CONFIG_NET_CLS_FW` | `translateQdisc`, `translateClass`, `translateFilter`, `u32FilterPair` (`internal/plugins/traffic/netlink/translate_linux.go`). `ze-traffic-control-conf.yang` offers every one of those qdisc types as an enum value |
| `iface-verbs`, `iface-ensure-rollback`: `create dummy` fails | `CONFIG_DUMMY` | `CreateDummy` (`internal/plugins/iface/netlink/manage_linux.go`); `list dummy` in `ze-iface-conf.yang` |
| VLAN sub-interfaces, port mirroring | `CONFIG_VLAN_8021Q`, `CONFIG_NET_SCH_INGRESS`, `CONFIG_NET_CLS_MATCHALL`, `CONFIG_NET_ACT_MIRRED` were all `=m` or unset | `CreateVLAN`, `setupIngressMirror`, `setupClsactMirror` (same package). A module is unusable in the VM at all |

All of these are now requested in `gokrazy/kernel/runtime.config` and pinned in
`gokrazy/kernel/runtime.require`. **This is the same defect as the netfilter one:
a config ze accepts and validates, that the kernel then refuses.** It is a
shipped-appliance gap, found because this spec was the first thing to run ze's
functional suites against ze's own kernel.

Class 2, NOT caused by the kernel, and not this spec's to fix:

| Red | Attribution |
|-----|-------------|
| `firewall-irr-*` (7 tests in `plugin`) | `failed to send request: usage: update firewall irr as-set <as-set>` -- a CLI argument-surface mismatch. `internal/component/firewall/plugins/irr` carries another agent's uncommitted work in this shared checkout. Nothing to do with nftables or the kernel |
| `cli-host-show-dmi` (`parse`) | NOT `CONFIG_DMI_SYSFS`. `detectDMI` (`internal/component/host/dmi_linux.go`) reads `/sys/class/dmi/id` only, which `CONFIG_DMIID=y` already provides. Cause not yet identified; it is not a symbol this spec removed |
| `ssh-cli-status-error-exit-code`, `ddos-*` | Unattributed at this point. Settled below by the re-run: both stayed red after the kernel config fix, so the switch did not cause them. `cos-dynamic-coa` and `cos-dynamic-session` were listed here in an earlier draft and are NOT unattributed: the re-run shows them green, so the tc symbols were their cause |

### Re-run after the config fix (same target, same kernel version, new symbols)

The rebuild passed the `.require` gate, which is the machine confirming every new
symbol resolved to `=y`, and the same target ran again. **Every Class 1 red went
green**, which makes the attribution a measurement rather than an argument:

| Was | Now | Read as |
|-----|-----|---------|
| `suite FAIL: policy` | **`suite PASS: policy`** | `IP_MULTIPLE_TABLES` was the cause |
| `cos-dynamic-coa`, `cos-dynamic-session` FAIL | gone from the failure list | the tc symbols were the cause |
| `iface-verbs`, `iface-ensure-rollback` FAIL | gone | `CONFIG_DUMMY` was the cause |
| `forked-route-install-kernel` FAIL | gone | multipath and the table set |
| `reload`: 2 failed + 8 timeout | 2 failed + 3 timeout | five of those timeouts were kernel-caused |
| `suite PASS: firewall` | **`suite PASS: firewall`** | holds |

### The install suite: a harness bug, found and fixed here

Five `install` tests were red in BOTH runs and the cause is neither the kernel
nor the tests. `test/install/kernel-arch-mapping-single.ci` derives the repo as
`dirname $(command -v ze)/..`, honoring `ZE_REPO_ROOT` first. In the VM `ze` is a
symlink in the PATH shim directory that `qemu-all-tests.sh` builds, so the root
resolves outside the repo, every grep path is absent, and **the check fails
having scanned no source at all** -- printing `found in:` with an EMPTY list,
which reads like a real violation.

Reproduced and fixed away from the VM, with the same shim shape:

```
### A: shim on PATH, ZE_REPO_ROOT unset (the VM condition)
repo=.../tmp/shimproof
files=[]
RESULT=FAIL

### B: same shim, ZE_REPO_ROOT set (the fix)
repo=.../ze/main
files=[tools/kernel-builder/run.py]
RESULT=OK
```

The harness broke the derivation, so the harness states the root:
`qemu-all-tests.sh` now exports `ZE_REPO_ROOT=/workspace`. That is the layer that
knows, and it fixes all five at once rather than five `.ci` files one at a time.

### Still red, and none of it caused by this work

Each was red in BOTH runs, so the kernel switch did not cause it:

| Red | Attribution |
|-----|-------------|
| `traffic`: `011-vpp-reject-hfsc`, `012-vpp-not-connected`, `020-vpp-accept-dscp-filter`, `021-cs6-priority-config`, `024-vpp-reject-prio`, `025-vpp-reject-mark`, `026-vpp-accept-multiclass` | Every one is a VPP-backend test. No VPP daemon runs in the QEMU VM, and `internal/plugins/traffic/vpp` and `internal/plugins/iface/vpp` carry another agent's uncommitted work in this shared checkout |
| `plugin`: `firewall-irr-*` (7) | `usage: update firewall irr as-set <as-set>` -- a CLI argument-surface mismatch in another agent's uncommitted `internal/component/firewall/plugins/irr` |
| `ospf` (exit 124), `vrrp-idle`, `ddos-detect-characterize`, `ddos-incident-confidence`, `ssh-cli-status-error-exit-code`, `cli-host-show-dmi`, `ze-stripped-surface`, `doctor-geodns`, `pki-reference-reload*` | Red in both runs. `cli-host-show-dmi` is NOT `CONFIG_DMI_SYSFS`: `detectDMI` (`internal/component/host/dmi_linux.go`) reads only `/sys/class/dmi/id`, which `CONFIG_DMIID=y` already provides |

**The honest gap, stated rather than papered over.** Every remaining red was red
before AND after the kernel config fix, which rules the fix out as their cause. It
does NOT prove they are green on the stock kernel, because this session produced
no stock-kernel baseline of the same suites. `ospf` timing out wholesale and the
two `ddos-*` tests are the ones where that matters, since both plausibly touch the
kernel. Before anyone calls those unrelated, run the same suites on stock and diff
the failure NAMES, not the numeric ids (an id is a position and the suite grew by
two files between the two runs recorded here).

## Goal Validation

| Goal | Evidence |
|------|----------|
| The QEMU targets run on ze's own 7.x kernel, not one ze declares unsupported | The VM prints `Linux localhost 7.1.4 #1 SMP PREEMPT ... aarch64`. Both targets fail closed without that kernel (three proofs above) |
| The firewall suite stops being skipped, and passes | `pass 23/23 100.0% 28.4s` in the VM, from a suite that ran in NEITHER target before this work |
| The silent gap cannot come back | Four unit tests, each mutation-verified: dropping `--kernel`, restoring `firewall` to either skip default, or keying the guard on `GOKRAZY_ARCH` each turns one red, and the tree is green with all four in place |
| ze's shipped appliance can run ze's own firewall | `CONFIG_NF_TABLES_INET=y` plus the two promotions, pinned by nine entries in `gokrazy/kernel/kernel.require` so the build fails rather than the daemon |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] `make ze-standard-test` green
- [ ] AC-1 answered by a real run BEFORE any wiring is designed
- [ ] No silent fallback to the stock kernel (AC-4)
- [ ] `firewall` removed from BOTH default skip lists (`mk/test-integration.mk`, `qemu-all-tests.sh`)
- [ ] `spec-fixit-qemu-artifact-cache` has landed; its handoff contract rows 1-6 are
      consumed, not reimplemented here
- [ ] The 21 firewall `.ci` carry `option=needs-linux`, so `ze-qemu-needs-linux-test`
      actually runs them (AC-10). Verified by a real run reporting PASS, not SKIP
- [ ] The kernel arch matches the VM arch without a hand-passed `GOKRAZY_ARCH` (AC-11)
- [ ] The firewall spec's owner has been told what R-5/R-6 actually are; that file was
      NOT edited by this spec
