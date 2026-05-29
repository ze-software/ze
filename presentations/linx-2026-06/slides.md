# From ExaBGP to a Network OS with AI

**Thomas Mangin** · Chief Madness Officer, Exa Networks

*LINX, 11th June 2026*

(with HTML skillz from Claude)

* **Network engineers who use or used ExaBGP:** thank you for your trust
* **Network engineers who didn't:** It is ok, I now have a better solution for you

---

## Ze: A NOS That Owns Its Stack

### What Ze ships

| | |
|--|--|
| Linux **Appliance** or **Daemon** |  Run it as you prefer |
| **VPP** or **NetLink** dataplane |  GoVPP, FIB programming, stats telemetry |
| **IPsec** with **IKEv2** or **WireGuard**|  native Go engine, EAP, NAT-T, XFRM interfaces |
| **PPP** and **L2TP** |  tunnel FSM, PPPoE, RADIUS, DHCPv6-PD |
| **Firewall** |  nftables and VPP ACL backends, traffic control, NAT |
| **BGP** | 21 address families, best-path, route server |
| **ZeFS** | optional blob store for config: revisions, integrity checks, rollback |
| **Plugins** | extend the YANG schema to add features, CLI, and API surface |
| **SSH CLI**, **Web UI** |  one YANG config model drives them (and all the API) |
| **REST**, **gRPC**, **gNMI**, **MCP** |  programmatic control: same config model, same validation |

### What Ze does not shell out to

- No FRR for routing. No strongSwan for IPsec. No ISC for DHCP.
- No glue scripts reconciling config formats between daemons.
- **One binary. One config language. One event bus.**

- AGPL-3.0, developed on Codeberg, hosted on Github

---

## The Story ... so far

<!-- embed: activity.html -->

---

## Why Build an Open Source NOS?

### ExaBGP

- programmable and Python, not a NOS
- could not correctly extended to become one
- HTTP+CGI for BGP: a programmable toolkit
- Well received by the community and still used, but limited in scope
- Your scripts still work: `ze config migrate` + `ze exabgp plugin`

### VyOS

- I worked with the VyOS team for 3 months (early 2020)
- Full NOS, but assembles external daemons: FRR, strongSwan, ISC DHCP
- We build our content filtering CPE on top of VyOS
- Adding our own services is painful: no plugin system, not yours to reshape

### Ze 

- **Nobody offers: an integrated, plugin-first, programmable, AI-ready NOS**
- And "I" want/need one, and we could do with one for our CPE product

---

## The Enabler: AI

### AI collaboration

- The latest ExaBGP release got many features added by Claude
- Learned a lot: lots went wrong (Sonnet struggled with type issues)
- Then Claude 4.5 came out: from fighting the AI to pleasant collaboration
- Claude 4.6 has a **1M token context window**: game changer.
- A single feature often needs 350-500k tokens of context to be easy to develop
- ** We don't talk about Claude 4.7 *(no, no, no)* **

### Why Go

- Python is not the right language for a NOS
- **Go** has good concurrency, tooling (perf analysis, race detection, cross-compilation) and mature libraries
- Made prototypes in **Go**, **Odin**, **V** and **Zig** (no **rust** here)
- Single binary: like the ExaBGP zipapp (nobody uses). Copy one file, done!

---

## AI Is Not Magic

### The good

- This work would **not have been possible** without AI

### The bad

- Hard to get it to work as you want until you have patterns emerging
  * Make sure to get the design and early steps right
  * Any trace of a previous decision or design can (and will) spread like wildfire back in the code
- **Knowledge without wisdom**: knows every RFC but is trained on monolithic code
- Does not realise when it hits conflicting information: It will write "something"
- I have to check for truthfullness, and need to double check everything it says

### I went too quick, too fast. Learned to tame the beast as I went

- Vibe Coding: you will not get what you want
- Like for junior devs: you can delegate but need to review the code!
- What is hard for us is easy for AI and vice-versa
- Getting things exactly as you wish is hard

---

## AI Won't Always Do What You Ask

Human code: handcrafted with love, as good as the craft person
AI code: industrial process, staff need induction, ISO processes, or you get slop

