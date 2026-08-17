#!/usr/bin/env python3
"""Scenario 02: eBGP IPv4 session Ze <-> BIRD.

Validates: Ze establishes an eBGP session with BIRD using IPv4 unicast.
Prevents:  OPEN/KEEPALIVE exchange failures between Ze and BIRD.
"""

import sys, os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import BIRD


def check():
    bird = BIRD()
    bird.wait_session("ze_peer")
