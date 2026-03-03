# 212 — Inline Config Reader

## Objective

Replace the external `ze-config-reader` binary with an in-process library, eliminating subprocess overhead and enabling direct config parsing from Go.

## Decisions

- `_default` is used as a sentinel key for singleton (non-list) blocks — it captures the single unnamed block within a container, as opposed to named list entries.
- `TokensToJSON` captures parent block structure including list membership — the function must emit the enclosing block key so callers can reconstruct the full path.

## Patterns

- In-process library with a clean Go API is strictly superior to subprocess: no fork overhead, no JSON serialization, error propagation is direct.

## Gotchas

- The `_default` sentinel was non-obvious — understanding when a block is a singleton vs. a named list entry requires careful handling of the config grammar.
- `parseBlocks` recursion had a subtle optimization: handling of nested blocks required tracking parent context to avoid re-emitting keys.

## Files

- `internal/component/config/reader.go` — in-process config reader replacing the binary
- `internal/component/config/tokenizer.go` — tokenizer feeding the reader
