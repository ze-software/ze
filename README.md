# Ze

**[ze-software.net](https://ze-software.net)**

> **Pre-release.** Ze is under active development and has not been released yet. The core BGP engine works, and it is covered by 20,000+ unit tests, 1,600+ functional tests, 70+ fuzz targets, chaos replay and 100+ Docker interop scenarios which run it against FRR, BIRD and GoBGP. I keep OpenBGPd, FreeRtr, RustyBGP and rustbgpd images alongside those, for comparison. Some of the more advanced features are still incomplete, and the API and the config syntax may change before a release.

Ze is an open-source configuration and protocol engine. The network operating system I built on it speaks BGP, manages Linux network interfaces, programs the FIB and serves its own configuration over SSH and a web UI.

None of that is in the core. The core is a supervisor which holds a message bus, a config provider and a plugin manager, and it knows nothing about BGP or about any other protocol. BGP, interface management and the rest of it register themselves as subsystems and plugins. Each one arrives with its own YANG and augments the configuration tree at the point where it belongs.

The CLI, the completion, the validation, the web editor and the MCP tools are all derived from that schema. A subsystem which declares its model gets every one of them without any code of its own.

A plugin can be a Go module compiled into the binary, and its YANG is loaded with the rest so the daemon holds the running config to it. It can also be a separate process written in whatever language suits you, and that shape only gets half of this today. `ze schema` runs the plugin with `--yang` and shows you its config model, while the daemon's own validator is still built from the compiled-in modules alone.

There is also an MCP server, which exposes every feature the running daemon has, plugins included, so an AI assistant can ask what this particular instance can do and then operate it.

I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp), and its users are the people I had in mind while building this. They get the same programmability, on a stack which also configures the device and which was written from the start for update rates ExaBGP was never meant to carry.

### Components

