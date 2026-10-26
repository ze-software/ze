# Route Reflection

Ze can operate as a route server (RFC 7947) or route reflector, forwarding received routes to other peers. The `bgp-rs` plugin handles route forwarding with zero-copy wire optimization when peers share the same encoding context.
<!-- source: internal/component/bgp/plugins/rs/register.go -- bgp-rs registration, RFC 7947 -->

## Configuration

```
plugin {
    internal rs {
        use bgp-rs
    }
    internal adj-rib-in {
        use bgp-adj-rib-in
    }
}

bgp {
    peer client-a {
        remote { ip 10.0.0.1; as 65001; }
        local { ip 10.0.0.254; as 65000; }
        router-id 10.0.0.254;

        family { ipv4/unicast; }

        attach process rs {
            receive [ update-received state open-received refresh ]
            send [ update ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }

    peer client-b {
        remote { ip 10.0.0.2; as 65002; }
        local { ip 10.0.0.254; as 65000; }
        router-id 10.0.0.254;

        family { ipv4/unicast; }

        attach process rs {
            receive [ update-received state open-received refresh ]
            send [ update ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }
}
```

### A Route Server Without Per-Peer Entries

An IXP route server can accept its members from a dynamic peer group instead of a `peer` block for each one. The group opens its own listening socket, so a configuration that names no static peer still accepts members.

```
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
                max-peers 500
            }
            local {
                ip 198.51.100.1
                accept true
            }
        }
        session {
            asn {
                local 64500
            }
            router-id 198.51.100.1
            family {
                ipv4/unicast {
                    prefix {
                        maximum 200000
                    }
                }
            }
        }
        role {
            import rs
        }
        filter {
            import {
                filters [ NO-TRANSIT ];
                final accept;
            }
            export {
                filters [ NO-TRANSIT ];
                final accept;
            }
        }

        attach process rs {
            receive [ update-received state open-received refresh ]
            send [ update ]
        }
        attach process adj-rib-in {
            receive [ update-received state ]
        }
    }
}
```

The `role { import rs }` leaf makes both `filter` chains mandatory: a declared RFC 9234 role obliges each bound chain to name a filter that can refuse a path through a transit provider (RFC 7454 Section 9), and `rs` binds both. To run a session without the check, name the filter with the `inactive:` member prefix. The chain records the decision, and the filter never executes:

```
filter {
    import {
        filters [ inactive:NO-TRANSIT ];
        final accept;
    }
    export {
        filters [ inactive:NO-TRANSIT ];
        final accept;
    }
}
```

The prefix goes on the member, inside the brackets. Inside each direction block,
`inactive: filters NO-TRANSIT;` deactivates an existing member. Keep the
`final accept;` decision when the member is inactive. See
[config deactivation](config-deactivate.md) for both forms.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- filter-policy -->

A member inherits the group's whole settings, its `attach process` blocks and its per-peer plugin config included. A static peer whose address falls inside the range keeps its own session and its own settings. See [BGP peering](bgp-peering.md) for the group leaves and the reload rules.

## How It Works

### Forwarding and Destination Election

Receive validation and each destination's export policy apply before a route can
reach a client. At a destination without ADD-PATH, a competing source is deferred
to `bgp-rib` election when that recovery owner receives the source's UPDATEs.
The first eligible route and an uncontested owner's re-announcement can be sent
directly; ADD-PATH destinations retain distinct paths.

All forwarding paths share the destination session's final writer ownership
check. For each native route and outgoing ADD-PATH identifier, the writer
remembers the source peer and received path that last advertised it. A received
withdrawal removes only that owner's route; an unknown withdrawal or a withdrawal
from another source cannot erase a replacement or a locally originated route.
Competing announcements are therefore not an unconditional latest-wins contract.
<!-- source: internal/component/bgp/reactor/session_ownership.go -- oweAnnouncement, adjOutContests -->

