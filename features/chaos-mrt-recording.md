# Chaos MRT Recording

## Meta

| Field | Value |
|-------|-------|
| Name | Chaos MRT Recording |
| Kind | dev-tool |
| Scope | complete |
| Level | experimental |
| Components | internal/chaos/report/mrtlog.go, internal/chaos/orchestrator/cli.go |
| Doc review | 2026-10-07: BGP4MP_MESSAGE_AS4 and BGP4MP_STATE_CHANGE_AS4 written in mrtlog.go; strftime patterns accepted by the --mrt-file flag in cli.go; readability by bgpdump and bgpkit-parser is unproven rather than known false |
| Defect review | 2026-10-07: no journal row or immediate spec names internal/chaos/report |
| Extra criteria | supported: a chaos run with --mrt-file read back by bgpdump = test/chaos/chaos-mrt-bgpdump.ci; supported: a guide page documents --mrt-file = docs/guide/chaos-mrt.md |

## Description

`--mrt-file` flag produces standard BGP4MP_MESSAGE_AS4 and BGP4MP_STATE_CHANGE_AS4 MRT records from chaos peer events, readable by bgpdump, bgpkit-parser and `./le mrt`. Strftime filename patterns for rotation. <!-- source: internal/chaos/report/mrtlog.go -- MRTLog consumer -->
