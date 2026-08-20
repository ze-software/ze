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
start and runs `make ze-verify-debt-clear`, which re-runs each owed gate ONCE per pass
and writes `cleared` only where that gate exits 0. A row is never marked by hand: the
ledger records what a gate did, never what a reader believed.

**Two kinds of row the pass leaves open, and neither is a failure of it.** A row whose
gate is `independent critical review` names a human judgement with no gate to run, so
the pass reports it UNRUNNABLE and `/ze-review` answers it. So does a row whose gate
string no gate is registered for, which is what an older wording leaves behind.
