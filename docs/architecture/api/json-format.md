# Ze JSON Output Format

**Purpose:** Document Ze's JSON output format for plugin communication.

**Version:** IPC Protocol 2.0

---

## Overview

Ze outputs JSON messages to external processes via stdout. All messages follow IPC Protocol 2.0 format with a top-level `type` field indicating which key contains the payload.

---

## Message Structure

All messages have a top-level `type` field. Event data is nested under both the namespace and event type:

```json
{
  "type": "<namespace>",
  "<namespace>": {
    "type": "<event-type>",
    "<event-type>": {
      ...event-specific fields...
    }
  }
}
```

This double-nesting allows routing by namespace (bgp, rib, response) and then by event type (update, state, etc.).

### Namespaces

| Namespace | Description |
|-----------|-------------|
| `bgp` | BGP protocol events (UPDATE, OPEN, etc.) |
| `rib` | RIB events (cache, route changes) |
| `response` | API command responses |

---

## BGP Events

All BGP events have `peer` at the `bgp` level. Event-specific data is nested under the event type key:

```json
{
  "type": "bgp",
  "bgp": {
    "type": "<event-type>",
    "peer": {"address": "<ip>", "asn": <n>},
    "<event-type>": {
      "message": {"id": <n>, "direction": "<dir>"},
      ...type-specific fields...
    }
  }
}
```

**Exception:** State events use a simple string value instead of a container (see State Events below).

### Common Fields

| Field | Type | Description |
|-------|------|-------------|
| bgp.type | string | Event type (update, state, open, etc.) |
| bgp.peer.address | string | Peer IP address |
| bgp.peer.asn | int | Peer AS number |
| bgp.\<type\>.message.id | int | Message identifier (0 for locally-originated) |
| bgp.\<type\>.message.direction | string | "received" or "sent" |

### Event Types

| Type | Description |
|------|-------------|
| state | Peer state change (up, down) |
| update | UPDATE message |
| open | OPEN message |
| keepalive | KEEPALIVE message |
| notification | NOTIFICATION message |
| refresh | ROUTE-REFRESH message |
| negotiated | Capabilities negotiated |

---

## State Events

State events use a simple string value for `state` (not a container):

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

State values: `"up"`, `"down"`, `"connected"`

For `"down"` events, a `reason` field is included:

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

---

## UPDATE Events

