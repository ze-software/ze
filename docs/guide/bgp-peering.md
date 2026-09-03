# BGP Peering

Use this guide after the quick start when a peer needs groups, multiple address families, capabilities, prefix limits, or an explicit relationship role.

## Minimal peer

A peer needs local and remote addresses, AS numbers, a router ID, and at least one address family:

```text
bgp {
    router-id 192.0.2.10

    peer transit-a {
        connection {
            local { ip 192.0.2.10 }
            remote { ip 192.0.2.1 }
        }
        session {
            asn { local 64500; remote 64496 }
            family {
                ipv4/unicast {
                    prefix { maximum 1000000 }
                }
            }
        }
    }
}
```

Validate before starting or reloading:

```console
ze config validate ze.conf
ze doctor
```

Use the generated [configuration reference](https://ze-software.net/reference/configuration/) for the current leaf names and constraints.

## Groups and inheritance

Put shared settings in a group and override only what differs on each peer:

```text
bgp {
    router-id 192.0.2.10

    group transit {
        session {
            asn { local 64500 }
            family {
                ipv4/unicast { prefix { maximum 1000000 } }
                ipv6/unicast { prefix { maximum 250000 } }
            }
        }
        timer { receive-hold-time 90 }

        peer transit-a {
            connection {
                local { ip 192.0.2.10 }
                remote { ip 192.0.2.1 }
            }
            session { asn { remote 64496 } }
        }

        peer transit-b {
            connection {
                local { ip 2001:db8::10 }
                remote { ip 2001:db8::1 }
            }
            session { asn { remote 64497 } }
        }
    }
}
```

Peer values override group values. Group values override the global BGP block. Inspect the resolved result with `show config dump` rather than reasoning about inheritance from the source file alone.

## Dynamic peer groups

A group with `connection { remote { ip dynamic } }` accepts a session from any address in its ranges. Use it for an IXP route server or any fabric where the members are not known when you write the configuration.

```text
bgp {
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 174 701 3356 ]
        }
    }
    group ix {
        connection {
            remote {
                ip dynamic
                connect false
                range 198.51.100.0/24
                range 2001:db8:1::/64
                max-peers 500
            }
            local {
                ip 198.51.100.1
                accept true
            }
        }
        session {
            asn { local 64500 }
            router-id 198.51.100.1
            family {
                ipv4/unicast { prefix { maximum 200000 } }
            }
        }
        role { import rs }
        filter {
            import [ NO-TRANSIT ]
            export [ NO-TRANSIT ]
        }
        attach process rs {
            receive [ update-received state open-received refresh ]
            send [ update ]
        }
    }
}
```

The `role { import rs }` leaf is what makes the two `filter` chains mandatory. A
declared RFC 9234 role obliges the bound chains to name a filter that can refuse a
path through a transit provider, and `rs` binds both. Write `inactive: import
NO-TRANSIT` in place of the active ref to record that this session runs without the
check. See [bgp-role.md](bgp-role.md).

`ip dynamic` is valid at group level only. A peer that states it is refused. Two more statements are required, and the configuration is refused without them: at least one `range`, and an explicit `connect false`. Ze only accepts on a dynamic group and never dials one, so an absent `connect` leaf, which YANG defaults to true, is refused rather than corrected.

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `connection / remote / ip` | `dynamic` | -- | Marks the group as a dynamic peer group. |
| `connection / remote / range` | leaf-list of prefixes | -- | The addresses that can open a session. Overlapping ranges take the longest match. |
| `connection / remote / max-peers` | uint32 | 1000 | Maximum number of members of this group. |

A member is built from the group's own resolved settings, so it inherits every leaf the group states. That covers:

- address families and their prefix limits
- ADD-PATH require and refuse sets
- import and export filter chains
- static routes
- MD5, BFD and capture
- `attach process` blocks
- community send, AS override, route reflector client and cluster ID
- the local-AS options

Seven values diverge, and only these seven.

| Field | Value on a member |
|-------|-------------------|
| Name | `dyn-<address>` |
| Group name | The group's name |
| Address | The address the connection arrived from |
| Port | The canonical BGP port |
| Remote AS | Learned from the peer's OPEN (RFC 4271 Section 4.2) |
| Connection mode | Passive |
| Dynamic flag | Set |

A dynamic group is held to the same rules as a static peer. A family with no prefix maximum is refused (RFC 4486). A hold time RFC 4271 Section 4.2 refuses is refused here too.

The group opens its own listening socket on `connection/local/ip` and the group's listen port. A configuration that names no static peer is enough to accept members. When a static peer and a dynamic group share one socket, Ze attributes each connection by remote address, so a static peer inside the group's range keeps its own session and its own settings.

A per-peer plugin config that a group states reaches its members. RFC 9234 role checks, the RPKI action, the blackhole block, and the community policy all resolve through the group when the member has no entry of its own. See the [BGP Role guide](bgp-role.md).

A configuration reload keeps a member's session up when its settings did not change. Ze removes a member in three cases only: its group is gone, its address left every range, or the group template would now build different settings for that address. A wider range or a raised `max-peers` changes no member and bounces no session.

## Address families

The engine provides IPv4 and IPv6 unicast and multicast. Plugins register VPN, EVPN, FlowSpec, labeled-unicast, BGP-LS, MVPN, RTC, MUP, VPLS, and SR Policy families. The current encode, decode, and route-configuration status is maintained on the [BGP protocol page](../features/bgp-protocol.md).

Each enabled family requires an explicit prefix limit. Set it from expected operational volume rather than accepting a migration default. A full-table peer and a customer announcing one aggregate should not share the same limit.

## Prefix limits are per family

Every leaf in the `prefix` container belongs to the family that holds it. The family that exceeded its maximum decides what happens next. Another family's value never applies.

```text
session {
    family {
        ipv4/unicast {
            prefix {
                maximum      1000000
                warning       900000
                teardown         true
                idle-timeout      300
                reconnect       timer
            }
        }
        ipv6/unicast {
            prefix {
                maximum       250000
                teardown        false
            }
        }
    }
}
```

| Leaf | Type | Default | Description |
|------|------|---------|-------------|
| `maximum` | uint32 | required | Hard cap. Exceeding it stops the session with NOTIFICATION Cease / Maximum Number of Prefixes Reached. |
| `warning` | uint32 | 90% of `maximum` | Warning threshold. Logged and exported as a metric. |
| `teardown` | boolean | `true` | Stop the session on exceed. `false` warns and drops further NLRI of that family. |
| `count` | enum | `offered` | `offered` counts each announcement on the wire. `installed` counts the prefix set this family holds. |
| `idle-timeout` | uint16 | `0` | Seconds to wait before reconnect. The wait doubles on each repeat teardown, up to one hour. |
| `reconnect` | enum | see below | `never`, `backoff`, or `timer`. |

`reconnect` with no value means `timer` when `idle-timeout` is above zero, and `never` when it is zero. A family that configured nothing therefore stays down after a teardown, and the peer state reads `idle-hold` until an operator restarts it. `ze show warnings` carries the prefix-hold warning for the whole hold.

The teardown NOTIFICATION and the log line name the family that exceeded its maximum.

## Attached programs

`attach process <name>` states which events a peer feeds a program, and which message types that program can originate toward the peer. Ze reads both halves at delivery.

```text
peer transit-a {
    attach process rib {
        receive [ update-received state refresh ]
        send [ update ]
    }
}
```

| Half | Meaning |
|------|---------|
| `receive` | The event types this peer feeds the program. |
| `send` | The message types the program can send toward this peer. |

A peer that attaches no program feeds it no peer-scoped event, and that program cannot announce to the peer. An event that carries no peer address is not peer-scoped, so an attach block describes none of them, and every subscriber still receives them.

The BGP namespace registers `update`, `open`, `notification`, `keepalive`, `refresh`, `state`, `negotiated`, `eor`, `congested`, `resumed`, `rpki`, `listener-ready`, and `update-notification`. Plugins register more, such as `update-rpki`.

An event type carries the direction it is fed in. `update` means both directions. `update-received` and `update-sent` mean one. A type a plugin registers keeps its whole name, so a plugin type that ends in `-sent` is never read as a direction. `*` feeds every type in both directions. `all` is not accepted, and a bare `sent` or `received` is refused with a message that names the token to write instead.

Both directions of `update` deadlock `bgp-rs` and `bgp-rr`, because forwarding an UPDATE raises the sent event the forward is waiting on. Give those programs `update-received`.

Ze logs a warning when the two halves disagree: a grant the plugin never declared, a declaration no peer grants, a direction mismatch, and a plugin no peer attaches at all.

An operator command is not gated by `send`. The `send` list grants authority to a program, and AAA already checks the operator.

## ADD-PATH

ADD-PATH is configured per family. The peers must negotiate compatible send and receive modes before additional paths can be exchanged. See the [ADD-PATH guide](add-path.md) for the mode matrix, path limits, and verification commands.

## BGP Role

RFC 9234 roles describe the business relationship between peers and let Ze enforce valid pairings and OTC handling. Configure roles deliberately on eBGP sessions. See the [BGP Role guide](bgp-role.md) for valid peer pairs and policy effects.

## Session lifecycle

### Hold timer expiry

When the hold timer expires, Ze runs the full RFC 4271 Section 8.2.2 Event 10 action list. It sends NOTIFICATION Hold Timer Expired (error 4), releases the session resources, moves the FSM to Idle, and increases the ConnectRetryCounter.

Ze granted one reprieve on a hold expiry until 2026-08-03. That grace is removed. Event 10 does not permit it, and worst-case dead-peer detection falls from two hold times to one. A daemon whose CPU is saturated now drops sessions that the grace used to keep.

ConnectRetryCounter is RFC 4271 Section 8.1.1 mandatory session attribute 2. Read it as `connect-retry-counter` in `show bgp peer <name> detail`, or as the `ze_bgp_connect_retry_counter` gauge. It is a gauge and not a counter, because the RFC resets it to zero on an operator start and on an operator stop.

### A second OPEN is refused

An OPEN that arrives while the connection is already in Established or OpenConfirm is refused with NOTIFICATION Cease (error 6). Ze does not process it, so a peer cannot rewrite the negotiated capability set of a live session. The `ze_bgp_open_in_established_total` counter names the peer that tried.

### The session always ends when the read loop ends

Every exit from the read loop ends the session and closes the connection. An import policy teardown, an OPEN that fails to unpack, an OPEN the validator rejects, and a local capability parse error each close the socket and mark the peer down. A peer marked up with a dead socket cannot happen through these paths.

### BGP Identifier validation (RFC 6286)

Ze answers an OPEN with Bad BGP Identifier when the identifier is zero, and when the identifier equals its own and the peer is internal. The same identifier from an external peer is accepted, which is what Section 2.2 requires.

Both checks now run for a dynamic peer. A dynamic peer has no configured remote AS when the OPEN arrives, so Ze reads the AS the peer advertises in its 4-octet AS capability (RFC 6793). A peer that advertises no such capability is judged on its My AS field.

The AS-wide uniqueness claim of Section 2.1 reads the same value. A 4-byte-AS speaker sends AS_TRANS (23456) in My AS, so every such peer used to claim in one shared bucket, and two peers in genuinely different 4-byte ASes that share an identifier collided. Section 2.1 scopes uniqueness per AS and licenses no such rejection.

## Verify the session

```console
ze cli -c "show bgp"
ze cli -c "show bgp peer transit-a detail"
ze cli -c "show bgp peer transit-a capabilities"
ze cli -c "show bgp rib status"
```

Check that:

1. The session is Established.
2. The negotiated families match the intended configuration.
3. Prefix counts remain below the configured limits.
4. Capabilities such as ADD-PATH, Graceful Restart, and BGP Role were negotiated rather than merely configured locally.
5. `show warnings` and `show errors` are clean.

## Related guides

- [Configuration](configuration.md) covers syntax and the full configuration model.
- [BGP policy](bgp-policy.md) covers import, export, and redistribution.
- [BGP resilience](bgp-resilience.md) covers route refresh, restart, persistence, and reflection.
- [Route injection](route-injection.md) covers on-demand announcements.
