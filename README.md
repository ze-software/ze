# Ze -- A Modern BGP Implementation

> **Status: Early Development** -- Ze is under heavy active development and is not yet ready for production use. APIs, configuration syntax, and plugin interfaces may change without notice. Feedback and contributions are welcome.

Ze is a BGP daemon written in Go, built by the creator of [ExaBGP](https://github.com/Exa-Networks/exabgp). Use it to announce routes to your network (traffic engineering, DDoS mitigation, anycast), accept and forward routes (route server at an IXP), monitor BGP sessions, or validate routes against RPKI.

Ze uses a **plugin-based architecture** where the engine handles the protocol and plugins implement everything else -- RIB storage, route policy, route reflection, and any custom logic you need.

## Why Ze

ExaBGP has been trusted in production networks worldwide for over a decade: traffic engineering, DDoS mitigation, route injection, SDN integration, and network monitoring. Ze applies those lessons to a ground-up redesign.

| Design goal | Ze's approach |
|-------------|---------------|
| Decouple RIB from FSM | RIB lives in a plugin -- swap, extend, or skip it entirely |
| Flexible policy | Plugins in any language (Go, Python, Rust) implement your logic |
| Efficient parsing | Wire-first: lazy parsing via iterators, zero-copy forwarding |
| Low memory overhead | Attribute deduplication pools with reference counting |
| Testable BGP behavior | Built-in chaos testing with deterministic virtual clock |

## Quick Start

```bash
git clone https://codeberg.org/thomas-mangin/ze.git
cd ze
make build    # produces bin/ze, bin/ze-test, bin/ze-chaos
```

Requires **Go 1.25+**. See the [Quick Start guide](docs/guide/quickstart.md) for full setup instructions.

```bash
bin/ze config.conf               # Start the BGP daemon
bin/ze validate config.conf      # Validate a configuration
bin/ze bgp decode FFFF...        # Decode a BGP message
```

### Minimal Configuration

```
plugin {
    external rib {
        run "ze plugin bgp-rib"
        encoder json
    }
}

bgp {
    router-id 10.0.0.254
    local { as 65533; }

    peer upstream1 {
        remote { ip 10.0.0.1; as 65001; }
        process rib {
            send [ update ]
            receive [ state ]
        }
    }
}
```

See the [Configuration guide](docs/guide/configuration.md) for groups, inheritance, static routes, and multi-family examples.

## Architecture

```
                         ZeBGP Engine
    +----------+  +----------+  +----------+
    |  Peer 1  |  |  Peer 2  |  |  Peer N  |    BGP sessions
    |   FSM    |  |   FSM    |  |   FSM    |    (per-peer goroutine)
    +----+-----+  +----+-----+  +----+-----+
         +--------------+--------------+
                        |
                 +------+------+
                 |   Reactor   |     Event loop, BGP cache
                 +------+------+
                        |
    ====================|========================
        JSON events     |    commands            Process boundary
    ====================|========================
                        |
         +--------------+--------------+
         |              |              |
    +--------+   +--------+   +--------+
    |  RIB   |   |  RR    |   | Custom |    Plugins
    | Plugin |   | Plugin |   | Plugin |    (any language)
    +--------+   +--------+   +--------+
```

The engine handles TCP, FSM, capability negotiation, and keepalive timers. Plugins own all routing decisions. This separation means you can run without a RIB (pure injection/monitoring), implement custom best-path, or build a route reflector with non-standard policy. See the [Architecture docs](docs/architecture/) for internals.

## Features

Ze ships with **21 plugins** covering core BGP features and every supported address family. See the [Plugin guide](docs/guide/plugins.md) for details and the [Feature Inventory](docs/features.md) for the complete list of supported protocols, attributes, capabilities, and CLI commands.

**Protocol highlights:** IPv4/IPv6 Unicast and Multicast, VPN, EVPN, FlowSpec, Labeled Unicast, BGP-LS (with SR/SRv6), VPLS, MVPN, RTC, MUP. 4-byte ASN, ADD-PATH, Extended Messages, Extended Next-Hop, Graceful Restart, Route Refresh, Role-based filtering, RPKI.

**Performance:** buffer-first encoding (no allocations in hot paths), zero-copy forwarding when peers share capabilities, lazy parsing via iterators, per-type attribute deduplication pools.

**ExaBGP compatibility:** a bridge lets existing ExaBGP plugins work unchanged, and `ze config migrate` converts ExaBGP configs. See the [Migration guide](docs/guide/exabgp-migration.md).

## Testing

18,000+ test functions with race detector, 26 linters, functional tests, ExaBGP wire compatibility tests, fuzz testing, and chaos testing with deterministic replay.

```bash
make ze-verify             # All tests except fuzz (before commits)
make ze-test               # All tests including fuzz
```

See the [Chaos Testing guide](docs/guide/chaos-testing.md) for fault injection, property validation, and failure shrinking.

## Project Status

Ze is in **early active development**. The protocol implementation is broad, the plugin architecture is functional, and the test suite is extensive -- but interfaces are still evolving and breaking changes should be expected.

It already establishes BGP sessions, exchanges routes across all listed address families, and passes thousands of unit, functional, fuzz, and chaos tests. If you're interested in the design or want to contribute, now is a great time to get involved.

### An AI-Assisted Project

Ze exists because large-language-model coding assistants made it feasible. Writing a full BGP implementation from scratch -- with comprehensive RFC compliance, a plugin architecture, and broad address family support -- would be an enormous undertaking for a solo developer. AI tooling (Claude Code) made it realistic to attempt, handling the volume of boilerplate, protocol encoding, and test generation while the author focused on architecture and design decisions informed by over a decade of ExaBGP experience.

Contributions, feedback, and bug reports are welcome on the [issue tracker](https://codeberg.org/thomas-mangin/ze/issues).

## Documentation

- **[User Guide](docs/guide/)** -- quickstart, configuration, plugins, operations, and feature guides
- **[Command Reference](docs/guide/command-reference.md)** -- all shell and runtime commands
- **[Feature Inventory](docs/features.md)** -- protocols, attributes, capabilities, and CLI commands
- **[Architecture](docs/architecture/)** -- internal design, wire format, pool architecture
- **[Plugin Development](docs/plugin-development/)** -- writing external plugins, IPC protocol, SDK
- **[Comparison](docs/comparison.md)** -- Ze vs FRR, BIRD, GoBGP, OpenBGPd, rustbgpd, RustyBGP

## License

[GNU Affero General Public License v3.0](LICENSE)

## Links

- **Repository:** [codeberg.org/thomas-mangin/ze](https://codeberg.org/thomas-mangin/ze)
- **ExaBGP:** [github.com/Exa-Networks/exabgp](https://github.com/Exa-Networks/exabgp)
