# spec-fixit-qemu-artifact-cache

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-16 |

> **SKELETON.** Captured intent, not designed work. Every file, step and test named
> below is a CANDIDATE. Research via `/ze-spec` before implementing.

## Task

The QEMU harness stores its expensive, rarely-changing inputs in the disposable
scratch tree: `tmp/qemu` measures **6.2 GB** today, and `tmp/kernel` holds a kernel
that costs ~30 minutes to rebuild. `tmp/` must stay safe to delete wholesale, so
these artifacts are in the wrong place. Move the version-keyed, expensive ones to a
durable cache with a real key and real eviction, and fix the stale-hit bug that
already exists in the ISO path.

Three defects, one mechanism:

1. Valuable artifacts live under `tmp/` (kernel `~30 min`, Alpine ISO 76 MB download).
2. Nothing evicts: a version bump strands the old artifact forever.
3. ~~The extracted Alpine initramfs is NOT version-keyed~~ **FIXED 2026-07-16**, see
   below. Defects 1 and 2 remain and are the work here.

### Fixed already
- **The stale-extract bug is fixed.** `_extract_dir_for()` (`qemu-run.py:118-127`)
  now keys the extract directory by the ISO filename, which already carries version
  and arch, so a cached extract cannot outlive the ISO it came from. Regression test:
  `qemu-run.py --selftest`, run by `TestQemuRunSelftest`
  (`scripts/evidence/qemu_run_test.go:27`), which `make ze-unit-test` picks up because
  `go list ./...` includes the package. Against the old fixed-name path the test
  failed with "stale hit: a version bump reused the previous ISO's initramfs"; it
  passes now. AC-3 is therefore already satisfied. Keep that test when reshaping the
  cache.
- Note for the eviction work: the legacy `tmp/qemu/iso/alpine-extract/` directory
  (~77 MB) is orphaned by the rename and nothing reclaims it. That is AC-4's job, not
  the bug fix's.

## Origin

Found during `spec-fixit-migrate-sleeps-infra` work, 2026-07-16, while specifying
`plan/spec-fixit-qemu-runtime-kernel.md`. User direction the same day: "we should not
have valuable cache in tmp as we must be able to delete it all"; "we should clear old
kernel / image when we updated the kernel version in the repo"; "do the same for the
alpine iso cache". User correction, same day: kernel updates land every few WEEKS,
not months, which raises both the leak rate and the value of caching.

## Required Reading

### Source (read before designing)
- [ ] `scripts/evidence/qemu-run.py` lines 84-92 - `cache_dir()`: everything lands
      under `root/"tmp"/"qemu"` with `iso`, `go-dl`, `go-cache`, `gomodcache`
      subdirs. This is the placement decision to revisit.
- [ ] `scripts/evidence/qemu-run.py` lines 99-115 - `ensure_iso()`: the ISO filename
      IS version-keyed (`:101`), the cache hit is bare existence (`:103-104`), and the
      download is a plain `curl -fSL` with no checksum or signature (`:111`).
- [ ] `scripts/evidence/qemu-run.py` lines 118-138 - `_extract_alpine_initramfs()`:
      `extract_dir = iso.parent / "alpine-extract"` (`:120`) is a FIXED name, and the
      early return only tests existence (`:122-123`). This is the confirmed stale-hit
      bug.
      -> Constraint: any fix must key the extract to the ISO it came from.
- [ ] `scripts/evidence/qemu-run.py` lines 29-31 - `ALPINE_VERSION = "3.21"`,
      `ALPINE_MINOR = "3"`, arch derived from the host. The version knobs that must
      drive both the key and the eviction.
- [ ] `mk/gokrazy.mk` lines 194-200 - `ze-kernel` builds and stages the kernel to
      `tmp/kernel/vmlinuz`, i.e. into the disposable tree.
- [ ] `gokrazy/kernel/Makefile` line 19 - `OUT := $(REPO_ROOT)/tmp/kernel/build`, and
      its own comment explains the choice: `tmp/` is gitignored so the "~400MB
      artifacts never pollute git status". Correct about git, wrong about durability.
- [ ] `.gitignore` line 12 - `tmp/*` (with the `!tmp/go.mod` exception at `:13`),
      which is why `tmp/` is both invisible to git and freely deletable.

### Architecture Docs
- [ ] `ai/rules/qemu-testing.md` - QEMU coverage is mandatory for linux-only code, so
      this harness is on the critical path for a large slice of the test suite.

## Current Behavior (MANDATORY)

