#!/usr/bin/env python3
"""Process plugin that originates one prefix on EACH of the two announce rails.

Both prefixes carry the same caller attributes: COMMUNITIES (type 8) and
LARGE_COMMUNITIES (type 32). The rail contributes ORIGIN (1), AS_PATH (2) and
NEXT_HOP (3), and `(*announceAttrs).emit`
(internal/component/bgp/reactor/announce_build.go) writes all five in ascending
type-code order. BIRD must install both prefixes and report both community values
on each.

Which rail runs is decided by `Peer.ShouldQueue`
(internal/component/bgp/reactor/peer.go), and the two rails reach `emit` with
DIFFERENT inputs. Measured in the ze container on 2026-08-05:

| Announce time | Rail | Producer | base | plan |
|---------------|------|----------|------|------|
| before the session establishes | queue | `buildRIBRouteUpdate` (peer_rib_routes.go) | nil, always | `[1 2 3 8 32]` |
| after the initial sync drains | batch | `buildBatchAnnounceUpdate` (reactor_api_batch.go) | the caller's block, codes 8 and 32 | `[1 2 3]` |

The first row's "before the session establishes" is CONFIGURED, not timed. ze is
passive (`connect false`, ze.conf) and BIRD holds its dial for 30 seconds
(`connect delay time 30`, bird.conf), so the establishment cannot happen before
this plugin has announced. The guard below asserts the rail anyway.

`plan/learned/1320-wire-edit-4-api-origin.md` converged both rails on one writer,
so a scenario that reaches only one of them leaves half the convergence untested
against a live peer. It also leaves the discrimination mutation rail-specific:
each rail is falsified by a different one (check.py, DISCRIMINATION).

The command supplies NO `origin` and NO `path`, and that is load-bearing.
`buildBatchAnnounceUpdate` synthesizes ORIGIN and AS_PATH only when the caller
supplied neither: `hasCode` and `hadASPath` answer from the caller's attributes
first. Leaving both out puts the two well-known mandatory attributes on the RAIL,
so the caller contributes exactly what the check asserts. A defect that loses the
caller's attributes then leaves a VALID route whose communities are gone, and the
check fails on the community assertions. With `origin igp path 65001` in the
command, the same defect also removes the mandatory attributes, BIRD applies
RFC 7606 treat-as-withdraw, and the check fails on a missing route instead.
Measured both ways on 2026-08-05.

One prefix carries both community types on purpose. Splitting them over two
prefixes would leave the COMBINED attribute block untested, and the combined
block is what the rail assembles.
"""

import os

from ze_api import _get_api, flush, ready, runtime_fail, wait_for_shutdown

QUEUE_PREFIX = "10.55.0.0/24"
BATCH_PREFIX = "10.55.1.0/24"

# BIRD's address, and the KEY of its row in `show bgp peer <sel> detail`:
# `HandleBgpPeerDetail` (internal/component/bgp/plugins/cmd/peer/peer.go) writes
# `result[p.Address.String()]`, so the row is keyed by address even when the
# selector is the peer name. `_render_scenario_dir` (test/interop/interop.py)
# rewrites the 172.30.0. prefix in every text file of the scenario, this one
# included, so the literal follows the subnet the run allocated.
BIRD_ADDR = "172.30.0.4"

ATTRS = "nhop 172.30.0.2 community [65001:100 65001:200] large-community [65001:0:1]"

# The harness gives the session SESSION_TIMEOUT seconds to establish and passes
# the value into the ze container (`docker_run(..., env=)`,
# test/interop/interop.py), so the barrier below is DERIVED from the harness
# budget instead of repeating a constant that a host setting SESSION_TIMEOUT
# above 120 would invert.
#
# The barrier MUST outlive the check's own wait. A plugin that gives up first
# calls runtime_fail, which dispatches `request shutdown` and stops ze while
# check.py is still waiting. check.py scans ze's log for the ZE-OBSERVER-FAIL
# sentinel and reports the plugin's own message, so the ordering is no longer
# silent either way; this margin keeps the check's diagnostics first.
#
# The margin is 90 seconds on top: check.py starts its clock after `setup`,
# which can take ~35s (container start plus `wait_containers_healthy(30)`),
# while this clock starts when the plugin does.
try:
    SESSION_TIMEOUT = int(os.environ.get("SESSION_TIMEOUT", "90"))
