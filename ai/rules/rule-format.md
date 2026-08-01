# Rule File Format

**When:** authoring or editing any `ai/rules/*.md` rule file
**Severity:** blocking
**Related:** canonical-sources, discovery-updates

## Directives

Every `ai/rules/*.md` rule MUST open with a title and a machine-readable
metadata block, so tooling can parse triggers and severity without guessing. An
ALL-CAPS stem is a generated artifact, never a rule: `INDEX.md`, `TRIGGERS.md`,
`CORE.md`, `CONDENSED.md`. The generators skip it by that shape, so a new
artifact needs no code change.

Required structure, in this exact order:

| Element | Requirement |
|---------|-------------|
| `# Title` | First non-blank line, a single H1. It MUST NOT contain "BLOCKING": `**Severity:**` carries that, and no tool can read a title marker. |
| `**When:** <trigger>` | Required. One line. The SITUATION that makes this rule apply, phrased so an agent can match it against the task at hand. See "The trigger is a routing key". |
| `**Severity:** blocking\|advisory` | Required. `blocking` = a gate/hook enforces it or violating it breaks correctness; `advisory` = strong convention. It MUST agree with the prose: a rule whose body says BLOCKING may not declare `advisory`. |
| `**Related:** slug, slug` | Optional. Comma-separated rule slugs (filename without `.md`), no paths. |

## The trigger is a routing key

`**When:**` is not a summary and not the rule's first directive. It is the only
field an agent matches against the task in hand before deciding whether to open
the file, so it MUST name a situation and nothing else.

| Requirement | Why |
|-------------|-----|
| Start with a temporal opener (`when`, `whenever`, `before`, `after`, `while`, `during`, `if`, `once`, `unless`, `upon`, `on`, `at`, `any/every/each time`, `prior to`, `as soon as`) or a gerund (`writing`, `adding`, `reviewing`, `naming`, `closing`, ...) | A uniform opening makes the column scannable, and both forms force a situation rather than an assertion |
| Name what the author is DOING or what has HAPPENED, never what they must do | "All CLI commands MUST follow these patterns" matches every task and therefore routes nothing. "adding or changing a CLI subcommand, flag, or exit code" routes |
| One complete clause, one line | A trigger that ends on a comma, a dangling `by`/`with`/`the`, or an unbalanced `**` was copied out of a wrapped bold body line. Three such triggers shipped into `CONDENSED.md` unnoticed |
| Do not restate the directive | The directive belongs under `## Directives`, where the digest picks it up. Duplicating it in the trigger costs tokens in every session and routes nothing |

Reference rules (a lookup table, a glossary, an architecture summary) get a
trigger too, phrased as the moment you would reach for them: "looking up which
check enforces a rule", "reasoning about where a component sits".

## The body has a budget too

The lint caps the trigger line. Nothing caps the body, which is why every long
rule in the corpus is format-legal. `ai/rules/detail-budget.md` sets the standard,
and these four points are the ones a rule author breaks first.

| Requirement | Why |
|-------------|-----|
| One example for one point. A second example earns its place only by showing a DIFFERENT reading | A second instance of the same reading teaches nothing and enters every session through the digest |
| An ambiguous directive gets both readings and a statement of which governs, never a third example | Examples hide an ambiguity. Named readings end it |
| One table per distinction. Delete the paragraph that repeats the table | Two statements of one cut drift apart, and then the reader must decide which is current |
| State the obligation and name the gate. Never narrate the gate's implementation | Flags, exit codes, guard order, and line offsets live in the script and its fixtures. A rule that copies them holds a stale second copy |

A rule over about 150 lines is carrying reference material. Move the tables to
`docs/` and link to them, or split the rule at its real seam.

`scripts/dev/rules_lint.py` enforces all of this. When a line legitimately
describes ANOTHER artifact's severity (as `hook-mapping.md` does), mark that
line `<!-- severity-note: whose severity this is -->`. The marker is
line-scoped on purpose: a file-scoped opt-out would silently cover every later
addition to that file.

- The metadata block MUST be contiguous and immediately follow the title
  (one blank line allowed). No prose, table, or heading may sit between the
  title and the block.
- Put imperative content under `## Directives` (or the rule's own directive
  sections). This is what the digest artifacts carry.
