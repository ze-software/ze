# 077 — Unify MessageID and JSON Format Restructure

## Objective

Remove duplicate `UpdateID` from `ReceivedUpdate` (same value as `WireUpdate.messageID`), and restructure JSON output to use a common `"message":{type, id, time}` wrapper with message-specific fields at the top level.

## Decisions

- Moved UPDATE's message ID into `WireUpdate.messageID` (set by reactor after creation via `SetMessageID()`): the natural owner of the ID is the message itself, not a separate struct.
- Chose `"message":{...}` common wrapper over flat structure: allows consumers to distinguish common metadata from message-specific fields without naming collisions.
- Breaking JSON API change accepted: Ze has never been released, no compatibility constraints apply.

## Gotchas

- The atomic counter (`msgIDCounter`) stayed in reactor, not in WireUpdate: ID is assigned by the reactor, not self-assigned by the update. Constructor leaves `messageID` zero; `SetMessageID()` is called once by reactor after creation.
- JSON field rename: `"msg-id"` → `"message"."id"`, `"type"` → `"message"."type"`, `"time"` → `"message"."time"`. Consumers accessing `msg["type"]` will break.

## Files

- `internal/component/plugin/wire_update.go` — messageID field, SetMessageID()
- `internal/reactor/received_update.go` — removed UpdateID field
- `internal/reactor/recent_cache.go` — cache key changed to WireUpdate.MessageID()
- `internal/component/plugin/json.go` — all message type encoders restructured
