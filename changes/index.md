# Changes

What shipped in Ze, newest first: the weekly updates, mined from git history and posted to Discord's `ze-news`. Each week lists the areas it touched; click a week for the full write-up. Ze is pre-release, so the configuration syntax can still change, and the [roadmap](../roadmap/) tracks the path to a stable release. For the landmark features on a timeline, see [Milestones](../milestones/).

## [Week of 2026-08-10](2026-08-10/index.md)

Web and Looking Glass rewrites, remote-triggered blackholing, dynamic BGP peer repairs, authenticated PPPoE and another standards pass shaped the week.

Areas: BGP, Route Server, RPKI, PPPoE, BNG, IPsec, DNS, Config, Web UI, Looking Glass, Appliance, Interfaces, RFC Compliance, Interop, Quality Improvement

## [Week of 2026-08-03](2026-08-03/index.md)

Ze is being checked against the RFCs it implements, one MUST at a time, and that is where most of this week's work came from. BGP took the largest share. IS-IS learned to answer a purge of its own LSP, gNMI stopped serving unauthenticated writes, and web and DNS listeners can now carry an operator's own certificate.

Areas: BGP, IS-IS, IPsec, DDoS, RADIUS, L2TP, Interfaces, DNS, gNMI, MCP, Looking Glass, Web UI, CLI, Config, Appliance, Storage, Diagnostics, Telemetry, Security, RFC Compliance, Interop, Under the Hood, Quality Improvement

## [Week of 2026-07-27](2026-07-27/index.md)

IPsec was the centre of the week. Ze ran against strongSwan and twenty defects came out. EAP-TLS worked with another implementation for the first time. The BGP announce path went from two encoders to one. OSPF, BMP, MRT and the MCP server each took correctness work.

Areas: IPsec, Security, Interop, BGP, Route Server, Flowspec, BMP, MRT, OSPF, MCP, CLI, Web UI, Plugins, L2TP, VPP, Storage, Diagnostics, Performance, DNS, IS-IS, RFC Compliance, Under the Hood, Quality Improvement

## [Week of 2026-07-20](2026-07-20/index.md)

Ze has left Codeberg for GitHub. Beyond the move: protocol correctness, a security triage over the code-scanning backlog, and build-time feature gates that let a deployment ship only the code it runs.

Areas: BGP, Route Server, Graceful Restart, BMP, OSPF, L2TP, IPsec, Firewall, DDoS, CoS, Security, AAA, Installer, Interfaces, Config, CLI, Feature Gates, Plugins, Storage, RFC Compliance, Under the Hood, Quality Improvement

## [Week of 2026-07-13](2026-07-13/index.md)

A busy routing week brought VRRP, BGP multipath in the FIB, BMP Loc-RIB monitoring, tighter RPKI policy, and several management-plane security fixes.

Areas: BGP, VRRP, RPKI, BMP, CLI, Looking Glass, Security, AAA, IPsec, L2TP, Firewall, Appliance, RFC Compliance, Performance, Under the Hood

## [Week of 2026-07-06](2026-07-06/index.md)

A dense week across VPN, DNS, subscriber access, the VPP data plane, the web UI, and observability.

Areas: IPsec, DNS, L2TP, PPPoE, DDoS, VPP, Web UI, RADIUS, RBAC, Looking Glass, Telemetry, MRT, MCP, Appliance, Interop, Under the Hood

## [Week of 2026-07-01](2026-07-01/index.md)

Work landed across DNS, traffic security, routing, and the CLI this week.

Areas: DNS, Anomaly Detection, BGP, OSPF, Redistribution, MPLS, CLI, Security, VPP, Under the Hood

## [Week of 2026-06-25](2026-06-25/index.md)

Shipped across security, the appliance, routing, and observability this week.

Areas: BGP, IS-IS, OSPF, DNS, L2TP, Control-Plane, DDoS, Appliance, Feature Gates, Installer, Under the Hood

## [Week of 2026-06-22](2026-06-22/index.md)

Focused on trimming attack surface and rounding out OSPF.

