# Changes

What shipped in Ze, newest first: the weekly updates, mined from git history and posted to Discord's `ze-news`. Each week lists the areas it touched; click a week for the full write-up. Ze is pre-release, so the configuration syntax can still change, and the [roadmap](../roadmap/) tracks the path to a stable release. For the landmark features on a timeline, see [Milestones](../milestones/).

## [Week of 2026-06-25](2026-06-25/index.md)

Shipped across security, the appliance, routing, and observability this week.

Areas: Control-Plane, DDoS, Feature Gates, Appliance, Installer, BGP, L2TP, IS-IS, OSPF, DNS, Under the Hood

## [Week of 2026-06-22](2026-06-22/index.md)

Focused on trimming attack surface and rounding out OSPF.

Areas: Feature Gates, OSPF, IS-IS, BGP, Web UI, Telemetry, Kernel, Installer

## [Week of 2026-06-15](2026-06-15/index.md)

Native IS-IS landed, MPLS gained fast reroute, and firewall rules can now pull straight from the IRR.

Areas: IS-IS, MPLS, RSVP-TE, BGP, Flowspec, Firewall, Interfaces, CLI, Appliance, Installer

## [Week of 2026-06-08](2026-06-08/index.md)

A week of operator-facing polish: a real Web Workbench UI, SR-Policy and IRR-based filtering in BGP, per-subscriber CoS, and a talk at LINX.

Areas: BGP, Web UI, CLI, CoS, Installer, Under the Hood, Presentation: LINX 126

## [Week of 2026-06-01](2026-06-01/index.md)

The CLI grammar rollout finished, MRT tooling arrived, and a handful of quiet-but-important reliability bugs got fixed.

Areas: CLI, MRT, Appliance, Installer, BGP, RPKI, L2TP, Config, Quality Improvement

## [Week of 2026-05-25](2026-05-25/index.md)

MPLS grew a full label-switching stack, flow export and gNMI landed, and config commits became transactional.

Areas: MPLS, LDP, RSVP-TE, Flow Export, gNMI, CLI, Config, BGP, Appliance, Installer

## [Week of 2026-05-18](2026-05-18/index.md)

A full native IPsec/IKEv2 VPN stack, a route server for IXPs, and a big allocation-hunting pass across the BGP hot path.

Areas: IPsec, BGP, Route Server, Flowspec, RPKI, PPPoE, L2TP, CLI, Diagnostics, Appliance, Installer, Performance

## [Week of 2026-05-11](2026-05-11/index.md)

CPE features round out, interface config gets restructured, and RPKI gains ASPA path verification.

Areas: DHCP, PPPoE, CPE, Interfaces, BGP, MPLS, RPKI, Telemetry, Web UI, Under the Hood

## [Week of 2026-05-04](2026-05-04/index.md)

Two major subsystems landed this week: a fleet management tool for appliances, and a PPPoE access concentrator alongside VPP-backed NAT and ACLs.

Areas: Fleet, Appliance, PPPoE, BNG, RADIUS, PPP, Kernel, Firewall, VPP, Diagnostics

## [Week of 2026-04-27](2026-04-27/index.md)

Interface configuration now rolls back cleanly on failure, the web UI got a dedicated CLI page, and traffic-control state survives more edge cases.

Areas: Interfaces, BGP, Web UI, CLI, CoS, Telemetry, API, L2TP, Under the Hood

## [Week of 2026-04-20](2026-04-20/index.md)

Security hardening, a new diagnostics subsystem, and a redesigned operator web UI headline this week.

Areas: Security, Diagnostics, Telemetry, Web UI, BGP, BFD, VPP, Performance, MCP, Firewall

## [Week of 2026-04-13](2026-04-13/index.md)

A complete L2TP/PPP access stack, TACACS+ and pluggable AAA, a VPP dataplane, and new nftables/tc firewall backends.

Areas: L2TP, PPP, TACACS+, AAA, Firewall, CoS, VPP, Interfaces, Performance, Under the Hood

## [Week of 2026-04-06](2026-04-06/index.md)

A full BFD engine, BGP route reflection and policy filters, a real REST/gRPC config editor, WireGuard support, and a talk at Net Manchester.

Areas: BFD, BGP, API, Config Editor, Interfaces, DHCP, Appliance, Presentation: Net Manchester

## [Week of 2026-03-30](2026-03-30/index.md)

A full interface management subsystem, offline DNS/RIR resolution tooling, config-driven plugin loading, BGP healthcheck and long-lived graceful restart support, and a Looking Glass overhaul.

