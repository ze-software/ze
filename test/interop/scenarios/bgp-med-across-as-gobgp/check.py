#!/usr/bin/env python3
"""Scenario 60: MULTI_EXIT_DISC does not cross an AS boundary, and Ze's own does.

A raw injector EXTERNAL to Ze (65004 against Ze's 65001) announces three
prefixes, each carrying ORIGIN, AS_PATH [65004], NEXT_HOP 172.30.0.9 and
MULTI_EXIT_DISC 100. Ze relays all three to two other EXTERNAL daemons: FRR
(65002) and GoBGP (65003). Ze also ORIGINATES 10.60.9.0/24 with a metric of its
own, 42, through a process plugin.

BOTH HALVES ARE ASSERTED, AND ONE WITHOUT THE OTHER PROVES NOTHING. Section
5.1.4 forbids relaying a RECEIVED metric to another neighboring AS and permits
the metric a speaker sets itself, so a daemon that stripped attribute 4 from
every external session would satisfy the first assertion and break MED as a
feature. The pair is what makes this a rule about provenance rather than a
blanket strip.

GOBGP IS THE WITNESS. It keeps the received attribute list on the path and
prints it verbatim in its Attrs column, so it reports what ARRIVED rather than
what it decided to keep. FRR is in the lab for the other half of the job: a
conformant peer accepts both UPDATEs, sends no NOTIFICATION, and installs the
routes.

The assertions:
  * FRR and GoBGP both hold all four prefixes  (the relay and the announce work)
  * GoBGP shows AS_PATH 65004 and NEXT_HOP .0.9 on the relayed three
                                                (the source block arrived)
  * GoBGP shows no Med at all on the relayed three          (Section 5.1.4)
  * GoBGP shows Med 42 on the originated one    (Section 5.1.4 governs a
                                                 RECEIVED value only)

The first two are what stop the third from being vacuous: "GoBGP has no MED" is
satisfied just as well by "GoBGP has nothing at all".
"""

# VALIDATES: plan/spec-rfc4271-med-across-as.md AC-1 and AC-2, against real peers
#   rather than against Ze's own view of what it sent.
#
# RFC requirement: RFC4271-5.1.4-1 positive -- "The MULTI_EXIT_DISC attribute
#   received from a neighboring AS MUST NOT be propagated to other neighboring
#   ASes" (Section 5.1.4). AS 65004's metric reaches neither AS 65002 nor AS
#   65003.
# RFC requirement: RFC4271-5.1.4-1 negative -- the MUST NOT covers a RECEIVED
#   value and nothing else. Ze's own metric of 42 arrives at AS 65003 intact,
#   judged by that daemon's RIB.
#
# PREVENTS: the live MUST NOT violation the spec fixed. Both forward rails
#   relayed the source attribute block verbatim for code 4, so a route learned
#   from one transit provider and readvertised to another carried the first
#   provider's metric into a network that was never meant to see it, and steered
#   that network's inbound traffic on it.
#
# DISCRIMINATION: removing the mods.Op(4, AttrModSuppress, nil) line from
#   applyFactsMED (reactor/forward_med.go) turns this scenario RED. Measured on
#   2026-08-15: GoBGP then prints {Med: 100} for the three relayed prefixes,
#   while the route-present, AS_PATH, NEXT_HOP and originated-metric assertions
#   stay GREEN, so the RED comes from the relayed-MED assertion alone.

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

# Relayed by Ze from AS 65004. Their metric must not cross.
RELAYED = ("10.60.0.0/24", "10.60.1.0/24", "10.60.2.0/24")

# Originated by Ze itself. Its metric must cross.
ORIGINATED = "10.60.9.0/24"

