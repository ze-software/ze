# 207 — Schema Plugin YANG

## Objective

Implement `ze schema show` and `ze schema list` using real YANG content, including YANG contributed by external plugins.

## Decisions

- External plugin YANG retrieved via `exec.CommandContext` with a 10-second timeout — plugins are already trusted (they run as subprocesses anyway); timeout prevents hang if a plugin is broken.
- Functional tests placed in `test/parse/` (existing location) rather than a new `test/schema/` directory — avoids creating infrastructure for a single command.

## Patterns

- `ze schema show <module>` → fetch YANG text → render; `ze schema list` → enumerate registered modules.

## Gotchas

- goyang's `Module.Import` field is `[]*Import` (a slice), not a `map[string]*Import` — iterating it with map syntax causes a compile error; always check the actual goyang struct definition, not what you expect from the name.

## Files

- `cmd/ze/schema/` — show and list subcommands
- `internal/component/config/yang/` — YANG module enumeration
- `test/parse/` — functional tests for schema commands