With `bgp-rib` loaded, a destination without ADD-PATH holds the route the
`bgp-rib` election picks, on an announcement as on a withdrawal: the first route
of a prefix is written as it arrives, and when another source announces a prefix
the destination already holds, the announcement is decided by that election
rather than written as the newest arrival. Explicit-final policy validation is a
separate check and does not decide which route the election picks.
<!-- source: internal/component/bgp/reactor/session_ownership.go -- oweAnnouncement -->
<!-- source: internal/component/bgp/reactor/recovery_owed.go -- recoveryWorker -->


When the owner withdraws its route, or `bgp-rib` removes it, and `bgp-rib` is
loaded, a destination without ADD-PATH is not sent the bare withdrawal. The
`bgp-rib` election decides instead: the destination receives the best remaining
route from another source, or the withdrawal when none remains. So when B
announces a prefix, then A announces it, then A withdraws it, the client keeps
B's route. When
`bgp-rib` is not loaded, the withdrawal is sent as before and a warning names the
missing plugin.
<!-- source: internal/component/bgp/reactor/session_ownership.go -- owesWithdrawal -->
<!-- source: internal/component/bgp/reactor/recovery_owed.go -- recoveryWorker -->

A policy-generated withdrawal is sent only to a client that was advertised the
route: a client that never received it is sent nothing, because only a
previously advertised route can be withdrawn (RFC 4271 Section 3.1). A received
withdrawal follows the same rule, and is sent only where its source still owns
the client's route. A buffered batch uses its ordered pending ownership,
so A, B, A-withdraw preserves B before the batch flushes. A write or flush failure
invalidates that ownership and closes only the failed destination session.
<!-- source: internal/component/bgp/reactor/adj_rib_out.go -- adjRIBOut -->
<!-- source: internal/component/bgp/reactor/session_write.go -- writeRawUpdateBody, flushWrites -->

When receive validation is enabled, pending and rejected paths are withheld on both the cached forwarding path and the reactor fast path. An UPDATE containing paths with different verdicts forwards only its eligible NLRIs. A later validation change withdraws an advertised path that has become ineligible; recovery replays the retained route with its received attributes, without waiting for another UPDATE. ADD-PATH withdrawals keep the identifier used for the advertisement.
<!-- source: internal/component/bgp/plugins/rs/server_validation.go -- validationChanged, processValidation -->
<!-- source: internal/component/bgp/reactor/forward_validation.go -- forwardUpdateCore, forwardValidationWire -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRS validation fallback -->

The route reflector also reconciles retained unicast paths on validation
changes. It coalesces notifications before requesting replay, so a receive-store
callback never waits for an RPC into that same store. Replayed routes pass the
reactor's ordinary reflection rules and egress policy.
<!-- source: internal/component/bgp/plugins/rr/validation.go -- startValidation, replayValidation -->

For IPv4 and IPv6 unicast from a source that negotiated ADD-PATH, the route
server and reflector track each path by prefix and Path Identifier, so a
withdrawal of one path leaves the source's other paths of that prefix to be
withdrawn when the source leaves. That departure withdrawal names the native
NLRI with the source's identifier. The route server and the reflector both hand
it to the engine as a recovery, which withdraws each destination's own copy
under the identifier that destination was sent. Toward a destination that
negotiated ADD-PATH send, that is ze's own identifier, not the source's
(RFC 7911 Section 2). The reflector's VPN and labeled departures go out the
same way.
<!-- source: internal/component/bgp/plugins/rr/withdrawal.go -- walkUnicastNLRIs -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- sendPeerDown, recoverRouteSel -->
<!-- source: test/plugin/addpath-departure-rr-addpath-receiver.ci -- unicast departure under ze's identifiers -->
<!-- source: test/plugin/addpath-departure-rr-addpath-receiver-vpn.ci -- VPNv4 departure under ze's identifiers -->
<!-- source: internal/component/bgp/plugins/rs/server_inventory.go -- appendUnicastRecords, recordKey -->

