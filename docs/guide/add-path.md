# ADD-PATH and PATHS-LIMIT

ADD-PATH (RFC 7911) allows multiple paths per prefix by including a Path Identifier with each NLRI. Route servers use it to forward all available paths rather than one best path.

PATHS-LIMIT (draft-abraitis-idr-addpath-paths-limit-04) lets a receiver request a maximum number of paths per prefix per family. Ze enforces the peer's negotiated limit on outbound routes, sending the paths `bgp-rib` ranks best, and enforces its own advertised limit on the routes it receives.
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path capability config -->

## Configuration

All ADD-PATH and PATHS-LIMIT config lives under `session > capability > add-path`.

### Default Direction and Limit

Enable ADD-PATH for all negotiated families, with an optional default path count limit:

```
capability {
    add-path {
        direction send/receive;
        limit { receive 10; }
    }
}
```

The `limit` container holds one path count per direction. Only `receive` exists: the number of paths per prefix Ze asks the peer to send. The container's `receive` is inherited by all families, and a per-family entry can override it.

### Per-Family Override

Override direction, set path count limits, or set negotiation mode per family:

```
capability {
    add-path {
        direction send;
        family {
            ipv4/unicast {
                direction send/receive;
                limit { receive 10; }
            }
            ipv6/unicast {
                direction receive;
                mode require;
            }
        }
    }
}
```

A family entry can carry no block at all. `family { ipv4/unicast; }` takes the container direction and the container `receive` limit. A bare entry adds no family when the container states no direction.
<!-- source: internal/component/bgp/reactor/config_capabilities.go -- parseAddPathFromTree -->

### Full Example

```
bgp {
    router-id 192.0.2.1;
    peer transit-a {
        connection {
            remote { ip 10.0.0.1; }
            local { ip 10.0.0.2; }
        }
        session {
            asn { local 65000; remote 65001; }
            capability {
                add-path {
                    direction send/receive;
                    family {
                        ipv4/unicast { limit { receive 10; } }
                    }
                }
            }
            family {
                ipv4/unicast { prefix { maximum 1000; } }
                ipv6/unicast { prefix { maximum 1000; } }
            }
        }
    }
}
```

## Direction and Mode

| Direction | Meaning |
|-----------|---------|
| `send` | Advertise multiple paths to peer |
| `receive` | Accept multiple paths from peer |
| `send/receive` | Both directions |

| Mode | Meaning |
|------|---------|
| `enable` (default) | Negotiate ADD-PATH if peer supports it |
| `require` | Reject peer if it does not support ADD-PATH |
| `refuse` | Reject peer if it advertises ADD-PATH |
| `disable` | Do not negotiate ADD-PATH for this family |
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- add-path direction/mode config -->

## PATHS-LIMIT

The `limit { receive N; }` setting advertises a PATHS-LIMIT capability (code 76) for the family. The value (1-65535) requests the maximum number of paths per prefix Ze wants to receive.

The scalar spelling `limit 10;` was removed. Ze refuses a config that still carries it, and the error names the line to write instead: `limit { receive VALUE; }`.

