# Spec Audit

Audit the current codebase against the selected spec BEFORE any implementation begins.

## Steps

1. **Read the spec:** Read `.claude/selected-spec`, then read `docs/plan/<spec-name>`
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
