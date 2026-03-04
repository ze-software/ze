# 345 — Hub Phase 2: Config Reader Process

## Objective

Create the `ze-config-reader` binary — a specialized process that receives YANG schemas from the Hub, parses the config file using the existing tokenizer, and sends `config verify` requests to the Hub for each parsed block.

## Decisions

- Config Reader does NOT participate in Stage 1 (no declare/capability/ready cycle). It is spawned after Stage 1 completes when all plugin schemas are already collected. Hub pushes schemas to it directly, then sends `config path`.
- Used existing `internal/config.NewTokenizer()` rather than creating a new parser — the tokenizer already handles the config syntax correctly.
- Handler path format: `block.nested[key=value]` (e.g., `bgp.peer[address=192.0.2.1]`) — consistent with SchemaRegistry predicate format from Phase 1.
- Longest-prefix match for handler routing — consistent with SchemaRegistry pattern.

## Patterns

- Hub sends YANG content inline (heredoc) to Config Reader, which receives all schemas before reading the config file. Sequence: init (schemas + path) → `config done` → parse and send verify requests → `config complete`.

## Gotchas

- Functional tests deferred — require Hub integration from Phases 3-4 to work end-to-end. Hub spawning logic similarly deferred.

## Files

- `cmd/ze-config-reader/main.go` — Config Reader binary with full message lifecycle
- `Makefile` — added `ze-config-reader` build target
