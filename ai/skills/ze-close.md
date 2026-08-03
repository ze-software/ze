---
name: ze-close
description: Close Spec
---

# Close Spec

Take an implemented spec through deliverables, security, documentation, the
Review Gate, and the two closure commits.

See also: `/ze-implement` (produces the diff this closes), `/ze-review` (the BLOCKING Review Gate this runs), `/ze-review-spec` (verification against the spec), `/ze-commit` (commit mechanics)

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn an agent to run this skill, hand it the
  spec path and the phase, then stop. Do not run the steps below inline. You do
  not need to ask permission first (`ai/INSTRUCTIONS.md`, STANDING REQUEST).
  Independent work goes out in ONE message with parallel `Agent` calls.
- **If you are that agent:** run the steps below. You have no LSP tool and cannot
  ask the user, so when you hit a STOP-and-ask condition, halt and put the
  question in your report for the main thread to carry.
- **Either way:** every claim in the report names the function that PRODUCES the
  behavior, as the file plus the symbol (`ai/rules/evidence.md`). The main
  thread verifies each one against source before acting; relaying a report
  unverified is fabrication with an extra hop. Report the conclusion and the
  evidence that would overturn it, never the search. Under 40 lines
  (`ai/rules/writing.md`).

## Why this is not part of /ze-implement

Two reasons, both load-bearing.

**Context.** Closure used to be steps 11-16 of a 16-step skill, reached with the
context already full of implementation detail. The measurable result: across 161
specs, the closure tables were byte-identical to the template in 65-75% of
in-progress specs, while sections authors added when they needed them were
untouched in 0%. `plan/TEMPLATE-CLOSURE.md` was split out of `plan/TEMPLATE.md`
for exactly this reason; this skill is the same fix applied to the instructions.

**Model.** `ai/rules/planning.md` puts implementation on Opus 4.8 and
"the Review Gate, spec closure, implementation audit" on Opus 5. A single skill
spanning both forced a phase boundary to be crossed silently mid-file. Announce
the boundary and let the operator switch before starting this skill.

## Precondition

The implementation is complete and `make ze-lint && make ze-unit-test && make
ze-functional-test` is green (`/ze-implement` steps 1-10). If feature code is
still missing, go back: this skill does not implement.

## Spec Sections Used by Each Step

| Step | Spec Section(s) Consumed |
|------|--------------------------|
| 1. Deliverables review | **Deliverables Checklist**, then APPEND `plan/TEMPLATE-CLOSURE.md` and fill **Goal Validation** + **Implementation Summary** |
| 2. Security review | **Security Review Checklist** (feature-specific concerns) |
| 4. Documentation review | **Documentation Update Checklist** (per-category doc updates) |
| 5. Review Gate | **Review Gate**: record via `scripts/dev/review_gate.py`, loop to 0 BLOCKER/0 ISSUE |
| 6. Close + commit | **Implementation Audit**, **Pre-Commit Verification**, **Deferrals Resolved**, `plan/learned/METHODOLOGY.md` |

## Steps

1. **Deliverables review:** Use the spec's **Deliverables Checklist** table. For each row:
   - **Append the closure sections FIRST (BLOCKING):** copy everything below the
     horizontal rule in `plan/TEMPLATE-CLOSURE.md` to the end of the spec. Those
     sections (Implementation Summary, Mistake Log, Implementation Audit, Goal
     Validation, Deferrals Resolved, Review Gate, Pre-Commit Verification) are
     what the steps below fill. They are deliberately absent until now.
   - Run the verification method specified in the table
   - Paste evidence (grep output, test output, ls output)
   - If anything is missing or incomplete, return to `/ze-implement` and implement it
   - Also re-read Acceptance Criteria -- verify each AC-N against the producing function
   - **Goal Validation (BLOCKING):** Fill the spec's **Goal Validation** table. For each goal stated in the Task section, provide concrete evidence (test name, interop scenario, benchmark result) that the goal is achieved. Per `ai/rules/interop-and-goal-validation.md`: "tests pass" alone is not sufficient; map goals to evidence.
   - **Assumptions Resolved (BLOCKING):** Every A-N row in **Risks & Assumptions** must be `confirmed` or `broken` with evidence -- none left `unvalidated`. Fill the spec's Pre-Commit Verification "Assumptions Resolved" table. Broken assumptions need Mistake Log + Deviations entries. Copy surviving R-N risks into the Executive Summary "Risks & observations".
   - **Interop (BLOCKING for protocol features):** If the spec adds/changes protocol behavior, verify an interop test scenario exists and passes. If none exists, create one before proceeding.
