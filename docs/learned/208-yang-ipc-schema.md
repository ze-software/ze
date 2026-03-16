# 208 — YANG IPC Schema

## Objective

Define the IPC wire format (NUL-terminated JSON) and create the `internal/ipc/` package with framing, message types, and method parsing. First of three specs implementing the YANG IPC system.

## Decisions

- NUL (`\x00`) as message terminator instead of newline — BGP and JSON payloads can legitimately contain newlines; NUL cannot appear in valid JSON or BGP wire bytes.
- API YANG modules placed near their domain code (e.g., `internal/component/hub/schema/`, `internal/bgp/schema/`) rather than centrally in `internal/yang/modules/` — schema belongs to the subsystem that owns the API.
- `ze-ipc` CLI and functional tests deferred to Spec 3 (spec-yang-ipc-3).

## Patterns

- `bufio.Scanner` with `SplitFunc` on NUL byte handles framing; scanner buffer must be set to `MaxMessageSize + 1` (exclusive upper bound).

## Gotchas

- `bufio.Scanner` max buffer is exclusive: a message of exactly `MaxMessageSize` bytes fails unless the buffer is `MaxMessageSize + 1`. Off-by-one causes silent truncation of maximum-length messages.

## Files

- `internal/ipc/` — NUL framing, message types, method parsing
- `internal/component/hub/schema/`, `internal/bgp/schema/`, etc. — domain-local API YANG