**Source files (cite file:line):**
- [ ] `scripts/evidence/qemu-run.py` lines 101-104 - the ISO is named
      `alpine-virt-{ALPINE_VERSION}.{ALPINE_MINOR}-{ALPINE_ARCH}.iso` and reused iff
      the path exists. Correctly keyed; never verified; never evicted.
- [ ] `scripts/evidence/qemu-run.py` lines 120-123 - the extract dir is
      `<iso-dir>/alpine-extract`, shared by every version, reused iff
      `boot/initramfs-virt` exists.
- [ ] `scripts/evidence/qemu-run.py` line 111 - `curl -fSL` with no integrity check;
      a mirror serving a bad file is cached indefinitely with no way to notice.
- [ ] `mk/gokrazy.mk` line 200 - `cp "$(KERNEL_BUILD_DIR)/vmlinuz" tmp/kernel/vmlinuz`.
- [ ] `mk/test-integration.mk` lines 410, 424, 456 - consumers guard with `test -f
      tmp/kernel/vmlinuz` and fail with an actionable message. The guard pattern to
      preserve.

**Measured, 2026-07-16 (this checkout, aarch64):**
- `tmp/qemu` totals 6.2 GB: `build` 3.4G, `go-cache` 1.8G, `gomodcache` 590M,
  `ccache` 251M, `iso` 153M, `go-dl` 55M.
- `tmp/qemu/iso` holds `alpine-virt-3.21.3-aarch64.iso` (76M) plus `alpine-extract/`,
  the un-keyed extract directory.

**Behavior to preserve:**
- `tmp/` stays safe to delete wholesale. That is the requirement driving this spec,
  not a nice-to-have.
- The build stays reproducible on demand: the cache is an optimisation, never the only
  copy of an artifact nobody can rebuild.
- Consumers keep failing loudly when an artifact is absent (`test -f` guards), never
  silently falling back.
- `tmp/go.mod` keeps marking `tmp/` a nested module so `go list ./...` skips these
  caches (`.gitignore:13`, `qemu-run.py:75`).

**Behavior to change:**
- Expensive, version-keyed artifacts move out of `tmp/`.
- A version bump reclaims the artifact it supersedes.
- The extracted initramfs is keyed to its ISO.
- The ISO download is verified before it is cached.

## Problem / Evidence

**CONFIRMED: valuable artifacts are in the disposable tree.** `tmp/qemu` is 6.2 GB
and `tmp/kernel` holds a ~30-minute build (`mk/gokrazy.mk:200`,
`gokrazy/kernel/Makefile:19`). `rm -rf tmp/` is supposed to be free; it currently
costs a kernel rebuild plus a 76 MB re-download. Every `.claude/worktrees/` worktree
has its own `tmp/`, so the cost is paid per worktree as well as per clone.

**CONFIRMED BUG: the extracted initramfs is not version-keyed.** `ensure_iso` names
the ISO per version (`qemu-run.py:101`), but `_extract_alpine_initramfs` extracts to
the fixed path `<iso-dir>/alpine-extract` (`:120`) and returns early when
`boot/initramfs-virt` merely exists (`:122-123`). Bumping `ALPINE_VERSION` therefore
downloads a NEW ISO and boots it with the OLD initramfs. On disk right now:
`tmp/qemu/iso/` contains one versioned ISO next to one unversioned `alpine-extract/`.
This is not hypothetical, it is the code's current behaviour; it has simply not been
triggered because the version has not moved since the extract path was added. It is
also the exact hazard the kernel spec calls out as worse than no cache: a stale hit
tests the wrong thing while appearing to work.

**CONFIRMED: nothing evicts.** Neither `ensure_iso` nor `ze-kernel` removes a
superseded artifact. With kernel updates every few weeks (user, 2026-07-16), a
~50 MB vmlinuz is stranded per bump, and Alpine bumps strand a 76 MB ISO each. The
cache grows monotonically until someone deletes `tmp/` wholesale, which is exactly
the operation that also destroys the artifacts worth keeping. The current design is
therefore the worst of both: it loses what is valuable and keeps what is garbage.

**CONFIRMED: the ISO download has no integrity check.** `qemu-run.py:111` runs
`curl -fSL` and caches whatever arrives. Alpine publishes `.sha256` and `.asc`
alongside each release. Today a corrupted or substituted image is cached indefinitely
and boots the VM that runs ze's tests, with no way to detect it short of deleting the
cache. HTTPS covers transport, not mirror content or a truncated-but-successful
write.

