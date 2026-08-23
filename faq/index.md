# Frequently asked questions

Straight answers about what Ze does, where it is ready, and what adopting it involves. Ask on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues) when your question is missing.

## Product and readiness

Start here if you are deciding whether Ze belongs in your network.

**What is Ze?**

Ze is an open-source configuration and protocol engine. The repository also builds a Linux network operating system around that engine. It speaks BGP, manages network interfaces, programs the FIB, and serves one configuration over SSH, a web UI, API, and MCP.

The core is a protocol-agnostic supervisor with an event bus, configuration provider, and plugin manager. Each subsystem registers its own YANG model. Ze uses that model to derive the CLI, completion, validation, web editor, generated reference, and MCP tools.

**Is Ze ready for production?**

No. Ze is pre-release software. The BGP engine is substantial and heavily tested, but production exposure remains limited, and both APIs and configuration syntax can still change.

Use Ze in a lab today. Build a route server, migrate an ExaBGP configuration, run the interop labs against FRR or BIRD, and report failures. The [roadmap](https://ze-software.net/project/roadmap/) lists the release work, while the [quality pages](https://ze-software.net/quality/) show the evidence already available.

**What can Ze actually do today?**

Ze has a broad BGP implementation, YANG-modeled configuration with commit and rollback, Linux interface and FIB management, an SSH CLI, a web interface, APIs, observability, and a plugin system.

OSPF, IS-IS, MPLS, firewall, VPN, PPPoE, and L2TP code also exists, but those areas remain experimental. Use the [feature catalog](https://ze-software.net/features/) for current status instead of treating this answer as a compatibility matrix.

**Why would I use Ze instead of BIRD, FRR, or GoBGP?**

Choose a mature daemon when you need an established production platform today. Ze is worth evaluating when you want its configuration model, generated operator surfaces, plugin architecture, ExaBGP migration path, or built-in evidence tooling.

The core remains protocol-agnostic, and each subsystem brings its own YANG model. One schema drives configuration, validation, CLI, web, generated reference, and MCP. The [comparison pages](https://ze-software.net/compare/) show where Ze leads, where it differs, and where the mature projects remain ahead.

## Running and adopting Ze

Practical questions about trying Ze without committing a network to it.

**Where does Ze run?**

The daemon runs on Linux under systemd or another process supervisor. Development builds work on macOS and Linux with the supported Go toolchain. Windows is not a supported development or runtime platform.

Start with the [quickstart](https://ze-software.net/guides/quickstart/). Use the [appliance lab](https://ze-software.net/labs/appliance-install/) when you want an immutable boot image for a VM or dedicated hardware.

**I run ExaBGP. Can I move to Ze?**

Yes. Ze includes `ze config migrate` for configuration conversion and a compatibility bridge that lets existing ExaBGP process scripts run while you port them.

The [ExaBGP migration guide](https://ze-software.net/use-cases/exabgp-migration/) covers conversion, known differences, and the point where a native Ze plugin becomes the better choice.

**The daemon or the appliance: which should I run?**

Start with the daemon. It fits into Linux infrastructure you already supervise and is easier to inspect while Ze is still pre-release.

The appliance packages the same binary and configuration into a read-only system with automatic supervision, no package manager, and no interactive shell. Use it when you want a purpose-built router image rather than another managed Linux host.

**Will my configuration keep working as Ze changes?**

Treat configuration syntax as unstable until the first release. Breaking changes belong in the [change log](https://ze-software.net/project/changes/) and should arrive with an automatic migration or a clear error. Silent reinterpretation is a bug.

Configuration stability is an explicit milestone on the [roadmap](https://ze-software.net/project/roadmap/).

**Does Ze need an LLM to run?**

No. Ze runs without an AI service. MCP is an optional operator interface generated from the same schemas and command catalog as the CLI and web interface. It lets an authorized assistant discover and operate the capabilities compiled into a running daemon.

## Project and support

How the project is licensed, maintained, and supported.

**What does the AGPLv3 license mean for me?**

You can run, inspect, modify, and redistribute Ze under the [GNU Affero General Public License v3](https://ze-software.net/license/). Running an unmodified Ze for your own network does not require publishing your configuration.

If you modify Ze and let users interact with that modified version over a network, the AGPL requires you to offer those users the corresponding source. Read the license itself when the distinction matters to a deployment.

**What does the contributor agreement grant?**

A signed-off commit (`git commit -s`) confirms agreement to the [Contributor License Agreement](https://ze-software.net/contribute/). You keep your copyright.

The agreement lets the maintainer offer Ze under additional license terms, including a commercial license. Every contribution remains available to the public under AGPLv3. The [contributor guide](https://ze-software.net/contribute/guide/) covers the submission process.

**Who builds Ze, and how is it funded?**

Thomas Mangin develops Ze, with engineering time supported by [Exa Networks](https://exa.net.uk). The ISP has backed the work since it began with ExaBGP in 2009.

Ze currently has no subscription, paid support tier, or separate commercial entity. The [contribute page](https://ze-software.net/contribute/) explains how the project is maintained and how to help.

**How do I get help or report a security problem?**

Use [Discord](https://discord.gg/T8s7CjPDne) for discussion and the [issue tracker](https://github.com/ze-software/ze/issues) for reproducible bugs and feature requests.

Follow the [security policy](https://ze-software.net/security/) for anything security-sensitive. Report vulnerabilities privately instead of opening a public issue.
