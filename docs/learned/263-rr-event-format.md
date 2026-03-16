# 263 — RR Event Format Fix

## Objective

Fix the route-reflector/route-server plugin's event parsing, which was completely non-functional because its Event struct and parsing code expected a schema that did not match the ze-bgp JSON envelope the engine actually sends.

## Decisions

- Two-phase JSON parsing (envelope unwrap then payload parse) is the established ze-bgp pattern — `bgp-rib/event.go` already does this correctly and is the reference implementation.
- RR only needs a small subset of event data: event type, peer address/ASN, message ID, families from OPEN, and family+prefix from UPDATE. No path attribute parsing needed because RR uses zero-copy forwarding (`cache N forward`).
- Complete rewrite of both `server.go` event types and `server_test.go` — the old tests bypassed JSON parsing entirely by constructing Go structs directly, giving false confidence while the bug existed for all production events.

## Patterns

- Tests for JSON-processing code must parse JSON strings, not construct structs — constructing structs bypasses the exact code path that contains the bug. The RR bug existed precisely because tests never exercised `parseEvent()`.

## Gotchas

- Four distinct format mismatches existed simultaneously: (1) envelope — RR expected flat JSON, engine sends `{"type":"bgp","bgp":{...}}`; (2) peer — RR expected nested `{"address":{"local":...}}`, engine sends flat `{"address":"10.0.0.1"}`; (3) UPDATE body — RR expected ExaBGP-style `announce`/`withdraw` maps, engine sends `nlri` key with `[{action, next-hop, nlri}]` arrays; (4) capabilities — RR expected space-delimited strings, engine sends objects with code/name/value.
- BoRR/EoRR (RFC 7313) route refresh subtypes arrive with `message.type = "borr"`/`"eorr"`, but the RR dispatch only matches `"refresh"` — silently ignored. Intentional: a forward-all route server has no need to track refresh cycle boundaries.
- Functional test deferred — requires two simultaneous ze-peer instances, which the current test framework does not support.

## Files

- `internal/component/bgp/plugins/rs/server.go` — Rewritten Event struct, parseEvent, all 4 handlers
- `internal/component/bgp/plugins/rs/server_test.go` — Rewritten with 21 tests, all parsing real ze-bgp JSON