| Component | Role |
|-----------|------|
| BGP engine | TCP connections, FSM, message parsing, capability negotiation |
| Interfaces | Linux network management: ethernet, bridge, VLAN, tunnels, WireGuard, DHCP, NTP |
| FIB | Route installation into the kernel forwarding table (netlink) or VPP data plane |
| Config | YANG-modeled configuration with commit, rollback, and live reload |
| CLI | SSH-accessible interactive editor and command shell |
| Web UI | Browser-based configuration editor and admin dashboard |
| Looking glass | Peer status and route viewer, [birdwatcher](https://github.com/alice-lg/birdwatcher)-compatible API |
| Telemetry | Prometheus metrics export with optional Basic Auth and Netdata-compatible OS collectors |
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

### Build Only What You Run

Thirty-six subsystems compile out behind `ze_<feature>` build tags, the BGP engine among them. If you leave a tag off, that code is not in the binary at all, which keeps the image small and the attack surface with it, and a config which selects a subsystem you compiled out is rejected as unknown rather than silently ignored. `make ze-build` builds everything and `make ze-stripped-build` keeps only the SSH management plane. The list of gates is declared once, in `feature-gates.txt`, and every consumer derives from it.

```bash
CGO_ENABLED=0 go build -tags 'ze_core ze_ssh ze_ospf' ./cmd/ze   # an OSPF-only router, no BGP
```

That build is 39 MB where the full binary is 83 MB, and none of the 1,201 BGP reactor symbols are linked into it.

### Wire Performance

| Aspect | Detail |
|--------|--------|
| Parsing | Lazy via offset iterators, no upfront deserialization |
| Forwarding | Zero-copy when source and destination share encoding context |
| Encoding | Buffer-first: all wire writes into pooled, bounded buffers |
| Dedup | Per-attribute-type pools with refcounted handles |

### ExaBGP

Existing ExaBGP plugins run unchanged through a bridge, and `ze config migrate` converts ExaBGP configs.

If you run ExaBGP, I would appreciate it if you could put your own config through `ze config migrate` and tell me what it gets wrong. At this stage that is the feedback which decides what I work on next. You can open an issue, or find me on [Discord](https://discord.gg/T8s7CjPDne).

### Testing

| Type | Scope |
|------|-------|
| Unit tests | 20,000+ test functions as of 2026-07 |
| Linting | 26 linters |
| Functional tests | 1,460+ `.ci` files and 160+ `.et` editor tests: config parsing, wire encoding, plugin behavior, reloads, UI/editor flows, L2TP, firewall, and web |
| Fuzz testing | 70+ targets covering external input parsing as of 2026-07 |
| Chaos testing | Deterministic replay with [configurable scenarios](docs/guide/chaos-testing.md) |
| RFC requirement gate | 2,900+ MUST-level requirements across 168 enrolled RFCs, drafts, and specifications, each proven by a positive and a negative test or annotated with a recorded reason, and every RFC carrying a gap flagged on the status ledger. See [how compliance is enforced](https://github.com/ze-software/ze/wiki/rfc-implementation) and the [RFC status ledger](docs/features/rfc-status.md) |

### Deployment

Ze runs as a daemon on any Linux, under systemd or under whatever else you use to supervise processes, and it also builds into a dedicated appliance image with [gokrazy](https://gokrazy.org) for hardware you never intend to log into. Both are the same binary reading the same config.

| Mode | Description |
|------|-------------|
| Any Linux | Standard daemon, integrates with systemd, journald, and your existing tooling. See [Operations](docs/guide/operations.md) |
| Appliance | Immutable boot image for N100 mini PCs or VMs: read-only root, no shell, automatic supervision. See [VM Appliance](docs/guide/appliance.md) |

The config, the plugins and the hardware are yours. There is no per-instance license to buy, no vendor portal, and nothing in the binary which phones home.

## Quick Start

```bash
git clone https://github.com/ze-software/ze.git && cd ze
make build              # produces bin/ze
bin/ze init             # set up SSH credentials (once)
bin/ze config import router.conf  # or: ze config edit
bin/ze start
```

Requires **Go 1.26+** on a macOS or Linux development host. Windows is not a supported development platform. See the [Quick Start guide](docs/guide/quickstart.md).

## I Want To...

| Task | Start here |
|------|-----------|
| Try Ze for the first time | [Quick Start](docs/guide/quickstart.md) |
| Announce routes to my upstream | [Route Injection](docs/guide/route-injection.md) |
| Migrate from ExaBGP | [ExaBGP Migration](docs/exabgp/exabgp-migration.md) |
| Monitor BGP sessions | [Monitoring](docs/guide/monitoring.md) |
| Restart without dropping routes | [Graceful Restart](docs/guide/graceful-restart.md) |
| Validate routes against RPKI | [RPKI](docs/guide/rpki.md) |
| Write a plugin (Go, Python, Rust) | [Plugin Development](docs/plugin-development/) |
| Understand the internals | [Architecture](docs/architecture.md) |
| Build a route server at an IXP | [Route Reflection](docs/guide/route-reflection.md) (please don't, not yet) |
| Run Ze in production | [Operations](docs/guide/operations.md) |
| Build a dedicated network appliance | [VM Appliance](docs/guide/appliance.md) |
| Compare Ze with other daemons | [Comparison](docs/comparison.md) |

## Documentation

| | |
|-|-|
| **[Architecture](docs/architecture.md)** | One-page overview: components, data flow, key abstractions |
| **[User Guide](docs/guide/)** | Configuration, plugins, operations, and feature guides |
| **[Design Document](docs/DESIGN.md)** | Full design rationale, wire format details, performance analysis |
| **[Feature Inventory](docs/features.md)** | Protocols, attributes, capabilities, CLI commands |
| **[RFC Compliance](https://github.com/ze-software/ze/wiki/rfc-implementation)** | How each MUST-level requirement is bound to tests, and where the gaps are published |
| **[Command Reference](docs/guide/command-reference.md)** | All shell and runtime commands |
| **[Plugin Development](docs/plugin-development/)** | Writing external plugins, IPC protocol, SDK |
| **[Comparison](docs/comparison.md)** | Ze vs FRR, BIRD, GoBGP, OpenBGPd, and others |

## An AI-Assisted Project

Ze is written with Claude Code, which is what made a ground-up BGP rewrite possible for one person. I decide the architecture, the tradeoffs and what the code is never allowed to break, and Claude turns that into implementation. My time is limited, and I would rather spend it on the judgement than on typing out the hundredth attribute codec.

That is also why the test counts above are what they are. Code does not become correct by having been generated, so it has to pass the narrow check and then the wider gate before it belongs here, and building those gates is most of what I do. The longer version of this argument is in [AI slop is the wrong test](https://ze-software.net/blog/ai-slop-is-the-wrong-test/).

Contributors using Claude Code have 28 project-specific slash commands for specs, implementation, review and testing. See the [Claude Code cheat sheet](docs/contributing/claude-code-cheatsheet.md).

## License and Contributions

[GNU Affero General Public License v3.0](LICENSE)

Contributions are welcome if they follow the [contribution process](CONTRIBUTING.md). A [Contributor License Agreement](CLA.md) applies.

## Links

| | |
|-|-|
| **Repo** | [github.com/ze-software/ze](https://github.com/ze-software/ze) |
| **Issues** | [github.com/ze-software/ze/issues](https://github.com/ze-software/ze/issues) |
| **Wiki** | [github.com/ze-software/ze/wiki](https://github.com/ze-software/ze/wiki) |
| **Discord** | [discord.gg/T8s7CjPDne](https://discord.gg/T8s7CjPDne) |
| **ExaBGP** | [github.com/Exa-Networks/exabgp](https://github.com/Exa-Networks/exabgp) |
