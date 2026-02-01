# JSON Format Standards

**BLOCKING:** All JSON output MUST follow these conventions.

## Field Naming: kebab-case (MANDATORY)

All JSON keys use **lowercase kebab-case**. Never camelCase or snake_case.

| Correct | Incorrect |
|---------|-----------|
| `"router-id"` | `"routerId"`, `"router_id"` |
| `"local-as"` | `"localAs"`, `"local_as"` |
| `"remote-as"` | `"remoteAs"`, `"remote_as"` |
| `"hold-time"` | `"holdTime"`, `"hold_time"` |
| `"restart-time"` | `"restartTime"`, `"restart_time"` |
| `"address-family"` | `"addressFamily"`, `"address_family"` |
| `"next-hop"` | `"nextHop"`, `"next_hop"` |
| `"as-path"` | `"asPath"`, `"as_path"` |
| `"local-preference"` | `"localPreference"`, `"local_pref"` |

## Ze IPC Protocol 2.0 Envelope

All BGP JSON messages use this structure:

```json
{
  "type": "bgp",
  "bgp": {
    "peer": {
      "address": "127.0.0.1",
      "asn": 65533
    },
    "message": {
      "id": 0,
      "direction": "received",
      "type": "open"
    },
    "open": { ... }
  }
}
```

### Envelope Fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Always `"bgp"` for BGP messages |
| `bgp.peer.address` | string | Peer IP address |
| `bgp.peer.asn` | integer | Peer ASN |
| `bgp.message.id` | integer | Message sequence number |
| `bgp.message.direction` | string | `"received"` or `"sent"` |
| `bgp.message.type` | string | `"open"`, `"update"`, `"notification"`, `"keepalive"` |

## OPEN Message Format

```json
{
  "open": {
    "asn": 65533,
    "router-id": "10.0.0.2",
    "hold-time": 180,
    "capabilities": [
      {"code": 1, "name": "multiprotocol", "value": "ipv4/unicast"},
      {"code": 65, "name": "asn4", "value": "65533"},
      {"code": 64, "name": "graceful-restart", "restart-time": 120},
      {"code": 2, "name": "route-refresh"},
      {"code": 6, "name": "extended-message"}
    ]
  }
}
```

### Capability Format

| Field | Type | Description |
|-------|------|-------------|
| `code` | integer | Capability code (RFC 5492) |
| `name` | string | Human-readable name or `"unknown"` |
| `value` | string/array | Capability-specific value (optional) |
| `raw` | string | Hex bytes for unknown capabilities |

### Standard Capability Names

| Code | Name | Value Format |
|------|------|--------------|
| 1 | `"multiprotocol"` | `"afi/safi"` (e.g., `"ipv4/unicast"`) |
| 2 | `"route-refresh"` | (no value) |
| 6 | `"extended-message"` | (no value) |
| 64 | `"graceful-restart"` | `restart-time` field instead |
| 65 | `"asn4"` | ASN as string |
| 69 | `"add-path"` | Array of family strings |
| 73 | `"hostname"` | Decoded by hostname plugin |

## UPDATE Message Format

```json
{
  "update": {
    "attr": {
      "origin": "igp",
      "as-path": [65001, 65002],
      "local-preference": 100,
      "med": 50
    },
    "ipv4/unicast": [
      {"next-hop": "192.168.1.1", "action": "add", "nlri": ["10.0.0.0/24", "10.0.1.0/24"]},
      {"action": "del", "nlri": ["10.0.2.0/24"]}
    ]
  }
}
```

### Update Structure

| Field | Type | Description |
|-------|------|-------------|
| `attr` | object | Path attributes |
| `<family>` | array | NLRI operations (family as direct key) |

### Attribute Names

| Attribute | JSON Key | Type |
|-----------|----------|------|
| ORIGIN | `"origin"` | string: `"igp"`, `"egp"`, `"incomplete"` |
| AS_PATH | `"as-path"` | array of integers |
| NEXT_HOP | `"next-hop"` | string (IP address) |
| MED | `"med"` | integer |
| LOCAL_PREF | `"local-preference"` | integer |
| ATOMIC_AGGREGATE | `"atomic-aggregate"` | boolean |
| AGGREGATOR | `"aggregator"` | string `"asn:ip"` |
| ORIGINATOR_ID | `"originator-id"` | string (IP address) |
| CLUSTER_LIST | `"cluster-list"` | array of strings |
| COMMUNITIES | `"community"` | array of strings |
| EXTENDED_COMMUNITIES | `"extended-community"` | array of objects |

### NLRI Operation Format

```json
{
  "next-hop": "192.168.1.1",
  "action": "add",
  "nlri": ["10.0.0.0/24"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `action` | string | `"add"` (announce) or `"del"` (withdraw) |
| `next-hop` | string | Only for `"add"` operations |
| `nlri` | array | Prefix strings or complex NLRI objects |

## Address Family Format

Family keys use `"afi/safi"` format:

| Family | Key |
|--------|-----|
| IPv4 Unicast | `"ipv4/unicast"` |
| IPv6 Unicast | `"ipv6/unicast"` |
| IPv4 Multicast | `"ipv4/multicast"` |
| IPv6 Multicast | `"ipv6/multicast"` |
| IPv4 VPN | `"ipv4/vpn"` |
| IPv6 VPN | `"ipv6/vpn"` |
| IPv4 FlowSpec | `"ipv4/flow"` |
| IPv6 FlowSpec | `"ipv6/flow"` |
| L2VPN EVPN | `"l2vpn/evpn"` |
| BGP-LS | `"bgp-ls/bgp-ls"` |

## Configuration JSON Format

```json
{
  "router-id": "1.2.3.4",
  "local-as": 65000,
  "peer": {
    "peer1": {
      "address": "192.0.2.1",
      "remote-as": 65001
    }
  }
}
```

## Error Response Format

```json
{
  "error": "description of what went wrong",
  "parsed": false
}
```

## CLI Response Format

```json
{
  "status": "ok",
  "error": "error message if status is error",
  "data": { ... }
}
```

## Numbers in JSON

- JSON numbers decode as `float64` in Go
- Use `formatNumber()` helper to display integers without decimal
- Large integers (ASN, etc.) may need string representation

```go
// Format numeric values, handling float64 from JSON unmarshaling
func formatNumber(v any) string {
    if n, ok := v.(float64); ok {
        if n == float64(int64(n)) {
            return fmt.Sprintf("%d", int64(n))
        }
        return fmt.Sprintf("%v", n)
    }
    return fmt.Sprintf("%v", v)
}
```

## Raw/Unparsed Data

For data that cannot be decoded, include:

```json
{
  "parsed": false,
  "raw": "DEADBEEF"
}
```

- `raw` is uppercase hex without `0x` prefix
- `parsed: false` indicates decode failure

## JSON Validation Checklist

Before outputting JSON:

```
[ ] All keys are kebab-case (not camelCase or snake_case)
[ ] Envelope structure matches Ze IPC Protocol 2.0
[ ] Family keys use "afi/safi" format
[ ] Numbers are proper JSON numbers (not strings unless necessary)
[ ] Raw hex is uppercase without 0x prefix
[ ] Error responses include "error" and "parsed": false
```
