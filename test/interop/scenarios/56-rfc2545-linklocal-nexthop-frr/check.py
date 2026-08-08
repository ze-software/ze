#!/usr/bin/env python3
"""Scenario 56: RFC 2545 Section 3's "if and only if" against FRR 10.3.1.

Ze announces two IPv6 prefixes to ONE peer on ONE session under ONE `link-local`
leaf. The routes differ in ONE variable, the global next hop:

  2001:db8:5601::/48  next hop ZE_IP6            -- on a locally connected /64
  2001:db8:5602::/48  next hop 2001:db8:ffff::1  -- on no locally connected prefix

The peer half of the Section 3 condition holds for both (the session address is on
the lab's connected /24), so what FRR reports is the answer to the next-hop half
alone. FRR's `nexthops` array is its decode of the Length of Next Hop Network
Address octet: 32 yields a global entry plus a link-local entry, 16 yields the
global entry alone (BGP_ATTR_NHLEN_IPV6_GLOBAL_AND_LL vs BGP_ATTR_NHLEN_IPV6_GLOBAL
in FRR's attribute parser). So the count and the scope labels below read the length
octet through an independent implementation.
"""

# The tags sit here rather than in the docstring above: scan_python_tags
# (scripts/dev/rfc_requirements.py) reads COMMENT tokens only, so a `#` inside a
# string is not a tag.
#
# RFC requirement: RFC2545-3-2 positive -- an independent conforming receiver (FRR
# 10.3.1) decodes a Length of Next Hop Network Address octet of 32 on the on-link
# route, reporting one global-scope and one link-local-scope next hop, and an octet
# of 16 on the off-link route, reporting the global-scope entry alone. The two
# routes cross the same session, so the length octet is the only thing that can
# differ between the two decodes.
#
# RFC requirement: RFC2545-3-3 positive -- the link-local address IS included when
# the speaker shares a common subnet with BOTH the entity named by the global next
# hop and the peer the route is advertised to. FRR reports fe80::be:ef:2 as a
# second, link-local-scope next hop for 2001:db8:5601::/48 and installs the route
# via it (`B>* ... via fe80::be:ef:2, eth0`), so the receiver both parsed and used
# the second address.
#
# RFC requirement: RFC2545-3-3 negative -- the link-local address is NOT included
# when the speaker shares no subnet with the entity named by the global next hop,
# even though the peer half of the condition holds and the same `link-local` leaf
# is configured. FRR reports 2001:db8:ffff::1 as the sole next hop of
# 2001:db8:5602::/48, with no link-local-scope entry. A leaf that decided inclusion
# by itself would put fe80::be:ef:2 on this route too, and this assertion is what
# fails when it does.

import ipaddress
import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    FRR,
    FRR_CONTAINER,
    ZE_IP,
    ZE_IP6,
    docker_exec_quiet,
    log_fail,
    log_info,
    log_pass,
)

# The `link-local` leaf of ze.conf. One leaf, both routes.
LINK_LOCAL = "fe80::be:ef:2"

# The route whose global next hop lies on the lab's connected IPv6 /64.
ON_LINK_PREFIX = "2001:db8:5601::/48"

# The route whose global next hop lies on no locally connected prefix. FRR marks it
# inaccessible and keeps it out of the forwarding table, which is what an unreachable
# next hop always earns -- Section 3's else-branch is about the WIRE FORM, and the
# BGP table entry below is where that form is observable.
OFF_LINK_PREFIX = "2001:db8:5602::/48"
OFF_LINK_NEXTHOP = "2001:db8:ffff::1"


def _same_addr(a, b):
    """Compare two IPv6 addresses by value, not by spelling.

    FRR re-renders every address canonically, so `fd00:1e:0::2` comes back as
    `fd00:1e::2`. A string compare against the harness's own spelling would fail on
    a route that is entirely correct.
    """
    return ipaddress.ip_address(a) == ipaddress.ip_address(b)


def _nexthops(frr, prefix):
    """Return FRR's next-hop entries for prefix as a list of (scope, ip).

    Raises when the route or its path is absent: an empty list would read as "no
    link-local next hop", which is the exact thing the negative assertion checks, so
    a missing route must never be able to satisfy it (ai/rules/evidence.md).
    """
    data = frr.route(prefix, family="ipv6 unicast")
    paths = data.get("paths") if isinstance(data, dict) else None
    if not paths:
        log_fail("FRR reports no path for %s" % prefix)
        raise AssertionError(
            "FRR has no BGP path for %s (cannot read next hops)" % prefix
        )
    if len(paths) != 1:
        log_fail("FRR reports %d paths for %s (expected 1)" % (len(paths), prefix))
        raise AssertionError(
            "FRR has %d paths for %s, expected 1" % (len(paths), prefix)
        )
    entries = paths[0].get("nexthops")
    if not entries:
        log_fail("FRR path for %s carries no nexthops array" % prefix)
        raise AssertionError("FRR path for %s has no nexthops array" % prefix)
    return [(e.get("scope"), e.get("ip")) for e in entries]


