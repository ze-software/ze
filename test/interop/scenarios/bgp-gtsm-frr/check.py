#!/usr/bin/env python3
"""Scenario 46: eBGP IPv4 session Ze <-> FRR with GTSM / TTL-security on both ends.

Validates: With single-hop GTSM enabled on both peers (Ze `ttl { max 1 }`,
           FRR `ttl-security hops 1`), the eBGP session still establishes.
           This proves Ze transmits segments with TTL 255 and accepts the
           peer's TTL-255 segments (RFC 5082), and that FRR's receive-side
           GTSM filter accepts Ze's traffic.
Prevents:  Ze omitting the outgoing TTL=255 (FRR would silently drop Ze's
           SYN/segments) or installing a min-TTL gate that rejects FRR's
           legitimate TTL-255 segments -- either breaks the session.
"""

import sys, os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