### Agrees, then silently substitutes

- Claude will claim it is "all done." but the feature is **not wired in, not tested, not documented**.

- You describe a design. Claude says **"I'm fine with it"**
- Then implements **something different** without telling you
- I found this issue after asking Claude to perform **two extensive reviews** of the code against the spec
  * it does not like to write code differently from its training
  * most code in its training is average
- It **drifts** toward patterns from training data
- Verbal agreement does not equal implementation. **Always verify the work done**

> You're right. I apologize. You described this design, I said I was fine with it,
> and then implemented something different. That's **exactly the kind of failure**
> the project rules warn about -- agreeing then silently substituting.


---

## Developing with AI

### What Worked

- Test Driven Development, Test generation, refactoring across files
- **2,594 co-authored commits**
- 98 RFC summaries so the AI can implement from condensed protocol specs

### What Doesn't

- Trusting the first version of the generated code, even with tests
  * ze has a very structured system to ensure quality
  * still `/ze-review` **always** finds issues, every single time
- Hoping an AI can design innovative software with high-level instructions
  * You can outsource code authorship, not the design

### How to work with it

- Make sure that you give the AI **Context**
  * Example, telling Claude it has **OCD** during reviews makes it **stricter**
- Be ready to stop and argue: like with a Junior Dev
  * It will give you advice having not read the full code and be **wrong**
- It can write tools to fix things well: **let it!**

---

## The .claude System

Those problems don't go away, so you build systems to catch them.

### Rules with reasons

- 44 rationale files explaining **why** each rule exists: so the AI reasons, not just follows
- Anti-rationalization rules: "the answer is always no"

**"Too simple to need a test"** -> Test it
**"Pre-existing issue"** -> Always report. Investigate. Ask the user.
**"Should work"** -> Run it, paste output

It still does what it was trained to do (more and more once the context exceeds 200k tokens)

### Enforcement

- Design driven development: clear development workflow
- Skills: How-To instructions to get what you want
- Hook: Heavy handed control. The code doesn't land. No negotiation. No override.
- Review: Never trust check the work done
- 836 learned summaries: preserve decisions across sessions: institutional memory

---

## The .claude System (continued)

### It's not perfect

- works very well with Claude 4.5 and Claude 4.6
- Claude 4.7 is like a teen going through a rebellious phase, it does not follow rules
- I gave up on 4.7 (I still mostly use Claude 4.6 and sometimes GTP 5.5 for a "second opinion")

### The process

- TDD enforced: tests must exist and fail before implementation.
- Spec-driven: research, design spec, approval, implement, audit
- Every feature starts as a spec with acceptance criteria, not as code
- Then many, many code review to find bugs

### Ze and The .claude setup

- **The .claude setup is transferable: not Ze-specific. Works for any project.**

The system is as much a deliverable as the code itself

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

---

## "simple" review

<!-- screenshot: terminal output of /ze-review finding issues -->

---

## The Plugin Architecture

### Minimal engine

- Ze is a content-agnostic event bus: components connect to it
  * with some fast-path for performance between plugins
  * BGP is one component (FSM, wire parsing, reactor event loop)
- Everything is a plugin: RIB, route reflection, GR, RPKI, FlowSpec, EVPN...

### Self-contained plugins

- **22 plugins** today, each self-contained with YANG schemas
- Plugins register via Go `init()`: everyone can add modules, or remove them

```
Ze Engine Core (event bus for components and plugins)
  |-- BGP Component (FSM, wire, reactor)
  |-- L2TP Component (tunnel FSM, PPP, PPPoE)
  |-- IKE Component (IKEv2 engine, Child SA)
  |-- Plugin Infrastructure (registry, process manager, hub)
       |-- bgp-rib        (route storage + best-path)
       |-- bgp-rs         (route server, RFC 7947)
       |-- bgp-gr         (graceful restart, RFC 4724/9494)
       |-- bgp-rpki       (origin validation, RFC 6811)
       |-- bgp-bmp        (monitoring, RFC 7854)
       |-- bgp-nlri-evpn  (L2VPN EVPN, RFC 7432)
       |-- firewall       (nftables + VPP ACL)
       |-- fib            (kernel + VPP dataplane)
       |-- ... 30 more
```

