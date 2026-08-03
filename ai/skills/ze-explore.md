---
name: ze-explore
description: Explore Topic
---

# Explore Topic

Find and read all files related to a topic before proposing any changes.

The user will specify the topic as an argument: `/ze-explore <topic>`

See also: `/ze-spec` (create a spec from findings), `/ze-design` (stress-test a design)

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

## Steps

1. **Search by category:** For each category, search for files related to the topic:

   | Category | Where to look |
   |----------|--------------|
   | Source code | `internal/`, `pkg/`, `cmd/` |
   | Tests | `*_test.go`, `test/` (`.ci`, `.et`) |
   | Specs | `plan/spec-*` |
   | Learned summaries | `plan/learned/` |
   | Docs | `docs/` |
   | Config/YANG | `schema/`, `*.yang` |
   | Rules | `.claude/rules/` |

2. **Read each file:** Read every match. Do not skim or skip.
3. **Summarize findings:**
   - Which files exist and what they do
   - Current behavior and patterns used
   - How the pieces connect (data flow, imports, callers/callees)
4. **Propose a plan:** Based on what exists, suggest what to change and how -- extending existing code, not duplicating

Do NOT edit anything. Summarize and propose only. Wait for user approval before making any changes.
