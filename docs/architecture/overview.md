# Ze Architecture Overview

**Status:** Implementation Reference
**Last Updated:** 2026-02-04
**Canonical Reference:** See `core-design.md` for detailed design

---

## 1. What is Ze?

Ze is a Go BGP implementation with a plugin architecture. Key characteristics:

- **Engine + Plugin model** - Engine handles BGP protocol, plugins implement policy/RIB
- **Wire-first design** - Lazy parsing, zero-copy forwarding where possible
- **ExaBGP heritage** - Similar concepts, different architecture

### Non-Goals

- FIB manipulation (BGP protocol only, like ExaBGP)
- Backwards compatibility with itself (no releases yet)

---

## 2. System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    BGP Subsystem  (internal/plugins/bgp/)                   │
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
│  Config Pipeline  (internal/config/)                                        │
│  File → Tree → ResolveBGPTree()                                             │
│    ├─ PeersFromTree()            → peer definitions → Reactor               │
│    └─ ExtractPluginsFromTree()   → plugin config   → Plugin Infrastructure  │
└─────────────────────────────────────────────────────────────────────────────┘
┌─────────────────────────────────────────────────────────────────────────────┐
│               Plugin Infrastructure  (internal/plugin/)                     │
│    Plugin Registry · Process Manager · Hub · SDK · DirectBridge             │
└─────────────────────────────────────────────────────────────────────────────┘
                              │                 ▲
          JSON events (down)  │                 │  commands (up)
          + base64 wire bytes │                 │  update/forward/withdraw
                              ▼                 │
═══════════════════════ PROCESS BOUNDARY (Unix socket pairs) ═══════════════
                              │                 ▲
                              ▼                 │
                      ┌───────────────┐
                      │    Plugin     │  (Go/Python/Rust/etc.)
                      │  (RIB / RR)   │
                      └───────────────┘
```

**Key principles:**
- **BGP Subsystem** handles BGP protocol, TCP, FSM, wire parsing, event dispatch
- **Config Pipeline** parses config and feeds both BGP Subsystem and Plugin Infrastructure
- **Plugin Infrastructure** manages plugin lifecycle, process spawning, message routing
- **Plugins** implement RIB storage, policy, route reflection
- **Pipes** carry JSON events (with base64 wire bytes) and text commands

---

## 3. Directory Structure

```
ze/
├── cmd/
│   ├── ze/                     # Main CLI
│   │   ├── main.go
│   │   ├── bgp/                # ze bgp subcommands
│   │   ├── config/             # ze config subcommands
│   │   ├── schema/             # ze schema subcommands
│   │   ├── hub/                # ze hub subcommands
│   │   └── exabgp/             # ze exabgp subcommands
│   ├── ze-peer/                # BGP test peer tool
│   ├── ze-test/                # Functional test runner
│   └── ze-subsystem/           # Subsystem utility
│
├── internal/
│   ├── config/                 # Configuration pipeline
│   │   ├── loader.go           # Config loading
│   │   ├── parser.go           # Config parsing
│   │   ├── editor/             # Interactive config editor
│   │   └── migration/          # Config migration
│   │
│   ├── plugin/                 # Plugin infrastructure (generic, zero BGP knowledge)
│   │   ├── server.go           # Plugin server
│   │   ├── process.go          # External process management
│   │   ├── hub.go              # Message routing
│   │   ├── types.go            # Shared types
│   │   ├── registry/           # Plugin registry
│   │   └── all/                # Blank imports triggering init()
│   │
│   ├── plugins/                # Plugin implementations
│   │   ├── bgp/                # BGP subsystem (engine core)
│   │   │   ├── message/        # BGP messages (OPEN, UPDATE, etc.)
│   │   │   ├── attribute/      # Path attributes
│   │   │   ├── nlri/           # NLRI types (INET, VPN, EVPN, FlowSpec, etc.)
│   │   │   ├── capability/     # BGP capabilities + negotiation
│   │   │   ├── context/        # Encoding context registry
│   │   │   ├── fsm/            # Finite state machine
│   │   │   ├── reactor/        # Core event loop
│   │   │   ├── server/         # EventDispatcher (reactor → plugin bridge)
│   │   │   ├── wire/           # Wire format utilities
│   │   │   └── schema/         # YANG schema
│   │   │
│   │   ├── bgp-rib/            # RIB storage + dedup plugin
│   │   ├── bgp-rs/             # Route server plugin
│   │   ├── bgp-gr/             # Graceful restart plugin
│   │   ├── bgp-role/           # Leak prevention plugin
│   │   ├── bgp-adj-rib-in/     # Adj-RIB-In plugin
│   │   └── bgp-nlri-*/         # NLRI plugins (VPN, FlowSpec, EVPN, BGP-LS, etc.)
│   │
│   ├── hub/                    # Hub architecture
│   │   └── schema/             # Hub schema
│   │
│   ├── attrpool/               # Memory pools
│   │   ├── pool.go             # Core Pool type
│   │   ├── handle.go           # Handle type
│   │   └── compaction.go       # Compaction logic
│   │
│   ├── store/                  # Deduplication stores
│   ├── slogutil/               # Logging utilities
│   ├── source/                 # Source identification
│   ├── selector/               # Peer selection
│   ├── env/                    # Environment handling
│   ├── exabgp/                 # ExaBGP compatibility bridge
│   ├── yang/                   # YANG modules
│   ├── tmpfs/                  # Temporary filesystem
│   └── test/                   # Test utilities
│       ├── ci/                 # CI test format
│       ├── peer/               # Test peer library
│       ├── runner/             # Test runner
│       └── syslog/             # Syslog testing
│
├── test/                       # Functional tests
│   ├── encode/                 # Encoding tests (.ci files)
│   ├── decode/                 # Decoding tests
│   ├── parse/                  # Config parsing tests
│   ├── plugin/                 # Plugin tests
│   ├── exabgp/                 # ExaBGP compatibility tests
│   └── integration/            # Integration tests
│
├── docs/
│   ├── architecture/           # Architecture documentation
│   └── plan/                   # Specs and plans
│
└── rfc/                        # RFC references
    ├── full/                   # Full RFC text
    └── short/                  # RFC summaries
