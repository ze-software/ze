# ExaBGP to Ze Code Map

## TL;DR (Read This First)

| ExaBGP | Ze | Notes |
|--------|-------|-------|
| `bgp/message/` | `internal/plugins/bgp/message/` | Message types |
| `bgp/message/open/capability/` | `internal/plugins/bgp/capability/` | Capabilities |
| `bgp/message/update/attribute/` | `internal/plugins/bgp/attribute/` | Path attributes |
| `bgp/message/update/nlri/` | `internal/plugins/bgp/nlri/` | NLRI types |
| `reactor/peer/` | `internal/plugins/bgp/reactor/peer.go` | Peer management |
| `rib/` | `internal/plugins/bgp/rib/` | Route storage |

**When to read full doc:** ExaBGP compatibility, finding reference implementation.

---

**Purpose:** Map ExaBGP source structure to Ze equivalents
**Source:** `/Users/thomas/Code/github.com/exa-networks/exabgp/main/src/exabgp/` (372 Python files)
**Target:** `internal/`, `internal/`, `cmd/`

---

## Overview

| ExaBGP Module | Files | Ze Equivalent | Notes |
|---------------|-------|------------------|-------|
| `bgp/` | 120+ | `internal/plugins/bgp/` | Core protocol |
| `reactor/` | 30+ | `internal/plugins/bgp/reactor/` | Event loop, peers |
| `configuration/` | 50+ | `internal/config/` | Config parsing |
| `rib/` | 10+ | `internal/plugins/bgp/rib/` | Route storage |
| `application/` | 20+ | `cmd/` | Entry points |
| `protocol/` | 15+ | `internal/wire/` | Wire utilities |
| `environment/` | 5+ | `internal/config/` | Env vars |
| `cli/` | 10+ | `internal/plugin/` | CLI interface |
| `logger/` | 8 | `slog` (stdlib) | Logging |
| `util/` | 12 | `internal/` | Utilities |
| `data/` | 2 | N/A | Data files |
| `debug/` | 4 | N/A | Debug utilities |
| `vendoring/` | 4 | N/A | Vendored deps |

---

## 1. BGP Protocol (`bgp/`)

### 1.1 Messages (`bgp/message/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `message.py` | Base message class | `internal/plugins/bgp/message/message.go` |
| `message_type.py` | Message type enum | `internal/plugins/bgp/message/types.go` |
| `keepalive.py` | KEEPALIVE message | `internal/plugins/bgp/message/keepalive.go` |
| `notification.py` | NOTIFICATION message | `internal/plugins/bgp/message/notification.go` |
| `refresh.py` | ROUTE-REFRESH message | `internal/plugins/bgp/message/refresh.go` |
| `operational.py` | Operational messages | `internal/plugins/bgp/message/operational.go` |
| `unknown.py` | Unknown message handling | `internal/plugins/bgp/message/unknown.go` |
| `direction.py` | Message direction enum | `internal/plugins/bgp/message/direction.go` |
| `action.py` | Message action enum | `internal/plugins/bgp/message/action.go` |
| `source.py` | Message source enum | `internal/plugins/bgp/message/source.go` |
| `scheduling.py` | Message scheduling | `internal/plugins/bgp/reactor/scheduler.go` |

### 1.2 OPEN Message (`bgp/message/open/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `open/capability/capability.py` | Capability base | `internal/plugins/bgp/capability/capability.go` |
| `open/capability/capabilities.py` | Capability collection | `internal/plugins/bgp/capability/capabilities.go` |
| `open/capability/negotiated.py` | Negotiated state | `internal/plugins/bgp/capability/negotiated.go` |
| `open/capability/mp.py` | Multiprotocol | `internal/plugins/bgp/capability/multiprotocol.go` |
| `open/capability/asn4.py` | 4-byte AS | `internal/plugins/bgp/capability/asn4.go` |
| `open/capability/addpath.py` | ADD-PATH | `internal/plugins/bgp/capability/addpath.go` |
| `open/capability/graceful.py` | Graceful Restart | `internal/plugins/bgp/capability/graceful.go` |
| `open/capability/extended.py` | Extended Message | `internal/plugins/bgp/capability/extended.go` |
| `open/capability/refresh.py` | Route Refresh | `internal/plugins/bgp/capability/refresh.go` |
| `open/capability/hostname.py` | FQDN capability | `internal/plugins/bgp/capability/hostname.go` |
| `open/capability/software.py` | Software Version | `internal/plugins/bgp/capability/software.go` |
| `open/capability/nexthop.py` | Extended Next Hop | `internal/plugins/bgp/capability/nexthop.go` |
| `open/capability/ms.py` | Multiple Paths | `internal/plugins/bgp/capability/multipaths.go` |
| `open/capability/operational.py` | Operational | `internal/plugins/bgp/capability/operational.go` |
| `open/capability/unknown.py` | Unknown capability | `internal/plugins/bgp/capability/unknown.go` |

