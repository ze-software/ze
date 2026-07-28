# 1281 -- Spec Delegation: Subagents Execute, the Main Thread Supervises

## Context

Main-thread sessions repeatedly did spec work inline: research, implementation,
and review all landed in the supervising context. Two things broke as a result.
The main thread filled with the detail of one phase, so it could no longer hold
the phase boundaries `ai/rules/model-selection.md` defines, and a session that
had just written code was the same context asked to review it, which
`ai/rules/critical-review.md` forbids. Thomas set the orchestration shape on
2026-07-28: use agents for spec work, main thread supervises only, drive each
phase through its `ze-*` skill.

## Decisions

- New rule `ai/rules/spec-delegation.md` (blocking) over a section inside `ai/rules/model-selection.md`: the two share a trigger but answer different questions (WHO executes vs WHICH model), and folding them would have changed model-selection's identity.
- The rule names a phase-to-skill table rather than a general "delegate when it is big" heuristic, because size-based judgement is exactly what the banned-reasoning table has to block.
- Two constraints written in rather than papered over: a subagent has no LSP tool and cannot hold a dialogue with the user, so the `/ze-spec` and `/ze-design` question gates, scope reductions, and RFC-compliance escalations stay in the main thread.
- Delegation is explicitly stated NOT to override phase-to-model boundaries, closing the loophole where a review session delegates an implementation phase to avoid the model switch.
- Subagent reports are treated as claims, not evidence: the rule requires the main thread to confirm a cited `file:line` before acting on a finding or relaying it, per `ai/rules/no-fabrication.md`.

## Consequences

- The main thread's job on any spec is now launch, verify, decide, gate. Doing the edit inline is a rule violation regardless of how small it looks.
- Independent phases are expected to launch as parallel `Agent` calls in one message, so review lenses and research questions stop being serialized.
- `ai/rules/model-selection.md` gained a back-pointer in its Subagents section; the two rules are read together at every phase boundary.
- Nothing enforces this mechanically. Like `model-selection.md`, the session is the enforcement point, so it lives in `CONDENSED.md` where every session loads it.

## Gotchas

- `ai/rules/rule-format.md` requires each directive to be a bullet, table row, or bold line on ONE physical line. Wrapped bullets are treated as prose and get truncated or dropped by the condenser, so the digest silently loses half a directive. Read your own section in the regenerated `CONDENSED.md`, not just the rule file.
- `CLAUDE.md` and `AGENTS.md` are generated from `ai/INSTRUCTIONS.md` and are gitignored. The "Before You..." row had to be added to the canonical source, then `make ze-ai-instructions` run; editing `CLAUDE.md` directly is blocked by `ai/rules/canonical-sources.md`.
- `ai/rules/INDEX.md` and `ai/rules/CONDENSED.md` are generated but tracked, so both must land in the SAME commit as the rule. A rule committed without its digest is green locally and red in CI, which regenerates from HEAD.

## Files

- `ai/rules/spec-delegation.md` (new)
- `ai/rules/model-selection.md` (back-pointer in Subagents)
- `ai/INSTRUCTIONS.md`, `ai/INDEX.md` (discovery rows)
- `ai/rules/INDEX.md`, `ai/rules/CONDENSED.md` (regenerated)
