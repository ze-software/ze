# Spec: iface-5 -- VM Integration Tests for Interface Management

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-iface-4-advanced |
| Phase | - |
| Updated | 2026-03-30 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-iface-0-umbrella.md` -- umbrella context
3. `internal/component/iface/` -- all `_linux.go` and `_test.go` files
4. `internal/component/iface/manage_linux.go` -- netlink operations

## Task

Add integration tests for the iface plugin that exercise real Linux kernel features: netlink interface management, netlink monitoring, sysctl writes, tc mirroring, DHCP client lifecycle, and make-before-break migration. These tests require `CAP_NET_ADMIN` (root or network namespace) and cannot run in normal CI. They validate that the code actually works against the kernel, filling the gap between the current unit tests (input validation and mocked logic only) and real-world operation.

### Why VM Tests

The current test suite covers input validation and in-process logic thoroughly. However, every netlink call, every tc operation, every DHCP exchange, and every sysctl write is untested against the actual kernel. The unit tests pass with mocks but cannot prove that `CreateDummy` creates a dummy interface, that `SetupMirror` installs a working mirred filter, or that `DHCPClient` obtains a real lease.

### Test Isolation Strategy

All tests run inside ephemeral Linux **network namespaces** created per test case via `netns.NewNamed()` + `netns.Set()`. This provides:
- Full `CAP_NET_ADMIN` within the namespace (no host interference)
- Clean teardown (deleting namespace removes all interfaces, addresses, routes, qdiscs)
- Parallelism-safe (each test gets its own namespace)
- No real network required for most tests (loopback + veth pairs suffice)

DHCP tests additionally need a DHCP server (`dnsmasq` or the Go `dhcpv4/server4` package) running inside the test namespace on one end of a veth pair.

### Build Tags

All VM test files use a combined build constraint: `//go:build integration && linux`. The `linux` tag is implicit (Go sets it from GOOS) but stated explicitly to make intent clear -- these tests use netlink, /proc/sys, and tc which are Linux-only kernel APIs. The `integration` tag prevents them from running in normal CI. Run with `go test -tags integration -count=1 ./internal/component/iface/...` on a Linux machine with `CAP_NET_ADMIN`, or via the dedicated `make ze-integration-iface-test` target.

The test file naming uses `_integration_linux_test.go` suffix (not `_integration_test.go`) so that Go's implicit `_linux` build constraint also applies. This means: the files compile only on Linux AND only with `-tags integration`.

## Required Reading

### Architecture Docs
- [ ] `plan/spec-iface-0-umbrella.md` -- Bus topics, payload format, OS operations table
  -> Constraint: all netlink operations are defined in the umbrella
  -> Decision: Linux-only via `_linux.go` suffixes
- [ ] `.claude/rules/testing.md` -- test patterns, build tags, make targets
  -> Constraint: `//go:build integration` for tests needing kernel features
  -> Constraint: one change, one test, then scale
- [ ] `.claude/rules/goroutine-lifecycle.md` -- long-lived workers
  -> Constraint: monitor and DHCP client are long-lived goroutines

### RFC Summaries (MUST for protocol work)
- [ ] DHCPv4/v6 RFCs -- referenced in spec-iface-4-advanced
  -> Constraint: DORA cycle, T1/T2 renewal, IA_NA/IA_PD

