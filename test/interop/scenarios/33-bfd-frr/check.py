#!/usr/bin/env python3
"""Scenario 33: Ze <-> FRR BGP with BFD opt-in.

Validates: A ze BGP peer that opts into BFD via the Stage 3 YANG
           augment establishes both the BGP session and the BFD
           session with FRR's bfdd. Exercises ze's BFD plugin on
           UDP 3784 (single-hop) end-to-end against an independent
           RFC 5880 implementation.
Prevents:  wire-format regressions in ze's BFD codec, FSM drift
           vs FRR's state machine, and the "ze publishes Service
           but BGP never calls EnsureSession" class of bugs that
           unit tests cannot catch.

The test is single-hop only (RFC 5881) because RFC 5883 §4
prohibits multi-hop echo and FRR's default `bfd { peer X }` block
assumes single-hop unless `multihop` is added.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
    frr.wait_bfd_up(ZE_IP)