### 1.3 UPDATE Message (`bgp/message/update/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `update/__init__.py` | UPDATE message | `internal/plugins/bgp/message/update.go` |
| `update/collection.py` | Route collection | `internal/plugins/bgp/message/update_collection.go` |
| `update/eor.py` | End of RIB | `internal/plugins/bgp/message/eor.go` |

### 1.4 Path Attributes (`bgp/message/update/attribute/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `attribute/attribute.py` | Attribute base | `internal/plugins/bgp/attribute/attribute.go` |
| `attribute/collection.py` | Attribute collection | `internal/plugins/bgp/attribute/collection.go` |
| `attribute/origin.py` | ORIGIN | `internal/plugins/bgp/attribute/origin.go` |
| `attribute/aspath.py` | AS_PATH, AS4_PATH | `internal/plugins/bgp/attribute/aspath.go` |
| `attribute/nexthop.py` | NEXT_HOP | `internal/plugins/bgp/attribute/nexthop.go` |
| `attribute/med.py` | MULTI_EXIT_DISC | `internal/plugins/bgp/attribute/med.go` |
| `attribute/localpref.py` | LOCAL_PREF | `internal/plugins/bgp/attribute/localpref.go` |
| `attribute/atomicaggregate.py` | ATOMIC_AGGREGATE | `internal/plugins/bgp/attribute/atomicaggregate.go` |
| `attribute/aggregator.py` | AGGREGATOR, AS4_AGGREGATOR | `internal/plugins/bgp/attribute/aggregator.go` |
| `attribute/originatorid.py` | ORIGINATOR_ID | `internal/plugins/bgp/attribute/originatorid.go` |
| `attribute/clusterlist.py` | CLUSTER_LIST | `internal/plugins/bgp/attribute/clusterlist.go` |
| `attribute/mprnlri.py` | MP_REACH_NLRI | `internal/plugins/bgp/attribute/mpreach.go` |
| `attribute/mpurnlri.py` | MP_UNREACH_NLRI | `internal/plugins/bgp/attribute/mpunreach.go` |
| `attribute/aigp.py` | AIGP | `internal/plugins/bgp/attribute/aigp.go` |
| `attribute/pmsi.py` | PMSI Tunnel | `internal/plugins/bgp/attribute/pmsi.go` |
| `attribute/generic.py` | Generic/unknown | `internal/plugins/bgp/attribute/generic.go` |
| `attribute/watchdog.py` | Watchdog (internal) | N/A (internal only) |

### 1.5 Communities (`bgp/message/update/attribute/community/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `community/initial/community.py` | Standard community | `internal/plugins/bgp/attribute/community.go` |
| `community/extended/*.py` | Extended communities | `internal/plugins/bgp/attribute/extcommunity.go` |
| `community/large/*.py` | Large communities | `internal/plugins/bgp/attribute/largecommunity.go` |

### 1.6 BGP-LS Attributes (`bgp/message/update/attribute/bgpls/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `bgpls/node/*.py` | Node attributes | `internal/plugins/bgp/attribute/bgpls/node.go` |
| `bgpls/link/*.py` | Link attributes | `internal/plugins/bgp/attribute/bgpls/link.go` |
| `bgpls/prefix/*.py` | Prefix attributes | `internal/plugins/bgp/attribute/bgpls/prefix.go` |

### 1.7 SR/SRv6 (`bgp/message/update/attribute/sr/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `sr/*.py` | Segment Routing | `internal/plugins/bgp/attribute/sr/sr.go` |
| `sr/srv6/*.py` | SRv6 | `internal/plugins/bgp/attribute/sr/srv6.go` |

