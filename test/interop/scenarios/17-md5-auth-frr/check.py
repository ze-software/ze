#!/usr/bin/env python3
"""Scenario 17: TCP MD5 authenticated session with FRR."""

import os, sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, log_pass


def check():
    frr = FRR()

    # If MD5 is misconfigured, the TCP handshake fails and session never establishes.
    frr.wait_session("172.30.0.2")
    log_pass("MD5-authenticated session established")