### Structure

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "update": {
      "message": {"id": 1, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001, 65002]
      },
      "nlri": {
        "ipv4/unicast": [
          {"next-hop": "192.0.2.1", "action": "add", "nlri": ["10.0.0.0/24", "10.0.1.0/24"]},
          {"action": "del", "nlri": ["172.16.0.0/16"]}
        ]
      }
    }
  }
}
```

### With Raw Wire Bytes (format=full)

```json
{
  "type": "bgp",
  "bgp": {
    "type": "update",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "update": {
      "message": {"id": 1, "direction": "received"},
      "attr": {
        "origin": "igp",
        "as-path": [65001]
      },
      "nlri": {
        "ipv4/unicast": [
          {"next-hop": "192.0.2.1", "action": "add", "nlri": ["10.0.0.0/24"]}
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

### Operation Format

Each address family has an array of operations under `nlri`:

```json
"nlri": {
  "<family>": [
    {"next-hop": "<ip>", "action": "add", "nlri": [...]},
    {"action": "del", "nlri": [...]}
  ]
}
```

- `next-hop`: Present only for "add" operations
- `action`: "add" (announce) or "del" (withdraw)
- `nlri`: Array of NLRI values

### Attributes

Attributes appear under the `attr` object:

| Attribute | Format |
|-----------|--------|
| origin | `"origin": "igp"` |
| as-path | `"as-path": [65001, 65002]` |
| med | `"med": 100` |
| local-preference | `"local-preference": 100` |
| communities | `"communities": ["65001:100", "65001:200"]` |
| large-communities | `"large-communities": ["65001:0:100"]` |
| extended-communities | `"extended-communities": ["0002..."]` (hex) |

---

## NLRI Formats

### Simple Prefixes (IPv4/IPv6 Unicast)

Without ADD-PATH:
```json
"nlri": ["10.0.0.0/24", "10.0.1.0/24"]
```

With ADD-PATH:
```json
"nlri": [{"prefix": "10.0.0.0/24", "path-id": 1}]
```

### Labeled Unicast (MPLS)

```json
"nlri": [{"prefix": "10.0.0.0/24", "labels": [100, 200]}]
```

### IPVPN (VPNv4/VPNv6)

```json
"nlri": [{"prefix": "10.0.0.0/24", "rd": "0:65000:1", "labels": [100]}]
```

### EVPN Type 2 (MAC/IP)

```json
"nlri": [{
  "route-type": "mac-ip-advertisement",
  "rd": "0:65000:1",
  "esi": "00:11:22:33:44:55:66:77:88:99",
  "ethernet-tag": 100,
  "mac": "aa:bb:cc:dd:ee:ff",
  "ip": "10.0.0.1",
  "labels": [200]
}]
```

### FlowSpec

```json
"nlri": {
  "ipv4/flowspec": [
    {
      "next-hop": "192.0.2.1",
      "action": "add",
      "nlri": [{
        "destination-ipv4": ["10.0.0.0/8"],
        "destination-port": ["=80", "=443"],
        "protocol": ["=6"],
        "string": "flow destination-ipv4 10.0.0.0/8 ..."
      }]
    }
  ]
}
```

Next-hop is at the **operation level** (same as all other families), not inside the NLRI object.

### FlowSpec-VPN

```json
"nlri": [{"rd": "0:65000:1", "spec": "destination 10.0.0.0/24 protocol tcp"}]
```

### BGP-LS

```json
"nlri": [{"code": 1, "parsed": false, "raw": "0001..."}]
```

---

## OPEN Events

```json
{
  "type": "bgp",
  "bgp": {
    "type": "open",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "open": {
      "message": {"id": 1, "direction": "received"},
      "asn": 65001,
      "router-id": "1.1.1.1",
      "hold-time": 90,
      "capabilities": [
        {"code": 1, "name": "multiprotocol", "value": "ipv4/unicast"},
        {"code": 65, "name": "asn4", "value": "65001"}
      ]
    }
  }
}
```

---

## NOTIFICATION Events

```json
{
  "type": "bgp",
  "bgp": {
    "type": "notification",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "notification": {
      "message": {"id": 3, "direction": "sent"},
      "code": 6,
      "subcode": 2,
      "code-name": "Cease",
      "subcode-name": "Administrative-Shutdown",
      "data": ""
    }
  }
}
```

---

## KEEPALIVE Events

```json
{
  "type": "bgp",
  "bgp": {
    "type": "keepalive",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "keepalive": {
      "message": {"id": 42, "direction": "sent"}
    }
  }
}
```

---

## Negotiated Capabilities

Sent after OPEN exchange:

```json
{
  "type": "bgp",
  "bgp": {
    "type": "negotiated",
    "peer": {"address": "192.0.2.1", "asn": 65001},
    "negotiated": {
      "hold-time": 90,
      "asn4": true,
      "families": ["ipv4/unicast", "ipv6/unicast"],
      "add-path": {
        "send": ["ipv4/unicast"],
        "receive": ["ipv4/unicast"]
      }
    }
  }
}
```

---

## RIB Events

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

## Response Format

API command responses:

```json
{
  "type": "response",
  "response": {
    "serial": "1",
    "status": "done",
    "data": {...}
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| response.serial | string | If request had serial | Correlation ID |
| response.status | string | Always | "done", "error", "warning", or "ack" |
| response.partial | bool | If streaming | true for intermediate chunks |
| response.data | any | Optional | Payload or error message |

---

## Text Format

Text format does NOT use JSON wrapping. All messages follow: `peer <ip> <direction> <type> <msg-id> ...`

**State:**
```
peer 192.0.2.1 asn 65001 state up
```

**UPDATE:**
```
peer 192.0.2.1 asn 65001 received update 1 announce origin igp as-path 65001 ipv4/unicast next-hop 192.0.2.1 nlri 10.0.0.0/24
```

**OPEN:**
```
peer 192.0.2.1 asn 65001 received open 1 asn 65001 router-id 1.1.1.1 hold-time 90 cap 1 multiprotocol ipv4/unicast
```

**KEEPALIVE:**
```
peer 192.0.2.1 asn 65001 sent keepalive 42
```

**NOTIFICATION:**
```
peer 192.0.2.1 asn 65001 sent notification 3 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data
```

---

**Last Updated:** 2026-01-31
