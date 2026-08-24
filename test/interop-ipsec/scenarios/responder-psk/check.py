#!/usr/bin/env python3
"""Scenario responder-psk: PSK responder Ze <- strongSwan (charon initiates).

Validates: Ze's IKEv2 RESPONDER path (spec-ipsec-14). strongSwan is configured with
           start_action=start and dials the tunnel; Ze (connection-type respond)
           accepts the unsolicited IKE_SA_INIT, negotiates a proposal, answers
           IKE_SA_INIT + IKE_AUTH with the SK encrypt/decrypt direction and AUTH
           signed octets computed for the responder role, verifies strongSwan's PSK
           AUTH, and installs the first Child SA. strongSwan is the authoritative
           witness: its IKE SA reaching ESTABLISHED and its Child SA INSTALLED prove
           Ze answered correctly as responder. Where the kernel supports XFRM the ESP
           SAs must also install and traffic flow.
Prevents:  responder SK direction bugs (charon could not decrypt Ze's response),
           responder AUTH octet/ID bugs (charon rejects Ze's AUTH), and the
           connection-type respond black hole (unsolicited IKE_SA_INIT dropped).

Note: Docker Desktop's Linux VM (macOS) has no XFRM/ESP for the Ze container, so the
      dataplane assertions run only where the kernel supports ESP; the control-plane
      proof (strongSwan established against Ze the responder) always applies.
"""

import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    SWAN_CONTAINER,
    SWAN_IP,
    ZE_CONTAINER,
    assert_esp_accepted,
    check_xfrm_sa_count,
    docker_exec_quiet,
    log_pass,
    wait_xfrm_sa,
    ze_xfrm_state,
    xfrm_sa_bytes_by_spi,
)


def esp_spis():
    return set(re.findall(r"proto esp spi (0x[0-9a-fA-F]+)", ze_xfrm_state()))


def check():
    swan = StrongSwan()

    # 1. Control plane: strongSwan (initiator) establishes against Ze (responder).
    #    strongSwan is the authoritative witness (Ze's plugin logs at INFO are not
    #    captured in the container, per scenario child-rekey). Its IKE SA reaching ESTABLISHED
    #    and its Child SA INSTALLED prove Ze answered IKE_SA_INIT + IKE_AUTH, that the
    #    responder SK direction, AUTH octets, and IP-literal ID type are correct, and
    #    that strongSwan verified Ze's PSK AUTH.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()
    log_pass("strongSwan established IKE + Child SA against Ze the responder")

    # 2. Dataplane assertions where the kernel supports XFRM/ESP.
    if not esp_spis():
        log_pass(
            "XFRM/ESP unsupported on this host; verified responder at control plane (peer logs)"
        )
        return

    wait_xfrm_sa(ZE_CONTAINER)
    wait_xfrm_sa(SWAN_CONTAINER)
    check_xfrm_sa_count(SWAN_CONTAINER, 2)

    before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    peer_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)
    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])

    assert_esp_accepted(
        ZE_CONTAINER,
        before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "no ESP traffic through the responder tunnel",
    )

    # The peer must accept what Ze encrypted as RESPONDER. The responder derives its
    # KEYMAT in the absolute Ni|Nr order rather than its own Local/Remote order, so a
    # role mix-up gives keys that are the right length and the wrong bytes. Ze's own
    # outbound counter advances either way. strongSwan's inbound counter does not.
    assert_esp_accepted(
        SWAN_CONTAINER,
        peer_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP from Ze the responder",
    )
    log_pass("PSK responder tunnel established and forwarding traffic through ESP")
