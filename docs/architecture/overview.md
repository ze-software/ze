# Ze Architecture Overview

**Status:** Implementation Reference
**Last Updated:** 2026-05-29
**Canonical Reference:** See `core-design.md` for detailed design

---

## 1. What is Ze?

Ze is a Go network operating system with a plugin-first architecture. BGP is the
oldest and deepest subsystem, but the current tree also contains interface,
firewall, traffic-control, FIB, VPP, L2TP/PPP, PPPoE, IPsec/IKE, LDP, RSVP-TE,
telemetry, web, API, gNMI, MCP, storage, audit, and operational diagnostic
components.

Key characteristics:

- **Engine + Plugin model** - The engine supervises lifecycle, config, bus, and plugin manager without BGP-specific knowledge
- **Wire-first design** - Lazy parsing, zero-copy forwarding where possible
- **ExaBGP heritage** - Similar concepts, different architecture

### Non-Goals

- Backwards compatibility with itself (no releases yet)

OSPF and IS-IS have in-tree implementations under `internal/plugins/ospf` and
`internal/plugins/isis` (adjacency, areas/circuits, authentication, BFD client);
they are no longer a non-goal.
<!-- source: internal/plugins/ospf, internal/plugins/isis -- IGP plugin trees (OSPF 245, IS-IS 94 non-test .go files, 2026-07-08) -->


---

## 2. System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    BGP Subsystem  (internal/component/bgp/)                   │
│                                                                             │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌────────────────────────────┐    │
│   │ Peer 1  │  │ Peer 2  │  │ Peer N  │  │ Capability Negotiation     │    │
│   │  FSM    │  │  FSM    │  │  FSM    │  │ (ASN4 · AddPath · ExtNH)  │    │
│   └────┬────┘  └────┬────┘  └────┬────┘  │ ContextID · EncodingContext│    │
│        │            │            │        └────────────────────────────┘    │
│        └────────────┼────────────┘                                          │
│                     ▼                                                       │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │  Wire Layer  (Session Buffer · Message Parse · WireUpdate)         │   │
│   └────────────────────────────────┬────────────────────────────────────┘   │
│                                    ▼                                        │
│   ┌─────────────────────┐  ┌──────────────────┐                            │
│   │   Reactor           │─▶│ EventDispatcher  │                            │
│   │ (event loop,        │  │ (type-safe bridge,│                            │
│   │  BGP cache)         │  │  JSON encoder)   │                            │
│   └─────────────────────┘  └────────┬─────────┘                            │
└─────────────────────────────────────┼──────────────────────────────────────┘
                                      │  formatted events
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  Config Pipeline  (internal/component/config/)                              │
│  File -> Tree -> ConfigProvider roots                                       │
│    ├─ BGP root -> ResolveBGPTree() -> PeersFromTree() -> Reactor            │
│    └─ ConfigRoots -> Plugin auto-load and config delivery                   │
└─────────────────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────────────────┐
│               Plugin Infrastructure  (internal/component/plugin/)                     │
│    Plugin Registry · Process Manager · Hub · SDK · DirectBridge             │
└─────────────────────────────────────────────────────────────────────────────┘
                              │                 ▲
          YANG RPC + events   │                 │  commands (up)
          cached msg-ids      │                 │  update/forward/withdraw
                              ▼                 │
═══════════════════════ PROCESS BOUNDARY (TLS / net.Pipe) ═══════════════
                              │                 ▲
                              ▼                 │
                      ┌───────────────┐
                      │    Plugin     │  (Go/Python/Rust/etc.)
                      │  (RIB / RR)   │
                      └───────────────┘
