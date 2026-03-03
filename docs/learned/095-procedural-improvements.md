# 095 — Procedural Improvements

## Objective
Audit pending specs for status, delete a stale 941-line CLAUDE_CONTINUATION.md, and bootstrap Claude Code infrastructure (hooks, commands, INDEX, RFC summaries).

## Decisions
- CLAUDE_CONTINUATION.md deleted entirely: it duplicated git log, specs, and rules. Technical debt items migrated to TODO.md; the rest discarded.
- RFC summaries live in `docs/architecture/rfc/rfcNNNN.md`, created on demand via `/rfc-summarisation`. 35 pre-populated on first pass.
- Spec status audit revealed `spec-parser-unification.md` was 0% implemented — design doc only, never built.

## Patterns
- Spec audit pattern: compare "Files to Create" and key functions against the actual filesystem — immediately reveals implemented vs. design-only.
- Infrastructure bootstrap (hooks, commands, rules) belongs in the same commit as the audit that identified the gaps.

## Gotchas
- A spec existing in `docs/plan/` does not mean it was implemented. Always verify against the codebase before building on top of it.

## Files
- `TODO.md` — created as central spec/debt tracker
- `.claude/INDEX.md`, `.claude/hooks/validate-spec.sh`, `.claude/commands/rfc-summarisation.md` — new infrastructure
- `docs/plan/CLAUDE_CONTINUATION.md` — deleted (941 lines, all stale)
