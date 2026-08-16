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
- **`scripts/dev/review_gate.py record` takes `--rounds N` and refuses more than
  three without `--rounds-reason`, which MUST name the PRODUCT defect a later
  round found.** The cap is not a ban: a genuinely defective implementation can
  need a fourth round and gets one for the cost of a sentence. That sentence is
  the one nobody can write when the loop is auditing its own bookkeeping, which is
  what makes it the right toll.
