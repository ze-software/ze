#!/usr/bin/env python3
"""Scenario 51: RFC 9234 Section 5 egress rule 1, both halves, against FRR.

Ze (AS 65001) is FRR's Provider, so FRR is a Customer and rule 1 applies to
everything Ze advertises to it:

    "If a route is to be advertised to a Customer, a Peer, or an RS-Client
     (when the sender is an RS), and the OTC Attribute is not present, then
     when advertising the route, an OTC Attribute MUST be added with a value
     equal to the AS number of the local AS."   -- RFC 9234 Section 5

"Is to be advertised" is the rule's CONDITION. This scenario asserts both
readings of it on ONE session with ONE peer:

  POSITIVE  the relayed advertisement of 10.10.0.0/24 reaches FRR carrying
            OTC 65001;
  NEGATIVE  the relayed withdrawal of the same prefix reaches FRR carrying no
            path attributes at all, so no OTC.

The pair is what makes the negative evidence. An "OTC is absent" assertion on
its own passes with the whole stamping mechanism deleted, and it also passes if
FRR never received the message -- both are the vacuity trap in
ai/rules/interop-and-goal-validation.md.

MUTATION EVIDENCE, measured 2026-08-04 against FRR 10.3.1. Forcing
payloadAdvertisesNLRI (internal/component/bgp/plugins/role/otc.go) to return true
reddens the NEGATIVE with FRR's own words:

    [EC 33554482] 172.30.0.2 Missing well-known attribute NEXT_HOP.
    [EC 33554455] 172.30.0.2(Unknown) rcvd UPDATE with errors in attr(s)!!
    Withdrawing route.

Forcing it to return false reddens the POSITIVE, because the advertisement then
reaches FRR unstamped.

WHAT THE NEGATIVE OBSERVES, exactly. It is the RECEIVER's verdict on the
attributes of the withdrawal, not a byte count this side inferred. That is the
polarity a peer can punish (RFC 7606 Section 5.2 permits a session reset), and it
is what a routing table cannot show: a table cannot distinguish "the withdrawal
had no attributes" from "the withdrawal had an OTC attribute FRR rejected the
message over", because both remove the route.

It is also NARROWER than "no OTC Attribute was added". A withdrawal carrying OTC
beside a complete well-known set would raise no error and would pass. Ze cannot
emit that shape, because no producer adds ORIGIN or NEXT_HOP to a relayed
withdrawal, and test/plugin/role-otc-fwd-withdraw.ci pins the emitted bytes
exactly. That `.ci` closes the gap, not this file. Do not read this assertion as
proving more than its own words.

The negative reads two lines FRR does emit: its decode of the withdrawal, and its
attribute-error verdict. Parsing `rcvd UPDATE wlen N attrlen M` instead cannot
fire at all, because FRR 10.3.1 never emits that line, and the only failure left
is then the "logged no UPDATE carrying withdrawn routes" guard. Measured on
2026-08-04: scenario 51 failed on that guard with the fix IN PLACE. Same shape as
scenario 52.

The two RFC requirement tags are below, as real comments.
"""

# The ledger's scanner (scripts/dev/rfc_requirements.py scan_python_tags)
# tokenizes the file and reads COMMENT tokens only, so a tag inside a string is
# invisible to it.
# RFC requirement: RFC9234-5-4 positive -- an independent conforming receiver (FRR 10.3.1) reports the OTC Attribute carrying ze's local AS on a route ze advertises to it as a Customer.
# RFC requirement: RFC9234-5-4 negative -- the same receiver raises no attribute error over ze's withdrawal of that route and keeps the session up. RFC 7606 Section 5.2 makes the opposite a session-reset hazard, so this is the polarity a peer can actually punish. Stamping a withdrawal produces the error (measured, see the docstring); the emitted bytes themselves are pinned by test/plugin/role-otc-fwd-withdraw.ci.

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, FRR_CONTAINER, docker_exec_quiet, log_pass, log_info

PREFIX = "10.10.0.0/24"
ZE_ASN = 65001

