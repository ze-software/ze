# `bfd` plugin

Bidirectional Forwarding Detection (RFC 5880, 5881, 5883)

## At a glance

| Field | Value |
|-------|-------|
| Registry area | BFD |
| Kind | Runtime plugin |
| Source path | `internal/component/bfd` |
| YANG modules | 3 |

## Configuration

`bfd`

## Dependencies

- Required: None
- Optional: None

## Used by

- Required dependency for: None
- Optional dependency for: None

## Startup ordering

Order only when both plugins are selected; never auto-loads a plugin.

- Start after: [`interface`](../interface/index.md)
- Starts before: None

## Repository artifacts

Package: `internal/component/bfd`

YANG files: `internal/component/bfd/yang/ze-bfd-api.yang`, `internal/component/bfd/yang/ze-bfd-cmd.yang`, `internal/component/bfd/yang/ze-bfd-conf.yang`
Metadata source: `Registration`
