# Functional Test Gate

**BLOCKING.** Every user-facing behavior MUST have a functional test that exercises
it through a user entry point. Unit tests (`_test.go`) prove internal logic.
Functional tests (`.ci`, `.et`) prove the feature works end-to-end through the daemon.
Both are required. Neither substitutes for the other.

## The Rule

When you add or change user-facing behavior, a corresponding functional test MUST
exist in the correct `test/` directory. "User-facing" means: reachable via CLI command,
config option, API call, web endpoint, plugin event, or wire protocol exchange.

## Required Test Type by Change

| Change type | Required functional test | Directory |
|-------------|------------------------|-----------|
| New/changed BGP wire behavior | `.ci` with `expect=bgp:` hex match | `test/encode/` or `test/decode/` |
| New/changed plugin behavior | `.ci` with API commands + expectations | `test/plugin/` |
| New/changed config option | `.ci` with parse success/failure | `test/parse/` |
| New/changed CLI subcommand | `.ci` with `cmd=foreground` + `expect=stdout` | `test/ui/` |
| New/changed web endpoint | `.ci` with HTTP expectations | `test/web/` |
| New/changed editor behavior | `.et` with input/expect directives | `test/editor/` |
| Config reload behavior | `.ci` with `action=sighup` | `test/reload/` |
| Managed/fleet behavior | `.ci` | `test/managed/` |
| Cross-component integration | `.ci` | `test/integration/` |
| Interoperability | `.ci` | `test/interop/` |

If the change does not fit any row (pure internal refactor, no user-visible effect),
no functional test is required. But if you are unsure, write one.

## When Unit Tests Alone Are Sufficient

Unit tests (`_test.go`) without a functional test are acceptable ONLY when:

| Condition | Example |
|-----------|---------|
| Pure internal logic with no user entry point | Helper function, data structure, algorithm |
| Existing functional test already covers the path | Bug fix where the `.ci` test already exercises the scenario |
| Wire encoding internals tested via round-trip | `pack -> unpack == original` in `_test.go`, AND a `.ci` encode test covers the message type |

In all other cases, both unit tests AND a functional test are required.

## Mechanical Check (MANDATORY before claiming done)

For every new or changed user-facing behavior in the diff:

```
# 1. Identify the feature's test directory from the table above
# 2. Check for a functional test covering the behavior
find test/<directory>/ -name "*.ci" -o -name "*.et" | xargs grep -l '<feature-keyword>'
```

If no functional test exists for a user-facing behavior, that is a BLOCKER.

## Common Violations

| Pattern | Why it's wrong |
|---------|----------------|
| "Unit tests cover this" | Unit tests prove the function works in isolation. They do not prove the daemon exposes the feature to users. |
| "The wiring test passes" | Wiring proves reachability. Functional tests prove correct behavior through the full path. |
| "I'll add the .ci test later" | Later never comes. The feature ships without end-to-end coverage. |
| "The behavior is too simple to need a functional test" | Simple behaviors break when config parsing, CLI dispatch, or plugin registration changes. The functional test catches that. |
| "There's no test infrastructure for this path" | Build the infrastructure or flag it as blocked. Do not skip the test. |

## Relationship to Other Rules

- `no-partial-completion.md` requirement #2 requires both unit AND functional tests per AC
- `wiring-completeness.md` checks that code is reachable; this rule checks that the reachable path is tested end-to-end
- `tdd.md` governs the test-first cycle; this rule governs test completeness at the feature level
- `testing.md` has the directory table and iteration workflow; this rule makes the directory mapping a gate
