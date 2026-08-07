#!/usr/bin/env python3
"""Scenario 55: BIRD accepts API-originated routes on BOTH announce rails.

A process plugin originates one prefix per rail through the `ze_api` rail. Each
carries COMMUNITIES (65001:100, 65001:200) and LARGE_COMMUNITIES (65001:0:1).
The announce rail adds ORIGIN, AS_PATH and NEXT_HOP, and `(*announceAttrs).emit`
(internal/component/bgp/reactor/announce_build.go) writes all five in ascending
type-code order. BIRD 2.15.1 must install both prefixes and report both community
values on each.

Every value assertion is a POSITIVE one over an exact value, taken from the
attribute LINE rather than from the whole dump. An absence assertion reads green
whether or not the behavior holds, because a receiving daemon that skips an
attribute during parse shows nothing either way (scenario 54 measured this).

The two community types print in DIFFERENT punctuation, so one literal form
cannot serve both:

    BGP.community:       (65001,100) (65001,200)     no space after the comma
    BGP.large_community: (65001, 0, 1)               a space after each comma

A large-community literal written `(65001,0,1)` never matches, and the check
would be silently vacuous. Each type is asserted in its own form.

Which rail each prefix takes is CONFIGURED, not raced: ze is passive
(`connect false`, ze.conf) and BIRD holds its dial for 30 seconds (`connect
delay time 30`, bird.conf), so the queue announce always lands with the session
down. The plugin asserts the rail as well, and this file reports its verdict
through `raise_if_observer_failed`.
"""

# VALIDATES: plan/spec-wire-edit-4-api-origin-deferred-bird-interop.md AC-1, AC-2
#   and AC-4, against a live peer rather than against Ze's own view of what it
#   sent. `plan/learned/1320-wire-edit-4-api-origin.md` converged the two announce
#   rails on one writer, `(*announceAttrs).emit`
#   (internal/component/bgp/reactor/announce_build.go), and its interop row was
#   owed.
# PREVENTS: an API-originated route whose caller attributes are lost or corrupted
#   when the rail merge-inserts its own. A peer would receive a route with no
#   community, and the ORDER test at test/plugin/wire-edit-api-origin-order.ci
#   would stay green because it pins Ze's own bytes, not a peer's verdict.
#
# NOT PROVEN HERE: attribute ORDER on the wire. BIRD accepts any order and
#   `birdc show route ... all` prints in BIRD's own canonical order, so reverting
#   the encoder leaves the dump identical. Order stays proven at the byte level by
#   test/plugin/wire-edit-api-origin-order.ci, and the live-peer half is homed at
#   plan/spec-interop-wire-capture.md.
#
# DISCRIMINATION: measured on 2026-08-05, in the containerised run, with the ze
#   image rebuilt for each mutation. The two prefixes take DIFFERENT rails and
#   each rail is falsified by a DIFFERENT mutation of `(*announceAttrs).emit`.
#   Neither mutation alone falsifies both, which is why the scenario carries two
#   prefixes.
#
#   The `ze-interop` tag is shared and carries no per-run suffix, so a build in
#   another session can swap the daemon under a run
#   (plan/spec-interop-image-tag-race.md). Every result below therefore names the
#   image it ran against, read with `docker images -q ze-interop`:
#   unmutated 286e880f17c6 and 5d0ceb8477c1 PASS, mutation A eba201f905aa,
#   mutation B bbbda3ea97cb.
#
#     nil `base` on entry to emit          -> 10.55.1.0/24 loses both community
#                                             lines. The batch rail
#                                             (buildBatchAnnounceUpdate) passes
#                                             the caller's block as the base:
#                                             `base=[8 32]`, `plan=[1 2 3]`.
#     keep only plan entries with code < 8 -> 10.55.0.0/24 loses both community
#                                             lines. The queue rail
#                                             (buildRIBRouteUpdate) passes base
#                                             nil and replays the caller's
#                                             attributes as contributions:
#                                             `base=nil`, `plan=[1 2 3 8 32]`.
#
#   Each mutation leaves the OTHER prefix intact, and leaves the session and the
#   route assertions green: the RED comes from the community assertions alone.
#   An earlier revision of this file recorded the second mutation as the only one
#   that works and BANNED the first. That was measured on a one-prefix scenario
#   that reached the queue rail only, so the ban generalised a rail-specific
#   result. Which rail runs is decided by `Peer.ShouldQueue`
#   (internal/component/bgp/reactor/peer.go), never by the command text.

