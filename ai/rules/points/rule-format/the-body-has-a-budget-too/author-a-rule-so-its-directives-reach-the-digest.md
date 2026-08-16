---
kind: directive
level: MUST
stage:
---
- The metadata block MUST be contiguous and immediately follow the title
  (one blank line allowed). Prose, tables, and headings MUST NOT sit between
  the title and the block.
- MUST put imperative content under `## Directives` (or the rule's own directive
  sections). This is what the digest artifacts carry.
- **One generator, `scripts/dev/rules_condensed.py`, emits two artifacts from one parse, and both load into every session.** `TRIGGERS.md` carries one routing line per rule, so no rule is ever invisible. `CORE.md` carries the directives of the always-on rules only. A rule's own file holds everything else, one Read away.
- **Your `**When:**` line is now the ONLY thing that reaches a session about your rule, unless the rule is in the core.** MUST write it as the situation a reader matches against the task in hand. A trigger that names no distinctive term routes nothing, and the rule is read only by someone who already went looking for it.
- **Core membership MUST be derived; it MUST NOT be listed.** Four conditions make a rule always-on: the ladder in `ai/rules/rule-precedence.md` names it on rung 1 or 2, it IS that ladder, it has no routable trigger, or no past task description in `plan/` would surface it. `make ze-rules-router-report` prints that last set. To make a rule always-on, MUST put it on the ladder.
- **`make ze-rules-payload-report` measures what a session loads.** The budget is 40,000 tokens.
- MUST put the "why" under `## Rationale` and code under `## Examples`. The digest
  DROPS these sections, fenced code blocks, and `Rationale:`/`See:` pointer
  lines. Anything an agent MUST obey to comply MUST be placed in a directive
  section, and MUST NOT sit only in `## Rationale`.
- **MUST write directives as bullets, table rows, or `**bold**` lines. Those reach the digest verbatim; prose does not.** The condenser keeps only the FIRST prose paragraph of each section, truncated to its first sentence or 220 characters, and drops every later prose paragraph in that section outright (`condense_body` / `flush_prose`, `scripts/dev/rules_condensed.py`).
- **MUST keep each bullet on ONE physical line when its full text MUST reach the digest.** A wrapped bullet's continuation lines do not match the list-item pattern (`scripts/dev/rules_condensed.py`), so they are treated as prose and are dropped or truncated by the paragraph rule above. A long single line is correct here; MUST NOT wrap it for looks.
- After editing a rule, MUST READ your rule's row in the regenerated `TRIGGERS.md`, and its section in `CORE.md` when the rule is always-on. A trigger that lost half its clause is not visible from the rule file alone.
- **MUST run the generators in order: `make ze-rules-render-update`, then `make ze-rules-condensed-update`, then `make ze-rules-index-update`.** The last two parse the RENDERED rules, so a render that has not run yet feeds them the previous text. `make ze-rules-lint` enforces the metadata block and `make ze-rules-gate-map-report` checks the bindings. All of them run inside `make ze-doc-verify`, and a rule that fails the lint cannot land.
- **MUST commit the points and all four generated artifacts in the SAME commit as the rule edit.** They are `ai/rules/<rule>.md`, `TRIGGERS.md`, `CORE.md` and `ai/rules/INDEX.md`. `ze-generated-files-check` regenerates from the WORKING TREE, so it is green on your machine while HEAD is inconsistent, and CI regenerates from HEAD and fails.
- **When a concurrent session holds an uncommitted rule edit, MUST generate the artifacts from HEAD plus your own points; MUST NOT generate from the shared working tree.** Your tree carries their unlanded text. `docs/contributing/rule-authoring.md` carries the recipe.
