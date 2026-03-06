# 369 — Editor Workflow Tests

## Objective
Create comprehensive end-to-end workflow tests exercising realistic editor sessions across all node types.

## Decisions
- 9 `.et` files in `test/editor/workflow/` covering add, delete, update, navigate, multiple changes, and error cases
- Tests exercise real code paths through HeadlessModel — not mocks

## Patterns
- Workflow tests combine multiple commands (set, delete, edit, commit, show) in one `.et` file
- `expect=viewport:contains=` verifies content after show commands
- `expect=context:path=` verifies navigation state

## Gotchas
- `delete` on nonexistent leaf/peer is a silent no-op — `Tree.Delete` and `RemoveListEntry` don't check existence
- `DeleteListEntry` sets `dirty=true` even when the key doesn't exist — pre-existing behavior
- AC-7 (delete nonexistent returns error) only partially testable due to these silent no-ops

## Files
- `test/editor/workflow/workflow-*.et` — 9 workflow test files
