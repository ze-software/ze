# Verification debt -- commit session bae6e1b4

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-30 | bae6e1b4 | fix(ike): compare algorithm identifiers with bytes.Equal | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T14:01:28Z) | open |
| 2026-08-30 | bae6e1b4 | fix(ike): compare algorithm identifiers with bytes.Equal | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-30 | bae6e1b4 | fix(bgp): one RFC 9072 OPEN parameter, and a gate for End-of-RIB | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T14:01:28Z) | open |
| 2026-08-30 | bae6e1b4 | fix(bgp): one RFC 9072 OPEN parameter, and a gate for End-of-RIB | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-30 | bae6e1b4 | fix(bgp): one RFC 9072 OPEN parameter, and a gate for End-of-RIB | discovery-index freshness | ran ./le discovery-index update over this tree: it rewrote ai/PACKAGE-MAP.md and produced no diff, so the map is already current for these files and naming it would add an unchanged path to the commit | open |
| 2026-08-30 | bae6e1b4 | fix(bgp): carry the definitions the last commit's files reference | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T14:01:28Z) | open |
| 2026-08-30 | bae6e1b4 | fix(bgp): carry the definitions the last commit's files reference | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-30 | bae6e1b4 | fix(bgp): carry the definitions the last commit's files reference | discovery-index freshness | ran ./le discovery-index update over this tree and it produced no diff, so ai/PACKAGE-MAP.md is already current for these files | open |
| 2026-08-30 | bae6e1b4 | fix(bgp): carry the definitions the last commit's files reference | repository tracked-build/check (HEAD does not compile) | HEAD does not build: internal/component/bgp/cli and internal/component/bgp/reactor both fail to compile on undefined constants, caused by the file set of fc9c8bcaa | open |
| 2026-08-30 | bae6e1b4 | test(bgp): the raw rail's wiring test, and the send word that gates it | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-08-29T14:01:28Z) | open |
| 2026-08-30 | bae6e1b4 | test(bgp): the raw rail's wiring test, and the send word that gates it | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
