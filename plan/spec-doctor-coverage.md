# Spec: doctor-coverage

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | 1/1 |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `ai/rules/doctor-checks.md` - doctor check rules
4. `cmd/ze/doctor/doctor.go` - existing checks
5. `cmd/ze/doctor/checks_linux.go` - Linux-specific checks

## Task

Audit every configurable feature in Ze for missing `ze doctor` checks. Each feature
with a runtime dependency (kernel module, listen port, external binary, file path,
external service) should have a corresponding doctor check that validates readiness
before the daemon starts.

Also ensure the `ai/rules/doctor-checks.md` rule and the spec template's Integration
Checklist are sufficient to prevent new gaps from forming.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/doctor-checks.md` - doctor check requirements for new runtime dependencies
  → Constraint: every feature with a runtime dep must have a doctor check
- [ ] `docs/features/ai-first.md` - doctor is part of agent-readiness tooling

**Key insights:**
- Doctor checks are config-driven: only check what is configured
- Linux-specific checks go in `checks_linux.go` with stubs in `checks_other.go`
- All codes must be registered in `codes.go` for `ze explain` support

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/doctor/doctor.go` - orchestrates all checks via `runChecks`, calls 15 check functions
- [ ] `cmd/ze/doctor/checks_linux.go` - VPP socket, kernel modules, interfaces, VPP version, kernel nexthop, MPLS
- [ ] `cmd/ze/doctor/checks_other.go` - no-op stubs for non-Linux
- [ ] `internal/core/diagnostic/codes.go` - 22 doctor-* codes registered

**Existing doctor checks:**

| Check function | What it validates | Config trigger |
|----------------|-------------------|----------------|
| `checkStoreIntegrity` | zefs database integrity | always |
| `checkIfaceBackend` | VPP API socket reachable | `interface/backend = vpp` |
| `checkInterfaces` | ethernet interfaces exist and are up | `interface/ethernet/*` |
| `checkKernelModules` | vhost_net (VPP), xfrm_user/xfrm_algo (IPsec) | `interface/backend`, `ipsec` block |
| `checkKernelNexthop` | /proc/net/nexthop exists | always (Linux) |
| `checkMPLSSupport` | mpls_router, mpls_iptunnel modules | `bgp` + `fib/kernel` blocks |
| `checkTLS` | cert/key files exist, cert not expired | `environment/mcp/tls`, `environment/api-server/grpc` |
| `checkWebTLS` | web cert/key in zefs storage | `environment/web` |
| `checkPlugins` | external plugin binaries on PATH | `plugin/*` |
| `checkSSHHostKey` | SSH host key file exists | `environment/ssh/enabled = true` |
| `checkListeners` | TCP ports bindable | web, mcp, looking-glass, api-rest, api-grpc, ssh |
| `checkDiskSpace` | config partition >= 5% free | always |
| `checkDNSResolvers` | configured name servers respond | `system/name-server` |
| `checkConfigReferences` | BGP filter refs resolve to defined policies | `bgp/policy`, `bgp/filter` |
| `checkClockSkew` | system clock within 5min of NTP | always |
| `checkVPPVersion` | vppctl shows compatible version | `interface/backend = vpp` |

## Gap Analysis

Features with runtime dependencies that have NO doctor check:

### Tier 1 -- High priority (runtime failure if missing, hard to diagnose)

| Feature | Config block | Runtime dependency | Suggested check | Suggested code |
|---------|-------------|--------------------|-----------------|--------------------|
| L2TP | `l2tp` | `l2tp_ppp` or `pppol2tp` kernel module (modprobe at start) | Check modules loaded when L2TP block present | `doctor-l2tp-module` |
| PPPoE | `pppoe` | `pppoe` kernel module (modprobe at start) | Check module loaded when PPPoE block present | `doctor-pppoe-module` |
| Firewall (nftables) | `firewall` (YANG `ze-firewall-conf`) | `nf_tables` kernel module, netlink socket | Check nftables kernel support when firewall configured | `doctor-firewall-nftables` |
| DHCP server | `service/dhcp-server` | Raw UDP port 67, interface binding | Check listen interfaces exist | `doctor-dhcp-iface` |
| BGP listener | `bgp` | TCP port 179 (privileged) | Check port 179 bindable | `doctor-bgp-listen` |

