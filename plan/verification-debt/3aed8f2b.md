# Verification debt -- commit session 3aed8f2b

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-05 | 3aed8f2b | close spec-traceroute-source-af (+1 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-09-05T17:25:15Z) | open |
| 2026-09-05 | 3aed8f2b | close spec-traceroute-source-af | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-05 | 3aed8f2b | close spec-traceroute-source-af | discovery-index freshness | ran ./le discovery-index update; its only delta is a row for internal/core/cpulist, a package another session added in this shared checkout, so committing ai/PACKAGE-MAP.md here would carry that session's work. This diff edits internal/core/probe/icmp.go inside an existing package and adds no package, so the index owes nothing to it. ai/PACKAGE-MAP.md is left regenerated and uncommitted for its owner. | open |
