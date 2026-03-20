#!/usr/bin/env python3
"""Scenario 18: eBGP session establishment with GoBGP.

Validates: Ze can establish a BGP session with a third implementation (GoBGP).
Prevents:  Implementation-specific assumptions that only work with FRR/BIRD.
"""

import os, sys
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import GoBGP, ZE_IP, log_pass


def check():
    gobgp = GoBGP()
    gobgp.wait_session(ZE_IP)
    log_pass("eBGP session established with GoBGP")
