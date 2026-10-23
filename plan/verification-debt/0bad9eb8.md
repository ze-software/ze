# Verification debt -- commit session 0bad9eb8

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-10-22 | 0bad9eb8 | meta: M14, interop peers run latest stable upstream (+8 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-10-19T14:57:42Z) | open |
| 2026-10-22 | 0bad9eb8 | meta: M14, interop peers run latest stable upstream (+1 more) | native structural checks (red) | commit is one markdown row in ai/META-DECISIONS.md; arch tier/check and arch iface-resolution are red from other sessions' Go work and cannot be affected by it | open |
| 2026-10-22 | 0bad9eb8 | interop: GoBGP lab on v4.10.0, the latest stable (M14) | native structural checks (red) | arch tier/check and arch iface-resolution are red from other sessions' Go work; this commit changes test-lab code, comments and docs only | open |
| 2026-10-22 | 0bad9eb8 | interop: GoBGP lab on v4.10.0, the latest stable (M14) (+3 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-10-22 | 0bad9eb8 | interop: perf freeRtr DUT on temurin 27, go 1.27, alpine 3.24.3 (M14) (+2 more) | native structural checks (red) | le arch tier check and le arch iface-resolution print OK over the working tree; the gate reads a stale 2026-10-19 verify run | open |
| 2026-10-22 | 0bad9eb8 | interop: sr-policy malformed receive proves no session reset on v4 (+2 more) | native structural checks (red) | le arch tier check and le arch iface-resolution print OK over the working tree; the gate reads a stale verify run | open |
| 2026-10-23 | 0bad9eb8 | interop: build BIRD 3.4.0 on Debian, the Alpine build dies on import (+2 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=130, at 2026-10-22T23:16:01Z) | open |
| 2026-10-23 | 0bad9eb8 | interop: build BIRD 3.4.0 on Debian, the Alpine build dies on import (+2 more) | native structural checks (red) | owner authorised structural-red-ok on 2026-10-23 ("authorise structural-red-ok"); the charged stages print OK when run directly and the stale charge comes from interrupted verify records | open |
