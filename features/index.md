# Every feature Ze ships.

52 shipped or experimental feature cards. Each card's category shows where the feature fits: operate, routing, services, automate, observe, secure, or platform. Everything shipped runs in both daemon and appliance modes unless a card says otherwise.

## Built for demanding operators.

Ze starts with a configuration and protocol engine. The shipped network operating system adds BGP, interface management, FIB programming, plugins, operator tools, a minimal appliance runtime, and diagnostics as one product.

### Policy Routing

*services* -- `nftables` `PBR`

- **L3/L4 match** criteria
- Table steering, **next-hop** actions
- TCP-MSS clamping, **interface** wildcards

[Learn more](https://ze-software.net/guides/policy-routing/)

## Experimental and growing.

Implemented, still waiting for production evidence or for a named part to be finished.

> These still need deployment evidence, hardening, or a named part implemented or proven before production claims. The feature status page lists the status of each. Configuration may change.

### Output Formatting

*operate / Experimental* -- `Shell-like pipes` `Offline`

- **table**, **json**, **yaml**, **ndjson**
- **match**, **count**, **first**/**last**
- Offline via **ze pipe**
- **Every command**, no rendering flags

[Learn more](https://ze-software.net/features/formatting/)

### Web Workbench

*operate / Experimental* -- `HTMX` `SSE`

- YANG-driven **config tree**
- Same **CLI grammar** in browser
- **Live updates** via SSE

[Learn more](https://ze-software.net/features/web-interface/)

### Looking Glass

*operate / Experimental* -- `Routes` `Topology` `Birdwatcher`

- Peer and **route viewer**
- **Topology** graph
- SSE streaming for **live state**

[Learn more](https://ze-software.net/features/looking-glass/)

### Development Activity

*observe / Experimental* -- `Heatmap` `Rebuilt each publish`

- A year of **commits** and added lines, at a glance
- Built from git history each time
- Current Go code composition

[Learn more](https://ze-software.net/project/activity/)

### Tech-Support Bundle

*observe / Experimental* -- `Offline` `JSON`

- **20 modules**, pure Go, no shell-outs
- Structured **JSON** per module
- Privacy-by-default, **gokrazy**-safe

[Learn more](https://ze-software.net/features/)

### Audit Trail

*secure / Experimental* -- `Commits` `Auth`

- Config **commit**, discard, and reload
- Failed **auth** on every interface
- Filter by action, actor, and **time**

[Learn more](https://ze-software.net/guides/audit/)

### PKI Store

*secure / Experimental* -- `X.509` `TLS`

- YANG-modelled **certificate** management
- Chain validation, **expiry** checks
- Shared by IPsec, **TLS**, mutual auth
- **Local CA** issues 24-hour certs to Ze's components

[Learn more](https://ze-software.net/features/)

### IPsec VPN

*services / Experimental* -- `IKEv2` `X.509` `EAP`

- Full **IKEv2** engine, rekeying, DPD
- **NAT-T**, keepalive, XFRM interfaces
- EAP-MSCHAPv2, **EAP-TLS**, road warrior
- **CRL** and OCSP revocation checks on EAP-TLS

[Learn more](https://ze-software.net/features/)

### L2TPv2 BNG

*services / Experimental* -- `PPP` `RADIUS` `CQM`

- RFC 2661 **LNS and LAC** with PPP
- **RADIUS** auth, accounting, CoA
- CQM monitoring, **shaping**, web UI

[Learn more](https://ze-software.net/guides/l2tp/)

### PPPoE Access

*services / Experimental* -- `RFC 2516` `PPP`

- **Access concentrator** with discovery FSM
- Shared **PPP driver** with L2TP
- **PAP**, **CHAP-MD5**, and **MS-CHAPv2** authentication

[Learn more](https://ze-software.net/guides/pppoe/)

### Interface Management

*services / Experimental* -- `Netlink` `DHCP`

- Ethernet, VLAN, bridge, **WireGuard**
- 8 tunnel kinds, **DHCP** client
- NTP sync, **offload** tuning, mirroring
- **IPv6 Router Advertisements**, the radvd role

[Learn more](https://ze-software.net/features/interfaces/)

### Firewall

*services / Experimental* -- `nftables` `NAT`

- **15 match** types, 19 actions
- SNAT, DNAT, **masquerade**
- FlowSpec-to-firewall **bridge**
- **DNS-sourced** address groups, TTL-tracked

[Learn more](https://ze-software.net/guides/firewall/)

### VPP Data Plane

*services / Experimental* -- `DPDK` `GoVPP`

- **FIB** programming via GoVPP
- MPLS **label** operations
- Per-interface **Prometheus** metrics

[Learn more](https://ze-software.net/guides/vpp/)

### MPLS / LDP / RSVP-TE

*routing / Experimental* -- `Labels` `Signaling`

- Kernel MPLS FIB, **push/swap/pop**
- LDP **discovery** and sessions
- RSVP-TE **ERO**, bandwidth admission

[Learn more](https://ze-software.net/features/)

### OSPFv2 / OSPFv3

*routing / Experimental* -- `RFC 2328` `RFC 5340` `ECMP`

- One **ospf** engine, IPv4 and IPv6 address families
- SPF/ABR, **NSSA**, virtual links, NBMA/P2MP
- Redistribution, **SR**, BFD, graceful restart
- Interface cost priced from **link speed**

[Learn more](https://ze-software.net/guides/ospf/)

### IS-IS

*routing / Experimental* -- `ISO 10589` `Dual-stack`

- **L1/L2** link-state IGP over Layer 2
- RFC 5304/5310 **authentication**, key chains
- Dual-stack **IPv6**, redistributes with BGP

[Learn more](https://ze-software.net/guides/isis/)

### VRRP

*routing / Experimental* -- `RFC 9568` `RFC 3768` `Virtual MAC`

- First-hop **gateway redundancy**, IPv4 and IPv6
- Per-group **virtual-MAC** macvlan for L2 failover
- **keepalived** interop, compile-out
- **Interface tracking**, accept-mode

[Learn more](https://ze-software.net/guides/vrrp/)

### Flow Export

*observe / Experimental* -- `sFlow` `NetFlow` `IPFIX`

- **sFlow v5**, NetFlow v9, IPFIX
- Packet sampling, **conntrack** flows
- BGP **next-hop** enrichment

[Learn more](https://ze-software.net/guides/flow-export/)

### DDoS and Anomaly Detection

*secure / Experimental* -- `DDoS` `Anomaly` `FlowSpec`

- **Volumetric** and behavioral detection
- Bounded incident **history** with durations
- Local and upstream **auto-mitigation**

[Learn more](https://ze-software.net/guides/anomaly/)

### ISO and PXE Install

*platform / Experimental* -- `PXE` `ISO`

- **PXE** bare-metal provisioning
- Current amd64 installation **ISO under 150 MB**
- Local **systemd** install and uninstall

[Learn more](https://ze-software.net/guides/ze-install/)

### Kernel Tunables

*platform / Experimental* -- `Sysctl` `Profiles`

- Three-layer **precedence**
- Named **profiles** (DSR, router, hardened)
- Originals **restored** on stop

[Learn more](https://ze-software.net/features/)

### AS112 Anycast DNS

*services / Experimental* -- `AS112` `Anycast`

- Authoritative **sink zones** on four fixed anycast addresses (RFC 7534/7535)
- Conditional **BGP origination** via healthcheck-gated watchdog
- Anycast IPs bound on **lo** automatically, never operator-typed

[Learn more](https://ze-software.net/guides/as112/)

### Segment Routing

*routing / Experimental* -- `SAFI 73` `SRv6`

- **SR-Policy** NLRI (RFC 9830), SAFI 73
- MPLS and **SRv6** binding SID, tunnel encap
- **ExaBGP bridge** for SR-Policy migration

[Learn more](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy)

### Fleet Management

*automate / Experimental* -- `Managed config` `TLS hub`

- Per-client **configuration** and **pinned** hub certificate
- Cached config with **reconnect** and heartbeat
- Version hashing and **two-phase** fetch

[Learn more](https://ze-software.net/guides/fleet-config/)

### IRR Route Filtering

*secure / Experimental* -- `IRR` `as-set`

- **Prefix-lists** from IRR data, live in the engine
- Sourced from **PeeringDB** and **RADB**
- Opt-in per peer, group or **global filter chain**

[Learn more](https://ze-software.net/guides/irr-filtering/)

### AI Tool Interfaces

*automate / Experimental* -- `MCP` `Generated` `AI tools`

- **MCP** exposes CLI/API commands
- AI tools read **structured output**
- Plugins expose **discoverable tools**

[Learn more](https://ze-software.net/features/ai-first/)

### SSH CLI

*operate / Experimental* -- `Built-in SSH` `RBAC`

- Manage Ze without **OS shell** accounts
- **Profiles**, audit, and accounting
- **commit**, rollback, diff, completion

[Learn more](https://ze-software.net/features/cli-commands/)

### YANG Configuration

*operate / Experimental* -- `YANG` `ExaBGP`

- Schema-driven **validation**
- **One model** feeds every surface
- **Plugin** defined config and commands

[Learn more](https://ze-software.net/features/bgp-configuration/)

### System Readiness

*operate / Experimental* -- `ze doctor` `ze explain`

- Offline **pre-start checks**
- Health, warnings, and **errors**
- Structured **remediation** with `ze explain`

[Learn more](https://ze-software.net/guides/production-diagnostics/)

### Native BGP Engine

*routing / Experimental* -- `BGP` `IPv4/IPv6` `FlowSpec`

- Full implementation in **Go**
- **Lazy parsing**, buffer-first encoding
- Negotiated **capabilities**

[Learn more](https://ze-software.net/features/bgp-protocol/)

### Static Routes

*routing / Experimental* -- `ECMP` `BFD` `PBR`

- Named tables, **policy routing**
- **BFD**-tracked failover
- Multi-path **ECMP** groups

[Learn more](https://ze-software.net/guides/static-routes/)

### BFD

*routing / Experimental* -- `RFC 5880` `Auth`

- **Single-hop** and **multi-hop**
- GTSM, jitter, **BGP** integration
- SHA1/MD5 **auth**, echo mode
- **Strict mode** holds a BGP peer down until BFD is up

[Learn more](https://ze-software.net/features/bgp-protocol/)

### MRT Recording

*routing / Experimental* -- `RFC 6396` `Analysis`

- Updates, messages, **RIB snapshots**
- **Strftime** file rotation
- Show, inject, replay, **filter**

[Learn more](https://ze-software.net/guides/mrt-analysis/)

### DNS Resolver

*services / Experimental* -- `Cache` `Pipes`

- Built-in **cached** resolver
- **| resolve** and **| origin** pipe operators
- No external **daemon** needed

[Learn more](https://ze-software.net/features/dns-resolver/)

### Plugin System

*automate / Experimental* -- `ExaBGP` `RPKI` `Policy`

- Plugins add **commands**, RPCs, events
- YANG roots join **CLI** and web
- Independent, **composable**
- Each plugin declares what its **failure** means

[Learn more](https://ze-software.net/reference/plugins/)

### Programmable

*automate / Experimental* -- `REST` `gRPC` `gNMI`

- **REST API**, **gRPC**, **gNMI**
- Shared engine for **identical output**
- Automate from **any language**

[Learn more](https://ze-software.net/features/api-commands/)

### AI-First Design

*automate / Experimental* -- `Self-describing` `Skills`

- **Self-describing** command catalogue from the live binary
- Every command is an **automation** surface
- Structured **diagnostics** and repair plans

[Learn more](https://ze-software.net/features/ai-first/)

### MCP Integration

*automate / Experimental* -- `MCP` `OAuth 2.1`

- **Streamable HTTP** transport, OAuth 2.1 resource server
- Server-initiated **elicitation**, task-augmented tool calls
- **MCP Apps UI** with embedded panels

[Learn more](https://ze-software.net/features/mcp-integration/)

### ExaBGP Compatibility

*automate / Experimental* -- `Migration` `Bridge`

- Automatic config **migration**
- **Plugin bridge** for existing workflows
- Migration path for existing scripts

[Learn more](https://ze-software.net/features/exabgp-compatibility/)

### Evidence Over Claims

*observe / Experimental* -- `Fuzz` `Interop` `Docker`

- Unit, functional, **fuzz**, chaos
- Performance **benchmarks**
- **Interop** vs FRR, BIRD, GoBGP

[Learn more](https://ze-software.net/features/interoperability-testing/)

### Prometheus Telemetry

*observe / Experimental* -- `Netdata` `Prometheus`

- **Prometheus metrics** from /proc and /sys
- **Netdata** metric names and labels
- Built for existing **Grafana** dashboards

[Learn more](https://ze-software.net/guides/monitoring/)

### Health Registry

*observe / Experimental* -- `HTTP` `503`

- **/health** HTTP endpoint
- Per-component **status** checks
- BGP, FIB, IPsec, L2TP, **VPP**

[Learn more](https://ze-software.net/features/)

### Host Inventory

*observe / Experimental* -- `CPU` `NIC` `SMART`

- **CPU**, NIC, DMI, memory, thermal
- **SMART** disk health and self-tests
- **JSON** output for pipelines

[Learn more](https://ze-software.net/features/)

### Crash Capture

*observe / Experimental* -- `Panic` `Syslog`

- Automatic **panic** stack traces
- Ring buffer **context** (last 64 entries)
- **show crashes** CLI command

[Learn more](https://ze-software.net/features/)

### Production Diagnostics

*observe / Experimental* -- `CLI` `MCP`

- Built-in tools replacing **ss, dmesg, lsof**
- **tcpdump**, traceroute, ping, mtr
- All exposed via **MCP** for AI debugging

[Learn more](https://ze-software.net/guides/production-diagnostics/)

### Secure by Default

*secure / Experimental* -- `SSH` `RBAC` `RPKI` `ASPA`

- **SSH** access to the CLI
- **RPKI** route origin validation
- No **other daemons** needed

[Learn more](https://ze-software.net/reference/plugins/)

### TACACS+ AAA

*secure / Experimental* -- `RFC 8907` `Accounting`

- SSH login via **TACACS+**
- Command **accounting** START/STOP
- Server failover, **local** fallback

[Learn more](https://ze-software.net/guides/tacacs/)

### Minimal Appliance Mode

*platform / Experimental* -- `Appliance` `Server`

- **Kernel, init, Ze** runtime
- No **package manager** or general shell
- **ISO/PXE** bare-metal install
- Linux server with **systemd**

[Learn more](https://ze-software.net/guides/appliance/)

### Runs Itself

*platform / Experimental* -- `Update` `Systemd`

- Binary **self-update**
- Built-in **readiness** checks
- No **orchestrator** needed

[Learn more](https://ze-software.net/features/introspection/)

### Docker Support

*platform / Experimental* -- `Daemon only` `Scratch` `Compose`

- **Static binary** on scratch base
- **Compose** support included
- Optional **build tags**

[Learn more](https://ze-software.net/features/)

### Feature Gates

*platform / Experimental* -- `36 subsystems` `Default on`

- Compile out **whole subsystems**, BGP included
- Smaller binary, smaller **attack surface**
- Config **fails closed** on blocks the build lacks

[Learn more](https://ze-software.net/guides/quickstart/)

## Release roadmap

The [release inventory](https://ze-software.net/project/roadmap/) lists remaining release work items and nice-to-haves from committed specs. It is an inventory preview pending owner classification.
