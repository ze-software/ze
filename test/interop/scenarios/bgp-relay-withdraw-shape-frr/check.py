#!/usr/bin/env python3
"""Scenario 52: a relayed withdrawal is ACCEPTED by FRR, on the plain eBGP rail.

Ze (AS 65001) relays between a raw injector (AS 65004) and FRR (AS 65002).
Neither peer is an RS client, so RFC 4271 Section 5.1.2 applies in full:

    "When a given BGP speaker advertises the route to an external peer, the
     advertising speaker updates the AS_PATH attribute as follows: [...]"
                                                    -- RFC 4271 Section 5.1.2

"Advertises the route" is the clause's CONDITION, and RFC 4271 Section 4.3 says
what the other case looks like:

    "An UPDATE message might advertise only routes that are to be withdrawn
     from service, in which case the message will not include path attributes
     or Network Layer Reachability Information."   -- RFC 4271 Section 4.3

Both readings are asserted on ONE session with ONE peer:

  POSITIVE  the relayed advertisement of 10.10.0.0/24 reaches FRR with AS_PATH
            "65001 65004" -- Ze's own AS prepended, as the clause requires;
  NEGATIVE  the relayed withdrawal of the same prefix is ACCEPTED: FRR removes
            the route, raises no attribute error over it, and keeps the session.

The pair is what makes the negative evidence. A "no error was logged" assertion
on its own passes with the whole forward rail deleted, and it also passes if FRR
never received anything -- both are the vacuity trap in
ai/rules/interop-and-goal-validation.md.

rfc-test-change-approved: 2026-08-04 -- Thomas approved. DOCSTRING ONLY, in a
file first written in this same session: the mutation paragraph below replaces a
predicted result with the two measured ones. No assertion, regex or fixture is
touched. ai/rules/evidence.md: a predicted mutant result is a hypothesis.

MUTATION EVIDENCE, both measured on 2026-08-04 against FRR 10.3.1:

  Removing the guard in ASPathEdit.Record (wireu/aspath_slot.go) outright --
  `if len(in.Prepend) == 0` -- reddens the POSITIVE. The relayed End-of-RIB is
  stamped with a synthesized AS_PATH too, stops being a marker, and the injector
  never releases the announce. RFC 7606 Section 5.2 names that hazard.

  Narrowing the guard to the End-of-RIB alone -- `len(payload) == 4` -- leaves
  the barrier intact and reddens the NEGATIVE below with FRR's own words:

      [EC 33554482] 172.30.0.2 Missing well-known attribute NEXT_HOP.
      [EC 33554455] 172.30.0.2(Unknown) rcvd UPDATE with errors in attr(s)!!
      Withdrawing route.

The negative reads FRR's OWN log rather than its routing table, because a routing
table cannot distinguish "FRR parsed the withdrawal" from "FRR rejected the
UPDATE and withdrew the route as the RFC 7606 treat-as-withdraw fallback": both
remove the prefix. The error line is the only place a receiver states its verdict.

The two RFC requirement tags are below, as real comments.
"""

# rfc-test-change-approved: 2026-08-04 -- Thomas approved. The two tags MOVE from
# the docstring above into comments, verbatim. Nothing else changes. Measured:
# rfc_requirements.scan_tree() returned [] for this file with them in the
# docstring, so the scenario would have run, passed, and been counted as evidence
# for NOTHING. Moving them makes the evidence real, which strengthens.
#
# The ledger's scanner (scripts/dev/rfc_requirements.py scan_python_tags)
# tokenizes the file and reads COMMENT tokens only, so a tag inside a string is
# invisible to it.
# RFC requirement: RFC4271-5.1.2-3 positive -- an independent conforming receiver (FRR 10.3.1) reports AS_PATH "65001 65004" on a route ze relays to it as an ordinary external peer, so the local AS is prepended when a route IS advertised.
# RFC requirement: RFC4271-5.1.2-3 negative -- the same receiver accepts ze's withdrawal of that route with no attribute error, because the clause's condition ("advertises the route") is not met and no AS_PATH is created. RFC 4271 Section 6.3 makes the opposite a Missing Well-known Attribute error.

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, FRR_CONTAINER, docker_exec_quiet, log_pass, log_info

