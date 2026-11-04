---
kind: directive
level: MUST
stage:
rationale: ai/rationale/testing.md
---
- **The tests you write for a change are written against its new contract, so they are green by construction and say nothing about whether the change is safe. The population that can go red is the one written against the old contract, which is exactly the population you did not edit, and it MUST be run before the change is claimed done.** Every gate here scopes itself to `git diff --name-status`, so that population is outside all of them and is yours to derive.
- **When a payload shape changes, you MUST search for the new key name as well as the old one.** Searching what you remove finds code that stops working; it cannot find a branch that already reads the key you added, for a different producer, and now handles your payload wrongly and quietly.
- **A design that changes a wire MUST derive its census from actual producer/consumer source searches and MUST account for every writer and reader site, including unchanged and non-Go carriers.** The search evidence MUST name its scope, terms and a known producer/consumer control; Files to Modify and the first files opened are not that population. The harness-model design judgment in `ai/skills/ze-spec.md`, "Design admission judgment", refuses each missing known site independently. A source-read digest proves freshness, not complete coverage.
