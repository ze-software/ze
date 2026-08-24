#!/usr/bin/env python3
"""Scenario invalid-ke-retry: Ze retries IKE_SA_INIT under the group strongSwan names.

Validates: RFC 7296 Section 1.2. Ze guesses dh-group 14 on its first
           IKE_SA_INIT. strongSwan accepts only ecp256, so it answers
           INVALID_KE_PAYLOAD naming group 19, and Ze MUST retry the
           IKE_SA_INIT under the corrected group. The tunnel then
           establishes and carries ESP that strongSwan accepts.
Prevents:  the defect this scenario was written for. Before the retry
           existed, an INVALID_KE_PAYLOAD response fell to the
           completeness gate in handleSAInitResponse and killed the SA;
           runInitiator then re-sent the SAME wrong-group request until
           its retransmit budget was spent, and the next cycle rebuilt
           the same group from the same config index. Ze could NEVER
           establish with a peer preferring another Diffie-Hellman group.

Discriminating power: establishment alone is not evidence. A strongSwan
whose proposal happened to accept modp2048 would establish without the
retry ever running, and reverting retrySAInit would leave this scenario
green. The assertions below therefore require BOTH peers to say the
correction happened: strongSwan must have emitted INVALID_KE_PAYLOAD, and
Ze must have logged the retry that answered it.
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
    docker_logs_all,
    log_pass,
    wait_xfrm_sa,
    wait_ze_log,
    xfrm_sa_bytes_by_spi,
)


def check():
    swan = StrongSwan()

    # 1. Ze acted on the notify. This is the row under test, and it is asserted
    #    before establishment so a failure names the retry rather than the tunnel.
    wait_ze_log("retrying IKE_SA_INIT")
    ze_logs = docker_logs_all(ZE_CONTAINER)
    if "cause=invalid-ke-payload" not in ze_logs:
        raise AssertionError(
            "Ze retried its IKE_SA_INIT for some other reason than INVALID_KE_PAYLOAD; "
            "the corrected-group path did not run"
        )

    # 2. The PEER really sent it. Without this the scenario would pass over a
    #    strongSwan whose proposal happened to accept Ze's first guess, and the
    #    retry would never have been exercised at all.
    #    charon names the notify by its abbreviation on the wire-level line, never as
    #    "INVALID_KE_PAYLOAD", so the needle is the generated response itself. Scenario
    #    18 reads N(COOKIE) the same way.
    swan_logs = docker_logs_all(SWAN_CONTAINER)
    if "generating IKE_SA_INIT response 0 [ N(INVAL_KE) ]" not in swan_logs:
        raise AssertionError(
            "strongSwan never sent INVALID_KE_PAYLOAD, so this scenario proves nothing "
            "about the retry; check that its proposal names a group Ze does not guess first"
        )

    # 3. Control plane: the corrected exchange completes and authenticates. The AUTH
    #    payload is computed over the IKE_SA_INIT message, so this also proves the
    #    retried request was re-anchored as the signed octets.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 4. Kernel state and data plane, as scenario psk-site-to-site asserts them.
    wait_xfrm_sa(ZE_CONTAINER)
    wait_xfrm_sa(SWAN_CONTAINER)

    ze_before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    peer_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)
    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])
    assert_esp_accepted(
        ZE_CONTAINER,
        ze_before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "traffic did not flow through the tunnel built by the retried IKE_SA_INIT",
    )
    assert_esp_accepted(
        SWAN_CONTAINER,
        peer_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP from the tunnel built by the retried IKE_SA_INIT",
    )
    log_pass(
        "Ze retried IKE_SA_INIT under the group strongSwan named, and the tunnel forwards"
    )
