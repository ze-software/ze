# 847 -- rules-index-generator

## Context

Agents had no compact overview of the 68 `ai/rules/*.md` files, so they discovered a relevant rule only by accident or by reading the hand-curated `ai/INSTRUCTIONS.md` "Before You..." table, which referenced 39 of them. The goal was an automatic, complete, always-fresh map of "which rule covers which topic" that an agent scans at session start and uses to decide when to open a rule in full, without dumping 68 lines into every session.

## Decisions

- Chose a generated `ai/rules/INDEX.md` (one row per rule: title, when-to-read, path) over dumping rule summaries into the session-start hook because a pull-model is one Read when needed, stays greppable, and does not pay a token cost every session.
- Chose to derive each summary from the rule file itself in priority order `**When:**` then `**BLOCKING:**` then first prose paragraph, so most rules need no change and the rule file stays the single source of its own overview.
- Added explicit `**When:**` lines only to the 5 rules whose top was a bare `Rationale:` pointer or a bold header (`architecture-summary`, `design-principles`, `git-safety`, `go-standards`, `no-partial-completion`); everything else extracts cleanly.
- Made the drift gate ADVISORY (folded `rules_index.py --check` into `ze-doc-test` and `ze-regen-check`, not a hard pre-commit blocker) because 3 sessions were active and a hard gate on a stale INDEX would block their commits over a file they never touched.
- Mirrored `code_to_docs.py` conventions exactly (Python in `scripts/dev/`, `--check` flag, `make` target, `_test.go` beside it) rather than inventing a new tool shape.

## Consequences

- A new or renamed rule cannot land discoverable-free: `--check` fails if `ai/rules/INDEX.md` is stale or any rule yields no summary, and `make ze-regen` regenerates it alongside the other generated indexes.
- `ai/rules/INDEX.md` is now a generated file even though `ai/rules/*.md` are otherwise hand-edited originals; `repo-maintenance.md` carries the carve-out so nobody hand-edits it.
- The session-start hook points at the index, so every session sees the pointer; `ai/INDEX.md` and `ai/INSTRUCTIONS.md` (hence CLAUDE.md/AGENTS.md) carry discovery rows.

## Gotchas

- The extractor must require the exact `**When:**` marker, not `startswith("**When")`, or a bold heading like `**When to use sync.Pool...**` is mistaken for a trigger (hit in `performance.md`).
- Paragraph grouping must drop code fences and break at `Rationale:`/`Principle:`/`Structural template:` pointer lines, else summaries pick up ASCII-art diagrams or trailing "Rationale: `file`" noise.
- `ai/rules/INDEX.md` summaries are a faithful extraction and may contain em dashes that exist verbatim in source rule files; that is not authored prose.
- `CLAUDE.md`/`AGENTS.md` are git-ignored generated artifacts: edit `ai/INSTRUCTIONS.md` and run `make ze-ai-instructions`; they never appear in `git status`.

## Files

- `scripts/dev/rules_index.py`
- `scripts/dev/rules_index_test.go`
- `ai/rules/INDEX.md`
- `ai/rules/repo-maintenance.md`
- `ai/rules/architecture.md`
- `ai/rules/architecture.md`
- `ai/rules/git-safety.md`
- `ai/rules/go-standards.md`
- `ai/rules/completion.md`
- `Makefile`
- `mk/inventory.mk`
- `.claude/hooks/session-start.sh`
- `ai/INDEX.md`
- `ai/INSTRUCTIONS.md`
