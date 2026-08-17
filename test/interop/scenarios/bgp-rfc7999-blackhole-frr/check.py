#!/usr/bin/env python3
"""Scenario bgp-rfc7999-blackhole-frr: Ze honors RFC 7999 BLACKHOLE from FRR.

Validates (plan/spec-bcp194-6-blackhole AC-1, AC-2, AC-3): a BLACKHOLE-tagged
announcement from a peer that agreed to honor it, for a prefix inside a covering
prefix that peer is authorized for, becomes a real discard route in the Linux
FIB. The same community from the same peer for an uncovered prefix, and from a
peer that did not agree, are forwarded normally.

Prevents: the honoring decision stopping short of the kernel. Every test above
this one reads a Ze table. Only this reads `ip route show`, which is what an
operator reads and the only place a wrong netlink message shows up: Linux
refuses RTN_BLACKHOLE carrying a gateway, and every BGP path resolves one.

RFC 7999 Section 3.3 states both conditions as one MUST, so the two negative
assertions are not decoration. Without them a check that only asserts a
blackhole appeared passes just as well when Ze discards everything it receives.
"""

# RFC requirement: RFC7999-3.3-1 positive -- "The announced prefix is covered by an equal or shorter prefix that the neighboring network is authorized to advertise" (RFC 7999 Section 3.3, first condition). FRR 10.3.1 announces 10.100.0.1/32 carrying 65535:666, inside the 10.100.0.0/24 that peer is authorized for, and the Linux FIB in the ze container holds `blackhole 10.100.0.1`. The condition holds and the announcement is honored, asserted on kernel state rather than on a Ze table.
# RFC requirement: RFC7999-3.3-1 negative -- the same FRR session announces 198.51.100.1/32 with the same community, outside every entry of that peer's blackhole `prefixes`. The kernel holds an ordinary `via` route for it and no discard route, so the first condition failing withholds honoring. Without this polarity the check passes equally when the community alone grants a discard.
# RFC requirement: RFC7999-3.3-2 positive -- "The receiving party agreed to honor the BLACKHOLE community on the particular BGP session" (RFC 7999 Section 3.3, second condition). The FRR session names that community in its blackhole `communities`, which is that agreement, and the same 10.100.0.1/32 reaches the kernel as a discard route. Both conditions of the one MUST sentence hold on this session, which is why one outcome is positive evidence for both.
# RFC requirement: RFC7999-3.3-2 negative -- BIRD announces 10.200.0.1/32 carrying 65535:666, inside the 10.200.0.0/24 that peer IS authorized for, on a session whose blackhole `communities` names 65001:666 alone. The kernel forwards it. The authorization is present and the session agreed to a DIFFERENT community, so this isolates the second condition rather than testing an absent config block, and it also pins that a stated community list is taken exactly: the well-known value is never added to it.

import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))

from interop import (  # noqa: E402
    BIRD,
    FRR,
    ZE_CONTAINER,
    ZE_IP,
    docker_exec,
    log_info,
    log_pass,
    poll,
)

# The prefix each announcement carries, and what the peer config makes of it.
BLACKHOLED = "10.100.0.1"  # FRR, honor true, inside authorized 10.100.0.0/24
UNCOVERED = "198.51.100.1"  # FRR, honor true, outside every authorized prefix
NOT_HONORED = "10.200.0.1"  # BIRD, honor false, inside authorized 10.200.0.0/24


def kernel_routes():
    """Return the Ze container's IPv4 route table as one string."""
    return docker_exec(ZE_CONTAINER, ["ip", "-4", "route", "show"])


def is_blackhole(table, address):
    """Report whether the kernel holds a discard route for address.

    busybox `ip route show` prints a host route without its /32, so the match is
    anchored on the address followed by end-of-field. Anchoring on `blackhole `
    at the start of the line is what separates a discard route from a forwarded
    one; a substring search for the address alone cannot tell them apart.
    """
    return (
        re.search(r"^blackhole %s(/32)?\b" % re.escape(address), table, re.MULTILINE)
        is not None
    )


def is_forwarded(table, address):
    """Report whether the kernel holds an ordinary unicast route for address."""
    return (
        re.search(r"^%s(/32)?\s+via\b" % re.escape(address), table, re.MULTILINE)
        is not None
    )


def check():
    frr = FRR()
    bird = BIRD()

    log_info("waiting for the FRR and BIRD sessions to establish...")
    frr.wait_session(ZE_IP)
    bird.wait_session("ze_peer")

    # Both negatives must be present before any verdict. Waiting only for the
    # blackhole would let the run assert "the others are absent" against a table
    # that had simply not been programmed yet, and pass for the wrong reason.
    log_info("waiting for all three prefixes to reach the Ze container's FIB...")
    table = poll(
        kernel_routes,
        lambda t: (
            is_blackhole(t, BLACKHOLED)
            and is_forwarded(t, UNCOVERED)
            and is_forwarded(t, NOT_HONORED)
        ),
        timeout=90,
        what="ip route show in the ze container",
    )

    log_info("kernel routes:\n%s" % table)

    # AC-2 / RFC 7999 Section 3.3: honored, covered, tagged -> discard.
    if not is_blackhole(table, BLACKHOLED):
        raise AssertionError(
            "%s carries BLACKHOLE from an opted-in peer inside its authorized "
            "10.100.0.0/24, but the kernel has no discard route for it:\n%s"
            % (BLACKHOLED, table)
        )

    # AC-3 / RFC 7999 Section 3.3 first condition: the peer is not authorized to
    # advertise a covering prefix, so the community must change nothing.
    if is_blackhole(table, UNCOVERED):
        raise AssertionError(
            "%s is outside every authorized-covering-prefix, so it must be "
            "forwarded, but the kernel discards it:\n%s" % (UNCOVERED, table)
        )
    if not is_forwarded(table, UNCOVERED):
        raise AssertionError(
            "%s reached neither the FIB nor a discard route:\n%s" % (UNCOVERED, table)
        )

    # AC-1 / RFC 7999 Section 3.3 second condition, and Section 4: the receiver
    # never agreed to honor BLACKHOLE on this session, and no explicit directive
    # tells the element to discard.
    if is_blackhole(table, NOT_HONORED):
        raise AssertionError(
            "%s comes from a peer with honor false, so the community must be "
            "ignored, but the kernel discards it:\n%s" % (NOT_HONORED, table)
        )
    if not is_forwarded(table, NOT_HONORED):
        raise AssertionError(
            "%s reached neither the FIB nor a discard route:\n%s" % (NOT_HONORED, table)
        )

    log_pass(
        "bgp-rfc7999-blackhole-frr: BLACKHOLE from FRR discards %s in the kernel; "
        "the uncovered %s and the un-agreed %s stay forwarded"
        % (BLACKHOLED, UNCOVERED, NOT_HONORED)
    )


if __name__ == "__main__":
    check()
