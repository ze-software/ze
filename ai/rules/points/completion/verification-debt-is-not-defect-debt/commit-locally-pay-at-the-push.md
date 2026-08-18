---
kind: directive
level: MUST NOT
stage:
---
**The override flags on `commit_helper.py create` are SELF-SERVICE. You MUST NOT stop
and ask Thomas before using one.** `--unverified`, `--structural-red-ok`,
`--missing-full-verify-ok`, `--stale-index-ok` and `--review-override` each admit one
unrun gate, and each writes its row. Give a truthful reason and proceed. Several
sessions share this checkout, so the shared verify record is red for somebody else's
in-flight work nearly always, and work that was finished but never landed is the most
expensive failure this repo has (`ai/rules/rule-precedence.md`).

**Enforcement is at the PUSH, which is where code reaches users: `create --push`
refuses while any row is open.** A commit that stays local costs nobody anything, so it
is not the place to hold the line. The next session reads the open rows at session
start, runs the gate, and sets Status to `cleared`.
