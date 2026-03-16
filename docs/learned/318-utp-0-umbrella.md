# 318 — Unified Text Protocol (Umbrella)

## Objective

Migrate all plugin IPC to a single unified text grammar — converging event delivery, text commands, and the JSON-RPC handshake onto shared keyword tables, with separate tokenizers per protocol path suited to their different framing needs.

## Decisions

- "Shared tokenizer" plan revised to "shared keyword tables" — `TextScanner` (events, zero-alloc field scanning) and `tokenize()` (commands, quote handling) serve fundamentally different needs. Shared keyword constants in `textparse/keywords.go` are the integration point, not a shared tokenizer.
- Accumulator model (`set`/`add`/`del` on attributes) removed — attributes are declared once; only NLRI operations need `add`/`del`. The accumulator was overengineered.
- Heredoc JSON chosen for config delivery over flattening config to key-value — config is arbitrary YANG-modeled JSON that cannot be expressed as flat key-value without schema-specific encoding.
- Auto-detect from first byte (`{` → JSON, letter → text) instead of negotiation protocol — JSON plugins continue working unchanged; text mode is purely additive.
- Event format restructuring (uniform header, `nlri add/del` alignment with commands) documented as "Still Proposed" — not yet implemented, deferred to future specs.

## Patterns

- Three separate IPC paths (events, commands, handshake) remain separate but share keyword constants via `textparse/keywords.go`.
- Umbrella tracker updated as child specs complete — migration tracker table is the authoritative record of what's done vs. still proposed.

## Gotchas

- Internal text producers (`FormatRouteCommand`, `handleRouteRefreshDecision`) were broken when the command parser switched to flat grammar in utp-2 — any new text producer must be updated in lockstep with parser changes.
- Functional `.ci` tests for co-existence scenarios deferred — covered by integration unit tests (`TestTextAutoDetectHandshake`).

## Files

- `internal/component/bgp/textparse/keywords.go` — shared keyword constants, alias tables, `ResolveAlias()`
- `pkg/plugin/rpc/text.go`, `text_conn.go`, `text_mux.go` — text handshake serialization and framing (utp-3)
- `pkg/plugin/sdk/sdk_text.go` — SDK text mode (utp-3)
- `internal/component/plugin/subsystem_text.go`, `server_startup_text.go` — engine-side text paths (utp-3)
- Child specs: `302-utp-1-event-format.md`, `306-utp-2-command-format.md`, `315-utp-3-handshake.md`
