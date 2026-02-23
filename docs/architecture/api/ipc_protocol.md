# Ze IPC Protocol Specification

**Status:** Canonical reference for Ze inter-process communication

---

## Overview

Ze uses a line-delimited protocol for communication between the engine and external processes (plugins, CLI clients). The protocol supports:

- Fire-and-forget commands (no response)
- Request-response with correlation IDs
- Streaming responses with partial results
- Event subscription model
- Bidirectional communication over Unix sockets or stdin/stdout pipes

---

## Concepts

### Subsystems

A **subsystem** is a protocol component that follows the ZE API for plugin communication:

| Subsystem | Description |
|-----------|-------------|
| `bgp` | BGP protocol subsystem |
| `rib` | RIB subsystem (protocol-agnostic) |
| Future | `bmp`, `rpki`, etc. |

### Event Subscription

Plugins subscribe to events via API commands (not config). This allows:
- Plugin declares what it needs (self-describing)
- Dynamic subscribe/unsubscribe at runtime
- Config file only specifies which plugins to run

### Plugin-Provided Commands

Some commands are registered by plugins via `declare cmd` during startup:
- Plugin-specific commands (e.g., `rib adjacent *` from RIB plugin)

Engine provides reactor methods; plugins register commands that use them.

### Message Cache

The engine maintains a message cache for efficient forwarding. Commands:
- `bgp cache <id> retain/release/expire` - cache control (engine builtins)
- `bgp cache <id> forward <sel>` - forward cached UPDATE to peers
- `bgp cache <id1>,<id2>,...,<idN> forward <sel>` - batch forward (comma-separated IDs)
- `bgp cache <id1>,<id2>,...,<idN> release` - batch release
- `bgp cache list` - list cached message IDs

---

## Transport Layer

### Unix Socket (CLI clients)

```
Path: configured via `api { socket "/path/to/socket"; }`
Mode: stream, line-delimited
Direction: bidirectional
```

### Subprocess Pipes (plugins)

```
stdin:  Engine → Plugin (events, config, requests)
stdout: Plugin → Engine (commands, responses)
Mode: line-delimited text
```

---

## Wire Format

### Message Framing

All messages are UTF-8 encoded, newline-delimited:

```
<message>\n
```

Each line is a complete message. No multi-line messages.

### Command Format (Plugin/CLI → Engine)

```abnf
command     = [serial] verb *argument
serial      = "#" 1*DIGIT SP
verb        = 1*ALPHA
argument    = token / quoted-string
token       = 1*(%x21-7E)  ; printable non-space
quoted-string = DQUOTE *(%x20-21 / %x23-7E) DQUOTE
```

### Response Format (Engine → Plugin/CLI)

JSON with `type` field indicating the payload key:

```json
{"type":"response","response":{"serial":"1","status":"done","data":{...}}}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Always | `"response"` - indicates payload is in `response` key |
| `response` | object | Always | Response payload |
| `response.serial` | string | If request had serial | Correlation ID |
| `response.status` | string | Always | `"done"`, `"error"`, `"warning"`, or `"ack"` |
| `response.partial` | bool | If streaming | `true` for intermediate chunks |
| `response.data` | any | Optional | Payload (result or error message) |

### Event Format (Engine → Plugin)

JSON with `type` field indicating which key contains the payload. The `peer` field is at the `bgp` level; event-specific data is nested under the event type key:

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "10.0.0.1", "asn": 65001},
    "update": {
      "message": {"id": 123, "direction": "received"},
      "attr": {"origin": "igp", "as-path": [65001]},
      "nlri": {"ipv4/unicast": [{"action": "add", "next-hop": "10.0.0.1", "nlri": ["10.0.0.0/24"]}]}
    }
  }
}
```

**Exception:** State events use a simple string value for `state` instead of a container:

