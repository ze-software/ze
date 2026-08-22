---
title: Project History
description: Project history and reasons to consider Ze: ExaBGP origin, appliance lessons, Go and YANG choices, lab-friendly install, netlab support, operator CLI, BGP workflow, and proof links.
---

# Project History

Ze is an open-source network operating system, and more: a Linux routing daemon, an appliance runtime, a lab router, and a configuration and protocol engine that can absorb work normally left to scripts around the router. It is meant to be easy to try: one binary for local installs, appliance paths for bootable systems, and netlab support for throwaway topologies.

> Ze is pre-release software. Use this page to decide whether Ze is worth a lab, not as a production stability claim.

## The short version

Ze is built to make routing work easier to operate. The value is the user experience around the pieces operators already maintain: IRR and PeeringDB data, generated prefix lists, looking glass views, operator commands, web and API access, lab topologies, generated docs, and evidence. Keeping those pieces in one model avoids a second system of scripts, dashboards, and private glue.

- **Powerful network-operator CLI:** Ze's SSH CLI includes commit, rollback, diff, completion, live BGP dashboards, ping, traceroute, structured filters, and shell-like output pipes.
- **Network operator protocols:** BGP, BFD, OSPF, IS-IS, IPsec VPN, L2TP, PPPoE, WireGuard interfaces, MPLS, SRv6, and VPP fit into one operator model.
- **BGP plus the surrounding workflow:** Ze speaks BGP, manages Linux interfaces, programs the FIB, and exposes operator actions through SSH, web, API, CLI, and MCP.
- **Easy labs and appliance bootstrap:** install one `ze` binary locally, build bootable or appliance images, or run Ze as a netlab daemon inside containerlab.
- **IRR filtering without a bgpq4 job:** the `bgp-filter-irr` plugin resolves AS-SET data, persists prefix lists, and rejects routes inside the BGP import chain.
- **PeeringDB and lookup data in the CLI:** Ze can query PeeringDB for max-prefix values, warn on stale data, and enrich output with resolver and origin pipe operators.
- **Looking glass built in:** routes, peers, AS-path search, topology graphs, and a Birdwatcher-compatible API are served by Ze instead of a separate stack.

## Replace sidecar automation

Traditional routing deployments often grow a shell around the daemon: cron jobs run `bgpq4`, scripts query PeeringDB, static prefix lists are pushed into config, and separate viewers poll route state. Ze's reason to exist is to pull those pieces into the router's own model when they affect routing behavior.