### 1.8 NLRI Types (`bgp/message/update/nlri/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `nlri/nlri.py` | NLRI base class | `internal/plugins/bgp/nlri/nlri.go` |
| `nlri/collection.py` | NLRI collection | `internal/plugins/bgp/nlri/collection.go` |
| `nlri/cidr.py` | CIDR utilities | `internal/plugins/bgp/nlri/cidr.go` |
| `nlri/inet.py` | IPv4/IPv6 unicast | `internal/plugins/bgp/nlri/inet.go` |
| `nlri/label.py` | MPLS labels | `internal/plugins/bgp/nlri/label.go` |
| `nlri/ipvpn.py` | VPNv4/VPNv6 | `internal/plugins/bgp/nlri/ipvpn.go` |
| `nlri/flow.py` | FlowSpec | `internal/plugins/bgp/nlri/flowspec.go` |
| `nlri/vpls.py` | VPLS | `internal/plugins/bgp/nlri/vpls.go` |
| `nlri/rtc.py` | Route Target Constraint | `internal/plugins/bgp/nlri/rtc.go` |
| `nlri/settings.py` | NLRI settings | `internal/plugins/bgp/nlri/settings.go` |
| `nlri/empty.py` | Empty NLRI | `internal/plugins/bgp/nlri/empty.go` |

### 1.9 EVPN (`bgp/message/update/nlri/evpn/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `evpn/nlri.py` | EVPN base | `internal/plugins/bgp/nlri/evpn/evpn.go` |
| `evpn/ethernetad.py` | Type 1: Ethernet AD | `internal/plugins/bgp/nlri/evpn/type1.go` |
| `evpn/mac.py` | Type 2: MAC/IP | `internal/plugins/bgp/nlri/evpn/type2.go` |
| `evpn/multicast.py` | Type 3: Inclusive Multicast | `internal/plugins/bgp/nlri/evpn/type3.go` |
| `evpn/segment.py` | Type 4: Ethernet Segment | `internal/plugins/bgp/nlri/evpn/type4.go` |
| `evpn/prefix.py` | Type 5: IP Prefix | `internal/plugins/bgp/nlri/evpn/type5.go` |

### 1.10 BGP-LS NLRI (`bgp/message/update/nlri/bgpls/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `bgpls/nlri.py` | BGP-LS base | `internal/plugins/bgp/nlri/bgpls/bgpls.go` |
| `bgpls/node.py` | Node NLRI | `internal/plugins/bgp/nlri/bgpls/node.go` |
| `bgpls/link.py` | Link NLRI | `internal/plugins/bgp/nlri/bgpls/link.go` |
| `bgpls/prefixv4.py` | IPv4 Prefix NLRI | `internal/plugins/bgp/nlri/bgpls/prefix.go` |
| `bgpls/prefixv6.py` | IPv6 Prefix NLRI | `internal/plugins/bgp/nlri/bgpls/prefix.go` |
| `bgpls/tlvs/*.py` | TLV types | `internal/plugins/bgp/nlri/bgpls/tlv.go` |

### 1.11 MUP (`bgp/message/update/nlri/mup/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `mup/nlri.py` | MUP base | `internal/plugins/bgp/nlri/mup/mup.go` |
| `mup/isd.py` | Interwork SD | `internal/plugins/bgp/nlri/mup/isd.go` |
| `mup/dsd.py` | Direct SD | `internal/plugins/bgp/nlri/mup/dsd.go` |
| `mup/t1st.py` | Type 1 ST | `internal/plugins/bgp/nlri/mup/t1st.go` |
| `mup/t2st.py` | Type 2 ST | `internal/plugins/bgp/nlri/mup/t2st.go` |

### 1.12 MVPN (`bgp/message/update/nlri/mvpn/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `mvpn/nlri.py` | MVPN base | `internal/plugins/bgp/nlri/mvpn/mvpn.go` |
| `mvpn/*.py` | MVPN route types | `internal/plugins/bgp/nlri/mvpn/*.go` |

