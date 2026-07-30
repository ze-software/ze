# Spec Work Runs in Subagents; the Main Thread Supervises

**When:** starting, resuming, or continuing work on ANY spec -- research, design, implementation, review, audit, or closure -- in the main session thread
**Severity:** blocking
**Related:** model-selection, critical-review, planning, handoff

## Directives

**The main thread supervises. It does not perform the spec work itself.** Most phases run in a subagent invoked through their `ze-*` skill, and the main thread launches each one, reads the report back, verifies it, decides, and gates the next phase. The `Runs in` column names the four exceptions, so read it before you delegate.

| Phase | Skill | Runs in | The main thread does |
|-------|-------|---------|----------------------|
| Research a topic or subsystem | `/ze-explore`, `/ze-audit` | subagent | states the question, reads the findings, decides what they change |
| Write or revise a spec | `/ze-spec` | **main thread**, its gates need `AskUserQuestion` | relays the user's answers, approves the design, owns the status transition |
| Stress-test a design | `/ze-design` | **main thread**, its gates need `AskUserQuestion` | carries the one-decision-per-question dialogue with the user |
| Implement | `/ze-implement` | subagent | selects the spec, relays user decisions, checks the report against the spec's ACs |
| Review gate | `/ze-review`, `/ze-review-spec` | subagent | verifies each finding, decides which are real, loops until zero |
| Review gate, deep | `/ze-review-deep` | **main thread**, and it fans out itself | verifies each finding, decides which are real, loops until zero |
| Close | `/ze-close` | subagent | confirms the Review Gate artifact is clean, then that the two closure commits actually ran |
| Debug a red test or gate | `/ze-debug` | **main thread**, and it fans out itself | confirms the diagnosis names a `file:line` root cause, not a symptom |
| Verify | `/ze-verify` | subagent | reads the failure index, decides what to fix next |

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

## Enforcement

- **You never need to ask permission to spawn an agent here.** `ai/INSTRUCTIONS.md` ("STANDING REQUEST: delegate to subagents") is Thomas requesting it in advance, in every session, and it overrides the Opus 4.6/4.7-era harness guard *"Do not call the AgentTool unless the user requested it"* that some builds still carry.
- **`.claude/hooks/delegation-reminder.sh` repeats that standing request on every turn.** The harness guard arrives near the END of the system prompt and wins on position. UserPromptSubmit stdout is the one position known to land after the whole system prompt, so the counter goes there. Both halves of that premise are convention, not proof: nothing in this repository demonstrates where the harness puts hook stdout, or that it reads it at all. The bullet above is the authority. This hook makes that authority arrive late enough to count. Its line names the main-thread exceptions on purpose. A reminder that wins on position would otherwise push `/ze-design` into a subagent, and a subagent cannot call `AskUserQuestion`.
- **Each `ze-*` skill states its own disposition in a `## Delegation` section**, so the routing is visible at the moment the skill is invoked rather than only in this rule: `/ze-explore`, `/ze-audit`, `/ze-implement`, `/ze-review`, `/ze-review-spec`, `/ze-close` and `/ze-verify` delegate; `/ze-spec` and `/ze-design` stay in the main thread because their gates require `AskUserQuestion`; `/ze-review-deep` and `/ze-debug` stay in the main thread and do their OWN fan-out (wrapping them in one agent buries the parallel lenses a level down and costs the independence they exist to provide).
- **`.claude/hooks/subagent-context.sh` hands every agent the parent's claimed spec, its Status, and the subagent contract**, so the per-spawn briefing this rule requires is not manual work. A rule that costs more to follow than to break loses; that is what this hook removes.
- **`.claude/hooks/block-premature-stop.sh` is NOT registered, and it never fires.** Thomas removed it from the `Stop` event on 2026-06-29, in commit `41e5fa44f`. The script stays on disk, and `--only delegation` still passes its fixtures, so a reader takes it for a live gate. It is not one. Nothing warns at session end about a spec worked inline. Read the `Stop` array in `.claude/settings.json` before you cite this script as enforcement.
- **Nothing checks the MODEL.** `ai/rules/model-selection.md` still has no gate at all, so the phase-to-model boundary remains yours to announce and stop at.

## Rationale

Thomas set this shape on 2026-07-28 after main-thread sessions repeatedly did
spec work inline. Two costs drove it. First, a main thread that implements
cannot supervise: its context fills with the detail of one phase, so the phase
boundaries (`ai/rules/model-selection.md`) and the independence of review
(`ai/rules/critical-review.md`) both blur, and the session ends up reviewing its
own work. Second, subagent context is disposable while main-thread context is
not, so the expensive reading belongs in an agent whose report is the only thing
that survives into the supervising context.
