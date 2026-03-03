# 191 — Decode Plugin Text

## Objective

Extend the decode protocol to support native text output from plugins (`decode text capability <code> <hex>`), and implement text formatting in Hostname and FlowSpec plugins.

## Decisions

- Format specifier (`json`/`text`) inserted between `decode` and object type — preserves backward compatibility with `decode json ...` while adding `decode text ...`; symmetric with encode direction.
- `decoded text` added as a new response type alongside `decoded json` — allows engine to forward plugin text response verbatim without re-serializing.

## Patterns

- None beyond the extended protocol format.

## Gotchas

- Test runner used `cmd:` as prefix but `.ci` files use `cmd=` — silent mismatch caused tests to not find the command. Always verify exact separator used in both test runner parser and test files.

## Files

- `internal/plugins/bgp/decode/` — protocol extension, text response type
- `internal/plugins/hostname/` — text formatter
- `internal/plugins/flowspec/` — text formatter
- `test/decode/` — .ci files with cmd= prefix
