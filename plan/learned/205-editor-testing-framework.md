# 205 — Editor Testing Framework

## Objective

Create a `.et` file-based test framework for the interactive config editor, enabling automated replay of key sequences with expected output assertions.

## Decisions

- `.et` file format (not Go table tests) — test cases are human-readable sequences of keystrokes and expected completions, easier to add cases without touching Go code.
- 90 functional tests added covering navigation, completion, and editing operations.

## Patterns

- Test runner replays key sequences against a headless editor instance and compares completion output to expectations.

## Gotchas

- `listKeyCompletions()` did not navigate to the correct YANG container before listing completions — node traversal must arrive at the target node, not the parent.
- Deep navigation path "peer 1.1.1.1" was stored as a single string instead of splitting on space into `["peer", "1.1.1.1"]` — any multi-token path element breaks split-based traversal.
- Environment completions limited: completer only supports the `ze-bgp` module; `ze-hub` and other modules return no completions (known limitation, not a bug).

## Files

- `internal/component/config/editor/testing/` — .et runner, headless editor, expect framework
- `test/editor/` — .et test files
- `internal/component/config/editor/completer.go` — listKeyCompletions bug fix