import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    BIRD,
    BIRD_CONTAINER,
    SESSION_TIMEOUT,
    ZE_CONTAINER,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
    raise_if_observer_failed,
)

# Prefixes no other scenario announces, so a pass cannot be inherited from
# another scenario's leftover state (AC-4). QUEUE_PREFIX is announced before the
# session establishes and reaches BIRD through the initial-sync drain;
# BATCH_PREFIX is announced after the drain and reaches BIRD through the
# established-peer rail.
QUEUE_PREFIX = "10.55.0.0/24"
BATCH_PREFIX = "10.55.1.0/24"

# BIRD prints a standard community with NO space after the comma.
STANDARD_COMMUNITIES = ("(65001,100)", "(65001,200)")

# BIRD prints a large community with a space after EACH comma. Measured on BIRD
# 2.15.1 (test/interop/Dockerfile.bird), 2026-08-05. The no-space form
# `(65001,0,1)` was run once against this scenario and failed, which is what
# proves this literal reads real output.
LARGE_COMMUNITY = "(65001, 0, 1)"

# BIRD writes this trace line once per establishment when the protocol carries
# `debug { states }` (bird.conf).
UP_TRACE = "ze_peer: State changed to up"

# All three plugin failure modes -- queue race lost, EOR never sent, quiesce
# failed -- reach ze's log as the ZE-OBSERVER-FAIL sentinel and nothing else.
# `raise_if_observer_failed` (test/interop/interop.py) reports the plugin's own
# message; without it each mode surfaces as `BIRD route 10.55.0.0/24 not found`,
# which names the symptom two hops from its cause. The harness calls the same
# helper when ze dies during setup, which is where the fastest of the three
# lands.


# The establishment barrier is CONFIGURED in bird.conf and the budget for it is
# `SESSION_TIMEOUT`, an environment knob (test/interop/interop.py). bird.conf is
# not templated, so it cannot read that knob, and the relation between the two
# was left implied until the round 5 fix pass. It is checked here instead.
#
# The margin covers the TCP connect plus the OPEN/KEEPALIVE exchange after the
# delay expires. It is deliberately small: the delay itself dominates.
FLOOR_MARGIN = 10


def _connect_delay():
    """Read `connect delay time N` out of this scenario's own bird.conf.

    Read rather than repeated, so the floor below cannot drift from the config
    it is a floor for. `_render_scenario_dir` (test/interop/interop.py) copies
    bird.conf next to this file, so `__file__` finds the config the run is
    actually using. A conf that does not state the delay fails closed: an
    unparsed barrier is not a barrier of zero.
    """
    path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "bird.conf")
    with open(path, "r", encoding="utf-8") as fh:
        match = re.search(r"^\s*connect delay time\s+(\d+)\s*;", fh.read(), re.M)
    if match is None:
        log_fail("bird.conf states no `connect delay time`: %s" % path)
        raise AssertionError(
            "the establishment barrier this scenario depends on is not in bird.conf"
        )
    return int(match.group(1))


def _check_session_budget():
    """Fail early, and name the floor, when SESSION_TIMEOUT is under it.

    `BIRD.wait_session` spends SESSION_TIMEOUT waiting for a session that BIRD
    holds for `connect delay time` seconds first, and ze never dials (`connect
    false`, ze.conf). Under the floor the run fails 100% of the time with `BIRD
    protocol ze_peer not Established`, which names no cause and reads as a
    broken daemon: measured at SESSION_TIMEOUT=20 on 2026-08-05.

    A scenario with an undocumented floor on a documented knob is a trap for
    whoever tunes the knob, so the floor is stated and checked rather than
    implied.
    """
    delay = _connect_delay()
    floor = delay + FLOOR_MARGIN
    if SESSION_TIMEOUT < floor:
        log_fail(
            "SESSION_TIMEOUT=%ds is below this scenario's floor of %ds"
            % (SESSION_TIMEOUT, floor)
        )
        raise AssertionError(
            "SESSION_TIMEOUT=%ds cannot cover this scenario: BIRD holds its dial "
            "for %ds (`connect delay time`, bird.conf) and ze never dials "
            "(`connect false`, ze.conf), so the session cannot establish inside "
            "the budget. Raise SESSION_TIMEOUT to at least %ds, or lower the "
            "delay -- but the delay is what puts the first announce on the queue "
            "rail, so lowering it costs the rail coverage"
            % (SESSION_TIMEOUT, delay, floor)
        )
    log_pass(
        "SESSION_TIMEOUT=%ds clears the %ds floor (connect delay %ds + %ds)"
        % (SESSION_TIMEOUT, floor, delay, FLOOR_MARGIN)
    )


