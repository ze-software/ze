# ExaBGP to ZeBGP Code Map

## TL;DR (Read This First)

| ExaBGP | ZeBGP | Notes |
|--------|-------|-------|
| `bgp/message/` | `pkg/bgp/message/` | Message types |
| `bgp/message/open/capability/` | `pkg/bgp/capability/` | Capabilities |
| `bgp/message/update/attribute/` | `pkg/bgp/attribute/` | Path attributes |
| `bgp/message/update/nlri/` | `pkg/bgp/nlri/` | NLRI types |
| `reactor/peer/` | `pkg/reactor/peer.go` | Peer management |
| `rib/` | `pkg/rib/` | Route storage |

**When to read full doc:** ExaBGP compatibility, finding reference implementation.

---

**Purpose:** Map ExaBGP source structure to ZeBGP equivalents
**Source:** `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/` (372 Python files)
**Target:** `pkg/`, `internal/`, `cmd/`

---

## Overview

| ExaBGP Module | Files | ZeBGP Equivalent | Notes |
|---------------|-------|------------------|-------|
| `bgp/` | 120+ | `pkg/bgp/` | Core protocol |
| `reactor/` | 30+ | `pkg/reactor/` | Event loop, peers |
| `configuration/` | 50+ | `pkg/config/` | Config parsing |
| `rib/` | 10+ | `pkg/rib/` | Route storage |
| `application/` | 20+ | `cmd/` | Entry points |
| `protocol/` | 15+ | `pkg/wire/` | Wire utilities |
| `environment/` | 5+ | `pkg/config/` | Env vars |
| `cli/` | 10+ | `pkg/api/` | CLI interface |
| `logger/` | 8 | `slog` (stdlib) | Logging |
| `util/` | 12 | `internal/` | Utilities |
| `data/` | 2 | N/A | Data files |
| `debug/` | 4 | N/A | Debug utilities |
| `vendoring/` | 4 | N/A | Vendored deps |

---

## 1. BGP Protocol (`bgp/`)

### 1.1 Messages (`bgp/message/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `message.py` | Base message class | `pkg/bgp/message/message.go` |
| `message_type.py` | Message type enum | `pkg/bgp/message/types.go` |
| `keepalive.py` | KEEPALIVE message | `pkg/bgp/message/keepalive.go` |
| `notification.py` | NOTIFICATION message | `pkg/bgp/message/notification.go` |
| `refresh.py` | ROUTE-REFRESH message | `pkg/bgp/message/refresh.go` |
| `operational.py` | Operational messages | `pkg/bgp/message/operational.go` |
| `unknown.py` | Unknown message handling | `pkg/bgp/message/unknown.go` |
| `direction.py` | Message direction enum | `pkg/bgp/message/direction.go` |
| `action.py` | Message action enum | `pkg/bgp/message/action.go` |
| `source.py` | Message source enum | `pkg/bgp/message/source.go` |
| `scheduling.py` | Message scheduling | `pkg/reactor/scheduler.go` |

### 1.2 OPEN Message (`bgp/message/open/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `open/capability/capability.py` | Capability base | `pkg/bgp/capability/capability.go` |
| `open/capability/capabilities.py` | Capability collection | `pkg/bgp/capability/capabilities.go` |
| `open/capability/negotiated.py` | Negotiated state | `pkg/bgp/capability/negotiated.go` |
| `open/capability/mp.py` | Multiprotocol | `pkg/bgp/capability/multiprotocol.go` |
| `open/capability/asn4.py` | 4-byte AS | `pkg/bgp/capability/asn4.go` |
| `open/capability/addpath.py` | ADD-PATH | `pkg/bgp/capability/addpath.go` |
| `open/capability/graceful.py` | Graceful Restart | `pkg/bgp/capability/graceful.go` |
| `open/capability/extended.py` | Extended Message | `pkg/bgp/capability/extended.go` |
| `open/capability/refresh.py` | Route Refresh | `pkg/bgp/capability/refresh.go` |
| `open/capability/hostname.py` | FQDN capability | `pkg/bgp/capability/hostname.go` |
| `open/capability/software.py` | Software Version | `pkg/bgp/capability/software.go` |
| `open/capability/nexthop.py` | Extended Next Hop | `pkg/bgp/capability/nexthop.go` |
| `open/capability/ms.py` | Multiple Paths | `pkg/bgp/capability/multipaths.go` |
| `open/capability/operational.py` | Operational | `pkg/bgp/capability/operational.go` |
| `open/capability/unknown.py` | Unknown capability | `pkg/bgp/capability/unknown.go` |

