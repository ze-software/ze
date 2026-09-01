# Ze, an OpenNOS

Plugin extensible Appliance option ExaBGP compatible

Ze speaks [**BGP**](https://ze-software.net/guides/bgp-peering/), [**ISIS**](https://ze-software.net/guides/isis/), [**OSPF**](https://ze-software.net/guides/ospf/), manages [**interfaces**](https://ze-software.net/features/interfaces/) and tunnels ([**IPsec VPN**](https://ze-software.net/guides/ipsec/), [**WireGuard**](https://ze-software.net/features/interfaces/#wireguard-configuration)), programs the [**FIB**](https://ze-software.net/reference/plugins/fib-kernel/), and exposes one [**YANG**](https://ze-software.net/features/bgp-configuration/)-modeled configuration through builtin SSH, a [**web**](https://ze-software.net/features/web-interface/) interface, [**APIs**](https://ze-software.net/features/api-commands/) ([**REST**](https://ze-software.net/guides/api/#rest-endpoints), [**gRPC**](https://ze-software.net/guides/api/#grpc-services), [**gNMI**](https://ze-software.net/guides/gnmi/)), [**CLI**](https://ze-software.net/features/cli-commands/), and [**MCP**](https://ze-software.net/features/mcp-integration/).

It is built on a configuration and protocol engine that runs as a [**daemon**](https://ze-software.net/architecture/) or [**immutable appliance**](https://ze-software.net/guides/appliance/).

**Live BGP dashboard** Replayable Ze terminal lab [Read transcript](https://ze-software.net/demos/terminal/#live-bgp-dashboard) [**Why Ze exists** Learn how Ze came to be.](https://ze-software.net/project/why-ze/) [**Watch a demo** Discover the web interface.](https://ze-software.net/demos/terminal/#web-configuration-commit) [**Search the site**Docs and commands.](https://ze-software.net/search/) [**Join Discord**Ask questions, get help.](https://discord.gg/T8s7CjPDne) [![Zeledon, the Ze bird mascot](https://ze-software.net/assets/zeledon.svg)](https://ze-software.net/zeledon/)

Expected initial release: Q4 2026.
 No stable release yet.

 [Protocol-agnostic core](https://ze-software.net/architecture/) [YANG per subsystem](https://ze-software.net/reference/configuration/) [BGP, interfaces, FIB](https://ze-software.net/features/#routing) [CLI, SSH, web, API, MCP](https://ze-software.net/features/ai-first/) [Compiled or external plugins](https://ze-software.net/reference/plugins/) [AGPLv3 source](https://ze-software.net/license/)

## Latest news

 [All updates](https://ze-software.net/project/changes/) Engineering note

### [Reference stays attached to code](https://ze-software.net/blog/reference-from-the-system/)

Generated references stay tied to code, registries and RFC evidence.

 Recently shipped

### [Week of 2026-08-24](https://ze-software.net/project/changes/2026-08-24/)

Standards closure was the plan for the week. The build and test tooling took it instead: the Makefile and…

 RFC compliance progress

### [Every MUST requirement tied to a test](https://ze-software.net/quality/rfc-compliance/)

Every gated MUST-level requirement links to source text, status, and test evidence.

## Release claims stay checkable.

Every homepage number links to the page where you can inspect the test layer, transcript, peer list, RFC gate, or generated source evidence behind it.

 [Read the evidence map](https://ze-software.net/quality/) [Watch product demos](https://ze-software.net/demos/terminal/) [**27,200+ unit tests**

- Wire encoding, parsing
- Config, FSM, plugins
- gomu mutates code to check assertions

 Local test, fuzz, and mutation evidence.](https://ze-software.net/quality/unit-fuzz-mutation/) [**3,264 RFC MUST checks**

- 181 RFCs inspected
- Gaps disclosed before claims
- Tests tied to requirement IDs

 RFC requirement ledger.](https://ze-software.net/quality/rfc-compliance/) [**1,800+ end to end tests**

- Peering, sessions, updates
- Editor, commits, reloads
- Commands checked as operators run them

 Functional transcript format and rerun path.](https://ze-software.net/quality/functional-ci/) [**79 fuzz targets**

- Parsers, external inputs
- Wire formats, config files
- Saved crashes become regression cases

 Fuzz crashes kept as regression cases.](https://ze-software.net/quality/unit-fuzz-mutation/#fuzz-targets-are-still-tests) [**9 interop targets**

- Real third-party daemons
- BGP sessions in Docker
- Routes checked by peer CLIs

 Docker interop peer list.](https://ze-software.net/quality/qemu-interop-release/#docker-interop) Tested against routing stacks [FRR](https://ze-software.net/quality/qemu-interop-release/#docker-interop) [BIRD](https://ze-software.net/quality/qemu-interop-release/#docker-interop) [GoBGP](https://ze-software.net/quality/qemu-interop-release/#docker-interop) [OpenBGPd](https://ze-software.net/quality/qemu-interop-release/#docker-interop) [FreeRtr](https://ze-software.net/quality/qemu-interop-release/#docker-interop) [RustyBGP](https://ze-software.net/quality/qemu-interop-release/#docker-interop) [rustbgpd](https://ze-software.net/quality/qemu-interop-release/#docker-interop) [ExaBGP migration path](https://ze-software.net/features/exabgp-compatibility/)

## [Why Ze?](https://ze-software.net/project/why-ze/)

Ze is a network operating system, and more: a routing daemon, appliance runtime, lab router, and protocol engine that brings policy data, generated references, APIs, MCP tools, and product evidence into one operator workflow.

**Decision**

### [Reasons to consider Ze](https://ze-software.net/project/why-ze/)

Start here for the selling points: powerful CLI, operator protocols, in-engine IRR filtering, PeeringDB data, looking glass APIs, one YANG model, and when to choose another NOS instead.

**Runtime**

### [Small core, registered subsystems](https://ze-software.net/architecture/)

The core holds the supervisor, message bus, config provider, and plugin manager. BGP and interface management register into it.

**Routing**

### [Network OS built on the engine](https://ze-software.net/architecture/)

The shipped daemon speaks BGP, manages Linux interfaces, programs the FIB, and serves its configuration through SSH and the web UI.

**Honest**

### [Plugins keep their own contract](https://ze-software.net/reference/plugins/)

Plugins can be compiled Go modules or external processes. Compiled modules load their YANG into the daemon validator; external plugins can expose their model through `ze schema`.

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

A bootable gokrazy image for appliance hardware: read-only root filesystem, no shell, no package manager, and automatic process supervision.

`gokrazy image` `Read-only root filesystem` Install guide PXE, ISO, or Ventoy

## Generated references are part of the product.

Read the generated references before you run Ze.

Ze is an open-source configuration and protocol engine. The network operating system built on it speaks BGP, manages Linux interfaces, programs the FIB, and serves the same YANG-modeled configuration through SSH, web, API, and MCP.

 [Operate (6)](https://ze-software.net/features/#operate) [Routing (9)](https://ze-software.net/features/#routing) [Services (9)](https://ze-software.net/features/#services) [Automate (7)](https://ze-software.net/features/#automate) [Observe (9)](https://ze-software.net/features/#observe) [Secure (6)](https://ze-software.net/features/#secure) [Platform (6)](https://ze-software.net/features/#platform)

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

## Safe ways to try Ze before the first release.

Ze is early enough that routing feedback can still change the system. These cards give each reader a low-risk starting point.

**IXP**

### [Route server](https://ze-software.net/use-cases/route-server/)

IXP operators can run the route-server lab first and compare policy behaviour before touching members.

`Peering` `Policy` `Replay` Route server policy and replay

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

**Observe**

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

## Recent engineering notes.

Weekly updates come from git history and Discord's `ze-news`. They stay specific and technical.

**Update**

 01

Week of 2026-08-24

### [Standards closure was the plan for the week. The build and test tooling took it instead: the Makefile and 256 shell and Python scripts are gone, replaced by Go, and that move is still in progress. Around it, output formatting moved off flags and onto the pipe operators, and a TACACS+ authentication bypass was closed.](https://ze-software.net/project/changes/2026-08-24/)

 CLI BGP PPPoE IPsec

**Update**

 02

Week of 2026-08-17

### [The CLI gained a clearer BGP workflow, traffic tools gained history and source-AS context, and IPsec changes now reach running tunnels.](https://ze-software.net/project/changes/2026-08-17/)

 BGP CLI API Flow Export

**Update**

 03

Week of 2026-08-10

### [Web and Looking Glass rewrites, remote-triggered blackholing, dynamic BGP peer repairs, authenticated PPPoE and another standards pass shaped the week.](https://ze-software.net/project/changes/2026-08-10/)

 BGP PPPoE IPsec Config - [See all updates](https://ze-software.net/project/changes/)
`Try safely`

## Try Ze before the first release.

Start where a mistake cannot affect a live network.

 [Run a BGP lab](https://ze-software.net/labs/bgp-interop/) [Read the quickstart](https://ze-software.net/guides/quickstart/) - **Release:** Expected Q4 2026. No stable release has shipped yet, and configuration may change.
- **BGP lab:** Exercise Ze against FRR, BIRD, and GoBGP without touching a production router.
- **Migration:** Try the ExaBGP migration path against an existing config before changing automation.
- **Appliance:** Boot Ze on spare hardware with the same binary and configuration model as daemon mode.
- **Source:** Read the code, generated docs, RFC gate, and test evidence before deciding where Ze belongs.
