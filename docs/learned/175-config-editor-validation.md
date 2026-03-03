# 175 — Config Editor Validation

## Objective

Add real-time validation, edit file persistence, and commit-time semantic validation to the config editor.

## Decisions

- Validator renamed `ConfigValidator` (not `Validator`) — `Validator` already exists in `internal/component/config/yang/validator.go`; collision discovered at compile time.
- Semantic validation in `validator.go`, not separate `semantic.go` — simpler structure, no benefit to splitting a small rule set.
- Warning prompts deferred — current implementation blocks on errors; distinguishing warning-only cases not yet implemented.
- Edit file is `<name>.conf.edit` adjacent to the original — easy to discover; deleted on commit or discard, preserved on exit.

## Patterns

- `commandResult` struct — commands return a result that the `Update()` handler applies, instead of mutating model state directly. Required to fix a Bubble Tea closure-capture bug where closures captured model by value, losing state changes.
- `LazyLogger` pattern foreshadowed here — debounced validation uses a `validationTickMsg` with a generation counter to ignore stale ticks after rapid keystrokes.
- Hierarchical context paths: `findFullContextPath()` walks the brace structure to build paths like `["bgp", "peer", "1.1.1.1"]`. Exact block matching required — prefix matching caused `peer` to match `peer-as`.

## Gotchas

- Brace handling order: `} foo {` on a single line must process `}` before `{`, otherwise depth tracking is wrong.
- Wildcard template mode (`edit peer *`) cannot use the normal block-not-found error path — needs `findParentOfKeyword()` to locate the parent block where the keyword exists.
- `cmdCommit()` must re-validate inline, not use the model's stale validation state — validation result cached in model may be from a previous state.

## Files

- `internal/component/config/editor/validator.go` — ConfigValidator: syntax, semantic, mandatory field validation
- `internal/component/config/editor/model.go` — commandResult pattern, debounced validation, hierarchical context paths, template mode
- `internal/component/config/editor/editor.go` — HasPendingEdit, LoadPendingEdit, SaveEditState, PendingEditDiff