**DECIDED (user, 2026-07-16): the Go caches STAY in `tmp/`.** "I do still want the
go-cache in tmp as it grows too much and we often need to clear it up." So `tmp/` is
not a mistake to be emptied, it is the deliberate home for one class of cache. The
two user constraints together give the rule this spec implements:

| Class | Home | Why | Here |
|-------|------|-----|------|
| High-churn, fast-growing, cheap to regenerate | `tmp/` (stays) | Must be nukeable on demand, and often is | `go-cache` 1.8G, `gomodcache` 590M, `go-dl` 55M, `ccache` 251M, `build` 3.4G |
| Expensive to obtain, rarely changing, version-keyed | durable cache (moves) | Losing it costs ~30 min or a 76 MB download for no benefit | kernel `vmlinuz`, Alpine ISO, extracted initramfs |

The sizing is what makes this cheap: only roughly 200 MB needs to be durable (a
~50 MB vmlinuz, the 76 MB ISO, its ~77 MB extract), while ~6 GB stays disposable.
This is a small, well-bounded cache, not a second scratch tree.

`ccache` and `tmp/qemu/build` follow the Go caches by the same logic, and there is a
second reason they can stay: they are build SCRATCH, needed only while actually
rebuilding. Once the vmlinuz is durably cached, wiping them costs nothing until a
version bump forces a rebuild anyway, which is precisely when they would be cold or
invalid regardless.

**UNVERIFIED.** What writes `tmp/qemu/build` (3.4G): most likely the qemu-path kernel
builder (`BUILDER=qemu`, `tools/kernel-builder/qemu-build.py`), but confirm before
relying on the classification above.

## Data Flow

### Entry Point
Any QEMU target (`make ze-qemu-all-test`, `ze-qemu-l2tp-ppp-test`, the installer
evidence scripts) invoking `scripts/evidence/qemu-run.py`, or `make ze-kernel`
producing the kernel those targets boot.

### Transformation Path
Today: `cache_dir()` creates `tmp/qemu/...` -> `ensure_iso()` downloads the ISO if
the path is absent -> `_extract_alpine_initramfs()` extracts to a fixed
`alpine-extract/` if `initramfs-virt` is absent -> QEMU boots. In parallel,
`ze-kernel` builds to `tmp/kernel/build` and copies to `tmp/kernel/vmlinuz`.
Candidate: a durable cache root holds `<artifact>/<key>/...`, where the key is
derived from the version (ISO) or the config-fragment content (kernel); a lookup
misses on any key change, forcing a rebuild or re-download; a bump evicts the
superseded key; consumers see the artifact at the path they already expect.

### Boundaries Crossed
| Boundary | Crossing | Consequence of divergence |
|----------|----------|---------------------------|
| Scratch tree -> durable cache | `tmp/` is deletable by definition; the cache must not be | Today they are the same directory, so "clean the scratch" and "destroy 30 minutes of work" are the same command |
| ISO version -> extracted initramfs | The ISO is keyed (`:101`), its extract is not (`:120`) | CONFIRMED stale hit: a new ISO boots with an old initramfs |
| Upstream mirror -> local cache | `curl -fSL` with no verification (`:111`) | A bad or substituted image is cached indefinitely and boots the test VM |
| Cache -> concurrent sessions/worktrees | Sessions and `.claude/worktrees/` share a host; a shared cache is shared mutable state | Eviction could delete an artifact another run is booting right now |
| Kernel build -> consumers | `test -f tmp/kernel/vmlinuz` guards (`mk/test-integration.mk:410`, `:424`, `:456`) | Moving the path silently breaks three labs unless they move together |

### Integration Points
`scripts/evidence/qemu-run.py` (`cache_dir`, `ensure_iso`,
`_extract_alpine_initramfs`), `mk/gokrazy.mk` (`ze-kernel` staging),
`gokrazy/kernel/Makefile` (`OUT`), `mk/test-integration.mk` (the `test -f` guards and
the QEMU targets), `scripts/evidence/effective-install-iso-qemu.py` (a second
`--kernel` consumer), `.gitignore`.

## Wiring Test