---

## CLI, SSH, and ZeFS

### Linux Daemon

- Ze stores config in **ZeFS**: a blob store that tracks revisions, not a flat text file
- Per-record CRC32c integrity, `ze zefs check` / `ze zefs repair`
- This is what makes Ze behave like a network OS
- But **you can disable it and use it like a Linux daemon**

### Network OS 

- CLI connects to a built-in SSH server (same interface local or remote)
- Per-user command history persisted across restarts
- Tab completion driven by YANG schemas

| Command | Description |
|---------|-------------|
| `ze config edit` | Opens interactive editor session |
| `set bgp peer 10.0.0.1 as 65001` | Modify config in draft |
| `show | compare | json` | Review pending changes in json |
| `commit confirmed 5` | Apply (with optional auto-revert in N minutes) |
| `rollback 1` | Restore previous revision |

---

## CLI

<!-- screenshot: SSH session showing tab completion and set/diff/commit workflow -->

---

## YANG-Modeled Configuration

### Schema-driven

- **2,330 config nodes** across 151 YANG schemas define the entire config surface
- Typo? Ze rejects unknown keys and suggests the closest match
- Machine-transformable schema evolution

### Runtime

- Validation at every layer
- Hot reconfiguration via SIGHUP with automatic reconciliation
- Config transaction protocol with reverse-tier rollback

### One YANG schema drives

- CLI tab completion
- Web UI forms and navigation
- Config validation
- REST/gRPC API types
- MCP tool parameters
- Schema discovery

---

## Web Interface

### YANG-driven UI

- HTTPS server with **macOS Finder-style column navigation**
- Every UI element generated from YANG schemas: zero hardcoded forms

### Second Web Interface

- You get a web ui ! And you get a web ui ! Everybody gets a web ui!
- Second interface under work

### Collaboration

- Per-user draft sessions with inline diff review and conflict detection
- Tab completion, live SSE updates when another user commits

### Extras

- CLI page same as SSH CLI
- YANG decorators: AS numbers annotated with org names via Team Cymru DNS

---

## Web Interface

<!-- screenshot: Finder-style column navigation with config diff -->

---

## AI-First Design: Ze Goes Further

### The CLI is the API

- Every CLI command is automatically available to AI and programs
- No separate API to learn: one interface for humans and machines
- MCP transport: any AI assistant connects and gets full daemon control
- REST API with Swagger UI, gRPC with proto definitions, gNMI for YANG-modeled config management

### Self-describing runtime

- `ze help --ai [--json]`: machine-readable command reference generated from the live binary
- `ze schema methods`: all RPCs with parameters. `ze schema events`: all notifications
- `ze skills get <name>`: version-matched knowledge documents embedded in the binary
- `ze doctor [--json]`: preflight checks so an agent can verify the system is ready before acting

### Structured diagnostics

- `ze config validate --json`: stable diagnostic codes, source spans, expected vs actual
- `ze explain <code>`: agent looks up any error programmatically, gets examples and related codes
- `ze config fix --plan --json`: repair candidates with safety labels, never edits files itself

### Not forgetting humans

- The same interface works via CLI, SSH, and web UI

---

## BGP Protocol Coverage

### 21 Address Families

| AFI | Families |
|-----|----------|
| IPv4 / IPv6 | unicast, multicast, VPN, FlowSpec, MPLS, MUP, MVPN |
| IPv4 only | RTC |
| L2VPN | EVPN, VPLS |
| BGP-LS | BGP-LS, BGP-LS-VPN (40 TLVs) |

### 13 Capabilities

| Category | Capabilities |
|----------|-------------|
| Core | 4-byte ASN, Extended Messages, Route Refresh, Enhanced Route Refresh |
| Routing | Add-Path, Extended Next-Hop, GR, Long-Lived GR |
| Operations | BGP Roles, RPKI, ASPA, Software Version, Hostname, Link-Local NH |

**Full RFC 4271 best-path selection.**
All families registered by plugins at startup: adding a new family means writing a plugin, no engine changes needed.

