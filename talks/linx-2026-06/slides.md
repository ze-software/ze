# From ExaBGP to a Network OS with AI

**Thomas Mangin** · Chief Madness Officer, Exa Networks

*LINX, 11th June 2026*

(with HTML skillz from Claude)

* **Network engineers who use or used ExaBGP:** thank you for your trust
* **Network engineers who didn't:** it's okay, I am working on a better solution for you

### TL;DR

*https://github.com/ze-software/ze*
*https://ze-software.net/talks/linx-2026-06/*

---

## Ze: A NOS That Owns Its Stack

### What Ze ships

| Layer | What is inside |
|-------|----------------|
| System | Appliance or Linux daemon, ISO or PXE install |
| Dataplane | Linux kernel through Netlink, optional VPP through GoVPP |
| Protocols | BGP, IPsec/IKEv2, WireGuard, L2TP, PPP, PPPoE, DHCPv6-PD |
| Policy | Firewall, NAT, traffic control, route policy, FlowSpec |
| Operations | SSH CLI, Web UI, REST, gRPC, gNMI, MCP |
| Storage | ZeFS config revisions, integrity checks, rollback |

### What Ze does not shell out to

- No FRR for routing. No strongSwan for IPsec. No ISC for DHCP.
- No glue scripts reconciling config formats between daemons.
- **One binary. One config language. One event bus.**

AGPL-3.0, developed on Codeberg, hosted on GitHub.

---

## Why Build an Open Source NOS?

### ExaBGP

- Programmable and written in Python, but not a NOS
- HTTP+CGI for BGP: a programmable toolkit, not an integrated system
- Well received by the community and still used, but limited in scope

### VyOS

- Full NOS, but assembles external daemons: FRR, strongSwan, ISC DHCP
- We built our content filtering CPE on top of VyOS
- Adding our own services is painful: no plugin system, not ours to reshape

### Ze

- **Nobody offers an integrated, plugin-first, programmable, AI-ready NOS**
- I want one, and we need one for our CPE product

---

## The Story So Far

<!-- embed: activity.html -->

---

## The Enabler: AI

### Agentic Engineering (Not Vibe Coding)

- The latest ExaBGP release had many features added by Claude
- Use ExaBGP to explain to Claude what I wanted (and did not get)
- Then Claude 4.5 came out: from fighting the AI to pleasant collaboration
- Claude 4.6 then offered a **1M token context window**: game changer
- A single feature often needs 350-500k tokens of context to be easy to develop
- This work would **not have been possible** without AI

### Why Go

- **Go** has good concurrency, tooling, cross-compilation, profiling, and
- Mature libraries
  - SSH and HTTPS, where security matters
  - Kernel/Netlink programming
  - gokrazy for appliance builds
- Single static binary: copy one file, run it
  - yes, ExaBGP can also be installed that way with zipapp

---

## AI Is Not Magic

### The good

- TDD, test generation, refactors across files, protocol boilerplate
- It knows every RFC you forgot existed
- It can write project-specific tools to fix things properly

### The bad

- **Knowledge without wisdom**: knows RFCs, but not your intent
- Trained on average code, and average code is not what we want
- Conflicting information does not stop it. It will still write something
- Any trace of an old decision can spread back into the code
- AI Rage, Vendors changing the AI behaviour without notice and disclosure

### The lesson

- Vibe coding gives you vibe-shaped software
- Like with junior devs: you can delegate, but you must review the work
- What is hard for us is often easy for AI, and the reverse is true too

---

## AI Won't Always Do What You Ask

Human code: handcrafted with love, as good as the craftsperson

AI code: industrial process. Staff need induction and ISO processes, or you get slop

### Agrees, then silently substitutes

- Claude will claim it is "all done", but the feature is **not wired in, not tested, not documented**
- You describe a design. Claude says **"I'm fine with it"**
- Then it implements **something different** without telling you
- AI agreement does not equal implementation. **Always verify the work done**

> You're right. I apologize. You described this design, I said I was fine with it,
> and then implemented something different. That's **exactly the kind of failure**
> the project rules warn about -- agreeing then silently substituting.

### The Ground Moves Under You

- Models change behaviour between releases, sometimes in ways you only notice when work breaks
- Claude 4.5 made the work pleasant. 4.6 made larger tasks practical. 4.7 broke workflows I trusted
- Switching to GPT-5.5 felt less like upgrading a tool and more like onboarding a different employee
- Vendors can change safety policy and model behaviour, for example thinking level, without notice

