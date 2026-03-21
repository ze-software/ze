# 001 — Initial Implementation Plan

## Objective

Foundation plan for rewriting ExaBGP in Go ("ZeBGP"). Shaped all subsequent structural decisions, directory layout, and design constraints.

## Decisions

- Zero-copy UPDATE forwarding when source and destination peers share encoding context — became the ContextID mechanism
- Per-attribute-type deduplication pools; plan used mutex-based typed stores, implementation evolved to pool-based handles with refcounting
- Lazy parsing of UPDATE messages via iterators (WireUpdate abstraction) — raw wire bytes kept, semantic content parsed on demand
- Goroutine-per-peer replaces Python's async reactor; channels coordinate FSM events
- No FIB manipulation — BGP protocol only, like ExaBGP; this boundary has never moved
- ExaBGP API compatibility as hard requirement: same JSON output, same API commands, same config format — drove the migration tooling
- AS-PATH-as-NLRI proposed as novel indexing approach; listed as risk with fallback, never carried into production

## Patterns

- Plugin/process communication over stdin/stdout: JSON events down, text commands up — this is the production plugin protocol
- Address families as "afi/safi" string format throughout JSON
- Config pipeline: file → parser → config struct → reactor → peers

## Gotchas

- `internal/reactor/` in the plan became `internal/component/bgp/` — reactor is inside the BGP subsystem, not top-level
- Plan used cobra+viper for CLI; actual implementation uses stdlib `flag.FlagSet` per subcommand
- Freeform config parsing was broken at plan close: nested data not extracted, block syntax required schema change — drove the YANG-based config redesign
- AS-PATH-as-NLRI was identified as risky and abandoned; production uses separate per-type attribute pools

## Files

Planned structure that shaped actual layout:
- `cmd/ze/` (planned as `cmd/ze/bgp/`)
- `internal/component/bgp/` (planned as `internal/bgp/` + `internal/reactor/`)
- `internal/component/config/` (planned as `internal/component/config/`)
- `internal/component/plugin/` (planned as `internal/component/plugin/` + `internal/api/`)
- `internal/exabgp/` (planned as migration guide only)
