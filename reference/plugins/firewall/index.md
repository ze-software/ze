# `firewall` plugin

Packet filter and NAT rules (nftables on Linux)

## At a glance

| Field | Value |
|-------|-------|
| Registry area | Firewall |
| Kind | Runtime plugin |
| Source path | `internal/component/firewall` |
| YANG modules | 2 |

## Configuration

`firewall`

## Dependencies

- Required: None
- Optional: None

## Used by

- Required dependency for: [`copp-input-chain`](../copp-input-chain/index.md), [`ddos-fake`](../ddos-fake/index.md), [`ddos-local`](../ddos-local/index.md), [`firewall-irr`](../firewall-irr/index.md), [`flowspec-firewall`](../flowspec-firewall/index.md), [`policy-routes`](../policy-routes/index.md)
- Optional dependency for: None

## Repository artifacts

Package: `internal/component/firewall`

YANG files: `internal/component/firewall/yang/ze-firewall-cmd.yang`, `internal/component/firewall/yang/ze-firewall-conf.yang`
Metadata source: `Registration`
