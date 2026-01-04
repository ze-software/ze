# API Capability Contract

## Problem

Some BGP capabilities require API process cooperation:
- **Graceful Restart**: API must resend routes on reconnection
- **Route Refresh**: API must resend routes on refresh request
- **Enhanced Route Refresh**: API must handle BoRR/EoRR markers

Router MUST NOT advertise capabilities it cannot fulfill.

## Solution

Two-phase capability declaration:
1. **Config declares requirements** - what capabilities API should support
2. **Process confirms support** - runtime confirmation before peer sessions start

## Config Syntax

Capabilities configured at **peer level** (unchanged):

```
peer 192.168.1.1 {
  capability {
    graceful-restart 120;
    route-refresh;
  }
  api announce-routes {
    send { update; }
  }
}
```

Router derives required API capabilities from peer config + API binding.

## Protocol

### Startup Sequence

```
┌─────────┐                      ┌─────────┐
│ Router  │                      │ Process │
└────┬────┘                      └────┬────┘
     │                                │
     │──── spawn process ────────────>│
     │                                │
     │──── capability route-refresh ──│  (router states what it needs)
     │                                │
     │<─── capability route-refresh ──│  (process confirms, within 5s)
     │                                │
     │──── validate ──────────────────│
     │     required ⊆ confirmed?      │
     │                                │
     │──── (if OK) start peers ──────>│
     │                                │
```

### Capability Query

Router sends required capabilities to process stdin.

**Text format:**
```
capability route-refresh
capability route-refresh enhanced-route-refresh
```

**JSON format:**
```json
{"type": "capability", "require": ["route-refresh"]}
{"type": "capability", "require": ["route-refresh", "enhanced-route-refresh"]}
```

### Capability Response

Process responds on stdout (single line).

**Text format:**
```
capability route-refresh
capability route-refresh enhanced-route-refresh
```

**JSON format:**
```json
{"type": "capability", "support": ["route-refresh"]}
{"type": "capability", "support": ["route-refresh", "enhanced-route-refresh"]}
```

**Rules:**
- Must respond within **5 seconds** of query
- Single line response
- Order doesn't matter
- Process confirms what it supports (may include extras, can lie)
- Empty response means no API-dependent capabilities supported:
  - Text: `capability`
  - JSON: `{"type": "capability", "support": []}`
- Router validates: all required caps present in response
- Encoding (text/JSON) determined by process config `encoder` setting

### Validation

Router validates: `config_required ⊆ process_confirmed`

| Router Sends | Process Responds | Result |
|-------------|------------------|--------|
| `capability route-refresh` | `capability route-refresh enhanced-route-refresh` | ✅ OK (extras) |
| `capability route-refresh enhanced-route-refresh` | `capability enhanced-route-refresh route-refresh` | ✅ OK (order differs) |
| `capability route-refresh enhanced-route-refresh` | `capability route-refresh` | ❌ FAIL (missing enhanced-route-refresh) |
| `capability route-refresh` | `capability` | ❌ FAIL (missing route-refresh) |
| `capability` | `capability route-refresh` | ✅ OK (no requirements) |

### Failure Behavior

On mismatch or timeout:
1. Log error with details (required vs confirmed)
2. Kill process
3. Do NOT start peer sessions bound to this process
4. Reactor startup fails if any required process fails

```
ERROR: process "announce-routes" capability mismatch
  required: [route-refresh]
  confirmed: []
  missing: [route-refresh]
```

## API-Dependent Capabilities

Only two capabilities require API confirmation:

| Capability | Config Key | API Contract |
|------------|------------|--------------|
| Route Refresh | `route-refresh` | Resend routes on `peer X refresh` command |
| Enhanced Route Refresh | `enhanced-route-refresh` | Send `borr`/`eorr` markers around route resend |

**Note:** Graceful Restart uses the same `peer X refresh` mechanism as Route Refresh.
If API confirms `route-refresh`, it implicitly supports GR resend behavior.
GR just triggers refresh for all negotiated families on reconnect.