def _route_dump(bird, prefix):
    """Return BIRD's full route dump for prefix, and fail if it is empty."""
    output = bird._birdc_quiet("show route for %s all" % prefix)
    if prefix not in output:
        log_fail(
            "BIRD route dump for %s is empty, so every value check is vacuous" % prefix
        )
        print(output)
        raise AssertionError("BIRD has no dump for %s" % prefix)
    return output


def _attr_line(output, prefix, attribute):
    """Return the FIRST `BGP.<attribute>:` line of a route dump.

    Scoped on purpose. Matching a community value against the WHOLE dump lets any
    other line BIRD happens to print satisfy the assertion for a route that never
    carried the attribute.

    First, not only: a dump holding two paths for one prefix carries two such
    lines, and this reads path 1 and ignores path 2. The scenario announces each
    prefix once, from one peer, so one path is what BIRD holds. A missing line
    fails closed here rather than returning an empty string, which is what keeps
    the caller's value check from passing on nothing.
    """
    wanted = "BGP.%s:" % attribute
    for line in output.splitlines():
        if line.strip().startswith(wanted):
            return line
    log_fail("BIRD route %s has no %s line" % (prefix, wanted))
    print(output)
    raise AssertionError("no %s line for %s" % (wanted, prefix))


def _check_communities(bird, prefix):
    """Both types must be present on prefix, each in its own punctuation."""
    output = _route_dump(bird, prefix)

    standard = _attr_line(output, prefix, "community")
    for value in STANDARD_COMMUNITIES:
        if value not in standard:
            log_fail("BIRD route %s is missing standard community %s" % (prefix, value))
            print(output)
            raise AssertionError(
                "standard community %s not delivered for %s" % (value, prefix)
            )
    log_pass("BIRD route %s carries both standard communities" % prefix)

    large = _attr_line(output, prefix, "large_community")
    if LARGE_COMMUNITY not in large:
        log_fail(
            "BIRD route %s is missing large community %s" % (prefix, LARGE_COMMUNITY)
        )
        print(output)
        raise AssertionError(
            "large community %s not delivered for %s"
            % (
                LARGE_COMMUNITY,
                prefix,
            )
        )
    log_pass("BIRD route %s carries the large community" % prefix)


def _check_session_never_bounced():
    """AC-2: BIRD reached Established exactly once over the whole run.

    A NOTIFICATION over the attribute block would tear the session down; ze would
    re-announce on the next one and every route assertion would pass again. The
    signal has to be one a bounce cannot erase, so it is BIRD's own log rather
    than its current state:

      * `bird.session_established` samples the CURRENT state, which is Established
        again after a bounce.
      * `Last error` in `show protocols all` is cleared when the session
        re-establishes (measured on BIRD 2.15.1, 2026-08-05).
      * BIRD's stderr is append-only, so a second `State changed to up` cannot be
        taken back.

    `docker_logs` (test/interop/interop.py) runs `docker logs --tail N`, so this
    reads the LAST 2000 lines rather than the whole log. BIRD writes about five
    lines over a scenario run, so the tail holds every one of them. A scenario
    that made BIRD chatty would have to raise the bound.

    Read with `strict=True`, which is the DECISION contract: this counts lines
    to decide. The default contract answers a docker failure with docker's own
    error text and a timeout with "(docker logs timed out)". Neither holds the
    trace line, so the count is 0, which this would report as "the peer did not
    hold one session": a red naming a cause it did not establish. The direction
    is a red either way, so an unreadable log cannot turn the run green, but the
    cause would be wrong (round 6 review).

    Exactly one, not at-least-one: zero means the trace is not being written and
    the assertion would be vacuous.

    Falsified on 2026-08-05: a `birdc restart ze_peer` inserted before this call
    turned it RED with `count=2`, while `show protocols` still read Established
    and the old `session_established` assertion still passed. That run is what
    makes this a replacement rather than a rewording.
    """
    logs = docker_logs(BIRD_CONTAINER, 2000, strict=True)
    count = logs.count(UP_TRACE)
    if count != 1:
        log_fail("BIRD logged %d `%s` lines, expected exactly 1" % (count, UP_TRACE))
        print(logs)
        raise AssertionError(
            "BIRD session came up %d times: the peer did not hold one session "
            "across the announces" % count
        )
    log_pass("BIRD held ONE session across both announces (no NOTIFICATION)")