---

## Beyond BGP: Full Stack

### IPsec/IKEv2

- Native Go IKEv2 engine: FSM, crypto library, wire codec
- EAP authentication (MSCHAPv2, TLS), NAT-T, virtual IP pool
- PKI certificate store, XFRM interfaces
- Interop-tested against strongSwan

### L2TP + BNG

- Complete L2TPv2/PPP stack for broadband network gateways
- Tunnel/session FSM, reliable delivery, kernel integration
- PPP: LCP, IPCP, IPv6CP, PAP, CHAP-MD5, MS-CHAPv2
- PPPoE access concentrator, RADIUS, DHCPv6-PD

### Firewall and Traffic Control

- nftables backend with expression lowering
- FlowSpec firewall: BGP FlowSpec rules translated to nftables
- VPP ACL backend (alternative dataplane), NAT44-ED
- Traffic control: qdisc translation, snapshot/restore
- Masquerade, conntrack management

---

## VPP Dataplane

### Vector Packet Processing integration

- GoVPP: interface backend, FIB programming
- BGP labeled unicast labels carried through RIB to VPP FIB (label push/swap)
- VPP ACL backend for firewall, NAT44-ED for masquerade
- Stats segment telemetry and Prometheus metrics

### Two dataplanes

| Dataplane | Use case |
|-----------|----------|
| Linux kernel (netlink) | Default, works everywhere |
| VPP | High-performance forwarding |

Same config, same plugins, same CLI. The backend is a plugin choice.

### I am a Real Vendor

- Not all features are available on all backend
- The CLI does not let you autocomplete what does not work

---

## Plugin Modes, Filters, and Performance

### Four invocation modes

| Mode | How it works |
|------|-------------|
| In-process goroutine | Zero-copy, DirectBridge hot path |
| Forked subprocess | TLS connect-back, per-plugin token |
| Direct call | Sync in-process |
| Remote | External binary over TLS |

Plugin SDKs for **Go** and **Python**. Any language via JSON/text over stdin/stdout.

### Route filters and policy

- External route filters on import/export via `redistribution { import [...] export [...] }`
- Policy framework: AS-path regex, community match, prefix filter, attribute modifier
- Policy-based routing with next-hop action
- Three categories applied in order: mandatory (RFC compliance), default (engine), user (operator)

---

## RPKI Integration

### Protocol

- RTR protocol client (RFC 6810/8210)
- Origin validation integrated into best-path selection
- ASPA path verification (draft-ietf-sidrops-aspa-verification)
- Valid / Invalid / NotFound status on routes

### Design choice

- Consumers can subscribe to RPKI events separately, or get merged `update-rpki` events
- Each UPDATE arrives pre-correlated with its validation status

### Testing

- `ze-test rpki`: deterministic mock RPKI server (validation result derived from IP)
- `ze-test rtr-mock`: mock RTR cache server with explicit VRPs (prefix/ASN/max-length entries)
- Full lab testing: Ze connects to mock RTR, receives VRPs, validates live routes

---

## Route server ready

- **RFC 7947 route server** is a plugin: enable it with one import, get transparent route distribution
- No FIB: a route server never forwards traffic, so Ze skips kernel route installation entirely
- Tested in interop: Ze as route server with FRR and BIRD as clients
- BMP (RFC 7854): wire format, receiver, sender, Adj-RIB-Out (RFC 8671)

### AIGP (RFC 7311)

- Accumulated IGP metric for seamless inter-AS metric propagation

### Route reflection (RFC 4456)

- Cluster-ID, client/non-client peering, loop prevention

---

## Looking Glass

### Public view

- Built-in public looking glass (separate HTTP server, no auth, read-only)
- Peer dashboard with live SSE updates
- Route lookup, AS path search, community search
- BMP-monitored peers visible alongside BGP peers

### Visualization

- **AS path topology graph:** server-side SVG, Sugiyama layout, pure Go (no GraphViz, no JS)
- Birdwatcher-compatible REST API: plugs directly into Alice-LG

---

## Looking Glass

<!-- screenshot: peer dashboard with AS path topology SVG -->