Areas: Interfaces, DHCP, DNS, Plugins, Config, CLI, Looking Glass, BGP, Graceful Restart, Under the Hood

## [Week of 2026-03-23](2026-03-23/index.md)

A full web interface, fleet management, redistribution filtering, and a stack of routing-protocol correctness work all landed together.

Areas: Web UI, Fleet, Redistribution, Plugins, Security, DNS, MCP, BGP, Telemetry, Performance

## [Week of 2026-03-16](2026-03-16/index.md)

RPKI route origin validation landed in full, alongside a wave of BGP RFC compliance work, CLI polish, and daemon security hardening.

Areas: RPKI, BGP, Graceful Restart, CLI, YANG, Security, Interop, Plugins

## [Week of 2026-03-09](2026-03-09/index.md)

A big week for access, config safety, and BGP session security.

Areas: Security, RBAC, Config, Config Editor, Storage, CLI, Telemetry, Graceful Restart

## [Week of 2026-03-02](2026-03-02/index.md)

Real best-path selection landed, along with outbound route tracking and a round of CLI/editor polish.

Areas: BGP, Graceful Restart, CLI, Config Editor, Under the Hood, Quality Improvement

## [Week of 2026-02-23](2026-02-23/index.md)

A route-server-focused week: reliability fixes for BGP Route Server under load, a new external plugin protocol option, and systematic config validation.

Areas: Route Server, BGP, Plugins, Config, Chaos, Under the Hood

## [Week of 2026-02-16](2026-02-16/index.md)

A hard round of route reflector hardening, a live web dashboard for the chaos tool, and a batch of BGP protocol and config improvements.

Areas: BGP, Chaos, IETF Draft: ATTR_TOMBSTONE, Config, Plugins, Under the Hood

## [Week of 2026-02-09](2026-02-09/index.md)

A new chaos-testing tool, matured config reload, RFC 7606 enforcement, and a hot-path allocation cleanup.

Areas: Chaos, BGP, Config, Plugins, Config Editor, ExaBGP Migration, Performance, Quality Improvement

## [Week of 2026-02-02](2026-02-02/index.md)

Mostly spent on the config editor, a new capability, and a round of decode/RIB bug fixes.

Areas: BGP, Plugins, Config Editor, ExaBGP Migration, Quality Improvement

## [Week of 2026-01-26](2026-01-26/index.md)

Config work, the ExaBGP migration path, and correctness fixes across the wire.

Areas: Plugins, BGP, Flowspec, ExaBGP Migration, Config, Config Editor, CLI, Quality Improvement, Under the Hood, Storage

## [Week of 2026-01-19](2026-01-19/index.md)

A big architecture week: Ze split into a hub process and a BGP child process, gained live config reload, and got a documented plugin SDK.

Areas: Under the Hood, Config, Plugins, YANG, Config Editor, CLI, BGP, Quality Improvement

## [Week of 2026-01-12](2026-01-12/index.md)

A week focused on Graceful Restart, ExaBGP migration tooling, and logging.

Areas: Graceful Restart, Plugins, ExaBGP Migration, Diagnostics, Config, API, BGP

## [Week of 2026-01-05](2026-01-05/index.md)

This week rounded out route-refresh, ADD-PATH and VPN/labeled-unicast support in the plugin API, alongside a batch of BGP correctness fixes.

Areas: BGP, Graceful Restart, Plugins, API, Flowspec, MPLS, Quality Improvement

## [Week of 2025-12-29](2025-12-29/index.md)

Route-family coverage rounded out (labeled-unicast, MPLS VPN, MUP, route reflection), a batch of protocol-correctness fixes, and a security-relevant change to how Ze listens for BGP sessions.

Areas: Security, BGP, MPLS, Route Server, Flowspec, CLI, Config, Config Editor, Quality Improvement

## [Week of 2025-12-22](2025-12-22/index.md)

Work on the BGP engine itself: route encoding correctness, session robustness, and a first real API surface for driving peers programmatically.

Areas: BGP, Flowspec, API, Plugins, Quality Improvement

## [Week of 2025-12-15](2025-12-15/index.md)

The first tracked week of development on Ze. In seven days the BGP engine went from nothing to a config-driven daemon that speaks the wire protocol, holds a RIB, and tests itself against ExaBGP.

Areas: BGP, Flowspec, ExaBGP Migration, Config, Config Editor, CLI, API
