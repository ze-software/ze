# Ze

A network OS with programmable routing, written in Go.

[Website](https://ze-software.net/) · [Quick start](docs/guide/quickstart.md) ·
[Demonstrations](https://ze-software.net/demos/terminal/) ·
[Development updates](https://ze-software.net/changes/) ·
[Discord](https://discord.gg/T8s7CjPDne)

Ze brings routing, Linux networking and network services together through a
configuration and plugin engine. You can use it as a programmable BGP controller,
or configure the interfaces and forwarding table of a router. Its own BGP engine
and network services run in one Go binary, with plugins for custom behavior.

One YANG-defined configuration is shared by Ze's SSH CLI, web editor and APIs.
Ze can run as a Linux daemon or an immutable appliance, and its Model Context
Protocol (MCP) server lets AI assistants discover and use the daemon's commands.

<!-- terminal-demo-hero:start -->
[![Ze's live BGP dashboard, opened over SSH](docs/demos/cli-dashboard.svg)](https://ze-software.net/demos/terminal/#live-bgp-dashboard)

The animation shows the configuration editor and live BGP dashboard in one
SSH session. The [site player and transcript](https://ze-software.net/demos/terminal/#live-bgp-dashboard)
provide playback controls and the command sequence. More recordings are on the
[demonstrations page](https://ze-software.net/demos/terminal/).
<!-- terminal-demo-hero:end -->

> **Pre-release.** Ze has not been released yet. Some features are incomplete or
> experimental, and APIs and configuration syntax can change. The
> [feature inventory](https://ze-software.net/reference/feature-status/) records
> their status. A lab is the right place to evaluate Ze before putting it on a
> live network.

I wrote [ExaBGP](https://github.com/Exa-Networks/exabgp), and its users are the
people I had in mind while building Ze. The aim is to keep programmable routing
and add control of the device around it, with one configuration for the protocols
and the system they run on.

## Start Here

| To explore | Start with |
|------------|------------|
| A first BGP session | [Quick Start](docs/guide/quickstart.md) |
| A network lab | [netlab and containerlab](docs/guide/netlab.md) |
| A container | [Docker guide](docs/guide/docker.md) |
| An existing ExaBGP deployment | [Migration guide](docs/exabgp/exabgp-migration.md) |
| A dedicated device or VM | [Appliance guide](docs/guide/appliance.md) |
| Testing Ze with Claude Code or Codex | [Use Ze with an AI agent](docs/guide/ai-agent.md): `ze create agent skill` sets the agent up |

## Why Ze

- **Programmable routing.** Ze keeps ExaBGP's controller model: scripts can
  receive routing events and inject routes. You can develop plugins in any
  programming language, or use the native APIs for your own integrations.
- **Tools for operators.** The CLI provides contextual completion, configuration
  comparisons and a live BGP dashboard. A looking glass exposes routing state,
  and `ze doctor` reports configuration and runtime problems with commands to
  address them.
- **A choice of deployment.** The same configuration model is available in a
  Linux service, a container or an immutable appliance.
- **Tools for AI assistants.** MCP exposes the running daemon's commands.
  For local testing, `ze create agent skill` installs a Ze skill and session-start
  hook for Claude Code or Codex, so an agent can use Ze's own help and tools.

## What You Can Do

Features depend on the build and configuration. The
[feature inventory](https://ze-software.net/reference/feature-status/) distinguishes
supported, experimental, and partial implementations.

| Area | What you can do | Guides |
|------|-----------------|--------|
| BGP and programmable routing | Exchange and inject routes, validate origins with RPKI, and maintain IRR prefix filters with AS-SET discovery through PeeringDB. | [BGP](docs/features/bgp-protocol.md), [route injection](docs/guide/route-injection.md), [RPKI](docs/guide/rpki.md), and [IRR filtering](docs/guide/irr-filtering.md) |
| Routing inside a network | Experiment with OSPF and IS-IS, redistribution and shared routing policy. These features remain experimental. | [OSPF](docs/guide/ospf.md), [IS-IS](docs/guide/isis.md), and [routing policy](docs/guide/route-policy.md) |
| Linux networking | Configure interfaces, routes, firewall rules, and IPsec. | [Interfaces](docs/features/interfaces.md), [static routes](docs/guide/static-routes.md), [firewall](docs/guide/firewall.md), and [IPsec](docs/guide/ipsec.md) |
| Access and network services | Evaluate PPPoE and L2TP access, DNS forwarding and DHCP service in a lab. | [PPPoE](docs/guide/pppoe.md), [L2TP](docs/guide/l2tp.md), [DNS resolver](docs/guide/dns-resolver.md), and [DHCP](https://ze-software.net/reference/plugins/dhcpserver/) |
| Configuration and operations | Edit configuration and inspect the running system. | [Configuration](docs/guide/configuration.md), [CLI](docs/guide/cli.md), [command reference](docs/guide/command-reference.md), and [operations](docs/guide/operations.md) |
| Automation and visibility | Extend Ze and observe its routing state. | [Plugins](docs/plugin-development/), [MCP](docs/guide/mcp/overview.md), [AI agents](docs/guide/ai-agent.md), [monitoring](docs/guide/monitoring.md), and [looking glass](docs/guide/looking-glass.md) |

In the [configuration editor](docs/guide/config-editor.md), you can edit a value,
review the changes with `show | compare`, validate them with `commit verify`,
and apply them with `commit now`. [Live configuration reload](docs/guide/config-reload.md)
can add peers and apply changes without restarting the daemon. A change that
affects BGP negotiation can restart the affected sessions.

## Build from Source

Building requires Git and Go on Linux or macOS. The supported Go toolchain is
pinned in [go.mod](go.mod), and the launcher selects it for the first build.
Linux is the platform for the network operating system's kernel features.

```bash
git clone https://github.com/ze-software/ze.git
cd ze
./ze version
```

On a fresh checkout, the `./ze` launcher builds the daemon before it runs the
command. It derives the feature tags from [feature-gates.txt](feature-gates.txt),
so there is no tag list to maintain by hand. Later invocations reuse the existing
binary.

The [Quick Start guide](docs/guide/quickstart.md) covers credentials, an example
configuration, and commands to start Ze and inspect its peers. The
[Ubuntu installation guide](docs/guide/ubuntu-build-install.md) covers a
systemd installation.

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

## Architecture and Plugins

The SSH CLI and web editor use a shared configuration schema for validation.
Protocols and system features register themselves and contribute their own YANG
modules to that schema.

The protocol-independent core manages subsystem lifecycles through a message
bus, a configuration provider, and a plugin manager. Commands register once
and are available through the CLI, SSH, web interface, REST, gRPC and MCP.

Plugins can be compiled Go modules or separate processes. Compiled plugins
contribute their YANG to the daemon's configuration validator. External plugins
can expose a model through `ze schema`, but that model does not automatically
extend the daemon's validator.

The [architecture overview](docs/architecture.md) describes the runtime and BGP
wire design. The [plugin development guide](docs/plugin-development/) covers
the SDK and process protocol.

### Build Only What You Run

Subsystems have `ze_<feature>` build tags. A custom build can omit BGP or an
operator interface along with its registered schema. The
[architecture guide](docs/architecture.md) explains the component boundaries,
and the feature manifest names the packages behind each tag.

## Testing and Project Status

The [testing overview](https://ze-software.net/quality/) explains the unit,
functional, fuzz, mutation, and chaos tests, plus Linux checks under QEMU.
The [interop guide](docs/architecture/testing/interop.md) describes scenarios
against other implementations, including FRR, BIRD, and GoBGP.

The [RFC requirement ledger](https://ze-software.net/quality/rfc-compliance/)
publishes requirements and their test evidence, including gaps. A requirement
counts as proven when a recorded test run detects a deliberate break in the
code behind it. Test evidence helps evaluate the implementation, but production
confidence also requires deployment experience.

The [development updates](https://ze-software.net/changes/) describe changes
and configuration differences to check when upgrading. The
[milestones](https://ze-software.net/project/milestones/) show how the project
has grown, and the [roadmap](https://ze-software.net/project/roadmap/) records
the work toward a first release.

### AI-Assisted Development

Ze is developed with AI coding assistants. I decide the architecture, the
tradeoffs, and what the code must preserve. Agreed specifications, tests and
independent review constrain how agents develop the code. The process is
iterative, and published code and documentation can still contain mistakes.
Reports of those mistakes are welcome. I explain the reasoning in
[AI slop is the wrong test](https://ze-software.net/blog/ai-slop-is-the-wrong-test/).

## Contributing and License

Bug reports and contributions are welcome through the
[contribution process](CONTRIBUTING.md). A
[Contributor License Agreement](CLA.md) applies to contributions.

Ze is copyright © 2025-2026 Thomas Mangin, and licensed under the
[GNU Affero General Public License, version 3 only](LICENSE)
(`AGPL-3.0-only`). See [NOTICE](NOTICE).
