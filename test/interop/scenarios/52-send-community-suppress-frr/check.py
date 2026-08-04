#!/usr/bin/env python3
"""Scenario 52: `session { community { send ... } }` decides the bytes on the wire.

A raw injector announces three prefixes to Ze. Each carries a standard community
(65004:100, 65004:200), an extended community (RT:65004:300) and a large
community (65004:1:2) in ONE UPDATE. Ze relays all three prefixes to two foreign
daemons that are configured differently:

  FRR   `send none`     must install the routes and show NO community of any type
  BIRD  `send standard` must install the routes, show the standard community, and
                        show neither the extended nor the large one

Both halves are asserted, so neither is vacuous. The routes must be present, so a
peer that received nothing cannot pass. The standard community must be present at
BIRD, so a change that strips every community from every peer cannot pass either.

Two prefixes reach the destinations through Ze's replay-on-peer-up path and one
through the live forward path. Both paths end in forwardUpdateCore, which is the
function that records the suppression (reactor/peer_forward_facts.go,
applyFactsSendCommunity).
"""

# VALIDATES: plan/spec-fixit-send-community-suppress-ignored.md AC-1 and AC-2,
#   against real peers rather than against Ze's own view of what it sent.
# PREVENTS: the regression the spec fixed. filter_community's handler read its
#   operation list for AttrModSet only, so a lone AttrModSuppress left every
#   source value retained and the attribute was re-emitted intact. A peer
#   configured to receive no communities received all of them, and no test saw it
#   because the suppression was consumed in silence.
#
# DISCRIMINATION: making the Suppress branch of genericCommunityHandler
#   (plugins/filter_community/handler.go) ignore AttrModSuppress turns this
#   scenario RED. FRR then shows community 65004:100 65004:200, extended
#   community RT:65004:300 and large community 65004:1:2 on all three prefixes,
#   and BIRD shows the extended and large ones it must not have. Verified on
#   2026-08-04: the three route-present assertions stayed GREEN under the
#   mutation, so the RED came from the community assertion alone.

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    BIRD,
    FRR,
    FRR_CONTAINER,
    INJECT_CONTAINER,
    ZE_CONTAINER,
    ZE_IP,
    docker_exec_quiet,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)

PREFIXES = ("10.52.0.0/24", "10.52.1.0/24", "10.52.2.0/24")

# The three FRR JSON keys, one per community type Ze can suppress.
FRR_COMMUNITY_KEYS = ("community", "extendedCommunity", "largeCommunity")

# The same three types as they read in FRR's text output. A key check alone would
# miss a value FRR reports under a name this list does not name.
FRR_COMMUNITY_TEXT = ("65004:100", "65004:200", "65004:300", "65004:1:2")


def _wait_route(daemon, prefix, timeout=60):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if daemon.has_route(prefix):
            return
        time.sleep(2)


def _frr_paths(frr, prefix):
    """Return the JSON paths for a prefix, and fail if the route is absent."""
    data = frr.route(prefix)
    paths = data.get("paths", [])
    if not paths:
        log_fail("FRR has no path for %s, so the suppression check is vacuous" % prefix)
        raise AssertionError("FRR missing route %s" % prefix)
    return paths


def _check_frr_has_no_community(frr, prefix):
    """FRR is the `send none` peer. Every community type must be gone."""
    for path in _frr_paths(frr, prefix):
        for key in FRR_COMMUNITY_KEYS:
            if path.get(key):
                log_fail("FRR route %s still carries %s: %r" % (prefix, key, path[key]))
                raise AssertionError("send none did not suppress %s" % key)

    text = frr._vtysh_quiet("show bgp ipv4 unicast %s" % prefix)
    for value in FRR_COMMUNITY_TEXT:
        if value in text:
            log_fail("FRR route %s still shows community value %s" % (prefix, value))
            print(text)
            raise AssertionError("send none did not suppress %s" % value)
    log_pass("FRR route %s has no community of any type" % prefix)


def _check_bird_keeps_standard_only(bird, prefix):
    """BIRD is the `send standard` peer. One type stays, two go."""
    output = bird._birdc_quiet("show route for %s all" % prefix)
    lower = output.lower()

    if "(65004,100)" not in lower or "(65004,200)" not in lower:
        log_fail("BIRD route %s lost the standard community it must keep" % prefix)
        print(output)
        raise AssertionError("send standard dropped the standard community")

    for banned in ("ext_community", "large_community"):
        if banned in lower:
            log_fail("BIRD route %s still carries %s" % (prefix, banned))
            print(output)
            raise AssertionError("send standard did not suppress %s" % banned)
    log_pass("BIRD route %s keeps the standard community and no other" % prefix)


def _check():
    frr = FRR()
    bird = BIRD()

    frr.wait_session(ZE_IP)
    bird.wait_session("ze_peer")

    for prefix in PREFIXES:
        log_info("waiting for %s at FRR and BIRD..." % prefix)
        _wait_route(frr, prefix)
        _wait_route(bird, prefix)
        frr.check_route(prefix)
        bird.check_route(prefix)

    for prefix in PREFIXES:
        _check_frr_has_no_community(frr, prefix)
        _check_bird_keeps_standard_only(bird, prefix)

    assert frr.session_established(ZE_IP), "FRR session dropped"
    assert bird.session_established("ze_peer"), "BIRD session dropped"
    log_pass("send none and send standard both reached the wire")


def check():
    """Entry point the runner calls. A silent injector looks like a suppression
    bug, so both sidecar logs are dumped when an assertion fails."""
    try:
        _check()
    except Exception:
        frr = FRR()
        bird = BIRD()
        print("--- FRR bgp table ---")
        print(frr._vtysh_quiet("show bgp ipv4 unicast"))
        for prefix in PREFIXES:
            print("--- FRR %s ---" % prefix)
            print(frr._vtysh_quiet("show bgp ipv4 unicast %s" % prefix))
            print("--- BIRD %s ---" % prefix)
            print(bird._birdc_quiet("show route for %s all" % prefix))
        print("--- FRR log ---")
        print(docker_exec_quiet(FRR_CONTAINER, ["sh", "-c", "tail -60 /tmp/frr.log"]))
        print(docker_logs(INJECT_CONTAINER, 40))
        print(docker_logs(ZE_CONTAINER, 60))
        raise
