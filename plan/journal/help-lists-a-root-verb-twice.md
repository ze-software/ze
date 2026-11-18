| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-08 | spec-web-cli-config-one-off-defects (AC-11) | `zeUsage` | `ze help` repeats names registered as roots and top-level YANG verbs, including `plugin` and `resolve`; dispatch already lets the root win. | fixed: `zeUsage` retains each root's section and summary and adds only YANG-only names. `TestRootUsageListsSharedNamesOnce` and `TestRootUsageRetainsCommandMetadata` pass, counting printed rows with root-only and YANG-only controls. |
