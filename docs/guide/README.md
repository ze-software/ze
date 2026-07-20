# Ze User Guide

**[Project Status](status.md)** -- what works, what doesn't, Ze vs ExaBGP, and advice for early adopters.

## I want to...

| Task | Start here |
|------|-----------|
| Get Ze running for the first time | [Quick Start](quickstart.md) |
| Announce routes to my upstream | [Quick Start](quickstart.md), then [Route Injection](route-injection.md) |
| Build a route server at an IXP | [Route Reflection](route-reflection.md) |
| Monitor BGP sessions in real time | [Monitoring](monitoring.md) |
| Validate routes against RPKI | [RPKI Origin Validation](rpki.md) |
| Filter customer BGP routes from IRR data | [IRR BGP Import Filtering](irr-filtering.md) |
| Restart without dropping routes | [Graceful Restart](graceful-restart.md) |
| Back up config on commit or a schedule | [Config Archive](config-archive.md) |
| Manage config through a browser | [Web Interface](web-interface.md) |
| Automate ze from scripts or tools | [REST and gRPC API](api.md) |
| Migrate from ExaBGP | [ExaBGP Migration](../exabgp/exabgp-migration.md) |
| Run Ze in production | [Operations](operations.md) |
| Run Ze in Docker | [Docker](docker.md) |
| Build a VM appliance for an N100 PC | [VM Appliance](appliance.md) |
| Debug a peer that won't come up | [Operations](operations.md#troubleshooting) |

## Getting Started

| Guide | Description |
|-------|-------------|
| [Quick Start](quickstart.md) | Build, configure, run, and verify in 5 minutes |
| [Configuration](configuration.md) | Config syntax, peer setup, groups, static routes |
| [Plugins](plugins.md) | Which plugins you need, how to load and bind them |
| [CLI Reference](cli.md) | Interactive CLI, runtime commands, peer/route/cache operations |
| [Command Reference](command-reference.md) | Complete reference for all shell and runtime commands |
| [Command Catalogue](command-catalogue.md) | Cross-vendor roadmap: VyOS/Junos/Nokia/Arista commands vs ze status and backend requirements |
| [Config Editor](config-editor.md) | Interactive NOS-like editor with YANG tab completion |
| [Config Archive](config-archive.md) | Archive configs to local/remote destinations on commit or schedule |
| [Config Deactivate](config-deactivate.md) | Junos-style inactive marking on any YANG node |
| [Config Reload](config-reload.md) | Live reload, what changes live vs. requires restart |
| [Environment Variables](environment-variables.md) | Runtime tuning via `ze.*` env vars |
| [Authentication](authentication.md) | User database, SSH keys, TACACS+, RADIUS, bcrypt |
| [Audit Trail](audit.md) | Local structured records for config changes, reloads, and failed logins |
| [Web Interface](web-interface.md) | HTTPS web UI for config viewing, editing, and admin commands |

## Features