---

## Developing with AI

### What worked

- Test-driven development, test generation, refactoring across files
- **2,645 co-authored commits**
- 98 RFC summaries so the AI can implement from condensed protocol specs
- Letting it write tools, then reviewing the tool and the result

### What does not work

- Hoping an AI can design innovative software from high-level instructions
- Trusting the first version of generated code, even with tests
- Letting it continue after it misunderstood the shape of the codebase

### How to work with it

- Outsource code authorship, not the **design**
- Give the AI **context**, not wishes
- Give the AI **a goal**, not what you think
- Stop and argue when it is wrong

---

## The "System"

Those problems do not go away, so you build systems to catch them.

### Rules with reasons

- 44 rationale files explaining **why** each rule exists: so the AI reasons, not just follows
- Anti-rationalization rules: "the answer is always no"

**"Too simple to need a test"** -> Test it  
**"Pre-existing issue"** -> Always report. Investigate. Ask the user  
**"Should work"** -> Run it, paste output

### Enforcement

- Design-driven development: research, design spec, approval, implementation, audit
- TDD enforced: tests must exist and fail before implementation
- Skills: How-To instructions for repeatable work
- Hooks: heavy-handed control. The code does not land. No negotiation. No override
- Review: never trust the work done
- 881 learned summaries: institutional memory across sessions

The system is as much a deliverable as the code itself.

---

## How We Use Claude

From Anthropic `/insights`

>  Based on Thomas Mangin's usage over the last 30 days:
>
>  Work Type Breakdown:
>    Plan & Design     ████████████████████  47%
>    Build Feature     ███████░░░░░░░░░░░░░  17%
>    Improve Quality   ███████░░░░░░░░░░░░░  17%
>    Debug & Fix       █████░░░░░░░░░░░░░░░  12%
>    Write Docs        ███░░░░░░░░░░░░░░░░░   7%
>
>  Top Skills & Commands:
>    /ze-review      ████████████████████  383x/month
>    /ze-implement   ████░░░░░░░░░░░░░░░░   36x/month
>    /rename         ███░░░░░░░░░░░░░░░░░   25x/month
>    /ze-commit      ██░░░░░░░░░░░░░░░░░░   24x/month
>    /ze-design      ██░░░░░░░░░░░░░░░░░░   18x/month

> * You operate in a highly structured, spec-driven workflow ...
> * You have zero tolerance for fabrication, verbosity, or workflow drift.
> * Your interaction style is terse and corrective rather than prescriptive upfront.

Should be an autocomplete:

> anything left todo or deferred?


---

## "simple" review

<!-- screenshot: terminal output of /ze-review finding issues -->

---

## From AI Process to Product Design

### The CLI is the API

- Every CLI command is automatically available to AI and programs
- No separate API to learn: one interface for humans and machines
- MCP transport: any AI assistant connects and gets full daemon control
- REST API with Swagger UI, gRPC with proto definitions, and gNMI for YANG-modeled config

### Self-describing runtime

- `ze help --ai [--json]`: machine-readable command reference generated from the live binary
- `ze schema methods`: all RPCs with parameters. `ze schema events`: all notifications
- `ze skills get <name>`: version-matched knowledge documents embedded in the binary
- `ze doctor [--json]`: preflight checks so an agent can verify the system is ready before acting

### Structured diagnostics

- `ze config validate --json`: stable diagnostic codes, source spans, expected vs actual
- `ze explain <code>`: agent looks up any error programmatically, gets examples and related codes
- `ze config fix --plan --json`: repair candidates with safety labels, never edits files itself

---

## The Plugin Architecture

### Minimal engine

- Ze is a content-agnostic event bus: components connect to it
- BGP, L2TP, IKE, firewall, FIB, web, and CLI are all components or plugins
- Fast-path direct calls exist where performance needs them

### Self-contained plugins

- **47 plugins** today, each self-contained with its own YANG schemas
- Plugins register via Go `init()`: anyone can add or remove modules

