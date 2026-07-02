#!/usr/bin/env python3
"""Process plugin that announces AS112 covering prefixes to all configured peers.

Announces the real AS112 anycast covering prefixes (RFC 7534 Section 3,
spec-as112-3 finding H3) with no explicit AS_PATH, so ze synthesizes AS_PATH
from each destination peer's own effective local AS (session > asn > local),
honoring any per-peer asn.local/replace-as override
(internal/component/bgp/reactor/reactor_api_batch.go writeASPath/
writeMandatoryAttrs). This proves AC-6/AC-7: the frr-origin112 peer-group
(asn.local 112, local-options replace-as) presents AS_PATH = [112]; the
bird-internal peer-group (no override) presents AS_PATH = [ze's real local
AS] -- both from the SAME injected route, controlled independently per
peer-group.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let both BGP sessions fully establish.
time.sleep(1)

# AS112 IPv4 covering prefixes (RFC 7534 Section 3 / spec-as112-3 H3).
flush(
    "peer * update text origin igp nhop 172.30.0.2 "
    "nlri ipv4/unicast add 192.175.48.0/24\n"
)
time.sleep(0.1)
flush(
    "peer * update text origin igp nhop 172.30.0.2 "
    "nlri ipv4/unicast add 192.31.196.0/24\n"
)

# Keep plugin alive for the check script.
wait_for_shutdown(timeout=120)
