Ze, an OpenNOS

# A network OS.
*A platform for immutable services.*

Self-contained Extend with plugins Appliance option ExaBGP compatible

Ze is built on a [configuration and plugin engine](https://ze-software.net/architecture/), with [pre-installed plugins](https://ze-software.net/reference/plugins/) for [BGP](https://ze-software.net/guides/bgp-peering/), [OSPF](https://ze-software.net/guides/ospf/), [IS-IS](https://ze-software.net/guides/isis/), [VPNs](https://ze-software.net/guides/ipsec/), [interfaces](https://ze-software.net/features/interfaces/), [DNS](https://ze-software.net/features/dns-resolver/), [DHCP](https://ze-software.net/reference/plugins/dhcpserver/) and [other network functions](https://ze-software.net/features/). Its single [YANG-defined configuration](https://ze-software.net/reference/configuration/) can be managed through Ze’s own [SSH server, CLI](https://ze-software.net/features/cli-commands/), [web interface](https://ze-software.net/features/web-interface/), [MCP](https://ze-software.net/features/mcp-integration/) and [APIs](https://ze-software.net/features/api-commands/). You can [develop your own plugins in any programming language](https://ze-software.net/developers/plugins/) and run Ze as a [Linux daemon](https://ze-software.net/guides/ubuntu-build-install/) or an [immutable OS](https://ze-software.net/guides/appliance/).

[Why Ze exists →](https://ze-software.net/project/why-ze/) · Ask questions and follow development on [Discord](https://discord.gg/T8s7CjPDne).

 [Try the BGP quickstart](https://ze-software.net/guides/quickstart/) [Watch the demos](https://ze-software.net/demos/terminal/)

Linux service or immutable appliance. No stable release yet.

**Ze / showcase.terminal**[Read transcript](https://ze-software.net/demos/terminal/#continuous-live-configuration-showcase) **One running router, eight live configuration topics**

Watch ExaBGP migration, inspect BGP peers, then configure, diagnose and repair services without restarting Ze. The transcript includes the commands and their output.

 [Watch the other demos →](https://ze-software.net/demos/terminal/)

Enlarged preview from the recording. Play shows the full terminal. Use full screen for a closer look.

**Ze / Looking glass**[Open full size ↗](https://ze-software.net/assets/product-routes.webp) [![Ze looking glass with prefix, AS path and community filters, a route graph and route table](https://ze-software.net/assets/product-routes.webp)](https://ze-software.net/assets/product-routes.webp) **Follow a route through the network.**

Inspect prefixes, AS paths and peers through a read-only BGP interface.

 [Explore the looking glass →](https://ze-software.net/features/looking-glass/)

Captured for the June 2026 LINX talk.

**Ze / Web configuration**[Open full size ↗](https://ze-software.net/assets/demos/web-config.webp?v=dd251ea02c) [![Ze web configuration showing the system identity form with the committed hostname edge-demo](https://ze-software.net/assets/demos/web-config.webp?v=dd251ea02c)](https://ze-software.net/assets/demos/web-config.webp?v=dd251ea02c) **Configure Ze in your browser.**

Edit a setting, review the changes, apply them and check the active value.

 [Explore the web interface →](https://ze-software.net/features/web-interface/#browser-configuration-workflow)

Captured after the hostname commit in an isolated local Ze lab. [Read the transcript.](https://ze-software.net/assets/demos/web-config.txt?v=a614767cf2)

**Ze / CLI configuration editor**[Open full size ↗](https://ze-software.net/assets/product-editor.webp) [![Ze configuration editor showing a transit peer, a pending remote ASN edit and contextual connection completions](https://ze-software.net/assets/product-editor.webp)](https://ze-software.net/assets/product-editor.webp) **Edit the configuration with context.**

Review the pending change and explore the settings available at the current configuration path.

 [Read about configuration →](https://ze-software.net/features/bgp-configuration/)

Captured from a local build on 23 October 2026 with an isolated lab configuration. The illustrated edit was not committed.

**Ze / CLI dashboard**[Open full size ↗](https://ze-software.net/assets/product-dashboard.webp) [![Ze terminal dashboard showing three established BGP peers, uptime and message counters](https://ze-software.net/assets/product-dashboard.webp)](https://ze-software.net/assets/product-dashboard.webp) **Keep the peers in view.**

Inspect BGP sessions from Ze’s own CLI.

 [Explore CLI interaction →](https://ze-software.net/features/cli-commands/)

A frame from the published terminal recording. Explore the recordings in the demo gallery.

 [![Zeledon, the Ze bird mascot](https://ze-software.net/assets/zeledon.svg)](https://ze-software.net/zeledon/)

**Planned release** Initial version
Q4 2026

 [**1.69 million** lines of Go source code](https://ze-software.net/project/activity/) [**2.01 million** lines of Go test code](https://ze-software.net/project/activity/) [**2.00 million** lines of third-party Go code](https://ze-software.net/project/activity/) [Core shared by all protocols](https://ze-software.net/architecture/) [Configuration models in YANG](https://ze-software.net/reference/configuration/) [BGP, interfaces, FIB](https://ze-software.net/features/#routing) [CLI, SSH, web, API, MCP](https://ze-software.net/features/ai-first/) [Compiled or external plugins](https://ze-software.net/reference/plugins/) [AGPLv3 source](https://ze-software.net/license/)

## Getting started with Ze.

Run the two-peer BGP lab by hand, or ask Claude Code or Codex to run it with Ze’s bundled skill. Both paths start on a Linux lab host with Go and Git.

```
# build from source
$ git clone https://github.com/ze-software/ze.git
$ cd ze && ./ze version

# copy the supplied local lab configuration
$ mkdir -p lab/r1 lab/r2
$ cp examples/quickstart/r1.conf lab/r1/ze.conf
$ cp examples/quickstart/r2.conf lab/r2/ze.conf

# one peer in each terminal
$ ./ze start lab/r1/ze.conf
$ ./ze start lab/r2/ze.conf

# inspect received routes from a third terminal
$ ZE_SSH_PASSWORD=lab ./ze cli --remote 127.0.0.1:2221 --user lab -c "show bgp rib | received"
```

```
# build from source
$ git clone https://github.com/ze-software/ze.git
$ cd ze && ./ze version

# make ze available to your agent
$ export PATH="$PWD:$PATH"
$ ze create agent skill
$ ze show agent skill

# start Claude Code or Codex from this shell
# in Codex, trust Ze's hook with /hooks
```

```
Use the ze skill and docs/guide/quickstart.md
to run the local two-peer lab.

Validate both configurations, start both peers,
and verify that the session is established.
Check that r1 receives 198.51.100.0/24 and
r2 receives 192.0.2.0/24. Show me the results.

Stop only the lab processes you started.
Keep the logs if a check fails.
```

## Why Ze?

Ze brings BGP, Linux interfaces and forwarding-table management together with network services and plugins in one open-source platform. Use the same configuration model and tools in a lab, as a daemon or as an appliance, with routing policy data, reference pages generated from the code and test results available alongside them.

### Manage Ze directly.

Ze runs its own SSH server, CLI and configuration editor. Connect directly to configure and operate it, without a host SSH service or shell. The web interface, REST, gRPC, gNMI and MCP expose the same YANG-modeled configuration.

 Ze SSH + CLIWebAPIs + MCP **YANG-modeled configuration** Routing · Interfaces · Network services [See the browser workflow →](https://ze-software.net/features/web-interface/) [Explore CLI interaction →](https://ze-software.net/features/cli-commands/)

### From connection to configuration

Change a setting through Ze’s SSH editor, review the diff, commit it and check the result.

1. Connect
2. Edit
3. Review
4. Commit

Playback starts at the SSH introduction (0:36) in the full recorded session. Use full screen for a closer look.

 [Full recording and quickstart →](https://ze-software.net/guides/quickstart/#initialize) [Read transcript →](https://ze-software.net/assets/demos/zefs-config.txt?v=586e5c6ada)

**ExaBGP**

### [ExaBGP on steroids](https://ze-software.net/features/exabgp-compatibility/)

Used as a controller, Ze behaves like ExaBGP: your scripts run unchanged and the host's routing table is left alone. Its native API offers much more: a program written in any language can filter each route (accept, reject or rewrite it), add its own commands and YANG configuration, and report problems through `ze doctor`.

**Peering**

### [Made for operators](https://ze-software.net/guides/irr-filtering/)

Give Ze a peer's ASN and it finds the AS-SET in PeeringDB, builds the IRR prefix filter and keeps it up to date, so no bgpq4 job is needed. RPKI validation and a looking glass with a Birdwatcher-compatible API are built in, and the CLI completes every command and explains what is wrong.

**Deployment**

### [Run it the way you want](https://ze-software.net/guides/ze-install/)

One binary, four ways to run it: a daemon on the Linux host you already have, a dedicated appliance image for a VM or mini PC, a Docker container, or a node in a containerlab topology. The configuration and the commands stay the same.

**AI**

### [Made for AI assistants](https://ze-software.net/guides/ai-agent/)

Connect an assistant to Ze's MCP server and describe the symptom. It runs `ze doctor`, reads peer, route and interface state from the live daemon, and explains the cause, using a reference built from that binary.

 Evidence for your decision

## Tests that catch bugs

Go tests, complete operator scenarios, generated inputs and exchanges with other implementations cover different kinds of failure. These figures show the inventory and the evidence behind it.

 By use case

### Start with the role you have in mind.

Inspect the requirements and remaining gaps for your network role, alongside the feature maturity information.

 [See completion by use case →](https://ze-software.net/quality/rfc-compliance/profiles/) [How Ze is tested](https://ze-software.net/quality/)Watch the configuration demo Standards requirements

### Mandatory RFC requirements proven

45.7%

2,172 of 4,752 requirements for Ze or the OS.

**196**RFCs implemented **0**unreviewed requirement lists excluded

Each test identifies the requirement it checks. The ledger names the evidence and missing coverage.

**Counting history**

Until 2026-10-31: all mandatory rows counted, including annotated rows; summary rows excluded.

 [Standards requirements and test results →](https://ze-software.net/quality/rfc-compliance/) Tests with other implementations

### Interoperability scenarios

411

BGP sessions, routes, policy and recovery, alongside routing, VPN, access and multi-node cases.

**8**lab suites **13**test images listed BIRDFRRGoBGPOpenBGPD freeRouterRustyBGPrustbgpdStayRTR keepaliveddnsmasqKeaKnot DNSUnbound

These test images are reused across many scenarios. Other labs and tools use additional images, including pmacct.

A scenario can contain many checks, and many scenarios reuse the same peer implementation. These counts describe the inventory, not current passing runs or individual assertions.

 [Docker scenarios and peer checks →](https://ze-software.net/quality/qemu-interop-release/#docker-interop) The scenarios behind the figure

### Where the 411 scenarios are defined.

 [Explore the BGP lab →](https://ze-software.net/labs/bgp-interop/)

| Protocol or test area | Scenarios | What the cases cover |
| --- | --- | --- |
| BGP | 151 | Route exchange, path attributes, graceful restart, route reflection, route-server and session cases. |
| OSPFv2 | 36 | Adjacencies, authentication, areas, LSAs, route policy and recovery. |
| OSPFv3 | 20 | IPv6 adjacencies, address families, IPv4 over OSPFv3, IPsec protection and recovery. |
| IS-IS | 16 | Adjacencies, authentication, levels, metrics, route policy and convergence. |
| BFD | 5 | Sessions, link-local peers and simple-password authentication. |
| BMP | 4 | Route monitoring, local-RIB export, receivers and statistics. |
| RPKI / RTR | 3 | Route-origin validation and validator-cache sessions and preference. |
| VRRP | 2 | Mastership and VRRPv2 address-owner cases. |
| DNS | 2 | Resolver forwarding and DNS service for DHCP clients. |
| DHCPv4 | 3 | Renewal, rebinding, NAK handling and shared-segment server cases. |
| DHCPv6 | 2 | Client confirmation and renewal against Kea. |
| LDP | 3 | Session backoff, unknown TLVs and keepalive expiry. |
| AS112 | 6 | Anycast route announcements, communities, origin attributes and redistribution. |
| GTSM | 1 | TTL protection for ICMP errors related to a protected BGP session. |
| IPsec | 85 | Tunnel negotiation, authentication and recovery scenarios. |
| L2TP | 15 | Access tunnels, sessions and PPP cases. |
| PPPoE | 16 | Client and access-session scenarios. |
| RADIUS | 12 | Operator login, authorization, subscriber accounting, CoA and disconnect cases. |
| RSVP-TE | 10 | Transit, label and signaling scenarios. |
| Multi-node topologies | 18 | Links, routes and service interactions between nodes. |
| Other protocols | 1 | Scenarios whose primary protocol is outside the named rows. |
| All scenarios | 411 | Each scenario counts toward one category only. If it tests several protocols, we group it only under the main one it tests. |

 Package behavior

### Go tests

47,400+

Check encoded bytes and parsed values. Verify configuration, state transitions and plugins. Mutation checks with gomu examine whether tests catch code changes.

[Go tests and mutation checks →](https://ze-software.net/quality/unit-fuzz-mutation/) Visible behavior

### End-to-end tests

3,000+

Start sessions and exchange route updates. Run editor commands, commits and reloads. Check output, errors and saved state.

[Scenarios and expected results →](https://ze-software.net/quality/functional-ci/) Generated inputs

### Fuzz targets

110

Generate inputs for parsers and decoders. Probe packets and configuration text for crashes. Replay saved failures as regression cases.

[Fuzz inputs and regression cases →](https://ze-software.net/quality/unit-fuzz-mutation/#fuzz-targets-are-still-tests)

Ze is written with AI agents working under a quality process based on ISO management systems. [How Ze is developed with AI](https://ze-software.net/project/ai-development/) explains the process.

 Tested against **BGP**[FRR](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[BIRD](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[GoBGP](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[OpenBGPd](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[FreeRtr](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[RustyBGP](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[rustbgpd](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[ExaBGP](https://ze-software.net/features/exabgp-compatibility/) **BGP support**[StayRTR](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[pmacct](https://ze-software.net/quality/qemu-interop-release/#docker-interop) **VPN**[strongSwan](https://ze-software.net/quality/qemu-interop-release/#docker-interop) **Access**[accel-ppp](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[xl2tpd](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[FreeRADIUS](https://ze-software.net/quality/qemu-interop-release/#docker-interop) **Redundancy**[keepalived](https://ze-software.net/quality/qemu-interop-release/#docker-interop)

## Capabilities and current limitations.

Feature pages explain behavior and limits. The [maturity record](https://ze-software.net/reference/feature-status/) separates available code from the evidence still needed before a feature can be recommended for production.

**Configuration**

**The YANG tree** Settings for the documented build, including plugins →

[Open page](https://ze-software.net/reference/configuration/)
**Commands**

**513 CLI commands** Syntax, arguments and pipe operators →

[Open page](https://ze-software.net/reference/cli/)
**Plugins**

**98 registered plugins** Purpose, dependencies, configuration roots and source →

[Open page](https://ze-software.net/reference/plugins/)

The configuration and command references come from the definitions used by Ze itself. When those definitions change, the next website build updates the reference, including settings supplied by plugins.

### Explore features by category

 [Operate (6)](https://ze-software.net/features/#operate) [Routing (9)](https://ze-software.net/features/#routing) [Services (9)](https://ze-software.net/features/#services) [Automate (7)](https://ze-software.net/features/#automate) [Observe (9)](https://ze-software.net/features/#observe) [Secure (6)](https://ze-software.net/features/#secure) [Setup (6)](https://ze-software.net/features/#platform)

### Operate

 [Output Formatting](https://ze-software.net/features/formatting/) [Web Workbench](https://ze-software.net/features/web-interface/) [Looking Glass](https://ze-software.net/features/looking-glass/) [SSH CLI](https://ze-software.net/features/cli-commands/) [YANG Configuration](https://ze-software.net/features/bgp-configuration/) [System Readiness](https://ze-software.net/guides/production-diagnostics/) [View operate features →](https://ze-software.net/features/#operate)

### Routing

 [MPLS / LDP / RSVP-TE](https://ze-software.net/features/) [OSPFv2 / OSPFv3](https://ze-software.net/guides/ospf/) [IS-IS](https://ze-software.net/guides/isis/) [VRRP](https://ze-software.net/guides/vrrp/) [Segment Routing](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy) [Native BGP Engine](https://ze-software.net/features/bgp-protocol/) [Static Routes](https://ze-software.net/guides/static-routes/) [BFD](https://ze-software.net/features/bgp-protocol/) [MRT Recording](https://ze-software.net/guides/mrt-analysis/) [View routing features →](https://ze-software.net/features/#routing)

### Services

 [IPsec VPN](https://ze-software.net/features/) [L2TPv2 BNG](https://ze-software.net/guides/l2tp/) [PPPoE Access](https://ze-software.net/guides/pppoe/) [Interface Management](https://ze-software.net/features/interfaces/) [Firewall](https://ze-software.net/guides/firewall/) [Policy Routing](https://ze-software.net/guides/policy-routing/) [VPP Data Plane](https://ze-software.net/guides/vpp/) [AS112 Anycast DNS](https://ze-software.net/guides/as112/) [DNS Resolver](https://ze-software.net/features/dns-resolver/) [View services features →](https://ze-software.net/features/#services)

### Automate

 [Fleet Management](https://ze-software.net/guides/fleet-config/) [AI Tool Interfaces](https://ze-software.net/features/ai-first/) [Plugin System](https://ze-software.net/reference/plugins/) [Programmable](https://ze-software.net/features/api-commands/) [AI-First Design](https://ze-software.net/features/ai-first/) [MCP Integration](https://ze-software.net/features/mcp-integration/) [ExaBGP Compatibility](https://ze-software.net/features/exabgp-compatibility/) [View automate features →](https://ze-software.net/features/#automate)

### Observe

 [Development Activity](https://ze-software.net/project/activity/) [Tech-Support Bundle](https://ze-software.net/features/) [Flow Export](https://ze-software.net/guides/flow-export/) [Evidence Over Claims](https://ze-software.net/features/interoperability-testing/) [Prometheus Telemetry](https://ze-software.net/guides/monitoring/) [Health Registry](https://ze-software.net/features/) [Host Inventory](https://ze-software.net/features/) [Crash Capture](https://ze-software.net/features/) [Production Diagnostics](https://ze-software.net/guides/production-diagnostics/) [View observe features →](https://ze-software.net/features/#observe)

### Secure

 [Audit Trail](https://ze-software.net/guides/audit/) [PKI Store](https://ze-software.net/features/) [DDoS and Anomaly Detection](https://ze-software.net/guides/anomaly/) [IRR Route Filtering](https://ze-software.net/guides/irr-filtering/) [Secure by Default](https://ze-software.net/reference/plugins/) [TACACS+ AAA](https://ze-software.net/guides/tacacs/) [View secure features →](https://ze-software.net/features/#secure)

### Setup

 [ISO and PXE Install](https://ze-software.net/guides/ze-install/) [Kernel Tunables](https://ze-software.net/features/) [Minimal Appliance Mode](https://ze-software.net/guides/appliance/) [Runs Itself](https://ze-software.net/features/introspection/) [Docker Support](https://ze-software.net/features/) [Choose Features at Build Time](https://ze-software.net/guides/quickstart/) [View setup features →](https://ze-software.net/features/#platform)

**Find the right page**

### Keep the reference close.

The documentation index groups guides by the job you want to do. The feature catalogue includes maturity filters.

 [Open documentation →](https://ze-software.net/docs/) [Browse the feature catalogue →](https://ze-software.net/features/)

## Choose how to try Ze.

A local BGP lab is the simplest starting point. If you already run ExaBGP or want a different deployment, follow the path that matches your setup.

 Local labStart here

### Exchange your first routes.

Build Ze, start two peers on one Linux host, and inspect the route each receives. The tutorial includes the configuration, verification and cleanup.

**Success check**Both sessions establish and each peer receives the other peer’s advertised prefix.

 [Open the two-peer quickstart](https://ze-software.net/guides/quickstart/) ExaBGP migration

### Try an existing configuration.

Convert an ExaBGP configuration, review the result, and use the compatibility bridge for process scripts. The guide explains the limits and cutover checks.

**Success check**Validate the converted configuration and verify your sessions and script behavior in a lab.

 [Explore migration](https://ze-software.net/use-cases/exabgp-migration/) Linux or Docker

### Keep your existing host.

Install the Linux service or build the Docker deployment image with persistent storage. The guides cover authenticated management, updates and removal; the Linux host keeps its existing interface configuration.

**Success check**Connect to authenticated management and check that your configuration survives the documented restart or container recreation.

 [Linux service →](https://ze-software.net/guides/ubuntu-build-install/)[Docker →](https://ze-software.net/guides/docker/) VM or appliance

### Give Ze its own system.

Build an immutable gokrazy system for QEMU, Proxmox or hardware, with a read-only root and persistent configuration. The guide covers ISO installation, authenticated SSH, updates and previous-slot recovery.

**Availability**Source builds produce disk images and installer ISOs. No public image release is available yet.

 [Explore the appliance](https://ze-software.net/guides/appliance/) **Other ways to evaluate** [netlab and containerlab](https://ze-software.net/guides/netlab/) [Explore a supplied topology](https://ze-software.net/labs/netlab/) [BGP interoperability labs](https://ze-software.net/labs/bgp-interop/) [Test with an AI agent](https://ze-software.net/guides/ai-agent/) [Explore the CLI](https://ze-software.net/reference/cli/) [Publish a looking glass](https://ze-software.net/guides/public-looking-glass/) [Compare all deployment paths →](https://ze-software.net/guides/ze-install/#choose-a-deployment-path)

The quickstart builds from source and needs Go and Git. It checks the BGP control plane; receiving routes does not establish forwarding connectivity. For a live topology that also checks kernel routes and traffic, use the netlab guide.

## Recent engineering notes.

The update reports describe changes recorded in git and shared in Discord's `ze-news` channel.

**Update**

 01

2026-10-25 – 2026-11-09 (16 days)

### [eBGP now rejects by default, routing policy is one shared tree for every protocol, RADIUS answers must be…](https://ze-software.net/project/changes/2026-10-25/)

 BGP CPE gNMI Config

**Update**

 02

2026-10-12 – 2026-10-25 (2 weeks)

### [Two weeks of stricter configuration, an IPsec drop-by-default policy and BGP filter changes, plus BGP…](https://ze-software.net/project/changes/2026-10-12/)

 BGP DHCP Config API

**Update**

 03

2026-10-05 – 2026-10-11 (1 week)

### [Safer configuration commits, API changes and a new appliance kernel, plus BGP, RSVP-TE and IPsec fixes.](https://ze-software.net/project/changes/2026-10-05/)

 BGP IPsec Config API - [See all updates](https://ze-software.net/project/changes/)
