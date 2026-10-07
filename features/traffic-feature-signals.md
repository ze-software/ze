# Traffic Feature Signals

## Meta

| Field | Value |
|-------|-------|
| Name | Traffic Feature Signals |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/component/trafficfeature |
| Real-path tests | test/plugin/traffic-feature-show.ci |
| Docs | docs/guide/anomaly.md |
| Doc review | 2026-10-07: every source anchor resolves; fan-out, port-entropy, new-peer, rare-port and beaconing are named in internal/component/trafficfeature |
| Defect review | 2026-10-07: audit found no open immediate spec against trafficfeature; journal rows naming a Component, not each re-verified here: comment-describes-superseded-behaviour.md:34, unwired-feature.md:132 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Neutral per-source detection SIGNALS (facts, never verdicts) derived from the observation feed by a second consumer alongside the traffic monitor: fan-out (distinct destinations), out/in byte ratio (exfiltration), destination-port entropy, new-peer, rare-port/proto, and coarse beaconing (interval regularity, bounded to periods of a few seconds by the 1s sampling tick). Computed via the shared `internal/core/stats` primitives (rolling window, Shannon entropy, interval regularity). Viewed via `show traffic feature`; bounded per-source state with idle eviction. The judgment layer (the `anomaly` detection family) consumes these facts. <!-- source: internal/component/trafficfeature/feature.go -- per-source feature aggregation --> <!-- source: internal/component/trafficfeature/service.go -- lazy refcounted feed consumer --> <!-- source: internal/core/stats/entropy.go -- Shannon entropy --> <!-- source: internal/core/stats/beacon.go -- interval regularity --> <!-- source: internal/component/trafficfeature/cmd/traffic_feature.go -- ze-show:traffic-feature handler -->
