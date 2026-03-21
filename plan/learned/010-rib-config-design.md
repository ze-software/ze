# 010 — RIB Config Design

## Objective

Move RIB batching configuration (`group-updates`, `auto-commit-delay`, `max-batch-size`) from neighbor level into a structured `rib { out { } }` block, while maintaining backward compatibility with the deprecated neighbor-level `group-updates`.

## Decisions

Mechanical refactor following pre-existing design intent. The `rib {}` block structure was already planned in spec 008. No novel design decisions in this spec.

## Patterns

- Parsing applies defaults first, then template inheritance, then neighbor-level override — the layering is explicit in `parseNeighborConfig()`.
- Legacy `group-updates` at neighbor level maps transparently to `rib.out.group-updates`; the legacy field is kept in sync.

## Gotchas

- `rib.in` block was planned for future incoming RIB config (e.g., `max-routes`) but not implemented — the nested structure was chosen partly to enable this extension without migration.

## Files

- `internal/component/config/bgp.go` — RIBOutConfig, parseRIBOutConfig(), PeerConfig.RIBOut
- `internal/component/config/bgp_test.go` — per-neighbor rib config tests
