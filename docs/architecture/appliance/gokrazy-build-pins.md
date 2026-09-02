# Derived parent instances keep the builddir pins

An appliance config that asks for hugepages cannot be built from the tracked
gokrazy instance directly, because the kernel command line has to change. The
build therefore prepares a temporary "derived parent" instance and runs `gok`
inside it.

<!-- source: internal/appliance/instance/prepare.go -- Prepare, copyBuildDir, absolutizeReplaces -->
<!-- source: internal/appliance/kernelargs.go -- hugepageKernelArgs, the caller that needs a derived parent -->

`gok` resolves the builddir relative to the instance directory it changes into.
A derived parent that omits the builddir therefore has no module pins at all:
`gok` synthesizes an empty module and resolves every package by fetching it. The
result is an image built from whatever upstream happened to hold that day.

That happened. `github.com/rtr7/kernel` has been pinned since the appliance
landed, and the module cache still held two later kernel versions, each fetched
one to two minutes before a build. Hugepage appliance images from those days
shipped an unpinned Linux kernel.

## Decisions

- The derived parent **copies** the builddir and rewrites each `go.mod`
  filesystem-path replace to an absolute path. A symlink and a plain copy both
  break, because the self-replace is a relative path six levels up and is
  therefore depth-sensitive. Synthesizing a module was rejected too: `gok`
  cannot reproduce the self-replace, and the tracked module is what makes the
  dependency cache populate.
- The rewrite is generic, driven by `modfile.IsDirectoryPath`. It is not a
  special case for the Ze module. Version replaces are left alone.
- **A builddir that yields no `go.mod` is an error, not a quiet copy.** The
  failure being defended against is silent: a missing pin does not fail a build,
  it changes what the build produces.
- The derived parent materializes under the project `tmp/`, not the system
  temporary directory.

## Traps

**The module cache is evidence.** The pin escape ran for at least five days and
no gate saw it, because a missing pin makes the build succeed against a newer
version. Disk usage is what noticed. Timestamps under `cache/download/*/@v/`
reconstruct which build fetched what, to the minute. A
`github.com/ze-software/ze@...` directory, or an off-pin copy of a pinned
module, means some path is preparing an instance without the builddir. Tracked
files live inside `gokrazy/modcache`, so deleting it is never the answer.

**A green test can encode the defect.** The old test asserted that the builddir
was absent from the derived directory, and carried a comment explaining why that
was deliberate. The comment was written by the same hand that made the mistake.
Both replacement assertions were mutation-verified: re-adding the exclusion, and
disabling the replace rewrite, each turn the test red.

## Proving a boot, and three ways it lied

Getting the QEMU boot proof to run turned up three defects stacked behind one
misleading message. Each is a general trap.

- **An accelerator probe must test access, not existence.** `/dev/kvm` exists
  for every user and is `root:kvm` 0660. Outside the group, QEMU does not fall
  back, it exits with a permission error. The probes use
  `os.access(..., R_OK|W_OK)`, and macOS takes an explicit `hvf` branch.
- **The appliance's SSH server is the Ze CLI, not a Unix shell.** A proof that
  ran `cat /proc/cmdline` over SSH got `unknown command`, and the harness read
  the non-zero exit as "not booted yet". It retried for 180 seconds and blamed
  the boot. That proof had never passed and could not have. It asks
  `show host kernel | json` now.
- **A SKIP is not a pass.** The scenario accepted `PASS|SKIP`, so the vacuous
  proof never showed as a failure. Under a hardware accelerator, an appliance
  that never answers is a FAIL now; only the `tcg` case may skip. When a skip
  path is reachable in normal operation, its reason must name the exact missing
  prerequisite.
- **A diagnostic that guesses is worse than one that reports.** The old message
  named the wrong subsystem twice, "did not reach SSH" and "slow tcg?", when SSH
  answered in 10 seconds under KVM. The rewritten one prints the last SSH error
  verbatim.

<!-- source: internal/le/qemu/actions.go -- Answer -->
<!-- source: internal/le/setup/actions.go -- Answer -->
<!-- source: internal/le/setup/actions.go -- Answer -->

`./le setup check` reports `kvm-access` as `present`, `pending`, `missing`, or
`n/a`. `pending` means the user is in the `kvm` group but the running session
predates it. That difference is the difference between "run this" and "log back
in".

**A page count is not kilobytes.** `HugePages_Total` is a count and
`Hugepagesize` is in kB. The meminfo parser multiplied everything by 1024, so
counts needed their own map. 64 pages would otherwise report as 65536.

<!-- source: internal/component/host/memory_linux.go -- meminfoCounts against meminfoFields -->


## Why a module cache is checked in

`gok` (`cmd/ze-gok`, wrapping `github.com/gokrazy/tools`) compiles every
appliance package in module mode and fetches with `go get`. It has no vendor
support at all: a `vendor/` tree in a builddir is ignored. The build therefore
resolves through a checked-in module cache, `gokrazy/modcache/`, with
`GOMODCACHE` set by `cmd/ze-gok/main.go`.

`gokrazy/modcache/.gitignore` ignores everything except the gokrazy init source
(`github.com/gokrazy/gokrazy@*/**`). That committed source carries upstream's
own `go.mod`, and GitHub's dependency graph scans every `go.mod` in the
repository as a manifest. When upstream's `go.mod` names a version with a later
advisory, the alert fires on that file even though the image never builds the
vulnerable version: the builddir modules pin the fix and minimal version
selection takes the maximum. A Dependabot alert on a `go.mod` under
`gokrazy/modcache/` is therefore almost always a stale vendored upstream
manifest rather than the real dependency graph.

<!-- source: cmd/ze-gok/main.go -- GOMODCACHE and the -modcacherw GOFLAGS append -->
<!-- source: gokrazy/modcache/.gitignore -- the init-source whitelist -->

