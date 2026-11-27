---
spec: traffic-qdisc-stats
topic: Telemetry
feature: traffic-control-lifecycle
---

## Before
`show traffic control` refused to run on Linux routers without a `traffic { control { } }` block. Kernel qdisc and class statistics were discarded, `class-count` stayed at `0`, and default kernel qdiscs (`noqueue`, `pfifo_fast`, `mq`) reported as `htb`. Prometheus `/metrics` exposed only interface-level drops with no per-qdisc or per-class queue visibility.

## After
`show traffic control`, `ze traffic control show interface <name>`, and Prometheus `/metrics` now report live Linux kernel qdisc and class statistics (`bytes`, `packets`, `qlen`, `backlog`, `drops`, `overlimits`, `requeues`), real qdisc kinds, handles, and configured class names. This works on unshaped Linux routers even without `traffic { control { } }` or `interface { }` config. Prometheus exports 14 `ze_traffic_qdisc_*` and `ze_traffic_class_*` gauges every second when `telemetry { prometheus { enabled true; } }` is active, independent of `netdata.enabled`, and removes stale series when interfaces or classes disappear. On VPP, `stats` and queue series are omitted rather than zero-filled.

## Corrects
none

## Exposure
none

## Upstream
none

## Asked by
none
