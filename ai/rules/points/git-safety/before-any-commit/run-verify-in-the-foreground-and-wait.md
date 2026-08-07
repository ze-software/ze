---
kind: note
level:
stage:
---
Each directive below is one physical line on purpose. `condense_body`
(`scripts/dev/rules_condensed.py`) emits a bold-led LINE raw into
`ai/rules/CORE.md`, so an instruction that wraps arrives there cut in half.

**Run `make ze-verify` in the foreground, wait for it, and never poll: the foreground return IS the completion signal.**
No background run, no sleep-and-check loop, no `tail` on a log that is still
growing.

**Do not kill it for being slow. Give the call the largest timeout your harness allows.**
A verify that is still running is not a verify that is hung, and killing one
costs the whole pass rather than the seconds it saves.

**Never take a timeout from a duration written in a rule. The one measurement is `tmp/.ze-verify-duration.txt`.**
This corpus disagrees with itself about how long a full pass takes: "25 to 30
minutes" below, "4-10 minutes" in `ai/rules/testing.md`. Both were typed by
hand. `_record_duration` (`scripts/dev/verify-lock.sh`) appends the real elapsed
seconds to that file, and when it is absent nothing here has measured it
(`plan/learned/1359-rules-corpus-paraphrase-drift.md`).

**Never edit the tree while a verify runs, yours or anybody's: it reads the working tree.**
An edit mid-run invalidates the run you are waiting for.
