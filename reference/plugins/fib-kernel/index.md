# `fib-kernel` plugin

FIB kernel: programs OS routes from system RIB via netlink/route socket

## At a glance

| Field | Value |
|-------|-------|
| Registry area | FIB |
| Kind | Runtime plugin |
| Source path | `internal/plugins/fib/kernel` |
| YANG modules | 1 |

## Configuration

`fib/kernel`

## Dependencies

- Required: [`rib`](../rib/index.md), [`sysctl`](../sysctl/index.md)
- Optional: None

## Used by

- Required dependency for: [`isis`](../isis/index.md), [`ldp`](../ldp/index.md), [`ospf`](../ospf/index.md), [`rsvp-te`](../rsvp-te/index.md)
- Optional dependency for: None

## Repository artifacts

Package: `internal/plugins/fib/kernel`

YANG files: `internal/plugins/fib/kernel/yang/ze-fib-conf.yang`
Metadata source: `Registration`
