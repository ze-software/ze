# 120 — JSON Direction In Message

## Objective

Move the `direction` field from the JSON root level into the `message` wrapper object, so all common message metadata (type, id, direction) lives in one place.

## Decisions

- Chose to add `GetDirection()` method on `Event` struct for easier consumer access after the field moved to a nested location.

## Patterns

- The `message` object in Ze BGP JSON is now the single owner of all wire metadata: `type`, `id`, `direction`, `time`. Fields that describe how/when a message was received belong in `message`, not at root.

## Gotchas

None.

## Files

- `internal/component/plugin/json.go` — `setMessageDirection()` helper, updated all message types
- `internal/component/plugin/text.go` — `formatFilterResultJSON()`, `formatRawFromResult()`
- `internal/component/plugin/rib/event.go` — `Direction` moved into `MessageInfo`, `GetDirection()` added