def _one(entries, scope):
    """Return the single address of the given scope, or None when there is none."""
    found = [ip for (s, ip) in entries if s == scope]
    if len(found) > 1:
        raise AssertionError(
            "FRR reports %d %s next hops, expected at most 1" % (len(found), scope)
        )
    return found[0] if found else None


def check():
    frr = FRR()

    frr.wait_session(ZE_IP)
    frr.wait_route(ON_LINK_PREFIX, family="ipv6 unicast")
    frr.wait_route(OFF_LINK_PREFIX, family="ipv6 unicast")

    on_link = _nexthops(frr, ON_LINK_PREFIX)
    off_link = _nexthops(frr, OFF_LINK_PREFIX)
    log_info("FRR next hops for %s: %s" % (ON_LINK_PREFIX, on_link))
    log_info("FRR next hops for %s: %s" % (OFF_LINK_PREFIX, off_link))

    # RFC2545-3-3 positive: both halves of the condition hold, so the link-local
    # address is in the Next Hop field.
    global_nh = _one(on_link, "global")
    assert global_nh is not None and _same_addr(global_nh, ZE_IP6), (
        "FRR reports global next hop %r for %s, expected %s"
        % (global_nh, ON_LINK_PREFIX, ZE_IP6)
    )
    ll_nh = _one(on_link, "link-local")
    assert ll_nh is not None and _same_addr(ll_nh, LINK_LOCAL), (
        "FRR reports link-local next hop %r for %s, expected %s -- RFC 2545 Section 3 "
        "requires the link-local address in the Next Hop field when the speaker shares "
        "a subnet with both the global next hop and the peer"
        % (ll_nh, ON_LINK_PREFIX, LINK_LOCAL)
    )
    log_pass(
        "FRR reports global %s and link-local %s for %s"
        % (global_nh, ll_nh, ON_LINK_PREFIX)
    )

    # RFC2545-3-3 positive, second half: FRR forwards via the link-local address, so
    # the second address was not merely parsed but used.
    routes = docker_exec_quiet(FRR_CONTAINER, ["vtysh", "-c", "show ipv6 route bgp"])
    installed = [line for line in routes.splitlines() if ON_LINK_PREFIX in line]
    assert installed, "FRR did not install %s in its IPv6 RIB: %r" % (
        ON_LINK_PREFIX,
        routes,
    )
    assert LINK_LOCAL in installed[0], (
        "FRR installed %s but not via the link-local next hop %s: %r"
        % (ON_LINK_PREFIX, LINK_LOCAL, installed[0])
    )
    log_pass(
        "FRR installed %s via %s: %s"
        % (ON_LINK_PREFIX, LINK_LOCAL, installed[0].strip())
    )

    # RFC2545-3-3 negative: the next-hop half fails, so the link-local address is
    # absent -- from the same session, under the same `link-local` leaf.
    off_global = _one(off_link, "global")
    assert off_global is not None and _same_addr(off_global, OFF_LINK_NEXTHOP), (
        "FRR reports global next hop %r for %s, expected %s"
        % (off_global, OFF_LINK_PREFIX, OFF_LINK_NEXTHOP)
    )
    off_ll = _one(off_link, "link-local")
    assert off_ll is None, (
        "FRR reports link-local next hop %s for %s, whose global next hop %s lies on "
        "no locally connected subnet -- RFC 2545 Section 3 includes the link-local "
        "address if and only if the speaker shares a subnet with that entity"
        % (off_ll, OFF_LINK_PREFIX, OFF_LINK_NEXTHOP)
    )
    log_pass(
        "FRR reports %s alone for %s, with no link-local next hop"
        % (off_global, OFF_LINK_PREFIX)
    )

    # RFC2545-3-2: the length octet, read through FRR's decode of it.
    assert len(on_link) == 2, (
        "FRR decoded %d next-hop addresses for %s, expected 2 (Length of Next Hop "
        "Network Address = 32)" % (len(on_link), ON_LINK_PREFIX)
    )
    assert len(off_link) == 1, (
        "FRR decoded %d next-hop addresses for %s, expected 1 (Length of Next Hop "
        "Network Address = 16)" % (len(off_link), OFF_LINK_PREFIX)
    )
    log_pass("length octet 32 for %s, 16 for %s" % (ON_LINK_PREFIX, OFF_LINK_PREFIX))

    assert frr.session_established(ZE_IP), (
        "session dropped after the two IPv6 route announcements"
    )
    log_pass("RFC 2545 Section 3 holds in both polarities, session stable")
