# Verification debt -- commit session da731efe

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-31 | da731efe | spec(rfc): measure the extraction drain and sign off four RFCs | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | feat(bfd): implement RFC 5880 Simple Password authentication | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | feat(bfd): implement RFC 5880 Simple Password authentication | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | da731efe | feat(bfd): implement RFC 5880 Simple Password authentication | discovery-index freshness | ran ./le discovery-index update: ai/PACKAGE-MAP.md is byte-identical, because simple.go joins the existing bfd/auth package and adds none | open |
