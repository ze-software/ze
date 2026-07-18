# spec-fixit-qemu-artifact-cache

| Field | Value |
|-------|-------|
| Status | superseded |
| Superseded-by | spec-relocate-scratch-and-cache |
| Depends | - |
| Phase | 0/N (research) |
| Updated | 2026-07-18 |

> **SUPERSEDED 2026-07-18 by `plan/spec-relocate-scratch-and-cache.md`.** Its durable-cache
> research and ACs are absorbed there, with the cache root reversed to `~/.cache/ze` (user
> decision) and a parallel `tmp` relocation added. Kept for its research history; do not
> implement from this file.

> **DESIGN, 2026-07-16.** Research done. The skeleton's central premise was WRONG:
> this is not the first durable cache in the repo. `internal/appliance/cache.go`
> already implements a durable, arch- and config-hash-keyed, checksum-verified cache
> under `~/.cache/ze`, and `resolveRuntimeKernel` (`cmd_kernel.go:336-366`) already
> caches the runtime kernel TREE there. The work is therefore to ROUTE the uncached
> paths through the cache that exists, not to design a new one. See "Current
> Behavior" and "Superseded by research" below.
>
> **DECISIONS TAKEN 2026-07-16 (Thomas). Both blocking questions are settled:**
> 1. **Cache root: Option 2, repo-local `cache/`** -- re-affirmed after being told its
>    premise (that this would be the first durable cache) was false. Not a stale decision.
>    **Refined 2026-07-16: a linked worktree does NOT get its own `cache/`; it points at the
>    MAIN checkout's cache**, resolved as
>    `dirname($(git rev-parse --path-format=absolute --git-common-dir))/cache`. This makes
>    **AC-2 DESIGNED rather than dropped** and removes the ~200MB-per-worktree duplication
>    that was Option 2's main cost, without moving the cache out of the repo. See Open
>    Questions -> "AC-2 is DESIGNED, not dropped".
> 2. **Make path: Option C -- `run.py` ASKS THE HOST ZE BINARY (`ze-host`) FOR THE CACHE
>    KEY.** **This DELIBERATELY REVERSES the recorded decision in
>    `plan/learned/988-kernel-build-consolidation.md` that "the make path must stay
>    Ze-binary-free".** Go stays the single source of truth for the key
>    (`kernelCacheVariantFor`, `cache.go:110-120`) and cross-language drift becomes
>    impossible rather than merely detectable. **Owned cost: `make ze-kernel` now requires a
>    compiled HOST ze binary first** (`cmd/ze`, tags `ze_core,ze_setup`, **no GOARCH
>    override**, named `ze-host`; the target arch is an ARGUMENT to it, never applied to its
>    own build -- CLAUDE.md "Binary naming convention").
>
> **If you are here to "restore" 988's Ze-binary-free rule: don't.** It was overruled by its
> owner with 988 in hand. Read Open Questions -> "Decision (user, 2026-07-16): Option C"
> before touching `run.py`. At closure, 988 needs an additive correction pointer recording
> the reversal (not done yet; 988 was out of scope to edit).
>
> ~~**Status stays `design`; promotion to `ready` is Thomas's gate and has not been given.**~~
>
> **PROMOTED TO `ready` 2026-07-17 (readiness pass, Thomas-authorized loop -- no user
> question permitted).** The pass re-read every load-bearing citation at its producing
> function (`kernelCacheVariantFor` `cache.go:110-120`; `resolveRuntimeKernel`
> `cmd_kernel.go:336-366`; `build_host_ze` precedent `effective-install-qemu.py:153-168`;
> 988's Ze-binary-free decision at `plan/learned/988-kernel-build-consolidation.md:18`; the
> `test -f` guards `mk/test-integration.mk:410,424,456`; no-eviction confirmed, `cache.go`
> has no evict path, `copyTree` `RemoveAll`s only the destination `:307`) and confirmed them
> real. Option C is internally consistent throughout: no Option B assumption is presented as
> live. The residual open markers (eviction policy, ISO digest pinning, "first durable
> cache") are resolved by conservative default in Open Questions below. Status -> `ready`,
> Updated 2026-07-17.

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

**Source files (cite file:line). Every claim below was read at the producing
function, not inferred from a caller.**

**The durable cache ALREADY EXISTS (keystone finding, 2026-07-16):**

- [ ] `internal/appliance/cache.go` lines 47-57 - `resolveCacheDir()` returns
      `$XDG_CACHE_HOME/ze`, else `~/.cache/ze`, else `./.cache/ze`. A durable root
      OUTSIDE `tmp/`, shared across clones and worktrees, already in production.
- [ ] `internal/appliance/cache.go` lines 21-33 - the namespaces: `installer-kernel`,
      `runtime-kernel`, `installer-initrd`. The runtime kernel already has a cache
      namespace.
- [ ] `internal/appliance/cache.go` lines 110-120 - `kernelCacheVariantFor(target,
      arch, profile)` keys on target + **arch** + profile name + a sha256 of every
      resolved config fragment and manifest + a sha256 of `build.py`/`run.py`/`ksource.py`.
      -> Constraint: this is EXACTLY the key the runtime-kernel spec's AC-9 asks for
      (hash the config inputs, not just `kernel.version`). It exists. Do not design a
      second one.
- [ ] `internal/appliance/cmd_kernel.go` lines 336-366 - `resolveRuntimeKernel()`: on a hit
      it `copyTree(cachedDir, td.outputDir)` (`:341`), materialising the cached tree
      into `tmp/kernel/build`; on a miss it builds, enforces the requirement floor
      (`:359`), then `copyTree(td.outputDir, cachedDir)` (`:362`) to populate the
      cache. `rm -rf tmp/` on this path already costs a COPY, not a rebuild.
      -> Decision: this is the pattern to reuse: durable cache = source of truth,
      `tmp/` = materialised view. It is already implemented and shipping.
- [ ] `internal/appliance/cache.go` lines 173-230 - `downloadAndVerify()`: fetches the
      checksum, streams into a temp file under the destination dir while hashing,
      rejects on mismatch (`:221-223`), and only then `os.Rename` (`:225`). Atomic,
      verified, never caches a bad file. This is AC-5, already implemented.
- [ ] `internal/appliance/cache.go` lines 232-257 - `downloadChecksum()` parses
      `fields[0]` and requires 64 hex chars, which is exactly the format Alpine
      publishes (validated below, A-4).

**What bypasses that cache:**

- [ ] `mk/gokrazy.mk` lines 194-200 - `ze-kernel` calls `$(MAKE) -C gokrazy/kernel`
      (`:197`), NOT `ze appliance kernel`. `gokrazy/kernel/Makefile:30` then invokes
      `tools/kernel-builder/run.py` directly. The make path therefore never consults
      `resolveRuntimeKernel`, so it never reads or writes `~/.cache/ze/runtime-kernel`.
      -> Constraint: this is the root cause of "the kernel dies with `tmp/`". Not a
      missing cache: an unrouted build path.
- [ ] On-disk proof, this checkout, 2026-07-16: `tmp/kernel` is 460M and
      `~/.cache/ze/runtime-kernel/` DOES NOT EXIST, while `~/.cache/ze/installer-kernel/`
      does. The Go path populated its cache; the make path never populated the runtime one.
- [ ] `plan/learned/988-kernel-build-consolidation.md` "Decisions" - the make path
      staying Ze-binary-free is a RECORDED, DELIBERATE decision: the fragment resolver
      is duplicated in `run.py` and `kernelreg.go` "because the make path must stay
      Ze-binary-free", with a cross-language fixture (`kernel-shared-fragment.ci` +
      `TestResolveSharedInclude`) guarding drift.
      -> Constraint: "just make `ze-kernel` shell out to `ze appliance kernel`"
      contradicts a recorded decision. See Open Questions; ~~this needs Thomas.~~
      -> RESOLVED 2026-07-17: Thomas chose Option C (Open Questions -> "Decision (user,
      2026-07-16): Option C"). `run.py` asks `ze-host` for the key rather than shelling out
      to `ze appliance kernel`, so the make path is no longer Ze-binary-free but never calls
      the Go build path directly. 988's Ze-binary-free corollary is deliberately reversed.
