# spec-unify-dep-stores

| Field | Value |
|-------|-------|
| Status | deferred |
| Depends | - (was `spec-relocate-scratch-and-cache`, LANDED and closed 2026-08-03) |
| Phase | - |
| Updated | 2026-07-18 |

## Task

The repo commits its Go dependencies TWICE, in two different formats:

- `vendor/` — a flat, one-version-per-package tree, read only by `-mod=vendor`, tied to the
  root module's `go.mod`. Used to build `bin/gok` (`mk/build-gokrazy.mk`, `-mod=vendor`) and the
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

## Audit + spike results (2026-07-18)

**The premise "two committed stores duplicating everything" is FALSE.** The committed content
is DISJOINT:
- `vendor/` (45M) commits the root module's gokrazy deps: `tools@20260406`, `updater@20250705`,
  `gokapi@20251205`, `internal@20251208` (+ all non-gokrazy deps). NOT `gokrazy/gokrazy`.
- `gokrazy/modcache/` commits ONLY `gokrazy/gokrazy@20260703` (init: dhcp/ntp/randomd/heartbeat,
  baked into the image, kept auditable). Its `.gitignore` whitelists just `gokrazy/gokrazy@*/**`;
  the other 1.4G (tools, x/*, serial-busybox, ...) is DOWNLOADED by `ze-gokrazy-deps-download` and
  gitignored. So committed overlap is ~zero.

**Spike (attempt: vendor `gokrazy/gokrazy` + `replace` to it) hit a HARD WALL — the pins do not
line up.** A never-compiled blank import of `gokrazy/gokrazy` + `go mod tidy`/`vendor` resolved
`gokrazy/gokrazy` to `@20260703` (matching the image pin, good) but forced `gokrazy/internal`
up `20251208 -> 20260625`, which BROKE the vendored `gokrazy/tools@20260406`:
`vendor/github.com/gokrazy/tools/internal/packer/sbom.go: undefined: config.InstanceConfigPath`.
So the April `tools` and the July `gokrazy/gokrazy` need incompatible `internal` versions. Spike
reverted; tree clean.

**Conclusion.** "Feed the image build from `vendor/`" is not a small edit at the current pins. It
requires bumping the WHOLE gokrazy stack (`tools`, `gokrazy`, `updater`, `internal`, `gokapi`) to
one mutually-consistent recent snapshot, THEN verifying the newer `gokrazy/tools` API still
compiles the ze code that uses it (`cmd/ze-gok`, `internal/appliance/updater`,
`internal/component/gokrazy`, `cmd/ze-serial-shell`).

## Bump landed (2026-07-18)

**Step 1 (the stack bump) is DONE and committed** (`chore(deps)`): `tools` Apr->Jul
(`20260703063348`), `updater`/`internal` -> Jun 2026, + transitive bumps. Every ze consumer
compiles with ZERO code changes and the appliance unit tests pass. Bonus: it fixes a real latent
mismatch (`gok`/`tools` were April while building the July `gokrazy@20260703` init; now aligned).
Committed on compile+unit-test strength (user-authorized); full `make ze-gokrazy-build` image build +
QEMU boot with the July `gok` is to be **verified separately**.

**Step 2 (image build reads `vendor/`) is BLOCKED and NOT pursued.** Verified empirically: `go mod
vendor` strips `go.mod` from vendored packages (`vendor/.../gokrazy/gokrazy/go.mod` absent), so a
`replace => vendor/...` (which needs a module `go.mod`) is impossible; and the builddir modules are
separate from the root module, so they cannot share the root `vendor/`. The only route to "one
committed source in `vendor/`" is a GENERATION step that materializes a module cache FROM `vendor/`
(the kernel's `replace => tmp/kernel/pkg` pattern, extended to gokrazy/serial-busybox/deps). That
is real machinery and a separate task; deferred pending a decision on whether the (zero-duplication)
status quo is worth changing.

## Required Reading

- `ai/rules/platform-linux.md` — the vendored-init bump runbook and modcache cache-permission rules
- `mk/build-gokrazy.mk` — how `bin/gok` and the image build consume the two stores today
- `spec-relocate-scratch-and-cache` (closed) — must land first (Depends)

## Current Behavior

**Source files read:** (during the 2026-07-18 audit + spike recorded above)
- [x] `mk/build-gokrazy.mk` — `bin/gok` built with `-mod=vendor` from the root module
- [x] `gokrazy/ze/builddir/*/go.mod` — independently-pinned builddir modules read `GOMODCACHE=$(CURDIR)/gokrazy/modcache`
- [x] `gokrazy/modcache/.gitignore` — whitelists only `gokrazy/gokrazy@*/**`; everything else is downloaded, not committed
- [x] `.gitignore:1` — "Dependencies are vendored and committed" (the property step 2 would reverse)

**Behavior to preserve:** reproducible `bin/gok` and image builds from committed sources only; the zero-duplication property established by the audit (committed overlap between the two stores is ~zero).

**Behavior to change:** none until the step-2 decision is made; step 1 (stack bump) already landed.

## Data Flow

### Entry Point
- `make ze-gokrazy-build` (image build) and the `bin/gok` build in `mk/build-gokrazy.mk` — both are build-time consumers of the committed dependency stores; there is no runtime data flow.

### Transformation Path
1. Root-module builds resolve deps from `vendor/` via `-mod=vendor`.
2. `ze-gokrazy-deps-download` downloads the gitignored remainder into `gokrazy/modcache/`.
3. Builddir module builds resolve from `GOMODCACHE=gokrazy/modcache` (committed `gokrazy/gokrazy@*` + downloaded rest).
4. (Blocked step 2 would insert: a generation step materializing a module cache from `vendor/`.)

### Boundaries Crossed
| Boundary | Format |
|----------|--------|
| Root module -> vendor tree | flat vendored packages, no per-package `go.mod` |
| Builddir modules -> modcache | versioned `pkg@vX.Y.Z` GOMODCACHE layout |

### Integration Points
- `mk/build-gokrazy.mk` build targets; `scripts` around `ze-gokrazy-deps-download`; `.gitignore` whitelists.

## Wiring Test

Build tooling only — no daemon feature, no new runtime entry point; existing test suite covers the consumers.

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `make ze-gokrazy-build` | -> | unified dependency store (step 2, if pursued) | `make ze-gokrazy-build` full image build + QEMU boot via `scripts/evidence/qemu-run.py` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates |
|------|------|-----------|
| existing appliance unit tests (no new Go code; change is Makefile/go.mod wiring) | `internal/appliance/...` | consumers still compile and pass against the bumped/unified store |

### Functional Tests
- N/A — no daemon behavior changes; existing test suite covers the build consumers.

## Files to Modify

| File | Change |
|------|--------|
| `mk/build-gokrazy.mk` | (step 2, if pursued) point `bin/gok` build at the unified store |
| `gokrazy/modcache/.gitignore` | (step 2, if pursued) adjust whitelists for the generated cache |

## Implementation Steps

1. (DONE 2026-07-18) Bump the gokrazy stack to one mutually-consistent snapshot.
2. Verify separately: full `make ze-gokrazy-build` image build + QEMU boot with the July `gok` (outstanding from step 1).
3. DECISION GATE (Thomas): is replacing the zero-duplication status quo with a vendor-derived generated modcache worth the machinery? If no, close this spec as cancelled.
4. If yes: build the generation step (kernel `replace => tmp/kernel/pkg` pattern extended), then flip `bin/gok` to `-mod=mod` against it.

## Checklist

- [ ] Tests written (build-level: the QEMU boot evidence run is the test for this spec)
- [ ] Tests FAIL before the change where applicable (N/A for the step-1 bump, already landed)
- [ ] Tests PASS: `make ze-gokrazy-build` image build + QEMU boot green
- [ ] `make ze-standard-test` / `make ze-precommit-verify` green before any commit