2. **Security review:** Use the spec's **Security Review Checklist** table as the starting point. For each row:
   - Check the specific concern described
   - Also apply generic security checks:
     - Injection flaws (command injection, SQL injection, format string)
     - Buffer overflows, out-of-bounds access, integer overflow/underflow
     - Untrusted input handling (missing validation, missing bounds checks, missing sanitization)
     - Path traversal and symlink attacks
     - Race conditions and TOCTOU vulnerabilities
     - Cryptographic misuse (weak algorithms, hardcoded secrets, predictable randomness)
     - Denial of service vectors (unbounded allocations, infinite loops, resource exhaustion)
     - Privilege escalation and missing authorization checks
     - Information leakage (error messages exposing internals, sensitive data in logs)
     - Any OWASP Top 10 relevant to the code's context
   - Fix every issue found. If a fix requires design changes, present to user before proceeding.
3. **Re-run verification:** `make ze-lint && make ze-unit-test && make ze-functional-test`
4. **Documentation review (BLOCKING):** Use the spec's **Documentation Update Checklist** table. For each row:
   - Answer Yes or No. Every Yes MUST name the file and describe the update needed.
   - Every No MUST be backed by source-aware evidence. At minimum, grep `docs/` for source anchors pointing at changed files and check the category does not apply.
   - Do NOT say "update the docs." Name the specific file, the specific section, and what to add.
   - Categories: feature list, user guide, config syntax, CLI reference, API/RPC docs, plugin SDK, wire format, RFC compliance, comparison table, test infrastructure, architecture design.
   - If the spec has no Documentation Update Checklist, use `ai/rules/planning.md` "Documentation Update Checklist" as the reference and fill it for the spec.
   - If config syntax changed, verify examples against the actual YANG/parser before writing docs.
   - If CLI/API/RPC changed, verify docs against the actual handler or RPC type, not the spec.
   - If plugins, commands, event types, send types, or capabilities changed, refresh runtime inventory docs from the registry or binary output, not memory.
   - If any existing doc has a `<!-- source: changed/file.go -- ... -->` anchor, re-read that doc claim and update it if stale.
   - Every factual doc update MUST include or update a `<!-- source: path -- symbol -->` anchor immediately after the claim.
   - **Doctor checks (BLOCKING):** If the implementation adds any runtime dependency (file path, external socket, kernel module, listen port, external binary, TLS cert), verify a corresponding `ze doctor` check exists per `ai/rules/repo-maintenance.md`. Add missing checks and register diagnostic codes in `internal/core/diagnostic/codes.go`.
   - **RFC status (BLOCKING):** If the change implements, changes, or newly proves any RFC-level protocol behavior, update the matching `docs/features/rfc-status.md` row (Status, Implemented coverage, Remaining) with a source anchor to the producing `file:line`, and reconcile `docs/comparison.md` / `docs/features.md` when the support level changes. Per `ai/rules/repo-maintenance.md`.
   - Write the doc updates, run `make ze-doc-test`, and record the result in the spec's Documentation Updates or Pre-Commit Verification section. Include docs in Commit A.