Areas: Web UI, BGP, IS-IS, OSPF, Telemetry, Feature Gates, Installer, Kernel

## [Week of 2026-06-15](2026-06-15/index.md)

Native IS-IS landed, MPLS gained fast reroute, and firewall rules can now pull straight from the IRR.

Areas: CLI, BGP, Flowspec, IS-IS, MPLS, RSVP-TE, Firewall, Appliance, Installer, Interfaces

## [Week of 2026-06-08](2026-06-08/index.md)

A week of operator-facing polish: the Web Workbench UI, SR-Policy and IRR-based filtering in BGP, per-subscriber CoS, and a talk at LINX.

Areas: CLI, Web UI, BGP, CoS, Installer, Under the Hood, Presentation: LINX 126

## [Week of 2026-06-01](2026-06-01/index.md)

The CLI grammar rollout finished, MRT tooling arrived, and a handful of quiet-but-important reliability bugs got fixed.

Areas: CLI, Config, BGP, RPKI, L2TP, MRT, Appliance, Installer, Quality Improvement

## [Week of 2026-05-25](2026-05-25/index.md)

MPLS grew a full label-switching stack, flow export and gNMI landed, and config commits became transactional.

Areas: CLI, Config, BGP, LDP, MPLS, RSVP-TE, gNMI, Flow Export, Appliance, Installer

## [Week of 2026-05-18](2026-05-18/index.md)

A full native IPsec/IKEv2 VPN stack, a route server for IXPs, and a big allocation-hunting pass across the BGP hot path.

Areas: CLI, BGP, Flowspec, Route Server, RPKI, L2TP, PPPoE, Diagnostics, IPsec, Appliance, Installer, Performance

## [Week of 2026-05-11](2026-05-11/index.md)

CPE features round out, interface config gets restructured, and RPKI gains ASPA path verification.

Areas: Web UI, BGP, MPLS, RPKI, CPE, DHCP, PPPoE, Telemetry, Interfaces, Under the Hood

## [Week of 2026-05-04](2026-05-04/index.md)

Two major subsystems landed this week: a fleet management tool for appliances, and a PPPoE access concentrator alongside VPP-backed NAT and ACLs.

Areas: BNG, PPP, PPPoE, RADIUS, Diagnostics, Firewall, Appliance, Fleet, Kernel, VPP

## [Week of 2026-04-27](2026-04-27/index.md)

Interface configuration now rolls back cleanly on failure, the web UI got a dedicated CLI page, and traffic-control state survives more edge cases.

Areas: CLI, Web UI, BGP, CoS, L2TP, API, Telemetry, Interfaces, Under the Hood

## [Week of 2026-04-20](2026-04-20/index.md)

Security hardening, a new diagnostics subsystem, and a redesigned operator web UI headline this week.

Areas: Web UI, BFD, BGP, MCP, Diagnostics, Telemetry, Firewall, Security, Performance, VPP

## [Week of 2026-04-13](2026-04-13/index.md)

A complete L2TP/PPP access stack, TACACS+ and pluggable AAA, a VPP dataplane, and new nftables/tc firewall backends.

Areas: CoS, L2TP, PPP, AAA, Firewall, TACACS+, Interfaces, Performance, Under the Hood, VPP

## [Week of 2026-04-06](2026-04-06/index.md)

A full BFD engine, BGP route reflection and policy filters, a REST/gRPC config editor, WireGuard support, and a talk at Net Manchester.

Areas: Config Editor, BFD, BGP, DHCP, API, Appliance, Interfaces, Presentation: Net Manchester

## [Week of 2026-03-30](2026-03-30/index.md)

A full interface management subsystem, offline DNS/RIR resolution tooling, config-driven plugin loading, BGP healthcheck and long-lived graceful restart support, and a Looking Glass overhaul.

Areas: CLI, Config, BGP, Graceful Restart, DHCP, DNS, Plugins, Looking Glass, Interfaces, Under the Hood

## [Week of 2026-03-23](2026-03-23/index.md)

A full web interface, fleet management, redistribution filtering, and a stack of routing-protocol correctness work all landed together.

