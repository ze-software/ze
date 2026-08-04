#!/usr/bin/env python3
"""Scenario 54: a route reflector adds no RFC 4456 attribute to a withdrawal.

rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
correctness-only test edits, "making a negative discriminate". This scenario was
first written hours ago in this same session with FRR as the RECEIVER, asserting
"FRR raised no attribute error over the reflected withdrawal". That negative was
then MEASURED VACUOUS: the mutant that stamps ORIGINATOR_ID and CLUSTER_LIST onto
the withdrawal SURVIVED, because FRR 10.3.1's mandatory-attribute check only
fires once NEXT_HOP or MP_REACH_NLRI is present. The rewrite moves the receiving
witness to a byte-exact one and moves FRR to the source side. Nothing is
weakened: an assertion that could not fail is replaced by one that fails on a
single changed byte.

Ze (AS 65000) is a route reflector with two clients: FRR at 172.30.0.3, which
ORIGINATES the route and later withdraws it, and a raw `ze-test peer` at
172.30.0.9, which receives what Ze reflects. Both sessions are INTERNAL, which is
what this scenario gives and scenarios 52 and 53 cannot: forwardUpdateCore
(internal/component/bgp/reactor/reactor_api_forward.go) records
Op(9, AttrModSet) and Op(10, AttrModPrepend) only when the source is internal and
the destination is not external.

    "When a RR reflects a route, it MUST prepend the local CLUSTER_ID to the
     CLUSTER_LIST."                                  -- RFC 4456 Section 8

"Reflects a route" is the clause's CONDITION, and RFC 4271 Section 4.3 says what
the other case looks like:

    "An UPDATE message might advertise only routes that are to be withdrawn
     from service, in which case the message will not include path attributes
     or Network Layer Reachability Information."   -- RFC 4271 Section 4.3

Both readings are asserted on one reflection:

  POSITIVE  the reflected ADVERTISEMENT of 10.20.0.0/24 reaches the peer carrying
            ORIGINATOR_ID and CLUSTER_LIST. This proves the injection is ENGAGED;
            without it every assertion below would pass on a build where
            reflection never fired at all;
  NEGATIVE  the reflected WITHDRAWAL of the same prefix arrives byte-exact as
            `0004180A14000000` -- withdrawn-len 4, the prefix, attr-len 0. One
            stamped attribute changes those bytes, so this cannot pass vacuously.

FRR is a real BGP speaker composing both messages, so the interop claim here is
that Ze reflects what a conforming implementation sent it without adding to a
message that reflects no route. Scenario 53 carries the complementary claim, that
a conforming implementation ACCEPTS what Ze relays.

The RFC requirement tags are below, as real comments: the ledger's scanner
(scripts/dev/rfc_requirements.py scan_python_tags) reads COMMENT tokens only, so
a tag inside this docstring would be invisible to it.
"""

# RFC requirement: RFC4456-8-1 positive -- Ze sets ORIGINATOR_ID on a route it reflects between two clients, observed on the wire by the receiving peer, so the identifier is set when a route IS reflected.
# RFC requirement: RFC4456-8-1 negative -- Ze creates no ORIGINATOR_ID on the withdrawal of that same route, asserted byte-exact on the wire, because the clause's condition ("reflects a route") is not met.
# RFC requirement: RFC4456-8-2 positive -- the same reflected route carries a CLUSTER_LIST, prepended with Ze's configured cluster-id.
# RFC requirement: RFC4456-8-2 negative -- and the withdrawal carries none, for the same reason, in the same byte-exact assertion.

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    FRR,
    FRR_CONTAINER,
    INJECT_CONTAINER,
    docker_exec_quiet,
    docker_logs,
    log_info,
    log_pass,
)

PREFIX = "10.20.0.0/24"
ZE_ADDR = "172.30.0.2"

# The reflected WITHDRAWAL of 10.20.0.0/24, exactly as RFC 4271 Section 4.3
# describes it: withdrawn-len 0004, prefix 18 0A 14 00, attr-len 0000.
#
# `ze-test peer --decode` prints every received message as
# <marker>:<length>:<type>:<body> (internal/test/peer/peer.go printPayload), so
# matching ":02:" plus this body is an exact match on the UPDATE body Ze sent.
WITHDRAWAL_BODY = ":02:0004180A14000000"

