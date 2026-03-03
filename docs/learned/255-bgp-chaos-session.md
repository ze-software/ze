# 255 — BGP Chaos Session

## Objective

Phase 1 of `ze-bgp-chaos`: CLI skeleton, seed-based deterministic scenario generation, Ze config output, and a working single-peer BGP session announcing `ipv4/unicast` routes.

## Decisions

- Seed-based generation: same seed + same flags → identical `PeerProfile` slice for deterministic test reproduction.
- `--peers` bounds: 1–50 (error on 0 or 51); `--ibgp-ratio` clamped 0.0–1.0.
- Chaos tool imports wire-encoding packages only — it is an external BGP peer, not an engine plugin.

## Patterns

- Standalone binary structure: `cmd/ze-bgp-chaos/scenario/` (profiles, routes, config gen) + `cmd/ze-bgp-chaos/peer/` (session, sender).
- Config generation is validated by `ze validate` — generated config must be YANG-valid.

## Gotchas

Spec archived with unfilled audit table — implementation status at archive time was a planned-but-not-yet-done state. Verify actual implementation before treating as complete.

## Files

- `cmd/ze-bgp-chaos/main.go` — CLI entry point, flag parsing
- `cmd/ze-bgp-chaos/scenario/generator.go`, `profile.go`, `routes.go`, `config.go` — seed → profiles → config
- `cmd/ze-bgp-chaos/peer/session.go`, `sender.go`, `simulator.go` — TCP BGP session + UPDATE sending
