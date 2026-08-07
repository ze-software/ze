---
kind: table
level:
stage:
---
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
