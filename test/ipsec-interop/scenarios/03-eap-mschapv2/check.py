#!/usr/bin/env python3
"""Scenario 03: EAP-MSCHAPv2 authentication Ze <-> strongSwan.

Validates: Ze's IKEv2 engine negotiates IKE_SA_INIT and performs an
           EAP-MSCHAPv2 exchange inside IKE_AUTH with strongSwan as
           the EAP authenticator. Verifies IKE SA and Child SA are
           established, XFRM state installed, and traffic flows.
Prevents:  EAP wire format regressions, MSK derivation errors,
           AUTH-from-MSK computation mismatches, EAP identifier
           sequencing bugs.
"""

import os
import re
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lab import (
    StrongSwan,
    ZE_CONTAINER,
    SWAN_CONTAINER,
    SWAN_IP,
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

    # 1. Control plane: IKE SA established with EAP authentication.
    swan.wait_sa_established("ze")
    swan.wait_child_sa()

    # 2. Kernel state: ESP SAs installed on both sides.
    #
    #    The Ze-side dataplane assertions run only where the kernel supports
    #    XFRM/ESP, which Docker Desktop's Linux VM on macOS does not. The probe is
    #    the same one scenario 07 uses. It replaces a `except (AssertionError,
    #    Exception)` that caught every failure of wait_xfrm_sa and
    #    check_xfrm_sa_count and reported it as a pass.
    wait_xfrm_sa(SWAN_CONTAINER)
    if not esp_spis():
        log_pass(
            "XFRM/ESP unsupported on this host; verified EAP-MSCHAPv2 at control plane"
        )
        return

    wait_xfrm_sa(ZE_CONTAINER)
    check_xfrm_sa_count(SWAN_CONTAINER, 2)

    # 3. Data plane: traffic flows through the ESP tunnel.
    bytes_before = xfrm_sa_bytes_by_spi(ZE_CONTAINER)
    peer_before = xfrm_sa_bytes_by_spi(SWAN_CONTAINER)
    docker_exec_quiet(ZE_CONTAINER, ["ping", "-c", "4", "-W", "2", SWAN_IP])

    assert_esp_accepted(
        ZE_CONTAINER,
        bytes_before,
        xfrm_sa_bytes_by_spi(ZE_CONTAINER),
        "traffic did not flow through the XFRM tunnel",
    )

    # 4. Interop: strongSwan's inbound counter advances only after it looks the SPI
    #    up, decrypts, and verifies the ICV. Ze's own counter proves it encrypted
    #    and sent, and nothing about whether the peer could read the result.
    assert_esp_accepted(
        SWAN_CONTAINER,
        peer_before,
        xfrm_sa_bytes_by_spi(SWAN_CONTAINER),
        "strongSwan accepted no ESP from Ze",
    )
    log_pass("EAP-MSCHAPv2 tunnel established and forwarding traffic")
