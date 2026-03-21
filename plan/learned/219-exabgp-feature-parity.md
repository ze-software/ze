# 219 — ExaBGP Feature Parity

## Objective

Achieve 37/37 ExaBGP compatibility test passes by implementing the remaining wire encoding features and bridging ExaBGP process invocations to Ze's API socket.

## Decisions

- ExaBGP processes are NOT converted to Ze plugins — the protocol is incompatible (ExaBGP uses stdout/stdin text, Ze uses NUL-framed JSON-RPC socket pairs). Instead, they are collected in `MigrateResult.Processes` and bridged by a wrapper script.
- The bridge script translates ExaBGP stdout text commands to Ze API socket RPC calls — this is an external-tools-only compatibility layer, per the compatibility rules.
- Flex-value tokenizer added to handle compound values with brackets and parentheses (e.g., `community (65000:1 65000:2)`) that the standard tokenizer rejected.

## Patterns

- When two systems have incompatible protocols, bridge at the boundary with a translation script rather than converting one system's internals to match the other.
- A flex-value tokenizer that handles grouped tokens (brackets/parens) is necessary for ExaBGP's config syntax, which uses these for multi-value attributes.

## Gotchas

- None.

## Files

- `internal/exabgp/migrate.go` — ExaBGP config migration producing MigrateResult
- `internal/exabgp/migrate_serialize.go` — serialization of migrated config to Ze format
- `internal/component/config/parser_freeform.go` — flex-value tokenizer
