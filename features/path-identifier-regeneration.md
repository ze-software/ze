# Path Identifier regeneration (RFC 7911)

## Meta

| Field | Value |
|-------|-------|
| Name | Path Identifier regeneration (RFC 7911) |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/reactor |
| Real-path tests | test/plugin/adj-rib-in-replay-addpath-source.ci |
| Interop | bgp/bgp-addpath-readvertise-collision-frr |
| RFCs | rfc7911 |
| Docs | docs/guide/add-path.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; fwdPathIDTable.generate keys on the received identifier including 0 and used tracks every assigned id (internal/component/bgp/reactor/forward_path_id.go) |
| Defect review | 2026-10-07: journal zero-value-as-valid-answer rows (2026-09-20) name the RIB-out and JSON surfaces not this table; judged not to touch the regeneration table |

## Description

Ze generates its own Path Identifier for every route it re-advertises rather than relaying the one the source chose. The identifier is keyed on the ingress path, and how much of the path the key holds follows what the SOURCE framed. A source that negotiated no ADD-PATH names a path by its prefix alone, so ze holds one identifier for that source's whole session and releases it when the peer is removed. A source that negotiated ADD-PATH names a path by (prefix, identifier), so ze holds one entry per family, received identifier and prefix, and frees it once it has relayed that pair's withdraw. Either way the same path always gets the same value, and two route-server clients that happened to pick the same identifier no longer collide at a third. A received identifier of 0 is a value and not an absence: RFC 7911 Section 3 makes it legal, and a source that negotiated no ADD-PATH sends every path under it. The table tracks every identifier currently assigned, so a wrapped counter cannot hand a live path's identifier to a second path. <!-- source: internal/component/bgp/reactor/forward_path_id.go -- fwdPathIDTable.generate, fwdPathIDTable.generatePath, fwdRegenerateRawPathIDs -->