**Key insights:**
- Network namespaces give full kernel isolation per test without root on the host
- `vishvananda/netns` package handles namespace creation/switching in Go
- `runtime.LockOSThread()` is mandatory before switching namespaces (Go scheduler moves goroutines between OS threads)
- DHCP tests need a server process; `insomniacslk/dhcp/dhcpv4/server4` can run in-process

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/iface/manage_linux.go` -- CreateDummy, CreateVeth, CreateBridge, CreateVLAN, DeleteInterface, AddAddress, RemoveAddress, SetMTU. All use `vishvananda/netlink` directly. No mocking layer.
- [ ] `internal/component/iface/manage_linux_test.go` -- Tests only `validateIfaceName`, `validateVLANID`, `validateMTU`. Zero netlink coverage.
- [ ] `internal/component/iface/monitor_linux.go` -- Monitor with `netlink.LinkSubscribe` and `netlink.AddrSubscribe`. Long-lived goroutine. Publishes to Bus.
- [ ] `internal/component/iface/monitor_linux_test.go` -- Tests event dispatch logic with `collectingBus` mock. Never calls `netlink.LinkSubscribe`.
- [ ] `internal/component/iface/sysctl_linux.go` -- Writes to `/proc/sys/net/...`. Uses overridable `sysctlRoot` for tests.
- [ ] `internal/component/iface/sysctl_linux_test.go` -- Full coverage via temp dir override. Never writes real `/proc/sys`.
- [ ] `internal/component/iface/slaac_linux.go` -- Thin wrapper over sysctl. No kernel interaction tested.
- [ ] `internal/component/iface/dhcp_linux.go` -- DHCPv4/v6 client. Full DORA/SARR cycle. Installs addresses via `netlink.AddrReplace`. Zero test coverage.
- [ ] `internal/component/iface/mirror_linux.go` -- tc qdisc + matchall + mirred. SetupMirror, RemoveMirror.
- [ ] `internal/component/iface/mirror_linux_test.go` -- Tests only empty-name validation. Zero tc coverage.
- [ ] `internal/component/iface/migrate_linux.go` -- 5-phase MigrateInterface. Uses CreateDummy/AddAddress/RemoveAddress/DeleteInterface + Bus subscription.
- [ ] `internal/component/iface/migrate_linux_test.go` -- Tests config validation, `resolveOSName`, `stripPrefix`, `bgpReadyConsumer`. Phase 2-4 operations fail without root (error path only).

**Behavior to preserve:**
- All existing unit tests continue to pass without `integration` build tag
- No changes to production code (this spec adds tests only)
- `make ze-unit-test` and `make ze-verify` unaffected (no integration tag)

**Behavior to change:**
- None -- this spec adds test files only

## Data Flow (MANDATORY)

### Entry Points
- Test code calls iface package functions directly (manage, mirror, sysctl)
- Test code starts Monitor and verifies Bus events
- Test code starts DHCPClient against an in-process DHCP server
- Test code calls MigrateInterface with a real Bus and real interfaces

### Transformation Path
1. Test creates network namespace
2. Test creates interfaces/addresses via iface package functions
3. Test verifies kernel state via netlink queries (`netlink.LinkByName`, `netlink.AddrList`, `netlink.QdiscList`)
4. For monitor tests: test modifies kernel state, verifies Bus events appear
5. For DHCP tests: test runs server + client in namespace, verifies lease and address

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Test -> Kernel | netlink syscalls via vishvananda/netlink | [ ] |
| Test -> /proc/sys | sysctl file reads for verification | [ ] |
| Test -> DHCP server | UDP in network namespace | [ ] |
| Monitor -> Bus | Real events from kernel via netlink multicast | [ ] |

### Integration Points
- `internal/component/iface/manage_linux.go` -- all create/delete/addr/mtu functions
- `internal/component/iface/monitor_linux.go` -- Monitor.Start/Stop with real netlink
- `internal/component/iface/sysctl_linux.go` -- real /proc/sys writes
- `internal/component/iface/mirror_linux.go` -- real tc qdisc/filter operations
- `internal/component/iface/dhcp_linux.go` -- real DHCP exchange
- `internal/component/iface/migrate_linux.go` -- real 5-phase migration

### Architectural Verification
- [ ] No bypassed layers (tests use same public API as production code)
- [ ] No unintended coupling (tests import only the iface package + netlink for verification)
- [ ] No duplicated functionality (tests verify existing code, add nothing new to production)
- [ ] Zero-copy preserved where applicable (N/A -- tests only)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `CreateDummy("test0")` | -> | `netlink.LinkAdd` creates real interface | `TestIntegrationCreateDummy` |
| `Monitor.Start()` + external link change | -> | Bus receives `interface/created` | `TestIntegrationMonitorLinkEvents` |
| `SetupMirror("src","dst",true,false)` | -> | tc qdisc + filter installed | `TestIntegrationMirrorIngress` |
| DHCPClient.Start() with DHCP server | -> | lease obtained, address installed | `TestIntegrationDHCPv4Lease` |
| `MigrateInterface(cfg, bus, timeout)` | -> | 5-phase migration completes | `TestIntegrationMigrateFullCycle` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `CreateDummy` called in namespace | Interface exists and is UP (verified via `netlink.LinkByName`) |
| AC-2 | `CreateVeth` called in namespace | Both ends exist and are UP |
| AC-3 | `CreateBridge` called in namespace | Bridge exists and is UP |
| AC-4 | `CreateVLAN` on a dummy parent | VLAN subinterface exists with correct VLAN ID |
| AC-5 | `DeleteInterface` on existing dummy | Interface no longer exists |
| AC-6 | `AddAddress` with IPv4 CIDR | Address appears in `netlink.AddrList` |
| AC-7 | `AddAddress` with IPv6 CIDR | Address appears in `netlink.AddrList` |
| AC-8 | `RemoveAddress` on existing address | Address no longer in `netlink.AddrList` |
| AC-9 | `SetMTU` on existing interface | MTU matches (verified via `link.Attrs().MTU`) |
| AC-10 | Monitor running, create dummy | Bus receives `interface/created` event with correct name |
| AC-11 | Monitor running, add address | Bus receives `interface/addr/added` with correct address and family |
| AC-12 | Monitor running, remove address | Bus receives `interface/addr/removed` |
| AC-13 | Monitor running, delete interface | Bus receives `interface/deleted` |
| AC-14 | Monitor running, link up/down | Bus receives `interface/up` and `interface/down` |
| AC-15 | `SetIPv4Forwarding` on real interface | `/proc/sys/net/ipv4/conf/<iface>/forwarding` reads "1" |
| AC-16 | `EnableSLAAC` on real interface | `/proc/sys/net/ipv6/conf/<iface>/autoconf` reads "1" |
| AC-17 | `SetupMirror` ingress-only | Ingress qdisc exists with matchall+mirred filter (verified via `netlink.QdiscList` and `netlink.FilterList`) |
| AC-18 | `SetupMirror` egress-only | Clsact qdisc exists with egress filter |
| AC-19 | `SetupMirror` both directions | Clsact qdisc with both ingress and egress filters |
| AC-20 | `RemoveMirror` after setup | No qdisc remains on the interface |
| AC-21 | `RemoveMirror` on interface with no mirroring | No error (idempotent) |
| AC-22 | DHCPv4 client with in-process server | Client obtains lease, address installed on interface, Bus event published |
| AC-23 | DHCPv4 client stopped | Leased address removed from interface |
| AC-24 | DHCPv6 client with in-process server (IA_NA) | Client obtains address, installed on interface |
| AC-25 | `MigrateInterface` with real interfaces and mock BGP readiness | New interface created, IP added, old IP removed after readiness signal |
| AC-26 | `MigrateInterface` timeout (no BGP signal) | Rolled back: new IP removed, new interface deleted |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestIntegrationCreateDummy` | `internal/component/iface/manage_integration_linux_test.go` | AC-1: dummy interface created and UP | |
| `TestIntegrationCreateVeth` | `internal/component/iface/manage_integration_linux_test.go` | AC-2: veth pair created, both UP | |
| `TestIntegrationCreateBridge` | `internal/component/iface/manage_integration_linux_test.go` | AC-3: bridge created and UP | |
| `TestIntegrationCreateVLAN` | `internal/component/iface/manage_integration_linux_test.go` | AC-4: VLAN subinterface with correct ID | |
| `TestIntegrationDeleteInterface` | `internal/component/iface/manage_integration_linux_test.go` | AC-5: interface removed | |
| `TestIntegrationAddIPv4Address` | `internal/component/iface/manage_integration_linux_test.go` | AC-6: IPv4 address present | |
| `TestIntegrationAddIPv6Address` | `internal/component/iface/manage_integration_linux_test.go` | AC-7: IPv6 address present | |
| `TestIntegrationRemoveAddress` | `internal/component/iface/manage_integration_linux_test.go` | AC-8: address gone | |
| `TestIntegrationSetMTU` | `internal/component/iface/manage_integration_linux_test.go` | AC-9: MTU matches | |
| `TestIntegrationMonitorLinkCreated` | `internal/component/iface/monitor_integration_linux_test.go` | AC-10: Bus event on link create | |
| `TestIntegrationMonitorAddrAdded` | `internal/component/iface/monitor_integration_linux_test.go` | AC-11: Bus event on addr add | |
| `TestIntegrationMonitorAddrRemoved` | `internal/component/iface/monitor_integration_linux_test.go` | AC-12: Bus event on addr remove | |
| `TestIntegrationMonitorLinkDeleted` | `internal/component/iface/monitor_integration_linux_test.go` | AC-13: Bus event on link delete | |
| `TestIntegrationMonitorLinkUpDown` | `internal/component/iface/monitor_integration_linux_test.go` | AC-14: Bus events on state change | |
| `TestIntegrationSysctlIPv4Forwarding` | `internal/component/iface/sysctl_integration_linux_test.go` | AC-15: real /proc/sys written | |
| `TestIntegrationSysctlSLAAC` | `internal/component/iface/sysctl_integration_linux_test.go` | AC-16: real autoconf sysctl | |
| `TestIntegrationMirrorIngress` | `internal/component/iface/mirror_integration_linux_test.go` | AC-17: ingress qdisc + filter | |
| `TestIntegrationMirrorEgress` | `internal/component/iface/mirror_integration_linux_test.go` | AC-18: clsact + egress filter | |
| `TestIntegrationMirrorBoth` | `internal/component/iface/mirror_integration_linux_test.go` | AC-19: clsact + both filters | |
| `TestIntegrationMirrorRemove` | `internal/component/iface/mirror_integration_linux_test.go` | AC-20: qdisc removed | |
| `TestIntegrationMirrorRemoveIdempotent` | `internal/component/iface/mirror_integration_linux_test.go` | AC-21: no error on clean interface | |
| `TestIntegrationDHCPv4Lease` | `internal/component/iface/dhcp_integration_linux_test.go` | AC-22: lease obtained, address installed | |
| `TestIntegrationDHCPv4Stop` | `internal/component/iface/dhcp_integration_linux_test.go` | AC-23: address removed on stop | |
| `TestIntegrationDHCPv6Lease` | `internal/component/iface/dhcp_integration_linux_test.go` | AC-24: v6 address obtained | |
| `TestIntegrationMigrateFullCycle` | `internal/component/iface/migrate_integration_linux_test.go` | AC-25: full migration completes | |
| `TestIntegrationMigrateTimeout` | `internal/component/iface/migrate_integration_linux_test.go` | AC-26: rollback on timeout | |

