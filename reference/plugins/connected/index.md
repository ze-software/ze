# `connected` plugin

Connected routes: redistribute directly connected interface prefixes

## At a glance

| Field | Value |
|-------|-------|
| Registry area | Connected |
| Kind | Runtime plugin |
| Source path | `internal/plugins/connected` |
| YANG modules | 1 |

## Configuration

`connected`

## Dependencies

- Required: None
- Optional: None

## Used by

- Required dependency for: None
- Optional dependency for: None

## Startup ordering

Order only when both plugins are selected; never auto-loads a plugin.

- Start after: [`redistribute-orchestrator`](../redistribute-orchestrator/index.md)
- Starts before: None

## Repository artifacts

Package: `internal/plugins/connected`

YANG files: `internal/plugins/connected/yang/ze-connected-conf.yang`
Metadata source: `Registration`
