# JSON Format Standards

**BLOCKING:** All JSON output MUST follow these conventions.
Rationale: `.claude/rationale/json-format.md`

## Field Naming: kebab-case (MANDATORY)

All JSON keys: lowercase kebab-case. Never camelCase or snake_case.
Examples: `"router-id"`, `"local-as"`, `"hold-time"`, `"as-path"`, `"next-hop"`, `"local-preference"`

## Ze IPC Envelope

```json
{"type":"bgp","bgp":{"peer":{"address":"...","asn":N},"message":{"id":N,"direction":"received","type":"open"},"open":{...}}}
```

## Attribute Names

| Attribute | Key | Type |
|-----------|-----|------|
| ORIGIN | `"origin"` | `"igp"`, `"egp"`, `"incomplete"` |
| AS_PATH | `"as-path"` | array of integers |
| NEXT_HOP | `"next-hop"` | IP string |
| MED | `"med"` | integer |
| LOCAL_PREF | `"local-preference"` | integer |
| ATOMIC_AGGREGATE | `"atomic-aggregate"` | boolean |
| AGGREGATOR | `"aggregator"` | `"asn:ip"` |
| ORIGINATOR_ID | `"originator-id"` | IP string |
| CLUSTER_LIST | `"cluster-list"` | array of strings |
| COMMUNITIES | `"community"` | array of strings |
| EXT_COMMUNITIES | `"extended-community"` | array of objects |

## Address Families

Format: `"afi/safi"` — `"ipv4/unicast"`, `"ipv6/unicast"`, `"ipv4/vpn"`, `"ipv6/vpn"`, `"ipv4/flow"`, `"ipv6/flow"`, `"l2vpn/evpn"`, `"bgp-ls/bgp-ls"`

## NLRI Operations

```json
{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}
{"action":"del","nlri":["10.0.2.0/24"]}
```

## Error/CLI Responses

- Error: `{"error":"description","parsed":false}`
- CLI: `{"status":"ok","data":{...}}` or `{"status":"error","error":"msg"}`
- Raw hex: uppercase, no `0x` prefix. `"parsed":false` + `"raw":"DEADBEEF"`

## Numbers

JSON numbers decode as `float64` in Go. Use `formatNumber()` to display integers without decimal.