Note: `.ci` functional tests are N/A as the change surface. This is test
infrastructure (a Python harness plus make targets) with no `.ci` of its own; the
`.ci` suites are what RUN on it. Proof is that the QEMU targets still boot and pass
after the move, plus the unit tests below.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `qemu-run.py` with a cold cache | -> | `ensure_iso()` / cache lookup (CANDIDATE) | `test_cold_cache_downloads_and_verifies` |
| `qemu-run.py` after `rm -rf tmp/` | -> | durable cache root (CANDIDATE) | `test_tmp_wipe_preserves_artifacts` |
| `ALPINE_VERSION` bumped | -> | keyed extract + eviction (CANDIDATE) | `test_version_bump_misses_and_evicts` |
| `make ze-qemu-l2tp-ppp-test` with no kernel | -> | the `test -f` guard (`mk/test-integration.mk:410`) | `test_missing_kernel_fails_loudly` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `test_version_bump_misses_and_evicts` (CANDIDATE) | new, alongside `qemu-run.py` | AC-3 and AC-4: a bumped `ALPINE_VERSION` MUST NOT reuse the old extract, and the superseded artifact is reclaimed. This is the regression test for the confirmed bug |
| `test_tmp_wipe_preserves_artifacts` (CANDIDATE) | new | AC-1: `rm -rf tmp/` costs nothing; the kernel and ISO survive |
| `test_cold_cache_downloads_and_verifies` (CANDIDATE) | new | AC-5: a checksum mismatch fails loudly and does NOT populate the cache |
| `test_corrupt_cached_artifact_is_rejected` (CANDIDATE) | new | AC-5: existence is not integrity; a truncated cached file must not be served |
| `test_missing_kernel_fails_loudly` (CANDIDATE) | new | AC-6: no silent fallback when an artifact is absent |

### Functional Tests
The existing QEMU suites ARE the functional test: they must still boot and pass after
the cache moves (AC-7). No new `.ci` is written; the change is the harness the `.ci`
files run on.

## Files to Modify

- [ ] `scripts/evidence/qemu-run.py` - cache root, keyed extract, verification,
      eviction (CANDIDATE)
- [ ] `mk/gokrazy.mk` - `ze-kernel` staging destination (CANDIDATE)
- [ ] `gokrazy/kernel/Makefile` - `OUT` if the build tree also moves (CANDIDATE)
- [ ] `mk/test-integration.mk` - the three `test -f` guards if the path changes
      (CANDIDATE)
- [ ] `scripts/evidence/effective-install-iso-qemu.py` - second `--kernel` consumer
      (CANDIDATE)
- [ ] `.gitignore` - if the cache root is repo-local (CANDIDATE)
- [ ] `ai/rules/qemu-testing.md` - document where artifacts live and how to reclaim
      them (CANDIDATE)

## Implementation Steps

1. ~~Fix the stale-extract bug first.~~ DONE 2026-07-16, landed alone as predicted:
   `_extract_dir_for()` keys the extract to its ISO, with `TestQemuRunSelftest` as the
   regression test. See "Fixed already" above.
2. CANDIDATE: identify `tmp/qemu/build` (3.4G) and decide which artifacts are
   genuinely durable versus disposable.
