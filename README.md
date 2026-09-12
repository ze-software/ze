# Ze

Ze is an open-source configuration and protocol engine written in Go. The network
operating system built on it speaks BGP, manages Linux network interfaces, and
programs the forwarding table. Operators use a shared configuration model through
an SSH CLI or a web editor.

> **Pre-release.** Ze has not been released yet. Some features are incomplete or
> experimental, and APIs and configuration syntax can change. The
> [feature inventory](docs/features.md) records their status. A lab is the right
> place to evaluate Ze before putting it on a live network.

I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp), and its users are the
people I had in mind while building Ze. The aim is to keep programmable routing
and add control of the device around it, with one configuration for the protocols
and the system they run on.

## Start Here

| To explore | Start with |
|------------|------------|
| A first BGP session | [Quick Start](docs/guide/quickstart.md) |
| A network lab | [netlab and containerlab](docs/guide/netlab.md) |
| An existing ExaBGP deployment | [Migration guide](docs/exabgp/exabgp-migration.md) |
| A dedicated device or VM | [Appliance guide](docs/guide/appliance.md) |

## What Ze Includes

Features depend on the build and configuration. The guides describe how to use
each area, and the [feature inventory](docs/features.md) distinguishes supported,
experimental, and partial implementations.

| Area | Guides |
|------|--------|
| Routing | [BGP](docs/features/bgp-protocol.md), [route injection](docs/guide/route-injection.md), [OSPF](docs/guide/ospf.md), and [RPKI validation](docs/guide/rpki.md) |
| Linux networking | [Interfaces](docs/features/interfaces.md), [static routes](docs/guide/static-routes.md), [firewall](docs/guide/firewall.md), and [IPsec](docs/guide/ipsec.md) |
| Configuration and operations | [Configuration](docs/guide/configuration.md), [CLI](docs/guide/cli.md), [command reference](docs/guide/command-reference.md), and [operations](docs/guide/operations.md) |
| Automation and visibility | [Plugins](docs/plugin-development/), [MCP](docs/guide/mcp/overview.md), [monitoring](docs/guide/monitoring.md), and [looking glass](docs/guide/looking-glass.md) |

## Build from Source

Development requires Go 1.27 or newer on Linux or macOS. Linux is the platform for
the network operating system's kernel features.

```bash
git clone https://github.com/ze-software/ze.git
cd ze
./ze --help
```

On a fresh checkout, the `./ze` launcher builds the daemon before it runs the
command. It derives the feature tags from [feature-gates.txt](feature-gates.txt),
so there is no tag list to maintain by hand. Later invocations reuse the existing
binary.

The [Quick Start guide](docs/guide/quickstart.md) covers credentials, an example
configuration, and commands to start Ze and inspect its peers. The
[Ubuntu installation guide](docs/guide/ubuntu-build-install.md) covers a
systemd installation.

### Build Only What You Run

Subsystems have `ze_<feature>` build tags. A custom build can omit BGP or an
operator interface along with its registered schema. The
[architecture guide](docs/architecture.md) explains the component boundaries,
and the feature manifest names the packages behind each tag.

## Architecture and Plugins

The core is a protocol-independent supervisor. It manages subsystem lifecycles
through a message bus, a configuration provider, and a plugin manager. Protocols
and system features register themselves and contribute their own YANG modules.

Those modules form a shared configuration schema for validation and the editors.
The Model Context Protocol (MCP) server derives tools from the running daemon's
command catalog, so AI clients can discover its available commands.

Plugins can be compiled Go modules or separate processes. Compiled plugins
contribute their YANG to the daemon's configuration validator. External plugins
can expose a model through `ze schema`, but that model does not automatically
extend the daemon's validator.

The [architecture overview](docs/architecture.md) describes the runtime and BGP
wire design. The [plugin development guide](docs/plugin-development/) covers
the SDK and process protocol.

## From ExaBGP

`ze exabgp migrate` converts ExaBGP configuration files, and `ze exabgp plugin`
provides a bridge for existing process scripts. The
[migration guide](docs/exabgp/exabgp-migration.md) documents the syntax and API
differences to check when moving a deployment.

If you run ExaBGP, I would appreciate reports from your own configurations and
process scripts. A report that shows what failed and what ExaBGP did with the
same input gives me something concrete to fix. You can use the
[issue tracker](https://github.com/ze-software/ze/issues) or
[Discord](https://discord.gg/T8s7CjPDne).

## Testing and Project Status

The [testing overview](https://ze-software.net/quality/) explains the unit,
functional, fuzz, mutation, and chaos tests, plus Linux checks under QEMU.
The [interop guide](docs/architecture/testing/interop.md) describes scenarios
against other implementations, including FRR, BIRD, and GoBGP.

The [RFC requirement ledger](https://ze-software.net/quality/rfc-compliance/)
publishes requirements and their test evidence, including gaps. These records
help evaluate the current implementation. They do not replace deployment
experience.

### AI-Assisted Development

Ze is developed with AI coding assistants. I decide the architecture, the
tradeoffs, and what the code must preserve. Tests and independent review are
part of that process. I explain the reasoning in
[AI slop is the wrong test](https://ze-software.net/blog/ai-slop-is-the-wrong-test/).

## Contributing and License

Bug reports and contributions are welcome through the
[contribution process](CONTRIBUTING.md). A
[Contributor License Agreement](CLA.md) applies to contributions.

Ze is licensed under the [GNU Affero General Public License v3.0](LICENSE).