- **One generator, `scripts/dev/rules_condensed.py`, emits three artifacts from one parse. Two are loaded into every session. The third is read on demand.** `TRIGGERS.md` carries one routing line per rule for all 97, so no rule is ever invisible. `CORE.md` carries the directives of the always-on rules only. `CONDENSED.md` carries every rule's directives and is NOT imported. Open it when several triggers match at once.
- **Your `**When:**` line is now the ONLY thing that reaches a session about your rule, unless the rule is in the core.** Write it as the situation a reader matches against the task in hand. A trigger that names no distinctive term routes nothing, and the rule is read only by someone who already went looking for it.
- **Core membership is derived, never listed.** Four conditions make a rule always-on: the ladder in `ai/rules/rule-precedence.md` names it on rung 1 or 2, it IS that ladder, it has no routable trigger, or no past task description in `plan/` would surface it. `make ze-rules-router-report` prints that last set. To make a rule always-on, put it on the ladder.
- **`make ze-rules-payload` measures what a session loads.** The budget is 40,000 tokens.
- Put the "why" under `## Rationale` and code under `## Examples`. The digest
  DROPS these sections, fenced code blocks, and `Rationale:`/`See:` pointer
  lines. Anything an agent must obey to comply belongs in a directive section,
  never only in `## Rationale`.
- **Write directives as bullets, table rows, or `**bold**` lines. Those reach the digest verbatim; prose does not.** The condenser keeps only the FIRST prose paragraph of each section, truncated to its first sentence or 220 characters, and drops every later prose paragraph in that section outright (`condense_body` / `flush_prose`, `scripts/dev/rules_condensed.py:106-148`).
- **Keep each bullet on ONE physical line when its full text must reach the digest.** A wrapped bullet's continuation lines do not match the list-item pattern (`scripts/dev/rules_condensed.py:52`), so they are treated as prose and are dropped or truncated by the paragraph rule above. A long single line is correct here; do not wrap it for looks.
- After editing a rule, READ your section in the regenerated `CONDENSED.md` before committing. A directive that lost half its sentence is not visible from the rule file alone.
- `make ze-rules-lint` enforces the block; `make ze-rules-condensed` regenerates
  all three artifacts. Both run in `make ze-doc-test`. A rule that fails the lint
  cannot land.
- **Commit all three regenerated artifacts in the SAME commit as the rule edit.**
  The freshness gate (`ze-rules-condensed-check`, inside the blocking
  `ze-regen-check-readonly`) regenerates from the WORKING TREE and compares
  against the working tree's digest, so it is green locally while HEAD is
  inconsistent. CI checks out HEAD, regenerates from HEAD's rules, and fails.
  A rule committed without its digest is therefore green on the author's
  machine and red for everyone else.
- When a **concurrent session** has an uncommitted rule edit, do NOT commit a
  digest generated from your working tree: it would publish their unlanded rule
  text, and still mismatch what CI regenerates from HEAD. Generate the digest
  from HEAD plus your own edits instead
  (`git archive HEAD ai/rules scripts/dev/rules_condensed.py | tar -x -C <scratch>`,
  copy your edited rules in, run the generator there), and commit that.

## Rationale

`CONDENSED.md` was imported into every agent session, so a fresh session saw
every rule's directives without opening 97 files. That only works if a tool can
mechanically separate a rule's directives from its explanation. Before this
format, directives and rationale were interleaved and no extraction was
reliable.

Eager loading cost about 99,600 tokens on every turn. Every session and every
subagent paid it, whether or not any of it applied. A session editing one
markdown file paid what a session rewriting the BGP wire encoder paid. No single
rule exceeded 6.4% of that, and the top 20 were only 55%. Trimming files
therefore cannot fix it.

`TRIGGERS.md` plus `CORE.md` replace the import. They measure about 21,100
tokens, an 80% reduction. The safety property is awareness, not inclusion. Every
rule keeps a line in `TRIGGERS.md`, so a rule whose body is not loaded is still
named in every session and is one Read away. That is why the `**When:**` trigger
is now load-bearing. It already let `INDEX.md` be derived instead of
hand-written.

## Examples

```
# Buffer-First Encoding

**When:** touching wire encoding or allocating memory
**Severity:** blocking
**Related:** memory-architecture, no-sprintf-alloc

## Directives
- All wire encoding MUST write into pooled, bounded buffers.

## Rationale
Allocation on the hot path dominates BGP churn cost, so ...
```
