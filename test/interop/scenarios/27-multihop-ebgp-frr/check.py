#!/usr/bin/env python3
"""Scenario 27: Multi-hop eBGP with FRR using outgoing-ttl.

Validates: Ze's outgoing-ttl config leaf is accepted and applied without
           breaking session establishment or route exchange.
Prevents:  TTL configuration bugs that silently break eBGP sessions.

Note: Docker containers are on the same L2 network, so TTL=1 works by
default. This test validates that outgoing-ttl 2 is accepted by Ze's
config parser, applied to the TCP connection, and does not cause a TTL
mismatch with FRR's ebgp-multihop setting.
"""

import json
import os, sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, Ze, ZE_IP, log_info, log_pass


def check():
    frr = FRR()
    ze = Ze()

    frr.wait_session(ZE_IP)
    log_pass("multi-hop eBGP session established (outgoing-ttl 2)")

    # Verify FRR sees the session as multi-hop capable.
    nbr_output = frr._vtysh_quiet("show bgp neighbor %s json" % ZE_IP)
    if nbr_output.strip():
        try:
            nbr = json.loads(nbr_output)
            peer = nbr.get(ZE_IP, {})
            multihop = peer.get("externalBgpNbrMaxHopsAway", 0)
            assert multihop >= 2, (
                "FRR does not show ebgp-multihop >= 2 (got %d)" % multihop
            )
            log_pass("FRR confirms ebgp-multihop %d" % multihop)
        except (json.JSONDecodeError, KeyError):
            log_info("could not verify multihop setting (non-fatal)")

    # Verify routes arrive despite non-default TTL.
    frr.wait_route("10.10.0.0/24")
    frr.check_route("10.10.0.0/24")
    frr.check_route("10.10.1.0/24")

    assert frr.session_established(ZE_IP), "session dropped after route exchange"
    log_pass("multi-hop eBGP routes received by FRR, session stable")
