# 069 — API Command Serial Numbers

## Objective

Add optional serial numbers to the process↔ZeBGP protocol for command/response correlation, enabling reliable delivery tracking and serving as the foundation for plugin command routing.

## Decisions

- Serial numbers are optional per-command (not per-process): commands with `#N` prefix get a response; commands without do not
- Two namespaces by design: process uses numeric serials (`#1`, `#2`), ZeBGP uses alpha serials (`#a`, `#b`) — zero collision, instantly distinguishable in logs
- ZeBGP→Process direction is configurable (JSON or text); Process→ZeBGP is always text
- Events (ZeBGP-initiated notifications) use empty serial (`"serial": ""`) — no response expected
- Alpha serial increments like Excel columns: a–z, aa–az, ba–bz... practically infinite (26^10 combinations)
- Per-command detection (not per-process flag) allows incremental adoption without breaking existing processes

## Patterns

- `Response.Serial` is always a string even for numeric serials — avoids JSON type ambiguity between numeric and alpha serials in the same channel
- `#` prefix = "I'm setting this serial"; `@` prefix = "I'm responding to this serial" — mnemonic: # sets, @ replies

## Gotchas

- Responses may arrive out of order (async processing) — API programs must not assume in-order delivery
- No per-process serial state needed: each command carries its serial, ZeBGP tracks which commands need responses based on presence of `#N` prefix

## Files

- `internal/plugin/process.go` — serial parsing, `nextAlphaSerial()`, response formatting
- `internal/plugin/types.go` — Response struct (Serial, Status, Data)
