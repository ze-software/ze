# Operational Report Bus

## Meta

| Field | Value |
|-------|-------|
| Name | Operational Report Bus |
| Kind | daemon |
| Scope | partial |
| Scope gaps | functional coverage of most named sources, empty-bus shutdown observation (plan/spec-finish-report-bus.md) |
| Level | experimental |
| Components | internal/core/report |
| Real-path tests | test/plugin/warnings-show.ci, test/plugin/warnings-filter-show.ci, test/plugin/errors-sent-show.ci, test/plugin/errors-received-show.ci, test/plugin/errors-config-abort-show.ci |
| Docs | docs/guide/operational-reports.md |
| Doc review | 2026-10-07: internal/core/report/report.go names show errors and the login banner as readers of the same source |
| Defect review | 2026-10-07: open: plan/spec-finish-report-bus.md (empty-bus shutdown hang not reproduced, coverage unfinished) |

## Description

Cross-subsystem `ze show warnings` and `ze show errors` commands with `source <name>` filtering: single place to surface prefix-threshold crossings, stale route data, BGP NOTIFICATIONs sent/received, unexpected session drops, session-stuck/flap/EOR-timeout, route-count-anomaly (>50% drop), FIB sync failures/orphans/programming-lag, firewall stale-table/drift, plugin crashes, interface error counters. State-based warnings + event-based error ring, login banner reads the same source.
