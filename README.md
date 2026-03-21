# Ze -- A Modern BGP Daemon

> **Early Development** -- not yet production-ready. APIs and config syntax may change.

Ze is a BGP daemon in Go from the creator of [ExaBGP](https://github.com/Exa-Networks/exabgp). Plugin-based architecture: the engine handles the protocol, plugins handle everything else -- RIB, policy, route reflection, graceful restart. Plugins can be written in any language.

**Wire-first performance.** Lazy parsing via iterators, zero-copy forwarding, buffer-first encoding, per-type attribute dedup pools.

**21 plugins.** IPv4/IPv6 Unicast/Multicast, VPN, EVPN, FlowSpec, BGP-LS, Labeled Unicast, VPLS, MVPN, RTC, MUP, ADD-PATH, Graceful Restart, Route Reflection, RPKI, and more.

**ExaBGP compatibility.** Existing plugins work unchanged via a bridge; `ze config migrate` converts configs.

**Tested.** 18,000+ test functions, 26 linters, functional tests, fuzz testing, and [chaos testing](docs/guide/chaos-testing.md) with deterministic replay.

## Quick Start

```bash
git clone https://codeberg.org/thomas-mangin/ze.git && cd ze
make build    # produces bin/ze
bin/ze config.conf
```

Requires **Go 1.25+**. See the [Quick Start guide](docs/guide/quickstart.md).

## I Want To...

| Task | Start here |
|------|-----------|
| Try Ze for the first time | [Quick Start](docs/guide/quickstart.md) |
| Announce routes to my upstream | [Route Injection](docs/guide/route-injection.md) |
| Build a route server at an IXP | [Route Reflection](docs/guide/route-reflection.md) |
| Monitor BGP sessions | [Monitoring](docs/guide/monitoring.md) |
| Validate routes against RPKI | [RPKI](docs/guide/rpki.md) |
| Restart without dropping routes | [Graceful Restart](docs/guide/graceful-restart.md) |
| Migrate from ExaBGP | [ExaBGP Migration](docs/guide/exabgp-migration.md) |
| Run Ze in production | [Operations](docs/guide/operations.md) |
| Write a plugin (Go, Python, Rust) | [Plugin Development](docs/plugin-development/) |
| Understand the internals | [Design Document](docs/DESIGN.md) |
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

## License

[GNU Affero General Public License v3.0](LICENSE)

**Repository:** [codeberg.org/thomas-mangin/ze](https://codeberg.org/thomas-mangin/ze) -- [Issues](https://codeberg.org/thomas-mangin/ze/issues) -- [ExaBGP](https://github.com/Exa-Networks/exabgp)
