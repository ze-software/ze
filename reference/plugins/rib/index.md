# `rib` plugin

System RIB: resolves authoritative routes and publishes forwarding changes

## At a glance

| Field | Value |
|-------|-------|
| Registry area | RIB |
| Kind | Runtime plugin |
| Source path | `internal/component/sysrib` |
| YANG modules | 2 |

## Configuration

`rib`

## Dependencies

- Required: [`policy`](../policy/index.md)
- Optional: None

## Used by

- Required dependency for: [`fib-kernel`](../fib-kernel/index.md), [`fib-p4`](../fib-p4/index.md), [`fib-vpp`](../fib-vpp/index.md), [`static`](../static/index.md)
- Optional dependency for: None

## Startup ordering

Order only when both plugins are selected; never auto-loads a plugin.

- Start after: [`bgp-rib`](../bgp-rib/index.md)
- Starts before: None

## Repository artifacts

Package: `internal/component/sysrib`

YANG files: `internal/component/sysrib/yang/ze-rib-conf.yang`, `internal/component/sysrib/yang/ze-sysrib-api.yang`
Metadata source: `Registration`
