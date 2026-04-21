# JSON Format Rationale

Why: `ai/rules/json-format.md`

## Full OPEN Message Example

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

## Full UPDATE Message Example

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

## Standard Capability Names

| Code | Name | Value |
|------|------|-------|
| 1 | `"multiprotocol"` | `"afi/safi"` |
| 2 | `"route-refresh"` | (none) |
| 6 | `"extended-message"` | (none) |
| 64 | `"graceful-restart"` | `restart-time` field |
| 65 | `"asn4"` | ASN string |
| 69 | `"add-path"` | Array of families |
| 73 | `"hostname"` | Decoded by plugin |

## formatNumber Helper

```go
func formatNumber(v any) string {
    if n, ok := v.(float64); ok {
        if n == float64(int64(n)) { return fmt.Sprintf("%d", int64(n)) }
        return fmt.Sprintf("%v", n)
    }
    return fmt.Sprintf("%v", v)
}
```
