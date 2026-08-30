# Rule File Format

**When:** authoring or editing any rule: a point file under `ai/rules/points/`, its manifest, or a check's binding comment
**Severity:** blocking
**Related:** repo-maintenance

## Directives

**A rule is a DIRECTORY of points, and `ai/rules/<rule>.md` is GENERATED from it. You MUST edit the points; you MUST NOT edit the rendered file.** `writeRenderedRule` in `internal/le/hookruntime/writeedit.go` refuses the edit and names the canonical point directory. The layout, the frontmatter fields, the renderer's refusals and the generator order are in `docs/contributing/rule-authoring.md`.

- **The manifest owns the whole spine.** Its body carries one section line, `<dir-slug> ## The Heading Verbatim`, then that section's point slugs indented by two spaces. The renderer emits the heading and those bodies in that order, joined by one blank line. A reorder is a manifest edit, never a rename.
- **The path is the id, and it is always `<rule>/<section>/<slug>`.** A `##` heading is the section DIRECTORY; a `###` or `####` heading stays a point inside it, so the depth is fixed at two. There is no content hash and no numeric prefix. A reworded instruction is a one-file diff, `git log` on the point file dates it, and every gate bound to that point keeps working.
- **MUST add a point in two edits: write the file in its section directory, then list its slug under that section in the manifest.** A point on disk that no manifest lists is a hard error, and so is a `*.md` sitting outside every section, so the file alone changes nothing.
- **A Write over a point that already exists is REFUSED.** `writePointOverwrite` in `internal/le/hookruntime/writeedit.go` blocks it and names both routes: MAY edit the point that is there, or pick a slug no file in that directory uses. An Edit, and a Write to a free slug, stay permitted.
- **A body is verbatim.** The renderer concatenates bodies and rewrites nothing inside one, which is what makes `./le rules points-roundtrip-check` a proof rather than a comparison.

**Every rendered rule MUST open with a title and a machine-readable metadata block**, so tooling parses triggers and severity without guessing. The manifest frontmatter carries both, and the renderer emits them.
**An ALL-CAPS stem is a generated artifact, never a rule**: `INDEX.md`, `TRIGGERS.md`, `CORE.md`. The generators skip it by that shape, so a new artifact needs no code change.

**A rule header MUST carry these elements, in this exact order:**

| Element | Requirement |
|---------|-------------|
| `# Title` | First non-blank line, a single H1. It MUST NOT contain "BLOCKING": `**Severity:**` carries that, and no tool can read a title marker |
| `**When:** <trigger>` | Required. One line. The SITUATION that makes this rule apply, phrased so an agent can match it against the task at hand |
| `**Severity:** blocking\|advisory` | Required. `blocking` means a gate or hook enforces it, or violating it breaks correctness; `advisory` means a strong convention. It MUST agree with the prose: a rule whose body says BLOCKING MUST NOT declare `advisory` |
| `**Related:** slug, slug` | Optional. Comma-separated rule slugs (filename without `.md`), no paths |

- **An always-on rule MUST hold prohibitions, and a PROCEDURE MUST live in its own rule under its own trigger.** `core_members` (`internal/le/rules/artifacts.go`) derives eagerness from the precedence ladder, so a procedure written inside a rung-1 rule is loaded in full by every session that will never carry it out. The ban and the how-to are separable: the ban earns its permanent seat because acting without it is unrecoverable, and the how-to is one Read away for the session that reaches the work.

## Every directive states a level