### Boundary Tests (MANDATORY for numeric inputs)

Boundary tests for numeric inputs already exist in the unit test suite (manage_linux_test.go). This spec does not add new numeric boundaries -- it validates that values accepted by validation actually work against the kernel.

| Field | Range | Integration Test |
|-------|-------|-----------------|
| MTU 68 (minimum) | 68-16000 | `TestIntegrationSetMTU` with MTU=68 |
| MTU 16000 (maximum) | 68-16000 | `TestIntegrationSetMTU` with MTU=16000 |
| VLAN ID 1 (minimum) | 1-4094 | `TestIntegrationCreateVLAN` with ID=1 |
| VLAN ID 4094 (maximum) | 1-4094 | `TestIntegrationCreateVLAN` with ID=4094 |

### Functional Tests

No `.ci` functional tests for this spec. Integration tests are Go test files with the `integration` build tag, run via a dedicated make target. They are not `.ci` tests because they require `CAP_NET_ADMIN` and network namespaces, which the `.ci` test runner does not provide.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All integration tests | `internal/component/iface/*_integration_test.go` | Kernel operations work correctly | |

### Future (if deferring any tests)
- Packet-level mirror verification (send packet on src, capture on dst) -- deferred to chaos framework. Current tests verify tc configuration exists, not packet flow.
- DHCPv6 prefix delegation (IA_PD) integration test -- requires more complex server setup. Deferred until DHCPv6 PD is a user priority.
- SLAAC address appearance after enabling autoconf -- requires RA sender in namespace. Deferred to future spec.

