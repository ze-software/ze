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
| 2026-09-03 | ba6dd6ad | test(plugin): a plugin's two help texts survive its declaration | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | test(plugin): a plugin's two help texts survive its declaration | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | ba6dd6ad | test(plugin): a plugin's two help texts survive its declaration | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: the fixture joins an existing package | open |
| 2026-09-03 | ba6dd6ad | fix(cli): a config node declares two texts, and each reaches one surface | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | fix(cli): a config node declares two texts, and each reaches one surface | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-03 | ba6dd6ad | fix(cli): a config node declares two texts, and each reaches one surface | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no package is added | open |
| 2026-09-03 | ba6dd6ad | docs(yang): split the last three config modules into two texts | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | docs(yang): split the last three config modules into two texts | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no package is added | open |
| 2026-09-03 | ba6dd6ad | docs(yang): explain 290 config nodes and name 50 that had no summary | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | docs(yang): explain 290 config nodes and name 50 that had no summary | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no package is added | open |
| 2026-09-03 | ba6dd6ad | journal: seven defects the help-text pass walked into | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-03 | ba6dd6ad | journal: seven defects the help-text pass walked into | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: journal rows touch no package | open |
| 2026-09-04 | ba6dd6ad | fix(isis): a maximum-metric link is not used in normal SPF | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | fix(isis): a maximum-metric link is not used in normal SPF | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | ba6dd6ad | fix(isis): a maximum-metric link is not used in normal SPF | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no package is added | open |
| 2026-09-04 | ba6dd6ad | fix(ipsec): a Child SA rekey performs the pfs its config asks for | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | fix(ipsec): a Child SA rekey performs the pfs its config asks for | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | ba6dd6ad | fix(ipsec): a Child SA rekey performs the pfs its config asks for | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: no package is added | open |
| 2026-09-04 | ba6dd6ad | journal: nine finds from the config help pass, and one tooling cost | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | journal: nine finds from the config help pass, and one tooling cost | discovery-index freshness | ./le discovery-index update regenerated ai/PACKAGE-MAP.md with no change: journal rows touch no package | open |
| 2026-09-04 | ba6dd6ad | config: give 2,900 YANG config nodes a description and an explanation | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | fix: four defects the config-explanation pass found, at their source | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | fix: four defects the config-explanation pass found, at their source | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | ba6dd6ad | fix: four defects the config-explanation pass found, at their source | discovery-index freshness | the only PACKAGE-MAP row that moved is another session's: it deletes internal/component/bgp/plugins/redistribute_ingress and adds internal/core/rib/distance, neither of which this commit touches. This commit adds and removes no package. | open |
| 2026-09-04 | ba6dd6ad | fix: the review found the fib config leaves were never reachable | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | fix: the review found the fib config leaves were never reachable | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | ba6dd6ad | fix: the review found the fib config leaves were never reachable | discovery-index freshness | the only PACKAGE-MAP row that moved is another session's half-landed internal/core/rib/distance; this commit adds and removes no package | open |
| 2026-09-04 | ba6dd6ad | probe | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | probe | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | ba6dd6ad | probe | discovery-index freshness | probe | open |
| 2026-09-04 | ba6dd6ad | fix(fib): an empty section is a removal, not a malformed one | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | fix(fib): an empty section is a removal, not a malformed one | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | ba6dd6ad | fix(fib): an empty section is a removal, not a malformed one | discovery-index freshness | the only PACKAGE-MAP row that moved is another session's half-landed internal/core/rib/distance; this commit adds and removes no package | open |
| 2026-09-04 | ba6dd6ad | spec: complete command-help-and-description, 3000 explanations | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-09-04 | ba6dd6ad | spec: complete command-help-and-description, 3000 explanations | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-04 | ba6dd6ad | spec: complete command-help-and-description, 3000 explanations | discovery-index freshness | the only PACKAGE-MAP row that moved is another session's half-landed internal/core/rib/distance; this commit adds and removes no package | open |
