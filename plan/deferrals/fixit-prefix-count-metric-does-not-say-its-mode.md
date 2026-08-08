# Deferrals: fixit-prefix-count-metric-does-not-say-its-mode

One issue, recorded not fixed (owner instruction, 2026-08-08). The aggregate
live backlog is folded on read from `plan/deferrals/` by `/ze-status`. Nothing
stores it (`ai/rules/planning.md`).

**Issue:** `ze_bgp_prefix_count` does not say which count mode produced it

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-08 | spec-bgp-per-peer-received-counter, after the `installed` mode was rewritten to count a set | `ze_bgp_prefix_count` carries `{peer, family}` only (`reactor_metrics.go`), and `ze_bgp_prefix_ratio`, `ze_bgp_prefix_warning_exceeded` and the two `_total` counters derive from the same number with the same labels. Since the per-family `count` leaf landed, two peers scraped into one dashboard can report numbers of different KINDS: `offered` is a tally of announcements and `installed` is the size of a set. An operator cannot tell a peer that overshot from a peer sitting at its limit, and a sum across peers adds two units. The enforcement LOG line now names the mode (`reportPrefixExceeded`, `session_prefix.go`), so the gap is on the metric surface alone | Adding a label to an existing gauge is a breaking change to every dashboard query and recording rule that aggregates it, and it is a surface decision rather than a defect: the alternative is a separate `ze_bgp_prefix_count_mode` info gauge, which is the usual Prometheus answer and breaks nothing. The rewrite's goal holds without either, so this is separable (`ai/rules/rule-precedence.md`). The spec `plan/spec-bgp-per-peer-received-counter.md` already carries the labelling obligation as a Decision annotation and is the natural home | `plan/spec-bgp-peer-metric-labels.md` owns the per-peer label surface; confirm it before writing a new spec | deferred |