except ValueError:
    SESSION_TIMEOUT = 90
EOR_DELAY = 0.25
EOR_ATTEMPTS = int((SESSION_TIMEOUT + 90) / EOR_DELAY)

ready()
api = _get_api()

# Rail 1, the QUEUE rail. The session is not established yet, so ShouldQueue is
# true, the route is stored in the RIB, and the initial-sync drain replays it
# through buildRIBRouteUpdate.
#
# Nothing sleeps before this announce. The scenario does not RACE the session
# any more: ze is passive (`connect false`, ze.conf) and BIRD holds its dial for
# 30 seconds (`connect delay time 30`, bird.conf), so the only route to
# Established opens long after this line. The guard below still asserts the rail
# rather than trusting that arithmetic.
#
# `_peer_row_or_fail` (test/scripts/ze_api.py), NOT `peer_counter`. The counter
# helper ends `return total if seen else default` with `default=0`, and
# `peer_fields` returns `{}` for any status other than "done" -- an RPC error, a
# selector matching no row, a payload without a `peers` key. A total failure to
# READ therefore returns 0, which is the exact value that means "safe to
# proceed", so the guard passed on a lookup that answered nothing
# (ai/rules/evidence.md: a zero value must never be a valid-looking answer).
# `_peer_row_or_fail` calls runtime_fail when the row is missing, which is why
# ze_api documents it as the helper for a per-peer absence claim.
row = api._peer_row_or_fail(  # noqa: SLF001  # documented per-peer absence helper
    BIRD_ADDR, "queue-rail guard"
)

# Two independent readings of the same row, both fail-closed.
#
# `state` is the DIRECT one: `Peer.ShouldQueue`
# (internal/component/bgp/reactor/peer.go) returns true immediately when
# `p.State() != PeerStateEstablished`, so any state but "established" puts this
# announce on the queue rail with no further condition.
#
# `eor-sent` covers the remaining window, in which the peer IS established and
# the initial sync is still draining. It is raised only by `IncrEORSent`
# (internal/component/bgp/reactor/peer_stats.go), and both of its initial-sync
# call sites are inside `(*Peer).sendInitialRoutes`
# (internal/component/bgp/reactor/peer_initial_sync.go) -- the pre-teardown EOR
# loop and the initial-sync EOR loop -- ahead of the
# `sendingInitialRoutes.Store(0)` that ends that function. So `eor-sent == 0`
# implies the sync has not finished. It is a per-peer LIFETIME counter, reset
# only by `ClearStats`, whose one non-test caller is `(*Peer).cleanup` on
# session teardown: it accumulates across flaps rather than being cleared per
# session. That direction is fail-closed here, since a stale count can only make
# this guard fire.
#
# The converse does not hold, and the comment used to claim it did: four paths
# reach `sendingInitialRoutes.Store(0)` without raising `eor-sent`, all four in
# `(*Peer).sendInitialRoutes` -- the panic-recovery `defer`, the `nc == nil`
# abort, the EOR-send failure `break`, and `ClaimInitialSyncEOR` returning false
# for every family. None of them yields a silent green: `wait_peer_eor_sent`
# below then spins out and this plugin fails loudly.
#
# THIS GUARD IS A BACKSTOP, NOT THE MECHANISM. The mechanism is `connect false`
# (ze.conf) plus `connect delay time` (bird.conf), which makes the queue rail a
# CONFIGURED barrier of about 28 seconds. The guard cannot be the mechanism,
# because it has a fail-open window by construction: `(*Peer).run`
# (internal/component/bgp/reactor/peer_run.go) calls `setState(PeerStateEstablished)`
# and only then `sendingInitialRoutes.Store(1)`. A row read between those two
# statements says `state=established` and `eor-sent=0`, which is exactly what
# this guard passes on, while `ShouldQueue` is already false and the announce
# would take the BATCH rail. The window is the few statements between them and
# is unreachable behind the 28s barrier, which is why the barrier is the
# mechanism and this is the check that the barrier held.
#
# Single-shot where the sibling helpers poll (`wait_peer_eor_sent` below,
# `wait_route` in check.py). That is correct here: the guard asserts a state
# that must hold AT THIS INSTANT, not one to wait for, and its failure direction
# is a false RED -- a row read a moment too late fails a run that could have
# been green, never the reverse.
#
# Without this guard the scenario reads GREEN on a broken queue rail. Deleting
# the barriers in ze.conf and bird.conf puts BOTH prefixes on the batch rail,
# and the mutation that falsifies the queue rail then falsifies nothing. A
# regression in ShouldQueue does the same thing with them in place.
state = row.get("state")
# An unreadable state DENIES. The vocabulary is `PeerState.String()`
# (internal/component/plugin/types_bgp.go); anything outside it means the row
# was not the row this guard thinks it read, and "not established" would then be
# true for the wrong reason.
if state not in ("stopped", "connecting", "active", "established", "idle-hold"):
    runtime_fail(
        "peer %s reports state=%r, which is not a peer state: the queue-rail "
        "guard cannot tell whether the session is up" % (BIRD_ADDR, state)
    )
