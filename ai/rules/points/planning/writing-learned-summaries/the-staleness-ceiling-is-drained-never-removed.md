---
kind: directive
level:
stage:
rationale: plan/learned/1356-learned-corpus-drain-over-archive.md
---
**The learned-staleness ceiling is drained, never removed and never widened.** A session that meets the number in `plan/.learned-staleness-baseline` will propose ending the tax: delete the gate, or turn `plan/learned/` into an ungated append-only archive. That proposal was made on 2026-08-07, measured, and rejected. The reasons are here so the next session does not re-derive the same wrong conclusion from the same data (`plan/learned/1356-learned-corpus-drain-over-archive.md`).
- **The ceiling is shrink-only by construction, and its own header states the intent: "grandfather the known rot, refuse to let it grow".** It went from 1,856 dead references to 318 that way. `write_baseline` (`scripts/dev/learned_staleness.py`) refuses a raise that carries no written reason.
- **When the tax reads as permanent, ARM THE DRAIN.** `plan/.learned-staleness-drain` carries a start date and a rate, ships at rate 0, and only Thomas arms it. It is the mechanism that answers "this never ends". Deleting the gate is not that answer, and neither is raising the ceiling.
- **Removing the gate from an uncited corpus converts "uncited but retrievable" into "uncited and quietly wrong".** Of 931 summaries, 428 are cited outside `plan/learned/` and outside the two generated indexes, so 503 of them, 54%, are cited nowhere else. That half is the half nobody opens, so nobody sees its references rot. The gate is the only reader it has.
- **Citation count measures RETRIEVAL, never worth.** A summary that carries a real constraint and that nothing points at is a routing failure, and the fix is to route it. The same error in mirror image made "17 of 27 rules carry a hook" read as two-thirds coverage, when the real figure was 3% per instruction.
- **`plan/learned/` is the rationale layer for hooks and code, not for rules.** Measured 2026-08-07: `.claude/` cites it 1,247 times and `internal/` 1,180 times, against 13 citations from a rule or a point, now 15 because this work added the `rationale:` links. `ai/rationale/` is per-rule and holds 45 files, so it cannot carry what a hook comment needs. Retiring the corpus retires the explanation behind 2,427 hook and code citations.
