# Frequently asked questions

The questions people tend to ask before they spend time on Ze. If yours is not here, ask on [Discord](https://discord.gg/T8s7CjPDne) or open an [issue](https://github.com/ze-software/ze/issues).

**What is Ze?**

Ze is an open, programmable network OS for Linux. It has a native BGP, OSPF, and IS-IS engine, it manages interfaces and programs the FIB, and it wraps all of that in one operator surface: an SSH CLI, a web UI, telemetry, a looking glass, an API, and a plugin system, all driven by a single YANG configuration model.

You can run it as a daemon on an existing distribution or build it into a dedicated appliance from the same binary and the same config.

**Is Ze ready for production?**

Not yet, and the site says so everywhere on purpose. The routing core is heavily tested, but operational mileage is still limited and the configuration syntax can still change before the first release.

The honest place for Ze today is a lab: build a route server, migrate an ExaBGP config, stand up a looking glass, run the interop labs against real FRR or BIRD, and tell us where it breaks. The [roadmap](https://ze-software.net/roadmap/) explains what stands between here and a release you can run in anger.

**Why would I use Ze instead of BIRD, FRR, or GoBGP?**

Those are mature, and Ze does not pretend otherwise. The [comparison page](https://ze-software.net/compare/) is blunt about where they are still ahead.

What Ze offers that they do not is a single design where the BGP engine, the configuration model, the plugins, and the operator tooling were built together: one YANG model for everything, a plugin architecture for extending the daemon, an SSH CLI with diff and commit, and an MCP server so an AI assistant can help you debug live state.

**What can Ze actually do today?**

The BGP implementation is broad: IPv4 and IPv6 unicast, labeled unicast, VPNv4 and VPNv6, EVPN, FlowSpec, add-path, graceful restart in several flavours, RPKI, route reflection, and a growing list of families and capabilities.

OSPFv2, OSPFv3, IS-IS, and MPLS are in the core. Around that sit a firewall, VPN, PPPoE and L2TP access concentration, flow export, and appliance packaging. The [features page](https://ze-software.net/features/) marks each capability by status, and the [comparison page](https://ze-software.net/compare/) puts every protocol feature next to the other daemons.

**I run ExaBGP. Can I move to Ze?**

That is one of the paths Ze is built for. Ze aims for an easy migration from ExaBGP rather than perfect compatibility.

There is a config converter (`ze exabgp migrate`) and a compatibility bridge that lets existing ExaBGP process scripts run with Ze as the engine while you port them over. The [ExaBGP migration usage example](https://ze-software.net/usage/exabgp-migration/) walks through the conversion, the known differences, and when it is worth rewriting a plugin against the native Ze SDK.

**What license is Ze under, and what does that mean for me?**

Ze is free software under the [GNU Affero General Public License v3](https://ze-software.net/license/). You can run it, read it, modify it, and redistribute it.

The AGPL adds one obligation beyond the GPL: if you offer a modified Ze to others over a network, you have to make your modified source available to those users. Running an unmodified Ze to route your own traffic carries no such obligation.

**I want to contribute. What is the CLA about?**

Contributions require a signed-off commit (`git commit -s`), which certifies agreement to the [Contributor License Agreement](https://ze-software.net/contribute/). You keep your copyright.

What you grant is a broad license that lets the maintainer relicense the project, for example to offer a commercial license alongside the AGPL if that ever helps Ze reach more people. Ze stays AGPLv3 for everyone regardless. The [contributor guide](https://ze-software.net/contribute/guide/) covers the rest of the process.

**The daemon or the appliance: which should I run?**

The same binary and the same config drive both. Run it as a **daemon** when you are fitting Ze into infrastructure you already operate under systemd or another process manager.

Build it as an **appliance** when you want a purpose-built box: a read-only root filesystem, no shell, no package manager, and automatic supervision, produced with gokrazy. Start with the daemon if you are unsure.

**Does the AI and MCP support mean Ze needs an LLM to run?**

No. Ze is a routing daemon and runs entirely on its own. The MCP server is an optional surface that lets an AI assistant read Ze's state and help you debug when you choose to use one. Nothing in the data path depends on it.

**Will my configuration keep working as Ze changes?**

Until the first release, treat the configuration syntax as not yet frozen. Breaking changes are called out in the [changes log](https://ze-software.net/changes/), and the policy is no silent breakage: a change that affects your config should come with either an automatic migration or a clear error.

Stabilising the syntax so it stays stable is an explicit milestone on the [roadmap](https://ze-software.net/roadmap/).

**Who builds Ze, and how is it funded?**

Ze is developed by Thomas Mangin, with his time supported by [Exa Networks](https://exa.net.uk), the ISP that has backed this work since 2009 when it began with ExaBGP.

There is no subscription, no paid support tier, and no separate commercial entity today. The [contribute page](https://ze-software.net/contribute/) has the detail.

**How do I get help, or report a security problem?**

For questions and discussion, use [Discord](https://discord.gg/T8s7CjPDne) or the [issue tracker](https://github.com/ze-software/ze/issues).

For anything security-sensitive, such as a bug an unauthenticated peer could trigger, follow the [security policy](https://ze-software.net/security/) and report it privately instead of opening a public issue.
