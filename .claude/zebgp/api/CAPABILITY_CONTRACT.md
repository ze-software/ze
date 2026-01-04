# API Capability Contract

## TL;DR

| Concept | Description |
|---------|-------------|
| **Problem** | Some BGP capabilities (GR, RR) require API to resend routes |
| **Solution** | Router queries API capability at startup, refuses if mismatch |
| **Protocol** | `capability route-refresh` query/response before peer sessions |
| **No Adj-RIB** | Router does NOT maintain Adj-RIB-Out - API handles refresh |
| **Fail-fast** | GR/RR configured without capable API = refuse to start |

**Full spec:** `plan/spec-api-capability-contract.md`

---

## Architecture

```
┌─────────────────┐     ┌─────────────────────┐
│  ZeBGP Router   │────▶│  API Process        │
│  (no Adj-RIB)   │◀────│  (maintains state)  │
└─────────────────┘     └─────────────────────┘
```

Router delegates route refresh to API. A reference implementation (`zebgp-rr`) will be provided.

---

## API-Dependent Capabilities

| Capability | API Contract |
|------------|--------------|
| Route Refresh | Resend routes on `peer X refresh` command |
| Enhanced Route Refresh | Send `borr`/`eorr` markers around resend |
| Graceful Restart | Same as Route Refresh (per-family) |

All other capabilities (ADD-PATH, 4-byte AS, etc.) are router-handled.

---

## Startup Protocol

1. Router spawns process
2. Router sends: `capability route-refresh` (or `enhanced-route-refresh`)
3. Process responds: `capability route-refresh`
4. Router validates: required ⊆ confirmed
5. If OK: start peer sessions
6. If mismatch/timeout: refuse to start

---

## Config Validation

If peer has `graceful-restart`, `route-refresh`, or `enhanced-route-refresh` but no API with `send { update; }`:

```
ERROR: peer 192.168.1.1 has graceful-restart but no API to resend routes
  hint: add "api <process> { send { update; } }" or remove capability
```

---

## Refresh Commands

**Router → API:**
```
peer 192.168.1.1 refresh ipv4/unicast
```

**API → Router:**
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

1. **Timeout**: 5 seconds for capability response
2. **Startup**: All-or-nothing (any process failure = reactor fails)
3. **Respawn**: Re-confirm capability on every spawn
4. **No Adj-RIB**: Router core doesn't track sent routes

---

**Last Updated:** 2026-01-03
