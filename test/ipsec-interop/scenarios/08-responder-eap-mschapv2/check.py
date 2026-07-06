#!/usr/bin/env python3
"""Scenario 08: EAP-MSCHAPv2 responder Ze <- strongSwan (charon is the EAP client).

Validates: Ze as the IKEv2 responder AND EAP authenticator (spec-ipsec-14 AC-4).
           strongSwan initiates with auth=eap-mschapv2 and expects the responder to
           authenticate with a certificate (auth=pubkey). Ze answers IKE_SA_INIT, then
           in the first IKE_AUTH sends IDr + its server certificate + AUTH (its
           long-term credential) + the first EAP-Request; it drives the real eap.Session
           (Begin/Process) through the MSCHAPv2 challenge/response, sends EAP-Success,
           derives the MSK, exchanges MSK-derived AUTH both ways, and installs the first
           Child SA. strongSwan reaching ESTABLISHED with its Child SA INSTALLED proves
           Ze's EAP-server path end to end.
Prevents:  the "NewEAPSession has zero callers" regression, EAP-server round desync,
           and MSK-AUTH direction bugs on the responder.

Note: macOS Docker has no XFRM for the Ze container, so this asserts the control-plane
      EAP establishment from strongSwan (authoritative); the deterministic full EAP
      handshake also runs host-independently in engine TestResponderEAPSessionWired.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    log_pass,
)


def check():
    swan = StrongSwan()

    # Control plane: strongSwan (EAP client) establishes against Ze (EAP server).
    # This only completes if Ze answered IKE_SA_INIT, presented a valid server cert +
    # AUTH, ran the EAP-MSCHAPv2 exchange as authenticator, and verified the peer's
    # MSK-derived AUTH.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()
    log_pass(
        "strongSwan (EAP-MSCHAPv2 client) established against Ze the EAP authenticator"
    )