## Dependencies

### Go packages (already in go.mod)
- `github.com/vishvananda/netlink` -- used by production code, also for test verification
- `github.com/insomniacslk/dhcp` -- used by production code, also for in-process DHCP server

### Go packages (new -- need user approval)
- `github.com/vishvananda/netns` -- network namespace creation and switching. Required for test isolation. Same author as netlink (Vishvananda). MIT license, 900+ stars.

### System requirements for running integration tests
- Linux kernel (namespaces, netlink, tc, /proc/sys)
- `CAP_NET_ADMIN` capability (or root)
- No external services (DHCP server runs in-process)

## Test Helper Design

### Namespace Helper

A shared test helper creates an ephemeral network namespace per test:

| Function | Purpose |
|----------|---------|
| `withNetNS(t, func())` | Creates namespace, switches to it, runs test func, restores original namespace, deletes namespace on cleanup |

Implementation constraints:
- Call `runtime.LockOSThread()` before namespace switch (Go scheduler must not move the goroutine)
- Call `runtime.UnlockOSThread()` in cleanup
- Use `t.Cleanup()` for namespace deletion
- Namespace name derived from test name (truncated to 15 chars for IFNAMSIZ)

### Bus Helper

Reuse the existing `collectingBus` / `subscribableBus` from the test files, extended with a `waitForTopic(topic, timeout)` method that blocks until an event on the given topic appears or times out.

