# 189 — Config Dispatch

## Objective

Allow `ze config.conf` to detect config type from content (BGP vs plugin) and dispatch accordingly, and have the hub parse config once and pass structured data to daemons.

## Decisions

- Detection uses `detectConfigType()` via tokenizer (not full parse) — cheap and sufficient; avoids duplicating the parser just for type sniffing.
- `cmd/ze/bgp/server.go` deleted (no-layering rule: replaced by dispatch logic, not kept alongside).

## Patterns

- Config type detection: scan tokens until a known top-level keyword (`bgp`, `plugin`) is found.

## Gotchas

- `--plugin` flag was intercepted as a subcommand rather than a flag — positional argument parsing must happen before flag parsing.
- `LoadReactorWithPlugins` merged plugin config AFTER reactor creation, so plugins were silently ignored. Plugin config must be passed at reactor construction, not appended later.

## Files

- `cmd/ze/config/` — dispatch logic, detectConfigType
- `internal/component/config/` — hub config parsing
