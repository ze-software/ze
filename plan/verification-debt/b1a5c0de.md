# Verification debt -- commit session b1a5c0de

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-08 | b1a5c0de | stress-repro: build ze from the tree, refuse an all-failed verdict (+109 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-01T12:40:37Z) | open |
| 2026-10-08 | b1a5c0de | stress-repro: build ze from the tree, refuse an all-failed verdict (+60 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-08 | b1a5c0de | plugin/server: refuse a second holder of a command name or wire method (+1 more) | owner approval for an RFC-tagged test change | RFC 8907 tagged tests changed only mechanically: each Dispatcher.Register call now checks the error the new refusal returns; no assertion or claim changed. Owner approval owed. | open |
| 2026-10-08 | b1a5c0de | rib: bestchange test drops the deleted eBGP distance constant | owner approval for an RFC-tagged test change | mechanical: a deleted constant replaced by its literal value 20; no assertion or claim changed | open |
| 2026-10-08 | b1a5c0de | rsvpte: send ADSPEC in PathErr and FLOWSPEC in ResvTear | owner approval for an RFC-tagged test change | mechanical call-site change only: buildPathErr gained an adspec parameter and these tests pass nil, so the message each builds is byte-identical and no assertion changed | open |