### Tier 2 -- Medium priority (degrades functionality)

| Feature | Config block | Runtime dependency | Suggested check | Suggested code |
|---------|-------------|--------------------|-----------------|--------------------|
| TACACS+ | `system/authentication/tacacs` | TCP to configured server(s) | Check at least one TACACS server reachable | `doctor-tacacs-unreachable` |
| RADIUS (L2TP auth) | `l2tp/auth/radius` | UDP to configured server(s) | Check at least one RADIUS server reachable | `doctor-radius-unreachable` |
| BFD | `bfd` | UDP port 3784 (requires CAP_NET_RAW or root) | Check BFD port bindable or capabilities present | `doctor-bfd-port` |
| PKI | `pki` | CA cert/key files | Check CA cert existence and expiry | `doctor-pki-cert` |
| IPsec/IKE listeners | `vpn/ipsec` | UDP ports 500, 4500 | Check IKE ports bindable | `doctor-ipsec-listen` |
| Telemetry (Linux) | `telemetry` | `/proc` access for procfs collectors | Check /proc readable when telemetry enabled | `doctor-telemetry-procfs` |

### Tier 3 -- Low priority (less likely to fail silently)

| Feature | Config block | Runtime dependency | Suggested check | Suggested code |
|---------|-------------|--------------------|-----------------|--------------------|
| TFTP server | `service/tftp-server` | UDP port 69 | Check port bindable | `doctor-tftp-listen` |
| Image server | `service/image-server` | Listen port | Check port bindable | `doctor-image-listen` |
| NTP | `environment/ntp` | UDP port 123 (if serving) | Check port bindable | `doctor-ntp-listen` |
| Sysctl | `sysctl` | `/proc/sys` writable (Linux) | Check /proc/sys writability | `doctor-sysctl-procfs` |
| Conntrack tuning | `system/conntrack` | `/proc/sys/net/netfilter/*` (Linux) | Check conntrack sysctl procfs writable | `doctor-conntrack-procfs` |
| Policy routing | `policy/rule` | netlink ip-rule support | Check netlink available | `doctor-policyroute-netlink` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | L2TP block present, `l2tp_ppp` module not loaded | `doctor-l2tp-module` error |
| AC-2 | PPPoE block present, `pppoe` module not loaded | `doctor-pppoe-module` error |
| AC-3 | Firewall block present, `nf_tables` module not loaded | `doctor-firewall-nftables` warning |
| AC-4 | DHCP server configured, listen interface missing | `doctor-dhcp-iface` error |
| AC-5 | BGP configured with local-address, port 179 not bindable | `doctor-bgp-listen` warning |
| AC-6 | TACACS server configured, server unreachable | `doctor-tacacs-unreachable` warning |
| AC-7 | RADIUS server configured, server unreachable | `doctor-radius-unreachable` warning |
| AC-8 | BFD configured, port 3784 not bindable | `doctor-bfd-port` warning |
| AC-9 | PKI configured, CA cert file missing | `doctor-pki-cert` error |
| AC-10 | IPsec configured, ports 500/4500 not bindable | `doctor-ipsec-listen` warning |
| AC-11 | Telemetry configured on Linux, /proc unreadable | `doctor-telemetry-procfs` warning |
| AC-12 | TFTP server configured, port 69 not bindable | `doctor-tftp-listen` warning |
| AC-13 | Image server configured, listen port not bindable | `doctor-image-listen` warning |
| AC-14 | NTP configured, port 123 not bindable | `doctor-ntp-listen` warning |
| AC-15 | Sysctl configured on Linux, /proc/sys not writable | `doctor-sysctl-procfs` warning |
| AC-16 | Conntrack tuning configured, `/proc/sys/net/netfilter/nf_conntrack_max` unavailable or not writable | `doctor-conntrack-procfs` warning |
| AC-17 | Every new diagnostic code registered in `codes.go` | `ze explain <code>` works |