### 1.13 Qualifiers (`bgp/message/update/nlri/qualifier/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `qualifier/rd.py` | Route Distinguisher | `internal/plugins/bgp/nlri/rd.go` |
| `qualifier/esi.py` | Ethernet Segment ID | `internal/plugins/bgp/nlri/esi.go` |
| `qualifier/labels.py` | MPLS Label Stack | `internal/plugins/bgp/nlri/labels.go` |
| `qualifier/mac.py` | MAC Address | `internal/plugins/bgp/nlri/mac.go` |
| `qualifier/etag.py` | Ethernet Tag | `internal/plugins/bgp/nlri/etag.go` |
| `qualifier/path_info.py` | ADD-PATH Path ID | `internal/plugins/bgp/nlri/pathid.go` |

### 1.14 Neighbor (`bgp/neighbor/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `neighbor/neighbor.py` | Neighbor config | `internal/config/neighbor.go` |
| `neighbor/*.py` | Neighbor helpers | `internal/config/neighbor_*.go` |

### 1.15 FSM (`bgp/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `fsm.py` | State machine | `internal/plugins/bgp/fsm/fsm.go` |
| `timer.py` | BGP timers | `internal/plugins/bgp/fsm/timer.go` |

---

## 2. Reactor (`reactor/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `loop.py` | Main event loop | `internal/plugins/bgp/reactor/reactor.go` |
| `asynchronous.py` | Async handling | `internal/plugins/bgp/reactor/reactor.go` (goroutines) |
| `daemon.py` | Daemon management | `cmd/ze/bgp/daemon.go` |
| `protocol.py` | Protocol handler | `internal/plugins/bgp/reactor/protocol.go` |
| `listener.py` | TCP listener | `internal/plugins/bgp/reactor/listener.go` |
| `timing.py` | Timing utilities | `internal/plugins/bgp/reactor/timing.go` |
| `delay.py` | Delay handling | `internal/plugins/bgp/reactor/delay.go` |
| `interrupt.py` | Signal handling | `internal/plugins/bgp/reactor/signals.go` |
| `keepalive.py` | Keepalive handling | `internal/plugins/bgp/reactor/keepalive.go` |

### 2.1 Peer Management (`reactor/peer/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `peer/__init__.py` | Peer class | `internal/plugins/bgp/reactor/peer.go` |
| `peer/handlers/*.py` | Message handlers | `internal/plugins/bgp/reactor/handlers/*.go` |

### 2.2 Network (`reactor/network/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `network/connection.py` | TCP connection | `internal/plugins/bgp/reactor/connection.go` |
| `network/*.py` | Network utilities | `internal/plugins/bgp/reactor/network.go` |

### 2.3 API (`reactor/api/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `api/__init__.py` | API server | `internal/plugin/server.go` |
| `api/processes.py` | External processes | `internal/plugin/process.go` |
| `api/error.py` | API errors | `internal/plugin/error.go` |
| `api/command/*.py` | Command handlers | `internal/plugin/command/*.go` |
| `api/dispatch/*.py` | Command dispatch | `internal/plugin/dispatch.go` |
| `api/response/*.py` | Response encoding | `internal/plugin/response.go` |
| `api/response/v4/*.py` | Legacy API format | N/A |

---

## 3. Configuration (`configuration/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `configuration.py` | Main parser | `internal/config/parser.go` |
| `parser.py` | Parser utilities | `internal/config/parser.go` |
| `schema.py` | Config schema | `internal/config/schema.go` |
| `validator.py` | Validation | `internal/config/validator.go` |
| `validators.py` | Validator helpers | `internal/config/validators.go` |
| `check.py` | Config checking | `internal/config/check.go` |
| `encoder.py` | Config encoding | `internal/config/encoder.go` |
| `storage.py` | Config storage | `internal/config/storage.go` |
| `constraints.py` | Constraints | `internal/config/constraints.go` |
| `command.py` | Command parsing | `internal/config/command.go` |
| `setup.py` | Setup helpers | `internal/config/setup.go` |
| `settings.py` | Settings | `internal/config/settings.go` |
| `capability.py` | Capability config | `internal/config/capability.go` |
| `example.py` | Example config | N/A |

### 3.1 Core (`configuration/core/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `core/tokeniser.py` | Tokenizer | `internal/config/tokenizer.go` |
| `core/section.py` | Section handling | `internal/config/section.go` |
| `core/*.py` | Core utilities | `internal/config/core.go` |

