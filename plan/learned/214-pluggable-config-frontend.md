# 214 — Pluggable Config Frontend

## Objective

Introduce a `ConfigFrontend` interface so tokenizer and SetParser share a common `map[string]any` output contract, enabling alternative frontends (YANG JSON, future formats) without changing downstream consumers.

## Decisions

- `tokensToNestedMap` produces `map[string]any` directly instead of going through a JSON roundtrip (`tokensToJSON` → `json.Unmarshal`). The roundtrip was removed as unnecessary indirection.
- `SetParser`'s `ValidateValue` method was removed: YANG is now the sole validator, so per-frontend validation is redundant and creates confusion about where validation lives.
- `parseBlocks` and `TokensToJSON` were removed as dead code once the direct map path was established.

## Patterns

- Eliminating an intermediate serialization format (JSON string) in favour of direct type construction reduces both latency and cognitive overhead.

## Gotchas

- None.

## Files

- `internal/component/config/setparser.go` — SetParser frontend implementation
- `internal/component/config/parser.go` — tokenizer-based frontend
