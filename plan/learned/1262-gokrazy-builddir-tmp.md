# 1262 -- gokrazy-builddir-tmp

## Context

Appliance image builds ran gok against the tracked `gokrazy/` instance. On the
hugepage path a derived instance was materialized WITHOUT its builddir, so gok
discarded every dependency pin and fell back to `go get`: offline builds failed,
and online builds silently shipped an UNPINNED kernel (proven: the modcache held
two rtr7/kernel versions newer than the only pin this repo ever declared).
`make ze-kernel` also wrote a kernel replace into a tracked `go.mod`, so any
later build inherited a custom kernel invisibly. The goal: no build step touches
a tracked path, every build runs from a prepared copy carrying the full pinned
builddir, and the kernel is selected per build.

Late in the spec, its one unmet Goal Validation row (the L2TP appliance boot
proof) was root-caused and fixed too: the proof crash-looped at first boot on
every host it was ever run on, and the reason was invisible from the serial
console.

## Decisions

- **D-1c: `cmd/ze-gok` prepares the instance itself** (rewrites `--parent_dir`
  through `internal/appliance/instance.Prepare` before vendored gok parses
  argv), over (a) making `make ze-gokrazy` delegate to `ze appliance build`
  (the D-1 audit found the flows genuinely diverge below the gok call, in
  database seeding) and over (b) editing the Makefile (the gok call line stays
  byte-identical; the binary owns correctness).
- **Copy-and-rewrite over symlink or synthesis** for the builddir: the relative
  ze self-replace is depth-sensitive, so the preparer absolutizes every
  filesystem-path replace against the go.mod's own directory (resolved through
  symlinks).
- **`GOPROXY=off` by default on both gok build sites** (overridable): a missing
  pin fails loudly instead of resolving a NEWER dependency over the network.
  `make ze-gokrazy-deps` is unaffected (bare `go mod download` path).
- **Kernel per build, never in-tree**: `KERNEL_PKG=tmp/kernel/pkg` (env
  `ze.gok.kernel-package`), assembled by `make ze-kernel` from the durable
  cache; `ze-kernel-clean` migrates away the legacy in-place modcache overlay.
- **The L2TP boot proof resolves an L2TP-capable kernel itself** over
  documenting "run make ze-kernel first" (the documented prerequisite was
  skipped in practice, and the failure mode was an undiagnosable crash-loop):
  the pinned rtr7 kernel has NO l2tp support (zero loadable modules, none
  builtin — verified from the proxy zip of the pinned version), while the
  baked template sets `l2tp enabled true`, so ze's RFC 2661 fail-closed probe
  correctly refuses startup and gokrazy restarts it forever. The proof
  validates an explicit `KERNEL_PKG` (arch magic + l2tp present), reuses a
  valid staged `tmp/kernel/pkg` (also pinned-version-checked), or materializes
  the runtime kernel from the durable cache, failing fast with the exact
  command otherwise — and always hands gok a per-run copy, because
  `tmp/kernel/pkg` is shared and every `make ze-kernel` rewrites it.
- **Fatal pre-serve daemon failures are mirrored onto slog**
  (`logStartupFailure`, kmsg on the appliance): stderr is captured by the
  gokrazy supervisor and never reaches serial, which is precisely why the
  crash-loop had been recorded as an unexplained "runtime appliance issue".
- **Kernel version single source of truth stays
  `internal/appliance/kernel.version`** (bumped 7.1.1 -> 7.1.4, owner
  direction); the proof's staleness check reads it, so a version bump
  invalidates staged kernel packages automatically.

## Consequences

- `gokrazy/` is read-only to builds; `git status` stays clean across any image
  build, and two builds in one checkout use isolated prepared instances (the
  one shared mutable path left is `tmp/kernel/pkg`; consumers copy).
- Offline builds are enforced, not aspirational: a modcache miss is a hard
  error naming the module.
- The L2TP deployment proof fails in seconds with a named remediation when the
  host cannot provide the runtime kernel, instead of a 90s timeout after a
  multi-minute build; a probe-refusal crash-loop is detected from serial via
  the new fatal needle.