# The reflected ADVERTISEMENT, identified by ORIGINATOR_ID: attribute flags 0x80
# (Optional, Non-transitive per RFC 4456), type 9 (0x09), length 4.
# rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
# correctness-only test edits. The value is FRR's own BGP Identifier, 172.30.0.3
# (AC1E0003), which is what a reflector puts in ORIGINATOR_ID for a route the
# client originated. The line first written minutes ago in this session carried
# 0A0000.., a value no message in this scenario can hold, so the positive
# assertion could only ever time out. Strictly more discriminating.
ORIGINATOR_ATTR = re.compile(r":02:[0-9A-F]*800904AC1E0003", re.IGNORECASE)
# CLUSTER_LIST: flags 0x80, type 10 (0x0A), length 4, value 172.30.0.2.
CLUSTER_ATTR = re.compile(r":02:[0-9A-F]*800A04AC1E0002", re.IGNORECASE)


def peer_log():
    """Every message Ze has sent the raw peer, as printed hex."""
    return docker_logs(INJECT_CONTAINER, lines=4000)


def wait_for(predicate, timeout, what):
    deadline = time.time() + timeout
    last = ""
    while time.time() < deadline:
        last = peer_log()
        if predicate(last):
            return last
        time.sleep(2)
    raise AssertionError(
        "%s did not appear in the reflected stream within %ds. Last 40 lines of "
        "what the peer received:\n%s"
        % (what, timeout, "\n".join(last.splitlines()[-40:]))
    )


def check():
    frr = FRR()
    frr.wait_session(ZE_ADDR)

    # POSITIVE. FRR originates the prefix; Ze reflects it to the raw peer with
    # both RFC 4456 Section 8 attributes added.
    log_info("waiting for the reflected advertisement of %s..." % PREFIX)
    log = wait_for(
        lambda t: ORIGINATOR_ATTR.search(t) is not None,
        90,
        "an UPDATE carrying ORIGINATOR_ID",
    )
    assert CLUSTER_ATTR.search(log), (
        "the reflected route carries ORIGINATOR_ID but no CLUSTER_LIST 172.30.0.2. "
        "RFC 4456 Section 8 obliges a route reflector to prepend its local "
        "CLUSTER_ID when it reflects a route. Last 40 lines:\n%s"
        % "\n".join(log.splitlines()[-40:])
    )
    log_pass(
        "reflection is engaged: the advertisement carries ORIGINATOR_ID and CLUSTER_LIST"
    )

    # Guard: the withdrawal must not already be in the stream, or the assertion
    # below would match something that predates the withdraw command.
    assert WITHDRAWAL_BODY not in log, (
        "a withdraw-shaped UPDATE reached the peer before FRR was asked to "
        "withdraw. The negative assertion below would be matching the wrong "
        "message. Last 40 lines:\n%s" % "\n".join(log.splitlines()[-40:])
    )

    # NEGATIVE. Ask FRR to withdraw the prefix it originated.
    log_info("asking FRR to withdraw %s..." % PREFIX)
    docker_exec_quiet(
        FRR_CONTAINER,
        [
            "vtysh",
            "-c",
            "configure terminal",
            "-c",
            "router bgp 65000",
            "-c",
            "address-family ipv4 unicast",
            "-c",
            "no network %s" % PREFIX,
        ],
    )

    wait_for(
        lambda t: WITHDRAWAL_BODY in t,
        90,
        "the reflected withdrawal with attr-len 0000 (body %s)" % WITHDRAWAL_BODY[4:],
    )
    log_pass(
        "the reflected withdrawal carries no path attribute: %s" % WITHDRAWAL_BODY[4:]
    )

    assert frr.session_established(ZE_ADDR), (
        "FRR session dropped during the withdrawal, so the bytes above may not "
        "describe a healthy reflection."
    )
    log_pass("RFC 4456 Section 8 relay-shape interop with FRR passed")
