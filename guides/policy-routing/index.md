# Policy Routing

Ze supports policy-based routing (PBR) to steer traffic through alternate
routing tables or next-hops based on L3/L4 match criteria. This is used
for content filtering (Surfprotect), split tunneling, and traffic
engineering scenarios where destination-based routing is insufficient.

Under the hood, ze uses nftables packet marking with kernel ip rules.
Each matching packet gets an fwmark; a kernel ip rule maps that mark to
the target routing table.

## Configuration

Set references (`@DstBypass`, `@SrcBypass`) must be defined in the
`firewall` config section. See [Firewall Guide](../firewall/index.md) for set
syntax.

```
policy {
    route surfprotect {
        interface "l2tp*";

        rule bypass-dst {
            from {
                destination-address @DstBypass;
                destination-port 80,443;
                protocol tcp;
            }
            then {
                accept;
            }
        }
        rule bypass-src {
            from {
                destination-port 80,443;
                protocol tcp;
                source-address @SrcBypass;
            }
            then {
                accept;
            }
        }
        rule block-quic {
            order 20;
            from {
                destination-address 0.0.0.0/0;
                destination-port 80,443;
                protocol udp;
            }
            then {
                drop;
            }
        }
        rule surfprotect-syn {
            order 30;
            from {
                destination-address 0.0.0.0/0;
                destination-port 80,443;
                protocol tcp;
                tcp-flags syn;
            }
            then {
                table 100;
                tcp-mss 1436;
            }
        }
        rule surfprotect-tcp {
            order 40;
            from {
                destination-address 0.0.0.0/0;
                destination-port 80,443;
                protocol tcp;
            }
            then {
                table 100;
            }
        }
    }
}
```

Table 100 must be populated separately via static routes. Static routes
reference tables by name, so define a named routing table mapping to kernel
table ID 100 and add the route under it:

```
routing-table {
    table pbr {
        id 100
    }
}

static {
    table pbr {
        route 0.0.0.0/0 {
            next { interface tun100 { } }
        }
    }
}
```

## Interface binding

Each policy route binds to one or more ingress interfaces. A trailing
`*` enables wildcard matching (e.g., `l2tp*` matches all L2TP
interfaces). The interface match is prepended to every rule in the
policy.

```
policy {
    route example {
        interface "l2tp*";
        interface eth1;
        ...
    }
}
```

## Match criteria (from block)

| Keyword | Description | Example |
|---------|-------------|---------|
| `source-address` | Source IP prefix or `@set` reference | `10.0.0.0/8`, `@AllowedSrc` |
| `destination-address` | Destination IP prefix or `@set` reference | `0.0.0.0/0`, `@DstBypass` |
| `source-port` | Source port or port range | `1024-65535` |
| `destination-port` | Destination port, range, or list | `80,443` |
| `protocol` | L4 protocol name | `tcp`, `udp`, `icmp`, `icmpv6`, `gre`, `esp`, `ah`, `ospf`, `vrrp`, `sctp` |
| `tcp-flags` | Comma-separated TCP flags | `syn`, `syn,ack` |

`protocol` takes one of the ten names the firewall backends can program, listed
with their IANA numbers in the firewall guide. A different spelling is refused
at commit, naming the leaf and the accepted values. Before this check existed
the commit succeeded and the rule failed later, inside the firewall reconcile,
where the failure was not local to the rule that caused it.

Set references (`@name`) resolve against firewall sets defined in the
firewall config section.

TCP flag names: `fin`, `syn`, `rst`, `psh`, `ack`, `urg`.

## Actions (then block)

| Keyword | Description |
|---------|-------------|
| `accept` | Skip this policy, packet routes normally |
| `drop` | Drop the packet |
| `table N` | Route via kernel table N (user must populate the table) |
| `next-hop IP` | Route via the specified next-hop (table auto-managed) |
| `tcp-mss N` | Clamp TCP MSS to N bytes (combinable with `table` or `next-hop`) |

Only one terminal action (`accept`, `drop`, `table`, or `next-hop`) is
allowed per rule. `tcp-mss` is a modifier and can be combined with a
terminal action.

### table action

`then { table 100; }` internally allocates an fwmark from the reserved
range 0x50000-0x5FFFF, adds a SetMark action to the nftables rule, and
creates a kernel ip rule mapping that fwmark to table 100. The user
must populate table 100 (e.g., via static routes).

### next-hop action

`then { next-hop 10.0.0.1; }` works like `table` but also auto-manages
the kernel routing table. Ze allocates a table from range 2000-2999,
adds a default route via the specified next-hop, and creates the fwmark
and ip rule. Multiple rules targeting the same next-hop share a single
auto-allocated table. The table and routes are cleaned up on config
reload or shutdown.

## Rule ordering

Rules are evaluated in `order` value order (lower first). Rules with
equal order are sorted alphabetically by name. If `order` is omitted,
it defaults to 0.

```
rule first  { order 10; ... }
rule second { order 20; ... }
rule third  { order 30; ... }
```

## Reserved ranges

Ze reserves these ranges to prevent collisions:

| Range | Owner | Purpose |
|-------|-------|---------|
| Tables 1-999 | user | Explicit table IDs in config |
| Tables 253-255 | kernel | default (253), main (254), local (255) |
| Tables 1000-1999 | ze VRF | Auto-allocated VRF tables |
| Tables 2000-2999 | ze policy-routing | Auto-allocated next-hop tables |
| Tables 3000 and above | user | Explicit table IDs in config |
| fwmarks 0x50000-0x5FFFF | ze policy-routing | Packet marks for table steering |

<!-- source: internal/plugins/policyroute/yang/ze-policyroute-conf.yang — leaf table range "1..999|3000..max" -->
<!-- source: internal/plugins/policyroute/marks.go — autoTableBase/autoTableMax 2000-2999 -->

User-specified table IDs in the 1000-2999 range are rejected at config
validation time, as are 0 and the kernel tables 253-255.

The upper bound is the kernel's own: table IDs run to 4294967295. A 32-bit
build cannot program an ID above 2147483647 (the netlink bindings carry a
table ID in a machine int), and rejects it at config validation rather than
installing a rule that would silently select no table. Ze's released targets
are 64-bit, where the whole range is available.

<!-- source: internal/plugins/policyroute/config.go — validateActionTable, maxEncodableTable -->
<!-- source: internal/core/routingtable/registry.go — validateTableID, maxEncodableTableID -->

## CLI

```
ze> show policy routes
```

Returns JSON with all configured policy routes, their interface
bindings, rules, and actions.

## nftables internals

All policy routes are merged into a single nftables table `ze_pr` with
family `inet`, chain type `route`, hook `prerouting`, and priority -150.
Term names follow the pattern `policyname-rulename`. The table is
managed through the firewall backend, sharing the same Apply
reconciliation used by the firewall component.

## Dependencies

The policy-routes plugin depends on the firewall plugin. Both must be
available for policy routing to function. The firewall backend (nft) is
loaded by the firewall plugin; policy routing registers its tables with
the shared firewall table registry.

Auto-managed routes use protocol `ze-policy-route` (RTPROT 252).
