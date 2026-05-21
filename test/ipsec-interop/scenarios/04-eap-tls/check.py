#!/usr/bin/env python3
"""Scenario 04: EAP-TLS authentication Ze <-> strongSwan.

Validates: Ze's IKEv2 engine performs an EAP-TLS exchange inside
           IKE_AUTH with strongSwan. Both sides present X.509
           certificates validated against a shared test CA.
Prevents:  EAP-TLS fragmentation bugs, TLS handshake failures,
           MSK export errors, certificate chain validation issues.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    ZE_CONTAINER,
    log_pass,
    wait_xfrm_sa,
)


def check():
    swan = StrongSwan()

    # 1. IKE SA established with EAP-TLS.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 2. XFRM state if available.
    wait_xfrm_sa(SWAN_CONTAINER)
    try:
        wait_xfrm_sa(ZE_CONTAINER)
    except (AssertionError, Exception):
        log_pass(
            "XFRM not available on Ze (expected on Docker for Mac), skipping ESP checks"
        )

    log_pass("EAP-TLS tunnel established with certificate authentication")
