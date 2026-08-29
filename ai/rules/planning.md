# Specs and Phases

**When:** before implementing any non-trivial feature, and whenever a spec phase starts, resumes, or closes
**Severity:** blocking
**Related:** completion, quality

## Directives

Complete before implementing any non-trivial feature.
Rationale: `ai/rationale/planning.md`

- **The main thread supervises. It does not perform the spec work itself.** Each phase runs in a subagent through its `ze-*` skill, except the four the `Runs in` column names ("Spec Work Runs in Subagents", below).
- **Each phase of Ze work runs on a specific model.** The model is chosen by phase, never by convenience, and never by "the session I happen to be in" ("Model Selection by Work Phase", below).
- **Before closing a spec or claiming a substantive change is done -- review is INDEPENDENT (subagents / fresh session), never the author's own inline reasoning, and is enforced by `internal/le/commit`.**
- **Obligation on you (not a hard gate):** Every decision to not perform in-scope work MUST be recorded AND land in a destination spec.
- **A spec that passes its Review Gate is not done until it is deleted from `plan/`,** and the completed spec MUST be committed to git first so it is preserved in history.
- **When the user asks how to continue, start with a short rationale section, then output exact edits.**

## Spec Work Runs in Subagents; the Main Thread Supervises

**The main thread MUST supervise. It MUST NOT perform the spec work itself.** Most phases run in a subagent invoked through their `ze-*` skill, and the main thread launches each one, reads the report back, verifies it, decides, and gates the next phase. The `Runs in` column names the four exceptions, so read it before you delegate.

**MUST say what you are about to spawn BEFORE you spawn it: the number of agents, what each does, and the rough cost.** The user pays for every agent and can see them running. A spawn they cannot map onto anything you wrote reads as a session out of control. Name the skill's own fan-out too: `/ze-close` spawns reviewers at its Review Gate, so "closure is running" understates it by three agents. Then report STATUS, never architecture: what runs, what finished, what is left, what it cost. A user who cannot follow you reads the tree instead, and what they find lands as a surprise you owed them.

| Phase | Skill | Runs in | The main thread does |
|-------|-------|---------|----------------------|
| Research a topic or subsystem | `/ze-explore`, `/ze-audit` | subagent | states the question, reads the findings, decides what they change |
| Write or revise a spec | `/ze-spec` | **main thread**, its gates need `AskUserQuestion` | relays the user's answers, approves the design, owns the status transition |
| Stress-test a design | `/ze-design` | **main thread**, its gates need `AskUserQuestion` | carries the one-decision-per-question dialogue with the user |
| Implement | `/ze-implement` | subagent | selects the spec, relays user decisions, checks the report against the spec's ACs |
| Review gate | `/ze-review`, `/ze-review-spec` | subagent | verifies each finding, decides which are real, loops until zero |
| Review gate, deep | `/ze-review-deep` | **main thread**, and it fans out itself | verifies each finding, decides which are real, loops until zero |
| Close | `/ze-close` | subagent | confirms the Review Gate artifact is clean, then that the two closure commits actually ran |
| Debug a red test or gate | `/ze-debug` | **main thread**, and it fans out itself | confirms the diagnosis names a root-cause function, not a symptom |
| Verify | `/ze-verify` | subagent | reads the failure index, decides what to fix next |

**MUST launch independent phases in ONE message with parallel `Agent` calls.** Two review lenses, two research questions, or two independent spec areas are concurrent work, not a queue.

- **Give every subagent the spec path, its phase, the rules that govern it, the parent session ID, and the exact per-session scratch path.** Name `plan/<spec>.md`, the applicable `ai/rules/` files, and what the report MUST contain.
- **When a delegation API does not run the native `subagent-context` action, the main thread MUST put the parent session ID and exact per-session scratch path in shared task context.** The OMP `task` API uses this fallback.
- **The subagent MUST use the provided scratch path.** When its environment does not contain `CLAUDE_CODE_SESSION_ID`, it MUST set that variable to the parent session ID for shell commands. It MUST NOT resolve a fresh session ID in this case.
- **A subagent cannot ask the user.** The main thread MUST NOT give it work that needs an answer from the user.
- **A subagent CAN resolve symbols.** It uses the LSP tool where its registry carries one, or `gopls` from Bash where it does not (`ai/rules/context-economy.md`).
- **A brief whose agent will write Go MUST name `docs/contributing/ze-go-style.md` as a PRECONDITION, in the brief's opening, and MUST NOT file it under a closing heading.** The owner directive is that the guide is read in full before any code, and a subagent inherits the session-start checklist through no mechanism you can verify from the main thread, so the brief is the only place the requirement reliably reaches it.
- **A precondition MUST be written where it is read BEFORE the work, never in a "before you finish" or "when done" list.** Measured 2026-08-19: three fix agents were briefed with "Read `docs/contributing/ze-go-style.md` before writing Go" under a heading reading "Before you finish", which reads as a closing checklist item and arrives after the code exists. The instruction was present and still bought nothing.
- **The brief MUST require the agent to report whether it read the guide before writing.** Native subagent context and the agent report are the repository-visible evidence.

**MUST verify what a subagent reports; MUST NOT relay it as fact.** An agent's report is a claim, not evidence (`ai/rules/evidence.md`). Before acting on a finding or repeating it to the user, confirm the code it cites actually produces the behavior it describes.

**Anything the user MUST answer MUST stay in the main thread.** A subagent cannot hold a dialogue with the user, so `/ze-spec` and `/ze-design` question gates, scope reductions, and RFC-compliance escalations (`ai/rules/rfc-compliance.md`) are raised by the main thread and MUST NOT be delegated away.

**Delegation never dilutes the independence of review.** Reviewer subagents MUST be spawned separately from the implementation agent and MUST NOT be given the implementer's reasoning as their starting point ("Critical Review Is the Central Deliverable", below).

**Delegation does not override phase-to-model boundaries.** Subagents inherit the PHASE, not the task shape ("Model Selection by Work Phase", below), so the main thread MUST still announce a boundary and stop rather than delegating an implementation phase from a review session to get around the switch.

**MUST supervise THINLY: launch, verify the report against source, decide, gate the next phase. The main thread MUST NOT run the exploration itself.** Exploration belongs in an agent because only its report needs to survive into the supervising context (`ai/rules/context-economy.md`).

**A main thread whose context passes 600k MUST write its per-spec state file and hand off rather than continuing.** Resolve the newest file with `./le spec session state latest spec <spec-stem>`.

**Implementation MUST be delegated ONE agent per implementation phase, not one agent per spec.** Give each agent the spec path, the phase it owns, and the per-spec state file; it writes its handoff there when the phase is green, and the next agent reads that file instead of re-deriving the phase before it. Measured: implementation agents ran 144 API calls each at 294k mean context, more of both than any other phase, because context grows with turns inside one agent.

**A work-package boundary is chosen at DECOMPOSITION, and it is never a license to stop early.** An agent whose package turns out too big MUST report the size to the main thread, which re-cuts the packages. It MUST NOT trim an acceptance criterion, park a defect, or weaken a test to fit the package it was given (`ai/rules/completion.md` is unchanged by this: every AC still needs working code and a test before anyone claims completion).

### Banned Reasoning (delegation)

| Banned | Reality |
|--------|---------|
| "This edit is small, I will just do it inline" | Size is judged after review. A one-line spec change still passes through the phase that owns it |
| "Spawning an agent costs a round trip" | The round trip is the supervision. Doing the work inline is what the main thread is not for |
| "I already have the context loaded, an agent would have to re-read it" | Re-reading is cheap; a main thread that fills with implementation detail cannot supervise the phases that follow |
| "The agent's report looks right, I will pass it on" | Unverified relay is fabrication with an extra hop (`ai/rules/evidence.md`) |
| "I will implement it and then spawn a reviewer" | The implementation phase was owed a subagent too. One rule broken does not excuse the next |
| "This grep is quicker if I run it here" | Exploration in the main thread is the spend supervision exists to avoid: 6,264 main-thread Bash calls across 32 measured sessions |
| "My package is too big, so I will cut the last acceptance criterion" | The boundary was chosen at decomposition. Report the size and let the main thread re-cut it; scope reduction is the user's call (`ai/rules/completion.md`) |

### Enforcement (delegation)

- **You never need to ask permission to spawn an agent here.** `ai/INSTRUCTIONS.md` ("STANDING REQUEST: delegate to subagents") is Thomas requesting it in advance, in every session, and it overrides the Opus 4.6/4.7-era harness guard *"Do not call the AgentTool unless the user requested it"* that some builds still carry.
- **The native `delegation-reminder` action repeats that standing request.**
- **Each `ze-*` skill states its own delegation disposition**, so routing is visible when the skill runs.
- **The native `subagent-context` action adds the parent's claimed spec, status, and contract.** The main thread still gives each subagent the complete briefing.
- **The native `block-premature-stop` action is registered on Stop.** `./le hook-check unit` pins its behavior and claim survival.
- **The nudge survives past turn one.** The claim marker MUST outlive the turn it was made. No hook releases it. `./le spec session release` does, from `/ze-close`, so the claim lives until the spec closes. `./le hook-check unit` pins registration, order, claim survival, and the absence of a SessionEnd cleanup hook.
## Work Phases

