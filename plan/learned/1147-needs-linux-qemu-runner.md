# 1147 — needs-linux QEMU runner

## Context
Stop Linux-only `.ci` functional tests from failing natively on darwin: mark them
and run them automatically inside the QEMU Alpine VM instead, so `make ze-verify`
stays green and fast on the dev host while the Linux surface is still validated.

## Decisions
- New runner directive `option=needs-linux`: on a non-Linux host the test is
  SKIPPED with a reason pointing at the QEMU runner; inside the VM (`GOOS==linux`)
  the directive is inert and the test runs normally.
- One VM for the whole Linux-only surface, not one VM per test. `make
  ze-qemu-needs-linux-test` boots a single VM with `ZE_QEMU_LINUX_ONLY=1`, which
  flips the runner to skip every test NOT marked `needs-linux`.
- First consumers: the cos VLAN tests (`test/plugin/cos-*.ci`) that boot a daemon
  applying interface/VLAN config.

## Patterns
- `Record.NeedsLinux bool` (`internal/test/runner/record.go`).
- Directive handled in `record_parse.go` `needs-linux` case; the
  `ZE_QEMU_LINUX_ONLY` finalization filter is applied after all options parse and
  never overrides an existing skip reason.

## Gotchas
- The directive is INERT inside the VM — do not also gate it behind `skip-os`.
- `ZE_QEMU_LINUX_ONLY=1` is the inverse filter: it skips everything that is NOT
  `needs-linux`, so the VM spends its time only on the Linux-only tests.

## Files
- `internal/test/runner/{record.go,record_parse.go}`, `mk/test-integration.mk`
  (`ze-qemu-needs-linux-test`), `ai/rules/qemu-testing.md`,
  `docs/architecture/testing/ci-format.md`.
