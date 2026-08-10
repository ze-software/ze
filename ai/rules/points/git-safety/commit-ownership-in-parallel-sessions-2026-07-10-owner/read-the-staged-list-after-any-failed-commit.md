---
kind: directive
level: MUST
stage:
---
**The failure mode is invisible from the failed run.** It exits non-zero, prints
`failed to write commit object`, and reads as "nothing happened". The staging IS
what happened. After ANY failed commit in a shared checkout, you MUST read
`git diff --cached --name-only`, then either fix the cause and re-run at once or
unstage your own paths. A signing failure is the usual trigger precisely because it
fails LAST, after every gate has passed and every file is already staged.
