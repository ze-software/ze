#!/usr/bin/env python3
"""Ze generates its own Path Identifier for every route it re-advertises.

BIRD (AS 65003) and GoBGP (AS 65004) announce 10.99.0.0/24 to Ze. Neither
negotiated ADD-PATH, so both paths arrive under Path Identifier 0. FRR (AS 65002)
negotiated ADD-PATH, so Ze re-advertises both paths to it. FRR holds the RIB that
answers whether it kept them.
"""

# RFC requirement: RFC7911-2-2 positive -- a BGP speaker that re-advertises a route
# generates its own Path Identifier and does not relay the received one. Asserted at
# FRR, a foreign daemon holding the RIB Ze filled, because the loss this requirement
# prevents happens at a RECEIVER: RFC 7911 Section 5 keys replacement on (prefix,
# Path Identifier), so two paths sharing one identifier collapse into one route, and
# nothing in Ze's own view of what it sent would show it.
#
# Discrimination, measured 2026-08-14 against a real lab: with the generator returning
# the received identifier, both paths reach FRR as (10.99.0.0/24, 0) and FRR holds ONE
# path, the AS 65003 path replaced by the AS 65004 one. With the generator in place FRR
# holds two, under identifiers 0 and 1.

import json
import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    FRR,
    GoBGP,
    ZE_IP,
    docker_exec_quiet,
    log_fail,
    log_info,
    log_pass,
)

PREFIX = "10.99.0.0/24"


def paths_at_frr(frr):
    """Return FRR's path list for PREFIX, or [] when the prefix is absent.

    `show bgp ipv4 unicast detail json` is the only form that carries the received
    Path Identifier. The per-prefix forms (`... <prefix> json`, `... <prefix> detail
    json`) return the summary shape, which has no addpath field at all, so a check
    written against them reads two paths and can say nothing about their
    identifiers. Measured against FRR 10.3.1.
    """
    output = docker_exec_quiet(
        frr.container, ["vtysh", "-c", "show bgp ipv4 unicast detail json"]
    )
    if not output.strip():
        return []
    try:
        data = json.loads(output)
    except json.JSONDecodeError:
        return []
    return data.get("routes", {}).get(PREFIX, [])


def path_identifiers(paths):
    """Map each path's AS_PATH to the Path Identifier FRR received for it.

    FRR omits `addpathRxId` from its JSON when the identifier is 0, so an absent key
    IS the value 0 (RFC 7911 Section 3 makes 0 legal, and this session negotiated
    ADD-PATH, so every NLRI on it carries an identifier). Reading the absence as
    "no identifier" would hide the collision this scenario measures: with the
    received identifier relayed, both paths arrive under 0 and neither carries the
    key.
    """
    ids = {}
    for path in paths:
        aspath = path.get("aspath", {})
        origin = aspath.get("string", "") if isinstance(aspath, dict) else str(aspath)
        ids[origin] = path.get("addpathRxId", 0)
    return ids


def established_epoch(frr):
    """Return the epoch second at which FRR's session with Ze last came up.

    This is what a session reset is watched by, rather than the route going away:
    Ze reconnects in about a second, so polling for the absence of the prefix loses
    the race and reports a reset that plainly happened as one that never did. A
    changed epoch says the paths read after it arrived over a NEW session, which is
    the peer-up replay this scenario is aiming at.
    """
    output = docker_exec_quiet(
        frr.container, ["vtysh", "-c", "show bgp neighbor %s json" % ZE_IP]
    )
    if not output.strip():
        return 0
    try:
        data = json.loads(output)
    except json.JSONDecodeError:
        return 0
    return data.get(ZE_IP, {}).get("bgpTimerUpEstablishedEpoch", 0)