```

---

## 4. Core Components

### 4.1 Plugin Server (`internal/plugin/server.go`)

Manages plugin lifecycle and communication:
- Starts/stops external processes
- Routes commands to appropriate plugins
- Handles JSON events from reactor

### 4.2 Reactor (`internal/plugins/bgp/reactor/`)

Core event loop:
- Manages peer FSM instances
- Routes BGP messages
- Maintains message cache for zero-copy forwarding

### 4.3 FSM (`internal/plugins/bgp/fsm/`)

RFC 4271 state machine:
- IDLE → CONNECT → ACTIVE → OPENSENT → OPENCONFIRM → ESTABLISHED
- Timer management (hold, keepalive, connect retry)

### 4.4 Messages (`internal/plugins/bgp/message/`)

BGP message types:
- OPEN, UPDATE, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH
- Header parsing and validation

### 4.5 Attributes (`internal/plugins/bgp/attribute/`)

Path attributes:
- ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF
- Communities (standard, extended, large)
- MP_REACH_NLRI, MP_UNREACH_NLRI

### 4.6 NLRI (`internal/plugins/bgp/nlri/`)

Network Layer Reachability Information:
- INET (IPv4/IPv6 unicast)
- VPN (VPNv4/VPNv6)
- EVPN (5 route types)
- FlowSpec
- BGP-LS
- MUP (Mobile User Plane)

### 4.7 Capabilities (`internal/plugins/bgp/capability/`)

BGP capabilities and negotiation:
- Multiprotocol, ASN4, ADD-PATH
- Extended Message, Route Refresh
- Graceful Restart

### 4.8 Pool (`internal/component/bgp/attrpool/`)

Memory-efficient deduplication:
- Per-attribute-type pools
- Handle-based references
- Alternating buffer compaction

---

## 5. CLI Commands

```bash
# Main daemon
ze bgp server <config>        # Run BGP server
ze bgp validate <config>      # Validate config

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

peer 192.168.1.2 {
    local-as 65001;
    peer-as 65002;
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
ze-test bgp encode --list     # List encoding tests
ze-test bgp encode 0 1 2      # Run specific tests
```

### Linting
```bash
make lint                     # 26 linters via golangci-lint
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