```json
{"type": "bgp", "bgp": {"type": "state", "peer": {...}, "state": "up"}}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Always | `"bgp"`, `"rib"` - indicates payload key |
| `bgp` | object | If type=bgp | BGP event payload |
| `rib` | object | If type=rib | RIB event payload |

**BGP event payload (`bgp` key):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Always | `update`, `open`, `notification`, `keepalive`, `refresh`, `state`, `negotiated` |
| `peer` | object | Always | `{"address":"<ip>", "asn":<asn>}` - at bgp level |
| `<type>` | object/string | Usually | Event data nested under event type key (string for state events) |

**BGP event data fields (inside `bgp.<type>` object, except state):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `message` | object | For wire messages | `{"id": <N>, "direction": "<dir>"}` |
| `attr` | object | For UPDATE | Path attributes (origin, as-path, etc.) |
| `nlri` | object | For UPDATE | `{"<family>": [operations...]}` |
| `raw` | object | If format=full | Wire bytes (see Raw Format below) |

**State events:** Use simple `"state": "up"` string at bgp level (no container). Down events include `"reason": "..."` field.

**RIB event payload (`rib` key):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Always | `cache`, `route` |
| `action` | string | Always | `new`, `evict`, `add`, `remove` |
| `msg-id` | uint64 | For cache events | Message cache ID |
| `peer` | object | Always | `{"address":"<ip>", "asn":<asn>}` |

**Message object fields (BGP wire messages only):**

| Field | Type | Description |
|-------|------|-------------|
| `id` | uint64 | Message cache ID (0 for locally-originated) |
| `direction` | string | `"received"` or `"sent"` |

**Raw object fields (format=full only):**

| Field | Type | Description |
|-------|------|-------------|
| `attributes` | string | Hex-encoded path attributes wire bytes |
| `nlri` | object | `{"<family>": "<hex>"}` - NLRI wire bytes per family |
| `withdrawn` | object | `{"<family>": "<hex>"}` - Withdrawn wire bytes per family |

Events are only sent to plugins that have subscribed.

---

## Serial Protocol

### Request-Response Correlation

Commands with `#N` prefix expect a response:

```
#1 bgp peer * update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
```

Response echoes serial:

```json
{"type":"response","response":{"serial":"1","status":"done"}}
```

### Fire-and-Forget

Commands without serial receive no response:

```
bgp peer * update text nhop set 1.2.3.4 nlri ipv4/unicast add 10.0.0.0/24
```

### Streaming Responses

For long-running commands, intermediate results use `partial: true`:

```
Plugin → Engine:
#1 rib show in

Engine → Plugin:
{"type":"response","response":{"serial":"1","status":"ack","partial":true,"data":{"chunk":1,"routes":[...]}}}
{"type":"response","response":{"serial":"1","status":"ack","partial":true,"data":{"chunk":2,"routes":[...]}}}
{"type":"response","response":{"serial":"1","status":"done","data":{"total":150}}}
```

Plugin responses use `@serial+` prefix:

```
Engine → Plugin:
{"type":"request","request":{"serial":"abc","command":"myapp status","args":[]}}

Plugin → Engine:
@abc+ {"chunk": 1, "data": [...]}
@abc+ {"chunk": 2, "data": [...]}
@abc done {"total": 100}
```

**Note:** Requests from engine to plugin follow the same wrapper pattern as responses and events. The top-level `type` indicates which key contains the payload.

---

## Status Codes

| Status | Meaning | Response Contains |
|--------|---------|-------------------|
| `done` | Success, command complete | Optional `data` with result |
| `error` | Failure | `data` contains error message (string) |
| `warning` | Partial success | `data` contains warning details |
| `ack` | Streaming chunk | `partial: true`, `data` contains chunk |

---

## Command Namespaces

Commands are organized by namespace. Each subsystem owns its introspection.

### Plugin Namespace

Plugin lifecycle operations:

| Command | Description |
|---------|-------------|
| `plugin help` | List plugin subcommands |
| `plugin command list` | List plugin commands |
| `plugin command help "<cmd>"` | Command details |
| `plugin command complete "<partial>"` | Completion |
| `plugin session ready` | Signal plugin init complete |
| `plugin session ping` | Health check (returns PID) |
| `plugin session bye` | Disconnect |

### BGP Namespace

**Introspection:**