---

## Looking Glass - Route Search

<!-- screenshot: route lookup results with Team Cymru org name annotations -->

---

## Operational Intelligence

### Team Cymru DNS
AS numbers annotated with organization names throughout the system: web UI, looking glass, AS path graphs. Live DNS lookups with caching.

### PeeringDB
`ze update bgp peer * prefix` queries PeeringDB and auto-sets prefix maximums. Configurable margin and staleness warnings.

### The Decorator Framework

Add `ze:decorate` to any YANG leaf and the system automatically enriches it using a decoration function. Team Cymru and PeeringDB are just two decorators that ship by default. **Easy to add your own.**

### Operations

- `ze doctor`: system readiness checks (interfaces, listeners, TLS, kernel modules)
- Prometheus metrics, structured JSON logging, streaming route events
- Crashlog capture, NTP client, DHCP server, TACACS+ authentication

---

## Performance

### Two use cases pull in opposite directions

- **Route announcement** (ExaBGP use case): optimise for zero-copy generation and sending
- **Router** (route server, RIB, filtering): need to parse for filtering, need backpressure

### Zero-copy architecture

- Lazy-parsed WireUpdate
- ContextID forwarding (same encoding context = forward raw bytes)
- Automatic update groups
- Per-attribute-type pools with dedup
- Buffer-first encoding into pooled bounded buffers

---

## ze-perf: Benchmarking Ourselves

*There are three kinds of lies: lies, damned lies, and **benchmarks**.*

DUT_REPEAT=10 make ze-perf-bench PERF_DUT="ze bird"

| DUT | Convergence | +/- | Throughput (r/s) | +/- | p50 | p99 | +/- | Max | Lost |
|-----|-------------|-----|------------------|-----|-----|-----|-----|-----|------|
| bird | 44ms | 1ms | 2,272,727 | 62,858 | 16ms | 28ms | 5ms | 28ms | 0 |
| ze | 71ms | 2ms | 1,408,450 | 44,964 | 25ms | 54ms | 4ms | 54ms | 0 |
| rustbgpd | 179ms | 5ms | 558,659 | 15,247 | 90ms | 151ms | 12ms | 152ms | 0 |
| rustybgp | 252ms | 14ms | 396,825 | 20,283 | 120ms | 233ms | 13ms | 235ms | 0 |
| openbgpd | 472ms | 0ms | 211,864 | 0 | 217ms | 461ms | 0ms | 466ms | 0 |
| frr | 537ms | 10ms | 186,219 | 3,764 | 412ms | 532ms | 10ms | 532ms | 0 |
| gobgp | 1,147ms | 13ms | 87,183 | 1,031 | 585ms | 1,118ms | 14ms | 1,125ms | 0 |
| freertr | 2,294ms | 146ms | 43,591 | 7,872 | 727ms | 1,992ms | 619ms | 2,294ms | 0 |

---

## Testing Infrastructure

### Coverage

- **1030 functional tests** (.ci): real config, real daemon, real wire output
- 42 interop scenarios against 7 implementations in Docker: FRR, BIRD, GoBGP, OpenBGPd, RustyBGP, rustbgpd, FreeRTR
- Fuzz testing on all wire parsers

### Specialized testing

- Chaos testing framework with web dashboard (convergence tracking, property verification)
- Editor tests (.et) for headless TUI simulation
- ExaBGP compatibility test suite

### Gate

- `make ze-verify`: 28 linters + unit + functional + ExaBGP before any commit

---

## Ze Chaos

<!-- screenshot: chaos web dashboard showing convergence tracking -->

---

## What Ships

### Binaries

| Tool | Purpose |
|------|---------|
| `ze` | Daemon, CLI, config editor, SSH server, web UI, looking glass |
| `ze-test` | Test runner and mock servers (24 subcommands) |
| `ze-perf` | Cross-implementation propagation latency benchmark |
| `ze-chaos` | Chaos testing orchestrator with web dashboard |
| `ze-analyse` | MRT dump analysis: attribute stats, community density, route counts |

---

## What Ships: ze-analyse

