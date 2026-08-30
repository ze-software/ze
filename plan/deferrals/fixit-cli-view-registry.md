# Deferrals: fixit-cli-view-registry

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-19 | spec-fixit-cli-view-registry Design2 | full owner-package view self-containment (Design 2) + migrate the generic monitor + future traffic views onto the registry | out of scope for a behavior-preserving refactor; noted follow-up ideal. **Triaged 2026-08-30 as an improvement, not a release defect:** the registry refactor already preserves behavior, so this is further refactoring and changes nothing an operator sees | plan/future/spec-finish-web-cli-ux.md | deferred |