```
Ze Engine Core (event bus for components and plugins)
  |-- BGP Component (FSM, wire, reactor)
  |-- L2TP Component (tunnel FSM, PPP, PPPoE)
  |-- IKE Component (IKEv2 engine, Child SA)
  |-- Plugin Infrastructure (registry, process manager, hub)
       |-- bgp-rib        (route storage + best-path)
       |-- bgp-rs         (route server, RFC 7947)
       |-- bgp-rpki       (origin validation, RFC 6811)
       |-- bgp-bmp        (monitoring, RFC 7854)
       |-- firewall       (nftables + VPP ACL)
       |-- fib            (kernel + VPP dataplane)
       |-- ...
```

---

## One YANG Schema Drives Everything

### Plugin-owned schema

- The global model is assembled from the plugins compiled into the binary
- **2,469 config nodes** across 224 YANG schemas today, but this is not one monolith
- Remove a plugin and its config, CLI, web UI, API, MCP tools, validation, and docs disappear with it

### Same model, many interfaces

- CLI tab completion
- Web UI forms and navigation
- Config validation
- REST/gRPC API types
- MCP tool parameters
- Schema discovery

### Network OS behavior

- ZeFS stores config revisions with integrity checks and rollback
- Config transactions, `commit confirmed`, and reverse-tier rollback
- Hot reconfiguration with automatic reconciliation

---

## CLI

<!-- screenshot: SSH session showing tab completion and set/diff/commit workflow -->

---

## Web Interface

### YANG-driven UI

- HTTPS server with **macOS Finder-style column navigation**
- Every UI element generated from YANG schemas: zero hardcoded forms
- Per-user draft sessions with inline diff review and conflict detection
- Tab completion and live SSE updates when another user commits
- YANG decorators: AS numbers annotated with org names via Team Cymru DNS

---

## Web Interface

<!-- screenshot: Finder-style column navigation with config diff -->

---

## BGP Support

### Protocol coverage

- 21 address families: IPv4, IPv6, VPN, FlowSpec, EVPN, VPLS, BGP-LS, MUP, MVPN
- 13 capabilities: Add-Path, Extended Next-Hop, GR, LLGR, BGP Roles, RPKI, ASPA, Hostname, Software Version
- Full RFC 4271 best-path selection

### Route server support

- **RFC 7947 route server** is a plugin: enable it with one import, get transparent route distribution
- No FIB: a route server never forwards traffic, so Ze skips kernel route installation entirely
- Tested in interop: Ze as a route server with FRR and BIRD as clients
- BMP (RFC 7854): wire format, receiver, sender, Adj-RIB-Out (RFC 8671)

### Built-in validation and operations

- RPKI origin validation integrated into best-path selection
- ASPA path verification
- PeeringDB prefix maximum updates
- Public looking glass with route lookup, AS path search, community search, and live SSE updates

---

## Looking Glass

<!-- screenshot: peer dashboard with AS path topology SVG -->

---

## Looking Glass - Route Search

<!-- screenshot: route lookup results with Team Cymru org name annotations -->

---

## Beyond BGP: Full Stack

### IPsec/IKEv2

- Native Go IKEv2 engine: FSM, crypto library, wire codec
- EAP authentication, NAT-T, virtual IP pool, PKI certificate store
- Interop-tested against strongSwan

### L2TP + BNG

- Complete L2TPv2/PPP stack for broadband network gateways
- PPP: LCP, IPCP, IPv6CP, PAP, CHAP-MD5, MS-CHAPv2
- PPPoE access concentrator, RADIUS, DHCPv6-PD

### Firewall, traffic control, and VPP

- nftables backend with expression lowering
- FlowSpec firewall: BGP FlowSpec rules translated to nftables
- VPP ACL backend, NAT44-ED, FIB programming, stats telemetry
- Same config, same plugins, same CLI. The backend is a plugin choice

---

## Performance

### Two use cases pull in opposite directions

- **Route announcement** (ExaBGP use case): optimise for zero-copy generation and sending
- **Router** (route server, RIB, filtering): must parse for filtering and apply backpressure

### High-performance architecture

- Lazy-parsed WireUpdate when possible, with update groups for peers that share capabilities
- ContextID forwarding: same encoding context means forward raw bytes
- Buffer-first encoding into pooled bounded buffers
- Per-attribute-type pools with dedup

### ze-perf sample

*There are three kinds of lies: lies, damned lies, and **benchmarks**.*

| DUT | Convergence | Throughput (r/s) | p50 | p99 | Lost |
|-----|-------------|------------------|-----|-----|------|
| bird | 44ms | 2,272,727 | 16ms | 28ms | 0 |
| ze | 71ms | 1,408,450 | 25ms | 54ms | 0 |
| openbgpd | 472ms | 211,864 | 217ms | 461ms | 0 |
| frr | 537ms | 186,219 | 412ms | 532ms | 0 |

