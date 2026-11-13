---
kind: directive
level: MUST
stage:
---
**The implementation phase MUST write the discrimination record of every RFC-tagged unit it adds or whose behavior or claim it changes, MUST re-judge every audit verdict its change stales, and MUST land both in the same commit as the code they prove.** `ai/rules/rfc-compliance.md` owns the obligation and `/ze-implement` step 5 carries the commands. The agent lists the exact `./le rfc record` and `./le rfc check` lines as owed commands, and the main thread runs them and makes that commit.
**Closure MUST only check the records and verdicts with `./le rfc check`, and MUST NOT author them.** The closing agent lists that check as an owed command. A record stale or missing at closure is a defect of the implementation phase: it gets a Mistake Log row and goes back to `/ze-implement`.