### 3.2 Sections

| ExaBGP Directory | Purpose | Ze File |
|------------------|---------|------------|
| `announce/` | Announce config | `internal/config/announce.go` |
| `flow/` | FlowSpec config | `internal/config/flow.go` |
| `l2vpn/` | L2VPN config | `internal/config/l2vpn.go` |
| `neighbor/` | Neighbor config | `internal/config/neighbor.go` |
| `operational/` | Operational | `internal/config/operational.go` |
| `process/` | Process config | `internal/config/process.go` |
| `static/` | Static routes | `internal/config/static.go` |
| `template/` | Templates | `internal/config/template.go` |

---

## 4. RIB (`rib/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `rib.py` | Main RIB | `internal/plugins/bgp/rib/rib.go` |
| `incoming.py` | Adj-RIB-In | `internal/plugins/bgp/rib/incoming.go` |
| `outgoing.py` | Adj-RIB-Out | `internal/plugins/bgp/rib/outgoing.go` |
| `store.py` | Route storage | `internal/plugins/bgp/rib/store.go` |
| `cache.py` | RIB cache | `internal/plugins/bgp/rib/cache.go` |

---

## 5. Application (`application/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `main.py` | Entry point | `cmd/ze/bgp/main.go` |
| `run.py` | Run daemon | `cmd/ze/bgp/run.go` |
| `cli.py` | CLI interface | `cmd/ze/bgp-cli/main.go` |
| `shell.py` | Interactive shell | `cmd/ze/bgp-cli/shell.go` |
| `unixsocket.py` | Unix socket client | `cmd/ze/bgp-cli/socket.go` |
| `decode.py` | Message decoder | `cmd/ze/bgp-decode/main.go` |
| `encode.py` | Message encoder | `cmd/ze/bgp-decode/encode.go` |
| `validate.py` | Config validator | `cmd/ze/bgp/validate.go` |
| `healthcheck.py` | Health checking | N/A (Kubernetes probes) |
| `pipe.py` | Pipe handling | `internal/plugin/pipe.go` |
| `server.py` | Server mode | `cmd/ze/bgp/server.go` |
| `flow.py` | Flow generator | N/A (separate tool) |
| `netlink.py` | Netlink interface | N/A (no FIB) |
| `shortcuts.py` | CLI shortcuts | `cmd/ze/bgp-cli/shortcuts.go` |
| `environ.py` | Environment | `internal/config/environ.go` |
| `schema.py` | Schema command | `cmd/ze/bgp/schema.go` |
| `version.py` | Version command | `cmd/ze/bgp/version.go` |
| `error.py` | Error handling | `internal/errors/errors.go` |

---

## 6. Protocol Utilities (`protocol/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `ip/address.py` | IP addresses | `netip` (stdlib) |
| `ip/tcp/*.py` | TCP utilities | `net` (stdlib) |
| `iso/*.py` | ISO utilities | `internal/wire/iso.go` |

---

## 7. Environment (`environment/`)

| ExaBGP File | Purpose | Ze File |
|-------------|---------|------------|
| `setup.py` | Environment setup | `internal/config/environment.go` |
| `*.py` | Env var handling | `internal/config/environment.go` |

---

## 8. Utilities (`util/`)

| ExaBGP File | Purpose | Ze File |
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
- **Ze:** Goroutine per peer, channels for communication

### 2. Memory Management
- **ExaBGP:** Python GC, no explicit pooling
- **Ze:** Explicit pool with handle-based deduplication

### 3. API Format
- **ExaBGP:** Multiple JSON format versions
- **Ze:** Single unified format (RFC-aligned)

### 4. Configuration
- **ExaBGP:** Text file parsing with tokenizer
- **Ze:** Same format + VyOS-like interactive editor

### 5. Type System
- **ExaBGP:** Runtime type checking, optional hints
- **Ze:** Compile-time type checking (Go)

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
4. `application/` - CLI tools (zebgp, ze-bgp-cli)
5. RIB show handlers (adj-rib in/out)
6. End-of-RIB (EOR) support

---

**Created:** 2025-12-19
**Last Updated: 2026-01-30
