# test-verdict-depends-on-the-date

A test fixes a date in its fixture while the code under test reads today's
date. The test passes until the calendar moves past a window, then goes red
with no change to the code.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-14 | route-policy-1-gate-proof-and-forked-sysrib (walked into) | `TestReportListsUnderClaimAsCandidate` (`internal/le/feature/report_test.go`) over the seeded tree of `check_test.go` | Red since 2026-11-06 with no edit to the package: the fixture's review dates are 2026-10-07 and the feature report's 30-day staleness rule reads today's date, so the fixture aged out. Met in the package run for cab4307742 | open: give the report the date it judges against, and seed the fixture relative to that date |
