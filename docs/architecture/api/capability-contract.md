# API Capability Contract

## Implementation Status

| Feature | Status | Notes |
|---------|--------|-------|
| `msg-id` in events | ✅ Done | `json.go`, `text.go` |
| `forward update-id` | ✅ Done | `forward.go` |
| `capability route-refresh` | ✅ Done | `rr/server.go:68` |
| `session api ready` | ✅ Done | `session.go` |
| Refresh event handling | ✅ Done | `rr/server.go`, `decode.go` |
| `msg-id retain/release/expire` | ✅ Done | `msgid.go` |
| `msg-id list` | ✅ Done | `msgid.go` |
| Stage timeout | ✅ Done | Configurable per-plugin, default 5s |
| Config validation (GR/RR→API) | ✅ Done | `bgp.go:validateProcessCapabilities` |
| `borr`/`eorr` markers | ✅ Done | RFC 7313 full support, RIB plugin responds to refresh |

---

## TL;DR

| Concept | Description |
|---------|-------------|
| **Problem** | Some BGP capabilities (GR, RR) require API to resend routes |
| **Solution** | API owns RIB, controls msg-id cache lifetime |
| **Protocol** | `capability route-refresh` advertised at startup |
| **RIB** | API program owns all route storage |
| **msg-id Control** | API retains msg-ids for replay, releases when done |
| **Fail-fast** | GR/RR configured without capable API = refuse to start |

**Full spec:** `docs/plan/spec-api-capability-contract.md`

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  ZeBGP Engine (Minimal)                                              │
│  • FSM, parsing, wire I/O                                           │
│  • msg-id cache (lifetime controlled by API)                        │
│  • NO RIB, NO route storage                                         │
└─────────────────────────────────────────────────────────────────────┘
                    │ JSON events + base64 wire bytes
                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  API Process (Full RIB Owner)                                        │
│  • Route storage with pool deduplication                            │
│  • Best-path selection (if needed)                                  │
│  • Graceful restart state                                           │
│  • msg-id retain/release control                                    │
└─────────────────────────────────────────────────────────────────────┘
```

Engine delegates all route storage to API. Reference implementations: `zebgp plugin rr`, `zebgp plugin rib`.

---

## API-Dependent Capabilities

| Capability | API Responsibility |
|------------|-------------------|
| Route Refresh | Resend routes from RIB on `refresh` event |
| Enhanced Route Refresh | Send `borr`/`eorr` markers around resend |
| Graceful Restart | Retain routes across peer restart, replay on reconnect |

All other capabilities (ADD-PATH, 4-byte AS, etc.) are engine-handled.

---

## msg-id Cache Control

API controls msg-id cache lifetime in engine:

| Command | Description |
|---------|-------------|
| `msg-id retain <id>` | Keep in cache until released |
| `msg-id release <id>` | Allow eviction (default 60s timeout) |
| `msg-id expire <id>` | Remove immediately |
| `msg-id list` | List cached msg-ids |

### Graceful Restart Flow

```
1. Peer A announces route (msg-id 123)
2. Engine sends event to API
3. API stores in RIB, sends: msg-id retain 123
4. ... Peer A goes down ...
5. ... Peer A reconnects ...
6. Engine sends state event: peer A up
7. API replays: peer A forward update-id 123
8. API sends: peer A eor ipv4/unicast
```

### Long Outage (msg-id expired)

If msg-id cache was cleared (shouldn't happen with retain), API can re-announce from pool:

```
peer A announce raw <base64-attrs> nlri ipv4/unicast <base64-nlri>
```

---

## Startup Protocol (✅ DONE)

> **Status:** 5-stage startup with configurable per-plugin timeout.

1. Engine spawns process
2. Process completes 5 stages: Declaration → Config → Capability → Registry → Running
3. Each stage must complete within timeout (default 5s, configurable per-plugin)
4. All plugins must complete each stage before any can proceed
5. On timeout/failure: plugin marked failed, startup aborted

**Timeout configuration:**
```
plugin myapp {
    run ./myapp;
    timeout 10s;    # per-stage timeout (default: 5s)
}
```

See `docs/architecture/config/syntax.md` for full plugin config options.

---

## Config Validation (✅ DONE)

> **Status:** Implemented in `pkg/config/bgp.go:validateProcessCapabilities`.

If peer has `graceful-restart` or `route-refresh` but no process with `send { update; }`:

```
peer 192.168.1.1: route-refresh requires process with send { update; }
  no process bindings configured
```

Or if process bindings exist but none have `send { update; }`:

```
peer 192.168.1.1: route-refresh requires process with send { update; }
  configured: process logger, process monitor - none have send { update; }
```

---

## Refresh Commands (✅ DONE)

> **Status:** Full RFC 7313 Enhanced Route Refresh support implemented.

**Router → API:** ✅ Implemented
```
peer 192.168.1.1 refresh ipv4/unicast
```

**API → Router:** ✅ Done (`refresh.go`, `reactor.go`)
```
peer 192.168.1.1 borr ipv4/unicast
announce route 10.0.0.0/24 next-hop self
peer 192.168.1.1 eorr ipv4/unicast
```
**RFC 7313 compliance:**
- Enhanced Route Refresh capability check before sending BoRR/EoRR
- Config `route-refresh` enables both RouteRefresh and EnhancedRouteRefresh capabilities

---

## JSON Format

When `encoder json`:

```json
{"type": "refresh", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
{"type": "borr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
{"type": "eorr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
```

---

## Design Decisions

1. **Timeout**: Default 5s per stage, configurable per-plugin via `timeout` keyword
2. **Startup**: All-or-nothing (any process failure = reactor fails)
3. **Respawn**: Re-confirm capability on every spawn
4. **RIB in API**: Engine has NO route storage - API owns all
5. **msg-id Control**: API decides cache lifetime, not engine
6. **Polyglot**: API can be Go, Python, Rust, etc.

---

## Reference Implementations

| Plugin | Use Case | RIB Type |
|--------|----------|----------|
| `zebgp plugin rr` | Route Server (multi-peer) | ribIn (routes FROM peers) |
| `zebgp plugin rib` | Full RIB (Adj-RIB-In/Out) | Both ribIn and ribOut |

See `docs/plan/spec-api-rr.md` for implementation details.

---

**Last Updated:** 2026-01-12 (configurable per-plugin stage timeout)
