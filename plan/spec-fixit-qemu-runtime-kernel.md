# spec-fixit-qemu-runtime-kernel

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-16 |

> **SKELETON.** Captured intent, not designed work. Every file, step and test named
> below is a CANDIDATE. Research via `/ze-spec` before implementing.

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

Scope therefore includes a durable, evicting cache for the built kernel, per user
constraints (2026-07-16): the artifact must NOT live in `tmp/`, because `tmp/` must
stay safe to delete wholesale, while a kernel costs ~30 minutes and changes only
every few months; and a `kernel.version` bump must reclaim the superseded kernel and
image rather than leak them. Today the repo violates both: it builds to
`tmp/kernel/build` and stages to `tmp/kernel/vmlinuz`, and nothing ever evicts.

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

**Source files (cite file:line):**
- [ ] `internal/appliance/kernel.version` - contains `7.1.1`. The 7.x kernel the user
      asked for already exists in-tree.
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
| `test_firewall_not_in_default_skips` (CANDIDATE) | new | AC-2: `firewall` is gone from `ZE_QEMU_SKIP_SUITES` defaults in both `mk/test-integration.mk:220` and `qemu-all-tests.sh:40` |
| `test_missing_kernel_fails_loudly` (CANDIDATE) | new | AC-4: an absent `tmp/kernel/vmlinuz` produces the actionable error the labs already emit, never a silent stock-kernel fallback |

### Functional Tests
The existing suites ARE the functional test: the `firewall` `.ci` suite must run and
pass inside the VM on the runtime kernel (AC-1/AC-2). No new `.ci` is written by this
work; the change is the harness those `.ci` files run on.

## Files to Modify

- [ ] `mk/test-integration.mk` - pass `--kernel`, drop `firewall` from the default
      skip, add the kernel precondition (CANDIDATE)
- [ ] `scripts/evidence/qemu-all-tests.sh` - the script-side default skip at line 40
      (CANDIDATE)
- [ ] `mk/gokrazy.mk` - `ze-kernel` staging/caching if the cost story needs it
      (CANDIDATE)
- [ ] `ai/rules/qemu-testing.md` - it currently assumes the Alpine VM; update once the
      VM runs ze's own kernel (CANDIDATE)
- [ ] `plan/spec-fixit-firewall-concurrency-deadlock.md` - R-5/R-6 are resolved or
      re-scoped by this work (CANDIDATE)

## Implementation Steps

1. CANDIDATE, BLOCKING: build the runtime kernel and run the firewall suite on it by
   hand. Answer AC-1 before designing anything. If 7.1.1 does not fix the crash, the
   whole premise changes and the rest of this spec is wrong.
2. CANDIDATE: prove the full suite boots and passes on the runtime kernel, not just
   the firewall suite. Watch the kernel/initramfs seam.