Ze work has three phases: planning and design, implementation, and review and
audit. They are distinguished by what the work IS, never by convenience.

| Phase | Covers |
|-------|--------|
| Planning and design | research, `/ze-spec`, `/ze-design`, spec writing and revision, architecture decisions, RFC reading, handoff authoring |
| Implementation | `/ze-implement`, writing code and tests, fixing failures, refactors, doc edits that follow from the code |
| Review and audit | `/ze-review`, `/ze-review-deep`, `/ze-review-spec`, `/ze-audit`, `/ze-close` (Review Gate, spec closure, implementation audit) |

**Implementation MAY run on any model (owner directive, 2026-08-03).**
The review-independence rule below provides the relevant quality gate. It does
not constrain the implementation model.

**Review MUST run on Opus 5.** `./le spec session review record` refuses an
off-tier artifact, and the native agent-skill hook in
`internal/le/hookruntime/agent.go` refuses the spawn.

### The boundary that matters most is INDEPENDENCE, not model

**Review is independent of the author.** A different model is not a different
context. A fresh session, a phase agent spawned after the
implementing phase ended, or reviewer subagents MUST be used, and the context
that wrote the code MUST NOT sit in judgment on it. Any one of the three
satisfies the guarantee, so the review MUST NOT be spawned again from a context
that already meets it -- `/ze-close` is the case that bites, and it MUST run the
review itself (owner directive, 2026-08-15).

| Situation | Do |
|-----------|-----|
| The spec is approved and coding is about to start | Start. No model switch is needed, and no announcement is owed |
| Implementation is complete and the Review Gate is next | Spawn ONE closure agent and let it run the review itself, or hand off to a fresh session. Never review your own implementation inline, and never let the closure agent spawn readers of its own |
| A review or audit produces fixes | The fixes are implementation, so make them. The re-review that follows is a fresh pass, not the same context re-reading itself |
| You are mid-phase and the work has changed shape | Say so plainly and let the operator decide. Do not silently continue as if nothing moved |
| The work is a one-line mechanical edit with no design or review content | Proceed. This rule governs phases, not keystrokes |

This never overrides "Critical Review Is the Central Deliverable" below.

### Two-Session Handoff (opt-in)

- **A spec whose metadata carries `| Handoff | verify |` is implemented, COMMITTED and stopped by one session, then reviewed and closed by another.** The row is declared before implementation starts, because not every spec is worked this way. Absent, or `-`, closure stays in the implementing session and nothing below applies.
- **The implementing session MUST set `| Status | verification |` before it commits, and MUST stop after the commit.** That status says the code is written, tested and in git, and that the spec awaits an independent review. It MUST NOT be used to park unfinished work: every acceptance criterion is implemented and green first (`ai/rules/completion.md`).
- **The handoff commit MUST carry neither a `plan/learned/` file nor a removal of the spec.** Either one makes `internal/le/commit` read it as a closure commit and demand the Review Gate artifact, which the implementing session MUST NOT produce over its own work.
- **This mode serves review INDEPENDENCE (above), and that is the only thing it buys.** The reviewing session reads a committed diff it did not write. A same-session close cannot give that.

| Session | Skill | Commits it produces |
|---------|-------|---------------------|
| Implementation, any model | `/ze-implement` | ONE commit: code, tests, docs, and the spec at `Status: verification`. Then `./le spec session release`, report the SHA, stop |
| Review and closure, Opus 5 | `/ze-close` | commit A (journal row, spec, closure edits) and commit B (`git rm` the spec), after a Review Gate over that committed diff |

### Subagents

- Subagents inherit the PHASE, not the task shape: reviewer subagents spawned during review stay on the review model, implementation subagents stay on the implementation model.
- The `Agent` tool's `model` parameter selects a family (`opus`, `sonnet`, `haiku`), not a minor version, so it cannot pin 4.8 against 5. The phase-to-model mapping above is about the session driving the work.
- MUST NOT downgrade a subagent to a cheaper model because its lens looks mechanical. `ai/skills/ze-review-deep.md` and `ai/skills/ze-debug.md` spawn every agent on `opus` for this reason. If cost forces a reduction, cut the NUMBER of agents, never the model they run on.

### Banned Reasoning (model phases)

| Banned | Reality |
|--------|---------|
| "I am already here, I will just implement it" | The phase changed. The model has to change with it |
| "It is a small implementation, review can stay on the same model" | Size is judged after review, not before |
| "Switching costs a round trip" | The round trip is the point. It is the boundary |
| "The review model can write the fix faster" | Then the fix is unreviewed work written by the reviewer. Two rules broken, not one |

### Enforcement (model phases)

- **No gate blocks an implementation edit by model.** An implementation phase MUST NOT be refused on the model that runs it.
- **Review is gated at both ends.** The native agent-skill hook refuses a review spawn on the wrong model, and `./le spec session review record` refuses the artifact.
- **A subagent inherits the phase, not the task shape.**
- **The record gate takes `model-override <reason>` only on operator instruction.** It MUST NOT be passed on your own judgement.
- **Both gates share `internal/le/specsession/model.go`.**

## Spec Selection

One spec at a time per session.

## Plan File Location

Prefer writing a spec (`plan/spec-<task>.md`) over a plan file.

## Creating a Spec (BLOCKING)

**MUST start from `plan/TEMPLATE.md`.** Read the template, copy its full
content, then fill in relevant sections and leave others as `(fill during
design)` placeholders. MUST NOT write a spec from memory -- the `validate-spec`
hook rejects files missing required section headers, and writing from scratch
always misses some. One read of the template before the first Write avoids the
rejected-then-rewrite cycle.

**Two templates, one per lifecycle half.** `plan/TEMPLATE.md` is design-time:
everything that MUST exist BEFORE code. The closure half lives in
`plan/TEMPLATE-CLOSURE.md` and is appended by `/ze-close` at step 1. MUST
NOT copy the closure sections into a new spec: measured across 161 specs,
sections copied at creation but used only at closure arrived there untouched in
65-75% of in-progress specs, while the sections authors added when they needed
them were untouched in 0%. Distance from use is what empties a section.

**Placeholders MAY appear only at `skeleton`.** A deferral holder fills `## Task`
and leaves the rest ("Creating the Deferral Spec", below). From `design` onward
the native placeholder guards in `internal/le/hookruntime/lifecycle.go` block,
because the status is a claim that those sections are written.

**One verification command.** The spec's Goal Gates name `./le verify worktree`, the
pre-commit gate (`ai/rules/precommit-verify.md`). Fast targets are for the inner
iteration loop and MUST NOT appear as the gate.

## Pre-Implementation

```
── RESEARCH ── (read, search, understand -- no code)
   Gate: Name 3 related files + describe current behavior.

[ ] 1. Check existing spec: plan/spec-<task>.md
[ ] 2. Read ai/INDEX.md for doc navigation
[ ] 3. Scan plan/spec-*.md for related specs
[ ] 4. Match keywords → docs (INDEX.md tables)
[ ] 5. Read identified architecture docs
[ ] 6. RFC check: verify rfc/short/rfcNNNN.md exists; create if missing
[ ] 7. Read docs/contributing/rfc-implementation-guide.md (protocol work)
[ ] 7. Read ACTUAL source files -- document current behavior
      BLOCKING: cannot write spec without "what does existing code do?"
[ ] 7. Trace data flow (rules/data-flow-tracing.md)

── DESIGN ── (write spec, get approval)
[ ] 7. Document existing behavior (preserve unless user says change)
[ ] 7. TDD planning -- identify tests BEFORE implementation
[ ] 7. Present plan -- WAIT for approval
[ ] 7. Write spec using plan/TEMPLATE.md -- complete Pre-Spec Verification first

── IMPLEMENT ── (TDD cycle)
[ ] 14. Test fails → implement → test passes. Log mistakes immediately.

── SELF-REVIEW ── (adversarial, BEFORE presenting to user)
   Gate: Adversarial Self-Review (rules/quality.md) -- all 5 questions answered, fixes applied.
[ ] 14. Run adversarial self-review. Fix what it reveals. Do NOT present work yet.
[ ] 14. Check for unanswered questions from earlier in conversation. Re-state them.

── VERIFY ── (complete checklist, present evidence)
[ ] 14. Complete Completion Checklist -- all 12 steps, in order, no skipping.
[ ] 14. Present work with evidence. Do NOT suggest committing.
```

## Implementation Plan Format

Present BEFORE writing code. Must include: docs read + insights, current behavior (source files, behavior to preserve/change), TDD plan, implementation phases, files affected, data flow, design decisions, RFC references (protocol code).

**WAIT FOR USER APPROVAL.** During design discussions (naming, alternatives, approach),
present options and wait. MUST NOT edit files until explicitly approved.

## Spec Rules

- **Style:** Tables and prose, never code (`ai/rules/spec-no-code.md`)
- **Editing:** Append-only. Strikethrough + reason for superseded content.
- **Deletion allowed:** writing the journal row, user requests, typo fixes only.
- **Research capture (MUST DO):** All findings from RESEARCH phase go in spec exhaustively -- file surveys, function lists, split decisions, reasons for NOT splitting. Spec is single source of truth. Implementation sessions execute from spec alone.

## Risks & Assumptions (BLOCKING for new specs)

