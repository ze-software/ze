# Behavioral Anomaly Detection

## Meta

| Field | Value |
|-------|-------|
| Name | Behavioral Anomaly Detection |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/anomaly/detect, internal/plugins/anomaly/observe |
| Real-path tests | test/plugin/anomaly-show.ci, test/plugin/anomaly-observe-show.ci |
| Docs | docs/guide/anomaly.md |
| Doc review | 2026-10-07: every source anchor resolves; ze_anomaly_incidents_total, ze_anomaly_active, ze_anomaly_tracked_entities and doctor-anomaly-detect-no-feature-source exist in internal/ |
| Defect review | 2026-10-07: audit found no open immediate spec against the detector; journal rows naming a Component, not each re-verified here: closure-deletes-a-cited-document.md:10, unwired-feature.md:133 |
| Extra criteria | supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Darktrace-style SECURITY anomaly detector (report-only), a domain separate from volumetric DDoS. Consumes the neutral `trafficfeature` signals and learns each entity's own pattern-of-life via a per-(entity,feature) EWMA baseline (`internal/core/stats`), scores self-deviation plus peer-group rarity (prefix cohorts), and correlates multiple weak feature deviations on one entity into a single incident (capped/discounted combine, not naive sum). An entity is a SOURCE (an anomalous sender), a DEST (a distributed sink or a probed host) or a PORT; a port is identified by proto and port rather than by an address, and is scored by self-deviation alone because it has no cohort. A confirm/clear state machine debounces; confirmed incidents emit on the `anomaly-detect` event bus and land in a bounded recent-incident ring viewed via `show anomaly detect`. It takes NO action (the `anomaly/shape` responder acts); scoring is bounded per-entity with idle eviction. A `ze doctor` check (`doctor-anomaly-detect-no-feature-source`) warns when enabled without a flow source. Prometheus: `ze_anomaly_incidents_total`, `ze_anomaly_active`, `ze_anomaly_tracked_entities` (labeled `dimension`=source/dest/port since 2026-08-18; it was a bare gauge, so an existing query stops matching and must sum over the label). <!-- source: internal/core/anomalyevent/event.go -- entity-kind event contract: source, dest, port --> <!-- source: internal/plugins/anomaly/detect/detector.go -- per-entity baseline, confirm/clear, emit --> <!-- source: internal/plugins/anomaly/detect/score.go -- pinned scoring and correlation rule --> <!-- source: internal/plugins/anomaly/detect/show.go -- ze-show:anomaly handler --> <!-- source: internal/plugins/anomaly/detect/yang/ze-anomaly-detect-conf.yang -- detector config --> The detector's ring records confirmations only, so a separate `anomaly-observe` plugin subscribes to the same events and keeps the incident LIFECYCLE: a bounded ring (`incident-ring-size`) opened on `AnomalyDetected` and finalized with an end time on `AnomalyCleared`, or by a one-second stale sweep (`stale-incident-timeout`) when a source goes silent and the detector evicts it without a clear. `show anomaly observe` returns that list newest-first with finalized incidents included, which is where a finished incident's duration is readable. <!-- source: internal/plugins/anomaly/observe/store.go -- lifecycle ring, eviction, stale sweep --> <!-- source: internal/plugins/anomaly/observe/show.go -- ze-show:anomaly-observe handler --> The detector, the lifecycle store and the shape responder are all compile-out-able with the `ze_anomaly` build tag: default-on in `ZE_FEATURES`, dropped from `ze-stripped` / bare `ze_core` builds, with an `anomaly {}` block rejected as unknown. <!-- source: feature-gates.txt -- ze_anomaly -->
