---
spec: traffic-usage-ipv6
topic: Telemetry
feature: traffic-usage
---

## Before

The `traffic-usage` TCX eBPF accounting programs accepted only IPv4 frames.
IPv6 traffic on dual-stack or IPv6 interfaces was absent from `show traffic
usage`, the `ze_traffic_usage_*` Prometheus metrics, and the source-IP
observation feed.

## After

`traffic-usage` now counts IPv6 frames alongside IPv4 in its per-port and
per-protocol maps (`tu_port_in`, `tu_port_out`). When `track-ip` is enabled, it
adds 16-byte LRU maps (`tu_ip6_in`, `tu_ip6_out`) alongside the 4-byte IPv4
maps. `show traffic usage` and `ze_traffic_usage_*` Prometheus metrics publish
canonical RFC 5952 IPv6 addresses, `icmpv6` labels, and `ip6_ingress` /
`ip6_egress` map counts. IPv6 source addresses also flow into the anomaly and
DDoS observation feed. IPv6 packets with extension headers before TCP or UDP
count per-IP (when `track-ip` is enabled) and under port `0` with their first
`Next Header` number.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
