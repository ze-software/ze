# API Capability Contract

## Implementation Status

| Feature | Status | Notes |
|---------|--------|-------|
| `msg-id` in events | ✅ Done | `internal/component/plugin/json.go` |
| `bgp cache forward <id> <sel>` | ✅ Done | `internal/component/bgp/plugins/cmd/cache/` |
| `capability route-refresh` | ✅ Done | `internal/component/plugin/rr/` |
| `plugin session ready` | ✅ Done | `internal/component/plugin/plugin.go` |
| Refresh event handling | ✅ Done | `internal/component/plugin/rr/` |
| `bgp cache retain/release/expire <id>` | ✅ Done | `internal/component/bgp/plugins/cmd/cache/` |
| `bgp cache list` | ✅ Done | `internal/component/bgp/plugins/cmd/cache/` |
| Stage timeout | ✅ Done | Configurable per-plugin, default 5s |
| Config validation (GR/RR→API) | ✅ Done | Config validation |
| `borr`/`eorr` markers | ✅ Done | RFC 7313 full support, RIB plugin responds to refresh |
<!-- source: internal/component/bgp/reactor/reactor.go -- cache management -->

---

## TL;DR

| Concept | Description |
|---------|-------------|
| **Problem** | Some BGP capabilities (GR, RR) require API to resend routes |
| **Solution** | API owns RIB, controls cache lifetime via `bgp cache` |
| **Protocol** | `capability route-refresh` advertised at startup |
| **RIB** | API program owns all route storage |
| **Cache Control** | API retains cache entries for replay, releases when done |
| **Fail-fast** | GR/RR configured without capable API = refuse to start |

**Design history:** `plan/learned/DESIGN-HISTORY.md`, "Plugin system: architecture"
(retired summary 172). Its Evolution subsection carries the capability-declaration
chain and the RFC-scoped config-key rule; its Load-bearing invariants carry the
`PluginFailed()`/`proc.Stop()` obligation on every startup error path.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Ze Engine (Minimal)                                              │
│  • FSM, parsing, wire I/O                                           │
│  • BGP cache (lifetime controlled by API via `bgp cache` commands)  │
│  • NO RIB, NO route storage                                         │
└─────────────────────────────────────────────────────────────────────┘
                    │ YANG RPC events + cached msg-ids
                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  API Process (Full RIB Owner)                                        │
│  • Route storage with pool deduplication                            │
│  • Best-path selection (if needed)                                  │
│  • Graceful restart state                                           │
│  • msg-id retain/release control                                    │
└─────────────────────────────────────────────────────────────────────┘
```

Engine delegates all route storage to API. Reference implementations: `bgp-rs`, `bgp-rib`.

---

## API-Dependent Capabilities

| Capability | API Responsibility |
|------------|-------------------|
| Route Refresh | Resend routes from RIB on `refresh` event |
| Enhanced Route Refresh | Send `borr`/`eorr` markers around resend |
| Graceful Restart | Retain routes across peer restart, replay on reconnect |

All other capabilities (ADD-PATH, 4-byte AS, etc.) are engine-handled.
<!-- source: internal/component/bgp/reactor/peer.go -- Peer capabilities -->

---

## BGP Cache Control

API controls BGP cache lifetime in engine:

| Command | Description |
|---------|-------------|
| `bgp cache retain <id>` | Keep in cache until released |
| `bgp cache release <id>` | Allow eviction (default 60s timeout) |
| `bgp cache expire <id>` | Remove immediately |
| `bgp cache list` | List cached msg-ids |
| `bgp cache forward <id> <sel>` | Forward cached UPDATE to peers |

### Graceful Restart Flow

```
1. Peer A announces route (msg-id 123)
2. Engine sends event to API
3. API stores in RIB, sends: bgp cache retain 123
4. ... Peer A goes down ...
5. ... Peer A reconnects ...
6. Engine sends state event: peer A up
7. API replays: bgp cache forward 123 A
8. API sends: peer A eor ipv4/unicast
```

### Long Outage (cache expired)

If cache was cleared (shouldn't happen with retain), API can re-announce from pool:

```
bgp peer 192.0.2.1 raw update hex <update-payload-hex>
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
plugin {
    external myapp {
        run ./myapp;
        timeout 10s;    # per-stage timeout (default: 5s)
    }
}
```
<!-- source: internal/component/plugin/process/process.go -- Process timeout -->

See `docs/architecture/config/syntax.md` for full plugin config options.

---

## Config Validation (✅ DONE)

> **Status:** Implemented in `validatePeerProcessCaps`.
<!-- source: internal/component/bgp/config/peers.go -- validatePeerProcessCaps -->

If a peer has `graceful-restart` or `route-refresh` and attaches no process:

```
peer 192.168.1.1: route-refresh requires an attached process with send [ update ] or send [ raw ]
  the peer attaches no process
