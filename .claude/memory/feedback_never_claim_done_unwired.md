---
name: Completion claims require wiring proof
description: Before claiming any feature is done, verify it is reachable by users and provide evidence. Hooks enforce this at commit time.
type: feedback
---

Before claiming a feature is "done", "complete", or "implemented", answer:
**"Can a user reach and use this feature right now through config/CLI/API?"**

If no: say "logic implemented but not yet wired to user entry point". Never say "done".

**Why:** This was a recurring pattern -- unit tests pass, library code exists, but no user can reach the feature. The fix is a combination of habit (this checklist) and mechanical enforcement (hooks).

**How to apply:**
1. Name the user entry point (CLI command, config option, API call)
2. Name the .ci test that exercises it
3. If neither exists, the feature is blocked, not done

**Mechanical enforcement:**
- `check-wiring-at-commit.sh` warns when plugin code is staged without .ci tests
- `pre-commit-spec-audit.sh` blocks commits when spec audit tables are incomplete
- These hooks catch the failure even if I forget the checklist