- [ ] `gokrazy/kernel/Makefile` lines 19-20 - `OUT := $(REPO_ROOT)/tmp/kernel/build`,
      whose comment justifies `tmp/` on gitignore grounds only. Correct about git,
      silent about durability.
- [ ] `internal/appliance/cmd_kernel.go` line 43 - `runtimeKernelOutputDir =
      "tmp/kernel/build"`. The Go path and the make path agree on the OUTPUT dir, which
      is what makes the materialised-view design work without moving consumers.

**The ISO path:**

- [ ] `scripts/evidence/qemu-run.py` lines 99-115 - `ensure_iso()`: the name is
      version+arch keyed (`:101`), the hit is bare existence (`:103-104`), the download
      is `curl -fSL` with no checksum (`:111`), and nothing is ever evicted.
- [ ] `tools/kernel-builder/qemu-build.py` lines 155-170 - a SECOND, independent
      `ensure_iso()` with its own `curl -fSL` (`:166-168`), no checksum, writing the
      same filename scheme into the same directory via `cache_dir()`
      (`qemu-build.py:66-70` -> `tmp/qemu/iso`).
      -> Decision: there are TWO ISO producers, not one. Any keying, verification or
      eviction that touches only `qemu-run.py` leaves the other half broken.
- [ ] `scripts/evidence/qemu-run.py` lines 29-30 and `tools/kernel-builder/qemu-build.py`
      lines 36-37 - `ALPINE_VERSION = "3.21"` / `ALPINE_MINOR = "3"` is DUPLICATED in
      both producers. They agree today by coincidence. Bumping one leaves the other
      downloading the superseded ISO into the same cache dir, and nothing evicts either.
- [ ] `scripts/evidence/qemu-run.py` lines 118-127 - `_extract_dir_for()` keys the
      extract by ISO stem. The stale-extract bug is FIXED (see "Fixed already"); the
      selftest guarding it is `qemu-run.py` lines 492-542, run by
      `scripts/evidence/qemu_run_test.go`.

**Consumers of the paths this spec might move:**

- [ ] `mk/test-integration.mk` lines 410, 424, 456 - three `test -f tmp/kernel/vmlinuz`
      guards. Existence only: they cannot detect a WRONG-ARCH kernel (see A-6).
- [ ] `mk/gokrazy.mk` lines 181-182, 238 - `KERNEL_BUILD_DIR`, `KERNEL_PKG_DIR`, and
      `ze-kernel-clean`'s `rm -rf tmp/kernel`.
- [ ] `internal/appliance/cmd_kernel.go` line 43, `internal/appliance/cmd_kernel_test.go`
      line 734.
- [ ] `test/install/*.ci` - `ze-kernel-overlay.ci:30-37,71-75`,
      `kernel-qemu-arch-alias.ci:18`, `kernel-version-provenance.ci:15-16,49-65`,
      `ze-kernel-no-modcache-mutation.ci:17,29`, `kernel-runtime-deps.ci:4-5`,
      `appliance-kernel-runtime.ci:18-21`.
- [ ] `scripts/evidence/effective-vrrp-keepalived.py` line 393 (error text),
      `scripts/evidence/effective-gokrazy-l2tp-ppp.py` line 644 (`tmp/kernel/pkg`).
- [ ] Docs: `ai/rules/qemu-testing.md:162,210`,
      `docs/architecture/testing/qemu-integration.md`, `docs/functional-tests.md`,
      `docs/guide/appliance.md`.
      -> Decision: the consumer set is ~20 sites across make, Go, `.ci` and docs, not
      "three guards plus one script". A-5 is BROKEN. This is the argument for NOT
      moving any consumer-facing path: keep `tmp/kernel/vmlinuz` and
      `tmp/kernel/build` where they are and make them materialised views.

**Measured, 2026-07-16 (this checkout, aarch64):**
- `tmp/qemu` totals 6.2 GB: `build` 3.4G, `go-cache` 1.8G, `gomodcache` 590M,
  `ccache` 251M, `iso` 153M, `go-dl` 55M. `tmp/kernel` is 460M.
- `tmp/qemu/iso` holds `alpine-virt-3.21.3-aarch64.iso` (76M) plus the ORPHANED
  legacy `alpine-extract/` (~77M), stranded by the `_extract_dir_for` rename. The
  keyed `alpine-virt-3.21.3-aarch64-extract/` does not exist yet, so the next
  `--kernel` run re-extracts ~77M and the orphan stays. Live proof of AC-4.
- `tmp/qemu/build` (3.4G) is written by `tools/kernel-builder/qemu-build.py:115-119`
  (`build_dir()` -> `tmp/qemu/build/<alpine_arch>`), and `ccache` by `:73-77`. The
  spec's UNVERIFIED item is now CONFIRMED: both are qemu-path build scratch and STAY.
- The cached ISO's sha256 is `693b5d99d00b27688617d3c64a12848d8961a9b4240d1472c3fe66a327c31c0b`,
  matching the published `.sha256` exactly. The current copy is intact and
  verification is implementable today.

**Behavior to preserve:**
- `tmp/` stays safe to delete wholesale. That is the requirement driving this spec,
  not a nice-to-have.
- The build stays reproducible on demand: the cache is an optimisation, never the only
  copy of an artifact nobody can rebuild.
- Consumers keep failing loudly when an artifact is absent (`test -f` guards), never
  silently falling back.
- `tmp/go.mod` keeps marking `tmp/` a nested module so `go list ./...` skips these
  caches (`.gitignore:13`, `qemu-run.py:75`).

**Behavior to preserve (added by research, 2026-07-16):**
- Every consumer-facing path stays: `tmp/kernel/vmlinuz`, `tmp/kernel/build`,
  `tmp/kernel/pkg`. ~20 sites depend on them (see above). They become MATERIALISED
  VIEWS of the durable cache, exactly as `resolveRuntimeKernel` already does
  (`cmd_kernel.go:341`).
- ~~The make kernel path stays Ze-binary-free unless Thomas overrules
  (`plan/learned/988-kernel-build-consolidation.md`).~~
  **SUPERSEDED -> Decision (user, 2026-07-16): THOMAS HAS OVERRULED.** The make kernel path
  does NOT stay Ze-binary-free: under Option C, `run.py` asks a host ze binary (`ze-host`)
  for the cache key, so `make ze-kernel` requires that binary to be built first. This is a
  deliberate reversal of `plan/learned/988`'s recorded decision, taken with 988 in hand.
  What DOES survive from 988: `run.py` remains the one driver that owns the build and the
  docker/qemu selection. Only the key is delegated to Go.
- `~/.cache/ze` layout and key format stay compatible: the Go path is live and its
  cache must not be invalidated wholesale by this work.

