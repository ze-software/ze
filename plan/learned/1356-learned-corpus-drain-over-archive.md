# 1356 -- Learned Corpus: Drain the Ceiling, Do Not Delete the Gate

## Context

`plan/learned/` holds 931 summaries and a staleness gate that fails when their
`## Files` paths or `plan/learned/NNN` citations stop resolving. The ceiling in
`plan/.learned-staleness-baseline` is a tax every session pays. A session
proposed ending it: drop the gate and let the corpus become an ungated
append-only archive, on the evidence that most summaries are cited nowhere. The
owner asked for a second opinion. The measurements below rejected the proposal,
and they are written here so the next session does not re-derive the same wrong
conclusion from the same data.

## Decisions

- Chose DRAINING the ceiling over deleting the gate: `plan/.learned-staleness-drain` carries a start date and a rate, ships at rate 0, and only Thomas arms it. The answer to "this tax is permanent" is a one-line rate, not a removal commit.
- Chose the shrink-only ratchet over an archive because the ratchet works: 1,856 dead references when the gate landed, 318 now. Its own header states the intent, "grandfather the known rot, refuse to let it grow".
- Rejected the ungated archive: removing the gate converts "uncited but retrievable" into "uncited and quietly wrong", and the uncited half is exactly the half nobody opens, so nobody sees its references rot.
- Rejected citation count as a relevance metric. It measures RETRIEVAL, not worth. A summary carrying a real constraint that nothing points at is a routing failure, and the fix is to route it.
- Chose a `rationale:` frontmatter key on a rule point over promoting summaries into `ai/rationale/`: the field names the record, the gate proves it resolves, and no prose is duplicated.

## Consequences

- The citation distribution reframes the corpus: `.claude/` cites it 1,247 times and `internal/` 1,180, against 13 citations from a rule or a point before this work and 15 after it. `plan/learned/` is the rationale layer for HOOKS and CODE, not for rules.
- 428 of the 931 summaries are cited outside `plan/learned/` and outside the two generated indexes, so 503 of them, 54%, are cited nowhere else.
- `ai/rationale/` cannot absorb it. It is 45 files and one per rule, so it structurally cannot hold what a hook comment needs to point at.
- Retiring the corpus retires the explanation behind 2,427 hook and code citations, which is the cost the "54% uncited" figure hides.
- The reasoning now lives in a rule point (`ai/rules/planning.md`, "The learned-staleness ceiling is drained"), so a future proposal meets a directive rather than an empty field.
- `ze-rules-gate-map` gained a failing set for a rationale naming a missing path, and a coverage measurement that never reds. Coverage over 1,589 instruction points can only be a measurement: a red would be a demand to invent 1,500 explanations.
- Ten of the fifteen points citing a numbered summary were linked. The other five were left unlinked because the citation sits in one row or item of a multi-topic point, so the summary explains a clause and not the instruction, and an invented link defeats what the coverage number measures.

## Gotchas

- The same query answers 100% or 46% depending on one generated file. Counting citations from outside `plan/learned/` and INCLUDING `ai/LEARNED-FULL-INDEX.md` and `ai/LEARNED-INDEX.md` gives 931 of 931 summaries cited. Excluding those two gives 428 of 931, so 46% cited and 54% not. A generated index cites everything, so any citation metric that does not exclude it reports total coverage of a corpus where fewer than half the files are reachable by any other route. The haystack effect reproduced itself inside the measurement of the haystack.
- The measurement of the corpus made the same class of error the corpus argument criticises. The first denominator was 934, which counted the `.md` files in the directory; four of them are not summaries, so the real figure was 930 at measurement and 931 now. The rejected proposal made the twin error, reading `du` block padding as content and calling 30 files 11 MB. Both are cheap, both look plausible, and both survive into durable prose because nobody re-derives a number that looks right. A figure quoted into a rule or a summary gets re-derived by the person writing it. A brief is not a source.
- "17 of 27 rules have a hook" is the same error in mirror image: it reads as two thirds until you count per instruction, where it is 3%. A ratio is evidence only once you know what its denominator counts.
- Allocating a number with `commit_helper.py learned-next` writes a 3-line stub immediately. A stub has no `## Files` section, which IS a staleness finding, so an allocated-and-unwritten summary reds the gate it belongs to.
- `format_point` writes `rationale:` only when it carries a path. An always-written empty line would have claimed every point was examined and found to need no record.

## Files

- `scripts/dev/rules_points.py` -- `POINT_KEYS`, `format_point`, `parse_point`, `rationale_problems`, `report_gate_map`
- `scripts/dev/rules_points_test.py` -- `RationaleTest`
- `scripts/dev/learned_staleness.py` -- `parse_drain_budget`, `drain_anchor`, `required_drain`, `check_drain`
- `scripts/dev/learned_staleness_test.py` -- `DrainPolicyTest`
- `plan/.learned-staleness-drain` -- the policy, inert at rate 0
- `plan/.learned-staleness-baseline` -- the shrink-only ceiling it drains
- `ai/rules/points/planning/writing-learned-summaries/the-staleness-ceiling-is-drained-never-removed.md`
- `ai/rules/points/planning/manifest.md`
- `ai/rules/planning.md`
