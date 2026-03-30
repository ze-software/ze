# 494 -- iface-5 VM Integration Tests

## Context

The iface plugin (Phases 1-4) had thorough unit tests for input validation and mocked logic, but zero test coverage against the real Linux kernel. Every netlink call (interface create/delete, address add/remove, MTU), every tc operation (qdisc, mirred filter), every sysctl write, and the full DHCPv4 DORA cycle were untested against actual kernel APIs. The gap meant the code could pass all tests but fail on a real machine.

## Decisions

- Chose network namespaces over Docker containers for test isolation, because namespaces are lighter, faster, and provide full `CAP_NET_ADMIN` without external tooling. Docker would have added orchestration complexity for no benefit here.
- Used `//go:build integration && linux` double tag plus `_integration_linux_test.go` filename suffix, so tests are excluded from both `make ze-verify` (no `integration` tag) and non-Linux builds (implicit `_linux` constraint). Chose this over a single `integration` tag because the tests are fundamentally Linux-only (netlink, /proc/sys, tc).
- Reused `collectingBus` and `subscribableBus` from existing test files rather than creating new mock types, since all files share the `iface` package.
- Used in-process DHCPv4 server (`insomniacslk/dhcp/dhcpv4/server4`) over external `dnsmasq`, because it runs inside the same network namespace with zero setup.
- Skipped DHCPv6 integration test over attempting it, because `server6` requires link-local address setup, multicast group joins, and DUID configuration that make a reliable in-process test prohibitively complex.
- Named make targets `ze-integration-iface-test` (specific) and `ze-integration-test` (umbrella) to establish a pattern for future integration test suites.

## Consequences

- `make ze-integration-test` is now the command for kernel-level validation. Requires `sudo` or `CAP_NET_ADMIN` on Linux.
- `make ze-verify` is unaffected -- integration tests are invisible without the build tag.
- `vishvananda/netns` was promoted from indirect to direct dependency (already in go.mod via netlink).
- The `withNetNS` helper and associated utilities (`linkExists`, `hasAddress`, `waitForEvent`) are reusable for any future kernel-level tests in the iface package.
- Future integration test suites (e.g., for BGP TCP operations) can follow the same pattern and be added to `ze-integration-test`.

## Gotchas

- `runtime.LockOSThread()` is mandatory before any namespace switch. Without it, the Go scheduler can move the goroutine to a different OS thread, leaving the test running in the wrong namespace. Forgetting this causes silent, baffling test failures.
- Monitor tests need a 200ms sleep after `Start()` before creating interfaces, because the netlink subscription is asynchronous. Without the delay, events are missed.
- The `sysctlRoot` variable must be saved and restored in sysctl integration tests, because unit tests override it to a temp directory. If not restored, subsequent unit tests in the same run would break.
- Namespace names are derived from test names but must be truncated to 15 characters (IFNAMSIZ). Long test names like `TestIntegrationMirrorRemoveIdempotent` silently break namespace creation without truncation.

## Files

- `internal/component/iface/integration_helpers_linux_test.go` -- shared helpers
- `internal/component/iface/manage_integration_linux_test.go` -- 9 manage tests
- `internal/component/iface/monitor_integration_linux_test.go` -- 5 monitor tests
- `internal/component/iface/sysctl_integration_linux_test.go` -- 2 sysctl tests
- `internal/component/iface/mirror_integration_linux_test.go` -- 5 mirror tests
- `internal/component/iface/dhcp_integration_linux_test.go` -- 2 DHCP tests
- `internal/component/iface/migrate_integration_linux_test.go` -- 2 migration tests
- `Makefile` -- `ze-integration-iface-test`, `ze-integration-test` targets
- `docs/functional-tests.md` -- integration test documentation
