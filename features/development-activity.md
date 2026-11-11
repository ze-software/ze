# Development Activity

## Meta

| Field | Value |
|-------|-------|
| Name | Development Activity |
| Kind | dev-tool |
| Scope | complete |
| Level | experimental |
| Components | internal/le/site/activity.go, internal/le/site/activitygrowth.go, internal/le/repo/rewrite/activitymeasure.go |
| Real-path tests | internal/le/site/activity_test.go::TestTheActivityPageReadsAsThePublishedPage, internal/le/site/activity_growth_integration_test.go::TestActivityGrowthUsesExistingDays |
| Doc review | 2026-10-07: the description read against renderActivityPage and activityBody (internal/le/site/activity.go), which publish /project/activity/ with a year of commits, added lines and Go composition from the window activitymeasure.go measures |
| Defect review | 2026-10-07: plan/journal/helper-bypassed-by-an-open-coded-copy.md 2026-08-31 row (activitystyle.go near-white surfaces under the dark theme): open, a display defect in this feature, so the level stays experimental |

## Description

The website's activity page has Calendar and Growth tabs. Calendar shows a year
of daily commits and added lines. Growth plots the cumulative commits and added
lines within that same window, with a date readout available by pointer or
keyboard. Added lines count additions, rather than the repository's net size.

The growth view also shows current Go line totals for code, tests and third-party
dependencies, plus their overall total. These are a current snapshot; the
existing measurements do not contain historical category totals.