For VPNv4 and VPNv6, a received withdrawal removes only the route with the
matching RD, prefix and negotiated Path Identifier. The Compatibility field
does not affect the match. In the route server and the reflector, a source-peer departure
reconciles the remaining inventory through the selecting RIB. On a destination without ADD-PATH, an
eligible surviving best route replaces the departed source with its own
attributes; a route without a usable replacement is withdrawn. A candidate
reflection may not send to that destination, a non-client's route toward a
non-client (RFC 4456 Section 6), is stepped past for the next in election
order. ADD-PATH
destinations keep surviving source paths and withdraw only the departed source's
advertised identifiers, including zero. Native VPN identity is preserved.
A departure is decided by the same `bgp-rib` election as an owner withdrawal.
Departure recovery remains owned until the destination operation completes or
the plugin shuts down; ordinary command timeouts do not discard it. If the RIB
cannot establish truthful sent ownership, the affected destination session is
reset rather than left with routes that should have been withdrawn. Recovery
does not reset a destination that has already reconnected.
<!-- source: internal/component/bgp/plugins/rs/server_inventory.go -- appendOpaqueRecords, recordKey -->
<!-- source: internal/component/bgp/plugins/rr/withdrawal.go -- walkVPNNLRIs -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- handleStateDown -->
<!-- source: internal/component/bgp/plugins/rib/rib_recovery.go -- recoveryRoutes, recoverySentNLRI -->
<!-- source: internal/component/bgp/reactor/relay_recovery.go -- recoverNLRIBatch, oweDeparture, queueRecoveryOutcome -->
<!-- source: test/plugin/addpath-departure-rr-replacement.ci -- the surviving route replaces the departed best -->
<!-- source: test/plugin/addpath-departure-rr-nonclient-skip.ci -- a non-client candidate is stepped past toward a non-client -->

A departure decides a prefix at a client only where the departed source owns
the route the client holds. When A and B both send a prefix and the client
holds B's route, A leaving writes nothing to that client for it. When the
client holds A's route, it is sent the best remaining route, or the withdrawal
when none remains. A labeled or VPN route that carried a stack of several labels
is withdrawn with the single Compatibility field 0x800000 in place of the stack
(RFC 8277 Section 2.4), so the client matches the route it holds.

The reflector honours Graceful Restart the way the route server does. When
`bgp-gr` retains a departed source's address family, the reflector leaves that
family's routes in place: clients keep them through the restart, because RFC
4724 Section 4.2 says the router "MUST NOT differentiate between stale and other
routing information during forwarding". They leave when `bgp-rib` purges them,
at the restart-time expiry, at the source's End-of-RIB, or on `release-routes`,
and that purge is decided like an owner withdrawal: the client receives the best
remaining route, or the withdrawal when none remains. A family the source did not
retain departs at once, as above.
<!-- source: internal/component/bgp/plugins/rr/rr.go -- handleStateDown, sendPeerDown, nlriDelCmd -->
<!-- source: internal/component/bgp/reactor/relay_recovery.go -- oweDeparture, departureOweFraming -->
<!-- source: internal/component/bgp/reactor/rr_withdrawal_replacement_test.go -- TestRRDepartureRecoversThroughOwedSet, TestRRDepartureHonoursRetention, TestRRGRPurgeRecoversSurvivor -->

The VPN daemon fixtures wait for the owning server or reflector to report both
peers UP before releasing the source's first route. They then fence each marker
on the destination's sent UPDATE count; the receiving peer checks the actual
VPN bytes, not those counters. A test can also hold back another source until
the destination is up (`release <peer address>`: the fixture sends that peer a
trigger route, and its script announces on receipt), and make the first source
wait until bgp-rib holds a route from a named peer (`held <peer address>`).
<!-- source: internal/test/fixture/register_vpn_withdrawal.go -- vpnWithdrawalInventory, vpnWithdrawalWaitSent, vpnWithdrawalPeerArgs -->