def _check():
    bird = BIRD()

    # Before anything is waited on: a budget under the floor cannot produce a
    # verdict about ze, only a timeout that looks like one.
    _check_session_budget()

    # The queue-rail guard fires within a second of ze starting, long before the
    # session can come up, so the fast failure is named here rather than waited
    # out at `wait_session`.
    raise_if_observer_failed("before waiting for the session")

    try:
        bird.wait_session("ze_peer")

        # `BIRD.wait_route` (test/interop/interop.py) returns only when the route
        # is present and raises otherwise, so it IS the AC-4 assertion. A
        # `check_route` call after it re-reads the same table and prints a green
        # tick that reads as independent verification.
        log_info("waiting for the queue-rail route %s at BIRD..." % QUEUE_PREFIX)
        bird.wait_route(QUEUE_PREFIX)
        log_pass("BIRD installed the queue-rail route %s" % QUEUE_PREFIX)

        # This wait is a RAIL PROOF, not only a delivery check, and that makes
        # it load-bearing: BATCH_PREFIX arriving at all is positive evidence it
        # did NOT take the queue rail.
        #
        # `Peer.QueueAnnounce` (internal/component/bgp/reactor/peer.go) appends
        # to `opQueue`, and every site that drains `opQueue` lives inside
        # `(*Peer).sendInitialRoutes`
        # (internal/component/bgp/reactor/peer_initial_sync.go): the main drain
        # loop, the post-teardown clear, and the post-EOR drain. Nothing
        # re-drains it on an established peer. So a BATCH_PREFIX that reached
        # the queue rail after the sync had already drained would sit in
        # `opQueue` for the rest of the session and never reach BIRD -- and
        # `_check_session_never_bounced` below is what closes the one escape,
        # since a second establishment would run `sendInitialRoutes` again.
        #
        # The plugin's own `quiesce` barrier asserts the same property from
        # inside ze (announce-api-origin.py). This is the independent half,
        # observed from the peer.
        log_info("waiting for the batch-rail route %s at BIRD..." % BATCH_PREFIX)
        bird.wait_route(BATCH_PREFIX, timeout=60)
        log_pass("BIRD installed the batch-rail route %s" % BATCH_PREFIX)

        _check_communities(bird, QUEUE_PREFIX)
        _check_communities(bird, BATCH_PREFIX)
    except Exception:
        # The later plugin failures (EOR barrier, quiesce) land while these waits
        # are running, and every one of them reads as a missing route. Re-raise
        # with the plugin's message when the plugin is the cause; otherwise let
        # the original assertion stand.
        #
        # There is a THIRD outcome, and it must not reach the runner: the read
        # is strict, so an unreadable ze log raises here and the bare `raise`
        # below never runs, replacing the assertion that actually failed with a
        # docker error (round 6 review). An unreadable log is reported as a
        # fact, and the original failure still stands.
        try:
            raise_if_observer_failed("while checking BIRD's table")
        except (RuntimeError, OSError) as exc:
            print("--- ze log could not be read: %s ---" % exc)
        raise

    _check_session_never_bounced()
    log_pass("BIRD accepted both API-originated routes")


def check():
    """Entry point the runner calls. A silent plugin looks like a lost attribute,
    so the Ze log is dumped when an assertion fails."""
    try:
        _check()
    except Exception:
        bird = BIRD()
        print("--- BIRD protocols ---")
        print(bird._birdc_quiet("show protocols all ze_peer"))
        for prefix in (QUEUE_PREFIX, BATCH_PREFIX):
            print("--- BIRD route %s ---" % prefix)
            print(bird._birdc_quiet("show route for %s all" % prefix))
        print("--- BIRD table ---")
        print(bird._birdc_quiet("show route all"))
        print("--- BIRD log ---")
        print(docker_logs(BIRD_CONTAINER, 200))
        print(docker_logs(ZE_CONTAINER, 60))
        raise
