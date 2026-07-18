# spec-relocate-scratch-and-cache

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Supersedes | spec-fixit-qemu-artifact-cache (durable-cache half absorbed here) |
| Phase | 1/6 (implementation) |
| Updated | 2026-07-18 |

> **DESIGN, 2026-07-18.** This spec absorbs the durable-cache work of
> `spec-fixit-qemu-artifact-cache` (fully researched, status `ready`) and adds a second,
> parallel relocation of the disposable scratch tree. Both follow one shape: **the real
> directory lives OUTSIDE the checkout; the repo holds only a symlink to it, created at
> build/session start.** The cache half is low-risk. The scratch half fights three
> documented, load-bearing constraints and therefore carries an explicit "pinned
> exceptions" design. All design questions below were settled by the user across the
> 2026-07-18 session (see Design Decisions). Awaiting approval before implementation.

## Task

Make the repository working tree hold **no heavy real directories**. Today two expensive,
non-source trees live inside the checkout:

1. `tmp/` (6.7 GB measured): Go build caches, ccache, QEMU build scratch, the Alpine ISO
   + extract, the runtime kernel materialised view, plus session/commit/verify machinery.
2. The kernel and Alpine ISO are the expensive, rarely-changing artifacts (~30 min to
   rebuild the kernel; 76 MB ISO download) and currently die with `rm -rf tmp/`.

Two relocations, one shape:

- **Durable cache** -> real at `~/.cache/ze` (already in production, `cache.go:47-57`);
  repo `cache` symlink to it, created at build if missing. Version- and arch-keyed,
  checksum-verified, shared across worktrees and clones, survives `git clean`. The make
  kernel path and the Alpine ISO are routed through it so `rm -rf tmp/` costs a copy, not a
  rebuild. Eviction reclaims superseded artifacts.
- **Disposable scratch** -> real at `$TMPDIR/ze/<checkout-id>` (fallback
  `/tmp/ze/<checkout-id>`), per-checkout; repo `tmp` symlink to it, created at build/session
  start if missing. Deleting the symlink (or letting the OS clear `$TMPDIR`) is always safe.
  Three things that MUST NOT move into `$TMPDIR` are pinned to a real repo-internal location.

After both, the checkout contains two symlinks (`tmp`, `cache`) and the real state is
outside it. `rm -rf tmp/` (or a full `$TMPDIR` wipe) never destroys anything expensive.

## Origin

- 2026-07-16..17: `spec-fixit-qemu-artifact-cache` researched the durable-cache half in
  full (status `ready`). User direction: "we should not have valuable cache in tmp as we
  must be able to delete it all"; "clear old kernel/image when we update the kernel
  version"; "do the same for the alpine iso cache".