**Behavior to change:**
- ~~Expensive, version-keyed artifacts move out of `tmp/`.~~ SUPERSEDED 2026-07-16.
  Nothing MOVES. The artifacts are already produced into `tmp/`, and ~20 consumers
  depend on that. What changes is that `tmp/` stops being the ONLY copy: the make
  kernel path gets routed through the durable cache that already exists, so `tmp/`
  becomes reconstructible in seconds. Same user requirement ("`tmp/` must be
  deletable"), far smaller blast radius.
- The make kernel path (`mk/gokrazy.mk:194-200`) reads and writes
  `~/.cache/ze/runtime-kernel`, keyed exactly as `kernelCacheVariantFor` keys it.
- The Alpine ISO joins the durable cache, verified against its published `.sha256`.
- The two ISO producers (`qemu-run.py:99-115`, `qemu-build.py:155-170`) stop being
  two: one keyed, verified implementation, one `ALPINE_VERSION` constant.
- A version or config-fragment bump reclaims the artifact it supersedes (new: the
  existing Go cache has NO eviction either, see A-7).

**Superseded by research (recorded, not deleted):**

| Skeleton claim | Reality | Evidence |
|----------------|---------|----------|
| "Is this the first durable cache? If it is, it sets the precedent" | It is NOT the first. A keyed, verified, durable cache is already in production | `internal/appliance/cache.go:47-57,110-120,173-230` |
| "a second `--kernel` consumer: `effective-install-iso-qemu.py:185`" | FALSE. That is `ze appliance iso --kernel`, a DIFFERENT flag on a DIFFERENT binary, consuming the operator-supplied INSTALLER kernel from `ZE_INSTALL_KERNEL`, never `tmp/kernel/vmlinuz` | `effective-install-iso-qemu.py:181-196`; producer `effective-install-qemu.py:128-132` returns `$ZE_INSTALL_KERNEL` or None |
| "Consumers are few and explicit: three `test -f` guards plus one script" (A-5) | BROKEN. ~20 sites across make, Go, `.ci`, evidence scripts and docs | grep evidence in "Consumers" above |
| "What writes `tmp/qemu/build` (3.4G)? probably qemu-build.py" | CONFIRMED, and it is scratch: it stays in `tmp/` | `tools/kernel-builder/qemu-build.py:115-119` |

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

Today, TWO kernel paths exist and only one is cached:

| Path | Route | Cached? |
|------|-------|---------|
| `ze appliance kernel --target runtime` | `resolveKernel` (`cmd_kernel.go:273`) -> `resolveRuntimeKernel` (`:336`) -> hit: `copyTree(cache -> tmp/kernel/build)` (`:341`); miss: build, then `copyTree(tmp/kernel/build -> cache)` (`:362`) | YES, `~/.cache/ze/runtime-kernel/<version>-<variant>` |
| `make ze-kernel` | `mk/gokrazy.mk:197` -> `gokrazy/kernel/Makefile:28-43` -> `run.py` -> `tmp/kernel/build`, then `cp` to `tmp/kernel/vmlinuz` (`mk/gokrazy.mk:200`) | NO. Dies with `tmp/` |

ISO today: `qemu-run.py:cache_dir()` (`:84-92`) creates `tmp/qemu/iso` ->
`ensure_iso()` (`:99-115`) downloads unverified if absent -> `_extract_dir_for()`
(`:118-127`) keys the extract by ISO stem -> `_extract_alpine_initramfs()` (`:130-150`)
extracts if absent -> `qemu_args()` (`:199-210`) passes `-kernel` + `-initrd`.
Independently, `qemu-build.py:ensure_iso()` (`:155-170`) downloads the same ISO into
the same dir, unverified.

Target: the durable cache is the source of truth; `tmp/` is a materialised view that
`rm -rf` may destroy at any time. The make kernel path performs the same
lookup/materialise/populate dance the Go path already performs. The ISO gains one
keyed, verified implementation shared by both producers. Eviction reclaims a
superseded key. No consumer-facing path moves.

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
| `qemu-run.py` with a cold cache | -> | verified `ensure_iso()` (`qemu-run.py:99-115`) routed through the durable root, `.sha256` checked as `downloadAndVerify` does (`cache.go:173-230`) | `test_cold_cache_downloads_and_verifies` |
| `qemu-run.py` after `rm -rf tmp/` | -> | durable cache root resolver `dirname($(git rev-parse --path-format=absolute --git-common-dir))/cache` (Open Questions -> AC-2 decision) | `test_tmp_wipe_preserves_artifacts` |
| `ALPINE_VERSION` bumped | -> | eviction of the superseded key in the durable cache (new: none today, A-7; extract already keyed by `_extract_dir_for`, `qemu-run.py:118-127`) | `test_version_bump_misses_and_evicts` |
| `make ze-qemu-l2tp-ppp-test` with no kernel | -> | the `test -f` guard (`mk/test-integration.mk:410`) | `test_missing_kernel_fails_loudly` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `qemu-run.py --selftest` (EXISTS) | `scripts/evidence/qemu_run_test.go:27` | AC-3: already green, guards `_extract_dir_for`. Extend it rather than writing a parallel harness: it is already in `make ze-unit-test` and needs no QEMU, no download, no 7z |
| `test_version_bump_evicts_superseded` (CANDIDATE) | extend the `--selftest` fixture | AC-4: a bumped key reclaims the old entry, including the orphaned `alpine-extract/` |
| `test_tmp_wipe_preserves_artifacts` (CANDIDATE) | new, `.ci` under `test/install/` beside `kernel-runtime-deps.ci` | AC-1: with a populated cache, wiping `tmp/` then re-materialising costs a copy. Point `XDG_CACHE_HOME` at a temp dir (`cache.go:48`) so the test never touches the developer's real cache. Precedent: `effective-install-scenarios-qemu.py:99-129` already drives `XDG_CACHE_HOME` exactly this way |
| `test_cold_cache_downloads_and_verifies` (CANDIDATE) | new | AC-5: a checksum mismatch fails loudly and does NOT populate the cache |
| `test_corrupt_cached_artifact_is_rejected` (CANDIDATE) | new | AC-5: existence is not integrity; a truncated cached file must not be served |
| ~~`test_make_and_go_paths_share_one_key`~~ -> `test_run_py_key_from_ze_host_or_fails_loud` (CANDIDATE, renamed 2026-07-17 per Option C) | new; drive `run.py`'s key query with `ze-host` present, then absent | AC-9: ~~the Python key and `kernelCacheVariantFor` (`cache.go:110-120`) agree byte-for-byte. Mandatory under Option B. Model it on the existing anti-drift pair `kernel-shared-fragment.ci` + `TestResolveSharedInclude`~~ **RESHAPED by the Option C decision (user, 2026-07-16): there is no Python key to compare.** `run.py` asks the host ze binary for the key, so the two paths share one implementation by construction and byte-for-byte agreement is structural, not asserted. The anti-drift fixture is NOT needed (that was Option B's mandatory mitigation). What DOES need a test: that `run.py` actually consults `ze-host` (built `-tags ze_core,ze_setup`, no GOARCH override) and fails loudly when the binary is absent or the key query errors, rather than falling back to a guessed key (AC-6). Test name resolved 2026-07-17: no longer "share one key" (an Option B framing), now "key from ze-host or fail loud" |
| `test_missing_kernel_fails_loudly` (CANDIDATE) | new | AC-6: no silent fallback when an artifact is absent |
| `test_kernel_clean_keeps_durable_cache` (CANDIDATE) | new | AC-10: `ze-kernel-clean` (`mk/gokrazy.mk:238`) clears `tmp/kernel` only |

-> Constraint: every test that touches the cache MUST set `XDG_CACHE_HOME`
(`cache.go:48`) to a temp dir. A test that evicts from the developer's real
`~/.cache/ze` would destroy the very artifact this spec exists to protect, and would
do it on a machine where a concurrent QEMU run may be booting it (R-1).

### Functional Tests
The existing QEMU suites ARE the functional test: they must still boot and pass after
the cache moves (AC-7). No new `.ci` is written; the change is the harness the `.ci`
files run on.

## Files to Modify

Scoped after research. The blast radius is far smaller than the skeleton assumed
BECAUSE no consumer-facing path moves.

