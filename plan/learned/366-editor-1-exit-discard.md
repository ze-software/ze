# 366 — Editor Exit/Discard Workflow

## Objective
Fix editor exit workflow: exit must work after discard/commit, editor must not appear dirty on open.

## Decisions
- Dirty flag is `atomic.Bool` on shared `*Editor` pointer — cleared by `Discard()` and `Save()`
- Exit checks `Dirty()` at `model.go:514` — blocks when true, allows when false
- Three-way commit message: committed + reloaded / committed + daemon not running / committed

## Patterns
- Bubbletea value-type Model with pointer-shared `*Editor` — mutations through pointer are visible across copies
- `.et` functional tests for lifecycle scenarios (dirty tracking, exit behavior)
- `testValidBGPConfigWithPeer` shared config constant for tests needing peers

## Gotchas
- User-reported "false dirty on open" could not be reproduced in tests — may be caused by `.edit` file from previous crash or serialization round-trip drift
- Serialization round-trip test confirms parse → serialize → parse → serialize is stable

## Files
- `internal/component/config/editor/model_test.go` — TestExitAfterDiscard, TestExitAfterCommit, TestNoFalseDirtyOnOpen
- `internal/component/config/editor/editor_test.go` — TestDirtyFalseAfterDiscard, TestSerializationRoundTrip
- `test/editor/lifecycle/exit-*.et` — 4 functional tests
