---
spec: l2tp-pool-reload-applies
topic: L2TP
feature: l2tpv2-bng
found-by: review
---

## Before

A committed pool edit could leave the old ranges active. RADIUS assignments could collide with pool allocations, and undelivered address answers could retain reservations.

An explicitly external pool plugin configured a separate process while the daemon's address handlers remained unconfigured.

## After

Pool edits and rollback rebuild reservations from live subscriber ownership. Existing subscribers retain their addresses. New assignments exclude held addresses, including RADIUS assignments outside current ranges. Both L2TP and PPPoE release reservations when an address answer cannot reach its subscriber.

Invalid or overlapping pools are refused before bitmap construction. Equal numeric session coordinates on L2TP and PPPoE retain separate ownership.

Doctor identifies an external pool block and gives its internal replacement. Runtime refuses the unsupported mode before configuration. Automatic loading and explicit internal mode continue to allocate across reloads.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