All other capabilities (ADD-PATH, 4-byte AS, Extended Message, etc.) are router-handled.
ADD-PATH: if API omits path-id, router defaults to 0.

## Capability Behavior Contracts

### graceful-restart

When peer reconnects after session drop:
1. Router sends to API (one per negotiated family):

   **Text:**
   ```
   peer 192.168.1.1 refresh ipv4/unicast
   peer 192.168.1.1 refresh ipv6/unicast
   ```
   **JSON:**
   ```json
   {"type": "refresh", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
   {"type": "refresh", "peer": "192.168.1.1", "afi": "ipv6", "safi": "unicast"}
   ```

2. Process resends routes using borr/eorr markers:

   **Text:**
   ```
   peer 192.168.1.1 borr ipv4/unicast
   announce route 10.0.0.0/24 next-hop self
   peer 192.168.1.1 eorr ipv4/unicast
   ```
   **JSON:** (borr/eorr are JSON, announce remains text command)
   ```json
   {"type": "borr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
   ```
   ```
   announce route 10.0.0.0/24 next-hop self
   ```
   ```json
   {"type": "eorr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
   ```

   **Note:** Route announce/withdraw commands use existing text format regardless of encoder setting. Only control messages (capability, refresh, borr, eorr) use JSON when `encoder json`.

3. Router tracks completion internally (borr/eorr NOT sent on wire for GR)
4. When all families complete, router sends End-of-RIB markers

**Note:** Same API commands as enhanced-route-refresh, different wire behavior.
GR: borr/eorr are internal signals. ERR: borr/eorr are sent on wire.

**Completion tracking:**
- If API sends borr/eorr: router knows exactly when each family is done
- If API only confirms `route-refresh` (not `enhanced-route-refresh`):
  - API may not send borr/eorr markers
  - Router uses restart-time from GR capability as timeout
  - After timeout, send End-of-RIB for remaining families

### route-refresh

When router receives ROUTE-REFRESH from peer:
1. Router sends to API:

   **Text:** `peer 192.168.1.1 refresh ipv4/unicast`

   **JSON:** `{"type": "refresh", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}`

2. Process MUST resend all routes for that peer/family
3. Router forwards routes to peer as they arrive

**Note:** Router doesn't wait for completion - routes are forwarded immediately.
For bounded refresh (knowing when done), use `enhanced-route-refresh`.

### enhanced-route-refresh

When router receives ROUTE-REFRESH from peer:
1. Router sends: `peer 192.168.1.1 refresh ipv4/unicast`
2. Process sends: `peer 192.168.1.1 borr ipv4/unicast`
3. Router sends BoRR to peer
4. Process resends routes
5. Process sends: `peer 192.168.1.1 eorr ipv4/unicast`
6. Router sends EoRR to peer

API commands for Enhanced RR:

**Text:**
```
peer 192.168.1.1 borr ipv4/unicast
peer 192.168.1.1 eorr ipv4/unicast
```

**JSON:**
```json
{"type": "borr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
{"type": "eorr", "peer": "192.168.1.1", "afi": "ipv4", "safi": "unicast"}
```

## Implementation

### Capability Derivation

Router derives required API capabilities per process from peer config:

```go
// pkg/reactor/reactor.go

func (r *Reactor) deriveAPICapabilities(peer *PeerConfig, binding *APIBinding) []string {
    // Only processes that can send updates need capability confirmation
    if !binding.Send.Update {
        return nil
    }

    var caps []string
    // GR requires route-refresh (same mechanism)
    if peer.Capabilities.GracefulRestart || peer.Capabilities.RouteRefresh {
        caps = append(caps, "route-refresh")
    }
    if peer.Capabilities.EnhancedRouteRefresh {
        caps = append(caps, "enhanced-route-refresh")
    }
    return caps
}
```

### Process Manager