```

Or if it attaches processes and none of them carries either word:

```
peer 192.168.1.1: route-refresh requires an attached process with send [ update ] or send [ raw ]
  configured: attach process logger, attach process monitor - none carry either word
```

The capability needs a program that can re-advertise the routes a refresh asks
for. Either rail is that program: ze builds the UPDATE from the process's own
route operation (`send [ update ]`), or the process hands over a whole message
it built itself (`send [ raw ]`). The check reads
`ProcessBinding.MayPushRoutes`, which is the same predicate the initial-sync
barrier counts, so a config that passes this check is a config whose announce
will not be refused.
<!-- source: internal/component/bgp/config/peers.go -- validatePeerProcessCaps -->
<!-- source: internal/component/bgp/reactor/peer_settings.go -- ProcessBinding.MayPushRoutes -->

---

## Refresh Commands (✅ DONE)

> **Status:** Full RFC 7313 Enhanced Route Refresh support implemented.

**Router → API:** ✅ Implemented
```
peer peer-east refresh ipv4/unicast
```

**API → Router:** ✅ Done (`refresh.go`, `reactor.go`)
```
peer peer-east borr ipv4/unicast
update text nhop set self nlri ipv4/unicast add 10.0.0.0/24
peer peer-east eorr ipv4/unicast
```
**RFC 7313 compliance:**
- Enhanced Route Refresh capability check before sending BoRR/EoRR
- Config `route-refresh` enables both RouteRefresh and EnhancedRouteRefresh capabilities
<!-- source: internal/component/bgp/format/text.go -- AppendRouteRefresh -->

---

## JSON Format

When `encoding json`:

```json
{"type":"bgp","bgp":{"message":{"type":"refresh","direction":"received"},"peer":{"address":"192.168.1.1","local":{"address":"192.168.1.2","as":65000},"remote":{"address":"192.168.1.1","as":65001}},"refresh":{"afi":"ipv4","safi":"unicast"}}}
{"type":"bgp","bgp":{"message":{"type":"borr","direction":"received"},"peer":{"address":"192.168.1.1","local":{"address":"192.168.1.2","as":65000},"remote":{"address":"192.168.1.1","as":65001}},"borr":{"afi":"ipv4","safi":"unicast"}}}
{"type":"bgp","bgp":{"message":{"type":"eorr","direction":"received"},"peer":{"address":"192.168.1.1","local":{"address":"192.168.1.2","as":65000},"remote":{"address":"192.168.1.1","as":65001}},"eorr":{"afi":"ipv4","safi":"unicast"}}}
```

---

## Design Decisions

1. **Timeout**: Default 5s per stage, configurable per-plugin via `timeout` keyword
2. **Startup**: All-or-nothing (any process failure = reactor fails)
3. **Respawn**: Re-confirm capability on every spawn
4. **RIB in API**: Engine has NO route storage - API owns all
5. **Cache Control**: API decides cache lifetime via `bgp cache` commands
6. **Polyglot**: API can be Go, Python, Rust, etc.

---

## Reference Implementations

| Plugin | Use Case | RIB Type |
|--------|----------|----------|
| `bgp-rs` | Route Server (multi-peer) | ribIn (routes FROM peers) |
| `bgp-rib` | Full RIB (Adj-RIB-In/Out) | Both ribIn and ribOut |
<!-- source: internal/component/bgp/plugins/rs/ -- route server plugin -->
<!-- source: internal/component/bgp/plugins/rib/ -- RIB plugin -->

See `plan/spec-api-rr.md` for implementation details.

---

**Last Updated:** 2026-01-12 (configurable per-plugin stage timeout)
