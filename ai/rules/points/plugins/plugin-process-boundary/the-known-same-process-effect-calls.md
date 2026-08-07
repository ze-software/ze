---
kind: table
level:
stage:
---
| Plugin | Same-process-effect call | Severity | Fix |
|--------|---------------------------|----------|-----|
| `as112` | `iface.RegisterOwnedAddresses`/`UnregisterOwnedAddresses` | total feature loss | refuse to start external |
| `cos` | `iface.GetBackend` | partial (dynamic QoS updates only) | warn |
| `traffic-usage` | `iface.SubscribeCollectNotify` | total feature loss (only attach mechanism) | refuse to start external |
| `flow-export` | `iface.SubscribeCollectNotify` | total feature loss (only data source) | refuse to start external |
| `ddos-detect` | `iface.SubscribeCollectNotify` + `trafficstat.EnsureGlobal`/`Global` | total feature loss (both paths affected) | warn |
