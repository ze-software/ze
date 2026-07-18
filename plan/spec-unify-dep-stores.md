# spec-unify-dep-stores

| Field | Value |
|-------|-------|
| Status | design (stub — deferred) |
| Depends | spec-relocate-scratch-and-cache (land first) |
| Updated | 2026-07-18 |

## Task

The repo commits its Go dependencies TWICE, in two different formats:

- `vendor/` — a flat, one-version-per-package tree, read only by `-mod=vendor`, tied to the
  root module's `go.mod`. Used to build `bin/gok` (`mk/gokrazy.mk:57`, `-mod=vendor`) and the
  rest of the repo.
- `gokrazy/modcache/` — a committed, versioned `pkg@vX.Y.Z` GOMODCACHE (60+ tracked files),
  read by normal module resolution (`GOMODCACHE=$(CURDIR)/gokrazy/modcache`), used by the
  gokrazy image/kernel build across several independently-pinned builddir modules
  (`gokrazy/ze/builddir/*/go.mod`).

The gokrazy package names appear in BOTH stores because gokrazy code is needed at both
stages. Collapse to ONE committed store.

## Decision (user, 2026-07-18)

- **Direction: single committed GOMODCACHE, drop `vendor/`.** Build `bin/gok` with
  `GOFLAGS=-mod=mod` and `GOMODCACHE=$(CURDIR)/gokrazy/modcache` (the store the image build
  already uses), so exactly one store, one format, both stages read it. The reverse
  (vendor-only) is NOT feasible: a flat `vendor/` cannot hold the multiple gokrazy versions
  the builddir modules pin.
- **Sequencing: its OWN spec, AFTER `spec-relocate-scratch-and-cache` lands.** Not folded into
  the relocation work (different category: these are committed source deps, not disposable
  scratch/cache).

## Owned tradeoffs (recorded so nobody reopens by surprise)

- Loses `-mod=vendor`'s tamper-evident, flat, auditable property (`.gitignore:1` "Dependencies
  are vendored and committed" is a deliberate choice being reversed).
- Repo-wide: moves the ENTIRE repo off vendoring, not just the gokrazy overlap.
- The committed GOMODCACHE is larger and its `@vX.Y.Z` layout is noisier in git than a flat
  vendor tree.

## MANDATORY first step (before any code)

**Version-alignment audit.** The unification is reasoned from Go tooling behavior + Makefile
wiring, NOT a byte-level audit. Before touching anything, verify the exact module versions the
root module and every `gokrazy/ze/builddir/*/go.mod` pin actually resolve against the single
committed `gokrazy/modcache`, and that `bin/gok` builds reproducibly with `-mod=mod` against
it. If the versions do not line up, this spec's approach needs rework, not a workaround.