- Appliance first-boot failures are diagnosable from the serial console alone
  (`msg="startup failed" stage=...`).
- The build-flow seeding divergence (make vs Go path) is deliberately NOT
  unified here; it is homed in
  `plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md` with the
  go.sum drift gate and the builddir rename.

## Gotchas

- **A file make target with no prerequisites is a time bomb**: `bin/gok` built
  once was never rebuilt, so image builds silently ran a pre-preparer gok that
  reached the network and skipped the prepared instance entirely (observed
  live during reproduction); the kernel sub-make's `all: $(OUT)/vmlinuz` had
  the same shape with a worse blast radius — a stale other-arch build view
  satisfied the target, built nothing, and would have populated the requested
  arch's durable cache key with the wrong-arch tree, permanently (the HIT
  check is existence-only). Both fixed (`.PHONY: bin/gok`; the MISS branch
  purges the build view first).
- **`/sys/module/<name>` is NOT a reliable built-in detector in general** (it
  needs params or a MODULE_VERSION), but l2tp_ppp/l2tp_core carry
  `MODULE_VERSION("V2.0")`, so the probe's stat works for the runtime kernel.
  The first crash-loop hypothesis ("probe can't see built-ins") died on that
  fact; always read the builtin modinfo before blaming the probe.
- **The pinned-kernel dir under `gokrazy/modcache` is gitignored local state**:
  on this build host it had been overlaid by an old in-place `make ze-kernel`
  with an ARM64 runtime kernel under the amd64 pin. The fresh vendored gok
  refuses the arch mismatch (good); the genuine pinned content is recoverable
  from the Go proxy zip. Never trust that dir's content matches the pin
  without checking.
- **A poisoned durable cache can be a STUB**: the amd64 runtime-kernel cache
  entry held 15-byte placeholder files that satisfy make's `-f vmlinuz` HIT
  check; content validation (magic + modules.builtin), not existence, is the
  only safe gate. Writer unidentified (likely in-flight work of the open
  qemu-runtime-kernel spec).
- **Reviewer disagreement is resolved by reading the producer**: one reviewer
  called the gok cleanup path safe, another flagged it; vendored
  `pack.Main`'s `os.Exit(1)` inside `Execute` (skipping every deferred
  cleanup) settled it — leading to the stale-dir reaper.
- **sudo resets HOME**: the durable kernel cache probed by a sudo'ed proof is
  root's, not the invoking user's; remediation messages must say so or they
  loop the operator.
- **darwin `/var` -> `/private/var`**: a test comparing path SPELLINGS against
  an `EvalSymlinks`-resolved producer fails only on macOS; compare resolved
  paths (file identity), not strings.

## Files

- `internal/appliance/instance/` (new): the one preparer (+ reaper).
- `internal/appliance/kernelargs.go`, `cmd_build.go`: always-prepare,
  `uniformArch`, `GOPROXY=off`.
- `cmd/ze-gok/main.go`: `--parent_dir` rewrite, kernel env, `GOPROXY=off`.
- `mk/gokrazy.mk`: `ze-kernel` durable-cache flow, `KERNEL_PKG`,
  `.PHONY: bin/gok`, MISS-branch purge.
- `scripts/evidence/effective-gokrazy-l2tp-ppp.py`: third preparer deleted;
  kernel resolution/validation (`kernel_pkg_problems`, `resolve_kernel_pkg`,
  `copy_kernel_pkg`), fatal needle.
- `cmd/ze/hub/main.go`: `logStartupFailure` mirror on every fatal pre-serve
  exit; `internal/component/l2tp/kernel_linux.go`: sentinel prefix dedup.
- `scripts/dev/gokrazy_l2tp_kernel_test.py`, `test/ui/startup-failure-slog.ci`
  (new tests); `internal/appliance/kernel.version` (7.1.4).
- `docs/functional-tests.md`, `docs/labs/l2tp-interop.md`,
  `docs/guide/appliance.md`.