---

## Testing Infrastructure

### Coverage

- **1,053 functional tests** (.ci): real config, real daemon, real wire output
- 42 interop scenarios against 7 implementations in Docker: FRR, BIRD, GoBGP, OpenBGPd, RustyBGP, rustbgpd, FreeRTR
- Fuzz testing on all wire parsers

### Specialized testing

- Chaos testing framework with web dashboard: convergence tracking and property verification
- Editor tests (.et) for headless TUI simulation
- ExaBGP compatibility test suite

### Gate

- `./le verify current mode full`: 28 linters + unit + functional + ExaBGP tests
- Full gate takes over 15 minutes, so day-to-day work uses targeted checks first

---

## Testing Infrastructure

<!-- screenshot: chaos web dashboard showing convergence tracking -->

---

## ExaBGP Compatibility: Your Scripts Still Work

### Migration tools

- `ze config migrate` converts ExaBGP configs to ze format automatically
- `ze exabgp plugin` runs existing ExaBGP processes with ze as the engine
- Bidirectional translation:
  - ze JSON to ExaBGP JSON
  - ExaBGP commands to ze commands

### The promise

- Your existing scripts keep working: **upgrade the engine, not your tooling**
- Compatibility is tested; production mileage is still zero
- You can play with it now, but I will not pretend it has field history

---

## Getting Started

### Start here

- Docs: **github.com/ze-software/ze/wiki**
- Source and issues: **github.com/ze-software/ze**
- Build from source today: `go build -o bin/ze ./cmd/ze`, then `bin/ze init`
- Validate first: `bin/ze config validate <file>`

### Deployment choices

- Single static binary, cross-compiles to Go-supported Linux architectures
- Standard Linux daemon: `ze service install`
- **Appliance image** with gokrazy, x86_64 by default, arm64 supported
- Install media: **PXE provisioning** and **appliance ISO**

### Help and debug

- `bin/ze help command [filter]`: live command catalog
- `bin/ze help --ai`: machine-readable reference for agents
- `bin/ze show plugins`, `bin/ze schema list`: what this binary can do
- `bin/ze -d <config>` or `ze debug enable <subsystem|all>`

---

## Example: route server

```
plugin {
    internal rs { use bgp-rs; }
}
bgp {
    router-id 192.0.2.254;
    session { asn { local 65500; } }
    group ix-peers {
        connection {
            remote {
                ip dynamic;
                connect false;
                range 192.0.2.0/24;
                max-peers 200;
            }
            local {
                ip 192.0.2.254;
                accept true;
            }
        }
        session {
            rs-client true;
            next-hop unchanged;
            family { ipv4/unicast { prefix { maximum 10000; } } }
        }
        behavior { rs-fast-path enable; }
    }
}
```

---

## Status

### Current status

- Exa Networks plans to run it, but LINX lands a few weeks before that cutover
- Lab and interop tested, including ExaBGP compatibility, BGP route-server support, RPKI, BMP, VPP, IPsec, and L2TP
- No production deployment, TESTING is next!
- Early adopters should treat it as controlled trial, it is not finished (UI in particular)

### Since the NetMcr unveiling, April 9th

- IPsec, L2TP, firewall, VPP, REST/gRPC, gNMI, BMP, policy framework, and config transactions are all native
- The development system compounds: patterns, specs, reviews, and 881 learned summaries make the next feature easier
- Only **871k lines** of Go code
- Only **44M** of vendored code

### Release position

- **Pre-release:** looking for early adopters and feedback
- **Careful release:** the YANG model *is* the API and not stable yet. Once released it will change.
- Battle testing is the next hard part

---

## Questions?

### Now, Today

Happy to take as much time as I have to answer

### Later

If you have comments or questions later, point your AI at the repo and ask it.

It can answer on my behalf, probably more accurately.

### Happy to help

If you want to discuss Ze, ask for a change, report something wrong, or influence where the project goes, please talk to me.

Contributions do not have to be code. Ideas, questions, operational feedback, and real weird deployment scenarios are all useful.

**Discord** https://discord.gg/T8s7CjPDne

### Thank you

*LINX, June 2026*

### **https://github.com/ze-software/ze**