### DHCP Server Helper

For DHCPv4 tests, start an in-process DHCP server using `dhcpv4/server4` from `insomniacslk/dhcp`:

| Function | Purpose |
|----------|---------|
| `startDHCPv4Server(t, ifaceName, subnet, gateway)` | Starts server on one end of a veth pair, returns cleanup func |

The server runs inside the same network namespace as the client. It serves a single lease from the configured subnet.

For DHCPv6 tests, use `dhcpv6/server6` similarly.

## Files to Modify

- `go.mod` -- add `github.com/vishvananda/netns` dependency (if approved)
- `Makefile` -- add `ze-integration-iface-test` target (iface-specific) and `ze-integration-test` umbrella target (runs all integration test suites)

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | -- |
| CLI commands/flags | No | -- |
| Functional test for new RPC/API | No | -- |
| Make target (specific) | Yes | `Makefile` -- `ze-integration-iface-test` |
| Make target (umbrella) | Yes | `Makefile` -- `ze-integration-test` (depends on `ze-integration-iface-test` + future suites) |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | -- |
| 2 | Config syntax changed? | No | -- |
| 3 | CLI command added/changed? | No | -- |
| 4 | API/RPC added/changed? | No | -- |
| 5 | Plugin added/changed? | No | -- |
| 6 | Has a user guide page? | No | -- |
| 7 | Wire format changed? | No | -- |
| 8 | Plugin SDK/protocol changed? | No | -- |
| 9 | RFC behavior implemented? | No | -- |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` -- document integration test tag, make target, namespace requirements |
| 11 | Affects daemon comparison? | No | -- |
| 12 | Internal architecture changed? | No | -- |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/component/iface/integration_helpers_linux_test.go` | Shared: `withNetNS`, `waitForTopic`, namespace bus helper |
| `internal/component/iface/manage_integration_linux_test.go` | Interface create/delete/addr/mtu against real kernel |
| `internal/component/iface/monitor_integration_linux_test.go` | Monitor with real netlink events |
| `internal/component/iface/sysctl_integration_linux_test.go` | Real /proc/sys writes and reads |
| `internal/component/iface/mirror_integration_linux_test.go` | tc qdisc/filter against real kernel |
| `internal/component/iface/dhcp_integration_linux_test.go` | DHCPv4/v6 with in-process server |
| `internal/component/iface/migrate_integration_linux_test.go` | Full 5-phase migration with real interfaces |

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Create, TDD Test Plan |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `go test -tags integration ./internal/component/iface/...` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report |

### Implementation Phases

1. **Phase: Test helpers** -- namespace helper, bus helper, DHCP server helper
   - Tests: Manual verification that `withNetNS` creates and cleans up namespace
   - Files: `integration_helpers_linux_test.go`
   - Verify: helper creates namespace, interface created inside is invisible outside

2. **Phase: Interface management** -- create/delete/addr/mtu integration tests
   - Tests: `TestIntegrationCreateDummy`, `TestIntegrationCreateVeth`, `TestIntegrationCreateBridge`, `TestIntegrationCreateVLAN`, `TestIntegrationDeleteInterface`, `TestIntegrationAddIPv4Address`, `TestIntegrationAddIPv6Address`, `TestIntegrationRemoveAddress`, `TestIntegrationSetMTU`
   - Files: `manage_integration_linux_test.go`
   - Verify: each test creates namespace, performs operation, verifies kernel state

