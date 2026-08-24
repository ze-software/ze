#!/usr/bin/env python3
"""Scenario 04: EAP-TLS Ze <-> a STOCK strongSwan, which lands on TLS 1.2.

Validates: ze drives the whole EAP-TLS exchange with strongSwan over TLS 1.2
           -- certificate validation, EAP-TLS fragmentation, and the TLS
           handshake -- and then REFUSES the peer at the RFC 5216 Section 2.3
           MSK export, with one log line that names the peer, the negotiated
           version, RFC 7627 and the three things an operator can change.
Prevents:  EAP-TLS fragmentation defects and certificate chain defects on the
           TLS 1.2 flights, an SA installed from key material ze never derived,
           and an operator left holding a raw crypto/tls sentence.

Why this scenario asserts a refusal rather than a tunnel. RFC 5216 Section 2.3
defines the EAP-TLS MSK as a crypto/tls ExportKeyingMaterial result, and Go
refuses that export when the session is below TLS 1.3 AND did not negotiate the
RFC 7627 extended master secret. strongSwan 5.9.14 meets both halves by DEFAULT:
charon ships `version_max = 1.2` and implements no RFC 7627. So ze cannot derive
the MSK here, and the correct behaviour is to say why and install nothing.

This is the only place that behaviour is observable. A `.ci` drives ze against
ze, and Go's own client offers RFC 7627 on every TLS 1.2 ClientHello
(`makeClientHello`, crypto/tls), so a Ze-driven peer always exports and the
refusal never happens.

The TLS 1.3 path is scenario 06-eap-tls13, which carries no ze-env file at all
and reaches an established SA on this SAME strongSwan image with one line of
peer config (`charon.tls.version_max = 1.3`). That pair is the evidence for the
first remedy the message below names: a peer config edit, not a peer upgrade.

Until 2026-08-23 this scenario set the `tlsunsafeekm` GODEBUG setting for the ze
container, which lifted the refusal for this scenario alone. Go 1.27 removed that
setting, and a removed key carrying its old value is a fatal error raised before
main(), so the line stopped ze at container start. See ze-env beside this file.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    SWAN_CONTAINER,
    ZE_CONTAINER,
    check_xfrm_sa_count,
    docker_logs_all,
    log_pass,
    wait_swan_log,
    wait_ze_log,
)

# The refusal ze must log, split into the facts an operator acts on. Each entry
# is asserted on its own so a failure names the fact that went missing rather
# than printing one 400-character diff.
#
# Producer: eapTLS12ExportRefused (internal/component/ike/eap/eap_tls.go). The
# peer subject is the strongSwan server certificate this lab's PKI issues
# (test/interop-ipsec/pki), so it is a fixed string here rather than a pattern.
REFUSAL_FACTS = (
    # what failed, and the RFC clause that defines it
    "cannot export the RFC 5216 Section 2.3 MSK",
    # who the peer is
    "for peer CN=172.28.0.3",
    # what was negotiated, which is the condition the operator must change
    "on TLS 1.2",
    # why the export is refused
    "RFC 7627 extended master secret",
    # the three remedies
    "Move the peer to TLS 1.3 (RFC 9190)",
    "add RFC 7627 to its TLS 1.2 stack",
    "configure another EAP method",
    # the crypto/tls sentence, kept so the log names the refusal ze actually met
    "crypto/tls: ExportKeyingMaterial is unavailable",
)


def check():
    # 1. The exchange runs to the end of the TLS handshake.
    #
    # These two strongSwan lines are what keeps this scenario's ORIGINAL value.
    # The refusal below happens after the last EAP-TLS fragment is reassembled
    # and both certificate chains are verified, so a fragmentation or chain
    # defect reds this check before the refusal is ever reached. The journal row
    # in plan/journal/shared-leniency-hides-the-defect.md records why that
    # matters: this is the only test with a peer that is not a second copy of
    # ze, and a ze-to-ze test shares every leniency it would need to detect.
    wait_swan_log("negotiated TLS 1.2")
    wait_swan_log("EAP method EAP_TLS succeeded, MSK established")

    # 2. Ze refuses at the export, and the refusal is attributed.
    wait_ze_log("cannot export the RFC 5216 Section 2.3 MSK")
    logs = docker_logs_all(ZE_CONTAINER)
    for fact in REFUSAL_FACTS:
        if fact not in logs:
            raise AssertionError("ze's EAP-TLS refusal does not state: %s" % fact)
    log_pass("ze's EAP-TLS refusal states all %d facts" % len(REFUSAL_FACTS))

    # 3. Fail closed. No key material was derived, so no SA may exist.
    #
    # strongSwan's own EAP method SUCCEEDED at step 1: it exports the MSK on a
    # TLS 1.2 session without RFC 7627 and ze does not. So the peer is willing,
    # and the only thing keeping an SA off the wire is ze refusing to continue
    # with an MSK it never derived. An SA here would carry keys the two ends
    # cannot agree on, which is the failure this assertion exists to catch.
    check_xfrm_sa_count(ZE_CONTAINER, 0)
    check_xfrm_sa_count(SWAN_CONTAINER, 0)

    log_pass(
        "EAP-TLS over TLS 1.2 without RFC 7627: ze completed the handshake, "
        "refused the MSK export with an attributed error, and installed no SA"
    )