# The value Ze sets on the route it originates (announce.py). Anything else at a
# destination is that daemon's own default rather than the wire.
LOCAL_MED = 42


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
    """The positive half: the relayed route is the injected one.

    Without this, a Ze that forwarded nothing, or that rebuilt the route from
    something other than the received block, would satisfy the absence check
    below for a reason that has nothing to do with Section 5.1.4.
    """
    if "65004" not in output:
        log_fail("GoBGP route %s has no 65004 in its AS_PATH: %s" % (prefix, output))
        raise AssertionError("relayed AS_PATH is not the injected one")
    if "172.30.0.9" not in output:
        log_fail("GoBGP route %s has the wrong next-hop: %s" % (prefix, output))
        raise AssertionError("relayed NEXT_HOP is not the injected one")
    log_pass("GoBGP route %s carries the injected AS_PATH and NEXT_HOP" % prefix)


def _check_no_med(output, prefix):
    """The prohibition: a metric received from AS 65004 stops at Ze."""
    if "Med:" in output:
        log_fail("GoBGP route %s still carries a Med: %s" % (prefix, output))
        raise AssertionError("MULTI_EXIT_DISC crossed to another neighboring AS")
    log_pass(
        "GoBGP route %s carries no MULTI_EXIT_DISC (RFC 4271 Section 5.1.4)" % prefix
    )


def _check_local_med(output, prefix):
    """The permission: Ze's own metric is what the attribute is for.

    This is the assertion a blanket strip of attribute 4 fails, and it is why
    the absence checks above are a rule about provenance rather than a strip.
    """
    if "Med: %d" % LOCAL_MED not in output:
        log_fail(
            "GoBGP route %s does not carry Ze's own Med %d: %s"
            % (prefix, LOCAL_MED, output)
        )
        raise AssertionError("a locally set MULTI_EXIT_DISC did not reach the peer")
    log_pass("GoBGP route %s carries Ze's own MULTI_EXIT_DISC %d" % (prefix, LOCAL_MED))


def _check():
    frr = FRR()
    gobgp = GoBGP()

    frr.wait_session(ZE_IP)
    gobgp.wait_session(ZE_IP)

    for prefix in RELAYED + (ORIGINATED,):
        log_info("waiting for %s at FRR and GoBGP..." % prefix)
        _wait_route(frr, prefix)
        _wait_route(gobgp, prefix)
        # FRR's half of the job: a conformant peer accepts both UPDATEs and
        # installs the routes.
        frr.check_route(prefix)
        gobgp.check_route(prefix)

    for prefix in RELAYED:
        output = _gobgp_rib(gobgp, prefix)
        _check_source_block_arrived(output, prefix)
        _check_no_med(output, prefix)

    _check_local_med(_gobgp_rib(gobgp, ORIGINATED), ORIGINATED)

    assert frr.session_established(ZE_IP), "FRR session dropped"
    log_pass("no received MED reached another neighboring AS, and Ze's own did")


def check():
    """Entry point the runner calls. A silent injector looks like a working
    strip, so every sidecar log is dumped when an assertion fails."""
    try:
        _check()
    except Exception:
        # Every read below runs inside the handler of an exception that is
        # re-raised at the end, so each one is printed and none is asserted on.
        frr = FRR()
        gobgp = GoBGP()
        print("--- GoBGP rib ---")
        # fail-open-ok: diagnostic print, the bare `raise` below is unconditional
        print(gobgp._gobgp_quiet(["global", "rib", "-a", "ipv4"]))
        print("--- FRR bgp table ---")
        # fail-open-ok: diagnostic print, the bare `raise` below is unconditional
        print(frr._vtysh_quiet("show bgp ipv4 unicast"))
        for prefix in RELAYED + (ORIGINATED,):
            print("--- FRR %s ---" % prefix)
            # fail-open-ok: diagnostic print, the bare `raise` below is unconditional
            print(frr._vtysh_quiet("show bgp ipv4 unicast %s" % prefix))
        print("--- FRR log ---")
        print(
            # fail-open-ok: diagnostic print, the bare `raise` below is unconditional
            docker_exec_quiet(
                FRR_CONTAINER, ["sh", "-c", "tail -60 /var/log/frr-med.log"]
            )
        )
        print(docker_logs(INJECT_CONTAINER, 40))
        print(docker_logs(ZE_CONTAINER, 60))
        raise
