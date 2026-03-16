# 040 — API Encoder Switching

## Objective

Redesign process API configuration to properly separate WHAT messages a peer sends/receives from HOW they are formatted, fixing a bug where `encoder json` processes received nothing or received text-format messages.

## Decisions

- New config syntax: `api foo { content { format ...; } receive {...} send {...} }` — named blocks per process instead of global flags.
- The old syntax (`api { processes [...]; neighbor-changes; }`) is preserved via `_anonymous` key for backward compatibility.
- `all;` keyword in receive/send blocks expands to all flags — convenience shorthand.
- Reserved name validation: process names starting with `_` are rejected (internal use by the anonymous key mechanism).
- Encoding inheritance: per-peer binding → named process config → default "text" — resolved in `GetPeerAPIBindings()`.
- Deterministic binding order: map keys sorted before iteration to avoid non-deterministic test failures.

## Patterns

- `PeerAPIBinding` is the new struct threading through config → loader → reactor → api, carrying process name + content/receive/send config.
- Process names starting with `_` are reserved — important for future internal use.

## Gotchas

- The original bug: `bgp.go:548` had `if pc.Encoder == "text" { pc.ReceiveUpdate = true }` — JSON processes got no messages because the flag was never set for them.
- `processes-match` and `neighbor-changes` are ExaBGP compatibility fields — needed in schema even though ZeBGP does not fully implement them.

## Files

- `internal/component/config/bgp.go` — new API binding parsing, `GetPeerAPIBindings()`
- `internal/reactor/reactor.go` — `APIBinding` struct, dispatch logic
- `internal/component/plugin/route.go` — per-binding format/encoding awareness
