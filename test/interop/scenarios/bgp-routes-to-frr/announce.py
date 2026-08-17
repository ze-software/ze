#!/usr/bin/env python3
"""Process plugin that announces 3 routes via Ze's JSON RPC protocol.

Completes the 5-stage plugin handshake, then sends UPDATE commands
to inject routes into all peers.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Small delay to let BGP session fully establish before sending routes.
time.sleep(1)

flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.0.0/24\n"
)
time.sleep(0.1)
flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.1.0/24\n"
)
time.sleep(0.1)
flush(
    "peer * update text origin igp path 65001 nhop 172.30.0.2 nlri ipv4/unicast add 10.10.2.0/24\n"
)

# Keep plugin alive long enough for the check script to verify routes.
# Ze withdraws routes when a process plugin exits.
wait_for_shutdown(timeout=120)
