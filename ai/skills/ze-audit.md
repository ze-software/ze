---
name: ze-audit
description: Spec Audit
---

# Spec Audit

Audit the current codebase against the selected spec BEFORE implementation begins. Answers: **"What already exists? What's left to build?"**

See also: `/ze-implement` (implement the spec), `/ze-review-spec` (post-impl verification)

## Delegation

`ai/rules/planning.md`: the main thread supervises, it does not run this
phase itself.

- **If you are the main thread:** spawn an agent to run this skill, hand it the
  spec path and the phase, then stop. Use `subagent_type: ze-read`, which costs
  about 6k fewer startup tokens per agent than the default
  (`ai/rules/context-economy.md`). Do not run the steps below inline. You do
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

1. **Read the spec:** Run `./le spec session current`, then read `plan/<spec-name>`
2. **Extract all requirements:** List every item from:
   - Task section (features/requirements)
   - TDD Test Plan (unit + functional tests)
   - Files to Modify / Files to Create
3. **Read the pages before the search (BLOCKING, `ai/rules/documentation.md`):** `ai/CODE-TO-DOCS.md` lists them per file. The files are **Files to Modify** and **Files to Create**. Read those pages. Read each file's `// Design:` header. A page states what the surface promises, which is half of this audit.
4. **Audit each requirement against the codebase** using Grep, Glob, and Read:
   - Does the code already exist? (file + symbol)
   - Is it partially implemented? What's missing?
   - Is it completely absent?
   - Does a page from step 3 disagree with what the code does? Report it as a finding with the page, the sentence, and the producing function.
5. **Check git history:** Run `git log --oneline -30` to find recent commits that may have implemented spec items
6. **Report findings** as a table:

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| [item] | Done / Partial / Missing | file + symbol | what's missing or done |

7. **Summarize:** Count done/partial/missing. Recommend which items to implement first based on dependencies. List every page-versus-code disagreement separately: those are defects the implementation repairs, not spec items.

Do NOT implement anything. Report the audit only. Wait for user to decide what to build.