| Command | Description |
|---------|-------------|
| `bgp help` | List bgp subcommands |
| `bgp command list` | List bgp commands |
| `bgp command help "<cmd>"` | Command details |
| `bgp command complete "<partial>"` | Completion |
| `bgp event list` | List available BGP event types |

**Plugin Configuration:**

| Command | Description |
|---------|-------------|
| `bgp plugin encoding json\|text` | Set event encoding format |
| `bgp plugin format hex\|base64\|parsed\|full` | Set wire bytes format (JSON only) |
| `bgp plugin ack sync\|async` | Set ACK timing |

Format relationship:
- `encoding text` → always parsed (human readable)
- `encoding json` → format applies (hex, base64, parsed, or full)

| Format | Wire Bytes | Parsed Fields | Use Case |
|--------|------------|---------------|----------|
| `hex` | ✅ as hex string | ❌ | Wire-level debugging |
| `base64` | ✅ as base64 | ❌ | Binary transport |
| `parsed` | ❌ | ✅ | Most plugins (default) |
| `full` | ✅ as hex | ✅ | Debugging + analysis |

**Timing:** Encoding applies at event delivery time. Subscribe first, configure encoding later—events use current encoding when delivered.

**Peer Operations:**

Selector patterns: `*` (all), `<ip>` (specific), `!<ip>` (all except)

| Command | Description |
|---------|-------------|
| `bgp peer <sel> list` | List matching peers (brief) |
| `bgp peer <sel> show` | Show matching peers (detailed) |
| `bgp peer <sel> teardown [subcode]` | Graceful close (NOTIFICATION) |
| `bgp peer <sel> update text\|hex\|base64 ...` | Announce/withdraw routes |
| `bgp peer <sel> borr <family>` | Begin-of-Route-Refresh (RFC 7313) |
| `bgp peer <sel> eorr <family>` | End-of-Route-Refresh (RFC 7313) |
| `bgp peer <sel> ready` | Signal peer replay complete |
| `bgp peer <sel> tcp reset` | Force TCP RST |
| `bgp peer <sel> tcp ttl <num>` | Set TTL (multi-hop) |

**Watchdog:**

| Command | Description |
|---------|-------------|
| `bgp watchdog announce <name>` | Send all routes in pool |
| `bgp watchdog withdraw <name>` | Withdraw all routes in pool |

**Commits (Batching):**

| Command | Description |
|---------|-------------|
| `bgp commit <name> start` | Begin batch |
| `bgp commit <name> end` | Flush batch |
| `bgp commit <name> eor` | Flush + send EOR |
| `bgp commit <name> rollback` | Discard batch |
| `bgp commit <name> show` | Show queued count |
| `bgp commit list` | List active batches |

**Raw Passthrough:**

| Command | Description |
|---------|-------------|
| `bgp raw <type> <enc> <data>` | Send raw BGP message |

### System Namespace

| Command | Description |
|---------|-------------|
| `system help` | List system subcommands |
| `system command list` | List system commands |
| `system command help "<cmd>"` | Command details |
| `system command complete "<partial>"` | Completion |
| `system subsystem list` | List available subsystems |
| `system version software` | Ze version |
| `system version api` | IPC protocol version |
| `daemon shutdown` | Gracefully shutdown the daemon |
| `daemon status` | Show daemon status |
| `daemon reload` | Reload the configuration |

### RIB Namespace

**Built-in:**

| Command | Description |
|---------|-------------|
| `rib help` | List rib subcommands |
| `rib command list` | List rib commands |
| `rib command help "<cmd>"` | Command details |
| `rib command complete "<partial>"` | Completion |
| `rib event list` | List available RIB event types |
| `rib show in [peer]` | Show Adj-RIB-In |
| `rib clear in [peer]` | Clear Adj-RIB-In |

**Future (BGP cache in bgp subsystem):**

| Command | Description |
|---------|-------------|
| `bgp cache <id> forward <sel>` | Forward cached UPDATE to peers |
| `bgp cache <id1>,<id2>,...,<idN> forward <sel>` | Batch forward (comma-separated IDs) |
| `bgp cache <id> retain` | Keep in cache until released |
| `bgp cache <id> release` | Allow eviction (TTL-based) |
| `bgp cache <id1>,<id2>,...,<idN> release` | Batch release |
| `bgp cache <id> expire` | Remove immediately |
| `bgp cache list` | List cached IDs |

