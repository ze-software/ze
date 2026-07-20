# Friction Reporting

**When:** During research, implementation, review, or verification, if you encounter a recurring problem pattern, repeated surprise, stale guidance, tooling friction, or wasted effort, report it immediately and say whether a new or changed rule would prevent it.
**Severity:** advisory

## Report immediately when

| Category | Examples |
|----------|----------|
| **Problem pattern** | The same mistake, rejected edit, missing wiring, misunderstood boundary, or unexpected failure appears more than once or is likely to recur |
| **Rule gap** | Existing rules did not say what to do, gave conflicting guidance, or made the wrong path look valid |
| **Missing docs** | Had to investigate something that should have been documented: file purpose, data flow step, registration pattern, gate behavior |
| **Stale info** | Rule or doc references deleted/renamed files, describes a pattern the code no longer follows |
| **Tooling friction** | Hook rejects valid code, linter config does not match rules, make target behaves unexpectedly |
| **Wasted effort** | Searched in the wrong place, duplicated existing functionality, misunderstood a layer boundary |

## Format

```
Friction: [what happened]
Pattern: [why this is likely to recur, or "one-off" if unsure]
Impact: [time/effort wasted, bug risk, or review risk]
Rule decision: [new rule needed / update existing rule / no rule, because...]
Proposed fix: [specific ai/rules, ai/INDEX, plan/learned, docs, or hook change]
```

## Timing

- Report as soon as you can describe the pattern. Do not wait until the end of the session.
- **Reporting in chat is not filing.** Chat scrolls away and the next session never sees it, so hook and tooling friction is not reported until it is written to `plan/learned/HOOK-FRICTION.md` in the Format above; a finding you only pass to the next agent in a handoff is folklore, not a record.
- If the user task is still in progress, keep working after reporting unless blocked or the rule change would alter scope.
- If the pattern changes a project workflow, add or update the narrowest rule or learned record before claiming completion.

## Do Not Report

- Things that are simply unfamiliar before reading the relevant docs.
- Intentional deviations already documented in specs or rationale files.
- One-off issues that will not recur and expose no rule gap.
