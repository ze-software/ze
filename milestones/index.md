# Milestones

The landmarks that mark Ze's path from a bare BGP speaker to a full network operating system. Each entry is the first time a whole capability arrived; the week-by-week detail lives in the Changes log and the blog. Newest first, color-coded by category.

## Q3 2026

### VRRP first-hop redundancy (Jul 2026)

*routing*

First-hop gateway redundancy with RFC 9568 VRRPv3 (IPv4 and IPv6) and RFC 3768 VRRPv2, a per-group virtual-MAC macvlan for transparent L2 failover, and keepalived interop under QEMU.

[Read the week](../changes/2026-07-13/)

## Q2 2026

### DDoS auto-mitigation (Jun 2026)

*secure*

Control-plane survival under attack: GTSM/TTL-security (RFC 5082), CoPP policing on TCP/179, and automatic DDoS detection with attack-characterized auto-mitigation.

[Read the week](../changes/2026-06-25/)

### OSPFv2 / OSPFv3 (Jun 2026)

*routing*

A unified OSPFv2/OSPFv3 engine with IPv6 interop coverage and live SSE state views in the web UI.

[Read the week](../changes/2026-06-22/)

### Native IS-IS (Jun 2026)

*routing*

A native IS-IS link-state IGP (ISO/IEC 10589, RFC 1195): full PDU/TLV codec, adjacency FSM, LSDB flooding, DIS election, and SPF with ECMP, interop-tested against FRR's isisd.

[Read the week](../changes/2026-06-15/)

### MPLS label switching (May 2026)

*routing*

Label switching across three layers, verified in QEMU against FRR: a kernel MPLS dataplane, LDP (RFC 5036), and RSVP-TE.

[Read the week](../changes/2026-05-25/)

### IPsec / IKEv2 VPN (May 2026)

*secure*

A native IKEv2 VPN stack built from the wire format up and interop-tested against strongSwan: child SA negotiation, EAP, NAT-T, and route-based VPN via XFRM.

[Read the week](../changes/2026-05-18/)

### Appliance fleet management (May 2026)

*platform*

`ze appliance` manages images end to end: encrypted secrets, TLS and SSH provisioning, day-2 operations, remote push, and export/import for disaster recovery.

[Read the week](../changes/2026-05-04/)

### L2TP/PPP broadband access (Apr 2026)

*services*

A full L2TPv2 stack for broadband access: tunnel and session FSMs, a reliable delivery engine, and PPP authentication (PAP, CHAP-MD5, MS-CHAPv2).

[Read the week](../changes/2026-04-13/)

### VPP dataplane (Apr 2026)

*platform*

A VPP dataplane backend for high-performance forwarding: connection management, DPDK binding, and FIB programming via GoVPP.

[Read the week](../changes/2026-04-13/)

### Firewall and traffic control (Apr 2026)

*secure*

nftables and tc-netlink backends sharing one YANG data model, with `show firewall` and `show traffic-control` commands.

[Read the week](../changes/2026-04-13/)

### BFD liveness detection (Apr 2026)

*routing*

A complete BFD implementation (RFC 5880/5881/5883): single- and multi-hop, authentication, echo mode, BGP session opt-in, and operator visibility.

[Read the week](../changes/2026-04-06/)

### gokrazy appliance build (Apr 2026)

*platform*

The first gokrazy VM appliance build for x86_64: the start of Ze shipping as a self-contained appliance image, not just a daemon.

[Read the week](../changes/2026-04-06/)

## Q1 2026

### Interfaces and kernel FIB (Mar 2026)

*platform*

A JunOS-style interface management subsystem (netlink monitoring, DHCP, SLAAC) and a FIB pipeline that installs best-path routes into the kernel via netlink.

[Read the week](../changes/2026-03-30/)

### Web interface (Mar 2026)

*operate*

A browser-based config editor with YANG-driven rendering, per-user drafts, inline diffs, live SSE updates, and a light/dark theme, started with `ze start --web`.

[Read the week](../changes/2026-03-23/)

### MCP server for AI operations (Mar 2026)

*automate*

An MCP server exposing tools for AI-assisted BGP operations: announce, withdraw, peer status, peer control, and command execution.

[Read the week](../changes/2026-03-23/)

### RPKI origin validation (Mar 2026)

*secure*

A full RPKI pipeline: an RTR-speaking plugin maintains a ROA cache and validates route origins as routes arrive on the adjacency RIB-in path, not after the fact.

[Read the week](../changes/2026-03-16/)

### SSH CLI, TCP-MD5, and RBAC (Mar 2026)

*secure*

An SSH server becomes the primary way to reach the CLI, alongside TCP-MD5 session authentication (RFC 2385) and end-to-end RBAC authorization.

[Read the week](../changes/2026-03-09/)

### Best-path selection (Mar 2026)

*routing*

On-demand best-path selection in the RIB, covering LOCAL_PREF, AS_PATH length, ORIGIN, MED, eBGP/iBGP preference, and the full tiebreak chain (RFC 4271 section 9.1.2).

[Read the week](../changes/2026-03-02/)

### BGP Route Server (Feb 2026)

*routing*

A forward-all Route Server (RFC 7947) for IXPs: targeted per-peer replay on reconnect, backpressure-safe forwarding, and plugin dependency resolution.

[Read the week](../changes/2026-02-23/)

### Hub architecture and Plugin SDK (Jan 2026)

*platform*

Ze splits into a hub orchestrator that forks BGP as a child process, gains live config reload over SIGHUP, and ships a documented SDK for writing plugins outside the core tree.

[Read the week](../changes/2026-01-19/)

### ExaBGP migration path (Jan 2026)

*automate*

`exabgp migrate` converts ExaBGP configs to Ze's format, and a bridge runs existing ExaBGP process plugins under Ze, translating JSON and commands both ways.

[Read the week](../changes/2026-01-12/)

## Q4 2025

### BGP engine (Dec 2025)

*routing*

The foundational BGP speaker lands: wire-format codec, every message type, capability negotiation, path attributes, a RIB, and the finite state machine, tested against ExaBGP from day one.

[Read the week](../changes/2025-12-15/)

### Config model and CLI editor (Dec 2025)

*operate*

A schema-driven configuration parser with ExaBGP-compatible syntax, wired straight to the reactor, plus an interactive CLI editor with autocomplete.

[Read the week](../changes/2025-12-15/)
