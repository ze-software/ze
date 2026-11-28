# Path MTU diagnostic for IPsec tunnels

## Meta

| Field | Value |
|-------|-------|
| Name | Path MTU diagnostic for IPsec tunnels |
| Kind | daemon |
| Scope | complete |
| Level | supported |
| Components | internal/component/mtu, internal/core/probe |
| Real-path tests | test/plugin/show-mtu-host.ci, test/plugin/show-mtu-json.ci, test/plugin/show-mtu-exhaustive.ci, test/plugin/show-mtu-ike-probe.ci, test/plugin/show-mtu-oversized-tunnels.ci, test/plugin/show-mtu-no-ipsec-component.ci, test/plugin/show-mtu-non-ipsec-tunnels.ci |
| Interop | ipsec/mtu-tunnel-sizing-strongswan, ipsec/mtu-negotiated-transform, ipsec/mtu-nat-installed-endpoint |
| Docs | docs/guide/ipsec.md |
| Doc review | 2026-11-09: re-read after the docs/guide/ipsec.md edits since 2026-10-26; none touches show mtu, every source anchor resolves (OpenICMP in socket.go, DrainErrorQueue in errqueue_linux.go, checkICMPProbeSocket in doctor.go), and the description still matches. Earlier, 2026-10-26: re-read after the IKE virtual-IP commits changed docs/guide/ipsec.md; host and exhaustive are still the keywords of parseMTUArgs (argHost, argExhaustive in internal/component/mtu/cmd/mtu.go); the description still matches |
| Defect review | 2026-10-07: journal 2026-09-17 overhead.go ike import row is stale (no non-test import at HEAD); no immediate spec names internal/component/mtu |

## Description

`show mtu` measures the path MTU to every IPsec peer and every GRE, GRETap, IP6GRE, IP6GRETap, VXLAN, WireGuard, IPIP, SIT, IP6Tnl, and IPIP6 tunnel endpoint plus a reference address. It uses Don't Fragment ICMP probes and padded IKE exchanges for live IPsec SAs. It derives exact encapsulation overheads from live dataplane state and reports each `xfrm`, `tunnel`, or `wireguard` interface as oversized, tight, under-utilized, ok, down, or without a usable MTU. It also lists the `set interface ... mtu` commands to apply and reports underlay clamping advice. `show mtu host <address>` measures one path. `exhaustive` bypasses the kernel's cached estimate. The command changes nothing and runs on Linux. <!-- source: internal/component/mtu/cmd/mtu.go -- parseMTUArgs --> <!-- source: internal/core/probe/socket.go -- OpenICMP; internal/core/probe/errqueue_linux.go -- DrainErrorQueue, classifyReportedMTU; internal/core/probe/doctor.go -- checkICMPProbeSocket --> <!-- source: internal/component/mtu/cmd/run.go -- runMTU, listTunnelSnapshot; internal/component/mtu/cmd/search.go -- searchPathMTU; internal/component/mtu/cmd/overhead.go -- deriveESPOverhead, deriveTunnelOverhead, deriveWireguardOverhead -->
