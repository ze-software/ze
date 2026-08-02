#!/usr/bin/env python3
"""Scenario 06: EAP-TLS over TLS 1.3, Ze <-> strongSwan.

Validates: Ze's IKEv2 engine completes an EAP-TLS exchange with strongSwan on a
           TLS 1.3 session, and derives its MSK by the RFC 9190 Section 2.3
           exporter rather than the RFC 5216 Section 2.3 one. The scenario ships
           no ze-env file, so ze runs with NO GODEBUG: reaching an established
           SA is itself the proof that the TLS 1.3 path needs no weakened
           setting, because Go's ExportKeyingMaterial is unconditional only on
           TLS 1.3.
Prevents:  a silent fall back to TLS 1.2 (which would need
           GODEBUG=tlsunsafeekm=1 and so cannot pass here), EAP-TLS
           fragmentation defects on the larger TLS 1.3 flights, and regressions
           in RFC 9190 MSK export.
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

    # 1. IKE SA established with EAP-TLS, and a child SA on top of it.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 2. The session really was TLS 1.3. Without this the scenario would still
    #    pass if charon ignored the version pin, and the claim in its name would
    #    be false. strongSwan logs the negotiated version once per session.
    swan_log = docker_logs(SWAN_CONTAINER, 400)
    if "negotiated TLS 1.3" not in swan_log:
        raise AssertionError(
            "strongSwan never logged 'negotiated TLS 1.3'; the session did not "
            "use TLS 1.3, so this scenario proves nothing about the RFC 9190 path"
        )
    if "negotiated TLS 1.2" in swan_log:
        raise AssertionError(
            "strongSwan logged 'negotiated TLS 1.2'; the version pin did not hold"
        )

    # 3. The TLS 1.3 CertificateRequest carried a POPULATED
    #    certificate_authorities extension. charon runs its shipped
    #    `send_certreq_authorities = yes` here, and write_certificate_authorities
    #    (src/libtls/tls_server.c) logs this line once per CA it enumerates. An
    #    empty list is not merely weaker, it is malformed: RFC 8446 Section 4.2.4
    #    declares `DistinguishedName authorities<3..2^16-1>`, and charon still
    #    emits the extension as `002f 0002 0000`, which Go's crypto/tls rejects
    #    with decode_error. Asserting the line means a regression that empties
    #    the list again fails this scenario instead of being papered over by
    #    setting send_certreq_authorities = no.
    if "sending TLS cert request" not in swan_log:
        raise AssertionError(
            "strongSwan never logged 'sending TLS cert request'; its "
            "certificate_authorities list was empty, so the extension it sent "
            "was the malformed 3-octet-minimum-violating '002f 0002 0000' and "
            "this scenario is no longer proving interop against a stock "
            "send_certreq_authorities"
        )

    # 4. XFRM state on BOTH ends.
    wait_xfrm_sa(SWAN_CONTAINER)
    wait_xfrm_sa(ZE_CONTAINER)

    log_pass("EAP-TLS tunnel established over TLS 1.3 with no GODEBUG on ze")
