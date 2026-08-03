# 1223 -- RFC gate regression ratchets: proof is monotonic, and a new RFC brings its own checking

## Context

`plan/spec-rfc-gate-regression-ratchets.md`. `make ze-rfc-check`
(`scripts/dev/rfc_requirements.py`) judged coverage from the WORKING TREE alone.
A tree cannot tell "never proven" from "stopped being proven": the first is the
backlog, the second is a regression, and they need opposite answers. Three holes
followed. A requirement proven by a positive and a negative test could be demoted
to `{gap}` and the gate stayed green. A newly added `rfc/short/*.md` was
un-enrolled by definition, so adding an RFC added exactly no checking. And the
edit-time guard `_rfc_tagged_change_err` (`.claude/hooks/pretool-writeedit.py`)
only inspected the edit HUNK, so editing the body of a tagged test with the tag
one line above escaped the one check written to stop that.

## What was built

- `check_coverage_ratchet`: the polarity SET per requirement id cannot shrink
  versus HEAD. Keying on the SET (not file:line, not a count) makes renames,
  moves and function splits invisible, so it fires only on real loss.
- `check_retired_requirements`: a requirement id of an enrolled RFC cannot vanish
  from its summary. NOT in the original design -- see lesson 2.
- `check_new_summaries`: a summary new since HEAD with gated MUSTs must be
  enrolled, must parse, and must not capture zero requirements while
  `rfc/full/<stem>.txt` shows MUST-level keywords.
- `_git_baseline_tag_polarities` / `_git_baseline_summary_stems` /
  `_git_cat_blobs` / `_scan_tags_tolerant`: the HEAD side, re-parsed with the
  SAME `scan_go_tags`/`scan_ci_tags` the tree goes through.
- `_go_func_scopes` + `_doc_comment_start`: the hook's tag search widened from
  the hunk to the enclosing test function, plus a tag-REMOVAL check.

## Lessons

1. **A ratchet must be checked in the direction it is consumed, not the direction
   it is written.** The three git baseline readers all "degrade to an empty set on
   failure", copied from `_git_baseline_enrolment`. For two of them that is
   fail-open-and-harmless: they are consumed as `baseline - current`, where an
   empty baseline accuses nobody. `check_new_summaries` consumes its baseline as
   `stems - baseline_stems`, where an empty baseline accuses EVERY summary in the
   repository of being new. Same idiom, same words in the docstring, opposite
   polarity -- and the docstring asserted the harmless reading. Verified on the
   real tree: one spurious violation naming a file committed years ago as "new".
   Fixed by returning `Optional[Set[str]]` and no-opping on a falsy baseline.
   **Before copying a degradation convention, check how the VALUE is consumed.**

2. **A ratchet creates an incentive; check where it points.** Making `{gap}` cost
   a red gate plus a public `rfc-status.md` disclosure row, while deleting the
   checklist line cost nothing, pointed straight at deleting the line.
   `check_coverage_ratchet` iterates the CURRENT requirements, so a deleted
   requirement is never visited and its lost tests are never noticed. The cheapest
   route from red to green was to hide the obligation. A ratchet that makes
   declaring a gap more expensive than concealing one is worse than no ratchet.

3. **A guard's scope has two boundaries and both bite.** Widening the hook's tag
   search from the hunk to "the enclosing func, ending at the next `func`
   keyword" swallowed the NEXT function's doc comment, where tags live -- so every
   function that merely preceded a tagged test became "tagged". Measured: **331 of
   3220** untagged functions falsely blocked. Ending at the next func's DOC COMMENT
   fixed that but made spans contiguous, which silently re-homed any tag in the gap
   between one func's closing brace and the next func's doc comment (a
   blank-line-separated tag, a hoisted table) onto the PRECEDING function -- and
   made the docstring's "a tag outside every scope widens to the whole file" claim
   unreachable, because no gap existed to fall into. Final: spans run from the doc
   comment to the func's OWN closing brace (column 0, gofmt's guarantee), capped at
   the next doc comment for one-line funcs.

4. **A false safety claim in a docstring is a defect, not a typo.** Lesson 3's
   middle state had correct-looking code, a passing fixture named for the property,
   and a docstring asserting a fallback that could never fire. The fixture only
   exercised the one sub-case that worked (a table before the FIRST func). The next
   author would have built on the claim. `ai/rules/evidence.md` applies to
   comments about your own code, not only to claims about someone else's.

5. **Fixture ORDER can hide the bug the fixture is named for.** The G1 fixture file
   put the tagged function first, so the boundary bug was invisible; reversing the
   order (untagged helper first, directly above the tagged test's doc comment) makes
   the same fixture fail. A reviewer proved this by stubbing the boundary back.
   **When a fixture asserts "X is not affected", place X where the bug would reach it.**

6. **Mutation testing found what two adversarial review rounds did not.** After two
   independent review rounds and their fixes, reverting each fix in a scratch copy
   showed 5 of 15 gate mutants and 2 of 8 hook mutants SURVIVING -- fixes nothing
   pinned. Worse, it exposed a wiring test that PASSED FOR THE WRONG REASON: tags
   left in the fixture for a deleted requirement id made `evaluate` emit "unknown
   RFC requirement: RFC7606-2-2", the same id and the same exit code as the check
   under test, which was never called. Both assertions passed while the code path
   was dead. **`assertIn(<id>, out)` is not enough when two producers can emit the
   same id: assert the message TEXT.** Final: 14/15 and 8/8.

7. **Reusing a production scanner for a baseline needs a tolerant variant.** The
   scanners raise on the first malformed tag, discarding the whole file's tags.
   Right for the tree (fail closed); backwards for the baseline, because the commit
   that FIXES a malformed tag is exactly when the tree parses and HEAD does not --
   so the file loses its baseline in the one change most likely to touch those
   tests. Go gets a per-line fallback; `.ci` deliberately does not, because its
   `terminator=` blocks make line position meaningful and a `//`-matching fallback
   would invent phantom tags.

8. **Measure before believing a performance panic.** A per-file `git show` baseline
   looked like a 47x slowdown (3.36s vs "0.07s at HEAD"). The 0.07s came from a copy
   of the gate placed in `tmp/`, where it resolved `PROJECT_DIR` one directory too
   high, exited immediately with "cannot run", and timed nothing. Real HEAD: 1.73s.
   The batch `git cat-file --batch` rewrite was worth keeping anyway (2.22s, +30%
   instead of +94%), but the number that justified it was invalid until redone.
   **A tool that resolves paths from its own location cannot be timed from a copy
   somewhere else.**

## Consequences

- `make ze-rfc-check` now costs +0.5s (1.73s -> 2.22s) and runs in both verify
  branches (`scripts/status/verify_run.go` `stagesForMode`).
- Enrolling an RFC is now mandatory in the change that adds its summary. The
  pre-existing backlog (9 un-enrolled summaries, all present at HEAD) is
  grandfathered deliberately: a rule that reds the gate on unrelated work gets
  removed rather than obeyed.
- Still NOT covered: a tagged test weakened IN PLACE. That is `c_test_weakening`
  and `scripts/dev/audit-test-relaxation.py`, plus the SHA ratchet
  (`check_audit_freshness`) wherever `/ze-rfc-audit` recorded a verdict -- which is
  1 of 165 enrolled RFCs. Generating the other 164 baselines was considered and
  rejected: it would assert an audit that never happened.
- The ratchet's baseline is HEAD, so coverage degraded and committed anyway
  becomes the new baseline. It slows decay; it does not reverse it.

## Files

None recorded.
