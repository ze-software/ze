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

Same-session refresh and automatic lifecycle cleanup carry the captured sent
message-ID receipt through lossless `sent-owner-message` metadata. The final
writer matches that receipt against its current native slot before accepting
the command. Refresh preserves the slot's original source/path owner and its
revision, because replay feedback intentionally does not rewrite this projection.
A stale command cannot overwrite a later advertisement.

Peer-up replay is a different authority, not an absent-owner exception to that
maintenance check. The peer-UP event carries the new Session's lossless
`initial-replay` token. The writer requires that token while the initial EOR is
owed or the peer-up replay fence remains open; EOR completion alone does not
expire a still-running replay. Neither a `replay` flag nor a delayed current-peer
lookup grants this authority. A stored locally originated path explicitly carries
`initial-local`; it can populate an empty slot but cannot displace an
advertisement already accepted on that session. Its stored sent receipt remains
the revision used by later refresh/cleanup.

Forwarded sent history retains a stable source-Peer incarnation receipt and the
received path identifier and presence, independently of the destination's
ADD-PATH encoding. The existing sent event carries typed body-common source
identity and a local-origin bit, plus family/announcement ordinals only for
received ADD-PATH paths. No generic metadata map is created or cloned for an
ordinary forward, no attributes are copied, and no second route inventory is
created. Replay resolves
that exact source incarnation and requires the original received path and message
revision to remain in the mandatory RIB. Changed source history is ineligible,
not a reason to restore stale attributes or elect a new route. The dispatch
captures the source's generation and state for the final writer, including a
still-retained GR/LLGR source. The source registry's address-based ID alone is
not incarnation proof. This preserves valid forwarded peer-up history even
without the optional Adj-RIB-In plugin and never reclassifies missing provenance
as local.

The RIB joins the existing initial-update fence: restoration runs before queued
live changes and automatic cleanup, without a new per-route inventory or a
synchronous callback waiting on its own delivery barrier. Restored routes retain
the old logical sent revision so deferred expiry can still withdraw them; every
actual new wire message has a fresh message ID. Old-session or closed initial
tokens return an error through the synchronous initial writer and cannot grant
authority on a replacement session. Ordinary Expected refresh/cleanup joins the
forward queue while the initial gate or older forwards are outstanding.

Replay completion uses `request peer <addr> plugin session ready session
<captured-token>`, including zero-group and completed error paths. The readiness
owner validates that same captured token under the peer's session lock before
crediting the named process. A stale or missing token cannot release the
replacement's live work merely because no UPDATE passed through the writer.
Readiness acknowledges that this producer's attempts finished, not successful
delivery of every historical route. If source history becomes ineligible after
collection, the writer still rejects it. The RIB clears the failed cursor and
its attribute-delta predecessor, attempts the remaining independent groups with
full attributes, then reports completion with the original token. The error
remains logged; neither retrying another group nor completion grants authority
to the rejected history or to a replacement Session.
<!-- source: internal/component/bgp/reactor/session_ownership.go -- beginAdjOut, allow, recordSection -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- sendBatchUpdate -->
<!-- source: internal/component/bgp/plugins/rib/rib_replay.go -- collectPeerUpReplay, formatCursorCommands -->

<!-- source: internal/component/bgp/plugins/rib/rib_structured.go -- handleSentStructured, storeSentEntries -->
<!-- source: internal/component/bgp/plugins/rib/ribout_entry.go -- ribOutEntry -->

An external plugin can use the same protocol for its own batched updates.

## Stale replay and evidence boundary

Stale groups carry their level through `CommandContext.Meta` into the typed
batch. The replay writer passes it to the existing readvertise egress chain:
the result can preserve or modify the advertisement, suppress it, or withdraw
it from a peer that cannot retain it. Those final bytes still pass through the
same sent-owner admission as other maintenance commands. An initial replay
that becomes a withdrawal cannot remove a newer owner or invent an old
advertisement on an empty new session.

This describes the producer, command, policy, and writer contracts, not a claim
that an external SDK process, every GR/LLGR lifecycle, or a runtime race gate has
been exercised.
<!-- source: internal/component/bgp/plugins/cmd/update/update_text.go -- staleLevelFromMeta -->
<!-- source: internal/component/bgp/reactor/reactor_api_batch.go -- sendStaleReadvertiseUnit, decideStaleReadvertise -->