### 1.3 UPDATE Message (`bgp/message/update/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `update/__init__.py` | UPDATE message | `pkg/bgp/message/update.go` |
| `update/collection.py` | Route collection | `pkg/bgp/message/update_collection.go` |
| `update/eor.py` | End of RIB | `pkg/bgp/message/eor.go` |

### 1.4 Path Attributes (`bgp/message/update/attribute/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `attribute/attribute.py` | Attribute base | `pkg/bgp/attribute/attribute.go` |
| `attribute/collection.py` | Attribute collection | `pkg/bgp/attribute/collection.go` |
| `attribute/origin.py` | ORIGIN | `pkg/bgp/attribute/origin.go` |
| `attribute/aspath.py` | AS_PATH, AS4_PATH | `pkg/bgp/attribute/aspath.go` |
| `attribute/nexthop.py` | NEXT_HOP | `pkg/bgp/attribute/nexthop.go` |
| `attribute/med.py` | MULTI_EXIT_DISC | `pkg/bgp/attribute/med.go` |
| `attribute/localpref.py` | LOCAL_PREF | `pkg/bgp/attribute/localpref.go` |
| `attribute/atomicaggregate.py` | ATOMIC_AGGREGATE | `pkg/bgp/attribute/atomicaggregate.go` |
| `attribute/aggregator.py` | AGGREGATOR, AS4_AGGREGATOR | `pkg/bgp/attribute/aggregator.go` |
| `attribute/originatorid.py` | ORIGINATOR_ID | `pkg/bgp/attribute/originatorid.go` |
| `attribute/clusterlist.py` | CLUSTER_LIST | `pkg/bgp/attribute/clusterlist.go` |
| `attribute/mprnlri.py` | MP_REACH_NLRI | `pkg/bgp/attribute/mpreach.go` |
| `attribute/mpurnlri.py` | MP_UNREACH_NLRI | `pkg/bgp/attribute/mpunreach.go` |
| `attribute/aigp.py` | AIGP | `pkg/bgp/attribute/aigp.go` |
| `attribute/pmsi.py` | PMSI Tunnel | `pkg/bgp/attribute/pmsi.go` |
| `attribute/generic.py` | Generic/unknown | `pkg/bgp/attribute/generic.go` |
| `attribute/watchdog.py` | Watchdog (internal) | N/A (internal only) |

### 1.5 Communities (`bgp/message/update/attribute/community/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `community/initial/community.py` | Standard community | `pkg/bgp/attribute/community.go` |
| `community/extended/*.py` | Extended communities | `pkg/bgp/attribute/extcommunity.go` |
| `community/large/*.py` | Large communities | `pkg/bgp/attribute/largecommunity.go` |

### 1.6 BGP-LS Attributes (`bgp/message/update/attribute/bgpls/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `bgpls/node/*.py` | Node attributes | `pkg/bgp/attribute/bgpls/node.go` |
| `bgpls/link/*.py` | Link attributes | `pkg/bgp/attribute/bgpls/link.go` |
| `bgpls/prefix/*.py` | Prefix attributes | `pkg/bgp/attribute/bgpls/prefix.go` |

### 1.7 SR/SRv6 (`bgp/message/update/attribute/sr/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `sr/*.py` | Segment Routing | `pkg/bgp/attribute/sr/sr.go` |
| `sr/srv6/*.py` | SRv6 | `pkg/bgp/attribute/sr/srv6.go` |

### 1.8 NLRI Types (`bgp/message/update/nlri/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `nlri/nlri.py` | NLRI base class | `pkg/bgp/nlri/nlri.go` |
| `nlri/collection.py` | NLRI collection | `pkg/bgp/nlri/collection.go` |
| `nlri/cidr.py` | CIDR utilities | `pkg/bgp/nlri/cidr.go` |
| `nlri/inet.py` | IPv4/IPv6 unicast | `pkg/bgp/nlri/inet.go` |
| `nlri/label.py` | MPLS labels | `pkg/bgp/nlri/label.go` |
| `nlri/ipvpn.py` | VPNv4/VPNv6 | `pkg/bgp/nlri/ipvpn.go` |
| `nlri/flow.py` | FlowSpec | `pkg/bgp/nlri/flowspec.go` |
| `nlri/vpls.py` | VPLS | `pkg/bgp/nlri/vpls.go` |
| `nlri/rtc.py` | Route Target Constraint | `pkg/bgp/nlri/rtc.go` |
| `nlri/settings.py` | NLRI settings | `pkg/bgp/nlri/settings.go` |
| `nlri/empty.py` | Empty NLRI | `pkg/bgp/nlri/empty.go` |

