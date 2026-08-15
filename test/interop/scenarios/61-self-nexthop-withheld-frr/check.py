#!/usr/bin/env python3
"""Scenario 61: a route is not advertised to the peer whose own address it names.

Ze (AS 65001) relays between a raw injector (AS 65004) and FRR (AS 65002) on the
GENERAL forward rail (no rs-fast-path). The injector sends two announcements that
differ only in prefix and NEXT_HOP.

    "A route originated by a BGP speaker SHALL NOT be advertised to a peer using
     an address of that peer as NEXT_HOP."          -- RFC 4271 Section 5.1.3

  NEGATIVE  10.11.0.0/24, NEXT_HOP 172.30.0.3, FRR's own address. FRR must never
            log a decode of it, because Ze must never put it on FRR's wire.
  POSITIVE  10.12.0.0/24, NEXT_HOP 172.30.0.9, the injector. FRR must receive it
            and install it, carrying that same third-party next hop.

THE POSITIVE IS WHAT MAKES THE NEGATIVE EVIDENCE. "FRR never saw 10.11.0.0/24"
passes with the forward rail deleted, with Ze crashed, and with the session never
established -- the vacuity trap in ai/rules/interop-and-goal-validation.md. The
control route travels the same rail, in the same fan-out, one message later, and
its arrival is the barrier the absence assertion is read against.

THE ASSERTION IS ON FRR'S LOG, NOT ON FRR'S TABLE, and that is not a stylistic
choice. FRR applies RFC 4271 Section 6.3(a) itself, so a route naming its own
address would be absent from its table whether Ze withheld it or sent it. Only
`debug bgp updates in` says which prefixes reached the wire, which is the fact
Section 5.1.3 is about. The table is still asserted for the control route,
because that half is about a route FRR does accept.

The RFC requirement tags are below, as real comments: the ledger's scanner
(scripts/dev/rfc_requirements.py scan_python_tags) reads COMMENT tokens only, so
a tag inside this docstring would be invisible to it.
"""

# RFC requirement: RFC4271-5.1.3-1 positive -- FRR, a conforming implementation, never decodes an UPDATE carrying 10.11.0.0/24, whose NEXT_HOP 172.30.0.3 is FRR's own address. The assertion is on FRR's own per-UPDATE log rather than on its table, because FRR applies Section 6.3(a) itself and would drop such a route either way.
# RFC requirement: RFC4271-5.1.3-1 negative -- the same session, one message later, receives 10.12.0.0/24 with the third-party NEXT_HOP 172.30.0.9 and installs it with that address. Section 5.1.3 case 2 permits a third-party next hop, so a relay that withheld everything would be a different violation, and this half is what stops the absence above from passing vacuously.

import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, FRR_CONTAINER, docker_exec_quiet, log_pass, log_info

WITHHELD = "10.11.0.0/24"
CONTROL = "10.12.0.0/24"
ZE_ADDR = "172.30.0.2"
INJECTOR_ADDR = "172.30.0.9"
FRR_ADDR = "172.30.0.3"


def rcvd_line(prefix):
    """FRR's own decode of an UPDATE that carried prefix, in either form.

    FRR 10.3.x writes `rcvd <prefix> IPv4 unicast` for an announcement and
    `rcvd UPDATE about <prefix> IPv4 unicast -- withdrawn` for a withdrawal.
    Both are matched, because the negative below asks whether the prefix ever
    reached FRR at all rather than in which half of a message.
    """
    p = re.escape(prefix)
    return re.compile(r"rcvd (?:UPDATE about )?%s IPv4 unicast" % p)


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

    # BARRIER. The control route is the SECOND announcement, so its arrival
    # proves the first has already been through the same rail.
    log_info("waiting for the control route %s to reach FRR..." % CONTROL)
    control = rcvd_line(CONTROL)
    deadline = time.time() + 90
    log = ""
    while time.time() < deadline:
        # An empty read satisfies nothing here: control.search("") is falsy, so the
        # loop runs to its deadline and the assertion below fails loudly. The
        # negative assertion further down reads the same variable and is never
        # reached on an empty log, because that one fires first.
        # fail-open-ok: an empty log fails the assertion below, it never passes one
        log = frr_log()
        if control.search(log):
            break
        time.sleep(1)

    assert control.search(log), (
        "FRR logged no decode of %s, so this scenario asserted nothing. Either "
        "the forward rail never carried it, or `debug bgp updates in` is not "
        "active. Last 40 log lines:\n%s" % (CONTROL, "\n".join(log.splitlines()[-40:]))
    )
    log_pass("the control route %s reached FRR" % CONTROL)

    # POSITIVE. The control route is installed with the injector's next hop, so
    # Ze is relaying third-party next hops rather than withholding everything.
    frr.wait_route(CONTROL, timeout=60)
    nexthops = route_nexthops(frr, CONTROL)
    assert nexthops == [INJECTOR_ADDR], (
        "FRR reports next-hop(s) %r on %s, want [%s]. RFC 4271 Section 5.1.3 "
        "case 2 permits a third-party NEXT_HOP, and Ze must keep relaying it. "
        "FRR route view:\n%s"
        % (
            nexthops,
            CONTROL,
            INJECTOR_ADDR,
            # fail-open-ok: diagnostic only, evaluated while this assertion already fails
            frr._vtysh_quiet("show bgp ipv4 unicast %s" % CONTROL),
        )
    )
    log_pass("%s installed with the third-party next hop %s" % (CONTROL, INJECTOR_ADDR))

    # NEGATIVE. The withheld route was sent FIRST and has never been decoded.
    withheld = rcvd_line(WITHHELD)
    hits = [line for line in log.splitlines() if withheld.search(line)]
    assert not hits, (
        "Ze advertised %s to FRR, whose NEXT_HOP %s is FRR's own address. "
        "RFC 4271 Section 5.1.3 forbids it: FRR would resolve that next hop to "
        "one of its own interfaces and the traffic would never leave it. FRR's "
        "decode:\n  %s" % (WITHHELD, FRR_ADDR, "\n  ".join(hits))
    )
    log_pass("%s was withheld: its next hop is FRR's own address" % WITHHELD)

    assert frr.session_established(ZE_ADDR), (
        "FRR session dropped. Withholding one route must not disturb the "
        "session: Section 5.1.3 states a prohibition on advertising, and RFC "
        "4271 Section 6.3 says a NOTIFICATION SHOULD NOT be sent over a "
        "semantically incorrect NEXT_HOP."
    )
    log_pass("RFC 4271 Section 5.1.3 egress interop with FRR passed")
