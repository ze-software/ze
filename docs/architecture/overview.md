# Ze Architecture Plan

**Status:** ✅ Implementation Complete (Phase 12-13 ongoing)
**Created:** 2025-12-19
**Purpose:** Comprehensive architecture reference

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [System Overview](#2-system-overview)
3. [Core Design Decisions](#3-core-design-decisions)
4. [Component Architecture](#4-component-architecture)
5. [Data Flow](#5-data-flow)
6. [Memory Management](#6-memory-management)
7. [Concurrency Model](#7-concurrency-model)
8. [Interface Contracts](#8-interface-contracts)
9. [Dependency Graph](#9-dependency-graph)
10. [ExaBGP Compatibility](#10-exabgp-compatibility)
11. [Testing Strategy](#11-testing-strategy)
12. [Implementation Phases](#12-implementation-phases)
13. [Risk Analysis](#13-risk-analysis)

---

## 1. Executive Summary

### What We're Building

Ze is a Go rewrite of ExaBGP with these goals:
- **100% API compatibility** with ExaBGP (JSON output, commands)
- **100% config compatibility** (same file format, env vars)
- **10x performance** improvement (parsing, memory)
- **Novel architecture** for better scalability

### Key Innovations

| Innovation | Benefit | Risk |
|------------|---------|------|
| AS-PATH as NLRI extension | Better attribute deduplication | Complexity |
| Per-attribute goroutine pools | Concurrent deduplication | Coordination |
| Zero-copy message passing | Memory efficiency | Lifetime tracking |
| Alternating buffer compaction | No stop-the-world pauses | Implementation complexity |

### Non-Goals

- FIB manipulation (BGP protocol only, like ExaBGP)
- API v4 compatibility (v6 only - simpler, cleaner JSON)

### Additional Features (Beyond ExaBGP)

- **VyOS-like configuration editor** - Interactive CLI for configuration management
  - Hierarchical navigation (`edit neighbor 192.168.1.1`)
  - Tab completion for all keywords
  - Commit/discard workflow
  - Show diff before commit
  - Configuration validation

---

## 2. System Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Ze Daemon                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │   Config    │    │   Reactor   │    │     API     │    │     CLI     │  │
│  │   Parser    │───▶│   (Core)    │◀───│   Server    │◀───│   Client    │  │
│  └─────────────┘    └──────┬──────┘    └─────────────┘    └─────────────┘  │
│                            │                                                │
│         ┌──────────────────┼──────────────────┐                            │
│         ▼                  ▼                  ▼                            │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                     │
│  │   Peer 1    │    │   Peer 2    │    │   Peer N    │   (goroutine/peer) │
│  │    FSM      │    │    FSM      │    │    FSM      │                     │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘                     │
│         │                  │                  │                            │
│         └──────────────────┼──────────────────┘                            │
│                            ▼                                                │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                              RIB                                     │   │
│  │   ┌───────────┐    ┌───────────┐    ┌───────────┐                   │   │
│  │   │ Adj-RIB-In│    │  Loc-RIB  │    │Adj-RIB-Out│                   │   │
│  │   └─────┬─────┘    └─────┬─────┘    └─────┬─────┘                   │   │
│  └─────────┼────────────────┼────────────────┼─────────────────────────┘   │
│            │                │                │                              │
│            └────────────────┼────────────────┘                              │
│                             ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                    Deduplication Layer                               │   │
│  │                                                                      │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │   │
│  │  │  NLRI Pools  │  │ Attr Pools   │  │  AS-PATH     │               │   │
│  │  │  (per-family)│  │ (per-type)   │  │  Pool        │               │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘               │   │
│  │                                                                      │   │
│  │  ┌──────────────────────────────────────────────────────────────┐   │   │
│  │  │              Global Compaction Scheduler                      │   │   │
│  │  └──────────────────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
ze/
├── cmd/
│   ├── ze/              # Main daemon
│   │   └── main.go
│   ├── ze-bgp-cli/          # Interactive CLI
│   │   └── main.go
│   └── ze-bgp-decode/       # Message decoder utility
│       └── main.go
│
├── internal/                    # Public API (importable)
│   ├── bgp/
│   │   ├── types.go        # AFI, SAFI, Family
│   │   ├── message/        # BGP messages
│   │   │   ├── message.go  # Message interface
│   │   │   ├── header.go   # Header parsing
│   │   │   ├── open.go     # OPEN message
│   │   │   ├── update.go   # UPDATE message
│   │   │   ├── notification.go
│   │   │   ├── keepalive.go
│   │   │   └── refresh.go
│   │   ├── attribute/      # Path attributes
│   │   │   ├── attribute.go # Attribute interface
│   │   │   ├── origin.go
│   │   │   ├── aspath.go
│   │   │   ├── nexthop.go
│   │   │   ├── med.go
│   │   │   ├── localpref.go
│   │   │   ├── community.go
│   │   │   ├── extcommunity.go
│   │   │   ├── largecommunity.go
│   │   │   └── ...         # 23+ attribute types
│   │   ├── nlri/           # NLRI types
│   │   │   ├── nlri.go     # NLRI interface
│   │   │   ├── inet.go     # IPv4/IPv6 unicast
│   │   │   ├── ipvpn.go    # VPNv4/VPNv6
│   │   │   ├── evpn.go     # EVPN (5 route types)
│   │   │   ├── flowspec.go # FlowSpec
│   │   │   ├── bgpls.go    # BGP-LS
│   │   │   ├── mup.go      # Mobile User Plane
│   │   │   └── ...
│   │   ├── capability/     # BGP capabilities
│   │   │   ├── capability.go
│   │   │   ├── multiprotocol.go
│   │   │   ├── asn4.go
│   │   │   ├── addpath.go
│   │   │   ├── graceful.go
│   │   │   └── ...         # 14+ capability types
│   │   └── fsm/            # Finite state machine
│   │       ├── fsm.go
│   │       ├── states.go
│   │       └── events.go
│   │
│   ├── reactor/            # Core event loop
│   │   ├── reactor.go      # Main reactor
│   │   ├── peer.go         # Peer management
│   │   ├── listener.go     # TCP listener
│   │   └── signals.go      # Signal handling
│   │
│   ├── rib/                # Routing Information Base
│   │   ├── rib.go          # Main RIB
│   │   ├── route.go        # Route structure
│   │   ├── incoming.go     # Adj-RIB-In
│   │   └── outgoing.go     # Adj-RIB-Out
│   │
│   ├── config/             # Configuration
│   │   ├── config.go       # Config structures
│   │   ├── parser.go       # ExaBGP format parser
│   │   ├── env.go          # Environment variables
│   │   └── validate.go     # Validation
│   │
│   ├── api/                # External API
│   │   ├── server.go       # Unix socket server
│   │   ├── client.go       # Client connections
│   │   ├── commands.go     # Command handlers
│   │   ├── json.go         # JSON encoding
│   │   └── process.go      # External process mgmt
│   │
│   └── wire/               # Wire format utilities
│       ├── buffer.go       # Zero-copy buffer
│       ├── reader.go       # Protocol reader
│       └── writer.go       # Protocol writer
│
├── internal/               # Private implementation
│   ├── pool/               # Buffer pools
│   │   ├── pool.go         # Core Pool type
│   │   ├── handle.go       # Handle type
│   │   ├── compaction.go   # Compaction logic
│   │   ├── scheduler.go    # Global scheduler
│   │   ├── metrics.go      # Pool metrics
│   │   └── debug.go        # Debug validation
│   │
│   └── store/              # Deduplication stores
│       ├── attribute.go    # Attribute store
│       └── nlri.go         # NLRI store
│
├── test/               # Test fixtures
│   ├── encoding/           # From ExaBGP qa/encoding
│   ├── decoding/           # From ExaBGP qa/decoding
│   └── messages/           # Raw BGP messages
│
└── docs/plan/                   # Implementation plans
    ├── README.md
    ├── ARCHITECTURE.md     # This file
    └── wip-pool-completion.md
```

---

## 3. Core Design Decisions

### 3.1 AS-PATH as NLRI Extension

**Traditional approach:**
```
Route = NLRI + Attributes (including AS-PATH)
```

**Our approach:**
```
Route = (NLRI + AS-PATH) + Attributes (excluding AS-PATH)
```

**Why this matters:**

In a route reflector scenario with 1000 peers:
- Traditional: Each peer's routes have unique AS-PATH → unique attribute sets
- Our approach: Same NLRI with different AS-PATHs share all other attributes

```
Traditional:
┌─────────────────────────────────────────┐
│ NLRI: 10.0.0.0/24                       │
│ Attrs: ORIGIN=IGP, AS-PATH=[65001],     │
│        NEXT-HOP=1.1.1.1, MED=100        │ ← Full copy per peer
└─────────────────────────────────────────┘

Our approach:
┌─────────────────────────────────────────┐
│ NLRI+AS-PATH: 10.0.0.0/24 + [65001]     │──┐
├─────────────────────────────────────────┤  │
│ NLRI+AS-PATH: 10.0.0.0/24 + [65002]     │──┼──▶ Shared: ORIGIN=IGP,
├─────────────────────────────────────────┤  │     NEXT-HOP=1.1.1.1,
│ NLRI+AS-PATH: 10.0.0.0/24 + [65003]     │──┘     MED=100
└─────────────────────────────────────────┘
```

**Memory savings:** Up to 80% for route reflector workloads.

---

### 3.2 Per-Attribute Type Pools

Each attribute type has its own deduplication pool:

```
┌──────────────────────────────────────────────────────────┐
│                    Attribute Pools                        │
├──────────────┬──────────────┬──────────────┬─────────────┤
│   ORIGIN     │   NEXT-HOP   │  COMMUNITY   │  EXT-COMM   │
│   Pool       │   Pool       │   Pool       │   Pool      │
│              │              │              │             │
│  3 values    │  ~1000       │  ~100        │  ~50        │
│  (IGP,EGP,?) │  unique      │  unique      │  unique     │
└──────────────┴──────────────┴──────────────┴─────────────┘
```

**Benefits:**
1. Type-specific optimization (ORIGIN has only 3 values)
2. Independent compaction (can compact COMMUNITY without touching ORIGIN)
3. Better cache locality

---

### 3.3 Zero-Copy Message Passing

When a message can be forwarded unchanged:

```
Peer A ──[UPDATE bytes]──▶ Ze ──[same bytes]──▶ Peer B
                              │
                              └─ No parsing, no repacking
```

**Conditions for zero-copy:**
- Same capabilities (ADD-PATH, ASN4, extended message)
- No policy modifications needed
- No AS-PATH manipulation

**Implementation:**
```go
type PassthroughMessage struct {
    rawData  []byte        // Original wire data
    families []Family      // Affected families (for filtering)
    parsed   atomic.Bool   // Lazy parsing flag
    update   *Update       // Parsed only if needed
}
```

---

### 3.4 Alternating Buffer Compaction

See `POOL_ARCHITECTURE.md` for full details. Key points:

```
Normal:     [Buffer 0: ████████████]  [Buffer 1: nil]

Compacting: [Buffer 0: ████████████]  [Buffer 1: ████░░░░]
            (old, draining refs)       (new, receiving data)

Complete:   [Buffer 0: nil]           [Buffer 1: ████████]
```

**Benefits:**
- No stop-the-world pauses
- Old handles remain valid during migration
- Incremental progress (configurable batch size)

---

### 3.5 Handle Design (Hybrid Layout)

> **Full details:** See `docs/architecture/pool-architecture.md`

```
Handle (uint32):
┌─────────┬─────────┬───────┬────────────────────────┐
│BufferBit│ PoolIdx │ Flags │        Slot            │
│ (1 bit) │ (5 bits)│(2 bit)│      (24 bits)         │
└─────────┴─────────┴───────┴────────────────────────┘
 31        30    26  25   24  23                    0
```

**Benefits:**
- Buffer bit distinguishes buffers during compaction
- Pool index validates handle belongs to correct pool
- Flags support ADD-PATH (bit 0 = hasPathID)
- 24-bit slot = 16.7M entries per pool

---

## 4. Component Architecture

### 4.1 Reactor (internal/reactor/)

**Responsibility:** Core event loop, peer lifecycle management

```go
type Reactor struct {
    config   *config.Config
    peers    map[string]*Peer    // address → peer
    listener *Listener           // TCP :179
    api      *api.Server         // Unix socket

    // Lifecycle
    ctx      context.Context
    cancel   context.CancelFunc
    wg       sync.WaitGroup

    // Stores (shared across peers)
    attrStore *store.AttributeStore
    nlriStore *store.NLRIStore
}

func (r *Reactor) Run() error {
    // 1. Start listener
    // 2. Start API server
    // 3. Start peers (goroutine each)
    // 4. Handle signals (SIGHUP, SIGUSR1, SIGTERM)
    // 5. Wait for shutdown
}
```

### 4.2 Peer (internal/reactor/peer.go)

**Responsibility:** Single BGP session management

```go
type Peer struct {
    neighbor   *config.Neighbor
    fsm        *fsm.FSM
    rib        *rib.PeerRIB      // This peer's view

    conn       net.Conn
    reader     *wire.Reader
    writer     *wire.Writer

    incoming   chan message.Message
    outgoing   chan message.Message

    negotiated *capability.Negotiated

    ctx        context.Context
    cancel     context.CancelFunc
}

func (p *Peer) Run() {
    for {
        if err := p.session(); err != nil {
            p.handleError(err)
            p.reconnect()
        }
    }
}
```

### 4.3 FSM (internal/bgp/fsm/)

**Responsibility:** RFC 4271 state machine

```
┌─────────┐  ManualStart   ┌─────────┐
│  IDLE   │───────────────▶│ CONNECT │
└────┬────┘                └────┬────┘
     │                          │ TCP Connected
     │ ManualStart (passive)    ▼
     │                    ┌─────────┐
     └───────────────────▶│ ACTIVE  │
                          └────┬────┘
                               │ TCP Connected
                               ▼
                         ┌──────────┐
                         │ OPENSENT │
                         └────┬─────┘
                              │ OPEN Received
                              ▼
                        ┌────────────┐
                        │OPENCONFIRM │
                        └─────┬──────┘
                              │ KEEPALIVE Received
                              ▼
                        ┌─────────────┐
                        │ ESTABLISHED │
                        └─────────────┘
```

### 4.4 RIB (internal/rib/)

**Responsibility:** Route storage and lookup

```go
type RIB struct {
    // Per-peer incoming routes
    incoming map[string]*PeerRIB  // peer-addr → rib

    // Local RIB (best routes)
    local    map[Family]*FamilyRIB

    // Per-peer outgoing routes
    outgoing map[string]*PeerRIB

    // Shared storage
    store    *RouteStore
}

type Route struct {
    nlriHandle   pool.Handle    // Points to NLRI+AS-PATH
    attrHandles  []pool.Handle  // Points to each attribute
    nextHop      netip.Addr     // Stored separately (frequently accessed)
    receivedAt   time.Time
    peerAddr     string
}
```

### 4.5 Pool (internal/pool/)

**Responsibility:** Memory-efficient deduplication with non-blocking compaction

See `POOL_ARCHITECTURE.md` for full design.

```go
type Pool struct {
    mu sync.RWMutex

    buffers    [2]buffer      // Alternating buffers
    currentBit uint32         // 0 or 1

    slots      []Slot         // Slot table
    index      map[string]Handle  // Dedup index (unsafe.String keys)

    state      PoolState      // Normal or Compacting

    metrics    PoolMetrics
    config     PoolConfig
}

// Core operations
func (p *Pool) Intern(data []byte) Handle
func (p *Pool) Get(h Handle) []byte
func (p *Pool) Release(h Handle)
func (p *Pool) AddRef(h Handle)
```

### 4.6 API (internal/plugin/)

**Responsibility:** External process communication, CLI

```go
type Server struct {
    reactor    *Reactor
    listener   net.Listener      // Unix socket
    clients    map[string]*Client
    processes  map[string]*Process  // External processes
}

// Command examples
// "show neighbor summary"
// "update text nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24"
// "update text nlri ipv4/unicast del 10.0.0.0/24"
```

### 4.7 Config Editor (internal/config/editor/)

**Responsibility:** VyOS-like interactive configuration management

```go
type Editor struct {
    root     *ConfigNode       // Configuration tree
    current  *ConfigNode       // Current edit position
    candidate *ConfigNode      // Uncommitted changes
    running  *ConfigNode       // Active configuration

    completer *Completer       // Tab completion
    validator *Validator       // Configuration validation
}

// Navigation
func (e *Editor) Edit(path string) error      // edit neighbor 192.168.1.1
func (e *Editor) Up() error                   // go up one level
func (e *Editor) Top() error                  // go to root

// Configuration
func (e *Editor) Set(path, value string) error   // set local-as 65001
func (e *Editor) Delete(path string) error       // delete family ipv6
func (e *Editor) Show() string                   // show current section

// Workflow
func (e *Editor) Compare() string             // show diff candidate vs running
func (e *Editor) Commit() error               // apply candidate to running
func (e *Editor) Discard() error              // revert candidate to running
func (e *Editor) Save(path string) error      // save to file
func (e *Editor) Load(path string) error      // load from file
```

**CLI Session Example:**
```
ze# configure
ze(config)# edit neighbor 192.168.1.2
ze(config-neighbor)# set description "Transit Provider"
ze(config-neighbor)# set peer-as 65002
ze(config-neighbor)# set local-as 65001
ze(config-neighbor)# edit family
ze(config-neighbor-family)# set ipv4/unicast
ze(config-neighbor-family)# set ipv6/unicast
ze(config-neighbor-family)# top
ze(config)# compare
+ neighbor 192.168.1.2 {
+     description "Transit Provider"
+     peer-as 65002
+     local-as 65001
+     family {
+         ipv4/unicast
+         ipv6/unicast
+     }
+ }
ze(config)# commit
Configuration committed.
ze(config)# exit
ze#
```

**Features:**
- Hierarchical navigation with `edit`, `up`, `top`
- Tab completion for keywords, neighbors, families
- `compare` shows diff before commit
- `commit` validates and applies atomically
- `discard` reverts uncommitted changes
- `save`/`load` for file persistence
- Import ExaBGP config files for migration

### 4.8 Configuration Format

**Design Decision:** Native format uses set commands matching the API.

**Three formats:**

| Format | Purpose | Example |
|--------|---------|---------|
| ExaBGP | Import only (migration) | `neighbor 1.2.3.4 { local-as 65001; }` |
| Set commands | Native storage & API | `set neighbor 1.2.3.4 local-as 65001` |
| Display | Human-readable output | Same as set commands |

**Native format example (set commands):**
```
set neighbor 192.168.1.2 description "Transit Provider"
set neighbor 192.168.1.2 peer-as 65002
set neighbor 192.168.1.2 local-as 65001
set neighbor 192.168.1.2 local-address 192.168.1.1
set neighbor 192.168.1.2 family ipv4/unicast
set neighbor 192.168.1.2 family ipv6/unicast

set process announce-routes run "/usr/bin/python3 /path/to/script.py"
set process announce-routes encoder json

set neighbor 192.168.1.2 api processes announce-routes
```

**Import from ExaBGP:**
```
ze# load exabgp /path/to/exabgp.conf
Imported 3 neighbors, 2 processes.
ze# show configuration
set neighbor 192.168.1.2 description "Transit Provider"
set neighbor 192.168.1.2 peer-as 65002
...
```

**Benefits:**
1. **API consistency** - Same syntax for config and runtime commands
2. **Easier parsing** - One command per line, no nested braces
3. **Diff-friendly** - Easy to see changes in version control
4. **Script-friendly** - Can generate config with simple echo/printf
5. **Tab completion** - Same completers for config and API

---

## 5. Data Flow

### 5.1 Incoming UPDATE Processing

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Incoming UPDATE Flow                              │
└─────────────────────────────────────────────────────────────────────────┘

   TCP Socket
       │
       ▼
┌─────────────┐
│ wire.Reader │  Read message bytes
└──────┬──────┘
       │
       ▼
┌─────────────┐
│Parse Header │  Validate marker, get length/type
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Lazy UPDATE │  Store raw bytes, don't parse yet
└──────┬──────┘
       │
       ├─────────────────────────────────────┐
       │ Can forward unchanged?              │
       │ (same caps, no policy)              │
       ▼                                     ▼
┌─────────────┐                       ┌─────────────┐
│Pass-through │                       │ Full Parse  │
│ to peers    │                       │             │
└─────────────┘                       └──────┬──────┘
                                             │
                      ┌──────────────────────┼──────────────────────┐
                      ▼                      ▼                      ▼
               ┌─────────────┐        ┌─────────────┐        ┌─────────────┐
               │ Parse NLRI  │        │ Parse Attrs │        │ Parse AS-PATH│
               │             │        │             │        │             │
               └──────┬──────┘        └──────┬──────┘        └──────┬──────┘
                      │                      │                      │
                      ▼                      ▼                      ▼
               ┌─────────────┐        ┌─────────────┐        ┌─────────────┐
               │ NLRI Pool   │        │ Attr Pools  │        │AS-PATH Pool │
               │ Intern()    │        │ Intern()    │        │ Intern()    │
               └──────┬──────┘        └──────┬──────┘        └──────┬──────┘
                      │                      │                      │
                      └──────────────────────┼──────────────────────┘
                                             │
                                             ▼
                                      ┌─────────────┐
                                      │  Create     │
                                      │  Route      │
                                      │  (handles)  │
                                      └──────┬──────┘
                                             │
                                             ▼
                                      ┌─────────────┐
                                      │ Adj-RIB-In  │
                                      │  Store      │
                                      └──────┬──────┘
                                             │
                                             ▼
                                      ┌─────────────┐
                                      │ API Output  │
                                      │ (JSON)      │
                                      └─────────────┘
```

### 5.2 Outgoing UPDATE Generation

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Outgoing UPDATE Flow                              │
└─────────────────────────────────────────────────────────────────────────┘

┌─────────────┐
│  API Input  │  "update text nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24"
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Parse Cmd   │  Command → Route specification
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Build Route │  Create NLRI, attributes
└──────┬──────┘
       │
       ▼
┌─────────────┐
│Adj-RIB-Out  │  Add to outgoing queue per peer
└──────┬──────┘
       │
       ├────────────────────────────────────────┐
       ▼                                        ▼
┌─────────────────┐                     ┌─────────────────┐
│ Peer 1          │                     │ Peer 2          │
│ Pack for caps   │                     │ Pack for caps   │
│ (ADD-PATH, etc) │                     │ (different)     │
└────────┬────────┘                     └────────┬────────┘
         │                                       │
         ▼                                       ▼
┌─────────────────┐                     ┌─────────────────┐
│ wire.Writer     │                     │ wire.Writer     │
│ TCP Send        │                     │ TCP Send        │
└─────────────────┘                     └─────────────────┘
```

---

## 6. Memory Management

### 6.1 Ownership Model

```
┌─────────────────────────────────────────────────────────────────┐
│                      Memory Ownership                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  RIB Entry                                                      │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  nlriHandle ────────▶ NLRI Pool (refCount++)            │   │
│  │  attrHandles[] ────▶ Attr Pools (refCount++ each)       │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  When RIB entry deleted:                                        │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │  NLRI Pool.Release(nlriHandle)   → refCount--           │   │
│  │  Attr Pool.Release(attrHandle)   → refCount-- (each)    │   │
│  │                                                          │   │
│  │  If refCount == 0: slot marked dead, reclaimable        │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 6.2 Lifetime Guarantees

| Component | Lifetime | Managed By |
|-----------|----------|------------|
| Wire message bytes | Until parsed or forwarded | Reader buffer pool |
| Pool slot data | While refCount > 0 | Pool |
| Route in RIB | While in RIB | RIB |
| Passthrough message | While pending sends > 0 | Passthrough refCount |

### 6.3 Estimated Memory Usage

For 1 million routes (full table scenario):

| Component | Per-Route | Total (1M routes) |
|-----------|-----------|-------------------|
| RIB entry | ~64 bytes | 64 MB |
| NLRI (deduplicated) | ~500K unique × 32 bytes | 16 MB |
| Attributes (deduplicated) | ~100K unique × 64 bytes | 6.4 MB |
| Pool overhead | ~20% | 17 MB |
| **Total** | | **~103 MB** |

Compare to naive approach: 1M × 256 bytes = **256 MB**

---

## 7. Concurrency Model

### 7.1 Goroutine Structure

```
main goroutine
    │
    ├── Reactor goroutine (coordinator)
    │       │
    │       ├── Listener goroutine (accept loop)
    │       │
    │       ├── Signal handler goroutine
    │       │
    │       └── Peer goroutines (one per peer)
    │               │
    │               ├── Reader goroutine (TCP → incoming chan)
    │               │
    │               └── Writer goroutine (outgoing chan → TCP)
    │
    ├── API Server goroutine
    │       │
    │       └── Client goroutines (one per connection)
    │
    └── Compaction Scheduler goroutine
            │
            └── (uses Reactor idle time, no dedicated goroutines)
```

### 7.2 Synchronization Strategy

| Resource | Protection | Contention |
|----------|------------|------------|
| Peer map | sync.RWMutex | Low (config changes only) |
| Per-peer RIB | Channel-based | None (single owner) |
| Global RIB | sync.RWMutex | Medium (route updates) |
| Pool | sync.RWMutex | Medium (see below) |
| Pool index | Within pool lock | Included above |

### 7.3 Pool Lock Contention Mitigation

If pool lock becomes bottleneck (monitor with metrics):

1. **Sharding:** 16 sub-pools by hash prefix
2. **Batch operations:** Intern multiple items per lock
3. **Lock-free refCount:** atomic.Int32 for hot path

---

## 8. Interface Contracts

### 8.1 Message Interface

```go
// internal/bgp/message/message.go

type Message interface {
    Type() MessageType
    Pack(negotiated *Negotiated) ([]byte, error)
}

type Unpacker interface {
    Unpack(data []byte, negotiated *Negotiated) (Message, error)
}
```

### 8.2 Attribute Interface

```go
// internal/bgp/attribute/attribute.go

type Attribute interface {
    Code() AttributeCode
    Flags() AttributeFlags
    Pack(negotiated *Negotiated) []byte

    // For deduplication
    Hash() uint64
    Equal(other Attribute) bool
}
```

### 8.3 NLRI Interface

```go
// internal/bgp/nlri/nlri.go

type NLRI interface {
    Family() Family
    Pack(negotiated *Negotiated) []byte

    // For indexing (includes AS-PATH hash in our design)
    Index() []byte

    // For API output
    JSON() map[string]any
}
```

### 8.4 Pool Interface

```go
// internal/pool/pool.go

type Pool interface {
    Intern(data []byte) Handle
    Get(h Handle) []byte
    Release(h Handle)
    AddRef(h Handle)
    Length(h Handle) int
    WriteTo(h Handle, w io.Writer) (int64, error)
    Metrics() PoolMetrics
    Shutdown()
}
```

---

## 9. Dependency Graph

### 9.1 Package Dependencies

```
                              ┌─────────────┐
                              │   cmd/*     │
                              └──────┬──────┘
                                     │
                    ┌────────────────┼────────────────┐
                    ▼                ▼                ▼
             ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
             │   reactor   │  │    api      │  │   config    │
             └──────┬──────┘  └──────┬──────┘  └─────────────┘
                    │                │
         ┌──────────┴──────────┬─────┘
         ▼                     ▼
  ┌─────────────┐       ┌─────────────┐
  │    rib      │       │  bgp/fsm    │
  └──────┬──────┘       └──────┬──────┘
         │                     │
         └──────────┬──────────┘
                    ▼
             ┌─────────────┐
             │ bgp/message │
             └──────┬──────┘
                    │
      ┌─────────────┼─────────────┐
      ▼             ▼             ▼
┌───────────┐ ┌───────────┐ ┌───────────┐
│ attribute │ │   nlri    │ │capability │
└─────┬─────┘ └─────┬─────┘ └───────────┘
      │             │
      └──────┬──────┘
             ▼
      ┌─────────────┐
      │    wire     │
      └──────┬──────┘
             │
             ▼
      ┌─────────────┐        ┌─────────────┐
      │    pool     │◀───────│   store     │
      └─────────────┘        └─────────────┘
           (internal)             (internal)
```

### 9.2 Build Order

1. **Layer 0 (no deps):** `internal/pool`, `internal/wire`
2. **Layer 1:** `internal/bgp/capability`, `internal/bgp/attribute`, `internal/bgp/nlri`
3. **Layer 2:** `internal/bgp/message`, `internal/store`
4. **Layer 3:** `internal/bgp/fsm`, `internal/rib`
5. **Layer 4:** `internal/config`, `internal/plugin`
6. **Layer 5:** `internal/reactor`
7. **Layer 6:** `cmd/*`

---

## 10. ExaBGP Compatibility

### 10.1 JSON API Output

Must produce identical JSON for same BGP events:

```json
{
  "exabgp": "6.0.0",
  "time": 1234567890.123,
  "type": "update",
  "neighbor": {
    "address": {
      "local": "192.168.1.1",
      "peer": "192.168.1.2"
    },
    "asn": {
      "local": 65001,
      "peer": 65002
    }
  },
  "message": {
    "type": "update",
    "direction": "received"
  },
  "ipv4/unicast": [
    {
      "action": "add",
      "next-hop": "192.168.1.2",
      "nlri": ["10.0.0.0/24"]
    }
  ]
}
```

### 10.2 API Commands

Must support all ExaBGP commands:

| Command | Priority |
|---------|----------|
| show neighbor | P0 |
| show neighbor summary | P0 |
| show adj-rib in | P0 |
| show adj-rib out | P0 |
| update text | P0 |
| update text nlri ipv4/unicast del | P0 |
| announce eor | P0 |
| teardown | P0 |
| shutdown | P0 |
| announce flow | P1 |
| reload | P1 |

### 10.3 Configuration

Must parse ExaBGP config files:

```
process announce-routes {
    run /usr/bin/python3 /path/to/script.py;
    encoder json;
}

peer 192.168.1.2 {
    router-id 192.168.1.1;
    local-address 192.168.1.1;
    local-as 65001;
    peer-as 65002;

    family {
        ipv4/unicast;
        ipv6/unicast;
    }

    api {
        processes [ announce-routes ];
    }
}
```

### 10.4 Environment Variables

Must honor all `exabgp_*` environment variables:

| Variable | Default | Purpose |
|----------|---------|---------|
| exabgp_tcp_bind | 0.0.0.0 | Listen address |
| exabgp_tcp_port | 179 | Listen port |
| exabgp_bgp_passive | false | Passive mode |
| exabgp_log_level | INFO | Log verbosity |
| exabgp_api_encoder | json | API format |
| ... | | |

---

## 11. Testing Strategy

### 11.1 Unit Tests

Each package has `*_test.go` files:

```bash
go test -race ./internal/bgp/message/...
go test -race ./internal/bgp/attribute/...
go test -race ./internal/pool/...
```

### 11.2 Encoding/Decoding Tests

Convert ExaBGP's test suite:

```bash
# ExaBGP has 72 encoding tests, 18 decoding tests
./scripts/convert-exabgp-tests.sh

# Run converted tests
go test -race ./test/encode/...
go test -race ./test/decode/...
```

### 11.3 Round-Trip Tests

```go
func TestRoundTrip(t *testing.T) {
    // For each test message:
    // 1. Unpack(bytes) → Message
    // 2. Pack(Message) → bytes2
    // 3. Assert bytes == bytes2
}
```

### 11.4 Integration Tests

```go
func TestPeerSession(t *testing.T) {
    // Start two Ze instances
    // Establish session
    // Exchange routes
    // Verify convergence
}
```

### 11.5 ExaBGP Interop Tests

```bash
# Run Ze as neighbor to ExaBGP
./scripts/test-exabgp-interop.sh
```

---

## 12. Implementation Phases

### Phase 0: Foundation ✅
- [x] Directory structure
- [x] .claude/ setup
- [x] Pool architecture design
- [x] Pool implementation

### Phase 1: Wire Format ✅
- [x] internal/wire/buffer.go
- [x] internal/bgp/message/header.go
- [x] Basic message parsing

### Phase 2: Messages ✅
- [x] OPEN, UPDATE, NOTIFICATION, KEEPALIVE, REFRESH
- [x] Message registry

### Phase 3: Capabilities ✅
- [x] All 14+ capability types
- [x] Negotiation logic

### Phase 4: Attributes ✅
- [x] All 23+ attribute types
- [x] Attribute deduplication store

### Phase 5: NLRI ✅
- [x] INET, IPVPN, EVPN, FlowSpec
- [x] NLRI deduplication store

### Phase 6: RIB ✅
- [x] Route structure
- [x] Adj-RIB-In/Out
- [x] Deduplication integration

### Phase 7: FSM ✅
- [x] State machine
- [x] Timer management
- [x] Event handling

### Phase 8: Reactor ✅
- [x] Peer management
- [x] Listener
- [x] Signal handling

### Phase 9: Config ✅
- [x] Schema-driven parser
- [x] Set-style syntax
- [x] Serializer
- [x] Validation

### Phase 10: CLI ✅
- [x] ze bgp validate command
- [x] ze bgp run command
- [ ] ze-bgp-cli (interactive)
- [ ] ze-bgp-decode (utility)

### Phase 11: API ✅
- [x] Reactor-config wiring
- [x] Process management structure
- [ ] Unix socket server (runtime)
- [ ] JSON encoder (runtime)

### Phase 12: Testing 🔄
- [ ] Convert ExaBGP tests
- [x] Unit tests (all packages)
- [ ] Integration tests
- [ ] Interop tests

### Phase 13: Polish 🔄
- [ ] Performance benchmarks
- [ ] Documentation
- [ ] Release artifacts

---

## 13. Risk Analysis

### Technical Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| AS-PATH-as-NLRI complexity | Medium | High | Fallback to traditional if needed |
| Pool compaction bugs | Medium | High | Extensive testing, debug mode |
| ExaBGP config parser complexity | High | Medium | Start with subset, expand |
| Performance not meeting goals | Low | Medium | Profile early, optimize hot paths |

### Schedule Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| NLRI type complexity (FlowSpec, BGP-LS) | High | Medium | Prioritize simple types first |
| Test conversion effort | Medium | Medium | Automate conversion |
| Scope creep | Medium | High | Strict phase boundaries |

### Mitigation Strategies

1. **AS-PATH-as-NLRI:** If too complex, can fall back to traditional model with some memory cost
2. **Pool bugs:** Debug mode with validation, extensive unit tests
3. **Config parser:** Port ExaBGP's tokenizer logic directly
4. **Performance:** Benchmark after each phase, not just at end

---

## Appendix A: Reference Documents

- `docs/architecture/pool-architecture.md` - Pool design
- `docs/architecture/pool-architecture-review.md` - Pool issues
- `docs/architecture/hub-architecture.md` - Future Hub-based architecture with Config Reader and YANG validation
- `docs/architecture/config/yang-config-design.md` - YANG schema design (VyOS-inspired)
- `../main/` - ExaBGP Python implementation (reference)

---

**Created:** 2025-12-19
**Last Updated:** 2025-12-20
