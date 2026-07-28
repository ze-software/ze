# 1287 -- Rule routing, severity honesty, and a WIP cap

## Context

A review of the agent system (94 rules, a 4800-line eager digest, 30 skills, 43
write/edit checks) found the enforcement layer strong and the *routing* layer
rotted. `**When:**` is the field an agent matches against its task before
deciding whether to open a rule, and 69 of 94 could not do that job: 62 stated a
directive ("All CLI commands MUST follow these patterns") that matches every
task and therefore routes nothing, and 8 were sentences copied out of a wrapped
bold body line and cut mid-clause, shipped verbatim into `CONDENSED.md` where
every session reads them. Separately, nine rules declared `Severity: advisory`
while their own prose said BLOCKING, teaching readers the field is decoration.

The system also had no answer to a second question: 173 specs were open, 28
in-progress, the oldest untouched for 40 days. Every rule governs how well ONE
spec is executed; none limited how many are open at once.

## Decisions

- **Gate the trigger's SHAPE, not just its presence.** `rules_lint.py` now
  requires `**When:**` to open with a temporal opener or a gerund, and rejects a
  dangling tail (`,`), a dangling last word (`...enforced by`), or an unbalanced
  `**`. Chosen over a looser "must be non-empty" check because the failure mode
  was never emptiness: it was a syntactically perfect line that routed nothing.
- **A closed opener set over free phrasing.** The value of a uniform routing
  column is that it can be scanned; every extra shape is one more thing to
  recognize. Reference rules (`hook-mapping`, `project-knowledge`) get a trigger
  too, phrased as the moment you reach for them ("looking up which check
  enforces a rule").
- **Severity must agree with the prose**, with a LINE-scoped
  `<!-- severity-note: ... -->` escape rather than a file-scoped one. A
  file-scoped opt-out silently covers every later addition to that file. Exactly
  one line in the tree needs it (`hook-mapping.md` describing the LSP gate).
- **Cap WIP at the `ready` -> `in-progress` transition**, not at claim time
  generally. Resuming an in-progress spec adds nothing, and claiming a skeleton
  for research starts no implementation. Blocking (exit 3) over warning, because
  a warning printed to a script's stdout is invisible to an agent that runs the
  command and moves on.
- **Split `/ze-implement` at the closure seam** into `/ze-implement` (steps
  1-10) and `/ze-close` (the old 11-16). Two independent reasons, either
  sufficient: closure instructions reached at the tail of a 16-step skill get
  partially followed (the same evidence that motivated
  `plan/TEMPLATE-CLOSURE.md`), and `ai/rules/model-selection.md` puts
  implementation on Opus 4.8 and the Review Gate / spec closure / implementation
  audit on Opus 5, so one skill spanning both forced a silent model-boundary
  crossing.
- **Scope the bash pipe check to the producers its rule names.** It blocked
  every `| tail` while letting `go test ./... | head -50` through. Fixing the
  false positive and closing the hole are the same change.
- **Add `ai/rules/rule-precedence.md`** rather than editing the four rules that
  collide. `no-asking`, `model-selection`, `spec-delegation` and
  `rfc-compliance` are each correct; what was missing was an ordering.

## Consequences

- `INDEX.md` gained a Severity column and its "When to read" column is now a
  real routing table. Two generator bugs were fixed on the way: the metadata
  block was parsed as a paragraph (so every row read "<trigger> Severity:
  blocking Related: ..."), and a blanket `**` strip rendered `test/**/*.ci` as
  `test//*.ci`.
- Severity moved from 60 blocking / 35 advisory to 70 / 26. Nothing became more
  binding; the metadata caught up with prose that already said BLOCKING.
- `scripts/dev/spec-session.sh wip` is the first view of work in flight.
  `ZE_SPEC_WIP_CAP` defaults to 12 against 28 currently in-progress, so it bites
  immediately. That is the intent, and the number is the owner's to set.
- A spec now needs TWO skills to finish. `/ze-implement` step 11 hands off; it
  must not append `TEMPLATE-CLOSURE.md`, run the Review Gate, or commit.

## Gotchas

- **The digest keeps only the FIRST SENTENCE of the first prose paragraph of a
  section.** The new rule's opening paragraph had to be rewritten so its first
  sentence carried the meaning; the original truncated to "Ze has 94 rules."
  Always read your own section in the regenerated `CONDENSED.md`.
- **A trigger gate catches its author.** The first draft of
  `rule-precedence.md` ended "...stop, ask, delegate, or carry on" and was
  rejected for the dangling `on`. Rewording beat weakening the check.
- **`learned_index.py` derives its root from `__file__`'s parents[2]**, so a
  concurrent session's uncommitted learned summary lands in any regeneration you
  run. Regenerating from a `git archive HEAD` scratch tree plus your own files
  is the only way to produce the index CI will produce (the same technique
  `ai/rules/rule-format.md` prescribes for `CONDENSED.md`).
- The verify status was STALE from a run predating this work, with reds in
  `test/plugin/rib-inject-rfc5549.ci` and `test/web/commit-flow.wb`. This change
  set contains no `.go`, `.yang`, or test fixture, so it cannot reach either.

## Files

- `scripts/dev/rules_lint.py` -- trigger-shape and severity-agreement checks
- `scripts/dev/rules_index.py` -- metadata block parsed properly, Severity column, code-span-safe bold strip
- `scripts/dev/spec-session.sh` -- `wip` subcommand, WIP cap on the ready transition
- `.claude/hooks/pretool-bash.py` -- pipe check scoped to expensive producers
- `scripts/dev/hook-parity-check.py` -- 4 new golden cases for the pipe scope
- `ai/rules/rule-precedence.md` -- new
- `ai/rules/rule-format.md` -- "The trigger is a routing key"
- `ai/skills/ze-close.md` -- new; `ai/skills/ze-implement.md` trimmed to steps 1-10
- 68 `ai/rules/*.md` trigger and severity lines
