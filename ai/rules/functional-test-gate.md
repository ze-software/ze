# Functional Test Gate

**When:** Every user-facing behavior MUST have a functional test that exercises
**Severity:** blocking

## Directives

Every user-facing behavior MUST have a functional test that exercises
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

## Mutation-Verify the Test Actually Gates (MANDATORY for behavior-guarding tests)

A functional test that EXISTS is not the same as one that GATES. A `.ci`/`.et` can
pass whether or not the feature works -- a **false-pass** -- when the observed effect
reaches the assertion by a path OTHER than the one under test. Real example: three
`redistribute-late-join*.ci` tests kept passing with the late-join replay
(`handleReplayBatch`) disabled, so the route reached the peer by some path other than
the replay -- they guarded nothing and shipped green
(`plan/learned/1062-redistribute-late-join-replay.md`).

For every NEW or CHANGED `.ci`/`.et` that is meant to guard a SPECIFIC behavior:

1. Disable the producing function (the code the test exists to prove) -- an early
   `return`, a no-op, or `if true { return }` at the top of the function.
2. Re-run the test. It MUST flip to RED. If it still passes, the test does not gate
   on the feature: find the alternate delivery path and design it out (inject with no
   peers, remove the fallback store, use a genuinely-new peer instead of a reconnect),
   or the test is worthless -- delete it, do not ship it.
3. Revert the mutation immediately and confirm the test is green again.

This is a MANUAL discipline. `make ze-mutation-test` / `ze-mutation-changed` (gomu,
see `testing.md`) mutates Go source and runs only `go test` UNIT tests -- it never
executes `.ci`/`.et`, so it cannot catch a functional false-pass. Nothing else in the
pipeline does either.

If a test genuinely cannot be made to fail under mutation because the behavior is not
observable end-to-end (e.g. the reactor suppresses a duplicate announce, so per-peer
targeting is wire-indistinguishable), guard it with a UNIT test that inspects the
producing value directly, and say so in the test comment. Do NOT keep a `.ci` that
passes with the feature disabled.

## Common Violations

| Pattern | Why it's wrong |
|---------|----------------|
| "Unit tests cover this" | Unit tests prove the function works in isolation. They do not prove the daemon exposes the feature to users. |
| "The wiring test passes" | Wiring proves reachability. Functional tests prove correct behavior through the full path. |
| "The `.ci` is green" | A test that passes with the feature DISABLED (false-pass) guards nothing. Mutation-verify it: disable the producing function, confirm the test goes red. |
| "I'll add the .ci test later" | Later never comes. The feature ships without end-to-end coverage. |
| "The behavior is too simple to need a functional test" | Simple behaviors break when config parsing, CLI dispatch, or plugin registration changes. The functional test catches that. |
| "There's no test infrastructure for this path" | Build the infrastructure or flag it as blocked. Do not skip the test. |

## Relationship to Other Rules

- `no-partial-completion.md` requirement #2 requires both unit AND functional tests per AC
- `wiring-completeness.md` checks that code is reachable; this rule checks that the reachable path is tested end-to-end
- `tdd.md` governs the test-first cycle; this rule governs test completeness at the feature level
- `testing.md` has the directory table and iteration workflow; this rule makes the directory mapping a gate
