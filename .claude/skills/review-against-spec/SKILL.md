# Review Against Spec

Post-implementation verification: does the implementation match the spec? Every requirement, every test, every file.

This review answers: **"Did we build what we said we would?"**

For pre-implementation audit (what already exists?), use `/spec-audit` instead.
For code quality, edge cases, and security, use `/review-for-issues` instead.

## Steps

1. **Read the spec:** Read `.claude/selected-spec`, then read `plan/<spec-name>`
2. **Check git history:** Run `git log --oneline -20` -- avoid proposing work that's already done
3. **Validate requirements:** For every AC in the spec, find the implementation (file:line). Is it correct? Complete?
4. **Check test existence:** For every test in the TDD Plan, verify it exists with the exact name listed. If renamed, note the actual name.
5. **Check file lists:** For every file in "Files to Modify" and "Files to Create", verify it was modified/created.
6. **Check wiring tests:** For every row in the Wiring Test table, verify the .ci file exists and tests the claimed path.
7. **Check documentation:** Were architecture docs, example configs, and syntax docs updated as spec requires?
8. **Check conventions:** kebab-case JSON keys, YANG `-conf`/`-api` suffixes, Go naming patterns.
9. **Report findings** as a numbered list with severity:
   - **BLOCKER:** Spec requirement not implemented, test missing, or file not created
   - **ISSUE:** Test name mismatch, documentation gap, or convention violation
   - **NOTE:** Minor observation

## Rules

- Do NOT fix anything. Report findings only.
- Do NOT review code quality, edge cases, or security -- that is `/review-for-issues`.
- After the user reviews your list, they will tell you which to fix.
- Maximum 2 review passes.
