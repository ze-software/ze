#!/usr/bin/env python3
"""Scenario 53: next-hop-self does not stamp a NEXT_HOP onto a relayed withdrawal.

Ze (AS 65001) relays between a raw injector (AS 65004) and FRR (AS 65002), the
plain eBGP rail scenario 52 uses. One thing differs: the FRR peer carries
`next-hop self`, so applyFactsNextHop (internal/component/bgp/reactor/
peer_forward_facts.go) records Op(3, AttrModSet) and Op(14, AttrModSet) against
every UPDATE relayed to FRR.

    "An UPDATE message might advertise only routes that are to be withdrawn
     from service, in which case the message will not include path attributes
     or Network Layer Reachability Information."   -- RFC 4271 Section 4.3

    "If any of the well-known mandatory attributes are not present, then the
     Error Subcode MUST be set to Missing Well-known Attribute."
                                                    -- RFC 4271 Section 6.3

Both readings are asserted on ONE session with ONE peer:

  POSITIVE  the relayed advertisement of 10.10.0.0/24 reaches FRR with next-hop
            172.30.0.2 -- Ze's own address, not the injector's 172.30.0.9. This
            is what proves the rewrite is ENGAGED. Without it every assertion
            below would also pass on a build where next-hop-self never fired;
  NEGATIVE  the relayed withdrawal of the same prefix is ACCEPTED: FRR removes
            the route, raises no attribute error over it, and keeps the session.

The pair is what makes the negative evidence. "No error was logged" on its own
passes with the whole forward rail deleted, and it passes if FRR never received
anything -- both are the vacuity trap in ai/rules/interop-and-goal-validation.md.

rfc-test-change-approved: 2026-08-04 -- Thomas standing authorisation for
correctness-only test edits. The two RFC4271-4.3-1 tags drafted here minutes ago,
in this same session, are REMOVED because that id is the Transitive-bit rule
(rfc/short/rfc4271.md:698) and has nothing to do with the shape this scenario
drives. A wrong tag claims evidence for an obligation nobody proved. Nothing
else in the file changes, and no assertion is touched.

NO `RFC requirement:` TAG, deliberately. The obligation this proves is RFC 4271
Section 4.3's message shape, which the summary carries as prose rather than as a
checklist row: Section 4.3's "will not include path attributes" is indicative,
and the MUST that bites is Section 6.3's Missing Well-known Attribute, whose
extracted row (RFC4271-6.3-1) is a RECEIVER obligation this scenario does not
drive. Section 5.1.3 DOES carry rows now -- RFC4271-5.1.3-1 and 5.1.3-2, added
with the guard in internal/component/bgp/reactor/forward_next_hop.go -- but they
are about a NEXT_HOP naming the peer's own address, which is scenario 61's
subject and not this one's: here the address is Ze's own and the defect was
stamping it onto a message that advertises nothing. Tagging any of those would
claim evidence for something else, so the scenario stands as interop evidence
under ai/rules/interop-and-goal-validation.md and claims nothing in the ledger. Scenario 54 IS tagged, to RFC4456-8-1 and
RFC4456-8-2, whose "when an RR reflects a route" condition is extracted.

The negative reads FRR's OWN log rather than its routing table, because a routing
table cannot distinguish "FRR parsed the withdrawal" from "FRR rejected the
UPDATE and withdrew the route as the RFC 7606 treat-as-withdraw fallback": both
remove the prefix. The error line is the only place a receiver states its verdict.
"""

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, FRR_CONTAINER, docker_exec_quiet, log_pass, log_info

PREFIX = "10.10.0.0/24"
ZE_ADDR = "172.30.0.2"
INJECTOR_ADDR = "172.30.0.9"

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


def route_nexthops(frr, prefix):
    """Every next-hop IP FRR attributes to prefix."""
    out = []
    data = frr.route(prefix)
    for path in data.get("paths") or []:
        for nh in path.get("nexthops") or []:
            if nh.get("ip"):
                out.append(nh["ip"])
    return out


def check():
    frr = FRR()

    frr.wait_session(ZE_ADDR)

    # POSITIVE. next-hop-self is engaged: the route arrives carrying Ze's own
    # address rather than the injector's.
    log_info("waiting for the relayed advertisement of %s..." % PREFIX)
    frr.wait_route(PREFIX, timeout=60)
    nexthops = route_nexthops(frr, PREFIX)
    assert nexthops == [ZE_ADDR], (
        "FRR reports next-hop(s) %r on %s, want [%s]. The peer carries "
        "`next-hop self`, so applyFactsNextHop must rewrite the injector's %s. "
        "A scenario where it did not fire would prove nothing about the "
        "withdrawal below. FRR route view:\n%s"
        % (
            nexthops,
            PREFIX,
            ZE_ADDR,
            INJECTOR_ADDR,
            frr._vtysh_quiet("show bgp ipv4 unicast %s" % PREFIX),
        )
    )
    log_pass(
        "next-hop-self is engaged: %s arrives with next-hop %s" % (PREFIX, ZE_ADDR)
    )

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
        "A withdraw-only UPDATE carries no route, so a per-destination next-hop "
        "rewrite has nothing to rewrite and RFC 4271 Section 4.3 says the "
        "message carries no path attributes. Stamping a lone NEXT_HOP leaves a "
        "well-known set that is incomplete by construction, which Section 6.3 "
        "makes a Missing Well-known Attribute error." % "\n  ".join(errors)
    )
    log_pass("FRR accepted the relayed withdrawal with no attribute error")

    assert frr.session_established(ZE_ADDR), (
        "FRR session dropped over the withdrawal. RFC 7606 Section 5.2 permits a "
        "receiver to answer an UPDATE that carries path attributes but encodes no "
        "reachable NLRI with a session reset."
    )
    log_pass("RFC 4271 Section 4.3 next-hop-self relay-shape interop with FRR passed")