The reflector forwards each received UPDATE through `UpdateRoute` with the
`cached <id>` command. The engine adds `send bgp <selector>` and dispatches
the registered cache command. Inventory-only tests do not exercise this
dispatch boundary; the VPN peer exchanges check both forwarding and withdrawal.
<!-- source: internal/component/bgp/plugins/rr/rr.go -- forwardUpdate -->
<!-- source: internal/component/plugin/server/dispatch_registry.go -- opUpdateRoute -->
<!-- source: internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang -- send bgp cached -->
<!-- source: test/plugin/vpn-withdrawal-inventory-rr-ipv4.ci -- complete VPNv4 MP attributes -->
<!-- source: test/plugin/vpn-withdrawal-inventory-rr-ipv6.ci -- complete VPNv6 MP attributes -->

### Zero-Copy Forwarding

When two peers negotiate identical capabilities (same ADD-PATH mode, same ASN format, same extended message support), they share the same encoding context. If no destination-specific rewrite is needed, their routes are forwarded as raw wire bytes without decoding attribute values, rebuilding the UPDATE, or allocating a new payload.

For a session configured as a route-server client, received optional attributes
are retained for transparent redistribution, including unknown transitive and
unknown non-transitive attributes. The route server does not add a Partial bit
to an unknown transitive attribute on client redistribution. Ordinary sessions
still mark unknown transitive attributes Partial and discard unknown
non-transitive attributes. Forwarding from a route-server client to an ordinary
peer applies those ordinary rules to a separate outgoing copy; it never changes
the bytes shared with other route-server clients. Explicit operator policy may
modify route-server attributes; malformed attributes still follow their
error-handling rules.
<!-- source: internal/component/bgp/reactor/session_validation.go -- publishBase -->
<!-- source: internal/component/bgp/reactor/forward_body.go -- forwardWire, buildFwdBody -->

### Forwarding and Congestion

Each destination peer has a dedicated forwarding worker (long-lived goroutine with a buffered channel). When a destination peer is slower than the update rate:

1. The channel buffer absorbs short bursts (default capacity: 64 items)
2. If the channel is full, items go into a per-worker overflow buffer
3. The worker fires a congestion event (visible in logs and Prometheus metrics)
4. When the peer catches up and the channel drains below 25%, congestion clears

Overflow uses a two-tier pool: per-peer pools (64 slots) absorb steady-state traffic, and a shared MixedBufMux overflow pool (auto-sized from peer prefix maximums, overridable via `ze.fwd.pool.size` byte budget) bounds overflow memory. When the overflow pool is exhausted, items proceed without a pool handle and a warning is logged. Routes are never dropped -- missing a route update is worse than using extra memory. Prometheus metrics expose per-destination overflow depth (`ze_bgp_overflow_items`), per-source overflow ratio (`ze_bgp_overflow_ratio`), and global pool utilization (`ze_bgp_pool_used_ratio`).
<!-- source: internal/component/bgp/reactor/forward_pool.go -- per-destination forward workers, overflow pool -->
<!-- source: internal/component/bgp/reactor/reactor_metrics.go -- overflow Prometheus metrics -->

### Convergent Replay

When a peer reconnects, the route server replays eligible stored routes from other peers:

1. Full snapshot replay of non-FlowSpec routes from adj-rib-in
2. Delta loop catches those routes received during replay
3. Authorized FlowSpec paths replay from the mandatory selecting RIB
4. End-of-RIB sent when replay completes

FlowSpec has one snapshot source: ordinary adj-rib-in store replay excludes
those families. Both the route server and reflector replay current authorized rules
from `bgp-rib` before End-of-RIB even when optional-store replay fails. The
route server preserves its received-generation cut, and both roles check the
destination's reconnect generation. Reconstructed routes still pass ordinary
authorization, reflection and export policy.
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- replayForPeer -->

