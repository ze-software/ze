# 1254 -- gokrazy derived parent discards every builddir pin

## Context

`gokrazy/modcache/` had grown to 2.1 GB. 1.23 GB of it was nine extracted copies
of **ze itself**, at nine consecutive pseudo-versions, one per commit pushed
between 18 and 22 July, each fetched from the module proxy with its zip. That
should be impossible: `gokrazy/ze/builddir/github.com/ze-software/ze/go.mod`
replaces ze with the working tree, so no build that reads the builddir can reach
the proxy for ze.

The builds in question did not read the builddir. `materializeDerivedParent`
(`internal/appliance/kernelargs.go`), which prepares a temp instance whenever an
appliance config requests hugepages, deliberately excluded `builddir` from what it
staged. `gok` resolves the builddir relative to the instance dir it chdirs into,
so it found none, synthesized an empty module, and resolved every package with
`go get` (`vendor/github.com/gokrazy/tools/packer/gotool.go`
`getPkg`/`getIncomplete`). The disk was the symptom; the defect was that those
images were built from whatever upstream happened to have.

The kernel is the part that matters. `github.com/rtr7/kernel` has been pinned to
`v0.0.0-20260403073601-5a996da3a37b` since the appliance landed (86960d858, never
changed), yet the cache held `@20260705070647` fetched 2026-07-18 15:02 and
`@20260719062436` fetched 2026-07-20 11:16, each one to two minutes before that
build's ze self-copy. Hugepage appliance images from those days shipped an
unpinned Linux kernel.

## Decisions

- The derived parent now **copies** the builddir and rewrites each go.mod's
  filesystem-path replaces to absolute paths, over symlinking it (the relative
  self-replace is six levels up, so it is depth-sensitive and a symlink or a plain
  copy both break it) and over synthesizing a module (`gok` cannot reproduce the
  self-replace, and the tracked module is also what makes `ze-gokrazy-deps`
  populate the cache).
- The rewrite is **generic**, driven by `modfile.IsDirectoryPath`, not a special
  case for the ze module. Version replaces (`serial-busybox`, `rtr7/kernel`) are
  left alone. This promoted `golang.org/x/mod` from indirect to direct; it was
  already vendored and in `go.mod`.
- A builddir that yields **no go.mod is an error**, not a quiet copy. The failure
  mode being defended against is silent: a missing pin does not fail a build, it
  changes what the build produces.
- The derived parent materializes under project `tmp/`, not the system temp dir,
  which the old code used in violation of `ai/rules/testing.md`.
- Only the derived-parent defect was fixed here. Preparing for *every* build,
  making the out-of-tree kernel an explicit parameter, and collapsing the three
  copies of "prepare an instance elsewhere" stay in
  `plan/learned/1262-gokrazy-builddir-tmp.md`.

## Consequences

- A hugepage appliance build now resolves entirely from `gokrazy/modcache`:
  verified with `GOPROXY=off`, exit 0, no `go get` in the log, cache byte-stable
  at 811 MB, only the pinned kernel present. The same A/B failed on 2026-07-22
  before the fix.
- `gokrazy/modcache` stops growing by ~140 MB per pushed commit.
- Cache growth regains diagnostic value: a `github.com/ze-software/ze@...`
  directory, or an off-pin copy of a pinned module, now means some path is
  preparing an instance without the builddir. `ai/rules/appliance-dep-bumps.md`
  records how to read the cache and why `rm -rf gokrazy/modcache` is never the
  answer (60 tracked files live inside it).

## Gotchas

- **The cache is evidence.** The pin escape had been live for at least five days
  and no gate noticed, because a missing pin makes the build *succeed* against a
  newer version. What noticed was disk usage. Timestamps in `cache/download/*/@v/`
  reconstruct which build fetched what, to the minute.
- **`go mod vendor` reverts the updater hard fork.** Running it (needed after the
  `x/mod` promotion) silently undid `vendor/github.com/gokrazy/updater/updater.go`.
  `scripts/dev/reapply-updater-fixes.py` restores it; run it after any re-vendor
  and confirm `git diff vendor/` is empty.
- **The test that asserted the bug.** `TestMaterializeDerivedParent` explicitly
  checked that builddir was *absent* from the derived dir, citing a cold-rebuild
  rationale. A green test can encode the defect; the comment explaining why it was
  deliberate was written by the same hand that made the mistake.
- Both replacement assertions were mutation-verified: re-adding the exclusion, and
  disabling the replace rewrite, each turn the test red.
