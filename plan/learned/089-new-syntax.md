# 089 — New API Syntax

## Objective

Define the unified `update <encoding> [<attr-ops>]... [nhop <set|del>]... [nlri <family> <add|del>...]...` grammar for both API commands and config file format, replacing `announce route` and `announce attributes`.

## Decisions

- Encoding keyword before attributes: `update text ...`, `update hex ...` — encoding determines how ALL subsequent tokens (attrs and NLRIs) are interpreted.
- `nhop` as a top-level accumulator, not inside `attr`: next-hop is set once and applies to all subsequent `nlri` sections until changed. Per-family nhop override possible inside `nlri` section.
- Input puts nhop outside nlri (convenience), output puts nhop inside nlri section (per-family explicit): different placement for ergonomics vs determinism.
- `path-information` is a scalar accumulator (like nhop): changes mid-command to assign different path IDs to different NLRI sections.
- API and config share the same token stream after tokenization: API splits on whitespace, config tokenizer strips `{ } ;`.
- `watchdog set <name>` inside nlri `add` op tags routes for pool membership; `watchdog announce/withdraw <name>` are standalone commands.
- Old commands (`announce route`, `announce attributes`, `withdraw route`) deprecated with migration table.

## Patterns

- `raw <msg-type> <encoding> <data>` passthrough command: sends bytes without any parsing or validation. Separate from `update` which is structured.

## Gotchas

- Wire mode only supports `attr set` (not add/del): you cannot manipulate raw bytes at the attribute level.
- This spec was partially implemented at time of filing: parser (Phase 1) complete, hex/b64 handlers (Phase 2) stubs, wire encoding output (Phase 3) partial, .run file migration (Phase 4) not started.

## Files

- Design document / grammar reference — implementation split across specs 081, 082, 087, 088
