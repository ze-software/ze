# BGP Resilience

BGP resilience is a set of separate mechanisms. Route Refresh reapplies policy without dropping the session. Graceful Restart retains forwarding during a restart. RIB persistence retains outbound state. Route reflection distributes iBGP routes without a full mesh. Use only the mechanisms the topology requires.

## Route Refresh

When both peers negotiate Route Refresh, request a new advertisement without tearing down TCP:

```console
ze cli -c "request peer transit-a refresh ipv4/unicast"
ze cli -c "request peer transit-a clear soft"
```

Enhanced Route Refresh adds Begin-of-RR and End-of-RR markers so stale routes can be removed after the replacement stream completes. Verify the negotiated capability with `show bgp peer transit-a capabilities` before relying on it.

## Graceful Restart

Graceful Restart requires the `bgp-gr` and `bgp-rib` plugins. The restarting speaker advertises the capability, while the receiving speaker retains stale routes until End-of-RIB or timer expiry.

```text
plugin {
    internal rib {
        use bgp-rib
    }
    internal gr {
        use bgp-gr
    }
}

bgp {
    peer upstream1 {
        session {
            capability {
                graceful-restart {
                    restart-time 120
                    long-lived-stale-time 3600
                }
            }
        }
        attach process gr {
            receive [ open-received state eor ]
            send [ update ]
        }
        attach process rib {
            receive [ update state refresh ]
            send [ update ]
        }
    }
}
```

Long-Lived Graceful Restart extends the stale period and deprioritises stale routes. It is active only when both peers negotiate it. Routes carrying `NO_LLGR` are removed rather than retained.

Load `bgp-gr` with `internal <name> { use bgp-gr }`, as the example above does. `run "ze plugin bgp-gr"` starts the plugin engine in a child process, and RFC 9494's egress filter in the daemon then reads no peer capability state. The filter fails closed for every peer. It withdraws stale routes from external peers, and it attaches `NO_EXPORT` with `LOCAL_PREF` 0 to the routes it sends to internal peers. `ze doctor` reports the arrangement as `doctor-bgp-gr-out-of-process`.

See [Graceful Restart](../graceful-restart/index.md) for timers, stale levels, communities, and failure handling.

## Restarting Ze

Use the restart path when peers should see a planned Graceful Restart:

```console
ze signal restart
```

A normal stop does not claim that forwarding state will survive:

```console
ze signal stop
```

After restart, confirm that peers negotiated GR, stale routes were retained only for the intended families, fresh routes replaced them, and no stale entries remain after End-of-RIB.

## RIB persistence

The `bgp-persist` plugin tracks outbound routes and can replay them after reconnect. It complements Graceful Restart but does not replace policy or best-path calculation. Persisted state must still be reconciled with the live configuration and destination peer.

Use `show bgp rib status`, the peer RIB commands, and warning reports to confirm that replay completed. A reconnect should not trigger repeated full-table floods to unrelated peers.

## Route reflection

Route reflection replaces an iBGP full mesh with client and non-client relationships. Ze applies ORIGINATOR_ID and CLUSTER_LIST loop protection and retains ordinary import and export policy per destination.

Design points:

- Give each reflector a stable router ID and cluster identity.
- Mark clients explicitly.
- Use at least two reflectors for a production cluster.
- Keep client export policy separate from eBGP policy.
- Verify that a route learned from one client reaches other clients and eligible non-clients, but does not return to its originator.

For a task-oriented example, see the [FlowSpec route-reflector guide](../flowspec-route-reflector/index.md). The same reflection topology applies to other negotiated families, subject to their plugin support.

## ADD-PATH and multipath

ADD-PATH advertises more than one path for a prefix. Multipath installs equal-cost next hops in the local RIB and FIB. These are independent:

- ADD-PATH changes what paths cross a BGP session.
- Multipath changes what equal candidates are installed locally.

Verify send and receive modes, path identifiers, best-path state, and installed ECMP next hops separately. See [ADD-PATH](../add-path/index.md) and the [BGP protocol page](../../features/bgp-protocol/index.md) for current support.

## Failure checklist

During a maintenance test:

1. Record peer state and RIB counts.
2. Trigger the intended refresh, session restart, or process restart.
3. Watch peer state and recent events.
4. Confirm forwarding remains valid where GR or LLGR promises it.
5. Confirm stale routes disappear after refresh completion or timer expiry.
6. Confirm every peer receives one coherent replacement stream.
7. Check `show warnings`, `show errors`, and `show health`.

## Related guides

- [BGP peering](../bgp-peering/index.md)
- [BGP policy](../bgp-policy/index.md)
- [Graceful Restart](../graceful-restart/index.md)
- [ADD-PATH](../add-path/index.md)
- [Lifecycle and rollback](../lifecycle/index.md)
