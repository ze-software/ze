# `redistribute-orchestrator` plugin

Redistribute orchestrator: dispatches protocol route events to registered consumers

## At a glance

| Field | Value |
|-------|-------|
| Registry area | Redistribute |
| Kind | Runtime plugin |
| Source path | `internal/plugins/redistribute` |
| YANG modules | 0 |

## Configuration

`redistribute`

## Dependencies

- Required: [`policy`](../policy/index.md)
- Optional: None

## Used by

- Required dependency for: None
- Optional dependency for: None

## Startup ordering

Order only when both plugins are selected; never auto-loads a plugin.

- Start after: [`bgp`](../bgp/index.md)
- Starts before: [`as112`](../as112/index.md), [`connected`](../connected/index.md), [`static`](../static/index.md)

## Repository artifacts

Package: `internal/plugins/redistribute`

YANG files: None
Metadata source: `Registration`
