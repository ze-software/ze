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

Reported and allowed through at EDIT time (exit 0, a notice, and the hook asks
for no row). Each is a COUNT falling, and a count cannot tell a deleted check
from three consolidated into one:

- removing assertions (any net drop)
- downgrading fatal assertions to non-fatal (`require` -> `assert`, `t.Fatal` -> `t.Error`)
- removing `t.Run` cases or table rows
- removing `.ci` `expect=` / `reject=` / `cmd=` lines

**The reported set MUST still be judged by the agent making the edit, and the
COMMIT that carries it MUST carry a row for it.** `weakened_problems`
(`scripts/dev/commit_helper.py`) records every weakening kind, count drops
included, so the commit asks for a row the hook did not. Say in the row which
happened: the coverage moved, or it went.