- 2026-07-18 (this session), the user extended the ask in four steps:
  1. Route the cache through the existing `~/.cache/ze` rather than a new repo-local
     `cache/` folder (reverses the old spec's Option 2 decision).
  2. "create a symlink from the repo to the cache folder when building if it does not
     exist so the user can see that there is a cache folder outside the folder".
  3. "we should use the OS tmp folder and /tmp/ze as the tmp folder and have a symlink to
     it in the repo".
  4. On being shown the conflicts (below): **relocate tmp, but pin the exceptions**;
     per-checkout target; `$TMPDIR/ze` with `/tmp/ze` fallback; author as one new combined
     spec superseding the old one.

## Design Decisions (settled by the user, 2026-07-18)

| # | Decision | Rationale / owned cost |
|---|----------|------------------------|
| D-1 | **Cache root = existing `~/.cache/ze`** (`resolveCacheDir`, `cache.go:47-57`), NOT a new repo-local `cache/` folder | One durable root, already shared across worktrees/clones, survives `git clean`, already `XDG_CACHE_HOME`-redirectable for tests. Reverses old spec's Option 2 |
| D-2 | **Repo `cache` symlink -> resolved `~/.cache/ze` dir**, created at build if missing, gitignored | Visibility: the user sees a `cache/` in the repo that points outside it. No duplication |
| D-3 | **Repo `tmp` symlink -> `$TMPDIR/ze/<checkout-id>`** (fallback `/tmp/ze/<checkout-id>`), created at build/session start if missing, gitignored | OS temp is auto-cleaned; `rm -rf tmp` is a no-op on real data. `$TMPDIR` (macOS `/var/folders/...`) avoids the hook `/tmp` string guards; `/tmp/ze` only as fallback |
| D-4 | **Per-checkout `<checkout-id>`** derived from `git rev-parse --show-toplevel` (hash) | Each worktree/checkout gets its OWN scratch so different branches/arches never collide on `tmp/kernel/build`, `tmp/qemu/build`, session/commit files. Already the key source in `commit_helper.py:95`, `spec-session.sh:33`. NEVER key on `--git-common-dir`/repo-name/HEAD (collapses all worktrees onto one) |
| D-5 | **CORRECTED 2026-07-18 (user): NO `.zcache`. GOCACHE/GOLANGCI_LINT_CACHE stay in `tmp/` exactly as before (unchanged).** The Makefile "not TMPDIR - breaks Unix socket tests" note describes a TEST bug (a socket path exceeding `sun_path` under a long `$TMPDIR`), NOT a cache-location requirement. If a socket test actually breaks on a symlinked tree, fix that ONE test's socket path; never constrain the cache location or invent a third directory. Container/VM mounts keep using `tmp/`; if the symlink dangles inside a mount, fix that specific consumer minimally, when it actually bites | Simplicity: two symlinks (`tmp`, `cache`), no third dir. The earlier "pin the exceptions" / `.zcache` design was over-engineering, reverted |
| D-6 | **Make kernel path routed through the cache via Option C**: `run.py`/`mk` ask a HOST `ze-host` binary for the cache key/dir; Go stays the single source of truth for the key (`kernelCacheVariantFor`, `cache.go:110-120`) | Cross-language drift impossible (not merely detectable). Reverses `plan/learned/988`'s Ze-binary-free rule deliberately (closure obligation: add correction pointer to 988). Owned cost: `make ze-kernel` requires building `ze-host` (`-tags ze_core,ze_setup`, NO GOARCH override) first |
| D-7 | **Eviction = keep-N (N=2), keyed by version/config, never by timer**, applied to the whole `~/.cache/ze` tree | Adopts old spec R-2's own recommendation. Timer-free, key-change-only eviction cannot race a live QEMU boot (AC-8) |
| D-8 | **ISO: version-string key + published `.sha256` verification** (do NOT pin by digest) | Reuse `downloadAndVerify` (`cache.go:173-230`). Digest-pinning is later hardening, no AC needs it |
| D-9 | **No consumer-facing path moves.** `tmp/kernel/vmlinuz`, `tmp/kernel/build`, `tmp/kernel/pkg` keep their spellings (now resolving through the `tmp` symlink); ~20 consumers unchanged | Old spec A-5 (BROKEN): the consumer set is ~20 sites, so moving spellings is the wrong lever. Materialised-view pattern, exactly as `resolveRuntimeKernel` already does |
| D-10 | **Conservative bootstrap + explicit one-time cutover.** ensure-links creates a symlink ONLY when `tmp`/`cache` is absent or already a symlink. If it finds a REAL populated `tmp/` (existing checkouts), it does NOT auto-convert; a separate explicit `make ze-migrate-scratch` performs the careful cutover (move contents to the target, then swap in the symlink), refusing if it looks busy | never-destroy-work + concurrent-session safety: the live main tree may be in use by other worktree agents. Fresh checkouts/worktrees get symlinks automatically; existing ones migrate on demand |

## GOCACHE and mounts (CORRECTED 2026-07-18 — no `.zcache`)

Only the sentinel is genuinely special. Everything else stays as-is; there is NO third
directory:

1. **Go build caches STAY in `tmp/` (unchanged).** `Makefile:14-15` keep `GOCACHE`/
   `GOLANGCI_LINT_CACHE` at `$(CURDIR)/tmp/...`. The `Makefile:13` note "not TMPDIR - breaks
   Unix socket tests" describes a TEST bug (a socket path exceeding `sun_path` under a long
   `$TMPDIR`), NOT a cache-location requirement. Do not repoint the cache. If a socket test
   actually fails on a symlinked tree, fix THAT test's socket path (short/relative), which is
   the real bug.
2. **Container/VM scratch keeps using `tmp/`** (`/src/tmp`, `/workspace/tmp`). On a migrated
   (symlinked) tree the host `tmp` symlink can dangle inside a bind/9p mount. Handle that ONLY
   if it actually bites, by fixing that one consumer minimally (e.g. a guest-local cache dir
   for that target), never with a global third directory.
3. **`tmp/go.mod` sentinel — KEPT (corrected 2026-07-18).** It stays TRACKED and is
   maintained by `scripts/dev/ensure-links.py` (`ensure_sentinel`, replacing the old
   `git show HEAD:tmp/go.mod` step in `make clean`). The default tree has a REAL `tmp/`,
   which `go list ./...` WOULD descend without the sentinel (verified: deleting it makes
   `go list` fail), so it is still required there. A-6 applies ONLY to the opt-in migrated
   case: a symlinked `tmp/` is skipped with no sentinel. There is NO `git rm tmp/go.mod`.

Everything under `tmp/` (Go caches, session/commit/verify files, mutation logs, ISO extract,
kernel materialised view, etc.) resolves through the symlink.

## Required Reading

