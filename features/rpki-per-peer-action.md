# RPKI Per-Peer Action

## Meta

| Field | Value |
|-------|-------|
| Name | RPKI Per-Peer Action |
| Kind | protocol |
| Scope | partial |
| Scope gaps | RFC 8210 gap row in rfc/short/rfc8210.md, invalid routes accepted under state policy (plan/immediate/spec-rpki-invalid-accepted-and-state-policy.md) |
| Level | experimental |
| Components | internal/component/bgp/plugins/rpki |
| Real-path tests | test/plugin/rpki-per-peer-action.ci, test/plugin/rpki-group-action.ci, test/plugin/rpki-revalidate-late-sync.ci, test/plugin/rpki-validate-reject.ci, test/plugin/rpki-validate-accept.ci |
| Interop | bgp/rpki-frr, bgp/rtr-stayrtr |
| RFCs | rfc8210, rfc6811 |
| Docs | docs/guide/rpki.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; peer > group > global resolution is what rpki-per-peer-action.ci and rpki-group-action.ci assert |
| Defect review | 2026-10-07: plan/immediate/spec-rpki-invalid-accepted-and-state-policy.md open; journal origin-rail disagreement row taken as open |
| Extra criteria | supported: blackhole-exempt exempts agreed blackhole routes through the daemon = test/plugin/rpki-blackhole-exempt.ci |

## Description

Origin (`invalid`/`not-found`) and ASPA (`invalid`/`unknown`) validation actions are settable globally and per-peer/per-group (`peer > group > global`, resolved per leaf), while cache servers and ASPA enable stay global. `rpki { blackhole-exempt true; }` exempts a session's agreed blackhole routes from origin validation, at the peer and group levels; setting it on a session that names no blackhole community logs that it has no effect rather than passing silently. `show bgp rpki status` reports the effective global and per-peer resolved actions, and separates two states an operator could not otherwise see: `sessions-synced` counts the caches that have completed a sync, and `synced` is false while `running` is true when none has. The RTR client separates the RFC 8210 Section 6 Refresh Interval from the Retry Interval, so a cache that answered and a cache that failed are not polled on one timer. A VRP change re-validates the routes already installed rather than only the ones that arrive next, per RFC 6811 Section 4, and the ASPA cache does the same for path verification. <!-- source: internal/component/bgp/plugins/rpki/origin_tracker.go -- origin re-validation on VRP change --> <!-- source: internal/component/bgp/plugins/rpki/aspa_tracker.go -- ASPA re-validation on cache change --> <!-- source: internal/component/bgp/plugins/rpki/rpki.go -- buildDecisions per-peer resolution, sessions-synced --> <!-- source: internal/component/bgp/plugins/rpki/rpki_config.go -- blackhole-exempt --> <!-- source: internal/component/bgp/plugins/rpki/rtr_session.go -- RFC 8210 Section 6 refresh and retry -->
