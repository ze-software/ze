---
kind: directive
level: MUST
stage:
---
- **The tests you write for a change are written against its NEW contract, so they are green by construction and say nothing about whether the change is safe. The population that CAN go red is the one written against the OLD contract, which is exactly the population you did not edit, and it MUST be run before the change is claimed done.** Every gate here scopes itself to `git diff --name-status`, so that population is outside all of them and is yours to derive.
- **When a payload SHAPE changes, you MUST search for the NEW key name as well as the old one.** Searching what you remove finds code that stops working; it cannot find a branch that already reads the key you added, for a different producer, and now handles your payload wrongly and quietly.

Measured on 2026-08-22: `clear_debt` (`internal/le/commit`) changed the argument its `GateRunner` receives from the repo root to the throwaway worktree. Four new tests were green and six existing `TestDebtClear` cases were red, two of them a genuine semantic break. Measured again on 2026-08-23, when `show bgp rib` moved to flat rows: `internal/component/lg` was never edited, and its `extractRoutes` captured the new shape and returned rows unnormalized, so the looking-glass graph answered `No routes found`, which reads as a true answer about an empty RIB.
