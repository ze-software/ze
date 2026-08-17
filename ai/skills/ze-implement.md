---
name: ze-implement
description: Implement Spec
---

# Implement Spec

Build the selected spec: wiring first, then feature phases, with review loops
until the diff is clean. **Closure is a separate skill** -- when this one ends,
run `/ze-close`.

See also: `/ze-close` (deliverables, security, docs, Review Gate, the two closure commits), `/ze-audit` (check what exists first), `/ze-review` (the adversarial pass), `/ze-precommit-verify` (run tests)

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread: spawn ONE AGENT PER IMPLEMENTATION PHASE, never
  one agent for the whole spec.** Read the spec's **Implementation Phases**
  section. Treat each phase as one work package. Hand each agent the spec path,
  its phase number, and the per-spec state file path (below), on
  `subagent_type: ze-work` (`ai/rules/context-economy.md`). Do not run the
  steps below inline. You do not need to ask permission first
  (`ai/INSTRUCTIONS.md`, STANDING REQUEST).
- **Order the phase agents by their dependencies.** Phases that do not depend on
  each other go out in ONE message with parallel `Agent` calls. A phase that
  consumes what an earlier phase wrote waits for that phase's handoff.
- **Resolve every symbol BEFORE you spawn, and put `file + symbol + line range`
  in the agent's prompt.** This is an optimisation, not a precondition: an agent
  can resolve symbols itself, by the LSP tool where its registry carries one and
  by `gopls` from Bash where it does not (`ai/rules/context-economy.md`). What
  you buy is paying the resolution ONCE rather than once per agent, and it is
  worth buying: `gopls symbols` on `internal/component/ike/engine/fsm.go` costs
  1,297 bytes against 44,164 for the file (34.1x), and an agent given a bare file
  name that resolves nothing reads all of it.
- **A phase whose VERIFICATION needs `findReferences` or `goToImplementation` IS
  delegatable: the agent answers it by the LSP tool or by `gopls references` /
  `gopls implementation`, whichever route is live for it.** "Every call site
  updated" and "every implementation of this interface handles the new case" are
  questions `grep` answers wrongly -- it matches text, comments and string
  literals included -- and that is a reason to name the operation in the agent's
  prompt, not a reason to keep the phase inline. Sizing an agent is a cost
  decision, and tool availability does not constrain it.
- **Why one agent per phase.** Cost per API call is the context size at that
  call, and context grows with turns. A long agent therefore pays more for every
  later call it makes. Measured over this machine's session transcripts
  (`make ze-token-economy-report`), implementation agents ran 144 API calls each
  at 294k mean context, more of both than any other phase. Splitting
  the spec across phase agents cuts the turns each one carries. It does not cut
  the work: every phase still runs the full steps below.
- **If you are a phase agent:** run the steps below for YOUR phase. When an
  earlier phase already did steps 1-3, take what it FOUND from the state file
  rather than re-deriving it. To place a symbol your prompt did not resolve, use
  the LSP tool if your registry carries it and `gopls symbols` / `definition` /
  `references` from Bash if it does not, rather than reading whole files to hunt
  for it. You cannot ask the user. When you hit a
  STOP-and-ask condition, halt and put the question in your report for the main
  thread to carry.
- **Taking steps 1-3 from the state file never skips a BLOCKING step.** The
  digest says what an earlier phase DID; it is not evidence about the tree you
  are about to edit (`ai/rules/context-economy.md`, "A digest is not evidence").
  Step 2's status edit and step 3's audit and assumption validation JUDGE THE
  CURRENT TREE, and in a shared checkout that tree moved between phase 1 and
  yours. Re-run them, using the digest to make them cheap: it tells you where to
  look, never what you will find there.
- **Either way:** every claim in the report names the function that PRODUCES the
  behavior, as the file plus the symbol (`ai/rules/evidence.md`). The main
  thread verifies each one against source before acting; relaying a report
  unverified is fabrication with an extra hop. Report the conclusion and the
  evidence that would overturn it, never the search. Under 40 lines
  (`ai/rules/writing.md`).

### Phase handoff: the per-spec state file

