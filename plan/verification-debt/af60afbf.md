# Verification debt -- commit session af60afbf

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-07 | af60afbf | fix(le): a scoped verify certificate answers about content, not HEAD (+8 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-09-05T17:25:15Z) | open |
| 2026-09-07 | af60afbf | fix(le): a scoped verify certificate answers about content, not HEAD (+4 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-07 | af60afbf | fix(le): an empty change selection is a skip or a refusal, never a pass (+5 more) | discovery-index freshness | ./le discovery-index update leaves ai/PACKAGE-MAP.md byte-identical: this commit adds no package and removes none | open |
