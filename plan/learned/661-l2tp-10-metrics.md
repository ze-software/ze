# Learned: spec-l2tp-10-metrics -- Prometheus Metrics

## What Was Built

Prometheus metrics for L2TP sessions (13 metrics) and RADIUS servers
(4 metrics). 30-second stats poller reads kernel counters. Per-session
series with full label set, cleaned up asynchronously on teardown.

## Key Decisions

- **`ze_l2tp_*` / `ze_radius_*` naming.** Follows ze's `ze_<component>_*`
  convention. No `nas_*` names (legacy convention dropped).

- **Per-session labels.** `sid`, `ifname`, `username`, `ip`, `caller_id`.
  `caller_id` is always empty (session struct does not track
  Calling-Station-Id); present for forward compatibility.

- **Asynchronous series cleanup.** Per-session series deleted at next poll
  tick (up to 30s delay) when pppN disappears from snapshot, not
  synchronously on SessionDown. Observer lacks the label set needed for
  immediate deletion.

- **CQM bucket observe.** When observer finalizes a 100s bucket,
  `observeCQMBucket` feeds min/avg/max RTT into the histogram and updates
  loss ratio and bucket state gauges. Per-login keyed by `username`.

- **Kernel counter wrap handling.** Stats poller computes deltas between
  polls, handling uint32 counter wraps correctly.

## Mistakes / Surprises

- `lcp_echo_loss_ratio` was registered but never set. Added loss
  computation after discovery.

- RADIUS metrics were called with empty server labels. Had to thread
  `serverAddr` through all call sites.

## Patterns Worth Reusing

- Snapshot-based polling (reactor.Snapshot() + iface.GetStats) decouples
  metrics collection from session lifecycle events. Simple, no races.
