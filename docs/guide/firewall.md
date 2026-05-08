# Firewall

Ze manages nftables packet filter and NAT rules from a single `firewall { }` YANG
section. The abstract data model describes matches (from) and actions (then);
the nft backend lowers them to nftables kernel expressions.

<!-- source: internal/component/firewall/model.go -- Match and Action type definitions -->
<!-- source: internal/component/firewall/config.go -- parseFromBlock, parseThenBlock -->
<!-- source: internal/component/firewall/engine.go -- runEngine, OnConfigure, OnConfigApply -->
<!-- source: internal/plugins/firewall/nft/lower_linux.go -- nftables expression lowering -->

## Backend

| Backend | Platform | Default | Mechanism |
|---------|----------|---------|-----------|
| `nft` | Linux | yes | google/nftables netlink library |

```
firewall {
    backend nft;
}
```

## Tables and Chains

Ze owns all tables whose kernel name starts with `ze_`. A table contains one or
more chains; a base chain has a type, hook, priority, and default policy.

```
firewall {
    backend nft;
    table wan {
        family inet;
        chain input {
            type filter;
            hook input;
            priority 0;
            policy drop;
            term allow-ssh {
                from {
                    destination-port 22;
                    protocol tcp;
                }
                then {
                    accept;
                }
            }
        }
    }
}
```

### Table Families

`inet` (dual-stack), `ip`, `ip6`, `arp`, `bridge`, `netdev`.

### Chain Types

`filter`, `nat`, `route`.

### Hooks

`input`, `output`, `forward`, `prerouting`, `postrouting`, `ingress`, `egress`.

## Match Types (from block)

| Config key | Match | Example |
|------------|-------|---------|
| `source-address` | IP prefix | `source-address 10.0.0.0/8;` |
| `destination-address` | IP prefix | `destination-address 192.168.1.0/24;` |
| `source-port` | Port or range | `source-port 1024-65535;` |
| `destination-port` | Port or range | `destination-port 22;` or `destination-port 80,443;` |
| `protocol` | L4 protocol | `protocol tcp;` |
| `input-interface` | Interface name | `input-interface eth0;` |
| `output-interface` | Interface name | `output-interface "l2tp*";` |
| `icmp-type` | ICMP type (name or number) | `icmp-type echo-request;` |
| `icmpv6-type` | ICMPv6 type (name or number) | `icmpv6-type nd-neighbor-solicit;` |
| `connection-state` | Conntrack states | `connection-state established,related;` |
| `connection-mark` | Mark value/mask | `connection-mark 0x10/0xff;` |
| `mark` | Packet mark value/mask | `mark 0x10/0xff;` |
| `dscp` | DSCP value (name or number) | `dscp ef;` |
| `tcp-flags` | TCP header flags | `tcp-flags syn;` |
| `match-set` | Named set lookup | `match-set blocked source-address;` |

### ICMP Type Names

Symbolic names for `icmp-type`: `echo-reply`, `destination-unreachable`,
`source-quench`, `redirect`, `echo-request`, `router-advertisement`,
`router-solicitation`, `time-exceeded`, `parameter-problem`,
`timestamp-request`, `timestamp-reply`, `info-request`, `info-reply`,
`address-mask-request`, `address-mask-reply`. Numeric values (0-255)
are also accepted.

Symbolic names for `icmpv6-type`: `destination-unreachable`, `packet-too-big`,
`time-exceeded`, `parameter-problem`, `echo-request`, `echo-reply`,
`mld-listener-query`, `mld-listener-report`, `mld-listener-done`,
`nd-router-solicit`, `nd-router-advert`, `nd-neighbor-solicit`,
`nd-neighbor-advert`, `nd-redirect`, `mld2-listener-report`.
Numeric values (0-255) are also accepted.

### Interface Wildcard

A trailing `*` on an interface name produces a prefix match. For example,
`input-interface "l2tp*"` matches any interface whose name starts with `l2tp`
(l2tp0, l2tp1, l2tp-peer42, etc.). Without the `*`, the match is exact.

## Action Types (then block)

| Config key | Action | Example |
|------------|--------|---------|
| `accept` | Accept packet | `accept;` |
| `drop` | Drop packet | `drop;` |
| `reject` | Reject with ICMP | `reject { with icmp; code 3; }` |
| `jump` | Jump to chain | `jump helper;` |
| `goto` | Goto chain | `goto cleanup;` |
| `return` | Return from chain | `return;` |
| `snat` | Source NAT | `snat { to "10.0.0.1"; }` |
| `dnat` | Destination NAT | `dnat { to "10.1.1.1:8080"; }` |
| `masquerade` | Masquerade | `masquerade;` |
| `redirect` | Redirect to port | `redirect { port 8080; }` |
| `notrack` | Disable conntrack | `notrack;` |
| `flow-offload` | Hardware offload | `flow-offload ft0;` |
| `mark` | Set packet mark | `mark { value 0x10; }` |
| `connection-mark` | Set connmark | `connection-mark { value 0x20; mask 0xff; }` |
| `dscp` | Set DSCP | `dscp { value 46; }` |
| `tcp-mss` | Clamp TCP MSS | `tcp-mss { size 1400; }` |
| `counter` | Count packets/bytes | `counter my-counter;` |
| `log` | Log packet | `log { prefix "DROPPED"; }` |
| `limit` | Rate limit | `limit { rate 10/second; burst 5; }` |
| `exclude` | Skip NAT (Return) | `exclude;` |

### NAT Exclude

In a NAT chain, `exclude` emits a Return verdict so matched traffic skips
the NAT translation. This replaces the VyOS `nat destination rule N exclude`
pattern.

```
firewall {
    backend nft;
    table nat-rules {
        family ip;
        chain prerouting {
            type nat;
            hook prerouting;
            priority -100;
            policy accept;
            term skip-local {
                from {
                    destination-address 10.0.0.0/8;
                }
                then {
                    exclude;
                }
            }
            term dnat-web {
                from {
                    destination-port 80;
                }
                then {
                    dnat { to "10.1.1.1"; }
                }
            }
        }
    }
}
```

### SNAT Address Ranges

SNAT and DNAT accept address ranges for pool-based NAT:

```
then {
    snat { to "10.0.0.1-10.0.0.10"; }
}
```

## Named Sets

```
firewall {
    backend nft;
    table wan {
        family inet;
        set blocked {
            type ipv4_addr;
            element 10.0.0.1;
            element 10.0.0.2 { timeout 3600; }
        }
        chain input {
            type filter;
            hook input;
            priority 0;
            policy drop;
            term block-list {
                from {
                    match-set blocked source-address;
                }
                then {
                    drop;
                }
            }
        }
    }
}
```

## CLI

| Command | Description |
|---------|-------------|
| `ze firewall show` | Display all firewall tables and rules |
| `ze firewall counters` | Show per-term packet and byte counters |

## Lifecycle

The firewall component registers with ze's engine via `registry.Register`.
On boot and config reload, the reactor parses the `firewall { }` section,
loads the selected backend, and calls `Apply([]Table)`. On failure,
`sdk.Journal` triggers a rollback to the previous state.
