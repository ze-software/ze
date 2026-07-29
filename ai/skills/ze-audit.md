---
name: ze-audit
description: Spec Audit
---

# Spec Audit

Audit the current codebase against the selected spec BEFORE implementation begins. Answers: **"What already exists? What's left to build?"**

See also: `/ze-implement` (implement the spec), `/ze-review-spec` (post-impl verification)

## Delegation

`ai/rules/spec-delegation.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn an agent to run this skill, hand it the
  spec path and the phase, then stop. Do not run the steps below inline. You do
  not need to ask permission first (`ai/INSTRUCTIONS.md`, STANDING REQUEST).
  Independent work goes out in ONE message with parallel `Agent` calls.
- **If you are that agent:** run the steps below. You have no LSP tool and cannot
  ask the user, so when you hit a STOP-and-ask condition, halt and put the
  question in your report for the main thread to carry.
- **Either way:** every claim in the report cites `file:line` for the function
  that PRODUCES the behavior (`ai/rules/no-fabrication.md`). The main thread
  verifies each one against source before acting; relaying a report unverified
  is fabrication with an extra hop.

## Steps

1. **Read the spec:** Run `scripts/dev/spec-session.sh current`, then read `plan/<spec-name>`
2. **Extract all requirements:** List every item from:
   - Task section (features/requirements)
   - TDD Test Plan (unit + functional tests)
   - Files to Modify / Files to Create
3. **Audit each requirement against the codebase** using Grep, Glob, and Read:
   - Does the code already exist? (file:line)
   - Is it partially implemented? What's missing?
   - Is it completely absent?
4. **Check git history:** Run `git log --oneline -30` to find recent commits that may have implemented spec items
5. **Report findings** as a table:

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| [item] | Done / Partial / Missing | file:line | what's missing or done |

6. **Summarize:** Count done/partial/missing. Recommend which items to implement first based on dependencies.

Do NOT implement anything. Report the audit only. Wait for user to decide what to build.