### 1.9 EVPN (`bgp/message/update/nlri/evpn/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `evpn/nlri.py` | EVPN base | `pkg/bgp/nlri/evpn/evpn.go` |
| `evpn/ethernetad.py` | Type 1: Ethernet AD | `pkg/bgp/nlri/evpn/type1.go` |
| `evpn/mac.py` | Type 2: MAC/IP | `pkg/bgp/nlri/evpn/type2.go` |
| `evpn/multicast.py` | Type 3: Inclusive Multicast | `pkg/bgp/nlri/evpn/type3.go` |
| `evpn/segment.py` | Type 4: Ethernet Segment | `pkg/bgp/nlri/evpn/type4.go` |
| `evpn/prefix.py` | Type 5: IP Prefix | `pkg/bgp/nlri/evpn/type5.go` |

### 1.10 BGP-LS NLRI (`bgp/message/update/nlri/bgpls/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `bgpls/nlri.py` | BGP-LS base | `pkg/bgp/nlri/bgpls/bgpls.go` |
| `bgpls/node.py` | Node NLRI | `pkg/bgp/nlri/bgpls/node.go` |
| `bgpls/link.py` | Link NLRI | `pkg/bgp/nlri/bgpls/link.go` |
| `bgpls/prefixv4.py` | IPv4 Prefix NLRI | `pkg/bgp/nlri/bgpls/prefix.go` |
| `bgpls/prefixv6.py` | IPv6 Prefix NLRI | `pkg/bgp/nlri/bgpls/prefix.go` |
| `bgpls/tlvs/*.py` | TLV types | `pkg/bgp/nlri/bgpls/tlv.go` |

### 1.11 MUP (`bgp/message/update/nlri/mup/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `mup/nlri.py` | MUP base | `pkg/bgp/nlri/mup/mup.go` |
| `mup/isd.py` | Interwork SD | `pkg/bgp/nlri/mup/isd.go` |
| `mup/dsd.py` | Direct SD | `pkg/bgp/nlri/mup/dsd.go` |
| `mup/t1st.py` | Type 1 ST | `pkg/bgp/nlri/mup/t1st.go` |
| `mup/t2st.py` | Type 2 ST | `pkg/bgp/nlri/mup/t2st.go` |

### 1.12 MVPN (`bgp/message/update/nlri/mvpn/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `mvpn/nlri.py` | MVPN base | `pkg/bgp/nlri/mvpn/mvpn.go` |
| `mvpn/*.py` | MVPN route types | `pkg/bgp/nlri/mvpn/*.go` |

### 1.13 Qualifiers (`bgp/message/update/nlri/qualifier/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `qualifier/rd.py` | Route Distinguisher | `pkg/bgp/nlri/rd.go` |
| `qualifier/esi.py` | Ethernet Segment ID | `pkg/bgp/nlri/esi.go` |
| `qualifier/labels.py` | MPLS Label Stack | `pkg/bgp/nlri/labels.go` |
| `qualifier/mac.py` | MAC Address | `pkg/bgp/nlri/mac.go` |
| `qualifier/etag.py` | Ethernet Tag | `pkg/bgp/nlri/etag.go` |
| `qualifier/path_info.py` | ADD-PATH Path ID | `pkg/bgp/nlri/pathid.go` |

### 1.14 Neighbor (`bgp/neighbor/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `neighbor/neighbor.py` | Neighbor config | `pkg/config/neighbor.go` |
| `neighbor/*.py` | Neighbor helpers | `pkg/config/neighbor_*.go` |

### 1.15 FSM (`bgp/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `fsm.py` | State machine | `pkg/bgp/fsm/fsm.go` |
| `timer.py` | BGP timers | `pkg/bgp/fsm/timer.go` |

---

## 2. Reactor (`reactor/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `loop.py` | Main event loop | `pkg/reactor/reactor.go` |
| `asynchronous.py` | Async handling | `pkg/reactor/reactor.go` (goroutines) |
| `daemon.py` | Daemon management | `cmd/zebgp/daemon.go` |
| `protocol.py` | Protocol handler | `pkg/reactor/protocol.go` |
| `listener.py` | TCP listener | `pkg/reactor/listener.go` |
| `timing.py` | Timing utilities | `pkg/reactor/timing.go` |
| `delay.py` | Delay handling | `pkg/reactor/delay.go` |
| `interrupt.py` | Signal handling | `pkg/reactor/signals.go` |
| `keepalive.py` | Keepalive handling | `pkg/reactor/keepalive.go` |

