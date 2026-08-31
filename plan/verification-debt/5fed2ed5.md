# Verification debt -- commit session 5fed2ed5

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-31 | 5fed2ed5 | rfc: sign off fourteen Supported RFCs against their own text | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | rfc: sign off fourteen Supported RFCs against their own text | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | fix(rfc): nine RFC 8092 citations naming the wrong section | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | fix(rfc): nine RFC 8092 citations naming the wrong section | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | fix(radius): the RFC conformance defects three extraction walks found | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | fix(radius): the RFC conformance defects three extraction walks found | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | fix(radius): the RFC conformance defects three extraction walks found | discovery-index freshness | ai/PACKAGE-MAP.md is already current: ./le discovery-index update was run against this tree and rewrote it with no diff, so there is nothing for this commit to carry. No package was added, removed or moved; the change is behaviour inside existing packages plus new test files in packages the map already lists. | open |
| 2026-08-31 | 5fed2ed5 | feat(bgp): prefer the shorter CLUSTER_LIST, per RFC 4456 Section 9 | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | feat(bgp): prefer the shorter CLUSTER_LIST, per RFC 4456 Section 9 | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | feat(bgp): prefer the shorter CLUSTER_LIST, per RFC 4456 Section 9 | discovery-index freshness | ai/PACKAGE-MAP.md is current: ./le discovery-index update rewrote it with no diff. No package added, removed or moved. | open |
| 2026-08-31 | 5fed2ed5 | feat(ipsec): PAD sub-tree matching and operator-ordered SPD entries | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | feat(ipsec): PAD sub-tree matching and operator-ordered SPD entries | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | feat(ipsec): PAD sub-tree matching and operator-ordered SPD entries | discovery-index freshness | ai/PACKAGE-MAP.md is current: ./le discovery-index update rewrote it with no diff. No package added, removed or moved. | open |
| 2026-08-31 | 5fed2ed5 | probe | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | probe | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | feat(rfc): refuse a Supported claim no extraction sign-off bounds | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | 5fed2ed5 | feat(rfc): refuse a Supported claim no extraction sign-off bounds | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | 5fed2ed5 | feat(rfc): refuse a Supported claim no extraction sign-off bounds | discovery-index freshness | ai/RFC-REQUIREMENTS.md and the rfc/requirements shards are regenerated from every session's summaries at once, so committing them here would publish four sessions' requirement rows under this message. They settle after the concurrent sessions land. | open |
