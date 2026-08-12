#!/usr/bin/env python3
"""Scenario 25: EAP-TLS 1.3 responder, Ze <- strongSwan (charon is the EAP client).

Validates: Ze as the IKEv2 responder AND EAP-TLS SERVER sends the RFC 9190
           Section 2.5 protected success result indication -- an encrypted TLS
           record carrying application data 0x00 -- before EAP-Success, and a
           real EAP-TLS 1.3 client accepts the conversation because of it.
Prevents:  the authenticator concluding a TLS 1.3 EAP-TLS exchange with a bare
           EAP-Success. That is what tlsMethod.Process did until 2026-08-12, and
           it is not a cosmetic omission: strongSwan's eap_tls.c get_msk returns
           FAILED without the indication, so the EAP MSK is never handed to
           IKEv2, the AUTH payload of RFC 7296 Section 2.16 cannot be computed,
           and the SA never establishes.

Why this scenario had to be built: scenarios 04 and 06 both put strongSwan in
the EAP SERVER role (swanctl.conf: local auth = pubkey, remote auth = eap-tls),
and eap_tls_handshake_test.go is Go against Go. Nothing in the lab read Ze's
EAP-TLS authenticator against another implementation, which is why a wire-visible
RFC violation on that path survived.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    ZE_CONTAINER,
    docker_logs,
    log_pass,
    wait_xfrm_sa,
)


def check():
    swan = StrongSwan()

    # 1. The control plane completed. This only happens if Ze answered
    #    IKE_SA_INIT, presented its certificate and AUTH, ran the whole EAP-TLS
    #    exchange as the authenticator, and both ends derived the same MSK.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    swan_log = docker_logs(SWAN_CONTAINER, 600)

    # 2. The session really was TLS 1.3. RFC 9190 Section 2.5 applies to nothing
    #    else, so a silent fall back to 1.2 would leave this scenario asserting
    #    an establishment that proves nothing about the indication.
    if "negotiated TLS 1.3" not in swan_log:
        raise AssertionError(
            "strongSwan never logged 'negotiated TLS 1.3'; the session did not use "
            "TLS 1.3, so this scenario proves nothing about RFC 9190 Section 2.5"
        )
    if "negotiated TLS 1.2" in swan_log:
        raise AssertionError(
            "strongSwan logged 'negotiated TLS 1.2'; the version pin in "
            "strongswan.conf did not hold"
        )

    # RFC requirement: RFC9190-2.5-1 positive -- charon's client_process reads the
    # protected success indication, requires it to be exactly one octet equal to
    # 0, and logs this line only after it has accepted it
    # (src/libcharon/plugins/eap_tls/eap_tls.c). Its presence is another
    # implementation confirming, on the wire, that Ze sent the encrypted TLS
    # record with application data 0x00 the section demands.
    if "received protected success indication via TLS" not in swan_log:
        raise AssertionError(
            "strongSwan never logged 'received protected success indication via "
            "TLS'; Ze did not send the RFC 9190 Section 2.5 encrypted record "
            "carrying application data 0x00 (or charon's tls loglevel is not 2)"
        )

    # RFC requirement: RFC9190-2.5-1 negative -- the same client refuses to
    # produce an MSK when the indication is absent, and says so:
    # get_msk logs "missing protected success indication for EAP-TLS with TLS
    # 1.3" and returns FAILED. MEASURED 2026-08-12 with tlsMethod.indicateSuccess
    # reverted: this line appears and wait_sa_established above times out, so the
    # assertion below is what names the cause rather than leaving a bare timeout.
    if "missing protected success indication" in swan_log:
        raise AssertionError(
            "strongSwan logged 'missing protected success indication'; Ze "
            "concluded the EAP-TLS 1.3 exchange with a bare EAP-Success"
        )

    # 3. Data plane on both ends.
    wait_xfrm_sa(SWAN_CONTAINER)
    wait_xfrm_sa(ZE_CONTAINER)

    log_pass(
        "strongSwan (EAP-TLS 1.3 client) accepted Ze's protected success indication"
    )
