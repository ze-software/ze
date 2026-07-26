# `interface` plugin

OS network interface monitoring and management

## At a glance

| Field | Value |
|-------|-------|
| Registry area | Interface |
| Kind | Runtime plugin |
| Source path | `internal/component/iface` |
| YANG modules | 6 |

## Configuration

`interface`

## Dependencies

- Required: [`sysctl`](../sysctl/index.md)
- Optional: None

## Used by

- Required dependency for: [`iface-dhcp`](../iface-dhcp/index.md), [`ipsec-interface`](../ipsec-interface/index.md), [`ospf`](../ospf/index.md), [`traffic-usage`](../traffic-usage/index.md), [`vrrp`](../vrrp/index.md)
- Optional dependency for: [`static`](../static/index.md)

## Repository artifacts

Package: `internal/component/iface`

YANG files: `internal/component/iface/yang/ze-iface-api.yang`, `internal/component/iface/yang/ze-iface-cmd.yang`, `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/iface/yang/ze-iface-interface-cmd.yang`, `internal/component/iface/yang/ze-iface-monitor-cmd.yang`, `internal/component/iface/yang/ze-iface-show-cmd.yang`
Metadata source: `Registration`
