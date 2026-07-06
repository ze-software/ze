#!/usr/bin/env python3
"""Scenario 09: IKE-SA rekey responder Ze <- strongSwan (charon rekeys the IKE SA).

Validates: Ze answers a peer-initiated IKE-SA rekey (spec-ipsec-14 AC-5, closing
           spec-ipsec-13's deferred responder). strongSwan initiates the tunnel and,
           with a 40s IKE rekey_time, sends a CREATE_CHILD_SA that rekeys the IKE SA
           (SA + KEi + Ni, no TS). Ze's owner loop routes it to respondIKERekey, which
           completes a fresh DH, derives the new IKE SA keys (RFC 7296 2.18), replies
           with SA + KEr + Nr, and swaps to the new SA when strongSwan deletes the old
           one. This is control-plane only (the IKE SA itself has no ESP state), so it
           is fully verifiable even without XFRM.
Prevents:  regression to dropping peer IKE rekeys (the pre-spec-ipsec-14 behavior),
           wrong SK direction on the rekeyed IKE SA, and make-before-break bugs.

Note: The IKE-SA rekey needs no XFRM, so both peer logs and Ze's own logs are the
      witnesses here regardless of host dataplane support.
"""

import os
import sys
import time

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    docker_logs_all,
    log_pass,
)


def check():
    swan = StrongSwan()

    # 1. Tunnel establishes with Ze as responder.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 2. Wait for strongSwan (rekey_time = 40s) to complete the IKE-SA rekey that Ze
    #    must answer. strongSwan is the authoritative witness (Ze's plugin logs at INFO
    #    are not captured in the container, per scenario 05): a successful rekey it
    #    initiated ("IKE_SA ze[N] rekeyed") could only complete if Ze's respondIKERekey
    #    derived the new keys and replied correctly.
    deadline = time.time() + 120
    swan_rekeyed = False
    while time.time() < deadline:
        swan_logs = docker_logs_all(SWAN_CONTAINER)
        if "INVALID_SYNTAX" in swan_logs or "no IKE config found" in swan_logs:
            raise AssertionError("strongSwan reported an error during IKE rekey")
        # charon logs "IKE_SA ze[2] rekeyed between ..." on a successful IKE rekey.
        if "rekeyed" in swan_logs and "IKE_SA ze[" in swan_logs:
            swan_rekeyed = True
            break
        time.sleep(3)

    if not swan_rekeyed:
        raise AssertionError(
            "strongSwan never completed the IKE-SA rekey (Ze did not answer respondIKERekey)"
        )
    log_pass(
        "strongSwan completed a peer-initiated IKE-SA rekey against Ze the responder"
    )

    # 3. The tunnel must survive the rekey: strongSwan's IKE SA still ESTABLISHED.
    if "ESTABLISHED" not in swan.list_sas():
        raise AssertionError("strongSwan IKE SA is not ESTABLISHED after the rekey")

    log_pass("IKE-SA rekey responder verified; tunnel survived (control plane)")
