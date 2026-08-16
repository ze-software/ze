---
kind: directive
level: MUST NOT
stage:
---
**Every weakening below MUST NOT be introduced. What the hook REFUSES is a
narrower set than what this rule forbids, and the difference is not permission.**

Refused on Edit / Write / MultiEdit to a test file (exit 2). Each is
one-directional: no innocent edit produces it.

- adding `t.Skip` / `t.Skipf` / `t.SkipNow` (the test stops running)
- commenting out assertions
- adding an `ignore` build tag (file dropped from the build)
- deleting a `Test`/`Fuzz`/`Benchmark` func
- emptying a `.ci` expectation's needle, so it matches anything
- introducing an assertion that cannot fail
- replacing a test's content with nothing

Reported and allowed through (exit 0, a notice, and NO `test-relax:` token is
wanted for it). Each is a COUNT falling, and a count cannot tell a deleted check
from three consolidated into one:

- removing assertions (any net drop)
- downgrading fatal assertions to non-fatal (`require` -> `assert`, `t.Fatal` -> `t.Error`)
- removing `t.Run` cases or table rows
- removing `.ci` `expect=` / `reject=` / `cmd=` lines

**The reported set MUST still be judged by the agent making the edit.** The hook
refused on these until 2026-08-16, and that is where the `test-relax:` corpus
came from: 780 tokens, of which reading the 402 nobody had triaged found 146
saying the coverage still existed and 19 recording a real loss. Three in four
excused an improvement, because consolidating cases and replacing a poll loop
with a barrier both lower a count exactly as deleting a check does. A gate whose
output nobody can read is not a gate.
