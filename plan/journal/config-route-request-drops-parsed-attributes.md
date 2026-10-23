| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-22 | exabgp-6-api-compatibility | `bgpconfig.buildConfigRouteRequest` | A plugin family's update block (flow, sr-policy) never copies `RawAttributes`, `LargeCommunity`, `Aggregator`, `AtomicAggregate` or `AIGPMetric` into `registry.ConfigRouteRequest`, so the route goes out without them, silently. | fixed: the dispatcher encodes every generic attribute once (`bgpconfig.genericPluginRouteAttrs`); plugins emit only NLRI and family attributes |
