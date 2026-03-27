---
name: Never disable GPG signing
description: Never bypass GPG commit signing -- investigate failures instead of disabling signing
type: feedback
---

Never use `--no-gpg-sign`, `-c commit.gpgsign=false`, or any other flag/config to bypass GPG signing on commits.

**Why:** Claude has repeatedly disabled GPG signing when commits failed, resulting in unsigned commits in the repository history. The user expects all commits to be signed. Unsigned commits 50+ back in history cannot be fixed without rewriting all descendants.

**How to apply:** When a `git commit` fails due to GPG/signing issues, investigate the root cause (expired key, missing agent, wrong key configured). Report the error to the user and ask how to proceed. Never silently bypass signing.