- [ ] `tools/kernel-builder/run.py` - the cache lookup/populate for the make kernel
      path, ~~mirroring `kernelCacheVariantFor` (`cache.go:110-120`). This is the
      Ze-binary-free option (Option B, see Open Questions)~~
      **SUPERSEDED -> Decision (user, 2026-07-16): Option C.** Do NOT mirror
      `kernelCacheVariantFor`; **ask the host ze binary for the key** so Go stays its single
      source of truth and cross-language drift is impossible. `run.py` still owns the build
      and the docker/qemu selection. Requires building `ze-host` first
      (`go build -tags ze_core,ze_setup -o ze-host ./cmd/ze`, NO GOARCH override; target arch
      is an argument). This reverses `plan/learned/988`'s Ze-binary-free decision deliberately
- [ ] `tools/kernel-builder/qemu-build.py` - drop its private `ensure_iso()`
      (`:155-170`) and `ALPINE_VERSION`/`ALPINE_MINOR` (`:36-37`); use the shared,
      verified ISO helper. `build_dir()` (`:115-119`) and `ccache_dir()` (`:73-77`)
      stay in `tmp/`: they are scratch
- [ ] `scripts/evidence/qemu-run.py` - move `ensure_iso()` (`:99-115`) to the durable
      root, add `.sha256` verification of both fresh downloads AND cached copies,
      export the helper the two producers share. `cache_dir()` (`:84-92`) keeps
      creating the `tmp/qemu` Go/scratch dirs unchanged
- [ ] `internal/appliance/cache.go` - eviction (new: nothing here evicts today), and
      an `alpine-iso` namespace if the ISO key lives in Go rather than Python
- [ ] `mk/gokrazy.mk` - `ze-kernel` (`:194-200`) materialises from the cache instead
      of always building. `ze-kernel-clean`'s `rm -rf tmp/kernel` (`:238`) must NOT
      delete the durable cache
- [ ] `ai/rules/qemu-testing.md` - lines 162 and 210 describe `tmp/kernel/vmlinuz` and
      `tmp/qemu/` as the cache; document the durable root, what survives `rm -rf tmp/`,
      and how to reclaim. This is the `ai/rules/discovery-updates.md` obligation
- [ ] `docs/architecture/testing/qemu-integration.md`, `docs/guide/appliance.md`,
      `docs/functional-tests.md` - same claim, three more places
- [ ] `.gitignore` - ~~ONLY if Thomas chooses the repo-local root (Open Questions)~~
      -> RESOLVED 2026-07-17: Thomas chose Option 2 (repo-local `cache/`), so this IS in
      scope. Add a `cache/` (or `/cache`) ignore entry next to the existing `tmp/*`
      (`.gitignore:12`) so the durable cache is invisible to git exactly as `tmp/` is
- [ ] `gokrazy/kernel/Makefile` - `OUT` (`:19-20`) stays. Listed to record the
      decision NOT to touch it

## Implementation Steps

1. ~~Fix the stale-extract bug first.~~ DONE 2026-07-16, landed alone as predicted:
   `_extract_dir_for()` keys the extract to its ISO, with `TestQemuRunSelftest` as the
   regression test. See "Fixed already" above.
2. ~~Identify `tmp/qemu/build`.~~ DONE by research: `qemu-build.py:115-119`, build
   scratch, stays in `tmp/`.
3. ~~Choose the cache root.~~ Answered by research, pending Thomas: the repo already
   has one (`cache.go:47-57`). See Open Questions.
4. ~~BLOCKING, Thomas: settle Option A vs B (Ze-binary-free make path) and the cache
   root. Both are recorded decisions being revisited, so neither is ours to take.~~
   **DONE 2026-07-16. Both settled by Thomas:** the cache root is **Option 2, repo-local
   `cache/`** (re-affirmed with the corrected premise in hand), and the make path is
   **Option C: `run.py` asks the host ze binary for the cache key**, which deliberately
   reverses `plan/learned/988`'s Ze-binary-free decision. Both recorded decisions were
   revisited by their owner, as required. No longer blocking.
   -> Constraint: build `ze-host` (`-tags ze_core,ze_setup`, NO GOARCH override) before
   `make ze-kernel` can resolve a key; the target arch is an argument to it, never applied to
   its own build (CLAUDE.md "Binary naming convention"; precedent
   `scripts/evidence/effective-install-qemu.py:153-168`).
5. Route the make kernel path through the cache. Prove it with the cheap test first:
   `make ze-kernel`, `rm -rf tmp/kernel`, `make ze-kernel` again, assert the second is
   seconds not ~30 minutes and that `~/.cache/ze/runtime-kernel/` is populated.
6. Give the ISO one keyed, verified implementation shared by both producers, and one
   `ALPINE_VERSION` constant.
7. Add eviction (keep-N by key, never by timer) for the whole `~/.cache/ze` tree, with
   the concurrency guard R-1 demands.
