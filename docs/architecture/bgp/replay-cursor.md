# Replay Cursor: batching the Adj-RIB-Out replay

When a BGP peer reconnects, the RIB plugin replays its stored Adj-RIB-Out to the
engine. The old path issued one `UpdateRoute` RPC per route, each costing about
100 to 200 microseconds in JSON marshal, text tokenization and NLRI parsing. A
peer holding 100K routes blocked for about 10 seconds. The dominant cost was per
call, not attribute encoding, so the goal was to cut the call COUNT from
O(routes) to O(distinct attribute sets).

<!-- source: internal/component/bgp/plugins/cmd/update/cursor.go -- cursor handler, cursor state, ClearProcessCursors -->
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- grouped collection, delta formatting, sorted replay -->

## The decisions

**A stateful text protocol with delta encoding, not a DirectBridge binary
protocol.** `update cursor` stays inside the existing dispatch path, works for
an external plugin, and can be logged and read.

**`update cursor` is a fourth encoding mode in the `handleUpdate` switch, not a
separate `replay` command tree.** It reuses the existing YANG registration and
dispatch through one switch case.

**Cursor state is a package-level `sync.Map`, not a field on `CommandContext` or
`Process`.** Handlers are stateless by existing convention, and this keeps the
cursor from coupling to plugin infrastructure.

**`del <attr>` came back in cursor mode only.** `update text` removed `set` and
`del` on purpose, and it keeps that simplicity.

**Cleanup registers through `RegisterProcessCleanup`, not through a direct
import.** The server cannot import the update command package without a cycle.
The hook is now available to any command package that needs per-process cleanup.

**The sort key is the wire hash without AS_PATH.** Consecutive groups that share
that hash need only an `as-path [...]` delta, which is the most common
transition between source peers. Sorting on message id alone loses that.

## What it buys

Replay drops from O(N) calls to O(M) calls, where M is the number of distinct
attribute sets and is far below N. Measured: 1.8ms for 1K groups covering 100K
routes. The grouped variant decodes each `AttrHandle` once, so the per-route
reconstruction path is no longer called from replay. Manual resend uses the same
grouped collection. Groups retain family, attribute handle, path identifier,
ADD-PATH presence, stale level, source peer and any next hop carried separately by JSON.
When immutable wire attributes are present, each batch uses the self-contained
`update hex` rail rather than reducing those attributes to cursor text fields.
Opaque families keep their native NLRI bytes; CIDR families encode their value
keys on this cold path. Hex batches do not alter text cursor state.

Config-static advertisements are retained in Adj-RIB-Out with their origin
flag. A ROUTE-REFRESH includes them, subject to the current outbound policy,
just like other routes of the requested family (RFC 2918 Section 4).
Peer-up replay excludes them because the reactor sends the current configured
routes itself. Replay feedback does not modify the inventory, so it cannot
resurrect a purged route, overwrite locally updated attributes, or change its
origin. A later non-replay advertisement replaces the entry. Lifecycle
withdrawals remove the owned entry before dispatch; their feedback is likewise
not a second mutation.

<!-- source: internal/component/bgp/plugins/rib/rib_structured.go -- handleSentStructured, storeSentEntries -->
<!-- source: internal/component/bgp/plugins/rib/ribout_entry.go -- ribOutEntry -->

An external plugin can use the same protocol for its own batched updates.

## The gap a reader must not overstate

**RFC 9494 stale metadata is threaded, not honored end to end.** The resend path
routes stale groups through `updateRouteWithMeta`, which carries
`map[string]any{"stale": level}` into `CommandContext.Meta`. No update handler
and no egress filter reads `CommandContext.Meta`: it is written and never
consumed. The only egress reads of `.Meta` are `ReceivedUpdate.Meta` on the
peer-to-peer forwarded path, which is not the plugin-originated cursor path.

The cursor carries the metadata. The update egress path does not act on it. Do
not describe this as preserving RFC 9494 metadata end to end.