### 2.1 Peer Management (`reactor/peer/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `peer/__init__.py` | Peer class | `pkg/reactor/peer.go` |
| `peer/handlers/*.py` | Message handlers | `pkg/reactor/handlers/*.go` |

### 2.2 Network (`reactor/network/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `network/connection.py` | TCP connection | `pkg/reactor/connection.go` |
| `network/*.py` | Network utilities | `pkg/reactor/network.go` |

### 2.3 API (`reactor/api/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `api/__init__.py` | API server | `pkg/api/server.go` |
| `api/processes.py` | External processes | `pkg/api/process.go` |
| `api/error.py` | API errors | `pkg/api/error.go` |
| `api/command/*.py` | Command handlers | `pkg/api/command/*.go` |
| `api/dispatch/*.py` | Command dispatch | `pkg/api/dispatch.go` |
| `api/response/*.py` | Response encoding | `pkg/api/response.go` |
| `api/response/v4/*.py` | API v4 format | N/A (v6 only) |

---

## 3. Configuration (`configuration/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `configuration.py` | Main parser | `pkg/config/parser.go` |
| `parser.py` | Parser utilities | `pkg/config/parser.go` |
| `schema.py` | Config schema | `pkg/config/schema.go` |
| `validator.py` | Validation | `pkg/config/validator.go` |
| `validators.py` | Validator helpers | `pkg/config/validators.go` |
| `check.py` | Config checking | `pkg/config/check.go` |
| `encoder.py` | Config encoding | `pkg/config/encoder.go` |
| `storage.py` | Config storage | `pkg/config/storage.go` |
| `constraints.py` | Constraints | `pkg/config/constraints.go` |
| `command.py` | Command parsing | `pkg/config/command.go` |
| `setup.py` | Setup helpers | `pkg/config/setup.go` |
| `settings.py` | Settings | `pkg/config/settings.go` |
| `capability.py` | Capability config | `pkg/config/capability.go` |
| `example.py` | Example config | N/A |

### 3.1 Core (`configuration/core/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `core/tokeniser.py` | Tokenizer | `pkg/config/tokenizer.go` |
| `core/section.py` | Section handling | `pkg/config/section.go` |
| `core/*.py` | Core utilities | `pkg/config/core.go` |

### 3.2 Sections

| ExaBGP Directory | Purpose | ZeBGP File |
|------------------|---------|------------|
| `announce/` | Announce config | `pkg/config/announce.go` |
| `flow/` | FlowSpec config | `pkg/config/flow.go` |
| `l2vpn/` | L2VPN config | `pkg/config/l2vpn.go` |
| `neighbor/` | Neighbor config | `pkg/config/neighbor.go` |
| `operational/` | Operational | `pkg/config/operational.go` |
| `process/` | Process config | `pkg/config/process.go` |
| `static/` | Static routes | `pkg/config/static.go` |
| `template/` | Templates | `pkg/config/template.go` |

---

## 4. RIB (`rib/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `rib.py` | Main RIB | `pkg/rib/rib.go` |
| `incoming.py` | Adj-RIB-In | `pkg/rib/incoming.go` |
| `outgoing.py` | Adj-RIB-Out | `pkg/rib/outgoing.go` |
| `store.py` | Route storage | `pkg/rib/store.go` |
| `cache.py` | RIB cache | `pkg/rib/cache.go` |

---

## 5. Application (`application/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `main.py` | Entry point | `cmd/zebgp/main.go` |
| `run.py` | Run daemon | `cmd/zebgp/run.go` |
| `cli.py` | CLI interface | `cmd/zebgp-cli/main.go` |
| `shell.py` | Interactive shell | `cmd/zebgp-cli/shell.go` |
| `unixsocket.py` | Unix socket client | `cmd/zebgp-cli/socket.go` |
| `decode.py` | Message decoder | `cmd/zebgp-decode/main.go` |
| `encode.py` | Message encoder | `cmd/zebgp-decode/encode.go` |
| `validate.py` | Config validator | `cmd/zebgp/validate.go` |
| `healthcheck.py` | Health checking | N/A (Kubernetes probes) |
| `pipe.py` | Pipe handling | `pkg/api/pipe.go` |
| `server.py` | Server mode | `cmd/zebgp/server.go` |
| `flow.py` | Flow generator | N/A (separate tool) |
| `netlink.py` | Netlink interface | N/A (no FIB) |
| `shortcuts.py` | CLI shortcuts | `cmd/zebgp-cli/shortcuts.go` |
| `environ.py` | Environment | `pkg/config/environ.go` |
| `schema.py` | Schema command | `cmd/zebgp/schema.go` |
| `version.py` | Version command | `cmd/zebgp/version.go` |
| `error.py` | Error handling | `pkg/errors/errors.go` |

