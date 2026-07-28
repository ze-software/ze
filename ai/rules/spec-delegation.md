# Spec Work Runs in Subagents; the Main Thread Supervises

**When:** starting, resuming, or continuing work on ANY spec -- research, design, implementation, review, audit, or closure -- in the main session thread
**Severity:** blocking
**Related:** model-selection, critical-review, planning, handoff

## Directives

**The main thread supervises. It does not perform the spec work itself.** Each phase runs in a subagent invoked through its `ze-*` skill; the main thread launches it, reads the report back, verifies it, decides, and gates the next phase.

| Phase | Delegate to a subagent running | The main thread does |
|-------|--------------------------------|----------------------|
| Research a topic or subsystem | `/ze-explore`, `/ze-audit` | states the question, reads the findings, decides what they change |
| Write or revise a spec | `/ze-spec` | relays the user's answers, approves the design, owns the status transition |
| Stress-test a design | `/ze-design` | carries the one-decision-per-question dialogue with the user |
| Implement | `/ze-implement` | selects the spec, relays user decisions, checks the report against the spec's ACs |
| Review gate | `/ze-review`, `/ze-review-deep`, `/ze-review-spec` | verifies each finding, decides which are real, loops until zero |
| Close | `/ze-close` | confirms the Review Gate artifact is clean, then that the two closure commits actually ran |
| Debug a red test or gate | `/ze-debug` | confirms the diagnosis names a `file:line` root cause, not a symptom |
| Verify | `/ze-verify` | reads the failure index, decides what to fix next |

**Launch independent phases in ONE message with parallel `Agent` calls.** Two review lenses, two research questions, or two independent spec areas are concurrent work, not a queue.

**Give every subagent the spec path, the phase it is in, and the rules that govern it.** A subagent inherits no session state: name `plan/<spec>.md`, the `ai/rules/` files that apply, and what its report must contain. It has no LSP tool and cannot ask the user -- do not hand it work that needs either.

**Verify what a subagent reports; never relay it as fact.** An agent's report is a claim, not evidence (`ai/rules/no-fabrication.md`). Before acting on a finding or repeating it to the user, confirm the `file:line` it cites actually produces the behavior it describes.

**Anything the user must answer stays in the main thread.** A subagent cannot hold a dialogue with the user, so `/ze-spec` and `/ze-design` question gates, scope reductions, and RFC-compliance escalations (`ai/rules/rfc-compliance.md`) are raised by the main thread, never delegated away.

**Delegation never dilutes the independence of review.** Reviewer subagents must be spawned separately from the implementation agent and must not be given the implementer's reasoning as their starting point (`ai/rules/critical-review.md`).

**Delegation does not override phase-to-model boundaries.** Subagents inherit the PHASE, not the task shape (`ai/rules/model-selection.md`), so the main thread still announces a boundary and stops rather than delegating an implementation phase from a review session to get around the switch.

## Banned Reasoning

| Banned | Reality |
|--------|---------|
| "This edit is small, I will just do it inline" | Size is judged after review. A one-line spec change still passes through the phase that owns it |
| "Spawning an agent costs a round trip" | The round trip is the supervision. Doing the work inline is what the main thread is not for |
| "I already have the context loaded, an agent would have to re-read it" | Re-reading is cheap; a main thread that fills with implementation detail cannot supervise the phases that follow |
| "The agent's report looks right, I will pass it on" | Unverified relay is fabrication with an extra hop (`ai/rules/no-fabrication.md`) |
| "I will implement it and then spawn a reviewer" | The implementation phase was owed a subagent too. One rule broken does not excuse the next |

## Rationale

Thomas set this shape on 2026-07-28 after main-thread sessions repeatedly did
spec work inline. Two costs drove it. First, a main thread that implements
cannot supervise: its context fills with the detail of one phase, so the phase
boundaries (`ai/rules/model-selection.md`) and the independence of review
(`ai/rules/critical-review.md`) both blur, and the session ends up reviewing its
own work. Second, subagent context is disposable while main-thread context is
not, so the expensive reading belongs in an agent whose report is the only thing
that survives into the supervising context.
