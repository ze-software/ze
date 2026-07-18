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

## Audit + spike results (2026-07-18)

**The premise "two committed stores duplicating everything" is FALSE.** The committed content
is DISJOINT:
- `vendor/` (45M) commits the root module's gokrazy deps: `tools@20260406`, `updater@20250705`,
  `gokapi@20251205`, `internal@20251208` (+ all non-gokrazy deps). NOT `gokrazy/gokrazy`.
- `gokrazy/modcache/` commits ONLY `gokrazy/gokrazy@20260703` (init: dhcp/ntp/randomd/heartbeat,
  baked into the image, kept auditable). Its `.gitignore` whitelists just `gokrazy/gokrazy@*/**`;
  the other 1.4G (tools, x/*, serial-busybox, ...) is DOWNLOADED by `ze-gokrazy-deps` and
  gitignored. So committed overlap is ~zero.

**Spike (attempt: vendor `gokrazy/gokrazy` + `replace` to it) hit a HARD WALL — the pins do not
line up.** A never-compiled blank import of `gokrazy/gokrazy` + `go mod tidy`/`vendor` resolved
`gokrazy/gokrazy` to `@20260703` (matching the image pin, good) but forced `gokrazy/internal`
up `20251208 -> 20260625`, which BROKE the vendored `gokrazy/tools@20260406`:
`vendor/github.com/gokrazy/tools/internal/packer/sbom.go:101: undefined: config.InstanceConfigPath`.
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
Committed on compile+unit-test strength (user-authorized); full `make ze-gokrazy` image build +
QEMU boot with the July `gok` is to be **verified separately**.

**Step 2 (image build reads `vendor/`) is BLOCKED and NOT pursued.** Verified empirically: `go mod
vendor` strips `go.mod` from vendored packages (`vendor/.../gokrazy/gokrazy/go.mod` absent), so a
`replace => vendor/...` (which needs a module `go.mod`) is impossible; and the builddir modules are
separate from the root module, so they cannot share the root `vendor/`. The only route to "one
committed source in `vendor/`" is a GENERATION step that materializes a module cache FROM `vendor/`
(the kernel's `replace => tmp/kernel/pkg` pattern, extended to gokrazy/serial-busybox/deps). That
is real machinery and a separate task; deferred pending a decision on whether the (zero-duplication)
status quo is worth changing.