The route server splits the UPDATEs for a peer that comes up at a message-id
cut. The replay delivers every UPDATE at or below the cut, and the live forward
delivers every UPDATE above it. The two rails are separate calls into the
engine, the replay a relay of stored routes and the live forward a forward of
cached UPDATEs, so the engine orders them for that peer alone. While a process that
reports the peer's initial update (the route server, the route reflector) has not
yet reported it done, the engine holds the peer's live forwards and validation
changes in the peer's forward queue. The replayed routes pass. The report comes
after the replay's End-of-RIB, and the held changes then go out in the order they
arrived. A live withdrawal can then never arrive before a replayed announcement
of the same prefix, and the first route the source announced arrives first.

Other peers are not held. A live UPDATE goes to every other destination at
once, whatever replay is running. The held changes wait in the peer's forward
overflow queue, so the congestion controls of that queue apply to them: overflow
denial, then teardown of the peer, never a silent drop (see
[the replay fence](../architecture/forward-congestion-pool.md#the-replay-fence)).

A peer-down withdrawal leaves the route server by selector, on the announce
rail. For a peer whose replay is still running, the engine puts it in the same
forward queue as the held live changes, so the replay passes it and it reaches
the peer after the replay's End-of-RIB. A replay read before the receive store
processed the peer-down can still carry the route, and the withdrawal that
follows it removes it.
<!-- source: internal/component/bgp/reactor/peer.go -- forwardOrderHold, initialUpdateOwed, withdrawBehindForwards -->
<!-- source: internal/component/bgp/plugins/rs/server_handlers.go -- handleStateUp, endReplay -->
<!-- source: internal/component/bgp/plugins/rs/server_forward.go -- flushBatch -->
<!-- source: internal/component/bgp/plugins/rs/server_validation.go -- replayFlowSpecs -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- replayForPeer -->
<!-- source: internal/component/bgp/plugins/rr/validation.go -- replayFlowSpecs -->

The reflector also completes its readiness report when the optional receive
store's replay returns an error. It first attempts the independent mandatory
FlowSpec replay, then reports the original peer-UP session token. Completion
means its attempts ended, not that every route was delivered: a send-permission
refusal stays a refusal. The reactor rejects a late token from an old Session,
so that error exit cannot release a replacement Session's queued live work.
<!-- source: internal/component/bgp/plugins/rr/rr.go -- replayForPeer, signalSessionReady -->
<!-- source: internal/component/bgp/reactor/peer.go -- creditAPIReady -->

If no other plugin owns replay for a particular peer, `bgp-adj-rib-in` obtains
the same authorized FlowSpec snapshot from `bgp-rib` before reporting that its
initial update is ready. A delegated replay owner suppresses this self-replay;
if no owner both receives state events and has permission to send UPDATEs for
the peer, the engine's unheld-role marker restores it. The bounded relay uses
the reactor's existing destination session, family, source-generation and
export checks. An up whose reason is `attached` replays nothing: a reload
attached `bgp-adj-rib-in` to a session that was already up, and the peer's
initial update went out without it.
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib.go -- handleStructuredState, handleState -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_replay_path.go -- replayFlowSpecs -->

The route server and reflector advertise the same `bgp-peer-up-replay` role
at startup. Adj-RIB-In consumes that claim during configure, before peers
start; the engine retracts it per peer when no owner both receives that peer's
state event and has `send [ update ]` permission. An observer or a plugin with
only `send [ raw ]` does not suppress Adj-RIB-In's self-replay.
If a reflector joins later or restarts, its post-startup callback uses the
existing `claim-replay` command to notify a receive store that is already
running. The startup declaration still provides ordering for the first peer.
<!-- source: internal/component/bgp/plugins/rr/register.go -- init -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_claims.go -- applyStartupClaims, replayDrivenElsewhere -->
<!-- source: internal/component/bgp/plugins/rr/rr.go -- runRouteReflector OnAllPluginsReady -->
<!-- source: internal/component/bgp/server/events.go -- onPeerStateChange -->

## Plugin Bindings

The `bgp-rs` plugin requires:
- `receive [ update-received state open-received refresh ]` -- the UPDATEs the peer
  sends, its session state, the capabilities in its OPEN, and route refresh
- `send [ update ]` -- sends forwarded routes back to the peer

The received direction is not a preference. A route server that also receives the
UPDATEs ze sends deadlocks: forwarding one raises the sent event that the forward
is waiting on.
<!-- source: internal/component/bgp/plugins/rs/server.go -- SetStartupSubscriptions and the deadlock rationale -->

The same two grants govern `behavior { rs-fast-path enable; }`. The fast path
forwards in place of `bgp-rs`, so it obeys the permissions `bgp-rs` would. It
relays an UPDATE only from a source peer that grants `bgp-rs`
`receive [ update-received ]` (or `update`, or `*`), and only to a destination
peer that grants `bgp-rs` `send [ update ]`. A source without the receive grant
gets no forwarding on either path: the fast path declines it, and `bgp-rs` never
sees its UPDATEs. A destination without the send grant is sent nothing: the fast
path hands it back to `bgp-rs`, whose forward is refused and logged as
`send refused`, exactly as with the fast path off. When a peer with
`rs-fast-path` lacks either grant, Ze logs one warning for it as its session
comes up (`rs-fast-path: this peer does not grant the route server what the
fast path needs`), naming the missing grant and the line to add. To fix either, add
`attach process <rs-name> { receive [ update-received ]; send [ update ]; }` to
the peer, where `<rs-name>` is the name your `plugin { internal ... }` block
gives `bgp-rs`.
<!-- source: internal/component/bgp/reactor/peer_forward_facts.go -- rsRelaySendGranted, rsRelayReceiveGranted, reportRSRelayGrants -->
<!-- source: internal/component/bgp/reactor/forward_rs.go -- reactorForwardRSSection -->
<!-- source: internal/component/bgp/reactor/reactor_notify.go -- notifyMessageReceiver fast-path gate -->
<!-- test: test/plugin/bgp-rs-fastpath-relay-grants.ci -- a destination without send [ update ] gets nothing and logs send refused; a source without receive [ update-received ] is relayed to nobody -->

The `bgp-adj-rib-in` plugin stores received routes for replay on peer reconnect.
It needs only `receive [ update-received state ]`. When a peer comes up, the
route server or reflector reads the stored routes from it, page by page, and
sends them to the peer itself, so the peer's `send [ update ]` for `rs` (or the
reflector) is the grant that covers the replay. Grant `bgp-adj-rib-in`
`send [ update ]` yourself only for a peer no route server or reflector serves,
where it replays on its own and sends under its own name.
<!-- source: internal/component/bgp/plugins/rs/ -- RunRouteServer; internal/component/bgp/plugins/adj_rib_in/ -- adj-rib-in for replay -->
<!-- source: internal/core/bgp/ribevents/replay.go -- RelayReplay -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/replay_page.go -- replayRoutesCommand -->
<!-- source: internal/component/bgp/reactor/send_permission.go -- maySend -->

## Cache Commands

Routes in transit can be managed via cache commands:

| Command | Description |
|---------|-------------|
| `show cache` | List cached messages |
| `request cache retain <id>` | Prevent cache eviction |
| `request cache release <id>` | Release a cached message |
| `request cache expire <id>` | Remove a cached message immediately |
| `send bgp <peer> cached <id>` | Forward a cached message to a peer |
<!-- source: internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang -- module ze-cli-cache-cmd -->

## Without Route Reflection

When `bgp-rs` is not loaded, received routes are not forwarded to other peers. The `bgp-rib` plugin stores routes and performs best-path selection, but does not re-advertise them. To forward routes between peers, load the `bgp-rs` plugin.
<!-- source: internal/component/bgp/plugins/rib/register.go -- bgp-rib registration -->