### Durable-cache half (from the superseded spec, re-verify at implementation)
- [ ] `internal/appliance/cache.go:47-57` `resolveCacheDir()`, `:21-33` namespaces,
      `:110-120` `kernelCacheVariantFor` (target+arch+config-hash+builder-script-hash),
      `:173-230` `downloadAndVerify`, `:306-312` `copyTree` (RemoveAll on dst only, no evict).
- [ ] `internal/appliance/cmd_kernel.go:263-287` `resolveKernel`, `:336-366`
      `resolveRuntimeKernel` (hit copies cache->tmp/kernel/build; miss builds then
      copies tmp->cache), `:43` `runtimeKernelOutputDir = "tmp/kernel/build"`.
- [ ] No entrypoint prints the cache key/dir without building (grep confirmed 2026-07-18):
      Option C needs a new lightweight `ze appliance kernel ... --print-cache-dir` (or
      equivalent) reusing the existing `--arch/--target/--profile/--version` flags to print
      `kernelTreeCachePath(version, kernelCacheVariantFor(...))`.
- [ ] `scripts/evidence/qemu-run.py:84-92` `cache_dir()`, `:99-115` `ensure_iso()`
      (version-keyed name, existence-only hit, `curl -fSL` no checksum), `:118-127`
      `_extract_dir_for()` (extract keyed by ISO stem, stale-extract bug already FIXED).
- [ ] `tools/kernel-builder/qemu-build.py:155-170` second `ensure_iso()`, `:36-37`
      duplicated `ALPINE_VERSION`/`ALPINE_MINOR`, `:73-77`/`:115-119` ccache/build scratch.
- [ ] `mk/gokrazy.mk:194-200` `ze-kernel` staging, `:226-239` `ze-kernel-clean` (`rm -rf
      tmp/kernel`), `:180-192` cache/pkg dir vars.
- [ ] `plan/learned/988-kernel-build-consolidation.md` Ze-binary-free decision (being
      reversed; closure obligation to add a correction pointer).

### Scratch-relocation half (blast-radius research, 2026-07-18, cite at implementation)
- [ ] Sentinel: `tmp/go.mod` is the ONLY tracked path under `tmp/` (`git ls-files tmp/`);
      `Makefile:18-25` `$(TMP_SENTINEL)` target, `:249`/`mk/test-unit.mk:15` prereqs;
      `qemu-run.py:49-92` `TMP_SENTINEL`/`ensure_tmp_sentinel`; `plan/learned/985:47-54`.
- [ ] `.gitignore:9-13` `tmp/*` + `!tmp/go.mod` (a symlink named `tmp` is matched by
      pattern `tmp`, NOT `tmp/*`); `:56-63` session-state comment.
- [ ] `Makefile:13-15` GOCACHE/lint pin; `:464-468` `clean` (`rm -rf tmp/`, `mkdir -p tmp`,
      `git show HEAD:tmp/go.mod`); `:470-477` `ze-clean-tmp`; `:220-231` `ze-linux-test`
      container GOCACHE; `:199-200` kernel staging.
- [ ] `mk/gokrazy.mk:38,69-127,181-200,238`, `mk/test-mutation.mk:28-123`,
      `mk/terminal-demo.mk:11`, `mk/appliance.mk:116-118`, `mk/test-integration.mk:410-459`
      (all write under `tmp/`; mkdir-materialization hazard).
- [ ] Session/commit/verify: `commit_helper.py:95-98,213-231,435,523,882-883`,
      `spec-session.sh:33-73`, `.claude/hooks/lib/state-file.sh:17-88`,
      `verify-status.sh:20,60`, `spec-closure-check.py:259`.
- [ ] Hooks: `pretool-writeedit.py:28,1217-1248,1314-1379,1648-1657,1990-1994`;
      `pretool-bash.py:51-68,135-143`; `.claude/settings.local.json:117`
      (`mkdir -p tmp/session` allow-rule = materialization vector).
- [ ] Runtime relative writes: `cmd_edit.go:79` (ephemeral-ssh addr FILE, TCP not socket),
      `cmd_kernel.go:43`, `internal/test/runner/timing.go:18`.
- [ ] Container mounts: `qemu-run.py:399-413`, `docker-run.py:62,70` + `Makefile:225-229`.
- [ ] Docs/rules describing tmp as a real dir: `CLAUDE.md`/`AGENTS.md:16,32` (generated,
      edit `ai/` source), `ai/rules/git-safety.md:15,50-64,129,208-234,322,328,374`,
      `ai/rules/testing.md:99,171,210`, `ai/rules/never-destroy-work.md:22`,
      `ai/rules/bash-output.md:49-75`, `docs/functional-tests.md`, `docs/guide/appliance.md`,
      `.claude/rules/{post-compaction,session-start,planning}.md`.