Every spec carries a `## Risks & Assumptions` section (see `plan/TEMPLATE.md`) that is
written during RESEARCH/DESIGN and kept live through implementation. Concerns raised at
/ze-spec gates (assumption challenge, Failure Mode Analysis) MUST be written into these
tables, not just spoken at the gate -- gate conversation does not survive the session.

| Table | Captures | Lifecycle |
|-------|----------|-----------|
| Assumptions (A-N) | Beliefs the design depends on, with Basis and a validation method | `unvalidated` → `confirmed` or `broken`. Validate cheap ones (grep/read) during the /ze-implement audit, before coding. |
| Risks (R-N) | Failure modes that exist even if assumptions hold, with early signal + mitigation | Reviewed at each phase; surviving risks copy forward to the Executive Summary and, when one is owed, to the journal row. |

Rules:

- An assumption without a validation method is a guess. Name the test, grep, or user confirmation that would settle it.
- A `broken` assumption gets a Mistake Log "Wrong Assumptions" row and a Deviations entry. If it invalidates the approved design, STOP and present to the user.
- No assumption MAY still be `unvalidated` at Pre-Commit Verification (the spec's "Assumptions Resolved" table records final status with evidence).
- Existing specs (created before this rule) are exempt; do not retrofit without user request.

## Spec Sets

When multiple specs form a related set (umbrella + child specs), use a shared prefix with numbering:

| Pattern | Example |
|---------|---------|
| Naming | `spec-<prefix>-<N>-<name>.md` |
| Umbrella | `spec-utp-0-umbrella.md` |
| Children | `spec-utp-1-event-format.md`, `spec-utp-2-command-format.md` |
| Done path | journal row in `plan/journal/<class>.md` naming the spec in the Spec column |

- **Prefix:** short mnemonic for the effort (e.g., `utp` = unified text protocol)
- **Number:** 0 = umbrella, 1+ = children in execution order
- **Cross-references:** all specs in a set MUST reference siblings by filename
- **Selected spec:** point to the umbrella; select children individually when implementing

## Spec Metadata (BLOCKING)

Every spec MUST have a metadata table immediately after the `# Spec:` title. This is the source of truth for spec status, parsed by `./le spec status` and validated by `hookValidateSpec` in `internal/le/hookruntime/lifecycle.go`.

| Field | Purpose | Values |
|-------|---------|--------|
| Status | Current state | `skeleton`, `design`, `ready`, `in-progress`, `verification`, `blocked`, `deferred` |
| Handoff | Who closes this spec | `verify` for the two-session handoff, `-` for closure in the same session |
| Depends | Blocking prerequisite | Spec filename (e.g., `spec-rib-04`) or `-` |
| Phase | Multi-phase progress | `N/M` (e.g., `3/5`) or `-` for single-phase |
| Updated | Date of last status change | `YYYY-MM-DD` -- NOT last file edit |

### When to Update (BLOCKING)

Status transitions happen at the BEGINNING of the phase, not at the end.
A spec that stays in `design` during implementation is lying about its state.

| Event | Status change | Phase | Updated | When exactly |
|-------|--------------|-------|---------|--------------|
| Start research | `skeleton` to `design` | - | Yes | When research begins |
| Spec approved | `design` to `ready` | - | Yes | After user approves design |
| Start coding | `ready` to `in-progress` | Set `1/N` | Yes | When coding begins |
| Finish a phase | - | Increment | Yes | After phase tests pass |
| Hand off for review (`Handoff: verify` only) | `in-progress` to `verification` | - | Yes | Before the implementation session commits and stops |
| Blocked | to `blocked` | - | Yes | When blocker identified |
| Deferred | to `deferred` | - | Yes | When user agrees to defer |

### Status Vocabulary

| Status | Meaning |
|--------|---------|
| `skeleton` | Task defined, design not started |
| `design` | Research/design in progress |
| `ready` | Design complete, ready for implementation |
| `in-progress` | Actively being implemented |
| `verification` | Implementation complete and COMMITTED, awaiting an independent review and closure on Opus 5. Reached only under `Handoff: verify` (see "Two-Session Handoff") |
| `blocked` | Waiting on prerequisite (see Depends) |
| `deferred` | Explicitly postponed |

### Viewing Status

`./le spec status` shows the full inventory table. Use `./le spec status | json` for machine-readable output.

## Pre-Spec Verification

```
[ ] Metadata table present with valid Status, Depends, Phase, Updated
[ ] INDEX.md keyword table checked
[ ] RFC summaries exist for all referenced RFCs
[ ] Template format followed (🧪 emoji, tables not prose)
[ ] Checkboxes use [ ] not [x]
[ ] No code snippets
[ ] Files to Modify includes feature code, not only tests
[ ] Current Behavior section completed
[ ] Data Flow section completed
[ ] AC-N table rows with testable assertions
[ ] Risks & Assumptions filled -- every assumption has Basis + validation method; failure modes recorded as risks
[ ] Required Reading has → Decision: / → Constraint: checkpoints
[ ] All research findings captured exhaustively
[ ] CLI grammar: if adding CLI commands, Integration Checklist marks "CLI grammar" as needed
[ ] Doctor checks: if adding runtime dependencies, Integration Checklist marks "Doctor check" as needed
```

## Retroactive Specs

If a spec describes work that is **already implemented**, run the full Completion Checklist immediately -- audit, append the journal row to `plan/journal/<class>.md`, include it in the same commit as the code. Never commit a spec in `plan/` for work that's already done.

## Completion Checklist

After all tests pass, complete IN ORDER:

```
[ ] 1. Documentation updates -- check Documentation Update Checklist below.
      Every question must be answered Yes/No. Every Yes requires a file path.
      BLOCKING: code that changes documented behavior without updating docs is not done.
[ ] 2. Env var check -- if YANG config leaves were added under `environment/`,
      verify matching `ze.<name>.<leaf>` env vars are registered via `env.MustRegister()`.
      Run `ze env registered` (or grep for `MustRegister`) to confirm.
[ ] 3. Dead code check -- search unused functions/types, ASK before removing
[ ] 3. File modularity check -- for each modified .go file:
      Line count: >1000 → review concerns, split only when the separation is right (rules/file-modularity.md)
      // Design: topic annotation still matches file's actual concern?
      If split: copy to new files, adjust annotation per new concern
      // Related: still accurate? Add/update for new couplings
      (rules/design-doc-references.md, rules/related-refs.md)
[ ] 4. Implementation Audit (BLOCKING -- rules/implementation-audit.md)
[ ] 5. Pre-Commit Verification (BLOCKING -- do NOT trust the audit)
      Re-read spec from scratch. For each item, independently verify:
      - Files Exist: `ls` every file from "Files to Create" -- paste output
      - AC Verified: for each AC-N, grep/test for fresh evidence -- do NOT copy from audit
      - Wiring Verified: read each .ci file, confirm it tests the claimed path
      - Assumptions Resolved: every A-N row is `confirmed` or `broken` with evidence --
        none `unvalidated`; broken ones have Mistake Log + Deviations entries
      Fill the "## Pre-Commit Verification" section in the spec.
      The closure review and `./le commit create` consume the completed spec.
[ ] 6. Critical Review (BLOCKING -- rules/quality.md)
[ ] 7. Review Mistake Log -- check MEMORY.md, promote if seen before
[ ] 7. Update spec -- Implementation Summary, Documentation Updates, Deviations
[ ] 7. Write journal row: append a row to `plan/journal/<class>.md` naming the spec in the Spec column
[ ] 7. Verify: `./le verify worktree` + git status + git diff, no unintended changes
[ ] 7. Executive Summary Report -- present to user with what was done and what is left (including deferred).
        BLOCKING: journal row (step 10) must exist. Name the file in the report.
        Do NOT ask to commit. The user will tell you when to commit.
[ ] 7. Commit (when user says so) -- ONE helper-generated script, TWO commits (per Spec Closure below):
        - **Commit A:** `./le commit create replace` with `--file` for all implementation files (code, tests, docs, schema)
          + `--file plan/journal/<class>.md` + `--file plan/spec-<name>.md` (preserves edits)
        - **Commit B:** `./le commit create append remove plan/spec-<name>.md` (spec closure)
        Run the generated script yourself and the work is done. There is no
        second step. If spec closure or the journal row is missing, it never happens.
        Disjoint systems (e.g., CLI and BGP encoding) get separate commits.
```

## Review Gate (BLOCKING)

Before final testing/verify, run a code review against the diff. Fill the
`## Review Gate` section in the spec with the findings list. If ANY finding
is severity BLOCKER or ISSUE (anything above NOTE), fix it and re-run the
review. Loop until the review returns only NOTEs (or nothing). Paste the
final clean review output into the spec. NOTE-only findings do NOT block.

**Each round reviews less than the last, and the loop MUST end.**
Round 1 covers the whole diff. Round N+1 covers only round N's fixes. A gate
that cannot stop is a gate that gets bypassed. One place settles what happens
to a finding outside the round's scope, which classes are always in scope, and
where a homed finding goes: "Bounding the loop", below. MUST NOT restate those
tests here. A second copy is how the corrected rule and the defective one
become one hop apart.

## Critical Review Is the Central Deliverable

Review is not the last box before commit. It is the highest-leverage step in
development, and it is the one most easily faked. This rule makes it independent,
evidenced, and structurally unskippable. Rationale and the failure that motivated
it: `ai/rationale/critical-review.md`.

### The one load-bearing rule

**A review MUST be performed by a DIFFERENT context than the author.** Independent
review subagents (`Agent` / `fork`) over the actual diff, or a fresh session.

**Your own inline reasoning about code you just wrote MUST NOT count as a review.** The
author is the one party guaranteed to share the blind spot that produced the bug.
Writing "I checked it, 0 issues" into a Review Gate from your own analysis is the
exact failure this rule exists to stop. It has shipped real bugs that independent
reviewers caught on the same diff minutes later.

### What a real review pass is

1. **Independent, and independence is a property of the CONTEXT, not of the agent
   count.** The reviewer MUST NOT be the context that wrote the code. A fresh
   session satisfies that, and so does a phase agent spawned after the
   implementing phase ended. What is required is ≥2 distinct LENSES over the diff
   (logic/wiring/removed-behavior; security/edge-cases/test-quality; the
   feature's own risk area), each reading the PRODUCER rather than the caller and
   verifying claims against source (`ai/rules/evidence.md`). One independent
   context MAY run every lens itself.
   **`/ze-close` MUST run them itself and MUST NOT spawn (owner directive,
   2026-08-15).** Its closure agent already satisfies the independence condition,
   so a spawned reader adds a hop and a full startup cost without adding a lens.
   Spawn reviewers only from a main thread invoking `/ze-review` directly, where
   the alternative is reviewing inline in the context that authored the diff.
2. **Adversarial.** The question is "what can go wrong that nobody planned for?"
   Default findings PLAUSIBLE, not dismissed. MUST NOT discard wiring, removed-guard,
   logic, or vacuous-test findings.
3. **Verify the reviewers too.** A reviewer can be wrong. Before acting on a
   finding, reproduce it (an empirical check beats an argument: a `.ci` exit
   assertion that "SHOULD fire" either fires or does not; run it).
4. **Looped to zero over a SHRINKING scope.** Every fix is new code and earns a fresh pass. Each pass reviews less than the one before it. Five passes are the session's to spend. The sixth is Thomas's to grant. Each pass carries a hard bound on what it covers. See "Bounding the loop" below.
5. **Evidenced by an artifact, not narrated.** Record the pass with
   `./le spec session review record` → `tmp/review/<spec-stem>-<session-id>.md`
   (session-scoped, so concurrent same-spec sessions never clobber each other). It pins the
   SHA-256 of every code/test file the reviewers examined. The spec's Review Gate
   section pastes the reviewers' actual findings and each fix.

### Bounding the loop

- **Round 1 reviews the whole diff. Round N+1 reviews ONLY the fixes round N made, plus what those fixes touched.** By default, a finding outside that scope does not re-open the round. Three bullets below override that default. Each override costs another pass (step 4). The overrides are: the goal depends on it, you are unsure whether it does, or it belongs to the always-in-scope list.
- **The loop ends when a round finds no BLOCKER and no ISSUE inside its OWN scope, AND no always-in-scope finding anywhere.** Both halves are required. The scope half alone lets an unconditional class satisfy the end condition by surfacing outside the round. A NOTE MUST NOT re-open a round, wherever it was found ("Review Gate", above). An always-in-scope finding is NEVER a NOTE, and its severity floor is ISSUE. Severity is the reviewer's own call. Without that floor, tagging one down is the cheapest exit from a list whose purpose is to have no exits.
- **The loop never required a round that finds nothing anywhere.** On a diff of any size, a full-diff pass always finds something. That reading has no state in which it stops, so finished work cannot close.
- **A finding outside the round's scope is fixed in this round when the goal this work exists to achieve depends on it. Otherwise it gets a spec, the work in hand closes, and Thomas decides whether that spec runs.** That is `ai/rules/completion.md`'s question unchanged. The test is DEPENDENCY, never causation. A defect this change did not introduce is in scope the moment the work depends on that path, which is what "pre-existing" never excuses.
- **If you are unsure whether the goal depends on it, you are on the fix-it side.** `ai/rules/completion.md` sets that tie-break and this bound does not soften it. Over-fixing costs some work. Homing a real blocker ships something that does not do what it claims. A rule that licenses closure is where an unsure call MUST fall towards fixing.
- **Eight classes are ALWAYS in scope, whatever round surfaces them and whoever caused them: an unwired symbol, a vacuous test, an acceptance criterion with no test, a user-facing behavior with no functional test, Linux-only code with no QEMU test, a removed guard, a newly added guard that fails open, and any RFC or interop non-conformance.** Each one passes a "no wrong result, no red gate" screen because its failure mode is silence. Nothing is wrong on the surface. The path is never exercised.
- **Where the round's scope and that list disagree, the list wins and the loop takes another pass.** The scope bound is a rung-3 instrument (`ai/rules/rule-precedence.md`). Conformance owed outside this repo sits on rung 2. Nothing about bounding a review loop CAN retire an RFC or interop obligation (`ai/rules/rfc-compliance.md`, `ai/rules/interop-and-goal-validation.md`).
- **Each class has its own authority.** Step 2 above covers wiring, removed-guard, logic and vacuous-test findings. `ai/rules/completion.md` covers an untested acceptance criterion, `ai/rules/testing.md` user-facing behavior, `ai/rules/platform-linux.md` Linux-only code, and `ai/rules/evidence.md` a guard that fails open.
- **The home is a destination spec that OUTLIVES this closure, never this spec's own deferral shard.** A shard whose rows are all terminal is `git rm`d at closure ("Deferral Tracking", below), and a row written into it minutes before closing is either resolved by that closure or is the thing keeping the shard alive. Neither outcome is a home: the shard records where a row came FROM. Name a `plan/spec-*.md` that exists on disk.
- **Two readings, and the one that governs.** "Fresh eyes on every pass, the full diff each time" asks a pass to see the whole change. "Loop until a pass finds nothing" asks the loop to converge. Applied to every round at once they contradict, and the agent that tries to satisfy both cannot close its work. Round 1 owns the whole-diff reading. Rounds 2 and later own convergence.
- **Write the round's scope down BEFORE the round runs, in the spec's Review Gate section.** Unwritten, "what those fixes touched" is chosen after the findings are known, and shrinks to whatever produces a clean round. Written first, it holds when the reviewer is tired, invested, or wrong about severity. It includes the sibling call sites of every changed function (`ai/rules/quality.md`, question 8), not only the edited hunks.

- **The review's subject is the PRODUCT. A false statement in the spec's own
  closure record is a NOTE, and a NOTE MUST NOT re-open a round.** Wrong arithmetic
  in an Audit Summary, a pasted command output that was condensed, a status word
  that contradicts the shard, a count nobody can reproduce: each is worth fixing
  and none of them ships. Collect every one of them, fix them in ONE edit, and
  MUST NOT spend a round confirming the fix.
- **The one exception is precise: a record defect is an ISSUE when it asserts a
  PRODUCT property that is false.** "This test discriminates" when it does not,
  "the guard fails closed" when it does not, "an interop test covers this" when
  none exists. Those are `ai/rules/evidence.md` false-safety-claim findings, they
  mislead the next reader about the code, and they keep their severity.
- **A round whose findings are ALL record defects is the last round.** The loop
  has stopped converging on the product: each prose fix creates fresh text to
  audit, so another round cannot establish product quality.
- **`./le spec session review record` takes `--rounds N` and refuses more than
  five without `--rounds-reason`, which MUST name the PRODUCT defect a later
  round found.** The cap is not a ban: a genuinely defective implementation can
  need a sixth round and gets one for the cost of a sentence. That sentence is
  the one nobody can write when the loop is auditing its own bookkeeping, which is
  what makes it the right toll.
- **Past FIVE rounds a session MUST NOT authorise itself. MORE THAN FIVE PASSES
  IS THOMAS'S DECISION** (owner ruling, 2026-08-17). `record` refuses a sixth
  round without `--owner-authorised` carrying what he said. `--rounds-reason`
  stays required alongside it. The product defect and his word are both owed,
  and neither substitutes for the other.
- **You MUST NOT set `--owner-authorised` on your own initiative.** The same ban
  covers `--push` on `internal/le/commit` (`ai/rules/git-safety.md`).
  At the cap you MUST stop. Report what the loop keeps finding, then ask him
  whether it runs another pass. A script cannot check who typed a flag. Setting
  it unasked is a recorded false statement about the owner, not a shortcut.

- **The same cut applies to TEST-ONLY code: a defect there that cannot reach the product is a NOTE, and a NOTE MUST NOT re-open a round.** Test helpers, fixture builders, `.ci` and `.et` scripts, the runners under `test/`, and the harness code that drives them ship in no binary an operator runs. An error branch nothing reaches, an edge case no caller has, a handle left open in a process that is about to exit: report it once if it is free to fix, and it earns no round, no spec, and no hold on a closure. Test code that runs and does its job is finished.
- **A bug in test code that leads to NO TESTING is load-bearing, and MUST be fixed.** The test does not run, the runner skips it and reports green, the harness never reaches the code under test, the fixture builds the wrong scenario, the assertion is swallowed, the `.ci` observer exits before it checks: nothing is being tested and the suite says otherwise. That is a silent loss of coverage, which is the failure mode the always-in-scope list exists for, so it is a BLOCKER or an ISSUE like any other.
- **The rest of the exception is the same cut one step on: a test defect keeps its severity when it changes what the test PROVES, or when it stops a gate refusing what that gate exists to refuse.** A test that cannot fail, one that passes while the behavior under test is broken, one that asserts the wrong value, an RFC-tagged test that no longer pins its requirement, a fixture that encodes a violation, a hook check that now lets its own class of mistake through: each damages the product's evidence rather than the harness, so the always-in-scope list above is unchanged.
- **Two readings, and the one that governs.** "Test code MUST be valid and correct" asks for a harness that runs and that tells the truth about the product. It does not ask for the product's own bar on the harness itself (`ai/rules/testing.md`, "Test Code Is Held to One Standard"). A round spent on an unreachable branch in a fixture builder found nothing the product can feel.

### State the review effort before you spend it

- **MUST name the pass count and the lenses BEFORE the first agent is spawned, so the operator can stop you.** An unannounced fan-out is a decision taken on the operator's behalf.
- **MUST match the effort to the ask ABOVE the floor step 1 sets, never below it: two lenses on round 1 always, three or more when the ask is "audit this" or "be thorough".** Round 1 is the only pass that ever sees the whole diff. Its lens count IS the whole change's coverage, and MUST NOT be cut to one. Effort is chosen from the request, never from how interesting the code turned out to be.

### Enforcement (critical review, structural: a hook, not discipline)

`internal/le/commit` refuses a spec-closure commit (one that adds a
`plan/journal/*.md` row naming the spec, or removes a `plan/spec-*.md`) unless `./le spec session review
check` passes: a CLEAN artifact exists, covers every reviewable file in the commit
(the ze-close closure commits all of a spec's code in commit A, so that is
full coverage), and its hashes still match (any edit after the review invalidates
it, forcing a fresh pass). A code-free closure still requires a clean artifact to
exist. Override with `--review-override <reason>` only as an explicit owner
decision (printed in the helper output alongside `--unverified`).

**What the hook can and cannot prove.** It proves a *fresh, hash-pinned, clean
artifact covering this commit's code exists*, so you cannot close by narrating
"0 issues" into the spec, and you cannot review then quietly edit. It does NOT
prove a genuinely independent context did the reviewing: the artifact is recorded
by convention (record the reviewer subagents' ids in `--reviewers`). It raises the
floor from "type clean into the doc" to "record a covering, still-matching review";
real independence rests on the skill mandate above, not on the gate. Known
residuals to not lean on: the coverage check only sees THIS commit (code committed
in earlier feature commits then closed code-free is under-covered, so commit all of
a spec's code at closure), and the check runs when the commit script is generated,
so code MUST NOT be edited after generating the script.

### Banned rationalizations

Each is a signal you are about to skip the independent pass. If you think one,
stop and spawn the reviewers.

| Banned | Reality |
|--------|---------|
| "I reviewed it as I wrote it." | That is authoring, not reviewing. Same blind spot. |
| "The tests pass, so it's correct." | Tests can be vacuous (dead exit codes, cumulative-match needles). A reviewer finds the vacuous test; the green bar does not. |
| "It's a small/mechanical change." | Renames collide roots; one-line guards fail open. Size is judged after review. |
| "I already know this code is correct." | The bug is precisely what you're sure isn't there. |
| "./le repository check / lint passed." | Those are mechanical gates. They are not a critical review. |
| "Re-running review is wasteful." | The fix is new code. Unreviewed new code is the next bug. |

### Scope

Every spec closure and every substantive code change runs this. Trivial doc-only
or generated-file-only changes are exempt (nothing to review). When unsure,
review.

## Spec Closure (BLOCKING)

**A spec that passes its Review Gate MUST NOT be considered done until it is deleted from `plan/`.**

The lifecycle is: `in-progress` -> Review Gate clean -> write journal row -> `git rm` spec.
Leaving a completed spec in `plan/` causes every future session to count it as open work.

**Closure MUST use TWO commits, ONE script.** The spec is edited during implementation (design notes,
status updates, corrected assumptions). Those edits are valuable design history.
`git rm` destroys the working copy. If the edited spec is never committed before
deletion, the design work is lost from git history forever.

The helper-generated commit script MUST produce two commits:
1. **Commit A (implementation + spec):** `./le commit create replace`
   with `--file` for all code, tests, docs, journal row,
   AND the spec file itself (with all edits from implementation).
2. **Commit B (spec closure):** `./le commit create append remove plan/<spec>` only.
   If the spec has a deferral shard AND every row in it is terminal,
   `--remove plan/deferrals/<spec-stem>.md` in the SAME commit B: deferrals are
   sharded per source ("Central Log", below). **A shard still holding a
   live row is NOT removed.** That row is homed at a different spec, and the shard is
   only where it is written down, so removing it deletes a record of live work. Read
   the Status column before you add the `--remove`; an all-terminal shard is residue
   and goes, a live-bearing shard survives its source spec and keeps its name.
   **This extends to FOREIGN shards this closure emptied.** Resolving a row homed
   here can set the last live row of another spec's shard to `done`; that shard is
   now residue and the same commit B removes it. Nobody else will: every other
   closure scopes its `--remove` to its own stem.

This preserves the final spec state in git history. `git log -p -- plan/<spec>` shows
the full design record. The deletion in commit B is a clean removal of a file whose
final state is already committed.

**EVERY reference survives closure, not only `// Design:`.** Before commit B, MUST grep
the WHOLE PATH `plan/spec-<stem>.md` across the tree, not the `// Design:` prefix,
and rewrite every hit to the appropriate destination (an `ai/rules/` file, a
`docs/architecture/` page, or one of the three `plan/learned/` aggregates:
`DESIGN-HISTORY.md`, `HOOK-FRICTION.md`, `RECURRING-PATTERNS.md`) inside commit A.

Three gates read three different reference kinds, and greping for one leaves the
other two red:

| Gate | Reads | Missed by a `// Design:`-only grep |
|------|-------|-------------------------------------|
| `./le doc check links` (`internal/le/doccheck`) | `// Design:` lines and tracked path citations | no |
| `./le spec citation` (`internal/le/speccitation`) | every `plan/spec-*.md` citation inside a spec | yes |
| `internal/le/doccheck.CheckLinks` tracked-citation pass | every tracked path citation, including a `plan/spec-*.md` target | yes |

A grep limited to `// Design:` misses other spec citations and dead learned-summary paths. Closure must search every citation form.

**Then re-read what the substitution produced.** A bulk repoint turns a sentence
ABOUT the spec into a sentence about its destination, and some of those become
false: a rule page, an architecture page and a journal row do not "own rows", are
not "active", have no "phase 2b", and are not somewhere work can be "implemented
inside". MUST grep the new path next to `that spec`,
`this spec`, `the pilot`, `owns`, `active` and `Depends`, and rewrite what reads
wrong. Naming the spec WITHOUT its `plan/` path is the way to keep a true sentence
about a file that is gone: the citation gate matches the path, not the name.

**A spec-to-spec citation has three repairs, and the baseline is the last of them.**
Repoint the citation at the durable document that replaced the spec. Restate the
fact inline. Add the stem to `plan/.citation-baseline` when the citation is a
historical record of the closed spec. All three ride on commit A, because commit
B removes a spec and adds nothing. A citation that still has a live source MUST be
repointed or restated; the baseline MUST NOT absorb it. `./le spec citation`
MUST pass after the repair.

**Closure resolves the spec's deferral rows.** Before commit B, grep
`plan/deferrals/` for this spec's filename (a row naming it as Destination MAY live in
any source's shard, not only this spec's own). Every row naming it as **Destination**
MUST be resolved inside commit A: set Status `done` and Destination to the journal
class file (`plan/journal/<class>.md`), which is where the knowledge now lives. This
is separate from the shard removal in commit B, above: this resolves rows that POINT
AT the spec, which closure MUST do because it is deleting their destination. It does
NOT retire the rows the spec SOURCED. Those are governed by commit B's condition:
a sourced row homed at another spec stays live, and its shard outlives this closure
("Deferral Tracking", below). Only an all-terminal shard is removed.

Why: closure DELETES the spec, and `deferral_unassigned_problems`
(`internal/le/commit`) checks that every live row's destination exists on
disk. A row left pointing at a closed spec can therefore never be satisfied: it
dangles forever, is reported on every future commit (as a WARNING: that gate is
advisory and does not block), and the next reader cannot tell whether the work was
done or silently lost. Closure must enforce both compatible rules:
"destination must exist" and "closure deletes the spec".

Resolving the row is not a claim that the deferred work was implemented. It records
that the deferral has a permanent home. If the record you resolve it to does NOT
carry the item (check, do not assume: a Review Gate NOTE on a deleted spec
evaporates, and a journal row states one symptom and one fix, nothing else), then
the row has no home: keep it live and give it a real destination spec per "Choosing
the Destination Spec", below. Never resolve a row to a record that does not mention
it: that is the fail-open the gate exists to catch (`ai/rules/evidence.md`).

**MUST NOT `git rm -f` a spec without committing it first.** The `-f` flag silently
discards uncommitted edits. If the spec was modified during implementation (it
almost always is), those modifications MUST be committed before deletion.

| Banned | Why |
|--------|-----|
| "I'll close it later" | Later never comes. Other sessions see it as in-progress. |
| `git rm` a spec while a deferral row still names it as Destination | The row dangles forever. Nothing blocks: the gate is advisory, so it is reported and ignored. Resolve it in commit A. |
| `git rm` a deferral shard that still holds a live row | Deletes the record AND silences every observer of it, because they all fold over the directory. `deferral_shard_removal_problems` blocks this one ("Central Log", below). |
| Resolving a row to a record that never mentions the item | Fail-open bookkeeping: the row goes quiet and the knowledge is lost. Verify the record carries it. |
| "The user will handle it" | The user asked us to implement. Closure is part of implementation. |
| `git rm` in the same commit as implementation | Spec edits are lost from history. Two commits required. |
| `git rm -f` without a prior commit of the spec | Destroys uncommitted design work. |
| "Run the commit, then I'll prepare closure" | The user will not ask. One script, one run, done. |

### Closure Enforcement (automated)

Closure once depended on remembering the two-commit step, so it was routinely
dropped and specs piled up in `plan/` as false "open work". Three mechanical
gates exist for it, and all three run. The `Stop` array in
`.claude/settings.json` is the authority on the hook gate:

| Gate | Where | Fires when |
|------|-------|-----------|
| Detector | `./le spec status closure` | `--list` reports completed-but-not-closed specs in two tiers; `--spec <s>` exits 3 only for a high-confidence one. High confidence = a **committed** journal row in `plan/journal/*.md` whose Spec cell exactly equals the spec stem, or a `plan/learned/NNN-<slug>.md` whose slug exactly equals the stem, while the spec is still `in-progress` and is **not an umbrella** (commit A ran, commit B did not). Weaker `[umbrella]` / `[weak-match]` candidates are listed under NEEDS VERIFICATION. Only the high-confidence set triggers the `--spec` block. |
| Stop-hook block | native `block-premature-stop` action in `internal/le/hookruntime/lifecycle.go` | This session claimed a spec, the detector exits 3 for it, and no acknowledgement exists. The hook refuses the stop. |
| Commit reminder | `internal/le/commit` | A commit adds a journal row or learned summary but removes no spec: it prints the closure-commit reminder to stderr. |

Run `./le spec status closure list` any time to see the backlog.

## Spec Preservation

Rationale: `ai/rationale/spec-preservation.md`

**MUST discard:** Audit tables, checklists, post-compaction instructions, BLOCKING markers, status columns, template scaffolding.

The original spec in `plan/` is deleted after the journal row is written, but the completed spec MUST be committed to git first so it is preserved in history.

Never delete the spec without committing it first. A spec that was never committed is lost forever -- its audit tables, verification evidence, and design decisions cannot be reviewed.

Principle: transform scaffolding into knowledge. See "Writing Journal Rows" below for what a row holds and when one is owed.

## Verify Specs Against Code (BLOCKING)

Never report spec progress by reading the spec alone. Grep the codebase to verify
claims. Spec "What Remains" and "Implementation Summary" sections go stale. Before
reporting any item as unimplemented, search for the function/type/test in the code.
If it exists, the spec is stale, not the code. Update the spec to match reality.

## Deferred Work (BLOCKING)

See "Deferral Tracking" below for the full deferral process and log format.

Before marking a spec done, for every deferral: verify the receiving spec exists, has the deferred item listed, and the deferral is recorded in the current spec's Deviations section.

## Spec Triage

**Before working a backlog spec, you MUST decide what it is: a defect in the
shipped product, a gap in the evidence that the product is correct, or an
improvement. The three carry different urgency and only the first two can hold a
release.**

**The triage verdict MUST be derived from the tree, never from the spec's own
Task or Status. A spec states what its author believed on one day, and a
backlog accumulates specs whose defect another change already fixed.**

**A spec judged an improvement MUST move out of the release backlog, and a spec
whose subject no longer exists MUST be raised for deletion rather than filed.
Filing work nobody can start costs every later reader who picks it up.**

## Deferral Tracking

**Obligation on you (not a hard gate):** Every decision to not perform in-scope work MUST be recorded AND land in a destination spec.
Rationale: Untracked deferrals are invisible scope reductions. They accumulate silently across sessions.
A deferral whose destination is prose ("later", "future work") is a deletion with a polite name.

**Home a deferral by making the DESTINATION record it. What the destination records, the row MUST NOT repeat.**
When homing writes a NEW spec for the item, that spec carries the date, the source, the item and the reason, so the row is duplicate bookkeeping and MUST be deleted in the same commit that adds the spec.
When homing points at an EXISTING spec, one of two things MUST happen: add the item to that spec and delete the row, or keep the row, because it is then the only link between the work and its home.
Rationale: the gate reports a row with NO destination, so a homed row is already silent and deleting it removes no signal.
The link stays greppable from the source side, because the destination spec names the source it came from.
A row deleted this way is not work dropped: `ai/rules/completion.md` still governs, and `plan/future/README.md` still refuses a defect.

The commit gate that checks homing **WARNS, it does not block** (see "Status Vocabulary (the gate reads this)"
and the gate note below). An unhomed deferral row is harmless to software behaviour: the
worst case is that it is committed too early or in the wrong commit. Blocking every commit on
it, including commits that never touched deferrals, and rows another session wrote into the
shared working tree, held real work back for no software reason. So the obligation to home
a deferral is a discipline the gate reminds you of, not one it enforces: the warning keeps an
unhomed row visible so it is not lost, but you are the one who must give it a home.

### Central Log

`plan/deferrals/` -- a sharded directory, **one file per source**, holding all
deferred work. There is NO single `plan/deferrals.md` and no committed aggregate: <!-- doc-links: ignore (the single file is deliberately retired) -->
the live backlog is a fold over the directory, computed on read (`/ze-status`) and
never stored. A stored aggregate would be a shared file every session appends to,
exactly the cross-commit hazard this layout removes (`ai/rules/git-safety.md`).

**Shard key.** Each row MUST live in the shard named for its source:

| Source of the row | Shard file |
|-------------------|------------|
| A spec (row's `Source` names `spec-<stem>`) | `plan/deferrals/<stem>.md` |
| Ad-hoc (no source spec) | `plan/deferrals/ad-hoc-<YYYY-MM-DD>-<sid>.md` |

A shard is a small markdown file with the six-column table header and only the rows
it owns. Add your row to the shard for its source (create the shard if it does not
exist); never touch another source's shard except to correct a row it owns. Because
each path has a single writer, `git add <shard>` stages only your row and git merges
disjoint shard creations without conflict.

**A spec's shard MUST NOT be deleted at the spec's closure unless every row in it is terminal.** Spec closure commit B ("Spec Closure", above) `git rm`s `plan/deferrals/<stem>.md` alongside the spec.

**Reading the Status column of the closing spec's OWN shard is a NEW step, and no earlier check covers it.** The grep closure already requires ("Spec Closure" above, "Closure resolves the spec's deferral rows") searches every shard for this spec as a **Destination**. It never reads the closing spec's own shard as a **Source**. MUST NOT assume the existing grep answered this question: it answers a different one.

**A shard that still holds a live row MUST survive its source spec, and keep its source-keyed name.** The row's home is the destination spec named in its Destination cell. The shard is only where the row is written down, so deleting the shard deletes a record of live work whose home is somewhere else entirely.

**You MUST delete a shard at closure only when all rows are terminal. A homed live row MUST remain until its destination is complete. You MUST run `internal/le/commit` instead of counting rows.**

**An orphaned shard is not a defect to sweep.** A shard whose `plan/spec-<stem>.md` is gone while live rows remain is the correct end state of the paragraph above, not leftover mess. MUST NOT bulk-delete orphaned shards to tidy the directory: read the rows first, and delete only a shard in which every row is terminal.

**`deferral_shard_removal_problems` (`internal/le/commit`) refuses the removal, so this is not honor-system: a shard MUST NOT be removed while any row is non-terminal.** It reads the shard at HEAD and BLOCKS when any row is non-terminal. It has to block rather than warn: every other signal over these rows folds across the `plan/deferrals/` DIRECTORY, so deleting a live-bearing shard LOWERS their counts instead of raising them, and the forbidden action is the one that silences every observer of the rows it destroys (`ai/rules/evidence.md`).

**An all-terminal orphaned shard is residue, and the actor who MUST delete it is the closer of the LAST spec that homed one of its rows.** Setting the final row to `done` makes the shard residue, so the same commit removes the file.

**A live row whose SOURCE spec closed still needs a real Destination, and that is the thing closure MUST check.** The source spec's disappearance makes a prose destination unrecoverable: nothing on disk is going to become "a future usability spec".

### When to Record

| Trigger | Action |
|---------|--------|
| Deciding work is "out of scope" | Record with reason |
| Moving work to another spec | Record with destination spec |
| Skipping a task item from a spec | Record with reason |
| Postponing for any reason | Record with reason |
| User asks to skip something | Record (user-requested, still tracked) |
| Finding a problem the work in hand does not depend on | Write its spec now, record the row against it, close the work in hand, then ask Thomas whether the spec runs (`completion.md`) |

### Table Format

```
| Date | Source | What | Reason | Destination | Status |
```

| Column | Content |
|--------|---------|
| Date | YYYY-MM-DD |
| Source | Spec filename, task description, or "ad-hoc" (also selects the shard, see "Central Log") |
| What | Specific work being deferred (not vague) |
| Reason | Why it is being deferred |
| Destination | Receiving spec filename (`plan/spec-*.md`), "cancelled", or "user-approved-drop" |
| Status | See Status Vocabulary below |

### Status Vocabulary (the gate reads this)

`deferral_unassigned_problems` (`internal/le/commit`) checks the
Destination of every row whose Status is NOT terminal. The terminal set is
`DEFERRAL_TERMINAL_STATUSES` in that file:

| Status | Meaning | Destination checked? |
|--------|---------|----------------------|
| `deferred` | Live: the work is outstanding. MUST name its home spec | YES |
| `open` | Live: synonym of `deferred`. Prefer `deferred` | YES |
| `done` | Terminal. The work landed, or the row was superseded | no |
| `cancelled` | Terminal. User decided not to do it | no |
| `resolved` | Terminal. Closed with evidence (a journal row, or the commit that landed the work) | no |

**A homed row MUST stay live.** The status answers "is this work still outstanding",
NOT "does it have a home". Homing is mandatory, so a live row is the NORMAL,
correct state of a deferral: it has a spec AND the work has not landed yet. It goes
`done` when the work is implemented, not when it is filed. A live row is not a
violation and is not a backlog of unfiled work.

This is the invariant the gate is built on: it re-checks every live row's
destination on every commit, so "outstanding work names a real spec" is surfaced
continuously (as a warning), for as long as the work is outstanding. Closing a row
early to quiet the warning hides the work from the only thing watching it.

`open` and `deferred` are synonyms, and the redundancy is a wart: it is what let the
gate and this rule teach different words in the first place. Do not add a third.
**Any word that is not in the terminal set is treated as live and checked**,
deliberately: the gate is a denylist of terminal states, not an allowlist of live
ones, so a status nobody has invented yet fails closed rather than slipping through
silently (`ai/rules/evidence.md`).

**Blind spot, stated rather than papered over:** a terminal status skips the
destination check entirely, so a `done` row whose Destination is prose is not
flagged. `done` is an assertion the gate trusts. That is tolerable only because
`done` means the work LANDED, so nobody is routed toward it while work is
outstanding, and its Destination is often a commit SHA rather than a file. A
row MUST NOT be marked `done` before the work lands: doing so both lies and
disables the check, which is why the row above stays live.

This table and `DEFERRAL_TERMINAL_STATUSES` must not drift apart. They did once,
and it cost: the gate tested only `status == "open"` while this rule's own prose
taught the word `deferred`, so rows written correctly per the rule were never
looked at. 23 live rows without a home had accumulated behind that hole.

### Rules (deferrals)

| Rule | Detail |
|------|--------|
| Always a destination spec | Every deferral names a `plan/spec-*.md` that exists on disk, whatever its Status. Only `cancelled` / `user-approved-drop` may name no spec |
| No prose destinations | "later", "future work", "a follow-up", "TBD" are not destinations. A destination is a filename |
| No vague What | "Edge cases" is not acceptable. Name the specific case |
| Record immediately | Do not batch. Record when the decision is made, not at commit time |
| Review at session end | Live rows are expected and fine. Check that each still names a real home, and close only the ones whose work actually landed |

The gate is one notch wider than this rule on purpose: it accepts any existing
`plan/**.md`, not only `plan/spec-*.md`. Do not use that slack. A destination is
a spec.

**`plan/known-failures/` MUST NOT be used as a destination** (`ai/rules/completion.md`).
A shard is the running log of an investigation you are still driving, so pointing
a deferral at one means "this red is somebody's problem later", which is the
parking this rule exists to prevent. A red test MUST be fixed. If the fix is genuinely
a separable piece of work, home it in a spec like anything else. In particular,
"fails under load" is a diagnosis and never a destination: the test asserts on
elapsed time, and that is fixed, not deferred.

### Choosing the Destination Spec (BLOCKING)

Deferred work ALWAYS has a destination spec. Decide which one in this order, at
the moment the deferral is made:

| Order | Action | Detail |
|-------|--------|--------|
| 1 | Find an existing spec that already covers the topic | Search `plan/spec-*.md` for the topic, and scan `./le spec status`. Prefer a `spec-finish-<subsystem>` / `spec-followup-<subsystem>` umbrella when one owns the area |
| 2 | If one exists, add the work to its `## Task` section | That spec is the home. Record the deferral with it as Destination, Status `deferred` |
| 3 | Only if no spec covers the topic, create a deferral spec | Named `plan/spec-<source>-deferred-<subtask>.md` (see below). Record the row with it as Destination, Status `deferred`, exactly as in step 2 |

An existing spec is preferred over a new file. Do not create a deferral spec to
avoid the grep.

**Both routes record a LIVE row.** Filing work in a spec is not finishing it, so the
row stays `deferred` and keeps naming its home until the work lands. MUST NOT close a
row at step 2 or 3: a `done` row is never destination-checked again, so closing it on
filing is precisely how the work stops being watched (see "Status Vocabulary (the gate reads this)").

#### Deferral Spec Naming (BLOCKING)

A spec created solely to hold work deferred out of another spec is named:

```
plan/spec-<source>-deferred-<subtask>.md
```

| Part | Content | Example |
|------|---------|---------|
| `<source>` | Stem of the spec the work was deferred FROM, without the `spec-` prefix | `bgp-rib-flush` |
| `<subtask>` | Short kebab-case name of the specific deferred work | `ipv6-coverage` |
| Result | | `plan/spec-bgp-rib-flush-deferred-ipv6-coverage.md` | <!-- doc-links: ignore (illustrative naming example, not a live spec) -->

- MUST use one subtask per file. Two deferrals from the same source spec are two files, not one file with two tasks.
- The name carries the provenance: a reader knows what dropped it and why the file exists without opening it.
- For ad-hoc deferrals with no source spec, `<source>` is the subsystem (`plan/spec-l2tp-deferred-session-teardown-race.md`). <!-- doc-links: ignore (illustrative naming example, not a live spec) -->
- **A source spec does not outlive the deferral.** Spec closure `git rm`s the spec ("Spec Closure", above), so `<source>` will usually name a file that no longer exists by the time someone picks the work up. That is correct and intended: the provenance lives in git history, and the deferral spec is the tracker now. But when the source spec is ALREADY closed at the moment you write the deferral spec (homing an old row), name `<source>` for the subsystem instead: a filename pointing at a spec nobody can open reads as a broken reference rather than as provenance. Record the closed source spec in the `## Task` section either way.
- This naming applies only to deferral holders. Specs written as intended work keep the normal `spec-<task>.md` / `spec-<prefix>-<N>-<name>.md` names ("Spec Sets", above).

#### Creating the Deferral Spec

| Step | Action |
|------|--------|
| 1 | Create the file from `plan/TEMPLATE.md` with `Status \| skeleton` |
| 2 | Fill only the `## Task` section with the points to complete, plus any constraint already known. Leave the rest as template placeholders |
| 3 | Name the source spec in the `## Task` section so the provenance survives |
| 4 | Record the deferral in its `plan/deferrals/<source>.md` shard with the new spec as Destination and Status `deferred` |

Keep it small. The goal is zero lost work, not a finished design: a skeleton is
captured intent, not a designed spec. It moves to `design` when someone picks it
up (status table in "Spec Metadata", above).

The commit gate `deferral_unassigned_problems` (`internal/le/commit`)
folds over every shard in `plan/deferrals/` and WARNS, it surfaces, it does not
block, on any LIVE deferral (any non-terminal Status, see Status Vocabulary) that
names no destination or names a spec file that does not exist, and on any row it
cannot parse. It is routed through
`commit_gate_warnings`, not `commit_gate_problems`: the message prints to stderr
and the commit proceeds. This is advisory by design, for the reason in the banner
above (an unhomed row is harmless to software; blocking unrelated and other-session
commits on it was too aggressive). Homing stays mandatory as an obligation on the
author; the warning is what keeps an unhomed or unparseable row visible so it is
not silently lost.

### Verify Before Deferring (BLOCKING)

Never claim "requires infrastructure that doesn't exist" without grepping for it first.
Before writing "deferred -- requires X" in any spec or journal row, grep for X. If it exists,
implement it. If genuinely missing, name the specific thing that is missing and where it
would need to be added.

### What Is NOT a Deferral

**The following MUST NOT be recorded as a deferral:**
- Completing work that was never in scope (no record needed)
- Choosing between two valid approaches (design decision, not deferral)
- Go `defer` keyword (language construct, excluded from pattern matching)

### Resolving Deferrals

A row is closed when the WORK is settled, never when it is merely filed.

| To close as | Set Status to | Set Destination to |
|-------------|---------------|--------------------|
| Implemented | `done` | Spec or commit where implemented |
| User decided not to do it | `cancelled` | `user-approved-drop` |
| Superseded (another row or spec now owns it) | `done` | The row or spec that took it over |

**Filing work in a spec MUST NOT be treated as a close.** Moving work into a
spec gives the row its Destination. The row stays `deferred` until the work
lands. If the work is not in the tree, the row MUST NOT be `done`.

## Executive Summary Report

Present to user when all work is complete. Format below.

The sections are a checklist of what to cover, never a quota to fill. A section
with nothing to report says "None" on one line. The whole report stays under
about 15 lines: what changed, what it means, what is not done. No investigation
narrative (`ai/rules/writing.md`).

```
## Executive Summary

**Objective:** [1-2 sentences -- what the work aimed to achieve, as understood]

**Changes:**
| File | What changed | Why |
|------|-------------|-----|
| path/file.go | Added X, modified Y | To achieve Z |

**Design decisions:**
- [Decision and reasoning, or "None, all choices were explicit"]

**Deviations:** [From spec/plan/instructions, or "None"]

**Not done:** [Scope boundaries, deferred items, or "N/A"]

**Risks & observations:**
- [Anything noteworthy for future sessions]

**Verification:** [Command run + result summary]
```

| Section | Purpose |
|---------|---------|
| Objective | Confirms alignment. If the goal was misunderstood, this is the last chance before it becomes a commit. |
| Changes | Per-file summary with *why*, not just *what*. `git diff --stat` says "planning.md +8 -5", which is useless. "Added modularity check as step 3, renumbered 4-10" is actionable. |
| Design decisions | Choices made during implementation that weren't explicitly dictated. The user should know what was decided on their behalf. "None, all choices were explicit" is valid. |
| Deviations | What differed from spec/plan/instructions and why. "None" is valid. |
| Not done | Explicit scope boundary. Prevents the assumption that everything related was handled. Surfaces deferred items. |
| Risks & observations | Things that might bite later: new coupling, stale references elsewhere, edge cases not covered, follow-up work needed. Start from the spec's Risks table (R-N rows that survived implementation): this section is a copy-forward, not an invention at the end. |
| Verification | What was run, what passed. Not "./le verify worktree passes" but actual output or specific test names. |

## Documentation Update Checklist (BLOCKING)

See `ai/rules/writing.md` for the canonical 12-row checklist.
Every row must be answered Yes/No. Every Yes must name the file and what to add.

- **A spec that CREATES a `docs/architecture/` page, or CHANGES a claim one of those pages makes about the code, MUST run `/ze-review-docs` over that page before it closes.** Record the pass in the spec's Pre-Commit Verification "Documentation Verified" table, which `/ze-close` already fills: name the page, the reviewing session, and the claims it checked.
- **A new page or a changed claim is the whole trigger. A typo, a link repair, a heading move, a rename and a formatting pass owe no reader**, because none of them states anything new about the code.
- **No gate discharges this.** A gate checks that a path resolves and that a named symbol is declared; a sentence that is WRONG about a symbol that exists passes every one of them, and the resolving anchor under it makes the sentence look checked. Only a reader can falsify prose.

## Writing Journal Rows

**A journal row MUST be written only when the work produced a lesson. It is a record of knowledge, never an artifact of closing a spec.** Decide this BEFORE you write: a spec that rejected no alternative, discovered no constraint, and hit no trap appends no row.
- **Write one when any of these holds:** a decision was made and something else was rejected; a constraint was found that the code does not state; something failed in a way the next session would repeat; an interface, a gate, or a default changed and the reason is not in the code.
- **Write none when the change only relocated content:** a move, a file rename, a reformat, or a rename applied everywhere. Same words, different place, nothing to explain.
- **When there is one, append a row to `plan/journal/<class>.md`** (create the file when the class is new). Include the journal file in the Commit A `--file` list.
- **When there is none, you MUST write nothing and pass no flag.**
- **The file name is the PROBLEM class in kebab-case, never the subsystem.** Recurrence is the row count, so a repeat is countable only when two sessions writing the same failure pick the same file. `plan/journal/README.md` holds the format, and the directory listing is the class vocabulary.
- **Fill the `Spec` cell with the spec stem, or `-` when the work ran outside a spec.** `spec_closure_stem` (`internal/le/commit`) reads that cell to recognise commit A as a spec closure and hands the stem to `review_gate_problems`, so a row that leaves it empty drops the review gate off the commit that carries the code.
- **A row holds exactly five cells.** `journal_row_cells` in `internal/le/journal/journal.go` and the native post-write hook name a malformed row.
- **No gate asks for a lesson, and none MUST be added (owner directive, 2026-08-10).** A gate that demands an artifact buys an archive instead of useful guidance.
- **A row is useful only if it helps future software development, explains past design, or prevents a past mistake from being repeated (owner directive, 2026-08-03). If it does none of the three, it is not useful and does not get written or kept.** The cells are the test: `Symptom` prevents a repeat when it names the trap in the words a future session would recognise, and `Fix` explains past design when it names what was done rather than that something was done.
- **Usefulness is a property of the content, not of whether anything cites it.** An uncited row carrying a real constraint is a ROUTING failure, so route it. Only a row that fails the content test AND is uncited is waste.
- **You MUST ROUTE the lesson to where it governs behaviour, and write a row only when it fits nowhere else yet.** A recurring trap belongs in a rule under `ai/rules/`, a design decision in `docs/architecture/`, a subsystem's data flow in `ai/digests/`, a protocol obligation in `rfc/short/`, an abandoned approach in `plan/learned/DESIGN-HISTORY.md`, hook or tooling friction in `plan/learned/HOOK-FRICTION.md`.
- **We do not SAVE a lesson, we UPDATE the system with it (owner directive, 2026-08-10).** Routing IS the deliverable, and the row is what a lesson gets when no surface governs it yet. A summary filed beside the commit changes no behaviour and is read by nobody.
- **This governs records of COMPLETED WORK, and it says nothing about records of DEFECTS.** A `plan/known-failures/` shard, a `plan/deferrals/` row, and an open red are governed by `ai/rules/completion.md` and `ai/rules/completion.md`, which forbid recording a defect INSTEAD of fixing it and equally forbid making one disappear. Nothing in this section is permission to prune a defect record: the two directions look alike and are opposite. A row nobody needs is noise; a defect record nobody kept is a bug that returns.

| Cell | Content |
|------|---------|
| `Date` | The date the problem was found, as `YYYY-MM-DD` |
| `Spec` | The stem of the spec that found it, or `-` when the work ran outside a spec |
| `Surface` | The subsystem the symptom appeared in |
| `Symptom` | What went wrong, in the words a future session would recognise |
| `Fix` | What was done about it |

General quality check: "If I deleted this row, would a future session repeat the problem it records?"
Source: extract from the Task, Design Insights, Mistake Log, and Deviations sections of the spec.
Include the journal file in the same commit as the code changes.

## Session Handoff

Rationale: New sessions waste tokens re-reading. Give exact edits, but lead
with the rationale so the user can verify the handoff matches the decisions
they believe were agreed.

### When User Asks How to Continue

Start with a short rationale section, then output **exact edits**.
The rationale exists so the user can catch a misaligned handoff BEFORE the next
session blindly applies the edits. If the rationale and the edits disagree,
the user must be able to spot it from the handoff alone.

| Include | Exclude |
|---------|---------|
| Rationale: what was agreed and why (3-6 bullets) | Re-derivation of background research |
| Design decisions the edits encode | File summaries unrelated to the edits |
| File path + line range per edit | Speculative future work |
| OLD text -> NEW text (copy-pasteable) | Redundant restatement of the codebase |
| "Don't re-read these files" list | |
| Final verification command | |

The rationale is a verification checkpoint, not an essay. Each bullet should
name a decision (not a fact) and tie to one or more edits below.

### Template (handoff)

```
RATIONALE (verify this matches what we agreed):
- Decision 1: [what + why] -> EDIT N
- Decision 2: [what + why] -> EDIT N
- Anything still open or assumed: [list]

If any bullet is wrong, STOP and fix the handoff before applying edits.

FILES ALREADY HANDLED (don't re-read): [list]

EDIT 1: [file:lines]
- Delete/Replace: [exact old text -> new text]

EDIT 2: [file:lines]
- Delete/Replace: [exact old text -> new text]

THEN: [test command with timeout]
```

### Handover Documents (`plan/handover/`)

When a handoff must survive beyond the chat (multi-session work, work picked
up days later), write it to `plan/handover/NN-<slug>.md` using the same template.

- `NN` = highest existing number in `plan/handover/` plus one. Check with `ls plan/handover/` first; MUST NOT reuse a number (collisions like two `13-*.md` defeat ordering).
- One handover per file, and only under `plan/handover/`. MUST NOT scatter handover documents elsewhere in `plan/` (the rest of `plan/` is specs, journal classes, deferral shards and known-failure shards).
- The receiving session follows `.claude/rules/session-start.md` "Receiving a Handoff": enumerate every outstanding item before planning.
- Delete the handover file in the commit that completes its last item.

### Rules (handoff)

- Rationale bullets map to edits. An edit with no rationale bullet is suspect.
- MUST NOT exceed 5 remaining edits per handoff. Split into phases if more.
- Each edit self-contained. No "update similarly", spell it out.
- Line numbers from current file state, not original.
- If a decision is assumed rather than agreed, mark it explicitly in the rationale so the user can correct it.

## Rationale

Thomas set the delegation shape on 2026-07-28. A main thread that implements
cannot supervise because its context fills with one phase's detail. That blurs
phase boundaries and the independence of review.
Subagent context is disposable while main-thread context is not, so expensive
reading belongs in an agent whose report is all that survives.
