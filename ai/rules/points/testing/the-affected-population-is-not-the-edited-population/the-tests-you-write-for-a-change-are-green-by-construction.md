---
kind: directive
level: MUST
stage:
rationale: plan/journal/green-that-could-not-have-been-red.md
---
**The tests you write for a change are written against its NEW contract, so they are green by construction and cannot tell you the change is safe.** The population that CAN go red is the one written against the OLD contract, and that is exactly the population you did not edit. "My tests pass" is therefore not evidence about a contract change. It is a restatement of what you just wrote down.

**So when a change alters what a function HANDS its callee, or what a shared artifact CLAIMS, you MUST run the whole suite for every file whose contract moved before claiming done, never only the cases you added.** Both of those are contracts, and neither is visible from a new test. A changed-file gate cannot help: it scopes to the diff, and the old-contract tests are outside it.

Measured on 2026-08-22. `clear_debt` (`scripts/dev/commit_helper.py`) changed the argument its `GateRunner` receives from the repo root to the throwaway worktree the gate runs in. Four new tests were green, and six existing `TestDebtClear` cases were RED. Four failed because the fixture never committed, so there was no HEAD to materialize. Two were a genuine semantic break: their runners wrote to the ledger THROUGH that argument, and the directory a gate runs in had stopped being the directory the ledger lives in. An author running only their own tests would have shipped it.

**A sentence that has been wrong in two OPPOSITE directions is a surface nobody owns, and it MUST be given a test rather than corrected again.** The same commit found one: a shard header claimed the pass judged `over the committed code` while every gate judged the working tree, was corrected to claim the WORKING TREE, and this change made that false in turn. Correcting it a third time buys nothing. Asserting the current claim, with both wrong versions named in the test, is what stops a fourth.
