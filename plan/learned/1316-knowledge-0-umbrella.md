# 1316 -- knowledge-0-umbrella

## Context

Ze's recorded knowledge had grown for five months with no retirement path. Measured
at the start: 1,286 numbered summaries (7.2 MB), 99 rules, and **zero of either ever
deleted**. 24% of the file paths those summaries cited no longer existed. Only 14%
of summaries were cited by any rule, skill, doc, spec or script. Meanwhile
`ai/rules/CONDENSED.md` put about 100,000 tokens into every session and every
subagent, whether or not any of it applied.

The question that started this was whether guidance written for older Claude models
still applied. It did: only 11 items across the whole corpus were genuinely
model-era. The real problem was that nothing was ever removed, and the cost was
paid per session rather than per relevance.

## Decisions

- **Retire by AGE BAND, not by citation count.** Decay is an age function: band
  1-200 was 78% dead paths, 201-400 was 44%, above 1000 was 5%. 46% of the rot sat
  in 24% of the files. Pruning "everything uncited" would have destroyed the 53% of
  the corpus that is durable knowledge; pruning a band did not.
- **Consolidate into `plan/learned/DESIGN-HISTORY.md`, not `ai/digests/`.** The
  digests README states the partition ("the historical record lives in
  `plan/learned/`") and `digest_check.py` requires every anchor to resolve TODAY,
  so history about deleted code cannot be anchored there at all.
- **Repair dead paths rather than amend the acceptance criterion.** The deferral
  called it "a much larger, separate piece of work". Wrong: the rot was code that
  MOVED and git had recorded where. 662 of 1,008 resolved from rename history.
- **Route rules by their `When` trigger, keeping awareness rather than inclusion.**
  Every rule stays NAMED in every session through `TRIGGERS.md`, and only bodies
  load on demand. Keeping all blocking rules eager was measured and caps the saving
  at 28%, so routing them is what makes 80% possible.
- **Derive the always-on core from `rule-precedence.md` rungs 1 and 2**, never a
  filename list. A copied list reads identically today and rots at the next rule.

## Consequences

- Per-session rule payload is 21,356 tokens, down from 107,548. `CLAUDE.md` is
  24 KB where it was about 423 KB.
- `make ze-learned-staleness` fails on decay regrowth, with a shrink-only ceiling
  that can only be raised through `--raise-baseline` with a reason recorded in the
  file.
- A learned summary is now written only when the work produced a lesson.
  `lesson_worthy` reads the diff, not the directory.
- **A-4 is unvalidated by construction.** Whether a model loads a rule when its
  trigger matches can only be measured AFTER the switch, because a session
  consulting an eagerly-loaded digest leaves no trace. The Stop-hook detector is
  the ongoing signal, and the switch is a one-line revert.

## Gotchas

- **A gate can assert its own bug.** `test_core_is_derived_not_hardcoded` asserted
  the fail-open behaviour of the ladder parser, so a header reword would silently
  have dropped `git-safety`, `never-destroy-work`, `rfc-compliance` and
  `interop-and-goal-validation` from the always-on core, forever green.
- **A shrink-only baseline with slack is worse than none.** A ceiling recorded at
  1,011 while the tree carried 341 left 670 references of room to regrow, while
  looking armed. The test that caught it demands the ceiling EQUAL the count.
- **`tempfile.mkstemp` creates at 0600.** 228 summaries were left with narrowed
  permissions, invisible in `git diff` because git tracks only the exec bit.
- **A generated file must not embed a count derived from a directory it does not
  own.** `CORE.md` carried a `plan/` corpus size, so every future spec closure
  reddened a structural gate on a file it never touched.
- **Three defects were found in these specs by IMPLEMENTING them**: a checker path
  that did not exist (a test written there would have run nowhere), an acceptance
  criterion requiring a token its target file never contained, and one that never
  named its measuring instrument, so two defensible readings straddle its
  threshold.
- Retirement governs records of COMPLETED WORK. `ai/rules/no-parking.md` and
  `ai/rules/fix-dont-record.md` govern DEFECT records, and nothing here licenses
  pruning a `plan/known-failures/` shard. The two directions look alike and are
  opposite.

## Files

- `scripts/dev/learned_staleness.py` - the decay gate and its shrink-only ratchet
- `scripts/dev/learned_repath.py` - resolves moved paths from git rename history
- `scripts/dev/learned_normalise.py` - section-format normalisation
- `scripts/dev/learned_retire.py` - band-limited retirement, ceiling 400
- `scripts/dev/rules_condensed.py` - emits `TRIGGERS.md` and `CORE.md`
- `scripts/dev/rules_router.py` - trigger coverage report
- `scripts/dev/rule_coverage.py` - the Stop-hook miss-detector
- `scripts/dev/check_doc_links.py` - pointer budget and dead-name lint
- `scripts/dev/commit_helper.py` - content-driven `lesson_worthy`
- `ai/INSTRUCTIONS.md` - imports `TRIGGERS.md` and `CORE.md`
- `plan/learned/DESIGN-HISTORY.md` - absorbed 263 items from the retired band
