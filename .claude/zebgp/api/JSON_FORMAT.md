# JSON Output Format

**Source:** ExaBGP `reactor/api/response/json.py`
**Purpose:** Document JSON output format for API compatibility

---

## Overview

ExaBGP outputs JSON messages to external processes via stdout. ZeBGP uses JSON encoding with nexthop not included in Flow NLRI.

---

## Message Structure

### Top-Level Format

```json
{
  "exabgp": "6.0.0",
  "time": 1234567890.123456,
  "host": "hostname",
  "pid": 12345,
  "ppid": 12344,
  "counter": 1,
  "type": "update",
  "header": "FFFFFFFF...",
  "body": "0000...",
  "neighbor": { ... }
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| exabgp | string | API version (e.g., "6.0.0", "4.2.22") |
| time | float | Unix timestamp |
| host | string | Hostname from `socket.gethostname()` |
| pid | int | Process ID |
| ppid | int | Parent process ID |
| counter | int | Per-neighbor message counter |
| type | string | Message type |
| header | string | Hex-encoded BGP header (optional) |
| body | string | Hex-encoded BGP body (optional) |
| neighbor | object | Neighbor and message details |

### Message Types

| Type | Trigger |
|------|---------|
| state | up, connected, down |
| update | UPDATE message |
| open | OPEN message |
| keepalive | KEEPALIVE message |
| notification | NOTIFICATION message |
| refresh | ROUTE-REFRESH message |
| operational | Operational message |
| negotiated | Capabilities negotiated |
| fsm | FSM state change |
| signal | Signal received |

---

## Neighbor Section

```json
"neighbor": {
  "address": {
    "local": "192.168.1.1",
    "peer": "192.168.1.2"
  },
  "asn": {
    "local": 65001,
    "peer": 65002
  },
  "direction": "receive",
  "message": { ... }
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| address.local | string | Local IP address |
| address.peer | string | Peer IP address |
| asn.local | int | Local AS number |
| asn.peer | int | Peer AS number |
| direction | string | "receive" or "send" (optional) |
| state | string | For state messages: "up", "connected", "down" |
| message | object | Message-specific content |

---

## State Messages

### ExaBGP Format

**Up:**
```json
{
  "exabgp": "6.0.0",
  "time": 1234567890.123,
  "type": "state",
  "neighbor": {
    "address": { "local": "192.168.1.1", "peer": "192.168.1.2" },
    "asn": { "local": 65001, "peer": 65002 },
    "state": "up"
  }
}
```

**Down:**
```json
{
  "exabgp": "6.0.0",
  "time": 1234567890.123,
  "type": "state",
  "neighbor": {
    "address": { "local": "192.168.1.1", "peer": "192.168.1.2" },
    "asn": { "local": 65001, "peer": 65002 },
    "state": "down",
    "reason": "peer closed connection"
  }
}
```

### ZeBGP Format

ZeBGP uses a simpler flat structure:

**JSON:**
```json
{"type":"state","peer":{"address":"192.0.2.1","asn":65001},"state":"up"}
{"type":"state","peer":{"address":"192.0.2.1","asn":65001},"state":"down"}
```

**Text:**
```
peer 192.0.2.1 asn 65001 state up
peer 192.0.2.1 asn 65001 state down
```

Key differences from ExaBGP:
- No envelope (`exabgp`, `time`, `host`, `pid`)
- Flat `peer` object (not nested `neighbor.address/asn`)
- No `reason` field (close reason not included in message)

State messages are emitted by the `apiStateObserver` when peers transition to/from Established state. See `.claude/zebgp/api/ARCHITECTURE.md` for implementation details.

---

## UPDATE Messages

### Structure

```json
"message": {
  "update": {
    "direction": "received",
    "msg-id": 12345,
    "attribute": { ... },
    "announce": {
      "ipv4/unicast": {
        "192.168.1.2": [
          { "nlri": "10.0.0.0/8" },
          { "nlri": "10.1.0.0/16" }
        ]
      }
    },
    "withdraw": {
      "ipv4/unicast": [
        { "nlri": "10.2.0.0/16" }
      ]
    }
  }
}
```

### Message ID and Direction (ZeBGP Extension)

The `msg-id` field is a unique identifier assigned to every BGP message (OPEN, UPDATE, KEEPALIVE, NOTIFICATION).
Used for route reflection via the `forward update-id` command (for UPDATE messages).

- Assigned per-message (all types, not just UPDATE)
- Included when non-zero
- UPDATE messages cached for forwarding (expires after configurable TTL, default 60s)
- See `plan/done/spec-message-direction.md` for implementation details

The `direction` field indicates whether the message was `"sent"` or `"received"`.
Included for all message types.

### Announce Section

Nested by family, then by next-hop:

```json
"announce": {
  "<afi> <safi>": {
    "<next-hop>": [
      { nlri1 },
      { nlri2 }
    ]
  }
}
```

### Withdraw Section

Nested by family only (no next-hop):

```json
"withdraw": {
  "<afi> <safi>": [
    { nlri1 },
    { nlri2 }
  ]
}
```

### Attribute Section

```json
"attribute": {
  "origin": "igp",
  "as-path": [ 65001, 65002 ],
  "local-preference": 100,
  "med": 0,
  "community": [ [ 65001, 100 ], [ 65001, 200 ] ],
  "extended-community": [ ... ],
  "large-community": [ ... ]
}
```

---

## NLRI JSON Formats

### INET (IPv4/IPv6 Unicast)

```json
{ "nlri": "10.0.0.0/8" }
```

With ADD-PATH:

```json
{ "nlri": "10.0.0.0/8", "path-information": "0.0.0.1" }
```

### Label (MPLS)

```json
{ "nlri": "10.0.0.0/8", "label": [ [100, 1601] ] }
```

Label format: `[label_value, raw_24bit_value]`

### IPVPN (VPNv4/VPNv6)

```json
{
  "nlri": "10.0.0.0/8",
  "rd": "65000:100",
  "label": [ [1000, 16001] ]
}
```

### EVPN Type 2 (MAC/IP)

```json
{
  "code": 2,
  "parsed": true,
  "raw": "02...",
  "name": "MAC/IP advertisement",
  "rd": "65000:100",
  "esi": "00:00:00:00:00:00:00:00:00:00",
  "etag": 0,
  "mac": "aa:bb:cc:dd:ee:ff",
  "label": [ [100, 1601] ],
  "ip": "10.0.0.1"
}
```

### FlowSpec

```json
{
  "destination-ipv4": [ "10.0.0.0/8" ],
  "destination-port": [ "=80", "=443" ],
  "protocol": [ "=6" ],
  "string": "flow destination-ipv4 10.0.0.0/8 ..."
}
```

Note: ExaBGP API v4 includes `"next-hop"` in FlowSpec NLRI; ZeBGP does not.

### BGP-LS

```json
{
  "code": 1,
  "parsed": false,
  "raw": "0001..."
}
```

---

## OPEN Messages

```json
"message": {
  "open": {
    "version": 4,
    "asn": 65001,
    "hold_time": 180,
    "router_id": "1.1.1.1",
    "capabilities": {
      "1": "{ \"afi\": 1, \"safi\": 1 }",
      "65": "{ \"asn\": 65001 }"
    }
  }
}
```

---

## NOTIFICATION Messages

```json
"message": {
  "notification": {
    "code": 6,
    "subcode": 2,
    "data": "0000",
    "message": "cease"
  }
}
```

---

## Negotiated Capabilities

Sent after OPEN exchange:

```json
"negotiated": {
  "message_size": 4096,
  "hold_time": 90,
  "asn4": true,
  "multisession": false,
  "operational": false,
  "refresh": "normal",
  "families": "[ ipv4/unicast, ipv6/unicast ]",
  "nexthop": "[ ]",
  "add_path": {
    "send": "[ \"ipv4/unicast\" ]",
    "receive": "[ \"ipv4/unicast\" ]"
  }
}
```

---

## ExaBGP Differences

ZeBGP has its own output format optimized for parseability.

| Aspect | ExaBGP | ZeBGP |
|--------|--------|-------|
| API versions | v4 and v6 configurable | Single format |
| Encoder | json or text | json or text |
| Flow nexthop | Included in v4, not in v6 | Not included |
| Version field | Configurable (4.x or 6.x) | Not present |

### Text Format Comparison

**ExaBGP:**
```
neighbor 192.0.2.1 update start
neighbor 192.0.2.1 update announced 10.0.0.0/8 next-hop 192.0.2.1 origin igp as-path [ 65001 ]
neighbor 192.0.2.1 update end
```

**ZeBGP:**
```
peer 192.0.2.1 received update 1 announce origin igp as-path 65001 ipv4/unicast next-hop 192.0.2.1 nlri 10.0.0.0/8
```

Key text differences:
- `neighbor` → `peer`
- Includes direction (`sent`/`received`) and `msg-id` for routing decisions
- Attributes before NLRI (easier to parse)
- Family explicitly stated (`ipv4/unicast`)
- Single line per UPDATE (no start/end)

### Text Format: All Message Types

All messages follow the pattern: `peer <ip> <direction> <type> <msg-id> ...`

**OPEN:**
```
peer 10.0.0.1 received open 1 asn 65001 router-id 1.1.1.1 hold-time 90 cap 1 multiprotocol ipv4/unicast cap 65 asn4 65001
```

**KEEPALIVE:**
```
peer 10.0.0.1 sent keepalive 42
```

**NOTIFICATION:**
```
peer 10.0.0.1 sent notification 3 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data
```

**UPDATE:**
```
peer 10.0.0.1 received update 5 announce origin igp as-path 65001 ipv4/unicast next-hop 10.0.0.1 nlri 192.168.0.0/24
```

Note: NOTIFICATION names are hyphenated for single-word parsing (e.g., "Administrative-Shutdown").

### JSON Format Comparison

**ExaBGP:**
```json
{
  "exabgp": "6.0.0",
  "time": 1234567890.123,
  "host": "router1",
  "pid": 1234,
  "type": "update",
  "neighbor": {
    "address": {"local": "192.0.2.2", "peer": "192.0.2.1"},
    "asn": {"local": 65000, "peer": 65001},
    "message": {
      "update": {
        "attribute": {"origin": "igp", "as-path": [65001]},
        "announce": {"ipv4/unicast": {"192.0.2.1": [{"nlri": "10.0.0.0/8"}]}}
      }
    }
  }
}
```

**ZeBGP:**
```json
{"type":"update","direction":"received","msg-id":1,"peer":{"address":"192.0.2.1","asn":65001},"announce":{"origin":"igp","as-path":[65001],"ipv4/unicast":{"192.0.2.1":["10.0.0.0/8"]}}}
```

Key JSON differences:
- No envelope (`exabgp`, `time`, `host`, `pid`) - external process can add if needed
- Flat structure (no `neighbor.message.update` nesting)
- Includes `direction` and `msg-id` for route decisions and reflection
- Prefixes as strings, not objects (`"10.0.0.0/8"` not `{"nlri": "10.0.0.0/8"}`)

---

## ZeBGP Implementation Notes

### JSON Encoder

```go
type JSONEncoder struct {
    version string
    compact bool
}

func (e *JSONEncoder) Update(neighbor *Neighbor, direction string,
    update *UpdateCollection, header, body []byte, neg *Negotiated) string {
    // Build JSON structure
}
```

### Per-Type JSON Methods

Each NLRI type implements JSON serialization:

```go
type NLRI interface {
    JSON(compact bool) string
}
```

### Timestamp

Use `time.Now().UnixNano() / 1e9` for float timestamp with microseconds.

### Counter

Maintain per-neighbor message counter:

```go
type JSONEncoder struct {
    counters map[string]int  // neighbor_uid -> count
}
```

---

**Last Updated:** 2026-01-03
