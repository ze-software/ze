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
    SWAN_IP,
    ZE_CONTAINER,
    assert_esp_accepted,
    docker_exec_quiet,
    log_pass,
    wait_xfrm_sa,
    xfrm_sa_bytes_by_spi,
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

    # 3. Data plane: the SAs carry ESP that the peer ACCEPTS.
    #
    # An XFRM SA is necessary and not sufficient. It proves the two ends installed
    # keys, never that those keys agree: an MSK exported on the wrong side of the
    # RFC 5216 / RFC 9190 split, or KEYMAT derived in the wrong nonce order, installs
    # a pair of SAs of the right shape whose bytes no peer can decrypt. This scenario
    # stopped at the SA and reported a pass for that case until 2026-08-15.
    #
    # A ping alone proves nothing either: both containers sit on one Docker bridge, so
    # ICMP arrives with the tunnel down. The oracle is the per-SPI byte counter, read
    # before and after a blocking ping rather than after a sleep, following scenarios 01
    # and 07. Nothing here waits on elapsed time (spec R-2).
    #
    # Discrimination, measured 2026-08-15: with one octet of the outbound ESP key flipped
    # in installChildSA (internal/component/ike/engine/child.go), so that both ends still
    # install SAs whose keys do not agree, the two wait_xfrm_sa calls above still passed
    # and so did ze's own counter. Only the strongSwan assertion below went red.
    ze_before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    swan_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)

    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])

    assert_esp_accepted(
        ZE_CONTAINER,
        ze_before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "no traffic through the EAP-TLS tunnel",
    )

    # The peer's inbound counter is the interop assertion. Ze's own counter advances
    # whatever key it encrypted with; strongSwan's advances only after it looks the SPI
    # up, decrypts, and verifies the tag, so this is where the EAP-TLS MSK the two sides
    # derived is proven to be the same key.
    assert_esp_accepted(
        SWAN_CONTAINER,
        swan_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP over the EAP-TLS tunnel",
    )

    log_pass("EAP-TLS tunnel established with certificate authentication, ESP accepted")