5. **/ze-review gate (BLOCKING -- the final review before closure):** `/ze-implement`'s inline reviews check the diff against the spec's own checklists. This gate runs the generic adversarial `/ze-review` over the COMPLETE diff -- including every fix those reviews produced -- and loops until it is clean. It satisfies the Review Gate defined in `ai/rules/planning.md`; the inline reviews do not substitute for it (they check the spec's own checklists; `/ze-review` checks what nobody planned for).
   - Invoke `/ze-review` on the uncommitted changes. It runs its own automated pre-checks (`make ze-validate`, `scripts/dev/audit-test-relaxation.py`) as its step 0.
   - **Record the machine artifact, not just prose:** `python3 scripts/dev/review_gate.py record --spec <spec> ...`, then `check`. `commit_helper.py` runs that same `check` on the closure commit and refuses without a fresh, hash-pinned, CLEAN artifact, so a hand-written table alone does not satisfy the gate. Put the artifact path and the `check` result in the Review Gate table.
   - Record every BLOCKER/ISSUE under `### Findings fixed` (Severity / Finding / Location / Fixed by) so the learned summary can carry them forward. NOTEs do not block: record and proceed.
   - Fix every BLOCKER and ISSUE (anything above NOTE) per `ai/rules/completion.md`. Write the root cause traced to the producing function. Take the `[source]` fix and record it under `### Fixes applied`. NOTE-only findings do not block -- record them and proceed.
   - Re-run `make ze-lint && make ze-unit-test && make ze-functional-test`.
   - Re-run `/ze-review`; add a `### Run 2+` block. Loop until a run reports 0 BLOCKER and 0 ISSUE. No cap on re-runs -- each fix is new code that needs a fresh review. If the same finding survives 3 fix attempts (3-Fix Rule, `ai/rules/completion.md`), STOP and ask the user.
   - Paste the final clean run into the Review Gate section. The gate is satisfied only when the last run shows 0 BLOCKER, 0 ISSUE.
6. **Close spec and commit (BLOCKING -- do ALL of this BEFORE running the commit script):**
   Precondition: the spec's **Review Gate** section (step 5) shows a final `/ze-review` run with
   0 BLOCKER and 0 ISSUE. Do not prepare the commit script until it does.

   Two more closure sections gate the script, both checked by `commit_helper.py`:
   - **Pre-Commit Verification:** EVERY sub-table needs at least one evidence row.
     `pre_commit_verification_gaps` checks them one at a time and names the empty
     ones, so a row in `Files Exist` is not evidence for `AC Verified`.
   - **Deferrals Resolved:** account for every row in the shard named in the spec
     metadata. `deferral_unassigned_problems` WARNS (it does not block) on a live
     row with no destination, so a missing home costs nothing and is the reason
     rows persist for weeks -- act on it here.
     **Add `--remove plan/deferrals/<spec-stem>.md` ONLY when every row in that
     shard is terminal.** A shard still holding a live row outlives its source
     spec: the row is homed at another spec and the shard is only where it is
     written down (`ai/rules/planning.md`).
     `deferral_shard_removal_problems` BLOCKS the removal if you get this wrong.
     **Then check the shards your resolutions just emptied.** Setting the last
     live row of a FOREIGN shard to `done` makes that shard residue, and you are
     the actor who removes it: add `--remove plan/deferrals/<that-stem>.md` in the
     same commit. Left for whoever comes next, it is never collected -- 14 such
     shards existed on 2026-08-03 (`ai/rules/planning.md`).
   Running the commit script finishes the work. There is no step 7. The script is the
   final action. Everything below MUST be in that single script.

   a. **Ask whether there is a lesson, THEN allocate.** A learned summary records
      what the code cannot tell a future reader. It is not an artifact of closing a
      spec, so a spec that produced no such knowledge writes no file at all: no
      number is allocated, nothing is created, and commit A carries the reason
      instead (`plan/learned/METHODOLOGY.md`, "When No Summary Is Written").
      Answer this first, from the finished work: does this spec leave a decision
      with a rejected alternative, a constraint discovered, or a trap that would
      catch the next session? A "Gotchas: None." with nothing above it is the
      answer being no.
      - **Yes, there is a lesson:** `python3 scripts/dev/commit_helper.py learned-next <spec-stem>` allocates NNN (max(existing `plan/learned/NNNN-*.md` prefixes) + 1) and creates `plan/learned/NNN-<spec-stem>.md` immediately, so concurrent sessions in one tree cannot collide. Write the summary into that file following `plan/learned/METHODOLOGY.md`.
        Use the extraction recipe: Context from Task + Current Behavior, Decisions from Key Design Decisions + annotations, Consequences from Design Insights + Limitations, Gotchas from Deviations + Mistake Log.
      - **No, there is none:** create nothing. Do not run `learned-next` -- it
        allocates and writes on the spot, so calling it to "see the number" leaves
        an empty summary behind. Carry the reason on commit A instead (step 6e).
      The commit helper asks the same question of the diff and will refuse commit A
      if the change adds content and neither a summary nor a reason is present, so
      an honest "no" costs one flag and a dishonest one is caught.
   b. Update `ai/LEARNED-INDEX.md` if the summary contains a structural decision (not just task completion).
   c. Release this session's spec claim: `scripts/dev/spec-session.sh release`. This also frees a slot against the WIP cap (`scripts/dev/spec-session.sh wip`).
   d. List all changes made (files modified/created, tests added, docs updated, issues found and fixed).
   e. Prepare ONE commit script with `scripts/dev/commit_helper.py` that produces TWO commits:
      - **Commit A (implementation + spec):** run `scripts/dev/commit_helper.py create --replace` with `--file` for every implementation file (code, tests, docs, schema), `plan/learned/NNN-<spec-stem>.md` when step 6a wrote one, `ai/LEARNED-INDEX.md` if updated, and `plan/<spec-name>` to preserve all implementation edits in git history.
      - **Commit B (spec closure):** run `scripts/dev/commit_helper.py create --append --remove plan/<spec-name>` with the spec closure commit message.
      - **Lesson flags follow step 6a's answer, and commit A never passes `--lesson-required`.** That flag is the operator demanding a summary; passing it on every closure is what made the summary unconditional. When a summary was written, `--file` on it is the whole story. When none was, pass `--lesson-not-needed "<why this spec taught nothing reusable>"` and say what the work was, not that a spec closed.
      - Commit B removes a spec and adds nothing, so the helper asks it for nothing. Pass `--lesson-not-needed "spec closure only; lesson is in Commit A"` only when commit A actually carried a summary.
      - The helper owns the session ID, message files, executable script, ignored-path rejection, `git commit -F`, and learned-summary checks.
   f. Run the generated script yourself (`bash tmp/commit-<SESSION>.sh`), then report the resulting commit SHA(s), the script path, message files, commit subjects, and included files. This is the end.

   **Why one script, two commits, no follow-up:** the user will not ask for a second step.
   They will not remember that the spec needs closing. They will not prompt you for the
   learned summary. If closure is not in the script, it will never happen and the spec
   rots in `plan/` forever. Include everything. There is nothing after this step.
   Two commits because `git rm` destroys the working copy. Commit A preserves the
   edited spec in git history; commit B cleanly removes it.

## Rules

- **Diagnosis before fix (BLOCKING).** When a review finding or a red gate appears, write the five-part Diagnosis before editing (`ai/rules/completion.md`). It gives the symptom, the root cause as a producing function, the owning layer, two fixes labeled `[workaround]`/`[source]`, and why not the workaround. Renaming, skipping, special-casing, or weakening a test to reach green is a workaround, not a fix.
- **No deferred work.** Closure is not a place to discover that something was skipped. If a deliverable is missing, the spec is not ready to close: return to `/ze-implement` and finish it, or raise the scope question with the user (`ai/rules/completion.md`).
- **The Review Gate (step 5) is BLOCKING and is not optional.** Step 6 may not prepare the commit script until the spec's **Review Gate** section shows a final `/ze-review` run with 0 BLOCKER and 0 ISSUE.
- If the spec is missing a **Deliverables Checklist**, **Security Review Checklist**, or **Documentation Update Checklist**, STOP and inform the user that the spec needs updating before it can be closed.
- Before reporting done, re-read the spec and confirm each item is actually implemented in the code.
