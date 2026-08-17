#!/usr/bin/env python3
"""Scenario 33: Ze <-> FRR BGP with BFD opt-in.

Validates: A ze BGP peer that opts into BFD via the Stage 3 YANG
           augment establishes both the BGP session and the BFD
           session with FRR's bfdd. Exercises ze's BFD plugin on
           UDP 3784 (single-hop) end-to-end against an independent
           RFC 5880 implementation. After the handshake, induces a
           BFD link failure via iptables and asserts the BGP session
           tears down within 2 seconds (ze timers: 300ms x 3 = 900ms
           BFD detection, plus BGP teardown + notification).
Prevents:  wire-format regressions in ze's BFD codec, FSM drift
           vs FRR's state machine, and the "ze publishes Service
           but BGP never calls EnsureSession" class of bugs that
           unit tests cannot catch. The failover assertion catches
           regressions in the BFD->BGP teardown signaling path.

The test is single-hop only (RFC 5881) because RFC 5883 §4
prohibits multi-hop echo and FRR's default `bfd { peer X }` block
assumes single-hop unless `multihop` is added.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP, log_pass


FAILOVER_BUDGET_S = 2.0
FAILOVER_POLL_INTERVAL_S = 0.1
FAILOVER_SAFETY_CAP_S = 5.0


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
    frr.wait_bfd_up(ZE_IP)

    t0 = time.monotonic()
    frr.break_link()
    try:
        deadline = t0 + FAILOVER_SAFETY_CAP_S
        t_down = None
        while time.monotonic() < deadline:
            if not frr.session_established(ZE_IP):
                t_down = time.monotonic()
                break
            time.sleep(FAILOVER_POLL_INTERVAL_S)
        if t_down is None:
            raise AssertionError(
                "BGP session still Established %.1fs after BFD link break"
                % FAILOVER_SAFETY_CAP_S
            )
        elapsed = t_down - t0
        if elapsed >= FAILOVER_BUDGET_S:
            raise AssertionError(
                "BGP teardown took %.3fs, expected < %.1fs"
                % (elapsed, FAILOVER_BUDGET_S)
            )
        log_pass(
            "BFD failover: BGP down in %.3fs (< %.1fs)" % (elapsed, FAILOVER_BUDGET_S)
        )
    finally:
        frr.restore_link()