| Guide | When to use | Description |
|-------|-------------|-------------|
| [RPKI Origin Validation](rpki.md) | Reject hijacked routes | RTR cache, origin validation, fail-open safety |
| [IRR BGP Import Filtering](irr-filtering.md) | Reject unregistered customer routes | Dynamic prefix-lists from ASNs and AS-SETs |
| [Graceful Restart](graceful-restart.md) | Restart without blackholing traffic | Hold routes during restart window (RFC 4724) |
| [Route Reflection](route-reflection.md) | Forward routes between peers | Route server / reflector setup (RFC 7947) |
| [ADD-PATH](add-path.md) | Forward all paths, not just best | Multiple paths per prefix (RFC 7911) |
| [BGP Role](bgp-role.md) | Prevent route leaks | OTC attribute filtering (RFC 9234) |
| [Monitoring](monitoring.md) | Watch sessions and routes | Real-time event streaming, JSON format |
| [Health Checks](health-checks.md) | Pre-start and runtime health | `ze doctor`, `show health`, report bus |
| [Production Diagnostics](production-diagnostics.md) | Debug without external tools | 11 commands replacing ss, dmesg, tcpdump, etc. |
| [Self-Update](self-update.md) | Automated firmware updates | SHA-256 verified download, fleet rollout, rollback |
| [Route Injection](route-injection.md) | Announce routes at runtime | Text, hex, base64 UPDATE commands, commit workflow |
| [Static Routes](static-routes.md) | ECMP, BFD failover, PBR | Named tables, weighted next-hops, blackhole/reject |
| [Firewall](firewall.md) | Packet filter and NAT | nftables and VPP backends, FlowSpec bridge |
| [DDoS Mitigation](ddos-mitigation.md) | Detect and stop volumetric floods | Baseline detection, local nftables, FlowSpec, and cloud reporting |
| [Anomaly Detection](anomaly.md) | Detect unusual source behaviour | Per-source baselines and shadow-first response |
| [BFD](bfd.md) | Sub-second failure detection | RFC 5880, auth, echo mode, BGP opt-in |
| [BMP](bmp.md) | BGP monitoring protocol | Receiver, sender, Adj-RIB-Out, looking glass |
| [L2TP/PPP](l2tp.md) | BNG subscriber access | RFC 2661, RADIUS, CQM, web UI |
| [PPPoE](pppoe.md) | Direct-attach subscribers | RFC 2516 access concentrator |
| [IPsec VPN](ipsec.md) | Site-to-site and remote-access VPN | Native IKEv2 initiator and responder, XFRM, EAP, NAT-T |
| [Zero-Touch Provisioning](ze-install.md) | PXE bare-metal provisioning | DHCP+TFTP+HTTP image server |
| [VPP Data Plane](vpp.md) | High-throughput forwarding | Ze manages VPP lifecycle and programs its FIB directly via GoVPP |

## Operations

| Guide | Description |
|-------|-------------|
| [Operations](operations.md) | SSH setup, signals, health checks, systemd, troubleshooting |
| [Health Checks](health-checks.md) | `ze doctor` pre-start checks and runtime `show health` |
| [Production Diagnostics](production-diagnostics.md) | Symptom-based troubleshooting with built-in diagnostic commands |
| [Self-Update](self-update.md) | Automated firmware updates with fleet rollout and rollback |
| [REST and gRPC API](api.md) | Programmatic API: OpenAPI, Swagger UI, SSE streaming, config sessions, TLS, per-user auth |
| [Docker](docker.md) | Container image for evaluation, labs, and lightweight deployments |
| [VM Appliance](appliance.md) | Bootable x86_64 image for N100 PCs using gokrazy |
| [Zero-Touch Provisioning](ze-install.md) | PXE bare-metal provisioning |
| [MCP Remote Access](mcp/remote-access.md) | SSH tunnels and WireGuard for remote MCP access |
| [Logging](logging.md) | Log levels, backends, per-subsystem tuning, runtime changes |
| [Operational Reports](operational-reports.md) | Warnings, errors, and the report bus |
| [Audit Trail](audit.md) | `show audit`, config commit/discard records, auth-fail records |
| [Benchmarking](benchmarking.md) | `ze-perf` cross-implementation latency benchmark |
| [ExaBGP Migration](../exabgp/exabgp-migration.md) | Config conversion and plugin compatibility bridge |
| [Chaos Testing](chaos-testing.md) | Fault injection, deterministic replay, property validation |
| [Fleet Configuration](fleet-config.md) | Centralized config management for multi-node deployments |
| [TACACS+ AAA](tacacs.md) | RFC 8907 SSH authentication and command accounting |
| [RADIUS admin AAA](radius.md) | RFC 2865 operator login (SSH/web/MCP) with Filter-Id profile mapping |

## Reference

- [Feature Inventory](../features.md) -- complete list of protocols, attributes, and CLI commands
- [Architecture](../architecture/) -- internal design, wire format, pool architecture
- [Plugin Development](../plugin-development/) -- writing external plugins, IPC protocol, SDK
