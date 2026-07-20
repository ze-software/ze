# Every feature Ze ships.

48 features plus a spec'd roadmap, color-coded by category. A card's category is how the feature fits into the system: operate, routing, services, automate, observe, secure, or platform. Everything shipped runs in both daemon and appliance modes unless a card says otherwise.

## Built for demanding operators.

Ze owns its BGP engine, configuration model, plugin system, operator surfaces, minimal appliance runtime, and diagnostics as one product.

### AI-Friendly Surfaces

*automate* -- `MCP` `Generated` `AI tools`

- **MCP** exposes CLI/API commands
- AI tools inspect **structured output**
- Plugins become **discoverable tools**

[Learn more](https://ze-software.net/docs/features/ai-first/)

### SSH CLI

*operate* -- `Built-in SSH` `RBAC`

- Manage Ze without **OS shell** accounts
- **Profiles**, audit, and accounting
- **commit**, rollback, diff, completion

[Learn more](https://ze-software.net/docs/features/cli-commands/)

### YANG Configuration

*operate* -- `YANG` `ExaBGP`

- Schema-driven **validation**
- **One model** feeds every surface
- **Plugin** defined config and commands

[Learn more](https://ze-software.net/docs/features/configuration/)

### Output Formatting

*operate* -- `Shell-like pipes` `Offline`

- **table**, **json**, **yaml**, **ndjson**
- **match**, **count**, **first**/**last**
- Offline via **ze format**

[Learn more](https://ze-software.net/docs/features/formatting/)

### Web Workbench

*operate* -- `HTMX` `SSE`

- YANG-driven **config tree**
- Same **CLI grammar** in browser
- **Live updates** via SSE

[Learn more](https://ze-software.net/docs/features/web-interface/)

### Looking Glass

*operate* -- `Routes` `Topology` `Birdwatcher`

- Peer and **route viewer**
- **Topology** graph
- SSE streaming for **live state**

[Learn more](https://ze-software.net/docs/features/looking-glass/)

### System Readiness

*operate* -- `ze doctor` `ze explain`

- Offline **pre-start checks**
- Health, warnings, and **errors**
- Structured **remediation** with `ze explain`

[Learn more](https://ze-software.net/docs/guide/production-diagnostics/)

### Native BGP Engine

*routing* -- `BGP` `IPv4/IPv6` `FlowSpec`

- Full implementation in **Go**
- **Lazy parsing**, buffer-first encoding
- Negotiated **capabilities**

[Learn more](https://ze-software.net/docs/features/bgp-protocol/)

### Static Routes

*routing* -- `ECMP` `BFD` `PBR`

- Named tables, **policy routing**
- **BFD**-tracked failover
- Multi-path **ECMP** groups

[Learn more](https://ze-software.net/docs/guide/static-routes/)

### BFD

*routing* -- `RFC 5880` `Auth`

- **Single-hop** and **multi-hop**
- GTSM, jitter, **BGP** integration
- SHA1/MD5 **auth**, echo mode

[Learn more](https://ze-software.net/docs/features/bgp-protocol/)

### MRT Recording

*routing* -- `RFC 6396` `Analysis`

- Updates, messages, **RIB snapshots**
- **Strftime** file rotation
- Show, inject, replay, **filter**

[Learn more](https://ze-software.net/docs/guide/mrt-analysis/)

### DNS Resolver

*services* -- `Cache` `Pipes`

- Built-in **cached** resolver
- **| resolve** and **| origin** pipe operators
- No external **daemon** needed

[Learn more](https://ze-software.net/docs/features/dns-resolver/)

### Plugin System

*automate* -- `ExaBGP` `RPKI` `Policy`

- Plugins add **commands**, RPCs, events
- YANG roots join **CLI** and web
- Independent, **composable**

[Learn more](https://ze-software.net/docs/features/plugins/)

### Programmable

*automate* -- `REST` `gRPC` `gNMI`

- **REST API**, **gRPC**, **gNMI**
- Shared engine for **identical output**
- Automate from **any language**

[Learn more](https://ze-software.net/docs/features/api-commands/)

### AI-First Design

*automate* -- `Self-describing` `Skills`

- **Self-describing** command catalog from the live binary
- Every command becomes **automation** surface
- Structured **diagnostics** and repair plans

[Learn more](https://ze-software.net/docs/features/ai-first/)

### MCP Integration

*automate* -- `MCP` `OAuth 2.1`

- **Streamable HTTP** transport, OAuth 2.1 resource server
- Server-initiated **elicitation**, task-augmented tool calls
- **MCP Apps UI** with embedded panels

[Learn more](https://ze-software.net/docs/features/mcp-integration/)

### ExaBGP Compatibility

*automate* -- `Migration` `Bridge`

- Automatic config **migration**
- **Plugin bridge** for existing workflows
- Smooth **transition** path

[Learn more](https://ze-software.net/docs/features/exabgp-compatibility/)

### Evidence Over Claims

*observe* -- `Fuzz` `Interop` `Docker`

- Unit, functional, **fuzz**, chaos
- Performance **benchmarks**
- **Interop** vs FRR, BIRD, GoBGP

[Learn more](https://ze-software.net/docs/features/interoperability-testing/)

### Development Activity

*observe* -- `Heatmap` `Live data`

- A year of **commits** and added lines, at a glance
- Regenerated from git history, not curated
- Top commit and line days, ranked

[Learn more](https://ze-software.net/activity/)

### Prometheus Telemetry

*observe* -- `Netdata` `Prometheus`

- **138 metrics** from /proc and /sys
- **Netdata** naming, drop-in replacement
- Existing **Grafana** dashboards keep working

[Learn more](https://ze-software.net/docs/guide/monitoring/)

### Health Registry

*observe* -- `HTTP` `503`

- **/health** HTTP endpoint
- Per-component **status** checks
- BGP, FIB, IPsec, L2TP, **VPP**

[Learn more](https://ze-software.net/docs/features/)

### Host Inventory

*observe* -- `CPU` `NIC` `SMART`

- **CPU**, NIC, DMI, memory, thermal
- **SMART** disk health and self-tests
- **JSON** output for pipelines

[Learn more](https://ze-software.net/docs/features/)

### Crash Capture

*observe* -- `Panic` `Syslog`

- Automatic **panic** stack traces
- Ring buffer **context** (last 64 entries)
- **show crashes** CLI command

[Learn more](https://ze-software.net/docs/features/)

### Tech-Support Bundle

*observe* -- `Offline` `JSON`

- **20 modules**, pure Go, no shell-outs
- Structured **JSON** per module
- Privacy-by-default, **gokrazy**-safe

[Learn more](https://ze-software.net/docs/features/)

### Production Diagnostics

*observe* -- `CLI` `MCP`

- 11 built-in tools replacing **ss, dmesg, lsof**
- **tcpdump**, traceroute, ping, mtr
- All exposed via **MCP** for AI debugging

[Learn more](https://ze-software.net/docs/guide/production-diagnostics/)

### Secure by Default

*secure* -- `SSH` `RBAC` `RPKI` `ASPA`

- **SSH** access to the CLI
- **RPKI** route origin validation
- No **other daemons** needed

[Learn more](https://ze-software.net/docs/features/plugins/)

### TACACS+ AAA

*secure* -- `RFC 8907` `Accounting`

- SSH login via **TACACS+**
- Command **accounting** START/STOP
- Server failover, **local** fallback

[Learn more](https://ze-software.net/docs/guide/tacacs/)

### Audit Trail

*secure* -- `Commits` `Auth`

- Config **commit**, discard, reload
- Failed **auth** across all surfaces
- Filter by action, actor, **time**

[Learn more](https://ze-software.net/docs/guide/audit/)

### PKI Store

*secure* -- `X.509` `TLS`

- YANG-modeled **certificate** management
- Chain validation, **expiry** checks
- Shared by IPsec, **TLS**, mutual auth

[Learn more](https://ze-software.net/docs/features/)

### Minimal Appliance Mode

*platform* -- `Appliance` `Server`

- **Kernel, init, Ze** runtime
- No **package manager** or general shell
- **ISO/PXE** bare-metal install
- Linux server with **systemd**

[Learn more](https://ze-software.net/docs/guide/appliance/)

### Runs Itself

*platform* -- `Update` `Systemd`

- Binary **self-update**
- Built-in **readiness** checks
- No **orchestrator** needed

[Learn more](https://ze-software.net/docs/features/introspection/)

### Docker Support

*platform* -- `Daemon only` `Scratch` `Compose`

- **Static binary** on scratch base
- **Compose** support included
- Optional **build tags**

[Learn more](https://ze-software.net/docs/features/)

## Experimental and growing.

Implemented and tested, not yet production-proven.

> These still need deployment evidence or hardening before production claims. Configuration may change.

### IPsec VPN

*services / Experimental* -- `IKEv2` `X.509` `EAP`

- Full **IKEv2** engine, rekeying, DPD
- **NAT-T**, keepalive, XFRM interfaces
- EAP-MSCHAPv2, **EAP-TLS**, road warrior

[Learn more](https://ze-software.net/docs/features/)

### L2TPv2 BNG

*services / Experimental* -- `PPP` `RADIUS` `CQM`

- RFC 2661 **LNS and LAC** with PPP
- **RADIUS** auth, accounting, CoA
- CQM monitoring, **shaping**, web UI

[Learn more](https://ze-software.net/docs/guide/l2tp/)

### PPPoE Access

*services / Experimental* -- `RFC 2516` `PPP`

- **Access concentrator** with discovery FSM
- Shared **PPP driver** with L2TP
- HMAC-SHA256 **cookie**, rate limiting

[Learn more](https://ze-software.net/docs/guide/pppoe/)

### Interface Management

*services / Experimental* -- `Netlink` `DHCP`

- Ethernet, VLAN, bridge, **WireGuard**
- 8 tunnel kinds, **DHCP** client
- NTP sync, **offload** tuning, mirroring

[Learn more](https://ze-software.net/docs/features/interfaces/)

### Firewall

*services / Experimental* -- `nftables` `NAT`

- **15 match** types, 19 actions
- SNAT, DNAT, **masquerade**
- FlowSpec-to-firewall **bridge**

[Learn more](https://ze-software.net/docs/guide/firewall/)

### Policy Routing

*services / Experimental* -- `nftables` `PBR`

- **L3/L4 match** criteria
- Table steering, **next-hop** actions
- TCP-MSS clamping, **interface** wildcards

[Learn more](https://ze-software.net/docs/guide/policy-routing/)

### VPP Data Plane

*services / Experimental* -- `DPDK` `GoVPP`

- **FIB** programming via GoVPP
- MPLS **label** operations
- Per-interface **Prometheus** metrics

[Learn more](https://ze-software.net/docs/guide/vpp/)

### MPLS / LDP / RSVP-TE

*routing / Experimental* -- `Labels` `Signaling`

- Kernel MPLS FIB, **push/swap/pop**
- LDP **discovery** and sessions
- RSVP-TE **ERO**, bandwidth admission

[Learn more](https://ze-software.net/docs/features/)

### OSPFv2 / OSPFv3

*routing / Experimental* -- `RFC 2328` `RFC 5340` `ECMP`

- One **ospf** engine, IPv4 and IPv6 address families
- SPF/ABR, **NSSA**, virtual links, NBMA/P2MP
- Redistribution, **SR**, BFD, graceful restart

[Learn more](https://ze-software.net/docs/guide/ospf/)

### IS-IS

*routing / Experimental* -- `ISO 10589` `Dual-stack`

- **L1/L2** link-state IGP over Layer 2
- RFC 5304/5310 **authentication**, key chains
- Dual-stack **IPv6**, redistributes with BGP

[Learn more](https://ze-software.net/docs/guide/isis/)

### VRRP

*routing / Experimental* -- `RFC 9568` `RFC 3768` `Virtual MAC`

- First-hop **gateway redundancy**, IPv4 and IPv6
- Per-group **virtual-MAC** macvlan for L2 failover
- **keepalived** interop, compile-out

[Learn more](https://ze-software.net/docs/guide/vrrp/)

### Flow Export

*observe / Experimental* -- `sFlow` `NetFlow` `IPFIX`

- **sFlow v5**, NetFlow v9, IPFIX
- Packet sampling, **conntrack** flows
- BGP **next-hop** enrichment

[Learn more](https://ze-software.net/docs/guide/flow-export/)

### ISO and PXE Install

*platform / Experimental* -- `PXE` `ISO`

- **PXE** bare-metal provisioning
- Installer **ISO** media
- Local **systemd** install and uninstall

[Learn more](https://ze-software.net/docs/guide/ze-install/)

### Kernel Tunables

*platform / Experimental* -- `Sysctl` `Profiles`

- Three-layer **precedence**
- Named **profiles** (DSR, router, hardened)
- Originals **restored** on stop

[Learn more](https://ze-software.net/docs/features/)

### AS112 Anycast DNS

*services / Experimental* -- `AS112` `Anycast`

- Authoritative **sink zones** on four fixed anycast addresses (RFC 7534/7535)
- Conditional **BGP origination** via healthcheck-gated watchdog
- Anycast IPs bound on **lo** automatically, never operator-typed

[Learn more](https://ze-software.net/docs/guide/as112/)

### Segment Routing

*routing / Experimental* -- `SAFI 73` `SRv6`

- **SR-Policy** NLRI (RFC 9830), SAFI 73
- MPLS and **SRv6** binding SID, tunnel encap
- **ExaBGP bridge** for SR-Policy migration

[Learn more](https://codeberg.org/thomas-mangin/ze/src/branch/main/internal/component/bgp/plugins/nlri/srpolicy)

## Spec'd, not built.

Aspirations with written, reviewed specs. Nothing here is usable today.

> Every card links to a pending spec in the main repo's `plan/` directory, where captured intent moves from skeleton to design to ready to in-progress, and a spec is deleted only when the work ships.

### OSPF L3VPN PE-CE

*routing / Spec'd* -- `RFC 4576` `RFC 4577` `L3VPN`

- PE-CE **DN bit** loop prevention
- Domain ID, route type, **VPN route tag**
- Blocked on **VRF/MPLS L3VPN** infrastructure

[Learn more](https://codeberg.org/thomas-mangin/ze/src/branch/main/plan/spec-ospf-ext-13-l3vpn-dn-bit.md)

### VRF

*routing / Spec'd* -- `VRF` `L3VPN`

- VRF as a **first-class** concept
- Per-VRF **BGP stacks**, YANG config
- Kernel **VRF devices**, table binding

[Learn more](https://codeberg.org/thomas-mangin/ze/src/branch/main/plan/spec-vrf-0-umbrella.md)

### Fleet Management

*automate / Spec'd* -- `Registry` `Rollout`

- Device **registry**, config templates
- **Staged rollout**, config freeze
- Fleet **audit trail**, inventory health

[Learn more](https://codeberg.org/thomas-mangin/ze/src/branch/main/plan/spec-fleet-0-umbrella.md)

### IRR Route Filtering

*secure / Spec'd* -- `IRR` `as-set`

- **Prefix-lists** from IRR data
- bgpq4-style, **live in the engine**
- Automatic from the peer's **remote ASN**

[Learn more](https://codeberg.org/thomas-mangin/ze/src/branch/main/plan/spec-filter-irr.md)

### Kernel Lockdown

*secure / Spec'd* -- `Lockdown` `Integrity`

- Kernel **lockdown** integrity mode
- Blocks unsigned **modules**, kexec, /dev/mem
- Design **reviewed**, not yet scheduled

[Learn more](https://codeberg.org/thomas-mangin/ze/src/branch/main/plan/spec-kernel-lockdown-hardening.md)

### Cloud-Init Provisioning

*platform / Spec'd* -- `Cloud-init` `User-data`

- Appliance identity from **cloud metadata**
- SSH keys and config via **user-data**
- No **pre-baked** seed image needed

[Learn more](https://codeberg.org/thomas-mangin/ze/src/branch/main/plan/spec-install-9-cloud-init.md)
