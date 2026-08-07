---
kind: note
level:
stage:
---
`.et` files in `test/editor/` test the interactive TUI editor via headless simulation.
Infrastructure: `internal/component/cli/testing/` (parser, expect, headless, input, runner).
Run: `make ze-editor-test` or `bin/ze-test editor --all`; select by id/name with `bin/ze-test editor N`, and filter with `bin/ze-test editor --pattern <name>`.
