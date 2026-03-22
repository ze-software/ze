# 413 -- prefix-limit

## Context

BGP peers can send an unbounded number of prefixes, exhausting memory and masking route leaks or misconfigurations. Every major BGP implementation supports per-peer prefix limits -- operators expect it. Ze also needed prefix maximum values as a prerequisite for `spec-forward-congestion`, which uses them to size per-peer buffer allocations.

## Decisions

- Prefix maximum is mandatory per family, not optional. Chose strictness over convenience because an unconfigured peer defeats the safety purpose.
- Enforcement runs in `processMessage()` before plugin delivery, not in `handleUpdate()` after. Chose pre-delivery over post-delivery because over-limit routes must never reach the RIB or be forwarded (AC-27).
- Withdrawals counted before announces in the same UPDATE. Chose this over announce-first because prefix replacement (withdraw old + announce new) would falsely trigger the limit.
- Wire-level NLRI counting over RIB-level counting. Chose this because the feature must not depend on the RIB plugin. Re-announcements count twice (conservative direction).
- Config parser enhanced: `parseListInlineBlock` supports block entries inside inline blocks. Chose general parser improvement over family-specific hack.
- `teardown=false` drops over-limit UPDATEs silently via a `drop` return from `checkPrefixLimits`. Chose drop-before-delivery over deliver-then-filter because the RIB should never see excess routes.

## Consequences

- Every `.ci` test and chaos config must include `prefix { maximum N; }` for each family. 176 files updated -- any new test must include it.
- `spec-forward-congestion` can read `PeerSettings.PrefixMaximum` for buffer sizing.
- Data infrastructure (zefs storage, CLI update commands, autocompletion, staleness detection) deferred to `spec-prefix-data.md` with explicit task items.
- Prometheus prefix metrics implemented: count, maximum, warning, warning_exceeded, exceeded_total, teardown_total (per peer+family labels).
- Enforcement `.ci` test has a race condition in ze-peer (connection closed before NOTIFICATION read). Needs ze-peer improvement for close-time message capture.

## Gotchas

- YANG `family` was already a `list`, not a `leaf-list`. The spec incorrectly claimed a structural migration was needed.
- Config parsing is in `reactor/config.go` (parsePeerFromTree), not `config/peers.go` (does not exist).
- Test asserting "no NOTIFICATION returned" does not prove "routes rejected." AC-linked tests must assert the behavior stated in the AC text, not a mechanism proxy. New rule added to `tdd.md`.
- Saying "tests pass" is not completion. Docs, audit, learned summary are part of the deliverable. New rule added to `quality.md`.

## Files

- `internal/component/bgp/reactor/session_prefix.go` -- prefix counting, enforcement, NOTIFICATION building
- `internal/component/bgp/reactor/session_read.go` -- prefix check before plugin delivery
- `internal/component/bgp/reactor/config.go` -- config parsing, mandatory validation
- `internal/component/bgp/reactor/peersettings.go` -- PrefixMaximum/Warning/Teardown/IdleTimeout fields
- `internal/component/bgp/reactor/peer.go` -- auto-reconnect with exponential backoff
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- prefix container in family list
- `internal/component/config/parser_list.go` -- block entries in inline list blocks