```go
// pkg/api/process.go

type Process struct {
    // ... existing fields
    confirmedCaps map[string]bool  // runtime confirmed capabilities
}

func (p *Process) QueryCapability(ctx context.Context, required []string) error {
    // 1. Send "capability cap1 cap2 ...\n" to process stdin
    // 2. Wait for "capability ..." response (5s timeout)
    // 3. Parse response into set of confirmed caps
    // 4. Validate: all required caps present in response
    // 5. Return error on mismatch or timeout
}
```

### Reactor Integration

```go
// pkg/reactor/reactor.go

func (r *Reactor) Start(ctx context.Context) error {
    // 1. Spawn all processes
    // 2. Derive required caps per process from peer configs
    // 3. Query capability from each process (parallel)
    //    - Send "capability cap1 cap2 ..." to each
    //    - Wait for responses (5s timeout each)
    // 4. Validate: each process confirms all required caps
    // 5. ONLY THEN start peer sessions
}
```

## Peer Capability Advertisement

Router advertises capability to peer ONLY IF:
1. Config enables the capability for that peer
2. ALL API processes with `send { update; }` bound to peer confirm support

```
peer 192.168.1.1 {
  capability {
    graceful-restart 120;  # Enabled in config
  }
  api announce-routes {    # Bound process
    send { update; }
  }
}
```

If process `announce-routes` doesn't confirm `route-refresh`:
- Peer session does NOT start
- Error logged

## Edge Cases

### No API Binding

If peer has `graceful-restart`, `route-refresh`, or `enhanced-route-refresh` configured but no `api` block with `send { update; }`:
- **Refuse to start** - configuration error
- Router does not integrate Adj-RIB-Out, cannot fulfill refresh contract
- User must either:
  - Remove the capability from peer config, OR
  - Add API binding with `send { update; }`

```
ERROR: peer 192.168.1.1 has graceful-restart but no API to resend routes
  hint: add "api <process> { send { update; } }" or remove capability
```

### Architecture Note

ZeBGP keeps Adj-RIB code separate from router core. A reference API program (`zebgp-rr` or similar) will be provided that:
- Maintains Adj-RIB-Out per peer
- Confirms `route-refresh` capability
- Handles `peer X refresh` commands by resending from Adj-RIB

Users who need GR/RR can use this program or implement their own.

### Multiple API Bindings

If peer has multiple `api` blocks:
- Only processes with `send { update; }` need capability confirmation
- Receive-only processes don't inject routes, can't resend

### Receive-Only Process

If process only has `receive { ... }` (no `send`):
- No capability confirmation required
- Process cannot inject routes, so refresh doesn't apply

### Process Bound to Multiple Peers

If process is bound to multiple peers with different capability requirements:
- Router aggregates: process must confirm union of all required caps
- Example: peer A needs `route-refresh`, peer B needs `route-refresh enhanced-route-refresh`
- Router queries: `capability route-refresh enhanced-route-refresh`

### Proactive borr/eorr

API can send `borr`/`eorr` without explicit refresh request:
- Initial route announcement after session establishment
- Triggered updates (e.g., policy change)
- Router behavior depends on peer capability:
  - Peer supports ERR → send BoRR/EoRR on wire
  - Peer doesn't support ERR → silently ignore borr/eorr (just forward routes)

### Invalid Response Handling

If process sends invalid/malformed capability response:
- Treat as timeout (5s passes with no valid response)
- Log parsing error
- Fail startup (same as missing capability)

## Design Decisions

1. **Timeout**: 5 seconds (fixed, not configurable)
2. **Partial startup**: All-or-nothing (any process failure = reactor startup fails)
3. **Respawn**: Re-confirm on every spawn (no caching)

### Respawn Failure Handling

If respawned process fails capability confirmation:
1. Log error
2. Tear down all peer sessions bound to this process
3. Mark process as failed (no further respawn attempts)
4. Router continues running with remaining peers (graceful degradation at runtime)

**Rationale**: Startup is strict (all-or-nothing), but runtime failures degrade gracefully to avoid cascading outages.

## References

- RFC 4724: Graceful Restart Mechanism for BGP
- RFC 2918: Route Refresh Capability for BGP-4
- RFC 7313: Enhanced Route Refresh Capability for BGP-4
