#!/usr/bin/env python3
"""Scenario 54: LOCAL_PREF does not cross to an external peer (RFC 4271 §5.1.5).

A raw injector INTERNAL to Ze (65001 on both sides) announces three prefixes,
each carrying ORIGIN, AS_PATH [65004], NEXT_HOP 172.30.0.9 and LOCAL_PREF 200.
Ze relays all three to two EXTERNAL daemons: FRR (65002) and GoBGP (65003).

GOBGP IS THE WITNESS, AND FRR CANNOT BE. Section 5.1.5 has a receive half as
well as a send half: "If it is contained in an UPDATE message that is received
from an external peer, then this attribute MUST be ignored" (RFC4271-5.1.5-3).
FRR implements that by skipping the attribute during parse, so a leaked
LOCAL_PREF is invisible in FRR's RIB, in its JSON and in its debug log. Measured
on 2026-08-04 with the strip removed from Ze: FRR showed nothing at all while
GoBGP showed `{LocalPref: 200}` on the same route from the same UPDATE. An
absence assertion read off FRR is green either way, which is the vacuity trap
this scenario was rewritten to escape. GoBGP keeps the received attribute list
on the path and prints it, so it reports what ARRIVED rather than what it
decided to keep.

FRR is still in the lab and has the other half of the job: a conformant peer
ACCEPTS the stripped UPDATE, sends no NOTIFICATION, and installs the route.

The assertions:
  * FRR and GoBGP both hold all three prefixes   (the relay works, both peers)
  * GoBGP shows AS_PATH 65004 and NEXT_HOP .0.9  (the source block arrived)
  * GoBGP shows no LocalPref at all              (the strip happened)

The first two are what stop the third from being vacuous: "GoBGP has no
LOCAL_PREF" is satisfied just as well by "GoBGP has nothing at all".

Ze runs with NO rib plugin, and the injector announces nothing until both peers
are established, so every prefix reaches them through the live forward rail
(forwardUpdateCore, reactor/reactor_api_forward.go). A rib plugin would replay
stored routes to a late peer through peer_rib_routes.go, a different rail that
has stripped LOCAL_PREF correctly all along, and the scenario would then assert
against the code that was already right.
"""

# VALIDATES: plan/spec-fixit-send-community-suppress-ignored.md AC-13, AC-14,
#   against real peers rather than against Ze's own view of what it sent.
#
# RFC requirement: RFC4271-5.1.5-2 positive -- "A BGP speaker MUST NOT include
#   this attribute in UPDATE messages it sends to external peers, except in the
#   case of BGP Confederations [RFC3065]." Ze has no confederation configuration
#   surface, so the exception cannot apply to either session here.
#
# PREVENTS: the live MUST NOT violation the spec fixed. Both forward rails
#   relayed the source attribute block verbatim for code 5, so a route LEARNED
#   from an internal peer and RELAYED to an external one carried the internal
#   preference across the AS boundary -- while the SAME prefix originated locally
#   did not, because buildAnnounceUpdate (reactor/reactor_api_batch.go) already
#   stripped it. applyFactsLocalPref (reactor/forward_local_pref.go) is the fix.
#
# DISCRIMINATION: removing the mods.Op(5, AttrModSuppress, nil) line from
#   applyFactsLocalPref turns this scenario RED. Measured on 2026-08-04: GoBGP
#   then prints `[{Origin: i} {LocalPref: 200}]` for the relayed prefixes, while
#   the route-present, AS_PATH and NEXT_HOP assertions stay GREEN, so the RED
#   comes from the LOCAL_PREF assertion alone.

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    FRR,
    FRR_CONTAINER,
    INJECT_CONTAINER,
    GoBGP,
    ZE_CONTAINER,
    ZE_IP,
    docker_exec_quiet,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)

PREFIXES = ("10.54.0.0/24", "10.54.1.0/24", "10.54.2.0/24")

# The injected value. Anything else at a destination is that daemon's own
# default rather than the wire.
INJECTED_LOCAL_PREF = 200


