# Autonomous Anomaly Response

## Meta

| Field | Value |
|-------|-------|
| Name | Autonomous Anomaly Response |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/anomaly/shape |
| Real-path tests | test/plugin/anomaly-shape-shadow.ci |
| Docs | docs/guide/anomaly.md |
| Doc review | 2026-10-07: every source anchor resolves; the four ze_anomaly_shape_* metrics and doctor-anomaly-shape-armed-no-firewall exist in internal/ |
| Defect review | 2026-10-07: audit found no open immediate spec against the shape responder; journal rows naming a Component, not each re-verified here: gate-excludes-part-of-its-population.md:71, green-that-could-not-have-been-red.md:206 |
| Extra criteria | supported: armed mode, auto-revert, blast-radius cap and kill-switch driven end to end = test/plugin/anomaly-shape-armed.ci; supported: each journal row naming a Component re-verified as fixed or not a defect = none yet |

## Description

Shadow-first responder (`anomaly/shape`) that subscribes to `anomaly-detect` incidents and installs a surgical per-SOURCE firewall action (rate-limit via `firewall.MatchSourceAddress`+`Limit`, drop fallback). SHADOW is the default: it logs the would-be action and installs nothing. In armed mode each anomalous source gets its own live nft rule with a mandatory timed AUTO-REVERT (withdraws after a TTL regardless of any clear event), a global BLAST-RADIUS cap (refuses to arm beyond N), a KILL-SWITCH (reverts all + forces shadow), and an ALLOWLIST (protected sources are never armed). One mutex guards the armed map; a per-timer generation guard makes a superseded timer a no-op. Status via `show anomaly shape`. Separate firewall owner key isolates it from `ddos/local`. A `ze doctor` check (`doctor-anomaly-shape-armed-no-firewall`) warns when armed without a firewall. Prometheus: `ze_anomaly_shape_armed`, `ze_anomaly_shape_reverted_total`, `ze_anomaly_shape_arm_refused_total`, `ze_anomaly_shape_killswitch_total`. <!-- source: internal/plugins/anomaly/shape/responder.go -- pinned responder state machine --> <!-- source: internal/plugins/anomaly/shape/match.go -- source term + per-family tables --> <!-- source: internal/plugins/anomaly/shape/show.go -- ze-anomaly:show-shape handler --> <!-- source: internal/plugins/anomaly/shape/yang/ze-anomaly-shape-conf.yang -- responder config -->