---

## Event Subscription

Plugins subscribe to events via commands (not config).

### Subscription Commands

```
subscribe [peer <sel> | plugin <name>] <namespace> event <type> [direction received|sent|both]
unsubscribe [peer <sel> | plugin <name>] <namespace> event <type> [direction received|sent|both]
```

Selector patterns:
- `peer <sel>` - filter by peer: `*` (all), `<ip>` (specific), `!<ip>` (all except)
- `plugin <name>` - filter by plugin name

Direction (for message events):
- `received` - messages received from peer
- `sent` - messages sent to peer
- `both` - both directions (default if omitted)

Examples:

```
subscribe bgp event update                              # all peers, both directions
subscribe bgp event update direction received           # all peers, received only
subscribe peer 10.0.0.1 bgp event update               # specific peer, both directions
subscribe peer * bgp event state                        # explicit all peers
subscribe peer !10.0.0.1 bgp event update direction sent # all except one, sent only
subscribe plugin rib-cache rib event cache             # events from specific plugin
subscribe rib event route                               # RIB route events
```

### BGP Event Types

| Event | Has Direction | Description |
|-------|---------------|-------------|
| `update` | ✅ | UPDATE message |
| `open` | ✅ | OPEN message |
| `notification` | ✅ | NOTIFICATION message |
| `keepalive` | ✅ | KEEPALIVE message |
| `refresh` | ✅ | ROUTE-REFRESH message |
| `state` | ❌ | Peer state change (up/down) |
| `negotiated` | ❌ | Capability negotiation complete |

### RIB Event Types

| Event | Description |
|-------|-------------|
| `cache` | Msg-id cache event (new entry, eviction) |
| `route` | Route change (add/remove) |

RIB events include `peer` field indicating which peer caused the event.

### Event Examples

**BGP UPDATE received from peer:**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "update": {
      "message": {"id": 123, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001, 65002],
        "local-preference": 100
      },
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "192.0.2.1", "nlri": ["10.0.0.0/24"]}
        ]
      }
    }
  }
}
```

**BGP UPDATE sent to peer (locally-originated):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "update": {
      "message": {"id": 0, "direction": "sent"},
      "attr": {
        "origin": "igp",
        "as-path": [65000]
      },
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "192.0.2.254", "nlri": ["172.16.0.0/16"]}
        ]
      }
    }
  }
}
```

**Note:** `bgp.update.message.id: 0` indicates locally-originated route (no cache entry for forwarding).

**BGP UPDATE with raw wire bytes (format=full):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "update": {
      "message": {"id": 123, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001]
      },
      "nlri": {
        "ipv4/unicast": [
          {"action": "add", "next-hop": "192.0.2.1", "nlri": ["10.0.0.0/24"]}
        ]
      },
      "raw": {
        "attr": "40010100400200040001fde8",
        "nlri": {"ipv4/unicast": "180a0000"},
        "withdrawn": {}
      }
    }
  }
}
```

**Peer state change (up):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "state",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "state": "up"
  }
}
```

**Peer state change (down with reason):**
```json
{
  "type": "bgp",
  "bgp": {
    "type": "state",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "state": "down",
    "reason": "hold timer expired"
  }
}
```

**RIB cache event:**
```json
{
  "type": "rib",
  "rib": {
    "type": "cache",
    "action": "new",
    "msg-id": 12345,
    "peer": {"address": "192.0.2.1", "asn": 65001}
  }
}
```

---

## Plugin Startup Protocol

Five-stage synchronized startup with barriers:

```
Stage 1: REGISTRATION     Plugin → Engine: declare cmd/conf/receive, declare done
Stage 2: CONFIG           Engine → Plugin: config peer <addr> <key> <value>, config done
Stage 3: CAPABILITY       Plugin → Engine: capability hex <code> <value>, capability done
Stage 4: REGISTRY         Engine → Plugin: registry cmd <name>, registry done
Stage 5: READY            Plugin → Engine: plugin session ready
```

