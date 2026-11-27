---
spec: fleet-ui-8-alarm-causes
topic: Fleet
feature: fleet-management
---

## Before

When a managed device went offline, its stale `component-down`, `component-degraded`, and `resource-high` alarms remained listed beside `device-offline` in `show fleet alarm` with no link showing which alarms were consequences of the outage. Operators had to visually separate stale downstream rows from root causes, could not filter hidden versus visible alarms on `show fleet alarm`, and alarm history rows gave no indication of whether an occurrence had overlapped an outage on the same device.

## After

Alarm kinds now declare their own same-device causes and settling behavior on their registration (`component-down`, `component-degraded`, and `resource-high` declare `device-offline`, while `config-diverged`, `config-rejected`, `software-behind`, `history-full`, and future `relay-not-granted` remain visible beside `device-offline`). At hub startup, the hub validates the full cause graph and refuses unknown cause names, cycles of any length, causes less severe than the kind they would hide, or hub-scoped causes, naming both kinds and the defect.

On `show fleet alarm`, the hub computes `hidden-behind` per query over each caller's authorized open alarms on the same device (breadth-first nearest open cause, ties broken by smallest stable occurrence identifier in byte order) without storing separate hidden or silencing state. Unfiltered queries return every authorized open alarm with `hidden-behind` set on hidden rows and omitted on shown rows; `show fleet alarm | hidden true` selects hidden rows, `show fleet alarm | hidden false` selects visible rows, and unsupported values return an operator refusal naming `true` and `false`. When `device-offline` clears, still-failing non-settling alarms (`component-down`, `component-degraded`) lose `hidden-behind` while keeping their original identifier and opening time, whereas settling `resource-high` occurrences close at reconnect and open a fresh occurrence only after post-reconnect samples stay above threshold for `resource-duration`. On `show fleet alarm history device <name>`, retained history rows project `hidden`, `hidden-behind`, and `hidden-unknown` (when retention expiry or truncation prevents proving overlap for an older row) directly from same-device cause intervals without persisting extra state.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
