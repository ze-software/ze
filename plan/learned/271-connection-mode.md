# 271 — Connection Mode Enum

## Objective

Replace the `passive bool` peer option with a `connection` enum (`both`, `passive`, `active`) to support active-only mode where Ze never binds/listens for a peer — needed for `.ci` tests using ze-peer where Ze should only dial out.

## Decisions

- FSM keeps internal `passive bool` and `SetPassive(bool)` — the session layer translates `ConnectionMode` → bool at the session boundary. Chosen over leaking the enum into FSM internals; FSM only needs passive/non-passive distinction.
- `active` mode prevents listener startup AND rejects inbound connections (defense in depth) — two separate checks in reactor.
- All 89 `.ci` test files updated to `connection active;` — Ze must not bind when ze-peer is already listening.
- ExaBGP migration: `passive true` → `connection passive`; no passive field → `connection active` (ExaBGP peers dial out by default).
- Environment var `ze_bgp_bgp_connection` wired as string field to avoid enum parsing complexity at env-var layer.
- Chaos runner sets `ModePassive` on profiles; config generator emits the string — runner controls mode, generator handles syntax.

## Patterns

- Adapter pattern at the session boundary: infrastructure enum → legacy bool API. Avoids cascading changes through FSM internals.
- Zero value `ConnectionBoth` is the safe default — existing behavior preserved when field absent.

## Gotchas

- Environment var was not initially wired to PeerSettings — required a follow-up fix (9918bba3). Always verify env vars reach PeerSettings, not just config file parsing.
- Invalid connection mode should error, not silently warn — an additional unit test was required after the bug was found (48d7f35a).

## Files

- `internal/component/bgp/reactor/peersettings.go` — `ConnectionMode` type (created)
- `ze-bgp-conf.yang`, `ze-bgp-api.yang`, `ze-hub-conf.yang` — connection leaf added
- `reactor.go`, `peer.go`, `session.go`, `config.go`, `environment.go` — updated
- `handler/bgp.go`, `validate/main.go`, `migrate.go`, `scenario/config.go` — updated
- 89 `.ci` files and 17+ Go test files — mechanical rename
