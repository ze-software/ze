---
spec: bgp-prefix-count-mode-metric
topic: BGP
found-by: review
---

## Before

The four count-derived BGP prefix-limit Prometheus metrics (`ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total`) carried only `peer` and `family` labels, so scrapes could not distinguish whether a peer/family's numbers came from `count offered` (wire event tally) or `count installed` (unique route-identity set). Peer removal and reloads that switched a family's count mode also left obsolete per-family series behind in `/metrics`.

## After

`ze_bgp_prefix_count`, `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded`, and `ze_bgp_prefix_maximum_exceeded_total` now carry the closed `mode` label (`offered` or `installed`, matching `prefix { count offered|installed; }` and enforcement logs), while static limit gauges (`ze_bgp_prefix_maximum`, `ze_bgp_prefix_warning`) and peer-level metrics (`ze_bgp_prefix_teardown_total`, `ze_bgp_prefix_stale`) keep their existing label sets. Peer removal and count-mode reloads now retire obsolete `(peer, family, mode)` series immediately, block late metric writes from stopping sessions, and sweep any pre-shutdown residual series on session exit without disturbing a replacement or sibling peer's active series.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
