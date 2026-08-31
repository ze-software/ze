# Verification debt -- commit session af1dcb16

Gates that had not run green over these commits when they were made.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-08-31 | af1dcb16 | plan,bgp: close the End-of-RIB barrier spec and its stale pages | full native verification (not FRESH-green) | verify-status has no certificate in this checkout and a full native verification does not fit this session's foreground budget; the scoped evidence is in the message | open |
| 2026-08-31 | af1dcb16 | plan,bgp: close the End-of-RIB barrier spec and its stale pages | full native verification over this commit's Go | the same reason; the debt row records it, and no push is ordered | open |
| 2026-08-31 | af1dcb16 | plan: close spec-fixit-peer-pending-sync-settles-too-early | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): run the two checkers that never left their guard | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): run the two checkers that never left their guard | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the three FRR-only checkers claim | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the three FRR-only checkers claim | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the three relay-shape checkers claim | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the three relay-shape checkers claim | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the two-observer checkers claim | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the two-observer checkers claim | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the GoBGP and strict-speaker checkers claim | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): assert what the GoBGP and strict-speaker checkers claim | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | af1dcb16 | fix(interop): wire the relay in five scenarios that assert on it | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): match FRR's own spelling of a neighbor in a decode line | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | fix(interop): match FRR's own spelling of a neighbor in a decode line | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | af1dcb16 | le: --name builds a session's own binary and refuses a wrong one | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | le: --name builds a session's own binary and refuses a wrong one | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-08-31 | af1dcb16 | plan,docs: close the restored bespoke interop checkers and their page | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | plan: close spec-restore-bespoke-interop-assertions | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
| 2026-08-31 | af1dcb16 | spec(rfc): gate whether an RFC tag's claim is what its test proves | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: no status file (never verified) | open |
