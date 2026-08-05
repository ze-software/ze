---
name: ze-review-spec
description: Review Against Spec
---

# Review Against Spec

Post-implementation verification: does the implementation match the spec? Every requirement, every test, every file.

This review answers: **"Did we build what we said we would?"**

See also: `/ze-audit` (pre-impl: what already exists?), `/ze-review` (code quality and edge cases), `/ze-review-deep` (exhaustive multi-agent review)

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn an agent to run this skill, hand it the
  spec path and the phase, then stop. Do not run the steps below inline. You do
  not need to ask permission first (`ai/INSTRUCTIONS.md`, STANDING REQUEST).
  Independent work goes out in ONE message with parallel `Agent` calls.
- **If you are that agent:** run the steps below. Resolve symbols with the LSP
  tool if your registry carries it and with `gopls` from Bash if it does not
  (`ai/rules/context-economy.md`). You cannot ask the user, so when you hit a
  STOP-and-ask condition, halt and put the question in your report for the main
  thread to carry.
- **Either way:** every claim in the report names the function that PRODUCES the
  behavior, as the file plus the symbol (`ai/rules/evidence.md`). The main
  thread verifies each one against source before acting; relaying a report
  unverified is fabrication with an extra hop. Report the conclusion and the
  evidence that would overturn it, never the search. Under 40 lines
  (`ai/rules/writing.md`).

## Steps

1. **Read the spec:** Run `scripts/dev/spec-session.sh current`, then read `plan/<spec-name>`
2. **Check git history:** Run `git log --oneline -20` -- avoid proposing work that's already done
3. **Validate requirements:** For every AC in the spec, find the implementation (file + symbol). Is it correct? Complete?
4. **Check test existence:** For every test in the TDD Plan, verify it exists with the exact name listed. If renamed, note the actual name.
5. **Check file lists:** For every file in "Files to Modify" and "Files to Create", verify it was modified/created.
6. **Check wiring tests:** For every row in the Wiring Test table, verify the .ci file exists and tests the claimed path.
7. **Check documentation:** Were architecture docs, example configs, and syntax docs updated as spec requires?
8. **Check conventions:** kebab-case JSON keys, YANG `-conf`/`-api` suffixes, Go naming patterns, CLI command tab-completion (every user-facing command must appear in the YANG command tree or have a plugin `CommandDecl` without `Hidden: true`; see `ai/rules/cli.md` "Command Completion").
9. **Task-vs-ACs cross-check (BLOCKING):** Re-read the spec's **Task** description and **Data Flow** section. For every operation the Task promises (e.g., "allows ExaBGP plugins to announce SR-Policy routes," "enables Ze to decode X"), verify there is at least one AC that covers that operation end-to-end. ACs tend to drift toward unit-level checks ("Parse returns correct fields") while the Task describes user-level capabilities ("announce routes"). If the Task promises an operation and no AC covers it, that is a BLOCKER: "spec gap -- Task promises [operation] but no AC covers the end-to-end path."
10. **Reference comparison (for new features):** If the spec adds a new feature type (NLRI family, plugin, protocol, command category), find the most similar existing feature. Compare registrations, handlers, wiring tests, and functional tests. If the reference has a component that the spec does not mention in ACs, Wiring Tests, or Files to Create, that is an ISSUE: "reference [feature] has [component] at [file] -- spec does not require it."
11. **End-to-end user stories check:** If the spec has an "End-to-End User Stories" section, verify every story has a corresponding functional or wiring test. If the spec lacks this section (older specs), construct the stories from the Task description and check them anyway.
12. **Report findings** as a numbered list with severity:
   - **BLOCKER:** Spec requirement not implemented, test missing, or file not created
   - **ISSUE:** Test name mismatch, documentation gap, or convention violation
   - **NOTE:** Minor observation

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT review code quality, edge cases, or security -- that is `/ze-review`.
- After the user reviews your list, they will tell you which to fix.
- No cap on the NUMBER of passes, a hard bound on each one's SCOPE. Fixes can break spec alignment, so every change earns a fresh pass, over the fixes that changed it: round 1 the whole spec, round N+1 only round N's fixes. Stop when a pass finds no BLOCKER and no ISSUE within its own scope (`ai/rules/planning.md`, "Bounding the loop").
