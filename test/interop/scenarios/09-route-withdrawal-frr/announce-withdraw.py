#!/usr/bin/env python3
"""Process plugin that announces 3 routes, then withdraws the middle one.

Announces 10.10.0.0/24, 10.10.1.0/24, 10.10.2.0/24, waits 5 seconds,
then withdraws 10.10.1.0/24. The check script verifies FRR sees the
withdrawal and removes the route while keeping the other two.
"""

import time

from ze_api import flush, ready, wait_for_shutdown

ready()

# Let BGP session fully establish.
time.sleep(1)

# Announce 3 routes.
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

# Wait for routes to propagate to FRR.
time.sleep(5)

# Withdraw the middle route.
flush("peer * update text nlri ipv4/unicast del 10.10.1.0/24\n")

# Keep plugin alive for the check script to verify withdrawal.
wait_for_shutdown(timeout=120)
