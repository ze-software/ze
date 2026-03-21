# 236 — API Command Restructure Overview

## Objective

Umbrella spec tracking an 8-step API command restructure (specs 140–148), reorganising commands under the `bgp` namespace and cleaning up legacy naming.

## Decisions

- Commands moved under `bgp` namespace.
- `session` → `plugin session`.
- `msg-id` → `bgp cache`.
- All 8 steps complete.

## Patterns

None beyond what the child specs cover.

## Gotchas

None.

## Files

- Umbrella only — see child specs 140–148 for implementation details.
