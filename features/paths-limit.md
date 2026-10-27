# PATHS-LIMIT (draft-abraitis-idr-addpath-paths-limit-04)

## Meta

| Field | Value |
|-------|-------|
| Name | PATHS-LIMIT (draft-abraitis-idr-addpath-paths-limit-04) |
| Kind | protocol |
| Scope | complete |
| Level | experimental |
| Components | internal/component/bgp/reactor |
| Real-path tests | test/plugin/paths-limit-live.ci, test/plugin/paths-limit-receive-ceiling.ci, test/decode/bgp-paths-limit.ci, test/encode/paths-limit.ci |
| Interop | bgp/bgp-paths-limit-frr |
| RFCs | draft-abraitis-idr-addpath-paths-limit |
| Docs | docs/guide/add-path.md |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; per-destination enforcement is what paths-limit-live.ci asserts |
| Defect review | 2026-10-07: no plan/immediate spec or journal row found naming PATHS-LIMIT |

## Description

Receiver-requested ADD-PATH limit per family and prefix. Ze enforces the peer's negotiated limit across batches on each destination connection, including route-server fast-path forwarding. First admitted paths keep their slots. Replacements remain permitted, withdrawals free slots, and a new connection resets the count. Local `capability { add-path { limit { receive N; } } }` advertises a receive limit, with per-family overrides, and Ze enforces it on received paths: an excess path is dropped, the rest of its UPDATE installs, and each refusal is counted. See [ADD-PATH and PATHS-LIMIT](guide/add-path.md). <!-- source: internal/component/bgp/reactor/session_paths_limit.go -- session-wide outbound admission --> <!-- source: internal/component/bgp/reactor/session_paths_limit_recv.go -- receiveCeiling -->
