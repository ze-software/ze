# 1314 -- A Rule Heading Inverted Its Own Directive

## Context

Two sessions asked Thomas to choose between full RFC compliance and a narrower option,
when conformance was the only right answer. They were not working without a rule. They
followed one that told them to ask: the section was headed "Ask Thomas Whenever Full
Compliance Is On The Table", making full compliance the TRIGGER for a question. Its
first sentence fused a declarative to an imperative ("that is the answer -- and you are
NOT authorized to choose anything narrower on your own. STOP and ask Thomas."), and the
qualifier that governs it came one paragraph later.

## Decisions

- Fixed the RULE, not the two sessions. They read the text accurately, so a correction
  aimed at them would not survive a fresh context.
- Renamed the section to "Implement Full Compliance. Ask Thomas Only Before Doing LESS",
  and named the wrong reading explicitly per `ai/rules/rule-format.md`, over a third example.
- Corrected all six other sites pointing at "ask". They differed in kind:
  `ai/INSTRUCTIONS.md` had the same fusion, reworded with the clauses reversed;
  `completion.md` and `rule-precedence.md` rung 2 had the inversion without the fusion;
  `ai/skills/ze-rfc.md` stated the CORRECT reading but cited the dead heading.
- Swept the four `plan/` heading citations, and the LINE citations too: the new table row
  and the "Two readings" paragraph shifted the file by three lines, staling 14 references
  across 9 `plan/` files and two in `scripts/`.

## Consequences

A session reaching full conformance now implements and proves it without asking. The
question is owed only before choosing something narrower, and then it is "which way do
I fix it".

## Gotchas

- **A heading is a directive**, and reaches `CONDENSED.md` as a section title. One
  naming the exception inverts the rule for anyone who stops there. Ordering matters
  equally: the correct qualifier existed, one paragraph too late.
- **The digest hides where a wrong reading survives.** Two `rule-precedence.md` sites
  still routed to "ask" after the first pass, in `## Rationale` and `## Examples`, which
  `scripts/dev/rules_condensed.py` drops. Grep the rule FILES, never `CONDENSED.md`.
- **Editing a rule breaks line citations far outside `ai/` and `plan/`.** Two lived in
  `scripts/dev/rfc_requirements.py` docstrings and one in `rfc/drain-budget.txt`. Verify
  by RESOLVING every `<rule>.md:<N>` reference against the file, not by grepping the old
  number: that also catches ranges already stale before you arrived.
- **`rules_condensed.py` has no argparse.** `main()` only looks for `--check`, so any
  other flag silently takes the WRITE path and regenerates the digest from the WORKING
  TREE, publishing a concurrent session's unlanded rule edits. Regenerate from a
  `git archive HEAD` view plus your own edits (`ai/rules/rule-format.md`), and never
  probe a generator for usage.
- **`learned-next` hands out a number another session claimed IN CODE.** It globs
  `plan/learned/`, so a spec that wrote `// Design: plan/learned/NNNN-<slug>.md` before
  writing the summary is invisible to it. This session was given 1313, which live IKE
  files cited, and took 1314. `make ze-doc-links` surfaces it, but reports THREE citers
  where grep finds four: `_test.go` is exempt (`ai/rules/go-standards.md`).
  Same hazard as [1155], one tree instead of two branches.

## Files

`ai/rules/rfc-compliance.md`, `ai/rules/completion.md`, `ai/rules/rule-precedence.md`,
`ai/INSTRUCTIONS.md`, `ai/skills/ze-rfc.md`, plus citation fixes in 11 `plan/` files and
`rfc/drain-budget.txt`.
