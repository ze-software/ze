# Verification debt -- commit session ba6dd6ad

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-03 | ba6dd6ad | fix(cli): one spelling for each of a command's two help texts | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | fix(cli): one spelling for each of a command's two help texts | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | ba6dd6ad | fix(cli): one spelling for each of a command's two help texts | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: this commit renames fields inside existing packages and adds none | open |
| 2026-09-03 | ba6dd6ad | feat(le): two checks for a command's summary and its explanation | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | feat(le): two checks for a command's summary and its explanation | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | ba6dd6ad | feat(le): two checks for a command's summary and its explanation | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: the new files join existing packages | open |
| 2026-09-03 | ba6dd6ad | docs(yang): give every command node a one-line summary | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | docs(yang): give every command node a one-line summary | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no package is added | open |
| 2026-09-03 | ba6dd6ad | journal: four finds from the command help pass | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | journal: four finds from the command help pass | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no Go package is touched | open |
| 2026-09-03 | ba6dd6ad | docs(yang): give every command an explanation beside its summary | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | docs(yang): give every command an explanation beside its summary | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | ba6dd6ad | docs(yang): give every command an explanation beside its summary | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no package is added | open |
| 2026-09-03 | ba6dd6ad | test(ui): the operator reads a command's summary and its explanation | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | test(ui): the operator reads a command's summary and its explanation | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: two .ci files add no package | open |
