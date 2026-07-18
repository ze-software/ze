# 1176 — Durable-cache directory-tree writes must be atomic (stage + rename)

**Spec:** plan/spec-relocate-scratch-and-cache.md (completes deferred R-1/AC-8)

## Context

The durable cache (`~/.cache/ze`) holds directory-tree artifacts — the runtime
kernel (vmlinuz + lib/modules + DTBs + overlays) and the Alpine ISO extract —
keyed by arch+config and SHARED across concurrent agents in the same checkout.
Both were written IN PLACE: `RemoveAll(dst)` then copy into `dst`
(`internal/appliance/cache.go` copyTree; the `cp -R` mirror in `mk/gokrazy.mk`;
`7z x` in `scripts/evidence/qemu-run.py`). On a shared key that exposed a
half-written tree to a concurrent reader for the whole copy, and a failed/killed
copy destroyed the existing tree. Single-file artifacts were already safe
(temp + rename); only the tree writes were not (the spec flagged this as R-1/AC-8
"lock/refcount — deferred").

## Fix

Stage the whole tree in a sibling `.copytree-*` dir, then swap it in with a
rename. A reader then sees the old complete tree, nothing (a loud miss that
rebuilds — readers use `os.Stat`/`test -f`), or the new complete tree, never a
partial; a failed copy leaves the old tree intact. The staging dir is a sibling
of `dst` so the rename is same-filesystem and atomic regardless of where the
cache and scratch live.

## Gotchas (the expensive ones)

- **Shell `mv` SILENTLY NESTS without `-T`.** When two agents populate the same
  key and the loser renames onto the winner's now-present dir, `os.Rename` (Go)
  errors and `os.replace` (Python) raises — but coreutils `mv staging dst`
  moves `staging` *inside* `dst` (`dst/.copytree-XXXX/…`, exit 0), doubling the
  key's size with a nested tree that `evictKeepN` never reaps. Use `mv -T`
  (`--no-target-directory`) so the shell mirror fails loudly like the others.
  A reviewer caught this; it is not visible from reading the happy path.
- **Staging dirs share the namespace `evictKeepN` scans.** They must be
  dot-prefixed and (a) NEVER counted as cache keys (skip `.`-prefixed entries,
  else keep-N miscounts) and (b) reaped when orphaned past the grace window
  (else a crash/lost-race leak accumulates forever — asymmetric with real keys,
  which age out). Real keys start with a version digit, never `.`.

## Consequences

Concurrent same-key populate now converges to one complete tree (last rename
wins; content-identical for a content-addressed key), and the loser fails
loudly + recovers on a retry HIT rather than corrupting the cache.