3. CANDIDATE: choose the cache root (see Open Questions).
4. CANDIDATE: add keying + verification for the ISO; move the kernel.
5. CANDIDATE: add eviction, with a concurrency-safe policy.
6. CANDIDATE: update the guards, the consumers and `ai/rules/qemu-testing.md`.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `rm -rf tmp/` then run a QEMU target | No kernel rebuild, no ISO re-download. `tmp/` is disposable again |
| AC-2 | A fresh clone or a new git worktree | Reuses the existing cached artifacts rather than paying ~30 minutes again |
| AC-3 | `ALPINE_VERSION` or `ALPINE_MINOR` is bumped | The extracted initramfs MISSES and is re-extracted from the new ISO. The current code silently reuses the old one |
| AC-4 | A kernel or Alpine version bump | The superseded artifact is reclaimed; the cache does not grow monotonically |
| AC-5 | A downloaded or cached ISO fails its checksum | Fails loudly and is not served. Existence is not integrity |
| AC-6 | Any artifact is absent | The target fails with an actionable message; never a silent fallback |
| AC-7 | Every existing QEMU target after the move | Still boots and passes; the three `test -f` consumers still work |
| AC-8 | Two sessions or worktrees run QEMU concurrently while eviction runs | Eviction never deletes an artifact a live run is using |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The extract bug is real and reachable | CONFIRMED by reading `qemu-run.py:120-123`: fixed `alpine-extract` path, existence-only early return, while the ISO is version-keyed at `:101` | n/a, confirmed | Already validated; AC-3 is its regression test | confirmed |
| A-2 | Nothing evicts today | CONFIRMED: neither `ensure_iso` (`:99-115`) nor `ze-kernel` (`mk/gokrazy.mk:194-200`) removes anything | n/a, confirmed | Already validated | confirmed |
| A-3 | The Go/ccache caches are disposable and STAY in `tmp/` | User decision 2026-07-16: "I do still want the go-cache in tmp as it grows too much and we often need to clear it up". Go and ccache also regenerate by design | n/a, decided. Do not move them: `tmp/` is their correct home, not an accident | Already decided by the user | confirmed |
| A-4 | Alpine publishes checksums usable for AC-5 | Alpine publishes `.sha256`/`.asc` next to releases | Verification needs another source, or pin-by-digest | Fetch the release directory listing | unvalidated |
| A-5 | Artifacts can move without breaking consumers | Consumers are few and explicit: three `test -f` guards plus `effective-install-iso-qemu.py:185` | A wider sweep is needed | grep for `tmp/kernel` and `tmp/qemu` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Eviction deletes an artifact a concurrent run is booting | A QEMU run dies mid-boot while another session builds | Same shared-mutable-state hazard as `plan/spec-fixit-shared-plan-file-contention.md`. Evict only on an explicit key change, never on a timer; consider a lock or refcount; prefer leaving garbage over racing a live boot |
| R-2 | Keep-exactly-one eviction makes bisecting across a version bump cost a rebuild each way | Developers complain after a kernel bump lands | Keep-N (2 is probably enough) keyed by version/config, not wall-clock age. Kernel bumps land every few weeks, so this is a recurring cost, not a one-off |
| R-3 | The cache becomes the only copy of an unbuildable artifact | Nobody can reproduce a kernel from source | Keep the build reproducible and exercised; the cache stays an optimisation |
| R-4 | A repo-local cache is still destroyed by `git clean -xdf` | Someone cleans and pays the rebuild | This is the argument for a user-level root. Whatever is chosen, document it in `ai/rules/qemu-testing.md` |
| R-5 | Verification is added but the cached copy is never re-checked | A file rots or is truncated after caching and is served forever | AC-5 covers cached copies, not just fresh downloads |

## Open Questions (research before design)

- ~~Cache root?~~ DECIDED by the user, 2026-07-16: "have a cache folder in the
  repository to not rebuild but be able to reuse". So the durable cache is a folder in
  the repo, gitignored, a SIBLING of `tmp/` and not inside it. Two trees with two
  jobs: `tmp/` is nukeable on demand (Go caches, ccache, build scratch, ~6 GB), the
  cache folder holds the ~200 MB of expensive version-keyed artifacts. Keeping it in
  the repo also keeps the state visible and self-contained rather than hidden in
  `$HOME`.

  Accepted costs of that choice, recorded so nobody reopens it by surprise: the cache
  is per-checkout, so each `.claude/worktrees/` worktree keeps its own copy, and
  `git clean -xdf` destroys it (it is gitignored). A user-level root
  (`~/.cache/ze/`) would survive both but was not what was asked for. If worktree
  duplication later proves painful, the smallest change that preserves the decision is
  a repo-local symlink into a shared root, NOT relocating the cache out of the repo.

  Remaining detail, genuinely open: the exact folder name and whether it sits at the
  repo root (`cache/`) or under an existing directory. Match whatever naming
  convention the repo already prefers.
- What is `tmp/qemu/build` (3.4G)? Identify the writer (probably
  `tools/kernel-builder/qemu-build.py`) and confirm it is build scratch rather than an
  artifact. It stays in `tmp/` if so.
- ~~Do the Go caches and ccache move too?~~ ANSWERED by the user, 2026-07-16: no. They
  stay in `tmp/` precisely because they grow and get cleared often. Only the ~200 MB
  of expensive, version-keyed artifacts moves. Do not move anything else reflexively:
  the goal is to split durable from disposable, not to empty `tmp/`.
- Eviction policy: keep-exactly-one (matches "clear old kernel when we update", bounds
  the cache, punishes bisect) versus keep-N. Kernel updates every few weeks make this
  a live tradeoff rather than academic.
- Should the ISO be pinned by digest rather than version string, making the key and
  the integrity check the same thing?
- Is there a shared cache convention already in this repo worth matching, or is this
  the first durable cache? If it is the first, it sets the precedent, so name it
  deliberately.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] `make ze-test` green
- [ ] The stale-extract bug has a regression test (AC-3)
- [ ] `rm -rf tmp/` costs nothing (AC-1)
- [ ] Eviction cannot race a live QEMU boot (AC-8)
