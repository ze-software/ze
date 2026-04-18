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
| Restart without dropping routes | [Graceful Restart](graceful-restart.md) |
| Back up config on commit or a schedule | [Config Archive](config-archive.md) |
| Manage config through a browser | [Web Interface](web-interface.md) |
| Automate ze from scripts or tools | [REST and gRPC API](api.md) |
| Migrate from ExaBGP | [ExaBGP Migration](exabgp-migration.md) |
| Run Ze in production | [Operations](operations.md) |
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
| [Config Reload](config-reload.md) | Live reload, what changes live vs. requires restart |
| [Web Interface](web-interface.md) | HTTPS web UI for config viewing, editing, and admin commands |

## Features

| Guide | When to use | Description |
|-------|-------------|-------------|
| [RPKI Origin Validation](rpki.md) | Reject hijacked routes | RTR cache, origin validation, fail-open safety |
| [Graceful Restart](graceful-restart.md) | Restart without blackholing traffic | Hold routes during restart window (RFC 4724) |
| [Route Reflection](route-reflection.md) | Forward routes between peers | Route server / reflector setup (RFC 7947) |
| [ADD-PATH](add-path.md) | Forward all paths, not just best | Multiple paths per prefix (RFC 7911) |
| [BGP Role](bgp-role.md) | Prevent route leaks | OTC attribute filtering (RFC 9234) |
| [Monitoring](monitoring.md) | Watch sessions and routes | Real-time event streaming, JSON format |
| [Route Injection](route-injection.md) | Announce routes at runtime | Text, hex, base64 UPDATE commands, commit workflow |
| [VPP Data Plane](vpp.md) | High-throughput forwarding | Ze manages VPP lifecycle and programs its FIB directly via GoVPP |

## Operations

| Guide | Description |
|-------|-------------|
| [Operations](operations.md) | SSH setup, signals, health checks, systemd, troubleshooting |
| [REST and gRPC API](api.md) | Programmatic API: OpenAPI, Swagger UI, SSE streaming, config sessions, TLS, per-user auth |
| [VM Appliance](appliance.md) | Bootable x86_64 image for N100 PCs using gokrazy |
| [MCP Remote Access](mcp/remote-access.md) | SSH tunnels and WireGuard for remote MCP access |
| [Logging](logging.md) | Log levels, backends, per-subsystem tuning, runtime changes |
| [ExaBGP Migration](exabgp-migration.md) | Config conversion and plugin compatibility bridge |
| [Chaos Testing](chaos-testing.md) | Fault injection, deterministic replay, property validation |
| [Fleet Configuration](fleet-config.md) | Centralized config management for multi-node deployments |

## Reference

- [Feature Inventory](../features.md) -- complete list of protocols, attributes, and CLI commands
- [Architecture](../architecture/) -- internal design, wire format, pool architecture
- [Plugin Development](../plugin-development/) -- writing external plugins, IPC protocol, SDK
