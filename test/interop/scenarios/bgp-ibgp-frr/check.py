#!/usr/bin/env python3
"""Scenario 03: iBGP session Ze <-> FRR (same AS).

Validates: Ze establishes an iBGP session (same AS number) with FRR.
Prevents:  Same-AS rejection bugs or LOCAL_PREF handling issues.
"""

import sys, os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
