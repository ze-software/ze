# 1329 -- gokrazy-init-bump

## Context

Dependabot alert #26 (CVE-2026-25680, `x/net/html` parser DoS) fired on a
`go.mod` under `gokrazy/modcache/`. That file is not a Ze dependency manifest.
It is the vendored upstream gokrazy init module, checked in because `gok` reads
a committed `GOMODCACHE` instead of a `vendor/` tree. The alert scanned a
manifest that never controlled what compiled. The builddir main module already
pinned `x/net v0.56.0`, and MVS takes the max. The goal was to remove the stale
manifest at its source. The vendored init moved from
`v0.0.0-20260218074004-791851666ca2` to `v0.0.0-20260703061218-a4a45a20149d`.
A runbook came with it, so the next alert of this class costs less.

## Decisions

- Bumped the upstream pseudo-version over owning an edited `go.mod` or
  dismissing the alert: the new upstream requires `x/net v0.56.0` natively, so
  the fix arrives in the manifest rather than beside it.
- Kept the work inside the modcache design over converting to `go mod vendor`:
  `gok` hardcodes `-mod=mod` and calls `go get`
  (`vendor/github.com/gokrazy/tools/packer/gotool.go`), so a builddir `vendor/`
  tree is ignored. Forking `gokrazy/tools` to add vendor support was rejected as
  fighting its design.
- Dropped the explicit `x/net v0.56.0` pin and its comment over keeping them:
  the comment claimed upstream pins `v0.38.0`, which the bump made false, and
  `go list -m golang.org/x/net` resolves `v0.56.0` in every builddir without it.
- Made `E2FS` autodetect in `mk/gokrazy.mk` over keeping the hardcoded homebrew
  path: `E2FS := /opt/homebrew/...` ignores an environment variable, so every
  Linux appliance build failed the e2fsprogs guard.
- Probed for BOTH `mkfs.ext4` and `debugfs` over probing the first alone. The
  build formats `/perm` with one and injects credentials with the other, so a
  one-tool probe can select a directory that dies later.
- Closed the spec with AC-6 (QEMU appliance boot plus full L2TP proof) UNRUN
  over holding the spec open. The run needs root, `/dev/ppp` and PPPoL2TP.
  `plan/spec-finish-appliance-qemu-evidence.md` already names it as a work item
  and exists to execute it on a qualifying host.

## Consequences

- A Dependabot alert on `gokrazy/modcache/**/go.mod` is a stale vendored
  upstream manifest, not a Ze dependency. The remediation is a version bump plus
  a re-vendor, and `ai/rules/platform-linux.md` now carries that runbook with an
  `ai/INDEX.md` pointer row.
- `.github/dependabot.yml` is scoped to the root module only. The gokrazy
  builddir modules are hand-pinned and deliberately excluded: a Dependabot PR
  against them would fight the pin. The file also does not suppress the
  always-on security scan, and says so in its own header.
- The autodetect is an immediately expanded `E2FS := $(shell ...)` and
  `mk/gokrazy.mk` is included unconditionally, so every `make` invocation in the
  repo forks one probe shell. Deferred expansion would confine that cost to the
  gokrazy targets. Measured as a NOTE at closure, not fixed.
- The image build (AC-5) is the proof that the bump compiles. The appliance boot
  is a separate proof owned by a separate spec. A green `make ze-gokrazy` is not
  evidence that the new init supervises services.
- AC-6 is unrun at closure. The evidence run lives at
  `plan/spec-finish-appliance-qemu-evidence.md`, which also holds the same run
  for `iface-absent-link-graceful` AC-3. One run satisfies both rows.

## Gotchas

- `gokrazy/modcache/.gitignore` ignores `*` and whitelists
  `github.com/gokrazy/gokrazy@*/**`. The `@*` glob auto-whitelists the NEW
  version path on re-vendor, so the tracked file set moves by itself. The OLD
  version directory does not: prune it with `rm -rf` and re-check `git status`,
  or it reappears in somebody else's commit.
- Deleting a builddir `go.sum` and running `ze-gokrazy-deps` regenerates it from
  the new build list. That is the way to drop an old version string without
  hand-editing hashes, which is a checksum mismatch waiting to happen.
- `make ze-qemu-*` boots an Alpine VM with host-compiled `ze` binaries. It
  exercises `ze`, NOT the gokrazy init. The init-specific proof is booting the
  gokrazy IMAGE: `make ze-vpp-hugepages-qemu-test` for a plain boot, and
  `ze-deployment-gokrazy-l2tp-ppp-test` for the L2TP path.
- `test/appliance/serial-login.ci` boots nothing. Its own header defers the QEMU
  plan "when appliance serial test infrastructure is ready", and what it asserts
  is the offline argv[0] shell-invocation gate. The spec cited it as a boot
  proof in its Wiring Test and in AC-6, and that citation was wrong.
  `ai/rules/platform-linux.md` strikes it out of the proof table for this
  reason. Never read a green `serial-login.ci` as an appliance boot.
- The `replace` directives in `serial-busybox/go.mod` and `rtr7/kernel/go.mod`
  point at gokrazy versions from 2020 and 2022. They exist to stop stale
  transitive `x/crypto` / `x/net` / `x/sys` copies entering the modcache, and
  they must move with every bump. A grep of `require` lines alone misses both.
- `make ze-gokrazy E2FS=` (explicitly empty) does NOT resume autodetect, and the
  reason is not the one it looks like. `ifndef` tests whether a variable expands
  to something NON-EMPTY, not whether it was defined, so an empty override still
  enters the block and the probe RUNS. Its result is then thrown away, because a
  command-line assignment beats the makefile's `:=`. Measured on GNU make with a
  two-line reproducer. The spec's claim that "override still works via
  `make ... E2FS=`" was loose. Pass a path, or leave `E2FS` unset.
- `debugfs` exits 0 even when its `-R` command fails, and reports the failure
  only on stderr. `mk/gokrazy.mk` discards that stderr on both credential-inject
  calls, so an image whose `/perm` database was never written builds green and
  fails at boot. Measured on e2fsprogs 1.47.0. Homed at
  `plan/spec-gokrazy-builddir-tmp-deferred-build-flow-unification.md`; removing
  the redirection alone does not fix it, because a successful run also prints a
  version banner there.

## Files

- `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/go.mod` + `go.sum`
- `gokrazy/ze/builddir/github.com/gokrazy/gokrazy/cmd/{dhcp,ntp,heartbeat,randomd}/go.mod` + `go.sum`
- `gokrazy/ze/builddir/github.com/gokrazy/serial-busybox/go.mod` + `go.sum`
- `gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod` + `go.sum`
- `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/**`
- `mk/gokrazy.mk` (E2FS autodetection; two-tool probe and guard)
- `ai/rules/platform-linux.md`, `ai/INDEX.md`, `.github/dependabot.yml`
- `scripts/dev/dev-setup.py` (xl2tpd and ppp listed as optional Linux deps)
- `plan/spec-kernel-lockdown-hardening.md` (two refreshed path anchors)
