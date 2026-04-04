# Ze -- The Programmable Plugin-Based Network OS

> **Pre-Alpha** -- Ze has not reached alpha. The core BGP engine works, but many advanced features are incomplete or untested, and there is significant work remaining before Ze is ready for end-users. APIs and config syntax will change without notice.

Ze is a BGP daemon written in Go, by the creator of [ExaBGP](https://github.com/Exa-Networks/exabgp).

The engine provides a message bus. Components and plugins connect to it. The engine handles TCP, FSM, OPEN/UPDATE parsing, and capability negotiation. Everything else is a plugin. Plugins can be written in any language.

### Components

| Component | Role |
|-----------|------|
| BGP engine | TCP connections, FSM, message parsing, capability negotiation |
| Config | YANG-modeled configuration, validation, live reload |
| CLI | SSH-accessible interactive editor and command shell |
| Web UI | Browser-based configuration editor |
| Looking glass | Peer status and route viewer, [birdwatcher](https://github.com/alice-lg/birdwatcher)-compatible API |
| Telemetry | Prometheus metrics export |
| MCP | Model Context Protocol server for AI tool integration |

### Plugins

| Type | Plugins |
|------|---------|
| Storage | bgp-rib, bgp-adj-rib-in, bgp-persist |
| Policy | bgp-rs, bgp-filter-community, bgp-role |
| Resilience | bgp-gr, bgp-watchdog, bgp-route-refresh |
| Validation | bgp-rpki, bgp-rpki-decorator |
| Capabilities | bgp-aigp, bgp-hostname, bgp-llnh, bgp-softver |
| Address families | bgp-nlri-vpn, bgp-nlri-evpn, bgp-nlri-flowspec, bgp-nlri-ls, bgp-nlri-labeled, bgp-nlri-vpls, bgp-nlri-mvpn, bgp-nlri-rtc, bgp-nlri-mup |

IPv4/IPv6 unicast and multicast are built into the engine. See [Feature Inventory](docs/features.md) for details.

### Wire Performance

| Aspect | Detail |
|--------|--------|
| Parsing | Lazy via offset iterators, no upfront deserialization |
| Forwarding | Zero-copy when source and destination share encoding context |
| Encoding | Buffer-first: all wire writes into pooled, bounded buffers |
| Dedup | Per-attribute-type pools with refcounted handles |

### ExaBGP

Existing ExaBGP plugins work unchanged via a bridge. `ze config migrate` converts ExaBGP configs.

If you are an ExaBGP user, we would love your feedback on the migration experience. Please try `ze config migrate` with your configs and let us know what works and what does not -- even at this early stage, that feedback shapes the project. File issues or reach out on [Discord](https://discord.gg/ykJb8meS4).

### Testing

| Type | Scope |
|------|-------|
| Unit tests | 18,000+ test functions |
| Linting | 26 linters |
| Functional tests | Config parsing, wire encoding, plugin behavior |
| Fuzz testing | All external input parsing |
| Chaos testing | Deterministic replay with [configurable scenarios](docs/guide/chaos-testing.md) |

## Quick Start

```bash
git clone https://codeberg.org/thomas-mangin/ze.git && cd ze
make build              # produces bin/ze
bin/ze init             # set up SSH credentials (once)
bin/ze config import router.conf  # or: ze config edit
bin/ze start
```

Requires **Go 1.25+**. See the [Quick Start guide](docs/guide/quickstart.md).

## I Want To...

| Task | Start here |
|------|-----------|
| Try Ze for the first time | [Quick Start](docs/guide/quickstart.md) |
| Announce routes to my upstream | [Route Injection](docs/guide/route-injection.md) |
| Migrate from ExaBGP | [ExaBGP Migration](docs/guide/exabgp-migration.md) |
| Monitor BGP sessions | [Monitoring](docs/guide/monitoring.md) |
| Restart without dropping routes | [Graceful Restart](docs/guide/graceful-restart.md) |
| Validate routes against RPKI | [RPKI](docs/guide/rpki.md) |
| Write a plugin (Go, Python, Rust) | [Plugin Development](docs/plugin-development/) |
| Understand the internals | [Design Document](docs/DESIGN.md) |
| Build a route server at an IXP | [Route Reflection](docs/guide/route-reflection.md) (please don't, not yet) |
| Run Ze in production | [Operations](docs/guide/operations.md) |
| Compare Ze with other daemons | [Comparison](docs/comparison.md) |

## Documentation

| | |
|-|-|
| **[User Guide](docs/guide/)** | Configuration, plugins, operations, and feature guides |
| **[Design Document](docs/DESIGN.md)** | Architecture, goals, and design rationale |
| **[Feature Inventory](docs/features.md)** | Protocols, attributes, capabilities, CLI commands |
| **[Command Reference](docs/guide/command-reference.md)** | All shell and runtime commands |
| **[Plugin Development](docs/plugin-development/)** | Writing external plugins, IPC protocol, SDK |
| **[Comparison](docs/comparison.md)** | Ze vs FRR, BIRD, GoBGP, OpenBGPd, and others |

## An AI-Assisted Project

Ze exists because AI coding assistants (Claude Code) made a ground-up BGP rewrite feasible for a solo developer. The author focused on architecture and design decisions informed by a decade of ExaBGP; AI handled the volume of protocol encoding, boilerplate, and test generation.

## License and Contributions

[GNU Affero General Public License v3.0](LICENSE)

Contributions are welcome if they follow the [contribution process](CONTRIBUTING.md). A [Contributor License Agreement](CLA.md) applies.

## Links

| | |
|-|-|
| **Official repo** | [github.com/ze-software/ze](https://github.com/ze-software/ze) |
| **Development** | [codeberg.org/thomas-mangin/ze](https://codeberg.org/thomas-mangin/ze) |
| **Issues** | [github.com/ze-software/ze/issues](https://github.com/ze-software/ze/issues) |
| **Discord** | [discord.gg/ykJb8meS4](https://discord.gg/ykJb8meS4) |
| **ExaBGP** | [github.com/Exa-Networks/exabgp](https://github.com/Exa-Networks/exabgp) |
