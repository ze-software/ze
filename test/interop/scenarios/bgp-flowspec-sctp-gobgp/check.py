#!/usr/bin/env python3
"""FlowSpec SCTP rule announced BY a real peer and installed BY ze.

Validates: a peer announces a FlowSpec route whose IP protocol is 132, and ze
           turns it into a kernel nftables rule. This is the receive direction
           of the FlowSpec bridge; bgp-flowspec-frr and bgp-flowspec-gobgp both
           cover the send direction only, and neither names a protocol outside
           tcp.
Prevents:  the five-name protocol table returning. It knew 1, 6, 17, 47 and 58
           and rendered every other value as decimal digits, which no firewall
           backend resolves. The nft backend returns that error from Apply
           BEFORE its single Flush, so one such route from one peer left the
           kernel holding the previous ruleset for every firewall owner.

The peer is GoBGP, not FRR. FRR 10.3.1 can RECEIVE FlowSpec and cannot
originate it: `address-family ipv4 flowspec` offers activate and policy
attachment and no route-origination command, and `router bgp` names flowspec
only in `bgp default`. Verified by listing both command sets in the scenario's
own FRR image on 2026-08-22.

Reverting the translator change makes this fail: the term would carry
MatchProtocol{"132"}, lowerProtoMatch would refuse it, and no table named
flowspec would ever appear in the kernel.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import (
    GOBGP_CONTAINER,
    ZE_CONTAINER,
    ZE_IP,
    GoBGP,
    Ze,
    docker_exec,
    log_info,
    log_pass,
)

FLOW_DEST = "10.99.5.0/24"


def check():
    gobgp = GoBGP()
    ze = Ze()

    gobgp.wait_session(ZE_IP)

    # docker_exec raises on a non-zero exit, so a refused announcement is a
    # failure here rather than an empty answer the assertions below read as
    # "the rule did not arrive".
    docker_exec(
        GOBGP_CONTAINER,
        [
            "gobgp",
            "global",
            "rib",
            "-a",
            "ipv4-flowspec",
            "add",
            "match",
            "destination",
            FLOW_DEST,
            "protocol",
            "==sctp",
            "then",
            "discard",
        ],
    )
    log_pass("GoBGP originated a FlowSpec route matching SCTP to %s" % FLOW_DEST)

    log_info("waiting for the kernel ruleset on ze to carry the rule...")
    deadline = time.time() + 30
    ruleset = ""
    while time.time() < deadline:
        ruleset = docker_exec(ZE_CONTAINER, ["nft", "list", "ruleset"])
        # nft prints the lowered value as `meta l4proto 132`, reserving its
        # symbolic names for the protocols its own inet_proto table carries, so
        # both spellings are accepted rather than one being asserted.
        if "table inet flowspec" in ruleset and (
            "l4proto 132" in ruleset or "l4proto sctp" in ruleset
        ):
            break
        time.sleep(1)

    assert "table inet flowspec" in ruleset, (
        "ze installed no flowspec table for the peer's SCTP route:\n%s\n%s"
        % (ruleset, ze.logs(30))
    )
    assert "l4proto 132" in ruleset or "l4proto sctp" in ruleset, (
        "the flowspec table carries no SCTP protocol match:\n%s" % ruleset
    )
    log_pass("ze installed a kernel rule matching SCTP")

    # The destination the peer announced must narrow the rule. A rule without
    # it drops more traffic than the peer asked ze to drop, which is the failure
    # a discarded match produces.
    assert FLOW_DEST in ruleset, (
        "the installed rule does not carry the announced destination %s:\n%s"
        % (FLOW_DEST, ruleset)
    )
    log_pass("the installed rule carries the announced destination %s" % FLOW_DEST)

    assert gobgp.session_established(ZE_IP), (
        "session dropped after the FlowSpec exchange"
    )
    log_pass("FlowSpec session with GoBGP stable")