```

**Key principles:**
- **Engine** supervises plugin manager and registered subsystems; it has no BGP-specific logic
- **BGP Subsystem** handles BGP protocol, TCP, FSM, wire parsing, and event dispatch
- **Config Pipeline** parses YANG-modeled config into roots consumed by subsystems and plugins
- **Plugin Infrastructure** manages lifecycle, process spawning, startup phases, message routing, and DirectBridge
- **Plugins and components** implement RIB storage, policy, route reflection, FIB programming, interface backends, firewall backends, telemetry exporters, and service integrations
- **Plugin IPC** uses newline-framed YANG RPC over `net.Pipe` or TLS connect-back, with DirectBridge for internal hot paths

---

## 3. Directory Structure

| Area | Location | Purpose |
|------|----------|---------|
| Main binary | `cmd/ze/` | CLI verbs, daemon startup, install/service/support tooling |
| Other binaries | Build tags: `ze_perf`, `ze_analyze` | Benchmarks, MRT/RIB analysis. ze-test, ze-chaos, ze-perf, and ze-analyze are build-tag variants of cmd/ze. |
| Components | `internal/component/` | Engine, BGP, config, CLI, command dispatcher, API, web, gNMI, MCP, interface, firewall, traffic, IPsec/IKE, L2TP, PPPoE, LDP, RSVP-TE, telemetry, storage, and related services |
| BGP subsystem | `internal/component/bgp/` | FSM, reactor, wire parsing, attributes, capabilities, NLRI, BGP plugins, and command handlers |
| Generic plugins | `internal/plugins/` | FIB, static routes, sysrib, sysctl, BFD, connected/kernel redistribution, policy routing, DHCP/TFTP/image services, VPP/firewall/traffic backends, L2TP helpers |
| Core utilities | `internal/core/` | Shared value types and services such as family, metrics, health, report bus, sysctl, routewatch, rib, paths, source IDs, and logging |
| Public packages | `pkg/` | Plugin SDK/RPC, Ze interfaces, and ZeFS storage |
| Functional tests | `test/` | `.ci` and `.et` tests plus interop and integration assets |
| Documentation | `docs/` | User, architecture, feature, plugin, migration, contributing, and research docs |
| Plans and history | `plan/` | Specs, learned summaries, deferrals, and design history |
| RFC references | `rfc/` | Full RFCs and short summaries |

---

## 4. Core Components

### 4.1 Plugin Server (`internal/component/plugin/server/`)

Manages plugin lifecycle and communication:
- Starts/stops external processes
- Routes commands to appropriate plugins
- Handles JSON events from reactor
<!-- source: internal/component/plugin/server/server.go -- plugin server implementation -->

### 4.2 Reactor (`internal/component/bgp/reactor/`)

Core event loop:
- Manages peer FSM instances
- Routes BGP messages
- Maintains message cache for zero-copy forwarding
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor struct -->

### 4.3 FSM (`internal/component/bgp/fsm/`)

RFC 4271 state machine:
- IDLE → CONNECT → ACTIVE → OPENSENT → OPENCONFIRM → ESTABLISHED
- Timer management (hold, keepalive, connect retry)
<!-- source: internal/component/bgp/fsm/ -- FSM state machine -->

### 4.4 Messages (`internal/component/bgp/message/`)

BGP message types:
- OPEN, UPDATE, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH
- Header parsing and validation
<!-- source: internal/component/bgp/message/ -- BGP message types -->

### 4.5 Attributes (`internal/core/bgp/attribute/`)

Path attributes:
- ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF
- Communities (standard, extended, large)
- MP_REACH_NLRI, MP_UNREACH_NLRI
<!-- source: internal/core/bgp/attribute/ -- path attribute types -->

### 4.6 NLRI (`internal/core/bgp/nlri/`)

Network Layer Reachability Information:
- INET (IPv4/IPv6 unicast)
- VPN (VPNv4/VPNv6)
- EVPN (5 route types)
- FlowSpec
- BGP-LS
- MUP (Mobile User Plane)

### 4.7 Capabilities (`internal/core/bgp/capability/`)

BGP capabilities and negotiation:
- Multiprotocol, ASN4, ADD-PATH
- Extended Message, Route Refresh
- Graceful Restart

### 4.8 Pool (`internal/component/bgp/attrpool/`)

Memory-efficient deduplication:
- Per-attribute-type pools
- Handle-based references
- Alternating buffer compaction
<!-- source: internal/component/bgp/attrpool/pool.go -- Pool struct -->
<!-- source: internal/component/bgp/attrpool/handle.go -- Handle type -->

---

## 5. CLI Commands

```bash
# Main daemon
ze bgp server <config>        # Run BGP server
ze config validate <config>   # Validate config

# Schema discovery
ze schema list                # List YANG schemas with namespaces
ze schema show <module>       # Show YANG content for a module
ze schema handlers            # Show handler → module mapping

# Testing
ze-peer --sink --port 1790    # Run test peer (sink mode)
ze-test bgp encode --all      # Run encoding tests

# Utilities
ze config validate <file>     # Validate config file
ze exabgp plugin <cmd>        # Run ExaBGP plugin with translation
```

---

## 6. Data Flow

### Incoming UPDATE

```
TCP recv → WireUpdate (lazy) → Plugin event (JSON + base64)
                                      │
                                      ▼
                              Plugin decides
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
              Store in RIB    Forward unchanged    Modify & forward
              (parse attrs)   (zero-copy)          (re-encode)
```

### Outgoing UPDATE

```
Plugin command → Parse → Build WireUpdate → Pack for peer caps → TCP send
```

---

## 7. Configuration

Ze uses a block-based config format:

```
environment {
    log {
        level info;
    }
}

peer peer-east {
    remote {
        ip 192.168.1.2;
        as 65002;
    }
    local-as 65001;
    local-address 192.168.1.1;

    family {
        ipv4 unicast;
        ipv6 unicast;
    }
}

process announce-routes {
    run "/usr/bin/python3 /path/to/script.py";
    encoder json;
}
```

---

## 8. Testing

### Unit Tests
```bash
make ze-unit-test             # Ze unit tests
```

### Functional Tests
```bash
make ze-functional-test       # All functional tests
ze-test bgp encode --list     # List N/TOTAL, id, and name
ze-test bgp encode 1 2 3      # Run specific tests
ze-test bgp encode --start 42 # Resume at id 42
```
<!-- source: internal/test/cli/cmd_bgp.go -- printRunUsage -->

### Linting
```bash
make lint                     # 27 linters via golangci-lint
```

---

## 9. Related Documents

| Document | Purpose |
|----------|---------|
| `core-design.md` | Canonical architecture reference |
| `buffer-architecture.md` | Iterators and lazy parsing |
| `pool-architecture.md` | Deduplication pool design |
| `wire/messages.md` | BGP message wire formats |
| `wire/attributes.md` | Path attribute formats |
| `wire/nlri.md` | NLRI type formats |
| `api/architecture.md` | Plugin communication protocol |
| `config/syntax.md` | Configuration syntax |

---

**Note:** This is an overview. See `core-design.md` for detailed architecture.
