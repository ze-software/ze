---
kind: directive
level: MUST
stage:
---
**A FAILED commit leaves the index STAGED, and the next session's commit inherits
it. You MUST clear it before you walk away.** The script stages first and commits second, so
a commit that fails has already staged everything. On 2026-08-03 a GPG passphrase
prompt with no TTY failed the signing step, eleven files sat staged in the shared
index for roughly forty minutes, and a concurrent session's 1467-file commit took
ten of them. Nothing was lost and every file's content was intact, but the work
landed under another commit's message.
