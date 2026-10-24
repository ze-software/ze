Ze, an OpenNOS

# A network OS.
*A platform for immutable services.*

Plugin extensible Appliance option ExaBGP compatible

Extend Ze with plugins for networking, automation and services of your own.

Ze speaks [**BGP**](https://ze-software.net/guides/bgp-peering/), [**ISIS**](https://ze-software.net/guides/isis/), [**OSPF**](https://ze-software.net/guides/ospf/), manages [**interfaces**](https://ze-software.net/features/interfaces/) and tunnels ([**IPsec VPN**](https://ze-software.net/guides/ipsec/), [**WireGuard**](https://ze-software.net/features/interfaces/#wireguard-configuration)), programs the [**FIB**](https://ze-software.net/reference/plugins/fib-kernel/), and exposes one [**YANG**](https://ze-software.net/features/bgp-configuration/)-modeled configuration through its own built-in SSH server, a [**web**](https://ze-software.net/features/web-interface/) interface, [**APIs**](https://ze-software.net/features/api-commands/) ([**REST**](https://ze-software.net/guides/api/#rest-endpoints), [**gRPC**](https://ze-software.net/guides/api/#grpc-services), [**gNMI**](https://ze-software.net/guides/gnmi/)), [**CLI**](https://ze-software.net/features/cli-commands/), and [**MCP**](https://ze-software.net/features/mcp-integration/).

It is built on a configuration and protocol engine that runs as a [**daemon**](https://ze-software.net/architecture/) or [**immutable appliance**](https://ze-software.net/guides/appliance/).

 [Watch the demos](https://ze-software.net/demos/terminal/) [Try the BGP quickstart](https://ze-software.net/guides/quickstart/) **Ze / Looking glass**[Open full size ↗](https://ze-software.net/assets/product-routes.png) [![Ze looking glass with prefix, AS path and community filters, a route graph and route table](https://ze-software.net/assets/product-routes.png)](https://ze-software.net/assets/product-routes.png) **Follow a route through the network.**

Inspect prefixes, AS paths and peers through a read-only BGP interface.

 [Explore the looking glass →](https://ze-software.net/features/looking-glass/)

Captured for the June 2026 LINX talk.

**Ze / Web configuration**[Open full size ↗](https://ze-software.net/assets/demos/web-config.png?v=0562bbf22f) [Download the browser recording](https://ze-software.net/assets/demos/web-config.webm?v=e49e84d93c). **Configure Ze in your browser.**

Edit a setting, review its diff, commit the draft and verify the active value.

 [Explore the web interface →](https://ze-software.net/features/web-interface/#browser-configuration-workflow)

A real session with an isolated local Ze lab. [Read the transcript.](https://ze-software.net/assets/demos/web-config.txt?v=a614767cf2)

**Ze / CLI configuration editor**[Open full size ↗](https://ze-software.net/assets/product-editor.jpg) [![Ze configuration editor showing a transit peer, a pending remote ASN edit and contextual connection completions](https://ze-software.net/assets/product-editor.jpg)](https://ze-software.net/assets/product-editor.jpg) **Edit the configuration with context.**

Review the pending change and explore the settings available at the current configuration path.

 [Read about configuration →](https://ze-software.net/features/bgp-configuration/)

Captured from a local build on 23 October 2026 with an isolated lab configuration. The illustrated edit was not committed.

**Ze / CLI dashboard**[Open full size ↗](https://ze-software.net/assets/product-dashboard.png) [![Ze terminal dashboard showing three established BGP peers, uptime and message counters](https://ze-software.net/assets/product-dashboard.png)](https://ze-software.net/assets/product-dashboard.png) **Keep the peers in view.**

Inspect BGP sessions from Ze’s own CLI.

 [Explore CLI interaction →](https://ze-software.net/features/cli-commands/)

A frame from the published terminal recording. Replay the demonstration below.

 [**Why Ze exists** Learn how Ze came to be.](https://ze-software.net/project/why-ze/) [**Watch a demo** Discover the web interface.](https://ze-software.net/features/web-interface/#browser-configuration-workflow) [**Search the site**Docs and commands.](https://ze-software.net/search/) [**Join Discord**Ask questions, get help.](https://discord.gg/T8s7CjPDne) [![Zeledon, the Ze bird mascot](https://ze-software.net/assets/zeledon.svg)](https://ze-software.net/zeledon/)

Expected initial release: Q4 2026.
 No stable release yet.

 [Protocol-agnostic core](https://ze-software.net/architecture/) [YANG per subsystem](https://ze-software.net/reference/configuration/) [BGP, interfaces, FIB](https://ze-software.net/features/#routing) [CLI, SSH, web, API, MCP](https://ze-software.net/features/ai-first/) [Compiled or external plugins](https://ze-software.net/reference/plugins/) [AGPLv3 source](https://ze-software.net/license/)

## Latest news

 [All updates](https://ze-software.net/project/changes/) Engineering note

### [Keeping documentation in step with the code](https://ze-software.net/blog/reference-from-the-system/)

Ze generates command, configuration and support reference data from the code and its records, so the…

 Recently shipped

### [Week of 2026-10-05](https://ze-software.net/project/changes/2026-10-05/)

A week on configuration safety, the public API and the appliance kernel, with fixes across BGP, RSVP-TE and…

 RFC compliance progress

### [Every MUST-level requirement, and what proves it](https://ze-software.net/quality/rfc-compliance/)

One page per RFC, naming each requirement, the test evidence behind it, and the ones that have none.

## Code quality, in numbers

Explore the tests behind Ze: unit tests, end-to-end scenarios, protocol checks, fuzzing and interoperability with other implementations. Each figure links to the evidence and its current coverage.

 [Read the evidence map](https://ze-software.net/quality/) [Watch product demos](https://ze-software.net/demos/terminal/) [**38,300+ unit tests**

- Wire encoding, parsing
- Config, FSM, plugins
- gomu mutates code to check assertions

 Local test, fuzz, and mutation evidence.](https://ze-software.net/quality/unit-fuzz-mutation/) [**2,500+ end to end tests**

- Peering, sessions, updates
- Editor, commits, reloads
- Commands checked as operators run them

 Functional transcript format and rerun path.](https://ze-software.net/quality/functional-ci/) [**154 RFCs supported**

- 4,163 MUSTs to test
- 2,758 checked with a test
- 66.3% done

 RFC requirement ledger.](https://ze-software.net/quality/rfc-compliance/) [**91 fuzz targets**

- Parsers, external inputs
- Wire formats, config files
- Saved crashes become regression cases

 Fuzz crashes kept as regression cases.](https://ze-software.net/quality/unit-fuzz-mutation/#fuzz-targets-are-still-tests) [**11 interop targets**

- BGP, OSPF, IS-IS, BFD, BMP, RPKI, VRRP
- IPsec, L2TP, PPPoE, RADIUS
- Real daemons in Docker, checked by peer CLIs

 Docker interop peer list.](https://ze-software.net/quality/qemu-interop-release/#docker-interop) Tested against **BGP**[FRR](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[BIRD](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[GoBGP](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[OpenBGPd](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[FreeRtr](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[RustyBGP](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[rustbgpd](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[ExaBGP](https://ze-software.net/features/exabgp-compatibility/) **BGP support**[StayRTR](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[pmacct](https://ze-software.net/quality/qemu-interop-release/#docker-interop) **VPN**[strongSwan](https://ze-software.net/quality/qemu-interop-release/#docker-interop) **Access**[accel-ppp](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[xl2tpd](https://ze-software.net/quality/qemu-interop-release/#docker-interop)[FreeRADIUS](https://ze-software.net/quality/qemu-interop-release/#docker-interop) **Redundancy**[keepalived](https://ze-software.net/quality/qemu-interop-release/#docker-interop)

## Why Ze?

Ze brings BGP, Linux interfaces and FIB programming together with network services and plugins in one open-source platform. Use the same configuration model and tools in a lab, as a daemon or as an appliance, with policy data, generated references and test evidence available alongside them.

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

**Decision**

### [Reasons to consider Ze](https://ze-software.net/project/why-ze/)

Start here for the selling points: powerful CLI, operator protocols, in-engine IRR filtering, PeeringDB data, looking glass APIs, one YANG model, and when to choose another NOS instead.

**Runtime**

### [Small core, registered subsystems](https://ze-software.net/architecture/)

The core holds the supervisor, message bus, config provider, and plugin manager. BGP and interface management register into it.

**Routing**

### [Network OS built on the engine](https://ze-software.net/architecture/)

The shipped daemon speaks BGP, manages Linux interfaces, programs the FIB, and serves its configuration through SSH and the web UI.

**Modular**

### [Plugins keep their own contract](https://ze-software.net/reference/plugins/)

Plugins can be compiled Go modules or external processes. Compiled modules load their YANG into the daemon validator; external plugins can expose their model through `ze schema`.

## Documentation that matches the code.

Read the generated references before you run Ze.

The same YANG schema and command definitions drive Ze and its references. Each website build brings changes into the documentation, including configuration supplied by plugins.

**Configuration**

### The full YANG tree

Browse every configuration setting in the documented build, including the settings supplied by plugins.

 [Configuration reference](https://ze-software.net/reference/configuration/)

**Commands**

### Command syntax and help

Find commands, arguments and pipe operators from the same definitions that supply Ze's built-in help.

 [CLI reference](https://ze-software.net/reference/cli/)

**Plugins**

### Plugins and dependencies

Explore registered plugins, their dependencies, configuration roots and the source that implements them.

 [Plugin catalogue](https://ze-software.net/reference/plugins/)

### Explore features by category

 [Operate (6)](https://ze-software.net/features/#operate) [Routing (9)](https://ze-software.net/features/#routing) [Services (9)](https://ze-software.net/features/#services) [Automate (7)](https://ze-software.net/features/#automate) [Observe (9)](https://ze-software.net/features/#observe) [Secure (6)](https://ze-software.net/features/#secure) [Setup (6)](https://ze-software.net/features/#platform)

Open a feature for its behavior and limitations, and [check its maturity](https://ze-software.net/reference/feature-status/) before choosing a deployment.

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

 [ISO and PXE Install](https://ze-software.net/guides/ze-install/) [Kernel Tunables](https://ze-software.net/features/) [Minimal Appliance Mode](https://ze-software.net/guides/appliance/) [Runs Itself](https://ze-software.net/features/introspection/) [Docker Support](https://ze-software.net/features/) [Feature Gates](https://ze-software.net/guides/quickstart/) [View setup features →](https://ze-software.net/features/#platform)

## Run it as a lab,
daemon, or appliance.

The same binary and configuration support each path, from a [netlab topology](https://ze-software.net/labs/netlab/) or BGP interop lab to spare hardware.

**Lab**

### [Run reproducible labs](https://ze-software.net/labs/bgp-interop/)

The netlab integration brings up a three-node Ze topology under containerlab. The BGP interop lab also runs Ze beside FRR, BIRD, and GoBGP and checks routes through each peer's own CLI.

`netlab` `containerlab` `Docker` `FRR` `BIRD` `GoBGP` Run BGP lab BGP interop

**Daemon**

### [Run as a daemon](https://ze-software.net/guides/quickstart/)

Ze runs on any existing Linux distro, managed by systemd or your chosen process manager. This is the easiest route when Ze has to fit into infrastructure you already run.

`Existing Linux` `systemd-ready` Quickstart two BGP peers

**Appliance**

### [Run as an appliance](https://ze-software.net/guides/ze-install/)

A bootable gokrazy image for appliance hardware: read-only root filesystem, no shell, no package manager, and automatic process supervision. The current amd64 installation ISO is under 150 MB.

`gokrazy image` `Read-only root filesystem` Install guide PXE, ISO, or Ventoy

## Watch Ze at work.

The screenshots introduce the interfaces. This recording shows the terminal in use, with its transcript available alongside it.

**Operate BGP from the live dashboard** Replayable Ze terminal lab [Read transcript](https://ze-software.net/features/cli-commands/#runtime-interaction)

## Safe ways to try Ze before the first release.

Ze is early enough that routing feedback can still change the system. These cards give each reader a low-risk starting point.

**Lab**

### [Lab router](https://ze-software.net/guides/quickstart/)

Network builders can bring up two BGP peers, inspect routes, and check whether Ze's operator tools fit their workflow.

`Quickstart` `BGP peers` Quickstart two peer lab

**Migration**

### [ExaBGP replacement](https://ze-software.net/use-cases/exabgp-migration/)

ExaBGP users can try the migrator against an existing config and see which process scripts still translate cleanly.

`Migration` `Plugins` Migration path config and process scripts

**Appliance**

### [White-box appliance](https://ze-software.net/guides/appliance/)

People with spare x86 hardware can boot the appliance image and test the same configuration model without a general-purpose shell.

`ISO` `PXE` `gokrazy` Appliance guide spare hardware

**Operate**

### [Looking glass](https://ze-software.net/guides/public-looking-glass/)

Operators who need read-only BGP visibility can publish a looking glass and inspect routes without giving shell access.

`Read-only` `BGP visibility` Looking glass read-only BGP

**Interop**

### [Protocol testbed](https://ze-software.net/labs/bgp-interop/)

Protocol implementers can run Docker interop scenarios against FRR, BIRD, and GoBGP, then turn a failure into a test case.

`Interop` `Docker` Interop lab real peer daemons

**MCP**

### [AI-assisted operations](https://ze-software.net/features/ai-first/)

MCP exposes Ze commands and structured output to AI tools without a separate command set.

`MCP` `CLI catalogue` AI via MCP same command catalogue

## First paths for routing feedback.

The BGP lab, ExaBGP migration, and appliance install are good starting points.

```
# build from source
$ git clone https://github.com/ze-software/ze.git
$ cd ze && make build

# set up credentials and configure
$ bin/ze init
$ bin/ze config import router.conf

# start
$ bin/ze start

# from another terminal
$ bin/ze cli -c "show bgp peer list"
$ bin/ze cli -c "monitor event"
```

`Good starting points`

A lab peer, a migrated ExaBGP config, or a looking-glass instance can produce useful reports from people who know routing operations.

- [BGP interop lab FRR, BIRD, and GoBGP in Docker](https://ze-software.net/labs/bgp-interop/)
- [ExaBGP migration try an existing config and process script](https://ze-software.net/use-cases/exabgp-migration/)
- [Appliance install ISO media, PXE provisioning, spare hardware](https://ze-software.net/guides/appliance/)
- [Looking glass publish read-only BGP visibility](https://ze-software.net/guides/public-looking-glass/)
- [AI-assisted operations MCP exposes Ze commands to tools](https://ze-software.net/features/ai-first/)

## Recent engineering notes.

Weekly updates come from git history and Discord's `ze-news`. They stay specific and technical.

**Update**

 01

Week of 2026-10-05

### [Safer configuration commits, API changes and a new appliance kernel, plus BGP, RSVP-TE and IPsec fixes.](https://ze-software.net/project/changes/2026-10-05/)

 BGP IPsec Config API

**Update**

 02

Week of 2026-09-28

### [RFC reviews led to fixes in BGP route selection, graceful restart, BFD, OSPF and LDP.](https://ze-software.net/project/changes/2026-09-28/)

 BGP L2TP Flow Export API

**Update**

 03

Week of 2026-09-21

### [Configuration backup and restore, AIGP and BGP-LS export, and a review of every RFC Ze implements.](https://ze-software.net/project/changes/2026-09-21/)

 Storage Config BGP IPsec - [See all updates](https://ze-software.net/project/changes/)
`Try safely`

## Try Ze before the first release.

Start where a mistake cannot affect a live network.

 [Run a BGP lab](https://ze-software.net/labs/bgp-interop/) [Read the quickstart](https://ze-software.net/guides/quickstart/) - **Release:** Expected Q4 2026. No stable release has shipped yet, and configuration may change.
- **BGP lab:** Exercise Ze against FRR, BIRD, and GoBGP without touching a production router.
- **Migration:** Try the ExaBGP migration path against an existing config before changing automation.
- **Appliance:** Boot Ze on spare hardware with the same binary and configuration model as daemon mode.
- **Source:** Read the code, generated docs, RFC gate, and test evidence before deciding where Ze belongs.
