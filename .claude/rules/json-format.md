# JSON Format

**BLOCKING:** All JSON output MUST follow these conventions.
Rationale: `.claude/rationale/json-format.md`

## Field Naming: kebab-case (MANDATORY)

All JSON keys: lowercase kebab-case. Never camelCase or snake_case.

## Ze IPC Envelope

```json
{"type":"bgp","bgp":{"peer":{"address":"...","asn":N},"message":{"id":N,"direction":"received","type":"open"},"open":{...}}}
```

## Attribute Names

| Attribute | Key | Type |
|-----------|-----|------|
| ORIGIN | `"origin"` | `"igp"` / `"egp"` / `"incomplete"` |
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

Format: `"afi/safi"`. Families are registered dynamically by plugins (not a static list). Current inventory:

| AFI | Families |
|-----|----------|
| ipv4 | `unicast`, `multicast`, `vpn`, `flow`, `flow-vpn`, `mpls-label`, `mup`, `mvpn`, `rtc` |
| ipv6 | `unicast`, `multicast`, `vpn`, `flow`, `flow-vpn`, `mpls-label`, `mup`, `mvpn` |
| l2vpn | `evpn`, `vpls` |
| bgp-ls | `bgp-ls`, `bgp-ls-vpn` |

Unicast and multicast are builtin (engine). All others registered by `bgp-nlri-*` plugins. Use `make ze-inventory` for the authoritative list.

## NLRI Operations

```json
{"next-hop":"192.168.1.1","action":"add","nlri":["10.0.0.0/24"]}
{"action":"del","nlri":["10.0.2.0/24"]}
```

## Conventions

- Error: `{"error":"description","parsed":false}`
- CLI: `{"status":"ok","data":{...}}` or `{"status":"error","error":"msg"}`
- Raw hex: uppercase, no `0x`. `"parsed":false` + `"raw":"DEADBEEF"`
- Numbers: JSON `float64` in Go → use `formatNumber()` for integer display
