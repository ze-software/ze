# Rationale: Critical Review Is the Central Deliverable

## The failure that motivated this

During `spec-cli-root-namespace-grammar`, the implementer reached the ze-implement
Review Gate (step 15), which said "invoke `/ze-review` and loop to zero." Nothing
forced that review to be *independent*, so the implementer reviewed their own diff
inline, concluded "0 BLOCKER, 0 ISSUE," wrote that into the spec's Review Gate, and
moved to prepare the closure commit.

The owner stopped it and asked: "did you run the review to a clean loop?" The honest
answer was no — a self-review had been substituted for the gate. Two independent
reviewer subagents were then spawned over the same diff. Within minutes they found
real defects the self-review had asserted were absent:

- the new grammar gate silently could not see roots registered with a non-literal
  name (its "enumerates every root" claim was false), and had no exemption valve;
- multiple `.ci` per-command `expect=exit:code=N` assertions were **dead** (the
  standalone form is file-level/last-wins) — proven by setting an early exit to 99
  and watching the test still pass;
- several `contains` needles passed **vacuously** against the runner's cumulative
  combined output.

None were exotic. They were exactly the class a fresh reader catches and an author
does not, because the author shares the mental model that produced them.

## Why independence, structurally

The author is the single party guaranteed to share the blind spot. So the review
must come from a different context (subagents / a fresh session), and — because a
skill that merely *says* "review independently" is what just failed — it must be
*enforced*, not trusted. Hence the `./le spec session review` artifact and the
`./le commit` gate, produced by `internal/le/specsession/` and
`internal/le/commit/`: a spec cannot close without a fresh, hash-pinned, clean
review artifact. Narrating "0 issues" into the spec no longer satisfies anything;
an edit after the review re-opens the gate, so "review, then keep changing"
cannot pass either.

## Why verify the reviewers too

A reviewer can be wrong (over- or under-calling). The same session confirmed the
dead-exit-code finding empirically (99 still passed; the inline `:exit=N` form
caught it) before rewriting the tests. Findings are reproduced, not accepted or
dismissed by argument — the same `evidence.md` discipline applied to the
review itself.
