# API Capability Contract

## Implementation Status

| Feature | Status | Notes |
|---------|--------|-------|
| `msg-id` in events | ✅ Done | `json.go`, `text.go` |
| `forward update-id` | ✅ Done | `forward.go` |
| `capability route-refresh` | ✅ Done | `rr/server.go:68` |
| `session api ready` | ✅ Done | `session.go` |
| Refresh event handling | ✅ Done | `rr/server.go`, `decode.go` |
| `msg-id retain/release/expire` | ❌ Not impl | No handlers registered |
| `msg-id list` | ❌ Not impl | No handler |
| 5s startup timeout | ❌ Not impl | No validation |
| Config validation (GR→API) | ❌ Not impl | No fail-fast check |
| `borr`/`eorr` markers | ❌ Not impl | No handlers |

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

Engine delegates all route storage to API. Reference implementations: `zebgp api rr`, `zebgp api persist`.

---

## API-Dependent Capabilities

| Capability | API Responsibility |
|------------|-------------------|
| Route Refresh | Resend routes from RIB on `refresh` event |
| Enhanced Route Refresh | Send `borr`/`eorr` markers around resend |
| Graceful Restart | Retain routes across peer restart, replay on reconnect |

All other capabilities (ADD-PATH, 4-byte AS, etc.) are engine-handled.

---

## msg-id Cache Control (❌ NOT IMPLEMENTED)

> **Status:** These commands are documented but not yet implemented.
> Requires: handler registration in `handler.go`, cache store in reactor.

API controls msg-id cache lifetime in engine:

| Command | Description |
|---------|-------------|
| `msg-id <id> retain` | Keep in cache until released |
| `msg-id <id> release` | Allow eviction (default 60s timeout) |
| `msg-id <id> expire` | Remove immediately |
| `msg-id list` | List cached msg-ids |

### Graceful Restart Flow

```
1. Peer A announces route (msg-id 123)
2. Engine sends event to API
3. API stores in RIB, sends: msg-id 123 retain
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

## Startup Protocol (⚠️ PARTIAL)

> **Status:** `session api ready` exists but no 5s timeout validation.
> Process can advertise capabilities but engine doesn't validate or fail-fast.

1. Engine spawns process
2. Process advertises: `capability route-refresh` (within 5s) ❌ no timeout
3. Engine collects all process capabilities ❌ not collected
4. Engine validates: config requirements ⊆ process capabilities ❌ no validation
5. If OK: start peer sessions
6. If mismatch/timeout: refuse to start ❌ no fail-fast

---

## Config Validation (❌ NOT IMPLEMENTED)

> **Status:** No validation exists. Config loader accepts GR/RR without API.

If peer has `graceful-restart`, `route-refresh`, or `enhanced-route-refresh` but no API with `send { update; }`:

```
ERROR: peer 192.168.1.1 has graceful-restart but no API to resend routes
  hint: add "api <process> { send { update; } }" or remove capability
```

---

## Refresh Commands (⚠️ PARTIAL)

> **Status:** `refresh` event sent to API. `borr`/`eorr` handlers not implemented.

**Router → API:** ✅ Implemented
```
peer 192.168.1.1 refresh ipv4/unicast
```

**API → Router:** ❌ borr/eorr not implemented
```
peer 192.168.1.1 borr ipv4/unicast
announce route 10.0.0.0/24 next-hop self
peer 192.168.1.1 eorr ipv4/unicast
```

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

1. **Timeout**: 5 seconds for capability advertisement
2. **Startup**: All-or-nothing (any process failure = reactor fails)
3. **Respawn**: Re-confirm capability on every spawn
4. **RIB in API**: Engine has NO route storage - API owns all
5. **msg-id Control**: API decides cache lifetime, not engine
6. **Polyglot**: API can be Go, Python, Rust, etc.

---

## Reference Implementations

| Plugin | Use Case | RIB Type |
|--------|----------|----------|
| `zebgp api rr` | Route Server (multi-peer) | ribIn (routes FROM peers) |
| `zebgp api persist` | State persistence | ribOut (routes TO peers) |

See `docs/plan/spec-api-rr.md` for implementation details.

---

**Last Updated:** 2026-01-04 (implementation status reviewed)
