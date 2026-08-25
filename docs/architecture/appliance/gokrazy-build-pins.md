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

<!-- source: scripts/evidence/effective-vpp-hugepages-qemu.py -- accelerator selection and the hugepage boot proof -->
<!-- source: scripts/le/devtools/system.py -- Kvm, kvm_state -->
<!-- source: scripts/le/application/setup.py -- _visit_kvm, the kvm-access pending/missing/n-a states -->

`./le setup` reports `kvm-access` as `present`, `pending`, `missing`, or
`n/a`. `pending` means the user is in the `kvm` group but the running session
predates it. That difference is the difference between "run this" and "log back
in".

**A page count is not kilobytes.** `HugePages_Total` is a count and
`Hugepagesize` is in kB. The meminfo parser multiplied everything by 1024, so
counts needed their own map. 64 pages would otherwise report as 65536.

<!-- source: internal/component/host/memory_linux.go -- meminfoCounts against meminfoFields -->
