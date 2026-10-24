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

RFC 6396 daemon-side MRT recording with three independent streams (updates, all messages, periodic TABLE_DUMP_V2 RIB snapshots). YANG config, per-peer and direction filtering, extended timestamps, add-path aware, on-demand CLI dump, async non-blocking writes, strftime file rotation. Analysis tools: show, routes, aspath, inject, replay, convert (pcap/json), statistics, filter. The reader derives each record's AS width from the record type rather than from one file-wide guess, decodes the RFC 8050 add-path TABLE_DUMP_V2 and BGP4MP subtypes, and REPORTS what it could not decode instead of printing it as fact: a record that fails to parse renders `[unparseable: <tag>] <err>`, and every analysis subcommand ends with `warning: N malformed MRT record(s) skipped or partially decoded; results are incomplete`. <!-- source: internal/mrt/types.go -- RFC 8050 add-path subtypes --> <!-- source: internal/analyze/mrt.go -- malformed-record warning --> <!-- source: internal/analyze/show.go -- unparseable record rendering --> The daemon MRT plugin is a BGP plugin gated under `ze_bgp`, with no build tag of its own: every MRT stream is BGP data and its TABLE_DUMP_V2 snapshot is filled only by `bgp-rib`, so `ze-stripped` and every build without `ze_bgp` drop `internal/plugins/mrt` and its config schema, and the `internal/mrt` format library plus the shared `internal/core/bgp/msgtype` leaf drop by dead-code elimination with the engine. In a BGP build the RIB dump always finds `bgp-rib`: it is auto-loaded by the same `bgp` config root that starts BGP. Its reactor bridges reach the engine through a plugin-registry seam so the always-on hub never imports the plugin. <!-- source: internal/plugins/mrt/component.go -- daemon MRT component --> <!-- source: internal/analyze/register.go -- le mrt subcommands --> <!-- source: feature-gates.txt -- ze_bgp internal/plugins/mrt --> <!-- source: internal/component/plugin/all/all_ze_bgp.go -- gated MRT imports -->
