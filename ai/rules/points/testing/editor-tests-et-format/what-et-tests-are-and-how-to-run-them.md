---
kind: directive
level: MUST
stage:
---
An editor test MUST be an `.et` file in `test/editor/`, which drives the interactive
TUI editor through headless simulation (`internal/component/cli/testing/`: parser,
expect, headless, input, runner). Run the suite with `./le functional editor`, and use
focused Go tests under the compiled runner while iterating.