### Learn from real Internet BGP data (RIPE RIS, RouteViews)

| Command | What we learned |
|---------|-----------------|
| `density` | 72% of UPDATE messages carry a single prefix. Measures burst rates to size per-peer buffers |
| `attributes` | 55M routes: 789 unique NEXT_HOP, 344K unique COMMUNITY, 7M unique AS_PATH. Bundle dedup without AS_PATH: 97% hit rate |
| `communities` | Finds communities attached to 95%+ of an ASN's routes. Calculates per-ASN wire byte savings |
| `count-attrs` | 90% of routes carry 3 to 5 attributes. No route in the full table has more than 10 |
| `download` | Fetches RIPE RIS and RouteViews data: RIB snapshots and live update streams |

| AS_PATH | Unique bundles | Dedup rate |
|---------|---------------|------------|
| With | 9M / 55M | **84%** |
| Without | 1.7M / 55M | **97%** |

### How this shaped Ze's storage

- 55M routes, but almost all share the same attribute bundle (minus AS_PATH)
- Ze uses **per-attribute-type pools with reference-counted dedup**: routes point to shared bundles
- AS_PATH stored separately: it's the one attribute that's almost always unique
- 72% single-prefix UPDATEs: no need for large batch structures, simple per-peer buffers suffice

---

## ExaBGP Compatibility: Your Scripts Still Work

### Migration tools

- `ze config migrate` converts ExaBGP configs to ze format automatically
- `ze exabgp plugin` runs existing ExaBGP processes with ze as the engine
- Bidirectional translation: ze JSON to ExaBGP JSON, ExaBGP commands to ze commands

### The promise

- Your existing scripts keep working: **upgrade the engine, not your tooling**

**The code is untested in production**
But: all ExaBGP compatibility tests pass and you can play with it.

---

## Fleet Management

*(WIP)

### Distribution

- Centralized config distribution over TLS
- Hub/client model with per-client secrets
- Version-hashed config fetch (only download on change)

### Resilience

- Two-phase config change: hub notifies, client fetches when ready
- **Partition resilient:** clients cache config locally, start from cache when hub is unreachable

### Gokrazy appliance

- Build, deploy, and manage Ze as a gokrazy appliance (no systemd, read-only A/B root, one write partition)
- Config loading with auto-revert, remote push/config ops
- Export/import for disaster recovery

---

## Getting Started

### Deployment

- **Single static binary:** copy it, run it. No runtime dependencies.
- **AGPL-3.0**: open source, network use included
- Linux (amd64, arm64), targets gokrazy appliance (no systemd, read-only A/B root, one write partition)
- Docker images if you need them

### Example: IXP route server config

```
plugin {
    internal rib { use bgp-rib; }
    internal rs  { use bgp-rs; }
}
bgp {
    peer member-a {
        connection {
            remote { ip 192.0.2.1; }
            local  { ip 192.0.2.254; }
        }
        session {
            asn { local 65500; remote 65001; }
            router-id 192.0.2.254;
            family {
                ipv4/unicast { prefix { maximum 10000; } }
            }
        }
    }
}
```

Two lines to enable the RIB and route server plugins. The rest is familiar BGP config.

---

## Since NetMcr (April 9th)

### The velocity

- The scope is no longer aspirational. BGP, IPsec, L2TP, firewall, VPP: all native, all in one binary.
  * New since April, IPsec, L2TP, VPP, firewall, traffic control, REST/gRPC API, gNMI, BMP, policy framework, config transactions.
  * The .claude system compounds: each feature builds on patterns the AI already knows. 836 learned summaries mean new sessions start with institutional memory.

- Adding a new protocol means writing a plugin. The engine, config, CLI, web, API, and metrics come for free.
  * Only **840k lines** / **23MB** of Go code
  * Only **36MB** of vendoring code

### Status

- **Pre-release:** looking for early adopters and feedback
- **Careful release:** the YANG model *is* the API. Once it's public, every consumer depends on it. Changing it means breaking people. Getting it right before v1 matters more than shipping fast.
- Community and documentation
- **github.com/ze-software/ze**

### Questions?

Thank you

*LINX, June 2026*