### The eight builddir modules

`gokrazy/ze/builddir/` holds eight modules. Seven are tracked locks whose
`go.sum` shows a diff on a bump:

```text
github.com/gokrazy/gokrazy
github.com/gokrazy/gokrazy/cmd/dhcp
github.com/gokrazy/gokrazy/cmd/heartbeat
github.com/gokrazy/gokrazy/cmd/ntp
github.com/gokrazy/gokrazy/cmd/randomd
github.com/gokrazy/serial-busybox
github.com/rtr7/kernel
```

The eighth, `github.com/ze-software/ze`, is only `replace ze => <repo root>`, so
every line of its sum is already in the root `go.sum`. Its `go.sum` is
gitignored (`.gitignore`). Regenerate it like the rest and expect no diff.

### Dependabot is off for these paths

`.github/dependabot.yml` scopes version updates to the root module. The builddir
modules and the checked-in cache are excluded on purpose: an automated PR would
fight the hand-pin, because the maximum minimal-version-selection answer is
chosen deliberately, and a bot bump reopens the stale-manifest churn. A
proactive review replaces the bot. Security-alert scanning is always on and
cannot be suppressed there, which is why the alert still arrives.

## Module cache hygiene

`gokrazy/modcache/` is a real Go module cache and Go never garbage-collects it.
Two kinds of growth are expected, and one is a defect.

Expected: superseded versions after a pin bump, which the bump runbook removes,
and the breadth of `go mod download all`, which is the whole module graph
including test-only dependencies and their fixtures (`pierrec/lz4` is 75 MB of
`testdata/`, `klauspost/compress` 46 MB). A second Go toolchain also lands here,
`golang.org/toolchain@...` at roughly 310 MB with its zip, whenever a builddir
`go` directive is newer than the host toolchain and `GOTOOLCHAIN=auto`.

A defect, because each one means a build resolved over the network instead of
through the pins:

| What you find | What it means |
|---------------|---------------|
| `github.com/ze-software/ze@v0.0.0-<date>-<hash>` | Ze was fetched from the proxy. The builddir replaces Ze with the working tree, so a build that reached the proxy for Ze did not read the builddir and compiled a PUSHED commit rather than your tree |
| A version of a builddir-pinned module that is not the pinned one | `gok` fell back to `go get` and took whatever upstream had. For `github.com/rtr7/kernel` that is the appliance's KERNEL |

Timestamps under `cache/download/*/@v/` reconstruct which build fetched what, to
the minute. A reappearance is a regression in whatever new path prepares an
instance: find that path rather than deleting the directory.
`TestPrepareRealInstanceCarriesEveryModule` and
`TestPreparedModulesResolveIdenticallyToTracked` gate preparation against the
real eight-module instance, the second by comparing `go list -m all` before and
after preparation.

<!-- source: internal/appliance/instance/prepare_repo_test.go -- TestPrepareRealInstanceCarriesEveryModule, TestPreparedModulesResolveIdenticallyToTracked -->

### Cache permissions

Go's default cache permissions leave directories `r-x`, which makes git unable
to delete or overwrite modcache files on a later checkout or rebase. Anything
that downloads into `gokrazy/modcache/` carries `-modcacherw`
(`GOFLAGS=-modcacherw`). `ze appliance build` sets it through `ensureModcacheRW`
(`internal/appliance/cmd_build.go`) and `ze-gok` sets it in
`cmd/ze-gok/main.go`. Keep the flag when running `go mod download` by hand. A
cache written before the flag existed needs a one-time
`chmod -R u+w gokrazy/modcache`.

<!-- source: internal/appliance/cmd_build.go -- ensureModcacheRW -->

## Which boot proof asserts what

| Proof | What it does | Use it for |
|-------|--------------|------------|
| `./le qemu vpp-hugepages-test` | Builds a real image through `ze appliance build`, boots it in QEMU, asserts the kernel command line and the reserved hugepage count | The default boot proof |
| `./le deployment gokrazy-l2tp-ppp-test` | Builds the appliance and boots it against a real LAC | The L2TP path |
| `test/appliance/serial-login.ci` | Boots nothing. Its header says the QEMU plan applies "when appliance serial test infrastructure is ready"; it asserts the argv[0] shell-invocation gate offline | Never a boot proof |

An image build alone is not a boot proof.

## Root-module pseudo-version pins

Some root `go.mod` direct dependencies are pinned to pseudo-versions
(`v0.0.0-<date>-<hash>`) because their upstreams publish no semver tag. This is
not a defect, and a reviewer should not "fix" it.

| Root dependency | Pin form | Upstream semver tag |
|-----------------|----------|---------------------|
| `github.com/gokrazy/tools` | pseudo-version | none published |
| `github.com/gokrazy/updater` | pseudo-version | none published |
| `github.com/insomniacslk/dhcp` | pseudo-version | none published |
| `github.com/packetcap/go-pcap` | pseudo-version | none published |
| `golang.zx2c4.com/wireguard/wgctrl` | pseudo-version | none published |

Confirm with `go list -m -versions`, `proxy.golang.org/<mod>/@v/list` and
`@latest` before classifying a pseudo-version pin as a defect. A module that
leaves this table has either been tagged or moved; find out which before
re-adding a row.

## GPLv2 source offer for the shipped kernel

The appliance image ships a GPLv2 Linux kernel, `github.com/rtr7/kernel`
(`gokrazy/ze/builddir/github.com/rtr7/kernel/go.mod`, pinned as an indirect
pseudo-version). Distributing a GPLv2 binary obliges the distributor to make the
corresponding source available, typically through a written offer accompanying
the image. No source-offer compliance sign-off is recorded today. That is a
licensing decision, not an engineering one.
