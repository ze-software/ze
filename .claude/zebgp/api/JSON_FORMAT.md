# JSON Output Format

**Source:** ExaBGP `reactor/api/response/json.py`
**Purpose:** Document JSON output format for API compatibility

---

## Overview

ExaBGP outputs JSON messages to external processes via stdout. Two API versions exist:
- **API v6** (current): JSON only, nexthop not included in NLRI
- **API v4** (legacy): JSON or text, nexthop included in Flow NLRI

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

### Up

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

### Down

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

---

## UPDATE Messages

### Structure

```json
"message": {
  "update": {
    "update-id": 12345,
    "attribute": { ... },
    "announce": {
      "ipv4 unicast": {
        "192.168.1.2": [
          { "nlri": "10.0.0.0/8" },
          { "nlri": "10.1.0.0/16" }
        ]
      }
    },
    "withdraw": {
      "ipv4 unicast": [
        { "nlri": "10.2.0.0/16" }
      ]
    }
  }
}
```

### Update ID (ZeBGP Extension)

The `update-id` field is a unique identifier for each received UPDATE message.
Used for route reflection via the `forward update-id` command.

- Assigned per-UPDATE (not per-NLRI)
- Included when API content config enables it
- Expires after configurable TTL (default 60s)
- See `plan/spec-route-id-forwarding.md` for implementation details

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

**API v4 only** includes `"next-hop"` in FlowSpec NLRI.

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
  "families": "[ ipv4 unicast, ipv6 unicast ]",
  "nexthop": "[ ]",
  "add_path": {
    "send": "[ \"ipv4 unicast\" ]",
    "receive": "[ \"ipv4 unicast\" ]"
  }
}
```

---

## API v4 vs v6 Differences

| Aspect | v4 | v6 |
|--------|----|----|
| Version string | "4.x.x" | "6.x.x" |
| Encoder | json or text | json only |
| Flow nexthop | Included in NLRI | Not included |
| Method | `nlri.v4_json()` | `nlri.json()` |

### Implementation

```python
# v6 encoder
class JSON:
    use_v4_json = False

    def _nlri_to_json(self, nlri, nexthop=None):
        if self.use_v4_json:
            return nlri.v4_json(compact=self.compact, nexthop=nexthop)
        return nlri.json(compact=self.compact)

# v4 encoder (wrapper)
class V4JSON:
    def __init__(self, version):
        self._v6 = JSON(json_version)
        self._v6.use_v4_json = True  # Enable v4 compat

    def _patch_version(self, result):
        # Replace v6 version with v4 version
        return result.replace('"exabgp": "6.x"', f'"exabgp": "{self.version}"')
```

---

## ZeBGP Implementation Notes

### JSON Encoder

```go
type JSONEncoder struct {
    version   string
    compact   bool
    useV4JSON bool
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
    V4JSON(compact bool, nexthop IP) string  // For v4 compat
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

**Last Updated:** 2025-12-19
