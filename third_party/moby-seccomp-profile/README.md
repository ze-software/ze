# Moby's default seccomp profile -- the source of a container's default filter

**This is foreign data. It is not built into any ze binary.** It is the file
the default system-call profile of a container was derived from, kept so the
derivation can be checked.

| Field | Value |
|-------|-------|
| Upstream | [github.com/moby/profiles](https://github.com/moby/profiles), `seccomp/default.json` |
| Commit | `f862455628f78f84c937e967e4a03902fb1dee11` (2026-11-13) |
| SHA-256 of `default.json` | `3320527d4568265ac97a077609b1bfd4cfd0d789eeb9239b7e094b1526675406` |
| Licence | Apache-2.0, `LICENSE` beside this file. The upstream repository has no NOTICE file |
| Copyright | The Moby project authors, not Exa Networks |

## What was derived from it

`internal/plugins/container/seccomp/profile.go` writes the profile's rules as
Go values, in the profile's order, with its default (EPERM for an unlisted
call) and its selection of a rule by architecture and by the container's
bounding capability set. Three things differ, and the page
`docs/architecture/container/runtime.md`, "The system-call filter", says why:

- A call name neither of Ze's two tables holds (x86-64, AArch64) is left out:
  the 32-bit calls such as `chown32`, `fcntl64` and the `*_time64` calls,
  `umount` and `stime`.
- The rules selected only on an architecture Ze ships no table for (ppc64le,
  arm, s390, riscv64) are left out.
- The rule the source applies from kernel 4.8 (`ptrace`, `process_vm_readv`,
  `process_vm_writev`) is applied always.

`TestDefaultProfileFollowsTheSourceProfile` reads `default.json` here, selects
its rules as the source does, and compares the result with Ze's profile for
both architectures and several capability sets. A refresh of this file from a
newer upstream commit updates the table above and fails that test until
`profile.go` follows it.