The peer is asked to honor this limit, and Ze also enforces it on what arrives: see [Inbound Enforcement](#inbound-enforcement). It does not set Ze's outbound limit.

| Setting | Effect |
|---------|--------|
| No effective `receive` | Ze requests no path count limit for that family |
| `limit { receive 10; }` | Request at most 10 paths per prefix from the peer |
| `limit { receive 1; }` | Request one path per prefix, still with ADD-PATH identifiers |
| `receive` under `mode disable` | Ze advertises no limit for the family. A limit reaches the wire only for a family Ze advertises in ADD-PATH |
| `receive` on a family whose direction is `send` | Ze advertises no limit for the family. A limit reaches the wire only for a family whose ADD-PATH direction includes receive |
<!-- source: internal/core/bgp/capability/capability.go -- PathsLimit struct -->
<!-- source: internal/component/bgp/reactor/config_capabilities.go -- parseAddPathFromTree -->

The peer's nonzero limit applies only when Ze negotiates ADD-PATH send for that family. Ze's local limit applies only to negotiated ADD-PATH receive. An absent limit or an unnegotiated ADD-PATH direction imposes no PATHS-LIMIT restriction.

### Inbound Enforcement

The `receive` limit Ze advertises is also the ceiling it applies to the paths it accepts. A peer that ignores capability 76 (BIRD has no support for it) cannot grow Ze's Adj-RIB-In past the number Ze asked for. Ze counts distinct path IDs for each AFI/SAFI and prefix on the session, with the same route identity as outbound enforcement.

| Event | Effect |
|-------|--------|
| New path below the limit | Admit the path and count its ID |
| New path at the limit | Drop that path only. Every other route in the same UPDATE installs normally |
| Replacement with an admitted path ID | Accept the replacement without consuming another slot |
| Withdrawal of an admitted path ID | Remove the ID and free its slot |
| Withdrawal of a refused path ID | Accept it. It frees no slot, because it never held one |
| UPDATE whose only paths were all refused | Consume it without delivering anything; the session stays up |
| Session reconnect | Start with an empty count |

The check runs after RFC 7606 error handling and before the prefix limits (`prefix { maximum N; }`), so a prefix limit counts the UPDATE without its refused paths. When a prefix limit then drops the UPDATE, the path count is restored to what it was before it. The withdrawals RFC 7606 treat-as-withdraw produces free slots like any other withdrawal.

Selection is by arrival order: the first paths admitted keep their slots, and a better path that arrives later is dropped until a withdrawal frees a slot. Ze does not hold a dropped path for later; the peer must announce it again.

Each refused path increments `ze_bgp_paths_limit_refused_total{peer,family}`. The first refusal for a route logs one warning naming the peer, family, limit, route and path ID; further refusals for that route are counted but not logged until the peer withdraws a refused path. Ze tracks at most one refused path ID per route beyond the limit, so a peer cannot decide how much memory this costs.

A limit configured on a family whose ADD-PATH direction does not include receive is neither advertised nor enforced.
<!-- source: internal/component/bgp/reactor/session_paths_limit_recv.go -- receiveCeiling, settleReceiveCeiling, reportPathsLimitRefusals -->
<!-- source: internal/component/bgp/reactor/session_read.go -- processValidatedMessage -->

### Outbound Enforcement

Ze counts distinct advertised path IDs for each AFI/SAFI and prefix on the destination connection. The count persists across UPDATE messages and batches. All normal send and forwarding paths share this count, including route-server fast-path forwarding.

The count uses each family's route identity, not its forwarding fields. Labels do not create a new labeled IP prefix; VPN route distinguishers do. EVPN, VPLS, and MUP omit their non-key forwarding fields. Insignificant prefix padding cannot create another slot.
<!-- source: internal/core/bgp/nlri/nlrisplit/prefix_key.go -- PrefixKeyFunc; internal/core/bgp/nlri/nlrisplit/register.go -- family key registration -->

| Event | Effect |
|-------|--------|
| New path below the limit | Admit the path and count its ID |
| New path at the limit | Suppress the announcement and mark the prefix contested |
| Replacement with an admitted path ID | Send the replacement without consuming another slot |
| Withdrawal of an admitted path ID | Remove the ID and free its slot |
| Destination reconnect | Start with an empty count for the new connection |

#### Best paths under the peer's limit

Arrival order fills the slots first, but it does not decide what the peer keeps. When a path is suppressed at the limit, or a slot of a contested prefix is released, Ze asks `bgp-rib` to rank the prefix's paths and sends the best ones in place of the first arrivals. The election runs on the destination's recovery worker, never on the write path, and repeated contests of one prefix merge into one election.

| Step | What the peer sees |
|------|--------------------|
| A held path the RIB no longer ranks among the best | A withdrawal of its path ID |
| A better path that was suppressed | An announcement under its own path ID, sent after the withdrawal that freed its slot |
| A held path the RIB still ranks among the best | Nothing: it keeps its slot and its path ID |
| A path Ze originates itself | Nothing: the RIB does not rank it, so it keeps its slot and the election fills only the remaining slots |
| A path graceful restart retains for a source whose session ended | Nothing: it keeps its slot. When the source is back and the RIB ranks the retained path among the best, it still counts as one slot, so the next best path fills the rest and the peer receives its full limit |

Withdrawals always go before the announcements that replace them, so the peer never holds more paths than its limit, not even for one UPDATE.

The election needs `bgp-rib` loaded and attached to every source peer with `receive [ update-received ]`, which a route server or route reflector configuration already has. Without a RIB that can rank the paths, the first paths to arrive keep their slots, each contested prefix increments `ze_bgp_paths_limit_election_failed_total{peer,family}`, and the session logs one warning per family naming the fix.
<!-- source: internal/component/bgp/reactor/session_paths_limit_elect.go -- owePathsLimitElections, planPathsLimitElection, queuePathsLimitElection, pathsLimitElectionFailed -->
<!-- source: internal/component/bgp/reactor/recovery_owed.go -- decide -->

A family Ze holds no NLRI decoder for cannot be counted path by path, so Ze never advertises a receive limit for it. A `limit { receive N; }` that would reach such a family, on the family entry or through the add-path container, is refused when the config loads, with an error naming the family and the fix: remove the receive limit for that family, set its add-path direction to `send`, or load the plugin that decodes it. A send limit the peer advertises for such a family is negotiated but not enforced, and the session logs one warning naming the family when it reaches Established.
<!-- source: internal/component/bgp/reactor/session_paths_limit.go -- pathsLimitEnforceable, newPathsLimitAdmission, warnPathsLimitUnenforced -->
<!-- source: internal/component/bgp/reactor/config_capabilities.go -- parseAddPathFromTree -->

Named transactions retain separate path IDs for one prefix. They emit routes in deterministic identity order, with ascending path IDs for the same prefix. A lower ID does not displace a path admitted earlier on the connection.

The deliberate `send ... raw message` diagnostic command sends exact bytes outside normal route admission. PATHS-LIMIT does not filter this command.
<!-- source: internal/component/bgp/reactor/session_paths_limit.go -- pathsLimitAdmission, pathsLimitFamily, admitPath, filter -->
<!-- source: internal/component/bgp/transaction/commit_manager.go -- path-aware transaction identity and ordering -->

## How It Works

When ADD-PATH is negotiated, each NLRI is prefixed with a 4-byte Path Identifier. This allows the same prefix (e.g., `10.0.0.0/24`) to appear multiple times with different path IDs, each carrying different attributes.

### Wire Format

Without ADD-PATH:
```
[prefix-length][prefix-bytes]
```

With ADD-PATH:
```
[4-byte path-id][prefix-length][prefix-bytes]
```

### Encoding Context

Peers that negotiate the same ADD-PATH modes share an encoding context (`ContextID`). The route server can forward wire bytes unchanged between peers with matching contexts, avoiding re-encoding.
<!-- source: internal/component/bgp/reactor/session.go -- ContextID; internal/component/bgp/reactor/reactor_api_forward.go -- zero-copy forwarding -->

### Route Withdrawal

In an ADD-PATH direction, a withdrawal includes the same path ID used in the announcement. Withdrawing one ID leaves the other paths for that prefix in place. Path ID zero is a valid identifier, not an instruction to withdraw every path.

### Ze Generates Its Own Path Identifiers

RFC 7911 Section 2 requires a speaker that re-advertises a route to generate its own Path Identifier, and to assign it so that (prefix, identifier) uniquely names a path advertised to a neighbor. Both forward rails do that instead of relaying the identifier the source chose.

Each route-server client picks its identifiers alone. Two clients that both pick `1` for one prefix would reach a third client as one (prefix, identifier) pair, and RFC 7911 Section 5 makes the receiver treat the second as a replacement for the first, so one path is lost. A source that negotiated no ADD-PATH sends every path under identifier `0`, so without regeneration every path from every ordinary client reached an ADD-PATH client as (prefix, 0).

| Property | Behavior |
|----------|----------|
| Key | The path at ingress, never the message and never the attributes. How much of the path the key holds follows what the source framed: a source that negotiated no ADD-PATH gets one identifier for its whole session, and a source that negotiated it gets one entry per family, received identifier and prefix |
| Withdrawal | A withdrawn route carries no path attributes, so the ingress key is the only key that can be recomputed when the path leaves. The withdrawal names the identifier Ze advertised |
| Re-announcement | A source that re-announces one path with changed attributes keeps the identifier it already has, so the destination sees a replacement rather than a duplicate |
| Release | A source that framed no identifier holds its one entry until the peer is removed. A source that framed one has each pair freed when Ze has relayed that pair's withdraw. Neither is released at session down, so a reconnecting peer re-announces under the identifiers its destinations already hold |
| Zero | Minted and accepted like any other value, which RFC 7911 Section 3 requires |

Regeneration runs whenever either side of the forward frames identifiers. A session where neither side negotiated ADD-PATH keeps its zero-copy forward.
<!-- source: internal/component/bgp/reactor/forward_path_id.go -- fwdPathIDs, fwdRegenerateRawPathIDs, fwdReleaseWithdrawnPathIDs -->
<!-- source: internal/component/bgp/reactor/forward_body.go -- fwdReencodeNLRIs -->

## Interaction with Route Reflection

ADD-PATH fits the route server plugin (`bgp-rs`). Without ADD-PATH, the route server can only forward one path per prefix to each peer. With ADD-PATH, it forwards received paths within each destination's negotiated PATHS-LIMIT. Downstream routers make their own best-path decisions.
<!-- source: internal/component/bgp/plugins/rs/ -- route server ADD-PATH forwarding -->
