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
    SWAN_IP,
    ZE_CONTAINER,
    assert_esp_accepted,
    docker_exec_quiet,
    docker_logs,
    log_pass,
    wait_xfrm_sa,
    xfrm_sa_bytes_by_spi,
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

    # 5. Data plane: the SAs carry ESP that the peer ACCEPTS.
    #
    # This matters more here than anywhere else in the tree. The scenario's own claim is
    # about the MSK: on TLS 1.3 it comes from the RFC 9190 Section 2.3 exporter, and on
    # TLS 1.2 from the RFC 5216 Section 2.3 one. Export the wrong 64 octets and the IKE
    # AUTH still verifies on both sides, the Child SA still installs, and the XFRM SAs
    # still appear on both ends holding keys that do not match. Only a decrypt at the
    # peer separates the two outcomes, and until 2026-08-15 this scenario stopped one
    # step short of asking for it.
    #
    # The counter is the oracle, not the ping: both containers share a Docker bridge, so
    # ICMP crosses with the tunnel down. The counters are read before and after a blocking
    # ping rather than after a sleep, so nothing here waits on elapsed time (spec R-2).
    #
    # Discrimination, measured 2026-08-15: with one octet of the outbound ESP key flipped
    # in installChildSA (internal/component/ike/engine/child.go), the two wait_xfrm_sa
    # calls above still passed and so did ze's own counter. Only the strongSwan assertion
    # below went red.
    ze_before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    swan_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)

    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])

    assert_esp_accepted(
        ZE_CONTAINER,
        ze_before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "no traffic through the TLS 1.3 EAP-TLS tunnel",
    )

    assert_esp_accepted(
        SWAN_CONTAINER,
        swan_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP from the RFC 9190 exported MSK",
    )

    log_pass("EAP-TLS over TLS 1.3 established with no GODEBUG on ze, ESP accepted")