- Not proven at first: the image was not booted, and chasing that down took
  three more defects. See the follow-up below; the boot proof now PASSES.

## Follow-up: the boot proof could never have passed (same day)

Getting the QEMU boot proof to run turned up three defects stacked on each other,
each one hiding the next behind the same misleading message.

1. **The accelerator probe tested existence, not access.**
   `effective-vpp-hugepages-qemu.py` chose `accel=kvm` on
   `Path("/dev/kvm").exists()`. The node exists for every user; it is `root:kvm`
   0660. Outside the group qemu does not fall back, it exits with "Could not
   access KVM kernel module: Permission denied", and the script reported
   "appliance did not reach SSH within the timeout (no KVM / slow tcg?)". Fixed
   in all three QEMU evidence scripts with `os.access(..., R_OK|W_OK)`, plus an
   explicit `darwin -> hvf` branch where it was missing (macOS has no `/dev/kvm`
   and was silently running tcg).
2. **The proof asked the appliance a question it cannot answer.** With KVM
   working, the appliance booted in ~10 seconds and logged `SSH server listening`
   — and the proof still "timed out". The script ran `cat /proc/cmdline` over
   SSH, but the appliance's SSH server IS the Ze CLI, not a Unix shell. It
   answered `error: unknown command`, `ssh_capture` treated any non-zero exit as
   "not up yet", retried for 180s and blamed the boot. **This proof had never
   passed and could not have.** It now asks `show host kernel | json`.
3. **The reservation was not observable through the CLI.** `show host memory`
   had no hugepage fields, so "the kernel actually reserved the pages" — the half
   the cmdline cannot prove — had no operator-visible source. Added
   `hugepages-total`, `hugepages-free`, `hugepage-size-bytes`.

Result: `VPP-HUGEPAGES-QEMU: PASS cmdline has hugepages=64, hugepages-total=64`,
mutation-verified (making `hugepageKernelArgs` return nil turns it red with the
missing argument named). R-2 in the spec is closed.

## Follow-up decisions

- `HugePages_Total` is a COUNT; `Hugepagesize` is kB. The existing meminfo
  parser multiplied everything by 1024, so counts needed a second map
  (`meminfoCounts`) rather than a new entry in `meminfoFields`. 64 pages would
  otherwise have been reported as 65536.
- Under a hardware accelerator, an appliance that never answers is now a **FAIL**,
  not a SKIP. A SKIP there is how this stayed broken; only the tcg case may skip.
  The serial console is captured to a file and its tail printed on failure,
  instead of being discarded to `/dev/null`.
- `make ze-setup` checks `kvm-access` and can apply `sudo usermod -aG kvm`,
  mirroring the existing `userns-unrestricted` system-state check. It reports
  `pending` (in the group, session predates it), `missing`, or `n/a` (no
  `/dev/kvm`, tcg is legitimate) as distinct states, because after `usermod` the
  group database and the running process disagree, and that difference is the
  difference between "run this" and "log back in".

## Follow-up gotchas

- **A SKIP is not a pass, and this one had been green for months.** The `.ci`
  accepts `PASS|SKIP`, so the vacuous proof never showed up as a failure. When a
  test's skip path is reachable in normal operation, the skip reason must name
  the specific missing prerequisite, or it becomes a place for defects to hide.
- **The failure message named the wrong subsystem twice.** "did not reach SSH"
  was false (SSH was reached in 10s) and "slow tcg?" was false (kvm was in use).
  A diagnostic that guesses is worse than one that reports what it saw: the
  rewritten one prints the last SSH error verbatim.
- CI is unaffected either way: `.github/workflows/evidence-nightly.yml` keeps
  `ze-qemu-integration-test` out on purpose because GitHub-hosted runners do not
  reliably provide nested virt, and `scripts/dev/github_workflows_test.go` pins
  that shape. The kvm-access check is a developer-host concern.

## Files

- `internal/appliance/kernelargs.go` -- `materializeDerivedParent` copies the
  builddir; new `copyBuildDir` and `absolutizeReplaces`
- `internal/appliance/kernelargs_test.go` -- rewritten derived-parent test plus
  fail-closed, isolation, and version-replace tests
- `ai/rules/appliance-dep-bumps.md` -- module cache hygiene section
- `plan/learned/1262-gokrazy-builddir-tmp.md` -- A-1b confirmed, landed work recorded
- `go.mod` -- `golang.org/x/mod` promoted to a direct dependency
