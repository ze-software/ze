# 243 — Proactive Methodology

## Objective

Improve spec methodology by adding named phases, checkpoint annotations, acceptance criteria, failure routing, and mistake logging to `planning.md`.

## Decisions

- Named phases: RESEARCH / DESIGN / IMPLEMENT / VERIFY.
- `→ Decision:` and `→ Constraint:` checkpoint annotations in Required Reading sections.
- `## Acceptance Criteria` section with AC-N assertions.
- Failure Routing table and Mistake Log added.
- Goal/Quality Gates split (goal = what, quality = how well).
- New `validate-spec.sh` checks added as warnings (not errors) for backwards compatibility.

## Patterns

None beyond what `planning.md` documents.

## Gotchas

- `grep -c pattern file || echo "0"` bug: grep exits 1 when match count is 0, so both grep output (0) AND the echo fallback fire. Fix: `grep -c pattern file || true`.

## Files

- `.claude/rules/planning.md` — methodology updates
- `scripts/validate-spec.sh` — spec validation checks