if state == "established":
    sent = row.get("eor-sent")
    try:
        sent = int(sent)
    except (TypeError, ValueError):
        runtime_fail(
            "peer %s reports eor-sent=%r, which is not a count: the queue-rail "
            "guard cannot tell whether the initial sync has drained" % (BIRD_ADDR, sent)
        )
    if sent != 0:
        runtime_fail(
            "peer %s has already sent %d initial-sync EOR(s) before the queue "
            "announce: the initial sync has drained and %s would take the "
            "BATCH rail, leaving the queue rail untested"
            % (BIRD_ADDR, sent, QUEUE_PREFIX)
        )

flush("peer * update text %s nlri ipv4/unicast add %s\n" % (ATTRS, QUEUE_PREFIX))

# Rail 2, the BATCH rail. `eor-sent` counts End-of-RIB markers that reached the
# socket, and sendInitialRoutes (peer_initial_sync.go) increments it after the
# drain, so this barrier says the session is established AND the queue is behind
# us.
if not api.wait_peer_eor_sent("peer1", attempts=EOR_ATTEMPTS, delay=EOR_DELAY):
    runtime_fail("peer1 initial-sync EOR never reached the wire")

# quiesce is the ONLY guarantee of the batch rail, so its verdict is asserted.
# The barrier above does NOT imply ShouldQueue is false: inside
# `(*Peer).sendInitialRoutes`
# (internal/component/bgp/reactor/peer_initial_sync.go) the `IncrEORSent` that
# raises eor-sent runs in the initial-sync EOR loop, and
# `sendingInitialRoutes.Store(0)` runs at the very end of the function, after
# the post-EOR drain. The bgp-peer-sync quiescer waits for
# `!PendingSync()` (internal/component/bgp/reactor/peer.go), which is
# `sendingInitialRoutes == 0` AND an empty opQueue -- exactly the remaining half
# of ShouldQueue on an established peer. `API.quiesce`
# (test/scripts/ze_api.py) returns False on a RuntimeError or on any status
# other than "done", and a discarded False puts BATCH_PREFIX on the queue rail
# and makes the nil-base mutation inert. `API.assert_no_leak` in the same file
# fails the same way.
if not api.quiesce():
    runtime_fail(
        "quiesce barrier did not settle: the initial sync can still own peer1, "
        "and %s would take the QUEUE rail" % BATCH_PREFIX
    )

flush("peer * update text %s nlri ipv4/unicast add %s\n" % (ATTRS, BATCH_PREFIX))

# Stay alive while the check reads BIRD's table.
wait_for_shutdown(timeout=120)
