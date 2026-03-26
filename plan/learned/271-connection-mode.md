# 271 — Connection Mode

## Objective

Replace the `passive bool` peer option with independent `connect` and `accept` booleans to control outbound and inbound connections separately -- needed for `.ci` tests using ze-peer where Ze should only dial out. Config syntax: `local { connect false; }` (don't initiate outbound), `remote { accept false; }` (don't accept inbound). Both default to true.

## Decisions

- FSM keeps internal `passive bool` and `SetPassive(bool)` -- the session layer translates connect/accept booleans at the session boundary. Chosen over leaking into FSM internals; FSM only needs passive/non-passive distinction.
- `connect false` prevents outbound connections; `accept false` rejects inbound connections (defense in depth) -- two separate checks in reactor.
- All `.ci` test files updated to `local { connect false; }` -- Ze must not bind when ze-peer is already listening.
- ExaBGP migration: `passive true` maps to `local { connect false; }`.
- Environment vars `ze.bgp.bgp.connect` and `ze.bgp.bgp.accept` (booleans).
- Chaos runner sets connect/accept on profiles; config generator emits the syntax -- runner controls mode, generator handles syntax.

## Patterns

- Adapter pattern at the session boundary: booleans to legacy FSM API. Avoids cascading changes through FSM internals.
- Defaults (connect=true, accept=true) are the safe default -- existing behavior preserved when fields absent.

## Gotchas

- Environment vars were not initially wired to PeerSettings -- required a follow-up fix. Always verify env vars reach PeerSettings, not just config file parsing.
- Invalid boolean values should error, not silently warn -- an additional unit test was required after the bug was found.

## Files

- `internal/component/bgp/reactor/peersettings.go` -- connect/accept booleans in PeerSettings
- `ze-bgp-conf.yang`, `ze-bgp-api.yang`, `ze-hub-conf.yang` -- connect/accept leaves
- `reactor.go`, `peer.go`, `session.go`, `config.go`, `environment.go` -- updated
- `handler/bgp.go`, `validate/main.go`, `migrate.go`, `scenario/config.go` -- updated
- `.ci` files and Go test files -- mechanical rename
