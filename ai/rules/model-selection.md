# Model Selection by Work Phase

**When:** starting, resuming, or handing off any spec/design, implementation, or review phase, and whenever you are about to cross from one phase into another in the same session
**Severity:** blocking
**Related:** planning, critical-review, handoff

## Directives

Each phase of Ze work runs on a specific model. The model is chosen by phase,
never by convenience, and never by "the session I happen to be in".

| Phase | Model | Covers |
|-------|-------|--------|
| Planning and design | Opus 5 | research, `/ze-spec`, `/ze-design`, spec writing and revision, architecture decisions, RFC reading, handoff authoring |
| Implementation | Opus 4.8 | `/ze-implement`, writing code and tests, fixing failures, refactors, doc edits that follow from the code |
| Review and audit | Opus 5 | `/ze-review`, `/ze-review-deep`, `/ze-review-spec`, `/ze-audit`, `/ze-close` (Review Gate, spec closure, implementation audit) |

Planning and review are the judgment-heavy phases and both run Opus 5.
Implementation is the execution phase and runs Opus 4.8.

## Phase Boundaries Are Model Boundaries

A session cannot change its own model. The operator selects it (`/model`, or
when launching the session), so the session's only job is to make the boundary
visible and refuse to blur it.

| Situation | Do |
|-----------|-----|
| The spec is approved and coding is about to start | State that the implementation phase wants Opus 4.8, then stop and let the operator switch or start an implementation session |
| Implementation is complete and the Review Gate is next | State that review wants Opus 5, then stop. Never review your own implementation on the implementation model |
| A review or audit produces fixes | The fixes are implementation. They belong on Opus 4.8, and the re-review that follows belongs back on Opus 5 |
| You are already mid-phase on the wrong model | Say so plainly, name the model the phase wants, and let the operator decide. Do not silently continue |
| The work is a one-line mechanical edit with no design or review content | Proceed on whatever model is loaded. This rule governs phases, not keystrokes |

This never overrides `ai/rules/critical-review.md`: review is INDEPENDENT of
the author. A different model is not a different context. A fresh session or
reviewer subagents are still required.

## Subagents

- Subagents inherit the PHASE, not the task shape: reviewer subagents spawned during review stay on the review model, implementation subagents stay on the implementation model.
- WHO executes a phase is governed separately by `ai/rules/spec-delegation.md`: the main thread supervises, and each phase runs in a subagent through its `ze-*` skill. That rule never lets a session delegate its way across a model boundary.
- The `Agent` tool's `model` parameter selects a family (`opus`, `sonnet`, `haiku`), not a minor version, so it cannot pin 4.8 against 5. The phase-to-model mapping above is about the session driving the work.
- Never downgrade a subagent to a cheaper model because its lens looks mechanical. `ai/skills/ze-review-deep.md` and `ai/skills/ze-debug.md` spawn every agent on `opus` for this reason. If cost forces a reduction, cut the NUMBER of agents, never the model they run on.

## Banned Reasoning

| Banned | Reality |
|--------|---------|
| "I am already here, I will just implement it" | The phase changed. The model has to change with it |
| "It is a small implementation, review can stay on the same model" | Size is judged after review, not before |
| "Switching costs a round trip" | The round trip is the point. It is the boundary |
| "The review model can write the fix faster" | Then the fix is unreviewed work written by the reviewer. Two rules broken, not one |

## Enforcement

- **`c_model_phase` in `.claude/hooks/pretool-writeedit.py` BLOCKS an implementation edit made on a planning/review model.** It resolves the running model from the transcript, because the hook payload carries none. It fires on `.go`, `.py`, `.sh`, `.ci`, `.et`, `.yang`, `.mk`, `.tmpl` and `.rego`, and never on `.md`. The table above puts "doc edits that follow from the code" in the implementation phase, so the gate is deliberately narrower than the rule: it cannot tell a spec from a doc that follows code, and blocking `/ze-spec` on its own model would be the worse error. Doc edits stay yours to judge.
- **The escape is a deliberate act, not a flag.** When the operator decides to proceed on this model, record the reason in `tmp/session/.model-ack-<sid>`. Write that file on the operator's instruction only. It is the same contract as the spec-closure ack.
- The gate cannot see PHASE, only the model. It reads an implementation edit as the boundary crossing, so a genuine one-line mechanical fix on the review model needs the ack too.
- **Review is gated at both ends.** `.claude/hooks/pretool-agent-skill.py` refuses to SPAWN a review agent when the session is not on Opus 5, and `scripts/dev/review_gate.py record` refuses to RECORD the artifact. The second is the one that matters, because recording is the moment a review is claimed.
- **A subagent inherits the PHASE, not the task shape.** Spawning a reviewer from an implementation session still reviews on the wrong model, and it is usually the session that wrote the code (`ai/rules/critical-review.md`).
- **The record gate has an operator escape: `--model-override "<reason>"`.** Their call, not yours.
- **All three gates share one reader, `scripts/dev/running_model.py`.** It resolves the model from the session transcript, skips subagent lines, and answers nothing when it cannot tell. Every caller then stands down and SAYS so, rather than going quiet.
