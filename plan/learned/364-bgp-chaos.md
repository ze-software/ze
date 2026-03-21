# 364 — BGP Chaos Tool (Umbrella)

## Objective

Build ze-chaos, a deterministic chaos monkey tool for testing Ze's BGP route server. Simulates 3-50 peers, generates predictable UPDATEs across multiple address families, validates route propagation, and injects chaos events (disconnects, hold-timer expiry, malformed messages).

## Decisions

- 10 implementation phases, each with its own spec and learned summary (255-265)
- Seed-based deterministic generation — same seed produces same scenario for reproducibility
- Unbounded event buffer — no events ever dropped (ring buffer rejected: losing route events breaks convergence counts)
- Virtual clock for in-process testing (264), property-based shrinking for minimal failing cases (262)
- Web dashboard added as separate specs (357-358): HTMX+SSE, active set, controls, advanced viz

## Patterns

- Scenario → Peers → Events → Validation pipeline, all seed-deterministic
- Per-peer goroutine with channel for UPDATE dispatch
- Event log as NDJSON — replayable for debugging
- Property-based testing: symmetry, convergence, completeness assertions

## Gotchas

- Integration tests (spec-bgp-chaos-integration) and v2 actions (spec-chaos-actions-v2) are separate specs, not part of the core tool
- Config reload (SIGHUP) chaos action was deferred from Phase 5 to future work
- Web controls partial: no restart-with-new-seed, no v2 parameterized actions

## Files

- `cmd/ze-chaos/` — CLI, orchestrator, scheduler
- `internal/chaos/` — 14 subpackages (peer, route, scenario, validation, report, etc.)
- Sub-summaries: 255-265 (phases 1-10), 357-358 (web dashboard/controls)