def _wait_route(daemon, prefix, timeout=120):
    deadline = time.time() + timeout
    while time.time() < deadline:
        if daemon.has_route(prefix):
            return
        time.sleep(2)


def _gobgp_rib(gobgp, prefix):
    """Return GoBGP's RIB rendering for a prefix, and fail if it is absent.

    The text form is read rather than the JSON: `gobgp global rib <prefix>`
    prints the received attribute list verbatim in its Attrs column, which is
    exactly the question this scenario asks.
    """
    output = gobgp._gobgp_quiet(["global", "rib", "-a", "ipv4", prefix])
    if not output.strip() or "not in table" in output:
        log_fail(
            "GoBGP has no path for %s, so the LOCAL_PREF check is vacuous" % prefix
        )
        raise AssertionError("GoBGP missing route %s" % prefix)
    return output


def _check_source_block_arrived(output, prefix):
    """The positive half: the relayed route is the injected one.

    Without this, a Ze that forwarded nothing, or that rebuilt the route from
    something other than the received block, would satisfy the absence check
    below for a reason that has nothing to do with Section 5.1.5.
    """
    if "65004" not in output:
        log_fail("GoBGP route %s has no 65004 in its AS_PATH: %s" % (prefix, output))
        raise AssertionError("relayed AS_PATH is not the injected one")
    if "172.30.0.9" not in output:
        log_fail("GoBGP route %s has the wrong next-hop: %s" % (prefix, output))
        raise AssertionError("relayed NEXT_HOP is not the injected one")
    log_pass("GoBGP route %s carries the injected AS_PATH and NEXT_HOP" % prefix)


def _check_no_local_pref(output, prefix):
    """The negative half: RFC 4271 Section 5.1.5 on an external session."""
    if "LocalPref" in output:
        log_fail("GoBGP route %s still carries a LocalPref: %s" % (prefix, output))
        raise AssertionError("LOCAL_PREF crossed to an external peer")
    log_pass("GoBGP route %s carries no LOCAL_PREF (RFC 4271 Section 5.1.5)" % prefix)


def _check():
    frr = FRR()
    gobgp = GoBGP()

    frr.wait_session(ZE_IP)
    gobgp.wait_session(ZE_IP)

    for prefix in PREFIXES:
        log_info("waiting for %s at FRR and GoBGP..." % prefix)
        _wait_route(frr, prefix)
        _wait_route(gobgp, prefix)
        # FRR's half of the job: a conformant peer accepts the stripped UPDATE
        # and installs the route. It cannot see the attribute either way
        # (RFC4271-5.1.5-3), which is why GoBGP carries the absence assertion.
        frr.check_route(prefix)
        gobgp.check_route(prefix)

    for prefix in PREFIXES:
        output = _gobgp_rib(gobgp, prefix)
        _check_source_block_arrived(output, prefix)
        _check_no_local_pref(output, prefix)

    assert frr.session_established(ZE_IP), "FRR session dropped"
    log_pass("no LOCAL_PREF reached an external peer, and every route did")


def check():
    """Entry point the runner calls. A silent injector looks like a working
    strip, so every sidecar log is dumped when an assertion fails."""
    try:
        _check()
    except Exception:
        frr = FRR()
        gobgp = GoBGP()
        print("--- GoBGP rib ---")
        print(gobgp._gobgp_quiet(["global", "rib", "-a", "ipv4"]))
        print("--- FRR bgp table ---")
        print(frr._vtysh_quiet("show bgp ipv4 unicast"))
        for prefix in PREFIXES:
            print("--- FRR %s ---" % prefix)
            print(frr._vtysh_quiet("show bgp ipv4 unicast %s" % prefix))
        print("--- FRR log ---")
        print(docker_exec_quiet(FRR_CONTAINER, ["sh", "-c", "tail -60 /tmp/frr.log"]))
        print(docker_logs(INJECT_CONTAINER, 40))
        print(docker_logs(ZE_CONTAINER, 60))
        raise
