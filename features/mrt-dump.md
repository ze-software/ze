# MRT Dump

## Meta

| Field | Value |
|-------|-------|
| Name | MRT Dump |
| Page | docs/guide/mrt-analysis.md |
| Kind | daemon |
| Scope | complete |
| Level | experimental |
| Components | internal/plugins/mrt, internal/mrt, internal/analyze |
| Real-path tests | test/plugin/mrt-dump-all.ci, test/plugin/mrt-dump-updates.ci |
| RFCs | rfc6396, rfc8050 |
| Docs | docs/guide/mrt-analysis.md |
| Doc review | 2026-10-07: malformed-record warning text read in internal/analyze/mrt.go; the row makes no writer AS-width claim |
| Defect review | 2026-10-07: plan/journal/helper-bypassed-by-an-open-coded-copy.md (2026-09-14) plugins/mrt direction words versus events.ParseDirection, unfixed; RFC6396-4.4.3-1 MUST gap in rfc/short/rfc6396.md |
| Extra criteria | supported: bgpdump reads a daemon-written MRT file = test/plugin/mrt-dump-bgpdump-readback.ci |

## Description

RFC 6396 daemon-side MRT recording with three independent streams (updates, all messages, periodic TABLE_DUMP_V2 RIB snapshots). YANG config, per-peer and direction filtering, extended timestamps, add-path aware, on-demand CLI dump, async non-blocking writes, strftime file rotation. Analysis tools: show, routes, aspath, inject, replay, convert (pcap/json), statistics, filter. The reader derives each record's AS width from the record type rather than from one file-wide guess, decodes the RFC 8050 add-path TABLE_DUMP_V2 and BGP4MP subtypes, and REPORTS what it could not decode instead of printing it as fact: a record that fails to parse renders `[unparseable: <tag>] <err>`, and every analysis subcommand ends with `warning: N malformed MRT record(s) skipped or partially decoded; results are incomplete`. <!-- source: internal/mrt/types.go -- RFC 8050 add-path subtypes --> <!-- source: internal/analyze/mrt.go -- malformed-record warning --> <!-- source: internal/analyze/show.go -- unparseable record rendering --> The daemon MRT plugin is compile-out-able with the `ze_mrt` build tag (default-on in `ZE_FEATURES`): `ze-stripped` and bare `ze_core` builds drop `internal/plugins/mrt` and its config schema, and the `internal/mrt` format library plus the shared `internal/core/bgp/msgtype` leaf drop by dead-code elimination once neither MRT nor the BGP engine references them. MRT loads inert without a BGP source, so it is independent of `ze_bgp`; its reactor bridges reach the engine through a plugin-registry seam so the always-on hub never imports the plugin. <!-- source: internal/plugins/mrt/component.go -- daemon MRT component --> <!-- source: internal/analyze/register.go -- le mrt subcommands --> <!-- source: feature-gates.txt -- ze_mrt --> <!-- source: internal/component/plugin/all/all_ze_mrt.go -- gated MRT imports -->
