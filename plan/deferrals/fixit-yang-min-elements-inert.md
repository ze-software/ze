# Deferrals: fixit-yang-min-elements-inert

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-20 | spec-fixit-yang-min-elements-inert | severity asymmetry: an absent `mandatory` leaf WARNS but an absent min-bounded leaf-list REJECTS, though both mean "required" | pre-existing severity mapping (`cmd_validate.go` ErrTypeMissing to Warning); reject is the defensible direction and no config is spuriously rejected; documented in the learned summary | plan/learned/1230-yang-min-elements-inert.md | cancelled |
| 2026-07-20 | spec-fixit-yang-min-elements-inert | walkTree LIST branch shares the count-0 hole the leaf-list branch had (no absent-scan for a min-bounded `list`) | zero live triggers: all three `min-elements` declarations in the tree are leaf-lists, no `list` declares `min-elements`; documented as theoretical in the learned summary | plan/learned/1230-yang-min-elements-inert.md | cancelled |

