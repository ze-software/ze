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
- `make ze-rules-lint` enforces the block; `make ze-rules-condensed` regenerates
  the digest. Both run in `make ze-doc-test`. A rule that fails the lint cannot
  land.

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