3. CANDIDATE: decide the cost story (see Open Questions). This is the design work.
4. CANDIDATE: wire `--kernel` into both targets with the existing `test -f` guard.
5. CANDIDATE: remove `firewall` from both default skip lists.
6. CANDIDATE: update `ai/rules/qemu-testing.md` and re-scope the firewall spec's R-5/R-6.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The firewall `.ci` suite runs on the 7.1.1 runtime kernel under QEMU | No kernel crash on nft set-element-timeout operations |
| AC-2 | `make ze-qemu-all-test` with no overrides | The firewall suite runs and passes; `firewall` is not in the default skips |
| AC-3 | `make ze-qemu-all-test` / `ze-qemu-needs-linux-test` | Both boot the runtime kernel, not stock Alpine |
| AC-4 | `tmp/kernel/vmlinuz` is absent | The target fails with an actionable message; it NEVER silently falls back to stock, which would restore the crash quietly |
| AC-5 | The suites that pass on stock today | Still pass on the runtime kernel; no suite regresses |
| AC-6 | A developer runs the full QEMU target | The kernel cost is paid once and cached, not on every run. The cache survives a full `tmp/` wipe (user constraint: `tmp/` must stay deletable) |
| AC-7 | `internal/appliance/kernel.version` is bumped, or a config fragment changes | The superseded kernel is reclaimed, not left behind. The cache does not grow by ~50MB per bump forever |
| AC-8 | `ALPINE_VERSION` / `ALPINE_MINOR` is bumped (`qemu-run.py:29-30`) | The superseded ISO is reclaimed too. Same artifact class, same GC, since `ensure_iso` names the file per version (`:101`) and today never removes the old one |
| AC-9 | A cached kernel exists but a config fragment changed since it was built | The cache MISSES and rebuilds. A stale hit is worse than no cache: it would test the old kernel while appearing to test the new config |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | 7.1.1 fixes the nft set-element-timeout crash | The skip names the Alpine kernel as the crashing component (`mk/test-integration.mk:211-213`), and ze targets 7.1.1 | The premise collapses: the crash may be QEMU or Alpine userland, not the kernel version. Re-scope to diagnosing the crash itself | Step 1: run the firewall suite on the runtime kernel | unvalidated |
| A-2 | The runtime kernel boots an Alpine live system well enough for the full suite | Three labs already boot it under QEMU with Alpine's initramfs | The kernel needs extra config (virtio, fs, modules) or a matching initramfs, which is a much bigger job | Step 2: run the full suite | unvalidated |
| A-3 | The runtime kernel has the nftables surface the firewall suite needs | `kernel.config:40,55,57` enable NF_TABLES + IPV4 + IPV6 | Config fragments must be extended; note `runtime.config` alone lacks NF_TABLES, so fragment resolution matters (`build.py:91-92`) | Diff the built `.config` against what the firewall `.ci` files exercise | unvalidated |
| A-4 | The ~30-minute build can be amortised | CONFIRMED for a single checkout: `gokrazy/kernel/Makefile` already declares `$(OUT)/vmlinuz` with the config fragments, patches and builder scripts as prerequisites, so make skips an unchanged rebuild; `mk/test-integration.mk:421-422` relies on exactly that. The ~30 minutes is a first build, not a per-run cost | Only the SCOPE is unsolved: `.gitignore:12` ignores `tmp/*`, so a fresh clone and every `.claude/worktrees/` worktree rebuilds | Confirmed by reading the Makefile prerequisites; the remaining work is choosing a cache scope | confirmed (scope open) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A-1 is wrong and 7.1.1 still crashes | Step 1 crashes the same way | STOP. Re-scope to diagnosing the crash. Do NOT keep the suite skipped silently: if it cannot run, say so in `plan/known-failures.md` with the evidence |
| R-2 | The kernel build makes the full QEMU target so slow nobody runs it | The target grows a ~30-minute prelude | This is the reason stock was chosen (`:441`). Cache/prebuild the artifact; consider a checked-in or CI-published kernel; keep a documented stock-kernel escape hatch for suites that do not need 7.x |
| R-3 | A silent fallback to stock when the kernel is missing | The suite passes suspiciously fast, or the crash returns | AC-4: hard-fail, mirroring the `test -f` guard the labs already use |
| R-4 | The kernel/initramfs seam breaks obscurely (VM boots, subtle driver differences) | Unrelated suites fail only under QEMU | Bisect per suite; AC-5 requires no regressions |
| R-5 | The two QEMU targets keep disagreeing about firewall | One skips it, the other runs it, on the same kernel | Resolve both together; the disagreement today (`:220` vs `:261`) is itself a bug |
| R-6 | Kernel/ISO arch mismatch on the two host arches | Boot fails on one arch only | The labs pass `GOKRAZY_ARCH=arm64` by hand; the targets must derive it like `QEMU_GOARCH` (`mk/test-integration.mk:216`) |

## Open Questions (research before design)

- Does 7.1.1 actually fix the crash? Everything else is downstream of this. Answer it
  first, by hand, before any design.
- How is the ~30-minute build amortised? User direction 2026-07-16: "can we build and
  cache this? have a cache folder in the repository to not rebuild but be able to
  reuse?" Partly answered already, so the remaining question is narrow:

  CONFIRMED, a cache already exists and already works within one checkout.
  `gokrazy/kernel/Makefile` declares `$(OUT)/vmlinuz` with `kernel.config`,
  `kernel.require`, `runtime.config`, `runtime.require`, `patches/series`, the patch
  files and the builder scripts as prerequisites, so make already skips the rebuild
  when nothing changed. The ~30 minutes is a FIRST build, not a per-run cost
  (`mk/test-integration.mk:421-422` says it "rebuilds when runtime.config changes").
  There is also an existing in-repo cache convention: `qemu-run.py:84-92`
  (`cache_dir`) creates `tmp/qemu/{iso,go-dl,go-cache,gomodcache}`, and `ensure_iso`
  (`:99-115`) downloads the Alpine ISO once and reuses it.

  CONFIRMED, the limit is scope, not correctness. `.gitignore:12` ignores `tmp/*`, so
  the cache is per-checkout: a fresh clone rebuilds, and EVERY git worktree gets its
  own `tmp/` (this repo uses `.claude/worktrees/`), so each worktree pays the 30
  minutes again. That, not the first build, is the cost worth attacking.

  USER CONSTRAINT, 2026-07-16, and it rules out the status quo: "we should not have
  valuable cache in tmp as we must be able to delete it all, and kernel rebuild only
  happens really every few months when the kernel is updated." So `tmp/` is by
  definition disposable, and the kernel is a ~30-minute artifact with a lifetime of
  MONTHS. Storing it in the scratch tree is backwards, and that is what the repo does
  today: `gokrazy/kernel/Makefile` builds to `tmp/kernel/build` and `mk/gokrazy.mk:200`
  stages to `tmp/kernel/vmlinuz`. `rm -rf tmp/` silently costs 30 minutes.

  -> Decision: the cache location must survive a full `tmp/` wipe. Any option that
  keeps the kernel under `tmp/` is rejected regardless of its other merits.

  The same defect applies to the Alpine ISO: `qemu-run.py:84-92` caches it under
  `tmp/qemu/iso`, so a `tmp/` wipe also re-downloads it. Whatever cache root is
  chosen should probably host both, since they are the same class of artifact
  (expensive to obtain, rarely changing, keyed by a version).

  Remaining options, given that constraint:
  1. ~~Status quo (`tmp/kernel`)~~: REJECTED. Valuable artifact in the disposable tree.
  2. Repo-local cache dir OUTSIDE `tmp/` (e.g. gitignored `.cache/kernel/<key>/`):
     survives `rm -rf tmp/`, is "in the repository" as asked, but is still per-worktree
     and is still destroyed by `git clean -xdf`.
  3. User-level shared cache (e.g. `~/.cache/ze/kernel/<key>/vmlinuz`), copied or
     symlinked to whatever path the targets expect: survives a `tmp/` wipe, `git
     clean`, a fresh clone, and is shared across `.claude/worktrees/`. Most durable
     for a months-lived artifact; the tradeoff is that it lives outside the repo.
  4. Prebuilt artifact fetched on demand: an `ensure_kernel()` mirroring
     `ensure_iso()`. Precedented and it removes the 30 minutes entirely for most
     people, but it needs somewhere to publish and there is no CI today.
  5. Commit a binary kernel: NOT recommended. Tens of MB per rebuild, permanent in git
     history, and it contradicts `gokrazy/kernel/Makefile`, which deliberately keeps
     the ~400MB build under gitignored `tmp/` so artifacts "never pollute git status".

  Options 2 and 3 are not exclusive: a shared cache root with a repo-local symlink
  gets durability plus discoverability. Whichever is chosen, the build must remain
  reproducible on demand, so the cache stays an optimisation and never becomes the
  only copy of a kernel nobody can rebuild.