# FRR 10.3.1 never emits `rcvd UPDATE wlen N attrlen M`, so a negative built on it
# can never fire. These two lines are ones it does emit, and an attribute error is
# the RECEIVER's own verdict rather than a byte count this side inferred.
#
# FRR's own decode of an UPDATE that withdrew this prefix, under
# `debug bgp updates in`. Its presence is what stops the negative below passing
# on silence.
WITHDRAWN_LINE = re.compile(
    r"rcvd UPDATE about %s IPv4 unicast -- withdrawn" % re.escape(PREFIX)
)
# FRR's verdict on an UPDATE whose attributes it refused. A withdraw-only UPDATE
# carrying nothing but an OTC Attribute has no well-known mandatory attribute at
# all, which RFC 4271 Section 6.3 makes a Missing Well-known Attribute error, and
# RFC 7606 Section 5.2 lets a receiver answer with a session reset. Both lines
# come from bgp_attr_parse / bgp_update_receive in FRR 10.3.x.
ATTR_ERROR = re.compile(
    r"(Missing well-known attribute|rcvd UPDATE with errors in attr)"
)


def frr_log():
    return docker_exec_quiet(FRR_CONTAINER, ["cat", "/tmp/frr.log"])


def otc_reported(frr):
    """Report the OTC value FRR attributes to PREFIX, or None.

    Tries the structured view first, then the human one, then FRR's own decode of
    the UPDATE. Returning None when every source is silent is deliberate: the
    caller fails on it rather than treating an unreadable answer as a pass
    (ai/rules/evidence.md).
    """
    data = frr.route(PREFIX)
    for path in data.get("paths") or []:
        if isinstance(path, dict) and path.get("otc") is not None:
            return int(path["otc"])
    text = frr._vtysh_quiet("show bgp ipv4 unicast %s" % PREFIX)
    m = re.search(r"[Oo][Tt][Cc][:= ]+(\d+)", text)
    if m:
        return int(m.group(1))
    m = re.search(r"rcvd UPDATE w/ attr:.*?otc[ =]+(\d+)", frr_log())
    if m:
        return int(m.group(1))
    return None


# FRR 10.3.1 never emits the `wlen/attrlen` line, so parsing it always returns []
# and the negative cannot fire.


def check():
    frr = FRR()

    frr.wait_session("172.30.0.2")

    # POSITIVE. Ze relays the injector's route; FRR is a Customer, so it must
    # arrive stamped.
    log_info("waiting for the relayed advertisement of %s..." % PREFIX)
    frr.wait_route(PREFIX, timeout=60)
    otc = otc_reported(frr)
    assert otc is not None, (
        "FRR reported no OTC Attribute on %s -- RFC 9234 Section 5 egress rule 1 "
        "requires one on every route advertised to a Customer. FRR route view:\n%s"
        % (PREFIX, frr._vtysh_quiet("show bgp ipv4 unicast %s" % PREFIX))
    )
    assert otc == ZE_ASN, (
        "FRR reported OTC %d on %s, want %d (the local AS, per rule 1)"
        % (otc, PREFIX, ZE_ASN)
    )
    log_pass("advertisement to a Customer carries OTC %d" % otc)

    # NEGATIVE. The injector withdraws the prefix two KEEPALIVEs after the
    # advertisement, so the positive assertion above has already had 30-60s of
    # window (see inject.msg, BARRIER 2).
    log_info("waiting for the relayed withdrawal...")
    frr.wait_route_absent(PREFIX, timeout=90)
    assert frr.session_established("172.30.0.2"), (
        "FRR session dropped over the withdrawal"
    )

    # The two lines below are ones FRR does emit. An attribute error is the RECEIVER's
    # own verdict rather than a byte count this side inferred.
    #
    # FRR writes each log line as it parses, so give the last one a moment to be
    # flushed to the file rather than racing the read. Bounded, and the assertion
    # below fails loudly if nothing ever appears -- it never passes on silence.
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
        "RFC 9234 Section 5 egress rule 1 is conditioned on a route being "
        "advertised, and a withdrawal advertises none, so no OTC Attribute may be "
        "added. A withdrawal carrying only an OTC Attribute has no well-known "
        "mandatory attribute at all, which RFC 4271 Section 6.3 makes a Missing "
        "Well-known Attribute error and RFC 7606 Section 5.2 makes a "
        "session-reset hazard." % "\n  ".join(errors)
    )
    log_pass("FRR accepted the relayed withdrawal with no attribute error")

    assert frr.session_established("172.30.0.2"), (
        "FRR session dropped after the exchange"
    )
    log_pass("RFC 9234 Section 5 egress rule 1 interop with FRR passed")
