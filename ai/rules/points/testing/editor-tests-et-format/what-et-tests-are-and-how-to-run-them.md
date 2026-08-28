---
kind: note
level:
stage:
---
`.et` files in `test/editor/` test the interactive TUI editor via headless simulation.
Infrastructure: `internal/component/cli/testing/` (parser, expect, headless, input, runner).
Run `./le functional editor`. Use focused Go tests under the compiled runner when iterating.