- Every point whose `kind` is `directive` MUST state its obligation in RFC 2119 language, and its `level:` MUST name the strongest TIER the body states. A directive whose weight a reader infers from tone is a directive two readers weigh differently.
- The tiers are MAY, then SHOULD with SHOULD NOT, then MUST with MUST NOT. RFC 2119 ranks obligation by STRENGTH and does not rank MUST against MUST NOT, so a point stating both MAY declare whichever its central clause carries. Ordering the two would force a point whose central clause is a prohibition to declare MUST, and the prohibition would go unrecorded.
- Use MUST and MUST NOT for an obligation. Use SHOULD and SHOULD NOT for a strong default that a reader MAY depart from with a stated reason. Use MAY for a permission. The linter accepts SHALL, SHALL NOT, REQUIRED, RECOMMENDED, NOT RECOMMENDED and OPTIONAL. It maps each keyword to its level, so `level:` has one spelling per level.
- The lowercase spellings `must`, `shall`, `should` and `may` MUST NOT appear in a directive body. They read as the obligation word and carry none of its force, and `ai/rules/writing.md` bans the hedging spelling outright. Capitalise the keyword, or rewrite the sentence so it carries no modal.
- A block that states no obligation is `kind: note` or `kind: table`, never `kind: directive`. The gate is scoped to directives on purpose: a two-column lookup gains a word and no obligation from being made to say MUST.
- Text inside a code span, fenced block, or Markdown blockquote is quoted, never stated. Neither gate reads it. Quoted text keeps its own spelling.
- `writePointLanguage` in `internal/le/hookruntime/writeedit.go` refuses the write, and `./le rules lint` refuses the finished tree. A Write carries the whole point, so a missing keyword is refused there. An Edit or MultiEdit carries fragments, so the hook refuses only lowercase modals in those fragments.

## The trigger is a routing key

**`**When:**` is not a summary and not the rule's first directive. It MUST name a situation and nothing else.** It is the only field an agent matches against the task in hand before deciding whether to open the file.

**A trigger line MUST meet every requirement below:**

| Requirement | Why |
|-------------|-----|
| Start with a temporal opener (`when`, `whenever`, `before`, `after`, `while`, `during`, `if`, `once`, `unless`, `upon`, `on`, `at`, `any/every/each time`, `prior to`, `as soon as`) or a gerund (`writing`, `adding`, `reviewing`, `naming`, `closing`) | A uniform opening makes the column scannable, and both forms force a situation rather than an assertion |
| Name what the author is DOING or what has HAPPENED, never what they are obliged to do | "All CLI commands MUST follow these patterns" matches every task and therefore routes nothing. "adding or changing a CLI subcommand, flag, or exit code" routes |
| One complete clause, one line | A trigger that ends on a comma, a dangling `by`, `with` or `the`, or an unbalanced `**` was copied out of a wrapped bold body line |
| Do not restate the directive | The directive belongs under `## Directives`, where the digest picks it up. Duplicating it in the trigger costs tokens in every session and routes nothing |

**A reference rule (a lookup table, a glossary, an architecture summary) MUST carry a trigger too, phrased as the moment you would reach for it:** "looking up which check enforces a rule", "reasoning about where a component sits".

- **Before a section is split into a rule of its own, its candidate trigger MUST be scored against the task corpus.** `distinctive_terms` (`internal/le/rules/artifacts.go`) drops every trigger term that too many other triggers share, and `unreachable_blocking` names each blocking rule no past task would surface. `core_members` then makes exactly that set always-on, so a split whose trigger scores nothing returns the new rule to the core at full size and saves nothing. `./le rules router-report` prints the set and the corpus it read.

## The body has a budget too

**A rule body MUST meet every requirement below. The lint caps the trigger line and nothing caps the body, so these are the author's to keep:**

| Requirement | Why |
|-------------|-----|
| One example for one point. A second example earns its place only by showing a DIFFERENT reading | A second instance of the same reading teaches nothing and enters every session through the digest |
| An ambiguous directive gets both readings and a statement of which governs, never a third example | Examples hide an ambiguity. Named readings end it |
| One table per distinction. Delete the paragraph that repeats the table | Two statements of one cut drift apart, and then the reader has to decide which is current |
| State the obligation and name the gate. Never narrate the gate's implementation | Flags, exit codes, guard order and line offsets live in the code and its fixtures. A rule that copies them holds a stale second copy |
| A point says what to DO next. It carries no history: no date a mechanism changed, no post-mortem count, no account of why the old way failed. Route those to `plan/learned/`, `plan/journal/`, `ai/rationale/` or the spec | A rule enters EVERY session's context and costs its tokens in all of them, forever. What a reader has to do is the only thing that earns a permanent seat; the story of how it got this way is read on demand or not at all |

**A rule over about 150 lines is carrying reference material. Its tables MUST be moved to `docs/` and linked, or the rule MUST be split at its real seam.**

