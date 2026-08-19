#!/usr/bin/env python3
"""Scenario no-family-peer-eor-frr: a Ze peer with no `family` block, against FRR.

Validates: Ze establishes with FRR while advertising no Multiprotocol
           capability, exchanges IPv4 unicast (RFC 4271 carries it natively),
           and sends the End-of-RIB marker RFC 4724 Section 4 requires for that
           address family. FRR's own decode of the marker is the assertion.
Prevents:  the silent skip. Before the fix in capability.Negotiate, a side that
           advertised no Multiprotocol capability contributed the empty set to
           the family intersection, so Ze sent no marker and FRR waited for a
           barrier that never arrived.

The capability assertion is the other half: FRR must report the IPv4-unicast
address-family capability as advertised by ITSELF and not received from Ze. That
pins the fix in the negotiation rather than in the OPEN builder, which is what
keeps the wire byte-identical for every peer configured this way.
"""

import json
import os
import re
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP, docker_exec_quiet, log_fail, log_info, log_pass

# RFC requirement: RFC4724-4-1 positive -- an independent conforming receiver
# (FRR 10.3.1) decodes ze's End-of-RIB marker for IPv4 unicast on a session where
# neither speaker advertised a Multiprotocol capability. Removing the per-side
# implicit family in capability.Negotiate leaves FRR receiving the route and no
# marker, which is the state this scenario was written against (measured
# 2026-08-17).

# FRR logs the marker it decodes as "rcvd End-of-RIB for IPv4 Unicast from <peer>"
# (bgp_packet.c). The spelling of the address family differs between releases, so
# the family half is matched case-insensitively and the peer half exactly.
EOR_LINE = re.compile(r"rcvd End-of-RIB for IPv4 Unicast from", re.IGNORECASE)


def frr_log(frr):
    return docker_exec_quiet(frr.container, ["cat", "/tmp/frr.log"])


def neighbor_json(frr):
    output = docker_exec_quiet(
        frr.container, ["vtysh", "-c", "show bgp neighbor %s json" % ZE_IP]
    )
    if not output.strip():
        log_fail("FRR returned no JSON for neighbor %s" % ZE_IP)
        raise RuntimeError("empty neighbor JSON")
    try:
        return json.loads(output).get(ZE_IP, {})
    except json.JSONDecodeError as exc:
        log_fail("FRR neighbor JSON did not parse: %s" % exc)
        raise


def check():
    frr = FRR()

    frr.wait_session(ZE_IP)
    log_pass(
        "session established with a peer that advertised no Multiprotocol capability"
    )

    # IPv4 unicast really is exchanged in this state. Without this the End-of-RIB
    # assertion below would be a barrier over an empty conversation.
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")
    log_pass(
        "FRR received 10.10.0.0/24, so IPv4 unicast flows with no capability declared"
    )

    peer = neighbor_json(frr)
    v4cap = (
        peer.get("neighborCapabilities", {})
        .get("multiprotocolExtensions", {})
        .get("ipv4Unicast", {})
    )
    if v4cap.get("advertisedAndReceived"):
        log_fail(
            "FRR reports the IPv4-unicast capability as advertisedAndReceived: Ze "
            "advertised a Multiprotocol capability it must not advertise for a peer "
            "with no family block"
        )
        raise AssertionError("Ze advertised an unexpected Multiprotocol capability")
    log_pass(
        "Ze advertised no Multiprotocol capability (FRR: %s)" % (v4cap or "absent")
    )

    # The marker is the last frame of the initial update, so it can land after the
    # route. Poll FRR's own decode rather than sleeping.
    log_info("waiting for FRR to decode the End-of-RIB marker from Ze...")
    deadline = time.time() + 60
    log = ""
    while time.time() < deadline:
        # An empty read satisfies nothing: the search is falsy, the loop runs to
        # its deadline, and the assertion below fails loudly with the tail.
        # fail-open-ok: an empty log fails the assertion below, it never passes one
        log = frr_log(frr)
        if EOR_LINE.search(log):
            break
        time.sleep(1)

    if not EOR_LINE.search(log):
        log_fail(
            "FRR logged no End-of-RIB decode for IPv4 unicast. Either Ze sent no "
            "marker (RFC 4724 Section 4), or `debug bgp updates in` is not active. "
            "Last 40 log lines:\n%s" % "\n".join(log.splitlines()[-40:])
        )
        raise AssertionError("no End-of-RIB received by FRR (RFC 4724 Section 4)")
    log_pass("FRR decoded the End-of-RIB marker from Ze for IPv4 unicast")

    if not frr.session_established(ZE_IP):
        log_fail("session dropped after the exchange")
        raise AssertionError("session not established at the end of the scenario")
    log_info("session stable after the initial update")
