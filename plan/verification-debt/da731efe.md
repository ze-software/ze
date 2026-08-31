# Verification debt -- commit session da731efe

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-31 | da731efe | spec(rfc): measure the extraction drain and sign off four RFCs | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | feat(bfd): implement RFC 5880 Simple Password authentication | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | feat(bfd): implement RFC 5880 Simple Password authentication | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | da731efe | feat(bfd): implement RFC 5880 Simple Password authentication | discovery-index freshness | ran ./le discovery-index update: ai/PACKAGE-MAP.md is byte-identical, because simple.go joins the existing bfd/auth package and adds none | open |
| 2026-08-31 | da731efe | fix(bgp): scope the RFC 7313 error handling to its own capability | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | fix(bgp): scope the RFC 7313 error handling to its own capability | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | da731efe | fix(bgp): scope the RFC 7313 error handling to its own capability | discovery-index freshness | ran ./le discovery-index update: ai/PACKAGE-MAP.md is byte-identical, no package added or moved | open |
| 2026-08-31 | da731efe | fix(ppp): follow RFC 1661 on a malformed configuration option | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | fix(ppp): follow RFC 1661 on a malformed configuration option | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | da731efe | fix(ppp): follow RFC 1661 on a malformed configuration option | discovery-index freshness | ran ./le discovery-index update: ai/PACKAGE-MAP.md is byte-identical, no package added or moved | open |
| 2026-08-31 | da731efe | chore(rfc): regenerate the ledgers the RFC 1661 and 7313 work derives | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | docs(rfc): record the trigger that arms the extraction drain quota | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | da731efe | docs(journal): record what this session's shared-checkout friction cost | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
