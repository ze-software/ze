# IRR BGP Import Filtering

## Meta

| Field | Value |
|-------|-------|
| Name | IRR BGP Import Filtering |
| Page | docs/guide/irr-filtering.md |
| Kind | daemon |
| Scope | complete |
| Level | stub-backed |
| Components | internal/component/bgp/plugins/filter_irr, internal/plugins/irr, internal/component/resolve/irr |
| Real-path tests | test/plugin/filter-irr.ci, test/plugin/filter-irr-fail.ci, test/plugin/filter-irr-update.ci, test/plugin/irr-restart-keeps-bgp-list.ci |
| Docs | docs/guide/irr-filtering.md |
| Stub evidence | test/plugin/filter-irr.ci, test/plugin/filter-irr-fail.ci, test/plugin/filter-irr-update.ci, test/plugin/irr-restart-keeps-bgp-list.ci |
| Doc review | 2026-11-15: every source anchor in the Description resolves to its file and symbol; status, prefix and dry-run commands are registered by filter_irr, the refresh commands (`update irr`, `clear irr`) by the irr plugin |
| Defect review | 2026-10-07: journal command-takes-an-untyped-positional-value row naming filter_irr taken as open |
| Extra criteria | supported: prefix lists built from a real IRR mirror or recorded answers = test/plugin/filter-irr-recorded-mirror.ci |

## Description

`bgp-filter-irr` enforces per-ASN import prefix-lists from explicit or PeeringDB-discovered IRR AS-SETs and rejects received prefixes outside the selected list. It fetches nothing: the `irr` plugin fetches, schedules (`system irr refresh-interval`) and stores every list, and the filter reads the stored copies, so a restart enforces the lists it enforced before with no IRR query. Includes status, prefix inspection and dry-run checks; `update irr` and `clear irr` refresh and remove the stored lists. <!-- source: internal/component/bgp/plugins/filter_irr/stored.go -- storeReader --> <!-- source: internal/plugins/irr/datasets.go -- writeDataset -->
