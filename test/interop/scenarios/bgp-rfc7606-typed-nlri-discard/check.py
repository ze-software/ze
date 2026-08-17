#!/usr/bin/env python3
"""Scenario 53: an INDEPENDENT peer proves ze discards an unrecognized EVPN route type.

RFC 7606 Section 5.4: "A BGP speaker advertising support for such a typed address family
MUST handle routes with unrecognized NLRI types within that address family by discarding
them, unless the relevant specification for that address family specifies otherwise."
RFC 7432 states no deviation, so the default binds EVPN.

Shape: the raw injector announces one l2vpn/evpn MP_REACH carrying route type 2 (assigned,
ze implements it) and route type 99 (unassigned) in the SAME attribute. Ze is a route
server, so it relays the received wire to the speaker, and the speaker's plugin reads what
actually arrived.

Discrimination, in both directions:
  - fix in place -> the speaker receives ONE EVPN NLRI, route type 2; result PASS.
  - fix reverted -> the relay is verbatim, the speaker receives route type 99 as well, and
    the plugin fails with "received EVPN route type 99, which is not assigned". Measured:
    an early return in applyTypedNLRIDiscard produced evpn-nlri: 2 and that exact failure.
Non-vacuous the other way too: this check asserts the speaker actually received an EVPN
NLRI, so a session that negotiated the family and then relayed nothing fails rather than
passing quietly. That matters here more than usual, because the whole change under test
REMOVES routes: a check that only asserted the absence of type 99 would pass just as well
if the relay were broken outright.

Why the speaker rather than FRR or GoBGP: a conforming daemon discards the unassigned type
itself, so its route table cannot distinguish "ze discarded it" from "the peer discarded
it". The speaker inspects the raw MP_REACH bytes ze put on the wire.
"""

# The tag must be a real comment, not a line inside the docstring above. The ledger's
# scanner (scripts/dev/rfc_requirements.py scan_python_tags) tokenizes the file and reads
# COMMENT tokens only, so a tag written inside a string is invisible to it: the scenario
# would run, pass, and be counted as evidence for nothing.
# RFC requirement: RFC7606-5.4-1 positive -- an independent peer receives the assigned EVPN route type and never the unassigned one ze was sent in the same attribute.

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (  # noqa: E402
    INJECT_CONTAINER,
    SPEAKER_CONTAINER,
    ZE_CONTAINER,
    docker_logs,
    log_fail,
    log_info,
    log_pass,
)


def _speaker_report(timeout=120):
    """Poll the speaker container's logs until it prints its final verdict, or time out."""
    deadline = time.time() + timeout
    while time.time() < deadline:
        logs = docker_logs(SPEAKER_CONTAINER, 60)
        if "result:" in logs:
            return logs
        time.sleep(2)
    return docker_logs(SPEAKER_CONTAINER, 60)


def _field(logs, key, default=None):
    # The engine prints its verdict as "result: PASS" and its notes as "note: <key>: <value>",
    # so match the "<key>:" token anywhere on the line, not only at the start.
    token = key + ":"
    for line in logs.splitlines():
        idx = line.find(token)
        if idx != -1:
            return line[idx + len(token) :].strip()
    return default


def _dump():
    log_fail("speaker logs:\n%s" % docker_logs(SPEAKER_CONTAINER, 80))
    log_fail("injector logs:\n%s" % docker_logs(INJECT_CONTAINER, 40))
    log_fail("ze logs:\n%s" % docker_logs(ZE_CONTAINER, 60))


def check():
    log_info("waiting for the speaker's verdict on the relayed EVPN routes...")
    logs = _speaker_report()

    established = _field(logs, "established")
    routes = int(_field(logs, "route-bearing-updates", "0") or "0")
    evpn = int(_field(logs, "evpn-nlri", "0") or "0")
    result = _field(logs, "result")

    if established != "yes" or routes < 1 or evpn < 1:
        _dump()
        raise AssertionError(
            "speaker established=%s route-bearing-updates=%d evpn-nlri=%d: "
            "the relayed EVPN route never arrived, so nothing was proven"
            % (established, routes, evpn)
        )
    log_pass("speaker established and received %d EVPN NLRI" % evpn)

    if result != "PASS":
        _dump()
        raise AssertionError(
            "speaker verdict %s: an unrecognized EVPN route type reached a peer, "
            "which RFC 7606 Section 5.4 forbids" % result
        )
    log_pass("RFC 7606 Section 5.4: only the assigned EVPN route type was relayed")