Areas: Web UI, BGP, Redistribution, DNS, MCP, Plugins, Telemetry, Security, Fleet, Performance

## [Week of 2026-03-16](2026-03-16/index.md)

RPKI route origin validation landed in full, alongside a wave of BGP RFC compliance work, CLI polish, and daemon security hardening.

Areas: CLI, YANG, BGP, Graceful Restart, RPKI, Plugins, Security, Interop

## [Week of 2026-03-09](2026-03-09/index.md)

A big week for access, config safety, and BGP session security.

Areas: CLI, Config, Config Editor, Graceful Restart, Telemetry, RBAC, Security, Storage

## [Week of 2026-03-02](2026-03-02/index.md)

Best-path selection landed, along with outbound route tracking and a round of CLI/editor polish.

Areas: CLI, Config Editor, BGP, Graceful Restart, Quality Improvement, Under the Hood

## [Week of 2026-02-23](2026-02-23/index.md)

A route-server-focused week: reliability fixes for BGP Route Server under load, a new external plugin protocol option, and systematic config validation.

Areas: Config, BGP, Route Server, Plugins, Chaos, Under the Hood

## [Week of 2026-02-16](2026-02-16/index.md)

A hard round of route reflector hardening, a live web dashboard for the chaos tool, and a batch of BGP protocol and config improvements.

Areas: Config, BGP, Plugins, Chaos, Under the Hood, IETF Draft: ATTR_TOMBSTONE

## [Week of 2026-02-09](2026-02-09/index.md)

A new chaos-testing tool, matured config reload, RFC 7606 enforcement, and a hot-path allocation cleanup.

Areas: Config, Config Editor, BGP, ExaBGP Migration, Plugins, Chaos, Performance, Quality Improvement

## [Week of 2026-02-02](2026-02-02/index.md)

Mostly spent on the config editor, a new capability, and a round of decode/RIB bug fixes.

Areas: Config Editor, BGP, ExaBGP Migration, Plugins, Quality Improvement

## [Week of 2026-01-26](2026-01-26/index.md)

Config work, the ExaBGP migration path, and correctness fixes across the wire.

Areas: CLI, Config, Config Editor, BGP, Flowspec, ExaBGP Migration, Plugins, Quality Improvement, Storage, Under the Hood

## [Week of 2026-01-19](2026-01-19/index.md)

A big architecture week: Ze split into a hub process and a BGP child process, gained live config reload, and got a documented plugin SDK.

Areas: CLI, Config, Config Editor, YANG, BGP, Plugins, Quality Improvement, Under the Hood

## [Week of 2026-01-12](2026-01-12/index.md)

A week focused on Graceful Restart, ExaBGP migration tooling, and logging.

Areas: Config, BGP, Graceful Restart, API, ExaBGP Migration, Plugins, Diagnostics

## [Week of 2026-01-05](2026-01-05/index.md)

This week rounded out route-refresh, ADD-PATH and VPN/labeled-unicast support in the plugin API, alongside a batch of BGP correctness fixes.

Areas: BGP, Flowspec, Graceful Restart, MPLS, API, Plugins, Quality Improvement

## [Week of 2025-12-29](2025-12-29/index.md)

Route-family coverage rounded out (labeled-unicast, MPLS VPN, MUP, route reflection), a batch of protocol-correctness fixes, and a security-relevant change to how Ze listens for BGP sessions.

Areas: CLI, Config, Config Editor, BGP, Flowspec, MPLS, Route Server, Security, Quality Improvement

## [Week of 2025-12-22](2025-12-22/index.md)

Work on the BGP engine itself: route encoding correctness, session robustness, and a first real API surface for driving peers programmatically.

Areas: BGP, Flowspec, API, Plugins, Quality Improvement

## [Week of 2025-12-15](2025-12-15/index.md)

The first tracked week of development on Ze. In seven days the BGP engine went from nothing to a config-driven daemon that speaks the wire protocol, holds a RIB, and tests itself against ExaBGP.

Areas: CLI, Config, Config Editor, BGP, Flowspec, API, ExaBGP Migration
