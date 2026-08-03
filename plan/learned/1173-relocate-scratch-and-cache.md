# 1173 — Relocate scratch + durable cache out of the repo (symlinks)

**Spec:** spec-relocate-scratch-and-cache (closed 2026-08-03) (supersedes spec-fixit-qemu-artifact-cache)

## Context

Expensive, rarely-changing build inputs (the ~30-min runtime kernel, the 76 MB Alpine ISO)
lived under the disposable `tmp/` tree, so `rm -rf tmp` cost a rebuild/redownload and worktrees
could not share them. Goal: move them to a durable cache and make the repo hold only symlinks
to out-of-tree state.

## Decisions

- **Durable cache = existing `~/.cache/ze`** (`resolveCacheDir`, `cache.go:47-57`), NOT a new
  repo-local `cache/` folder. A repo `cache` symlink points at it (created by
  `scripts/dev/ensure-links.py`) for visibility. Shared across worktrees, survives `git clean`.
- **Make kernel path routed through the cache via Option C:** `mk/gokrazy.mk ze-kernel` asks a
  HOST `ze-host` binary for the arch+config-keyed dir (`ze appliance kernel --print-cache-dir`,
  reusing `kernelCacheVariantFor`), materializes on a HIT (`cp -R`, matching `copyTree`), builds
  + populates on a MISS, and calls `--evict-cache`. Go stays the single source of truth for the
  key, so the make and Go paths cannot drift. Result: `make ze-kernel` = 23s/5.5s vs ~30 min.
- **Keep-N=2 eviction** (`evictKeepN`, `cache.go`), key-change-only, with an `evictGrace` window
  so a concurrent materialize/boot is never deleted (AC-8/R-1).
- **Alpine ISO** moved to `~/.cache/ze/alpine-iso`, `.sha256`-verified, via one shared importable
  module `scripts/evidence/alpine_iso.py` used by both `qemu-run.py` and `qemu-build.py` (one
  `ALPINE_VERSION`, no drift).
- **`tmp`→symlink is OPT-IN** (`make ze-migrate-scratch`), not automatic. Fresh/existing checkouts
  keep a REAL `tmp/`; migration moves contents to `$TMPDIR/ze/<checkout-id>` and swaps in a
  symlink. Per-checkout key = `git rev-parse --show-toplevel` hash.

## Consequences

- `rm -rf tmp` (or a `$TMPDIR` wipe) costs a copy, not a rebuild; a worktree reuses the compiled
  kernel; QEMU boots the cached kernel (verified, 7.1.1 aarch64).

## Gotchas (the expensive lessons)

- **Do NOT treat a code comment as a hard requirement.** `Makefile:13` "keep build caches within
  CURDIR (not TMPDIR - breaks Unix socket tests)" describes a TEST bug (a socket path exceeding
  `sun_path` under a long `$TMPDIR`), not a cache-location law. Building a `.zcache` "pinned
  exceptions" directory around it was over-engineering; reverted. If a socket test breaks on a
  symlinked tree, fix that test's socket path. Question documented constraints before designing.
- **The `tmp/go.mod` sentinel must be KEPT.** A-6 (`go list ./...` skips a directory SYMLINK)
  is true, but the DEFAULT tree has a REAL `tmp/`, which `go list` DOES descend — removing the
  sentinel makes it fail (verified). It stays tracked and is recreated by `ensure-links.py`
  (`ensure_sentinel`); no `git rm`. Only the opt-in migrated (symlink) case needs no sentinel.
- **gopls "BrokenImport / not in workspace" after edits is transient noise.** Real `go build`
  passed every time. Verify with an actual compile, not the LSP snapshot.
- **Never auto-migrate a live 6.7 GB `tmp/`** other worktree agents may be using. Migration is
  explicit (`ze-migrate-scratch`), and `ensure-links` SKIPs (never clobbers) an existing real dir.

## Deferred

Unifying `vendor/` + `gokrazy/modcache/` onto one committed GOMODCACHE is its own spec
(plan/spec-unify-dep-stores.md), gated on a version-alignment audit.

## Files

Recorded at closure, 2026-08-03. The spec that named these is removed, so this is
the only place they are listed together.

- `Makefile` -- `GOCACHE := $(CURDIR)/cache/go-cache`, and the `ze-migrate-scratch`
  target that converts a real `tmp/` or `cache/` into the symlink
- `scripts/dev/ensure-links.py` -- creates the links, and SKIPs rather than
  clobbers an existing real directory
- `scripts/evidence/alpine_iso.py` -- the substituted-image guard (AC-5, R-5)
- `internal/appliance/cache.go` -- `evictKeepN`, the retention policy
- `internal/appliance/cmd_kernel.go` -- `--print-cache-dir`
- `plan/learned/1176-cache-tree-atomic-stage-rename.md` -- completes the deferred
  R-1 / AC-8 of this work
