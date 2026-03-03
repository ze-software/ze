# 170 — API Command Restructure

## Objective

Restructure the IPC command namespace into `plugin`, `bgp`, `rib`, and `system` namespaces, replace config-driven event routing with API `subscribe` commands, and migrate `msgid`/`forward` commands to the RIB plugin.

## Decisions

- Event subscription is API-driven (not config-driven): plugin declares what it needs at runtime, enabling dynamic subscribe/unsubscribe and self-describing plugins without config file changes.
- `bgp cache` and `rib show/clear` commands become plugin-provided: engine exposes reactor methods; the RIB plugin registers the commands via Stage 1 `declare cmd`. Core engine has no knowledge of these commands.
- CBOR encoding removed: incompatible with line-delimited text protocol; no use case existed in practice.
- `session reset` removed: was resetting BGP settings; no longer needed with the new namespace structure.
- Format defaults to `parsed` (decoded fields, no wire bytes): most efficient for plugins that don't need raw bytes; `full` adds raw hex under `raw` key.
- Encoding (hex/base64/parsed/full) applies at event delivery time, not subscription time: plugin can change encoding mid-session.
- "sent" events for locally-originated routes have `message.id: 0` — no cached wire bytes for self-originated routes; only received UPDATEs have non-zero id for forwarding.

## Patterns

- Top-level `type` as discriminator (`"bgp"`, `"rib"`, `"response"`): consumer reads `type`, then reads the matching key. Avoids ambiguity between responses and events.
- `bgp cache <id> forward <sel>` — `<id>` is NOT a dispatcher-extracted selector; it's parsed by the RIB plugin handler itself. The dispatcher only extracts `peer <sel>` patterns.

## Gotchas

- Subscription timing: plugin subscribing after peers are established gets "future events only"; it must call `bgp peer * show` to get current state and reconcile.
- Event format changed structurally: attributes moved under `attributes` key, NLRI under `nlri` key, raw bytes under `raw`. All parsers (especially `rib/event.go`) needed updating.

## Files

- `internal/plugin/handler.go` — restructured dispatch
- `internal/plugin/session.go` — `plugin session` namespace
- `internal/plugin/subscribe.go` — subscription handlers (new)
- `internal/plugin/msgid.go`, `internal/plugin/forward.go` — removed (plugin-provided)
- `internal/plugin/rib/rib.go` — registers `bgp cache` + `rib show/clear` commands