## Current Behavior (MANDATORY)

**Durable cache already exists and is bypassed by the make path.** `resolveCacheDir()`
returns `$XDG_CACHE_HOME/ze` or `~/.cache/ze` (`cache.go:47-57`). `ze appliance kernel
--target runtime` routes the runtime kernel tree through it (`resolveRuntimeKernel`,
`cmd_kernel.go:336-366`), but `make ze-kernel` calls `gokrazy/kernel/Makefile` -> `run.py`
-> `tmp/kernel/build` directly (`mk/gokrazy.mk:197`), never consulting the cache. On disk:
`~/.cache/ze/installer-kernel/` exists; `~/.cache/ze/runtime-kernel/` does not. `cache.go`
has NO eviction (`copyTree` RemoveAlls the destination only, `:306-312`).

**ISO is unverified and duplicated.** `qemu-run.py:99-115` and `qemu-build.py:155-170` are
two independent `ensure_iso()` producers, both `curl -fSL` with no checksum, both writing
into `tmp/qemu/iso`, with `ALPINE_VERSION`/`ALPINE_MINOR` duplicated. Alpine publishes a
`.sha256` that `downloadChecksum` (`cache.go:232-257`) parses as-is (confirmed 2026-07-16).

**Scratch tree is real and full of assumptions.** `tmp/` is a real gitignored directory
with one tracked file, `tmp/go.mod`, whose sole job is to stop `go list ./...` descending
into the Go caches under it. Session state, commit scripts, verify status, and per-checkout
isolation all assume `tmp/` is a real, per-checkout directory. Details in Required Reading.

**Behavior to preserve:**
- `tmp` stays safe to delete wholesale (now trivially: it is a symlink).
- The build stays reproducible: the cache is an optimisation, never the only copy.
- Consumers keep failing loudly when an artifact is absent (`test -f` guards).
- `go list ./...` still skips the Go caches (sentinel or symlink-skip; see AC-11).
- Every consumer-facing `tmp/kernel/*` path keeps its spelling (D-9).
- GOCACHE stays out of `$TMPDIR` and inside the container mount (D-5).
- Per-checkout isolation of commit scripts / verify status / session markers (D-4).

## Data Flow

### Entry points
- Any QEMU/Docker target invoking `qemu-run.py`/`docker-run.py`; `make ze-kernel`; any
  `make`/hook/script that first touches `tmp/` or `cache/`.

### Transformation path (target)
- **Bootstrap chokepoint:** a single "ensure links" step (a make prerequisite + a helper
  invoked by hooks/scripts) resolves `<checkout-id>`, creates `$TMPDIR/ze/<id>` and
  `~/.cache/ze` if absent, and creates the repo `tmp`/`cache` symlinks if absent. It runs
  before any `mkdir -p tmp*` so the materialization hazard cannot fire.
- **Kernel:** `make ze-kernel` asks `ze-host` for the cache dir (Option C); hit ->
  `copyTree(cache -> tmp/kernel/build)` + stage; miss -> build -> populate cache. `tmp/` is
  a materialised view.
- **ISO:** one keyed, verified helper shared by both producers; one `ALPINE_VERSION`.
- **Eviction:** on a resolved key change, keep-N=2 keyed entries, timer-free.

### Boundaries crossed
| Boundary | Crossing | Divergence consequence |
|----------|----------|------------------------|
| Scratch symlink -> `$TMPDIR` | repo `tmp` resolves outside the checkout | container mounts dangle unless guest-local scratch is used (D-5) |
| GOCACHE placement | must be repo-internal + non-`$TMPDIR` | Unix-socket tests break; container cache unwritable |
| Durable cache -> concurrent worktrees | one shared `~/.cache/ze` across N trees | eviction must never race a live boot (AC-8) |
| Kernel build -> ~20 consumers | spellings preserved (D-9) | moving any breaks labs/`.ci`/docs |
| Tracked sentinel vs symlink | git cannot hold both | must untrack + regenerate (AC-11) |

## Wiring Test