---

## 6. Protocol Utilities (`protocol/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `ip/address.py` | IP addresses | `netip` (stdlib) |
| `ip/tcp/*.py` | TCP utilities | `net` (stdlib) |
| `iso/*.py` | ISO utilities | `pkg/wire/iso.go` |

---

## 7. Environment (`environment/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `setup.py` | Environment setup | `pkg/config/environment.go` |
| `*.py` | Env var handling | `pkg/config/environment.go` |

---

## 8. Utilities (`util/`)

| ExaBGP File | Purpose | ZeBGP File |
|-------------|---------|------------|
| `dns.py` | DNS utilities | `net` (stdlib) |
| `ip.py` | IP utilities | `netip` (stdlib) |
| `cache.py` | Caching | `internal/cache/cache.go` |
| `od.py` | Hex dump | `encoding/hex` (stdlib) |
| `*.py` | Various utilities | `internal/util/*.go` |

---

## 9. Files NOT Mapped (Python-specific or N/A)

| ExaBGP File | Reason |
|-------------|--------|
| `vendoring/` | Go modules instead |
| `debug/` | Go debugger/pprof instead |
| `logger/` | Using `slog` stdlib |
| `data/` | Data files, not code |
| `__init__.py` | Python-specific |
| `py.typed` | Python typing marker |

---

## Key Differences

### 1. Concurrency Model
- **ExaBGP:** Python async/generator-based reactor
- **ZeBGP:** Goroutine per peer, channels for communication

### 2. Memory Management
- **ExaBGP:** Python GC, no explicit pooling
- **ZeBGP:** Explicit pool with handle-based deduplication

### 3. API Format
- **ExaBGP:** v4 and v6 JSON formats
- **ZeBGP:** v6 only (cleaner, RFC-aligned)

### 4. Configuration
- **ExaBGP:** Text file parsing with tokenizer
- **ZeBGP:** Same format + VyOS-like interactive editor

### 5. Type System
- **ExaBGP:** Runtime type checking, optional hints
- **ZeBGP:** Compile-time type checking (Go)

---

## Implementation Priority

### P0: Core Protocol ✅ COMPLETE
1. `bgp/message/` - All message types
2. `bgp/message/open/capability/` - All capabilities
3. `bgp/message/update/attribute/` - Common attributes (MP-REACH/UNREACH, AS4)
4. `bgp/message/update/nlri/inet.py` - IPv4/IPv6 unicast
5. `bgp/fsm.py` - State machine
6. `reactor/` - Event loop, peer management
7. `rib/` - Adj-RIB-In, Adj-RIB-Out, RouteStore

### P1: Common NLRI ✅ COMPLETE
1. `bgp/message/update/nlri/label.py` - MPLS labels
2. `bgp/message/update/nlri/ipvpn.py` - VPNv4/VPNv6
3. `bgp/message/update/nlri/evpn/` - EVPN types 1-5
4. FlowSpec API commands
5. VPLS/L2VPN API commands

### P2: Advanced NLRI ✅ COMPLETE
1. `bgp/message/update/nlri/flow.py` - FlowSpec
2. `bgp/message/update/nlri/bgpls/` - BGP-LS (incl. SRv6)
3. `bgp/message/update/nlri/mup/` - MUP
4. `bgp/message/update/nlri/vpls.py` - VPLS
5. `bgp/message/update/nlri/mvpn/` - MVPN
6. `bgp/message/update/nlri/rtc.py` - Route Target Constraint

### P3: Operational ✅ COMPLETE
1. `configuration/` - Config parsing
2. `environment/` - ExaBGP-compatible env var configuration
3. `reactor/api/` - API server with route announce/withdraw
4. `application/` - CLI tools (zebgp, zebgp-cli)
5. RIB show handlers (adj-rib in/out)
6. End-of-RIB (EOR) support

---

**Created:** 2025-12-19
**Last Updated:** 2025-12-20
