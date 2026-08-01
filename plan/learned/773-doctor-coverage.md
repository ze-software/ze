# 773 -- Doctor Coverage Audit

## Context

Ze's `ze doctor` command validates system readiness before daemon startup, but only covered a subset of configurable features. Many features with runtime dependencies (kernel modules, listen ports, external services, procfs paths, netlink) had no doctor check, meaning failures surfaced only at daemon startup or later.

## Decisions

- Extended existing `checkKernelModules` for L2TP and PPPoE rather than creating separate check functions, since the probe mechanism (read /proc/modules) is identical. Used dedicated diagnostic codes (`doctor-l2tp-module`, `doctor-pppoe-module`) to keep the error messages specific.
- Extended `checkListeners` with service-specific codes (e.g., `doctor-bgp-listen`, `doctor-bfd-port`) instead of the generic `doctor-listen-unavailable`. Added UDP bind probes alongside the existing TCP probes.
- Used a `listenerProbe` function variable seam for testing service-specific listener codes without binding real privileged ports.
- RADIUS probe sends a real authenticated Access-Request rather than relying on UDP Dial (which always succeeds for unbound ports). This requires shared-key configuration in the config.
- PKI check validates embedded base64 DER certificates (the actual YANG model) rather than file paths.
- Linux-specific checks use `env.Get` overrides for /proc paths and modules file, enabling functional tests without real kernel state.

## Consequences

- `ze doctor` now covers all 3 tiers of runtime dependencies: kernel modules (L2TP, PPPoE, nftables), service listeners (BGP, BFD, IPsec, TFTP, image server, NTP), external service reachability (TACACS+, RADIUS), procfs access (telemetry, sysctl, conntrack), netlink availability (policy routing), and certificate validity (PKI).
- The `ai/rules/doctor-checks.md` rule now covers UDP listeners, embedded certificates, procfs/sysctl, and netlink, with a test requirement section.
- The spec template Integration Checklist now enumerates all runtime dependency categories.

## Gotchas

- IPsec module detection previously only checked a root `ipsec` block. The actual config uses `vpn/ipsec`, so the check now accepts both paths.
- RADIUS probe logs a failed authentication on the RADIUS server every time `ze doctor` runs (uses hardcoded `ze-doctor` username).
- `configEnabled(tree, defaultValue)` returns `false` when tree is nil, which means checking nil + configEnabled is redundant (the nil check inside configEnabled handles it).

## Files

- `internal/component/doctor/doctor.go` -- checkPKICerts, checkDHCPInterfaces, checkTACACSServers, checkRADIUSServers, extractBGPListeners, extractBFDListeners, extractIPsecListeners, extractTFTPListeners, extractImageListeners, extractNTPListeners, dedupeListeners, probeListener
- `internal/component/doctor/checks_linux.go` -- checkFirewallBackend, checkTelemetryProcfs, checkSysctlProcfs, checkConntrackProcfs, checkPolicyRouteNetlink
- `internal/component/doctor/checks_other.go` -- stubs for new Linux-only checks
- `internal/component/doctor/doctor_test.go` -- cross-platform unit tests
- `internal/component/doctor/checks_linux_test.go` -- Linux-only unit tests
- `internal/core/diagnostic/codes.go` -- 17 new doctor-* codes
- `test/ui/doctor-*.ci` -- 10 functional test files
