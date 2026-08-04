#!/usr/bin/env python3
"""Fail if any relayed EVPN NLRI carries a route type the receiver cannot read.

RFC 7606 Section 5.4: "A BGP speaker advertising support for such a typed address family
MUST handle routes with unrecognized NLRI types within that address family by discarding
them, unless the relevant specification for that address family specifies otherwise."
RFC 7432 states no deviation, so the default binds EVPN.

This plugin is the INDEPENDENT half of the proof. Ze's own unit and .ci tests assert on
bytes ze produced; here a peer that ze does not control reads what actually arrived on the
wire and judges it against RFC 7432 Section 7.1's route-type list. Route types 1..5 are
assigned (Ethernet Auto-Discovery, MAC/IP Advertisement, Inclusive Multicast Ethernet Tag,
Ethernet Segment, IP Prefix); anything else is unassigned or reserved, and a relay that
passes one on has failed the MUST.

Non-vacuity is the caller's job as well as this one's: the plugin notes how many EVPN NLRIs
it saw, so check.py can refuse a silent PASS from a session that received nothing.
"""

import struct

NAME = "no-unrecognized-evpn-type"

AFI_L2VPN = 25
SAFI_EVPN = 70
MP_REACH = 14
MP_UNREACH = 15
# RFC 7432 Section 7.1 and the "EVPN Route Types" registry (Section 20), plus RFC 9136
# route type 5 (IP Prefix).
ASSIGNED_ROUTE_TYPES = (1, 2, 3, 4, 5)


def _evpn_nlri_bytes(attr):
    """Return the NLRI portion of an EVPN MP attribute, or None when it is not one.

    RFC 4760 Section 3: MP_REACH_NLRI is AFI(2) SAFI(1) NextHopLen(1) NextHop(n)
    Reserved(1) then NLRI. Section 4: MP_UNREACH_NLRI is AFI(2) SAFI(1) then NLRI.
    """
    if attr.code not in (MP_REACH, MP_UNREACH):
        return None
    value = attr.value
    if len(value) < 3:
        return None
    afi, safi = struct.unpack(">HB", value[0:3])
    if (afi, safi) != (AFI_L2VPN, SAFI_EVPN):
        return None
    if attr.code == MP_UNREACH:
        return value[3:]
    if len(value) < 4:
        return None
    start = 4 + value[3] + 1
    if start > len(value):
        return None
    return value[start:]


def _route_types(nlri):
    """Walk [route-type:1][length:1][body:length] and yield each route type.

    A framing error stops the walk and is reported, because a receiver that cannot frame
    the section cannot judge the types inside it either.
    """
    types = []
    off = 0
    while off < len(nlri):
        if off + 2 > len(nlri):
            return types, "truncated EVPN header at offset %d" % off
        length = nlri[off + 1]
        if off + 2 + length > len(nlri):
            return types, "EVPN NLRI at offset %d overruns the section" % off
        types.append(nlri[off])
        off += 2 + length
    return types, None


def on_update(update, ctx):
    seen = ctx.store.setdefault("evpn_nlri", 0)
    for attr in update.attributes:
        nlri = _evpn_nlri_bytes(attr)
        if nlri is None:
            continue
        types, err = _route_types(nlri)
        if err:
            ctx.fail("EVPN NLRI section is not well framed: %s" % err)
        for route_type in types:
            seen += 1
            if route_type not in ASSIGNED_ROUTE_TYPES:
                ctx.fail(
                    "RFC 7606 Section 5.4: received EVPN route type %d, which is not "
                    "assigned; it must have been discarded, not relayed" % route_type
                )
    ctx.store["evpn_nlri"] = seen


def on_end(ctx):
    ctx.note("evpn-nlri: %d" % ctx.store.get("evpn_nlri", 0))
