# Critical Review

Perform a critical review of the current implementation against its spec.

Do NOT agree with the spec blindly — challenge architectural assumptions against the actual code.

## Steps

1. **Read the spec:** Read `.claude/selected-spec`, then read `docs/plan/<spec-name>`
2. **Check git history:** Run `git log --oneline -20` — avoid proposing work that's already done
3. **Read the actual code:** For every file in "Files to Modify" and "Files to Create", read the source. Do NOT rely on spec descriptions or memory.
4. **Trace data flow:** For each changed component, trace data from entry through transformations to exit. Verify boundaries are respected (Engine/Plugin, Wire/RIB, FSM/Reactor).
5. **Validate against spec:** Go through every requirement. For each: is it implemented (file:line)? Is it correct? Complete?
6. **Check test coverage:** For every test in the TDD Plan: does it exist? Does it test what the spec says? Boundary cases covered?
7. **Check conventions:** kebab-case JSON, YANG `-conf`/`-api` suffixes, naming patterns, no layering, no identity wrappers, no YAGNI
8. **Report findings** as a numbered list with severity:
   - **BLOCKER:** Spec requirement not implemented or incorrect
   - **ISSUE:** Convention violation, missing test, or quality problem
   - **NOTE:** Suggestion or minor observation

## Rules

- Do NOT fix anything. Report findings only.
- After the user reviews your list, they will tell you which to fix.
- Maximum 2 review passes. If issues remain after 2 passes, list them and stop.
