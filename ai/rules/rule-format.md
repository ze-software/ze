# Rule File Format

**When:** authoring or editing any `ai/rules/*.md` rule file
**Severity:** blocking
**Related:** canonical-sources, discovery-updates

## Directives

Every `ai/rules/*.md` rule (except the generated `INDEX.md` and `CONDENSED.md`)
MUST open with a title and a machine-readable metadata block, so tooling can
parse triggers and severity without guessing.

Required structure, in this exact order:

| Element | Requirement |
|---------|-------------|
| `# Title` | First non-blank line, a single H1. |
| `**When:** <trigger>` | Required. One line. The situation that makes this rule apply, phrased so an agent can match it against the task at hand. |
| `**Severity:** blocking\|advisory` | Required. `blocking` = a gate/hook enforces it or violating it breaks correctness; `advisory` = strong convention. |
| `**Related:** slug, slug` | Optional. Comma-separated rule slugs (filename without `.md`), no paths. |

- The metadata block MUST be contiguous and immediately follow the title
  (one blank line allowed). No prose, table, or heading may sit between the
  title and the block.
- Put imperative content under `## Directives` (or the rule's own directive
  sections). This is what `CONDENSED.md` loads into every session.
- Put the "why" under `## Rationale` and code under `## Examples`. The digest
  DROPS these sections, fenced code blocks, and `Rationale:`/`See:` pointer
  lines. Anything an agent must obey to comply belongs in a directive section,
  never only in `## Rationale`.
- **Write directives as bullets, table rows, or `**bold**` lines. Those reach the digest verbatim; prose does not.** The condenser keeps only the FIRST prose paragraph of each section, truncated to its first sentence or 220 characters, and drops every later prose paragraph in that section outright (`condense_body` / `flush_prose`, `scripts/dev/rules_condensed.py:106-148`).
- **Keep each bullet on ONE physical line when its full text must reach the digest.** A wrapped bullet's continuation lines do not match the list-item pattern (`scripts/dev/rules_condensed.py:52`), so they are treated as prose and are dropped or truncated by the paragraph rule above. A long single line is correct here; do not wrap it for looks.
- After editing a rule, READ your section in the regenerated `CONDENSED.md` before committing. A directive that lost half its sentence is not visible from the rule file alone.
- `make ze-rules-lint` enforces the block; `make ze-rules-condensed` regenerates
  the digest. Both run in `make ze-doc-test`. A rule that fails the lint cannot
  land.
- **Commit the regenerated `CONDENSED.md` in the SAME commit as the rule edit.**
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

`CONDENSED.md` is imported into every agent session (`@ai/rules/CONDENSED.md`
in `ai/INSTRUCTIONS.md`), so a fresh session sees every rule's directives
without opening 89 files. That only works if a tool can mechanically separate a
rule's directives from its explanation; before this format, directives and
rationale were interleaved and no extraction was reliable. The `**When:**`
trigger also lets `INDEX.md` be derived instead of hand-written.

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
