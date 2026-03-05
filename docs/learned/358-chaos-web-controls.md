# 358 — Chaos Web Controls

## Objective

Add interactive controls to the chaos web dashboard: control channel from web server to orchestrator, scheduler pause/resume/setRate, control panel UI, parameterized trigger form for targeting specific peers with specific chaos actions, and stop/restart with new seed.

## Decisions

- Buffered control channel (capacity 16) from HTTP handlers to orchestrator — non-blocking send, HTTP 503 if full
- Manual triggers produce standard EventChaosExecuted flowing through the event pipeline — replayable in NDJSON log
- Control actions (pause/resume/rate) logged as informational "control" record type, not as chaos events
- Horizontal control strip layout (moved from sidebar in UX-5 iteration) — compact, always visible
- Parameterized trigger form per action type — works for existing 10 actions; v2 action params deferred to spec-chaos-actions-v2

## Patterns

- Control commands processed in same `select` as event channel — no priority inversion, sequential processing on main loop
- Restart sends seed to restartCh, calls onStop — orchestrator exits cleanly, main loop re-enters with new seed

## Gotchas

- Controls were implemented as Phase 3 of spec-chaos-web-dashboard, not as a separate implementation pass — the controls spec was written before dashboard implementation absorbed it
- Stop handler stops chaos scheduler only (peers stay connected); restart cancels the entire run context

## Files

- `cmd/ze-chaos/web/control.go` — 5 control handlers (pause, resume, rate, trigger, stop/restart)
- `cmd/ze-chaos/orchestrator.go` — controlCh field, select in event loop
- `cmd/ze-chaos/chaos/scheduler.go` — SetRate method added
- Sub-summary: 308 (control strip layout)
