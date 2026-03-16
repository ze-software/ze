# 158 — Hub Phase 1: Foundation

## Objective

Create `internal/component/hub/` as a thin entry point that composes existing `internal/component/plugin/` infrastructure (SubsystemHandler, SchemaRegistry, Hub) to orchestrate forked child processes.

## Decisions

- New `internal/component/hub/` package rather than extending `internal/component/plugin/` — hub is the orchestrator entry point; `internal/component/plugin/` provides the components. Mixing them would blur concerns.
- No new forking or pipe code — `plugin.SubsystemManager`, `plugin.Process`, and `plugin.SubsystemHandler` already handle all of that. `internal/component/hub/` only wires them together.
- Implementation Summary left blank in spec — this was a planning spec for work tracked in the master overview (157).

## Patterns

None beyond the explicit "thin entry point composing existing components" pattern.

## Gotchas

None documented — spec was a planning document; actual implementation tracked in 157-hub-separation-phases.

## Files

- `internal/component/hub/hub.go` — entry point composing plugin.SubsystemManager, plugin.SchemaRegistry, plugin.Hub
