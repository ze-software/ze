# 1228 -- rule-format-condensed-eager-load

Gave every `ai/rules/*.md` a machine-parseable metadata block (`**When:**` /
`**Severity:**` / optional `**Related:**`), added a lint gate that enforces it, a
generator that emits `ai/rules/CONDENSED.md` (the directive-only digest of all 89
rules), and an `@ai/rules/CONDENSED.md` import in `ai/INSTRUCTIONS.md` so a fresh
session loads every rule's directives instead of only the one-line INDEX pointers.

## Why this was needed

A fresh session's context held CLAUDE.md's "Before You" table (~30 rules named as
one-liners) plus `ai/rules/INDEX.md` as a lazy dispatch. That is a discovery index,
not a knowledge base: the ~59 rules absent from the table were invisible, and even
the named ones were headlines, not directives. Working from that alone produces
confidently-wrong output on rules the session never opened.

## Key decisions / gotchas

- **The rule corpus is directive-dense, so "condense to a fraction" was the wrong
  premise.** Full rules = ~125K tokens. Dropping code fences + rationale prose only
  reached ~76K (40% off), and a keyword-only pass reached ~63K -- because the mass
  is the "banned -> fix" TABLES that *are* the rules, not prose padding.
  `stale-comments` is a checklist table plus a Do-Not list with nothing to strip.
  Measured before promising a number; cutting below ~63K means deleting directives.

- **`@import` in CLAUDE.md loads at launch, it does NOT defer.** Confirmed against
  Claude Code docs: `@path` expands the file into context at session start (gitignore
  status irrelevant, paths resolve relative to the importing file, max 4 hops, a
  one-time approval dialog on first use). For genuine deferred loading the mechanism
  is path-scoped `.claude/rules/*.md` with a `paths:` frontmatter, which loads only
  when touching matching files. The marker-block format is the prerequisite for
  either mechanism.

- **Marker block, not YAML frontmatter.** The repo's skills use YAML frontmatter, but
  the rules already had a de-facto `**When:**` / `**BLOCKING**` convention (16 and 55
  files), and a bold-line block is lower-churn and needs no parser. `rules_lint.py`
  hard-enforces `**When:**` + `**Severity:**`; `## Directives` is recommended, not
  required, because directives and rationale are interleaved *within paragraphs*
  (`**No abbreviations.** Operators read...`) and a mechanical body split would be a
  semantic rewrite, not reformatting.

- **A new sibling file under `ai/rules/` silently joins every `*.md` glob.**
  `rules_index.py` skipped only `INDEX.md`, so `CONDENSED.md` showed up as a phantom
  90th "rule" (INDEX said 91, CONDENSED said 90). Any generator that globs a
  directory it also writes into needs the output in its own skip set.

- **Deriving `**When:**` for the 73 rules that lacked one is best-effort.** The
  migration takes an explicit `**When:**`, else the `**BLOCKING**` sentence, else the
  first prose sentence. Two traps: a multi-line `**When:**` value must absorb
  continuation lines (else the trigger truncates and the tail orphans into the body),
  and cutting a sentence at the first `:` yields fragments ("the registration
  pattern") -- prefer `.` and only accept `:` past a substantial clause. A handful of
  derived triggers are still weak and worth hand-polishing.

## Files

**Format + tooling:** `ai/rules/rule-format.md` (the schema, self-conforming),
`scripts/dev/rules_lint.py` (validator), `scripts/dev/rules_condensed.py`
(digest generator), `scripts/dev/rules_reformat.py` (one-shot migration),
`scripts/dev/rules_index.py` (skip `CONDENSED.md`).
**Eager load:** `ai/INSTRUCTIONS.md` (a CONDENSED.md import, since removed).
**Generated:** CONDENSED.md (deleted 2026-08-03), `ai/rules/INDEX.md`, and all 89 rule files
(metadata block).
**Wiring:** `Makefile` + `mk/inventory.mk` (`ze-rules-condensed`, `ze-rules-lint`
targets; wired into `ze-doc-test`, `ze-regen`, `ze-regen-check-readonly`,
`ze-regen-check`), `ai/rules/repo-maintenance.md` (CONDENSED generated-file note).