**A line that legitimately describes ANOTHER artifact's severity MUST be marked `<!-- severity-note: whose severity this is -->`.** `internal/le/rules/lint.go` reads the RENDERED rule, so an unmarked line reads as this rule's own severity. The marker is line-scoped on purpose: a file-scoped opt-out would silently cover every later addition to that file.

- The metadata block MUST be contiguous and immediately follow the title
  (one blank line allowed). Prose, tables, and headings MUST NOT sit between
  the title and the block.
- MUST put imperative content under `## Directives` (or the rule's own directive
  sections). This is what the digest artifacts carry.
- **One generator, `internal/le/rules/artifacts.go`, emits two artifacts from one parse, and both load into every session.** `TRIGGERS.md` carries one routing line per rule, so no rule is ever invisible. `CORE.md` carries the directives of the always-on rules only. A rule's own file holds everything else, one Read away.
- **Your `**When:**` line is now the ONLY thing that reaches a session about your rule, unless the rule is in the core.** MUST write it as the situation a reader matches against the task in hand. A trigger that names no distinctive term routes nothing, and the rule is read only by someone who already went looking for it.
- **Core membership MUST be derived; it MUST NOT be listed.** Four conditions make a rule always-on: the ladder in `ai/rules/rule-precedence.md` names it on rung 1 or 2, it IS that ladder, it has no routable trigger, or no past task description in `plan/` would surface it. `./le rules router-report` prints that last set. To make a rule always-on, MUST put it on the ladder.
- **`./le rules payload-report` measures what a session loads.** The budget is 40,000 tokens.
- MUST put the "why" under `## Rationale` and code under `## Examples`. The digest
  DROPS these sections, fenced code blocks, and `Rationale:`/`See:` pointer
  lines. Anything an agent MUST obey to comply MUST be placed in a directive
  section, and MUST NOT sit only in `## Rationale`.
- **MUST write directives as bullets, table rows, or `**bold**` lines. Those reach the digest verbatim; prose does not.** The condenser keeps only the FIRST prose paragraph of each section, truncated to its first sentence or 220 characters, and drops every later prose paragraph in that section outright (`condense_body` / `flush_prose`, `internal/le/rules/artifacts.go`).
- **MUST keep each bullet on ONE physical line when its full text MUST reach the digest.** A wrapped bullet's continuation lines do not match the list-item pattern (`internal/le/rules/artifacts.go`), so they are treated as prose and are dropped or truncated by the paragraph rule above. A long single line is correct here; MUST NOT wrap it for looks.
- After editing a rule, MUST READ your rule's row in the regenerated `TRIGGERS.md`, and its section in `CORE.md` when the rule is always-on. A trigger that lost half its clause is not visible from the rule file alone.
- **MUST run the generators in order: `./le rules render-update`, then `./le rules condensed-update`, then `./le rules index-update`.** The last two parse the RENDERED rules, so a render that has not run yet feeds them the previous text. `./le rules lint` enforces the metadata block and `./le rules gate-map-report` checks the bindings. All of them run inside `./le doc check verify`, and a rule that fails the lint cannot land.
- **MUST commit the points and all four generated artifacts in the SAME commit as the rule edit.** They are `ai/rules/<rule>.md`, `TRIGGERS.md`, `CORE.md` and `ai/rules/INDEX.md`. `./le repository generated-check` regenerates from the WORKING TREE, so it is green on your machine while HEAD is inconsistent, and CI regenerates from HEAD and fails.
- **When a concurrent session holds an uncommitted rule edit, MUST generate the artifacts from HEAD plus your own points; MUST NOT generate from the shared working tree.** Your tree carries their unlanded text. `docs/contributing/rule-authoring.md` carries the recipe.

## Binding a check to a point

**A native hook check MUST declare the point it enforces with a `// ze point: <rule>/<section>/<slug>` line in its Go function's doc comment.** The function MUST be a top-level function named in `nativeHookActions`.
**A check that enforces no written point MUST say `// ze point: none -- <why>`, and the reason is REQUIRED.** Without it, "nobody bound this yet" and "there is nothing to bind" look the same.
**`./le rules gate-map-report` refuses a dangling, regressed, or bare binding.** What each of its sets means is in `docs/contributing/rule-authoring.md`.
