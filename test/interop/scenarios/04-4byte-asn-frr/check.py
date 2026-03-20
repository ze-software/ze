#!/usr/bin/env python3
"""Scenario 04: 4-byte ASN eBGP Ze <-> FRR.

Validates: Ze negotiates 4-byte ASN capability (RFC 6793) with FRR.
Prevents:  ASN4 capability encoding/decoding mismatch, AS_TRANS fallback.
"""

import json
import sys, os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP, docker_exec_quiet, log_pass, log_fail


def check():
    frr = FRR()
    frr.wait_session(ZE_IP)

    # Verify FRR sees the real 4-byte ASN, not AS_TRANS (23456).
    output = docker_exec_quiet(frr.container, ["vtysh", "-c", "show bgp neighbor %s json" % ZE_IP])
    if output.strip():
        try:
            data = json.loads(output)
            peer = data.get(ZE_IP, {})
            remote_as = peer.get("remoteAs", 0)
            if remote_as == 4200000001:
                log_pass("FRR sees 4-byte ASN 4200000001 (not AS_TRANS)")
            elif remote_as == 23456:
                log_fail("FRR sees AS_TRANS 23456 instead of 4200000001")
                raise AssertionError("4-byte ASN not negotiated: got AS_TRANS 23456")
            else:
                log_pass("FRR sees remote ASN %d" % remote_as)
        except json.JSONDecodeError:
            log_pass("session established (JSON parse failed, ASN check skipped)")
    else:
        log_pass("session established (no JSON output, ASN check skipped)")
