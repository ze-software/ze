#!/usr/bin/env python3
"""Scenario 61: the configured MULTI_EXIT_DISC removal, judged by GoBGP.

A raw injector EXTERNAL to Ze (65005 against Ze's 65001) announces two prefixes,
both carrying ORIGIN, AS_PATH [65005], NEXT_HOP 172.30.0.9 and MULTI_EXIT_DISC
100. Only 10.61.0.0/24 also carries COMMUNITY 65005:1. Ze's import chain on that
session runs `modify DROP-MED`, whose set block is `med-remove true` and whose
match container names that community. Ze relays both routes to GoBGP, which is
an INTERNAL peer.

THE DESTINATION IS INTERNAL BECAUSE THAT IS WHAT MAKES THE ABSENCE MEAN
SOMETHING. Ze's automatic Section 5.1.4 strip fires toward a different
neighboring AS only, so on an iBGP session nothing removes attribute 4 unless an
operator asked for it. Scenario 60 measures the automatic rule from external
destinations; this one measures the mechanism the same section separately
requires.

THE UNTAGGED PREFIX IS THE IN-RUN POSITIVE CONTROL. It travels the same session,
the same forward rail and the same policy chain, and it keeps Med 100. Without
it, "GoBGP has no Med" would be satisfied just as well by a broken relay, by a
blanket strip, or by a daemon printing no attributes at all.

The assertions:
  * GoBGP holds both prefixes                     (the relay works)
  * GoBGP shows AS_PATH 65005 and NEXT_HOP .0.9   (the source block arrived)
  * GoBGP shows no Med on 10.61.0.0/24            (Section 5.1.4's mechanism)
  * GoBGP shows Med 100 on 10.61.1.0/24           (the mechanism is a policy an
                                                   operator states, not a strip)
"""

# VALIDATES: plan/spec-rfc4271-med-across-as.md AC-5, against a real peer rather
#   than against Ze's own view of what it sent.
#
# RFC requirement: RFC4271-5.1.4-4 positive -- "A BGP speaker MUST implement a
#   mechanism (based on local configuration) that allows the MULTI_EXIT_DISC
#   attribute to be removed from a route" (Section 5.1.4). The route the
#   operator's policy matches reaches AS 65001's other speaker with no metric.
# RFC requirement: RFC4271-5.1.4-4 negative -- the mechanism is what an operator
#   ASKS for and never a default. The route the same policy does not match keeps
#   its metric, judged by the same daemon in the same run.
#
# PREVENTS: the gap RFC4271-5.1.4-4 recorded. Ze removed MULTI_EXIT_DISC on one
#   condition it derived itself and on no condition an operator could state, so
#   a speaker that had to drop a neighbor's metric from the route itself -- and
#   not only from sessions toward other ASes -- had no way to say so.
#
# DISCRIMINATION: removing the ExtractMEDRemoveOps call from
#   runIngressPolicyChain (reactor/filter_ordered.go) turns this scenario RED on
#   the tagged prefix alone: GoBGP then prints Med: 100 for both, while the
#   route-present, AS_PATH, NEXT_HOP and untagged-metric assertions stay GREEN.

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    INJECT_CONTAINER,
    GoBGP,
    ZE_CONTAINER,
    ZE_IP,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)

# Tagged 65005:1, so the operator's policy removes its metric.
REMOVED = "10.61.0.0/24"

# Untagged, so the same policy leaves it alone.
KEPT = "10.61.1.0/24"

# What the injector puts on the wire for both.
INJECTED_MED = 100


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
        log_fail("GoBGP has no path for %s, so the MED check is vacuous" % prefix)
        raise AssertionError("GoBGP missing route %s" % prefix)
    return output


def _check_source_block_arrived(output, prefix):
    """The route GoBGP holds is the injected one, relayed.

    Without this, a Ze that rebuilt the route from something other than the
    received block would satisfy the absence check below for a reason that has
    nothing to do with the configured removal.
    """
    if "65005" not in output:
        log_fail("GoBGP route %s has no 65005 in its AS_PATH: %s" % (prefix, output))
        raise AssertionError("relayed AS_PATH is not the injected one")
    if "172.30.0.9" not in output:
        log_fail("GoBGP route %s has the wrong next-hop: %s" % (prefix, output))
        raise AssertionError("relayed NEXT_HOP is not the injected one")
    log_pass("GoBGP route %s carries the injected AS_PATH and NEXT_HOP" % prefix)


def _check_removed(output, prefix):
    """The mechanism: the operator asked for the metric to go, and it is gone."""
    if "Med:" in output:
        log_fail("GoBGP route %s still carries a Med: %s" % (prefix, output))
        raise AssertionError("the configured MULTI_EXIT_DISC removal did not happen")
    log_pass(
        "GoBGP route %s carries no MULTI_EXIT_DISC: the configured mechanism ran "
        "(RFC 4271 Section 5.1.4)" % prefix
    )


def _check_kept(output, prefix):
    """The control: a route the policy does not match keeps its metric.

    This is the assertion a blanket strip fails, and it is why the absence above
    is a mechanism an operator states rather than a rule Ze applies anyway.
    """
    if "Med: %d" % INJECTED_MED not in output:
        log_fail(
            "GoBGP route %s does not carry the injected Med %d: %s"
            % (prefix, INJECTED_MED, output)
        )
        raise AssertionError("a route outside the policy lost its MULTI_EXIT_DISC")
    log_pass(
        "GoBGP route %s still carries Med %d: the mechanism is opt-in"
        % (prefix, INJECTED_MED)
    )


def _check():
    gobgp = GoBGP()
    gobgp.wait_session(ZE_IP)

    for prefix in (REMOVED, KEPT):
        log_info("waiting for %s at GoBGP..." % prefix)
        _wait_route(gobgp, prefix)
        gobgp.check_route(prefix)

    removed = _gobgp_rib(gobgp, REMOVED)
    _check_source_block_arrived(removed, REMOVED)
    _check_removed(removed, REMOVED)

    kept = _gobgp_rib(gobgp, KEPT)
    _check_source_block_arrived(kept, KEPT)
    _check_kept(kept, KEPT)

    log_pass(
        "the configured removal took the metric off the route it named, and only that one"
    )


def check():
    """Entry point the runner calls. A silent injector looks like a working
    removal, so every sidecar log is dumped when an assertion fails."""
    try:
        _check()
    except Exception:
        gobgp = GoBGP()
        print("--- GoBGP rib ---")
        print(gobgp._gobgp_quiet(["global", "rib", "-a", "ipv4"]))
        print(docker_logs(INJECT_CONTAINER, 40))
        print(docker_logs(ZE_CONTAINER, 60))
        raise