**Note:** During startup stages 1-4, plugins use minimal commands (`declare`, `capability`, `config`). After `plugin session ready`, the full command namespace is available.

**After ready:** Plugin configures itself and subscribes to events:

```
plugin session ready

bgp plugin encoding json
bgp plugin format full
bgp plugin ack async

subscribe bgp event update direction received
subscribe bgp event update direction sent
subscribe peer * bgp event state
subscribe rib event cache
subscribe rib event route
```

**Barrier semantics:** All plugins must complete each stage before any proceed to next.

**Timeout:** 5s per stage (configurable via `timeout` in plugin config).

**Subscription timing:** Subscriptions receive FUTURE events only. To get current state:
1. Subscribe to events first
2. Query `bgp peer * show` for existing peer states
3. Process both query results and incoming events

---

## Subsystem Discovery

Before subscribing to events from a subsystem, plugins should check availability:

```
system subsystem list
```

Response:
```json
{"type":"response","response":{"status":"done","data":{"subsystems":["bgp","rib"]}}}
```

Subscribing to unavailable subsystem returns error:
```json
{"type":"response","response":{"status":"error","data":"rib not available"}}
```

---

## Event Discovery

Each subsystem that supports events provides an `event list` command:

```
bgp event list
```

Response:
```json
{"type":"response","response":{"status":"done","data":{"events":["update","open","notification","keepalive","refresh","state","negotiated"]}}}
```

```
rib event list
```

Response:
```json
{"type":"response","response":{"status":"done","data":{"events":["cache","route"]}}}
```

---

## Error Codes

Errors are returned as strings in `data` field:

| Error Message | Cause |
|---------------|-------|
| `unknown command` | Unrecognized verb |
| `invalid peer address` | Malformed IP in selector |
| `no peers match selector` | No peers found for selector |
| `no peers have family negotiated` | No peers support requested AFI/SAFI |
| `invalid attribute` | Unrecognized attribute name |
| `missing required attribute` | Required field not provided |
| `parse error: <detail>` | Syntax error in command |
| `timeout` | Request timed out (default 30s) |
| `process not ready` | Plugin not yet at READY stage |
| `queue full` | Write queue backpressure triggered |
| `<subsystem> not available` | Subsystem not configured |

---

## Backpressure

| Constant | Value | Description |
|----------|-------|-------------|
| `WRITE_QUEUE_HIGH_WATER` | 1000 | Pause writes at this queue depth |
| `WRITE_QUEUE_LOW_WATER` | 100 | Resume when drained to this level |
| `PENDING_REQUEST_LIMIT` | 100 | Max pending requests per process |
| `DEFAULT_TIMEOUT` | 30s | Request timeout |
| `COMPLETION_TIMEOUT` | 500ms | Tab completion timeout |
| `RESPAWN_LIMIT` | 5 | Max respawns per 60s |

When backpressure triggers:
1. Events dropped for affected process
2. Warning logged
3. Counter incremented
4. Resumes when queue drains

---

## Text Format (Alternative Encoding)

When `bgp plugin encoding text` is set, events use human-readable text format (not JSON wrapper):

```
peer <ip> asn <asn> <direction> <type> <msg-id> <fields...>
```

**Note:** Text format intentionally stays flat for human readability. No JSON wrapping is applied.

Examples:

```
peer 192.0.2.1 asn 65001 state up
peer 192.0.2.1 asn 65001 received update 1 announce origin igp as-path 65001 ipv4/unicast next-hop 192.0.2.1 nlri 10.0.0.0/24
peer 192.0.2.1 asn 65001 sent keepalive 42
```

---

## Plugin Registration

Plugins can register custom commands:

| Command | Description |
|---------|-------------|
| `register command "<name>" description "<help>" [args "<usage>"] [completable] [timeout <dur>]` | Register command |
| `unregister command "<name>"` | Unregister command |

---

## References

- `architecture.md` - Full API architecture
- `process-protocol.md` - Plugin startup details
- `commands.md` - Command syntax details
- `json-format.md` - JSON output format details
- `capability-contract.md` - GR/RR capability handling