8. Update `ai/rules/qemu-testing.md` and the three docs. Add the discovery entry.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `make ze-kernel`, `rm -rf tmp/`, `make ze-kernel` | The second run materialises from the durable cache in seconds, no ~30-minute rebuild. `tmp/` is disposable again |
| AC-2 | A fresh clone or a new git worktree on the same host | Reuses the cached kernel and ISO rather than paying ~30 minutes again. ~~Free if the root is `~/.cache/ze`; NOT satisfiable if the root is repo-local (see Open Questions)~~ **DESIGNED 2026-07-16 (user): a linked worktree does not get its own `cache/`; it resolves to the MAIN checkout's cache via `dirname($(git rev-parse --path-format=absolute --git-common-dir))/cache`.** So the WORKTREE half of AC-2 is satisfied with a repo-local root. The FRESH-CLONE half is NOT and is out of scope: a clone has its own `--git-common-dir`, so it legitimately pays the rebuild once. Do not conflate the two halves. See Open Questions -> "AC-2 is DESIGNED, not dropped" |
| AC-3 | `ALPINE_VERSION` or `ALPINE_MINOR` is bumped | The extracted initramfs MISSES and is re-extracted from the new ISO. ALREADY SATISFIED by `_extract_dir_for` (`qemu-run.py:118-127`); `qemu-run.py --selftest` is the regression test. Keep it green |
| AC-4 | A kernel or Alpine version bump | The superseded artifact is reclaimed; the cache does not grow monotonically. Includes reclaiming the orphaned legacy `tmp/qemu/iso/alpine-extract/` (~77M, present on disk today) |
| AC-5 | A downloaded or cached ISO fails its checksum | Fails loudly and is not served. Existence is not integrity. `downloadAndVerify` (`cache.go:173-230`) is the reference: hash while streaming, atomic rename, never publish a bad file |
| AC-6 | Any artifact is absent | The target fails with an actionable message; never a silent fallback |
| AC-7 | Every existing QEMU target after the change | Still boots and passes; all ~20 consumers of `tmp/kernel/*` still work, including `test/install/*.ci` |
| AC-8 | Two sessions or worktrees run QEMU concurrently while eviction runs | Eviction never deletes an artifact a live run is using |
| AC-9 | `make ze-kernel` and `ze appliance kernel --target runtime` for the same arch + config | Both resolve the SAME cache entry. Two build paths, one cache, or the bug returns by another door |
| AC-10 | `make ze-kernel-clean` (`mk/gokrazy.mk:238`, `rm -rf tmp/kernel`) | Clears the materialised view only. The durable cache SURVIVES: it is not scratch |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | The extract bug is real and reachable | CONFIRMED by reading `qemu-run.py:120-123`: fixed `alpine-extract` path, existence-only early return, while the ISO is version-keyed at `:101` | n/a, confirmed | Already validated; AC-3 is its regression test | confirmed |
| A-2 | Nothing evicts today | CONFIRMED: neither `ensure_iso` (`:99-115`) nor `ze-kernel` (`mk/gokrazy.mk:194-200`) removes anything | n/a, confirmed | Already validated | confirmed |
| A-3 | The Go/ccache caches are disposable and STAY in `tmp/` | User decision 2026-07-16: "I do still want the go-cache in tmp as it grows too much and we often need to clear it up". Go and ccache also regenerate by design | n/a, decided. Do not move them: `tmp/` is their correct home, not an accident | Already decided by the user | confirmed |
| A-4 | Alpine publishes checksums usable for AC-5 | Alpine publishes `.sha256`/`.asc` next to releases | Verification needs another source, or pin-by-digest | Fetch the release directory listing | **confirmed 2026-07-16**: `HTTP/2 200` for `.../alpine-virt-3.21.3-aarch64.iso.sha256`, body `693b5d99...  alpine-virt-3.21.3-aarch64.iso`. Single-space `<hex>  <name>`, parsed as-is by `downloadChecksum` (`cache.go:248-256`: `fields[0]`, 64 hex). The ISO on disk matches that hash, so the current copy is intact |
| A-5 | ~~Artifacts can move without breaking consumers~~ | ~~Consumers are few and explicit: three `test -f` guards plus `effective-install-iso-qemu.py:185`~~ | The design changes: do not move anything | grep for `tmp/kernel` and `tmp/qemu` | **BROKEN 2026-07-16**. Both halves wrong. (a) ~20 consumers, not four: `mk/gokrazy.mk:181-182,238`, `mk/test-integration.mk:410,424,456`, `cmd_kernel.go:43`, `cmd_kernel_test.go:734`, six `test/install/*.ci`, two evidence scripts, four docs. (b) `effective-install-iso-qemu.py:185` is NOT a consumer: it is `ze appliance iso --kernel` taking the operator-supplied installer kernel (`effective-install-qemu.py:128-132`), unrelated to `tmp/kernel/vmlinuz`. -> Design consequence: nothing moves; `tmp/` becomes a materialised view |
| A-6 | The `test -f tmp/kernel/vmlinuz` guard is a sufficient precondition | The three labs use it and the pattern was described as "the guard pattern to preserve" | The guard passes on a kernel that cannot boot | Read the guard and the arch derivation | **BROKEN 2026-07-16**. `GOKRAZY_ARCH ?= amd64` (`mk/gokrazy.mk:37`) and `KERNEL_ARCH ?= $(GOKRAZY_ARCH)` (`:177`), so a bare `make ze-kernel` on Apple Silicon stages an **amd64** vmlinuz to the arch-unkeyed `tmp/kernel/vmlinuz` (`:200`), while `QEMU_GOARCH` derives **arm64** (`mk/test-integration.mk:216`). `test -f` passes; the boot fails. The guards' own error text ("run: make ze-kernel GOKRAZY_ARCH=arm64", `:410`) is the workaround. The durable cache is arch-keyed (`cache.go:110-120`), so routing through it FIXES this for free |
| A-7 | The existing `~/.cache/ze` cache evicts | Not claimed by the skeleton; assumed while designing on top of it | Eviction is new work, not a pre-existing feature to reuse | Read `cache.go` for any removal | **broken/confirmed-absent 2026-07-16**: `cache.go` has no eviction. `copyTree` (`:306-312`) does `os.RemoveAll(dst)` on the DESTINATION only. Every superseded `<version>-<variant>` entry is stranded forever. AC-4 is genuinely new work and it applies to `installer-kernel` and `installer-initrd` too, not only the QEMU artifacts |
| A-8 | Routing the make path through the cache is a small change | `run.py` already resolves the fragments the key hashes (`build.py:91-92`) | If the key must be byte-identical across Python and Go, this is a cross-language contract, not a one-liner | Read how the repo handled the same problem before | **confirmed, with a caveat 2026-07-16**: the repo already accepted exactly this duplication for the fragment resolver, and guards it with a cross-language fixture (`plan/learned/988-kernel-build-consolidation.md`: resolver duplicated in `run.py` + `kernelreg.go`, `kernel-shared-fragment.ci` + `TestResolveSharedInclude` guard drift). Precedent exists AND ships its own anti-drift mechanism. Caveat: a second cross-language hash contract is a real maintenance cost, which is what makes Option A vs B a Thomas decision. **-> Decision (user, 2026-07-16): that caveat DECIDED it, and neither A nor B won -- Option C did.** The precedent's anti-drift fixture detects drift; Option C makes drift impossible by deleting the second implementation (`run.py` asks `ze-host` for the key). A drifted hash key serves a stale kernel silently, so "detectable" was judged not good enough. The 988 precedent therefore does NOT transfer to the key, even though it stands for the fragment resolver |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | Eviction deletes an artifact a concurrent run is booting | A QEMU run dies mid-boot while another session builds | Same shared-mutable-state hazard as `plan/spec-fixit-shared-plan-file-contention.md`. Evict only on an explicit key change, never on a timer; consider a lock or refcount; prefer leaving garbage over racing a live boot |
| R-2 | Keep-exactly-one eviction makes bisecting across a version bump cost a rebuild each way | Developers complain after a kernel bump lands | Keep-N (2 is probably enough) keyed by version/config, not wall-clock age. Kernel bumps land every few weeks, so this is a recurring cost, not a one-off |
| R-3 | The cache becomes the only copy of an unbuildable artifact | Nobody can reproduce a kernel from source | Keep the build reproducible and exercised; the cache stays an optimisation |
| R-4 | A repo-local cache is still destroyed by `git clean -xdf` | Someone cleans and pays the rebuild | ~~This is the argument for a user-level root.~~ Whatever is chosen, document it in `ai/rules/qemu-testing.md`. **AMENDED 2026-07-16 by the AC-2 decision: SOFTENED for worktrees, CONCENTRATED for main.** A `git clean -xdf` in a worktree can no longer destroy the cache (it is not in the worktree); a `git clean -xdf` in the MAIN checkout now destroys it for every worktree at once. Blast radius is worse, frequency is lower. R-4 is NOT retired: the AC-6 loud guard is now the single protection for all trees, so a wiped cache must always cost a rebuild and never yield a silently-wrong artifact |
| R-6 | **NEW (2026-07-16, from the AC-2 decision).** A worktree writes into the main checkout's `cache/`, which superficially resembles the cross-tree writes `.claude/hooks/pretool-bash.py` blocks and `CLAUDE.md` forbids | A hook blocks a cache write from a worktree, or a reviewer flags it as a violation | Legitimate and distinct: the cache is **gitignored build output**, not tracked files and not anyone's working tree, so it carries none of the "overwrites uncommitted work" hazard the rule exists to prevent. Record the distinction in `ai/rules/qemu-testing.md` alongside the cache-root documentation so the next agent does not "fix" it. Verify during implementation whether the pretool hook's cross-tree-copy check actually fires on this path; if it does, the hook needs an explicit carve-out for the cache root, NOT a workaround in the build scripts |
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

  Remaining detail, ~~genuinely open~~: the exact folder name and whether it sits at the
  repo root (`cache/`) or under an existing directory. Match whatever naming
  convention the repo already prefers.
  -> AUTONOMOUS DEFAULT (2026-07-17): **`cache/` at the repo root**, a sibling of `tmp/`.
  Rationale: it is the name already used everywhere in this spec (line 26 "MAIN checkout's
  cache", the resolver `dirname(...)/cache`, the Files-to-Modify `.gitignore` entry), it
  mirrors `tmp/`'s repo-root placement so "durable sibling of the disposable tree" is legible
  at a glance, and the repo has no competing convention (`~/.cache/ze` is the OTHER root and
  stays authoritative for the appliance path). Thomas: override if a nested location is
  preferred.

  **REOPENED BY RESEARCH, 2026-07-16. ~~NEEDS THOMAS.~~ RESOLVED below (Option 2, user,
  2026-07-16) -- this heading is answered, not open.** The decision above was taken on
  a premise that is now known false. The spec asked "is this the first durable cache,
  or is there a convention to match?" and answered "if it is the first, it sets the
  precedent, so name it deliberately". It is NOT the first. The repo already has a
  durable cache root, in production, with the exact properties this spec wants:
  `resolveCacheDir()` (`internal/appliance/cache.go:47-57`) returns
  `$XDG_CACHE_HOME/ze` or `~/.cache/ze`, and already holds `installer-kernel`,
  `installer-initrd` and a `runtime-kernel` namespace keyed by arch + config hash.
  So the real question is no longer "what do we name our new cache" but "do we build a
  SECOND cache root next to the one we have".

  | Option | Cost | Benefit |
  |--------|------|---------|
  | **1. Reuse `~/.cache/ze`** (`cache.go:47-57`) | Contradicts the user's stated "cache folder in the repository"; state lives outside the checkout | Zero new mechanism. Survives `git clean -xdf`. **Shared across worktrees, so AC-2 is free.** One eviction policy for all ze artifacts. `XDG_CACHE_HOME` already makes it redirectable and testable |
  | **2. Repo-local `cache/`** (as decided) | A second cache root in the same repo, with the same job. Per-worktree duplication (~200MB x N). `git clean -xdf` destroys it (R-4). Two eviction policies, or one that must understand both roots | State is visible in the checkout, which is what was asked for |

  -> Decision (user, 2026-07-16): **Option 2, repo-local `cache/`. The original call
  STANDS**, re-affirmed after being told its premise was wrong. Research recommended
  Option 1 and the user overrode it with the corrected facts in hand, so this is a
  deliberate choice and NOT the stale decision the research took it for. Do not reopen
  it on the "it is not the first cache root" argument: that argument has been made and
  answered. The reason of record is the Option 2 benefit column, state visible in the
  checkout, and it outweighs the duplication cost for this user's workflow.

  -> Constraint: Option 2's costs are now OWNED, not hypothetical, and the spec must
  carry them rather than rediscover them. Concretely: (a) `git clean -xdf` destroys the
  cache (R-4), so the loud-guard AC-4 matters more, not less, and a wiped cache must
  cost a rebuild and never a silently-wrong artifact; (b) ~~per-worktree duplication of
  ~200MB x N is accepted~~ **RESOLVED 2026-07-16 by the AC-2 decision below: a worktree
  no longer gets its own copy, so the duplication cost is removed, not merely accepted**;
  (c) ~~AC-2 (worktree reuse) is NOT free and must be designed or
  explicitly dropped -- flag it rather than quietly shipping without it~~ **DESIGNED
  2026-07-16, not dropped -- see below**; (d) the repo now
  has two durable cache roots with one job, so eviction must state which root it governs
  and `~/.cache/ze` stays untouched and authoritative for the appliance path.

  #### -> Decision (user, 2026-07-16): AC-2 is DESIGNED, not dropped. A worktree resolves to the MAIN checkout's cache.

  Thomas's design, verbatim: **"we should be able to point the cache to the main copy when
  in a worktree."** So the repo-local cache is repo-local to the **main checkout**, and a
  linked worktree does NOT get its own `cache/`: it resolves to the main checkout's one.

  -> Decision: this preserves Option 2's reason of record (state visible in the checkout,
  a sibling of `tmp/`) while removing the duplication that was its main cost. It is NOT a
  reversal of the Option 1 vs 2 call and must not be read as one: the cache stays in the
  repo, there is still exactly one repo-local root, and `~/.cache/ze` stays untouched and
  authoritative for the appliance path (d). The `~200MB x N` cost from (b) goes to ~200MB x 1.

  -> Constraint: this is the "repo-local symlink into a shared root" escape hatch the
  original decision already named as the smallest change preserving it ("If worktree
  duplication later proves painful, the smallest change that preserves the decision is a
  repo-local symlink into a shared root, NOT relocating the cache out of the repo"), except
  the shared root is the MAIN CHECKOUT rather than `$HOME`. It is the sanctioned move, taken
  deliberately and early rather than under pain.

  **How a worktree identifies the main checkout (mechanism verified by execution,
  2026-07-16, git 2.55.0):**

  | Command | In main checkout (from repo root) | In main checkout (from a SUBDIR) | In a linked worktree |
  |---------|-----------------------------------|----------------------------------|----------------------|
  | `git rev-parse --git-dir` | `.git` | `/Users/.../ze/main/.git` | `/Users/.../ze/main/.git/worktrees/<name>` |
  | `git rev-parse --git-common-dir` | `.git` | `../.git` **(RELATIVE!)** | `/Users/.../ze/main/.git` |
  | `git rev-parse --path-format=absolute --git-common-dir` | `/Users/.../ze/main/.git` | `/Users/.../ze/main/.git` | `/Users/.../ze/main/.git` |

  -> Decision: **resolve the cache root as
  `dirname($(git rev-parse --path-format=absolute --git-common-dir))/cache`.** One
  expression, no branching, no worktree detection needed: `--git-common-dir` points at the
  SHARED git directory, which lives in the main checkout, so its parent IS the main
  checkout root from every context above. Executed and confirmed to yield
  `/Users/.../ze/main` from the main checkout, from a subdirectory of it, and from
  `.claude/worktrees/agent-af24655dd2ac354ab`.

  -> Constraint: **`--path-format=absolute` is MANDATORY, not cosmetic.** Bare
  `git rev-parse --git-common-dir` returns a path relative to CWD (`../.git` from
  `scripts/`, executed and confirmed above), so a naive `dirname $(git rev-parse
  --git-common-dir)` silently resolves against the wrong directory depending on where the
  make target was invoked from. `--path-format=absolute` requires git >= 2.31 (2021); the
  host runs 2.55.0. If a floor that high is unacceptable, the portable equivalent is
  `cd "$(git rev-parse --git-common-dir)" && pwd`, NOT dropping the flag.

  -> Constraint: **do NOT detect a worktree by path prefix.** `.claude/worktrees/` is where
  `EnterWorktree` puts agent worktrees, but it is NOT where all worktrees live:
  `git worktree list` on this host shows `/Users/.../ze/gh-pages` as a linked worktree
  OUTSIDE that directory (executed 2026-07-16). A `case "$PWD" in */.claude/worktrees/*)`
  test would miss it and give `gh-pages` its own cache. `--git-common-dir` is correct for
  every worktree regardless of location, which is the reason to prefer it over a path test.
  Equivalent-but-worse alternative, recorded so it is not re-proposed: the first line of
  `git worktree list` is the main worktree, but it needs output parsing and gives nothing
  `--git-common-dir` does not.

  -> Constraint: **cache absent, or main checkout moved.** The resolved root is computed
  fresh from git on every invocation, so a MOVED main checkout is self-healing: nothing
  stores an absolute path, and the next resolve returns the new location. This is the
  decisive advantage over a committed symlink or a recorded path, both of which would
  dangle after a move. Two cases the implementation must still handle explicitly:
  (a) the resolved `cache/` does not exist yet -- create it (the main checkout may never
  have built), and a worktree creating it in the main checkout is EXPECTED, not a violation
  of the no-cross-tree-writes rule, because the cache is gitignored build output and not
  someone's working tree; (b) `git rev-parse` fails entirely (not a git repo, e.g. a
  released tarball) -- fail loudly per AC-6, or fall back to a checkout-local `cache/`, but
  never silently to a stale or stock artifact.

  -> Constraint: **this SOFTENS R-4 for worktrees ONLY, and AC-4's loud guard still
  matters.** A `git clean -xdf` inside a worktree can no longer destroy the cache, because
  the cache is not in the worktree. But `git clean -xdf` in the MAIN checkout destroys it
  for every worktree at once, which is strictly worse blast radius than before, not better.
  R-4 is therefore not retired: it is concentrated. The AC-6 guard (loud failure, never a
  silent fallback) is what keeps a wiped cache a rebuild rather than a silently-wrong
  artifact, and it is now the single point of protection for all trees.

  -> Constraint: **AC-8 (concurrent-run eviction safety) gets HARDER, not easier.** Sharing
  one cache across N worktrees means N concurrent QEMU runs contend on one root, so R-1's
  shared-mutable-state hazard now spans trees rather than being confined to one. AC-8 was
  already written for "two sessions or worktrees", so it does not change, but its mitigation
  (lock/refcount, evict only on explicit key change, never on a timer) is now load-bearing
  for the common case rather than an edge case.

  Superseded recommendation, kept for history: Option 1, on the "grep ze before proposing,
  do not invent what exists" rule (`ai/rules/design-context.md`). It also happens to
  satisfy AC-2 and R-4, which Option 2 cannot. If Option 2 wins, record why the repo now has two durable cache roots so the
  next reader does not treat it as an accident.