Every phase agent ends by APPENDING its handoff to the per-spec state file
`tmp/session/<YYYY-MM-DD>-<SID>/state/session-state-<spec-stem>-<SID>.md`, the path `_state_file`
computes in `.claude/hooks/lib/state-file.sh` and `_find_latest_state_for_spec`
recovers across sessions. That file already exists and already carries digests
after compaction. Write into it. A new handoff file family is layering
(`ai/rules/no-layering.md`).

The next phase's agent READS that file before it reads any source. It does not
re-derive the phase before it. Re-reading a source file the handoff already
digests is the cost this decomposition removes. Read the source when the digest
lacks the detail you need. That is a judgement, never a default.

A handoff carries four things:

| Part | Content |
|------|---------|
| Files changed | one line per file, in the digest format `.claude/rules/post-compaction.md` already defines: `` - `path/to/file.go` (380L): what it holds. Key: `Run()`, `handleOpen()`. Uses `wire.SessionBuffer`. `` |
| Acceptance criteria covered | each AC-N this phase now satisfies, with the test name or command that is its evidence |
| Verified green | the exact targets run and their result (`make ze-unit-test`, the wiring test name, the phase's Verify line from the spec) |
| Do not assume | what the next phase must NOT take for granted. A stub still standing, an A-N still `unvalidated`, a gate not yet run, a file left untouched |

### Work-package size (BLOCKING)

**A work package is sized so ONE agent finishes it, and the boundary is chosen
at DECOMPOSITION. It is never permission to stop mid-package.** Reaching the
edge of your context is not reaching the edge of your package.

| Situation | Do |
|-----------|-----|
| Your package is finished | Write the handoff, report, stop |
| Your package is bigger than you can finish | Write the handoff for what IS finished, then REPORT to the main thread that the package needs a continuation. The main thread spawns it |
| An acceptance criterion inside your package looks too expensive | It stays in the package. You do not trim it, and you do not narrow it |
| You are running long | Finish the package. "I was near a budget" is not a reason to leave a stub, a TODO, or a deferral row |

`ai/rules/completion.md` is untouched by this decomposition. No partial work, no
parking, no stub, no deferral, no weakened test. Every one of those bans applies
to a phase agent as it applies to one agent implementing a whole spec. The only
thing that changed is how many agents share the work.

## Scope: this skill stops before closure

This skill ends when the implementation is complete and green. It does NOT run
the deliverables review, the security review, the documentation review, the
Review Gate, or the commits. Those are `/ze-close`, for two reasons:

- **Context.** Closure instructions reached at the tail of a long skill get
  partially followed. Across 161 specs the closure tables were byte-identical to
  the template in 65-75% of in-progress specs, while sections authors added when
  they needed them were untouched in 0%.
- **Model.** Implementation carries no model requirement (`ai/rules/planning.md`, 2026-08-03).
  The Review Gate, spec closure and implementation audit still run on Opus 5, and
  review is INDEPENDENT of the author: end this skill and hand the review to a
  fresh session or to the `/ze-close` agent, never to this context. That agent
  runs every review lens ITSELF and spawns nothing (`ai/rules/planning.md`,
  owner directive 2026-08-15); the independence comes from the phase boundary
  you are crossing here, not from the number of agents behind it.

## Spec Sections Used by Each Stage

| Stage | Spec Section(s) Consumed |
|-------|--------------------------|
| 1. Read spec | Entire spec |
| 2. Update status | Spec metadata |
| 3. Audit | Files to Modify, Files to Create, TDD Test Plan, **Risks & Assumptions** (validate assumptions before coding) |
| 4. Wiring phase | **Wiring Test** table (entry points, registration, skeleton) |
| 5. Implement | Implementation Phases, TDD Test Plan, Acceptance Criteria |
| 6. Verify | (make targets) |
| 7. Critical review | **Critical Review Checklist** (feature-specific checks) |

The spec carries only design-time sections while this skill runs. The closure
half lives in `plan/TEMPLATE-CLOSURE.md` and `/ze-close` appends it when it is
first needed.

**Verification: inner loop vs gate.** Steps 6 and 9 use the fast targets to
iterate. `make ze-precommit-verify` is the pre-commit GATE (`ai/rules/git-safety.md`) and
is the only command the spec's Goal Gates name. Do not add a third spelling.

## Steps

1. **Read the spec:** Run `scripts/dev/spec-session.sh current` to find this session's spec. If empty, use the spec named in the conversation and claim it with `scripts/dev/spec-session.sh claim <spec-name>`. Then read `plan/<spec-name>`.
   - If `claim` exits 3, the WIP cap refused it: too many specs are already in-progress. That is a decision for the user, not something to route around. Show them the list it printed and ask whether to close one first or raise `ZE_SPEC_WIP_CAP`. Do NOT edit the spec's Status by hand to skip the check.
2. **Update spec status (BLOCKING -- do this FIRST, before any other work):**
   Edit the spec file NOW: set `Status` to `in-progress`, `Phase` to `1/N`, `Updated` to today.
   This is the FIRST action after reading. Not after audit, not after implementation, not at the end.
   Do not proceed to step 3 until the spec file on disk shows `in-progress`.
   **Why this is BLOCKING:** other sessions check spec status to avoid collisions. A spec that
   stays in `design` or `ready` during implementation lies about its state.
3. **Audit first:** Run `/ze-audit` logic. Check Files to Modify, Files to Create, and TDD Test Plan against the codebase. Identify what's already implemented, partially done, or missing. Do not redo existing work.
   - **BGP Family Checklist (BLOCKING):** If the spec involves a new SAFI, capability, or attribute
     but has no "BGP Family Checklist" section, STOP. Read `ai/patterns/bgp-family.md`, add the
     checklist section to the spec, and present to the user before coding. This gate exists because
     SR-Policy shipped incomplete (3 commits) due to missing integration points that the generic
     wiring rules did not catch.
   - **Validate assumptions (BLOCKING):** Read the spec's **Risks & Assumptions** Assumptions
     table. For every A-N row whose validation method is cheap (grep, read a file, run an
     existing test), run it NOW — before any feature code — and flip Status to `confirmed`
     or `broken`. A `broken` assumption gets a Mistake Log "Wrong Assumptions" row; if it
     invalidates the approved design, STOP and present to the user before coding.
   - If the spec has the section but with only placeholder rows, treat it as missing (see Rules).
4. **Wiring phase (MANDATORY FIRST — before any feature code):**
   Read the spec's **Wiring Test** table. For each row:
   - Identify the entry point (CLI command, web route, config leaf, plugin event, RPC handler).
   - If the entry point does not exist yet: implement the registration/skeleton now (handler that returns "not implemented" or equivalent). This is Phase 1 regardless of what the spec's Implementation Phases say.
   - If the entry point exists: verify it with `grep` or LSP and record file + symbol.
   - Write the wiring test (the `.ci` or `_test.go` that exercises entry-point-to-feature-code). It should fail because the feature logic is a stub.
   Gate: every Wiring Test row has a registered entry point and a failing test before proceeding.
5. **Implement feature phases:** Follow the spec's **Implementation Phases** section in order, filling in the stubs created in step 4. For each phase:
   - Write the tests listed for that phase (TDD — test must fail before implementation)
   - Implement minimal code to pass
   - Run `make ze-unit-test` until green
   - Confirm the wiring test from step 4 now passes (or progresses) after each phase
   - Update the **Risks & Assumptions** tables: flip A-N statuses as evidence arrives;
     when an assumption breaks mid-phase, add the Mistake Log row immediately and STOP
     if the approved design no longer holds. Add new A-N/R-N rows as they surface.
   - Before the phase ends, re-read the Go you wrote for it against `docs/contributing/ze-style.md`.
     A style defect caught in the phase that produced it costs one edit. The same defect
     caught in step 7 costs a re-read of every phase before it.
   - Move to next phase
6. **Run full verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
7. **Critical review:** Use the spec's **Critical Review Checklist** table. For each row:
   - Verify the "What to verify" column against the actual implementation
   - Document pass/fail for each check
   - Also apply generic checks from `ai/rules/quality.md` (Correctness, Simplicity, Consistency, Completeness, Quality, Tests, Style)
   - **Ze Style (BLOCKING):** Apply the Style row of `ai/rules/quality.md` to every Go file this spec touched, against `docs/contributing/ze-style.md`. Four questions, and the first is a BLOCKER on its own. Can a peer reach any `panic()` you added? Trace the input back to the socket: a malformed message is an operating error and returns an error. What bounds each new loop, queue, retry, and cache? Does each new name say what the value IS rather than its Go type? Does each new lifecycle or paired call state its obligation with MUST on BOTH sides?
   - **CLI grammar (BLOCKING):** If any CLI command was added or changed, verify it follows action-before-identifier per `ai/rules/cli.md`. Run the mechanical check: `args[0]` must always be a keyword, never a user identifier.
   - **Invocation-form change (BLOCKING):** If the change REMOVES or ALTERS how a binary is invoked (a launch/dispatch form, a positional's meaning, a flag's meaning), enumerate EVERY invocation site by grepping the bare invocation token (`\bze <positional>`), NOT just the framework directive (`exec=ze`). Invocations hide in `.ci` `exec=` directives, **embedded `tmpfs=*.sh` script bodies** (run via `exec=./script.sh`), helper `.sh`/`.py`, the test-runner launch code, wrapper scripts (`test/exabgp-compat/bin/exabgp`), and docs. A directive-only grep is blind to shell-script-mediated launches. Then prove the change against the **FULL affected suite, never a sample** -- only the full run executes the embedded launches, so a passing sample is a false green. (Learned 1248: removing the bare `ze <config>` sink broke 26 auth `.ci` that launched the daemon from an embedded `tmpfs=*.sh` `ze <config>` line the migration grep never saw; the full functional suite caught it, a sampled run would not have.)
   - **Doctor checks (BLOCKING):** If the implementation adds any runtime dependency (file path, socket, kernel module, port, TLS cert, external binary), verify a `ze doctor` check exists per `ai/rules/repo-maintenance.md`. Register diagnostic codes in `internal/core/diagnostic/codes.go`.
   - **Prometheus counters:** If the feature has observable state (connections, errors, rates, gauges), verify counters are defined, registered in telemetry, and listed in the spec's Integration Checklist.
   - **YANG validation:** If YANG leaves were added, verify each has maximum native constraints (`range`, `length`, `pattern`, `enumeration`). If native is insufficient, verify a custom validator with `CompleteFn` exists per `ai/patterns/config-option.md`. A leaf with `type string` and no constraint is a red flag.
   - Do NOT agree with the spec blindly -- challenge architectural assumptions
8. **Fix every issue found** in the review. For each fix apply `ai/rules/completion.md`: write the root cause traced to the producing function and choose the `[source]` fix over the `[workaround]` before editing. Never make a finding disappear by weakening a test, renaming a symbol, or special-casing the failing input — that fixes where the problem shows up, not where it is.
9. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
10. **Repeat steps 7-9** until the review finds zero issues and all tests pass. There is no cap on the NUMBER of passes, because each fix is new code that needs a fresh review. Each pass covers LESS than the one before it: round 1 the whole diff, round N+1 only round N's fixes and what they touched. Stop when a pass finds no BLOCKER and no ISSUE inside its own scope. "Stop only when a pass finds nothing anywhere" has no state in which it stops, which is why finished work fails to close (`ai/rules/planning.md`, "Bounding the loop").
11. **Stop here and hand off to `/ze-close`.** The implementation is done when
    steps 7-9 find nothing, every target is green, AND you have read the whole
    diff yourself, every phase agent's hunks included. Read it here, before the
    report: `/ze-close` runs the Review Gate. A report that calls the work green
    before anybody has read it makes that gate establish what you already
    announced. Report what was built, what
    the tests prove, and any surviving risk from the spec's R-N rows. Then state
    plainly that closure (deliverables, security, docs, Review Gate, commits) is
    `/ze-close`, and that `ai/rules/planning.md` puts it on the review
    model. Do NOT append `plan/TEMPLATE-CLOSURE.md`, do NOT run `/ze-review` as
    the gate, and do NOT prepare a commit script here. The one exception is the
    two-session handoff below, which the spec declares before implementation
    starts.
    - **Write the handoff first.** Before you report, append your phase handoff
      to the per-spec state file (see Delegation, "Phase handoff"). The report
      is for the main thread. The handoff is for the next phase's agent. Both
      are owed, and neither replaces the other.
    - Closure is reached when the LAST phase is green, not when your phase is.
      A phase agent that is not the last one reports its phase and stops. The
      main thread spawns the next phase.
    - **`| Handoff | verify |` in the spec metadata changes this step, and
      nothing else in this skill.** The mode is declared before implementation
      starts, so read the row rather than choosing here. When the row says
      `verify` and the LAST phase is green, commit the work before you stop:
      1. Set `| Status | verification |` and `| Updated |` to today.
      2. Prepare ONE commit with `scripts/dev/commit_helper.py create`, carrying
         the code, the tests, the docs and the spec file. Carry NO
         `plan/learned/` file and NO `--remove` of the spec: those two make it a
         closure commit, and a closure commit needs the Review Gate artifact
         this session MUST NOT produce. Run the script the helper prints
         (`ai/rules/git-safety.md`).
      3. Run `scripts/dev/spec-session.sh release`.
      4. Report the commit SHA, and state that `/ze-close` on Opus 5 is the next
         phase. It reviews that commit.
      When the row is absent or `-`, none of this applies: hand the uncommitted
      diff to `/ze-close` as above.

## Rules

- **Diagnosis before fix (BLOCKING).** When a test, gate, or review finding fails, write the five-part Diagnosis before editing (`ai/rules/completion.md`): symptom, root cause traced to the producing function, owning layer, two fixes labeled `[workaround]`/`[source]`, why not the workaround. Fix the root cause at the owning layer. Renaming, skipping, special-casing, or weakening a test to reach green is a workaround, not a fix. When a check rejects you, ask: is the check wrong, is the input wrong, or is the check's data/config incomplete?
- **No deferred work.** Every item in the spec must be implemented fully before reporting completion. No TODOs, no stubs, no placeholder implementations, no "left as future work" notes, no comments like "// TODO: handle X later". If an item turns out to be blocked, ambiguous, or harder than expected, stop and raise it with the user to re-negotiate scope. Never silently skip or defer.
- **Design-doc "Deferred to a later phase" sections are not authoritative.** When the user picks an option whose design doc carves out follow-on work as deferred, do NOT parrot that carve-out. Treat the entire problem as in scope and ask before excluding anything.
- Do NOT skip the audit step -- re-implementing existing code wastes time
- If the same issue reappears after 3 fix attempts (3-Fix Rule, `ai/rules/completion.md`), STOP and ask for guidance. Otherwise keep reviewing -- there is no pass limit.
- If the spec is missing a **Critical Review Checklist**, STOP and inform the user that the spec needs updating before implementation can proceed. (`/ze-close` makes the same check for the Deliverables, Security Review, and Documentation Update checklists it consumes.)
- If the spec has a **Risks & Assumptions** section containing only template placeholder rows, STOP and ask the user to complete it (or confirm there are genuinely none). Specs created before the section existed are exempt -- do not retrofit without user request.
- Before handing off, re-read the spec and confirm each item is actually implemented in the code
- **"Implemented" is not "done".** This skill produces a clean diff, not a closed spec. Do not say done, complete, or ready to commit at step 11 -- say the implementation is green and closure is next (`ai/rules/completion.md`).
- **"Green" is a claim about the DIFF, never about the gates.** Requirement 10 in `ai/rules/completion.md` binds this word: say it only after you have read the diff hunk by hunk, your own and every phase agent's. A gate covers what somebody thought to check, so a defect on a surface no gate reads passes every target. When you have run the gates and not read the change, say exactly that instead. It is one line, and it tells the reader which claim they are getting. Reporting "green" and leaving the reading to the Review Gate inverts the order: the review then establishes what you already announced.
- **The Review Gate is BLOCKING and lives in `/ze-close`.** The inline reviews in steps 7-10 do NOT satisfy it: they check the spec's own checklists, `/ze-review` checks what nobody planned for. This is the Review Gate from `ai/rules/planning.md`.
