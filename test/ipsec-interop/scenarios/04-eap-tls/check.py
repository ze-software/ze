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

    # 2. XFRM state on BOTH ends.
    #
    # This assertion used to sit inside `except (AssertionError, Exception): log_pass(...)`.
    # That handler caught the AssertionError the check raises when ESP is BROKEN and then
    # logged a pass, so the scenario reported success whatever the dataplane did. Its
    # "expected on Docker for Mac" reason is also stale: scenario 01-psk-site-to-site runs
    # in this same lab with the Ze-side XFRM SA present and ESP counters advancing on both
    # peers, measured 2026-08-01.
    wait_xfrm_sa(SWAN_CONTAINER)
    wait_xfrm_sa(ZE_CONTAINER)

    log_pass("EAP-TLS tunnel established with certificate authentication")
