# Development Activity

## Meta

| Field | Value |
|-------|-------|
| Name | Development Activity |
| Kind | dev-tool |
| Scope | complete |
| Level | experimental |
| Components | internal/le/site/activity.go, internal/le/repo/rewrite/activitymeasure.go |
| Real-path tests | internal/le/site/activity_test.go::TestTheActivityPageReadsAsThePublishedPage |
| Doc review | 2026-10-07: the description read against renderActivityPage and activityBody (internal/le/site/activity.go), which publish /project/activity/ with a year of commits, added lines and Go composition from the window activitymeasure.go measures |
| Defect review | 2026-10-07: plan/journal/helper-bypassed-by-an-open-coded-copy.md 2026-08-31 row (activitystyle.go near-white surfaces under the dark theme): open, a display defect in this feature, so the level stays experimental |

## Description

The website's activity page shows a year of Ze's commits and added lines, and the composition of its Go code.