- Eviction. USER CONSTRAINT, 2026-07-16: "we should clear old kernel / image when we
  updated the kernel version in the repo." A durable cache that never evicts is just
  a leak with better uptime: each `kernel.version` bump strands a vmlinuz (plus the
  ~400MB build tree if that is cached too), and `ensure_iso` already names ISOs per
  version (`qemu-run.py:101`) and never deletes the old one, so Alpine bumps leak the
  same way. AC-7/AC-8 make reclamation a requirement, not a nicety.

  The open part is the policy, and it is a genuine tradeoff:
  - Keep-exactly-one (purge every key but the current one on build): simplest, matches
    the user's wording, bounds the cache at one artifact. But switching branches or
    bisecting across a `kernel.version` bump then costs a ~30-minute rebuild EACH WAY,
    which is the exact cost this spec exists to remove.
  - Keep-N (say 2) or age-based: survives branch switching and bisect, at the price of
    a bounded amount of disk.
  Prefer a policy tied to the version/config KEY rather than to wall-clock age, so the
  behaviour is predictable: an operator who bumps the kernel and reverts should not be
  punished, and one who never reverts should not accumulate. Whatever is chosen, GC
  must be safe to run concurrently: sessions and worktrees share this repo, so
  eviction must not delete a kernel another run is booting right now (the same shared
  mutable state hazard as `plan/spec-fixit-shared-plan-file-contention.md`).

  The load-bearing detail for options 2 and 3: the cache KEY must hash the config
  inputs (`kernel.config` + `runtime.config` + `*.require` + `patches/`), not just
  `internal/appliance/kernel.version`. Keying on version alone means a
  `runtime.config` edit silently reuses a stale kernel, which is precisely the
  failure the current make prerequisites prevent. A cache that loses that property is
  worse than no cache: it would let a config change appear to work while testing the
  old kernel.
- Should `ze-qemu-all-test` require `make ze-gokrazy-deps` (`mk/gokrazy.mk:202-205`)?
  That is a new dependency for a target that currently needs only QEMU.
- Does the runtime kernel need config additions to host the full suite (virtio, fs,
  modules the Alpine userland expects), and does adding them bloat the appliance
  kernel? The runtime profile is shared with the shipped appliance, so test-only
  additions may warrant a separate profile rather than growing `runtime.config`.
- Should the stock-kernel path be retired entirely, or kept for suites that do not
  need 7.x (the isis-frr/ldp-frr/vrrp labs deliberately use it for speed)?
- Is `ze-qemu-needs-linux-test` running the firewall suite against the crashing stock
  kernel today (`:261` skips only `web`) actually producing failures, and if so why
  has nobody seen them? Either it crashes and is unreported, or the crash needs a
  trigger only some tests hit.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] `make ze-test` green
- [ ] AC-1 answered by a real run BEFORE any wiring is designed
- [ ] No silent fallback to the stock kernel (AC-4)
- [ ] `firewall` removed from BOTH default skip lists (`mk/test-integration.mk:220`, `qemu-all-tests.sh:40`)
