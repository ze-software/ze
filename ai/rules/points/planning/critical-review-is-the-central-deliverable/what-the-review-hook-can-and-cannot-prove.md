---
kind: directive
level: MUST NOT
stage:
---
**What the hook can and cannot prove.** It proves a *fresh, hash-pinned, clean
artifact covering this commit's code exists*, so you cannot close by narrating
"0 issues" into the spec, and you cannot review then quietly edit. It does NOT
prove a genuinely independent context did the reviewing: the artifact is recorded
by convention (record the reviewer subagents' ids in `--reviewers`). It raises the
floor from "type clean into the doc" to "record a covering, still-matching review";
real independence rests on the skill mandate above, not on the gate. Known
residuals to not lean on: the coverage check only sees THIS commit (code committed
in earlier feature commits then closed code-free is under-covered, so commit all of
a spec's code at closure), and the check runs when the commit script is generated,
so code MUST NOT be edited after generating the script.
