# Precommit-verify rationale

Why: `ai/rules/precommit-verify.md`

## Land first, then verify (2026-10-26)

Measured over the subagent transcripts of 2026-10-25 and 2026-10-26: 186 agents
ran more than 500 commands that built or used a private export of HEAD to
pre-test a commit set, while `./le verify changed` ran 12 times. Agents whose
brief was only to continue, finish or commit already-written work took 16% of
the subagent spend; one spent 95 calls committing a change that was written and
tested, most of them rebuilding a 521 MB export, re-running lint and tests in
it, and proving that a red it did not cause pre-existed. A subagent's prompt
cache lives five minutes, so the long check belongs to the main thread, whose
cache outlives it; the subagent's part is the commit and `./le repo compiles
check`.

## History moved from rule points (2026-09-26)

- `ai/rules/points/precommit-verify/directives/the-gate-runs-on-a-commit-not-the-working-tree.md`: "(owner directive, 2026-08-21)"