PREFIX = "10.10.0.0/24"
ZE_ASN = "65001"
SOURCE_ASN = "65004"

# FRR's verdict on an UPDATE whose attributes it refused. Both lines are emitted
# by bgp_attr_parse / bgp_update_receive in FRR 10.3.x.
ATTR_ERROR = re.compile(
    r"(Missing well-known attribute|rcvd UPDATE with errors in attr)"
)
# FRR's own decode of an UPDATE that withdrew routes.
WITHDRAWN_LINE = re.compile(
    r"rcvd UPDATE about %s IPv4 unicast -- withdrawn" % re.escape(PREFIX)
)


def frr_log():
    return docker_exec_quiet(FRR_CONTAINER, ["cat", "/tmp/frr.log"])


def route_aspath(frr, prefix):
    """The AS_PATH string FRR attributes to prefix, or "" when it has none."""
    data = frr.route(prefix)
    for path in data.get("paths") or []:
        aspath = path.get("aspath")
        if isinstance(aspath, dict):
            return aspath.get("string", "")
        if isinstance(aspath, str):
            return aspath
    return ""


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")

    # POSITIVE. Ze relays the injector's route as an ordinary eBGP speaker, so it
    # must arrive with Ze's own AS at the head of the path.
    log_info("waiting for the relayed advertisement of %s..." % PREFIX)
    frr.wait_route(PREFIX, timeout=60)
    aspath = route_aspath(frr, PREFIX).split()
    assert aspath == [ZE_ASN, SOURCE_ASN], (
        "FRR reports AS_PATH %r on %s, want [%s, %s]. RFC 4271 Section 5.1.2 "
        "requires the advertising speaker to prepend its own AS when it "
        "advertises a route to an external peer. FRR route view:\n%s"
        % (
            aspath,
            PREFIX,
            ZE_ASN,
            SOURCE_ASN,
            frr._vtysh_quiet("show bgp ipv4 unicast %s" % PREFIX),
        )
    )
    log_pass("advertisement reaches FRR with AS_PATH %s" % " ".join(aspath))

    # NEGATIVE. The injector withdraws the prefix two KEEPALIVEs after the
    # advertisement, so the positive assertion above has already had 30-60s of
    # window (see inject.msg, BARRIER 2).
    log_info("waiting for the relayed withdrawal...")
    frr.wait_route_absent(PREFIX, timeout=90)

    # FRR writes each log line as it parses, so give the last one a moment to be
    # flushed rather than racing the read. Bounded, and the assertion below fails
    # loudly if nothing ever appears -- it never passes on silence.
    deadline = time.time() + 20
    log = ""
    while time.time() < deadline:
        log = frr_log()
        if WITHDRAWN_LINE.search(log):
            break
        time.sleep(1)

    assert WITHDRAWN_LINE.search(log), (
        "FRR logged no decode of a withdrawal for %s, so there is nothing to "
        "assert on. Either the withdrawal never reached FRR, or `debug bgp "
        "updates in` is not active. Last 40 log lines:\n%s"
        % (PREFIX, "\n".join(log.splitlines()[-40:]))
    )

    errors = [line for line in log.splitlines() if ATTR_ERROR.search(line)]
    assert not errors, (
        "FRR refused the attributes of an UPDATE Ze relayed:\n  %s\n"
        "A withdraw-only UPDATE carries no route, so RFC 4271 Section 5.1.2's "
        "prepend does not arise and Section 4.3 says the message carries no path "
        "attributes. Creating a lone AS_PATH leaves a well-known set that is "
        "incomplete by construction, which Section 6.3 makes a Missing "
        "Well-known Attribute error." % "\n  ".join(errors)
    )
    log_pass("FRR accepted the relayed withdrawal with no attribute error")

    assert frr.session_established("172.30.0.2"), (
        "FRR session dropped over the withdrawal. RFC 7606 Section 5.2 permits a "
        "receiver to answer an UPDATE that carries path attributes but encodes no "
        "reachable NLRI with a session reset."
    )
    log_pass("RFC 4271 Section 4.3 relay-shape interop with FRR passed")