| External job today | Ze path | Why it matters |
| --- | --- | --- |
| Run `bgpq4`, generate prefix lists, deploy them separately | [IRR Route Filtering](../../guides/irr-filtering/) | Ze resolves the AS-SET, keeps last-known-good data, persists it in ZeFS, and applies it in the import filter chain. |
| Look up PeeringDB prefix counts and copy limits into peer config | [`resolve peeringdb max-prefix <asn>`](../../features/bgp-configuration/#prefix-limits) | Ze can update prefix maximums automatically, add a configurable margin, and warn when data is stale. |
| Expose BGP state with a separate looking glass service | [Built-in Looking Glass](../../features/looking-glass/) | Ze serves peer state, route lookup, AS-path search, topology graphs, HTMX UI, and a Birdwatcher-compatible REST API. |
| Write one-off scripts to inspect route data | [Command filters and output pipes](../../features/formatting/) | Server-side filters can count or narrow routes before serializing rows, and output can be formatted as table, JSON, YAML, NDJSON, text, or raw. |
| Build a lab router image or write ad hoc lab glue | [Installation](../../guides/ze-install/) and [netlab](../../labs/netlab/) | Ze can be copied as one binary for local installs, packaged as an appliance, or rendered by netlab as a daemon node with BGP, OSPF, IS-IS, BFD, and routing modules. |

## Make external data operational

Ze does not treat third-party data as comments beside the router. It turns that data into checked state the operator can inspect.

- **IRR and PeeringDB for filtering.** If a peer has an AS-SET, Ze can resolve it through IRR, fall back through PeeringDB when configured, and apply the generated prefix list per peer.
- **Last-known-good behavior.** A failed or empty IRR refresh does not silently replace a useful allow-list with nothing. Ze keeps the previous list, marks it stale, and reports the reason.
- **PeeringDB for max-prefix.** Prefix limits can be refreshed from PeeringDB with a configurable margin and private mirror URL.
- **Output enrichment.** The built-in resolver supports `| resolve` and `| origin` pipe operators, so command output can include DNS and ASN context without a second terminal workflow.

## One model, many surfaces

Each subsystem brings YANG. The same model feeds configuration validation, CLI completion, the web editor, API output, generated docs, audit, diagnostics, and MCP tools. That matters most for network-operator protocols such as BGP, BFD, OSPF, IS-IS, IPsec, L2TP, PPPoE, WireGuard, MPLS, and SRv6, because each feature should show up through the same commit, diff, validation, and troubleshooting workflow. Adding a plugin should not mean inventing a second command tree, a second web model, and a separate AI tool contract.

1. Declare the model.
2. Use it through CLI, web UI, API, MCP, generated references, and validation.
3. Prove it with functional transcripts, browser tests, demos, interop, and generated evidence.

## Operate as a product

Ze is useful when you want the router, lab, and documentation to agree. The site links product claims to runnable demos, netlab support, and evidence instead of only listing features.

- [Live BGP dashboard demo](../../demos/terminal/#live-bgp-dashboard): connect over SSH, sort peers, and inspect one session.
- [Web configuration commit demo](../../demos/terminal/#web-configuration-commit): see the browser workflow instead of a static screenshot.
- [netlab guide](../../labs/netlab/): start Ze as a daemon node in a generated containerlab topology.
- [Quality evidence map](../../quality/): unit tests, functional transcripts, fuzzing, mutation checks, QEMU, interop, and release evidence.
- [Use cases](../../use-cases/): route server, transit edge with RPKI, FlowSpec injection, AS112, ExaBGP migration, and looking-glass topology pages.

## Why Ze was created

Ze exists because ExaBGP worked.

ExaBGP proved that BGP could be programmable without pretending to be a router CLI. It let routes and protocol events become input and output for normal processes. That idea was useful, and it still is. I did not start Ze because I thought ExaBGP had failed. I started Ze because I wanted to keep the useful idea and stop carrying the limits around it.

ExaBGP was written for the job I needed at the time. Ze is written for the job I need now.

### Python was the wrong base for the next step

Python was a good choice for ExaBGP. It made it easy to express protocol work, glue processes together, and let operators script behaviour around BGP. It also made ExaBGP approachable.

The next system needed a different base.

Python is slow for the kind of work I wanted Ze to do. BGP can produce a lot of state, and once you start adding policy, route storage, filtering, APIs, web views, telemetry, and appliance behaviour around it, the cost of the runtime matters. More important than speed, Python is interpreted. Too many mistakes are found when a path runs, not when the program builds. That is not what I want for a system that validates configuration, manages routing state, and may run as the main software on a box.

I wanted more mistakes to be found before anything boots.

### ExaCube was the appliance lesson

At Exa Networks we had already built an appliance before Ze. ExaCube was our content filtering appliance.

It was useful, but it also showed how much effort disappears into packaging, updates, and recovery when the appliance base is not the product architecture. An appliance is not just software installed on a box. Once you ship one, you own boot, upgrades, recovery, configuration, observability, support, and every strange state the customer can leave it in.

I also wanted more control over the management experience. The web management features were not where I wanted them to be, and adding a product on top of someone else's appliance shape leaves you working around decisions that were made for a different product.

I did not want to repeat that for SurfProtect.

### SurfProtect needed the same base

SurfProtect needed an appliance base I could trust and maintain. I did not want to build another private stack of scripts, packaging, configuration glue, upgrade logic, and web pages around a daemon. That would have moved the product forward for a while, but it would also have created another system only I could understand.

Ze is the base I wanted for that work.

By making Ze open source, the useful infrastructure is not trapped inside SurfProtect. The protocol engine, YANG model, plugin system, CLI, web interface, APIs, generated documentation, tests, and appliance work can improve in the open. SurfProtect can use that foundation and still remain a product with its own purpose.

This is how I can move the business forward without hiding the base. I get the appliance foundation I need, and the community gets the part that should not have been proprietary in the first place.

### Gokrazy made the appliance shape clearer

I had wanted an appliance base for a while. Finding gokrazy made the shape concrete.

Gokrazy builds small appliances from one or more Go programs. It uses a minimal Go userland instead of a normal Linux distribution, runs without a default shell or OpenSSH, keeps the root filesystem read-only, and updates by replacing the system image. That was close to what I wanted for Ze: not a routing daemon surrounded by packaging scripts, but a bootable system where the process model, updates, configuration, and recovery path are part of the design.

It changed the question from how do I install a daemon to what should the whole box look like if Ze is the main program running on it.

That also reinforced the Go choice. A Go program can sit at the appliance boundary in a way that feels natural instead of forced.

### Why Go

Most of my serious work is now in Go. That matters. I know how to build, profile, debug, and maintain Go systems. The language is small enough that the code remains readable months later, and the toolchain is excellent.

Go also works well with how I now develop software with AI assistance. The compiler catches many classes of mistakes. The formatter removes arguments about style. The profiler is practical. The debugger works. Cross-compilation and static builds are boring. Boring is good when the result may become an appliance.

I do not write Go as if it were Python with braces. We write Go much more like C than Python. We care about allocations, buffers, explicit data flow, and predictable hot paths. Go gives enough control over memory for the way we write network code without making every change a fight with the language.

I want the code to be easy to inspect. I want the fast path to be visible. I want a profiler trace to point at something I can fix.

### Why not Rust, Zig, Odin, or V

I looked at Rust, Zig, Odin, and V. I did more than read their websites. I built prototypes in those languages to see how the code would feel once there was a parser, a daemon, a configuration path, and some real state to move around. I might even have tried Jai if it had been publicly available.

V was the nicest to write. Zig gave the most control. Odin felt the most rounded. Rust was the most opinionated, and probably the best language for AI-assisted development if you only look at the compiler. Its strictness is useful when a model changes code, because the compiler catches a lot before the program runs.

I still picked Go because it was the best all-round choice for Ze. The ecosystem is large, the third-party libraries are good, the toolchain is boring, and the language has a lot of high-quality public code for AI systems to learn from. Since Ze is being written with AI assistance, the language, tooling, compiler, profiler, debugger, libraries, and training corpus mattered more than my personal enjoyment of the syntax.

### Clean room, not a port

Ze is not ExaBGP translated to Go.

I wanted a clean-room reimplementation. The useful lessons from ExaBGP remain: programmable routing, process integration, and a migration path for existing ExaBGP users. The internals are new because the scope is different.

ExaBGP was a programmable BGP speaker. Ze needs to be a configuration and protocol engine, a daemon, a lab router, and an appliance base. It needs a native BGP engine, route storage, policy, Linux interface management, FIB programming, operator access, APIs, a web interface, MCP, telemetry, generated documentation, tests, and release evidence.

That cannot be added cleanly by continuing to grow the old shape.

### Why YANG

The other important decision was YANG.

I wanted one model. The CLI, web UI, API, generated documentation, validation, diagnostics, and AI tooling should not each invent their own view of configuration. Each subsystem should declare what it owns, and the rest of the system should use that declaration.

That is why Ze is YANG-based. It gives the project a schema before it gives users another interface. It also makes plugins cleaner. A plugin should not have to invent a command tree, a web model, and an API contract separately.

This is part of the appliance idea as well. If the system is going to boot, validate, commit, roll back, expose state, and recover from mistakes, the model has to be shared.

### What Ze keeps

Ze keeps the part of ExaBGP that mattered: routing should be programmable by operators, not only configured through a vendor-shaped CLI.

It also keeps the migration path. Existing ExaBGP configurations and process scripts should have a way forward. Compatibility is there to reduce the cost of moving, not because Ze is an ExaBGP clone.

The difference is that Ze is built for the system I want now: compiled, inspectable, appliance-ready, YANG-modeled, open source, and designed so the product and the evidence can be checked from the same source tree.

### The first development period

The first tracked development week began in December 2025 with the BGP wire engine, capability negotiation, path attributes, address families, RIB work, session state, and ExaBGP interop. The project then expanded through configuration and plugin architecture, route server and reflector paths, policy, operator surfaces, interfaces, FIB backends, routing protocols, access services, security, diagnostics, packaging, and interoperability.

The [weekly changes](https://ze-software.net/changes/) retain the detailed chronological record. [Milestones](https://ze-software.net/milestones/) selects the larger product changes, while the [roadmap](https://ze-software.net/roadmap/) describes what remains before a stable release.

## When to choose something else

Use FRR, BIRD, OpenBGPd, GoBGP, or a vendor NOS when production mileage matters more than integration, when you need a feature Ze does not have, or when AGPL is a blocker. Ze is a good candidate for netlab labs, migration experiments, programmable routing workflows, appliances, and networks that want routing plus surrounding operational data in one system.

For the full trade-off list, read [Why Ze / Why Not Ze](../why-ze-or-not/).
