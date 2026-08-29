---
kind: directive
level: MUST
stage:
---
- **The review's subject is the PRODUCT. A false statement in the spec's own
  closure record is a NOTE, and a NOTE MUST NOT re-open a round.** Wrong arithmetic
  in an Audit Summary, a pasted command output that was condensed, a status word
  that contradicts the shard, a count nobody can reproduce: each is worth fixing
  and none of them ships. Collect every one of them, fix them in ONE edit, and
  MUST NOT spend a round confirming the fix.
- **The one exception is precise: a record defect is an ISSUE when it asserts a
  PRODUCT property that is false.** "This test discriminates" when it does not,
  "the guard fails closed" when it does not, "an interop test covers this" when
  none exists. Those are `ai/rules/evidence.md` false-safety-claim findings, they
  mislead the next reader about the code, and they keep their severity.
- **A round whose findings are ALL record defects is the last round.** The loop
  has stopped converging on the product: each prose fix creates fresh text to
  audit, so another round cannot establish product quality.
- **`./le spec session review record` takes `--rounds N` and refuses more than
  five without `--rounds-reason`, which MUST name the PRODUCT defect a later
  round found.** The cap is not a ban: a genuinely defective implementation can
  need a sixth round and gets one for the cost of a sentence. That sentence is
  the one nobody can write when the loop is auditing its own bookkeeping, which is
  what makes it the right toll.
- **Past FIVE rounds a session MUST NOT authorise itself. MORE THAN FIVE PASSES
  IS THOMAS'S DECISION** (owner ruling, 2026-08-17). `record` refuses a sixth
  round without `--owner-authorised` carrying what he said. `--rounds-reason`
  stays required alongside it. The product defect and his word are both owed,
  and neither substitutes for the other.
- **You MUST NOT set `--owner-authorised` on your own initiative.** The same ban
  covers `--push` on `internal/le/commit` (`ai/rules/git-safety.md`).
  At the cap you MUST stop. Report what the loop keeps finding, then ask him
  whether it runs another pass. A script cannot check who typed a flag. Setting
  it unasked is a recorded false statement about the owner, not a shortcut.