- ~~What is `tmp/qemu/build` (3.4G)?~~ ANSWERED 2026-07-16:
  `tools/kernel-builder/qemu-build.py:115-119` (`build_dir()` ->
  `tmp/qemu/build/<alpine_arch>`), with `ccache_dir()` at `:73-77`. Both are qemu-path
  build scratch, needed only while actually rebuilding. They STAY in `tmp/`, as the
  spec predicted.
- ~~**NEW, needs Thomas: does `make ze-kernel` stay Ze-binary-free?**~~ **ANSWERED 2026-07-16.
  NO -- it does not. See "Decision (user, 2026-07-16)" below. This decided the
  whole kernel half.**

  | Option | How | Cost |
  |--------|-----|------|
  | ~~**A. `make ze-kernel` calls `ze appliance kernel --target runtime`**~~ NOT CHOSEN | The cache logic already exists in Go and is already tested. Near-zero new code | Contradicts a RECORDED decision (`plan/learned/988-kernel-build-consolidation.md`: the make path must stay Ze-binary-free, which is why the fragment resolver is duplicated at all). Requires building a HOST `ze` binary with `-tags ze_core,ze_setup` first (`ze appliance kernel` lives in `ze-setup`, per `plan/learned/870-kernel-build-convergence.md:22`) |
  | ~~**B. `run.py` grows the same cache**~~ **REJECTED** | ~~Honours the Ze-binary-free constraint. Direct precedent: the fragment resolver is already duplicated Python/Go with a cross-language fixture guarding drift (A-8)~~ | A SECOND cross-language contract, this time on a hash key. **Drift here is silent and serves a stale kernel.** This cost is what killed it |
  | **C. `run.py` ASKS THE GO BINARY for the cache key** -- **CHOSEN 2026-07-16** | The make path keeps its structure and keeps owning the BUILD; it queries the host ze binary for the KEY rather than reimplementing `kernelCacheVariantFor`. Go stays the single source of truth for the key | `make ze-kernel` now requires a compiled HOST ze binary first. Reverses `learned/988`'s Ze-binary-free decision. See the owned constraint below |

  ### -> Decision (user, 2026-07-16): Option C. `run.py` asks the Go binary for the cache key

  > **READ THIS BEFORE "RESTORING" THE `learned/988` DECISION.**
  > **This DELIBERATELY REVERSES a recorded decision in
  > `plan/learned/988-kernel-build-consolidation.md`.** That summary's Decisions section
  > states, verbatim: *"Resolver duplicated in `run.py` (single python copy, make path) and
  > `kernelreg.go` (Go verified path) over a single resolver, **because the make path must
  > stay Ze-binary-free**; a cross-language fixture (`kernel-shared-fragment.ci` +
  > `TestResolveSharedInclude`) guards drift."* Thomas has overruled the Ze-binary-free
  > constraint **for the cache key**, with that decision in hand. It is not an oversight, not
  > a regression, and not a decision made in ignorance of 988. A future agent that finds
  > `run.py` invoking a ze binary and "fixes" it back to Ze-binary-free would silently
  > reintroduce the cross-language hash contract this decision exists to prevent. **Do not.**

  **Rationale (Thomas):** Go stays the **single source of truth** for the cache key
  (`kernelCacheVariantFor`, `internal/appliance/cache.go:110-120`, re-verified 2026-07-16:
  it keys on target + arch + profile name + a sha256 over every resolved config fragment and
  manifest + a sha256 over `build.py`/`run.py`/`ksource.py`). Asking for the key makes
  **cross-language drift impossible** rather than merely detectable. Option B's fixture
  detects drift after the fact; Option C removes the second implementation entirely, so
  there is nothing to drift. This is the decisive difference: a drifted hash key does not
  fail loudly, it **serves a stale kernel**, which is the exact class of bug this spec
  exists to kill (see the stale-extract bug in Task).

  -> Constraint (OWNED COST, user-accepted 2026-07-16): **`make ze-kernel` now requires a
  compiled HOST ze binary first.** This is the price of Option C and it is accepted, not
  discovered later. Per CLAUDE.md "Binary naming convention" (read 2026-07-16), that binary
  is:
  - `cmd/ze` built with tags **`ze_core,ze_setup`** (`ze appliance kernel` lives under
    `ze_setup`), and
  - **NO `GOOS`/`GOARCH` override.** It must execute on the build host.
  - Named **`ze-host`** by convention.
  - The **target arch is passed as an argument** to that host binary, and is **NEVER**
    applied to the host tool's own build. A target-arch `ze-host` cannot exec on the build
    host ("exec format error").

  **Precedent, verified 2026-07-16:** `scripts/evidence/effective-install-qemu.py:153-168`
  is `build_host_ze(root, work)`, whose docstring reads *"Build a HOST ze binary (runs on
  the build machine, not the target). **No GOARCH override: must execute on this host.** The
  appliance commands (init/build/initrd) are under ze_setup."* and whose body is
  `go build -tags ze_core,ze_setup -o ze-host ./cmd/ze` (cwd = repo root). Copy that shape.

  -> Constraint: this trap is **live here specifically** because the artifact being built IS
  an arm64/amd64 kernel, so a target arch is already in hand and trivially applied to the
  wrong thing. The arch belongs in the ze-host **argument**, never in its `go build`.

  -> Constraint: at closure, `plan/learned/988-kernel-build-consolidation.md` needs a
  **superseding note** recording that its Ze-binary-free decision was reversed here, and by
  whom, so the next reader of 988 finds the reversal at the source rather than trusting a
  stale rule. Per `ai/rules/planning.md` a learned summary records what was decided THEN and
  must not be rewritten; the correct form is an **additive correction pointer**, exactly as
  988 itself already carries one for `plan/learned/870-kernel-build-convergence.md` (see
  988's own Files section). **Not actioned by the recording session** (2026-07-16): 988 was
  explicitly out of scope to edit. This is a closure obligation, not a suggestion.

  -> Constraint: Option C does NOT make the make path call `ze appliance kernel` (that was
  Option A, not chosen). `run.py` still owns the BUILD and the docker/qemu selection that
  988 consolidated into it. Only the **key** is delegated. The distinction matters: 988's
  "one driver" consolidation stands; only its "Ze-binary-free" corollary is reversed.

  -> Constraint (CLAUDE.md "Binary naming convention"): if ~~Option A is chosen~~ **any
  option invoking a host ze binary is chosen (Option C is)**, the ze
  binary that runs `ze appliance kernel` is a **HOST** binary and MUST NOT be
  cross-compiled. Build it with no `GOOS`/`GOARCH` override, name it `ze-host` by
  convention, exactly as `scripts/evidence/effective-install-qemu.py:153-168` does
  (`go build -tags ze_core,ze_setup -o ze-host ./cmd/ze`, with the docstring "No GOARCH
  override: must execute on this host"). The TARGET arch is passed as `--arch <arch>`
  to that host binary, never applied to its own build. A target-arch `ze-host` cannot
  exec on the build host ("exec format error"). This trap is live here precisely
  because the thing being built IS an arm64/amd64 kernel, so the arch is already in
  hand and easy to apply to the wrong thing.
- ~~Do the Go caches and ccache move too?~~ ANSWERED by the user, 2026-07-16: no. They
  stay in `tmp/` precisely because they grow and get cleared often. Only the ~200 MB
  of expensive, version-keyed artifacts moves. Do not move anything else reflexively:
  the goal is to split durable from disposable, not to empty `tmp/`.
- Eviction policy: keep-exactly-one (matches "clear old kernel when we update", bounds
  the cache, punishes bisect) versus keep-N. Kernel updates every few weeks make this
  a live tradeoff rather than academic.
  -> AUTONOMOUS DEFAULT (2026-07-17): **keep-N with N=2, keyed by version/config, never by
  wall-clock timer.** Rationale: adopts R-2's own recommendation ("Keep-N (2 is probably
  enough) keyed by version/config, not wall-clock age") -- the spec already records this as
  the mitigation, so this is the "adopt the RECOMMEND" default, not a new call. N=2 bounds
  the cache while sparing a bisect across a single version bump the cost R-2 warns about;
  timer-free, key-change-only eviction is exactly what R-1/AC-8 require so eviction can never
  race a live QEMU boot. Applies to the whole `~/.cache/ze` tree (installer-kernel,
  installer-initrd, runtime-kernel) since A-7 confirmed none of them evict today. Thomas:
  override to keep-exactly-one if the tighter bound is preferred over bisect comfort.
- Should the ISO be pinned by digest rather than version string, making the key and
  the integrity check the same thing?
  -> AUTONOMOUS DEFAULT (2026-07-17): **NO -- keep version-string keying plus published
  `.sha256` verification; do not pin the ISO by digest.** Rationale: the smaller, more
  reversible option. `downloadAndVerify` (`cache.go:173-230`) already hashes-while-streaming
  and rejects on mismatch against the published checksum, and A-4 confirmed Alpine's
  `.sha256` format parses as-is, so integrity is fully covered (AC-5) without coupling the
  cache key to a digest. Digest-pinning is a later hardening (it defends against a mirror
  serving a different-but-valid image under the same version), required by no AC here, and
  can be layered on without reworking the key. Thomas: override if supply-chain pinning is
  wanted now.
- Is there a shared cache convention already in this repo worth matching, or is this
  the first durable cache? If it is the first, it sets the precedent, so name it
  deliberately.
  -> RESOLVED (2026-07-17): already answered by the research above -- this is **NOT** the
  first durable cache. `~/.cache/ze` is in production (`resolveCacheDir` `cache.go:47-57`;
  namespaces `installer-kernel`/`runtime-kernel`/`installer-initrd` at `:21-33`). Thomas
  chose Option 2 (repo-local `cache/`) deliberately anyway, with that fact in hand (see the
  "Cache root?" decision above). This stale skeleton bullet is superseded by that decision
  and by the folder-name default (`cache/` at repo root); kept for history, not reopened.

## Handoff Contract: what `spec-fixit-qemu-runtime-kernel` consumes

That spec declares `Depends | spec-fixit-qemu-artifact-cache`. This section is the
interface between them, so they can be implemented in sequence without rework. If any
row changes during implementation, update the runtime-kernel spec in the SAME commit.

| # | This spec delivers | The runtime-kernel spec consumes it as | Why it cannot be built there |
|---|--------------------|-----------------------------------------|------------------------------|
| 1 | `tmp/kernel/vmlinuz` keeps its path and meaning, now materialisable from the durable cache | The three existing labs and the two new `--kernel` call sites need no path change at all | Moving it would touch ~20 sites (A-5 broken) |
| 2 | A make target that ENSURES the kernel exists: cache hit materialises in seconds, miss builds. Name it here, e.g. `ze-kernel-ensure` | The prerequisite that makes `ze-qemu-all-test` runnable without a manual ~30-minute prelude. This is the answer to that spec's R-2, its only real blocker | It is a cache concern; duplicating it there would fork the policy |
| 3 | The cache key includes **arch** (`cache.go:110-120`) and the make path honours it | Its R-6 (kernel/ISO arch mismatch) stops being a hand-passed `GOKRAZY_ARCH=arm64` workaround. See A-6: today a bare `make ze-kernel` stages an amd64 kernel that the `test -f` guard happily accepts | The arch keying lives in the cache variant |
| 4 | The cache key hashes the config fragments + builder scripts (`cache.go:110-120`) | **Its AC-9 is delivered here, in full.** That spec should DROP AC-9 and cite this row: a `runtime.config` edit already misses the cache | It is the same key; two implementations would drift |
| 5 | Artifacts survive `rm -rf tmp/` (AC-1) and superseded ones are reclaimed (AC-4) | **Its AC-6, AC-7 and AC-8 are delivered here.** That spec should drop all three and cite this row | Same mechanism, same artifact class |
| 6 | A guard that fails loudly and actionably when the kernel is absent (AC-6) | Its AC-4 (never silently fall back to stock) reuses this guard verbatim | Same pattern, one implementation |

-> Decision: after this contract, the runtime-kernel spec keeps only AC-1 (does 7.1.1
fix the nft crash), AC-2 (firewall runs by default), AC-3 (both targets boot the
runtime kernel) and AC-5 (no suite regresses). Everything else is cache work. That is
the whole reason it declares a dependency, and it is why this spec lands first.

## Checklist

- [ ] Tests written
- [ ] Tests FAIL (red) before implementation
- [ ] Tests PASS (green) after implementation
- [ ] `make ze-test` green
- [ ] The stale-extract bug has a regression test (AC-3)
- [ ] `rm -rf tmp/` costs nothing (AC-1)
- [ ] Eviction cannot race a live QEMU boot (AC-8)
- [ ] ~~Thomas has settled the cache root (Option 1 vs 2) and the Ze-binary-free
      question (Option A vs B) before any code is written~~ **DONE 2026-07-16: cache root =
      Option 2 (repo-local `cache/`); make path = Option C (`run.py` asks the host ze binary
      for the key), reversing `plan/learned/988`'s Ze-binary-free decision**
- [ ] `run.py` obtains the cache key from `ze-host`, never reimplements
      `kernelCacheVariantFor`; no second key implementation exists anywhere (the Option C
      guarantee)
- [ ] `ze-host` is built with `-tags ze_core,ze_setup` and NO `GOOS`/`GOARCH` override; the
      target arch is passed as an argument to it (CLAUDE.md "Binary naming convention")
- [ ] `plan/learned/988-kernel-build-consolidation.md` carries an additive correction
      pointer recording that its Ze-binary-free decision was reversed by this spec
      (closure obligation; NOT done at decision-recording time)
- [ ] No consumer-facing path moved; all ~20 `tmp/kernel` consumers still work (AC-7)
- [ ] Every cache test redirects `XDG_CACHE_HOME`; none can touch the real cache
- [ ] The handoff contract above matches what was actually built
