#!/usr/bin/env python3
"""Scenario 01: eBGP IPv4 session Ze <-> FRR.

Validates: Ze establishes an eBGP session with FRR using IPv4 unicast.
Prevents:  OPEN/KEEPALIVE exchange failures between Ze and FRR.
"""

import sys, os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