Note: `.ci` functional tests are N/A as the change surface (this is build/test infra). Proof
is that the QEMU/Docker suites still run and the unit tests below.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make <anything>` on a fresh checkout with no `tmp`/`cache` | -> | ensure-links bootstrap creates both symlinks + targets before any `mkdir -p tmp*` | `test_bootstrap_creates_symlinks` |
| `rm -rf $(readlink tmp)` then `make ze-kernel` | -> | rematerialise kernel from `~/.cache/ze` in seconds | `test_scratch_wipe_preserves_artifacts` |
| A second worktree | -> | its own `$TMPDIR/ze/<id2>`, shared `~/.cache/ze` | `test_per_checkout_scratch_isolation` |
| `qemu-run.py` cold cache | -> | verified `ensure_iso` via `downloadAndVerify` semantics | `test_cold_cache_downloads_and_verifies` |
| `ALPINE_VERSION`/kernel key bumped | -> | keep-N=2 eviction of superseded key | `test_version_bump_evicts_superseded` |
| `make ze-qemu-l2tp-ppp-test` with no kernel | -> | `test -f tmp/kernel/vmlinuz` guard fails loudly | `test_missing_kernel_fails_loudly` |
| `go list ./...` on the relocated tree | -> | Go skips the caches (sentinel or symlink-skip) | `test_go_list_skips_caches` (AC-11) |

## TDD Test Plan

### Unit / harness tests
| Test | File | Validates |
|------|------|-----------|
| `qemu-run.py --selftest` (EXISTS) | `scripts/evidence/qemu_run_test.go:27` | extract keying; keep green. Extend for eviction |
| `test_go_list_skips_caches` | new `.ci` or Go test | AC-11: relocated tree does not break `go list ./...` |
| `test_bootstrap_creates_symlinks` | new `.ci` under `test/install/` | AC-1/AC-12: fresh checkout gets both symlinks + targets; no real `tmp/` materialises |
| `test_scratch_wipe_preserves_artifacts` | new `.ci` | AC-1: wipe scratch, rematerialise costs a copy. Redirect `XDG_CACHE_HOME` to a temp dir |
| `test_per_checkout_scratch_isolation` | new | AC-4/D-4: two `--show-toplevel` values -> two scratch dirs |
| `test_cold_cache_downloads_and_verifies` | new | AC-5: checksum mismatch fails loudly, no cache populate |
| `test_corrupt_cached_artifact_is_rejected` | new | AC-5: existence != integrity |
| `test_run_py_key_from_ze_host_or_fails_loud` | new | AC-9/D-6: `run.py` gets key from `ze-host`; absent binary fails loud, no guessed key |
| `test_missing_kernel_fails_loudly` | new | AC-6 |
| `test_kernel_clean_keeps_durable_cache` | new | AC-10 |
| _(dropped)_ `test_gocache_not_in_tmpdir` | - | Removed with the `.zcache` design (D-5 corrected). GOCACHE stays in `tmp/`; no location assertion needed |

Every cache test MUST set `XDG_CACHE_HOME` to a temp dir (`cache.go:48`) so it can never
evict the developer's real `~/.cache/ze`.

### Functional tests
The existing QEMU/Docker suites ARE the functional test: they must still run and pass after
the relocation (AC-7). Precedent for `XDG_CACHE_HOME` redirection:
`effective-install-scenarios-qemu.py:99-129`.

## Files to Modify / Create

**Create**
- [ ] `scripts/dev/ensure-links.sh` (or `.py`) - the bootstrap chokepoint: resolve
      `<checkout-id>`, create `$TMPDIR/ze/<id>` + `~/.cache/ze`, create `tmp`/`cache`
      symlinks, regenerate the sentinel. Idempotent, safe to call from make + hooks.
- [ ] New `ze appliance kernel ... --print-cache-dir` entrypoint (Option C key query).
- [ ] New `.ci`/unit tests per the TDD plan.

**Modify (cache half)**
- [ ] `internal/appliance/cache.go` - keep-N=2 eviction; optional `alpine-iso` namespace.
- [ ] `internal/appliance/cmd_kernel.go` - the `--print-cache-dir` flag.
- [ ] `tools/kernel-builder/run.py` - obtain the key from `ze-host`; route materialise/populate.
- [ ] `tools/kernel-builder/qemu-build.py` - drop private `ensure_iso`/`ALPINE_VERSION`.
- [ ] `scripts/evidence/qemu-run.py` - shared verified ISO helper; guest-local scratch (D-5).
- [ ] `mk/gokrazy.mk` - `ze-kernel` materialises from cache; `ze-kernel-clean` never touches cache.

**Modify (scratch half)**
- [ ] `Makefile` - ensure-links prerequisite; GOCACHE/lint stay in `tmp/` (unchanged); fix
      `clean` and `ze-clean-tmp`; drop `git show HEAD:tmp/go.mod`.
- [ ] `.gitignore` - ignore the `tmp` and `cache` symlinks; remove the dead
      `!tmp/go.mod` re-inclusion; update comments.
- [ ] `tmp/go.mod` - stays TRACKED; only its comment is updated to name `ensure-links.py` as
      its maintainer (no `git rm`).
- [ ] `scripts/evidence/qemu-run.py`, `tools/kernel-builder/qemu-build.py` - sentinel writer;
      guest-side scratch redirects.
- [ ] `.claude/hooks/lib/state-file.sh`, `scripts/dev/spec-session.sh`,
      `scripts/dev/verify-status.sh`, `scripts/dev/commit_helper.py` - call ensure-links (or
      tolerate a symlinked `tmp`); keep per-checkout key on `--show-toplevel`.
- [ ] `mk/*.mk` writing under `tmp/` - ensure-links ordering; no `mkdir -p tmp` before the link.
- [ ] `.claude/settings.local.json` - the `mkdir -p tmp/session` allow-rule (materialization).

**Docs/rules (discovery-updates obligation)**
- [ ] `ai/rules/git-safety.md`, `ai/rules/testing.md`, `ai/rules/bash-output.md`,
      `ai/rules/never-destroy-work.md`, `ai/rules/qemu-testing.md`,
      `docs/functional-tests.md`, `docs/guide/appliance.md`, `docs/architecture/testing/
      qemu-integration.md`, `.claude/rules/{post-compaction,session-start,planning}.md`,
      and the `ai/` sources that generate `CLAUDE.md`/`AGENTS.md`.
- [ ] `plan/learned/988-kernel-build-consolidation.md` - additive correction pointer (D-6).

## Implementation Phases

1. **Bootstrap + scratch symlink (lowest layer).** ensure-links helper; make prerequisite;
   sentinel KEPT (tracked) + maintained by ensure-links; `.gitignore`; `make clean`/`ze-clean-tmp` fixes;
   `test_go_list_skips_caches`, `test_bootstrap_creates_symlinks`. Prove `go list ./...`
   and `make ze-unit-test` still green.
2. **(Corrected) No pinning.** GOCACHE/lint stay in `tmp/`. Prove a QEMU/Docker target runs;
   if a socket test fails on a symlinked tree, fix that ONE test's socket path (not the cache).
3. **Cache symlink + route make kernel path (Option C).** `--print-cache-dir`; `run.py`
   key query; `ze-host` build; materialise/populate; `test_run_py_key_from_ze_host_or_fails_loud`,
   `test_scratch_wipe_preserves_artifacts`. Prove AC-1.
4. **ISO unification + verification.** one helper, one `ALPINE_VERSION`, `.sha256`;
   `test_cold_cache_downloads_and_verifies`, `test_corrupt_cached_artifact_is_rejected`.
5. **Eviction.** keep-N=2, key-change-only, concurrency-safe; `test_version_bump_evicts_superseded`.
6. **Docs/rules + 988 pointer.**

## Acceptance Criteria

| AC | Input / Condition | Expected Behavior |
|----|-------------------|-------------------|
| AC-1 | `make ze-kernel`, wipe scratch, `make ze-kernel` | Second run materialises from `~/.cache/ze` in seconds, no ~30-min rebuild |
| AC-2 | Fresh clone or new worktree on the same host | Reuses the cached kernel + ISO (shared `~/.cache/ze`), no rebuild |
| AC-3 | `ALPINE_VERSION`/`ALPINE_MINOR` bumped | Extract misses and re-extracts. Already satisfied by `_extract_dir_for`; keep the selftest green |
| AC-4 | Two worktrees | Each has its own `$TMPDIR/ze/<id>` scratch; commit scripts / verify status / session markers never collide |
| AC-5 | Downloaded or cached ISO fails its checksum | Fails loudly, not served, cache not populated |
| AC-6 | Any artifact absent | Actionable failure; never a silent fallback |
| AC-7 | Every existing QEMU/Docker target after the change | Still runs and passes; all ~20 `tmp/kernel/*` consumers work |
| AC-8 | Concurrent QEMU runs while eviction runs | Eviction never deletes an artifact a live run is using |
| AC-9 | `make ze-kernel` and `ze appliance kernel --target runtime` for the same arch+config | Both resolve the SAME cache entry (one key implementation, D-6) |
| AC-10 | `make ze-kernel-clean` | Clears the materialised view only; `~/.cache/ze` survives |
| AC-11 | `go list ./...` / `make ze-unit-test` on the relocated tree | Still skips the Go caches; no "outside main module" error |
| AC-12 | Fresh checkout, first `make` | `tmp` and `cache` symlinks + targets created before any `mkdir -p tmp*`; no real `tmp/` dir materialises |
| AC-13 | `rm -rf tmp` (the symlink), or a full `$TMPDIR` wipe | Harmless: no expensive artifact lost; next build recreates the link and rematerialises |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Status |
|----|-----------|-------|----------|--------|
| A-1 | Extract stale-hit bug already fixed | `qemu-run.py:118-127`, selftest green | n/a | confirmed |
| A-2 | Nothing evicts today | `cache.go` has no evict path | n/a | confirmed |
| A-3 | Alpine `.sha256` parses as-is | `cache.go:232-257`, fetched 2026-07-16 | pin by digest | confirmed |
| A-4 | ~20 consumers depend on `tmp/kernel/*` spellings | grep, old spec A-5 | design changes | confirmed |
| A-5 | GOCACHE must be repo-internal + non-`$TMPDIR` + in-mount | `Makefile:13`, `:220-229`, `qemu-run.py:407` | pinning is wrong | **confirmed 2026-07-18** |
| A-6 | `go list ./...` behaviour on a symlinked `tmp` | **CONFIRMED 2026-07-18** by empirical test: `go list ./...` (same `./...` matcher as build/test/vet) does NOT descend a directory symlink; a foreign `go.mod` reachable via a `tmp` symlink is skipped (exit 0). BUT the default tree has a REAL `tmp/`, where `go list` DOES descend and fails without the sentinel (also verified) | **Corrected conclusion:** keep the sentinel TRACKED, maintained by `ensure-links.py` for the real-dir default; it is only unnecessary in the opt-in symlink case. NO `git rm`. The earlier "just git rm it" conclusion was wrong and would have broken `go list` on every un-migrated checkout | confirmed |
| A-7 | `--show-toplevel` is a stable per-checkout key | already used in `commit_helper.py:95` | collisions | confirmed |
| A-8 | `ze-host` builds with `-tags ze_core,ze_setup`, no GOARCH | `effective-install-qemu.py:153-168` precedent | Option C blocked | confirmed |

### Risks
| ID | Risk | Mitigation |
|----|------|------------|
| R-1 | Eviction races a live QEMU boot | keep-N, key-change-only, never timer; consider lock/refcount (AC-8) |
| R-2 | `mkdir -p tmp*` runs before the symlink -> real dir materialises | ensure-links is a hard prerequisite of every entry target; audit every `mkdir -p tmp*` site |
| R-3 | Container/VM guest still references the host `tmp` symlink | guest-local scratch redirects (D-5); explicit test that a QEMU target runs |
| R-4 | Sentinel untrack breaks `make clean`'s `git show` | drop that step; regenerate the sentinel from a string constant instead |
| R-5 | `$TMPDIR` path length + Unix-socket `sun_path` limit | scratch sockets already use `os.TempDir()` directly (`crypto.go:165`); the `tmp`-relative ephemeral-ssh file is TCP text, not a socket. Verify no test creates a unix socket under `tmp/` |
| R-6 | macOS `git clean -xdf` in main wipes `~/.cache/ze`? No - it is outside the repo | cache is out-of-tree by D-1; only the `tmp`/`cache` symlinks are in-tree |
| R-7 | The hook `/tmp` string guards trip on the `/tmp/ze` fallback | prefer `$TMPDIR`; only fall back to `/tmp/ze` where `$TMPDIR` is unset (Linux/CI, where the hooks may not run) |

## Critical Review Checklist
| Check | What to verify |
|-------|----------------|
| Bootstrap ordering | ensure-links precedes every `mkdir -p tmp*`; no path materialises a real `tmp/` |
| Sentinel / `go list` | `go list ./...` clean on the relocated tree; sentinel handled per A-6 outcome |
| GOCACHE unchanged | GOCACHE stays `$(CURDIR)/tmp/go-cache`; no third cache dir exists. If a socket test fails on a symlinked tree, the fix is that test's path, not the cache location |
| Consumer paths | all ~20 `tmp/kernel/*` consumers unchanged; `test -f` guards still fire |
| Option C key | exactly one key implementation (Go); `run.py` never guesses; `ze-host` is a HOST build (no GOARCH) |
| Eviction safety | key-change-only, keep-N=2, cannot race a live boot; touches only the intended namespaces |
| Per-checkout key | derived from `--show-toplevel`, never `--git-common-dir` |
| Fail-closed | absent artifact / absent `ze-host` / checksum mismatch all fail loudly, never silent fallback |

## Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Repo `tmp`/`cache` symlinks created at build | `ls -l tmp cache` after a clean `make`; both are symlinks to the expected targets |
| Sentinel kept + maintained | `git ls-files tmp/` still lists `tmp/go.mod`; deleting it then running ensure-links recreates it byte-for-byte and `go list ./...` recovers |
| Make kernel routed through cache | AC-1 timing; `~/.cache/ze/runtime-kernel/` populated after `make ze-kernel` |
| ISO verified + unified | one `ALPINE_VERSION` (grep); a corrupted cached ISO is rejected |
| Eviction | after a key bump, superseded entry gone, current + one prior kept |
| GOCACHE pinned | `test_gocache_not_in_tmpdir`; a container build writes its cache successfully |
| Docs/rules updated | `make ze-doc-test`; grep for stale `tmp/` real-dir claims |
| 988 correction pointer | present in `plan/learned/988` |

## Security Review Checklist
| Concern | Check |
|---------|-------|
| ISO integrity | `.sha256` verified while streaming; atomic rename; a truncated/substituted image never served (AC-5) |
| Symlink target injection | `<checkout-id>` derived from `git`, not from untrusted env; targets created under `$TMPDIR`/`~/.cache` with safe perms |
| Path traversal in eviction | evict only within the resolved namespace dir; never follow a symlink out of it |
| Cross-tree writes | a worktree creating `~/.cache/ze` is legitimate (gitignored build output, not a working tree); ensure hooks do not falsely block it |
| Guest scratch | guest-local scratch is not attacker-controlled; no host path leaks into the guest as a trust boundary |
| TOCTOU on cache hit | verify-then-copy; a concurrent evictor must not delete between check and use (AC-8 lock/refcount) |

## Documentation Update Checklist
| Category | Update? | File / action |
|----------|---------|---------------|
| Test infrastructure | Yes | `ai/rules/qemu-testing.md`, `docs/architecture/testing/qemu-integration.md`: document the durable root, the `tmp`/`cache` symlinks, what survives a scratch wipe |
| Dev workflow / git | Yes | `ai/rules/git-safety.md`, `ai/rules/bash-output.md`, `ai/rules/testing.md`, `ai/rules/never-destroy-work.md`: `tmp/` is now a symlink; per-checkout scratch; the `.gitignore` key-paths list |
| Session/plan machinery | Yes | `.claude/rules/{post-compaction,session-start,planning}.md`: session-state path is through the symlink |
| Generated instructions | Yes | edit `ai/` sources that regenerate `CLAUDE.md`/`AGENTS.md` (`bash tmp/commit-*.sh` references) |
| User guide | Yes | `docs/guide/appliance.md`, `docs/functional-tests.md`: `tmp/kernel/*` still valid (through the symlink) |
| RFC status | No | no protocol behavior changes |

## Review Gate
_To be filled by `/ze-review` at implementation. Loop to 0 BLOCKER / 0 ISSUE._

### Run 1 (initial)
| Severity | Finding | Location | Action |
|----------|---------|----------|--------|
| _pending_ | | | |

## Goal Validation
| Goal (from Task) | Evidence |
|------------------|----------|
| Repo holds no heavy real dirs; only the `tmp`/`cache` symlinks | `ls -l` after clean build |
| `rm -rf tmp` is always safe | AC-1, AC-13 |
| Worktrees reuse the precompiled kernel | AC-2 |
| tmp deletable in full | AC-13 |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| scripts/dev/ensure-links.py | yes | created; create/skip/migrate/per-checkout all tested in throwaway repos |
| scripts/evidence/alpine_iso.py | yes | shared ISO module; imported by qemu-run.py + qemu-build.py (`--help` OK) |
| plan/learned/1173-relocate-scratch-and-cache.md | yes | this commit's learned summary |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | rm scratch -> rematerialize in seconds | `make ze-kernel` HIT: `real 23.08`, then `real 5.50` (vs ~30 min) |
| AC-3 | extract keyed by ISO stem | `qemu-run.py --selftest` -> `qemu-run selftest OK` |
| AC-4 | keep-N=2 eviction + grace | `go test ./internal/appliance -run Evict` -> PASS |
| AC-5 | ISO `.sha256`-verified, durable | boot log "Downloading Alpine virt ISO"; `~/.cache/ze/alpine-iso/` has `.iso` + `.sha256` |
| AC-7 | cached kernel boots | QEMU: `Linux localhost 7.1.1 ... aarch64` + `BOOT_OK_MARKER`, exit 0 |
| AC-9 | same key as `ze appliance kernel` | `--print-cache-dir` -> `.../runtime-kernel/7.1.1-runtime-arm64-runtime-e2940fc5-063a7289` |
| AC-10 | `ze-kernel-clean` keeps cache | after clean, `ls ~/.cache/ze/runtime-kernel/` shows the entry present |
| AC-11 | `go list` works (sentinel) | `rm tmp/go.mod` -> `go list` FAILS; ensure-links recreates byte-for-byte -> `go list` OK |
| lint | changed Go clean | `make ze-lint-changed` -> `0 issues` |

### Deferred (recorded, not blocking)
| Item | Why |
|------|-----|
| AC-12 auto-symlink on fresh checkout | DEVIATION: opt-in `make ze-migrate-scratch` (D-10 safety); the `cache` symlink IS auto-created |
| Full `make ze-verify` | exceeds 10-min budget; RED pre-existing 2026-07-17 (committed `--unverified`, user-authorized) |
| `/ze-review` independent gate | deferred (`--review-override`, user-authorized) |

## Supersession
`spec-fixit-qemu-artifact-cache` is superseded by this spec: its durable-cache research and
ACs are absorbed here (cache root reversed to `~/.cache/ze` per D-1). At closure, mark that
spec superseded with a pointer here and remove it per the two-commit closure rule.
