# 1258 -- The QEMU gate ran a feature-stripped daemon, and every run hijacked cache/

## Context

`test/firewall` (23 tests) and `test/policy` (6) were failing 27-for-27 on the
development host, and had been read as a product problem. They were not. Every one
of them programs nftables, which needs `CAP_NET_ADMIN`, and their authors knew:
`test/firewall/001-boot-apply.ci` says in prose "Requires CAP_NET_ADMIN on the test
host. Without it, the nft backend Apply() fails with EPERM". That requirement was
written as a COMMENT, which no runner can read, while the runner has had a
machine-readable marker for exactly this since `internal/test/runner/caps.go`
(`capsNetAdmin`). Chasing that surfaced two infrastructure defects sitting
underneath, each of which invalidated the QEMU gate in a different way.

## Decisions

- **A capability requirement goes in `option=needs-linux:caps=net-admin`, never in
  a comment.** Replaced `option=skip-os:value=darwin` on all 27 files (plus
  `test/l2tp/session-stopccn-cascade.ci`). `needs-linux` strictly supersedes
  `skip-os:value=darwin`: it skips on every non-Linux host AND on a Linux host
  lacking the capability. Chosen over leaving them red, and over deleting them.

- **Marking them was not a coverage reduction, it was the opposite.**
  `record_parse.go:236` skips every test NOT marked `needs-linux` when
  `ZE_QEMU_LINUX_ONLY=1`. So before this change the 27 failed on the host and were
  SKIPPED in the QEMU run: they executed successfully nowhere. The marker is what
  enrols them in `make ze-qemu-needs-linux-test`. Firewall went from 21 failing to
  19 of 21 passing in the VM.

- **The QEMU daemon must be built with `TestBuildTags`' tag set.** Both
  `ze-qemu-all-test` and `ze-qemu-needs-linux-test` built `ZE_QEMU_BIN` with
  `-tags 'ze_core zetest ze_distro'` -- no `ze_setup`, no `$(ZE_FEATURES)` -- so the
  VM exercised a daemon with all 16 feature gates OFF: no ssh, no web, and no BGP.
  `internal/test/runner/runner.go:43-49` had already solved this for the host
  binary by deriving tags from `feature-gates.txt` "without a hand-maintained
  list"; the QEMU targets kept one anyway.

- **`cache/` is repointed only when its current target is missing.**
  `ensure-links.py` unconditionally repointed a symlink whose target differed.
  `tmp/` is keyed on the checkout path, so a mismatch there is real drift (the
  checkout moved) and must be followed. `cache/` is keyed on HOME/XDG, so a
  mismatch means a foreign environment. Added `repoint_live=False` for `cache/`
  rather than special-casing the VM, because any foreign HOME has the same effect.

## Consequences

- The QEMU gate is now trustworthy signal. Its remaining failures are real bugs,
  not build-tag artifacts: 26 after the fix, down from 43. Two examples that were
  previously unreachable -- `firewall 4` gets far enough to time out driving
  `ze cli` over SSH (impossible before: `ze_ssh` was compiled out), and
  `firewall 9` shows nft programming `elements = { 10.0.0.1, 10.0.0.2 }` with no
  per-element timeout where the test expects `10.0.0.1 timeout 1h`.
- `test/policy` still fails 6-for-6 in the VM, and it is NOT the marker: as root it
  fails with `netlink receive: operation not supported` (EOPNOTSUPP, not EPERM),
  which points at the VM kernel lacking `type route` chain support. Unresolved.
- A QEMU run can no longer corrupt the host checkout's `cache/` symlink.

## Gotchas

- **The cache hijack was invisible for as long as `cache/` held nothing critical.**
  `qemu-run.py` exports `HOME=/root`, `qemu-all-tests.sh` runs `make
  ze-unit-test-cached` inside the VM, that target depends on `ze-ensure-links`, and
  the repo is 9p-mounted read-write. It only became a hard failure when `GOCACHE`
  moved to `cache/go-cache` (`Makefile:17`), after which the next host build died
  with `failed to initialize build cache ... mkdir cache: file exists`.
- **A green run that did nothing looks exactly like a green run.** The first
  re-run reported ZERO failures, down from 43. It had not started: the build died
  on the dangling symlink before the VM booted. Always confirm a suite EXECUTED
  before believing an improvement.
- `ensure_symlink` created its target directory before deciding whether to use it,
  so a foreign unwritable HOME raised `PermissionError` instead of declining.

## Files

- `test/firewall/*.ci` (21), `test/policy/*.ci` (6), `test/l2tp/session-stopccn-cascade.ci`
- `mk/test-integration.mk` -- both QEMU targets' daemon build tags
- `scripts/dev/ensure-links.py`, `scripts/dev/ensure_links_test.py` (new, 5 tests)
