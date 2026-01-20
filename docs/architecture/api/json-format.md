# ZeBGP JSON Output Format

**Purpose:** Document ZeBGP's JSON output format for plugin communication.

---

## Overview

ZeBGP outputs JSON messages to external processes via stdout. The format is optimized for parseability with a flat structure and consistent patterns across all message types.

---

## Message Structure

All messages share a common structure:

```json
{
  "message": {"type": "<type>", "id": <n>, "direction": "<dir>"},
  "peer": {"address": "<ip>", "asn": <n>},
  ...type-specific fields...
}
```

### Common Fields

| Field | Type | Description |
|-------|------|-------------|
| message.type | string | Message type (update, state, open, etc.) |
| message.id | int | Message identifier (when non-zero) |
| message.direction | string | "received" or "sent" |
| peer.address | string | Peer IP address |
| peer.asn | int | Peer AS number |

### Message Types

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

## State Messages

```json
{"message":{"type":"state"},"peer":{"address":"192.0.2.1","asn":65001},"state":"up"}
{"message":{"type":"state"},"peer":{"address":"192.0.2.1","asn":65001},"state":"down"}
```

---

## UPDATE Messages

### Structure

```json
{
  "message": {"type": "update", "id": 1, "direction": "received"},
  "peer": {"address": "192.0.2.1", "asn": 65001},
  "origin": "igp",
  "as-path": [65001, 65002],
  "ipv4/unicast": [
    {"next-hop": "192.0.2.1", "action": "add", "nlri": ["10.0.0.0/24", "10.0.1.0/24"]},
    {"action": "del", "nlri": ["172.16.0.0/16"]}
  ]
}
```

### Operation Format

Each address family has an array of operations at the top level:

```json
"<family>": [
  {"next-hop": "<ip>", "action": "add", "nlri": [...]},
  {"action": "del", "nlri": [...]}
]
```

- `next-hop`: Present only for "add" operations
- `action`: "add" (announce) or "del" (withdraw)
- `nlri`: Array of NLRI values

### Attributes

Attributes appear at the top level (not nested):

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

## OPEN Messages

```json
{
  "message": {"type": "open", "id": 1, "direction": "received"},
  "peer": {"address": "192.0.2.1", "asn": 65001},
  "asn": 65001,
  "router-id": "1.1.1.1",
  "hold-time": 90,
  "capabilities": [
    {"code": 1, "name": "multiprotocol", "value": "ipv4/unicast"},
    {"code": 65, "name": "asn4", "value": "65001"}
  ]
}
```

---

## NOTIFICATION Messages

```json
{
  "message": {"type": "notification", "id": 3, "direction": "sent"},
  "peer": {"address": "192.0.2.1", "asn": 65001},
  "code": 6,
  "subcode": 2,
  "code-name": "Cease",
  "subcode-name": "Administrative-Shutdown",
  "data": ""
}
```

---

## KEEPALIVE Messages

```json
{
  "message": {"type": "keepalive", "id": 42, "direction": "sent"},
  "peer": {"address": "192.0.2.1", "asn": 65001}
}
```

---

## Negotiated Capabilities

Sent after OPEN exchange:

```json
{
  "message": {"type": "negotiated"},
  "peer": {"address": "192.0.2.1", "asn": 65001},
  "hold-time": 90,
  "asn4": true,
  "families": ["ipv4/unicast", "ipv6/unicast"],
  "add-path": {
    "send": ["ipv4/unicast"],
    "receive": ["ipv4/unicast"]
  }
}
```

---

## Text Format

All messages follow: `peer <ip> <direction> <type> <msg-id> ...`

**State:**
```
peer 192.0.2.1 asn 65001 state up
```

**UPDATE:**
```
peer 192.0.2.1 received update 1 announce origin igp as-path 65001 ipv4/unicast next-hop 192.0.2.1 nlri 10.0.0.0/24
```

**OPEN:**
```
peer 192.0.2.1 received open 1 asn 65001 router-id 1.1.1.1 hold-time 90 cap 1 multiprotocol ipv4/unicast
```

**KEEPALIVE:**
```
peer 192.0.2.1 sent keepalive 42
```

**NOTIFICATION:**
```
peer 192.0.2.1 sent notification 3 code 6 subcode 2 code-name Cease subcode-name Administrative-Shutdown data
```

---

**Last Updated:** 2026-01-20
