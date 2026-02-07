# Explore Topic

Find and read all files related to a topic before proposing any changes.

The user will specify the topic as an argument: `/explore <topic>`

## Steps

1. **Search broadly:** Use Glob and Grep to find all files related to the topic — source code, tests, docs, specs, config
2. **Read each file:** Read every match. Do not skim or skip.
3. **Summarize findings:**
   - Which files exist and what they do
   - Current behavior and patterns used
   - How the pieces connect (data flow, imports, callers/callees)
4. **Propose a plan:** Based on what exists, suggest what to change and how — extending existing code, not duplicating

Do NOT edit anything. Summarize and propose only. Wait for user approval before making any changes.
