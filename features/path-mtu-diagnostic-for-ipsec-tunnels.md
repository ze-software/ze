# Path MTU diagnostic for IPsec tunnels

## Meta

| Field | Value |
|-------|-------|
| Name | Path MTU diagnostic for IPsec tunnels |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/mtu, internal/core/probe |
| Real-path tests | test/plugin/show-mtu-host.ci, test/plugin/show-mtu-json.ci, test/plugin/show-mtu-exhaustive.ci, test/plugin/show-mtu-ike-probe.ci, test/plugin/show-mtu-oversized-tunnels.ci, test/plugin/show-mtu-no-ipsec-component.ci |
| Interop | ipsec/mtu-tunnel-sizing-strongswan, ipsec/mtu-negotiated-transform, ipsec/mtu-nat-installed-endpoint |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-10-07: host and exhaustive keywords read in internal/component/mtu/cmd/mtu.go; ICMP error queue path in internal/core/probe |
| Defect review | 2026-10-07: journal 2026-09-17 overhead.go ike import row is stale (no non-test import at HEAD); no immediate spec names internal/component/mtu |

## Description

`show mtu` measures the path MTU to every IPsec peer's installed endpoint and to a reference address with Don't Fragment ICMP probes (a reported MTU confirmed on the wire, then a candidate ladder and a bisection when ICMP is filtered), derives each tunnel's ESP ceiling from the transform it negotiated, and reports each bound xfrm interface as oversized, tight, under-utilized, ok, down or without a usable MTU, with the `set interface ... mtu` commands to apply and whether the access circuit or only the peer paths are clamped. A peer whose IKE SA is up is measured a second time over the SA itself, with an INFORMATIONAL request padded to the size under test: the authenticated reply confirms the ICMP figure or refutes it where ICMP errors are filtered, and the row names its prober (`ike`, `icmp`). `show mtu host <address>` measures one path; `exhaustive` bypasses the kernel's cached estimate. The command changes nothing. Linux only. <!-- source: internal/component/mtu/cmd/mtu.go -- parseMTUArgs --> <!-- source: internal/core/probe/socket.go -- OpenICMP; internal/core/probe/errqueue_linux.go -- DrainErrorQueue, classifyReportedMTU; internal/core/probe/doctor.go -- checkICMPProbeSocket --> <!-- source: internal/component/mtu/cmd/run.go -- runMTU; internal/component/mtu/cmd/search.go -- searchPathMTU; internal/component/mtu/cmd/overhead.go -- deriveESPOverhead -->
