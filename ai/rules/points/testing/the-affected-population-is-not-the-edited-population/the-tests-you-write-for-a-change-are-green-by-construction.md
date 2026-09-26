---
kind: directive
level: MUST
stage:
rationale: ai/rationale/testing.md
---
- **The tests you write for a change are written against its new contract, so they are green by construction and say nothing about whether the change is safe. The population that can go red is the one written against the old contract, which is exactly the population you did not edit, and it MUST be run before the change is claimed done.** Every gate here scopes itself to `git diff --name-status`, so that population is outside all of them and is yours to derive.
- **When a payload shape changes, you MUST search for the new key name as well as the old one.** Searching what you remove finds code that stops working; it cannot find a branch that already reads the key you added, for a different producer, and now handles your payload wrongly and quietly.
