# IRR BGP Import Filtering

## Meta

| Field | Value |
|-------|-------|
| Name | IRR BGP Import Filtering |
| Page | docs/guide/irr-filtering.md |
| Kind | daemon |
| Scope | complete |
| Level | stub-backed |
| Components | internal/component/bgp/plugins/filter_irr, internal/component/resolve/irr |
| Real-path tests | test/plugin/filter-irr.ci, test/plugin/filter-irr-fail.ci, test/plugin/filter-irr-update.ci |
| Docs | docs/guide/irr-filtering.md |
| Stub evidence | test/plugin/filter-irr.ci, test/plugin/filter-irr-fail.ci, test/plugin/filter-irr-update.ci |
| Doc review | 2026-10-07: every source anchor in the Description resolves to its file and symbol; status prefix dry-run and refresh commands are registered by filter_irr |
| Defect review | 2026-10-07: journal command-takes-an-untyped-positional-value row naming filter_irr taken as open |
| Extra criteria | supported: prefix lists built from a real IRR mirror or recorded answers = test/plugin/filter-irr-recorded-mirror.ci |

## Description

`bgp-filter-irr` builds per-ASN import prefix-lists from explicit or PeeringDB-discovered IRR AS-SETs, refreshes and persists the lists, and rejects received prefixes outside the selected list. Includes status, prefix inspection, dry-run checks, and manual refresh commands. <!-- source: internal/component/bgp/plugins/filter_irr/filter_irr.go -- IRR import filter --> <!-- source: internal/component/resolve/irr/store/store.go -- shared persisted prefix store -->
