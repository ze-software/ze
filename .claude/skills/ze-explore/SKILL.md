# Explore Topic

Find and read all files related to a topic before proposing any changes.

The user will specify the topic as an argument: `/ze-explore <topic>`

See also: `/ze-spec` (create a spec from findings), `/ze-design` (stress-test a design)

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