## Files to Modify

- `cmd/ze/doctor/doctor.go` - add new check functions and wire into `runChecks`
- `cmd/ze/doctor/checks_linux.go` - Linux-specific checks (kernel modules, procfs, netlink)
- `cmd/ze/doctor/checks_other.go` - no-op stubs for new Linux-only checks
- `internal/core/diagnostic/codes.go` - register new diagnostic codes

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | - |
| CLI commands/flags | No | - |
| CLI grammar (action before identifier) | No | - |
| Editor autocomplete | No | - |
| Functional test for new RPC/API | No | - |
| Doctor check for runtime dependencies | Yes | this spec IS the doctor check work |

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config tree parsed from ze.conf (or zefs storage)
- `runChecks(configPath)` in `cmd/ze/doctor/doctor.go`

### Transformation Path
1. Config loaded and parsed into `*config.Tree`
2. Each `check*` function reads relevant config subtree via `tree.GetContainer`
3. Check probes runtime dependency (file stat, port bind, module list, network dial)
4. Returns `[]diagnostic.Diagnostic` with code, severity, message

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> Doctor | `tree.GetContainer("l2tp")` etc. | [ ] |
| Doctor -> OS | `os.Stat`, `net.Listen`, `/proc/modules` | [ ] |

### Integration Points
- `runChecks` in `doctor.go` calls each check function and aggregates diagnostics
- `diagnostic.Register` in `codes.go` makes codes explainable via `ze explain`

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze doctor --json <config-with-l2tp>` | -> | `checkKernelModules` | `TestCheckKernelModules_L2TP` |
| `ze doctor --json <config-with-pppoe>` | -> | `checkKernelModules` | `TestCheckKernelModules_PPPoE` |
| `ze doctor --json <config-with-firewall>` | -> | `checkFirewallBackend` | `TestCheckFirewallBackend` |
| `ze doctor --json <config-with-dhcp>` | -> | `checkDHCPInterfaces` | `TestCheckDHCPInterfaces` |
| `ze doctor --json <config-with-bgp>` | -> | `checkListeners` (BGP) | `TestCheckListeners_BGP` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckKernelModules_L2TP` | `cmd/ze/doctor/doctor_test.go` | L2TP module check when l2tp block present | |
| `TestCheckKernelModules_PPPoE` | `cmd/ze/doctor/doctor_test.go` | PPPoE module check when pppoe block present | |
| `TestCheckFirewallBackend` | `cmd/ze/doctor/doctor_test.go` | nftables module check when firewall configured | |
| `TestCheckDHCPInterfaces` | `cmd/ze/doctor/doctor_test.go` | DHCP listen interfaces exist | |
| `TestCheckListeners_BGP` | `cmd/ze/doctor/doctor_test.go` | BGP port 179 bindable | |
| `TestCheckTACACSServers` | `cmd/ze/doctor/doctor_test.go` | TACACS server reachability | |
| `TestCheckRADIUSServers` | `cmd/ze/doctor/doctor_test.go` | RADIUS server reachability | |
| `TestCheckPKICerts` | `cmd/ze/doctor/doctor_test.go` | PKI cert/key existence and expiry | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A -- no numeric inputs | - | - | - | - |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-doctor-l2tp` | `test/doctor/*.ci` | Doctor reports missing L2TP module | |
| `test-doctor-firewall` | `test/doctor/*.ci` | Doctor reports missing nftables | |

### Future (if deferring any tests)
- Tier 3 checks (sysctl, conntrack, policy-route) deferred until tier 1+2 complete

## Implementation Steps

Implementation should follow tiers. Each tier is independently shippable.

### Tier 1 (high priority)

1. **L2TP kernel modules** -- extend `checkKernelModules` to check `l2tp_ppp`/`pppol2tp` when `l2tp` block present
2. **PPPoE kernel module** -- extend `checkKernelModules` to check `pppoe` when `pppoe` block present
3. **Firewall nftables** -- new `checkFirewallBackend` for `nf_tables` module check
4. **DHCP server interfaces** -- new `checkDHCPInterfaces` to verify listen interfaces exist
5. **BGP listener** -- extend `checkListeners` to include BGP port 179

### Tier 2 (medium priority)

6. **TACACS+ reachability** -- new `checkTACACSServers` with TCP probe
7. **RADIUS reachability** -- new `checkRADIUSServers` with UDP probe
8. **BFD port** -- extend `checkListeners` for UDP 3784
9. **PKI certs** -- new `checkPKICerts` for CA cert/key existence and expiry
10. **IPsec/IKE ports** -- extend `checkListeners` for UDP 500/4500
11. **Telemetry procfs** -- new `checkTelemetryProcfs` for /proc access

### Tier 3 (low priority)

12. **TFTP/Image/NTP server ports** -- extend `checkListeners` for service ports
13. **Sysctl procfs** -- new check for /proc/sys writability
14. **Conntrack procfs** -- new check for /proc/sys/net/netfilter conntrack sysctl access
15. **Policy routing** -- new check for netlink ip-rule support

### Rule reinforcement

16. Verify `ai/rules/doctor-checks.md` covers all dependency types found in this audit
17. Verify spec template Integration Checklist row for doctor checks is clear enough

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has a check function wired into `runChecks` |
| Correctness | Each check only fires when the relevant config block is present |
| Naming | All diagnostic codes follow `doctor-<component>-<condition>` pattern |
| Data flow | Checks read config tree, probe OS, return diagnostics |
| Doctor checks | This spec IS the doctor check work |
| Rule: no-layering | Extend existing check functions where possible (e.g. `checkKernelModules`) |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| New check functions in doctor.go | `grep 'func check' cmd/ze/doctor/doctor.go` |
| Linux checks in checks_linux.go | `grep 'func check' cmd/ze/doctor/checks_linux.go` |
| Stubs in checks_other.go | `grep 'func check' cmd/ze/doctor/checks_other.go` |
| Diagnostic codes registered | `grep 'doctor-' internal/core/diagnostic/codes.go \| wc -l` |
| All checks wired into runChecks | `grep 'diags = append(diags, check' cmd/ze/doctor/doctor.go` |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Config values used in file paths or network addresses are sanitized |
| Resource exhaustion | Network probes have timeouts (3-5s) to avoid hanging |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Lint failure | Fix inline |
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

## Implementation Summary

### What Was Implemented
- Added config-triggered doctor checks for L2TP, PPPoE, firewall nftables, DHCP interfaces, BGP listeners, TACACS+, RADIUS, BFD, PKI certificates, IPsec/IKE listeners, telemetry procfs, TFTP, image server, NTP, sysctl procfs, conntrack procfs, and policy-route netlink.
- Registered every new `doctor-*` diagnostic code in `internal/core/diagnostic/codes.go`.
- Added unit tests for cross-platform checks, Linux-only checks, code registration, and listener coverage.
- Added UI functional tests for `ze doctor --json` DHCP, BGP, PKI, L2TP, PPPoE, firewall, AAA reachability, listener, procfs, and netlink paths.
- Added the doctor package to the QEMU integration target so Linux-only doctor checks run there.

### Bugs Found/Fixed
- Existing IPsec kernel module detection checked only a root `ipsec` block; current config uses `vpn/ipsec`, so the module check now accepts both paths.
- `TestDoctorValidConfigText` assumed every platform has zero environmental warnings; Linux can legitimately warn about missing kernel nexthop support, so the test now accepts ready output with zero errors.
- The original PKI gap described CA certificate/key files, but the current PKI schema stores base64 DER certificate material in config. The implemented check validates the current schema exactly instead of inventing file-path behavior.
- The first RADIUS probe treated UDP `Dial` success as reachability. It now sends a shared-key authenticated RADIUS Access-Request and requires a valid response authenticator.
- The first conntrack probe checked `/proc/net/nf_conntrack`. It now checks the procfs sysctl path Ze writes, `/proc/sys/net/netfilter/nf_conntrack_max`.

### Documentation Updates
- `ai/rules/doctor-checks.md` now covers UDP listeners, embedded certificates, procfs/sysctl, netlink, and required unit plus functional tests.
- `plan/TEMPLATE.md` Integration Checklist now names the runtime dependency categories and required doctor files/tests.

### Deviations from Plan
- PKI checks validate embedded base64 DER certificates because that is the actual current YANG model; no file-path PKI leaves exist to stat.
- Policy-route netlink was implemented even though the AC table omitted a numbered AC, because it is listed in the gap analysis and implementation steps.
- QEMU target was run and the doctor package passed, but the full target failed later in unrelated iface and firewall packages.

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Audit runtime dependencies and add missing doctor checks | Done | `cmd/ze/doctor/doctor.go`, `cmd/ze/doctor/checks_linux.go` | Checks are config-triggered. |
| Reinforce doctor-check rule | Done | `ai/rules/doctor-checks.md` | Added missing dependency classes and test requirement. |
| Reinforce spec template checklist | Done | `plan/TEMPLATE.md` | Doctor row now names files and test expectations. |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestCheckKernelModules_L2TP`, `doctor-l2tp-module.ci` | Linux functional test uses private modules-file override. |
| AC-2 | Done | `TestCheckKernelModules_PPPoE`, `doctor-pppoe-module.ci` | Linux functional test uses private modules-file override. |
| AC-3 | Done | `TestCheckFirewallBackend`, `doctor-firewall-nftables.ci` | Linux functional test uses private modules-file override. |
| AC-4 | Done | `TestCheckDHCPInterfaces`, `doctor-dhcp-iface.ci` | Missing interface is deterministic. |
| AC-5 | Done | `TestCheckListeners_BGP`, `doctor-bgp-listen.ci` | Functional test binds an unroutable local address. |
| AC-6 | Done | `TestCheckTACACSServers`, `doctor-aaa-unreachable.ci` | Functional test probes an unused TCP port. |
| AC-7 | Done | `TestCheckRADIUSServers`, `TestUDPServerReachableRequiresResponse`, `doctor-aaa-unreachable.ci` | RADIUS probe requires an authenticated response. |
| AC-8 | Done | `TestCheckListeners_ServicePorts`, `doctor-listeners.ci` | BFD listener path verified by unit and functional test seam. |
| AC-9 | Done | `TestCheckPKICerts_MissingCA`, `TestCheckPKICerts_ExpiredCA`, `doctor-pki-cert.ci` | Validates actual embedded DER schema. |
| AC-10 | Done | `TestCheckListeners_ServicePorts`, `doctor-listeners.ci` | IKE UDP 500/4500 listener paths. |
| AC-11 | Done | `TestCheckTelemetryProcfs`, `doctor-procfs-netlink.ci` | Linux functional test uses private procfs-root override. |
| AC-12 | Done | `TestCheckListeners_ServicePorts`, `doctor-listeners.ci` | TFTP UDP listener path. |
| AC-13 | Done | `TestCheckListeners_ServicePorts`, `doctor-listeners.ci` | Image server TCP listener path. |
| AC-14 | Done | `TestCheckListeners_ServicePorts`, `doctor-listeners.ci` | NTP UDP listener path. |
| AC-15 | Done | `TestCheckSysctlProcfs`, `doctor-procfs-netlink.ci` | Linux functional test uses private procfs-root override. |
| AC-16 | Done | `TestCheckConntrackProcfs`, `doctor-procfs-netlink.ci` | Checks conntrack sysctl procfs, not `/proc/net/nf_conntrack`. |
| AC-17 | Done | `TestDoctorCoverageCodesRegistered` | All new codes registered for `ze explain`. |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestCheckKernelModules_L2TP` | Done | `cmd/ze/doctor/checks_linux_test.go` | Linux-only. |
| `TestCheckKernelModules_PPPoE` | Done | `cmd/ze/doctor/checks_linux_test.go` | Linux-only. |
| `TestCheckFirewallBackend` | Done | `cmd/ze/doctor/checks_linux_test.go` | Linux-only. |
| `TestCheckDHCPInterfaces` | Done | `cmd/ze/doctor/doctor_test.go` | Cross-platform. |
| `TestCheckListeners_BGP` | Done | `cmd/ze/doctor/doctor_test.go` | Cross-platform. |
| `TestCheckTACACSServers` | Done | `cmd/ze/doctor/doctor_test.go` | Cross-platform with probe seam. |
| `TestCheckRADIUSServers` | Done | `cmd/ze/doctor/doctor_test.go` | Cross-platform with probe seam. |
| `TestCheckPKICerts` | Done | `cmd/ze/doctor/doctor_test.go` | Split into missing and expired cases. |
| `doctor-l2tp-module.ci` | Done | `test/ui/doctor-l2tp-module.ci` | Skips non-Linux, verified in Docker. |
| `doctor-pppoe-module.ci` | Done | `test/ui/doctor-pppoe-module.ci` | Skips non-Linux, verified in Docker. |
| `doctor-firewall-nftables.ci` | Done | `test/ui/doctor-firewall-nftables.ci` | Skips non-Linux, verified in Docker. |
| `doctor-aaa-unreachable.ci` | Done | `test/ui/doctor-aaa-unreachable.ci` | Covers TACACS+ TCP and RADIUS UDP reachability. |
| `doctor-listeners.ci` | Done | `test/ui/doctor-listeners.ci` | Covers BFD, IPsec, TFTP, image server, and NTP listener diagnostics. |
| `doctor-procfs-netlink.ci` | Done | `test/ui/doctor-procfs-netlink.ci` | Covers telemetry procfs, sysctl procfs, conntrack procfs, and policy-route netlink. |
| Additional doctor UI tests | Done | `test/ui/doctor-dhcp-iface.ci`, `doctor-bgp-listen.ci`, `doctor-pki-cert.ci`, `doctor-config-reference.ci` | Covers deterministic cross-platform user paths. |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `cmd/ze/doctor/doctor.go` | Changed | Added cross-platform checks and listener extraction. |
| `cmd/ze/doctor/checks_linux.go` | Changed | Added module, nftables, procfs, sysctl, conntrack, and netlink probes. |
| `cmd/ze/doctor/checks_other.go` | Changed | Added no-op stubs. |
| `internal/core/diagnostic/codes.go` | Changed | Added all new codes. |
| `cmd/ze/doctor/doctor_test.go` | Changed | Added unit coverage and code registration checks. |
| `cmd/ze/doctor/checks_linux_test.go` | Created | Linux-only unit coverage. |
| `test/ui/doctor-*.ci` | Created | Functional coverage. |
| `ai/rules/doctor-checks.md` | Changed | Rule reinforcement. |
| `plan/TEMPLATE.md` | Changed | Checklist reinforcement. |
| `mk/test-integration.mk` | Changed | Added doctor package to QEMU integration target. |

### Audit Summary
- **Total items:** 17 ACs plus policy-route netlink from implementation steps
- **Done:** 18 implementation checks, diagnostic registration, unit tests, functional tests, rule/template updates
- **Partial:** None in implementation scope. Full QEMU target failed after doctor package passed, in unrelated existing packages
- **Skipped:** None in implementation scope
- **Changed:** PKI check adapted to current embedded-certificate schema

## Goal Validation (BLOCKING)

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| All features with runtime deps have doctor checks | unit and functional tests | Test names and `.ci` files per AC above |
| No new gaps can form | rule audit | `ai/rules/doctor-checks.md` covers all dep types |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | High | RADIUS UDP probe used unauthenticated Status-Server and could warn on healthy servers | `cmd/ze/doctor/doctor.go` | Switched to shared-key authenticated RADIUS Access-Request and valid response verification. |
| 2 | High | Conntrack probe required directory `W_OK` before checking sysctl key | `cmd/ze/doctor/checks_linux.go` | Changed directory probe to existence check and key probe to writable access. |
| 3 | Medium | Linux-only `.ci` tests skipped only Darwin | `test/ui/doctor-*.ci` | Expanded skip list to all current non-Linux GOOS values. |

### Fixes applied
- RADIUS now uses configured `shared-key`, optional `source-address`, and `nas-identifier` in an authenticated Access-Request probe.
- Conntrack now probes `/proc/sys/net/netfilter/nf_conntrack_max`, matching the sysctl path Ze writes.
- Linux-only UI tests skip all current non-Linux GOOS values.

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 2.1 | none | Re-review reported no blocker/issue findings | doctor changes | Residual risks recorded below. |

### Final status
- [x] Focused review re-run shows 0 blocker/issue findings
- [x] Residual risks: RADIUS probe depends on server policy returning a valid response to authenticated Access-Request; Linux-only skip list is explicit and may need updating for future GOOS values.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-17 | Doctor checks and diagnostic codes covered | `go test ./cmd/ze/doctor ./internal/core/diagnostic` passed |
| Linux ACs | Linux-specific doctor unit coverage | `make ze-linux-test ZE_LINUX_TEST_PACKAGES=./cmd/ze/doctor` passed |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze doctor --json` | `doctor-dhcp-iface`, `doctor-bgp-listen`, `doctor-pki-cert`, `doctor-aaa-unreachable`, `doctor-listeners`, `doctor-config-reference` | macOS `go run ./cmd/ze-test ui ...` passed, Linux-only tests skipped |
| `ze doctor --json` | `doctor-l2tp-module`, `doctor-firewall-nftables`, `doctor-pppoe-module`, `doctor-procfs-netlink`, `doctor-aaa-unreachable`, `doctor-listeners` | Linux Docker `go run ./cmd/ze-test ui ...` passed |
| QEMU integration | `./cmd/ze/doctor` package in QEMU target | Doctor package passed; aggregate target failed later in unrelated `internal/component/iface` and `internal/plugins/firewall/nft` tests |

## Checklist

### Goal Gates (MUST pass)
- [x] AC-1..AC-17 all demonstrated
- [x] Wiring Test table complete
- [x] Focused review gate clean
- [ ] `make ze-test` passes
- [x] Feature code integrated
- [x] Integration completeness proven end-to-end
- [x] Architecture docs updated
- [x] Critical Review passes

### Quality Gates (SHOULD pass)
- [x] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [x] Tests written
- [ ] Tests FAIL (paste output)
- [x] Tests PASS (paste output)
- [x] Boundary tests for all numeric inputs
- [x] Functional tests for end-to-end behavior
- [x] Goal Validation table filled with concrete evidence

### Completion (BLOCKING)
- [x] Critical Review passes
- [x] Partial/Skipped items have user approval (none in implementation scope; unrelated QEMU failures recorded)
- [x] Implementation Summary filled
- [x] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-doctor-coverage.md`
- [ ] **Summary included in commit**
