# Verification debt -- commit session c2c5e8ba

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-29 | c2c5e8ba | fix(site): stamp the publication time the native build stopped writing | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T11:55:15Z) | open |
| 2026-08-29 | c2c5e8ba | fix(site): stamp the publication time the native build stopped writing | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-29 | c2c5e8ba | docs(plan): open the spec restoring the site page renderers | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T12:29:18Z) | open |
| 2026-08-29 | c2c5e8ba | docs(plan): open the spec restoring the site page renderers | native structural checks (red) | The recorded staticcheck-feature-matrix red was mine and is repaired. I added github.com/yuin/goldmark to go.mod without running go mod vendor, which left the checkout inconsistently vendored; the 13:29 verify run recorded the resulting stage failure with no path attribution, so it is charged to every commit. Another session ran go mod vendor and reapplied the three vendorpatch patches. A fresh ./le staticcheck-feature-matrix check now exits 0, twice. This commit carries one markdown spec file and no Go. | open |
