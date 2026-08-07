---
kind: table
level:
stage:
---
| Pattern | Why it's wrong |
|---------|----------------|
| "Unit tests cover this" | Unit tests prove the function works in isolation. They do not prove the daemon exposes the feature to users. |
| "The wiring test passes" | Wiring proves reachability. Functional tests prove correct behavior through the full path. |
| "The `.ci` is green" | A test that passes with the feature DISABLED (false-pass) guards nothing. Mutation-verify it: disable the producing function, confirm the test goes red. |
| "I'll add the .ci test later" | Later never comes. The feature ships without end-to-end coverage. |
| "The behavior is too simple to need a functional test" | Simple behaviors break when config parsing, CLI dispatch, or plugin registration changes. The functional test catches that. |
| "There's no test infrastructure for this path" | Build the infrastructure or flag it as blocked. Do not skip the test. |
