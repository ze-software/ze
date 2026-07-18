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

### Clean-shutdown teardown

`flush-on-shutdown` controls whether stopping the `ze` process removes ze-owned
tables from the kernel. It defaults to `true`: an orderly stop (SIGTERM, e.g.
`systemctl stop ze`) tears the tables down so a stopped daemon leaves no rules
behind. It keys off how the process exits, and is unrelated to BGP graceful
restart -- that concerns only RIB/FIB route retention while a peer re-establishes
and never restarts the daemon, so it never reaches this path.

Set it to `false` to use `ze` as a **one-shot provisioner**: apply the firewall
(and interface) configuration, let the process exit, and leave the rules running
in the kernel -- the way `nft -f` programs a ruleset and returns. Nothing is torn
down when `ze` quits.

```
firewall {
    backend nft;
    flush-on-shutdown false;   # one-shot: program the rules, exit, leave them running
}
```

A crash (SIGKILL, panic, power loss) never runs the shutdown path, so ze-owned
tables **always** persist across a crash regardless of this setting. The option
governs every ze table producer that shares the backend -- the firewall
component, `control-plane-protection`, `policy-routes`, and `ddos-local` -- and
the firewall component performs the teardown as a single ordered actor so
plugin table removal never races the backend close.

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
| `source-address @set` | Named set lookup | `source-address @blocked;` |

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
| `masquerade` | Masquerade | `masquerade;` or `masquerade { port-range "1024-65535"; }` or `masquerade { random; }` |
| `redirect` | Redirect to port | `redirect { to 8080; }` |
| `notrack` | Disable conntrack | `notrack;` |
| `flow-offload` | Hardware offload | `flow-offload { flowtable ft0; }` |
| `mark-set` | Set packet mark | `mark-set { value 0x10; }` |
| `connection-mark-set` | Set connmark | `connection-mark-set { value 0x20/0xff; }` |
| `dscp-set` | Set DSCP | `dscp-set 46;` |
| `tcp-mss-set` | Clamp TCP MSS | `tcp-mss-set 1400;` |
| `counter` | Count packets/bytes | `counter;` |
| `log` | Log packet | `log { prefix "DROPPED"; }` |
| `limit-rate` | Rate limit | `limit-rate { rate 10/second; burst 5; }` |
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
            type ipv4;
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
                    source-address @blocked;
                }
                then {
                    drop;
                }
            }
        }
    }
}
```

## Global Options

The `global-options` container provides keyword toggles for common network
security defaults. Each keyword maps to a kernel sysctl. At
config apply time, the firewall component emits these as sysctl defaults via
EventBus. Explicit `sysctl { setting { ... } }` entries always override
global-options (three-layer priority: config > transient > default).

<!-- source: internal/component/firewall/config.go -- ExtractGlobalOptions, globalOptionDefs -->
<!-- source: internal/component/firewall/engine.go -- emitGlobalOptionsSysctlDefaults -->

```
firewall {
    backend nft;
    global-options {
        all-ping enable;
        syn-cookies enable;
        source-validation strict;
        log-martians enable;
    }
}
```

| Keyword | Sysctl | enable | disable |
|---------|--------|--------|---------|
| `all-ping` | `net.ipv4.icmp_echo_ignore_all` | 0 (allow) | 1 (ignore) |
| `broadcast-ping` | `net.ipv4.icmp_echo_ignore_broadcasts` | 0 (allow) | 1 (ignore) |
| `syn-cookies` | `net.ipv4.tcp_syncookies` | 1 | 0 |
| `receive-redirects` | `net.ipv4.conf.all.accept_redirects` | 1 | 0 |
| `send-redirects` | `net.ipv4.conf.all.send_redirects` | 1 | 0 |
| `source-validation` | `net.ipv4.conf.all.rp_filter` | disable=0, strict=1, loose=2 | - |
| `log-martians` | `net.ipv4.conf.all.log_martians` | 1 | 0 |
| `ipv6-receive-redirects` | `net.ipv6.conf.all.accept_redirects` | 1 | 0 |
| `ipv6-src-route` | `net.ipv6.conf.all.accept_source_route` | 1 | 0 |

Note: `all-ping` and `broadcast-ping` have inverted semantics because the
underlying sysctl controls "ignore" behavior.

## IRR Prefix-List Filtering

<!-- source: internal/component/firewall/plugins/irr/irr.go -- firewall-irr plugin entry point -->

Firewall rules can match traffic by ASN or AS-SET using IRR-resolved prefix
lists. The `firewall-irr` plugin resolves references via the IRR whois client,
caches results in zefs, and populates nftables interval sets.

### Operator Workflow

1. Fetch prefix data: `update firewall irr asn 13335`
2. Inspect cached data: `show firewall irr`
3. Commit config with `source-asn 13335` in a term's from-block
4. Refresh all cached entries: `update firewall irr all`

### Config Leaves

| Leaf | Type | Description |
|------|------|-------------|
| `source-asn` | uint32 (1-4294967294) | Match source address against IRR-resolved prefixes for this ASN |
| `source-as-set` | string | Match source address against IRR-resolved prefixes for this AS-SET |
| `destination-asn` | uint32 (1-4294967294) | Match destination address against IRR-resolved prefixes for this ASN |
| `destination-as-set` | string | Match destination address against IRR-resolved prefixes for this AS-SET |

### IRR Policy

```
firewall {
    irr {
        server whois.radb.net;
        peeringdb-url https://www.peeringdb.com;
        refresh-interval 0;  /* 0 = manual only; 60-86400 = auto-refresh seconds */
    }
}
```

Config commit rejects if a referenced ASN/AS-SET has no cached prefix data,
with an actionable error naming the missing entry and the command to run.

Auto-refresh (when `refresh-interval > 0`) is fail-closed: a failed IRR query
preserves the last-good cache and logs an error.

### Per-Interface Source Validation

<!-- source: internal/component/firewall/plugins/irr/sets.go -- buildIfaceTables -->

Bind an AS-SET to a customer-facing interface. Packets arriving on that
interface with source addresses not in the AS-SET's IRR-resolved prefixes
are dropped (ingress source validation, BCP 38).

```
firewall {
    irr {
        interface eth1 {
            source-as-set AS-CUSTOMER-A;
        }
        interface eth2 {
            source-as-set AS-CUSTOMER-B;
        }
    }
}
```

The plugin generates a `ze_irr_iface` table with a prerouting base chain.
For each bound interface, accept terms match `input-interface` + `source-address
in set` for both IPv4 and IPv6, followed by a drop term for that interface.
Unconfigured interfaces pass through unfiltered (chain policy accept).

Same fail-closed semantics apply: config commit rejects if any bound AS-SET
has no cached prefix data. Removing an interface binding removes its filter
on the next apply.

## CLI

| Command | Description |
|---------|-------------|
| `ze firewall show` | Display all firewall tables and rules |
| `ze firewall counters` | Show per-term packet and byte counters |
| `show firewall irr` | Show IRR filter status for all cached entries |
| `show firewall irr prefix <name>` | List cached prefixes for an ASN or AS-SET |
| `update firewall irr asn <N>` | Fetch/refresh IRR prefix-list for an ASN |
| `update firewall irr as-set <name>` | Fetch/refresh IRR prefix-list for an AS-SET |
| `update firewall irr all` | Refresh all cached IRR entries |

## Lifecycle

The firewall component registers with ze's engine via `registry.Register`.
On boot and config reload, the reactor parses the `firewall { }` section,
loads the selected backend, and calls `Apply([]Table)`. On failure,
`sdk.Journal` triggers a rollback to the previous state.

On a **clean** shutdown the component removes every ze-owned table (unless
`flush-on-shutdown false`; see [Clean-shutdown teardown](#clean-shutdown-teardown)),
then closes the backend. Because the firewall component owns the shared backend,
it is the single actor that performs this teardown -- plugins that register
tables (`control-plane-protection`, `policy-routes`, `ddos-local`) do not run
their own shutdown withdrawal, so table removal never races the backend close.
A crash bypasses this path entirely, leaving tables in place.