def wait_reestablished(frr, before, timeout=90):
    """Poll until FRR reports a session that came up later than `before`."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        now = established_epoch(frr)
        if now and now != before:
            log_pass("FRR re-established with Ze (new session at epoch %d)" % now)
            return
        time.sleep(2)
    log_fail("FRR did not re-establish with Ze within %ds" % timeout)
    raise AssertionError("the session reset did not produce a new session")


def wait_paths(frr, want, timeout=60):
    """Poll until FRR holds `want` paths for PREFIX. Returns the path list."""
    deadline = time.time() + timeout
    paths = []
    while time.time() < deadline:
        paths = paths_at_frr(frr)
        if len(paths) >= want:
            return paths
        time.sleep(2)
    return paths


def report(frr, message):
    """Fail with FRR's own view of the prefix attached."""
    log_fail(message)
    print(
        # fail-open-ok: diagnostic print on an already-failed run
        docker_exec_quiet(
            frr.container, ["vtysh", "-c", "show bgp ipv4 unicast %s" % PREFIX]
        )
    )
    raise AssertionError(message)


def check():
    frr = FRR()
    gobgp = GoBGP()

    frr.wait_session(ZE_IP)
    gobgp.wait_session(ZE_IP)

    # ADD-PATH must be negotiated on the Ze -> FRR session. Without it FRR keeps one
    # path for a reason that has nothing to do with the Path Identifier, and every
    # assertion below reads a table whose shape the capability decided.
    # An empty answer carries no capability token either, so a failed query raises
    # here exactly as a missing capability does.
    # fail-open-ok: an empty answer fails the capability test below
    neighbor = docker_exec_quiet(
        frr.container, ["vtysh", "-c", "show bgp neighbor %s json" % ZE_IP]
    )
    if '"rxAdvertisedAndReceived":true' not in neighbor.replace(" ", ""):
        log_fail("ADD-PATH receive was not negotiated between Ze and FRR")
        for line in neighbor.splitlines()[:60]:
            print("  %s" % line)
        raise AssertionError("ADD-PATH not negotiated")
    log_pass("ADD-PATH receive negotiated between Ze and FRR")

    # BIRD announces at session up. GoBGP announces here, so the two paths reach FRR
    # by different rails: BIRD's through the peer-up replay Ze runs for a client that
    # joined after the route was stored, GoBGP's through the live forward.
    log_info("injecting the second path for %s from GoBGP..." % PREFIX)
    gobgp.inject_route(PREFIX)

    paths = wait_paths(frr, 2)
    if len(paths) < 2:
        report(
            frr,
            "FRR holds %d path(s) for %s, expected 2 (one via AS 65003, one via AS "
            "65004): a re-advertised path was lost at the receiver"
            % (len(paths), PREFIX),
        )

    live = path_identifiers(paths)
    if set(live) != {"65003", "65004"}:
        report(
            frr,
            "FRR holds paths from %s, expected one from AS 65003 and one from AS "
            "65004" % sorted(live),
        )
    if len(set(live.values())) != 2:
        report(
            frr,
            "Ze re-advertised both paths for %s under one Path Identifier: %s"
            % (PREFIX, live),
        )
    log_pass(
        "FRR kept both paths for %s, each under its own Path Identifier: %s"
        % (PREFIX, live)
    )

    # The identifier belongs to the path, not to the delivery. A session reset makes
    # Ze replay both paths from store, which is the other forward rail. The
    # identifiers must come back unchanged, or a client that reconnects sees every
    # path it already knew renumbered, and the (prefix, identifier) state it holds
    # is wrong from then on.
    log_info("clearing the FRR session to force Ze's peer-up replay...")
    before = established_epoch(frr)
    docker_exec_quiet(frr.container, ["vtysh", "-c", "clear bgp %s" % ZE_IP])
    wait_reestablished(frr, before)

    replayed = wait_paths(frr, 2)
    if len(replayed) < 2:
        report(
            frr,
            "after the session reset FRR holds %d path(s) for %s, expected 2: the "
            "peer-up replay lost a path" % (len(replayed), PREFIX),
        )
    after = path_identifiers(replayed)
    if after != live:
        report(
            frr,
            "the replayed Path Identifiers differ from the live ones (live %s, "
            "replayed %s): the two forward rails disagree" % (live, after),
        )
    log_pass("the peer-up replay carries the same Path Identifiers: %s" % after)

    assert frr.session_established(ZE_IP), "FRR session dropped"
    assert gobgp.session_established(ZE_IP), "GoBGP session dropped"
    log_pass("both sessions stable after the exchange")