3. **Phase: Monitor** -- real netlink event integration tests
   - Tests: `TestIntegrationMonitorLinkCreated`, `TestIntegrationMonitorAddrAdded`, `TestIntegrationMonitorAddrRemoved`, `TestIntegrationMonitorLinkDeleted`, `TestIntegrationMonitorLinkUpDown`
   - Files: `monitor_integration_linux_test.go`
   - Verify: monitor in namespace sees real kernel events on Bus

4. **Phase: Sysctl** -- real /proc/sys integration tests
   - Tests: `TestIntegrationSysctlIPv4Forwarding`, `TestIntegrationSysctlSLAAC`
   - Files: `sysctl_integration_linux_test.go`
   - Verify: sysctl values readable from /proc/sys after write

5. **Phase: Mirror** -- tc qdisc/filter integration tests
   - Tests: `TestIntegrationMirrorIngress`, `TestIntegrationMirrorEgress`, `TestIntegrationMirrorBoth`, `TestIntegrationMirrorRemove`, `TestIntegrationMirrorRemoveIdempotent`
   - Files: `mirror_integration_linux_test.go`
   - Verify: qdisc and filter lists match expectations after setup/teardown

6. **Phase: DHCP** -- real DHCP exchange integration tests
   - Tests: `TestIntegrationDHCPv4Lease`, `TestIntegrationDHCPv4Stop`, `TestIntegrationDHCPv6Lease`
   - Files: `dhcp_integration_linux_test.go`
   - Verify: address installed on interface after lease, removed after stop

7. **Phase: Migration** -- full 5-phase migration integration test
   - Tests: `TestIntegrationMigrateFullCycle`, `TestIntegrationMigrateTimeout`
   - Files: `migrate_integration_linux_test.go`
   - Verify: new interface created, IP migrated, old cleaned up; or rollback on timeout

8. **Make target + docs** -- add `ze-integration-iface-test` to Makefile, update test docs
9. **Full verification** -- run all integration tests in namespace
10. **Complete spec** -- fill audit tables, write learned summary

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1 through AC-26 each have a passing integration test |
| Correctness | Tests verify kernel state (netlink queries), not just absence of errors |
| Isolation | Every test runs in its own namespace, no cross-test interference |
| Cleanup | Namespaces deleted even on test failure (t.Cleanup) |
| Build tag | All files have `//go:build integration`, excluded from normal CI |
| No production changes | Only test files and Makefile added/modified |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| `integration_helpers_linux_test.go` exists | `ls -la internal/component/iface/integration_helpers_linux_test.go` |
| `manage_integration_linux_test.go` exists | `ls -la internal/component/iface/manage_integration_linux_test.go` |
| `monitor_integration_linux_test.go` exists | `ls -la internal/component/iface/monitor_integration_linux_test.go` |
| `sysctl_integration_linux_test.go` exists | `ls -la internal/component/iface/sysctl_integration_linux_test.go` |
| `mirror_integration_linux_test.go` exists | `ls -la internal/component/iface/mirror_integration_linux_test.go` |
| `dhcp_integration_linux_test.go` exists | `ls -la internal/component/iface/dhcp_integration_linux_test.go` |
| `migrate_integration_linux_test.go` exists | `ls -la internal/component/iface/migrate_integration_linux_test.go` |
| `make ze-integration-iface-test` target exists | `grep ze-integration-iface-test Makefile` |
| All tests pass with integration tag | `go test -tags integration -count=1 ./internal/component/iface/...` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Namespace cleanup | Namespaces deleted on test failure (t.Cleanup, not defer after fatal) |
| No host network modification | Tests never operate outside their namespace |
| DHCP server scoped | Server binds to namespace-local veth, not host interfaces |
| No persistent state | Tests leave no interfaces, addresses, or qdiscs on the host |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Namespace permission denied | Document CAP_NET_ADMIN requirement, skip with helpful message |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

N/A -- tests only, no new protocol implementation.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from plan and why]

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-26 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `go test -tags integration` passes
- [ ] `make ze-verify` still passes (no regression)
- [ ] No production code modified
- [ ] Test docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for kernel-level numeric limits (MTU, VLAN)
- [ ] Integration tests cover all kernel-touching code paths

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-iface-5-vm-tests.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
